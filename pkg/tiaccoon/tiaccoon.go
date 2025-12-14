package tiaccoon

import (
	"context"
	"net"

	"github.com/hiroyaonoe/tiaccoon/pkg/log"
	"github.com/hiroyaonoe/tiaccoon/pkg/tiaccoon/manage"
	"github.com/hiroyaonoe/tiaccoon/pkg/tiaccoon/seccomp"
)

func Start(ctx context.Context, socketPath string, defaultPolicy bool, myVIP net.IP, featureRDMA bool) {
	logger := log.FromContext(ctx)

	logger.InfoContext(ctx, "Starting tiaccoon")

	manager := manage.NewManager(defaultPolicy, myVIP, featureRDMA)
	sae, cae, de := manager.Start(ctx)
	defer manager.Close(ctx)

	sHandler := seccomp.NewHandler(sae, cae, de, socketPath, myVIP, featureRDMA)

	go sHandler.Start(ctx)
	defer sHandler.Close(ctx)

	<-ctx.Done()
}
