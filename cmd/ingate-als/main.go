package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
	kratoslog "github.com/go-kratos/kratos/v3/log"
	"github.com/lgc202/go-kit/version"

	"github.com/lgc202/ingate/internal/als"
	"github.com/lgc202/ingate/internal/als/conf"

	_ "go.uber.org/automaxprocs"
)

const defaultConfigFile = "configs/ingate-als.yaml"

func main() {
	configFile := flag.String("config", defaultConfigFile, "configuration file")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.Get().Text())
		return
	}
	if err := run(*configFile); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(configFile string) error {
	instanceID, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("read hostname: %w", err)
	}

	loaded := config.New(config.WithSource(file.NewSource(configFile)))
	defer loaded.Close()
	if err := loaded.Load(); err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	var bootstrap conf.Bootstrap
	if err := loaded.Scan(&bootstrap); err != nil {
		return fmt.Errorf("scan configuration: %w", err)
	}
	if err := bootstrap.Validate(); err != nil {
		return fmt.Errorf("validate configuration: %w", err)
	}

	logger, level := newLogger(bootstrap.GetLogging(), instanceID)
	kratoslog.SetDefault(logger)
	watchLogging(loaded, level, logger)
	app, err := als.NewApp(&bootstrap, logger, instanceID)
	if err != nil {
		return err
	}
	logger.Info("service starting", "config_file", configFile)
	return app.Run()
}

func newLogger(config *conf.Logging, instanceID string) (*slog.Logger, *kratoslog.LevelVar) {
	level := new(kratoslog.LevelVar)
	level.Set(kratoslog.ParseLevel(config.GetLevel()))
	format := kratoslog.FormatText
	if strings.EqualFold(config.GetFormat(), "json") {
		format = kratoslog.FormatJSON
	}
	handler := kratoslog.NewHandler(
		kratoslog.WithWriter(os.Stderr),
		kratoslog.WithFormat(format),
		kratoslog.WithLevel(level),
		kratoslog.WithAddSource(config.GetAddSource()),
	)
	logger := kratoslog.NewLogger(handler).With(
		"service.id", instanceID,
		"service.name", als.Name,
		"service.version", version.Get().String(),
	)
	return logger, level
}

// watchLogging 只热更新日志级别，其他 ALS 配置在重启后生效
func watchLogging(loaded config.Config, level *kratoslog.LevelVar, logger *slog.Logger) {
	if err := loaded.Watch("logging", func(_ string, value config.Value) {
		var next conf.Logging
		if err := value.Scan(&next); err != nil {
			logger.Error("ignore invalid logging configuration", "error", err)
			return
		}
		switch strings.ToLower(next.GetLevel()) {
		case "debug", "info", "warn", "error":
			level.Set(kratoslog.ParseLevel(next.GetLevel()))
			logger.Info("log level updated", "level", next.GetLevel())
		default:
			logger.Error("ignore invalid log level", "level", next.GetLevel())
		}
	}); err != nil {
		logger.Warn("configuration watch unavailable", "error", err)
	}
}
