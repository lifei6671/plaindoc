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

type adminSpaceExportWorkspaceReader interface {
	ListTreeNodesBySpaceID(ctx context.Context, spaceID string) ([]repository.WorkspaceTreeNodeRecord, error)
	GetDocumentByDocumentID(ctx context.Context, documentID string) (*repository.WorkspaceDocumentRecord, error)
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
	job := &AdminSpaceExportJob{
		JobID:                jobID,
		ActorUserID:          strings.TrimSpace(input.ActorUserID),
		SpaceID:              spaceID,
		Format:               input.Format,
		IncludeAttachments:   input.IncludeAttachments,
		IncludeOfficeSources: input.IncludeOfficeSources,
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
	if s.canGenerateAdminSpaceExportPackage() {
		go s.runAdminSpaceExportJob(context.WithoutCancel(ctx), jobID)
	}

	return StartAdminSpaceExportResult{
		JobID:     jobID,
		StreamURL: "/api/admin/spaces/" + spaceID + "/exports/" + jobID + "/events?token=" + streamToken,
	}, nil
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
	return s.adminAccessService.CanManageSpace(ctx, strings.TrimSpace(actorUserID), strings.TrimSpace(spaceID))
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
	return s.store.Complete(strings.TrimSpace(jobID), strings.TrimSpace(fileName), "", sizeBytes, downloadToken, downloadTokenHash, s.now())
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
	return s.store.ConsumeDownloadToken(strings.TrimSpace(jobID), strings.TrimSpace(actorUserID), strings.TrimSpace(token), s.now())
}

func (s *AdminSpaceExportService) runAdminSpaceExportJob(ctx context.Context, jobID string) {
	if err := s.BeginExportJob(ctx, jobID); err != nil {
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
		return
	}

	downloadToken, downloadTokenHash, err := generateAdminSpaceTransferToken()
	if err != nil {
		s.store.Fail(jobID, "token", "生成下载令牌失败", s.now())
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
		return "", "", 0, errcode.ErrAdminSpaceExportFormatUnsupported
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

	fileName := buildAdminSpaceExportFileName(job.SpaceID, exportedAt)
	exportDir := filepath.Clean(s.exportDir)
	if err := os.MkdirAll(exportDir, 0o700); err != nil {
		return "", "", 0, err
	}
	partPath := filepath.Join(exportDir, job.JobID+".part")
	finalPath := filepath.Join(exportDir, job.JobID+".zip")
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
				entry, content, err := s.buildAdminSpaceExportDocumentEntry(ctx, row, parentPath, usedNames)
				if err != nil {
					return nil, err
				}
				documents = append(documents, entry)
				if len(content) > 0 && entry.Path != "" {
					files[entry.Path] = content
				}
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

	return adminSpaceExportPackage{
		RootEntryPrefix: "space-" + sanitizeAdminSpaceExportPathSegment(space.SpaceID, "space"),
		Manifest: AdminSpaceExportManifest{
			Version:     AdminSpaceExportPackageVersion,
			PackageType: AdminSpaceExportPackageType,
			ExportedAt:  exportedAt.Format(time.RFC3339),
			Format:      job.Format,
			Importable:  true,
			Space: AdminSpaceExportManifestSpace{
				SpaceID:     strings.TrimSpace(space.SpaceID),
				Name:        strings.TrimSpace(space.Name),
				Description: strings.TrimSpace(space.Description),
				CategoryID:  strings.TrimSpace(space.CategoryID),
				Visibility:  string(space.Visibility),
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

func (s *AdminSpaceExportService) buildAdminSpaceExportDocumentEntry(
	ctx context.Context,
	row repository.WorkspaceTreeNodeRecord,
	parentPath string,
	usedNames map[string]int,
) (AdminSpaceExportDocumentEntry, []byte, error) {
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

	switch format {
	case models.DocumentFormatMarkdown:
		fileName := uniqueAdminSpaceExportName(usedNames, title, "document-"+row.NodeID, ".md")
		entry.Path = safeAdminSpaceExportZipEntry("documents", parentPath, fileName)
		content := []byte(document.ContentMD)
		sum := sha256.Sum256(content)
		entry.ContentSHA256 = hex.EncodeToString(sum[:])
		return entry, content, nil
	case models.DocumentFormatDOCX, models.DocumentFormatXLSX:
		sourceFileName := uniqueAdminSpaceExportName(usedNames, title, "source-"+row.NodeID, "."+string(format))
		sourcePath := safeAdminSpaceExportZipEntry("sources", documentID, sourceFileName)
		entry.Path = sourcePath
		entry.Source = &AdminSpaceExportSourceEntry{Path: sourcePath, Included: false}
		return entry, nil, nil
	default:
		return AdminSpaceExportDocumentEntry{}, nil, errcode.ErrAdminSpaceExportFormatUnsupported
	}
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

func buildAdminSpaceExportFileName(spaceID string, exportedAt time.Time) string {
	return fmt.Sprintf(
		"space-%s-%s.zip",
		sanitizeAdminSpaceExportPathSegment(spaceID, "space"),
		exportedAt.Format("20060102150405"),
	)
}

func writeAdminSpaceExportZip(partPath string, pkg adminSpaceExportPackage) error {
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
	job.FileName = strings.TrimSpace(fileName)
	job.FilePath = strings.TrimSpace(filePath)
	job.SizeBytes = sizeBytes
	job.LastEvent = AdminSpaceTransferEvent{
		Type:        AdminSpaceTransferEventTypeCompleted,
		Stage:       "done",
		Progress:    100,
		Message:     "导出完成",
		DownloadURL: "/api/admin/space-exports/" + jobID + "/download?token=" + plainToken,
		FileName:    strings.TrimSpace(fileName),
		SizeBytes:   sizeBytes,
	}
	job.UpdatedAt = now
	event := job.LastEvent
	s.mu.Unlock()
	s.Publish(jobID, event, now)
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
		job.DownloadTokenUsed ||
		job.DownloadTokenHash == "" ||
		tokenHash(token) != job.DownloadTokenHash {
		return AdminSpaceExportDownload{}, errcode.ErrAdminSpaceExportDownloadForbidden
	}
	if !now.Before(job.DownloadTokenExpiresAt) {
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
	job.DownloadTokenUsed = true
	job.UpdatedAt = now
	return AdminSpaceExportDownload{
		FileName:  strings.TrimSpace(job.FileName),
		FilePath:  strings.TrimSpace(job.FilePath),
		SizeBytes: info.Size(),
	}, nil
}

func isActiveAdminSpaceExportStatus(status AdminSpaceExportStatus) bool {
	return status == AdminSpaceExportStatusQueued || status == AdminSpaceExportStatusRunning
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
