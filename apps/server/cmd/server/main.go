package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/lifei6671/plaindoc/apps/server/internal/config"
	"github.com/lifei6671/plaindoc/apps/server/internal/observability"
	"github.com/lifei6671/plaindoc/apps/server/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	logWriter, closeLogWriter, err := resolveLogWriter(cfg)
	if err != nil {
		log.Fatalf("resolve log writer failed: %v", err)
	}
	if closeLogWriter != nil {
		defer func() {
			if closeErr := closeLogWriter(); closeErr != nil {
				log.Printf("close log writer failed: %v", closeErr)
			}
		}()
	}

	logger := observability.NewLoggerWithWriter(cfg.LogLevel, logWriter)
	router := server.NewRouter(cfg, logger)
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ReadHeaderTimeout: cfg.ReadTimeout,
	}

	logger.Info("server starting",
		"addr", cfg.Addr,
		"env", cfg.Env,
		"log_level", cfg.LogLevel.String(),
		"log_output", cfg.LogOutput,
	)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server exited unexpectedly", "error", err.Error())
		log.Fatalf("server exited: %v", err)
	}
}

func resolveLogWriter(cfg config.Config) (io.Writer, func() error, error) {
	switch cfg.LogOutput {
	case "stdout":
		return os.Stdout, nil, nil
	case "file":
		file, err := os.OpenFile(cfg.LogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, nil, err
		}
		return file, file.Close, nil
	case "both":
		file, err := os.OpenFile(cfg.LogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, nil, err
		}
		writer := io.MultiWriter(os.Stdout, file)
		return writer, file.Close, nil
	default:
		// 中文注释：理论上不会触发（配置层已校验），此处仅保底防御。
		return nil, nil, errors.New("unsupported log output")
	}
}
