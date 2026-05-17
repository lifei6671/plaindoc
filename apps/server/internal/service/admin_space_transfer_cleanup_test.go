package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

func TestAdminSpaceExportService_CleanupExpiredTransfersDeletesExpiredJobAndFile(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	exportDir := t.TempDir()
	expiredFilePath := path.Join(exportDir, "expired.zip")
	activeFilePath := path.Join(exportDir, "active.zip")
	if err := os.WriteFile(expiredFilePath, []byte("expired"), 0o600); err != nil {
		t.Fatalf("write expired export file failed: %v", err)
	}
	if err := os.WriteFile(activeFilePath, []byte("active"), 0o600); err != nil {
		t.Fatalf("write active export file failed: %v", err)
	}

	svc := newAllowExportService()
	svc.exportDir = exportDir
	svc.store.jobs["expired-job"] = &AdminSpaceExportJob{
		JobID:                  "expired-job",
		Status:                 AdminSpaceExportStatusCompleted,
		FilePath:               expiredFilePath,
		DownloadTokenExpiresAt: now.Add(-time.Second),
		UpdatedAt:              now.Add(-time.Hour),
	}
	svc.store.jobs["active-job"] = &AdminSpaceExportJob{
		JobID:                  "active-job",
		Status:                 AdminSpaceExportStatusCompleted,
		FilePath:               activeFilePath,
		DownloadTokenExpiresAt: now.Add(time.Minute),
		UpdatedAt:              now,
	}

	result := svc.CleanupExpiredTransfers(context.Background(), now)
	if result.DeletedJobs != 1 || result.DeletedFiles != 1 || len(result.Errors) != 0 {
		t.Fatalf("unexpected cleanup result: %#v", result)
	}
	if _, err := svc.store.Get("expired-job"); err == nil {
		t.Fatalf("expected expired job to be removed")
	}
	if _, err := os.Stat(expiredFilePath); !os.IsNotExist(err) {
		t.Fatalf("expected expired file to be removed, stat err=%v", err)
	}
	if _, err := svc.store.Get("active-job"); err != nil {
		t.Fatalf("expected active job to remain: %v", err)
	}
	if _, err := os.Stat(activeFilePath); err != nil {
		t.Fatalf("expected active file to remain: %v", err)
	}
}

func TestAdminSpaceExportService_CleanupExpiredTransfersKeepsRunningJobPastStreamTokenTTL(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	exportDir := t.TempDir()
	runningFilePath := path.Join(exportDir, "running.zip")
	if err := os.WriteFile(runningFilePath, []byte("running"), 0o600); err != nil {
		t.Fatalf("write running export file failed: %v", err)
	}

	svc := newAllowExportService()
	svc.exportDir = exportDir
	svc.store.jobs["running-job"] = &AdminSpaceExportJob{
		JobID:                "running-job",
		Status:               AdminSpaceExportStatusRunning,
		FilePath:             runningFilePath,
		StreamTokenExpiresAt: now.Add(-time.Second),
		UpdatedAt:            now.Add(-time.Hour),
	}

	result := svc.CleanupExpiredTransfers(context.Background(), now)
	if result.DeletedJobs != 0 || result.DeletedFiles != 0 || len(result.Errors) != 0 {
		t.Fatalf("unexpected cleanup result: %#v", result)
	}
	if _, err := svc.store.Get("running-job"); err != nil {
		t.Fatalf("expected running job to remain: %v", err)
	}
	if _, err := os.Stat(runningFilePath); err != nil {
		t.Fatalf("expected running export file to remain: %v", err)
	}
}

func TestAdminSpaceExportService_CleanupExpiredTransfersKeepsPersistedJobWhenFileDeleteFails(t *testing.T) {
	t.Parallel()

	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-admin-space-export-cleanup-keeps-persisted-job-on-file-delete-failure?mode=memory&cache=shared",
	})
	if err != nil {
		t.Fatalf("open database failed: %v", err)
	}
	defer func() {
		_ = database.Close()
	}()
	ctx := context.Background()
	if err := storage.MigrateUp(ctx, database.ORM, storage.DriverSQLite); err != nil {
		t.Fatalf("migrate up failed: %v", err)
	}

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	exportDir := t.TempDir()
	nonEmptyDir := path.Join(exportDir, "stuck-export")
	if err := os.Mkdir(nonEmptyDir, 0o700); err != nil {
		t.Fatalf("create non-empty export dir failed: %v", err)
	}
	if err := os.WriteFile(path.Join(nonEmptyDir, "part"), []byte("stuck"), 0o600); err != nil {
		t.Fatalf("write nested export file failed: %v", err)
	}

	transferJobRepo := repository.NewGormAdminSpaceTransferJobRepository(database.ORM)
	job := models.AdminSpaceTransferJob{
		JobID:       "01cleanupkeepjob00000001",
		Kind:        models.AdminSpaceTransferJobKindExport,
		ActorUserID: "actor-user",
		Status:      models.AdminSpaceTransferJobStatusCompleted,
		Stage:       "done",
		Progress:    100,
		FilePath:    nonEmptyDir,
		CreatedAt:   now.Add(-time.Hour),
		UpdatedAt:   now.Add(-time.Hour),
		ExpiresAt:   now.Add(-time.Minute),
	}
	if err := transferJobRepo.Create(ctx, &job); err != nil {
		t.Fatalf("create persisted transfer job failed: %v", err)
	}

	svc := newAllowExportService()
	svc.exportDir = exportDir
	svc.transferJobRepo = transferJobRepo

	result := svc.CleanupExpiredTransfers(ctx, now)
	if result.DeletedJobs != 0 || result.DeletedFiles != 0 || len(result.Errors) != 1 {
		t.Fatalf("unexpected cleanup result: %#v", result)
	}
	if _, err := transferJobRepo.GetByKindAndJobID(ctx, models.AdminSpaceTransferJobKindExport, job.JobID); err != nil {
		t.Fatalf("expected persisted job to remain for retry, got %v", err)
	}
}

func TestAdminSpaceImportService_CleanupExpiredTransfersDeletesExpiredStagingFile(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	stagingDir := t.TempDir()
	expiredFilePath := path.Join(stagingDir, "expired.zip")
	activeFilePath := path.Join(stagingDir, "active.zip")
	if err := os.WriteFile(expiredFilePath, []byte("expired"), 0o600); err != nil {
		t.Fatalf("write expired staging file failed: %v", err)
	}
	if err := os.WriteFile(activeFilePath, []byte("active"), 0o600); err != nil {
		t.Fatalf("write active staging file failed: %v", err)
	}

	svc := NewAdminSpaceImportService(nil)
	svc.stagingDir = stagingDir
	svc.store.SaveStaging(AdminSpaceImportStaging{
		ImportID:    "expired-import",
		ActorUserID: "actor-user",
		FilePath:    expiredFilePath,
		ExpiresAt:   now.Add(-time.Second),
	})
	svc.store.SaveStaging(AdminSpaceImportStaging{
		ImportID:    "active-import",
		ActorUserID: "actor-user",
		FilePath:    activeFilePath,
		ExpiresAt:   now.Add(time.Minute),
	})

	result := svc.CleanupExpiredTransfers(context.Background(), now)
	if result.DeletedStagings != 1 || result.DeletedStagingFiles != 1 || len(result.Errors) != 0 {
		t.Fatalf("unexpected cleanup result: %#v", result)
	}
	if _, err := svc.store.GetStaging("expired-import", "actor-user", now); !errorsIsStagingNotFound(err) {
		t.Fatalf("expected expired staging to be removed, got %v", err)
	}
	if _, err := os.Stat(expiredFilePath); !os.IsNotExist(err) {
		t.Fatalf("expected expired staging file to be removed, stat err=%v", err)
	}
	if _, err := svc.store.GetStaging("active-import", "actor-user", now); err != nil {
		t.Fatalf("expected active staging to remain: %v", err)
	}
	if _, err := os.Stat(activeFilePath); err != nil {
		t.Fatalf("expected active staging file to remain: %v", err)
	}
}

func TestAdminSpaceImportService_CleanupExpiredTransfersKeepsRunningJobPastStreamTokenTTL(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	svc := NewAdminSpaceImportService(nil)
	svc.store.jobs["running-job"] = &AdminSpaceImportJob{
		JobID:                "running-job",
		ActorUserID:          "actor-user",
		Status:               AdminSpaceImportStatusRunning,
		StreamTokenExpiresAt: now.Add(-time.Second),
		UpdatedAt:            now.Add(-time.Hour),
	}

	result := svc.CleanupExpiredTransfers(context.Background(), now)
	if result.DeletedJobs != 0 || len(result.Errors) != 0 {
		t.Fatalf("unexpected cleanup result: %#v", result)
	}
	if _, err := svc.store.GetJob("running-job"); err != nil {
		t.Fatalf("expected running job to remain: %v", err)
	}
}

func TestAdminSpaceTransferCleanupLoopExitsOnContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
	done := startAdminSpaceTransferCleanupLoop(
		ctx,
		logger,
		newAllowExportService(),
		NewAdminSpaceImportService(nil),
		time.Hour,
	)

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("cleanup loop did not exit after context cancellation")
	}
}

func errorsIsStagingNotFound(err error) bool {
	return errors.Is(err, errcode.ErrAdminSpaceImportStagingNotFound)
}
