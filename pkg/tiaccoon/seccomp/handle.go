package seccomp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/hiroyaonoe/tiaccoon/pkg/log"
	"github.com/hiroyaonoe/tiaccoon/pkg/tiaccoon/accesscontrol"
	"github.com/hiroyaonoe/tiaccoon/pkg/tiaccoon/destination"
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

type Handler struct {
	sae *accesscontrol.Entries
	cae *accesscontrol.Entries
	de  *destination.Entries

	l      net.Listener
	closed bool

	socketPath  string
	myVIP       net.IP
	featureRDMA bool
}

func NewHandler(sae, cae *accesscontrol.Entries, de *destination.Entries, socketPath string, myVIP net.IP, featureRDMA bool) *Handler {
	return &Handler{
		sae:         sae,
		cae:         cae,
		de:          de,
		closed:      false,
		socketPath:  socketPath,
		myVIP:       myVIP,
		featureRDMA: featureRDMA,
	}
}

func (h *Handler) Close(ctx context.Context) {
	logger := log.FromContext(ctx).With("component", "seccomp handler")
	logger.DebugContext(ctx, "Closing seccomp handler")
	h.closed = true
	if h.l != nil {
		h.l.Close()
	}
}

func (h *Handler) Start(ctx context.Context) {
	logger := log.FromContext(ctx).With("component", "seccomp handler")
	ctx = log.ContextWithLogger(ctx, logger)
	logger.DebugContext(ctx, "Starting seccomp handler")
	var err error

	// Create directory if it doesn't exist
	dir := filepath.Dir(h.socketPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.ErrorContext(ctx, "Failed to create directory for seccomp notify socket", "error", err, "dir", dir)
		return
	}

	// This function is derived from:
	//   https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/bypass4netns/bypass4netns.go#L762
	//
	// Original copyright:
	//   Copyright [yyyy] [name of copyright owner]
	//
	// Licensed under the Apache License, Version 2.0.
	h.l, err = net.Listen("unix", h.socketPath)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to listen seccomp notify socket", "error", err, "socketPath", h.socketPath)
		return // TODO: Fatal
	}
	logger.DebugContext(ctx, "Listening seccomp notify socket", "socketPath", h.socketPath)

	for {
		conn, err := h.l.Accept()
		if err != nil {
			if h.closed {
				logger.DebugContext(ctx, "Closing seccomp notify socket")
				return
			}
			logger.ErrorContext(ctx, "Failed to accept seccomp notify socket", "error", err)
			continue
		}
		socket, err := conn.(*net.UnixConn).File()
		conn.Close()
		if err != nil {
			logger.ErrorContext(ctx, "Failed to get file descriptor from seccomp notify socket", "error", err)
			continue
		}
		logger.DebugContext(ctx, "Received seccomp notify socket", "socket", socket)
		newFd, state, err := handleNewMessage(int(socket.Fd()))
		socket.Close()
		if err != nil {
			logger.ErrorContext(ctx, "Failed to receive seccomp file descriptor", "error", err)
			continue
		}

		logger.InfoContext(ctx, "Received seccomp file descriptor", "fd", newFd)
		notifHandler := h.newNotifHandler(newFd, state, h.sae, h.cae, h.de, h.myVIP, h.featureRDMA)

		logger.InfoContext(ctx, "Start to handle seccomp notif", "fd", newFd)
		go notifHandler.handle(ctx)
	}
}

// handleNewMessage is derived from:
//   https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/bypass4netns/bypass4netns.go#L227
//
// Original copyright:
//   Copyright [yyyy] [name of copyright owner]
//
// Licensed under the Apache License, Version 2.0.
func handleNewMessage(sockfd int) (uintptr, *specs.ContainerProcessState, error) {
	const maxNameLen = 4096
	stateBuf := make([]byte, maxNameLen)
	oobSpace := unix.CmsgSpace(4)
	oob := make([]byte, oobSpace)

	n, oobn, _, _, err := unix.Recvmsg(sockfd, stateBuf, oob, 0)
	if err != nil {
		return 0, nil, err
	}
	if n >= maxNameLen || oobn != oobSpace {
		return 0, nil, fmt.Errorf("recvfd: incorrect number of bytes read (n=%d oobn=%d)", n, oobn)
	}

	// Truncate.
	stateBuf = stateBuf[:n]
	oob = oob[:oobn]

	scms, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		return 0, nil, err
	}
	if len(scms) != 1 {
		return 0, nil, fmt.Errorf("recvfd: number of SCMs is not 1: %d", len(scms))
	}
	scm := scms[0]

	fds, err := unix.ParseUnixRights(&scm)
	if err != nil {
		return 0, nil, err
	}

	containerProcessState := &specs.ContainerProcessState{}
	err = json.Unmarshal(stateBuf, containerProcessState)
	if err != nil {
		closeStateFds(fds)
		return 0, nil, fmt.Errorf("cannot parse OCI state: %w", err)
	}

	fd, err := parseStateFds(containerProcessState.Fds, fds)
	if err != nil {
		closeStateFds(fds)
		return 0, nil, err
	}

	return fd, containerProcessState, nil
}

// closeStateFds is derived from:
//   https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/bypass4netns/bypass4netns.go#L34
//
// Original copyright:
//   Copyright [yyyy] [name of copyright owner]
//
// Licensed under the Apache License, Version 2.0.
func closeStateFds(recvFds []int) {
	for i := range recvFds {
		unix.Close(i)
	}
}

// parseStateFds is derived from:
//   https://github.com/rootless-containers/bypass4netns/blob/b9bca3046e413e80d9e556c22443e87d324de847/pkg/bypass4netns/bypass4netns.go#L44
//
// Original copyright:
//   Copyright [yyyy] [name of copyright owner]
//
// Licensed under the Apache License, Version 2.0.
//
// parseStateFds returns the seccomp-fd and closes the rest of the fds in recvFds.
// In case of error, no fd is closed.
// StateFds is assumed to be formatted as specs.ContainerProcessState.Fds and
// recvFds the corresponding list of received fds in the same SCM_RIGHT message.
func parseStateFds(stateFds []string, recvFds []int) (uintptr, error) {
	// Let's find the index in stateFds of the seccomp-fd.
	idx := -1
	err := false

	for i, name := range stateFds {
		if name == specs.SeccompFdName && idx == -1 {
			idx = i
			continue
		}

		// We found the seccompFdName twice. Error out!
		if name == specs.SeccompFdName && idx != -1 {
			err = true
		}
	}

	if idx == -1 || err {
		return 0, errors.New("seccomp fd not found or malformed containerProcessState.Fds")
	}

	if idx >= len(recvFds) || idx < 0 {
		return 0, errors.New("seccomp fd index out of range")
	}

	fd := uintptr(recvFds[idx])

	for i := range recvFds {
		if i == idx {
			continue
		}

		unix.Close(recvFds[i])
	}

	return fd, nil
}
