package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

type gormAdminSpaceTransferJobRepository struct {
	db *gorm.DB
}

// NewGormAdminSpaceTransferJobRepository 创建后台空间传输任务仓储实现。
func NewGormAdminSpaceTransferJobRepository(db *gorm.DB) AdminSpaceTransferJobRepository {
	return &gormAdminSpaceTransferJobRepository{db: db}
}

func (r *gormAdminSpaceTransferJobRepository) Create(ctx context.Context, job *models.AdminSpaceTransferJob) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("admin space transfer job repository db is nil")
	}
	if job == nil {
		return fmt.Errorf("admin space transfer job is nil")
	}
	normalizeAdminSpaceTransferJob(job, time.Now().UTC())
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *gormAdminSpaceTransferJobRepository) UpdateProgress(
	ctx context.Context,
	params UpdateAdminSpaceTransferJobProgressParams,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("admin space transfer job repository db is nil")
	}
	jobID := strings.TrimSpace(params.JobID)
	if jobID == "" {
		return gorm.ErrRecordNotFound
	}
	now := params.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	progress := params.Progress
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	updateTx := r.db.WithContext(ctx).
		Model(&models.AdminSpaceTransferJob{}).
		Where(models.AdminSpaceTransferJobColumns.JobID+" = ?", jobID).
		Where(models.AdminSpaceTransferJobColumns.Status+" IN ?", []string{
			models.AdminSpaceTransferJobStatusQueued,
			models.AdminSpaceTransferJobStatusRunning,
		}).
		Updates(map[string]any{
			models.AdminSpaceTransferJobColumns.Status:    models.AdminSpaceTransferJobStatusRunning,
			models.AdminSpaceTransferJobColumns.Stage:     strings.TrimSpace(params.Stage),
			models.AdminSpaceTransferJobColumns.Progress:  progress,
			models.AdminSpaceTransferJobColumns.Message:   strings.TrimSpace(params.Message),
			models.AdminSpaceTransferJobColumns.StartedAt: gorm.Expr("COALESCE("+models.AdminSpaceTransferJobColumns.StartedAt+", ?)", now),
			models.AdminSpaceTransferJobColumns.UpdatedAt: now,
		})
	if updateTx.Error != nil {
		return updateTx.Error
	}
	if updateTx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *gormAdminSpaceTransferJobRepository) MarkCompleted(
	ctx context.Context,
	params MarkAdminSpaceTransferJobCompletedParams,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("admin space transfer job repository db is nil")
	}
	jobID := strings.TrimSpace(params.JobID)
	if jobID == "" {
		return gorm.ErrRecordNotFound
	}
	completedAt := params.CompletedAt
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	expiresAt := params.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = completedAt.Add(10 * time.Minute)
	}
	updateTx := r.db.WithContext(ctx).
		Model(&models.AdminSpaceTransferJob{}).
		Where(models.AdminSpaceTransferJobColumns.JobID+" = ?", jobID).
		Updates(map[string]any{
			models.AdminSpaceTransferJobColumns.Status:      models.AdminSpaceTransferJobStatusCompleted,
			models.AdminSpaceTransferJobColumns.Stage:       strings.TrimSpace(params.Stage),
			models.AdminSpaceTransferJobColumns.Progress:    100,
			models.AdminSpaceTransferJobColumns.Message:     strings.TrimSpace(params.Message),
			models.AdminSpaceTransferJobColumns.FilePath:    strings.TrimSpace(params.FilePath),
			models.AdminSpaceTransferJobColumns.FileName:    strings.TrimSpace(params.FileName),
			models.AdminSpaceTransferJobColumns.SizeBytes:   params.SizeBytes,
			models.AdminSpaceTransferJobColumns.NewSpaceID:  strings.TrimSpace(params.NewSpaceID),
			models.AdminSpaceTransferJobColumns.CompletedAt: completedAt,
			models.AdminSpaceTransferJobColumns.UpdatedAt:   completedAt,
			models.AdminSpaceTransferJobColumns.ExpiresAt:   expiresAt,
		})
	if updateTx.Error != nil {
		return updateTx.Error
	}
	if updateTx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *gormAdminSpaceTransferJobRepository) MarkFailed(
	ctx context.Context,
	params MarkAdminSpaceTransferJobFailedParams,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("admin space transfer job repository db is nil")
	}
	jobID := strings.TrimSpace(params.JobID)
	if jobID == "" {
		return gorm.ErrRecordNotFound
	}
	failedAt := params.FailedAt
	if failedAt.IsZero() {
		failedAt = time.Now().UTC()
	}
	expiresAt := params.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = failedAt.Add(10 * time.Minute)
	}
	updateTx := r.db.WithContext(ctx).
		Model(&models.AdminSpaceTransferJob{}).
		Where(models.AdminSpaceTransferJobColumns.JobID+" = ?", jobID).
		Updates(map[string]any{
			models.AdminSpaceTransferJobColumns.Status:       models.AdminSpaceTransferJobStatusFailed,
			models.AdminSpaceTransferJobColumns.Stage:        strings.TrimSpace(params.Stage),
			models.AdminSpaceTransferJobColumns.Message:      strings.TrimSpace(params.Message),
			models.AdminSpaceTransferJobColumns.ErrorMessage: strings.TrimSpace(params.ErrorMessage),
			models.AdminSpaceTransferJobColumns.CompletedAt:  failedAt,
			models.AdminSpaceTransferJobColumns.UpdatedAt:    failedAt,
			models.AdminSpaceTransferJobColumns.ExpiresAt:    expiresAt,
		})
	if updateTx.Error != nil {
		return updateTx.Error
	}
	if updateTx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *gormAdminSpaceTransferJobRepository) ListByActor(
	ctx context.Context,
	params ListAdminSpaceTransferJobsParams,
) ([]models.AdminSpaceTransferJob, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("admin space transfer job repository db is nil")
	}
	actorUserID := strings.TrimSpace(params.ActorUserID)
	if actorUserID == "" {
		return []models.AdminSpaceTransferJob{}, nil
	}
	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := r.db.WithContext(ctx).
		Model(&models.AdminSpaceTransferJob{}).
		Where(models.AdminSpaceTransferJobColumns.ActorUserID+" = ?", actorUserID)
	if len(params.Statuses) > 0 {
		query = query.Where(models.AdminSpaceTransferJobColumns.Status+" IN ?", normalizedTransferJobStatuses(params.Statuses))
	}
	var jobs []models.AdminSpaceTransferJob
	if err := query.
		Order(models.AdminSpaceTransferJobColumns.UpdatedAt + " DESC").
		Order(models.AdminSpaceTransferJobColumns.ID + " DESC").
		Limit(limit).
		Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}

func (r *gormAdminSpaceTransferJobRepository) GetByKindAndJobID(
	ctx context.Context,
	kind string,
	jobID string,
) (models.AdminSpaceTransferJob, error) {
	if r == nil || r.db == nil {
		return models.AdminSpaceTransferJob{}, fmt.Errorf("admin space transfer job repository db is nil")
	}
	var job models.AdminSpaceTransferJob
	if err := r.db.WithContext(ctx).
		Model(&models.AdminSpaceTransferJob{}).
		Where(models.AdminSpaceTransferJobColumns.Kind+" = ?", strings.TrimSpace(kind)).
		Where(models.AdminSpaceTransferJobColumns.JobID+" = ?", strings.TrimSpace(jobID)).
		First(&job).Error; err != nil {
		return models.AdminSpaceTransferJob{}, err
	}
	return job, nil
}

func (r *gormAdminSpaceTransferJobRepository) MarkActiveJobsFailed(
	ctx context.Context,
	params MarkActiveAdminSpaceTransferJobsFailedParams,
) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("admin space transfer job repository db is nil")
	}
	now := params.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	expiresAfter := params.ExpiresAfter
	if expiresAfter <= 0 {
		expiresAfter = 10 * time.Minute
	}
	message := strings.TrimSpace(params.Message)
	if message == "" {
		message = "服务重启，任务已中断，请重新发起"
	}
	updateTx := r.db.WithContext(ctx).
		Model(&models.AdminSpaceTransferJob{}).
		Where(models.AdminSpaceTransferJobColumns.Status+" IN ?", []string{
			models.AdminSpaceTransferJobStatusQueued,
			models.AdminSpaceTransferJobStatusRunning,
		}).
		Updates(map[string]any{
			models.AdminSpaceTransferJobColumns.Status:       models.AdminSpaceTransferJobStatusFailed,
			models.AdminSpaceTransferJobColumns.Stage:        "interrupted",
			models.AdminSpaceTransferJobColumns.Message:      message,
			models.AdminSpaceTransferJobColumns.ErrorMessage: message,
			models.AdminSpaceTransferJobColumns.CompletedAt:  now,
			models.AdminSpaceTransferJobColumns.UpdatedAt:    now,
			models.AdminSpaceTransferJobColumns.ExpiresAt:    now.Add(expiresAfter),
		})
	if updateTx.Error != nil {
		return 0, updateTx.Error
	}
	return updateTx.RowsAffected, nil
}

func (r *gormAdminSpaceTransferJobRepository) ListExpired(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]models.AdminSpaceTransferJob, error) {
	return r.listExpired(ctx, "", now, limit)
}

func (r *gormAdminSpaceTransferJobRepository) ListExpiredByKind(
	ctx context.Context,
	kind string,
	now time.Time,
	limit int,
) ([]models.AdminSpaceTransferJob, error) {
	return r.listExpired(ctx, strings.TrimSpace(kind), now, limit)
}

func (r *gormAdminSpaceTransferJobRepository) DeleteByIDs(ctx context.Context, ids []int64) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("admin space transfer job repository db is nil")
	}
	normalizedIDs := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		normalizedIDs = append(normalizedIDs, id)
	}
	if len(normalizedIDs) == 0 {
		return 0, nil
	}
	deleteTx := r.db.WithContext(ctx).
		Where(models.AdminSpaceTransferJobColumns.ID+" IN ?", normalizedIDs).
		Delete(&models.AdminSpaceTransferJob{})
	if deleteTx.Error != nil {
		return 0, deleteTx.Error
	}
	return deleteTx.RowsAffected, nil
}

func (r *gormAdminSpaceTransferJobRepository) listExpired(
	ctx context.Context,
	kind string,
	now time.Time,
	limit int,
) ([]models.AdminSpaceTransferJob, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("admin space transfer job repository db is nil")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := r.db.WithContext(ctx).
		Model(&models.AdminSpaceTransferJob{}).
		Where(models.AdminSpaceTransferJobColumns.Status+" IN ?", []string{
			models.AdminSpaceTransferJobStatusCompleted,
			models.AdminSpaceTransferJobStatusFailed,
		}).
		Where(models.AdminSpaceTransferJobColumns.ExpiresAt+" <= ?", now)
	if kind != "" {
		query = query.Where(models.AdminSpaceTransferJobColumns.Kind+" = ?", kind)
	}
	expired := make([]models.AdminSpaceTransferJob, 0, limit)
	if err := query.
		Order(models.AdminSpaceTransferJobColumns.ExpiresAt + " ASC").
		Order(models.AdminSpaceTransferJobColumns.ID + " ASC").
		Limit(limit).
		Find(&expired).Error; err != nil {
		return nil, err
	}
	return expired, nil
}

func normalizeAdminSpaceTransferJob(job *models.AdminSpaceTransferJob, now time.Time) {
	job.JobID = strings.TrimSpace(job.JobID)
	job.Kind = strings.TrimSpace(job.Kind)
	job.ActorUserID = strings.TrimSpace(job.ActorUserID)
	job.SpaceID = strings.TrimSpace(job.SpaceID)
	job.SpaceName = strings.TrimSpace(job.SpaceName)
	job.Format = strings.TrimSpace(job.Format)
	job.ImportID = strings.TrimSpace(job.ImportID)
	job.Status = strings.TrimSpace(job.Status)
	job.Stage = strings.TrimSpace(job.Stage)
	job.Message = strings.TrimSpace(job.Message)
	job.FilePath = strings.TrimSpace(job.FilePath)
	job.FileName = strings.TrimSpace(job.FileName)
	job.NewSpaceID = strings.TrimSpace(job.NewSpaceID)
	job.ErrorMessage = strings.TrimSpace(job.ErrorMessage)
	if job.Status == "" {
		job.Status = models.AdminSpaceTransferJobStatusQueued
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	if job.UpdatedAt.IsZero() {
		job.UpdatedAt = job.CreatedAt
	}
	if job.ExpiresAt.IsZero() {
		job.ExpiresAt = job.CreatedAt.Add(30 * time.Minute)
	}
	if job.Progress < 0 {
		job.Progress = 0
	}
	if job.Progress > 100 {
		job.Progress = 100
	}
}

func normalizedTransferJobStatuses(statuses []string) []string {
	normalized := make([]string, 0, len(statuses))
	for _, status := range statuses {
		status = strings.TrimSpace(status)
		if status == "" {
			continue
		}
		normalized = append(normalized, status)
	}
	return normalized
}
