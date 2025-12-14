#!/bin/bash -eux

if [ $# -le 1 ]; then
	echo "Usage: $0 <protocol> <tiaccoon>"
	exit 1
fi

PROTOCOL=$1
TIACCOON=$2

RDMA=""

if [ "$PROTOCOL" = "tcp_remote" ]; then
	ADDR="0.0.0.0,4"
elif [ "$PROTOCOL" = "udp_remote" ]; then
	ADDR="0.0.0.0,4"
elif [ "$PROTOCOL" = "rdma_stream_remotepf" ]; then
	ADDR="0.0.0.0,4"
	RDMA="env LD_PRELOAD=/home/onoe/src/github.com/hiroyaonoe/rdma-core/build/lib/librspreload.so "
elif [ "$PROTOCOL" = "rdma_dgram_remotepf" ]; then
	ADDR="0.0.0.0,4"
	RDMA="env LD_PRELOAD=/home/onoe/src/github.com/hiroyaonoe/rdma-core/build/lib/librspreload.so "
elif [ "$PROTOCOL" = "rdma_stream_remotevf" ]; then
	ADDR="0.0.0.0,4"
	RDMA="env LD_PRELOAD=/home/onoe/src/github.com/hiroyaonoe/rdma-core/build/lib/librspreload.so "
elif [ "$PROTOCOL" = "rdma_dgram_remotevf" ]; then
	ADDR="0.0.0.0,4"
	RDMA="env LD_PRELOAD=/home/onoe/src/github.com/hiroyaonoe/rdma-core/build/lib/librspreload.so "
elif [ "$PROTOCOL" = "rdma_stream_local" ]; then
	ADDR="0.0.0.0,4"
	RDMA="env LD_PRELOAD=/home/onoe/src/github.com/hiroyaonoe/rdma-core/build/lib/librspreload.so "
elif [ "$PROTOCOL" = "rdma_dgram_local" ]; then
	ADDR="0.0.0.0,4"
	RDMA="env LD_PRELOAD=/home/onoe/src/github.com/hiroyaonoe/rdma-core/build/lib/librspreload.so "
elif [ "$PROTOCOL" = "tcp_local" ]; then
	ADDR="0.0.0.0,4"
elif [ "$PROTOCOL" = "udp_local" ]; then
	ADDR="0.0.0.0,4"
elif [ "$PROTOCOL" = "unix_stream" ]; then
	ADDR="/tmp/tiaccoon/netperf-remote.sock,af_unix"
elif [ "$PROTOCOL" = "unix_dgram" ]; then
	ADDR="/tmp/tiaccoon/netperf-remote.sock,af_unix"
elif [ "$PROTOCOL" = "other" ]; then
	ADDR="0.0.0.0,4"
else
	echo "Invalid protocol: $PROTOCOL"
	exit 1
fi

if [ "$TIACCOON" = "true" ]; then
	ADDR="0.0.0.0,4"
fi

ulimit -s unlimited
ulimit -l unlimited

NETSERVER_CMD="${RDMA}netserver -L $ADDR -D -f"
echo "NETSERVER_CMD: $NETSERVER_CMD"

$NETSERVER_CMD
