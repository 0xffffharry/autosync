package core

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/fsnotify/fsnotify"
)

const (
	filterModeExclude = "exclude"
	filterModeInclude = "include"
)

type SingleCore struct {
	ctx          context.Context
	logger       slog.Logger
	dir          string
	rclonePath   string
	rcloneConfig string
	args         []string
	filterRules  []*regexp.Regexp
	filterMode   string
	remotePaths  []string
	//
	watcher *fsnotify.Watcher
	tasks   []*Task
}

func NewSingleCore(
	ctx context.Context,
	logger slog.Logger,
	options CoreOptions,
) (Core, error) {
	s := &SingleCore{
		ctx:          ctx,
		logger:       logger,
		dir:          options.Dir,
		rclonePath:   options.RclonePath,
		rcloneConfig: options.RcloneConfig,
		args:         options.Arg,
		filterMode:   options.FilterMode,
		remotePaths:  options.RemotePath,
	}
	if s.dir == "" {
		return nil, fmt.Errorf("missing dir")
	}
	if s.rclonePath == "" {
		return nil, fmt.Errorf("missing rclone_path")
	}
	if s.rcloneConfig == "" {
		return nil, fmt.Errorf("missing rclone_config")
	}
	if len(s.remotePaths) == 0 {
		return nil, fmt.Errorf("missing remote_path")
	}
	switch s.filterMode {
	case "":
		s.filterMode = filterModeExclude
	case filterModeExclude, filterModeInclude:
		if len(options.FilterRule) == 0 {
			return nil, fmt.Errorf("missing filter_rule")
		}
	default:
		return nil, fmt.Errorf("invalid filter_mode: %s", s.filterMode)
	}
	if len(options.FilterRule) > 0 {
		s.filterRules = make([]*regexp.Regexp, 0, len(options.FilterRule))
		for i, r := range options.FilterRule {
			re, err := regexp.Compile(r)
			if err != nil {
				return nil, fmt.Errorf("invalid filter_rule[%d]: %w", i, err)
			}
			s.filterRules = append(s.filterRules, re)
		}
	}
	return s, nil
}

func (s *SingleCore) newWatcher() (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fs watcher: %w", err)
	}
	err = filepath.Walk(s.dir, func(path string, d fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to watch fs: %w", err)
	}
	return watcher, nil
}

func (s *SingleCore) filter(path string) bool {
	if len(s.filterRules) == 0 {
		return true
	}
	for _, r := range s.filterRules {
		if r.MatchString(path) {
			return s.filterMode == filterModeInclude
		}
	}
	return s.filterMode == filterModeExclude
}

func (s *SingleCore) newCommand(ctx context.Context, remotePath string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, s.rclonePath)
	cmd.Dir = s.dir
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env,
		"RCLONE_IGNORE_ERRORS=true",
	)
	cmd.Args = append(cmd.Args,
		"sync",
		"-v",
		"--config",
		s.rcloneConfig,
		s.dir,
		remotePath,
	)
	if len(s.filterRules) > 0 {
		var args []string
		for _, r := range s.filterRules {
			if s.filterMode == filterModeInclude {
				args = append(args, "--include", "'"+r.String()+"'")
			} else {
				args = append(args, "--exclude", "'"+r.String()+"'")
			}
		}
		cmd.Args = append(cmd.Args, args...)
	}
	return cmd
}

func (s *SingleCore) sync(ctx context.Context, path string) {
	s.logger.Info(
		"start a sync task",
		slog.String("local_path", s.dir),
		slog.String("remote_path", path),
	)
	defer s.logger.Info(
		"sync task done",
		slog.String("local_path", s.dir),
		slog.String("remote_path", path),
	)
	cmd := s.newCommand(ctx, path)
	err := cmd.Run()
	if err != nil {
		s.logger.Error(
			"sync task error",
			slog.String("local_path", s.dir),
			slog.String("remote_path", path),
			slog.String("error", err.Error()),
		)
	} else {
		s.logger.Info(
			"sync task success",
			slog.String("local_path", s.dir),
			slog.String("remote_path", path),
		)
	}
}

func (s *SingleCore) call() {
	for _, t := range s.tasks {
		t.Call()
	}
}

func (s *SingleCore) close() {
	s.watcher.Close()
	for _, t := range s.tasks {
		t.Close()
	}
}

func (s *SingleCore) Init() error {
	watcher, err := s.newWatcher()
	if err != nil {
		return err
	}
	s.watcher = watcher
	s.tasks = make([]*Task, 0, len(s.remotePaths))
	for _, p := range s.remotePaths {
		s.tasks = append(s.tasks, NewTask(s.ctx, p, s.sync))
	}
	return nil
}

func (s *SingleCore) Run() {
	defer s.close()
	s.logger.Info("watcher is started")
	defer s.logger.Info("watcher is stopped")
	for {
		select {
		case <-s.ctx.Done():
			return
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			s.logger.Debug(
				"watcher event",
				slog.String("event", event.Op.String()),
				slog.String("path", event.Name),
			)
			switch {
			case event.Has(fsnotify.Create):
				f, err := os.Stat(event.Name)
				if err != nil {
					s.logger.Error(
						"failed to get path stat",
						slog.String("event", event.Op.String()),
						slog.String("path", event.Name),
						slog.String("error", err.Error()),
					)
				} else if f.IsDir() {
					err = s.watcher.Add(event.Name)
					if err != nil {
						s.logger.Error(
							"failed to watch dir",
							slog.String("event", event.Op.String()),
							slog.String("path", event.Name),
							slog.String("error", err.Error()),
						)
					}
				}
			case event.Has(fsnotify.Write):
			case event.Has(fsnotify.Remove):
				if runtime.GOOS == "windows" {
					s.watcher.Remove(event.Name)
				}
			case event.Has(fsnotify.Rename):
			default:
				continue
			}
			if s.filter(event.Name) {
				s.call()
			}
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			s.logger.Error("watcher error", slog.String("error", err.Error()))
		}
	}
}
