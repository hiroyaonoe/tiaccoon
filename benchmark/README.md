# benchmark

## Install Netperf
```bash
cd ../netperf
sudo apt update
sudo apt-get install git build-essential autoconf texinfo -y
./autogen.sh
./configure --enable-unixdomain=yes --enable-cpuutil=procstat
make CFLAGS=-fcommon
sudo make install
```

## Setup Kubernetes
```bash
# Setup containerd and nerdctl
kubeadm init --config kubernetes/kubeadm-config.yaml
kubectl apply -f kubernetes/namespace.yaml
kubectl apply -f kubernetes/device-manager.yaml
```

## Benchmark
### host-local-unix
#### Throughput and CPU time
```bash
cd netperf
./kill.sh

# on the same host
./netserver.sh unix_stream false
./netperf.sh prod2025011801 unix_stream 0,2,4,6,8,10,12,14,16,18,20 0,1,2 STREAM false host-local-unix ""
```
#### Latency
```bash
cd netperf
./kill.sh

# on the same host
./netserver.sh unix_stream false
./netperf.sh prod2025011801 unix_stream 0,2,4,6,8,10,12,14,16,18,20 0,1,2 RR false host-local-unix ""
```

#### Connection establishment and close time
```bash
cd cc

# on the same host
./server.sh unix_stream false
./benchmark.sh prod2025011801 unix_stream false host-local-unix ""
```

### host-local-tcp
#### Throughput and CPU time
```bash
cd netperf
./kill.sh

# on the same host
./netserver.sh tcp_local false
./netperf.sh prod2025011801 tcp_local 0,2,4,6,8,10,12,14,16,18,20 0,1,2 STREAM false host-local-tcp ""
```
#### Latency
```bash
cd netperf
./kill.sh

# on the same host
./netserver.sh tcp_local false
./netperf.sh prod2025011801 tcp_local 0,2,4,6,8,10,12,14,16,18,20 0,1,2 RR false host-local-tcp ""
```

#### Connection establishment and close time
```bash
cd cc

# on the same host
./server.sh tcp_local false
./benchmark.sh prod2025011801 tcp_local false host-local-tcp ""
```

### host-remote-tcp
#### Throughput and CPU time
```bash
cd netperf
./kill.sh

# on different hosts
./netserver.sh tcp_remote false
./netperf.sh prod2025011801 tcp_remote 0,2,4,6,8,10,12,14,16,18,20 0,1,2 STREAM false host-remote-tcp ""
```
#### Latency
```bash
cd netperf
./kill.sh

# on different hosts
./netserver.sh tcp_remote false
./netperf.sh prod2025011801 tcp_remote 0,2,4,6,8,10,12,14,16,18,20 0,1,2 RR false host-remote-tcp ""
```

#### Connection establishment and close time
```bash
cd cc

# on different hosts
./server.sh tcp_remote false
./benchmark.sh prod2025011801 tcp_remote false host-remote-tcp ""
```

### host-remote-roce
```bash
cd github.com/hiroyaonoe/rdma-core
git checkout master
bash ./build.sh
```

#### Throughput and CPU time
```bash
cd netperf
./kill.sh

# on different hosts
./netserver.sh rdma_stream_remotepf false
./netperf.sh prod2025011801 rdma_stream_remotepf 0,2,4,6,8,10,12,14,16,18,20 0,1,2 STREAM false host-remote-roce ""
```
#### Latency
```bash
cd netperf
./kill.sh

# on different hosts
./netserver.sh tcp_remote false
./netperf.sh prod2025011801 tcp_remote 0,2,4,6,8,10,12,14,16,18,20 0,1,2 RR false host-remote-tcp ""
```

#### Connection establishment and close time
```bash
cd cc

# on different hosts
./server.sh tcp_remote false
./benchmark.sh prod2025011801 tcp_remote false host-remote-tcp ""
```

### sriov-local-roce
```bash
cd github.com/hiroyaonoe/rdma-core
git checkout master
bash ./build.sh

./test/k8s-sriov.sh <SRIOV_VF_NAME> 192.168.20.20/24 "" netperf <NODENAME1>
./test/k8s-sriov.sh <SRIOV_VF_NAME> 192.168.20.21/24 "" netserver <NODENAME1>
```

#### Throughput and CPU time
```bash
cd netperf
./kill.sh

# on different containers on the same host
./netserver.sh rdma_stream_local false
./netperf.sh prod2025011801 rdma_stream_local 0,2,4,6,8,10,12,14,16,18,20 0,1,2 STREAM false sriov-local-roce ""

```
#### Latency
```bash
cd netperf
./kill.sh

# on different containers on the same host
./netserver.sh rdma_stream_local false
./netperf.sh prod2025011801 rdma_stream_local 0,2,4,6,8,10,12,14,16,18,20 0,1,2 RR false sriov-local-roce ""
```

#### Connection establishment and close time
```bash
cd cc

# on different containers on the same host
./server.sh rdma_stream_local  false
./benchmark.sh prod2025011801 rdma_stream_local  false sriov-local-roce ""
```


### sriov-remote-roce
```bash
cd github.com/hiroyaonoe/rdma-core
git checkout master
bash ./build.sh

./test/k8s-sriov.sh <SRIOV_VF_NAME> 192.168.20.20/24 "" netperf <NODENAME1>
./test/k8s-sriov.sh <SRIOV_VF_NAME> 192.168.20.30/24 "" netserver <NODENAME2>
```

#### Throughput and CPU time
```bash
cd netperf
./kill.sh

# on different containers on different hosts
./netserver.sh rdma_stream_remotevf false
./netperf.sh prod2025011801 rdma_stream_remotevf 0,2,4,6,8,10,12,14,16,18,20 0,1,2 STREAM false sriov-remote-roce ""

```
#### Latency
```bash
cd netperf
./kill.sh

# on different containers on different hosts
./netserver.sh rdma_stream_remotevf false
./netperf.sh prod2025011801 rdma_stream_remotevf 0,2,4,6,8,10,12,14,16,18,20 0,1,2 RR false sriov-remote-roce ""
```

#### Connection establishment and close time
```bash
cd cc

# on different containers on different hosts
./server.sh rdma_stream_remotevf  false
./benchmark.sh prod2025011801 rdma_stream_remotevf false sriov-remote-roce ""
```



### bridge-local-tcp
```bash
# Remove all CNI plugin except for nerdctl default CNI plugin

./test/k8s-sriov.sh "" "" "" netperf <NODENAME1>
./test/k8s-sriov.sh "" "" "" netserver <NODENAME1>
```

#### Throughput and CPU time
```bash
cd netperf
./kill.sh

# on different containers on the same host
./netserver.sh other false
./netperf.sh prod2025011801 other 0,2,4,6,8,10,12,14,16,18,20 0,1,2 STREAM false bridge-local-tcp <container's virtual IP address>
```

#### Latency
```bash
cd netperf
./kill.sh

# on different containers on the same host
./netserver.sh other false
./netperf.sh prod2025011801 other 0,2,4,6,8,10,12,14,16,18,20 0,1,2 RR false bridge-local-tcp <container's virtual IP address>
```

#### Connection establishment and close time
```bash
cd cc

# on different containers on the same host
./server.sh other  false
./benchmark.sh prod2025011801 other false bridge-local-tcp <container's virtual IP address>
```


### flannel-remote-tcp
```bash
kubectl apply -f kubernetes/kube-flannel.yaml

./test/k8s-sriov.sh "" "" "" netperf <NODENAME1>
./test/k8s-sriov.sh "" "" "" netserver <NODENAME2>
```

#### Throughput and CPU time
```bash
cd netperf
./kill.sh

# on different containers on different hosts
./netserver.sh other false
./netperf.sh prod2025011801 other 0,2,4,6,8,10,12,14,16,18,20 0,1,2 STREAM false flannel-remote-tcp "10.10.0.2"
```

#### Latency
```bash
cd netperf
./kill.sh

# on different containers on different hosts
./netserver.sh other false
./netperf.sh prod2025011801 other 0,2,4,6,8,10,12,14,16,18,20 0,1,2 RR false flannel-remote-tcp "10.10.0.3"
```

#### Connection establishment and close time
```bash
cd cc

# on different containers on different hosts
./server.sh other false
./benchmark.sh prod2025011801 other false flannel-remote-tcp "10.10.0.2"
```



### tiaccoon-local-unix
```bash
# Remove all CNI plugin except for nerdctl default CNI plugin

# Uncomment m.netperfUNIX(ctx, yourVIP) in pkg/tiaccoon/manage/manage.go
PKG_CONFIG_PATH=/usr/lib/x86_64-linux-gnu/pkgconfig make

./test/seccomp.json.sh /run/user/1005/tiaccoon-netperf.sock > /var/lib/kubelet/seccomp/tiaccoon-netperf.json
./test/seccomp.json.sh /run/user/1005/tiaccoon-netserver.sock > /var/lib/kubelet/seccomp/tiaccoon-netserver.json

# on the same host
sudo ./build/tiaccoon --socket /run/user/1005/tiaccoon-netperf.sock --log-level error --default-policy=allow --ip 10.0.10.40 --feature-rdma
sudo ./build/tiaccoon --socket /run/user/1005/tiaccoon-netserver.sock --log-level error --default-policy=allow --ip 10.0.10.50 --feature-rdma

./test/k8s-sriov.sh "" "" "seccomp-netperf.json" netperf <NODENAME1>
./test/k8s-sriov.sh "" "" "seccomp-netserver.json" netserver <NODENAME1>
```

#### Throughput and CPU time
```bash
cd netperf
./kill.sh

# on different containers on the same host
./netserver.sh other true
./netperf.sh prod2025011801 other 0,2,4,6,8,10,12,14,16,18,20 0,1,2 STREAM true tiaccoon-local-unix ""
```

#### Latency
```bash
cd netperf
./kill.sh

# on different containers on the same host
./netserver.sh other true
./netperf.sh prod2025011801 other 0,2,4,6,8,10,12,14,16,18,20 0,1,2 RR true tiaccoon-local-unix ""
```

#### Connection establishment and close time
```bash
cd cc

# on different containers on the same host
./server.sh other true
./benchmark.sh prod2025011801 other true tiaccoon-local-unix ""
```


### tiaccoon-local-tcp
```bash
# Remove all CNI plugin except for nerdctl default CNI plugin

# Uncomment m.netperfTCPLocal(ctx, yourVIP) in pkg/tiaccoon/manage/manage.go
PKG_CONFIG_PATH=/usr/lib/x86_64-linux-gnu/pkgconfig make

./test/seccomp.json.sh /run/user/1005/tiaccoon-netperf.sock > /var/lib/kubelet/seccomp/tiaccoon-netperf.json
./test/seccomp.json.sh /run/user/1005/tiaccoon-netserver.sock > /var/lib/kubelet/seccomp/tiaccoon-netserver.json

# on the same host
sudo ./build/tiaccoon --socket /run/user/1005/tiaccoon-netperf.sock --log-level error --default-policy=allow --ip 10.0.10.40 --feature-rdma
sudo ./build/tiaccoon --socket /run/user/1005/tiaccoon-netserver.sock --log-level error --default-policy=allow --ip 10.0.10.50 --feature-rdma

./test/k8s-sriov.sh "" "" "seccomp-netperf.json" netperf <NODENAME1>
./test/k8s-sriov.sh "" "" "seccomp-netserver.json" netserver <NODENAME1>
```

#### Throughput and CPU time
```bash
cd netperf
./kill.sh

# on different containers on the same host
./netserver.sh other true
./netperf.sh prod2025011801 other 0,2,4,6,8,10,12,14,16,18,20 0,1,2 STREAM true tiaccoon-local-tcp ""
```

#### Latency
```bash
cd netperf
./kill.sh

# on different containers on the same host
./netserver.sh other true
./netperf.sh prod2025011801 other 0,2,4,6,8,10,12,14,16,18,20 0,1,2 RR true tiaccoon-local-tcp ""
```

#### Connection establishment and close time
```bash
cd cc

# on different containers on the same host
./server.sh other true
./benchmark.sh prod2025011801 other true tiaccoon-local-tcp ""
```


### tiaccoon-local-roce
```bash
# Remove all CNI plugin except for nerdctl default CNI plugin

cd github.com/hiroyaonoe/rdma-core
git checkout tiaccoon
bash ./build.sh

# Uncomment m.netperfRDMALocal(ctx, yourVIP) in pkg/tiaccoon/manage/manage.go
PKG_CONFIG_PATH=/usr/lib/x86_64-linux-gnu/pkgconfig make

./test/seccomp.json.sh /run/user/1005/tiaccoon-netperf.sock > /var/lib/kubelet/seccomp/tiaccoon-netperf.json
./test/seccomp.json.sh /run/user/1005/tiaccoon-netserver.sock > /var/lib/kubelet/seccomp/tiaccoon-netserver.json

# on the same host
sudo ./build/tiaccoon --socket /run/user/1005/tiaccoon-netperf.sock --log-level error --default-policy=allow --ip 10.0.10.40 --feature-rdma
sudo ./build/tiaccoon --socket /run/user/1005/tiaccoon-netserver.sock --log-level error --default-policy=allow --ip 10.0.10.50 --feature-rdma

./test/k8s-sriov.sh <SRIOV_VF_NAME> 192.168.20.20/24 "seccomp-netperf.json" netperf <NODENAME1>
./test/k8s-sriov.sh <SRIOV_VF_NAME> 192.168.20.21/24 "seccomp-netserver.json" netserver <NODENAME1>
```

#### Throughput and CPU time
```bash
cd netperf
./kill.sh

# on different containers on the same host
./netserver.sh rdma_stream_local true
./netperf.sh prod2025011801 rdma_stream_local 0,2,4,6,8,10,12,14,16,18,20 0,1,2 STREAM true tiaccoon-local-roce ""
```

#### Latency
```bash
cd netperf
./kill.sh

# on different containers on the same host
./netserver.sh rdma_stream_local true
./netperf.sh prod2025011801 rdma_stream_local 0,2,4,6,8,10,12,14,16,18,20 0,1,2 RR true tiaccoon-local-roce ""

```

#### Connection establishment and close time
```bash
cd cc

# on different containers on the same host
./server.sh rdma_stream_local true
./benchmark.sh prod2025011801 rdma_stream_local true tiaccoon-local-roce ""
```




### tiaccoon-remote-tcp
```bash
# Remove all CNI plugin except for nerdctl default CNI plugin

# Uncomment m.netperfTCPRemote(ctx, yourVIP) in pkg/tiaccoon/manage/manage.go
PKG_CONFIG_PATH=/usr/lib/x86_64-linux-gnu/pkgconfig make

./test/seccomp.json.sh /run/user/1005/tiaccoon-netperf.sock > /var/lib/kubelet/seccomp/tiaccoon-netperf.json
./test/seccomp.json.sh /run/user/1005/tiaccoon-netserver.sock > /var/lib/kubelet/seccomp/tiaccoon-netserver.json

# on the same host
sudo ./build/tiaccoon --socket /run/user/1005/tiaccoon-netperf.sock --log-level error --default-policy=allow --ip 10.0.10.40 --feature-rdma
sudo ./build/tiaccoon --socket /run/user/1005/tiaccoon-netserver.sock --log-level error --default-policy=allow --ip 10.0.10.50 --feature-rdma

./test/k8s-sriov.sh "" "" "seccomp-netperf.json" netperf <NODENAME1>
./test/k8s-sriov.sh "" "" "seccomp-netserver.json" netserver <NODENAME2>
```

#### Throughput and CPU time
```bash
cd netperf
./kill.sh

# on different containers on differennt hosts
./netserver.sh other true
./netperf.sh prod2025011801 other 0,2,4,6,8,10,12,14,16,18,20 0,1,2 STREAM true tiaccoon-remote-tcp ""
```

#### Latency
```bash
cd netperf
./kill.sh

# on different containers on differennt hosts
./netserver.sh other true
./netperf.sh prod2025011801 other 0,2,4,6,8,10,12,14,16,18,20 0,1,2 RR true tiaccoon-remote-tcp ""
```

#### Connection establishment and close time
```bash
cd cc

# on different containers on differennt hosts
./server.sh other true
./benchmark.sh prod2025011801 other true tiaccoon-remote-tcp ""
```




### tiaccoon-remote-roce
```bash
# Remove all CNI plugin except for nerdctl default CNI plugin

cd github.com/hiroyaonoe/rdma-core
git checkout tiaccoon
bash ./build.sh

# Uncomment m.netperfRDMARemote(ctx, yourVIP) in pkg/tiaccoon/manage/manage.go
PKG_CONFIG_PATH=/usr/lib/x86_64-linux-gnu/pkgconfig make

./test/seccomp.json.sh /run/user/1005/tiaccoon-netperf.sock > /var/lib/kubelet/seccomp/tiaccoon-netperf.json
./test/seccomp.json.sh /run/user/1005/tiaccoon-netserver.sock > /var/lib/kubelet/seccomp/tiaccoon-netserver.json

# on the same host
sudo ./build/tiaccoon --socket /run/user/1005/tiaccoon-netperf.sock --log-level error --default-policy=allow --ip 10.0.10.40 --feature-rdma
sudo ./build/tiaccoon --socket /run/user/1005/tiaccoon-netserver.sock --log-level error --default-policy=allow --ip 10.0.10.50 --feature-rdma

./test/k8s-sriov.sh <SRIOV_VF_NAME> 192.168.20.20/24 "seccomp-netperf.json" netperf <NODENAME1>
./test/k8s-sriov.sh <SRIOV_VF_NAME> 192.168.20.30/24 "seccomp-netserver.json" netserver <NODENAME2>
```

#### Throughput and CPU time
```bash
cd netperf
./kill.sh

# on different containers on different hosts
./netserver.sh rdma_stream_remotevf true
./netperf.sh prod2025011801 rdma_stream_remotevf 0,2,4,6,8,10,12,14,16,18,20 0,1,2 STREAM true tiaccoon-remote-roce ""
```

#### Latency
```bash
cd netperf
./kill.sh

# on different containers on different hosts
./netserver.sh rdma_stream_remotevf true
./netperf.sh prod2025011801 rdma_stream_remotevf 0,2,4,6,8,10,12,14,16,18,20 0,1,2 RR true tiaccoon-remote-roce ""

```

#### Connection establishment and close time
```bash
cd cc

# on different containers on different hosts
./server.sh rdma_stream_remotevf true
./benchmark.sh prod2025011801 rdma_stream_remotevf true tiaccoon-remote-roce ""
```
