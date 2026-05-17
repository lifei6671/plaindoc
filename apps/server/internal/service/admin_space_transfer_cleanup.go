package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

const defaultAdminSpaceTransferCleanupInterval = 5 * time.Minute

// AdminSpaceExportCleanupResult 描述一次导出临时任务清理结果。
type AdminSpaceExportCleanupResult struct {
	DeletedJobs  int
	DeletedFiles int
	Errors       []error
}

// AdminSpaceImportCleanupResult 描述一次导入 staging 清理结果。
type AdminSpaceImportCleanupResult struct {
	DeletedStagings     int
	DeletedStagingFiles int
	DeletedJobs         int
	Errors              []error
}

// CleanupExpiredTransfers 清理已过期导出任务及其私有文件。
func (s *AdminSpaceExportService) CleanupExpiredTransfers(
	ctx context.Context,
	now time.Time,
) AdminSpaceExportCleanupResult {
	if s == nil || s.store == nil {
		return AdminSpaceExportCleanupResult{}
	}
	if now.IsZero() {
		now = s.now()
	}
	expiredJobs := s.store.DeleteExpired(now.UTC())
	result := AdminSpaceExportCleanupResult{DeletedJobs: len(expiredJobs)}
	if s.transferJobRepo != nil {
		persistedExpiredJobs, err := s.transferJobRepo.ListExpiredByKind(ctx, models.AdminSpaceTransferJobKindExport, now.UTC(), 100)
		if err != nil {
			result.Errors = append(result.Errors, err)
		} else {
			cleanupResult := s.cleanupExpiredPersistedExportJobs(ctx, persistedExpiredJobs)
			result.DeletedJobs += cleanupResult.DeletedJobs
			result.DeletedFiles += cleanupResult.DeletedFiles
			result.Errors = append(result.Errors, cleanupResult.Errors...)
		}
	}
	for _, job := range expiredJobs {
		if err := ctx.Err(); err != nil {
			result.Errors = append(result.Errors, err)
			return result
		}
		filePath := strings.TrimSpace(job.FilePath)
		if filePath == "" {
			continue
		}
		deleted, err := removeAdminSpaceTransferFile(filePath, s.exportDir)
		if err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		if deleted {
			result.DeletedFiles++
		}
	}
	return result
}

// CleanupExpiredTransfers 清理已过期导入 staging、staging zip 和导入任务。
func (s *AdminSpaceImportService) CleanupExpiredTransfers(
	ctx context.Context,
	now time.Time,
) AdminSpaceImportCleanupResult {
	if s == nil || s.store == nil {
		return AdminSpaceImportCleanupResult{}
	}
	if now.IsZero() {
		now = s.now()
	}
	deletedStagings, deletedJobs := s.store.DeleteExpired(now.UTC())
	result := AdminSpaceImportCleanupResult{
		DeletedStagings: len(deletedStagings),
		DeletedJobs:     len(deletedJobs),
	}
	if s.transferJobRepo != nil {
		persistedExpiredJobs, err := s.transferJobRepo.ListExpiredByKind(ctx, models.AdminSpaceTransferJobKindImport, now.UTC(), 100)
		if err != nil {
			result.Errors = append(result.Errors, err)
		} else {
			cleanupResult := s.cleanupExpiredPersistedImportJobs(ctx, persistedExpiredJobs)
			result.DeletedJobs += cleanupResult.DeletedJobs
			result.DeletedStagingFiles += cleanupResult.DeletedStagingFiles
			result.Errors = append(result.Errors, cleanupResult.Errors...)
		}
	}
	for _, staging := range deletedStagings {
		if err := ctx.Err(); err != nil {
			result.Errors = append(result.Errors, err)
			return result
		}
		filePath := strings.TrimSpace(staging.FilePath)
		if filePath == "" {
			continue
		}
		deleted, err := removeAdminSpaceTransferFile(filePath, s.stagingDir)
		if err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		if deleted {
			result.DeletedStagingFiles++
		}
	}
	return result
}

func (s *AdminSpaceExportService) cleanupExpiredPersistedExportJobs(
	ctx context.Context,
	jobs []models.AdminSpaceTransferJob,
) AdminSpaceExportCleanupResult {
	result := AdminSpaceExportCleanupResult{}
	if len(jobs) == 0 || s == nil || s.transferJobRepo == nil {
		return result
	}
	deletableIDs := make([]int64, 0, len(jobs))
	for _, job := range jobs {
		if err := ctx.Err(); err != nil {
			result.Errors = append(result.Errors, err)
			return result
		}
		if strings.TrimSpace(job.Kind) != models.AdminSpaceTransferJobKindExport {
			continue
		}
		deleted, err := removeAdminSpaceTransferFile(strings.TrimSpace(job.FilePath), s.exportDir)
		if err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		if deleted {
			result.DeletedFiles++
		}
		if job.ID > 0 {
			deletableIDs = append(deletableIDs, job.ID)
		}
	}
	deletedJobs, err := s.transferJobRepo.DeleteByIDs(ctx, deletableIDs)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}
	result.DeletedJobs += int(deletedJobs)
	return result
}

func (s *AdminSpaceImportService) cleanupExpiredPersistedImportJobs(
	ctx context.Context,
	jobs []models.AdminSpaceTransferJob,
) AdminSpaceImportCleanupResult {
	result := AdminSpaceImportCleanupResult{}
	if len(jobs) == 0 || s == nil || s.transferJobRepo == nil {
		return result
	}
	deletableIDs := make([]int64, 0, len(jobs))
	for _, job := range jobs {
		if err := ctx.Err(); err != nil {
			result.Errors = append(result.Errors, err)
			return result
		}
		if strings.TrimSpace(job.Kind) != models.AdminSpaceTransferJobKindImport {
			continue
		}
		deleted, err := removeAdminSpaceTransferFile(strings.TrimSpace(job.FilePath), s.stagingDir)
		if err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		if deleted {
			result.DeletedStagingFiles++
		}
		if job.ID > 0 {
			deletableIDs = append(deletableIDs, job.ID)
		}
	}
	deletedJobs, err := s.transferJobRepo.DeleteByIDs(ctx, deletableIDs)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}
	result.DeletedJobs += int(deletedJobs)
	return result
}

// StartAdminSpaceTransferCleanupLoop 启动空间导入/导出临时文件清理循环。
func StartAdminSpaceTransferCleanupLoop(
	ctx context.Context,
	logger *slog.Logger,
	exportService *AdminSpaceExportService,
	importService *AdminSpaceImportService,
) <-chan struct{} {
	return startAdminSpaceTransferCleanupLoop(
		ctx,
		logger,
		exportService,
		importService,
		defaultAdminSpaceTransferCleanupInterval,
	)
}

func startAdminSpaceTransferCleanupLoop(
	ctx context.Context,
	logger *slog.Logger,
	exportService *AdminSpaceExportService,
	importService *AdminSpaceImportService,
	interval time.Duration,
) <-chan struct{} {
	done := make(chan struct{})
	if ctx == nil {
		close(done)
		return done
	}
	if interval <= 0 {
		interval = defaultAdminSpaceTransferCleanupInterval
	}
	go func() {
		defer close(done)
		runAdminSpaceTransferCleanupOnce(ctx, logger, exportService, importService)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runAdminSpaceTransferCleanupOnce(ctx, logger, exportService, importService)
			}
		}
	}()
	return done
}

func runAdminSpaceTransferCleanupOnce(
	ctx context.Context,
	logger *slog.Logger,
	exportService *AdminSpaceExportService,
	importService *AdminSpaceImportService,
) {
	if err := ctx.Err(); err != nil {
		return
	}
	now := time.Now().UTC()
	exportResult := exportService.CleanupExpiredTransfers(ctx, now)
	importResult := importService.CleanupExpiredTransfers(ctx, now)
	logAdminSpaceTransferCleanupErrors(logger, "export", exportResult.Errors)
	logAdminSpaceTransferCleanupErrors(logger, "import", importResult.Errors)
	if logger != nil && (exportResult.DeletedJobs > 0 ||
		exportResult.DeletedFiles > 0 ||
		importResult.DeletedStagings > 0 ||
		importResult.DeletedStagingFiles > 0 ||
		importResult.DeletedJobs > 0) {
		logger.InfoContext(
			ctx,
			"admin space transfer cleanup completed",
			"export_deleted_jobs", exportResult.DeletedJobs,
			"export_deleted_files", exportResult.DeletedFiles,
			"import_deleted_stagings", importResult.DeletedStagings,
			"import_deleted_staging_files", importResult.DeletedStagingFiles,
			"import_deleted_jobs", importResult.DeletedJobs,
		)
	}
}

func logAdminSpaceTransferCleanupErrors(logger *slog.Logger, kind string, cleanupErrors []error) {
	if logger == nil {
		return
	}
	for _, cleanupErr := range cleanupErrors {
		if cleanupErr == nil || errors.Is(cleanupErr, context.Canceled) {
			continue
		}
		logger.Warn(
			"admin space transfer cleanup failed",
			"kind", kind,
			"error", cleanupErr.Error(),
		)
	}
}

func removeAdminSpaceTransferFile(filePath string, rootDir string) (bool, error) {
	normalizedFilePath := strings.TrimSpace(filePath)
	if normalizedFilePath == "" {
		return false, nil
	}
	if !isAdminSpaceTransferPathInsideRoot(normalizedFilePath, rootDir) {
		return false, nil
	}
	if err := os.Remove(normalizedFilePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func isAdminSpaceTransferPathInsideRoot(filePath string, rootDir string) bool {
	normalizedRootDir := strings.TrimSpace(rootDir)
	if normalizedRootDir == "" {
		return false
	}
	absFilePath, err := filepath.Abs(filepath.Clean(filePath))
	if err != nil {
		return false
	}
	absRootDir, err := filepath.Abs(filepath.Clean(normalizedRootDir))
	if err != nil {
		return false
	}
	return absFilePath == absRootDir || strings.HasPrefix(absFilePath, absRootDir+string(os.PathSeparator))
}
