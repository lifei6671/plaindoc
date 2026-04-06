package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/recordtime"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

type gormSearchIndexJobRepository struct {
	db *gorm.DB
}

type searchIndexJobRow = searchIndexJobRowDB

type searchIndexJobIDRow = searchIndexJobIDRowDB

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
		idRows := make([]searchIndexJobIDRow, 0, limit)
		if err := tx.Model(&models.SearchIndexJob{}).
			Select(models.SearchIndexJobColumns.ID).
			Where(models.SearchIndexJobColumns.Status+" IN ?", claimableStatuses).
			Where(models.SearchIndexJobColumns.NextRunAt+" <= ?", now).
			Order(models.SearchIndexJobColumns.Priority + " ASC").
			Order(models.SearchIndexJobColumns.NextRunAt + " ASC").
			Order(models.SearchIndexJobColumns.ID + " ASC").
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

		if err := tx.Model(&models.SearchIndexJob{}).
			Where(models.SearchIndexJobColumns.ID+" IN ?", candidateIDs).
			Where(models.SearchIndexJobColumns.Status+" IN ?", claimableStatuses).
			Updates(map[string]any{
				models.SearchIndexJobColumns.Status:    models.SearchIndexJobStatusRunning,
				models.SearchIndexJobColumns.StartedAt: now,
				models.SearchIndexJobColumns.UpdatedAt: now,
			}).Error; err != nil {
			return err
		}

		rows := make([]searchIndexJobRow, 0, len(candidateIDs))
		if err := tx.Model(&models.SearchIndexJob{}).
			Select(
				models.SearchIndexJobColumns.ID,
				models.SearchIndexJobColumns.JobID,
				models.SearchIndexJobColumns.Provider,
				models.SearchIndexJobColumns.JobType,
				models.SearchIndexJobColumns.DedupeKey,
				models.SearchIndexJobColumns.PayloadJSON,
				models.SearchIndexJobColumns.Status,
				models.SearchIndexJobColumns.Priority,
				models.SearchIndexJobColumns.RetryCount,
				models.SearchIndexJobColumns.NextRunAt+" AS next_run_at_raw",
				models.SearchIndexJobColumns.StartedAt+" AS started_at_raw",
				models.SearchIndexJobColumns.LastError,
				models.SearchIndexJobColumns.CreatedAt+" AS created_at_raw",
				models.SearchIndexJobColumns.UpdatedAt+" AS updated_at_raw",
			).
			Where(models.SearchIndexJobColumns.ID+" IN ?", candidateIDs).
			Where(models.SearchIndexJobColumns.Status+" = ?", models.SearchIndexJobStatusRunning).
			Order(models.SearchIndexJobColumns.Priority + " ASC").
			Order(models.SearchIndexJobColumns.NextRunAt + " ASC").
			Order(models.SearchIndexJobColumns.ID + " ASC").
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
		Model(&models.SearchIndexJob{}).
		Where(models.SearchIndexJobColumns.JobID+" = ?", normalizedJobID).
		Where(models.SearchIndexJobColumns.Status+" = ?", models.SearchIndexJobStatusRunning).
		Updates(map[string]any{
			models.SearchIndexJobColumns.Status:    models.SearchIndexJobStatusSuccess,
			models.SearchIndexJobColumns.StartedAt: nil,
			models.SearchIndexJobColumns.LastError: "",
			models.SearchIndexJobColumns.UpdatedAt: finishedAt,
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
		Model(&models.SearchIndexJob{}).
		Where(models.SearchIndexJobColumns.JobID+" = ?", normalizedJobID).
		Where(models.SearchIndexJobColumns.Status+" = ?", models.SearchIndexJobStatusRunning).
		Updates(map[string]any{
			models.SearchIndexJobColumns.Status:     models.SearchIndexJobStatusFailed,
			models.SearchIndexJobColumns.RetryCount: gorm.Expr(models.SearchIndexJobColumns.RetryCount + " + 1"),
			models.SearchIndexJobColumns.NextRunAt:  nextRunAt,
			models.SearchIndexJobColumns.StartedAt:  nil,
			models.SearchIndexJobColumns.LastError:  strings.TrimSpace(params.LastError),
			models.SearchIndexJobColumns.UpdatedAt:  time.Now().UTC(),
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
	existingErr := tx.Model(&models.SearchIndexJob{}).
		Select(
			models.SearchIndexJobColumns.ID,
			models.SearchIndexJobColumns.JobID,
			models.SearchIndexJobColumns.Provider,
			models.SearchIndexJobColumns.JobType,
			models.SearchIndexJobColumns.DedupeKey,
			models.SearchIndexJobColumns.PayloadJSON,
			models.SearchIndexJobColumns.Status,
			models.SearchIndexJobColumns.Priority,
			models.SearchIndexJobColumns.RetryCount,
			models.SearchIndexJobColumns.NextRunAt+" AS next_run_at_raw",
			models.SearchIndexJobColumns.StartedAt+" AS started_at_raw",
			models.SearchIndexJobColumns.LastError,
			models.SearchIndexJobColumns.CreatedAt+" AS created_at_raw",
			models.SearchIndexJobColumns.UpdatedAt+" AS updated_at_raw",
		).
		Where(
			models.SearchIndexJobColumns.DedupeKey+" = ? AND "+models.SearchIndexJobColumns.Status+" IN ?",
			normalized.DedupeKey,
			[]string{models.SearchIndexJobStatusPending, models.SearchIndexJobStatusFailed},
		).
		Order(models.SearchIndexJobColumns.ID + " DESC").
		Take(&existing).Error
	switch {
	case existingErr == nil:
		merged, keepExisting := mergeSearchIndexJob(existing, normalized)
		if keepExisting {
			return nil
		}
		updates := map[string]any{
			models.SearchIndexJobColumns.Provider:    merged.Provider,
			models.SearchIndexJobColumns.JobType:     merged.JobType,
			models.SearchIndexJobColumns.PayloadJSON: merged.PayloadJSON,
			models.SearchIndexJobColumns.Status:      models.SearchIndexJobStatusPending,
			models.SearchIndexJobColumns.Priority:    merged.Priority,
			models.SearchIndexJobColumns.NextRunAt:   merged.NextRunAt,
			models.SearchIndexJobColumns.StartedAt:   nil,
			models.SearchIndexJobColumns.LastError:   "",
			models.SearchIndexJobColumns.UpdatedAt:   now,
		}
		return tx.Model(&models.SearchIndexJob{}).
			Where(models.SearchIndexJobColumns.ID+" = ?", existing.ID).
			Updates(updates).Error
	case errors.Is(existingErr, gorm.ErrRecordNotFound):
		if normalized.NextRunAt.IsZero() {
			normalized.NextRunAt = now
		}
		return tx.Model(&models.SearchIndexJob{}).Create(map[string]any{
			models.SearchIndexJobColumns.JobID:       strings.ToLower(ulid.Make().String()),
			models.SearchIndexJobColumns.Provider:    normalized.Provider,
			models.SearchIndexJobColumns.JobType:     normalized.JobType,
			models.SearchIndexJobColumns.DedupeKey:   normalized.DedupeKey,
			models.SearchIndexJobColumns.PayloadJSON: normalized.PayloadJSON,
			models.SearchIndexJobColumns.Status:      models.SearchIndexJobStatusPending,
			models.SearchIndexJobColumns.Priority:    normalized.Priority,
			models.SearchIndexJobColumns.RetryCount:  0,
			models.SearchIndexJobColumns.NextRunAt:   normalized.NextRunAt,
			models.SearchIndexJobColumns.StartedAt:   nil,
			models.SearchIndexJobColumns.LastError:   "",
			models.SearchIndexJobColumns.CreatedAt:   now,
			models.SearchIndexJobColumns.UpdatedAt:   now,
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
		NextRunAt:   recordtime.Parse(row.NextRunAtRaw),
		StartedAt:   parseOptionalRecordTime(row.StartedAtRaw),
		LastError:   strings.TrimSpace(row.LastError),
		CreatedAt:   recordtime.Parse(row.CreatedAtRaw),
		UpdatedAt:   recordtime.Parse(row.UpdatedAtRaw),
	}
}

func parseOptionalRecordTime(raw *string) *time.Time {
	return recordtime.ParseNullable(raw)
}
