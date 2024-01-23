package core

import (
	"context"
	"fmt"
	"log/slog"
)

type Core interface {
	Init() error
	Run()
}

func New(ctx context.Context, logger slog.Logger, options []CoreOptions) (Core, error) {
	if len(options) == 0 {
		return nil, fmt.Errorf("missing options")
	}
	if len(options) == 1 {
		return NewSingleCore(ctx, *slog.New(logger.Handler().WithAttrs([]slog.Attr{slog.String("dir", options[0].Dir)})), options[0])
	}
	return NewCoreGroup(ctx, logger, options)
}
