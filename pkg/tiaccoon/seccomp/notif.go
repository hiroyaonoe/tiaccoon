package seccomp

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/hiroyaonoe/tiaccoon/pkg/log"
	"github.com/hiroyaonoe/tiaccoon/pkg/tiaccoon/accesscontrol"
	"github.com/hiroyaonoe/tiaccoon/pkg/tiaccoon/destination"
	"github.com/opencontainers/runtime-spec/specs-go"
	libseccomp "github.com/seccomp/libseccomp-golang"
	"golang.org/x/sys/unix"
)

type pidInfoPidType int

const (
	PROCESS pidInfoPidType = iota
	THREAD
)

type pidInfo struct {
	pidType pidInfoPidType
	pidfd   int
	tgid    int
}

type notifHandler struct {
	fd    libseccomp.ScmpFd
	state *specs.ContainerProcessState

	// key is pid
	processes map[int]*processStatus

	// cache /proc/<pid>/mem's fd to reduce latency. key is pid, value is fd
	memfds map[int]int

	// cache pidfd to reduce latency, key is pid.
	pidInfos map[int]pidInfo

	sae *accesscontrol.Entries
	cae *accesscontrol.Entries
	de  *destination.Entries

	myVIP       net.IP
	featureRDMA bool
}

func (h *Handler) newNotifHandler(fd uintptr, state *specs.ContainerProcessState, sae, cae *accesscontrol.Entries, de *destination.Entries, myVIP net.IP, featureRDMA bool) *notifHandler {
	notifHandler := notifHandler{
		fd:          libseccomp.ScmpFd(fd),
		state:       state,
		processes:   map[int]*processStatus{},
		memfds:      map[int]int{},
		pidInfos:    map[int]pidInfo{},
		sae:         sae,
		cae:         cae,
		de:          de,
		myVIP:       myVIP,
		featureRDMA: featureRDMA,
	}

	return &notifHandler
}

func (h *notifHandler) handle(ctx context.Context) {
	logger := log.FromContext(ctx).With("fd", h.fd)
	ctx = log.ContextWithLogger(ctx, logger)
	logger.DebugContext(ctx, "Handling seccomp notification")

	// This function is derived from:
	//   https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/bypass4netns/bypass4netns.go#L574
	//
	// Original copyright:
	//   Copyright [yyyy] [name of copyright owner]
	//
	// Licensed under the Apache License, Version 2.0.
	defer unix.Close(int(h.fd))

	for { // TODO: handle ctx.Done()
		req, err := libseccomp.NotifReceive(h.fd)
		if err != nil {
			logger.ErrorContext(ctx, "Error in NotifReceive()", "error", err)
			continue
		}

		resp := &libseccomp.ScmpNotifResp{
			ID:    req.ID,
			Error: 0,
			Val:   0,
			Flags: libseccomp.NotifRespFlagContinue,
		}

		// TOCTOU check
		if err := libseccomp.NotifIDValid(h.fd, req.ID); err != nil {
			logger.ErrorContext(ctx, "TOCTOU check failed: req.ID is no longer valid", "error", err)
			continue
		}

		h.handleReq(ctx, h.fd, req, resp)

		if err := libseccomp.NotifRespond(h.fd, resp); err != nil {
			logger.ErrorContext(ctx, "Error in NotifRespond", "error", err)
			continue
		}
	}
}

func (h *notifHandler) handleReq(ctx context.Context, notifFd libseccomp.ScmpFd, req *libseccomp.ScmpNotifReq, resp *libseccomp.ScmpNotifResp) {
	// TODO: Write test for handleReq
	logger := log.FromContext(ctx).With("req", req)
	ctx = log.ContextWithLogger(ctx, logger)
	logger.DebugContext(ctx, "Handling seccomp notification request")

	syscallName, err := req.Data.Syscall.GetName()
	if err != nil {
		logger.ErrorContext(ctx, "Error decoding syscall", "error", err, "syscall", req.Data.Syscall)
		// TODO: error handle
		// resp.Error = unix.EINVAL
		return
	}
	logger = logger.With("syscall", syscallName)
	ctx = log.ContextWithLogger(ctx, logger)
	logger.DebugContext(ctx, "Received syscall")

	resp.Flags |= SeccompUserNotifFlagContinue

	// ensure pid is registered in notifHandler.pidInfos
	pidInfo, err := h.getPidFdInfo(ctx, int(req.Pid))
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get pidInfo", "error", err)
		return
	}

	// threads shares file descriptors in the same process space.
	// so use tgid as pid to process socket file descriptors
	pid := pidInfo.tgid
	logger = logger.With("pid", pid)
	ctx = log.ContextWithLogger(ctx, logger)

	if pidInfo.pidType == THREAD {
		logger.DebugContext(ctx, fmt.Sprintf("pid %d is thread. use process's tgid %d as pid", req.Pid, pid))
	}

	// cleanup sockets when the process exits
	if syscallName == "_exit" || syscallName == "exit_group" {
		if pidInfo, ok := h.pidInfos[int(req.Pid)]; ok {
			err = syscall.Close(int(pidInfo.pidfd))
			if err != nil {
				logger.ErrorContext(ctx, "failed to close pidfd", "error", err, "pidInfo", pidInfo)
			}
			delete(h.pidInfos, int(req.Pid))
		}
		if pidInfo.pidType == THREAD {
			logger.InfoContext(ctx, "thread is removed", "tgid", pid)
		}

		if pidInfo.pidType == PROCESS {
			if proc, ok := h.processes[pid]; ok {
				for _, sock := range proc.sockets {
					sock.removeSocket(ctx)
				}
				delete(h.processes, pid)
			}
			if memfd, ok := h.memfds[pid]; ok {
				syscall.Close(memfd)
				delete(h.memfds, pid)
			}
			logger.InfoContext(ctx, "process is removed")
		}
		return
	}

	sockfd := int(req.Data.Args[0])
	logger = logger.With("sockfd", sockfd)
	ctx = log.ContextWithLogger(ctx, logger)

	// remove socket when closed
	if syscallName == "close" { // TODO: handle shutdown(2)
		h.removeSocket(ctx, pid, sockfd)
		return
	}

	sock := h.getSocket(ctx, pid, sockfd)
	if sock == nil {
		sock, err = h.registerSocket(ctx, pid, sockfd)
		if err != nil {
			logger.ErrorContext(ctx, "failed to register socket", "error", err)
			return
		}
	}
	logger = logger.With("state", sock.state.String())

	if h.featureRDMA && syscallName == "connect" {
		ok := h.initializeRsocket(ctx, notifFd, req, resp, pid, sock)
		if ok {
			return
		}
	}

	switch sock.state {
	case NotBypassable:
		// sometimes close(2) is not called for the fd.
		// To handle such condition, re-register fd when connect or bind is called for not bypassable fd.
		if syscallName == "connect" || syscallName == "bind" {
			logger.DebugContext(ctx, "re-registering socket")
			h.removeSocket(ctx, pid, sockfd)
			sock, err = h.registerSocket(ctx, pid, sockfd)
			if err != nil {
				logger.ErrorContext(ctx, "failed to re-register socket", "error", err)
				return
			}
		}
		if sock.state != NotBypassed {
			return
		}

		// when sock.state == NotBypassed, continue
	case Bypassed:
		if syscallName != "getpeername" && syscallName != "getsockname" {
			return
		}
	default:
	}

	// TODO: resp.Flags &= (^uint32(SeccompUserNotifFlagContinue))

	logger.DebugContext(ctx, "Handling syscall")
	switch syscallName {
	case "bind":
		sock.handleSysBind(ctx, notifFd, req, resp, h, pid)
	case "listen":
		sock.handleSysListen(ctx, notifFd, req, resp, h, pid)
	case "accept":
		sock.handleSysAccept(ctx, notifFd, req, resp, h, pid)
	case "accept4":
		sock.handleSysAccept4(ctx, notifFd, req, resp, h, pid)
	case "connect":
		sock.handleSysConnect(ctx, notifFd, req, resp, h, pid)
	case "setsockopt":
		sock.handleSysSetsockopt(ctx, notifFd, req, resp, h, pid)
	case "fcntl":
		sock.handleSysFcntl(ctx, notifFd, req, resp)
	case "getpeername":
		sock.handleSysGetpeername(ctx, notifFd, req, resp, h, pid)
	case "getsockname":
		sock.handleSysGetsockname(ctx, notifFd, req, resp, h, pid)
	default:
		logger.ErrorContext(ctx, "Unknown syscall")
		// TODO: error handle
		return
	}
}

// getFdInProcess get the file descriptor in other process
func (h *notifHandler) getFdInProcess(ctx context.Context, pid, targetFd int) (int, error) {
	targetPidfd, err := h.getPidFdInfo(ctx, pid)
	if err != nil {
		return 0, fmt.Errorf("pidfd Open failed: %s", err)
	}

	fd, err := unix.PidfdGetfd(targetPidfd.pidfd, targetFd, 0)
	if err != nil {
		return 0, fmt.Errorf("pidfd GetFd failed: %s", err)
	}

	return fd, nil
}

// getSocketArgs retrieves socket(2) arguments from fd.
// return values are (sock_domain, sock_type, sock_protocol, error)
func getSocketArgs(ctx context.Context, sockfd int) (int, int, int, error) {
	logger := log.FromContext(ctx)
	logger.DebugContext(ctx, "got sockfd in getSocketArgs()")
	sock_domain, err := syscall.GetsockoptInt(sockfd, syscall.SOL_SOCKET, syscall.SO_DOMAIN)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("getsockopt(SO_DOMAIN) failed: %s", err)
	}

	sock_type, err := syscall.GetsockoptInt(sockfd, syscall.SOL_SOCKET, syscall.SO_TYPE)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("getsockopt(SO_TYPE) failed: %s", err)
	}

	sock_protocol, err := syscall.GetsockoptInt(sockfd, syscall.SOL_SOCKET, syscall.SO_PROTOCOL)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("getsockopt(SO_PROTOCOL) failed: %s", err)
	}

	return sock_domain, sock_type, sock_protocol, nil
}

// registerSocket is derived from:
//   https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/bypass4netns/bypass4netns.go#L386
//
// Original copyright:
//   Copyright [yyyy] [name of copyright owner]
//
// Licensed under the Apache License, Version 2.0.
func (h *notifHandler) registerSocket(ctx context.Context, pid int, sockfd int) (*socketStatus, error) {
	logger := log.FromContext(ctx).With("func", "registerSocket")
	proc, ok := h.processes[pid]
	if !ok {
		proc = newProcessStatus()
		h.processes[pid] = proc
		logger.DebugContext(ctx, "process is registered")
	}

	sock, ok := proc.sockets[sockfd]
	if ok {
		logger.WarnContext(ctx, "socket is already registered")
		return sock, nil
	}

	// If the pid is thread, its process can have corresponding socket
	procInfo, ok := h.pidInfos[int(pid)]
	if ok && procInfo.pidType == THREAD {
		return nil, fmt.Errorf("unexpected procInfo")
	}

	sockFdHost, err := h.getFdInProcess(ctx, int(pid), sockfd)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(sockFdHost)

	sockDomain, sockType, sockProtocol, err := getSocketArgs(ctx, sockFdHost)
	sock = newSocketStatus(pid, sockfd, sockDomain, sockType, sockProtocol)
	if err != nil {
		// non-socket fd is not bypassable
		sock.state = NotBypassable
		logger.DebugContext(ctx, "failed to get socket args", "error", err)
	} else {
		if sockDomain != syscall.AF_INET && sockDomain != syscall.AF_INET6 {
			// non IP sockets are not handled.
			sock.state = NotBypassable
			logger.DebugContext(ctx, fmt.Sprintf("socket domain=0x%x", sockDomain))
		} else if sockType != syscall.SOCK_STREAM {
			// only accepting TCP socket
			// TODO: datagram
			sock.state = NotBypassable
			logger.DebugContext(ctx, fmt.Sprintf("socket type=0x%x", sockType))
		} else {
			// only newly created socket is allowed.
			_, err := syscall.Getpeername(sockFdHost)
			if err == nil {
				logger.InfoContext(ctx, "socket is already connected. socket is created via accept or forked")
				sock.state = NotBypassable
			}
		}
	}

	proc.sockets[sockfd] = sock
	if sock.state == NotBypassable {
		logger.DebugContext(ctx, "socket is registered", "state", sock.state.String())
	} else {
		logger.InfoContext(ctx, "socket is registered", "state", sock.state.String())
	}

	return sock, nil
}

func (h *notifHandler) getSocket(_ context.Context, pid int, sockfd int) *socketStatus {
	proc, ok := h.processes[pid]
	if !ok {
		return nil
	}
	sock := proc.sockets[sockfd]
	return sock
}

func (h *notifHandler) removeSocket(ctx context.Context, pid int, sockfd int) {
	logger := log.FromContext(ctx).With("func", "removeSocket")
	defer logger.DebugContext(ctx, "socket is removed")
	proc, ok := h.processes[pid]
	if !ok {
		return
	}
	sock, ok := proc.sockets[sockfd]
	if ok {
		sock.removeSocket(ctx)
	}
	delete(proc.sockets, sockfd)
}

// getPidFdInfo is derived from:
//   https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/bypass4netns/bypass4netns.go#L281
//
// Original copyright:
//   Copyright [yyyy] [name of copyright owner]
//
// Licensed under the Apache License, Version 2.0.
func (h *notifHandler) getPidFdInfo(ctx context.Context, pid int) (*pidInfo, error) {
	logger := log.FromContext(ctx).With("func", "getPidFdInfo", "pid", pid)

	// retrieve pidfd from cache
	if pidfd, ok := h.pidInfos[pid]; ok {
		return &pidfd, nil
	}

	targetPidfd, err := unix.PidfdOpen(int(pid), 0)
	if err == nil {
		info := pidInfo{
			pidType: PROCESS,
			pidfd:   targetPidfd,
			tgid:    pid, // process's pid is equal to its tgid
		}
		h.pidInfos[pid] = info
		return &info, nil
	}

	// pid can be thread and pidfd_open fails with thread's pid.
	// retrieve process's pid (tgid) from /proc/<pid>/status and retry to get pidfd with the tgid.
	logger.WarnContext(ctx, "pidfd Open failed: this pid maybe thread and retrying with tgid", "error", err)
	st, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return nil, fmt.Errorf("failed to read %d's status err=%q", pid, err)
	}

	nextTgid := -1
	for _, s := range strings.Split(string(st), "\n") {
		if strings.Contains(s, "Tgid") {
			tgids := strings.Split(s, "\t")
			if len(tgids) < 2 {
				return nil, fmt.Errorf("unexpected /proc/%d/status len=%q status=%q", pid, len(tgids), string(st))
			}
			tgid, err := strconv.Atoi(tgids[1])
			if err != nil {
				return nil, fmt.Errorf("unexpected /proc/%d/status err=%q status=%q", pid, err, string(st))
			}
			nextTgid = tgid
		}
		if nextTgid > 0 {
			break
		}
	}
	if nextTgid < 0 {
		logger.ErrorContext(ctx, fmt.Sprintf("cannot get Tgid from /proc/%d/status", pid), "status", string(st))
	}
	targetPidfd, err = unix.PidfdOpen(nextTgid, 0)
	if err != nil {
		return nil, fmt.Errorf("pidfd Open failed with Tgid: pid=%d %s", nextTgid, err)
	}

	logger.InfoContext(ctx, "successfully got pidfd", "tgid", nextTgid)
	info := pidInfo{
		pidType: THREAD,
		pidfd:   targetPidfd,
		tgid:    nextTgid,
	}
	h.pidInfos[pid] = info
	return &info, nil
}

func (h *notifHandler) readSockaddrFromProcess(ctx context.Context, pid int, offset uint64, addrlen uint64) (*sockaddr, error) {
	buf, err := h.readProcMem(ctx, pid, offset, addrlen)
	if err != nil {
		return nil, fmt.Errorf("failed to readProcMem pid %v offset 0x%x: %s", pid, offset, err)
	}
	return newSockaddr(buf)
}

func (h *notifHandler) writeSockaddrToProcess(ctx context.Context, pid int, offset uint64, addrlen uint64, sa *sockaddr) error {
	buf, err := sockaddrToByte(sa)
	if err != nil {
		return fmt.Errorf("failed to sockaddrToByte pid %v offset 0x%x sockaddr %v : %s", pid, offset, sa, err)
	}
	return h.writeProcMem(ctx, pid, offset, buf)
}

// readProcMem read data from memory of specified pid process at the specified offset.
func (h *notifHandler) readProcMem(ctx context.Context, pid int, offset uint64, len uint64) ([]byte, error) {
	buffer := make([]byte, len) // PATH_MAX

	memfd, err := h.openMem(ctx, pid)
	if err != nil {
		return nil, err
	}

	size, err := unix.Pread(memfd, buffer, int64(offset))
	if err != nil {
		return nil, err
	}

	return buffer[:size], nil
}

// writeProcMem writes data to memory of specified pid process at the specified offset.
func (h *notifHandler) writeProcMem(ctx context.Context, pid int, offset uint64, buffer []byte) error {
	memfd, err := h.openMem(ctx, pid)
	if err != nil {
		return err
	}

	size, err := unix.Pwrite(memfd, buffer, int64(offset))
	if err != nil {
		return err
	}
	if len(buffer) != size {
		return fmt.Errorf("data is not written successfully. expected size=%d actual size=%d", len(buffer), size)
	}

	return nil
}

func (h *notifHandler) openMem(ctx context.Context, pid int) (int, error) {
	logger := log.FromContext(ctx)

	if memfd, ok := h.memfds[pid]; ok {
		return memfd, nil
	}
	memfd, err := unix.Open(fmt.Sprintf("/proc/%d/mem", pid), unix.O_RDWR, 0o777)
	if err != nil {
		logger.WarnContext(ctx, "failed to open mem due to permission error. retrying with agent.")
		newMemfd, err := openMemWithNSEnter(ctx, pid)
		if err != nil {
			return 0, fmt.Errorf("failed to open mem with agent (pid=%d)", pid)
		}
		logger.InfoContext(ctx, "succeeded to open mem with agent. continue to process")
		memfd = newMemfd
	}
	h.memfds[pid] = memfd

	return memfd, nil
}

func openMemWithNSEnter(ctx context.Context, pid int) (int, error) {
	logger := log.FromContext(ctx)

	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return 0, err
	}

	// configure timeout
	timeout := &syscall.Timeval{
		Sec:  0,
		Usec: 500 * 1000,
	}
	err = syscall.SetsockoptTimeval(fds[0], syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, timeout)
	if err != nil {
		return 0, fmt.Errorf("failed to set receive timeout")
	}
	err = syscall.SetsockoptTimeval(fds[1], syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, timeout)
	if err != nil {
		return 0, fmt.Errorf("failed to set send timeout")
	}

	fd1File := os.NewFile(uintptr(fds[0]), "")
	defer fd1File.Close()
	fd1Conn, err := net.FileConn(fd1File)
	if err != nil {
		return 0, err
	}
	_ = fd1Conn

	selfExe, err := os.Executable()
	if err != nil {
		return 0, err
	}
	nsenter, err := exec.LookPath("nsenter")
	if err != nil {
		return 0, err
	}
	nsenterFlags := []string{
		"-t", strconv.Itoa(int(pid)),
		"-F",
	}
	selfPid := os.Getpid()
	ok, err := sameUserNS(int(pid), selfPid)
	if err != nil {
		return 0, fmt.Errorf("failed to check sameUserNS(%d, %d)", pid, selfPid)
	}
	if !ok {
		nsenterFlags = append(nsenterFlags, "-U", "--preserve-credentials")
	}
	nsenterFlags = append(nsenterFlags, "--", selfExe, fmt.Sprintf("--mem-nsenter-pid=%d", pid))
	cmd := exec.CommandContext(ctx, nsenter, nsenterFlags...)
	cmd.ExtraFiles = []*os.File{os.NewFile(uintptr(fds[1]), "")}
	stdout := bytes.Buffer{}
	cmd.Stdout = &stdout
	err = cmd.Start()
	if err != nil {
		return 0, fmt.Errorf("failed to exec mem open agent: %w", err)
	}
	memfd, recvMsgs, err := recvMsg(fd1Conn)
	if err != nil {
		return 0, fmt.Errorf("failed to receive message: %w: stdout=%q", err, stdout.String())
	}
	logger.DebugContext(ctx, fmt.Sprintf("recvMsgs=%q", string(recvMsgs)))
	err = cmd.Wait()
	if err != nil {
		return 0, err
	}

	return memfd, nil
}

// sameUserNS is derived from:
//   https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/util/util.go#L22
//
// Original copyright:
//   Copyright [yyyy] [name of copyright owner]
//
// Licensed under the Apache License, Version 2.0.
func sameUserNS(pidX, pidY int) (bool, error) {
	nsX := fmt.Sprintf("/proc/%d/ns/user", pidX)
	nsY := fmt.Sprintf("/proc/%d/ns/user", pidY)
	nsXResolved, err := os.Readlink(nsX)
	if err != nil {
		return false, err
	}
	nsYResolved, err := os.Readlink(nsY)
	if err != nil {
		return false, err
	}
	return nsXResolved == nsYResolved, nil
}

// recvMsg is derived from:
//   https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/util/util.go#L55
//
// Original copyright:
//   Copyright [yyyy] [name of copyright owner]
//
// Licensed under the Apache License, Version 2.0.
func recvMsg(via net.Conn) (int, []byte, error) {
	conn, ok := via.(*net.UnixConn)
	if !ok {
		return 0, nil, fmt.Errorf("failed to cast via to *net.UnixConn")
	}
	connf, err := conn.File()
	if err != nil {
		return 0, nil, err
	}
	socket := int(connf.Fd())
	defer connf.Close()

	buf := make([]byte, syscall.CmsgSpace(4))
	b := make([]byte, 500)
	//nolint:dogsled
	n, _, _, _, err := syscall.Recvmsg(socket, b, buf, 0)
	if err != nil {
		return 0, nil, err
	}

	var msgs []syscall.SocketControlMessage
	msgs, err = syscall.ParseSocketControlMessage(buf)
	if err != nil {
		return 0, nil, err
	}

	fds, err := syscall.ParseUnixRights(&msgs[0])
	if err != nil {
		return 0, nil, err
	}

	return fds[0], b[:n], err
}
