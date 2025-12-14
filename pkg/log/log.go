package log

import (
	"context"
	"log/slog"
)

type loggerKey struct{}

// FromContext returns a logger from the context.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

// ContextWithLogger returns a new context with the logger.
func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}
