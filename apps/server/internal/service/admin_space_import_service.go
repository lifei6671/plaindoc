package service

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/oklog/ulid/v2"
)

const (
	defaultAdminSpaceImportStagingTTL = 30 * time.Minute
	maxRunningAdminSpaceImportJobs    = 1
)

// InspectAdminSpaceImportInput 后台空间导入包解析参数。
type InspectAdminSpaceImportInput struct {
	ActorUserID string
	FileName    string
	ContentType string
	Reader      io.Reader
}

// AdminSpaceImportInspectResult 后台空间导入包解析结果。
type AdminSpaceImportInspectResult struct {
	ImportID       string
	PackageVersion int
	PackageType    string
	Importable     bool
	Space          AdminSpaceImportPreviewSpace
	Summary        AdminSpaceExportManifestSummary
	Warnings       []string
}

// AdminSpaceImportPreviewSpace 展示导入包中的源空间信息。
type AdminSpaceImportPreviewSpace struct {
	SpaceID    string `json:"spaceId"`
	Name       string `json:"name"`
	CategoryID string `json:"categoryId,omitempty"`
	Visibility string `json:"visibility"`
}

// CommitAdminSpaceImportInput 后台空间导入提交参数。
type CommitAdminSpaceImportInput struct {
	ActorUserID string
	ImportID    string
	SpaceName   string
	SpaceID     string
	CategoryID  string
	Visibility  string
}

// CommitAdminSpaceImportResult 后台空间导入任务启动结果。
type CommitAdminSpaceImportResult struct {
	JobID     string
	StreamURL string
}

// AdminSpaceImportStaging 记录导入包暂存信息。
type AdminSpaceImportStaging struct {
	ImportID       string
	ActorUserID    string
	FileName       string
	ContentType    string
	PackageVersion int
	PackageType    string
	Importable     bool
	Space          AdminSpaceImportPreviewSpace
	Summary        AdminSpaceExportManifestSummary
	Warnings       []string
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

// AdminSpaceImportStatus 定义后台空间导入任务状态。
type AdminSpaceImportStatus string

const (
	AdminSpaceImportStatusQueued    AdminSpaceImportStatus = "queued"
	AdminSpaceImportStatusRunning   AdminSpaceImportStatus = "running"
	AdminSpaceImportStatusCompleted AdminSpaceImportStatus = "completed"
	AdminSpaceImportStatusFailed    AdminSpaceImportStatus = "failed"
)

// AdminSpaceImportJob 记录进程内导入任务状态。
type AdminSpaceImportJob struct {
	JobID                string
	ImportID             string
	ActorUserID          string
	Status               AdminSpaceImportStatus
	StreamTokenHash      string
	StreamTokenExpiresAt time.Time
	LastEvent            AdminSpaceTransferEvent
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// AdminSpaceImportStore 是进程内导入 staging 和任务表。
type AdminSpaceImportStore struct {
	mu          sync.Mutex
	stagings    map[string]*AdminSpaceImportStaging
	jobs        map[string]*AdminSpaceImportJob
	subscribers map[string]map[chan AdminSpaceTransferEvent]struct{}
}

// NewAdminSpaceImportStore 创建导入任务表。
func NewAdminSpaceImportStore() *AdminSpaceImportStore {
	return &AdminSpaceImportStore{
		stagings:    make(map[string]*AdminSpaceImportStaging),
		jobs:        make(map[string]*AdminSpaceImportJob),
		subscribers: make(map[string]map[chan AdminSpaceTransferEvent]struct{}),
	}
}

// AdminSpaceImportService 封装空间导入任务入口。
type AdminSpaceImportService struct {
	adminAccessService *AdminAccessService
	store              *AdminSpaceImportStore
	nowFn              func() time.Time
}

// NewAdminSpaceImportService 创建空间导入服务。
func NewAdminSpaceImportService(adminAccessService *AdminAccessService) *AdminSpaceImportService {
	return &AdminSpaceImportService{
		adminAccessService: adminAccessService,
		store:              NewAdminSpaceImportStore(),
		nowFn:              time.Now,
	}
}

// Inspect 解析导入包元数据。真实 zip 解析在后续阶段补齐。
func (s *AdminSpaceImportService) Inspect(
	ctx context.Context,
	input InspectAdminSpaceImportInput,
) (AdminSpaceImportInspectResult, error) {
	if s == nil || s.store == nil {
		return AdminSpaceImportInspectResult{}, errcode.ErrAdminSpaceImportCommitForbidden
	}
	if ok, err := s.CanImportSpace(ctx, input.ActorUserID); err != nil {
		return AdminSpaceImportInspectResult{}, err
	} else if !ok {
		return AdminSpaceImportInspectResult{}, errcode.ErrAdminSpaceImportCommitForbidden
	}
	if input.Reader == nil {
		return AdminSpaceImportInspectResult{}, errcode.ErrAdminSpaceImportFileRequired
	}
	if _, err := io.ReadAll(input.Reader); err != nil {
		return AdminSpaceImportInspectResult{}, errcode.ErrAdminSpaceImportZipInvalid
	}

	importID := strings.ToLower(ulid.Make().String())
	now := s.now()
	staging := AdminSpaceImportStaging{
		ImportID:       importID,
		ActorUserID:    strings.TrimSpace(input.ActorUserID),
		FileName:       strings.TrimSpace(input.FileName),
		ContentType:    strings.TrimSpace(input.ContentType),
		PackageVersion: AdminSpaceExportPackageVersion,
		PackageType:    AdminSpaceExportPackageType,
		Importable:     false,
		Warnings:       []string{"导入解析将在后续阶段实现"},
		CreatedAt:      now,
		ExpiresAt:      now.Add(defaultAdminSpaceImportStagingTTL),
	}
	s.store.SaveStaging(staging)
	return AdminSpaceImportInspectResult{
		ImportID:       importID,
		PackageVersion: staging.PackageVersion,
		PackageType:    staging.PackageType,
		Importable:     staging.Importable,
		Space:          staging.Space,
		Summary:        staging.Summary,
		Warnings:       staging.Warnings,
	}, nil
}

// Commit 创建空间导入任务。真实导入在后续阶段补齐。
func (s *AdminSpaceImportService) Commit(
	ctx context.Context,
	input CommitAdminSpaceImportInput,
) (CommitAdminSpaceImportResult, error) {
	if s == nil || s.store == nil {
		return CommitAdminSpaceImportResult{}, errcode.ErrAdminSpaceImportCommitForbidden
	}
	if ok, err := s.CanImportSpace(ctx, input.ActorUserID); err != nil {
		return CommitAdminSpaceImportResult{}, err
	} else if !ok {
		return CommitAdminSpaceImportResult{}, errcode.ErrAdminSpaceImportCommitForbidden
	}
	if strings.TrimSpace(input.ImportID) == "" {
		return CommitAdminSpaceImportResult{}, errcode.ErrAdminSpaceImportStagingNotFound
	}
	staging, err := s.store.GetStaging(strings.TrimSpace(input.ImportID), strings.TrimSpace(input.ActorUserID), s.now())
	if err != nil {
		return CommitAdminSpaceImportResult{}, err
	}
	if !staging.Importable {
		return CommitAdminSpaceImportResult{}, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	if err := s.store.EnsureActorCanStartJob(strings.TrimSpace(input.ActorUserID)); err != nil {
		return CommitAdminSpaceImportResult{}, err
	}

	jobID := strings.ToLower(ulid.Make().String())
	streamToken, streamTokenHash, tokenErr := generateAdminSpaceTransferToken()
	if tokenErr != nil {
		return CommitAdminSpaceImportResult{}, tokenErr
	}
	now := s.now()
	job := AdminSpaceImportJob{
		JobID:                jobID,
		ImportID:             staging.ImportID,
		ActorUserID:          strings.TrimSpace(input.ActorUserID),
		Status:               AdminSpaceImportStatusQueued,
		StreamTokenHash:      streamTokenHash,
		StreamTokenExpiresAt: now.Add(defaultAdminSpaceTransferTokenTTL),
		LastEvent: AdminSpaceTransferEvent{
			Type:     AdminSpaceTransferEventTypeProgress,
			Stage:    "queued",
			Progress: 0,
			Message:  "导入任务已创建",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.CreateJob(job); err != nil {
		return CommitAdminSpaceImportResult{}, err
	}
	return CommitAdminSpaceImportResult{
		JobID:     jobID,
		StreamURL: "/api/admin/space-imports/" + jobID + "/events?token=" + streamToken,
	}, nil
}

// CanImportSpace 判断 actor 是否具备导入 zip 创建新空间的能力。
func (s *AdminSpaceImportService) CanImportSpace(_ context.Context, actorUserID string) (bool, error) {
	// 当前系统的协作端创建空间入口对所有已登录用户开放；后续若增加开关，
	// 这里统一接入配置判定，避免导入权限散落在 handler。
	return strings.TrimSpace(actorUserID) != "", nil
}

// Subscribe 校验 importStreamToken 并订阅导入任务事件。
func (s *AdminSpaceImportService) Subscribe(
	_ context.Context,
	jobID string,
	actorUserID string,
	streamToken string,
) (AdminSpaceTransferEvent, <-chan AdminSpaceTransferEvent, func(), error) {
	if s == nil || s.store == nil {
		return AdminSpaceTransferEvent{}, nil, func() {}, errcode.ErrAdminSpaceImportStagingNotFound
	}
	return s.store.Subscribe(strings.TrimSpace(jobID), strings.TrimSpace(actorUserID), strings.TrimSpace(streamToken), s.now())
}

// PublishProgress 广播导入任务进度。
func (s *AdminSpaceImportService) PublishProgress(jobID string, event AdminSpaceTransferEvent) {
	if s == nil || s.store == nil {
		return
	}
	s.store.Publish(strings.TrimSpace(jobID), event, s.now())
}

// BeginImportJob 将导入任务切到 running；真实 worker 在后续阶段调用。
func (s *AdminSpaceImportService) BeginImportJob(ctx context.Context, jobID string) error {
	if s == nil || s.store == nil {
		return errcode.ErrAdminSpaceImportStagingNotFound
	}
	job, err := s.store.GetJob(strings.TrimSpace(jobID))
	if err != nil {
		return err
	}
	if ok, err := s.CanImportSpace(ctx, job.ActorUserID); err != nil {
		return err
	} else if !ok {
		s.store.Fail(job.JobID, "permission", "导入权限已失效", s.now())
		return errcode.ErrAdminSpaceImportCommitForbidden
	}
	return s.store.MarkRunning(job.JobID, s.now())
}

func (s *AdminSpaceImportService) now() time.Time {
	if s != nil && s.nowFn != nil {
		return s.nowFn().UTC()
	}
	return time.Now().UTC()
}

func (s *AdminSpaceImportStore) SaveStaging(staging AdminSpaceImportStaging) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	copyValue := staging
	s.stagings[importStagingKey(staging.ImportID, staging.ActorUserID)] = &copyValue
}

func (s *AdminSpaceImportStore) GetStaging(importID string, actorUserID string, now time.Time) (AdminSpaceImportStaging, error) {
	if s == nil || importID == "" || actorUserID == "" {
		return AdminSpaceImportStaging{}, errcode.ErrAdminSpaceImportStagingNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	staging, ok := s.stagings[importStagingKey(importID, actorUserID)]
	if !ok || staging == nil {
		return AdminSpaceImportStaging{}, errcode.ErrAdminSpaceImportStagingNotFound
	}
	if !now.Before(staging.ExpiresAt) {
		return AdminSpaceImportStaging{}, errcode.ErrAdminSpaceImportStagingExpired
	}
	return *staging, nil
}

func (s *AdminSpaceImportStore) EnsureActorCanStartJob(actorUserID string) error {
	if s == nil {
		return errcode.ErrAdminSpaceImportJobRunningLimit
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, job := range s.jobs {
		if job == nil || job.ActorUserID != actorUserID {
			continue
		}
		if isActiveAdminSpaceImportStatus(job.Status) {
			return errcode.ErrAdminSpaceImportJobRunningLimit
		}
	}
	return nil
}

func (s *AdminSpaceImportStore) CreateJob(job AdminSpaceImportJob) error {
	if s == nil || strings.TrimSpace(job.JobID) == "" {
		return errcode.ErrAdminSpaceImportStagingNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	activeCount := 0
	for _, existing := range s.jobs {
		if existing == nil || existing.ActorUserID != job.ActorUserID {
			continue
		}
		if isActiveAdminSpaceImportStatus(existing.Status) {
			activeCount++
		}
	}
	if activeCount >= maxRunningAdminSpaceImportJobs {
		return errcode.ErrAdminSpaceImportJobRunningLimit
	}
	copyJob := job
	s.jobs[job.JobID] = &copyJob
	return nil
}

func (s *AdminSpaceImportStore) GetJob(jobID string) (AdminSpaceImportJob, error) {
	if s == nil || jobID == "" {
		return AdminSpaceImportJob{}, errcode.ErrAdminSpaceImportStagingNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[jobID]
	if !ok || job == nil {
		return AdminSpaceImportJob{}, errcode.ErrAdminSpaceImportStagingNotFound
	}
	return *job, nil
}

func (s *AdminSpaceImportStore) Subscribe(
	jobID string,
	actorUserID string,
	streamToken string,
	now time.Time,
) (AdminSpaceTransferEvent, <-chan AdminSpaceTransferEvent, func(), error) {
	if s == nil || jobID == "" {
		return AdminSpaceTransferEvent{}, nil, func() {}, errcode.ErrAdminSpaceImportStagingNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok || job == nil {
		return AdminSpaceTransferEvent{}, nil, func() {}, errcode.ErrAdminSpaceImportStagingNotFound
	}
	if job.ActorUserID != actorUserID ||
		job.StreamTokenHash == "" ||
		tokenHash(streamToken) != job.StreamTokenHash ||
		!now.Before(job.StreamTokenExpiresAt) {
		return AdminSpaceTransferEvent{}, nil, func() {}, errcode.ErrAdminSpaceImportJobTokenInvalid
	}

	ch := make(chan AdminSpaceTransferEvent, adminSpaceTransferEventBufferSize)
	if s.subscribers[jobID] == nil {
		s.subscribers[jobID] = make(map[chan AdminSpaceTransferEvent]struct{})
	}
	s.subscribers[jobID][ch] = struct{}{}
	unsubscribe := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if subscribers := s.subscribers[jobID]; subscribers != nil {
			delete(subscribers, ch)
			if len(subscribers) == 0 {
				delete(s.subscribers, jobID)
			}
		}
		close(ch)
	}
	return job.LastEvent, ch, unsubscribe, nil
}

func (s *AdminSpaceImportStore) Publish(jobID string, event AdminSpaceTransferEvent, now time.Time) {
	if s == nil || jobID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[jobID]
	if !ok || job == nil {
		return
	}
	job.LastEvent = event
	job.UpdatedAt = now
	for subscriber := range s.subscribers[jobID] {
		select {
		case subscriber <- event:
		default:
		}
	}
}

func (s *AdminSpaceImportStore) MarkRunning(jobID string, now time.Time) error {
	if s == nil || jobID == "" {
		return errcode.ErrAdminSpaceImportStagingNotFound
	}
	s.mu.Lock()
	job, ok := s.jobs[jobID]
	if !ok || job == nil {
		s.mu.Unlock()
		return errcode.ErrAdminSpaceImportStagingNotFound
	}
	job.Status = AdminSpaceImportStatusRunning
	job.LastEvent = AdminSpaceTransferEvent{Type: AdminSpaceTransferEventTypeProgress, Stage: "running", Progress: 1, Message: "导入任务开始执行"}
	job.UpdatedAt = now
	event := job.LastEvent
	s.mu.Unlock()
	s.Publish(jobID, event, now)
	return nil
}

func (s *AdminSpaceImportStore) Fail(jobID string, stage string, message string, now time.Time) {
	if s == nil || jobID == "" {
		return
	}
	s.mu.Lock()
	job, ok := s.jobs[jobID]
	if !ok || job == nil {
		s.mu.Unlock()
		return
	}
	job.Status = AdminSpaceImportStatusFailed
	job.LastEvent = AdminSpaceTransferEvent{Type: AdminSpaceTransferEventTypeFailed, Stage: stage, Message: message}
	job.UpdatedAt = now
	event := job.LastEvent
	s.mu.Unlock()
	s.Publish(jobID, event, now)
}

func importStagingKey(importID string, actorUserID string) string {
	return strings.TrimSpace(actorUserID) + ":" + strings.TrimSpace(importID)
}

func isActiveAdminSpaceImportStatus(status AdminSpaceImportStatus) bool {
	return status == AdminSpaceImportStatusQueued || status == AdminSpaceImportStatusRunning
}
