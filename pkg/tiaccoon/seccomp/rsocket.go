package seccomp

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"syscall"

	"github.com/hiroyaonoe/tiaccoon/pkg/log"
	libseccomp "github.com/seccomp/libseccomp-golang"
)

const rsocketVirtualPath = "tiaccoon-rsocket-control"

// initializeRsocket handles rsocket control path
func (h *notifHandler) initializeRsocket(ctx context.Context, notifFd libseccomp.ScmpFd, req *libseccomp.ScmpNotifReq, resp *libseccomp.ScmpNotifResp, pid int, sock *socketStatus) bool {
	logger := log.FromContext(ctx).With("func", "handleRsocket")
	logger.DebugContext(ctx, "handleRsocket")

	if sock.sockDomain != syscall.AF_UNIX || sock.sockType != syscall.SOCK_STREAM {
		return false
	}

	buf, err := h.readProcMem(ctx, sock.pid, req.Data.Args[1], req.Data.Args[2])
	if err != nil {
		logger.ErrorContext(ctx, fmt.Sprintf("failed to readProcMem pid %v offset 0x%x", pid, req.Data.Args[2]), "err", err)
		return false
	}
	path, err := getSockaddrUnixPath(buf)
	if err != nil {
		logger.ErrorContext(ctx, "failed to getSockaddrUnixPath", "err", err)
		return false
	}
	if path != rsocketVirtualPath {
		logger.DebugContext(ctx, "path is not rsocket control path", "path", path)
		return false
	}
	logger.InfoContext(ctx, "path is rsocket control path", "path", path)

	var fds [2]int
	fds, err = syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create socketpair", "err", err)
		return false
	}

	go h.handleRsocket(ctx, fds[0])

	addfd := seccompNotifAddFd{
		id:         req.ID,
		flags:      SeccompAddFdFlagSetFd,
		srcfd:      uint32(fds[1]),
		newfd:      uint32(req.Data.Args[0]),
		newfdFlags: 0,
	}

	_, err = addfd.ioctlNotifAddFd(notifFd)
	if err != nil {
		logger.ErrorContext(ctx, "ioctl NotifAddFd failed", "error", err)
		sock.state = NotBypassable
		return false
	}

	// TODO: rewrite dest address to virtual dest address
	// https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/bypass4netns/socket.go#L267

	sock.state = Bypassed
	resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))
	resp.Error = 0
	resp.Val = 0

	logger.InfoContext(ctx, "bypassed connect socket for rsocket control")

	return true
}

func (h *notifHandler) handleRsocket(ctx context.Context, sockfd int) {
	logger := log.FromContext(ctx).With("func", "handleRsocket")
	logger.DebugContext(ctx, "handleRsocket")
	buf := make([]byte, 64)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, err := syscall.Read(sockfd, buf)
			if err != nil {
				logger.ErrorContext(ctx, "failed to read from rsocket control socket", "err", err)
				continue
			}

			cmd := string(buf[:4])
			logger.DebugContext(ctx, "received message", "cmd", cmd, "buf", fmt.Sprintf("%+v", buf))
			var resp []byte
			switch cmd {
			case "PING": // ping
				resp = []byte("OK")
				err = nil
			case "MVIP": // get my VIP
				resp, err = h.handleRsocketMYVIP(ctx)
			case "ACON": // access control
				resp, err = h.handleRsocketAccessControl(ctx, buf[4:20])
			default:
				logger.ErrorContext(ctx, "unexpected command", "cmd", cmd, "buf", buf)
				resp = []byte("ER")
			}
			if err != nil {
				logger.ErrorContext(ctx, "failed to handle rsocket control message", "err", err)
				resp = []byte("ER")
			}
			logger.DebugContext(ctx, "response", "resp", string(resp))
			_, err = syscall.Write(sockfd, resp)
			if err != nil {
				logger.ErrorContext(ctx, "failed to write to rsocket control socket", "err", err)
				continue
			}
		}
	}
}

func (h *notifHandler) handleRsocketMYVIP(ctx context.Context) ([]byte, error) {
	sa, err := newSockAddrFromIPPort(syscall.AF_INET, h.myVIP, 0, 0, 0) // ephemeral port
	if err != nil {
		return nil, fmt.Errorf("failed to create sockaddr: %w", err)
	}
	buf, err := sockaddrToByte(sa)
	if err != nil {
		return nil, fmt.Errorf("failed to convert sockaddr to byte: %w", err)
	}
	return append([]byte("OK"), buf...), nil
}

func (h *notifHandler) handleRsocketAccessControl(ctx context.Context, addr []byte) ([]byte, error) {
	if len(addr) != 16 {
		return nil, fmt.Errorf("unexpected addr: %v", addr)
	}

	rsa, err := newSockaddr(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to create remote sockaddr: %w", err)
	}

	ok := h.sae.Apply(ctx, rsa.IP)
	if !ok {
		return []byte("NO"), nil
	}

	return []byte("OK"), nil
}

func getSockaddrUnixPath(buf []byte) (string, error) {
	reader := bytes.NewReader(buf)
	// TODO: support big endian hosts
	endian := binary.LittleEndian
	addrun := syscall.RawSockaddrUnix{}
	if _, err := reader.Seek(0, 0); err != nil {
		return "", err
	}
	if err := binary.Read(reader, endian, &addrun); err != nil {
		return "", fmt.Errorf("cannot cast byte array to RawSockaddrUnix: %w", err)
	}
	if addrun.Family != syscall.AF_UNIX {
		return "", fmt.Errorf("unexpected family: %d", addrun.Family)
	}
	bp := make([]byte, 0, 108)
	for i := 0; i < len(addrun.Path); i++ {
		if addrun.Path[i] == 0 {
			break
		}
		bp = append(bp, byte(addrun.Path[i]))
	}
	return fmt.Sprintf("%s", bp), nil
}
