package main

import (
	"autosync/core"
	"autosync/util"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var mainCommand = &cobra.Command{
	Use: "autosync",
	Run: func(cmd *cobra.Command, args []string) {
		code := run()
		if code != 0 {
			os.Exit(code)
		}
	},
}

var (
	paramConfig   string
	paramLogLevel string
	logLevel      slog.Level

	envRclonePath   string
	envRcloneConfig string
)

func init() {
	mainCommand.PersistentFlags().StringVarP(&paramConfig, "config", "c", "config.json", "config file")
	mainCommand.PersistentFlags().StringVarP(&paramLogLevel, "log-level", "l", "info", "log level")
	envRclonePath = os.Getenv("RCLONE_PATH")
	if envRclonePath == "" {
		envRclonePath = "rclone"
	}
	envRcloneConfig = os.Getenv("RCLONE_CONFIG")
}

func main() {
	err := mainCommand.Execute()
	if err != nil {
		panic(err)
	}
}

type Options struct {
	CoreOptions util.Listable[core.CoreOptions] `json:"core"`
}

func run() int {
	err := logLevel.UnmarshalText([]byte(paramLogLevel))
	if err != nil {
		slog.Error("parse log level failed", slog.String("error", err.Error()))
		return 1
	}
	content, err := os.ReadFile(paramConfig)
	if err != nil {
		slog.Error("read config file failed", slog.String("error", err.Error()))
		return 1
	}
	var options Options
	err = json.Unmarshal(content, &options)
	if err != nil {
		slog.Error("parse config file failed", slog.String("error", err.Error()))
		return 1
	}
	for i := range options.CoreOptions {
		if options.CoreOptions[i].RclonePath == "" {
			options.CoreOptions[i].RclonePath = envRclonePath
		}
		if options.CoreOptions[i].RcloneConfig == "" {
			options.CoreOptions[i].RcloneConfig = envRcloneConfig
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	c, err := core.New(ctx, *logger, options.CoreOptions)
	if err != nil {
		logger.Error(err.Error())
		return 1
	}
	err = c.Init()
	if err != nil {
		logger.Error(err.Error())
		return 1
	}
	go signalHandle(*logger, cancel)
	c.Run()
	return 0
}

func signalHandle(logger slog.Logger, cancel func()) {
	ch := make(chan os.Signal, 1)
	defer close(ch)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, os.Interrupt)
	<-ch
	logger.Warn("receive signal, exiting...")
	cancel()
}
