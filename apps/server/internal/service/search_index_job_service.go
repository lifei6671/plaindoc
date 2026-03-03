package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

const (
	defaultSearchIndexJobBatchSize   = 32
	defaultSearchIndexJobTaskTimeout = 45 * time.Second
)

var (
	// ErrSearchIndexJobServiceUnavailable 表示任务服务依赖未准备就绪。
	ErrSearchIndexJobServiceUnavailable = errors.New("search index job service dependencies are nil")
)

// SearchIndexJobRunResult 表示一次任务消费轮询结果。
type SearchIndexJobRunResult struct {
	Claimed   int
	Succeeded int
	Retried   int
}

// SearchIndexJobService 负责消费检索 Outbox 任务。
type SearchIndexJobService struct {
	jobRepo            repository.SearchIndexJobRepository
	searchIndexService *SearchIndexService
}

type searchIndexDocumentJobPayload struct {
	DocumentID string `json:"documentId"`
}

type searchIndexSpaceJobPayload struct {
	SpaceID string `json:"spaceId"`
}

// NewSearchIndexJobService 创建检索 Outbox 任务消费服务。
func NewSearchIndexJobService(
	jobRepo repository.SearchIndexJobRepository,
	searchIndexService *SearchIndexService,
) *SearchIndexJobService {
	return &SearchIndexJobService{
		jobRepo:            jobRepo,
		searchIndexService: searchIndexService,
	}
}

// RunOnce 消费一批可执行检索任务。
func (s *SearchIndexJobService) RunOnce(
	ctx context.Context,
) (SearchIndexJobRunResult, error) {
	if s == nil || s.jobRepo == nil || s.searchIndexService == nil {
		return SearchIndexJobRunResult{}, ErrSearchIndexJobServiceUnavailable
	}

	jobs, err := s.jobRepo.ClaimRunnableJobs(ctx, repository.ClaimSearchIndexJobsParams{
		Limit: defaultSearchIndexJobBatchSize,
		Now:   time.Now().UTC(),
	})
	if err != nil {
		return SearchIndexJobRunResult{}, err
	}

	result := SearchIndexJobRunResult{
		Claimed: len(jobs),
	}
	for _, job := range jobs {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		executeErr := s.executeJob(ctx, job)
		if executeErr != nil {
			nextRunAt := time.Now().UTC().Add(resolveSearchIndexJobRetryDelay(job.RetryCount + 1))
			retryErr := s.jobRepo.MarkRetry(ctx, repository.MarkSearchIndexJobRetryParams{
				JobID:     strings.TrimSpace(job.JobID),
				NextRunAt: nextRunAt,
				LastError: executeErr.Error(),
			})
			if retryErr != nil {
				return result, retryErr
			}
			result.Retried += 1
			continue
		}

		if err := s.jobRepo.MarkSuccess(ctx, strings.TrimSpace(job.JobID), time.Now().UTC()); err != nil {
			return result, err
		}
		result.Succeeded += 1
	}
	return result, nil
}

func (s *SearchIndexJobService) executeJob(
	ctx context.Context,
	job models.SearchIndexJob,
) error {
	if s == nil || s.searchIndexService == nil {
		return ErrSearchIndexJobServiceUnavailable
	}

	taskCtx, cancel := context.WithTimeout(ctx, defaultSearchIndexJobTaskTimeout)
	defer cancel()

	jobType := strings.TrimSpace(strings.ToUpper(job.JobType))
	switch jobType {
	case models.SearchIndexJobTypeDocUpsert:
		payload, err := decodeSearchIndexDocumentPayload(job.PayloadJSON)
		if err != nil {
			return err
		}
		return s.searchIndexService.SyncDocumentByID(taskCtx, payload.DocumentID)
	case models.SearchIndexJobTypeDocDelete:
		payload, err := decodeSearchIndexDocumentPayload(job.PayloadJSON)
		if err != nil {
			return err
		}
		return s.searchIndexService.DeleteDocumentByID(taskCtx, payload.DocumentID)
	case models.SearchIndexJobTypeRebuildSpace:
		payload, err := decodeSearchIndexSpacePayload(job.PayloadJSON)
		if err != nil {
			return err
		}
		return s.searchIndexService.SyncSpaceByID(taskCtx, payload.SpaceID)
	case models.SearchIndexJobTypeSpacePurge:
		payload, err := decodeSearchIndexSpacePayload(job.PayloadJSON)
		if err != nil {
			return err
		}
		return s.searchIndexService.PurgeSpaceByID(taskCtx, payload.SpaceID)
	default:
		return fmt.Errorf("unsupported search index job type: %s", jobType)
	}
}

func decodeSearchIndexDocumentPayload(
	payloadJSON string,
) (searchIndexDocumentJobPayload, error) {
	normalized := strings.TrimSpace(payloadJSON)
	if normalized == "" {
		return searchIndexDocumentJobPayload{}, errors.New("search index document payload is empty")
	}

	var payload searchIndexDocumentJobPayload
	if err := json.Unmarshal([]byte(normalized), &payload); err != nil {
		return searchIndexDocumentJobPayload{}, fmt.Errorf("decode search index document payload: %w", err)
	}
	payload.DocumentID = strings.TrimSpace(payload.DocumentID)
	if payload.DocumentID == "" {
		return searchIndexDocumentJobPayload{}, errors.New("search index document id is empty")
	}
	return payload, nil
}

func decodeSearchIndexSpacePayload(
	payloadJSON string,
) (searchIndexSpaceJobPayload, error) {
	normalized := strings.TrimSpace(payloadJSON)
	if normalized == "" {
		return searchIndexSpaceJobPayload{}, errors.New("search index space payload is empty")
	}

	var payload searchIndexSpaceJobPayload
	if err := json.Unmarshal([]byte(normalized), &payload); err != nil {
		return searchIndexSpaceJobPayload{}, fmt.Errorf("decode search index space payload: %w", err)
	}
	payload.SpaceID = strings.TrimSpace(payload.SpaceID)
	if payload.SpaceID == "" {
		return searchIndexSpaceJobPayload{}, errors.New("search index space id is empty")
	}
	return payload, nil
}

func resolveSearchIndexJobRetryDelay(retryCount int) time.Duration {
	if retryCount <= 0 {
		retryCount = 1
	}

	base := 2 * time.Second
	maxDelay := 5 * time.Minute
	delay := base
	for i := 1; i < retryCount && delay < maxDelay; i += 1 {
		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
			break
		}
	}

	// 追加 0~20% 抖动，减少重试集中打点。
	jitterWindow := delay / 5
	if jitterWindow <= 0 {
		return delay
	}
	jitter := time.Duration(time.Now().UTC().UnixNano() % int64(jitterWindow+1))
	return delay + jitter
}
