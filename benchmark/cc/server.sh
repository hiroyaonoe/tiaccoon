#!/bin/bash -eu

if [ $# -le 1 ]; then
	echo "Usage: $0 <protocol> <tiaccoon>"
	exit 1
fi

PROTOCOL=$1
TIACCOON=$2

RDMA=""
CLEANUP=""

if [ "$PROTOCOL" = "tcp_remote" ]; then
	ADDR="22865"
	TESTNAME=TCP
elif [ "$PROTOCOL" = "rdma_stream_remotepf" ]; then
	ADDR="22865"
	TESTNAME=TCP
	RDMA="env LD_PRELOAD=/home/onoe/src/github.com/hiroyaonoe/rdma-core/build/lib/librspreload.so "
elif [ "$PROTOCOL" = "rdma_stream_remotevf" ]; then
	ADDR="22865"
	TESTNAME=TCP
	RDMA="env LD_PRELOAD=/home/onoe/src/github.com/hiroyaonoe/rdma-core/build/lib/librspreload.so "
elif [ "$PROTOCOL" = "rdma_stream_local" ]; then
	ADDR="22865"
	TESTNAME=TCP
	RDMA="env LD_PRELOAD=/home/onoe/src/github.com/hiroyaonoe/rdma-core/build/lib/librspreload.so "
elif [ "$PROTOCOL" = "tcp_local" ]; then
	ADDR="22865"
	TESTNAME=TCP
elif [ "$PROTOCOL" = "unix_stream" ]; then
	ADDR="/tmp/tiaccoon/perf-cc.sock"
	TESTNAME=UNIX
	CLEANUP="rm /tmp/tiaccoon/perf-cc.sock"
elif [ "$PROTOCOL" = "other" ]; then
	ADDR="22865"
	TESTNAME=TCP
else
	echo "Invalid protocol: $PROTOCOL"
	exit 1
fi

if [ "$TIACCOON" = "true" ]; then
	ADDR="22865"
	TESTNAME=TCP
elif [ "$TIACCOON" = "false" ]; then
	:
else
	echo "Invalid tiaccoon: $TIACCOON"
	exit 1
fi

ulimit -s unlimited
ulimit -l unlimited

if [ "$CLEANUP" != "" ]; then
	echo "CLEANUP: $CLEANUP"
	$CLEANUP || true
fi

SERVER_CMD="${RDMA}./server $TESTNAME $ADDR"
echo "SERVER_CMD: $SERVER_CMD"

$SERVER_CMD || true

if [ "$CLEANUP" != "" ]; then
	echo "CLEANUP: $CLEANUP"
	$CLEANUP
fi
