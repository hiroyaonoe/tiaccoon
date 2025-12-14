package seccomp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/hiroyaonoe/tiaccoon/pkg/log"
	"github.com/hiroyaonoe/tiaccoon/pkg/tiaccoon/destination"
)

func (s *socketStatus) transportConnectUNIX(ctx context.Context, entry *destination.Entry) (int, error) {
	logger := log.FromContext(ctx).With("func", "transportConnectUNIX")

	sockfdOnHost, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return 0, fmt.Errorf("failed to create socket: %w", err)
	}
	logger.DebugContext(ctx, "created socket", "sockfdOnHost", sockfdOnHost)

	err = s.configureSocket(ctx, sockfdOnHost)
	if err != nil {
		return 0, fmt.Errorf("failed to configure socket: %w", err)
	}
	logger.DebugContext(ctx, "configured socket", "sockfdOnHost", sockfdOnHost)

	addr, ok := entry.Address.(destination.TransportAddrUNIX)
	if !ok {
		return 0, errors.New("UNEXPECTED: Address is not TransportAddrUNIX")
	}

	err = syscall.Connect(sockfdOnHost, &syscall.SockaddrUnix{
		Name: addr.Path(),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to connect: %w", err)
	}

	return sockfdOnHost, nil
}

func (s *socketStatus) transportBindUNIX(ctx context.Context, entry *destination.Entry) (int, error) {
	logger := log.FromContext(ctx).With("func", "transportBindUNIX")

	addr, ok := entry.Address.(destination.TransportAddrUNIX)
	if !ok {
		return 0, errors.New("UNEXPECTED: Address is not TransportAddrUNIX")
	}

	// remove the socket file if it exists
	if _, err := os.Stat(addr.Path()); err == nil {
		if err := os.Remove(addr.Path()); err != nil {
			return 0, fmt.Errorf("failed to remove existing socket file: %w", err)
		}
	}

	sockfdOnHost, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return 0, fmt.Errorf("failed to create socket: %w", err)
	}
	logger.DebugContext(ctx, "created socket", "sockfdOnHost", sockfdOnHost)

	err = s.configureSocket(ctx, sockfdOnHost)
	if err != nil {
		return 0, fmt.Errorf("failed to configure socket: %w", err)
	}
	logger.DebugContext(ctx, "configured socket", "sockfdOnHost", sockfdOnHost)

	err = syscall.Bind(sockfdOnHost, &syscall.SockaddrUnix{
		Name: addr.Path(),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to bind: %w", err)
	}

	return sockfdOnHost, nil
}

func (s *socketStatus) transportAcceptUNIX(ctx context.Context, sockfdOnHost int) (*hostSocket, error) {
	acceptedSockfd, srcAddr, err := syscall.Accept(sockfdOnHost)
	if err != nil {
		return nil, fmt.Errorf("failed to accept: %w", err)
	}

	srcAddrUn, ok := srcAddr.(*syscall.SockaddrUnix)
	if !ok {
		return nil, fmt.Errorf("failed to cast srcAddr to srcAddr4: %v", srcAddr)
	}

	vsa, err := recvDstVAddr(acceptedSockfd)
	if err != nil {
		return nil, fmt.Errorf("failed to receive destination virtual address: %w", err)
	}

	hsCtx, hsCancel := context.WithCancel(context.Background())
	as := &hostSocket{
		Sockfd: acceptedSockfd,
		Entry: &destination.Entry{
			VIP:       vsa.IP,
			VPort:     uint16(vsa.Port),
			Transport: destination.TransportUNIX,
			Address:   destination.NewTransportAddrUNIX(srcAddrUn.Name),
		},
		State:  HostSocketAccepted,
		Ctx:    hsCtx,
		Cancel: hsCancel,
	}
	return as, nil
}
