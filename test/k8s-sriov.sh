#!/bin/bash -eux

if [ $# -ne 6 ]; then
  echo "Usage: $0 <SRIOV_LINK_NAME> <SRIOV_LINK_ADDR> <SECCOMP> <NAME> <NODE> <APP>"
  exit 1
fi

SRIOVLINKNAME=$1
SRIOVLINKADDR=$2
SECCOMP=$3
NAME=$4
NODE=$5
APP_RAW=$6

NETNSNAME="tiaccoon-test"

TMPL_FILE=test/pod-seccomp.tmpl.yaml
if [ "$SECCOMP" == "" ]; then
  TMPL_FILE=test/pod.tmpl.yaml
fi

kubectl delete -n tiaccoon-test pod $NAME || true

if [ ! -d /tmp/tiaccoon ]; then
  mkdir /tmp/tiaccoon
fi

if [ "$APP_RAW" == "netperf" ]; then
  APP="test"
else
  APP=$APP_RAW
fi

cat $TMPL_FILE | sed -e "s/\$(NAME)/$NAME/g" -e "s/\$(SECCOMP)/$SECCOMP/g" -e "s/\$(NODE)/$NODE/g" -e "s/\$(APP)/$APP/g" | kubectl apply -f -

CONTAINERID=null
while [ "$CONTAINERID" == "null" ]; do
  sleep 1
  # CONTAINERID=$(kubectl get -n tiaccoon-test pod $NAME -o json | jq -r '."metadata"."annotations"."cni.projectcalico.org/containerID"') # pause container
  CONTAINERID=$(kubectl get -n tiaccoon-test pod $NAME -o json | jq -r '."status"."containerStatuses"[0]."containerID"' | sed 's/containerd:\/\///') # tiaccoon-test container
done

if [ "$SRIOVLINKNAME" != "" ]; then
  PID=$(sudo nerdctl --namespace k8s.io inspect $CONTAINERID --format '{{.State.Pid}}')

  NETNSSRC="/proc/$PID/ns/net"
  NETNSDST="/var/run/netns/$NETNSNAME"

  sudo unlink $NETNSDST || true
  sudo ln -s $NETNSSRC $NETNSDST

  sudo ip link set dev $SRIOVLINKNAME netns $NETNSNAME
  sudo ip -n $NETNSNAME link set dev $SRIOVLINKNAME up
  sudo ip -n $NETNSNAME addr add $SRIOVLINKADDR dev $SRIOVLINKNAME
fi

kubectl exec -n tiaccoon-test -it $NAME -- /bin/bash || true
# sudo nerdctl --namespace k8s.io exec -it --privileged $CONTAINERID /bin/bash || true

kubectl delete -n tiaccoon-test pod $NAME || true
