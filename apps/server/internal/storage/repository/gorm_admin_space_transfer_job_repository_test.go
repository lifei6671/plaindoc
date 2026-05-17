package repository

import (
	"context"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

func TestGormAdminSpaceTransferJobRepository_ListByActorActive(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-admin-space-transfer-job-list?mode=memory&cache=shared",
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

	repo := NewGormAdminSpaceTransferJobRepository(database.ORM)
	now := time.Now().UTC().Truncate(time.Millisecond)
	jobs := []models.AdminSpaceTransferJob{
		{
			JobID:       "01activeexportjob000000001",
			Kind:        models.AdminSpaceTransferJobKindExport,
			ActorUserID: "actor-a",
			SpaceID:     "space-a",
			SpaceName:   "空间 A",
			Format:      "source_zip",
			Status:      models.AdminSpaceTransferJobStatusRunning,
			Stage:       "documents",
			Progress:    45,
			Message:     "正在导出文档",
			CreatedAt:   now.Add(-2 * time.Minute),
			UpdatedAt:   now.Add(-1 * time.Minute),
			ExpiresAt:   now.Add(30 * time.Minute),
		},
		{
			JobID:       "01activeimportjob000000001",
			Kind:        models.AdminSpaceTransferJobKindImport,
			ActorUserID: "actor-a",
			ImportID:    "01importstaging0000000001",
			Status:      models.AdminSpaceTransferJobStatusQueued,
			Stage:       "queued",
			Progress:    0,
			Message:     "导入任务已创建",
			CreatedAt:   now.Add(-3 * time.Minute),
			UpdatedAt:   now,
			ExpiresAt:   now.Add(30 * time.Minute),
		},
		{
			JobID:       "01otheractorjob0000000001",
			Kind:        models.AdminSpaceTransferJobKindExport,
			ActorUserID: "actor-b",
			SpaceID:     "space-b",
			Status:      models.AdminSpaceTransferJobStatusRunning,
			Stage:       "zip",
			Progress:    10,
			CreatedAt:   now,
			UpdatedAt:   now,
			ExpiresAt:   now.Add(30 * time.Minute),
		},
	}
	for index := range jobs {
		if err := repo.Create(ctx, &jobs[index]); err != nil {
			t.Fatalf("create job %d failed: %v", index, err)
		}
	}

	items, err := repo.ListByActor(ctx, ListAdminSpaceTransferJobsParams{
		ActorUserID: "actor-a",
		Statuses: []string{
			models.AdminSpaceTransferJobStatusQueued,
			models.AdminSpaceTransferJobStatusRunning,
		},
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("list active jobs failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 active jobs for actor-a, got %d", len(items))
	}
	if items[0].JobID != "01activeimportjob000000001" {
		t.Fatalf("expected newest updated job first, got %s", items[0].JobID)
	}
	for _, item := range items {
		if item.ActorUserID != "actor-a" {
			t.Fatalf("expected only actor-a jobs, got actor %s", item.ActorUserID)
		}
	}
}

func TestGormAdminSpaceTransferJobRepository_UpdateProgressSetsStartedAt(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-admin-space-transfer-job-started-at?mode=memory&cache=shared",
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

	repo := NewGormAdminSpaceTransferJobRepository(database.ORM)
	createdAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Millisecond)
	job := models.AdminSpaceTransferJob{
		JobID:       "01startedatexportjob000001",
		Kind:        models.AdminSpaceTransferJobKindExport,
		ActorUserID: "actor-a",
		SpaceID:     "space-a",
		Status:      models.AdminSpaceTransferJobStatusQueued,
		Stage:       "queued",
		Progress:    0,
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
		ExpiresAt:   createdAt.Add(30 * time.Minute),
	}
	if err := repo.Create(ctx, &job); err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	startedAt := createdAt.Add(5 * time.Second)
	if err := repo.UpdateProgress(ctx, UpdateAdminSpaceTransferJobProgressParams{
		JobID:    job.JobID,
		Stage:    "documents",
		Progress: 25,
		Message:  "正在导出文档",
		Now:      startedAt,
	}); err != nil {
		t.Fatalf("update progress failed: %v", err)
	}

	got, err := repo.GetByKindAndJobID(ctx, models.AdminSpaceTransferJobKindExport, job.JobID)
	if err != nil {
		t.Fatalf("get updated job failed: %v", err)
	}
	if got.StartedAt == nil {
		t.Fatal("expected started_at to be set on first progress update")
	}
	if !got.StartedAt.Equal(startedAt) {
		t.Fatalf("expected started_at %s, got %s", startedAt, got.StartedAt)
	}

	laterAt := startedAt.Add(10 * time.Second)
	if err := repo.UpdateProgress(ctx, UpdateAdminSpaceTransferJobProgressParams{
		JobID:    job.JobID,
		Stage:    "zip",
		Progress: 60,
		Message:  "正在打包",
		Now:      laterAt,
	}); err != nil {
		t.Fatalf("update progress again failed: %v", err)
	}
	got, err = repo.GetByKindAndJobID(ctx, models.AdminSpaceTransferJobKindExport, job.JobID)
	if err != nil {
		t.Fatalf("get updated job again failed: %v", err)
	}
	if got.StartedAt == nil {
		t.Fatal("expected started_at to remain set")
	}
	if !got.StartedAt.Equal(startedAt) {
		t.Fatalf("expected started_at to remain %s, got %s", startedAt, got.StartedAt)
	}
	if !got.UpdatedAt.Equal(laterAt) {
		t.Fatalf("expected updated_at %s, got %s", laterAt, got.UpdatedAt)
	}
}

func TestGormAdminSpaceTransferJobRepository_MarkActiveJobsFailed(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-admin-space-transfer-job-mark-active-failed?mode=memory&cache=shared",
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

	repo := NewGormAdminSpaceTransferJobRepository(database.ORM)
	now := time.Now().UTC().Truncate(time.Millisecond)
	activeJob := models.AdminSpaceTransferJob{
		JobID:       "01runningjob000000000001",
		Kind:        models.AdminSpaceTransferJobKindExport,
		ActorUserID: "actor-a",
		Status:      models.AdminSpaceTransferJobStatusRunning,
		Stage:       "documents",
		Progress:    50,
		CreatedAt:   now.Add(-time.Minute),
		UpdatedAt:   now.Add(-time.Minute),
		ExpiresAt:   now.Add(30 * time.Minute),
	}
	completedAt := now.Add(-30 * time.Second)
	completedJob := models.AdminSpaceTransferJob{
		JobID:       "01completedjob0000000001",
		Kind:        models.AdminSpaceTransferJobKindExport,
		ActorUserID: "actor-a",
		Status:      models.AdminSpaceTransferJobStatusCompleted,
		Stage:       "done",
		Progress:    100,
		CreatedAt:   now.Add(-2 * time.Minute),
		UpdatedAt:   completedAt,
		CompletedAt: &completedAt,
		ExpiresAt:   now.Add(30 * time.Minute),
	}
	if err := repo.Create(ctx, &activeJob); err != nil {
		t.Fatalf("create active job failed: %v", err)
	}
	if err := repo.Create(ctx, &completedJob); err != nil {
		t.Fatalf("create completed job failed: %v", err)
	}

	affected, err := repo.MarkActiveJobsFailed(ctx, MarkActiveAdminSpaceTransferJobsFailedParams{
		Now:          now,
		Message:      "服务重启，任务已中断，请重新发起",
		ExpiresAfter: 10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("mark active failed failed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected affected=1, got %d", affected)
	}

	gotActive, err := repo.GetByKindAndJobID(ctx, models.AdminSpaceTransferJobKindExport, activeJob.JobID)
	if err != nil {
		t.Fatalf("get active job failed: %v", err)
	}
	if gotActive.Status != models.AdminSpaceTransferJobStatusFailed {
		t.Fatalf("expected active job failed, got %s", gotActive.Status)
	}
	if gotActive.Message != "服务重启，任务已中断，请重新发起" {
		t.Fatalf("unexpected failed message: %s", gotActive.Message)
	}

	gotCompleted, err := repo.GetByKindAndJobID(ctx, models.AdminSpaceTransferJobKindExport, completedJob.JobID)
	if err != nil {
		t.Fatalf("get completed job failed: %v", err)
	}
	if gotCompleted.Status != models.AdminSpaceTransferJobStatusCompleted {
		t.Fatalf("expected completed job unchanged, got %s", gotCompleted.Status)
	}
}

func TestGormAdminSpaceTransferJobRepository_ListExpiredTerminalJobsThenDeleteByIDs(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-admin-space-transfer-job-delete-expired?mode=memory&cache=shared",
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

	repo := NewGormAdminSpaceTransferJobRepository(database.ORM)
	now := time.Now().UTC().Truncate(time.Millisecond)
	expired := models.AdminSpaceTransferJob{
		JobID:       "01expiredjob000000000001",
		Kind:        models.AdminSpaceTransferJobKindExport,
		ActorUserID: "actor-a",
		Status:      models.AdminSpaceTransferJobStatusCompleted,
		Stage:       "done",
		Progress:    100,
		FilePath:    "/tmp/expired.plaindoc",
		FileName:    "expired.plaindoc",
		CreatedAt:   now.Add(-time.Hour),
		UpdatedAt:   now.Add(-time.Hour),
		ExpiresAt:   now.Add(-time.Minute),
	}
	active := models.AdminSpaceTransferJob{
		JobID:       "01activejob000000000001",
		Kind:        models.AdminSpaceTransferJobKindImport,
		ActorUserID: "actor-a",
		Status:      models.AdminSpaceTransferJobStatusRunning,
		Stage:       "documents",
		Progress:    50,
		CreatedAt:   now.Add(-time.Minute),
		UpdatedAt:   now.Add(-time.Minute),
		ExpiresAt:   now.Add(-time.Minute),
	}
	if err := repo.Create(ctx, &expired); err != nil {
		t.Fatalf("create expired job failed: %v", err)
	}
	if err := repo.Create(ctx, &active); err != nil {
		t.Fatalf("create active job failed: %v", err)
	}

	expiredJobs, err := repo.ListExpired(ctx, now, 10)
	if err != nil {
		t.Fatalf("list expired failed: %v", err)
	}
	if len(expiredJobs) != 1 || expiredJobs[0].JobID != expired.JobID {
		t.Fatalf("expected only expired terminal job listed, got %#v", expiredJobs)
	}
	if _, err := repo.GetByKindAndJobID(ctx, expired.Kind, expired.JobID); err != nil {
		t.Fatalf("expected listed expired job to remain before explicit delete, got %v", err)
	}
	deletedCount, err := repo.DeleteByIDs(ctx, []int64{expiredJobs[0].ID})
	if err != nil {
		t.Fatalf("delete by ids failed: %v", err)
	}
	if deletedCount != 1 {
		t.Fatalf("expected deleted count 1, got %d", deletedCount)
	}
	if _, err := repo.GetByKindAndJobID(ctx, expired.Kind, expired.JobID); err == nil {
		t.Fatalf("expected expired job deleted")
	}
	if _, err := repo.GetByKindAndJobID(ctx, active.Kind, active.JobID); err != nil {
		t.Fatalf("expected active job kept, got %v", err)
	}
}

func TestGormAdminSpaceTransferJobRepository_ListExpiredByKind(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-admin-space-transfer-job-delete-expired-by-kind?mode=memory&cache=shared",
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

	repo := NewGormAdminSpaceTransferJobRepository(database.ORM)
	now := time.Now().UTC().Truncate(time.Millisecond)
	for _, job := range []models.AdminSpaceTransferJob{
		{
			JobID:       "01expiredexport00000001",
			Kind:        models.AdminSpaceTransferJobKindExport,
			ActorUserID: "actor-a",
			Status:      models.AdminSpaceTransferJobStatusCompleted,
			Progress:    100,
			CreatedAt:   now.Add(-time.Hour),
			UpdatedAt:   now.Add(-time.Hour),
			ExpiresAt:   now.Add(-time.Minute),
		},
		{
			JobID:       "01expiredimport00000001",
			Kind:        models.AdminSpaceTransferJobKindImport,
			ActorUserID: "actor-a",
			Status:      models.AdminSpaceTransferJobStatusFailed,
			Progress:    20,
			CreatedAt:   now.Add(-time.Hour),
			UpdatedAt:   now.Add(-time.Hour),
			ExpiresAt:   now.Add(-time.Minute),
		},
	} {
		job := job
		if err := repo.Create(ctx, &job); err != nil {
			t.Fatalf("create job failed: %v", err)
		}
	}

	expiredJobs, err := repo.ListExpiredByKind(ctx, models.AdminSpaceTransferJobKindExport, now, 10)
	if err != nil {
		t.Fatalf("list expired by kind failed: %v", err)
	}
	if len(expiredJobs) != 1 || expiredJobs[0].Kind != models.AdminSpaceTransferJobKindExport {
		t.Fatalf("expected only expired export listed, got %#v", expiredJobs)
	}
	if _, err := repo.GetByKindAndJobID(ctx, models.AdminSpaceTransferJobKindExport, "01expiredexport00000001"); err != nil {
		t.Fatalf("expected listed export job kept before explicit delete, got %v", err)
	}
	if _, err := repo.GetByKindAndJobID(ctx, models.AdminSpaceTransferJobKindImport, "01expiredimport00000001"); err != nil {
		t.Fatalf("expected import job kept, got %v", err)
	}
}
