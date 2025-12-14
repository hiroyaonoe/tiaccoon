package seccomp

import (
	"context"
	"errors"
	"fmt"
	"syscall"

	"github.com/hiroyaonoe/tiaccoon/pkg/log"
	"github.com/hiroyaonoe/tiaccoon/pkg/tiaccoon/destination"
)

func (s *socketStatus) transportConnectIPv4(ctx context.Context, entry *destination.Entry) (int, error) {
	logger := log.FromContext(ctx).With("func", "transportConnectIPv4")

	sockfdOnHost, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
	if err != nil {
		return 0, fmt.Errorf("failed to create socket: %w", err)
	}
	logger.InfoContext(ctx, "created socket", "sockfdOnHost", sockfdOnHost, "localVAddr", s.localVAddr)

	// if s.localVAddr.Port != 0 {
	// 	logger.InfoContext(ctx, "binded socket before connect", "sockfdOnHost", sockfdOnHost, "localVAddr", s.localVAddr)
	// 	err = syscall.Bind(sockfdOnHost, &syscall.SockaddrInet4{
	// 		Addr: [4]byte{0, 0, 0, 0},
	// 		Port: int(s.localVAddr.Port), // TODO: use ephemeral port
	// 	})
	// 	if err != nil {
	// 		logger.ErrorContext(ctx, fmt.Errorf("failed to bind before connect: %w", err).Error(), "sockfdOnHost", sockfdOnHost, "localVAddr", s.localVAddr)
	// 		err = syscall.Bind(sockfdOnHost, &syscall.SockaddrInet4{
	// 			Addr: [4]byte{0, 0, 0, 0},
	// 			Port: 0, // ephemeral port
	// 		})
	// 		if err != nil {
	// 			return 0, fmt.Errorf("failed to bind with ephemeral port before connect: %w", err)
	// 		}
	// 	}
	// }

	err = s.configureSocket(ctx, sockfdOnHost)
	if err != nil {
		return 0, fmt.Errorf("failed to configure socket: %w", err)
	}
	logger.DebugContext(ctx, "configured socket", "sockfdOnHost", sockfdOnHost)

	addr, ok := entry.Address.(destination.TransportAddrIPv4)
	if !ok {
		return 0, errors.New("UNEXPECTED: Address is not TransportAddrIPv4")
	}

	err = syscall.Connect(sockfdOnHost, &syscall.SockaddrInet4{
		Addr: addr.IP(),
		Port: addr.Port(),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to connect: %w", err)
	}

	return sockfdOnHost, nil
}

func (s *socketStatus) transportBindIPv4(ctx context.Context, entry *destination.Entry) (int, error) {
	logger := log.FromContext(ctx).With("func", "transportBindIPv4")

	sockfdOnHost, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
	if err != nil {
		return 0, fmt.Errorf("failed to create socket: %w", err)
	}
	logger.DebugContext(ctx, "created socket", "sockfdOnHost", sockfdOnHost)

	err = s.configureSocket(ctx, sockfdOnHost)
	if err != nil {
		return 0, fmt.Errorf("failed to configure socket: %w", err)
	}
	logger.DebugContext(ctx, "configured socket", "sockfdOnHost", sockfdOnHost)

	addr, ok := entry.Address.(destination.TransportAddrIPv4)
	if !ok {
		return 0, errors.New("UNEXPECTED: Address is not TransportAddrIPv4")
	}

	err = syscall.Bind(sockfdOnHost, &syscall.SockaddrInet4{
		Addr: addr.IP(),
		Port: addr.Port(),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to bind: %w", err)
	}

	return sockfdOnHost, nil
}

func (s *socketStatus) transportAcceptIPv4(ctx context.Context, sockfdOnHost int) (*hostSocket, error) {
	acceptedSockfd, srcAddr, err := syscall.Accept(sockfdOnHost)
	if err != nil {
		return nil, fmt.Errorf("failed to accept: %w", err)
	}

	srcAddr4, ok := srcAddr.(*syscall.SockaddrInet4)
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
			Transport: destination.TransportIPv4,
			Address:   destination.NewTransportAddrIPv4(srcAddr4.Addr, int(srcAddr4.Port)),
		},
		State:  HostSocketAccepted,
		Ctx:    hsCtx,
		Cancel: hsCancel,
	}
	return as, nil
}
