package seccomp

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"syscall"

	"github.com/hiroyaonoe/tiaccoon/pkg/log"
	"github.com/hiroyaonoe/tiaccoon/pkg/tiaccoon/destination"
	libseccomp "github.com/seccomp/libseccomp-golang"
	"golang.org/x/sys/unix"
)

const (
	AcceptedSocketQueueCapacity = 1<<16 - 1
	VirtualSockAddrSize         = syscall.SizeofSockaddrInet4
)

type socketOption struct {
	level   uint64
	optname uint64
	optval  []byte
	optlen  uint64
}

// Handle F_SETFL, F_SETFD options
type fcntlOption struct {
	cmd   uint64
	value uint64
}

type hostSocketState int

const (
	// HostSocketBinded means that the socket is binded
	HostSocketBinded hostSocketState = iota
	// HostSocketListening means that the socket is listening
	HostSocketListening
	// HostSocketAccepted means that the socket is accepted and not bypassed yet
	HostSocketAccepted
	// HostSocketError happened after bypass. Nothing can be done to recover from this state.
	HostSocketError
)

func (hs hostSocketState) String() string {
	switch hs {
	case HostSocketBinded:
		return "HostSocketBinded"
	case HostSocketListening:
		return "HostSocketListening"
	case HostSocketAccepted:
		return "HostSocketAccepted"
	case HostSocketError:
		return "HostSocketError"
	default:
		panic(fmt.Sprintf("unexpected enum %d: String() is not implemented", hs))
	}
}

type hostSocket struct {
	Sockfd int                `json:"sockfd"`
	Entry  *destination.Entry `json:"entry"`
	State  hostSocketState    `json:"state"`
	Ctx    context.Context    `json:"-"`
	Cancel context.CancelFunc `json:"-"`
}

type socketState int

const (
	// NotBypassable means that the fd is not socket or not bypassed
	NotBypassable socketState = iota
	// NotBypassed means that the socket is not bypassed.
	NotBypassed
	// Binded means that the socket is binded
	Binded
	// Listening means that the socket is listening
	Listening
	// Bypassed means that the socket is replaced by one created on the host after accepted or connected
	Bypassed
	// Error happened after bypass. Nothing can be done to recover from this state.
	Error
)

func (ss socketState) String() string {
	switch ss {
	case NotBypassable:
		return "NotBypassable"
	case NotBypassed:
		return "NotBypassed"
	case Binded:
		return "Binded"
	case Listening:
		return "Listening"
	case Bypassed:
		return "Bypassed"
	case Error:
		return "Error"
	default:
		panic(fmt.Sprintf("unexpected enum %d: String() is not implemented", ss))
	}
}

type processStatus struct {
	sockets map[int]*socketStatus
}

func newProcessStatus() *processStatus {
	return &processStatus{
		sockets: map[int]*socketStatus{},
	}
}

type socketStatus struct {
	state           socketState
	pid             int
	sockfd          int
	sockDomain      int
	sockType        int
	sockProto       int
	localVAddr      *sockaddr
	remoteVAddr     *sockaddr
	socketOptions   []socketOption
	fcntlOptions    []fcntlOption
	hostSockets     sync.Map
	acceptedSockets chan *hostSocket
	Ctx             context.Context
	Cancel          context.CancelFunc
}

func newSocketStatus(pid int, sockfd int, sockDomain, sockType, sockProto int) *socketStatus {
	ctx, cancel := context.WithCancel(context.Background())
	return &socketStatus{
		state:           NotBypassed,
		pid:             pid,
		sockfd:          sockfd,
		sockDomain:      sockDomain,
		sockType:        sockType,
		sockProto:       sockProto,
		localVAddr:      zeroSockaddr(),
		remoteVAddr:     zeroSockaddr(),
		socketOptions:   []socketOption{},
		fcntlOptions:    []fcntlOption{},
		acceptedSockets: make(chan *hostSocket, AcceptedSocketQueueCapacity),
		Ctx:             ctx,
		Cancel:          cancel,
	}
}

// handleSysGetpeername is derived from:
//   https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/bypass4netns/socket.go#L399
//
// Original copyright:
//   Copyright [yyyy] [name of copyright owner]
//
// Licensed under the Apache License, Version 2.0.
func (s *socketStatus) handleSysGetpeername(ctx context.Context, notifFd libseccomp.ScmpFd, req *libseccomp.ScmpNotifReq, resp *libseccomp.ScmpNotifResp, handler *notifHandler, pid int) {
	logger := log.FromContext(ctx)

	err := handler.writeSockaddrToProcess(ctx, pid, req.Data.Args[1], req.Data.Args[2], s.remoteVAddr)
	if err != nil {
		logger.ErrorContext(ctx, "failed to write sockaddr to process", "error", err)
		return
	}

	resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))
	resp.Error = 0
	resp.Val = 0

	logger.InfoContext(ctx, "set sockaddr", "sockaddr", s.remoteVAddr)
}

func (s *socketStatus) handleSysGetsockname(ctx context.Context, notifFd libseccomp.ScmpFd, req *libseccomp.ScmpNotifReq, resp *libseccomp.ScmpNotifResp, handler *notifHandler, pid int) {
	logger := log.FromContext(ctx)

	err := handler.writeSockaddrToProcess(ctx, pid, req.Data.Args[1], req.Data.Args[2], s.localVAddr)
	if err != nil {
		logger.ErrorContext(ctx, "failed to write sockaddr to process", "error", err)
		return
	}

	resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))
	resp.Error = 0
	resp.Val = 0

	logger.InfoContext(ctx, "set sockaddr", "sockaddr", s.localVAddr)
}

// handleSysBind is derived from:
//   https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/bypass4netns/socket.go#L309
//
// Original copyright:
//   Copyright [yyyy] [name of copyright owner]
//
// Licensed under the Apache License, Version 2.0.
func (s *socketStatus) handleSysBind(ctx context.Context, notifFd libseccomp.ScmpFd, req *libseccomp.ScmpNotifReq, resp *libseccomp.ScmpNotifResp, handler *notifHandler, pid int) {
	logger := log.FromContext(ctx)

	if s.state != NotBypassed {
		logger.ErrorContext(ctx, "unexpected state", "state", s.state.String())
		resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))
		resp.Error = int32(syscall.EINVAL)
		return
	}

	dstAddr, err := handler.readSockaddrFromProcess(ctx, pid, req.Data.Args[1], req.Data.Args[2])
	if err != nil {
		logger.ErrorContext(ctx, "failed to read sockaddr from process", "error", err)
		s.state = Error
		resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))
		resp.Error = int32(syscall.EACCES)
		return
	}
	s.localVAddr = dstAddr
	logger = logger.With("dstAddr", dstAddr.String())

	// TODO: check whether the destination is bypassed or not.
	// TODO: handle loopback address
	// TODO: handle interface's address as loopback
	// TODO: handle c2c communication
	// https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/bypass4netns/socket.go#L179

	// TODO: handle only virtual IP address (c2c communication)
	// if dstAddr.IP.IsPrivate() {
	// }

	// TODO: check whether the destination container socket is bypassed or not.
	// https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/bypass4netns/socket.go#L229

	dEntries := handler.de.GetServer(ctx, dstAddr.Port)
	if dEntries == nil {
		// TODO: Set NotBypassable when the destination is not found
		logger.WarnContext(ctx, "destination not found, but virtual addr is recorded: (maybe called before connect)")
		resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))
		resp.Error = 0
		resp.Val = 0
		return
	}

	ok := false
	rdma := false
	for _, entry := range dEntries {
		sockfdOnHost, err := s.transportBind(ctx, entry)
		if err != nil {
			if handler.featureRDMA && errors.Is(err, ErrTryRDMA) { // RDMA
				logger.InfoContext(ctx, "try RDMA", "entry", entry, "sockfdOnHost(new addrlen)", sockfdOnHost)
				paddr := entry.Address.(destination.TransportAddrRDMA)
				sa, err := newSockAddrFromIPPort(paddr.Family(), paddr.NetIP(), paddr.Port(), 0, 0)
				if err != nil {
					logger.WarnContext(ctx, "failed to create sockaddr", "error", err)
					continue
				}
				err = handler.writeSockaddrToProcess(ctx, pid, req.Data.Args[1], uint64(sockfdOnHost), sa)
				if err != nil {
					logger.WarnContext(ctx, "failed to write sockaddr to process", "error", err)
					continue
				}
				ok = true
				rdma = true
				resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))
				resp.Error = 0
				resp.Val = uint64(ErrTryRDMA) + uint64(sockfdOnHost) // 999 + new addrlen
				continue
			}
			logger.WarnContext(ctx, "failed to bind", "error", err, "entry", entry)
			continue
		}
		ok = true
		hsCtx, hsCancel := context.WithCancel(context.Background())
		hs := &hostSocket{
			Sockfd: sockfdOnHost,
			Entry:  entry,
			State:  HostSocketBinded,
			Ctx:    hsCtx,
			Cancel: hsCancel,
		}
		s.hostSockets.Store(sockfdOnHost, hs)
		logger.InfoContext(ctx, "binded on host", "hostSocket", hs)
	}

	if !ok {
		logger.ErrorContext(ctx, "failed to bind on all entries", "entries", dEntries)
		s.state = Error
		resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))
		resp.Error = int32(syscall.EACCES)
		return
	}

	s.state = Binded
	if !rdma {
		resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))
		resp.Error = 0
		resp.Val = 0
	}

	logger.InfoContext(ctx, "binded socket")
}

func (s *socketStatus) handleSysListen(ctx context.Context, notifFd libseccomp.ScmpFd, req *libseccomp.ScmpNotifReq, resp *libseccomp.ScmpNotifResp, handler *notifHandler, pid int) {
	logger := log.FromContext(ctx)

	if s.state == NotBypassable {
		logger.ErrorContext(ctx, "not bypassable", "state", s.state.String())
		return
	}

	if s.state == Listening {
		logger.ErrorContext(ctx, "already listening", "state", s.state.String())
		resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))
		resp.Error = int32(syscall.EADDRINUSE)
		return
	}

	if s.state != Binded {
		logger.ErrorContext(ctx, "unexpected state", "state", s.state.String())
		s.state = NotBypassable
		return
	}

	// Set bigger value if on-demand accept.
	backlog := int(req.Data.Args[1])

	ok := false
	s.hostSockets.Range(func(key, value any) bool {
		hs := value.(*hostSocket)
		if hs.State != HostSocketBinded {
			return true
		}
		err := s.transportListen(ctx, hs, backlog)
		if err != nil {
			logger.WarnContext(ctx, "failed to listen", "error", err, "hostSocket", hs)
			return true
		}
		// Go cannot cancel blocking syscall such as accept. ( https://github.com/golang/go/issues/41054 )
		// Tiaccoon can't use only an accepted host socket and cancel no-accepted host sockets when container calls accept (called on-demand accept).
		// So, Tiaccoon calls accept on host socket when container calls listen.
		// Tiaccoon expects applications to call accept immediately after calling listen.
		//
		// TODO: We may cancel accept by setsockopt(SO_ACCEPTCONN, 0).
		go s.transportAccept(ctx, hs, handler.sae)
		ok = true
		logger.InfoContext(ctx, "listening and accepting on host", "hostSocket", hs)
		return true
	})

	if !handler.featureRDMA && !ok {
		logger.ErrorContext(ctx, "failed to listen on all binded socket or binded socket not found")
		s.state = Error
		resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))
		resp.Error = int32(syscall.EACCES)
		return
	}

	s.state = Listening
	resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))
	resp.Error = 0
	resp.Val = 0

	logger.InfoContext(ctx, "listening socket")
}

func (s *socketStatus) handleSysAccept(ctx context.Context, notifFd libseccomp.ScmpFd, req *libseccomp.ScmpNotifReq, resp *libseccomp.ScmpNotifResp, handler *notifHandler, pid int) {
	s.handleSysAcceptWithFlags(ctx, notifFd, req, resp, handler, pid, 0)
	return
}

func (s *socketStatus) handleSysAccept4(ctx context.Context, notifFd libseccomp.ScmpFd, req *libseccomp.ScmpNotifReq, resp *libseccomp.ScmpNotifResp, handler *notifHandler, pid int) {
	flags := int(req.Data.Args[3])
	s.handleSysAcceptWithFlags(ctx, notifFd, req, resp, handler, pid, flags)
}

func (s *socketStatus) handleSysAcceptWithFlags(ctx context.Context, notifFd libseccomp.ScmpFd, req *libseccomp.ScmpNotifReq, resp *libseccomp.ScmpNotifResp, handler *notifHandler, pid, flags int) {
	logger := log.FromContext(ctx)

	if s.state == NotBypassable {
		logger.ErrorContext(ctx, "not bypassable", "state", s.state.String())
		return
	}

	if s.state != Listening {
		logger.ErrorContext(ctx, "unexpected state", "state", s.state.String())
		resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))
		resp.Error = int32(syscall.EINVAL)
		return
	}

	// TODO: handle flags with fcntl and setsockopt
	// Cannot set flags when accepting a host socket because Tiaccoon does not do on-demand accept.

	logger.InfoContext(ctx, "Waiting accept")
	select {
	case <-s.Ctx.Done():
		return
	case hs := <-s.acceptedSockets:
		if hs.State != HostSocketAccepted {
			logger.InfoContext(ctx, "unexpected status", "hostSocket", hs)
			s.state = NotBypassable
			return
		}
		logger.InfoContext(ctx, "accepted", "acceptedSockfd", hs.Sockfd)

		defer syscall.Close(hs.Sockfd)

		addfd := seccompNotifAddFd{
			id:         req.ID,
			flags:      0,
			srcfd:      uint32(hs.Sockfd),
			newfd:      0,
			newfdFlags: 0,
		}

		newfd, err := addfd.ioctlNotifAddFd(notifFd)
		if err != nil {
			logger.ErrorContext(ctx, "ioctl NotifAddFd failed", "error", err)
			s.state = NotBypassable
			return
		}

		asock, err := handler.registerSocket(ctx, pid, newfd)
		if err != nil {
			logger.ErrorContext(ctx, "failed to register accepted socket", "error", err)
		}
		asock.state = Bypassed
		asock.localVAddr = s.localVAddr // We may need to copy sockaddr
		copy(asock.socketOptions, s.socketOptions)

		// TODO: rewrite src address to virtual src address
		// https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/bypass4netns/socket.go#L267

		srcAddr, err := newSockAddrFromIPPort(s.localVAddr.Family, hs.Entry.VIP, hs.Entry.VPort, s.localVAddr.Flowinfo, s.localVAddr.ScopeID)
		if err != nil {
			logger.WarnContext(ctx, "failed to create sockaddr", "error", err)
		}
		err = handler.writeSockaddrToProcess(ctx, pid, req.Data.Args[1], req.Data.Args[2], srcAddr)
		if err != nil {
			logger.WarnContext(ctx, "failed to write sockaddr to process", "error", err)
		}
		asock.remoteVAddr = srcAddr

		s.hostSockets.Delete(hs.Sockfd)

		resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))
		resp.Error = 0
		resp.Val = uint64(newfd)

		logger.InfoContext(ctx, "bypassed accepted socket", "hostSocket", hs, "newfd", newfd)
		return
	}
}

// handleSysConnect is derived from:
//   https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/bypass4netns/socket.go#L143
//
// Original copyright:
//   Copyright [yyyy] [name of copyright owner]
//
// Licensed under the Apache License, Version 2.0.
func (s *socketStatus) handleSysConnect(ctx context.Context, notifFd libseccomp.ScmpFd, req *libseccomp.ScmpNotifReq, resp *libseccomp.ScmpNotifResp, handler *notifHandler, pid int) {
	var err error
	logger := log.FromContext(ctx)

	// if s.localVAddr.Port == 0 {
	// 	s.localVAddr, err = newSockAddrFromIPPort(
	// 		syscall.AF_INET,
	// 		net.IPv4(0, 0, 0, 0), // TODO: handler.myVIP
	// 		0,                    // TODO: ephemeral port
	// 		0, 0,
	// 	)
	// 	if err != nil {
	// 		logger.WarnContext(ctx, "failed to create localVAddr", "error", err)
	// 		s.localVAddr = zeroSockaddr()
	// 	}
	// }

	// TODO: s.pid and pid may be the same
	dstAddr, err := handler.readSockaddrFromProcess(ctx, s.pid, req.Data.Args[1], req.Data.Args[2])
	if err != nil {
		logger.ErrorContext(ctx, "Failed to read sockaddr from process", "error", err)
		s.state = NotBypassable
		return
	}
	s.remoteVAddr = dstAddr
	logger = logger.With("dstAddr", dstAddr.String())

	ok := handler.cae.Apply(ctx, dstAddr.IP)
	if !ok {
		logger.ErrorContext(ctx, "access control denied")
		s.state = Error
		resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))
		resp.Error = int32(syscall.EACCES)
		return
	}
	logger.InfoContext(ctx, "access control allowed")

	// TODO: check whether the destination is bypassed or not.
	// TODO: handle loopback address
	// TODO: handle interface's address as loopback
	// TODO: handle c2c communication
	// https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/bypass4netns/socket.go#L179

	// TODO: handle only virtual IP address (c2c communication)
	// if dstAddr.IP.IsPrivate() {
	// }

	// TODO: check whether the destination container socket is bypassed or not.
	// https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/bypass4netns/socket.go#L229

	dEntries := handler.de.GetClient(ctx, dstAddr.IP, dstAddr.Port)
	if dEntries == nil {
		// TODO: Set NotBypassable when the destination is not found
		logger.ErrorContext(ctx, "destination not found")
		s.state = Error
		resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))
		resp.Error = int32(syscall.EACCES)
		return
	}

	var sockfdOnHost int
	ok = false
	for _, entries := range dEntries { // Prioritize the first transport type
		tried := make(map[int]bool, len(entries))
		cnt := 0
		for {
			if cnt >= len(entries) {
				break
			}
			n := rand.Intn(len(entries)) // TODO: round-robin
			if tried[n] {
				continue
			}
			tried[n] = true
			cnt++
			entry := entries[n]
			sockfdOnHost, err = s.transportConnect(ctx, entry)
			if err != nil {
				if handler.featureRDMA && errors.Is(err, ErrTryRDMA) { // RDMA
					logger.InfoContext(ctx, "try RDMA", "entry", entry, "sockfdOnHost(new addrlen)", sockfdOnHost)
					paddr := entry.Address.(destination.TransportAddrRDMA)
					sa, err := newSockAddrFromIPPort(paddr.Family(), paddr.NetIP(), paddr.Port(), 0, 0)
					if err != nil {
						logger.WarnContext(ctx, "failed to create sockaddr", "error", err)
						continue
					}
					err = handler.writeSockaddrToProcess(ctx, pid, req.Data.Args[1], uint64(sockfdOnHost), sa)
					if err != nil {
						logger.WarnContext(ctx, "failed to write sockaddr to process", "error", err)
						continue
					}
					s.state = Bypassed
					resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))
					resp.Error = 0
					resp.Val = uint64(ErrTryRDMA) + uint64(sockfdOnHost) // 999 + new addrlen
					return
				}
				logger.WarnContext(ctx, "failed to connect", "error", err, "entry", entry)
				continue
			}
			defer syscall.Close(sockfdOnHost)
			logger.InfoContext(ctx, "connected on host", "sockfdOnHost", sockfdOnHost, "entry", entry)
			ok = true
			break
		}
		if ok {
			break
		}
	}
	if !ok {
		logger.ErrorContext(ctx, "failed to connect to all destination")
		s.state = Error
		resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))
		resp.Error = int32(syscall.EACCES)
		return
	}

	localVAddrByte, err := sockaddrToByte(s.localVAddr)
	if err == nil {
		if len(localVAddrByte) == VirtualSockAddrSize {
			err = syscall.Sendto(sockfdOnHost, localVAddrByte, 0, nil)
			if err != nil {
				logger.ErrorContext(ctx, "failed to sendto", "error", err)
			}
		} else {
			logger.ErrorContext(ctx, "unexpected sockaddr size", "size", len(localVAddrByte))
		}
	} else {
		logger.ErrorContext(ctx, "failed to convert sockaddr to byte", "error", err)
	}

	addfd := seccompNotifAddFd{
		id:         req.ID,
		flags:      SeccompAddFdFlagSetFd,
		srcfd:      uint32(sockfdOnHost),
		newfd:      uint32(req.Data.Args[0]),
		newfdFlags: 0,
	}

	_, err = addfd.ioctlNotifAddFd(notifFd)
	if err != nil {
		logger.ErrorContext(ctx, "ioctl NotifAddFd failed", "error", err)
		s.state = NotBypassable
		return
	}

	s.state = Bypassed
	resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))
	resp.Error = 0
	resp.Val = 0

	logger.InfoContext(ctx, "bypassed connect socket")
}

// handleSysSetsockopt is derived from:
//   https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/bypass4netns/socket.go#L103
//
// Original copyright:
//   Copyright [yyyy] [name of copyright owner]
//
// Licensed under the Apache License, Version 2.0.
func (s *socketStatus) handleSysSetsockopt(ctx context.Context, notifFd libseccomp.ScmpFd, req *libseccomp.ScmpNotifReq, resp *libseccomp.ScmpNotifResp, handler *notifHandler, pid int) {
	logger := log.FromContext(ctx)

	level := req.Data.Args[1]
	optname := req.Data.Args[2]
	optlen := req.Data.Args[4]
	optval, err := handler.readProcMem(ctx, pid, req.Data.Args[3], optlen)
	if err != nil {
		logger.ErrorContext(ctx, "setsockopt readProcMem failed",
			"error", err,
			"pid", pid,
			"level", level,
			"optname", optname,
			"optlen", optlen,
			"offset", req.Data.Args[3])
		return
	}

	value := socketOption{
		level:   level,
		optname: optname,
		optval:  optval,
		optlen:  optlen,
	}
	s.socketOptions = append(s.socketOptions, value)

	if s.state == Binded || s.state == Listening {
		err = setsockopt(s.sockfd, value)
		if err != nil {
			logger.ErrorContext(ctx, "failed to configure socket", "error", err)
			return
		}
	}
	logger.DebugContext(ctx, "setsockopt was recorded",
		"pid", pid,
		"level", level,
		"optname", optname,
		"optval", optval,
		"optlen", optlen)
}

// handleSysFcntl is derived from:
//   https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/bypass4netns/socket.go#L124
//
// Original copyright:
//   Copyright [yyyy] [name of copyright owner]
//
// Licensed under the Apache License, Version 2.0.
func (s *socketStatus) handleSysFcntl(ctx context.Context, notifFd libseccomp.ScmpFd, req *libseccomp.ScmpNotifReq, resp *libseccomp.ScmpNotifResp) {
	logger := log.FromContext(ctx)

	fcntlCmd := req.Data.Args[1]
	var opt fcntlOption
	switch fcntlCmd {
	case unix.F_SETFD: // 0x2
	case unix.F_SETFL: // 0x4
		opt = fcntlOption{
			cmd:   fcntlCmd,
			value: req.Data.Args[2],
		}
		s.fcntlOptions = append(s.fcntlOptions, opt)
		logger.DebugContext(ctx, fmt.Sprintf("fcntl cmd=0x%x value=%d was recorded.", fcntlCmd, opt.value))
	case unix.F_GETFL: // 0x3
		// ignore these
	default:
		logger.WarnContext(ctx, fmt.Sprintf("Unknown fcntl command 0x%x ignored.", fcntlCmd))
	}

	if s.state == Binded || s.state == Listening {
		err := fcntl(s.sockfd, opt)
		if err != nil {
			logger.ErrorContext(ctx, "failed to configure socket", "error", err)
			return
		}
	}
}

func (s *socketStatus) removeSocket(ctx context.Context) {
	logger := log.FromContext(ctx)
	s.hostSockets.Range(func(key, value any) bool {
		hs := value.(*hostSocket)
		if hs.Cancel != nil {
			hs.Cancel()
		}
		err := syscall.Shutdown(hs.Sockfd, syscall.SHUT_RDWR)
		if err != nil {
			logger.ErrorContext(ctx, "failed to shutdown host socket", "error", err, "hostSocket", hs)
		} else {
			logger.DebugContext(ctx, "shutdowned host socket", "hostSocket", hs)
		}
		err = syscall.Close(hs.Sockfd)
		if err != nil {
			logger.ErrorContext(ctx, "failed to close host socket", "error", err, "hostSocket", hs)
		} else {
			logger.DebugContext(ctx, "closed host socket", "hostSocket", hs)
		}
		return true
	})
	s.Cancel()
}
