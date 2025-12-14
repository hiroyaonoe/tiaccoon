package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"log/slog"

	"github.com/hiroyaonoe/tiaccoon/pkg/log"
	"github.com/hiroyaonoe/tiaccoon/pkg/tiaccoon"
	"github.com/hiroyaonoe/tiaccoon/pkg/version"
	"golang.org/x/sys/unix"
)

func main() {
	unix.Umask(0o077) // https://github.com/golang/go/issues/11822#issuecomment-123850227
	xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")

	var (
		versionFlag      bool
		helpFlag         bool
		logLevelStr      string
		logSource        bool
		socketPath       string
		defaultPolicyStr string
		myVIPStr         string
		featureRDMA      bool
	)
	flag.BoolVar(&versionFlag, "version", false, "Print the version")
	flag.BoolVar(&helpFlag, "help", false, "Print help information")
	flag.StringVar(&logLevelStr, "log-level", "info", "Set the log level (debug, info, warn, error)")
	flag.BoolVar(&logSource, "log-source", false, "Include source information in log output")
	flag.StringVar(&socketPath, "socket", filepath.Join(xdgRuntimeDir, "tiaccoon.sock"), "Socket path for seccomp notify")
	flag.StringVar(&defaultPolicyStr, "default-policy", "", "Set the default policy (allow, deny)")
	flag.StringVar(&myVIPStr, "ip", "", "Set the IP of the container")
	flag.BoolVar(&featureRDMA, "feature-rdma", false, "Enable feature RDMA")
	flag.Parse()

	if versionFlag {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	if helpFlag {
		flag.Usage()
		os.Exit(0)
	}

	var logLevel slog.Level
	switch logLevelStr {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	if socketPath == "" && xdgRuntimeDir == "" {
		fmt.Println("--socket or $XDG_RUNTIME_DIR are needed to be set")
		flag.Usage()
		os.Exit(1)
	}

	var defaultPolicy bool
	switch defaultPolicyStr {
	case "allow":
		defaultPolicy = true
	case "deny":
		defaultPolicy = false
	default:
		fmt.Println("--default-policy must be either 'allow' or 'deny'")
		flag.Usage()
		os.Exit(1)
	}

	myVIP := net.ParseIP(myVIPStr)

	os.Exit(run(logLevel, logSource, socketPath, defaultPolicy, myVIP, featureRDMA))
}

func run(logLevel slog.Level, logSource bool, socketPath string, defaultPolicy bool, myVIP net.IP, featureRDMA bool) int {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: logSource,
		Level:     logLevel,
	}))
	slog.SetDefault(logger)
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	ctx = log.ContextWithLogger(ctx, logger)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-quit
		logger.InfoContext(ctx, "Received signal, shutting down")
		cancel()
	}()

	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.ErrorContext(ctx, "Cannot cleanup socket file", "error", err)
		return 1
	}
	defer func() {
		if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			logger.ErrorContext(ctx, "Cannot cleanup socket file", "error", err)
		}
	}()

	tiaccoon.Start(ctx, socketPath, defaultPolicy, myVIP, featureRDMA)
	return 0
}
