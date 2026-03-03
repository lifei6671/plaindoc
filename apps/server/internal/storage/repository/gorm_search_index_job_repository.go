package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

type gormSearchIndexJobRepository struct {
	db *gorm.DB
}

type searchIndexJobRow struct {
	ID           int64   `gorm:"column:id"`
	JobID        string  `gorm:"column:job_id"`
	Provider     string  `gorm:"column:provider"`
	JobType      string  `gorm:"column:job_type"`
	DedupeKey    string  `gorm:"column:dedupe_key"`
	PayloadJSON  string  `gorm:"column:payload_json"`
	Status       string  `gorm:"column:status"`
	Priority     int     `gorm:"column:priority"`
	RetryCount   int     `gorm:"column:retry_count"`
	NextRunAtRaw string  `gorm:"column:next_run_at"`
	StartedAtRaw *string `gorm:"column:started_at"`
	LastError    string  `gorm:"column:last_error"`
	CreatedAtRaw string  `gorm:"column:created_at"`
	UpdatedAtRaw string  `gorm:"column:updated_at"`
}

// NewGormSearchIndexJobRepository 创建检索索引任务仓储实现。
func NewGormSearchIndexJobRepository(db *gorm.DB) SearchIndexJobRepository {
	return &gormSearchIndexJobRepository{db: db}
}

func (r *gormSearchIndexJobRepository) Enqueue(
	ctx context.Context,
	params EnqueueSearchIndexJobParams,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("search index job repository db is nil")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return r.enqueueInTx(ctx, tx, params)
	})
}

func (r *gormSearchIndexJobRepository) EnqueueInTx(
	ctx context.Context,
	tx *gorm.DB,
	params EnqueueSearchIndexJobParams,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("search index job repository db is nil")
	}
	if tx == nil {
		return fmt.Errorf("search index job transaction is nil")
	}
	return r.enqueueInTx(ctx, tx.WithContext(ctx), params)
}

func (r *gormSearchIndexJobRepository) ClaimRunnableJobs(
	ctx context.Context,
	params ClaimSearchIndexJobsParams,
) ([]models.SearchIndexJob, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("search index job repository db is nil")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 32
	}
	now := params.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	claimableStatuses := []string{
		models.SearchIndexJobStatusPending,
		models.SearchIndexJobStatusFailed,
	}

	claimed := make([]models.SearchIndexJob, 0, limit)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		type idRow struct {
			ID int64 `gorm:"column:id"`
		}
		idRows := make([]idRow, 0, limit)
		if err := tx.Table("search_index_jobs").
			Select("id").
			Where("status IN ? AND next_run_at <= ?", claimableStatuses, now).
			Order("priority ASC, next_run_at ASC, id ASC").
			Limit(limit).
			Scan(&idRows).Error; err != nil {
			return err
		}
		if len(idRows) == 0 {
			return nil
		}

		candidateIDs := make([]int64, 0, len(idRows))
		for _, row := range idRows {
			if row.ID <= 0 {
				continue
			}
			candidateIDs = append(candidateIDs, row.ID)
		}
		if len(candidateIDs) == 0 {
			return nil
		}

		if err := tx.Table("search_index_jobs").
			Where("id IN ? AND status IN ?", candidateIDs, claimableStatuses).
			Updates(map[string]any{
				"status":     models.SearchIndexJobStatusRunning,
				"started_at": now,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}

		rows := make([]searchIndexJobRow, 0, len(candidateIDs))
		if err := tx.Table("search_index_jobs").
			Select(
				"id",
				"job_id",
				"provider",
				"job_type",
				"dedupe_key",
				"payload_json",
				"status",
				"priority",
				"retry_count",
				"next_run_at",
				"started_at",
				"last_error",
				"created_at",
				"updated_at",
			).
			Where("id IN ? AND status = ?", candidateIDs, models.SearchIndexJobStatusRunning).
			Order("priority ASC, next_run_at ASC, id ASC").
			Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			claimed = append(claimed, mapSearchIndexJobRow(row))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

func (r *gormSearchIndexJobRepository) MarkSuccess(
	ctx context.Context,
	jobID string,
	finishedAt time.Time,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("search index job repository db is nil")
	}

	normalizedJobID := strings.TrimSpace(jobID)
	if normalizedJobID == "" {
		return gorm.ErrRecordNotFound
	}

	if finishedAt.IsZero() {
		finishedAt = time.Now().UTC()
	}

	updateTx := r.db.WithContext(ctx).
		Table("search_index_jobs").
		Where("job_id = ? AND status = ?", normalizedJobID, models.SearchIndexJobStatusRunning).
		Updates(map[string]any{
			"status":     models.SearchIndexJobStatusSuccess,
			"started_at": nil,
			"last_error": "",
			"updated_at": finishedAt,
		})
	if updateTx.Error != nil {
		return updateTx.Error
	}
	if updateTx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *gormSearchIndexJobRepository) MarkRetry(
	ctx context.Context,
	params MarkSearchIndexJobRetryParams,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("search index job repository db is nil")
	}

	normalizedJobID := strings.TrimSpace(params.JobID)
	if normalizedJobID == "" {
		return gorm.ErrRecordNotFound
	}
	nextRunAt := params.NextRunAt
	if nextRunAt.IsZero() {
		nextRunAt = time.Now().UTC()
	}

	updateTx := r.db.WithContext(ctx).
		Table("search_index_jobs").
		Where("job_id = ? AND status = ?", normalizedJobID, models.SearchIndexJobStatusRunning).
		Updates(map[string]any{
			"status":      models.SearchIndexJobStatusFailed,
			"retry_count": gorm.Expr("retry_count + 1"),
			"next_run_at": nextRunAt,
			"started_at":  nil,
			"last_error":  strings.TrimSpace(params.LastError),
			"updated_at":  time.Now().UTC(),
		})
	if updateTx.Error != nil {
		return updateTx.Error
	}
	if updateTx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *gormSearchIndexJobRepository) enqueueInTx(
	ctx context.Context,
	tx *gorm.DB,
	params EnqueueSearchIndexJobParams,
) error {
	normalized, err := normalizeSearchIndexJobEnqueueParams(params)
	if err != nil {
		return err
	}
	now := time.Now().UTC()

	var existing searchIndexJobRow
	existingErr := tx.Table("search_index_jobs").
		Select(
			"id",
			"job_id",
			"provider",
			"job_type",
			"dedupe_key",
			"payload_json",
			"status",
			"priority",
			"retry_count",
			"next_run_at",
			"started_at",
			"last_error",
			"created_at",
			"updated_at",
		).
		Where(
			"dedupe_key = ? AND status IN ?",
			normalized.DedupeKey,
			[]string{models.SearchIndexJobStatusPending, models.SearchIndexJobStatusFailed},
		).
		Order("id DESC").
		Take(&existing).Error
	switch {
	case existingErr == nil:
		merged, keepExisting := mergeSearchIndexJob(existing, normalized)
		if keepExisting {
			return nil
		}
		updates := map[string]any{
			"provider":     merged.Provider,
			"job_type":     merged.JobType,
			"payload_json": merged.PayloadJSON,
			"status":       models.SearchIndexJobStatusPending,
			"priority":     merged.Priority,
			"next_run_at":  merged.NextRunAt,
			"started_at":   nil,
			"last_error":   "",
			"updated_at":   now,
		}
		return tx.Table("search_index_jobs").
			Where("id = ?", existing.ID).
			Updates(updates).Error
	case errors.Is(existingErr, gorm.ErrRecordNotFound):
		if normalized.NextRunAt.IsZero() {
			normalized.NextRunAt = now
		}
		return tx.Table("search_index_jobs").Create(map[string]any{
			"job_id":       strings.ToLower(ulid.Make().String()),
			"provider":     normalized.Provider,
			"job_type":     normalized.JobType,
			"dedupe_key":   normalized.DedupeKey,
			"payload_json": normalized.PayloadJSON,
			"status":       models.SearchIndexJobStatusPending,
			"priority":     normalized.Priority,
			"retry_count":  0,
			"next_run_at":  normalized.NextRunAt,
			"started_at":   nil,
			"last_error":   "",
			"created_at":   now,
			"updated_at":   now,
		}).Error
	default:
		return existingErr
	}
}

func normalizeSearchIndexJobEnqueueParams(
	input EnqueueSearchIndexJobParams,
) (EnqueueSearchIndexJobParams, error) {
	output := EnqueueSearchIndexJobParams{
		Provider:    strings.ToLower(strings.TrimSpace(input.Provider)),
		JobType:     strings.TrimSpace(strings.ToUpper(input.JobType)),
		DedupeKey:   strings.TrimSpace(input.DedupeKey),
		PayloadJSON: strings.TrimSpace(input.PayloadJSON),
		Priority:    input.Priority,
		NextRunAt:   input.NextRunAt.UTC(),
	}
	if !models.IsValidSearchIndexJobType(output.JobType) {
		return EnqueueSearchIndexJobParams{}, fmt.Errorf("invalid search index job type: %s", output.JobType)
	}
	if output.DedupeKey == "" {
		return EnqueueSearchIndexJobParams{}, fmt.Errorf("search index job dedupe key is empty")
	}
	if output.PayloadJSON == "" {
		return EnqueueSearchIndexJobParams{}, fmt.Errorf("search index job payload json is empty")
	}
	if output.Priority <= 0 {
		output.Priority = models.SearchIndexJobPriorityNormal
	}
	if output.NextRunAt.IsZero() {
		output.NextRunAt = time.Now().UTC()
	}
	return output, nil
}

func mergeSearchIndexJob(
	existing searchIndexJobRow,
	incoming EnqueueSearchIndexJobParams,
) (EnqueueSearchIndexJobParams, bool) {
	existingType := strings.TrimSpace(strings.ToUpper(existing.JobType))
	switch incoming.JobType {
	case models.SearchIndexJobTypeDocDelete, models.SearchIndexJobTypeSpacePurge:
		// 删除/清空任务优先级更高，直接覆盖旧任务类型与载荷。
		return EnqueueSearchIndexJobParams{
			Provider:    incoming.Provider,
			JobType:     incoming.JobType,
			DedupeKey:   incoming.DedupeKey,
			PayloadJSON: incoming.PayloadJSON,
			Priority:    minSearchIndexJobPriority(existing.Priority, incoming.Priority),
			NextRunAt:   incoming.NextRunAt,
		}, false
	case models.SearchIndexJobTypeDocUpsert:
		// 旧 delete 任务保留，避免“删后回写”。
		if existingType == models.SearchIndexJobTypeDocDelete {
			return EnqueueSearchIndexJobParams{}, true
		}
	case models.SearchIndexJobTypeRebuildSpace:
		// purge 已在队列中时，rebuild 被吞并。
		if existingType == models.SearchIndexJobTypeSpacePurge {
			return EnqueueSearchIndexJobParams{}, true
		}
	}

	return EnqueueSearchIndexJobParams{
		Provider:    incoming.Provider,
		JobType:     incoming.JobType,
		DedupeKey:   incoming.DedupeKey,
		PayloadJSON: incoming.PayloadJSON,
		Priority:    minSearchIndexJobPriority(existing.Priority, incoming.Priority),
		NextRunAt:   incoming.NextRunAt,
	}, false
}

func minSearchIndexJobPriority(left int, right int) int {
	if left <= 0 {
		return right
	}
	if right <= 0 {
		return left
	}
	if left < right {
		return left
	}
	return right
}

func mapSearchIndexJobRow(row searchIndexJobRow) models.SearchIndexJob {
	return models.SearchIndexJob{
		ID:          row.ID,
		JobID:       strings.TrimSpace(row.JobID),
		Provider:    strings.TrimSpace(row.Provider),
		JobType:     strings.TrimSpace(strings.ToUpper(row.JobType)),
		DedupeKey:   strings.TrimSpace(row.DedupeKey),
		PayloadJSON: row.PayloadJSON,
		Status:      strings.TrimSpace(strings.ToLower(row.Status)),
		Priority:    row.Priority,
		RetryCount:  row.RetryCount,
		NextRunAt:   parseRecordTime(row.NextRunAtRaw),
		StartedAt:   parseOptionalRecordTime(row.StartedAtRaw),
		LastError:   strings.TrimSpace(row.LastError),
		CreatedAt:   parseRecordTime(row.CreatedAtRaw),
		UpdatedAt:   parseRecordTime(row.UpdatedAtRaw),
	}
}

func parseOptionalRecordTime(raw *string) *time.Time {
	if raw == nil {
		return nil
	}
	parsedAt := parseRecordTime(*raw)
	if parsedAt.IsZero() {
		return nil
	}
	value := parsedAt.UTC()
	return &value
}
