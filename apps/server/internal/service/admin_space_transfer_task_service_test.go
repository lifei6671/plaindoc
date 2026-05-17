package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

func TestAdminSpaceTransferTaskService_ListMyTasksReturnsActiveActorTasks(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-admin-space-transfer-task-service-list?mode=memory&cache=shared",
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

	repo := repository.NewGormAdminSpaceTransferJobRepository(database.ORM)
	now := time.Now().UTC()
	for _, job := range []models.AdminSpaceTransferJob{
		{
			JobID:       "01actorarunning000000001",
			Kind:        models.AdminSpaceTransferJobKindExport,
			ActorUserID: "actor-a",
			Status:      models.AdminSpaceTransferJobStatusRunning,
			Progress:    30,
			UpdatedAt:   now,
			CreatedAt:   now,
			ExpiresAt:   now.Add(30 * time.Minute),
		},
		{
			JobID:       "01actoracompleted0000001",
			Kind:        models.AdminSpaceTransferJobKindExport,
			ActorUserID: "actor-a",
			Status:      models.AdminSpaceTransferJobStatusCompleted,
			Progress:    100,
			UpdatedAt:   now,
			CreatedAt:   now,
			ExpiresAt:   now.Add(30 * time.Minute),
		},
		{
			JobID:       "01actorbrunning000000001",
			Kind:        models.AdminSpaceTransferJobKindImport,
			ActorUserID: "actor-b",
			Status:      models.AdminSpaceTransferJobStatusRunning,
			Progress:    60,
			UpdatedAt:   now,
			CreatedAt:   now,
			ExpiresAt:   now.Add(30 * time.Minute),
		},
	} {
		job := job
		if err := repo.Create(ctx, &job); err != nil {
			t.Fatalf("create job failed: %v", err)
		}
	}

	svc := NewAdminSpaceTransferTaskService(repo)
	tasks, err := svc.ListMyTasks(ctx, ListAdminSpaceTransferTasksInput{
		ActorUserID: "actor-a",
		Status:      "active",
	})
	if err != nil {
		t.Fatalf("list my tasks failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 active task for actor-a, got %d", len(tasks))
	}
	if tasks[0].JobID != "01actorarunning000000001" {
		t.Fatalf("unexpected task returned: %#v", tasks[0])
	}
}

func TestAdminSpaceTransferTaskService_RecoverInterruptedActiveJobs(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-admin-space-transfer-task-service-recover?mode=memory&cache=shared",
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

	repo := repository.NewGormAdminSpaceTransferJobRepository(database.ORM)
	now := time.Now().UTC()
	activeJob := models.AdminSpaceTransferJob{
		JobID:       "01interruptedjob00000001",
		Kind:        models.AdminSpaceTransferJobKindImport,
		ActorUserID: "actor-a",
		Status:      models.AdminSpaceTransferJobStatusQueued,
		Stage:       "queued",
		Progress:    0,
		CreatedAt:   now.Add(-time.Minute),
		UpdatedAt:   now.Add(-time.Minute),
		ExpiresAt:   now.Add(30 * time.Minute),
	}
	if err := repo.Create(ctx, &activeJob); err != nil {
		t.Fatalf("create active job failed: %v", err)
	}

	svc := NewAdminSpaceTransferTaskService(repo)
	affected, err := svc.RecoverInterruptedActiveJobs(ctx, now)
	if err != nil {
		t.Fatalf("recover interrupted jobs failed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected affected=1, got %d", affected)
	}

	got, err := repo.GetByKindAndJobID(ctx, activeJob.Kind, activeJob.JobID)
	if err != nil {
		t.Fatalf("get recovered job failed: %v", err)
	}
	if got.Status != models.AdminSpaceTransferJobStatusFailed {
		t.Fatalf("expected recovered job failed, got %s", got.Status)
	}
}

func TestAdminSpaceTransferTaskService_GetMyTaskRejectsOtherActor(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-admin-space-transfer-task-service-get-forbidden?mode=memory&cache=shared",
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

	repo := repository.NewGormAdminSpaceTransferJobRepository(database.ORM)
	now := time.Now().UTC()
	job := models.AdminSpaceTransferJob{
		JobID:       "01actorownedtask0000001",
		Kind:        models.AdminSpaceTransferJobKindExport,
		ActorUserID: "actor-a",
		Status:      models.AdminSpaceTransferJobStatusRunning,
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   now.Add(30 * time.Minute),
	}
	if err := repo.Create(ctx, &job); err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	svc := NewAdminSpaceTransferTaskService(repo)
	_, err = svc.GetMyTask(ctx, GetAdminSpaceTransferTaskInput{
		ActorUserID: "actor-b",
		Kind:        job.Kind,
		JobID:       job.JobID,
	})
	if !errors.Is(err, errcode.ErrAdminForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestAdminSpaceTransferTaskService_IssueStreamURLDispatchesByKind(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-admin-space-transfer-task-service-stream-token?mode=memory&cache=shared",
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

	repo := repository.NewGormAdminSpaceTransferJobRepository(database.ORM)
	now := time.Now().UTC()
	for _, job := range []models.AdminSpaceTransferJob{
		{
			JobID:       "01exportstreamjob000001",
			Kind:        models.AdminSpaceTransferJobKindExport,
			ActorUserID: "actor-a",
			SpaceID:     "space-a",
			Status:      models.AdminSpaceTransferJobStatusRunning,
			CreatedAt:   now,
			UpdatedAt:   now,
			ExpiresAt:   now.Add(30 * time.Minute),
		},
		{
			JobID:       "01importstreamjob000001",
			Kind:        models.AdminSpaceTransferJobKindImport,
			ActorUserID: "actor-a",
			Status:      models.AdminSpaceTransferJobStatusQueued,
			CreatedAt:   now,
			UpdatedAt:   now,
			ExpiresAt:   now.Add(30 * time.Minute),
		},
	} {
		job := job
		if err := repo.Create(ctx, &job); err != nil {
			t.Fatalf("create job failed: %v", err)
		}
	}

	exportIssuer := &fakeAdminSpaceTransferStreamIssuer{url: "/api/admin/spaces/space-a/exports/01exportstreamjob000001/events?token=export"}
	importIssuer := &fakeAdminSpaceTransferStreamIssuer{url: "/api/admin/space-imports/01importstreamjob000001/events?token=import"}
	svc := NewAdminSpaceTransferTaskService(
		repo,
		WithAdminSpaceTransferExportStreamIssuer(exportIssuer),
		WithAdminSpaceTransferImportStreamIssuer(importIssuer),
	)

	exportResult, err := svc.IssueStreamURL(ctx, IssueAdminSpaceTransferStreamInput{
		ActorUserID: "actor-a",
		Kind:        models.AdminSpaceTransferJobKindExport,
		JobID:       "01exportstreamjob000001",
	})
	if err != nil {
		t.Fatalf("issue export stream url failed: %v", err)
	}
	if exportResult.StreamURL != exportIssuer.url {
		t.Fatalf("unexpected export stream url: %s", exportResult.StreamURL)
	}
	if exportIssuer.calls != 1 || importIssuer.calls != 0 {
		t.Fatalf("unexpected issuer calls: export=%d import=%d", exportIssuer.calls, importIssuer.calls)
	}

	importResult, err := svc.IssueStreamURL(ctx, IssueAdminSpaceTransferStreamInput{
		ActorUserID: "actor-a",
		Kind:        models.AdminSpaceTransferJobKindImport,
		JobID:       "01importstreamjob000001",
	})
	if err != nil {
		t.Fatalf("issue import stream url failed: %v", err)
	}
	if importResult.StreamURL != importIssuer.url {
		t.Fatalf("unexpected import stream url: %s", importResult.StreamURL)
	}
	if exportIssuer.calls != 1 || importIssuer.calls != 1 {
		t.Fatalf("unexpected issuer calls after import: export=%d import=%d", exportIssuer.calls, importIssuer.calls)
	}
}

func TestAdminSpaceTransferTaskService_IssueDownloadURLRejectsNonOwner(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-admin-space-transfer-task-service-download-forbidden?mode=memory&cache=shared",
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

	repo := repository.NewGormAdminSpaceTransferJobRepository(database.ORM)
	now := time.Now().UTC()
	job := models.AdminSpaceTransferJob{
		JobID:       "01exportdownloadjob0001",
		Kind:        models.AdminSpaceTransferJobKindExport,
		ActorUserID: "actor-a",
		Status:      models.AdminSpaceTransferJobStatusCompleted,
		FileName:    "space.plaindoc",
		SizeBytes:   1024,
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   now.Add(30 * time.Minute),
	}
	if err := repo.Create(ctx, &job); err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	downloadIssuer := &fakeAdminSpaceTransferDownloadIssuer{url: "/api/admin/space-exports/01exportdownloadjob0001/download?token=download"}
	svc := NewAdminSpaceTransferTaskService(
		repo,
		WithAdminSpaceTransferExportDownloadIssuer(downloadIssuer),
	)

	_, err = svc.IssueDownloadURL(ctx, IssueAdminSpaceTransferDownloadInput{
		ActorUserID: "actor-b",
		Kind:        models.AdminSpaceTransferJobKindExport,
		JobID:       job.JobID,
	})
	if !errors.Is(err, errcode.ErrAdminForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if downloadIssuer.calls != 0 {
		t.Fatalf("download issuer should not be called, got %d", downloadIssuer.calls)
	}
}

type fakeAdminSpaceTransferStreamIssuer struct {
	url   string
	calls int
}

func (f *fakeAdminSpaceTransferStreamIssuer) IssueStreamURL(_ context.Context, _ string, _ string) (string, error) {
	f.calls++
	return f.url, nil
}

type fakeAdminSpaceTransferDownloadIssuer struct {
	url   string
	calls int
}

func (f *fakeAdminSpaceTransferDownloadIssuer) IssueDownloadURL(_ context.Context, _ string, _ string) (string, error) {
	f.calls++
	return f.url, nil
}
