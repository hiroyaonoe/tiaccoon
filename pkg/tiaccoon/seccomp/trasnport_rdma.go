package seccomp

import (
	"context"
	"syscall"

	"github.com/hiroyaonoe/tiaccoon/pkg/tiaccoon/destination"
)

var (
	ErrTryRDMA = syscall.Errno(999)
)

func (s *socketStatus) transportConnectRDMA(ctx context.Context, entry *destination.Entry) (int, error) {
	return syscall.SizeofSockaddrInet4, ErrTryRDMA
}

func (s *socketStatus) transportBindRDMA(ctx context.Context, entry *destination.Entry) (int, error) {
	return syscall.SizeofSockaddrInet4, ErrTryRDMA
}
