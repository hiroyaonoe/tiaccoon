#!/bin/bash -eu

if [ $# -le 7 ]; then
	echo "Usage: $0 <prefix> <protocol> <size_pows,> <buf_size_idxs,> <type> <tiaccoon> <name> [ip addr if other protocol]"
	exit 1
fi

PREFIX=$1
PROTOCOL=$2
SIZE_POWS=$3
BUF_SIZE_IDXS=$4
TYPE=$5
TIACCOON=$6
NAME=$7
IPADDR=$8

CONFID="95,5"
ITERS="10,3"
TIME=10
SIZES=($(python3 -c "print(' '.join(map(lambda x:str(2**x),range(21))))"))
SIZE_POWS=(${SIZE_POWS//,/ })
# BUF_SIZE=212992 # 208 KB linuxのmax
# BUF_SIZE=65536 # 2^16
# BUF_SIZE=2304 # 2^8 * 3^2 min send buf size
# TODO: buf_size検討
# https://linuxjm.sourceforge.io/html/LDP_man-pages/man7/tcp.7.html
# https://hana-shin.hatenablog.com/entry/2022/10/10/212039
BUF_SIZES=(2304 65536 212992) # 425984 2129920 これ以上大きくしても変わらない
BUF_SIZE_IDXS=(${BUF_SIZE_IDXS//,/ })

RDMA=""
CLEANUP=""
TESTOPTS=""

PROTOCOL_DIR=$NAME

if [ "$PROTOCOL" = "tcp_remote" ]; then
	ADDR="192.168.20.3,4 -p 12865"
	TESTNAME=TCP
	# PROTOCOL_DIR="remote-tcp"
	TESTPORT="-P ,22865"
elif [ "$PROTOCOL" = "udp_remote" ]; then
	ADDR="192.168.20.3,4 -p 12865"
	TESTNAME=UDP
	# PROTOCOL_DIR="remote-udp"
	TESTPORT="-P ,22865"
elif [ "$PROTOCOL" = "rdma_stream_remotepf" ]; then
	ADDR="192.168.20.3,4 -p 12865"
	TESTNAME=TCP
	# PROTOCOL_DIR="remote-rdma_stream"
	TESTPORT="-P ,22865"
	RDMA="env LD_PRELOAD=/home/onoe/src/github.com/hiroyaonoe/rdma-core/build/lib/librspreload.so "
elif [ "$PROTOCOL" = "rdma_dgram_remotepf" ]; then
	ADDR="192.168.20.3,4 -p 12865"
	TESTNAME=UDP
	# PROTOCOL_DIR="remote-rdma_dgram"
	TESTPORT="-P ,22865"
	RDMA="env LD_PRELOAD=/home/onoe/src/github.com/hiroyaonoe/rdma-core/build/lib/librspreload.so "
elif [ "$PROTOCOL" = "rdma_stream_remotevf" ]; then
	ADDR="192.168.20.30,4 -p 12865"
	TESTNAME=TCP
	# PROTOCOL_DIR="remote-rdma_stream"
	TESTPORT="-P ,22865"
	RDMA="env LD_PRELOAD=/home/onoe/src/github.com/hiroyaonoe/rdma-core/build/lib/librspreload.so "
elif [ "$PROTOCOL" = "rdma_dgram_remotevf" ]; then
	ADDR="192.168.20.30,4 -p 12865"
	TESTNAME=UDP
	# PROTOCOL_DIR="remote-rdma_dgram"
	TESTPORT="-P ,22865"
	RDMA="env LD_PRELOAD=/home/onoe/src/github.com/hiroyaonoe/rdma-core/build/lib/librspreload.so "
elif [ "$PROTOCOL" = "rdma_stream_local" ]; then
	ADDR="192.168.20.21,4 -p 12865"
	TESTNAME=TCP
	# PROTOCOL_DIR="remote-rdma_stream"
	TESTPORT="-P ,22865"
	RDMA="env LD_PRELOAD=/home/onoe/src/github.com/hiroyaonoe/rdma-core/build/lib/librspreload.so "
elif [ "$PROTOCOL" = "rdma_dgram_local" ]; then
	ADDR="192.168.20.21,4 -p 12865"
	TESTNAME=UDP
	# PROTOCOL_DIR="remote-rdma_dgram"
	TESTPORT="-P ,22865"
	RDMA="env LD_PRELOAD=/home/onoe/src/github.com/hiroyaonoe/rdma-core/build/lib/librspreload.so "
elif [ "$PROTOCOL" = "tcp_local" ]; then
	ADDR="127.0.0.1,4 -p 12865"
	TESTNAME=TCP
	# PROTOCOL_DIR="local-tcp"
	TESTPORT="-P ,22865"
elif [ "$PROTOCOL" = "udp_local" ]; then
	ADDR="127.0.0.1,4 -p 12865"
	TESTNAME=UDP
	# PROTOCOL_DIR="local-udp"
	TESTPORT="-P ,22865"
elif [ "$PROTOCOL" = "unix_stream" ]; then
	ADDR="/tmp/tiaccoon/netperf-remote.sock,af_unix -L /tmp/tiaccoon/netperf-local.sock,af_unix"
	TESTNAME=STREAM
	# PROTOCOL_DIR="local-unix_stream"
	TESTPORT=""
	CLEANUP="rm /tmp/tiaccoon/netperf-local.sock"
elif [ "$PROTOCOL" = "unix_dgram" ]; then
	ADDR="/tmp/tiaccoon/netperf-remote.sock,af_unix -L /tmp/tiaccoon/netperf-local.sock,af_unix"
	TESTNAME=DG
	# PROTOCOL_DIR="local-unix_dgram"
	TESTPORT=""
	CLEANUP="rm /tmp/tiaccoon/netperf-local.sock"
elif [ "$PROTOCOL" = "other" ]; then
	ADDR=${IPADDR}",4 -p 12865"
	TESTNAME=TCP
	# PROTOCOL_DIR=$NAME
	TESTPORT="-P ,22865"
else
	echo "Invalid protocol: $PROTOCOL"
	exit 1
fi

if [ "$TIACCOON" = "true" ]; then
	ADDR="10.0.10.50,4 -p 12865"
	TESTPORT="-P ,22865"
	if [ "$PROTOCOL" = "unix_stream" ]; then
		TESTNAME=TCP
	elif [ "$PROTOCOL" = "unix_dgram" ]; then
		TESTNAME=UDP
	fi
elif [ "$TIACCOON" = "false" ]; then
	:
else
	echo "Invalid tiaccoon: $TIACCOON"
	exit 1
fi

TIMESTAMP=$(date +%Y%m%d-%H%M%S)

ulimit -s unlimited
ulimit -l unlimited

for BUF_SIZE_IDX in ${BUF_SIZE_IDXS[@]}; do
	BUF_SIZE=${BUF_SIZES[$BUF_SIZE_IDX]}
	for SIZE_POW in ${SIZE_POWS[@]}; do
		SIZE=${SIZES[$SIZE_POW]}

		if [ "$TYPE" = "STREAM" ]; then
			PARAMS="-s $BUF_SIZE -S $BUF_SIZE -m $SIZE"
		elif [ "$TYPE" = "RR" ]; then
			PARAMS="-s $BUF_SIZE -S $BUF_SIZE -r $SIZE,$SIZE"
		elif [ "$TYPE" = "CC" ]; then
			PARAMS=""
		else
			echo "Invalid type: $TYPE"
			exit 1
		fi

		OUTPUT_DIR="data/result/$PREFIX/$TYPE/$BUF_SIZE/$PROTOCOL_DIR/$SIZE"
		mkdir -p $OUTPUT_DIR

		OUTPUT=$OUTPUT_DIR/$TIMESTAMP.log
		NETPERF_CMD="${RDMA}netperf -c -C -H $ADDR -I $CONFID -i $ITERS -j -l $TIME -t ${TESTNAME}_${TYPE} -- $TESTOPTS $TESTPORT $PARAMS"

		echo "OUTPUT: $OUTPUT"
		echo "NETPERF_CMD: $NETPERF_CMD"

		echo "\$ $NETPERF_CMD" >> $OUTPUT

		$NETPERF_CMD |& tee -a $OUTPUT

		if [ "$CLEANUP" != "" ]; then
			echo "CLEANUP: $CLEANUP"
			$CLEANUP
		fi
		sleep 1
	done
done

echo "notify-iterm-ok"

# TCP_NODELAY
# TCP_CC
# STREAMの-M $SIZE

# go run organize/main.go -prefix $PREFIX -input data/result -output data/organize
