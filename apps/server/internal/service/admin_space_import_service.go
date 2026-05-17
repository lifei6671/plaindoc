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
	ImportID       string
	PackageVersion int
	PackageType    string
	ExportedAt     string
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
	ImportID       string
	ActorUserID    string
	FileName       string
	ContentType    string
	FilePath       string
	PackageVersion int
	PackageType    string
	ExportedAt     string
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

	manifest, _, warnings, err := inspectAdminSpaceImportZip(payload)
	if err != nil {
		return AdminSpaceImportInspectResult{}, err
	}

	importID := strings.ToLower(ulid.Make().String())
	now := s.now()
	stagingPath, err := s.writeStagingFile(importID, payload)
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
		Warnings:  warnings,
		CreatedAt: now,
		ExpiresAt: now.Add(defaultAdminSpaceImportStagingTTL),
	}
	s.store.SaveStaging(staging)
	return AdminSpaceImportInspectResult{
		ImportID:       importID,
		PackageVersion: staging.PackageVersion,
		PackageType:    staging.PackageType,
		ExportedAt:     staging.ExportedAt,
		Importable:     staging.Importable,
		Space:          staging.Space,
		Summary:        staging.Summary,
		Warnings:       staging.Warnings,
	}, nil
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
	return strings.EqualFold(filepath.Ext(strings.TrimSpace(fileName)), ".plaindoc")
}

func (s *AdminSpaceImportService) writeStagingFile(importID string, payload []byte) (string, error) {
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
	fileName := sanitizeAdminSpaceExportPathSegment(importID, "import") + ".plaindoc"
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
	job := AdminSpaceImportJob{
		JobID:                jobID,
		ImportID:             staging.ImportID,
		ActorUserID:          strings.TrimSpace(input.ActorUserID),
		RequestedSpaceID:     strings.TrimSpace(input.SpaceID),
		RequestedSpaceName:   strings.TrimSpace(input.SpaceName),
		RequestedCategoryID:  strings.TrimSpace(input.CategoryID),
		RequestedVisibility:  strings.TrimSpace(input.Visibility),
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
	if err := s.recordImportAudit(ctx, job, "space_import", job.ImportID, adminSpaceImportAuditQueued, "queued", "", ""); err != nil {
		s.store.Fail(jobID, "audit", "记录导入审计失败", s.now())
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

func (s *AdminSpaceImportService) runAdminSpaceImportJob(ctx context.Context, jobID string) {
	if err := s.BeginImportJob(ctx, jobID); err != nil {
		if job, getErr := s.store.GetJob(strings.TrimSpace(jobID)); getErr == nil {
			_ = s.recordImportAudit(ctx, job, "space_import", job.ImportID, adminSpaceImportAuditFailed, "permission", err.Error(), "")
		}
		return
	}
	job, err := s.store.GetJob(strings.TrimSpace(jobID))
	if err != nil {
		s.store.Fail(jobID, "load", "导入任务不存在", s.now())
		return
	}
	newSpaceID, err := s.restoreAdminSpaceImportPackage(ctx, job)
	if err != nil {
		failureMessage := err.Error()
		s.store.Fail(jobID, "restore", failureMessage, s.now())
		targetType, targetID := adminSpaceImportAuditTarget(job, newSpaceID)
		_ = s.recordImportAudit(ctx, job, targetType, targetID, adminSpaceImportAuditFailed, "restore", failureMessage, newSpaceID)
		return
	}
	s.store.Complete(jobID, newSpaceID, s.now())
	_ = s.recordImportAudit(ctx, job, "space", newSpaceID, adminSpaceImportAuditSuccess, "completed", "", newSpaceID)
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
		Type:     AdminSpaceTransferEventTypeCompleted,
		Stage:    "completed",
		Progress: 100,
		Message:  "导入完成",
		SpaceID:  job.NewSpaceID,
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
