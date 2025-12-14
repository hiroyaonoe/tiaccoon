package seccomp

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/hiroyaonoe/tiaccoon/pkg/log"
	"github.com/hiroyaonoe/tiaccoon/pkg/tiaccoon/accesscontrol"
	"github.com/hiroyaonoe/tiaccoon/pkg/tiaccoon/destination"
)

func (s *socketStatus) transportConnect(ctx context.Context, entry *destination.Entry) (int, error) {
	logger := log.FromContext(ctx).With("entry", entry)
	ctx = log.ContextWithLogger(ctx, logger)
	switch entry.Transport {
	case destination.TransportUNIX:
		return s.transportConnectUNIX(ctx, entry)
	case destination.TransportRDMA:
		return s.transportConnectRDMA(ctx, entry)
	case destination.TransportIPv6:
		return 0, errors.New("NOT IMPLEMENTED: IPv6")
	case destination.TransportIPv4:
		return s.transportConnectIPv4(ctx, entry)
	default:
		return 0, errors.New("UNEXPECTED: Unknown transport")
	}
}

func (s *socketStatus) transportBind(ctx context.Context, entry *destination.Entry) (int, error) {
	logger := log.FromContext(ctx).With("entry", entry)
	ctx = log.ContextWithLogger(ctx, logger)
	switch entry.Transport {
	case destination.TransportUNIX:
		return s.transportBindUNIX(ctx, entry)
	case destination.TransportRDMA:
		return s.transportBindRDMA(ctx, entry)
	case destination.TransportIPv6:
		return 0, errors.New("NOT IMPLEMENTED: IPv6")
	case destination.TransportIPv4:
		return s.transportBindIPv4(ctx, entry)
	default:
		return 0, errors.New("UNEXPECTED: Unknown transport")
	}
}

func (s *socketStatus) transportListen(ctx context.Context, hs *hostSocket, backlog int) error {
	logger := log.FromContext(ctx).With("sockfdOnHost", hs.Sockfd, "backlog", backlog)
	ctx = log.ContextWithLogger(ctx, logger)
	var err error
	switch hs.Entry.Transport {
	case destination.TransportIPv4, destination.TransportIPv6, destination.TransportUNIX:
		err = s.transportListenSocket(ctx, hs.Sockfd, backlog)
	case destination.TransportRDMA:
		err = errors.New("UNEXPECTED: RDMA")
	default:
		err = errors.New("UNEXPECTED: Unknown transport")
	}
	if err == nil {
		hs.State = HostSocketListening
	} else {
		hs.State = HostSocketError
	}
	return err
}

func (s *socketStatus) transportListenSocket(ctx context.Context, sockfdOnHost, backlog int) error {
	err := syscall.Listen(sockfdOnHost, backlog)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	return nil
}

func (s *socketStatus) transportAccept(ctx context.Context, hs *hostSocket, sae *accesscontrol.Entries) {
	logger := log.FromContext(ctx).With("sockfdOnHost", hs.Sockfd)
	ctx = log.ContextWithLogger(ctx, logger)
	var err error
	var as *hostSocket
	for {
		select {
		case <-hs.Ctx.Done():
			return
		default:
			switch hs.Entry.Transport {
			case destination.TransportUNIX:
				as, err = s.transportAcceptUNIX(ctx, hs.Sockfd)
			case destination.TransportRDMA:
				err = errors.New("UNEXPECTED: RDMA")
			case destination.TransportIPv6:
				err = errors.New("NOT IMPLEMENTED: IPv6")
			case destination.TransportIPv4:
				as, err = s.transportAcceptIPv4(ctx, hs.Sockfd)
			default:
				err = errors.New("UNEXPECTED: Unknown transport")
			}
			if err != nil {
				logger.ErrorContext(ctx, "failed to accept", "error", err)
				// TODO: Cleanup hostSocket if changed to HostSocketError
				hs.State = HostSocketError
				hs.Cancel()
				return
			}

			ok := sae.Apply(ctx, as.Entry.VIP)
			if !ok {
				logger.ErrorContext(ctx, "access control denied", "acceptedHostSocket", as)
				syscall.Close(as.Sockfd) // TODO: Close socket more precisely
				continue
			}
			logger.InfoContext(ctx, "access control allowed", "acceptedHostSocket", as)

			s.hostSockets.Store(as.Sockfd, as)
			s.acceptedSockets <- as
		}
	}
}

func (s *socketStatus) configureSocket(ctx context.Context, sockfd int) error {
	logger := log.FromContext(ctx)

	for _, optVal := range s.socketOptions {
		err := setsockopt(sockfd, optVal)
		if err != nil {
			return err
		}
		logger.DebugContext(ctx, fmt.Sprintf("configured socket option val=%v", optVal))
	}

	for _, fcntlVal := range s.fcntlOptions {
		err := fcntl(sockfd, fcntlVal)
		if err != nil {
			return err
		}
		logger.DebugContext(ctx, fmt.Sprintf("configured socket fcntl val=%v", fcntlVal))
	}

	return nil
}

func recvDstVAddr(sockfd int) (*sockaddr, error) {
	buf := make([]byte, VirtualSockAddrSize)
	n, _, err := syscall.Recvfrom(sockfd, buf, 0)
	if err != nil || n != VirtualSockAddrSize {
		return nil, fmt.Errorf("failed to recvfrom %d: %w", n, err)
	}
	return newSockaddr(buf)
}

func setsockopt(sockfd int, v socketOption) error {
	_, _, errno := syscall.Syscall6(syscall.SYS_SETSOCKOPT, uintptr(sockfd), uintptr(v.level), uintptr(v.optname), uintptr(unsafe.Pointer(&v.optval[0])), uintptr(v.optlen), 0)
	if errno != 0 {
		return fmt.Errorf("setsockopt failed(%v): %s", v, errno)
	}
	return nil
}

func fcntl(sockfd int, v fcntlOption) error {
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(sockfd), uintptr(v.cmd), uintptr(v.value))
	if errno != 0 {
		return fmt.Errorf("fcntl failed(%v): %s", v, errno)
	}
	return nil
}
