#!/bin/bash -eux

if [ $# -ne 3 ]; then
  echo "Usage: $0 <SRIOV_LINK_NAME> <SRIOV_LINK_ADDR> <SECCOMP>"
  exit 1
fi

SRIOVLINKNAME=$1
SRIOVLINKADDR=$2
SECCOMP=$3

# SRIOVLINKNAME="enp65s0v0"
# SRIOVLINKADDR="192.168.20.20/24"

NETNSNAME="tiaccoon-test"
NERDCTLOPTS=""
if [ -n "$SECCOMP" ]; then
  NERDCTLOPTS="--cap-add=SYS_PTRACE --security-opt seccomp=/var/lib/kubelet/seccomp/$SECCOMP "
fi

NERDCTLOPTS=${NERDCTLOPTS}"--ulimit memlock=-1 " # ulimit -l unlimited See rdma-core/Documentation/libibverbs.md
NERDCTLOPTS=${NERDCTLOPTS}"--ulimit stack=-1 " # ulimit -s unlimited Prevent segmentation fault at huge send/recv data

NERDCTLMNT="--volume $HOME/src/github.com/hiroyaonoe:$HOME/src/github.com/hiroyaonoe"
NERDCTLMNT=${NERDCTLMNT}" --volume /tmp/tiaccoon:/tmp/tiaccoon" # Tiaccoon create sock files on host, but my benchmark script on container need to remove the sock files.

# NERDCTLDEV="--volume /dev/infiniband:/dev/infiniband "
NERDCTLDEV=""
for file in /dev/infiniband/*; do
  NERDCTLDEV=${NERDCTLDEV}"--device=$file "
done

if [ ! -d /tmp/tiaccoon ]; then
  mkdir /tmp/tiaccoon
fi

CONTAINERID=$(sudo nerdctl create -it $NERDCTLMNT $NERDCTLDEV $NERDCTLOPTS tiaccoon-test /bin/bash)

sudo nerdctl start $CONTAINERID

if [ "$SRIOVLINKNAME" != "" ]; then
  PID=$(sudo nerdctl inspect $CONTAINERID --format '{{.State.Pid}}')

  NETNSSRC="/proc/$PID/ns/net"
  NETNSDST="/var/run/netns/$NETNSNAME"

  sudo unlink $NETNSDST || true
  sudo ln -s $NETNSSRC $NETNSDST

  sudo ip link set dev $SRIOVLINKNAME netns $NETNSNAME
  sudo ip -n $NETNSNAME link set dev $SRIOVLINKNAME up
  sudo ip -n $NETNSNAME addr add $SRIOVLINKADDR dev $SRIOVLINKNAME
fi

sudo nerdctl exec -it $CONTAINERID /bin/bash || true

sudo nerdctl kill $CONTAINERID
