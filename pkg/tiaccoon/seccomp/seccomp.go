// Package seccomp is derived from:
//   https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/bypass4netns/seccomp.go#L29
//
// Original copyright:
//   Copyright [yyyy] [name of copyright owner]
//
// Licensed under the Apache License, Version 2.0.
package seccomp

import (
	"fmt"
	"syscall"
	"unsafe"

	libseccomp "github.com/seccomp/libseccomp-golang"
	"github.com/vtolstov/go-ioctl"
)

const (
	SeccompAddFdFlagSetFd        = 1
	SeccompUserNotifFlagContinue = 1
	SeccompIocMagic              = '!'
)

func seccompIOW(nr, typ uintptr) uintptr {
	return ioctl.IOW(uintptr(SeccompIocMagic), nr, typ)
}

// C.SECCOMP_IOCTL_NOTIF_ADDFD become error
// Error Message: could not determine kind of name for C.SECCOMP_IOCTL_NOTIF_ADDFD
// TODO: use C.SECCOMP_IOCTL_NOTIF_ADDFD or add equivalent variable to libseccomp-go
func seccompIoctlNotifAddfd() uintptr {
	return seccompIOW(3, uintptr(unsafe.Sizeof(seccompNotifAddFd{})))
}

type seccompNotifAddFd struct {
	id         uint64
	flags      uint32
	srcfd      uint32
	newfd      uint32
	newfdFlags uint32
}

func (addfd *seccompNotifAddFd) ioctlNotifAddFd(notifFd libseccomp.ScmpFd) (int, error) {
	ioctl_op := seccompIoctlNotifAddfd()
	ret, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(notifFd), ioctl_op, uintptr(unsafe.Pointer(addfd)))
	if errno != 0 {
		return 0, fmt.Errorf("ioctl(SECCOMP_IOCTL_NOTIF_ADDFD) failed: %s", errno)
	}
	return int(ret), nil
}
