package core

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

type CoreGroup struct {
	ctx    context.Context
	logger slog.Logger
	dirs   []string
	cores  []Core
}

func NewCoreGroup(
	ctx context.Context,
	logger slog.Logger,
	options []CoreOptions,
) (Core, error) {
	g := &CoreGroup{
		ctx:    ctx,
		logger: logger,
	}
	if len(options) == 0 {
		return nil, fmt.Errorf("missing options")
	}
	g.cores = make([]Core, len(options))
	g.dirs = make([]string, len(options))
	for i, o := range options {
		c, err := NewSingleCore(ctx, *slog.New(logger.Handler().WithAttrs([]slog.Attr{slog.String("dir", o.Dir)})), o)
		if err != nil {
			return nil, fmt.Errorf("create core[%d] failed: %w", i, err)
		}
		g.cores[i] = c
		g.dirs[i] = o.Dir
	}
	return g, nil
}

func (g *CoreGroup) Init() error {
	var err error
	for i, c := range g.cores {
		err = c.Init()
		if err != nil {
			return fmt.Errorf(
				"core[%d] init failed: %w, dir: %s",
				i,
				err,
				g.dirs[i],
			)
		}
	}
	return nil
}

func (g *CoreGroup) Run() {
	var wg sync.WaitGroup
	for _, c := range g.cores {
		wg.Add(1)
		go func(c Core) {
			defer wg.Done()
			c.Run()
		}(c)
	}
	wg.Wait()
}
