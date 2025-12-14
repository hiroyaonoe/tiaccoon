#!/bin/bash -eu

if [ $# -le 3 ]; then
	echo "Usage: $0 <prefix> <protocol> <tiaccoon> <name> [ip addr if other protocol]"
	exit 1
fi

PREFIX=$1
PROTOCOL=$2
TIACCOON=$3
NAME=$4
IPADDR=$5

COUNT=10000

RDMA=""

if [ "$PROTOCOL" = "tcp_remote" ]; then
	ADDR="192.168.20.3:22865"
	TESTNAME=TCP
elif [ "$PROTOCOL" = "rdma_stream_remotepf" ]; then
	ADDR="192.168.20.3:22865"
	TESTNAME=TCP
	RDMA="env LD_PRELOAD=/home/onoe/src/github.com/hiroyaonoe/rdma-core/build/lib/librspreload.so "
elif [ "$PROTOCOL" = "rdma_stream_remotevf" ]; then
	ADDR="192.168.20.30:22865"
	TESTNAME=TCP
	RDMA="env LD_PRELOAD=/home/onoe/src/github.com/hiroyaonoe/rdma-core/build/lib/librspreload.so "
elif [ "$PROTOCOL" = "rdma_stream_local" ]; then
	ADDR="192.168.20.21:22865"
	TESTNAME=TCP
	RDMA="env LD_PRELOAD=/home/onoe/src/github.com/hiroyaonoe/rdma-core/build/lib/librspreload.so "
elif [ "$PROTOCOL" = "tcp_local" ]; then
	ADDR="127.0.0.1:22865"
	TESTNAME=TCP
elif [ "$PROTOCOL" = "unix_stream" ]; then
	ADDR="/tmp/tiaccoon/perf-cc.sock"
	TESTNAME=UNIX
elif [ "$PROTOCOL" = "other" ]; then
	ADDR=${IPADDR}":22865"
	TESTNAME=TCP
else
	echo "Invalid protocol: $PROTOCOL"
	exit 1
fi

if [ "$TIACCOON" = "true" ]; then
	ADDR="10.0.10.50:22865"
	TESTNAME=TCP
elif [ "$TIACCOON" = "false" ]; then
	:
else
	echo "Invalid tiaccoon: $TIACCOON"
	exit 1
fi

TIMESTAMP=$(date +%Y%m%d-%H%M%S)

ulimit -s unlimited
ulimit -l unlimited


OUTPUT_DIR="data/result/$PREFIX/$NAME"
LOG_OUTPUT_DIR="data/log/$PREFIX/$NAME"
mkdir -p $OUTPUT_DIR
mkdir -p $LOG_OUTPUT_DIR

OUTPUT=$OUTPUT_DIR/$TIMESTAMP.txt
LOG_OUTPUT=$LOG_OUTPUT_DIR/$TIMESTAMP.log
PERF_CMD="${RDMA}./perf $TESTNAME $ADDR $COUNT $OUTPUT"

echo "OUTPUT: $OUTPUT"
echo "PERF_CMD: $PERF_CMD"

echo "\$ $PERF_CMD" >> $LOG_OUTPUT

$PERF_CMD |& tee -a $LOG_OUTPUT

echo "notify-iterm-ok"

# go run organize/main.go data/result data/organize $PREFIX
