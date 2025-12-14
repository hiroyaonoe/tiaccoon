#!/bin/sh
# seccomp.json.sh is derived from:
#   https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/test/seccomp.json.sh
#
# Original copyright:
#   Copyright [yyyy] [name of copyright owner]
#
# Licensed under the Apache License, Version 2.0.

# Usage:
# $ ./seccomp.json.sh <socket name> >$HOME/seccomp.json
# $ nerdctl run -it --rm --security-opt seccomp=$HOME/seccomp.json alpine

# TODO: support non-x86
# TODO: inherit the default seccomp profile (https://github.com/containerd/containerd/blob/v1.6.0-rc.1/contrib/seccomp/seccomp_default.go#L52)

SOCKET_NAME=$1

set -eu
cat <<EOF
{
  "defaultAction": "SCMP_ACT_ALLOW",
  "architectures": [
    "SCMP_ARCH_X86_64",
    "SCMP_ARCH_X86",
    "SCMP_ARCH_X32"
  ],
  "listenerPath": "${XDG_RUNTIME_DIR}/${SOCKET_NAME}",
  "syscalls": [
    {
      "names": [
        "bind",
        "listen",
        "accept",
        "accept4",
        "close",
        "connect",
        "setsockopt",
        "fcntl",
        "_exit",
        "exit_group",
        "getpeername",
        "getsockname"
      ],
      "action": "SCMP_ACT_NOTIFY"
    }
  ]
}
EOF
