package repository

import (
	"context"
	"testing"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
)

func TestGormSearchIndexJobRepository_Enqueue_DeleteDominatesUpsert(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-search-index-job-repo-delete-dominate?mode=memory&cache=shared",
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

	repo := NewGormSearchIndexJobRepository(database.ORM)

	upsertParams, err := BuildSearchIndexDocUpsertJob("doc-1")
	if err != nil {
		t.Fatalf("build upsert job params failed: %v", err)
	}
	if err := repo.Enqueue(ctx, upsertParams); err != nil {
		t.Fatalf("enqueue upsert job failed: %v", err)
	}

	deleteParams, err := BuildSearchIndexDocDeleteJob("doc-1")
	if err != nil {
		t.Fatalf("build delete job params failed: %v", err)
	}
	if err := repo.Enqueue(ctx, deleteParams); err != nil {
		t.Fatalf("enqueue delete job failed: %v", err)
	}

	type row struct {
		Count   int64  `gorm:"column:count"`
		JobType string `gorm:"column:job_type"`
		Status  string `gorm:"column:status"`
	}
	var current row
	if err := database.ORM.WithContext(ctx).
		Table("search_index_jobs").
		Select("COUNT(*) AS count", "MAX(job_type) AS job_type", "MAX(status) AS status").
		Scan(&current).Error; err != nil {
		t.Fatalf("query search index jobs failed: %v", err)
	}

	if current.Count != 1 {
		t.Fatalf("expected 1 merged job, got %d", current.Count)
	}
	if current.JobType != models.SearchIndexJobTypeDocDelete {
		t.Fatalf("expected merged job type %q, got %q", models.SearchIndexJobTypeDocDelete, current.JobType)
	}
	if current.Status != models.SearchIndexJobStatusPending {
		t.Fatalf("expected merged job status pending, got %q", current.Status)
	}
}

func TestGormSearchIndexJobRepository_ClaimAndRetry(t *testing.T) {
	database, err := storage.OpenDatabase(storage.OpenConfig{
		Driver: storage.DriverSQLite,
		DSN:    "file:test-search-index-job-repo-claim-retry?mode=memory&cache=shared",
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

	repo := NewGormSearchIndexJobRepository(database.ORM)
	upsertParams, err := BuildSearchIndexDocUpsertJob("doc-2")
	if err != nil {
		t.Fatalf("build upsert job params failed: %v", err)
	}
	if err := repo.Enqueue(ctx, upsertParams); err != nil {
		t.Fatalf("enqueue upsert job failed: %v", err)
	}

	firstClaim, err := repo.ClaimRunnableJobs(ctx, ClaimSearchIndexJobsParams{
		Limit: 10,
		Now:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("first claim failed: %v", err)
	}
	if len(firstClaim) != 1 {
		t.Fatalf("expected first claim size=1, got %d", len(firstClaim))
	}

	retryAt := time.Now().UTC().Add(3 * time.Second)
	if err := repo.MarkRetry(ctx, MarkSearchIndexJobRetryParams{
		JobID:     firstClaim[0].JobID,
		NextRunAt: retryAt,
		LastError: "mock failed",
	}); err != nil {
		t.Fatalf("mark retry failed: %v", err)
	}

	secondClaim, err := repo.ClaimRunnableJobs(ctx, ClaimSearchIndexJobsParams{
		Limit: 10,
		Now:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("second claim failed: %v", err)
	}
	if len(secondClaim) != 0 {
		t.Fatalf("expected second claim size=0 before retry time, got %d", len(secondClaim))
	}

	thirdClaim, err := repo.ClaimRunnableJobs(ctx, ClaimSearchIndexJobsParams{
		Limit: 10,
		Now:   retryAt.Add(1 * time.Second),
	})
	if err != nil {
		t.Fatalf("third claim failed: %v", err)
	}
	if len(thirdClaim) != 1 {
		t.Fatalf("expected third claim size=1 after retry time, got %d", len(thirdClaim))
	}
	if thirdClaim[0].RetryCount != 1 {
		t.Fatalf("expected retry_count=1 after retry, got %d", thirdClaim[0].RetryCount)
	}
}
