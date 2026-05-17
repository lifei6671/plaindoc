package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

const (
	defaultAdminSpaceImportStagingTTL = 30 * time.Minute
	maxRunningAdminSpaceImportJobs    = 1
	defaultAdminSpaceImportStagingDir = "data/imports/admin-space"
	maxAdminSpaceImportZipEntries     = 10000
	maxAdminSpaceImportEntryBytes     = 512 << 20
	maxAdminSpaceImportTotalBytes     = 2 << 30
	maxAdminSpaceImportMetadataBytes  = 4 << 20
)

// MaxAdminSpaceImportUploadBytes 是后台空间导入包文件体积上限。
const MaxAdminSpaceImportUploadBytes int64 = 512 << 20

const maxAdminSpaceImportUploadBytes = MaxAdminSpaceImportUploadBytes

const (
	AdminSpaceImportPackageTypeEPUB = "epub"
)

const (
	adminSpaceImportMIMEDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	adminSpaceImportMIMEXLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
)

// InspectAdminSpaceImportInput 后台空间导入包解析参数。
type InspectAdminSpaceImportInput struct {
	ActorUserID string
	FileName    string
	ContentType string
	SizeBytes   int64
	Reader      io.Reader
}

// AdminSpaceImportInspectResult 后台空间导入包解析结果。
type AdminSpaceImportInspectResult struct {
	ImportID          string
	PackageVersion    int
	PackageType       string
	ExportedAt        string
	SourcePublishedAt string
	SourceAuthors     []string
	Importable        bool
	Space             AdminSpaceImportPreviewSpace
	Summary           AdminSpaceExportManifestSummary
	Warnings          []string
}

// AdminSpaceImportPreviewSpace 展示导入包中的源空间信息。
type AdminSpaceImportPreviewSpace struct {
	SpaceID    string `json:"spaceId"`
	Name       string `json:"name"`
	CategoryID string `json:"categoryId,omitempty"`
	Visibility string `json:"visibility"`
	HasCover   bool   `json:"hasCover"`
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
	ImportID          string
	ActorUserID       string
	FileName          string
	ContentType       string
	FilePath          string
	PackageVersion    int
	PackageType       string
	ExportedAt        string
	SourcePublishedAt string
	SourceAuthors     []string
	Importable        bool
	Space             AdminSpaceImportPreviewSpace
	Summary           AdminSpaceExportManifestSummary
	Warnings          []string
	CreatedAt         time.Time
	ExpiresAt         time.Time
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
	PackageType          string
	RequestedSpaceID     string
	RequestedSpaceName   string
	RequestedCategoryID  string
	RequestedVisibility  string
	NewSpaceID           string
	SpaceIDMappings      map[string]string
	NodeIDMappings       map[string]string
	DocumentIDMappings   map[string]string
	AttachmentIDMappings map[string]string
	Status               AdminSpaceImportStatus
	StreamTokenHash      string
	StreamTokenExpiresAt time.Time
	LastEvent            AdminSpaceTransferEvent
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// AdminSpaceImportIDMappings 保存导入过程中的旧 ID 到新 ID 映射。
type AdminSpaceImportIDMappings struct {
	SpaceIDMappings      map[string]string
	NodeIDMappings       map[string]string
	DocumentIDMappings   map[string]string
	AttachmentIDMappings map[string]string
}

type adminSpaceImportSpaceWriter interface {
	HardDelete(ctx context.Context, spaceID string) (bool, error)
}

type adminSpaceImportCoverWriter interface {
	CreateCoverAsset(ctx context.Context, asset *models.SpaceCoverAsset) error
}

type adminSpaceImportCoverAssetDeleter interface {
	DeleteCoverAssetByAssetID(ctx context.Context, assetID string) (bool, error)
}

type adminSpaceImportWorkspaceWriter interface {
	GetDefaultCategory(ctx context.Context) (*models.SpaceCategory, error)
	CreateSpace(ctx context.Context, space *models.Space) error
	CreateNode(ctx context.Context, params repository.WorkspaceCreateNodeParams) error
}

type adminSpaceImportCategoryReader interface {
	GetByCategoryID(ctx context.Context, categoryID string) (*models.SpaceCategory, error)
}

type adminSpaceImportAttachmentWriter interface {
	Create(ctx context.Context, attachment *models.DocumentAttachment) error
	FindBlobByHash(
		ctx context.Context,
		storageProvider string,
		contentHashAlgo string,
		contentHash string,
		sizeBytes int64,
	) (*models.DocumentAttachmentBlob, error)
	CreateBlob(ctx context.Context, blob *models.DocumentAttachmentBlob) error
	HardDeleteBlobIfUnreferenced(ctx context.Context, blobID string) (bool, error)
}

type adminSpaceImportOfficeHTMLRenderer interface {
	Enqueue(ctx context.Context, task OfficeHTMLRenderTask) error
}

// AdminSpaceImportServiceOption 用于按运行环境注入导入落地依赖。
type AdminSpaceImportServiceOption func(*AdminSpaceImportService)

// WithAdminSpaceImportRepositories 注入导入落地所需仓储。
func WithAdminSpaceImportRepositories(
	spaceWriter adminSpaceImportSpaceWriter,
	workspaceWriter adminSpaceImportWorkspaceWriter,
	categoryReader adminSpaceImportCategoryReader,
	attachmentWriter adminSpaceImportAttachmentWriter,
) AdminSpaceImportServiceOption {
	return func(s *AdminSpaceImportService) {
		s.spaceWriter = spaceWriter
		s.workspaceWriter = workspaceWriter
		s.categoryReader = categoryReader
		s.attachmentWriter = attachmentWriter
	}
}

// WithAdminSpaceImportBlobStorage 覆盖导入附件与 Office source 的本地落地目录。
func WithAdminSpaceImportBlobStorage(localRootDir string) AdminSpaceImportServiceOption {
	return func(s *AdminSpaceImportService) {
		if trimmed := strings.TrimSpace(localRootDir); trimmed != "" {
			s.localBlobRootDir = trimmed
		}
	}
}

// WithAdminSpaceImportOfficeHTMLRenderer 注入 Office source 导入后的 HTML 渲染队列。
func WithAdminSpaceImportOfficeHTMLRenderer(renderer adminSpaceImportOfficeHTMLRenderer) AdminSpaceImportServiceOption {
	return func(s *AdminSpaceImportService) {
		s.officeHTMLRenderer = renderer
	}
}

// WithAdminSpaceImportAuditRecorder 注入后台审计记录器。
func WithAdminSpaceImportAuditRecorder(auditRecorder adminAuditRecorder) AdminSpaceImportServiceOption {
	return func(s *AdminSpaceImportService) {
		s.auditRecorder = auditRecorder
	}
}

// WithAdminSpaceImportTransferJobRepository 注入全局任务持久化仓储。
func WithAdminSpaceImportTransferJobRepository(
	transferJobRepo repository.AdminSpaceTransferJobRepository,
) AdminSpaceImportServiceOption {
	return func(s *AdminSpaceImportService) {
		s.transferJobRepo = transferJobRepo
	}
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
	spaceWriter        adminSpaceImportSpaceWriter
	workspaceWriter    adminSpaceImportWorkspaceWriter
	categoryReader     adminSpaceImportCategoryReader
	attachmentWriter   adminSpaceImportAttachmentWriter
	officeHTMLRenderer adminSpaceImportOfficeHTMLRenderer
	auditRecorder      adminAuditRecorder
	transferJobRepo    repository.AdminSpaceTransferJobRepository
	stagingDir         string
	localBlobRootDir   string
	nowFn              func() time.Time
	canImportSpace     func(context.Context, string) (bool, error)
}

// NewAdminSpaceImportService 创建空间导入服务。
func NewAdminSpaceImportService(
	adminAccessService *AdminAccessService,
	options ...AdminSpaceImportServiceOption,
) *AdminSpaceImportService {
	svc := &AdminSpaceImportService{
		adminAccessService: adminAccessService,
		store:              NewAdminSpaceImportStore(),
		stagingDir:         defaultAdminSpaceImportStagingDir,
		localBlobRootDir:   "uploads",
		nowFn:              time.Now,
	}
	svc.canImportSpace = svc.defaultCanImportSpace
	for _, option := range options {
		if option != nil {
			option(svc)
		}
	}
	return svc
}

// Inspect 解析导入包元数据，并将可识别的空间交换包写入私有 staging。
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
	fileName := strings.TrimSpace(input.FileName)
	contentType := strings.TrimSpace(input.ContentType)
	if !isSupportedAdminSpaceImportUploadType(fileName, contentType) {
		return AdminSpaceImportInspectResult{}, errcode.ErrAdminSpaceImportPackageUnsupported
	}
	if input.SizeBytes > maxAdminSpaceImportUploadBytes {
		return AdminSpaceImportInspectResult{}, errcode.ErrAdminSpaceImportZipInvalid
	}
	payload, err := readAdminSpaceImportUpload(input.Reader, maxAdminSpaceImportUploadBytes)
	if err != nil {
		return AdminSpaceImportInspectResult{}, errcode.ErrAdminSpaceImportZipInvalid
	}
	if len(payload) == 0 {
		return AdminSpaceImportInspectResult{}, errcode.ErrAdminSpaceImportZipInvalid
	}

	importID := strings.ToLower(ulid.Make().String())
	if strings.EqualFold(filepath.Ext(fileName), ".epub") {
		preview, warnings, err := inspectAdminSpaceImportEPUB(payload)
		if err != nil {
			slog.WarnContext(ctx, "EPUB 导入包预检失败",
				"actorUserID", strings.TrimSpace(input.ActorUserID),
				"fileName", fileName,
				"error", err,
			)
			return AdminSpaceImportInspectResult{}, err
		}
		now := s.now()
		preview.Space.SpaceID = "epub-" + importID
		stagingPath, err := s.writeStagingFile(importID, fileName, payload)
		if err != nil {
			return AdminSpaceImportInspectResult{}, err
		}
		staging := AdminSpaceImportStaging{
			ImportID:          importID,
			ActorUserID:       strings.TrimSpace(input.ActorUserID),
			FileName:          fileName,
			ContentType:       contentType,
			FilePath:          stagingPath,
			PackageVersion:    AdminSpaceExportPackageVersion,
			PackageType:       AdminSpaceImportPackageTypeEPUB,
			SourcePublishedAt: strings.TrimSpace(preview.SourcePublishedAt),
			SourceAuthors:     append([]string(nil), preview.SourceAuthors...),
			Importable:        true,
			Space:             preview.Space,
			Summary:           preview.Summary,
			Warnings:          cloneAdminSpaceImportWarnings(warnings),
			CreatedAt:         now,
			ExpiresAt:         now.Add(defaultAdminSpaceImportStagingTTL),
		}
		s.store.SaveStaging(staging)
		slog.InfoContext(ctx, "EPUB 导入包预检完成",
			"actorUserID", strings.TrimSpace(input.ActorUserID),
			"importID", importID,
			"title", staging.Space.Name,
			"documentCount", staging.Summary.DocumentCount,
			"imageCount", staging.Summary.ImageCount,
		)
		return AdminSpaceImportInspectResult{
			ImportID:          importID,
			PackageVersion:    staging.PackageVersion,
			PackageType:       staging.PackageType,
			SourcePublishedAt: staging.SourcePublishedAt,
			SourceAuthors:     append([]string(nil), staging.SourceAuthors...),
			Importable:        staging.Importable,
			Space:             staging.Space,
			Summary:           staging.Summary,
			Warnings:          cloneAdminSpaceImportWarnings(staging.Warnings),
		}, nil
	}

	manifest, _, warnings, err := inspectAdminSpaceImportZip(payload)
	if err != nil {
		return AdminSpaceImportInspectResult{}, err
	}

	now := s.now()
	stagingPath, err := s.writeStagingFile(importID, fileName, payload)
	if err != nil {
		return AdminSpaceImportInspectResult{}, err
	}
	staging := AdminSpaceImportStaging{
		ImportID:       importID,
		ActorUserID:    strings.TrimSpace(input.ActorUserID),
		FileName:       fileName,
		ContentType:    contentType,
		FilePath:       stagingPath,
		PackageVersion: manifest.Version,
		PackageType:    manifest.PackageType,
		ExportedAt:     strings.TrimSpace(manifest.ExportedAt),
		Importable:     manifest.Importable,
		Space: AdminSpaceImportPreviewSpace{
			SpaceID:    strings.TrimSpace(manifest.Space.SpaceID),
			Name:       strings.TrimSpace(manifest.Space.Name),
			CategoryID: strings.TrimSpace(manifest.Space.CategoryID),
			Visibility: strings.TrimSpace(manifest.Space.Visibility),
			HasCover:   manifest.Space.Cover != nil,
		},
		Summary:   manifest.Summary,
		Warnings:  cloneAdminSpaceImportWarnings(warnings),
		CreatedAt: now,
		ExpiresAt: now.Add(defaultAdminSpaceImportStagingTTL),
	}
	s.store.SaveStaging(staging)
	return AdminSpaceImportInspectResult{
		ImportID:          importID,
		PackageVersion:    staging.PackageVersion,
		PackageType:       staging.PackageType,
		ExportedAt:        staging.ExportedAt,
		SourcePublishedAt: staging.SourcePublishedAt,
		SourceAuthors:     append([]string(nil), staging.SourceAuthors...),
		Importable:        staging.Importable,
		Space:             staging.Space,
		Summary:           staging.Summary,
		Warnings:          cloneAdminSpaceImportWarnings(staging.Warnings),
	}, nil
}

// cloneAdminSpaceImportWarnings 保证 inspect 响应中的 warnings 始终编码为 JSON 数组。
// Go 的 nil slice 会被编码成 null，前端按数组渲染时会触发运行时错误。
func cloneAdminSpaceImportWarnings(warnings []string) []string {
	normalized := make([]string, 0, len(warnings))
	return append(normalized, warnings...)
}

func readAdminSpaceImportUpload(reader io.Reader, limitBytes int64) ([]byte, error) {
	if reader == nil {
		return nil, errcode.ErrAdminSpaceImportFileRequired
	}
	if limitBytes <= 0 {
		limitBytes = maxAdminSpaceImportUploadBytes
	}
	payload, err := io.ReadAll(io.LimitReader(reader, limitBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > limitBytes {
		return nil, fmt.Errorf("admin space import upload exceeds limit")
	}
	return payload, nil
}

func isSupportedAdminSpaceImportUploadType(fileName string, contentType string) bool {
	extension := filepath.Ext(strings.TrimSpace(fileName))
	return strings.EqualFold(extension, ".plaindoc") || strings.EqualFold(extension, ".epub")
}

func (s *AdminSpaceImportService) writeStagingFile(importID string, sourceFileName string, payload []byte) (string, error) {
	if len(payload) == 0 {
		return "", errcode.ErrAdminSpaceImportZipInvalid
	}
	stagingDir := defaultAdminSpaceImportStagingDir
	if s != nil && strings.TrimSpace(s.stagingDir) != "" {
		stagingDir = strings.TrimSpace(s.stagingDir)
	}
	cleanDir := filepath.Clean(stagingDir)
	if err := os.MkdirAll(cleanDir, 0o700); err != nil {
		return "", err
	}
	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(sourceFileName)))
	if extension != ".epub" {
		extension = ".plaindoc"
	}
	fileName := sanitizeAdminSpaceExportPathSegment(importID, "import") + extension
	filePath := filepath.Join(cleanDir, fileName)
	if err := os.WriteFile(filePath, payload, 0o600); err != nil {
		return "", err
	}
	return filePath, nil
}

func inspectAdminSpaceImportZip(payload []byte) (AdminSpaceExportManifest, AdminSpaceExportTree, []string, error) {
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return AdminSpaceExportManifest{}, AdminSpaceExportTree{}, nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	if isAdminSpaceImportEPUB(reader) {
		return AdminSpaceExportManifest{}, AdminSpaceExportTree{}, nil, errcode.ErrAdminSpaceImportPackageUnsupported
	}
	if len(reader.File) > maxAdminSpaceImportZipEntries {
		return AdminSpaceExportManifest{}, AdminSpaceExportTree{}, nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	entries := make(map[string]*zip.File, len(reader.File))
	var totalUncompressedBytes uint64
	for _, file := range reader.File {
		if file == nil {
			continue
		}
		if file.UncompressedSize64 > maxAdminSpaceImportEntryBytes {
			return AdminSpaceExportManifest{}, AdminSpaceExportTree{}, nil, errcode.ErrAdminSpaceImportZipInvalid
		}
		totalUncompressedBytes += file.UncompressedSize64
		if totalUncompressedBytes > maxAdminSpaceImportTotalBytes {
			return AdminSpaceExportManifest{}, AdminSpaceExportTree{}, nil, errcode.ErrAdminSpaceImportZipInvalid
		}
		entryName := cleanAdminSpaceImportZipEntry(file.Name)
		if entryName == "" {
			return AdminSpaceExportManifest{}, AdminSpaceExportTree{}, nil, errcode.ErrAdminSpaceImportZipInvalid
		}
		if _, ok := entries[entryName]; ok {
			return AdminSpaceExportManifest{}, AdminSpaceExportTree{}, nil, errcode.ErrAdminSpaceImportZipInvalid
		}
		entries[entryName] = file
	}

	manifestFile, manifestRoot, err := findAdminSpaceImportManifestEntry(entries)
	if err != nil {
		return AdminSpaceExportManifest{}, AdminSpaceExportTree{}, nil, err
	}
	if manifestFile == nil {
		return AdminSpaceExportManifest{}, AdminSpaceExportTree{}, nil, errcode.ErrAdminSpaceImportManifestMissing
	}
	treeFile := entries[cleanAdminSpaceImportZipEntry(path.Join(manifestRoot, "tree.json"))]
	if treeFile == nil {
		return AdminSpaceExportManifest{}, AdminSpaceExportTree{}, nil, errcode.ErrAdminSpaceImportTreeMissing
	}

	var manifest AdminSpaceExportManifest
	if err := readAdminSpaceImportZipJSON(manifestFile, &manifest); err != nil {
		return AdminSpaceExportManifest{}, AdminSpaceExportTree{}, nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	if manifest.PackageType != AdminSpaceExportPackageType || manifest.Version != AdminSpaceExportPackageVersion {
		return AdminSpaceExportManifest{}, AdminSpaceExportTree{}, nil, errcode.ErrAdminSpaceImportPackageUnsupported
	}

	var tree AdminSpaceExportTree
	if err := readAdminSpaceImportZipJSON(treeFile, &tree); err != nil {
		return AdminSpaceExportManifest{}, AdminSpaceExportTree{}, nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	if tree.Version != AdminSpaceExportPackageVersion {
		return AdminSpaceExportManifest{}, AdminSpaceExportTree{}, nil, errcode.ErrAdminSpaceImportPackageUnsupported
	}
	if err := validateAdminSpaceImportManifestFiles(manifest, entries, manifestRoot); err != nil {
		return AdminSpaceExportManifest{}, AdminSpaceExportTree{}, nil, err
	}
	if err := validateAdminSpaceImportTree(tree); err != nil {
		return AdminSpaceExportManifest{}, AdminSpaceExportTree{}, nil, err
	}

	warnings := []string{}
	if !manifest.Importable {
		warnings = append(warnings, "导入包标记为不可原样导入，禁止提交导入")
	}
	return manifest, tree, warnings, nil
}

func isAdminSpaceImportEPUB(reader *zip.Reader) bool {
	if reader == nil {
		return false
	}
	for _, file := range reader.File {
		if file == nil || cleanAdminSpaceImportZipEntry(file.Name) != "mimetype" {
			continue
		}
		payload, err := readAdminSpaceImportZipFile(file)
		return err == nil && strings.TrimSpace(string(payload)) == "application/epub+zip"
	}
	return false
}

func cleanAdminSpaceImportZipEntry(entry string) string {
	normalized := strings.TrimSpace(entry)
	if normalized == "" ||
		strings.HasPrefix(normalized, "/") ||
		strings.Contains(normalized, "\\") ||
		strings.Contains(normalized, ":") {
		return ""
	}
	for _, segment := range strings.Split(normalized, "/") {
		if segment == ".." {
			return ""
		}
	}
	cleaned := path.Clean(normalized)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return ""
	}
	return cleaned
}

func findAdminSpaceImportManifestEntry(entries map[string]*zip.File) (*zip.File, string, error) {
	var manifestFile *zip.File
	manifestRoot := ""
	for entryName, file := range entries {
		if path.Base(entryName) != "manifest.json" {
			continue
		}
		if manifestFile != nil {
			return nil, "", errcode.ErrAdminSpaceImportZipInvalid
		}
		manifestFile = file
		root := path.Dir(entryName)
		if root == "." {
			root = ""
		}
		manifestRoot = root
	}
	return manifestFile, manifestRoot, nil
}

func readAdminSpaceImportZipJSON(file *zip.File, value any) error {
	payload, err := readAdminSpaceImportZipFile(file)
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, value)
}

func readAdminSpaceImportZipFile(file *zip.File) ([]byte, error) {
	if file == nil {
		return nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	if file.UncompressedSize64 > maxAdminSpaceImportMetadataBytes {
		return nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	payload, err := io.ReadAll(io.LimitReader(reader, maxAdminSpaceImportMetadataBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxAdminSpaceImportMetadataBytes {
		return nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	return payload, nil
}

func validateAdminSpaceImportManifestFiles(
	manifest AdminSpaceExportManifest,
	entries map[string]*zip.File,
	rootPrefix string,
) error {
	if !manifest.Importable {
		return nil
	}
	if manifest.Space.Cover != nil {
		coverPath := strings.TrimSpace(manifest.Space.Cover.Path)
		if coverPath == "" {
			return errcode.ErrAdminSpaceImportPackageNotImportable
		}
		coverSource := strings.TrimSpace(manifest.Space.Cover.Source)
		if coverSource != "" && normalizeAdminSpaceCoverSource(AdminSpaceCoverSource(coverSource)) == "" {
			return errcode.ErrAdminSpaceImportPackageNotImportable
		}
		if !adminSpaceImportZipPathExists(entries, rootPrefix, coverPath) {
			return errcode.ErrAdminSpaceImportPackageNotImportable
		}
	}
	for _, document := range manifest.Documents {
		if strings.TrimSpace(document.Path) == "" {
			return errcode.ErrAdminSpaceImportPackageNotImportable
		}
		if !adminSpaceImportZipPathExists(entries, rootPrefix, document.Path) {
			return errcode.ErrAdminSpaceImportPackageNotImportable
		}
		for _, attachmentPath := range document.Attachments {
			if strings.TrimSpace(attachmentPath) != "" && !adminSpaceImportZipPathExists(entries, rootPrefix, attachmentPath) {
				return errcode.ErrAdminSpaceImportPackageNotImportable
			}
		}
		for _, attachmentEntry := range document.AttachmentEntries {
			attachmentPath := strings.TrimSpace(attachmentEntry.Path)
			if attachmentPath != "" && !adminSpaceImportZipPathExists(entries, rootPrefix, attachmentPath) {
				return errcode.ErrAdminSpaceImportPackageNotImportable
			}
		}
		if document.Source != nil && document.Source.Included {
			if strings.TrimSpace(document.Source.Path) == "" {
				return errcode.ErrAdminSpaceImportPackageNotImportable
			}
			if !adminSpaceImportZipPathExists(entries, rootPrefix, document.Source.Path) {
				return errcode.ErrAdminSpaceImportPackageNotImportable
			}
		}
	}
	return nil
}

func adminSpaceImportZipPathExists(entries map[string]*zip.File, rootPrefix string, packagePath string) bool {
	entryName := cleanAdminSpaceImportZipEntry(path.Join(rootPrefix, packagePath))
	if entryName == "" {
		return false
	}
	_, ok := entries[entryName]
	return ok
}

func validateAdminSpaceImportTree(tree AdminSpaceExportTree) error {
	seen := make(map[string]struct{})
	var walk func(nodes []AdminSpaceExportTreeNode, ancestors map[string]struct{}) error
	walk = func(nodes []AdminSpaceExportTreeNode, ancestors map[string]struct{}) error {
		for _, node := range nodes {
			nodeID := strings.TrimSpace(node.NodeID)
			if nodeID == "" {
				return errcode.ErrAdminSpaceImportPackageNotImportable
			}
			if _, ok := ancestors[nodeID]; ok {
				return errcode.ErrAdminSpaceImportPackageNotImportable
			}
			if _, ok := seen[nodeID]; ok {
				return errcode.ErrAdminSpaceImportPackageNotImportable
			}
			seen[nodeID] = struct{}{}
			nextAncestors := make(map[string]struct{}, len(ancestors)+1)
			for ancestor := range ancestors {
				nextAncestors[ancestor] = struct{}{}
			}
			nextAncestors[nodeID] = struct{}{}
			if err := walk(node.Children, nextAncestors); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(tree.Root, nil)
}

type adminSpaceImportPackage struct {
	Manifest AdminSpaceExportManifest
	Tree     AdminSpaceExportTree
	Root     string
	Entries  map[string]*zip.File
	closer   io.Closer
}

func (p adminSpaceImportPackage) Close() error {
	if p.closer == nil {
		return nil
	}
	return p.closer.Close()
}

func readAdminSpaceImportPackage(filePath string) (adminSpaceImportPackage, error) {
	normalizedFilePath := strings.TrimSpace(filePath)
	if normalizedFilePath == "" {
		return adminSpaceImportPackage{}, errcode.ErrAdminSpaceImportStagingNotFound
	}
	reader, err := zip.OpenReader(normalizedFilePath)
	if err != nil {
		return adminSpaceImportPackage{}, errcode.ErrAdminSpaceImportZipInvalid
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			_ = reader.Close()
		}
	}()

	if isAdminSpaceImportEPUB(&reader.Reader) {
		return adminSpaceImportPackage{}, errcode.ErrAdminSpaceImportPackageUnsupported
	}
	entries, err := collectAdminSpaceImportZipEntries(&reader.Reader)
	if err != nil {
		return adminSpaceImportPackage{}, err
	}
	manifestFile, manifestRoot, err := findAdminSpaceImportManifestEntry(entries)
	if err != nil {
		return adminSpaceImportPackage{}, err
	}
	if manifestFile == nil {
		return adminSpaceImportPackage{}, errcode.ErrAdminSpaceImportManifestMissing
	}
	treeFile := entries[cleanAdminSpaceImportZipEntry(path.Join(manifestRoot, "tree.json"))]
	if treeFile == nil {
		return adminSpaceImportPackage{}, errcode.ErrAdminSpaceImportTreeMissing
	}

	var manifest AdminSpaceExportManifest
	if err := readAdminSpaceImportZipJSON(manifestFile, &manifest); err != nil {
		return adminSpaceImportPackage{}, errcode.ErrAdminSpaceImportZipInvalid
	}
	if manifest.PackageType != AdminSpaceExportPackageType || manifest.Version != AdminSpaceExportPackageVersion {
		return adminSpaceImportPackage{}, errcode.ErrAdminSpaceImportPackageUnsupported
	}
	var tree AdminSpaceExportTree
	if err := readAdminSpaceImportZipJSON(treeFile, &tree); err != nil {
		return adminSpaceImportPackage{}, errcode.ErrAdminSpaceImportZipInvalid
	}
	if tree.Version != AdminSpaceExportPackageVersion {
		return adminSpaceImportPackage{}, errcode.ErrAdminSpaceImportPackageUnsupported
	}
	if err := validateAdminSpaceImportManifestFiles(manifest, entries, manifestRoot); err != nil {
		return adminSpaceImportPackage{}, err
	}
	if err := validateAdminSpaceImportTree(tree); err != nil {
		return adminSpaceImportPackage{}, err
	}
	referencedEntries := collectAdminSpaceImportReferencedEntryNames(manifest, manifestRoot)
	for entryName := range referencedEntries {
		file := entries[entryName]
		if file == nil {
			return adminSpaceImportPackage{}, errcode.ErrAdminSpaceImportPackageNotImportable
		}
	}
	closeOnError = false
	return adminSpaceImportPackage{
		Manifest: manifest,
		Tree:     tree,
		Root:     manifestRoot,
		Entries:  entries,
		closer:   reader,
	}, nil
}

func collectAdminSpaceImportReferencedEntryNames(
	manifest AdminSpaceExportManifest,
	rootPrefix string,
) map[string]struct{} {
	result := make(map[string]struct{})
	addPackagePath := func(packagePath string) {
		entryName := cleanAdminSpaceImportZipEntry(path.Join(rootPrefix, packagePath))
		if entryName != "" {
			result[entryName] = struct{}{}
		}
	}
	if manifest.Space.Cover != nil {
		addPackagePath(manifest.Space.Cover.Path)
	}
	for _, document := range manifest.Documents {
		addPackagePath(document.Path)
		for _, attachmentPath := range document.Attachments {
			addPackagePath(attachmentPath)
		}
		for _, attachmentEntry := range document.AttachmentEntries {
			addPackagePath(attachmentEntry.Path)
		}
		if document.Source != nil && document.Source.Included {
			addPackagePath(document.Source.Path)
		}
	}
	return result
}

func collectAdminSpaceImportZipEntries(reader *zip.Reader) (map[string]*zip.File, error) {
	if reader == nil {
		return nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	if len(reader.File) > maxAdminSpaceImportZipEntries {
		return nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	entries := make(map[string]*zip.File, len(reader.File))
	var totalUncompressedBytes uint64
	for _, file := range reader.File {
		if file == nil {
			continue
		}
		if file.UncompressedSize64 > maxAdminSpaceImportEntryBytes {
			return nil, errcode.ErrAdminSpaceImportZipInvalid
		}
		totalUncompressedBytes += file.UncompressedSize64
		if totalUncompressedBytes > maxAdminSpaceImportTotalBytes {
			return nil, errcode.ErrAdminSpaceImportZipInvalid
		}
		entryName := cleanAdminSpaceImportZipEntry(file.Name)
		if entryName == "" {
			return nil, errcode.ErrAdminSpaceImportZipInvalid
		}
		if _, ok := entries[entryName]; ok {
			return nil, errcode.ErrAdminSpaceImportZipInvalid
		}
		entries[entryName] = file
	}
	return entries, nil
}

type adminSpacePackageImporter struct {
	service             *AdminSpaceImportService
	job                 AdminSpaceImportJob
	pkg                 adminSpaceImportPackage
	newSpaceID          string
	oldToNewNodes       map[string]string
	oldToNewDocs        map[string]string
	oldToNewAttachments map[string]string
	createdBlobs        []models.DocumentAttachmentBlob
	createdCoverAsset   *models.SpaceCoverAsset
}

func (i *adminSpacePackageImporter) restoreTree(
	ctx context.Context,
	nodes []AdminSpaceExportTreeNode,
	parentNodeID *string,
) error {
	for _, node := range nodes {
		newNodeID, err := i.restoreNode(ctx, node, parentNodeID)
		if err != nil {
			return err
		}
		if err := i.restoreTree(ctx, node.Children, &newNodeID); err != nil {
			return err
		}
	}
	return nil
}

func (i *adminSpacePackageImporter) restoreNode(
	ctx context.Context,
	treeNode AdminSpaceExportTreeNode,
	parentNodeID *string,
) (string, error) {
	if i == nil || i.service == nil || i.service.workspaceWriter == nil {
		return "", fmt.Errorf("导入服务依赖未配置")
	}
	oldNodeID := strings.TrimSpace(treeNode.NodeID)
	if oldNodeID == "" {
		return "", errcode.ErrAdminSpaceImportPackageNotImportable
	}
	newNodeID := strings.ToLower(ulid.Make().String())
	actorUserID := strings.TrimSpace(i.job.ActorUserID)
	now := i.service.now()
	node := &models.Node{
		NodeID:          newNodeID,
		SpaceID:         i.newSpaceID,
		ParentNodeID:    cloneStringPointer(parentNodeID),
		Type:            models.NodeType(strings.TrimSpace(treeNode.Type)),
		Title:           strings.TrimSpace(treeNode.Title),
		Sort:            treeNode.Sort,
		CreatedByUserID: trimOptionalUserID(actorUserID),
		UpdatedByUserID: trimOptionalUserID(actorUserID),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	switch node.Type {
	case models.NodeTypeFolder:
		if err := i.service.workspaceWriter.CreateNode(ctx, repository.WorkspaceCreateNodeParams{
			Node:       node,
			TouchSpace: i.newSpaceID,
			TouchedAt:  now,
		}); err != nil {
			return "", err
		}
	case models.NodeTypeDoc:
		documentEntry, ok := i.findDocumentEntry(treeNode)
		if !ok {
			return "", errcode.ErrAdminSpaceImportPackageNotImportable
		}
		document, revision, fileRevision, renderTask, err := i.buildImportedDocument(ctx, treeNode, documentEntry, newNodeID, now)
		if err != nil {
			return "", err
		}
		if err := i.service.workspaceWriter.CreateNode(ctx, repository.WorkspaceCreateNodeParams{
			Node:         node,
			Document:     document,
			Revision:     revision,
			FileRevision: fileRevision,
			TouchSpace:   i.newSpaceID,
			TouchedAt:    now,
		}); err != nil {
			return "", err
		}
		if renderTask != nil && i.service.officeHTMLRenderer != nil {
			if err := i.service.officeHTMLRenderer.Enqueue(ctx, *renderTask); err != nil {
				slog.WarnContext(ctx, "Office HTML 渲染排队失败，导入继续完成",
					"spaceID", i.newSpaceID,
					"documentID", document.DocumentID,
				)
			}
		}
		i.oldToNewDocs[strings.TrimSpace(documentEntry.DocumentID)] = document.DocumentID
		if err := i.restoreDocumentAttachments(ctx, documentEntry, document.DocumentID, now); err != nil {
			return "", err
		}
	default:
		return "", errcode.ErrAdminSpaceImportPackageNotImportable
	}
	i.oldToNewNodes[oldNodeID] = newNodeID
	return newNodeID, nil
}

func (i *adminSpacePackageImporter) findDocumentEntry(
	treeNode AdminSpaceExportTreeNode,
) (AdminSpaceExportDocumentEntry, bool) {
	documentID := strings.TrimSpace(treeNode.DocumentID)
	nodeID := strings.TrimSpace(treeNode.NodeID)
	for _, entry := range i.pkg.Manifest.Documents {
		if documentID != "" && strings.TrimSpace(entry.DocumentID) == documentID {
			return entry, true
		}
		if nodeID != "" && strings.TrimSpace(entry.NodeID) == nodeID {
			return entry, true
		}
	}
	return AdminSpaceExportDocumentEntry{}, false
}

func (i *adminSpacePackageImporter) buildImportedDocument(
	ctx context.Context,
	treeNode AdminSpaceExportTreeNode,
	entry AdminSpaceExportDocumentEntry,
	newNodeID string,
	now time.Time,
) (*models.Document, *models.DocumentRevision, *models.DocumentFileRevision, *OfficeHTMLRenderTask, error) {
	format := models.NormalizeDocumentFormat(models.DocumentFormat(strings.TrimSpace(entry.Format)))
	if treeNode.Format != "" {
		format = models.NormalizeDocumentFormat(models.DocumentFormat(strings.TrimSpace(treeNode.Format)))
	}
	visibility := models.Visibility(strings.TrimSpace(entry.Visibility))
	if !models.IsValidVisibility(visibility) {
		visibility = models.VisibilityMember
	}
	documentID := strings.ToLower(ulid.Make().String())
	actorUserID := strings.TrimSpace(i.job.ActorUserID)
	document := &models.Document{
		DocumentID:      documentID,
		NodeID:          newNodeID,
		ThemeID:         "default",
		Visibility:      visibility,
		Status:          models.EntityStatusActive,
		Title:           firstNonEmptyString(strings.TrimSpace(entry.Title), strings.TrimSpace(treeNode.Title), "未命名文档"),
		Format:          format,
		Version:         1,
		ContentVersion:  1,
		CreatedByUserID: trimOptionalUserID(actorUserID),
		UpdatedByUserID: trimOptionalUserID(actorUserID),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	switch format {
	case models.DocumentFormatMarkdown:
		payload, err := i.readPackageFile(entry.Path)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if err := validateAdminSpaceImportSHA256(payload, entry.ContentSHA256); err != nil {
			return nil, nil, nil, nil, err
		}
		document.ContentMD = string(payload)
		return document, &models.DocumentRevision{
			DocumentRevisionID: strings.ToLower(ulid.Make().String()),
			DocumentID:         documentID,
			Version:            1,
			ContentMD:          document.ContentMD,
			BaseVersion:        0,
			EditorUserID:       trimOptionalUserID(actorUserID),
			Source:             models.RevisionSourceLocal,
			CreatedAt:          now,
		}, nil, nil, nil
	case models.DocumentFormatDOCX, models.DocumentFormatXLSX:
		if entry.Source == nil || !entry.Source.Included {
			return nil, nil, nil, nil, errcode.ErrAdminSpaceImportPackageNotImportable
		}
		payload, err := i.readPackageFile(entry.Source.Path)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if err := validateAdminSpaceImportSHA256(payload, entry.Source.SHA256); err != nil {
			return nil, nil, nil, nil, err
		}
		sourceFileName := path.Base(strings.TrimSpace(entry.Source.Path))
		sourceMimeType := resolveAdminSpaceImportOfficeSourceMimeType(format)
		blob, created, err := i.service.ensureImportedBlob(ctx, payload, sourceFileName, sourceMimeType, i.newSpaceID, documentID, now)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if created {
			i.createdBlobs = append(i.createdBlobs, *blob)
		}
		document.RenderStatus = models.DocumentRenderStatusPending
		document.SourceBlobID = trimOptionalUserID(blob.BlobID)
		document.SourceFileName = trimOptionalUserID(sourceFileName)
		document.SourceMimeType = trimOptionalUserID(sourceMimeType)
		fileRevision := &models.DocumentFileRevision{
			DocumentFileRevisionID: strings.ToLower(ulid.Make().String()),
			DocumentID:             documentID,
			BlobID:                 blob.BlobID,
			FileName:               sourceFileName,
			MimeType:               sourceMimeType,
			Version:                1,
			BaseVersion:            0,
			EditorUserID:           trimOptionalUserID(actorUserID),
			Source:                 models.RevisionSourceLocal,
			CreatedAt:              now,
		}
		renderTask := &OfficeHTMLRenderTask{
			DocumentID:     documentID,
			SpaceID:        i.newSpaceID,
			Format:         format,
			ContentVersion: document.ContentVersion,
			SourceBlobID:   blob.BlobID,
			SourceContent:  bytes.Clone(payload),
		}
		return document, nil, fileRevision, renderTask, nil
	default:
		return nil, nil, nil, nil, errcode.ErrAdminSpaceImportPackageNotImportable
	}
}

func (i *adminSpacePackageImporter) restoreDocumentAttachments(
	ctx context.Context,
	entry AdminSpaceExportDocumentEntry,
	newDocumentID string,
	now time.Time,
) error {
	attachmentEntries := normalizeAdminSpaceImportAttachmentEntries(entry)
	if len(attachmentEntries) == 0 {
		return nil
	}
	if i.service.attachmentWriter == nil {
		return fmt.Errorf("附件导入依赖未配置")
	}
	for _, attachmentEntry := range attachmentEntries {
		payload, err := i.readPackageFile(attachmentEntry.Path)
		if err != nil {
			return err
		}
		if err := validateAdminSpaceImportSHA256(payload, attachmentEntry.SHA256); err != nil {
			return err
		}
		fileName := strings.TrimSpace(attachmentEntry.FileName)
		if fileName == "" {
			fileName = path.Base(strings.TrimSpace(attachmentEntry.Path))
		}
		mimeType := strings.TrimSpace(attachmentEntry.MimeType)
		if mimeType == "" {
			mimeType = detectAdminSpaceImportContentType(payload, fileName)
		}
		blob, created, err := i.service.ensureImportedBlob(ctx, payload, fileName, mimeType, i.newSpaceID, newDocumentID, now)
		if err != nil {
			return err
		}
		if created {
			i.createdBlobs = append(i.createdBlobs, *blob)
		}
		actorUserID := strings.TrimSpace(i.job.ActorUserID)
		attachment := &models.DocumentAttachment{
			AttachmentID:    strings.ToLower(ulid.Make().String()),
			BlobID:          blob.BlobID,
			DocumentID:      newDocumentID,
			SpaceID:         i.newSpaceID,
			StorageProvider: blob.StorageProvider,
			FileName:        fileName,
			ObjectKey:       blob.ObjectKey,
			ObjectURL:       blob.ObjectURL,
			MimeType:        blob.MimeType,
			SizeBytes:       blob.SizeBytes,
			ContentHashAlgo: blob.ContentHashAlgo,
			ContentHash:     blob.ContentHash,
			PreviewKind:     resolveAdminSpaceImportAttachmentPreviewKind(mimeType, fileName),
			Status:          models.EntityStatusActive,
			CreatedByUserID: trimOptionalUserID(actorUserID),
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := i.service.attachmentWriter.Create(ctx, attachment); err != nil {
			return err
		}
		oldAttachmentID := strings.TrimSpace(attachmentEntry.AttachmentID)
		if oldAttachmentID != "" {
			i.oldToNewAttachments[oldAttachmentID] = attachment.AttachmentID
		}
	}
	return nil
}

func normalizeAdminSpaceImportAttachmentEntries(
	entry AdminSpaceExportDocumentEntry,
) []AdminSpaceExportAttachmentEntry {
	if len(entry.AttachmentEntries) > 0 {
		normalized := make([]AdminSpaceExportAttachmentEntry, 0, len(entry.AttachmentEntries))
		for _, attachmentEntry := range entry.AttachmentEntries {
			if strings.TrimSpace(attachmentEntry.Path) == "" {
				continue
			}
			normalized = append(normalized, attachmentEntry)
		}
		return normalized
	}
	normalized := make([]AdminSpaceExportAttachmentEntry, 0, len(entry.Attachments))
	for _, attachmentPath := range entry.Attachments {
		if strings.TrimSpace(attachmentPath) == "" {
			continue
		}
		normalized = append(normalized, AdminSpaceExportAttachmentEntry{Path: attachmentPath})
	}
	return normalized
}

func (i *adminSpacePackageImporter) readPackageFile(packagePath string) ([]byte, error) {
	return readAdminSpaceImportPackageFile(i.pkg, packagePath)
}

func readAdminSpaceImportZipPayload(file *zip.File) ([]byte, error) {
	if file == nil || file.UncompressedSize64 > maxAdminSpaceImportEntryBytes {
		return nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	payload, err := io.ReadAll(io.LimitReader(reader, maxAdminSpaceImportEntryBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxAdminSpaceImportEntryBytes {
		return nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	return payload, nil
}

func validateAdminSpaceImportSHA256(payload []byte, expected string) error {
	normalizedExpected := strings.TrimSpace(expected)
	if normalizedExpected == "" {
		return nil
	}
	sum := sha256.Sum256(payload)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), normalizedExpected) {
		return errcode.ErrAdminSpaceImportPackageNotImportable
	}
	return nil
}

func (s *AdminSpaceImportService) ensureImportedBlob(
	ctx context.Context,
	content []byte,
	fileName string,
	contentType string,
	spaceID string,
	documentID string,
	now time.Time,
) (*models.DocumentAttachmentBlob, bool, error) {
	if s == nil || s.attachmentWriter == nil {
		return nil, false, fmt.Errorf("附件导入依赖未配置")
	}
	if len(content) == 0 {
		return nil, false, errcode.ErrAdminSpaceImportZipInvalid
	}
	sum := sha256.Sum256(content)
	contentHash := hex.EncodeToString(sum[:])
	sizeBytes := int64(len(content))
	existingBlob, err := s.attachmentWriter.FindBlobByHash(ctx, string(ImageHostingProviderLocal), "sha256", contentHash, sizeBytes)
	if err == nil && existingBlob != nil {
		return existingBlob, false, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	blobID := strings.ToLower(ulid.Make().String())
	objectKey, err := buildAdminSpaceImportObjectKey(fileName, contentType, spaceID, documentID, blobID, now)
	if err != nil {
		return nil, false, err
	}
	targetPath, err := s.resolveImportedLocalBlobPath(objectKey)
	if err != nil {
		return nil, false, err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return nil, false, err
	}
	if err := os.WriteFile(targetPath, content, 0o644); err != nil {
		return nil, false, err
	}
	blob := &models.DocumentAttachmentBlob{
		BlobID:          blobID,
		StorageProvider: string(ImageHostingProviderLocal),
		ObjectKey:       objectKey,
		ObjectURL:       "/uploads/" + strings.TrimLeft(objectKey, "/"),
		MimeType:        strings.TrimSpace(contentType),
		SizeBytes:       sizeBytes,
		ContentHashAlgo: "sha256",
		ContentHash:     contentHash,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if blob.MimeType == "" {
		blob.MimeType = "application/octet-stream"
	}
	if err := s.attachmentWriter.CreateBlob(ctx, blob); err != nil {
		if cleanupErr := os.Remove(targetPath); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
			return nil, false, cleanupErr
		}
		return nil, false, err
	}
	return blob, true, nil
}

func (s *AdminSpaceImportService) localizeAdminSpaceEPUBChapterImages(
	ctx context.Context,
	input adminSpaceEPUBImageLocalizeInput,
	spaceID string,
	documentID string,
) (string, []string, []models.DocumentAttachmentBlob, error) {
	createdBlobs := make([]models.DocumentAttachmentBlob, 0)
	input.Localize = func(asset adminSpaceEPUBImageAsset) (string, error) {
		// EPUB 图片本地化统一复用导入侧 blob 写入能力，确保 URL、hash 去重和失败清理语义一致。
		blob, created, err := s.ensureImportedBlob(
			ctx,
			asset.Payload,
			asset.FileName,
			asset.ContentType,
			spaceID,
			documentID,
			s.now(),
		)
		if err != nil {
			return "", err
		}
		if blob == nil {
			return "", fmt.Errorf("图片 blob 为空")
		}
		if created {
			createdBlobs = append(createdBlobs, *blob)
		}
		return strings.TrimSpace(blob.ObjectURL), nil
	}
	rewrittenHTML, warnings, err := localizeAdminSpaceEPUBChapterImages(input)
	if err != nil {
		return "", warnings, createdBlobs, err
	}
	return rewrittenHTML, warnings, createdBlobs, nil
}

func (s *AdminSpaceImportService) cleanupCreatedImportBlobs(ctx context.Context, blobs []models.DocumentAttachmentBlob) error {
	if s == nil || s.attachmentWriter == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(blobs))
	var firstErr error
	for _, blob := range blobs {
		normalizedBlobID := strings.TrimSpace(blob.BlobID)
		if normalizedBlobID == "" {
			continue
		}
		if _, ok := seen[normalizedBlobID]; ok {
			continue
		}
		seen[normalizedBlobID] = struct{}{}
		deleted, err := s.attachmentWriter.HardDeleteBlobIfUnreferenced(ctx, normalizedBlobID)
		if err != nil && firstErr == nil {
			firstErr = err
			continue
		}
		if deleted {
			if err := s.removeImportedLocalBlobObject(blob); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (s *AdminSpaceImportService) removeImportedLocalBlobObject(blob models.DocumentAttachmentBlob) error {
	if ImageHostingProvider(strings.ToLower(strings.TrimSpace(blob.StorageProvider))) != ImageHostingProviderLocal {
		return nil
	}
	targetPath, err := s.resolveImportedLocalBlobPath(blob.ObjectKey)
	if err != nil {
		return err
	}
	if err := os.Remove(targetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *AdminSpaceImportService) resolveImportedLocalBlobPath(objectKey string) (string, error) {
	cleanObjectKey := cleanAdminSpaceImportZipEntry(strings.TrimLeft(objectKey, "/"))
	if cleanObjectKey == "" || !strings.HasPrefix(cleanObjectKey, "attachments/") {
		return "", errcode.ErrAdminSpaceImportZipInvalid
	}
	root := strings.TrimSpace(s.localBlobRootDir)
	if root == "" {
		root = "uploads"
	}
	targetPath := filepath.Join(root, filepath.FromSlash(cleanObjectKey))
	targetAbsPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", err
	}
	rootAbsPath, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(targetAbsPath, rootAbsPath+string(os.PathSeparator)) && targetAbsPath != rootAbsPath {
		return "", errcode.ErrAdminSpaceImportZipInvalid
	}
	return targetPath, nil
}

func buildAdminSpaceImportObjectKey(
	fileName string,
	contentType string,
	spaceID string,
	documentID string,
	assetID string,
	now time.Time,
) (string, error) {
	extension := adminSpaceImportFileExtension(fileName, contentType)
	replaced, err := RenderImageHostingUploadPathTemplate(
		DefaultImageHostingAttachmentUploadPathTemplate,
		map[string]string{
			"spaceId":    sanitizeAdminSpaceExportPathSegment(spaceID, "space"),
			"docId":      sanitizeAdminSpaceExportPathSegment(documentID, "doc"),
			"yyyy":       fmt.Sprintf("%04d", now.Year()),
			"mm":         fmt.Sprintf("%02d", int(now.Month())),
			"dd":         fmt.Sprintf("%02d", now.Day()),
			"hh":         fmt.Sprintf("%02d", now.Hour()),
			"assetId":    sanitizeAdminSpaceExportPathSegment(assetID, "asset"),
			"origName":   sanitizeAdminSpaceExportPathSegment(adminSpaceImportOriginName(fileName), "file"),
			"ext":        sanitizeAdminSpaceExportPathSegment(extension, "bin"),
			"uploaderId": "space-import",
		},
	)
	if err != nil {
		return "", err
	}
	cleanObjectKey := cleanAdminSpaceImportZipEntry(strings.TrimLeft(replaced, "/"))
	if cleanObjectKey == "" || !strings.HasPrefix(cleanObjectKey, "attachments/") || len(cleanObjectKey) > 512 {
		return "", errcode.ErrAdminSpaceImportZipInvalid
	}
	return cleanObjectKey, nil
}

func adminSpaceImportFileExtension(fileName string, contentType string) string {
	extension := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(path.Ext(fileName)), "."))
	if extension != "" && !strings.ContainsAny(extension, `/\:`) {
		return extension
	}
	extensions, err := mime.ExtensionsByType(strings.TrimSpace(contentType))
	if err == nil && len(extensions) > 0 {
		resolved := strings.TrimPrefix(strings.ToLower(extensions[0]), ".")
		if resolved != "" {
			return resolved
		}
	}
	return "bin"
}

func adminSpaceImportOriginName(fileName string) string {
	base := path.Base(strings.TrimSpace(fileName))
	extension := path.Ext(base)
	name := strings.TrimSuffix(base, extension)
	if strings.TrimSpace(name) == "" {
		return "file"
	}
	return name
}

func detectAdminSpaceImportContentType(payload []byte, fileName string) string {
	if len(payload) > 0 {
		detected := strings.TrimSpace(http.DetectContentType(payload))
		if detected != "" && detected != "application/octet-stream" {
			return detected
		}
	}
	extensionType := strings.TrimSpace(mime.TypeByExtension(path.Ext(fileName)))
	if extensionType != "" {
		return extensionType
	}
	return "application/octet-stream"
}

func resolveAdminSpaceImportOfficeSourceMimeType(format models.DocumentFormat) string {
	switch models.NormalizeDocumentFormat(format) {
	case models.DocumentFormatXLSX:
		return adminSpaceImportMIMEXLSX
	default:
		return adminSpaceImportMIMEDOCX
	}
}

func resolveAdminSpaceImportAttachmentPreviewKind(contentType string, fileName string) string {
	normalizedContentType := strings.ToLower(strings.TrimSpace(contentType))
	extension := strings.ToLower(strings.TrimSpace(path.Ext(fileName)))
	switch {
	case strings.HasPrefix(normalizedContentType, "image/"):
		return "image"
	case normalizedContentType == "application/pdf" || extension == ".pdf":
		return "pdf"
	case strings.HasPrefix(normalizedContentType, "text/"):
		return "text"
	case extension == ".doc" || extension == ".docx" || extension == ".xls" || extension == ".xlsx" || extension == ".ppt" || extension == ".pptx":
		return "office"
	default:
		return "none"
	}
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func trimOptionalUserID(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
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
	request := resolveAdminSpaceImportCommitRequest(staging, input)
	job := AdminSpaceImportJob{
		JobID:                jobID,
		ImportID:             staging.ImportID,
		ActorUserID:          strings.TrimSpace(input.ActorUserID),
		PackageType:          request.packageType,
		RequestedSpaceID:     request.spaceID,
		RequestedSpaceName:   request.spaceName,
		RequestedCategoryID:  request.categoryID,
		RequestedVisibility:  request.visibility,
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
	if err := s.persistImportTransferJobCreated(ctx, job, staging.FilePath); err != nil {
		s.store.Fail(jobID, "persist", "记录导入任务失败", s.now())
		return CommitAdminSpaceImportResult{}, err
	}
	if err := s.recordImportAudit(ctx, job, "space_import", job.ImportID, adminSpaceImportAuditQueued, "queued", "", ""); err != nil {
		failedAt := s.now()
		s.store.Fail(jobID, "audit", "记录导入审计失败", failedAt)
		s.persistImportTransferJobFailed(ctx, jobID, "audit", "记录导入审计失败", failedAt)
		return CommitAdminSpaceImportResult{}, err
	}
	if s.canRestoreAdminSpaceImportPackage() {
		go s.runAdminSpaceImportJob(context.WithoutCancel(ctx), jobID)
	}
	return CommitAdminSpaceImportResult{
		JobID:     jobID,
		StreamURL: "/api/admin/space-imports/" + jobID + "/events?token=" + streamToken,
	}, nil
}

type adminSpaceImportCommitRequest struct {
	packageType string
	spaceID     string
	spaceName   string
	categoryID  string
	visibility  string
}

func resolveAdminSpaceImportCommitRequest(
	staging AdminSpaceImportStaging,
	input CommitAdminSpaceImportInput,
) adminSpaceImportCommitRequest {
	request := adminSpaceImportCommitRequest{
		packageType: strings.TrimSpace(staging.PackageType),
		spaceID:     strings.TrimSpace(input.SpaceID),
		spaceName:   strings.TrimSpace(input.SpaceName),
		categoryID:  strings.TrimSpace(input.CategoryID),
		visibility:  strings.TrimSpace(input.Visibility),
	}
	if request.packageType == AdminSpaceImportPackageTypeEPUB {
		// EPUB 导入永远创建新空间，客户端传入的 spaceId 只能用于旧空间覆盖场景，这里必须忽略。
		request.spaceID = ""
		if request.spaceName == "" {
			request.spaceName = strings.TrimSpace(staging.Space.Name)
		}
	}
	return request
}

func (s *AdminSpaceImportService) canRestoreAdminSpaceImportPackage() bool {
	return s != nil && s.workspaceWriter != nil
}

// CanImportSpace 判断 actor 是否具备导入 zip 创建新空间的能力。
func (s *AdminSpaceImportService) CanImportSpace(ctx context.Context, actorUserID string) (bool, error) {
	if s == nil {
		return false, nil
	}
	if s.canImportSpace != nil {
		return s.canImportSpace(ctx, strings.TrimSpace(actorUserID))
	}
	return s.defaultCanImportSpace(ctx, actorUserID)
}

func (s *AdminSpaceImportService) defaultCanImportSpace(_ context.Context, actorUserID string) (bool, error) {
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

// IssueStreamURL 为当前 actor 的活跃导入任务重新签发 SSE 订阅 URL。
func (s *AdminSpaceImportService) IssueStreamURL(
	_ context.Context,
	actorUserID string,
	jobID string,
) (string, error) {
	if s == nil || s.store == nil {
		return "", errcode.ErrAdminSpaceImportStagingNotFound
	}
	streamToken, streamTokenHash, err := generateAdminSpaceTransferToken()
	if err != nil {
		return "", err
	}
	job, err := s.store.IssueStreamToken(
		strings.TrimSpace(jobID),
		strings.TrimSpace(actorUserID),
		streamTokenHash,
		s.now(),
	)
	if err != nil {
		return "", err
	}
	return "/api/admin/space-imports/" + job.JobID + "/events?token=" + streamToken, nil
}

// PublishProgress 广播导入任务进度。
func (s *AdminSpaceImportService) PublishProgress(jobID string, event AdminSpaceTransferEvent) {
	if s == nil || s.store == nil {
		return
	}
	now := s.now()
	trimmedJobID := strings.TrimSpace(jobID)
	s.store.Publish(trimmedJobID, event, now)
	if event.Type == AdminSpaceTransferEventTypeProgress {
		s.persistImportTransferJobProgress(context.Background(), trimmedJobID, event, now)
	}
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
		failedAt := s.now()
		s.store.Fail(job.JobID, "permission", "导入权限已失效", failedAt)
		s.persistImportTransferJobFailed(ctx, job.JobID, "permission", "导入权限已失效", failedAt)
		return errcode.ErrAdminSpaceImportCommitForbidden
	}
	now := s.now()
	if err := s.store.MarkRunning(job.JobID, now); err != nil {
		return err
	}
	s.persistImportTransferJobProgress(ctx, job.JobID, AdminSpaceTransferEvent{
		Stage:    "running",
		Progress: 0,
		Message:  "导入任务开始执行",
	}, now)
	return nil
}

func (s *AdminSpaceImportService) runAdminSpaceImportJob(ctx context.Context, jobID string) {
	if err := s.BeginImportJob(ctx, jobID); err != nil {
		if job, getErr := s.store.GetJob(strings.TrimSpace(jobID)); getErr == nil {
			_ = s.recordImportAudit(ctx, job, "space_import", job.ImportID, adminSpaceImportAuditFailed, "permission", err.Error(), "")
		}
		return
	}
	job, err := s.store.GetJob(strings.TrimSpace(jobID))
	if err != nil {
		failedAt := s.now()
		s.store.Fail(jobID, "load", "导入任务不存在", failedAt)
		s.persistImportTransferJobFailed(ctx, jobID, "load", "导入任务不存在", failedAt)
		return
	}
	newSpaceID, err := s.restoreAdminSpaceImportJob(ctx, job)
	if err != nil {
		failureMessage := err.Error()
		failedAt := s.now()
		s.store.Fail(jobID, "restore", failureMessage, failedAt)
		s.persistImportTransferJobFailed(ctx, jobID, "restore", failureMessage, failedAt)
		targetType, targetID := adminSpaceImportAuditTarget(job, newSpaceID)
		_ = s.recordImportAudit(ctx, job, targetType, targetID, adminSpaceImportAuditFailed, "restore", failureMessage, newSpaceID)
		return
	}
	completedAt := s.now()
	s.store.Complete(jobID, newSpaceID, completedAt)
	s.persistImportTransferJobCompleted(ctx, strings.TrimSpace(jobID), newSpaceID, completedAt)
	_ = s.recordImportAudit(ctx, job, "space", newSpaceID, adminSpaceImportAuditSuccess, "completed", "", newSpaceID)
}

func (s *AdminSpaceImportService) restoreAdminSpaceImportJob(ctx context.Context, job AdminSpaceImportJob) (string, error) {
	switch strings.TrimSpace(job.PackageType) {
	case AdminSpaceImportPackageTypeEPUB:
		return s.restoreAdminSpaceImportEPUBPackage(ctx, job)
	default:
		return s.restoreAdminSpaceImportPackage(ctx, job)
	}
}

func (s *AdminSpaceImportService) restoreAdminSpaceImportEPUBPackage(
	ctx context.Context,
	job AdminSpaceImportJob,
) (string, error) {
	if s == nil || s.workspaceWriter == nil {
		return "", fmt.Errorf("导入服务依赖未配置")
	}
	if ok, err := s.CanImportSpace(ctx, job.ActorUserID); err != nil {
		return "", err
	} else if !ok {
		return "", errcode.ErrAdminSpaceImportCommitForbidden
	}
	staging, err := s.store.GetStaging(job.ImportID, job.ActorUserID, s.now())
	if err != nil {
		return "", err
	}
	if !staging.Importable {
		return "", errcode.ErrAdminSpaceImportPackageNotImportable
	}

	s.publishImportProgress(ctx, job, "epub_parse", 10, "正在解析 EPUB 包")
	epubPackage, err := readAdminSpaceEPUBImportPackage(staging.FilePath)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := epubPackage.Close(); closeErr != nil {
			slog.WarnContext(ctx, "关闭 EPUB 导入包失败", "importID", job.ImportID, "error", closeErr)
		}
	}()

	plan, planWarnings := planAdminSpaceEPUBImportTree(adminSpaceEPUBPlanInput{
		OPFRoot:                    epubPackage.OPFRoot,
		Items:                      epubPackage.NavItems,
		ChapterHTMLByCanonicalHref: epubPackage.ChapterHTMLByCanonicalHref,
		NewNodeID: func() string {
			return strings.ToLower(ulid.Make().String())
		},
		NewDocumentID: func() string {
			return strings.ToLower(ulid.Make().String())
		},
	})
	if len(plan.Root) == 0 {
		return "", errcode.ErrAdminSpaceImportPackageNotImportable
	}
	for _, warning := range planWarnings {
		slog.WarnContext(ctx, "EPUB 目录规划出现可恢复 warning",
			"jobID", strings.TrimSpace(job.JobID),
			"importID", strings.TrimSpace(job.ImportID),
			"warning", warning,
		)
	}

	s.publishImportProgress(ctx, job, "epub_space", 30, "正在创建 EPUB 导入空间")
	newSpace, coverAsset, err := s.createImportedEPUBSpace(ctx, job, staging, epubPackage)
	if err != nil {
		return "", err
	}
	importer := adminSpaceEPUBPackageImporter{
		service:        s,
		job:            job,
		pkg:            epubPackage,
		newSpaceID:     newSpace.SpaceID,
		plan:           plan,
		converter:      NewHTMLMarkdownConverter(),
		totalDocuments: max(1, countAdminSpaceEPUBPlannedDocuments(plan.Root)),
		createdBlobs:   make([]models.DocumentAttachmentBlob, 0),
	}
	// 文档写入阶段使用“已导入文档数 / 总文档数”推进；这里先发布阶段起点，后续每写入一个文档再递增。
	s.publishImportProgress(ctx, job, "epub_convert", importer.documentProgress(0), fmt.Sprintf("正在转换并导入 EPUB 文档（0/%d）", importer.totalDocuments))
	if err := importer.restoreTree(ctx, plan.Root, nil); err != nil {
		if cleanupErr := s.cleanupFailedImportSpace(ctx, newSpace.SpaceID, importer.createdBlobs, coverAsset); cleanupErr != nil {
			return newSpace.SpaceID, fmt.Errorf("%v；EPUB 导入失败后清理已创建资源: %w", err, cleanupErr)
		}
		return newSpace.SpaceID, err
	}
	s.publishImportProgress(ctx, job, "epub_done", 95, "EPUB 导入写入完成")
	s.store.SaveMappings(job.JobID, AdminSpaceImportIDMappings{
		SpaceIDMappings: map[string]string{
			strings.TrimSpace(staging.Space.SpaceID): newSpace.SpaceID,
		},
		NodeIDMappings:     collectAdminSpaceEPUBNodeIDMappings(plan.Root),
		DocumentIDMappings: collectAdminSpaceEPUBDocumentIDMappings(plan.Root),
	}, s.now())
	s.removeCompletedImportStaging(ctx, staging)
	slog.InfoContext(ctx, "EPUB 导入任务写库完成",
		"jobID", strings.TrimSpace(job.JobID),
		"importID", strings.TrimSpace(job.ImportID),
		"spaceID", newSpace.SpaceID,
		"sourceAuthors", epubPackage.Authors,
		"sourcePublishedAt", epubPackage.PublishedAt,
		"planWarningCount", len(planWarnings),
	)
	return newSpace.SpaceID, nil
}

func (s *AdminSpaceImportService) publishImportProgress(
	ctx context.Context,
	job AdminSpaceImportJob,
	stage string,
	progress int,
	message string,
) {
	if s == nil {
		return
	}
	normalizedJobID := strings.TrimSpace(job.JobID)
	if normalizedJobID == "" {
		return
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 99 {
		progress = 99
	}
	normalizedStage := strings.TrimSpace(stage)
	normalizedMessage := strings.TrimSpace(message)
	s.PublishProgress(normalizedJobID, AdminSpaceTransferEvent{
		Type:     AdminSpaceTransferEventTypeProgress,
		Stage:    normalizedStage,
		Progress: progress,
		Message:  normalizedMessage,
	})
	slog.InfoContext(ctx, "空间导入任务进度",
		"jobID", normalizedJobID,
		"importID", strings.TrimSpace(job.ImportID),
		"packageType", strings.TrimSpace(job.PackageType),
		"stage", normalizedStage,
		"progress", progress,
		"message", normalizedMessage,
	)
}

func readAdminSpaceEPUBImportPackage(filePath string) (adminSpaceEPUBImportPackage, error) {
	reader, err := zip.OpenReader(strings.TrimSpace(filePath))
	if err != nil {
		return adminSpaceEPUBImportPackage{}, errcode.ErrAdminSpaceImportZipInvalid
	}
	closed := false
	closeOnError := func() {
		if !closed {
			_ = reader.Close()
			closed = true
		}
	}

	entries, err := collectAdminSpaceEPUBEntries(&reader.Reader)
	if err != nil {
		closeOnError()
		return adminSpaceEPUBImportPackage{}, err
	}
	mimetypePayload, err := readAdminSpaceEPUBMetadataFile(entries["mimetype"])
	if err != nil || strings.TrimSpace(string(mimetypePayload)) != adminSpaceEPUBMimetype {
		closeOnError()
		return adminSpaceEPUBImportPackage{}, errcode.ErrAdminSpaceImportZipInvalid
	}
	containerPayload, err := readAdminSpaceEPUBMetadataFile(entries["META-INF/container.xml"])
	if err != nil {
		closeOnError()
		return adminSpaceEPUBImportPackage{}, errcode.ErrAdminSpaceImportZipInvalid
	}
	var container adminSpaceEPUBContainer
	warnings := []string{}
	if err := unmarshalAdminSpaceEPUBXML(containerPayload, &container, &warnings, "META-INF/container.xml"); err != nil {
		closeOnError()
		return adminSpaceEPUBImportPackage{}, errcode.ErrAdminSpaceImportZipInvalid
	}
	opfPath := firstAdminSpaceEPUBRootfilePath(container)
	if opfPath == "" {
		closeOnError()
		return adminSpaceEPUBImportPackage{}, errcode.ErrAdminSpaceImportZipInvalid
	}
	opfPayload, err := readAdminSpaceEPUBMetadataFile(entries[opfPath])
	if err != nil {
		closeOnError()
		return adminSpaceEPUBImportPackage{}, errcode.ErrAdminSpaceImportZipInvalid
	}
	var opf adminSpaceEPUBOPFPackage
	if err := unmarshalAdminSpaceEPUBXML(opfPayload, &opf, &warnings, opfPath); err != nil {
		closeOnError()
		return adminSpaceEPUBImportPackage{}, errcode.ErrAdminSpaceImportZipInvalid
	}

	opfRoot := path.Dir(opfPath)
	if opfRoot == "." {
		opfRoot = ""
	}
	manifestByID := buildAdminSpaceEPUBManifestByID(opf.Manifest)
	chapterHTMLByCanonicalHref, err := readAdminSpaceEPUBChapterHTML(entries, opfRoot, opf.Manifest)
	if err != nil {
		closeOnError()
		return adminSpaceEPUBImportPackage{}, err
	}
	navItems := readAdminSpaceEPUBNavItems(entries, opfRoot, opf, manifestByID)
	if len(navItems) == 0 {
		navItems = buildAdminSpaceEPUBSpineNavItems(opf, manifestByID)
	}
	if len(navItems) == 0 {
		closeOnError()
		return adminSpaceEPUBImportPackage{}, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	title := firstNonEmptyEPUBString(opf.Metadata.Titles)
	if title == "" {
		title = strings.TrimSuffix(path.Base(strings.TrimSpace(opfPath)), filepath.Ext(opfPath))
	}
	if strings.TrimSpace(title) == "" {
		title = "EPUB 导入空间"
	}
	return adminSpaceEPUBImportPackage{
		closer:                     reader,
		OPFPath:                    opfPath,
		OPFRoot:                    opfRoot,
		Title:                      title,
		Authors:                    compactAdminSpaceEPUBStrings(opf.Metadata.Creators),
		PublishedAt:                firstNonEmptyEPUBString(opf.Metadata.Dates),
		Description:                firstNonEmptyEPUBString(opf.Metadata.Descriptions),
		CoverPath:                  resolveAdminSpaceEPUBCoverPath(opfRoot, opf),
		Entries:                    entries,
		NavItems:                   navItems,
		ChapterHTMLByCanonicalHref: chapterHTMLByCanonicalHref,
	}, nil
}

func (pkg adminSpaceEPUBImportPackage) Close() error {
	if pkg.closer == nil {
		return nil
	}
	return pkg.closer.Close()
}

func buildAdminSpaceEPUBManifestByID(items []adminSpaceEPUBOPFItem) map[string]adminSpaceEPUBOPFItem {
	result := make(map[string]adminSpaceEPUBOPFItem, len(items))
	for _, item := range items {
		if id := strings.TrimSpace(item.ID); id != "" {
			result[id] = item
		}
	}
	return result
}

func readAdminSpaceEPUBChapterHTML(
	entries map[string]*zip.File,
	opfRoot string,
	items []adminSpaceEPUBOPFItem,
) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for _, item := range items {
		if !isAdminSpaceEPUBDocumentItem(item) {
			continue
		}
		canonicalHref := cleanAdminSpaceImportZipEntry(path.Join(strings.TrimSpace(opfRoot), strings.TrimSpace(item.Href)))
		if canonicalHref == "" {
			return nil, errcode.ErrAdminSpaceImportPackageNotImportable
		}
		payload, err := readAdminSpaceEPUBContentFile(entries[canonicalHref])
		if err != nil {
			return nil, err
		}
		result[canonicalHref] = payload
	}
	if len(result) == 0 {
		return nil, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	return result, nil
}

func readAdminSpaceEPUBContentFile(file *zip.File) ([]byte, error) {
	if file == nil {
		return nil, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	if file.UncompressedSize64 > maxAdminSpaceEPUBEntryBytes {
		return nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("打开 EPUB 章节 entry: %w", err)
	}
	defer reader.Close()
	payload, err := io.ReadAll(io.LimitReader(reader, maxAdminSpaceEPUBEntryBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取 EPUB 章节 entry: %w", err)
	}
	if len(payload) == 0 || len(payload) > maxAdminSpaceEPUBEntryBytes {
		return nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	return payload, nil
}

func readAdminSpaceEPUBNavItems(
	entries map[string]*zip.File,
	opfRoot string,
	opf adminSpaceEPUBOPFPackage,
	manifestByID map[string]adminSpaceEPUBOPFItem,
) []adminSpaceEPUBNavItem {
	var navItem adminSpaceEPUBOPFItem
	var tocItem adminSpaceEPUBOPFItem
	for _, item := range opf.Manifest {
		if strings.Contains(" "+strings.TrimSpace(item.Properties)+" ", " nav ") {
			navItem = item
			break
		}
		if isAdminSpaceEPUBTOCItem(item) {
			tocItem = item
		}
	}
	if strings.TrimSpace(navItem.Href) != "" {
		navPath := cleanAdminSpaceImportZipEntry(path.Join(strings.TrimSpace(opfRoot), strings.TrimSpace(navItem.Href)))
		if navPath != "" {
			payload, err := readAdminSpaceEPUBMetadataFile(entries[navPath])
			if err == nil {
				items, parseErr := parseAdminSpaceEPUBNavDocument(payload)
				if parseErr == nil && len(items) > 0 {
					return items
				}
			}
		}
	}
	if strings.TrimSpace(tocItem.Href) != "" {
		tocPath := cleanAdminSpaceImportZipEntry(path.Join(strings.TrimSpace(opfRoot), strings.TrimSpace(tocItem.Href)))
		if tocPath != "" {
			payload, err := readAdminSpaceEPUBMetadataFile(entries[tocPath])
			if err == nil {
				items, parseErr := parseAdminSpaceEPUBTOCDocument(payload)
				if parseErr == nil && len(items) > 0 {
					return items
				}
			}
		}
	}
	return nil
}

func buildAdminSpaceEPUBSpineNavItems(
	opf adminSpaceEPUBOPFPackage,
	manifestByID map[string]adminSpaceEPUBOPFItem,
) []adminSpaceEPUBNavItem {
	items := make([]adminSpaceEPUBNavItem, 0, len(opf.Spine))
	for _, itemRef := range opf.Spine {
		item, ok := manifestByID[strings.TrimSpace(itemRef.IDRef)]
		if !ok || !isAdminSpaceEPUBDocumentItem(item) {
			continue
		}
		title := strings.TrimSuffix(path.Base(strings.TrimSpace(item.Href)), path.Ext(strings.TrimSpace(item.Href)))
		items = append(items, adminSpaceEPUBNavItem{
			Title: title,
			Href:  strings.TrimSpace(item.Href),
		})
	}
	return items
}

type adminSpaceEPUBPackageImporter struct {
	service            *AdminSpaceImportService
	job                AdminSpaceImportJob
	pkg                adminSpaceEPUBImportPackage
	newSpaceID         string
	plan               adminSpaceEPUBPlan
	converter          HTMLMarkdownConverter
	totalDocuments     int
	processedDocuments int
	createdBlobs       []models.DocumentAttachmentBlob
}

func (i *adminSpaceEPUBPackageImporter) restoreTree(
	ctx context.Context,
	nodes []adminSpaceEPUBPlannedNode,
	parentNodeID *string,
) error {
	for index, node := range nodes {
		if err := i.restoreNode(ctx, node, parentNodeID, index); err != nil {
			return err
		}
	}
	return nil
}

func (i *adminSpaceEPUBPackageImporter) restoreNode(
	ctx context.Context,
	planned adminSpaceEPUBPlannedNode,
	parentNodeID *string,
	sort int,
) error {
	if i == nil || i.service == nil || i.service.workspaceWriter == nil {
		return fmt.Errorf("导入服务依赖未配置")
	}
	nodeID := strings.TrimSpace(planned.NodeID)
	if nodeID == "" {
		return errcode.ErrAdminSpaceImportPackageNotImportable
	}
	actorUserID := strings.TrimSpace(i.job.ActorUserID)
	now := i.service.now()
	node := &models.Node{
		NodeID:          nodeID,
		SpaceID:         i.newSpaceID,
		ParentNodeID:    cloneStringPointer(parentNodeID),
		Title:           strings.TrimSpace(planned.Title),
		Sort:            sort,
		CreatedByUserID: trimOptionalUserID(actorUserID),
		UpdatedByUserID: trimOptionalUserID(actorUserID),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	switch planned.Type {
	case adminSpaceEPUBPlannedNodeTypeFolder:
		node.Type = models.NodeTypeFolder
		if err := i.service.workspaceWriter.CreateNode(ctx, repository.WorkspaceCreateNodeParams{
			Node:       node,
			TouchSpace: i.newSpaceID,
			TouchedAt:  now,
		}); err != nil {
			return err
		}
		return i.restoreTree(ctx, planned.Children, &nodeID)
	case adminSpaceEPUBPlannedNodeTypeDoc:
		node.Type = models.NodeTypeDoc
		document, revision, createdBlobs, warnings, err := i.buildDocument(ctx, planned, nodeID, now)
		if err != nil {
			return err
		}
		i.createdBlobs = append(i.createdBlobs, createdBlobs...)
		for _, warning := range warnings {
			slog.WarnContext(ctx, "EPUB 章节导入出现可恢复 warning",
				"jobID", strings.TrimSpace(i.job.JobID),
				"importID", strings.TrimSpace(i.job.ImportID),
				"spaceID", i.newSpaceID,
				"documentID", document.DocumentID,
				"title", planned.Title,
				"warning", warning,
			)
		}
		if err := i.service.workspaceWriter.CreateNode(ctx, repository.WorkspaceCreateNodeParams{
			Node:       node,
			Document:   document,
			Revision:   revision,
			TouchSpace: i.newSpaceID,
			TouchedAt:  now,
		}); err != nil {
			return err
		}
		i.markDocumentImported(ctx, planned.Title)
		return nil
	default:
		return errcode.ErrAdminSpaceImportPackageNotImportable
	}
}

func (i *adminSpaceEPUBPackageImporter) buildDocument(
	ctx context.Context,
	planned adminSpaceEPUBPlannedNode,
	nodeID string,
	now time.Time,
) (*models.Document, *models.DocumentRevision, []models.DocumentAttachmentBlob, []string, error) {
	documentID := strings.TrimSpace(planned.DocumentID)
	if documentID == "" {
		return nil, nil, nil, nil, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	contentMD, warnings, createdBlobs, err := i.buildDocumentMarkdown(ctx, planned, documentID)
	if err != nil {
		return nil, nil, createdBlobs, warnings, err
	}
	actorUserID := strings.TrimSpace(i.job.ActorUserID)
	title := firstNonEmptyString(strings.TrimSpace(planned.Title), "未命名章节")
	document := &models.Document{
		DocumentID:      documentID,
		NodeID:          nodeID,
		ThemeID:         "default",
		Visibility:      resolveAdminSpaceEPUBDocumentVisibility(i.job.RequestedVisibility),
		Status:          models.EntityStatusActive,
		Title:           title,
		Format:          models.DocumentFormatMarkdown,
		ContentMD:       contentMD,
		Version:         1,
		ContentVersion:  1,
		CreatedByUserID: trimOptionalUserID(actorUserID),
		UpdatedByUserID: trimOptionalUserID(actorUserID),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	revision := &models.DocumentRevision{
		DocumentRevisionID: strings.ToLower(ulid.Make().String()),
		DocumentID:         documentID,
		Version:            1,
		ContentMD:          contentMD,
		BaseVersion:        0,
		EditorUserID:       trimOptionalUserID(actorUserID),
		Source:             models.RevisionSourceLocal,
		CreatedAt:          now,
	}
	return document, revision, createdBlobs, warnings, nil
}

func (i *adminSpaceEPUBPackageImporter) buildDocumentMarkdown(
	ctx context.Context,
	planned adminSpaceEPUBPlannedNode,
	documentID string,
) (string, []string, []models.DocumentAttachmentBlob, error) {
	if planned.Reference {
		return strings.TrimSpace(planned.ContentMD), nil, nil, nil
	}
	sourceKey := strings.TrimSpace(planned.TargetKey)
	if sourceKey == "" {
		sourceKey = strings.TrimSpace(planned.CanonicalHref)
	}
	payload := i.pkg.ChapterHTMLByCanonicalHref[strings.TrimSpace(planned.CanonicalHref)]
	if len(payload) == 0 {
		return "", nil, nil, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	sanitizedHTML, sanitizeWarnings, err := sanitizeAdminSpaceEPUBChapterHTML(adminSpaceEPUBHTMLSanitizeInput{
		SourceKey: sourceKey,
		Title:     planned.Title,
		HTML:      payload,
	})
	if err != nil {
		return "", sanitizeWarnings, nil, err
	}
	rewrittenHTML, linkWarnings, err := rewriteAdminSpaceEPUBInternalLinks(adminSpaceEPUBLinkRewriteInput{
		SourceKey:           sourceKey,
		SourceCanonicalHref: planned.CanonicalHref,
		HTML:                []byte(sanitizedHTML),
		Plan:                i.plan,
	})
	if err != nil {
		return "", append(sanitizeWarnings, linkWarnings...), nil, err
	}
	localizedHTML, imageWarnings, createdBlobs, err := i.service.localizeAdminSpaceEPUBChapterImages(ctx, adminSpaceEPUBImageLocalizeInput{
		SourceKey:           sourceKey,
		SourceCanonicalHref: planned.CanonicalHref,
		HTML:                []byte(rewrittenHTML),
		Entries:             i.pkg.Entries,
	}, i.newSpaceID, documentID)
	warnings := append(append(sanitizeWarnings, linkWarnings...), imageWarnings...)
	if err != nil {
		return "", warnings, createdBlobs, err
	}
	result, err := i.converter.Convert(ctx, ConvertHTMLMarkdownInput{
		HTML:         localizedHTML,
		SourceKey:    sourceKey,
		PlainDocMode: true,
	})
	warnings = append(warnings, result.Warnings...)
	if err != nil {
		return "", warnings, createdBlobs, err
	}
	return result.Markdown, warnings, createdBlobs, nil
}

func (i *adminSpaceEPUBPackageImporter) markDocumentImported(ctx context.Context, title string) {
	if i == nil || i.service == nil {
		return
	}
	i.processedDocuments++
	// 文档导入阶段按“已写入文档数 / 总文档数”推进，避免大 EPUB 在章节转换期间长时间停在固定进度。
	i.service.publishImportProgress(ctx, i.job, "epub_documents", i.documentProgress(i.processedDocuments), i.importedDocumentProgressMessage(title))
}

func (i *adminSpaceEPUBPackageImporter) documentProgress(processedDocuments int) int {
	total := i.totalDocuments
	if total <= 0 {
		total = 1
	}
	if processedDocuments < 0 {
		processedDocuments = 0
	}
	if processedDocuments > total {
		processedDocuments = total
	}
	const base = 35
	const span = 55
	return base + processedDocuments*span/total
}

func (i *adminSpaceEPUBPackageImporter) importedDocumentProgressMessage(title string) string {
	total := i.totalDocuments
	if total <= 0 {
		total = 1
	}
	processed := i.processedDocuments
	if processed < 0 {
		processed = 0
	}
	if processed > total {
		processed = total
	}
	message := fmt.Sprintf("已导入 EPUB 文档（%d/%d）", processed, total)
	if trimmedTitle := strings.TrimSpace(title); trimmedTitle != "" {
		message += "：" + trimmedTitle
	}
	return message
}

func resolveAdminSpaceEPUBDocumentVisibility(rawVisibility string) models.Visibility {
	visibility := models.Visibility(strings.TrimSpace(rawVisibility))
	if models.IsValidVisibility(visibility) {
		return visibility
	}
	return models.VisibilityMember
}

func (s *AdminSpaceImportService) createImportedEPUBSpace(
	ctx context.Context,
	job AdminSpaceImportJob,
	staging AdminSpaceImportStaging,
	pkg adminSpaceEPUBImportPackage,
) (*models.Space, *models.SpaceCoverAsset, error) {
	if s == nil || s.workspaceWriter == nil {
		return nil, nil, fmt.Errorf("导入服务依赖未配置")
	}
	name := firstNonEmptyString(
		strings.TrimSpace(job.RequestedSpaceName),
		strings.TrimSpace(staging.Space.Name),
		strings.TrimSpace(pkg.Title),
		"EPUB 导入空间",
	)
	if strings.TrimSpace(name) == "" {
		return nil, nil, errcode.ErrAdminSpaceInvalidName
	}
	visibility := models.Visibility(strings.TrimSpace(job.RequestedVisibility))
	if !models.IsValidVisibility(visibility) {
		visibility = models.Visibility(strings.TrimSpace(staging.Space.Visibility))
	}
	if !models.IsValidVisibility(visibility) {
		visibility = models.VisibilityMember
	}
	category, err := s.resolveImportCategory(ctx, job.RequestedCategoryID)
	if err != nil {
		return nil, nil, err
	}
	now := s.now()
	coverAsset, err := s.createImportedEPUBSpaceCoverAsset(ctx, job, pkg, now)
	if err != nil {
		return nil, nil, err
	}
	space := &models.Space{
		SpaceID:     strings.ToLower(ulid.Make().String()),
		Name:        name,
		Description: buildAdminSpaceEPUBSpaceDescription(pkg),
		CategoryID:  strings.TrimSpace(category.CategoryID),
		Category:    strings.TrimSpace(category.Name),
		OwnerUserID: strings.TrimSpace(job.ActorUserID),
		Visibility:  visibility,
		Status:      models.EntityStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if coverAsset != nil {
		space.CoverAssetID = &coverAsset.AssetID
		space.CoverKey = coverAsset.ObjectKey
		space.CoverURL = coverAsset.ObjectURL
		space.CoverWidth = coverAsset.Width
		space.CoverHeight = coverAsset.Height
		space.CoverSource = coverAsset.Source
	}
	if err := s.workspaceWriter.CreateSpace(ctx, space); err != nil {
		if coverAsset != nil {
			if cleanupErr := removeAdminSpaceCoverObject(coverAsset.ObjectKey); cleanupErr != nil {
				return nil, nil, fmt.Errorf("创建 EPUB 导入空间失败后清理封面对象: %w", cleanupErr)
			}
			if cleanupErr := s.deleteImportedSpaceCoverAsset(ctx, coverAsset.AssetID); cleanupErr != nil {
				return nil, nil, fmt.Errorf("创建 EPUB 导入空间失败后清理封面资产: %w", cleanupErr)
			}
		}
		return nil, nil, err
	}
	return space, coverAsset, nil
}

func buildAdminSpaceEPUBSpaceDescription(pkg adminSpaceEPUBImportPackage) string {
	if description := strings.TrimSpace(pkg.Description); description != "" {
		return description
	}
	parts := make([]string, 0, 2)
	if len(pkg.Authors) > 0 {
		parts = append(parts, "作者："+strings.Join(pkg.Authors, "、"))
	}
	if strings.TrimSpace(pkg.PublishedAt) != "" {
		parts = append(parts, "出版日期："+strings.TrimSpace(pkg.PublishedAt))
	}
	if len(parts) == 0 {
		return "由 EPUB 导入创建。"
	}
	return "由 EPUB 导入创建。" + strings.Join(parts, "；")
}

func (s *AdminSpaceImportService) createImportedEPUBSpaceCoverAsset(
	ctx context.Context,
	job AdminSpaceImportJob,
	pkg adminSpaceEPUBImportPackage,
	now time.Time,
) (*models.SpaceCoverAsset, error) {
	coverPath := strings.TrimSpace(pkg.CoverPath)
	if coverPath == "" {
		return nil, nil
	}
	writer, ok := s.spaceWriter.(adminSpaceImportCoverWriter)
	if !ok || writer == nil {
		return nil, fmt.Errorf("空间封面导入依赖未配置")
	}
	entry := pkg.Entries[coverPath]
	payload, err := readAdminSpaceEPUBImageEntry(entry)
	if err != nil {
		// EPUB 封面是空间装饰信息，源文件缺失或不可识别时跳过封面，不阻断正文导入。
		slog.WarnContext(ctx, "EPUB 封面读取失败，已跳过空间封面导入",
			"jobID", strings.TrimSpace(job.JobID),
			"importID", strings.TrimSpace(job.ImportID),
			"coverPath", coverPath,
			"error", err,
		)
		return nil, nil
	}
	mimeType, coverWidth, coverHeight, err := validateAdminSpaceEPUBCoverPayload(coverPath, payload)
	if err != nil {
		slog.WarnContext(ctx, "EPUB 封面格式不支持，已跳过空间封面导入",
			"jobID", strings.TrimSpace(job.JobID),
			"importID", strings.TrimSpace(job.ImportID),
			"coverPath", coverPath,
			"error", err,
		)
		return nil, nil
	}
	objectKey, err := buildAdminSpaceEPUBCoverObjectKey(now, mimeType)
	if err != nil {
		return nil, err
	}
	if err := saveAdminSpaceCoverObject(objectKey, payload); err != nil {
		return nil, err
	}
	asset := &models.SpaceCoverAsset{
		AssetID:         strings.ToLower(ulid.Make().String()),
		Source:          string(AdminSpaceCoverSourceUserUpload),
		ObjectKey:       objectKey,
		ObjectURL:       resolveAdminSpaceCoverPublicURL(objectKey),
		MimeType:        mimeType,
		Width:           coverWidth,
		Height:          coverHeight,
		SizeBytes:       int64(len(payload)),
		Normalized:      false,
		CreatedByUserID: strings.TrimSpace(job.ActorUserID),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := writer.CreateCoverAsset(ctx, asset); err != nil {
		if cleanupErr := removeAdminSpaceCoverObject(objectKey); cleanupErr != nil {
			return nil, fmt.Errorf("持久化 EPUB 封面资产失败后清理封面对象: %w", cleanupErr)
		}
		return nil, err
	}
	return asset, nil
}

func validateAdminSpaceEPUBCoverPayload(coverPath string, payload []byte) (string, int, int, error) {
	if len(payload) == 0 {
		return "", 0, 0, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	declaredMimeType := strings.TrimSpace(strings.ToLower(adminSpaceEPUBImageContentType(coverPath)))
	detectedMimeType := strings.TrimSpace(strings.ToLower(detectAdminSpaceCoverContentType(payload, declaredMimeType)))
	if detectedMimeType == "image/jpg" {
		detectedMimeType = "image/jpeg"
	}
	if detectedMimeType == "" || !isSupportedAdminSpaceEPUBCoverContentType(detectedMimeType) {
		return "", 0, 0, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(payload))
	if err != nil {
		return "", 0, 0, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	decodedMimeType := adminSpaceEPUBCoverContentTypeForImageFormat(format)
	if decodedMimeType == "" || decodedMimeType != detectedMimeType {
		return "", 0, 0, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	if config.Width <= 0 || config.Height <= 0 {
		return "", 0, 0, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	if config.Width > adminSpaceCoverMaxImageDimension || config.Height > adminSpaceCoverMaxImageDimension {
		return "", 0, 0, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	if int64(config.Width)*int64(config.Height) > adminSpaceCoverMaxPixelCount {
		return "", 0, 0, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	return decodedMimeType, config.Width, config.Height, nil
}

func isSupportedAdminSpaceEPUBCoverContentType(contentType string) bool {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

func adminSpaceEPUBCoverContentTypeForImageFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "png":
		return "image/png"
	case "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	default:
		return ""
	}
}

func buildAdminSpaceEPUBCoverObjectKey(now time.Time, contentType string) (string, error) {
	randomSuffix, err := randomAdminSpaceCoverHex(4)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"%s/%04d/%02d/%02d/%d-%s%s",
		adminSpaceCoverObjectPrefix,
		now.Year(),
		int(now.Month()),
		now.Day(),
		now.UnixMilli(),
		randomSuffix,
		adminSpaceEPUBImageExtensionForContentType(contentType),
	), nil
}

func collectAdminSpaceEPUBNodeIDMappings(nodes []adminSpaceEPUBPlannedNode) map[string]string {
	result := make(map[string]string)
	var walk func([]adminSpaceEPUBPlannedNode)
	walk = func(items []adminSpaceEPUBPlannedNode) {
		for _, item := range items {
			if id := strings.TrimSpace(item.NodeID); id != "" {
				result[id] = id
			}
			walk(item.Children)
		}
	}
	walk(nodes)
	return result
}

func collectAdminSpaceEPUBDocumentIDMappings(nodes []adminSpaceEPUBPlannedNode) map[string]string {
	result := make(map[string]string)
	var walk func([]adminSpaceEPUBPlannedNode)
	walk = func(items []adminSpaceEPUBPlannedNode) {
		for _, item := range items {
			if id := strings.TrimSpace(item.DocumentID); id != "" {
				result[id] = id
			}
			walk(item.Children)
		}
	}
	walk(nodes)
	return result
}

func countAdminSpaceEPUBPlannedDocuments(nodes []adminSpaceEPUBPlannedNode) int {
	count := 0
	var walk func([]adminSpaceEPUBPlannedNode)
	walk = func(items []adminSpaceEPUBPlannedNode) {
		for _, item := range items {
			if item.Type == adminSpaceEPUBPlannedNodeTypeDoc {
				count++
			}
			walk(item.Children)
		}
	}
	walk(nodes)
	return count
}

func (s *AdminSpaceImportService) removeCompletedImportStaging(ctx context.Context, staging AdminSpaceImportStaging) {
	if s == nil {
		return
	}
	if s.store != nil {
		s.store.DeleteStaging(staging.ImportID, staging.ActorUserID)
	}
	deleted, err := removeAdminSpaceTransferFile(strings.TrimSpace(staging.FilePath), s.stagingDir)
	if err != nil {
		slog.WarnContext(ctx, "删除已完成导入 staging 文件失败",
			"importID", strings.TrimSpace(staging.ImportID),
			"filePath", strings.TrimSpace(staging.FilePath),
			"error", err,
		)
		return
	}
	if deleted {
		slog.InfoContext(ctx, "已删除完成导入的 staging 文件",
			"importID", strings.TrimSpace(staging.ImportID),
			"filePath", strings.TrimSpace(staging.FilePath),
		)
	}
}

const (
	adminSpaceImportAuditQueued  = "queued"
	adminSpaceImportAuditSuccess = "success"
	adminSpaceImportAuditFailed  = "failed"
)

func adminSpaceImportAuditTarget(job AdminSpaceImportJob, newSpaceID string) (string, string) {
	if trimmed := strings.TrimSpace(newSpaceID); trimmed != "" {
		return "space", trimmed
	}
	return "space_import", strings.TrimSpace(job.ImportID)
}

func (s *AdminSpaceImportService) recordImportAudit(
	ctx context.Context,
	job AdminSpaceImportJob,
	targetType string,
	targetID string,
	status string,
	stage string,
	message string,
	newSpaceID string,
) error {
	if s == nil || s.auditRecorder == nil {
		return nil
	}
	normalizedTargetType := strings.ToLower(strings.TrimSpace(targetType))
	normalizedTargetID := strings.TrimSpace(targetID)
	if normalizedTargetType == "" || normalizedTargetID == "" {
		return nil
	}
	detail := map[string]any{
		"jobId":               strings.TrimSpace(job.JobID),
		"importId":            strings.TrimSpace(job.ImportID),
		"packageType":         strings.TrimSpace(job.PackageType),
		"requestedSpaceId":    strings.TrimSpace(job.RequestedSpaceID),
		"requestedSpaceName":  strings.TrimSpace(job.RequestedSpaceName),
		"requestedCategoryId": strings.TrimSpace(job.RequestedCategoryID),
		"requestedVisibility": strings.TrimSpace(job.RequestedVisibility),
		"status":              strings.TrimSpace(status),
		"stage":               strings.TrimSpace(stage),
		"abilityType":         "space_create",
	}
	if trimmed := strings.TrimSpace(newSpaceID); trimmed != "" {
		detail["newSpaceId"] = trimmed
	}
	if trimmed := sanitizeAdminSpaceTransferAuditMessage(message, s.stagingDir, defaultAdminSpaceImportStagingDir); trimmed != "" {
		detail["error"] = trimmed
	}
	return s.auditRecorder.Record(ctx, RecordAdminAuditInput{
		ActorUserID: strings.TrimSpace(job.ActorUserID),
		Module:      AdminAuditModuleSpace,
		Action:      AdminAuditActionImport,
		TargetType:  normalizedTargetType,
		TargetID:    normalizedTargetID,
		Summary:     "space import " + strings.TrimSpace(status) + ": " + normalizedTargetID,
		Detail:      detail,
	})
}

func (s *AdminSpaceImportService) persistImportTransferJobCreated(
	ctx context.Context,
	job AdminSpaceImportJob,
	stagingFilePath string,
) error {
	if s == nil || s.transferJobRepo == nil {
		return nil
	}
	return s.transferJobRepo.Create(ctx, &models.AdminSpaceTransferJob{
		JobID:       strings.TrimSpace(job.JobID),
		Kind:        models.AdminSpaceTransferJobKindImport,
		ActorUserID: strings.TrimSpace(job.ActorUserID),
		SpaceID:     strings.TrimSpace(job.RequestedSpaceID),
		SpaceName:   strings.TrimSpace(job.RequestedSpaceName),
		ImportID:    strings.TrimSpace(job.ImportID),
		Status:      models.AdminSpaceTransferJobStatusQueued,
		Stage:       "queued",
		Progress:    0,
		Message:     "导入任务已创建",
		FilePath:    strings.TrimSpace(stagingFilePath),
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
		ExpiresAt:   job.CreatedAt.Add(30 * time.Minute),
	})
}

func (s *AdminSpaceImportService) persistImportTransferJobProgress(
	ctx context.Context,
	jobID string,
	event AdminSpaceTransferEvent,
	now time.Time,
) {
	if s == nil || s.transferJobRepo == nil {
		return
	}
	if now.IsZero() {
		now = s.now()
	}
	_ = s.transferJobRepo.UpdateProgress(ctx, repository.UpdateAdminSpaceTransferJobProgressParams{
		JobID:    strings.TrimSpace(jobID),
		Stage:    strings.TrimSpace(event.Stage),
		Progress: event.Progress,
		Message:  strings.TrimSpace(event.Message),
		Now:      now,
	})
}

func (s *AdminSpaceImportService) persistImportTransferJobCompleted(
	ctx context.Context,
	jobID string,
	newSpaceID string,
	completedAt time.Time,
) {
	if s == nil || s.transferJobRepo == nil {
		return
	}
	if completedAt.IsZero() {
		completedAt = s.now()
	}
	_ = s.transferJobRepo.MarkCompleted(ctx, repository.MarkAdminSpaceTransferJobCompletedParams{
		JobID:       strings.TrimSpace(jobID),
		Stage:       "done",
		Message:     "导入完成",
		NewSpaceID:  strings.TrimSpace(newSpaceID),
		CompletedAt: completedAt,
		ExpiresAt:   completedAt.Add(defaultAdminSpaceTransferTokenTTL),
	})
}

func (s *AdminSpaceImportService) persistImportTransferJobFailed(
	ctx context.Context,
	jobID string,
	stage string,
	message string,
	failedAt time.Time,
) {
	if s == nil || s.transferJobRepo == nil {
		return
	}
	if failedAt.IsZero() {
		failedAt = s.now()
	}
	_ = s.transferJobRepo.MarkFailed(ctx, repository.MarkAdminSpaceTransferJobFailedParams{
		JobID:        strings.TrimSpace(jobID),
		Stage:        strings.TrimSpace(stage),
		Message:      strings.TrimSpace(message),
		ErrorMessage: strings.TrimSpace(message),
		FailedAt:     failedAt,
		ExpiresAt:    failedAt.Add(defaultAdminSpaceTransferTokenTTL),
	})
}

func (s *AdminSpaceImportService) restoreAdminSpaceImportPackage(
	ctx context.Context,
	job AdminSpaceImportJob,
) (string, error) {
	if s == nil || s.workspaceWriter == nil {
		return "", fmt.Errorf("导入服务依赖未配置")
	}
	if ok, err := s.CanImportSpace(ctx, job.ActorUserID); err != nil {
		return "", err
	} else if !ok {
		return "", errcode.ErrAdminSpaceImportCommitForbidden
	}
	staging, err := s.store.GetStaging(job.ImportID, job.ActorUserID, s.now())
	if err != nil {
		return "", err
	}
	if !staging.Importable {
		return "", errcode.ErrAdminSpaceImportPackageNotImportable
	}

	pkg, err := readAdminSpaceImportPackage(staging.FilePath)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := pkg.Close(); closeErr != nil {
			slog.WarnContext(ctx, "关闭导入包失败", "importID", job.ImportID, "error", closeErr)
		}
	}()
	newSpace, coverAsset, err := s.createImportedSpace(ctx, job, pkg)
	if err != nil {
		return "", err
	}
	importer := adminSpacePackageImporter{
		service:             s,
		job:                 job,
		pkg:                 pkg,
		newSpaceID:          newSpace.SpaceID,
		oldToNewNodes:       make(map[string]string),
		oldToNewDocs:        make(map[string]string),
		oldToNewAttachments: make(map[string]string),
		createdCoverAsset:   coverAsset,
	}
	if err := importer.restoreTree(ctx, pkg.Tree.Root, nil); err != nil {
		if cleanupErr := s.cleanupFailedImportSpace(ctx, newSpace.SpaceID, importer.createdBlobs, importer.createdCoverAsset); cleanupErr != nil {
			return newSpace.SpaceID, fmt.Errorf("%v；导入失败后清理已创建资源: %w", err, cleanupErr)
		}
		return newSpace.SpaceID, err
	}
	s.store.SaveMappings(job.JobID, AdminSpaceImportIDMappings{
		SpaceIDMappings: map[string]string{
			strings.TrimSpace(pkg.Manifest.Space.SpaceID): newSpace.SpaceID,
		},
		NodeIDMappings:       importer.oldToNewNodes,
		DocumentIDMappings:   importer.oldToNewDocs,
		AttachmentIDMappings: importer.oldToNewAttachments,
	}, s.now())
	return newSpace.SpaceID, nil
}

func (s *AdminSpaceImportService) createImportedSpace(
	ctx context.Context,
	job AdminSpaceImportJob,
	pkg adminSpaceImportPackage,
) (*models.Space, *models.SpaceCoverAsset, error) {
	manifest := pkg.Manifest
	requestedSpaceID, hasCustomSpaceID, err := normalizeAdminSpaceID(job.RequestedSpaceID)
	if err != nil {
		return nil, nil, err
	}
	spaceID := requestedSpaceID
	if !hasCustomSpaceID {
		spaceID = strings.ToLower(ulid.Make().String())
	}
	name := strings.TrimSpace(job.RequestedSpaceName)
	if name == "" {
		name = strings.TrimSpace(manifest.Space.Name)
	}
	if name == "" {
		return nil, nil, errcode.ErrAdminSpaceInvalidName
	}
	visibility := models.Visibility(strings.TrimSpace(job.RequestedVisibility))
	if !models.IsValidVisibility(visibility) {
		visibility = models.Visibility(strings.TrimSpace(manifest.Space.Visibility))
	}
	if !models.IsValidVisibility(visibility) {
		visibility = models.VisibilityMember
	}
	category, err := s.resolveImportCategory(ctx, job.RequestedCategoryID)
	if err != nil {
		return nil, nil, err
	}
	now := s.now()
	actorUserID := strings.TrimSpace(job.ActorUserID)
	coverAsset, err := s.createImportedSpaceCoverAsset(ctx, job, pkg, now)
	if err != nil {
		return nil, nil, err
	}
	space := &models.Space{
		SpaceID:     spaceID,
		Name:        name,
		Description: strings.TrimSpace(manifest.Space.Description),
		CategoryID:  strings.TrimSpace(category.CategoryID),
		Category:    strings.TrimSpace(category.Name),
		OwnerUserID: actorUserID,
		Visibility:  visibility,
		Status:      models.EntityStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if coverAsset != nil {
		space.CoverAssetID = &coverAsset.AssetID
		space.CoverKey = coverAsset.ObjectKey
		space.CoverURL = coverAsset.ObjectURL
		space.CoverWidth = coverAsset.Width
		space.CoverHeight = coverAsset.Height
		space.CoverSource = coverAsset.Source
	}
	if err := s.workspaceWriter.CreateSpace(ctx, space); err != nil {
		if coverAsset != nil {
			if cleanupErr := removeAdminSpaceCoverObject(coverAsset.ObjectKey); cleanupErr != nil {
				return nil, nil, fmt.Errorf("创建空间失败后清理封面对象: %w", cleanupErr)
			}
			if cleanupErr := s.deleteImportedSpaceCoverAsset(ctx, coverAsset.AssetID); cleanupErr != nil {
				return nil, nil, fmt.Errorf("创建空间失败后清理封面资产: %w", cleanupErr)
			}
		}
		return nil, nil, err
	}
	return space, coverAsset, nil
}

func (s *AdminSpaceImportService) cleanupCreatedImportArtifacts(
	ctx context.Context,
	blobs []models.DocumentAttachmentBlob,
	coverAsset *models.SpaceCoverAsset,
) error {
	var firstErr error
	if coverAsset != nil {
		if err := removeAdminSpaceCoverObject(coverAsset.ObjectKey); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := s.deleteImportedSpaceCoverAsset(ctx, coverAsset.AssetID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := s.cleanupCreatedImportBlobs(ctx, blobs); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (s *AdminSpaceImportService) cleanupFailedImportSpace(
	ctx context.Context,
	spaceID string,
	blobs []models.DocumentAttachmentBlob,
	coverAsset *models.SpaceCoverAsset,
) error {
	var firstErr error
	normalizedSpaceID := strings.TrimSpace(spaceID)
	if normalizedSpaceID != "" {
		if s == nil || s.spaceWriter == nil {
			firstErr = fmt.Errorf("空间回滚依赖未配置")
		} else if _, err := s.spaceWriter.HardDelete(ctx, normalizedSpaceID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := s.cleanupCreatedImportArtifacts(ctx, blobs, coverAsset); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (s *AdminSpaceImportService) deleteImportedSpaceCoverAsset(ctx context.Context, assetID string) error {
	assetID = strings.TrimSpace(assetID)
	if assetID == "" {
		return nil
	}
	deleter, ok := s.spaceWriter.(adminSpaceImportCoverAssetDeleter)
	if !ok || deleter == nil {
		return fmt.Errorf("空间封面资产清理依赖未配置")
	}
	_, err := deleter.DeleteCoverAssetByAssetID(ctx, assetID)
	return err
}

func (s *AdminSpaceImportService) createImportedSpaceCoverAsset(
	ctx context.Context,
	job AdminSpaceImportJob,
	pkg adminSpaceImportPackage,
	now time.Time,
) (*models.SpaceCoverAsset, error) {
	cover := pkg.Manifest.Space.Cover
	if cover == nil {
		return nil, nil
	}
	writer, ok := s.spaceWriter.(adminSpaceImportCoverWriter)
	if !ok || writer == nil {
		return nil, fmt.Errorf("空间封面导入依赖未配置")
	}
	payload, err := readAdminSpaceImportPackageFile(pkg, cover.Path)
	if err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return nil, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	expectedHash := strings.TrimSpace(cover.SHA256)
	if expectedHash == "" {
		return nil, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	sum := sha256.Sum256(payload)
	if !strings.EqualFold(expectedHash, hex.EncodeToString(sum[:])) {
		return nil, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	mimeType, coverWidth, coverHeight, err := validateAdminSpaceImportCoverPayload(cover, payload)
	if err != nil {
		return nil, err
	}
	objectKey, err := buildAdminSpaceCoverObjectKey(now)
	if err != nil {
		return nil, err
	}
	if err := saveAdminSpaceCoverObject(objectKey, payload); err != nil {
		return nil, err
	}
	assetID := strings.ToLower(ulid.Make().String())
	source := string(normalizeAdminSpaceCoverSource(AdminSpaceCoverSource(cover.Source)))
	if source == "" {
		source = string(AdminSpaceCoverSourceUserUpload)
	}
	asset := &models.SpaceCoverAsset{
		AssetID:         assetID,
		Source:          source,
		ObjectKey:       objectKey,
		ObjectURL:       resolveAdminSpaceCoverPublicURL(objectKey),
		MimeType:        mimeType,
		Width:           coverWidth,
		Height:          coverHeight,
		SizeBytes:       int64(len(payload)),
		Normalized:      cover.Normalized,
		CreatedByUserID: strings.TrimSpace(job.ActorUserID),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := writer.CreateCoverAsset(ctx, asset); err != nil {
		if cleanupErr := removeAdminSpaceCoverObject(objectKey); cleanupErr != nil {
			return nil, fmt.Errorf("持久化封面资产失败后清理封面对象: %w", cleanupErr)
		}
		return nil, err
	}
	return asset, nil
}

func validateAdminSpaceImportCoverPayload(
	cover *AdminSpaceExportCoverEntry,
	payload []byte,
) (string, int, int, error) {
	if cover == nil || len(payload) == 0 {
		return "", 0, 0, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	if cover.SizeBytes > 0 && cover.SizeBytes != int64(len(payload)) {
		return "", 0, 0, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	if declaredMimeType := strings.TrimSpace(strings.ToLower(cover.MimeType)); declaredMimeType != "" && declaredMimeType != "image/webp" {
		return "", 0, 0, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	if contentType := detectAdminSpaceCoverContentType(payload, cover.MimeType); strings.TrimSpace(strings.ToLower(contentType)) != "image/webp" {
		return "", 0, 0, errcode.ErrAdminSpaceImportPackageNotImportable
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(payload))
	if err != nil || format != "webp" {
		return "", 0, 0, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	if config.Width <= 0 || config.Height <= 0 {
		return "", 0, 0, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	if config.Width > adminSpaceCoverMaxImageDimension || config.Height > adminSpaceCoverMaxImageDimension {
		return "", 0, 0, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	if int64(config.Width)*int64(config.Height) > adminSpaceCoverMaxPixelCount {
		return "", 0, 0, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	return "image/webp", config.Width, config.Height, nil
}

func readAdminSpaceImportPackageFile(pkg adminSpaceImportPackage, packagePath string) ([]byte, error) {
	entryName := cleanAdminSpaceImportZipEntry(path.Join(pkg.Root, packagePath))
	if entryName == "" {
		return nil, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	file, ok := pkg.Entries[entryName]
	if !ok || file == nil {
		return nil, errcode.ErrAdminSpaceImportPackageNotImportable
	}
	payload, err := readAdminSpaceImportZipPayload(file)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func (s *AdminSpaceImportService) resolveImportCategory(
	ctx context.Context,
	categoryID string,
) (*models.SpaceCategory, error) {
	normalizedCategoryID := strings.TrimSpace(categoryID)
	if normalizedCategoryID != "" && s.categoryReader != nil {
		category, err := s.categoryReader.GetByCategoryID(ctx, normalizedCategoryID)
		if err != nil {
			return nil, err
		}
		return category, nil
	}
	category, err := s.workspaceWriter.GetDefaultCategory(ctx)
	if err != nil {
		return nil, err
	}
	if category == nil || strings.TrimSpace(category.CategoryID) == "" {
		return nil, errcode.ErrAdminSpaceInvalidCategory
	}
	return category, nil
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

func (s *AdminSpaceImportStore) DeleteStaging(importID string, actorUserID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.stagings, importStagingKey(importID, actorUserID))
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
	return cloneAdminSpaceImportJob(*job), nil
}

func (s *AdminSpaceImportStore) SaveMappings(jobID string, mappings AdminSpaceImportIDMappings, now time.Time) {
	if s == nil || strings.TrimSpace(jobID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[strings.TrimSpace(jobID)]
	if !ok || job == nil {
		return
	}
	job.SpaceIDMappings = cloneStringMap(mappings.SpaceIDMappings)
	job.NodeIDMappings = cloneStringMap(mappings.NodeIDMappings)
	job.DocumentIDMappings = cloneStringMap(mappings.DocumentIDMappings)
	job.AttachmentIDMappings = cloneStringMap(mappings.AttachmentIDMappings)
	job.UpdatedAt = now
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
	return job.LastEvent, ch, unsubscribe, nil
}

func (s *AdminSpaceImportStore) IssueStreamToken(
	jobID string,
	actorUserID string,
	tokenHashValue string,
	now time.Time,
) (AdminSpaceImportJob, error) {
	if s == nil || jobID == "" {
		return AdminSpaceImportJob{}, errcode.ErrAdminSpaceImportStagingNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok || job == nil {
		return AdminSpaceImportJob{}, errcode.ErrAdminSpaceImportStagingNotFound
	}
	if job.ActorUserID != actorUserID || !isActiveAdminSpaceImportStatus(job.Status) {
		return AdminSpaceImportJob{}, errcode.ErrAdminSpaceImportJobTokenInvalid
	}
	job.StreamTokenHash = tokenHashValue
	job.StreamTokenExpiresAt = now.Add(defaultAdminSpaceTransferTokenTTL)
	job.UpdatedAt = now
	return *job, nil
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
		publishAdminSpaceImportEventToSubscriber(subscriber, event)
	}
}

func publishAdminSpaceImportEventToSubscriber(subscriber chan AdminSpaceTransferEvent, event AdminSpaceTransferEvent) {
	select {
	case subscriber <- event:
		return
	default:
	}
	if !isTerminalAdminSpaceImportEvent(event) {
		return
	}
	// 终态事件必须优先送达；如果慢客户端塞满了进度缓冲，丢弃一条旧进度以保留 completed/failed。
	select {
	case <-subscriber:
	default:
	}
	select {
	case subscriber <- event:
	default:
	}
}

func isTerminalAdminSpaceImportEvent(event AdminSpaceTransferEvent) bool {
	return event.Type == AdminSpaceTransferEventTypeCompleted || event.Type == AdminSpaceTransferEventTypeFailed
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

func (s *AdminSpaceImportStore) Complete(jobID string, newSpaceID string, now time.Time) {
	if s == nil || jobID == "" {
		return
	}
	s.mu.Lock()
	job, ok := s.jobs[jobID]
	if !ok || job == nil {
		s.mu.Unlock()
		return
	}
	job.Status = AdminSpaceImportStatusCompleted
	job.NewSpaceID = strings.TrimSpace(newSpaceID)
	job.LastEvent = AdminSpaceTransferEvent{
		Type:       AdminSpaceTransferEventTypeCompleted,
		Stage:      "completed",
		Progress:   100,
		Message:    "导入完成",
		SpaceID:    job.NewSpaceID,
		NewSpaceID: job.NewSpaceID,
	}
	job.UpdatedAt = now
	event := job.LastEvent
	s.mu.Unlock()
	s.Publish(jobID, event, now)
}

func (s *AdminSpaceImportStore) DeleteExpired(now time.Time) ([]AdminSpaceImportStaging, []AdminSpaceImportJob) {
	if s == nil {
		return nil, nil
	}
	s.mu.Lock()
	expiredStagings := make([]AdminSpaceImportStaging, 0)
	for key, staging := range s.stagings {
		if staging == nil || now.Before(staging.ExpiresAt) {
			continue
		}
		expiredStagings = append(expiredStagings, *staging)
		delete(s.stagings, key)
	}

	expiredJobs := make([]AdminSpaceImportJob, 0)
	channelsToClose := make([]chan AdminSpaceTransferEvent, 0)
	for jobID, job := range s.jobs {
		if job == nil || !isExpiredAdminSpaceImportJob(*job, now) {
			continue
		}
		expiredJobs = append(expiredJobs, cloneAdminSpaceImportJob(*job))
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
	return expiredStagings, expiredJobs
}

func importStagingKey(importID string, actorUserID string) string {
	return strings.TrimSpace(actorUserID) + ":" + strings.TrimSpace(importID)
}

func isActiveAdminSpaceImportStatus(status AdminSpaceImportStatus) bool {
	return status == AdminSpaceImportStatusQueued || status == AdminSpaceImportStatusRunning
}

func isExpiredAdminSpaceImportJob(job AdminSpaceImportJob, now time.Time) bool {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if isActiveAdminSpaceImportStatus(job.Status) {
		return false
	}
	if !job.StreamTokenExpiresAt.IsZero() {
		return !now.Before(job.StreamTokenExpiresAt)
	}
	return !now.Before(job.UpdatedAt.Add(defaultAdminSpaceTransferTokenTTL))
}

func cloneAdminSpaceImportJob(job AdminSpaceImportJob) AdminSpaceImportJob {
	job.SpaceIDMappings = cloneStringMap(job.SpaceIDMappings)
	job.NodeIDMappings = cloneStringMap(job.NodeIDMappings)
	job.DocumentIDMappings = cloneStringMap(job.DocumentIDMappings)
	job.AttachmentIDMappings = cloneStringMap(job.AttachmentIDMappings)
	return job
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		cloned[trimmedKey] = trimmedValue
	}
	return cloned
}
