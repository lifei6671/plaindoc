package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"gorm.io/gorm"
)

const defaultAdminSpaceTransferInterruptedRetention = 10 * time.Minute

// AdminSpaceTransferTask 是前端全局任务中心使用的任务快照。
type AdminSpaceTransferTask struct {
	JobID        string `json:"jobId"`
	Kind         string `json:"kind"`
	Status       string `json:"status"`
	Stage        string `json:"stage,omitempty"`
	Progress     int    `json:"progress"`
	Message      string `json:"message,omitempty"`
	SpaceID      string `json:"spaceId,omitempty"`
	SpaceName    string `json:"spaceName,omitempty"`
	Format       string `json:"format,omitempty"`
	ImportID     string `json:"importId,omitempty"`
	FileName     string `json:"fileName,omitempty"`
	SizeBytes    int64  `json:"sizeBytes,omitempty"`
	NewSpaceID   string `json:"newSpaceId,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	ExpiresAt    string `json:"expiresAt"`
}

// ListAdminSpaceTransferTasksInput 描述当前用户任务列表查询参数。
type ListAdminSpaceTransferTasksInput struct {
	ActorUserID string
	Status      string
	Limit       int
}

// GetAdminSpaceTransferTaskInput 描述当前用户任务详情查询参数。
type GetAdminSpaceTransferTaskInput struct {
	ActorUserID string
	Kind        string
	JobID       string
}

// IssueAdminSpaceTransferStreamInput 描述 SSE stream token 续签参数。
type IssueAdminSpaceTransferStreamInput struct {
	ActorUserID string
	Kind        string
	JobID       string
}

// IssueAdminSpaceTransferStreamResult 是 SSE stream URL 续签结果。
type IssueAdminSpaceTransferStreamResult struct {
	StreamURL string
}

// IssueAdminSpaceTransferDownloadInput 描述导出下载 token 续签参数。
type IssueAdminSpaceTransferDownloadInput struct {
	ActorUserID string
	Kind        string
	JobID       string
}

// IssueAdminSpaceTransferDownloadResult 是导出下载 URL 续签结果。
type IssueAdminSpaceTransferDownloadResult struct {
	DownloadURL string
}

type adminSpaceTransferStreamIssuer interface {
	IssueStreamURL(ctx context.Context, actorUserID string, jobID string) (string, error)
}

type adminSpaceTransferDownloadIssuer interface {
	IssueDownloadURL(ctx context.Context, actorUserID string, jobID string) (string, error)
}

// AdminSpaceTransferTaskServiceOption 用于注入导入/导出 token 续签器。
type AdminSpaceTransferTaskServiceOption func(*AdminSpaceTransferTaskService)

// WithAdminSpaceTransferExportStreamIssuer 注入导出 SSE token 续签器。
func WithAdminSpaceTransferExportStreamIssuer(issuer adminSpaceTransferStreamIssuer) AdminSpaceTransferTaskServiceOption {
	return func(s *AdminSpaceTransferTaskService) {
		s.exportStreamIssuer = issuer
	}
}

// WithAdminSpaceTransferImportStreamIssuer 注入导入 SSE token 续签器。
func WithAdminSpaceTransferImportStreamIssuer(issuer adminSpaceTransferStreamIssuer) AdminSpaceTransferTaskServiceOption {
	return func(s *AdminSpaceTransferTaskService) {
		s.importStreamIssuer = issuer
	}
}

// WithAdminSpaceTransferExportDownloadIssuer 注入导出下载 token 续签器。
func WithAdminSpaceTransferExportDownloadIssuer(issuer adminSpaceTransferDownloadIssuer) AdminSpaceTransferTaskServiceOption {
	return func(s *AdminSpaceTransferTaskService) {
		s.exportDownloadIssuer = issuer
	}
}

// AdminSpaceTransferTaskService 聚合空间导入/导出任务快照。
type AdminSpaceTransferTaskService struct {
	repo                 repository.AdminSpaceTransferJobRepository
	exportStreamIssuer   adminSpaceTransferStreamIssuer
	importStreamIssuer   adminSpaceTransferStreamIssuer
	exportDownloadIssuer adminSpaceTransferDownloadIssuer
}

// NewAdminSpaceTransferTaskService 创建空间传输任务聚合服务。
func NewAdminSpaceTransferTaskService(
	repo repository.AdminSpaceTransferJobRepository,
	options ...AdminSpaceTransferTaskServiceOption,
) *AdminSpaceTransferTaskService {
	svc := &AdminSpaceTransferTaskService{repo: repo}
	for _, option := range options {
		if option != nil {
			option(svc)
		}
	}
	return svc
}

// ListMyTasks 返回当前 actor 可见的导入/导出任务。
func (s *AdminSpaceTransferTaskService) ListMyTasks(
	ctx context.Context,
	input ListAdminSpaceTransferTasksInput,
) ([]AdminSpaceTransferTask, error) {
	if s == nil || s.repo == nil {
		return []AdminSpaceTransferTask{}, nil
	}
	actorUserID := strings.TrimSpace(input.ActorUserID)
	if actorUserID == "" {
		return []AdminSpaceTransferTask{}, nil
	}
	statuses := normalizeAdminSpaceTransferTaskStatusFilter(input.Status)
	jobs, err := s.repo.ListByActor(ctx, repository.ListAdminSpaceTransferJobsParams{
		ActorUserID: actorUserID,
		Statuses:    statuses,
		Limit:       input.Limit,
	})
	if err != nil {
		return nil, err
	}
	tasks := make([]AdminSpaceTransferTask, 0, len(jobs))
	for _, job := range jobs {
		tasks = append(tasks, mapAdminSpaceTransferTask(job))
	}
	return tasks, nil
}

// GetMyTask 返回当前 actor 可见的单个任务。
func (s *AdminSpaceTransferTaskService) GetMyTask(
	ctx context.Context,
	input GetAdminSpaceTransferTaskInput,
) (AdminSpaceTransferTask, error) {
	job, err := s.getOwnedJob(ctx, input.ActorUserID, input.Kind, input.JobID)
	if err != nil {
		return AdminSpaceTransferTask{}, err
	}
	return mapAdminSpaceTransferTask(job), nil
}

// IssueStreamURL 续签当前 actor 任务的 SSE 订阅 URL。
func (s *AdminSpaceTransferTaskService) IssueStreamURL(
	ctx context.Context,
	input IssueAdminSpaceTransferStreamInput,
) (IssueAdminSpaceTransferStreamResult, error) {
	job, err := s.getOwnedJob(ctx, input.ActorUserID, input.Kind, input.JobID)
	if err != nil {
		return IssueAdminSpaceTransferStreamResult{}, err
	}
	if !isActiveAdminSpaceTransferTaskStatus(job.Status) {
		return IssueAdminSpaceTransferStreamResult{}, errcode.ErrAdminSpaceTransferTaskStreamURLUnavailable
	}
	var issuer adminSpaceTransferStreamIssuer
	switch job.Kind {
	case models.AdminSpaceTransferJobKindExport:
		issuer = s.exportStreamIssuer
	case models.AdminSpaceTransferJobKindImport:
		issuer = s.importStreamIssuer
	default:
		return IssueAdminSpaceTransferStreamResult{}, errcode.ErrAdminSpaceTransferTaskKindUnsupported
	}
	if issuer == nil {
		return IssueAdminSpaceTransferStreamResult{}, errcode.ErrAdminSpaceTransferTaskStreamURLUnavailable
	}
	streamURL, err := issuer.IssueStreamURL(ctx, job.ActorUserID, job.JobID)
	if err != nil {
		return IssueAdminSpaceTransferStreamResult{}, err
	}
	return IssueAdminSpaceTransferStreamResult{StreamURL: streamURL}, nil
}

// IssueDownloadURL 续签当前 actor 已完成导出任务的一次性下载 URL。
func (s *AdminSpaceTransferTaskService) IssueDownloadURL(
	ctx context.Context,
	input IssueAdminSpaceTransferDownloadInput,
) (IssueAdminSpaceTransferDownloadResult, error) {
	job, err := s.getOwnedJob(ctx, input.ActorUserID, input.Kind, input.JobID)
	if err != nil {
		return IssueAdminSpaceTransferDownloadResult{}, err
	}
	if job.Kind != models.AdminSpaceTransferJobKindExport ||
		job.Status != models.AdminSpaceTransferJobStatusCompleted ||
		s.exportDownloadIssuer == nil {
		return IssueAdminSpaceTransferDownloadResult{}, errcode.ErrAdminSpaceTransferTaskDownloadUnavailable
	}
	downloadURL, err := s.exportDownloadIssuer.IssueDownloadURL(ctx, job.ActorUserID, job.JobID)
	if err != nil {
		return IssueAdminSpaceTransferDownloadResult{}, err
	}
	return IssueAdminSpaceTransferDownloadResult{DownloadURL: downloadURL}, nil
}

// RecoverInterruptedActiveJobs 将服务重启遗留的 active 任务标记为 failed。
func (s *AdminSpaceTransferTaskService) RecoverInterruptedActiveJobs(
	ctx context.Context,
	now time.Time,
) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return s.repo.MarkActiveJobsFailed(ctx, repository.MarkActiveAdminSpaceTransferJobsFailedParams{
		Now:          now,
		Message:      "服务重启，任务已中断，请重新发起",
		ExpiresAfter: defaultAdminSpaceTransferInterruptedRetention,
	})
}

func (s *AdminSpaceTransferTaskService) getOwnedJob(
	ctx context.Context,
	actorUserID string,
	kind string,
	jobID string,
) (models.AdminSpaceTransferJob, error) {
	if s == nil || s.repo == nil {
		return models.AdminSpaceTransferJob{}, errcode.ErrAdminSpaceTransferTaskNotFound
	}
	actorUserID = strings.TrimSpace(actorUserID)
	kind = strings.TrimSpace(kind)
	jobID = strings.TrimSpace(jobID)
	if actorUserID == "" || kind == "" || jobID == "" {
		return models.AdminSpaceTransferJob{}, errcode.ErrAdminSpaceTransferTaskNotFound
	}
	if kind != models.AdminSpaceTransferJobKindExport && kind != models.AdminSpaceTransferJobKindImport {
		return models.AdminSpaceTransferJob{}, errcode.ErrAdminSpaceTransferTaskKindUnsupported
	}
	job, err := s.repo.GetByKindAndJobID(ctx, kind, jobID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.AdminSpaceTransferJob{}, errcode.ErrAdminSpaceTransferTaskNotFound
		}
		return models.AdminSpaceTransferJob{}, err
	}
	if strings.TrimSpace(job.ActorUserID) != actorUserID {
		return models.AdminSpaceTransferJob{}, errcode.ErrAdminForbidden
	}
	return job, nil
}

func normalizeAdminSpaceTransferTaskStatusFilter(status string) []string {
	switch strings.TrimSpace(status) {
	case "active":
		return []string{
			models.AdminSpaceTransferJobStatusQueued,
			models.AdminSpaceTransferJobStatusRunning,
		}
	case models.AdminSpaceTransferJobStatusQueued,
		models.AdminSpaceTransferJobStatusRunning,
		models.AdminSpaceTransferJobStatusCompleted,
		models.AdminSpaceTransferJobStatusFailed:
		return []string{strings.TrimSpace(status)}
	default:
		return nil
	}
}

func isActiveAdminSpaceTransferTaskStatus(status string) bool {
	return status == models.AdminSpaceTransferJobStatusQueued || status == models.AdminSpaceTransferJobStatusRunning
}

func mapAdminSpaceTransferTask(job models.AdminSpaceTransferJob) AdminSpaceTransferTask {
	return AdminSpaceTransferTask{
		JobID:        strings.TrimSpace(job.JobID),
		Kind:         strings.TrimSpace(job.Kind),
		Status:       strings.TrimSpace(job.Status),
		Stage:        strings.TrimSpace(job.Stage),
		Progress:     job.Progress,
		Message:      strings.TrimSpace(job.Message),
		SpaceID:      strings.TrimSpace(job.SpaceID),
		SpaceName:    strings.TrimSpace(job.SpaceName),
		Format:       strings.TrimSpace(job.Format),
		ImportID:     strings.TrimSpace(job.ImportID),
		FileName:     strings.TrimSpace(job.FileName),
		SizeBytes:    job.SizeBytes,
		NewSpaceID:   strings.TrimSpace(job.NewSpaceID),
		ErrorMessage: strings.TrimSpace(job.ErrorMessage),
		CreatedAt:    formatAdminSpaceTransferTaskTime(job.CreatedAt),
		UpdatedAt:    formatAdminSpaceTransferTaskTime(job.UpdatedAt),
		ExpiresAt:    formatAdminSpaceTransferTaskTime(job.ExpiresAt),
	}
}

func formatAdminSpaceTransferTaskTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
