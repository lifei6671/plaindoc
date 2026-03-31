package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/buildinfo"
	"github.com/lifei6671/plaindoc/apps/server/internal/config"
	"github.com/lifei6671/plaindoc/apps/server/internal/logit"
	"github.com/lifei6671/plaindoc/apps/server/internal/server"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/ssr/pool"
	"github.com/lifei6671/plaindoc/apps/server/internal/ssr/worker"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

const startupDatabaseMigrationTimeout = 5 * time.Minute

func main() {
	if handleMetaCommand(os.Args[1:], os.Stdout) {
		return
	}

	loadDotEnvCandidates()

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

	logger := logit.NewLoggerWithWriter(cfg.LogLevel, logWriter)

	if err := validateSSRWorkerRuntime(cfg); err != nil {
		logger.Error("ssr worker runtime validation failed", logit.Error("error", err))
		log.Fatalf("ssr worker runtime validation failed: %v", err)
	}

	var ssrWorkerPool *pool.Pool
	var ssrDispatcher *pool.Dispatcher
	if cfg.SSRWorker.Enabled {
		ssrWorkerPool = pool.New(pool.Config{
			WorkerCount: cfg.SSRWorker.Count,
			Worker: worker.Config{
				Exec:            cfg.SSRWorker.Exec,
				Entry:           cfg.SSRWorker.Entry,
				ProtocolVersion: cfg.SSRWorker.ProtocolVersion,
				RenderTimeout:   cfg.SSRWorker.RenderTimeout,
				MaxPayloadBytes: cfg.SSRWorker.MaxPayloadBytes,
				Logger:          logger,
			},
			Logger: logger,
		})

		workerPoolStartContext, cancelWorkerPoolStart := context.WithTimeout(context.Background(), cfg.SSRWorker.StartTimeout)
		if err := ssrWorkerPool.Start(workerPoolStartContext); err != nil {
			cancelWorkerPoolStart()
			logger.Error("ssr worker pool start failed", logit.Error("error", err))
			log.Fatalf("ssr worker pool start failed: %v", err)
		}
		cancelWorkerPoolStart()
		ssrDispatcher = pool.NewDispatcher(ssrWorkerPool)

		defer func() {
			workerPoolStopContext, cancelWorkerPoolStop := context.WithTimeout(context.Background(), 3*time.Second)
			if stopErr := ssrWorkerPool.Close(workerPoolStopContext); stopErr != nil {
				logger.Error("ssr worker pool close failed", logit.Error("error", stopErr))
			}
			cancelWorkerPoolStop()
		}()
	}

	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: cfg.Database.Driver,
		DSN:    cfg.Database.DSN,
	})
	if err != nil {
		logger.Error("open database failed", logit.Error("error", err))
		log.Fatalf("open database failed: %v", err)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			logger.Error("close database failed", logit.Error("error", closeErr))
		}
	}()
	logger.Info("database connected", "db_driver", database.Driver)

	if cfg.Database.AutoMigrate {
		logger.Info("database migrations starting", "timeout", startupDatabaseMigrationTimeout.String())
		migrateCtx, cancelMigrate := context.WithTimeout(context.Background(), startupDatabaseMigrationTimeout)
		defer cancelMigrate()
		if err := storage.MigrateUpWithOptions(migrateCtx, database.ORM, database.Driver, storage.MigrateOptions{
			Logger: logger,
		}); err != nil {
			logger.Error("database migrate up failed", logit.Error("error", err))
			log.Fatalf("database migrate up failed: %v", err)
		}
		logger.Info("database migrations applied")
	}

	systemConfigRepo := repository.NewGormSystemConfigRepository(database.ORM)
	dataRetentionCleanupService := service.NewDataRetentionCleanupService(database.ORM, systemConfigRepo)
	dataRetentionCleanupCtx, cancelDataRetentionCleanup := context.WithCancel(context.Background())
	defer cancelDataRetentionCleanup()
	go runDataRetentionCleanupLoop(dataRetentionCleanupCtx, logger, dataRetentionCleanupService)

	router := server.NewRouterWithSSR(cfg, logger, database.ORM, ssrDispatcher)
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
		"web_dist_dir", cfg.WebDistDir,
		"ssr_worker_enabled", cfg.SSRWorker.Enabled,
		"ssr_worker_count", cfg.SSRWorker.Count,
		"ssr_render_timeout", cfg.SSRWorker.RenderTimeout.String(),
	)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server exited unexpectedly", "error", err.Error())
		log.Fatalf("server exited: %v", err)
	}
}

// 处理轻量元信息命令（例如 version）：命中后直接打印并退出，不进入服务启动流程。
func handleMetaCommand(args []string, output io.Writer) bool {
	if len(args) == 0 {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "version", "--version", "-version", "-v":
		printBuildInfo(output)
		return true
	default:
		return false
	}
}

func printBuildInfo(output io.Writer) {
	currentBuildInfo := buildinfo.Current()
	_, _ = fmt.Fprintf(
		output,
		"version: %s\ncommit_sha: %s\nbuild_time_utc: %s\ngo_version: %s\n",
		currentBuildInfo.Version,
		currentBuildInfo.CommitSHA,
		currentBuildInfo.BuildTimeUTC,
		currentBuildInfo.GoVersion,
	)
}

func validateSSRWorkerRuntime(cfg config.Config) error {
	if !cfg.SSRWorker.Enabled {
		return nil
	}

	if _, err := exec.LookPath(cfg.SSRWorker.Exec); err != nil {
		return fmt.Errorf("find SSR_WORKER_EXEC %q failed: %w", cfg.SSRWorker.Exec, err)
	}

	entry := strings.TrimSpace(cfg.SSRWorker.Entry)
	if entry == "" {
		return errors.New("SSR_WORKER_ENTRY is empty")
	}

	normalizedEntry := entry
	if !filepath.IsAbs(normalizedEntry) {
		normalizedEntry = filepath.Clean(normalizedEntry)
	}

	entryInfo, err := os.Stat(normalizedEntry)
	if err != nil {
		return fmt.Errorf("stat SSR_WORKER_ENTRY %q failed: %w", normalizedEntry, err)
	}
	if entryInfo.IsDir() {
		return fmt.Errorf("SSR_WORKER_ENTRY %q must be a file, got directory", normalizedEntry)
	}

	return nil
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

func loadDotEnvCandidates() {
	candidatePaths := []string{
		".env",
		"apps/server/.env",
		"cmd/server/.env",
		"apps/server/cmd/server/.env",
	}

	seenPaths := make(map[string]struct{}, len(candidatePaths))
	for _, candidatePath := range candidatePaths {
		normalizedPath := filepath.Clean(strings.TrimSpace(candidatePath))
		if normalizedPath == "" {
			continue
		}
		if _, exists := seenPaths[normalizedPath]; exists {
			continue
		}
		seenPaths[normalizedPath] = struct{}{}

		if err := loadDotEnvFile(normalizedPath); err != nil {
			log.Printf("load dotenv file %s failed: %v", normalizedPath, err)
		}
	}
}

func loadDotEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber += 1 {
		rawLine := strings.TrimSpace(scanner.Text())
		if rawLine == "" || strings.HasPrefix(rawLine, "#") {
			continue
		}
		if strings.HasPrefix(rawLine, "export ") {
			rawLine = strings.TrimSpace(strings.TrimPrefix(rawLine, "export "))
		}

		separatorIndex := strings.Index(rawLine, "=")
		if separatorIndex <= 0 {
			continue
		}
		key := strings.TrimSpace(rawLine[:separatorIndex])
		value := strings.TrimSpace(rawLine[separatorIndex+1:])
		if key == "" {
			continue
		}
		if _, alreadySet := os.LookupEnv(key); alreadySet {
			// 明确环境变量优先，避免覆盖外部注入配置。
			continue
		}

		unquotedValue, unquoteErr := parseDotEnvValue(value)
		if unquoteErr != nil {
			log.Printf("dotenv parse warning (%s:%d): %v", path, lineNumber, unquoteErr)
			continue
		}
		if setErr := os.Setenv(key, unquotedValue); setErr != nil {
			log.Printf("dotenv setenv warning (%s:%d): %v", path, lineNumber, setErr)
		}
	}
	return scanner.Err()
}

func parseDotEnvValue(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 2 {
		return trimmed, nil
	}

	if (strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"")) ||
		(strings.HasPrefix(trimmed, "'") && strings.HasSuffix(trimmed, "'")) {
		if strings.HasPrefix(trimmed, "'") {
			// 单引号按字面值处理，不做转义展开。
			return trimmed[1 : len(trimmed)-1], nil
		}
		unquoted, err := strconv.Unquote(trimmed)
		if err != nil {
			return "", err
		}
		return unquoted, nil
	}
	return trimmed, nil
}

func runDataRetentionCleanupLoop(
	ctx context.Context,
	logger *slog.Logger,
	cleanupService *service.DataRetentionCleanupService,
) {
	if cleanupService == nil {
		return
	}

	nextInterval := cleanupService.ResolveNextRunInterval(ctx)
	if nextInterval <= 0 {
		nextInterval = 60 * time.Minute
	}

	for {
		result, err := cleanupService.RunOnce(ctx)
		if err != nil {
			nextInterval = service.ResolveCleanupRetryInterval()
			if logger != nil {
				logger.Error(
					"data retention cleanup failed",
					logit.Error("error", err),
					slog.Duration("next_run_in", nextInterval),
				)
			}
		} else {
			nextInterval = cleanupService.ResolveNextRunInterval(ctx)
			if nextInterval <= 0 {
				nextInterval = 60 * time.Minute
			}
			if logger != nil {
				logger.Info(
					"data retention cleanup finished",
					slog.Bool("enabled", result.Policy.Enabled),
					slog.Int("schedule_minutes", result.Policy.ScheduleMinutes),
					slog.Int("cleanup_batch_size", result.Policy.CleanupBatchSize),
					slog.Any("cleanup_tables", result.Policy.CleanupTables),
					slog.Int64("deleted_audit_logs", result.DeletedAuditLogs),
					slog.Int64("deleted_auth_captcha_challenges", result.DeletedAuthCaptchaChallenges),
					slog.Int64("deleted_auth_risk_states", result.DeletedAuthRiskStates),
					slog.Int64("deleted_user_sessions", result.DeletedUserSessions),
					slog.Int64("deleted_document_image_assets", result.DeletedDocumentImageAssets),
					slog.Duration("duration", result.FinishedAt.Sub(result.StartedAt)),
					slog.Duration("next_run_in", nextInterval),
				)
			}
		}

		waitTimer := time.NewTimer(nextInterval)
		select {
		case <-ctx.Done():
			waitTimer.Stop()
			return
		case <-waitTimer.C:
		}
	}
}
