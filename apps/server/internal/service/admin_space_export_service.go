package service

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

const (
	defaultAdminSpaceTransferTokenTTL = 10 * time.Minute
	adminSpaceTransferEventBufferSize = 8
	maxRunningAdminSpaceExportJobs    = 2
	defaultAdminSpaceExportDir        = "data/exports/admin-space"
	maxAdminSpaceExportBlobReadBytes  = 512 << 20
)

// AdminSpaceExportFormat 定义后台空间导出格式。
type AdminSpaceExportFormat string

const (
	AdminSpaceExportFormatMarkdownZip AdminSpaceExportFormat = "markdown_zip"
	AdminSpaceExportFormatSourceZip   AdminSpaceExportFormat = "source_zip"
	AdminSpaceExportFormatEPUB        AdminSpaceExportFormat = "epub"
)

// StartAdminSpaceExportInput 后台空间导出启动参数。
type StartAdminSpaceExportInput struct {
	ActorUserID          string
	SpaceID              string
	Format               AdminSpaceExportFormat
	IncludeAttachments   bool
	IncludeOfficeSources bool
}

// StartAdminSpaceExportResult 后台空间导出启动结果。
type StartAdminSpaceExportResult struct {
	JobID     string
	StreamURL string
}

// AdminSpaceExportStatus 定义后台空间导出任务状态。
type AdminSpaceExportStatus string

const (
	AdminSpaceExportStatusQueued    AdminSpaceExportStatus = "queued"
	AdminSpaceExportStatusRunning   AdminSpaceExportStatus = "running"
	AdminSpaceExportStatusCompleted AdminSpaceExportStatus = "completed"
	AdminSpaceExportStatusFailed    AdminSpaceExportStatus = "failed"
)

// AdminSpaceTransferEventType 定义导入导出 SSE 事件类型。
type AdminSpaceTransferEventType string

const (
	AdminSpaceTransferEventTypeProgress  AdminSpaceTransferEventType = "progress"
	AdminSpaceTransferEventTypeCompleted AdminSpaceTransferEventType = "completed"
	AdminSpaceTransferEventTypeFailed    AdminSpaceTransferEventType = "failed"
)

// AdminSpaceTransferEvent 是导入导出后台任务的 SSE 事件。
type AdminSpaceTransferEvent struct {
	Type        AdminSpaceTransferEventType `json:"type"`
	Stage       string                      `json:"stage,omitempty"`
	Progress    int                         `json:"progress,omitempty"`
	Message     string                      `json:"message,omitempty"`
	DownloadURL string                      `json:"downloadUrl,omitempty"`
	FileName    string                      `json:"fileName,omitempty"`
	SizeBytes   int64                       `json:"sizeBytes,omitempty"`
	SpaceID     string                      `json:"spaceId,omitempty"`
	SpaceName   string                      `json:"spaceName,omitempty"`
}

// AdminSpaceExportJob 记录进程内导出任务状态。
type AdminSpaceExportJob struct {
	JobID                  string
	ActorUserID            string
	SpaceID                string
	Format                 AdminSpaceExportFormat
	IncludeAttachments     bool
	IncludeOfficeSources   bool
	Status                 AdminSpaceExportStatus
	StreamTokenHash        string
	StreamTokenExpiresAt   time.Time
	DownloadTokenHash      string
	DownloadTokenExpiresAt time.Time
	DownloadTokenUsed      bool
	FileName               string
	FilePath               string
	SizeBytes              int64
	LastEvent              AdminSpaceTransferEvent
	CreatedAt              time.Time
	UpdatedAt              time.Time
	downloadTokens         map[string]adminSpaceExportDownloadToken
}

type adminSpaceExportDownloadToken struct {
	ExpiresAt time.Time
	Used      bool
}

// AdminSpaceExportDownload 是通过短期 token 解析出的服务端私有下载文件。
type AdminSpaceExportDownload struct {
	FileName  string
	FilePath  string
	SizeBytes int64
}

type adminSpaceExportSpaceReader interface {
	GetBySpaceID(ctx context.Context, spaceID string) (*models.Space, error)
}

type adminSpaceExportCoverReader interface {
	GetCoverAssetByAssetID(ctx context.Context, assetID string) (*models.SpaceCoverAsset, error)
}

type adminSpaceExportWorkspaceReader interface {
	ListTreeNodesBySpaceID(ctx context.Context, spaceID string) ([]repository.WorkspaceTreeNodeRecord, error)
	GetDocumentByDocumentID(ctx context.Context, documentID string) (*repository.WorkspaceDocumentRecord, error)
}

type adminSpaceExportAttachmentReader interface {
	ListByDocumentID(ctx context.Context, documentID string, includeDeleted bool) ([]models.DocumentAttachment, error)
	GetBlobByBlobID(ctx context.Context, blobID string) (*models.DocumentAttachmentBlob, error)
}

type adminSpaceExportBlobContentReader interface {
	ReadBlobContent(ctx context.Context, blob models.DocumentAttachmentBlob, fallbackFileName string) ([]byte, error)
}

type adminSpaceExportImageHostingService interface {
	GetConfig(ctx context.Context) (ImageHostingConfig, error)
	BuildObjectReadURL(
		ctx context.Context,
		config ImageHostingConfig,
		input BuildImageHostingObjectReadURLInput,
	) (string, error)
}

type adminSpaceExportOfficeHTMLRenderer interface {
	RenderExportHTML(
		ctx context.Context,
		format models.DocumentFormat,
		sourceContent []byte,
		spaceID string,
		documentID string,
	) (string, error)
}

// AdminSpaceExportReaderHTMLRenderInput 是 EPUB 导出复用阅读页 SSR 的 Markdown 渲染输入。
type AdminSpaceExportReaderHTMLRenderInput struct {
	Space      models.Space
	Document   AdminSpaceExportDocumentEntry
	Content    string
	Tree       AdminSpaceExportTree
	ExportedAt time.Time
}

type adminSpaceExportReaderHTMLRenderer interface {
	RenderMarkdownHTML(ctx context.Context, input AdminSpaceExportReaderHTMLRenderInput) (string, error)
}

type adminAuditRecorder interface {
	Record(ctx context.Context, input RecordAdminAuditInput) error
}

// AdminSpaceExportServiceOption 用于按运行环境注入导出依赖。
type AdminSpaceExportServiceOption func(*AdminSpaceExportService)

// WithAdminSpaceExportRepositories 注入导出所需的空间与工作区读取仓储。
func WithAdminSpaceExportRepositories(
	spaceReader adminSpaceExportSpaceReader,
	workspaceReader adminSpaceExportWorkspaceReader,
) AdminSpaceExportServiceOption {
	return func(s *AdminSpaceExportService) {
		s.spaceReader = spaceReader
		s.workspaceReader = workspaceReader
	}
}

// WithAdminSpaceExportAttachmentReader 注入附件与 blob 元数据读取仓储。
func WithAdminSpaceExportAttachmentReader(
	attachmentReader adminSpaceExportAttachmentReader,
) AdminSpaceExportServiceOption {
	return func(s *AdminSpaceExportService) {
		s.attachmentReader = attachmentReader
	}
}

// WithAdminSpaceExportBlobContentReader 覆盖 blob 内容读取器，主要用于测试或对象存储适配。
func WithAdminSpaceExportBlobContentReader(
	blobReader adminSpaceExportBlobContentReader,
) AdminSpaceExportServiceOption {
	return func(s *AdminSpaceExportService) {
		s.blobReader = blobReader
	}
}

// WithAdminSpaceExportImageHostingService 为远端附件/source 读取注入图床配置与签名 URL 能力。
func WithAdminSpaceExportImageHostingService(
	imageHostingService adminSpaceExportImageHostingService,
) AdminSpaceExportServiceOption {
	return func(s *AdminSpaceExportService) {
		reader, _ := s.blobReader.(adminSpaceExportDefaultBlobContentReader)
		reader.imageHostingService = imageHostingService
		if strings.TrimSpace(reader.localRootDir) == "" {
			reader.localRootDir = "uploads"
		}
		if reader.httpClient == nil {
			reader.httpClient = &http.Client{Timeout: 30 * time.Second}
		}
		s.blobReader = reader
	}
}

// WithAdminSpaceExportOfficeHTMLRenderer 注入 Office 源文件到 HTML 的渲染能力，供 EPUB 导出复用。
func WithAdminSpaceExportOfficeHTMLRenderer(
	renderer adminSpaceExportOfficeHTMLRenderer,
) AdminSpaceExportServiceOption {
	return func(s *AdminSpaceExportService) {
		s.officeHTMLRenderer = renderer
	}
}

// WithAdminSpaceExportReaderHTMLRenderer 注入阅读页 SSR 渲染器，供 EPUB Markdown 章节复用。
func WithAdminSpaceExportReaderHTMLRenderer(
	renderer adminSpaceExportReaderHTMLRenderer,
) AdminSpaceExportServiceOption {
	return func(s *AdminSpaceExportService) {
		s.readerHTMLRenderer = renderer
	}
}

// WithAdminSpaceExportAuditRecorder 注入后台审计记录器。
func WithAdminSpaceExportAuditRecorder(auditRecorder adminAuditRecorder) AdminSpaceExportServiceOption {
	return func(s *AdminSpaceExportService) {
		s.auditRecorder = auditRecorder
	}
}

// WithAdminSpaceExportDir 覆盖导出 zip 私有目录。
func WithAdminSpaceExportDir(exportDir string) AdminSpaceExportServiceOption {
	return func(s *AdminSpaceExportService) {
		if trimmed := strings.TrimSpace(exportDir); trimmed != "" {
			s.exportDir = trimmed
		}
	}
}

// AdminSpaceExportJobStore 是进程内导出任务表。
type AdminSpaceExportJobStore struct {
	mu          sync.Mutex
	jobs        map[string]*AdminSpaceExportJob
	subscribers map[string]map[chan AdminSpaceTransferEvent]struct{}
}

// NewAdminSpaceExportJobStore 创建导出任务表。
func NewAdminSpaceExportJobStore() *AdminSpaceExportJobStore {
	return &AdminSpaceExportJobStore{
		jobs:        make(map[string]*AdminSpaceExportJob),
		subscribers: make(map[string]map[chan AdminSpaceTransferEvent]struct{}),
	}
}

// AdminSpaceExportService 封装空间导出任务入口。
type AdminSpaceExportService struct {
	adminAccessService *AdminAccessService
	store              *AdminSpaceExportJobStore
	spaceReader        adminSpaceExportSpaceReader
	workspaceReader    adminSpaceExportWorkspaceReader
	attachmentReader   adminSpaceExportAttachmentReader
	blobReader         adminSpaceExportBlobContentReader
	officeHTMLRenderer adminSpaceExportOfficeHTMLRenderer
	readerHTMLRenderer adminSpaceExportReaderHTMLRenderer
	auditRecorder      adminAuditRecorder
	exportDir          string
	nowFn              func() time.Time
	canExportSpace     func(context.Context, string, string) (bool, error)
}

// NewAdminSpaceExportService 创建空间导出服务。
func NewAdminSpaceExportService(
	adminAccessService *AdminAccessService,
	options ...AdminSpaceExportServiceOption,
) *AdminSpaceExportService {
	svc := &AdminSpaceExportService{
		adminAccessService: adminAccessService,
		store:              NewAdminSpaceExportJobStore(),
		exportDir:          defaultAdminSpaceExportDir,
		nowFn:              time.Now,
	}
	svc.blobReader = adminSpaceExportDefaultBlobContentReader{
		localRootDir: "uploads",
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
	svc.canExportSpace = svc.defaultCanExportSpace
	for _, option := range options {
		if option != nil {
			option(svc)
		}
	}
	return svc
}

// StartExport 创建空间导出任务。真实文件生成在后续阶段补齐。
func (s *AdminSpaceExportService) StartExport(
	ctx context.Context,
	input StartAdminSpaceExportInput,
) (StartAdminSpaceExportResult, error) {
	if s == nil || s.store == nil {
		return StartAdminSpaceExportResult{}, errcode.ErrAdminForbidden
	}
	spaceID := strings.TrimSpace(input.SpaceID)
	if spaceID == "" {
		return StartAdminSpaceExportResult{}, errcode.ErrAdminSpaceExportSpaceIDRequired
	}
	if !isSupportedAdminSpaceExportFormat(input.Format) {
		return StartAdminSpaceExportResult{}, errcode.ErrAdminSpaceExportFormatUnsupported
	}
	if ok, err := s.CanExportSpace(ctx, input.ActorUserID, spaceID); err != nil {
		return StartAdminSpaceExportResult{}, err
	} else if !ok {
		return StartAdminSpaceExportResult{}, errcode.ErrAdminForbidden
	}

	jobID := strings.ToLower(ulid.Make().String())
	streamToken, streamTokenHash, tokenErr := generateAdminSpaceTransferToken()
	if tokenErr != nil {
		return StartAdminSpaceExportResult{}, tokenErr
	}
	now := s.now()
	includeAttachments, includeOfficeSources := normalizeAdminSpaceExportOptions(
		input.Format,
		input.IncludeAttachments,
		input.IncludeOfficeSources,
	)
	job := &AdminSpaceExportJob{
		JobID:                jobID,
		ActorUserID:          strings.TrimSpace(input.ActorUserID),
		SpaceID:              spaceID,
		Format:               input.Format,
		IncludeAttachments:   includeAttachments,
		IncludeOfficeSources: includeOfficeSources,
		Status:               AdminSpaceExportStatusQueued,
		StreamTokenHash:      streamTokenHash,
		StreamTokenExpiresAt: now.Add(defaultAdminSpaceTransferTokenTTL),
		LastEvent: AdminSpaceTransferEvent{
			Type:     AdminSpaceTransferEventTypeProgress,
			Stage:    "queued",
			Progress: 0,
			Message:  "导出任务已创建",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.Create(job); err != nil {
		return StartAdminSpaceExportResult{}, err
	}
	if err := s.recordExportAudit(ctx, *job, adminSpaceExportAuditQueued, "queued", "", "", 0); err != nil {
		s.store.Fail(jobID, "audit", "记录导出审计失败", s.now())
		return StartAdminSpaceExportResult{}, err
	}
	if s.canGenerateAdminSpaceExportPackage() {
		go s.runAdminSpaceExportJob(context.WithoutCancel(ctx), jobID)
	}

	return StartAdminSpaceExportResult{
		JobID:     jobID,
		StreamURL: "/api/admin/spaces/" + spaceID + "/exports/" + jobID + "/events?token=" + streamToken,
	}, nil
}

func normalizeAdminSpaceExportOptions(
	format AdminSpaceExportFormat,
	includeAttachments bool,
	includeOfficeSources bool,
) (bool, bool) {
	switch format {
	case AdminSpaceExportFormatSourceZip:
		return true, true
	case AdminSpaceExportFormatEPUB:
		return false, true
	default:
		return includeAttachments, includeOfficeSources
	}
}

func (s *AdminSpaceExportService) canGenerateAdminSpaceExportPackage() bool {
	return s != nil && s.spaceReader != nil && s.workspaceReader != nil && strings.TrimSpace(s.exportDir) != ""
}

// CanExportSpace 判断 actor 是否可导出目标空间。
func (s *AdminSpaceExportService) CanExportSpace(ctx context.Context, actorUserID string, spaceID string) (bool, error) {
	if s == nil {
		return false, nil
	}
	if s.canExportSpace != nil {
		return s.canExportSpace(ctx, strings.TrimSpace(actorUserID), strings.TrimSpace(spaceID))
	}
	return s.defaultCanExportSpace(ctx, actorUserID, spaceID)
}

func (s *AdminSpaceExportService) defaultCanExportSpace(ctx context.Context, actorUserID string, spaceID string) (bool, error) {
	if s == nil || s.adminAccessService == nil {
		return false, nil
	}
	actorUserID = strings.TrimSpace(actorUserID)
	spaceID = strings.TrimSpace(spaceID)
	canManage, err := s.adminAccessService.CanManageSpace(ctx, actorUserID, spaceID)
	if err != nil || canManage {
		return canManage, err
	}
	return s.canExportOwnedSpace(ctx, actorUserID, spaceID)
}

func (s *AdminSpaceExportService) canExportOwnedSpace(ctx context.Context, actorUserID string, spaceID string) (bool, error) {
	if s == nil || s.spaceReader == nil || actorUserID == "" || spaceID == "" {
		return false, nil
	}
	space, err := s.spaceReader.GetBySpaceID(ctx, spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("读取空间归属: %w", err)
	}
	if space == nil || space.DeletedAt != nil || space.Status == models.EntityStatusDeleted {
		return false, nil
	}
	return strings.TrimSpace(space.OwnerUserID) == actorUserID, nil
}

// Subscribe 校验 streamToken 并订阅导出任务事件。
func (s *AdminSpaceExportService) Subscribe(
	_ context.Context,
	jobID string,
	actorUserID string,
	streamToken string,
) (AdminSpaceTransferEvent, <-chan AdminSpaceTransferEvent, func(), error) {
	if s == nil || s.store == nil {
		return AdminSpaceTransferEvent{}, nil, func() {}, errcode.ErrAdminSpaceExportJobNotFound
	}
	return s.store.Subscribe(strings.TrimSpace(jobID), strings.TrimSpace(actorUserID), strings.TrimSpace(streamToken), s.now())
}

// PublishProgress 广播导出任务进度。
func (s *AdminSpaceExportService) PublishProgress(jobID string, event AdminSpaceTransferEvent) {
	if s == nil || s.store == nil {
		return
	}
	s.store.Publish(strings.TrimSpace(jobID), event, s.now())
}

// BeginExportJob 将导出任务切到 running；真实 worker 在后续阶段调用。
func (s *AdminSpaceExportService) BeginExportJob(ctx context.Context, jobID string) error {
	if s == nil || s.store == nil {
		return errcode.ErrAdminSpaceExportJobNotFound
	}
	job, err := s.store.Get(strings.TrimSpace(jobID))
	if err != nil {
		return err
	}
	if ok, err := s.CanExportSpace(ctx, job.ActorUserID, job.SpaceID); err != nil {
		return err
	} else if !ok {
		s.store.Fail(job.JobID, "permission", "导出权限已失效", s.now())
		return errcode.ErrAdminForbidden
	}
	return s.store.MarkRunning(job.JobID, s.now())
}

// CompleteExportJob 标记导出任务完成并一次性生成下载 token。
func (s *AdminSpaceExportService) CompleteExportJob(jobID string, fileName string, sizeBytes int64) (AdminSpaceTransferEvent, error) {
	if s == nil || s.store == nil {
		return AdminSpaceTransferEvent{}, errcode.ErrAdminSpaceExportJobNotFound
	}
	downloadToken, downloadTokenHash, err := generateAdminSpaceTransferToken()
	if err != nil {
		return AdminSpaceTransferEvent{}, err
	}
	event, err := s.store.Complete(strings.TrimSpace(jobID), strings.TrimSpace(fileName), "", sizeBytes, downloadToken, downloadTokenHash, s.now())
	if err != nil {
		return AdminSpaceTransferEvent{}, err
	}
	if job, getErr := s.store.Get(strings.TrimSpace(jobID)); getErr == nil {
		_ = s.recordExportAudit(context.Background(), job, adminSpaceExportAuditSuccess, "completed", "", strings.TrimSpace(fileName), sizeBytes)
	}
	return event, nil
}

// ConsumeDownloadToken 校验并消费导出下载 token。
func (s *AdminSpaceExportService) ConsumeDownloadToken(
	jobID string,
	actorUserID string,
	token string,
) (AdminSpaceExportDownload, error) {
	if s == nil || s.store == nil {
		return AdminSpaceExportDownload{}, errcode.ErrAdminSpaceExportJobNotFound
	}
	download, err := s.store.ConsumeDownloadToken(strings.TrimSpace(jobID), strings.TrimSpace(actorUserID), strings.TrimSpace(token), s.now())
	if err != nil {
		return AdminSpaceExportDownload{}, err
	}
	validatedPath, err := s.validateDownloadFilePath(download.FilePath)
	if err != nil {
		return AdminSpaceExportDownload{}, err
	}
	download.FilePath = validatedPath
	return download, nil
}

func (s *AdminSpaceExportService) validateDownloadFilePath(filePath string) (string, error) {
	normalizedFilePath := strings.TrimSpace(filePath)
	if normalizedFilePath == "" {
		return "", errcode.ErrAdminSpaceExportFileNotReady
	}
	extension := strings.ToLower(filepath.Ext(normalizedFilePath))
	if extension != ".zip" && extension != ".plaindoc" && extension != ".epub" {
		return "", errcode.ErrAdminSpaceExportDownloadForbidden
	}
	exportDir := defaultAdminSpaceExportDir
	if s != nil && strings.TrimSpace(s.exportDir) != "" {
		exportDir = strings.TrimSpace(s.exportDir)
	}
	rootAbsPath, err := filepath.Abs(filepath.Clean(exportDir))
	if err != nil {
		return "", err
	}
	targetAbsPath, err := filepath.Abs(normalizedFilePath)
	if err != nil {
		return "", err
	}
	if !isAdminSpaceExportPathWithinRoot(rootAbsPath, targetAbsPath) {
		return "", errcode.ErrAdminSpaceExportDownloadForbidden
	}
	return targetAbsPath, nil
}

func (s *AdminSpaceExportService) runAdminSpaceExportJob(ctx context.Context, jobID string) {
	if err := s.BeginExportJob(ctx, jobID); err != nil {
		if job, getErr := s.store.Get(strings.TrimSpace(jobID)); getErr == nil {
			_ = s.recordExportAudit(ctx, job, adminSpaceExportAuditFailed, "permission", err.Error(), "", 0)
		}
		return
	}

	job, err := s.store.Get(strings.TrimSpace(jobID))
	if err != nil {
		s.store.Fail(jobID, "load", "导出任务不存在", s.now())
		return
	}
	fileName, filePath, sizeBytes, err := s.exportAdminSpaceZipPackage(ctx, job)
	if err != nil {
		s.store.Fail(jobID, "zip", err.Error(), s.now())
		_ = s.recordExportAudit(ctx, job, adminSpaceExportAuditFailed, "zip", err.Error(), "", 0)
		return
	}

	downloadToken, downloadTokenHash, err := generateAdminSpaceTransferToken()
	if err != nil {
		s.store.Fail(jobID, "token", "生成下载令牌失败", s.now())
		_ = s.recordExportAudit(ctx, job, adminSpaceExportAuditFailed, "token", "生成下载令牌失败", "", 0)
		return
	}
	_, _ = s.store.Complete(
		strings.TrimSpace(jobID),
		fileName,
		filePath,
		sizeBytes,
		downloadToken,
		downloadTokenHash,
		s.now(),
	)
	_ = s.recordExportAudit(ctx, job, adminSpaceExportAuditSuccess, "completed", "", fileName, sizeBytes)
}

const (
	adminSpaceExportAuditQueued  = "queued"
	adminSpaceExportAuditSuccess = "success"
	adminSpaceExportAuditFailed  = "failed"
)

func (s *AdminSpaceExportService) recordExportAudit(
	ctx context.Context,
	job AdminSpaceExportJob,
	status string,
	stage string,
	message string,
	fileName string,
	sizeBytes int64,
) error {
	if s == nil || s.auditRecorder == nil {
		return nil
	}
	targetID := strings.TrimSpace(job.SpaceID)
	if targetID == "" {
		return nil
	}
	detail := map[string]any{
		"jobId":                strings.TrimSpace(job.JobID),
		"spaceId":              targetID,
		"format":               string(job.Format),
		"includeAttachments":   job.IncludeAttachments,
		"includeOfficeSources": job.IncludeOfficeSources,
		"status":               strings.TrimSpace(status),
		"stage":                strings.TrimSpace(stage),
		"abilityType":          "space_manage",
	}
	if trimmed := sanitizeAdminSpaceTransferAuditMessage(message, s.exportDir, defaultAdminSpaceExportDir); trimmed != "" {
		detail["error"] = trimmed
	}
	if trimmed := strings.TrimSpace(fileName); trimmed != "" {
		detail["fileName"] = trimmed
	}
	if sizeBytes > 0 {
		detail["sizeBytes"] = sizeBytes
	}
	return s.auditRecorder.Record(ctx, RecordAdminAuditInput{
		ActorUserID: strings.TrimSpace(job.ActorUserID),
		Module:      AdminAuditModuleSpace,
		Action:      AdminAuditActionExport,
		TargetType:  "space",
		TargetID:    targetID,
		Summary:     "space export " + strings.TrimSpace(status) + ": " + targetID,
		Detail:      detail,
	})
}

const adminSpaceTransferAuditHiddenError = "任务失败，详情见服务端日志"

func sanitizeAdminSpaceTransferAuditMessage(message string, privateHints ...string) string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return ""
	}
	normalized := strings.ToLower(trimmed)
	if strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "downloadurl") ||
		strings.Contains(normalized, "streamurl") {
		return adminSpaceTransferAuditHiddenError
	}
	for _, hint := range privateHints {
		if auditMessageContainsPath(trimmed, hint) {
			return adminSpaceTransferAuditHiddenError
		}
	}
	if auditMessageContainsAbsolutePath(trimmed) {
		return adminSpaceTransferAuditHiddenError
	}
	return trimmed
}

func auditMessageContainsPath(message string, pathHint string) bool {
	trimmedHint := strings.TrimSpace(pathHint)
	if trimmedHint == "" {
		return false
	}
	candidates := []string{trimmedHint, filepath.Clean(trimmedHint)}
	if absoluteHint, err := filepath.Abs(trimmedHint); err == nil {
		candidates = append(candidates, absoluteHint)
	}
	normalizedMessage := filepath.ToSlash(message)
	for _, candidate := range candidates {
		normalizedCandidate := filepath.ToSlash(strings.TrimSpace(candidate))
		if normalizedCandidate == "" || normalizedCandidate == "." {
			continue
		}
		if strings.Contains(normalizedMessage, normalizedCandidate) {
			return true
		}
	}
	return false
}

func auditMessageContainsAbsolutePath(message string) bool {
	fields := strings.FieldsFunc(message, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', '"', '\'', '`', '(', ')', '[', ']', '{', '}', '<', '>', ',':
			return true
		default:
			return false
		}
	})
	for _, field := range fields {
		token := strings.Trim(field, ".;:")
		if token == "" {
			continue
		}
		if filepath.IsAbs(token) {
			return true
		}
		if len(token) >= 3 && token[1] == ':' && (token[2] == '\\' || token[2] == '/') {
			return true
		}
	}
	return false
}

func isSupportedAdminSpaceExportFormat(format AdminSpaceExportFormat) bool {
	switch format {
	case AdminSpaceExportFormatMarkdownZip, AdminSpaceExportFormatSourceZip, AdminSpaceExportFormatEPUB:
		return true
	default:
		return false
	}
}

func (s *AdminSpaceExportService) exportAdminSpaceZipPackage(
	ctx context.Context,
	job AdminSpaceExportJob,
) (string, string, int64, error) {
	if s == nil || s.spaceReader == nil || s.workspaceReader == nil {
		return "", "", 0, fmt.Errorf("导出服务依赖未配置")
	}
	if job.Format == AdminSpaceExportFormatEPUB {
		return s.exportAdminSpaceEPUBPackage(ctx, job)
	}

	s.PublishProgress(job.JobID, AdminSpaceTransferEvent{
		Type:     AdminSpaceTransferEventTypeProgress,
		Stage:    "metadata",
		Progress: 5,
		Message:  "正在读取空间元数据",
	})

	space, err := s.spaceReader.GetBySpaceID(ctx, job.SpaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", 0, errcode.ErrAdminForbidden
		}
		return "", "", 0, err
	}
	if space == nil || space.DeletedAt != nil || space.Status == models.EntityStatusDeleted {
		return "", "", 0, errcode.ErrAdminForbidden
	}

	rows, err := s.workspaceReader.ListTreeNodesBySpaceID(ctx, job.SpaceID)
	if err != nil {
		return "", "", 0, err
	}

	s.PublishProgress(job.JobID, AdminSpaceTransferEvent{
		Type:     AdminSpaceTransferEventTypeProgress,
		Stage:    "tree",
		Progress: 20,
		Message:  "正在构建目录树",
	})

	exportedAt := s.now()
	pkg, err := s.buildAdminSpaceExportPackage(ctx, job, *space, rows, exportedAt)
	if err != nil {
		return "", "", 0, err
	}

	fileName := buildAdminSpaceExportFileName(job.SpaceID, exportedAt, job.Format)
	exportDir := filepath.Clean(s.exportDir)
	if err := os.MkdirAll(exportDir, 0o700); err != nil {
		return "", "", 0, err
	}
	partPath := filepath.Join(exportDir, job.JobID+".part")
	finalPath := filepath.Join(exportDir, job.JobID+adminSpaceExportFileExtension(job.Format))
	if err := writeAdminSpaceExportZip(partPath, pkg); err != nil {
		_ = os.Remove(partPath)
		return "", "", 0, err
	}
	if err := os.Rename(partPath, finalPath); err != nil {
		_ = os.Remove(partPath)
		return "", "", 0, err
	}
	info, err := os.Stat(finalPath)
	if err != nil {
		return "", "", 0, err
	}
	return fileName, finalPath, info.Size(), nil
}

type adminSpaceExportPackage struct {
	RootEntryPrefix string
	Manifest        AdminSpaceExportManifest
	Tree            AdminSpaceExportTree
	Files           map[string][]byte
}

func (s *AdminSpaceExportService) buildAdminSpaceExportPackage(
	ctx context.Context,
	job AdminSpaceExportJob,
	space models.Space,
	rows []repository.WorkspaceTreeNodeRecord,
	exportedAt time.Time,
) (adminSpaceExportPackage, error) {
	sortedRows := append([]repository.WorkspaceTreeNodeRecord(nil), rows...)
	sort.SliceStable(sortedRows, func(i int, j int) bool {
		return compareAdminSpaceExportTreeRows(sortedRows[i], sortedRows[j]) < 0
	})

	nodes := make(map[string]repository.WorkspaceTreeNodeRecord, len(sortedRows))
	children := make(map[string][]repository.WorkspaceTreeNodeRecord)
	for _, row := range sortedRows {
		nodeID := strings.TrimSpace(row.NodeID)
		if nodeID == "" {
			continue
		}
		nodes[nodeID] = row
		parentKey := ""
		if row.ParentNodeID != nil {
			parentKey = strings.TrimSpace(*row.ParentNodeID)
		}
		children[parentKey] = append(children[parentKey], row)
	}
	for parentKey := range children {
		sort.SliceStable(children[parentKey], func(i int, j int) bool {
			return compareAdminSpaceExportTreeRows(children[parentKey][i], children[parentKey][j]) < 0
		})
	}

	files := make(map[string][]byte)
	documents := make([]AdminSpaceExportDocumentEntry, 0)
	summary := AdminSpaceExportSummary{}

	var walk func(parentKey string, parentPath string) ([]AdminSpaceExportTreeNode, error)
	walk = func(parentKey string, parentPath string) ([]AdminSpaceExportTreeNode, error) {
		usedNames := make(map[string]int)
		treeNodes := make([]AdminSpaceExportTreeNode, 0, len(children[parentKey]))
		for _, row := range children[parentKey] {
			treeNode := adminSpaceExportTreeNodeFromRecord(row)
			switch row.Type {
			case models.NodeTypeFolder:
				summary.FolderCount++
				folderName := uniqueAdminSpaceExportName(usedNames, row.Title, "folder-"+row.NodeID, "")
				childPath := path.Join(parentPath, folderName)
				childTreeNodes, err := walk(strings.TrimSpace(row.NodeID), childPath)
				if err != nil {
					return nil, err
				}
				treeNode.Children = childTreeNodes
			case models.NodeTypeDoc:
				summary.DocumentCount++
				entry, documentFiles, err := s.buildAdminSpaceExportDocumentEntry(ctx, job, row, parentPath, usedNames)
				if err != nil {
					return nil, err
				}
				documents = append(documents, entry)
				for filePath, content := range documentFiles {
					files[filePath] = content
				}
				summary.AttachmentCount += len(entry.Attachments)
				if entry.Source != nil && entry.Source.Included {
					summary.OfficeSourceCount++
				}
				childPath := path.Join(parentPath, uniqueAdminSpaceExportName(usedNames, row.Title, "document-"+row.NodeID, ""))
				childTreeNodes, err := walk(strings.TrimSpace(row.NodeID), childPath)
				if err != nil {
					return nil, err
				}
				treeNode.Children = childTreeNodes
			default:
				continue
			}
			treeNodes = append(treeNodes, treeNode)
		}
		return treeNodes, nil
	}

	root, err := walk("", "")
	if err != nil {
		return adminSpaceExportPackage{}, err
	}

	coverEntry, coverContent, err := s.buildAdminSpaceExportCoverEntry(ctx, space)
	if err != nil {
		return adminSpaceExportPackage{}, err
	}
	if coverEntry != nil {
		files[coverEntry.Path] = coverContent
	}

	return adminSpaceExportPackage{
		RootEntryPrefix: "space-" + sanitizeAdminSpaceExportPathSegment(space.SpaceID, "space"),
		Manifest: AdminSpaceExportManifest{
			Version:     AdminSpaceExportPackageVersion,
			PackageType: AdminSpaceExportPackageType,
			ExportedAt:  exportedAt.Format(time.RFC3339),
			Format:      job.Format,
			Importable:  isAdminSpaceExportPackageImportable(job),
			Space: AdminSpaceExportManifestSpace{
				SpaceID:     strings.TrimSpace(space.SpaceID),
				Name:        strings.TrimSpace(space.Name),
				Description: strings.TrimSpace(space.Description),
				CategoryID:  strings.TrimSpace(space.CategoryID),
				Visibility:  string(space.Visibility),
				Cover:       coverEntry,
			},
			Summary:   summary,
			Documents: documents,
		},
		Tree: AdminSpaceExportTree{
			Version: AdminSpaceExportPackageVersion,
			Root:    root,
		},
		Files: files,
	}, nil
}

func (s *AdminSpaceExportService) buildAdminSpaceExportCoverEntry(
	ctx context.Context,
	space models.Space,
) (*AdminSpaceExportCoverEntry, []byte, error) {
	coverAssetID := ""
	if space.CoverAssetID != nil {
		coverAssetID = strings.TrimSpace(*space.CoverAssetID)
	}
	if coverAssetID == "" && strings.TrimSpace(space.CoverKey) == "" && strings.TrimSpace(space.CoverURL) == "" {
		return nil, nil, nil
	}

	asset := adminSpaceExportCoverAssetFromSpace(space, coverAssetID)
	if coverAssetID != "" {
		if reader, ok := s.spaceReader.(adminSpaceExportCoverReader); ok {
			storedAsset, err := reader.GetCoverAssetByAssetID(ctx, coverAssetID)
			if err != nil {
				return nil, nil, err
			}
			if storedAsset != nil {
				asset = *storedAsset
			}
		}
	}
	objectKey := strings.TrimSpace(asset.ObjectKey)
	if objectKey == "" {
		return nil, nil, fmt.Errorf("空间封面 object key 为空: %s", space.SpaceID)
	}
	fileName := sanitizeAdminSpaceExportPathSegment(path.Base(objectKey), "space-cover.webp")
	if fileName == "" || fileName == "." {
		fileName = "space-cover.webp"
	}
	if !strings.Contains(fileName, ".") {
		fileName += ".webp"
	}
	blobID := strings.TrimSpace(asset.AssetID)
	if blobID == "" {
		blobID = coverAssetID
	}
	if blobID == "" {
		blobID = strings.TrimSpace(space.SpaceID) + "-cover"
	}
	mimeType := strings.TrimSpace(asset.MimeType)
	if mimeType == "" {
		mimeType = "image/webp"
	}
	content, err := s.blobReader.ReadBlobContent(ctx, models.DocumentAttachmentBlob{
		BlobID:          blobID,
		StorageProvider: string(ImageHostingProviderLocal),
		ObjectKey:       objectKey,
		MimeType:        mimeType,
		SizeBytes:       asset.SizeBytes,
	}, fileName)
	if err != nil {
		return nil, nil, fmt.Errorf("读取空间封面失败: %w", err)
	}
	if len(content) == 0 {
		return nil, nil, fmt.Errorf("空间封面内容为空: %s", space.SpaceID)
	}
	sum := sha256.Sum256(content)
	entry := &AdminSpaceExportCoverEntry{
		AssetID:    strings.TrimSpace(asset.AssetID),
		Path:       safeAdminSpaceExportZipEntry("covers", fileName),
		FileName:   fileName,
		MimeType:   mimeType,
		SizeBytes:  int64(len(content)),
		Width:      asset.Width,
		Height:     asset.Height,
		Source:     strings.TrimSpace(asset.Source),
		Normalized: asset.Normalized,
		SHA256:     hex.EncodeToString(sum[:]),
	}
	if entry.Source == "" {
		entry.Source = strings.TrimSpace(space.CoverSource)
	}
	return entry, content, nil
}

func adminSpaceExportCoverAssetFromSpace(space models.Space, coverAssetID string) models.SpaceCoverAsset {
	return models.SpaceCoverAsset{
		AssetID:   strings.TrimSpace(coverAssetID),
		Source:    strings.TrimSpace(space.CoverSource),
		ObjectKey: strings.TrimSpace(space.CoverKey),
		ObjectURL: strings.TrimSpace(space.CoverURL),
		MimeType:  "image/webp",
		Width:     space.CoverWidth,
		Height:    space.CoverHeight,
		SizeBytes: 0,
	}
}

func (s *AdminSpaceExportService) buildAdminSpaceExportDocumentEntry(
	ctx context.Context,
	job AdminSpaceExportJob,
	row repository.WorkspaceTreeNodeRecord,
	parentPath string,
	usedNames map[string]int,
) (AdminSpaceExportDocumentEntry, map[string][]byte, error) {
	documentID := ""
	if row.DocumentID != nil {
		documentID = strings.TrimSpace(*row.DocumentID)
	}
	if documentID == "" {
		return AdminSpaceExportDocumentEntry{}, nil, fmt.Errorf("文档节点 %s 缺少 documentId", row.NodeID)
	}

	document, err := s.workspaceReader.GetDocumentByDocumentID(ctx, documentID)
	if err != nil {
		return AdminSpaceExportDocumentEntry{}, nil, err
	}
	if document == nil || strings.TrimSpace(document.SpaceID) != strings.TrimSpace(row.SpaceID) {
		return AdminSpaceExportDocumentEntry{}, nil, fmt.Errorf("文档 %s 不属于空间 %s", documentID, row.SpaceID)
	}

	format := document.Format
	if row.DocumentFormat != nil {
		format = *row.DocumentFormat
	}
	format = models.NormalizeDocumentFormat(format)
	visibility := string(models.VisibilityMember)
	if row.DocumentVisibility != nil && strings.TrimSpace(*row.DocumentVisibility) != "" {
		visibility = strings.TrimSpace(*row.DocumentVisibility)
	}

	title := strings.TrimSpace(row.Title)
	if title == "" {
		title = strings.TrimSpace(document.Title)
	}
	entry := AdminSpaceExportDocumentEntry{
		DocumentID:  documentID,
		NodeID:      strings.TrimSpace(row.NodeID),
		Title:       title,
		Format:      string(format),
		Sort:        row.Sort,
		Visibility:  visibility,
		Attachments: []string{},
		Source:      nil,
	}
	if row.ParentNodeID != nil {
		entry.ParentNodeID = strings.TrimSpace(*row.ParentNodeID)
	}

	files := make(map[string][]byte)
	switch format {
	case models.DocumentFormatMarkdown:
		fileName := uniqueAdminSpaceExportName(usedNames, title, "document-"+row.NodeID, ".md")
		entry.Path = safeAdminSpaceExportZipEntry("documents", parentPath, fileName)
		content := []byte(document.ContentMD)
		sum := sha256.Sum256(content)
		entry.ContentSHA256 = hex.EncodeToString(sum[:])
		files[entry.Path] = content
	case models.DocumentFormatDOCX, models.DocumentFormatXLSX:
		sourceTitle := title
		if document.SourceFileName != nil && strings.TrimSpace(*document.SourceFileName) != "" {
			sourceTitle = strings.TrimSpace(*document.SourceFileName)
		}
		sourceFileName := uniqueAdminSpaceExportName(usedNames, sourceTitle, "source-"+row.NodeID, "."+string(format))
		sourcePath := safeAdminSpaceExportZipEntry("sources", documentID, sourceFileName)
		entry.Source = &AdminSpaceExportSourceEntry{Path: sourcePath, Included: false}
		if shouldIncludeAdminSpaceExportOfficeSource(job) {
			entry.Path = sourcePath
			sourceContent, err := s.readAdminSpaceExportDocumentSource(ctx, *document, sourcePath)
			if err != nil {
				return AdminSpaceExportDocumentEntry{}, nil, err
			}
			sum := sha256.Sum256(sourceContent)
			entry.Source.Included = true
			entry.Source.SHA256 = hex.EncodeToString(sum[:])
			files[sourcePath] = sourceContent
		}
	default:
		return AdminSpaceExportDocumentEntry{}, nil, errcode.ErrAdminSpaceExportFormatUnsupported
	}

	if job.IncludeAttachments {
		if err := s.appendAdminSpaceExportAttachments(ctx, documentID, &entry, files); err != nil {
			return AdminSpaceExportDocumentEntry{}, nil, err
		}
	}
	return entry, files, nil
}

func isAdminSpaceExportPackageImportable(job AdminSpaceExportJob) bool {
	return job.Format == AdminSpaceExportFormatSourceZip &&
		job.IncludeAttachments &&
		job.IncludeOfficeSources
}

func shouldIncludeAdminSpaceExportOfficeSource(job AdminSpaceExportJob) bool {
	return job.IncludeOfficeSources &&
		(job.Format == AdminSpaceExportFormatSourceZip || job.Format == AdminSpaceExportFormatEPUB)
}

func (s *AdminSpaceExportService) appendAdminSpaceExportAttachments(
	ctx context.Context,
	documentID string,
	entry *AdminSpaceExportDocumentEntry,
	files map[string][]byte,
) error {
	if s == nil || s.attachmentReader == nil || s.blobReader == nil {
		return fmt.Errorf("附件导出依赖未配置")
	}
	attachments, err := s.attachmentReader.ListByDocumentID(ctx, documentID, false)
	if err != nil {
		return err
	}
	usedNames := make(map[string]int)
	for _, attachment := range attachments {
		if attachment.Status == models.EntityStatusDeleted || attachment.DeletedAt != nil {
			continue
		}
		blobID := strings.TrimSpace(attachment.BlobID)
		if blobID == "" {
			return fmt.Errorf("附件 %s 缺少 blobId", attachment.AttachmentID)
		}
		blob, err := s.attachmentReader.GetBlobByBlobID(ctx, blobID)
		if err != nil {
			return err
		}
		if blob == nil || blob.DeletedAt != nil {
			return fmt.Errorf("附件 %s 的 blob 不存在", attachment.AttachmentID)
		}
		payload, err := s.blobReader.ReadBlobContent(ctx, *blob, attachment.FileName)
		if err != nil {
			return err
		}
		fileName := uniqueAdminSpaceExportName(usedNames, attachment.FileName, "attachment-"+attachment.AttachmentID, "")
		attachmentPath := safeAdminSpaceExportZipEntry("attachments", documentID, fileName)
		files[attachmentPath] = payload
		sum := sha256.Sum256(payload)
		entry.Attachments = append(entry.Attachments, attachmentPath)
		entry.AttachmentEntries = append(entry.AttachmentEntries, AdminSpaceExportAttachmentEntry{
			AttachmentID: strings.TrimSpace(attachment.AttachmentID),
			Path:         attachmentPath,
			FileName:     strings.TrimSpace(attachment.FileName),
			MimeType:     strings.TrimSpace(attachment.MimeType),
			SizeBytes:    attachment.SizeBytes,
			SHA256:       hex.EncodeToString(sum[:]),
		})
	}
	return nil
}

func (s *AdminSpaceExportService) readAdminSpaceExportDocumentSource(
	ctx context.Context,
	document repository.WorkspaceDocumentRecord,
	sourcePath string,
) ([]byte, error) {
	if s == nil || s.attachmentReader == nil || s.blobReader == nil {
		return nil, fmt.Errorf("Office source 导出依赖未配置")
	}
	if document.SourceBlobID == nil || strings.TrimSpace(*document.SourceBlobID) == "" {
		return nil, fmt.Errorf("Office 文档 %s 缺少 source blob", document.DocumentID)
	}
	blob, err := s.attachmentReader.GetBlobByBlobID(ctx, strings.TrimSpace(*document.SourceBlobID))
	if err != nil {
		return nil, err
	}
	if blob == nil || blob.DeletedAt != nil {
		return nil, fmt.Errorf("Office 文档 %s 的 source blob 不存在", document.DocumentID)
	}
	fileName := path.Base(sourcePath)
	if document.SourceFileName != nil && strings.TrimSpace(*document.SourceFileName) != "" {
		fileName = strings.TrimSpace(*document.SourceFileName)
	}
	return s.blobReader.ReadBlobContent(ctx, *blob, fileName)
}

func adminSpaceExportTreeNodeFromRecord(row repository.WorkspaceTreeNodeRecord) AdminSpaceExportTreeNode {
	node := AdminSpaceExportTreeNode{
		NodeID:       strings.TrimSpace(row.NodeID),
		ParentNodeID: nil,
		Type:         string(row.Type),
		Title:        strings.TrimSpace(row.Title),
		Sort:         row.Sort,
	}
	if row.ParentNodeID != nil {
		parentNodeID := strings.TrimSpace(*row.ParentNodeID)
		node.ParentNodeID = &parentNodeID
	}
	if row.DocumentID != nil {
		node.DocumentID = strings.TrimSpace(*row.DocumentID)
	}
	if row.DocumentFormat != nil {
		node.Format = string(models.NormalizeDocumentFormat(*row.DocumentFormat))
	}
	return node
}

func compareAdminSpaceExportTreeRows(a repository.WorkspaceTreeNodeRecord, b repository.WorkspaceTreeNodeRecord) int {
	if a.Sort != b.Sort {
		if a.Sort < b.Sort {
			return -1
		}
		return 1
	}
	if result := strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title)); result != 0 {
		return result
	}
	return strings.Compare(a.NodeID, b.NodeID)
}

func uniqueAdminSpaceExportName(usedNames map[string]int, title string, fallback string, extension string) string {
	base := sanitizeAdminSpaceExportPathSegment(title, fallback)
	ext := strings.TrimSpace(extension)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	if ext != "" && strings.EqualFold(path.Ext(base), ext) {
		base = strings.TrimSuffix(base, path.Ext(base))
	}
	candidate := base + ext
	key := strings.ToLower(candidate)
	count := usedNames[key]
	usedNames[key] = count + 1
	if count == 0 {
		return candidate
	}
	for {
		candidate = fmt.Sprintf("%s (%d)%s", base, count, ext)
		key = strings.ToLower(candidate)
		if usedNames[key] == 0 {
			usedNames[key] = 1
			return candidate
		}
		count++
	}
}

func sanitizeAdminSpaceExportPathSegment(value string, fallback string) string {
	var builder strings.Builder
	for _, r := range strings.TrimSpace(value) {
		switch {
		case r == '/' || r == '\\' || r == '<' || r == '>' || r == ':' || r == '"' || r == '|' || r == '?' || r == '*':
			builder.WriteRune('-')
		case unicode.IsControl(r):
			continue
		default:
			builder.WriteRune(r)
		}
	}
	result := strings.Trim(builder.String(), " .")
	result = strings.TrimLeft(result, ".")
	if result == "" || result == "." || result == ".." {
		result = strings.TrimSpace(fallback)
	}
	if result == "" {
		result = "untitled"
	}
	return result
}

func safeAdminSpaceExportZipEntry(parts ...string) string {
	cleanParts := make([]string, 0, len(parts))
	for index, part := range parts {
		part = strings.TrimSpace(strings.ReplaceAll(part, "\\", "/"))
		if part == "" {
			continue
		}
		for _, segment := range strings.Split(part, "/") {
			segment = sanitizeAdminSpaceExportPathSegment(segment, fmt.Sprintf("entry-%d", index))
			if segment != "" {
				cleanParts = append(cleanParts, segment)
			}
		}
	}
	entry := path.Clean(path.Join(cleanParts...))
	if entry == "." || path.IsAbs(entry) || strings.HasPrefix(entry, "../") || strings.Contains(entry, "/../") {
		return "entry"
	}
	return entry
}

type adminSpaceExportDefaultBlobContentReader struct {
	localRootDir         string
	httpClient           *http.Client
	imageHostingService  adminSpaceExportImageHostingService
	maxBlobReadSizeBytes int64
}

func (r adminSpaceExportDefaultBlobContentReader) ReadBlobContent(
	ctx context.Context,
	blob models.DocumentAttachmentBlob,
	fallbackFileName string,
) ([]byte, error) {
	provider := ImageHostingProvider(strings.ToLower(strings.TrimSpace(blob.StorageProvider)))
	if provider == "" {
		provider = ImageHostingProviderLocal
	}
	switch provider {
	case ImageHostingProviderLocal:
		targetPath, err := resolveAdminSpaceExportLocalBlobPath(r.localRootDir, blob.ObjectKey)
		if err != nil {
			return nil, err
		}
		file, err := os.Open(targetPath)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return readAdminSpaceExportLimitedBlob(file, r.readLimit(blob.SizeBytes))
	case ImageHostingProviderCloudflareR2, ImageHostingProviderAliyunOSS:
		return r.readRemoteBlobContent(ctx, provider, blob, fallbackFileName)
	default:
		return nil, fmt.Errorf("不支持的附件存储 provider: %s", provider)
	}
}

func (r adminSpaceExportDefaultBlobContentReader) readRemoteBlobContent(
	ctx context.Context,
	provider ImageHostingProvider,
	blob models.DocumentAttachmentBlob,
	fallbackFileName string,
) ([]byte, error) {
	if r.imageHostingService == nil {
		return nil, fmt.Errorf("远端附件读取依赖未配置")
	}
	config, err := r.imageHostingService.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	rawURL, err := r.imageHostingService.BuildObjectReadURL(ctx, config, BuildImageHostingObjectReadURLInput{
		Provider:  provider,
		ObjectKey: strings.TrimSpace(blob.ObjectKey),
		ObjectURL: "",
		FileName:  strings.TrimSpace(fallbackFileName),
		Purpose:   DocumentAttachmentLinkPurposeDownload,
	})
	if err != nil {
		return nil, err
	}
	if rawURL == "" {
		return nil, fmt.Errorf("远端附件 %s 缺少可读取 URL", strings.TrimSpace(fallbackFileName))
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("远端附件 URL scheme 不支持: %s", parsedURL.Scheme)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, err
	}
	client := r.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("读取远端附件失败: status=%d", response.StatusCode)
	}
	return readAdminSpaceExportLimitedBlob(response.Body, r.readLimit(blob.SizeBytes))
}

func (r adminSpaceExportDefaultBlobContentReader) readLimit(blobSizeBytes int64) int64 {
	limit := r.maxBlobReadSizeBytes
	if limit <= 0 {
		limit = maxAdminSpaceExportBlobReadBytes
	}
	if blobSizeBytes > 0 && blobSizeBytes < limit {
		return blobSizeBytes
	}
	return limit
}

func readAdminSpaceExportLimitedBlob(reader io.Reader, limitBytes int64) ([]byte, error) {
	if reader == nil {
		return nil, fmt.Errorf("附件内容读取器为空")
	}
	if limitBytes <= 0 {
		limitBytes = maxAdminSpaceExportBlobReadBytes
	}
	payload, err := io.ReadAll(io.LimitReader(reader, limitBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > limitBytes {
		return nil, fmt.Errorf("附件内容超过导出读取上限")
	}
	return payload, nil
}

func resolveAdminSpaceExportLocalBlobPath(localRootDir string, objectKey string) (string, error) {
	normalizedObjectKey := strings.TrimSpace(strings.TrimPrefix(objectKey, "/"))
	if normalizedObjectKey == "" {
		return "", fmt.Errorf("附件 object key 为空")
	}
	cleanObjectKey := path.Clean(normalizedObjectKey)
	if cleanObjectKey == "." || cleanObjectKey == "/" || strings.HasPrefix(cleanObjectKey, "../") {
		return "", fmt.Errorf("附件 object key 非法")
	}
	rootDir := strings.TrimSpace(localRootDir)
	if rootDir == "" {
		rootDir = "uploads"
	}
	targetPath := filepath.Join(rootDir, filepath.FromSlash(cleanObjectKey))
	targetAbsPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", err
	}
	rootAbsPath, err := filepath.Abs(rootDir)
	if err != nil {
		return "", err
	}
	if !isAdminSpaceExportPathWithinRoot(rootAbsPath, targetAbsPath) {
		return "", fmt.Errorf("附件 object key 超出本地根目录")
	}
	return targetAbsPath, nil
}

func isAdminSpaceExportPathWithinRoot(rootAbsPath string, targetAbsPath string) bool {
	rel, err := filepath.Rel(rootAbsPath, targetAbsPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != "" && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..")
}

func buildAdminSpaceExportFileName(spaceID string, exportedAt time.Time, format AdminSpaceExportFormat) string {
	return fmt.Sprintf(
		"space-%s-%s%s",
		sanitizeAdminSpaceExportPathSegment(spaceID, "space"),
		exportedAt.Format("20060102150405"),
		adminSpaceExportFileExtension(format),
	)
}

func adminSpaceExportFileExtension(format AdminSpaceExportFormat) string {
	if format == AdminSpaceExportFormatSourceZip {
		return ".plaindoc"
	}
	return ".zip"
}

func writeAdminSpaceExportZip(partPath string, pkg adminSpaceExportPackage) error {
	if err := validateAdminSpaceExportPackage(pkg); err != nil {
		return err
	}
	file, err := os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	closeFile := true
	defer func() {
		if closeFile {
			_ = file.Close()
		}
	}()

	zipWriter := zip.NewWriter(file)
	if err := writeAdminSpaceExportJSONEntry(zipWriter, safeAdminSpaceExportZipEntry(pkg.RootEntryPrefix, "manifest.json"), pkg.Manifest); err != nil {
		_ = zipWriter.Close()
		return err
	}
	if err := writeAdminSpaceExportJSONEntry(zipWriter, safeAdminSpaceExportZipEntry(pkg.RootEntryPrefix, "tree.json"), pkg.Tree); err != nil {
		_ = zipWriter.Close()
		return err
	}

	filePaths := make([]string, 0, len(pkg.Files))
	for filePath := range pkg.Files {
		filePaths = append(filePaths, filePath)
	}
	sort.Strings(filePaths)
	for _, filePath := range filePaths {
		if err := writeAdminSpaceExportFileEntry(zipWriter, safeAdminSpaceExportZipEntry(pkg.RootEntryPrefix, filePath), pkg.Files[filePath]); err != nil {
			_ = zipWriter.Close()
			return err
		}
	}
	if err := zipWriter.Close(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		closeFile = false
		return err
	}
	closeFile = false
	return nil
}

func validateAdminSpaceExportPackage(pkg adminSpaceExportPackage) error {
	if !pkg.Manifest.Importable {
		return nil
	}
	if pkg.Manifest.Space.Cover != nil {
		coverPath := strings.TrimSpace(pkg.Manifest.Space.Cover.Path)
		if coverPath == "" {
			return fmt.Errorf("manifest cover 路径为空")
		}
		if _, ok := pkg.Files[coverPath]; !ok {
			return fmt.Errorf("manifest 引用封面缺失: %s", coverPath)
		}
	}
	attachmentCount := 0
	officeSourceCount := 0
	for _, document := range pkg.Manifest.Documents {
		if strings.TrimSpace(document.Path) != "" {
			if _, ok := pkg.Files[document.Path]; !ok {
				return fmt.Errorf("manifest 引用文件缺失: %s", document.Path)
			}
		}
		for _, attachmentPath := range document.Attachments {
			attachmentPath = strings.TrimSpace(attachmentPath)
			if attachmentPath == "" {
				continue
			}
			attachmentCount++
			if _, ok := pkg.Files[attachmentPath]; !ok {
				return fmt.Errorf("manifest 引用附件缺失: %s", attachmentPath)
			}
		}
		if document.Source != nil && document.Source.Included {
			sourcePath := strings.TrimSpace(document.Source.Path)
			if sourcePath == "" {
				return fmt.Errorf("manifest source 路径为空: %s", document.DocumentID)
			}
			officeSourceCount++
			if _, ok := pkg.Files[sourcePath]; !ok {
				return fmt.Errorf("manifest 引用 source 缺失: %s", sourcePath)
			}
		}
	}
	if pkg.Manifest.Summary.AttachmentCount != attachmentCount {
		return fmt.Errorf("manifest 附件数量不一致: summary=%d actual=%d", pkg.Manifest.Summary.AttachmentCount, attachmentCount)
	}
	if pkg.Manifest.Summary.OfficeSourceCount != officeSourceCount {
		return fmt.Errorf("manifest Office source 数量不一致: summary=%d actual=%d", pkg.Manifest.Summary.OfficeSourceCount, officeSourceCount)
	}
	return nil
}

func writeAdminSpaceExportJSONEntry(zipWriter *zip.Writer, entry string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeAdminSpaceExportFileEntry(zipWriter, entry, payload)
}

func writeAdminSpaceExportFileEntry(zipWriter *zip.Writer, entry string, payload []byte) error {
	if zipWriter == nil {
		return fmt.Errorf("zip writer is nil")
	}
	header := &zip.FileHeader{Name: entry, Method: zip.Deflate}
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = writer.Write(payload)
	return err
}

func (s *AdminSpaceExportService) now() time.Time {
	if s != nil && s.nowFn != nil {
		return s.nowFn().UTC()
	}
	return time.Now().UTC()
}

func (s *AdminSpaceExportJobStore) Create(job *AdminSpaceExportJob) error {
	if s == nil || job == nil {
		return errcode.ErrAdminSpaceExportJobNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	activeCount := 0
	for _, existing := range s.jobs {
		if existing == nil || !isActiveAdminSpaceExportStatus(existing.Status) {
			continue
		}
		activeCount++
		if existing.ActorUserID == job.ActorUserID && existing.SpaceID == job.SpaceID {
			return errcode.ErrAdminSpaceExportJobRunningLimit
		}
	}
	if activeCount >= maxRunningAdminSpaceExportJobs {
		return errcode.ErrAdminSpaceExportJobRunningLimit
	}
	copyJob := *job
	s.jobs[job.JobID] = &copyJob
	return nil
}

func (s *AdminSpaceExportJobStore) Get(jobID string) (AdminSpaceExportJob, error) {
	if s == nil || jobID == "" {
		return AdminSpaceExportJob{}, errcode.ErrAdminSpaceExportJobNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok || job == nil {
		return AdminSpaceExportJob{}, errcode.ErrAdminSpaceExportJobNotFound
	}
	return *job, nil
}

func (s *AdminSpaceExportJobStore) Subscribe(
	jobID string,
	actorUserID string,
	streamToken string,
	now time.Time,
) (AdminSpaceTransferEvent, <-chan AdminSpaceTransferEvent, func(), error) {
	if s == nil || jobID == "" {
		return AdminSpaceTransferEvent{}, nil, func() {}, errcode.ErrAdminSpaceExportJobNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok || job == nil {
		return AdminSpaceTransferEvent{}, nil, func() {}, errcode.ErrAdminSpaceExportJobNotFound
	}
	if job.ActorUserID != actorUserID ||
		job.StreamTokenHash == "" ||
		tokenHash(streamToken) != job.StreamTokenHash ||
		!now.Before(job.StreamTokenExpiresAt) {
		return AdminSpaceTransferEvent{}, nil, func() {}, errcode.ErrAdminSpaceExportJobTokenInvalid
	}

	initialEvent := job.LastEvent
	if job.Status == AdminSpaceExportStatusCompleted && strings.TrimSpace(initialEvent.DownloadURL) == "" {
		if !job.DownloadTokenExpiresAt.IsZero() && !now.Before(job.DownloadTokenExpiresAt) {
			return AdminSpaceTransferEvent{}, nil, func() {}, errcode.ErrAdminSpaceExportFileExpired
		}
		plainToken, tokenHashValue, err := generateAdminSpaceTransferToken()
		if err != nil {
			return AdminSpaceTransferEvent{}, nil, func() {}, err
		}
		if job.downloadTokens == nil {
			job.downloadTokens = make(map[string]adminSpaceExportDownloadToken)
		}
		expiresAt := job.DownloadTokenExpiresAt
		if expiresAt.IsZero() {
			expiresAt = now.Add(defaultAdminSpaceTransferTokenTTL)
		}
		job.downloadTokens[tokenHashValue] = adminSpaceExportDownloadToken{ExpiresAt: expiresAt}
		job.UpdatedAt = now
		initialEvent.DownloadURL = "/api/admin/space-exports/" + jobID + "/download?token=" + plainToken
	}

	ch := make(chan AdminSpaceTransferEvent, adminSpaceTransferEventBufferSize)
	if s.subscribers[jobID] == nil {
		s.subscribers[jobID] = make(map[chan AdminSpaceTransferEvent]struct{})
	}
	s.subscribers[jobID][ch] = struct{}{}
	var unsubscribeOnce sync.Once
	unsubscribe := func() {
		unsubscribeOnce.Do(func() {
			s.mu.Lock()
			removed := false
			defer s.mu.Unlock()
			if subscribers := s.subscribers[jobID]; subscribers != nil {
				if _, ok := subscribers[ch]; ok {
					delete(subscribers, ch)
					removed = true
				}
				if len(subscribers) == 0 {
					delete(s.subscribers, jobID)
				}
			}
			if removed {
				close(ch)
			}
		})
	}
	return initialEvent, ch, unsubscribe, nil
}

func (s *AdminSpaceExportJobStore) Publish(jobID string, event AdminSpaceTransferEvent, now time.Time) {
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

func (s *AdminSpaceExportJobStore) MarkRunning(jobID string, now time.Time) error {
	if s == nil || jobID == "" {
		return errcode.ErrAdminSpaceExportJobNotFound
	}
	s.mu.Lock()
	job, ok := s.jobs[jobID]
	if !ok || job == nil {
		s.mu.Unlock()
		return errcode.ErrAdminSpaceExportJobNotFound
	}
	job.Status = AdminSpaceExportStatusRunning
	job.LastEvent = AdminSpaceTransferEvent{Type: AdminSpaceTransferEventTypeProgress, Stage: "running", Progress: 1, Message: "导出任务开始执行"}
	job.UpdatedAt = now
	event := job.LastEvent
	s.mu.Unlock()
	s.Publish(jobID, event, now)
	return nil
}

func (s *AdminSpaceExportJobStore) Fail(jobID string, stage string, message string, now time.Time) {
	if s == nil || jobID == "" {
		return
	}
	s.mu.Lock()
	job, ok := s.jobs[jobID]
	if !ok || job == nil {
		s.mu.Unlock()
		return
	}
	job.Status = AdminSpaceExportStatusFailed
	job.LastEvent = AdminSpaceTransferEvent{Type: AdminSpaceTransferEventTypeFailed, Stage: stage, Message: message}
	job.UpdatedAt = now
	event := job.LastEvent
	s.mu.Unlock()
	s.Publish(jobID, event, now)
}

func (s *AdminSpaceExportJobStore) Complete(
	jobID string,
	fileName string,
	filePath string,
	sizeBytes int64,
	plainToken string,
	tokenHashValue string,
	now time.Time,
) (AdminSpaceTransferEvent, error) {
	if s == nil || jobID == "" {
		return AdminSpaceTransferEvent{}, errcode.ErrAdminSpaceExportJobNotFound
	}
	s.mu.Lock()
	job, ok := s.jobs[jobID]
	if !ok || job == nil {
		s.mu.Unlock()
		return AdminSpaceTransferEvent{}, errcode.ErrAdminSpaceExportJobNotFound
	}
	job.Status = AdminSpaceExportStatusCompleted
	job.DownloadTokenHash = tokenHashValue
	job.DownloadTokenExpiresAt = now.Add(defaultAdminSpaceTransferTokenTTL)
	job.DownloadTokenUsed = false
	job.downloadTokens = map[string]adminSpaceExportDownloadToken{
		tokenHashValue: {ExpiresAt: job.DownloadTokenExpiresAt},
	}
	job.FileName = strings.TrimSpace(fileName)
	job.FilePath = strings.TrimSpace(filePath)
	job.SizeBytes = sizeBytes
	event := AdminSpaceTransferEvent{
		Type:        AdminSpaceTransferEventTypeCompleted,
		Stage:       "done",
		Progress:    100,
		Message:     "导出完成",
		DownloadURL: "/api/admin/space-exports/" + jobID + "/download?token=" + plainToken,
		FileName:    strings.TrimSpace(fileName),
		SizeBytes:   sizeBytes,
	}
	job.LastEvent = event
	job.LastEvent.DownloadURL = ""
	job.UpdatedAt = now
	for subscriber := range s.subscribers[jobID] {
		select {
		case subscriber <- event:
		default:
		}
	}
	s.mu.Unlock()
	return event, nil
}

func (s *AdminSpaceExportJobStore) ConsumeDownloadToken(
	jobID string,
	actorUserID string,
	token string,
	now time.Time,
) (AdminSpaceExportDownload, error) {
	if s == nil || jobID == "" {
		return AdminSpaceExportDownload{}, errcode.ErrAdminSpaceExportJobNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok || job == nil {
		return AdminSpaceExportDownload{}, errcode.ErrAdminSpaceExportJobNotFound
	}
	if job.ActorUserID != actorUserID ||
		job.Status != AdminSpaceExportStatusCompleted ||
		strings.TrimSpace(token) == "" {
		return AdminSpaceExportDownload{}, errcode.ErrAdminSpaceExportDownloadForbidden
	}
	tokenHashValue := tokenHash(token)
	tokenState, hasToken := job.downloadTokens[tokenHashValue]
	if !hasToken && job.DownloadTokenHash != "" && tokenHashValue == job.DownloadTokenHash {
		tokenState = adminSpaceExportDownloadToken{ExpiresAt: job.DownloadTokenExpiresAt, Used: job.DownloadTokenUsed}
		hasToken = true
	}
	if !hasToken || tokenState.Used {
		return AdminSpaceExportDownload{}, errcode.ErrAdminSpaceExportDownloadForbidden
	}
	if !now.Before(tokenState.ExpiresAt) {
		return AdminSpaceExportDownload{}, errcode.ErrAdminSpaceExportFileExpired
	}
	if strings.TrimSpace(job.FilePath) == "" {
		return AdminSpaceExportDownload{}, errcode.ErrAdminSpaceExportFileNotReady
	}
	info, statErr := os.Stat(job.FilePath)
	if statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return AdminSpaceExportDownload{}, errcode.ErrAdminSpaceExportFileNotReady
		}
		return AdminSpaceExportDownload{}, statErr
	}
	if info.IsDir() {
		return AdminSpaceExportDownload{}, errcode.ErrAdminSpaceExportFileNotReady
	}
	tokenState.Used = true
	if job.downloadTokens != nil {
		job.downloadTokens[tokenHashValue] = tokenState
	}
	if tokenHashValue == job.DownloadTokenHash {
		job.DownloadTokenUsed = true
	}
	job.UpdatedAt = now
	return AdminSpaceExportDownload{
		FileName:  strings.TrimSpace(job.FileName),
		FilePath:  strings.TrimSpace(job.FilePath),
		SizeBytes: info.Size(),
	}, nil
}

func (s *AdminSpaceExportJobStore) DeleteExpired(now time.Time) []AdminSpaceExportJob {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	expiredJobs := make([]AdminSpaceExportJob, 0)
	channelsToClose := make([]chan AdminSpaceTransferEvent, 0)
	for jobID, job := range s.jobs {
		if job == nil || !isExpiredAdminSpaceExportJob(*job, now) {
			continue
		}
		expiredJobs = append(expiredJobs, *job)
		for subscriber := range s.subscribers[jobID] {
			channelsToClose = append(channelsToClose, subscriber)
		}
		delete(s.subscribers, jobID)
		delete(s.jobs, jobID)
	}
	s.mu.Unlock()
	for _, subscriber := range channelsToClose {
		close(subscriber)
	}
	return expiredJobs
}

func isActiveAdminSpaceExportStatus(status AdminSpaceExportStatus) bool {
	return status == AdminSpaceExportStatusQueued || status == AdminSpaceExportStatusRunning
}

func isExpiredAdminSpaceExportJob(job AdminSpaceExportJob, now time.Time) bool {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if isActiveAdminSpaceExportStatus(job.Status) {
		return false
	}
	if job.Status == AdminSpaceExportStatusCompleted {
		if !job.DownloadTokenExpiresAt.IsZero() {
			return !now.Before(job.DownloadTokenExpiresAt)
		}
		return !now.Before(job.UpdatedAt.Add(defaultAdminSpaceTransferTokenTTL))
	}
	if !job.StreamTokenExpiresAt.IsZero() {
		return !now.Before(job.StreamTokenExpiresAt)
	}
	return !now.Before(job.UpdatedAt.Add(defaultAdminSpaceTransferTokenTTL))
}

func generateAdminSpaceTransferToken() (string, string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", "", err
	}
	plain := base64.RawURLEncoding.EncodeToString(raw[:])
	return plain, tokenHash(plain), nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}
