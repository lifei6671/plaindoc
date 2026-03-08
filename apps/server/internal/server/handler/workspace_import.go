package handler

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/oklog/ulid/v2"
	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/text/encoding/simplifiedchinese"
	"gorm.io/gorm"

	"github.com/lifei6671/plaindoc/apps/server/internal/server/response"
	"github.com/lifei6671/plaindoc/apps/server/internal/service"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
)

const (
	workspaceImportMultipartMemory      = 32 << 20
	workspaceImportMaxFilesPerRequest   = 200
	workspaceImportMaxFileSizeBytes     = 32 << 20
	workspaceImportMaxZipExpandedBytes  = 128 << 20
	workspaceImportMaxZipEntryCount     = 2000
	workspaceImportMaxZipRecursionDepth = 3
	workspaceImportMaxZipDirectoryDepth = 16
	workspaceImportRootParentMarker     = "__workspace_import_root__"
	importItemStageUnzipping            = "unzipping"
	importItemStageParsing              = "parsing"
	importItemStageConverting           = "converting"
	importItemStageCreatingFolder       = "creating_folder"
	importItemStageCreatingDocument     = "creating_document"
	importItemStageDone                 = "done"
	importItemStatusSuccess             = "success"
	importItemStatusFailed              = "failed"
	importItemStatusSkipped             = "skipped"
	importDetectedTypeMarkdown          = "markdown"
	importDetectedTypeText              = "text"
	importDetectedTypeHTML              = "html"
	importDetectedTypeDOCX              = "docx"
	importDetectedTypeXLSX              = "xlsx"
	importDetectedTypeZIP               = "zip"
	importDetectedTypeDirectory         = "directory"
	importDetectedTypeImage             = "image"
	importDetectedTypeUnsupported       = "unsupported"
)

var (
	errWorkspaceImportInvalidTargetNode = errors.New("workspace import target node is invalid")
	errWorkspaceImportNoFiles           = errors.New("workspace import files are required")
	errWorkspaceImportTooManyFiles      = errors.New("workspace import file count exceeds limit")
	errWorkspaceImportFileTooLarge      = errors.New("workspace import file is too large")
	errWorkspaceImportZipExpandedTooBig = errors.New("workspace import zip expanded content exceeds limit")
	errWorkspaceImportZipEntryLimit     = errors.New("workspace import zip entry count exceeds limit")
	errWorkspaceImportZipPathInvalid    = errors.New("workspace import zip path is invalid")
	errWorkspaceImportTextEncoding      = errors.New("workspace import text encoding is not recognized")
	errWorkspaceImportUnsupportedType   = errors.New("workspace import file type is not supported")
)

type workspaceImportResponse struct {
	TotalCount   int                                  `json:"totalCount"`
	SuccessCount int                                  `json:"successCount"`
	FailedCount  int                                  `json:"failedCount"`
	Items        []workspaceImportItemResponse        `json:"items"`
	FailureItems []workspaceImportItemResponse        `json:"failureItems"`
	CreatedNodes []workspaceImportCreatedNodeResponse `json:"createdNodes"`
}

type workspaceImportItemResponse struct {
	SourceName        string  `json:"sourceName"`
	SourcePath        string  `json:"sourcePath"`
	DetectedType      string  `json:"detectedType"`
	Status            string  `json:"status"`
	Stage             string  `json:"stage"`
	ErrorMessage      string  `json:"errorMessage,omitempty"`
	CreatedNodeID     *string `json:"createdNodeId,omitempty"`
	CreatedDocumentID *string `json:"createdDocumentId,omitempty"`
}

type workspaceImportCreatedNodeResponse struct {
	NodeID     string                 `json:"nodeId"`
	DocumentID *string                `json:"documentId,omitempty"`
	ParentID   *string                `json:"parentId,omitempty"`
	Title      string                 `json:"title"`
	Type       models.NodeType        `json:"type"`
	Format     *models.DocumentFormat `json:"format,omitempty"`
}

type workspaceImportedSource struct {
	SourceName string
	SourcePath string
	Content    []byte
}

type workspaceImportRuntime struct {
	spaceID                       string
	actorUserID                   string
	defaultDocumentVisibility     models.Visibility
	onlyOfficeEnabled             bool
	autoExtractTitle              bool
	rootParentID                  *string
	existingSiblingTitles         map[string]map[string]struct{}
	createdDirectoryByPath        map[string]string
	createdLogicalDirectoryByPath map[string]string
	createdReadmeNodeByDir        map[string]string
	directoryReadmeSourcePath     map[string]string
	importableChildDirsByDir      map[string][]string
	assetSourcesByPath            map[string]workspaceImportedSource
	localizedImageURLBySource     map[string]string
	createdNodes                  []workspaceImportCreatedNodeResponse
	now                           time.Time
}

type workspaceImportConversionResult struct {
	Format         models.DocumentFormat
	Title          string
	ContentMD      string
	SourceContent  []byte
	SourceFileName string
	SourceMimeType string
	DetectedType   string
}

// ImportDocuments 在当前工作区目录树中执行一次性导入。
func (h *workspaceHandler) ImportDocuments(c *gin.Context) {
	actorUserID, ok := h.requireActorUserID(c)
	if !ok {
		return
	}
	if h == nil || h.workspaceRepo == nil {
		response.InternalError(c)
		return
	}

	spaceID := strings.TrimSpace(c.Param("spaceId"))
	if spaceID == "" {
		response.WorkspaceErrSpaceIDRequired.Write(c)
		return
	}

	spaceAccess, err := h.ensureSpaceWritable(c.Request.Context(), spaceID, actorUserID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSpaceNotFound):
			response.WorkspaceErrSpaceNotFound.Write(c)
		case errors.Is(err, service.ErrSpaceAccessDenied):
			response.WorkspaceErrInsufficientSpacePermission.Write(c)
		default:
			response.InternalError(c)
		}
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, workspaceImportMaxZipExpandedBytes+int64(workspaceImportMultipartMemory))
	if err := c.Request.ParseMultipartForm(workspaceImportMultipartMemory); err != nil {
		setRequestErrmsg(c, err, "解析导入表单失败")
		response.Error(c, http.StatusBadRequest, response.CodeInvalidRequest, "导入请求无效")
		return
	}

	targetNodeID := strings.TrimSpace(c.PostForm("targetNodeId"))
	autoExtractTitle, err := parseWorkspaceImportBoolFormValue(c.PostForm("autoExtractTitle"))
	if err != nil {
		setRequestErrmsg(c, err, "解析导入选项失败")
		response.Error(c, http.StatusBadRequest, response.CodeInvalidRequest, "导入选项无效")
		return
	}
	rootParentID, err := h.resolveWorkspaceImportParentID(c.Request.Context(), spaceID, targetNodeID)
	if err != nil {
		setRequestErrmsg(c, err, "解析导入目标节点失败")
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, errWorkspaceImportInvalidTargetNode):
			response.WorkspaceErrNodeNotFound.Write(c)
		default:
			response.InternalError(c)
		}
		return
	}

	fileHeaders := c.Request.MultipartForm.File["files"]
	if len(fileHeaders) == 0 {
		fileHeaders = c.Request.MultipartForm.File["file"]
	}
	if len(fileHeaders) == 0 {
		setRequestErrmsg(c, errWorkspaceImportNoFiles, "导入文件为空")
		response.Error(c, http.StatusBadRequest, response.CodeInvalidRequest, "至少选择一个导入文件")
		return
	}
	if len(fileHeaders) > workspaceImportMaxFilesPerRequest {
		setRequestErrmsg(c, errWorkspaceImportTooManyFiles, "导入文件数量超限")
		response.Error(c, http.StatusBadRequest, response.CodeInvalidRequest, "导入文件数量超过限制")
		return
	}

	onlyOfficeEnabled := false
	if h.onlyOfficeConfigService != nil {
		config, configErr := h.onlyOfficeConfigService.GetConfig(c.Request.Context())
		if configErr != nil {
			setRequestErrmsg(c, configErr, "读取 ONLYOFFICE 配置失败")
			response.InternalError(c)
			return
		}
		onlyOfficeEnabled = config.Enabled
	}

	treeNodes, err := h.workspaceRepo.ListTreeNodesBySpaceID(c.Request.Context(), spaceID)
	if err != nil {
		response.InternalError(c)
		return
	}

	runtimeState := workspaceImportRuntime{
		spaceID:                       spaceID,
		actorUserID:                   actorUserID,
		defaultDocumentVisibility:     spaceAccess.Visibility,
		onlyOfficeEnabled:             onlyOfficeEnabled,
		autoExtractTitle:              autoExtractTitle,
		rootParentID:                  rootParentID,
		existingSiblingTitles:         buildWorkspaceImportSiblingTitleIndex(treeNodes),
		createdDirectoryByPath:        make(map[string]string),
		createdLogicalDirectoryByPath: make(map[string]string),
		createdReadmeNodeByDir:        make(map[string]string),
		directoryReadmeSourcePath:     make(map[string]string),
		importableChildDirsByDir:      make(map[string][]string),
		assetSourcesByPath:            make(map[string]workspaceImportedSource),
		localizedImageURLBySource:     make(map[string]string),
		now:                           time.Now().UTC(),
	}
	if !models.IsValidVisibility(runtimeState.defaultDocumentVisibility) {
		runtimeState.defaultDocumentVisibility = models.VisibilityMember
	}

	sources, assetSources, skippedItems, err := h.collectWorkspaceImportedSources(fileHeaders)
	if err != nil {
		setRequestErrmsg(c, err, "收集导入源文件失败")
		switch {
		case errors.Is(err, errWorkspaceImportFileTooLarge),
			errors.Is(err, errWorkspaceImportZipExpandedTooBig),
			errors.Is(err, errWorkspaceImportZipEntryLimit),
			errors.Is(err, errWorkspaceImportZipPathInvalid),
			errors.Is(err, errWorkspaceImportTooManyFiles):
			response.Error(c, http.StatusBadRequest, response.CodeInvalidRequest, err.Error())
		default:
			response.InternalError(c)
		}
		return
	}
	for _, source := range assetSources {
		runtimeState.assetSourcesByPath[source.SourcePath] = source
		if isWorkspaceImportReadmeSourcePath(source.SourcePath) {
			dirPath := normalizeWorkspaceImportDirPath(path.Dir(source.SourcePath))
			runtimeState.directoryReadmeSourcePath[dirPath] = source.SourcePath
		}
	}
	runtimeState.importableChildDirsByDir = buildWorkspaceImportChildDirIndex(sources)

	items := make([]workspaceImportItemResponse, 0, len(sources)+len(skippedItems))
	items = append(items, skippedItems...)
	sortWorkspaceImportedSources(sources)

	for _, source := range sources {
		item := workspaceImportItemResponse{
			SourceName:   source.SourceName,
			SourcePath:   source.SourcePath,
			DetectedType: detectWorkspaceImportType(source.SourcePath),
			Status:       importItemStatusFailed,
			Stage:        importItemStageParsing,
		}
		createdNodeID, createdDocID, processErr := h.processWorkspaceImportSource(c.Request.Context(), &runtimeState, source, &item)
		if createdNodeID != nil {
			item.CreatedNodeID = createdNodeID
		}
		if createdDocID != nil {
			item.CreatedDocumentID = createdDocID
		}
		if processErr != nil {
			item.ErrorMessage = strings.TrimSpace(processErr.Error())
			items = append(items, item)
			continue
		}
		item.Status = importItemStatusSuccess
		item.Stage = importItemStageDone
		items = append(items, item)
	}

	failureItems := make([]workspaceImportItemResponse, 0)
	successCount := 0
	failedCount := 0
	for _, item := range items {
		if item.Status == importItemStatusSuccess {
			successCount++
			continue
		}
		if item.Status == importItemStatusFailed {
			failedCount++
			failureItems = append(failureItems, item)
		}
	}

	response.JSON(c, http.StatusOK, workspaceImportResponse{
		TotalCount:   len(items),
		SuccessCount: successCount,
		FailedCount:  failedCount,
		Items:        items,
		FailureItems: failureItems,
		CreatedNodes: runtimeState.createdNodes,
	})
}

func parseWorkspaceImportBoolFormValue(rawValue string) (bool, error) {
	normalized := strings.TrimSpace(rawValue)
	if normalized == "" {
		return false, nil
	}
	value, err := strconv.ParseBool(normalized)
	if err != nil {
		return false, err
	}
	return value, nil
}

func (h *workspaceHandler) resolveWorkspaceImportParentID(
	ctx context.Context,
	spaceID string,
	targetNodeID string,
) (*string, error) {
	normalizedSpaceID := strings.TrimSpace(spaceID)
	normalizedTargetNodeID := strings.TrimSpace(targetNodeID)
	if normalizedTargetNodeID == "" {
		return nil, nil
	}
	node, err := h.workspaceRepo.GetNodeByNodeID(ctx, normalizedTargetNodeID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(node.SpaceID) != normalizedSpaceID {
		return nil, errWorkspaceImportInvalidTargetNode
	}
	return &node.NodeID, nil
}

func (h *workspaceHandler) collectWorkspaceImportedSources(
	fileHeaders []*multipart.FileHeader,
) ([]workspaceImportedSource, []workspaceImportedSource, []workspaceImportItemResponse, error) {
	sources := make([]workspaceImportedSource, 0, len(fileHeaders))
	assetSources := make([]workspaceImportedSource, 0, len(fileHeaders))
	skippedItems := make([]workspaceImportItemResponse, 0)
	state := workspaceZipImportState{}

	for _, fileHeader := range fileHeaders {
		if fileHeader == nil {
			continue
		}
		fileName := strings.TrimSpace(fileHeader.Filename)
		if fileName == "" {
			fileName = "未命名文件"
		}
		content, err := readWorkspaceImportFileHeader(fileHeader)
		if err != nil {
			return nil, nil, nil, err
		}
		normalizedPath := sanitizeWorkspaceImportPath(fileName)
		if normalizedPath == "" {
			normalizedPath = "未命名文件"
		}
		if detectWorkspaceImportType(normalizedPath) == importDetectedTypeZIP {
			if err := expandWorkspaceImportZip(content, path.Dir(normalizedPath), path.Base(normalizedPath), 0, &state); err != nil {
				return nil, nil, nil, err
			}
			continue
		}
		state.files = append(state.files, workspaceImportedSource{
			SourceName: path.Base(normalizedPath),
			SourcePath: normalizedPath,
			Content:    content,
		})
	}

	assetSources = append(assetSources, state.files...)
	for _, file := range state.files {
		switch detectWorkspaceImportType(file.SourcePath) {
		case importDetectedTypeUnsupported, importDetectedTypeZIP, importDetectedTypeImage:
			skippedItems = append(skippedItems, workspaceImportItemResponse{
				SourceName:   file.SourceName,
				SourcePath:   file.SourcePath,
				DetectedType: detectWorkspaceImportType(file.SourcePath),
				Status:       importItemStatusSkipped,
				Stage:        importItemStageParsing,
				ErrorMessage: "当前文件类型不会作为文档直接导入",
			})
		default:
			sources = append(sources, file)
		}
	}
	trimPrefix := detectWorkspaceImportRootTrimPrefix(sources)
	if trimPrefix != "" {
		assetSources = trimWorkspaceImportedSourcePaths(assetSources, trimPrefix)
		sources = trimWorkspaceImportedSourcePaths(sources, trimPrefix)
		for index := range skippedItems {
			skippedItems[index].SourcePath = trimWorkspaceImportTrimmedPath(skippedItems[index].SourcePath, trimPrefix)
			skippedItems[index].SourceName = path.Base(skippedItems[index].SourcePath)
		}
	}
	return sources, assetSources, skippedItems, nil
}

type workspaceZipImportState struct {
	files      []workspaceImportedSource
	entryCount int
	totalBytes int64
}

func expandWorkspaceImportZip(
	content []byte,
	parentDir string,
	fileName string,
	depth int,
	state *workspaceZipImportState,
) error {
	if depth > workspaceImportMaxZipRecursionDepth {
		return fmt.Errorf("%w: nested zip depth exceeds limit", errWorkspaceImportZipPathInvalid)
	}
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return err
	}

	containerDir := strings.TrimSpace(strings.TrimSuffix(fileName, path.Ext(fileName)))
	baseDir := strings.Trim(strings.TrimSpace(parentDir), "/")
	if containerDir != "" && depth > 0 {
		if baseDir == "" {
			baseDir = containerDir
		} else {
			baseDir = path.Join(baseDir, containerDir)
		}
	}

	for _, zipFile := range reader.File {
		state.entryCount++
		if state.entryCount > workspaceImportMaxZipEntryCount {
			return errWorkspaceImportZipEntryLimit
		}
		normalizedEntryPath, isDirectory, err := normalizeWorkspaceImportZipEntryPath(baseDir, zipFile.Name)
		if err != nil {
			return err
		}
		if isDirectory {
			continue
		}
		if normalizedEntryPath == "" {
			continue
		}

		fileReader, err := zipFile.Open()
		if err != nil {
			return err
		}
		entryContent, readErr := readWorkspaceImportReaderWithLimit(fileReader, workspaceImportMaxFileSizeBytes)
		_ = fileReader.Close()
		if readErr != nil {
			return readErr
		}
		state.totalBytes += int64(len(entryContent))
		if state.totalBytes > workspaceImportMaxZipExpandedBytes {
			return errWorkspaceImportZipExpandedTooBig
		}
		if detectWorkspaceImportType(normalizedEntryPath) == importDetectedTypeZIP {
			if err := expandWorkspaceImportZip(entryContent, path.Dir(normalizedEntryPath), path.Base(normalizedEntryPath), depth+1, state); err != nil {
				return err
			}
			continue
		}
		state.files = append(state.files, workspaceImportedSource{
			SourceName: path.Base(normalizedEntryPath),
			SourcePath: normalizedEntryPath,
			Content:    entryContent,
		})
	}
	return nil
}

func normalizeWorkspaceImportZipEntryPath(baseDir string, rawPath string) (string, bool, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(rawPath), "\\", "/")
	normalized = strings.TrimPrefix(normalized, "/")
	if normalized == "" {
		return "", false, errWorkspaceImportZipPathInvalid
	}
	isDirectory := strings.HasSuffix(normalized, "/")
	cleaned := path.Clean(strings.TrimSuffix(normalized, "/"))
	if cleaned == "." || cleaned == "" || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return "", false, errWorkspaceImportZipPathInvalid
	}
	if strings.Contains(cleaned, ":") {
		return "", false, errWorkspaceImportZipPathInvalid
	}
	if strings.Count(cleaned, "/") >= workspaceImportMaxZipDirectoryDepth {
		return "", false, errWorkspaceImportZipPathInvalid
	}
	if strings.TrimSpace(baseDir) != "" {
		cleaned = path.Join(strings.Trim(baseDir, "/"), cleaned)
	}
	return cleaned, isDirectory, nil
}

func readWorkspaceImportFileHeader(fileHeader *multipart.FileHeader) ([]byte, error) {
	if fileHeader == nil {
		return nil, errWorkspaceImportNoFiles
	}
	if fileHeader.Size > workspaceImportMaxFileSizeBytes {
		return nil, errWorkspaceImportFileTooLarge
	}
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()
	return readWorkspaceImportReaderWithLimit(file, workspaceImportMaxFileSizeBytes)
}

func readWorkspaceImportReaderWithLimit(reader io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		limit = workspaceImportMaxFileSizeBytes
	}
	limitedReader := &io.LimitedReader{R: reader, N: limit + 1}
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > limit {
		return nil, errWorkspaceImportFileTooLarge
	}
	return content, nil
}

func detectWorkspaceImportType(filePath string) string {
	extension := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(path.Ext(filePath))), ".")
	switch extension {
	case "md", "markdown":
		return importDetectedTypeMarkdown
	case "txt":
		return importDetectedTypeText
	case "html", "htm":
		return importDetectedTypeHTML
	case "docx":
		return importDetectedTypeDOCX
	case "xlsx":
		return importDetectedTypeXLSX
	case "zip":
		return importDetectedTypeZIP
	case "png", "jpg", "jpeg", "gif", "webp", "bmp", "svg":
		return importDetectedTypeImage
	default:
		return importDetectedTypeUnsupported
	}
}

func isWorkspaceImportReadmeSourcePath(filePath string) bool {
	normalized := strings.ToLower(strings.TrimSpace(path.Base(filePath)))
	switch normalized {
	case "readme.md", "readme.markdown", "readme.txt", "readme.html", "readme.mark",
		"readmd.md", "readmd.markdown", "readmd.txt", "readmd.html", "readmd.mark":
		return true
	default:
		return false
	}
}

func sortWorkspaceImportedSources(sources []workspaceImportedSource) {
	sort.SliceStable(sources, func(i int, j int) bool {
		leftDir := normalizeWorkspaceImportDirPath(path.Dir(strings.TrimSpace(sources[i].SourcePath)))
		rightDir := normalizeWorkspaceImportDirPath(path.Dir(strings.TrimSpace(sources[j].SourcePath)))
		leftDepth := strings.Count(leftDir, "/")
		rightDepth := strings.Count(rightDir, "/")
		if leftDir != "" {
			leftDepth++
		}
		if rightDir != "" {
			rightDepth++
		}
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		if leftDir != rightDir {
			return leftDir < rightDir
		}
		leftReadme := isWorkspaceImportReadmeSourcePath(sources[i].SourcePath)
		rightReadme := isWorkspaceImportReadmeSourcePath(sources[j].SourcePath)
		if leftReadme != rightReadme {
			return leftReadme
		}
		return sources[i].SourcePath < sources[j].SourcePath
	})
}

func sanitizeWorkspaceImportPath(rawPath string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(rawPath), "\\", "/")
	normalized = strings.TrimPrefix(normalized, "/")
	if normalized == "" {
		return ""
	}
	cleaned := path.Clean(normalized)
	if cleaned == "." || cleaned == "/" {
		return ""
	}
	return strings.TrimPrefix(cleaned, "/")
}

func detectWorkspaceImportRootTrimPrefix(sources []workspaceImportedSource) string {
	if len(sources) == 0 {
		return ""
	}

	commonPrefixSegments := []string{}
	initialized := false
	for _, source := range sources {
		sourcePath := sanitizeWorkspaceImportPath(source.SourcePath)
		if sourcePath == "" {
			continue
		}
		dirPath := normalizeWorkspaceImportDirPath(path.Dir(sourcePath))

		segments := strings.Split(dirPath, "/")
		if dirPath == "" {
			segments = []string{}
		}
		if !initialized {
			commonPrefixSegments = append(commonPrefixSegments, segments...)
			initialized = true
			continue
		}
		maxCommonLength := min(len(commonPrefixSegments), len(segments))
		matchedLength := 0
		for matchedLength < maxCommonLength && commonPrefixSegments[matchedLength] == segments[matchedLength] {
			matchedLength++
		}
		commonPrefixSegments = commonPrefixSegments[:matchedLength]
	}
	if !initialized || len(commonPrefixSegments) == 0 {
		return ""
	}
	return strings.Join(commonPrefixSegments, "/")
}

func trimWorkspaceImportedSourcePaths(
	sources []workspaceImportedSource,
	trimPrefix string,
) []workspaceImportedSource {
	if trimPrefix == "" || len(sources) == 0 {
		return sources
	}
	trimmedSources := make([]workspaceImportedSource, 0, len(sources))
	for _, source := range sources {
		trimmedPath := trimWorkspaceImportTrimmedPath(source.SourcePath, trimPrefix)
		trimmedSources = append(trimmedSources, workspaceImportedSource{
			SourceName: path.Base(trimmedPath),
			SourcePath: trimmedPath,
			Content:    source.Content,
		})
	}
	return trimmedSources
}

func trimWorkspaceImportTrimmedPath(sourcePath string, trimPrefix string) string {
	normalizedPath := sanitizeWorkspaceImportPath(sourcePath)
	normalizedPrefix := normalizeWorkspaceImportDirPath(trimPrefix)
	if normalizedPrefix == "" {
		return normalizedPath
	}
	trimmedPath := strings.TrimPrefix(normalizedPath, normalizedPrefix+"/")
	if trimmedPath == normalizedPath || strings.TrimSpace(trimmedPath) == "" {
		return normalizedPath
	}
	return trimmedPath
}

func normalizeWorkspaceImportDirPath(rawDirPath string) string {
	normalized := sanitizeWorkspaceImportPath(strings.TrimSpace(rawDirPath))
	if normalized == "" || normalized == "." {
		return ""
	}
	return strings.Trim(normalized, "/")
}

func joinWorkspaceImportDirPath(baseDir string, child string) string {
	baseDir = normalizeWorkspaceImportDirPath(baseDir)
	child = strings.Trim(strings.TrimSpace(child), "/")
	if child == "" {
		return baseDir
	}
	if baseDir == "" {
		return child
	}
	return path.Join(baseDir, child)
}

func parentWorkspaceImportDirPath(dirPath string) string {
	normalized := normalizeWorkspaceImportDirPath(dirPath)
	if normalized == "" {
		return ""
	}
	return normalizeWorkspaceImportDirPath(path.Dir(normalized))
}

func buildWorkspaceImportChildDirIndex(sources []workspaceImportedSource) map[string][]string {
	childDirSetByDir := make(map[string]map[string]struct{})
	for _, source := range sources {
		dirPath := normalizeWorkspaceImportDirPath(path.Dir(strings.TrimSpace(source.SourcePath)))
		if dirPath == "" {
			continue
		}
		segments := strings.Split(dirPath, "/")
		for index, segment := range segments {
			parentDir := ""
			if index > 0 {
				parentDir = strings.Join(segments[:index], "/")
			}
			if _, exists := childDirSetByDir[parentDir]; !exists {
				childDirSetByDir[parentDir] = make(map[string]struct{})
			}
			childDirSetByDir[parentDir][segment] = struct{}{}
		}
	}

	index := make(map[string][]string, len(childDirSetByDir))
	for dirPath, childSet := range childDirSetByDir {
		children := make([]string, 0, len(childSet))
		for child := range childSet {
			children = append(children, child)
		}
		sort.Strings(children)
		index[dirPath] = children
	}
	return index
}

func computeWorkspaceImportLogicalSubdirPath(
	runtimeState *workspaceImportRuntime,
	readmeDirPath string,
	targetDirPath string,
) string {
	if runtimeState == nil {
		return ""
	}
	readmeDirPath = normalizeWorkspaceImportDirPath(readmeDirPath)
	targetDirPath = normalizeWorkspaceImportDirPath(targetDirPath)
	if targetDirPath == "" || targetDirPath == readmeDirPath {
		return ""
	}

	var relativePath string
	if readmeDirPath == "" {
		relativePath = targetDirPath
	} else {
		prefix := readmeDirPath + "/"
		if !strings.HasPrefix(targetDirPath, prefix) {
			return ""
		}
		relativePath = strings.TrimPrefix(targetDirPath, prefix)
	}
	if strings.TrimSpace(relativePath) == "" {
		return ""
	}

	currentDir := readmeDirPath
	preservedSegments := make([]string, 0)
	for _, segment := range strings.Split(relativePath, "/") {
		if strings.TrimSpace(segment) == "" {
			continue
		}
		if len(runtimeState.importableChildDirsByDir[currentDir]) > 1 {
			preservedSegments = append(preservedSegments, segment)
		}
		currentDir = joinWorkspaceImportDirPath(currentDir, segment)
	}
	return strings.Join(preservedSegments, "/")
}

func workspaceImportReadmeOwnsChildDirectories(
	runtimeState *workspaceImportRuntime,
	readmeDirPath string,
) bool {
	if runtimeState == nil {
		return true
	}
	return len(runtimeState.importableChildDirsByDir[normalizeWorkspaceImportDirPath(readmeDirPath)]) <= 1
}

func buildWorkspaceImportSiblingTitleIndex(nodes []repository.WorkspaceTreeNodeRecord) map[string]map[string]struct{} {
	index := make(map[string]map[string]struct{})
	for _, node := range nodes {
		parentKey := workspaceImportParentKey(normalizeOptionalString(node.ParentNodeID))
		if _, exists := index[parentKey]; !exists {
			index[parentKey] = make(map[string]struct{})
		}
		index[parentKey][normalizeWorkspaceImportTitleKey(node.Title)] = struct{}{}
	}
	return index
}

func workspaceImportParentKey(parentID *string) string {
	if parentID == nil {
		return workspaceImportRootParentMarker
	}
	normalized := strings.TrimSpace(*parentID)
	if normalized == "" {
		return workspaceImportRootParentMarker
	}
	return normalized
}

func normalizeWorkspaceImportTitleKey(title string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(title)), " "))
}

func ensureWorkspaceImportUniqueTitle(
	index map[string]map[string]struct{},
	parentID *string,
	desiredTitle string,
) string {
	title := strings.TrimSpace(desiredTitle)
	if title == "" {
		title = "未命名文档"
	}
	parentKey := workspaceImportParentKey(parentID)
	if _, exists := index[parentKey]; !exists {
		index[parentKey] = make(map[string]struct{})
	}

	baseTitle := title
	suffix := 0
	for {
		candidate := baseTitle
		if suffix > 0 {
			candidate = fmt.Sprintf("%s (%d)", baseTitle, suffix)
		}
		key := normalizeWorkspaceImportTitleKey(candidate)
		if _, exists := index[parentKey][key]; exists {
			suffix++
			continue
		}
		index[parentKey][key] = struct{}{}
		return candidate
	}
}

func (h *workspaceHandler) ensureWorkspaceImportDirectory(
	ctx context.Context,
	runtimeState *workspaceImportRuntime,
	relativeDirPath string,
) (string, error) {
	normalizedPath := normalizeWorkspaceImportDirPath(relativeDirPath)
	if normalizedPath == "" {
		if runtimeState.rootParentID == nil {
			return "", nil
		}
		return strings.TrimSpace(*runtimeState.rootParentID), nil
	}
	if existingID, exists := runtimeState.createdDirectoryByPath[normalizedPath]; exists {
		return existingID, nil
	}

	parentRelativePath := parentWorkspaceImportDirPath(normalizedPath)
	var parentNodeID *string
	if parentRelativePath != "" {
		parentID, err := h.resolveWorkspaceImportDirectoryContainerNodeID(ctx, runtimeState, parentRelativePath, parentRelativePath, nil)
		if err != nil {
			return "", err
		}
		parentNodeID = parentID
	} else {
		parentNodeID = runtimeState.rootParentID
	}

	now := runtimeState.now
	title := ensureWorkspaceImportUniqueTitle(runtimeState.existingSiblingTitles, parentNodeID, path.Base(normalizedPath))
	maxSort, err := h.workspaceRepo.GetMaxNodeSort(ctx, runtimeState.spaceID, parentNodeID)
	if err != nil {
		return "", err
	}
	nodeID := strings.ToLower(ulid.Make().String())
	node := &models.Node{
		NodeID:          nodeID,
		SpaceID:         runtimeState.spaceID,
		ParentNodeID:    parentNodeID,
		Type:            models.NodeTypeFolder,
		Title:           title,
		Sort:            maxSort + 1,
		CreatedByUserID: &runtimeState.actorUserID,
		UpdatedByUserID: &runtimeState.actorUserID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := h.workspaceRepo.CreateNode(ctx, repository.WorkspaceCreateNodeParams{
		Node:       node,
		TouchSpace: runtimeState.spaceID,
		TouchedAt:  now,
	}); err != nil {
		return "", err
	}
	runtimeState.createdDirectoryByPath[normalizedPath] = nodeID
	runtimeState.createdNodes = append(runtimeState.createdNodes, workspaceImportCreatedNodeResponse{
		NodeID:   nodeID,
		ParentID: parentNodeID,
		Title:    title,
		Type:     models.NodeTypeFolder,
	})
	return nodeID, nil
}

func (h *workspaceHandler) ensureWorkspaceImportLogicalDirectoryChain(
	ctx context.Context,
	runtimeState *workspaceImportRuntime,
	baseNodeID *string,
	cacheBaseKey string,
	logicalDirPath string,
) (*string, error) {
	logicalDirPath = normalizeWorkspaceImportDirPath(logicalDirPath)
	if logicalDirPath == "" {
		return baseNodeID, nil
	}

	currentParentID := baseNodeID
	currentLogicalPath := ""
	for _, segment := range strings.Split(logicalDirPath, "/") {
		if strings.TrimSpace(segment) == "" {
			continue
		}
		currentLogicalPath = joinWorkspaceImportDirPath(currentLogicalPath, segment)
		cacheKey := cacheBaseKey + "::" + currentLogicalPath
		if existingID, exists := runtimeState.createdLogicalDirectoryByPath[cacheKey]; exists && strings.TrimSpace(existingID) != "" {
			currentParentID = normalizeOptionalString(&existingID)
			continue
		}

		now := runtimeState.now
		title := ensureWorkspaceImportUniqueTitle(runtimeState.existingSiblingTitles, currentParentID, segment)
		maxSort, err := h.workspaceRepo.GetMaxNodeSort(ctx, runtimeState.spaceID, currentParentID)
		if err != nil {
			return nil, err
		}
		nodeID := strings.ToLower(ulid.Make().String())
		node := &models.Node{
			NodeID:          nodeID,
			SpaceID:         runtimeState.spaceID,
			ParentNodeID:    currentParentID,
			Type:            models.NodeTypeFolder,
			Title:           title,
			Sort:            maxSort + 1,
			CreatedByUserID: &runtimeState.actorUserID,
			UpdatedByUserID: &runtimeState.actorUserID,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := h.workspaceRepo.CreateNode(ctx, repository.WorkspaceCreateNodeParams{
			Node:       node,
			TouchSpace: runtimeState.spaceID,
			TouchedAt:  now,
		}); err != nil {
			return nil, err
		}
		runtimeState.createdLogicalDirectoryByPath[cacheKey] = nodeID
		runtimeState.createdNodes = append(runtimeState.createdNodes, workspaceImportCreatedNodeResponse{
			NodeID:   nodeID,
			ParentID: currentParentID,
			Title:    title,
			Type:     models.NodeTypeFolder,
		})
		currentParentID = normalizeOptionalString(&nodeID)
	}
	return currentParentID, nil
}

func (h *workspaceHandler) resolveWorkspaceImportSourceParentNodeID(
	ctx context.Context,
	runtimeState *workspaceImportRuntime,
	sourcePath string,
	item *workspaceImportItemResponse,
) (*string, error) {
	dirPath := normalizeWorkspaceImportDirPath(path.Dir(strings.TrimSpace(sourcePath)))
	if isWorkspaceImportReadmeSourcePath(sourcePath) {
		if dirPath == "" {
			return runtimeState.rootParentID, nil
		}
		return h.resolveWorkspaceImportDirectoryContainerNodeID(ctx, runtimeState, dirPath, parentWorkspaceImportDirPath(dirPath), item)
	}
	return h.resolveWorkspaceImportDirectoryContainerNodeID(ctx, runtimeState, dirPath, dirPath, item)
}

func (h *workspaceHandler) resolveWorkspaceImportDirectoryContainerNodeID(
	ctx context.Context,
	runtimeState *workspaceImportRuntime,
	dirPath string,
	readmeLookupDirPath string,
	item *workspaceImportItemResponse,
) (*string, error) {
	normalizedDirPath := normalizeWorkspaceImportDirPath(dirPath)
	normalizedLookupDirPath := normalizeWorkspaceImportDirPath(readmeLookupDirPath)
	if readmeDirPath, readmePath, hasReadme := findWorkspaceImportNearestReadmeDir(runtimeState.directoryReadmeSourcePath, normalizedLookupDirPath); hasReadme {
		readmeNodeID, exists := runtimeState.createdReadmeNodeByDir[readmeDirPath]
		if !exists || strings.TrimSpace(readmeNodeID) == "" {
			return nil, fmt.Errorf("readme container is not ready for %s", readmePath)
		}
		if normalizedDirPath == readmeDirPath {
			return normalizeOptionalString(&readmeNodeID), nil
		}

		baseNodeID := normalizeOptionalString(&readmeNodeID)
		cacheBaseKey := "readme:" + readmeDirPath
		if !workspaceImportReadmeOwnsChildDirectories(runtimeState, readmeDirPath) {
			if readmeDirPath == "" {
				baseNodeID = runtimeState.rootParentID
			} else {
				readmeParentNodeID, err := h.resolveWorkspaceImportDirectoryContainerNodeID(
					ctx,
					runtimeState,
					readmeDirPath,
					parentWorkspaceImportDirPath(readmeDirPath),
					item,
				)
				if err != nil {
					return nil, err
				}
				baseNodeID = readmeParentNodeID
			}
			cacheBaseKey = "readme-peer:" + readmeDirPath
		}
		logicalDirPath := computeWorkspaceImportLogicalSubdirPath(runtimeState, readmeDirPath, normalizedDirPath)
		return h.ensureWorkspaceImportLogicalDirectoryChain(
			ctx,
			runtimeState,
			baseNodeID,
			cacheBaseKey,
			logicalDirPath,
		)
	}
	if normalizedDirPath == "" {
		return runtimeState.rootParentID, nil
	}
	if item != nil {
		item.Stage = importItemStageCreatingFolder
	}
	folderNodeID, err := h.ensureWorkspaceImportDirectory(ctx, runtimeState, normalizedDirPath)
	if err != nil {
		return nil, err
	}
	return normalizeOptionalString(&folderNodeID), nil
}

func findWorkspaceImportNearestReadmeDir(
	readmeSourcePathByDir map[string]string,
	dirPath string,
) (string, string, bool) {
	currentDirPath := normalizeWorkspaceImportDirPath(dirPath)
	for {
		if readmePath, exists := readmeSourcePathByDir[currentDirPath]; exists {
			return currentDirPath, readmePath, true
		}
		if currentDirPath == "" {
			break
		}
		parentDirPath := parentWorkspaceImportDirPath(currentDirPath)
		if parentDirPath == currentDirPath {
			break
		}
		currentDirPath = parentDirPath
	}
	return "", "", false
}

func (h *workspaceHandler) processWorkspaceImportSource(
	ctx context.Context,
	runtimeState *workspaceImportRuntime,
	source workspaceImportedSource,
	item *workspaceImportItemResponse,
) (*string, *string, error) {
	detectedType := detectWorkspaceImportType(source.SourcePath)
	item.DetectedType = detectedType

	parentNodeID, err := h.resolveWorkspaceImportSourceParentNodeID(ctx, runtimeState, source.SourcePath, item)
	if err != nil {
		return nil, nil, err
	}

	item.Stage = importItemStageConverting
	conversion, err := h.convertWorkspaceImportSource(ctx, runtimeState, source)
	if err != nil {
		return nil, nil, err
	}

	item.Stage = importItemStageCreatingDocument
	switch conversion.Format {
	case models.DocumentFormatMarkdown:
		createdNodeID, createdDocID, createErr := h.createWorkspaceImportedMarkdownDocument(ctx, runtimeState, parentNodeID, conversion, source.SourcePath)
		if createErr == nil && createdNodeID != nil && isWorkspaceImportReadmeSourcePath(source.SourcePath) {
			runtimeState.createdReadmeNodeByDir[normalizeWorkspaceImportDirPath(path.Dir(source.SourcePath))] = strings.TrimSpace(*createdNodeID)
		}
		return createdNodeID, createdDocID, createErr
	case models.DocumentFormatDOCX, models.DocumentFormatXLSX:
		createdNodeID, createdDocID, createErr := h.createWorkspaceImportedOfficeDocument(ctx, runtimeState, parentNodeID, conversion, source.SourcePath)
		if createErr == nil && createdNodeID != nil && isWorkspaceImportReadmeSourcePath(source.SourcePath) {
			runtimeState.createdReadmeNodeByDir[normalizeWorkspaceImportDirPath(path.Dir(source.SourcePath))] = strings.TrimSpace(*createdNodeID)
		}
		return createdNodeID, createdDocID, createErr
	default:
		return nil, nil, errWorkspaceImportUnsupportedType
	}
}

func (h *workspaceHandler) convertWorkspaceImportSource(
	ctx context.Context,
	runtimeState *workspaceImportRuntime,
	source workspaceImportedSource,
) (workspaceImportConversionResult, error) {
	title := strings.TrimSpace(strings.TrimSuffix(path.Base(source.SourcePath), path.Ext(source.SourcePath)))
	if isWorkspaceImportReadmeSourcePath(source.SourcePath) {
		title = "README"
	}
	if title == "" {
		title = "未命名文档"
	}

	switch detectWorkspaceImportType(source.SourcePath) {
	case importDetectedTypeMarkdown:
		content, err := decodeWorkspaceImportText(source.Content)
		if err != nil {
			return workspaceImportConversionResult{}, err
		}
		if runtimeState.autoExtractTitle {
			title = extractWorkspaceImportMarkdownTitle(content, title)
		}
		content, err = h.rewriteWorkspaceImportMarkdownImageSources(ctx, runtimeState, source.SourcePath, content)
		if err != nil {
			return workspaceImportConversionResult{}, err
		}
		return workspaceImportConversionResult{
			Format:       models.DocumentFormatMarkdown,
			Title:        title,
			ContentMD:    content,
			DetectedType: importDetectedTypeMarkdown,
		}, nil
	case importDetectedTypeText:
		content, err := decodeWorkspaceImportText(source.Content)
		if err != nil {
			return workspaceImportConversionResult{}, err
		}
		return workspaceImportConversionResult{
			Format:       models.DocumentFormatMarkdown,
			Title:        title,
			ContentMD:    content,
			DetectedType: importDetectedTypeText,
		}, nil
	case importDetectedTypeHTML:
		content, err := decodeWorkspaceImportText(source.Content)
		if err != nil {
			return workspaceImportConversionResult{}, err
		}
		if runtimeState.autoExtractTitle {
			title = extractWorkspaceImportHTMLTitle(content, title)
		}
		cleanHTML, err := h.normalizeWorkspaceImportedHTML(ctx, runtimeState, source.SourcePath, content)
		if err != nil {
			return workspaceImportConversionResult{}, err
		}
		markdown, err := convertWorkspaceImportHTMLToMarkdown(ctx, cleanHTML)
		if err != nil {
			return workspaceImportConversionResult{}, err
		}
		return workspaceImportConversionResult{
			Format:       models.DocumentFormatMarkdown,
			Title:        title,
			ContentMD:    markdown,
			DetectedType: importDetectedTypeHTML,
		}, nil
	case importDetectedTypeDOCX:
		if runtimeState.onlyOfficeEnabled {
			return workspaceImportConversionResult{
				Format:         models.DocumentFormatDOCX,
				Title:          title,
				SourceContent:  bytes.Clone(source.Content),
				SourceFileName: path.Base(source.SourcePath),
				SourceMimeType: officeDocumentMIMEDOCX,
				DetectedType:   importDetectedTypeDOCX,
			}, nil
		}
		if h.officeHTMLRenderService == nil {
			return workspaceImportConversionResult{}, errors.New("office html render service is unavailable")
		}
		renderedHTML, err := h.officeHTMLRenderService.RenderImportHTML(
			ctx,
			models.DocumentFormatDOCX,
			source.Content,
			runtimeState.spaceID,
			"",
		)
		if err != nil {
			return workspaceImportConversionResult{}, err
		}
		markdown, err := convertWorkspaceImportHTMLToMarkdown(ctx, renderedHTML)
		if err != nil {
			return workspaceImportConversionResult{}, err
		}
		return workspaceImportConversionResult{
			Format:       models.DocumentFormatMarkdown,
			Title:        title,
			ContentMD:    markdown,
			DetectedType: importDetectedTypeDOCX,
		}, nil
	case importDetectedTypeXLSX:
		if runtimeState.onlyOfficeEnabled {
			return workspaceImportConversionResult{
				Format:         models.DocumentFormatXLSX,
				Title:          title,
				SourceContent:  bytes.Clone(source.Content),
				SourceFileName: path.Base(source.SourcePath),
				SourceMimeType: officeDocumentMIMEXLSX,
				DetectedType:   importDetectedTypeXLSX,
			}, nil
		}
		if h.officeHTMLRenderService == nil {
			return workspaceImportConversionResult{}, errors.New("office html render service is unavailable")
		}
		renderedHTML, err := h.officeHTMLRenderService.RenderImportHTML(
			ctx,
			models.DocumentFormatXLSX,
			source.Content,
			runtimeState.spaceID,
			"",
		)
		if err != nil {
			return workspaceImportConversionResult{}, err
		}
		markdown, err := convertWorkspaceImportHTMLToMarkdown(ctx, renderedHTML)
		if err != nil {
			return workspaceImportConversionResult{}, err
		}
		return workspaceImportConversionResult{
			Format:       models.DocumentFormatMarkdown,
			Title:        title,
			ContentMD:    markdown,
			DetectedType: importDetectedTypeXLSX,
		}, nil
	default:
		return workspaceImportConversionResult{}, errWorkspaceImportUnsupportedType
	}
}

func sanitizeWorkspaceImportIdentifierCandidate(rawValue string) string {
	normalized := strings.ToLower(strings.TrimSpace(rawValue))
	if normalized == "" {
		return ""
	}
	var builder strings.Builder
	previousDash := false
	for _, runeValue := range normalized {
		switch {
		case runeValue >= 'a' && runeValue <= 'z', runeValue >= '0' && runeValue <= '9', runeValue == '.', runeValue == '-':
			builder.WriteRune(runeValue)
			previousDash = false
		case runeValue == ' ' || runeValue == '_' || runeValue == '/':
			if !previousDash && builder.Len() > 0 {
				builder.WriteByte('-')
				previousDash = true
			}
		default:
			if !previousDash && builder.Len() > 0 {
				builder.WriteByte('-')
				previousDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-.")
}

func buildWorkspaceImportRandomIdentifier(seed string) string {
	normalizedSeed := sanitizeWorkspaceImportIdentifierCandidate(seed)
	if normalizedSeed == "" {
		normalizedSeed = "import"
	}
	suffix := strings.ToLower(ulid.Make().String()[:8])
	extension := path.Ext(normalizedSeed)
	stem := strings.TrimSuffix(normalizedSeed, extension)
	if stem == "" {
		stem = "import"
	}
	return sanitizeWorkspaceImportIdentifierCandidate(stem + "-" + suffix + extension)
}

func resolveWorkspaceImportReadmeIdentifierCandidate(
	runtimeState *workspaceImportRuntime,
	sourcePath string,
) string {
	dirPath := normalizeWorkspaceImportDirPath(path.Dir(strings.TrimSpace(sourcePath)))
	childDirs := runtimeState.importableChildDirsByDir[dirPath]
	if len(childDirs) == 1 {
		return sanitizeWorkspaceImportIdentifierCandidate(childDirs[0])
	}
	if len(childDirs) > 1 {
		return ""
	}
	if dirPath != "" {
		return sanitizeWorkspaceImportIdentifierCandidate(path.Base(dirPath))
	}
	return ""
}

func resolveWorkspaceImportIdentifierCandidate(
	runtimeState *workspaceImportRuntime,
	sourcePath string,
) *string {
	var candidate string
	if isWorkspaceImportReadmeSourcePath(sourcePath) {
		candidate = resolveWorkspaceImportReadmeIdentifierCandidate(runtimeState, sourcePath)
		if candidate == "" && !workspaceImportReadmeOwnsChildDirectories(runtimeState, path.Dir(sourcePath)) {
			candidate = buildWorkspaceImportRandomIdentifier("readme")
		}
	} else {
		candidate = sanitizeWorkspaceImportIdentifierCandidate(path.Base(strings.TrimSpace(sourcePath)))
	}
	if candidate == "" {
		candidate = buildWorkspaceImportRandomIdentifier(path.Base(strings.TrimSpace(sourcePath)))
	}
	normalizedIdentifier, err := normalizeWorkspaceDocumentIdentifier(&candidate)
	if err != nil || normalizedIdentifier == nil {
		fallback := buildWorkspaceImportRandomIdentifier(path.Base(strings.TrimSpace(sourcePath)))
		normalizedIdentifier, _ = normalizeWorkspaceDocumentIdentifier(&fallback)
	}
	return normalizedIdentifier
}

func extractWorkspaceImportMarkdownTitle(content string, fallbackTitle string) string {
	lines := strings.Split(content, "\n")
	for index := 0; index < len(lines); index++ {
		line := strings.TrimSpace(lines[index])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "# ") {
			title := strings.TrimSpace(strings.TrimPrefix(line, "# "))
			if title != "" {
				return title
			}
			continue
		}
		if index+1 < len(lines) {
			underline := strings.TrimSpace(lines[index+1])
			if underline != "" && strings.Trim(underline, "=") == "" {
				title := strings.TrimSpace(line)
				if title != "" {
					return title
				}
			}
		}
	}
	return fallbackTitle
}

func extractWorkspaceImportHTMLTitle(rawHTML string, fallbackTitle string) string {
	rootNode, err := xhtml.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return fallbackTitle
	}
	if titleNode := findWorkspaceImportHTMLElement(rootNode, atom.Title); titleNode != nil {
		if title := strings.TrimSpace(collectWorkspaceImportNodeText(titleNode)); title != "" {
			return title
		}
	}
	if h1Node := findWorkspaceImportHTMLElement(rootNode, atom.H1); h1Node != nil {
		if title := strings.TrimSpace(collectWorkspaceImportNodeText(h1Node)); title != "" {
			return title
		}
	}
	return fallbackTitle
}

func collectWorkspaceImportNodeText(node *xhtml.Node) string {
	if node == nil {
		return ""
	}
	var builder strings.Builder
	var walk func(current *xhtml.Node)
	walk = func(current *xhtml.Node) {
		if current == nil {
			return
		}
		if current.Type == xhtml.TextNode {
			builder.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return strings.Join(strings.Fields(builder.String()), " ")
}

func decodeWorkspaceImportText(content []byte) (string, error) {
	normalized := bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF})
	if utf8.Valid(normalized) {
		return string(normalized), nil
	}
	decoded, err := simplifiedchinese.GB18030.NewDecoder().Bytes(normalized)
	if err != nil {
		return "", errWorkspaceImportTextEncoding
	}
	if !utf8.Valid(decoded) {
		return "", errWorkspaceImportTextEncoding
	}
	return string(bytes.TrimPrefix(decoded, []byte{0xEF, 0xBB, 0xBF})), nil
}

func (h *workspaceHandler) normalizeWorkspaceImportedHTML(
	ctx context.Context,
	runtimeState *workspaceImportRuntime,
	sourcePath string,
	rawHTML string,
) (string, error) {
	rootNode, err := xhtml.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return "", err
	}

	bodyNode := findWorkspaceImportHTMLElement(rootNode, atom.Body)
	if bodyNode == nil {
		bodyNode = rootNode
	}
	stripWorkspaceImportHTMLNodes(bodyNode)
	if err := h.rewriteWorkspaceImportHTMLImageSources(ctx, runtimeState, sourcePath, bodyNode); err != nil {
		return "", err
	}

	var builder strings.Builder
	for child := bodyNode.FirstChild; child != nil; child = child.NextSibling {
		if renderErr := xhtml.Render(&builder, child); renderErr != nil {
			return "", renderErr
		}
	}
	return builder.String(), nil
}

func findWorkspaceImportHTMLElement(node *xhtml.Node, target atom.Atom) *xhtml.Node {
	if node == nil {
		return nil
	}
	if node.Type == xhtml.ElementNode && node.DataAtom == target {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if matched := findWorkspaceImportHTMLElement(child, target); matched != nil {
			return matched
		}
	}
	return nil
}

func stripWorkspaceImportHTMLNodes(node *xhtml.Node) {
	if node == nil {
		return
	}
	for child := node.FirstChild; child != nil; {
		nextChild := child.NextSibling
		if child.Type == xhtml.ElementNode {
			switch child.DataAtom {
			case atom.Script, atom.Style, atom.Noscript, atom.Form, atom.Input, atom.Button:
				node.RemoveChild(child)
				child = nextChild
				continue
			}
		}
		stripWorkspaceImportHTMLNodes(child)
		child = nextChild
	}
}

func (h *workspaceHandler) rewriteWorkspaceImportHTMLImageSources(
	ctx context.Context,
	runtimeState *workspaceImportRuntime,
	sourcePath string,
	node *xhtml.Node,
) error {
	if node == nil {
		return nil
	}
	if node.Type == xhtml.ElementNode && node.DataAtom == atom.Img {
		for index, attribute := range node.Attr {
			if !strings.EqualFold(attribute.Key, "src") {
				continue
			}
			resolvedSrc, err := h.resolveWorkspaceImportHTMLImageSource(ctx, runtimeState, sourcePath, attribute.Val)
			if err != nil {
				return err
			}
			node.Attr[index].Val = resolvedSrc
			break
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if err := h.rewriteWorkspaceImportHTMLImageSources(ctx, runtimeState, sourcePath, child); err != nil {
			return err
		}
	}
	return nil
}

func (h *workspaceHandler) resolveWorkspaceImportHTMLImageSource(
	ctx context.Context,
	runtimeState *workspaceImportRuntime,
	sourcePath string,
	rawSrc string,
) (string, error) {
	normalizedSrc := strings.TrimSpace(rawSrc)
	if normalizedSrc == "" {
		return normalizedSrc, nil
	}
	return h.resolveWorkspaceImportImageSource(ctx, runtimeState, sourcePath, normalizedSrc)
}

func (h *workspaceHandler) resolveWorkspaceImportImageSource(
	ctx context.Context,
	runtimeState *workspaceImportRuntime,
	sourcePath string,
	rawSrc string,
) (string, error) {
	normalizedSrc := strings.TrimSpace(rawSrc)
	if normalizedSrc == "" {
		return normalizedSrc, nil
	}
	lowerSrc := strings.ToLower(normalizedSrc)
	if strings.HasPrefix(lowerSrc, "data:") {
		return normalizedSrc, nil
	}
	if isRemoteImageURL(normalizedSrc) {
		return h.localizeWorkspaceImportRemoteImageSource(ctx, runtimeState, normalizedSrc)
	}

	resolvedPath := resolveWorkspaceImportRelativeAssetPath(sourcePath, normalizedSrc)
	if resolvedPath == "" {
		return normalizedSrc, nil
	}
	assetSource, exists := runtimeState.assetSourcesByPath[resolvedPath]
	if !exists {
		return normalizedSrc, nil
	}
	if detectWorkspaceImportType(assetSource.SourcePath) != importDetectedTypeImage {
		return normalizedSrc, nil
	}
	return h.localizeWorkspaceImportAssetSource(ctx, runtimeState, assetSource)
}

func (h *workspaceHandler) localizeWorkspaceImportAssetSource(
	ctx context.Context,
	runtimeState *workspaceImportRuntime,
	assetSource workspaceImportedSource,
) (string, error) {
	cacheKey := "asset:" + strings.TrimSpace(assetSource.SourcePath)
	if localizedURL := strings.TrimSpace(runtimeState.localizedImageURLBySource[cacheKey]); localizedURL != "" {
		return localizedURL, nil
	}

	contentType := strings.TrimSpace(mime.TypeByExtension(strings.ToLower(path.Ext(assetSource.SourcePath))))
	if contentType == "" {
		contentType = strings.TrimSpace(http.DetectContentType(assetSource.Content))
	}
	if !strings.HasPrefix(contentType, "image/") {
		return "", nil
	}

	blob, err := h.ensureBlobForContent(
		ctx,
		assetSource.Content,
		contentType,
		path.Base(assetSource.SourcePath),
		runtimeState.spaceID,
		"",
		runtimeState.actorUserID,
		runtimeState.now,
	)
	if err != nil {
		return "", err
	}
	localizedURL := strings.TrimSpace(blob.ObjectURL)
	runtimeState.localizedImageURLBySource[cacheKey] = localizedURL
	return localizedURL, nil
}

func (h *workspaceHandler) localizeWorkspaceImportRemoteImageSource(
	ctx context.Context,
	runtimeState *workspaceImportRuntime,
	rawURL string,
) (string, error) {
	normalizedURL := strings.TrimSpace(rawURL)
	if normalizedURL == "" {
		return normalizedURL, nil
	}

	cacheKey := "remote:" + normalizedURL
	if localizedURL := strings.TrimSpace(runtimeState.localizedImageURLBySource[cacheKey]); localizedURL != "" {
		return localizedURL, nil
	}

	content, contentType, sourceFileName, err := h.fetchRemoteImageWithRetry(ctx, normalizedURL)
	if err != nil {
		return normalizedURL, nil
	}

	blob, err := h.ensureBlobForContent(
		ctx,
		content,
		contentType,
		sourceFileName,
		runtimeState.spaceID,
		"",
		runtimeState.actorUserID,
		runtimeState.now,
	)
	if err != nil {
		return "", err
	}
	localizedURL := strings.TrimSpace(blob.ObjectURL)
	runtimeState.localizedImageURLBySource[cacheKey] = localizedURL
	return localizedURL, nil
}

func (h *workspaceHandler) rewriteWorkspaceImportMarkdownImageSources(
	ctx context.Context,
	runtimeState *workspaceImportRuntime,
	sourcePath string,
	markdownContent string,
) (string, error) {
	normalizedMarkdown := strings.TrimSpace(markdownContent)
	if normalizedMarkdown == "" {
		return markdownContent, nil
	}

	imageURLMapping := make(map[string]string)
	matches := markdownImagePattern.FindAllStringSubmatch(markdownContent, -1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		imageSource := strings.TrimSpace(match[2])
		if imageSource == "" {
			continue
		}
		if _, exists := imageURLMapping[imageSource]; exists {
			continue
		}
		resolvedSource, err := h.resolveWorkspaceImportImageSource(ctx, runtimeState, sourcePath, imageSource)
		if err != nil {
			return "", err
		}
		imageURLMapping[imageSource] = resolvedSource
	}
	if len(imageURLMapping) == 0 {
		return markdownContent, nil
	}

	return markdownImagePattern.ReplaceAllStringFunc(markdownContent, func(fullMatch string) string {
		match := markdownImagePattern.FindStringSubmatch(fullMatch)
		if len(match) < 3 {
			return fullMatch
		}
		altText := match[1]
		imageSource := strings.TrimSpace(match[2])
		mappedURL, exists := imageURLMapping[imageSource]
		if !exists || strings.TrimSpace(mappedURL) == "" || mappedURL == imageSource {
			return fullMatch
		}
		imageTitle := ""
		if len(match) >= 4 {
			imageTitle = strings.TrimSpace(match[3])
		}
		if imageTitle != "" {
			return fmt.Sprintf("![%s](%s \"%s\")", altText, mappedURL, imageTitle)
		}
		return fmt.Sprintf("![%s](%s)", altText, mappedURL)
	}), nil
}

func resolveWorkspaceImportRelativeAssetPath(sourcePath string, rawSrc string) string {
	normalizedSrc := strings.TrimSpace(strings.SplitN(rawSrc, "#", 2)[0])
	normalizedSrc = strings.TrimSpace(strings.SplitN(normalizedSrc, "?", 2)[0])
	if normalizedSrc == "" {
		return ""
	}
	if strings.HasPrefix(normalizedSrc, "/") {
		return sanitizeWorkspaceImportPath(strings.TrimPrefix(normalizedSrc, "/"))
	}
	return sanitizeWorkspaceImportPath(path.Join(path.Dir(strings.TrimSpace(sourcePath)), normalizedSrc))
}

func (h *workspaceHandler) createWorkspaceImportedMarkdownDocument(
	ctx context.Context,
	runtimeState *workspaceImportRuntime,
	parentNodeID *string,
	conversion workspaceImportConversionResult,
	sourcePath string,
) (*string, *string, error) {
	now := time.Now().UTC()
	title := ensureWorkspaceImportUniqueTitle(runtimeState.existingSiblingTitles, parentNodeID, conversion.Title)
	readerSlug := resolveWorkspaceImportIdentifierCandidate(runtimeState, sourcePath)
	maxSort, err := h.workspaceRepo.GetMaxNodeSort(ctx, runtimeState.spaceID, parentNodeID)
	if err != nil {
		return nil, nil, err
	}

	var nodeID string
	var documentID string
	created := false
	for attempt := 0; attempt < 8; attempt++ {
		nodeID = strings.ToLower(ulid.Make().String())
		documentID = nodeID
		node := &models.Node{
			NodeID:          nodeID,
			SpaceID:         runtimeState.spaceID,
			ParentNodeID:    parentNodeID,
			ReaderSlug:      readerSlug,
			Type:            models.NodeTypeDoc,
			Title:           title,
			Sort:            maxSort + 1,
			CreatedByUserID: &runtimeState.actorUserID,
			UpdatedByUserID: &runtimeState.actorUserID,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		document := &models.Document{
			DocumentID:      documentID,
			NodeID:          nodeID,
			ThemeID:         "default",
			Visibility:      runtimeState.defaultDocumentVisibility,
			Status:          models.EntityStatusActive,
			Title:           title,
			Format:          models.DocumentFormatMarkdown,
			ContentMD:       conversion.ContentMD,
			Version:         1,
			ContentVersion:  1,
			CreatedByUserID: &runtimeState.actorUserID,
			UpdatedByUserID: &runtimeState.actorUserID,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		revision := &models.DocumentRevision{
			DocumentRevisionID: strings.ToLower(ulid.Make().String()),
			DocumentID:         documentID,
			Version:            1,
			ContentMD:          conversion.ContentMD,
			BaseVersion:        0,
			EditorUserID:       &runtimeState.actorUserID,
			Source:             models.RevisionSourceRemote,
			CreatedAt:          now,
		}
		if err := h.workspaceRepo.CreateNode(ctx, repository.WorkspaceCreateNodeParams{
			Node:       node,
			Document:   document,
			Revision:   revision,
			TouchSpace: runtimeState.spaceID,
			TouchedAt:  now,
		}); err != nil {
			if readerSlug != nil && isWorkspaceUniqueConstraintError(err) {
				randomized := buildWorkspaceImportRandomIdentifier(derefOptionalString(readerSlug))
				readerSlug, _ = normalizeWorkspaceDocumentIdentifier(&randomized)
				continue
			}
			return nil, nil, err
		}
		created = true
		break
	}
	if !created {
		return nil, nil, errors.New("create imported markdown document exhausted identifier retries")
	}
	if h.documentImageAssetService != nil {
		_ = h.documentImageAssetService.SyncDocumentImageAssets(ctx, service.SyncDocumentImageAssetsInput{
			DocumentID:   documentID,
			SpaceID:      runtimeState.spaceID,
			ContentMD:    conversion.ContentMD,
			ReferencedAt: now,
		})
	}
	runtimeState.createdNodes = append(runtimeState.createdNodes, workspaceImportCreatedNodeResponse{
		NodeID:     nodeID,
		DocumentID: &documentID,
		ParentID:   parentNodeID,
		Title:      title,
		Type:       models.NodeTypeDoc,
		Format:     ptrDocumentFormat(models.DocumentFormatMarkdown),
	})
	return &nodeID, &documentID, nil
}

func (h *workspaceHandler) createWorkspaceImportedOfficeDocument(
	ctx context.Context,
	runtimeState *workspaceImportRuntime,
	parentNodeID *string,
	conversion workspaceImportConversionResult,
	sourcePath string,
) (*string, *string, error) {
	now := time.Now().UTC()
	title := ensureWorkspaceImportUniqueTitle(runtimeState.existingSiblingTitles, parentNodeID, conversion.Title)
	maxSort, err := h.workspaceRepo.GetMaxNodeSort(ctx, runtimeState.spaceID, parentNodeID)
	if err != nil {
		return nil, nil, err
	}
	readerSlug := resolveWorkspaceImportIdentifierCandidate(runtimeState, sourcePath)
	var nodeID string
	var documentID string
	var sourceBlob *models.DocumentAttachmentBlob
	created := false
	for attempt := 0; attempt < 8; attempt++ {
		nodeID = strings.ToLower(ulid.Make().String())
		documentID = nodeID
		sourceBlob, err = h.ensureBlobForContent(
			ctx,
			conversion.SourceContent,
			conversion.SourceMimeType,
			conversion.SourceFileName,
			runtimeState.spaceID,
			documentID,
			runtimeState.actorUserID,
			now,
		)
		if err != nil {
			return nil, nil, err
		}
		node := &models.Node{
			NodeID:          nodeID,
			SpaceID:         runtimeState.spaceID,
			ParentNodeID:    parentNodeID,
			ReaderSlug:      readerSlug,
			Type:            models.NodeTypeDoc,
			Title:           title,
			Sort:            maxSort + 1,
			CreatedByUserID: &runtimeState.actorUserID,
			UpdatedByUserID: &runtimeState.actorUserID,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		document := &models.Document{
			DocumentID:      documentID,
			NodeID:          nodeID,
			ThemeID:         "default",
			Visibility:      runtimeState.defaultDocumentVisibility,
			Status:          models.EntityStatusActive,
			Title:           title,
			Format:          conversion.Format,
			ContentMD:       "",
			Version:         1,
			ContentVersion:  1,
			SourceBlobID:    &sourceBlob.BlobID,
			SourceFileName:  &conversion.SourceFileName,
			SourceMimeType:  &conversion.SourceMimeType,
			CreatedByUserID: &runtimeState.actorUserID,
			UpdatedByUserID: &runtimeState.actorUserID,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		fileRevision := &models.DocumentFileRevision{
			DocumentFileRevisionID: strings.ToLower(ulid.Make().String()),
			DocumentID:             documentID,
			BlobID:                 sourceBlob.BlobID,
			FileName:               conversion.SourceFileName,
			MimeType:               conversion.SourceMimeType,
			Version:                1,
			BaseVersion:            0,
			EditorUserID:           &runtimeState.actorUserID,
			Source:                 models.RevisionSourceRemote,
			CreatedAt:              now,
		}
		if err := h.workspaceRepo.CreateNode(ctx, repository.WorkspaceCreateNodeParams{
			Node:         node,
			Document:     document,
			FileRevision: fileRevision,
			TouchSpace:   runtimeState.spaceID,
			TouchedAt:    now,
		}); err != nil {
			if readerSlug != nil && isWorkspaceUniqueConstraintError(err) {
				randomized := buildWorkspaceImportRandomIdentifier(derefOptionalString(readerSlug))
				readerSlug, _ = normalizeWorkspaceDocumentIdentifier(&randomized)
				continue
			}
			return nil, nil, err
		}
		created = true
		break
	}
	if !created {
		return nil, nil, errors.New("create imported office document exhausted identifier retries")
	}
	if h.officeHTMLRenderService != nil {
		_ = h.officeHTMLRenderService.Enqueue(ctx, service.OfficeHTMLRenderTask{
			DocumentID:     documentID,
			SpaceID:        runtimeState.spaceID,
			Format:         conversion.Format,
			ContentVersion: 1,
			SourceBlobID:   sourceBlob.BlobID,
			SourceContent:  conversion.SourceContent,
		})
	}
	runtimeState.createdNodes = append(runtimeState.createdNodes, workspaceImportCreatedNodeResponse{
		NodeID:     nodeID,
		DocumentID: &documentID,
		ParentID:   parentNodeID,
		Title:      title,
		Type:       models.NodeTypeDoc,
		Format:     ptrDocumentFormat(conversion.Format),
	})
	return &nodeID, &documentID, nil
}

func ptrDocumentFormat(value models.DocumentFormat) *models.DocumentFormat {
	copied := value
	return &copied
}
