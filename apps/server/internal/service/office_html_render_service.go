package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"mime"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/repository"
	"github.com/oklog/ulid/v2"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

const (
	defaultOfficeHTMLRenderQueueSize = 64
	defaultOfficeRenderTimeout       = 45 * time.Second
	mammothImagePlaceholderPrefix    = "mammoth-image:"
)

type mammothRenderRequest struct {
	DOCXBase64 string `json:"docxBase64"`
}

type mammothRenderResult struct {
	HTML     string                   `json:"html"`
	Assets   []mammothRenderImage     `json:"assets"`
	Messages []map[string]interface{} `json:"messages"`
}

type mammothRenderImage struct {
	ID          string `json:"id"`
	ContentType string `json:"contentType"`
	DataBase64  string `json:"dataBase64"`
}

type xlsxSheetRenderData struct {
	Key       string
	Name      string
	Active    bool
	HasTable  bool
	TableHTML string
}

type xlsxMergeSpan struct {
	Rowspan int
	Colspan int
	Value   string
}

type xlsxColumnRenderMeta struct {
	Label   string
	WidthPX int
}

// OfficeHTMLRenderTask 表示一次 Office 正文渲染任务。
type OfficeHTMLRenderTask struct {
	DocumentID     string
	SpaceID        string
	Format         models.DocumentFormat
	ContentVersion int
	SourceBlobID   string
	SourceContent  []byte
}

// OfficeHTMLRenderService 负责异步将 Office 源文件转换为当前阅读 HTML。
type OfficeHTMLRenderService struct {
	db                        *gorm.DB
	officeRenderingConfig     *OfficeRenderingConfigService
	imageHostingService       *ImageHostingService
	documentAttachmentRepo    repository.DocumentAttachmentRepository
	documentImageAssetService *DocumentImageAssetService
	searchIndexService        *SearchIndexService
	localRootDir              string

	workerOnce sync.Once
	tasks      chan OfficeHTMLRenderTask
}

// NewOfficeHTMLRenderService 创建 Office HTML 渲染服务。
func NewOfficeHTMLRenderService(
	db *gorm.DB,
	officeRenderingConfig *OfficeRenderingConfigService,
	imageHostingService *ImageHostingService,
	documentAttachmentRepo repository.DocumentAttachmentRepository,
	documentImageAssetService *DocumentImageAssetService,
	searchIndexService *SearchIndexService,
) *OfficeHTMLRenderService {
	return &OfficeHTMLRenderService{
		db:                        db,
		officeRenderingConfig:     officeRenderingConfig,
		imageHostingService:       imageHostingService,
		documentAttachmentRepo:    documentAttachmentRepo,
		documentImageAssetService: documentImageAssetService,
		searchIndexService:        searchIndexService,
		localRootDir:              "uploads",
		tasks:                     make(chan OfficeHTMLRenderTask, defaultOfficeHTMLRenderQueueSize),
	}
}

// Enqueue 提交 Office 渲染任务。
func (s *OfficeHTMLRenderService) Enqueue(_ context.Context, task OfficeHTMLRenderTask) error {
	if s == nil || s.db == nil || s.officeRenderingConfig == nil {
		return errors.New("office html render service dependencies are nil")
	}

	documentID := strings.TrimSpace(task.DocumentID)
	spaceID := strings.TrimSpace(task.SpaceID)
	sourceBlobID := strings.TrimSpace(task.SourceBlobID)
	if documentID == "" || spaceID == "" || sourceBlobID == "" || task.ContentVersion <= 0 {
		return errors.New("office html render task is invalid")
	}
	if !models.IsOfficeDocumentFormat(task.Format) {
		return nil
	}
	if len(task.SourceContent) == 0 {
		return errors.New("office html render source content is empty")
	}

	task.DocumentID = documentID
	task.SpaceID = spaceID
	task.SourceBlobID = sourceBlobID
	task.SourceContent = bytes.Clone(task.SourceContent)

	s.workerOnce.Do(func() {
		go s.runWorker()
	})

	select {
	case s.tasks <- task:
		return nil
	default:
		go s.processTask(task)
		return nil
	}
}

func (s *OfficeHTMLRenderService) runWorker() {
	for task := range s.tasks {
		s.processTask(task)
	}
}

func (s *OfficeHTMLRenderService) processTask(task OfficeHTMLRenderTask) {
	config, err := s.officeRenderingConfig.GetConfig(context.Background())
	if err != nil {
		s.markFailed(task, fmt.Sprintf("load office rendering config failed: %v", err))
		return
	}
	if !config.IndependentRenderEnabled {
		s.markIdle(task)
		return
	}

	timeout := time.Duration(config.RenderTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultOfficeRenderTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	renderedHTML, err := s.renderOfficeHTML(ctx, task)
	if err != nil {
		s.markFailed(task, err.Error())
		return
	}
	renderedHTML = strings.TrimSpace(renderedHTML)
	if renderedHTML == "" {
		s.markFailed(task, "rendered html is empty")
		return
	}

	renderedAt := time.Now().UTC()
	result := s.db.WithContext(ctx).
		Table("documents").
		Where(
			"document_id = ? AND content_version = ? AND source_blob_id = ?",
			task.DocumentID,
			task.ContentVersion,
			task.SourceBlobID,
		).
		Updates(map[string]any{
			"content_md":    renderedHTML,
			"render_status": models.DocumentRenderStatusSuccess,
			"render_error":  "",
			"rendered_at":   renderedAt,
		})
	if result.Error != nil || result.RowsAffected == 0 {
		return
	}

	if s.documentImageAssetService != nil {
		_ = s.documentImageAssetService.SyncDocumentImageAssets(ctx, SyncDocumentImageAssetsInput{
			DocumentID:   task.DocumentID,
			SpaceID:      task.SpaceID,
			ContentMD:    renderedHTML,
			ReferencedAt: renderedAt,
		})
	}
	s.enqueueSearchSync(task.DocumentID)
}

func (s *OfficeHTMLRenderService) markIdle(task OfficeHTMLRenderTask) {
	if s == nil || s.db == nil {
		return
	}
	_ = s.db.WithContext(context.Background()).
		Table("documents").
		Where(
			"document_id = ? AND content_version = ? AND source_blob_id = ?",
			task.DocumentID,
			task.ContentVersion,
			task.SourceBlobID,
		).
		Updates(map[string]any{
			"render_status": models.DocumentRenderStatusIdle,
			"render_error":  "",
			"rendered_at":   nil,
		}).Error
	s.enqueueSearchSync(task.DocumentID)
}

func (s *OfficeHTMLRenderService) markFailed(task OfficeHTMLRenderTask, message string) {
	if s == nil || s.db == nil {
		return
	}
	normalizedMessage := strings.TrimSpace(message)
	if normalizedMessage == "" {
		normalizedMessage = "office html render failed"
	}
	_ = s.db.WithContext(context.Background()).
		Table("documents").
		Where(
			"document_id = ? AND content_version = ? AND source_blob_id = ?",
			task.DocumentID,
			task.ContentVersion,
			task.SourceBlobID,
		).
		Updates(map[string]any{
			"render_status": models.DocumentRenderStatusFailed,
			"render_error":  normalizedMessage,
			"rendered_at":   nil,
		}).Error
	s.enqueueSearchSync(task.DocumentID)
}

func (s *OfficeHTMLRenderService) enqueueSearchSync(documentID string) {
	if s == nil || s.searchIndexService == nil {
		return
	}
	_ = s.searchIndexService.EnqueueSyncDocumentByID(documentID)
}

func (s *OfficeHTMLRenderService) renderOfficeHTML(
	ctx context.Context,
	task OfficeHTMLRenderTask,
) (string, error) {
	if s == nil {
		return "", errors.New("office html render service is nil")
	}
	switch task.Format {
	case models.DocumentFormatDOCX:
		return s.renderDOCXHTML(ctx, task)
	case models.DocumentFormatXLSX:
		return renderXLSXHTML(task.SourceContent)
	default:
		return "", errors.New("unsupported office render format")
	}
}

func (s *OfficeHTMLRenderService) renderDOCXHTML(
	ctx context.Context,
	task OfficeHTMLRenderTask,
) (string, error) {
	result, err := s.runMammothRender(ctx, task.SourceContent)
	if err != nil {
		return "", err
	}

	renderedHTML := strings.TrimSpace(result.HTML)
	if renderedHTML == "" {
		return "", errors.New("mammoth rendered html is empty")
	}
	if len(result.Assets) > 0 {
		renderedHTML, err = s.materializeMammothImageAssets(ctx, task, renderedHTML, result.Assets)
		if err != nil {
			return "", err
		}
	}
	return `<div class="office-docx-reader">` + renderedHTML + `</div>`, nil
}

func (s *OfficeHTMLRenderService) runMammothRender(
	ctx context.Context,
	sourceContent []byte,
) (*mammothRenderResult, error) {
	if len(sourceContent) == 0 {
		return nil, errors.New("docx source content is empty")
	}

	scriptPath, err := resolveMammothRenderScriptPath()
	if err != nil {
		return nil, err
	}
	if _, err := exec.LookPath("node"); err != nil {
		return nil, errors.New("node runtime is required for mammoth rendering")
	}

	inputPayload, err := json.Marshal(mammothRenderRequest{
		DOCXBase64: base64.StdEncoding.EncodeToString(sourceContent),
	})
	if err != nil {
		return nil, err
	}

	command := exec.CommandContext(ctx, "node", scriptPath)
	command.Stdin = bytes.NewReader(inputPayload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("mammoth render failed: %s", message)
	}

	var output mammothRenderResult
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return nil, fmt.Errorf("decode mammoth render output failed: %w", err)
	}
	return &output, nil
}

func (s *OfficeHTMLRenderService) materializeMammothImageAssets(
	ctx context.Context,
	task OfficeHTMLRenderTask,
	renderedHTML string,
	assets []mammothRenderImage,
) (string, error) {
	output := renderedHTML
	for index, asset := range assets {
		assetID := strings.TrimSpace(asset.ID)
		if assetID == "" {
			assetID = fmt.Sprintf("asset-%d", index+1)
		}
		contentType := strings.TrimSpace(asset.ContentType)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		content, err := base64.StdEncoding.DecodeString(strings.TrimSpace(asset.DataBase64))
		if err != nil {
			return "", fmt.Errorf("decode mammoth image asset %s failed: %w", assetID, err)
		}
		fileName := fmt.Sprintf("%s-%s.%s", task.DocumentID, assetID, resolveOfficeRenderFileExtension("", contentType))
		blob, err := s.ensureImageBlobForContent(
			ctx,
			content,
			contentType,
			fileName,
			task.SpaceID,
			task.DocumentID,
			time.Now().UTC(),
		)
		if err != nil {
			return "", err
		}
		placeholder := mammothImagePlaceholderPrefix + assetID
		output = strings.ReplaceAll(output, placeholder, html.EscapeString(strings.TrimSpace(blob.ObjectURL)))
	}
	return output, nil
}

func resolveMammothRenderScriptPath() (string, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("resolve mammoth render script path failed")
	}
	scriptPath := filepath.Join(filepath.Dir(currentFile), "scripts", "render_docx_with_mammoth.mjs")
	if _, err := os.Stat(scriptPath); err != nil {
		return "", err
	}
	return scriptPath, nil
}

func renderXLSXHTML(sourceContent []byte) (string, error) {
	workbook, err := excelize.OpenReader(bytes.NewReader(sourceContent))
	if err != nil {
		return "", err
	}
	defer func() {
		_ = workbook.Close()
	}()

	sheetNames := workbook.GetSheetList()
	if len(sheetNames) == 0 {
		return "", errors.New("xlsx workbook has no sheets")
	}

	sheets := make([]xlsxSheetRenderData, 0, len(sheetNames))
	for index, sheetName := range sheetNames {
		tableHTML, hasTable, err := renderXLSXSheetTableHTML(workbook, sheetName)
		if err != nil {
			return "", err
		}
		sheets = append(sheets, xlsxSheetRenderData{
			Key:       fmt.Sprintf("sheet-%d", index+1),
			Name:      strings.TrimSpace(sheetName),
			Active:    index == 0,
			HasTable:  hasTable,
			TableHTML: tableHTML,
		})
	}

	var builder strings.Builder
	builder.WriteString(`<div class="office-xlsx-reader" data-office-xlsx-reader="1">`)
	builder.WriteString(`<div class="office-xlsx-tabs" role="tablist" aria-label="工作表">`)
	for _, sheet := range sheets {
		builder.WriteString(`<button type="button" class="office-xlsx-tab`)
		if sheet.Active {
			builder.WriteString(` office-xlsx-tab--active`)
		}
		builder.WriteString(`" role="tab" data-office-sheet-tab="`)
		builder.WriteString(html.EscapeString(sheet.Key))
		builder.WriteString(`" aria-selected="`)
		if sheet.Active {
			builder.WriteString(`true`)
		} else {
			builder.WriteString(`false`)
		}
		builder.WriteString(`">`)
		builder.WriteString(html.EscapeString(defaultIfBlank(sheet.Name, "未命名工作表")))
		builder.WriteString(`</button>`)
	}
	builder.WriteString(`</div><div class="office-xlsx-panels">`)
	for _, sheet := range sheets {
		builder.WriteString(`<section class="office-xlsx-sheet`)
		if sheet.Active {
			builder.WriteString(` office-xlsx-sheet--active`)
		}
		builder.WriteString(`" data-office-sheet-panel="`)
		builder.WriteString(html.EscapeString(sheet.Key))
		builder.WriteString(`" role="tabpanel"`)
		if !sheet.Active {
			builder.WriteString(` hidden`)
		}
		builder.WriteString(`>`)
		if sheet.HasTable {
			builder.WriteString(sheet.TableHTML)
		} else {
			builder.WriteString(`<p class="office-xlsx-sheet__empty">当前工作表没有可显示的数据。</p>`)
		}
		builder.WriteString(`</section>`)
	}
	builder.WriteString(`</div></div>`)
	return builder.String(), nil
}

func renderXLSXSheetTableHTML(workbook *excelize.File, sheetName string) (string, bool, error) {
	rows, err := workbook.GetRows(sheetName)
	if err != nil {
		return "", false, err
	}
	mergeStarts, coveredCells, maxRow, maxCol, err := buildXLSXMergeLayout(workbook, sheetName)
	if err != nil {
		return "", false, err
	}
	if len(rows) > maxRow {
		maxRow = len(rows)
	}
	for _, row := range rows {
		if len(row) > maxCol {
			maxCol = len(row)
		}
	}
	if maxRow == 0 || maxCol == 0 {
		return "", false, nil
	}
	columnMetas := buildXLSXColumnRenderMetas(rows, mergeStarts, coveredCells, maxRow, maxCol)
	headerRowIndex := detectXLSXHeaderRowIndex(rows, mergeStarts, coveredCells, maxRow, maxCol)
	summaryRows := detectXLSXSummaryRows(rows, mergeStarts, coveredCells, maxRow, maxCol, headerRowIndex)

	var builder strings.Builder
	builder.WriteString(`<div class="office-xlsx-sheet__table-wrap"><table class="office-xlsx-sheet__table"><colgroup>`)
	builder.WriteString(`<col class="office-xlsx-sheet__axis-col" style="width:58px" />`)
	for _, columnMeta := range columnMetas {
		builder.WriteString(`<col class="office-xlsx-sheet__data-col" style="width:`)
		builder.WriteString(strconv.Itoa(columnMeta.WidthPX))
		builder.WriteString(`px" />`)
	}
	builder.WriteString(`</colgroup><thead><tr>`)
	builder.WriteString(`<th class="office-xlsx-sheet__axis office-xlsx-sheet__axis--corner" aria-hidden="true"></th>`)
	for _, columnMeta := range columnMetas {
		builder.WriteString(`<th scope="col" class="office-xlsx-sheet__axis office-xlsx-sheet__axis--col">`)
		builder.WriteString(html.EscapeString(columnMeta.Label))
		builder.WriteString(`</th>`)
	}
	builder.WriteString(`</tr></thead><tbody>`)
	hasVisibleCell := false
	for rowIndex := 1; rowIndex <= maxRow; rowIndex++ {
		builder.WriteString(`<tr class="office-xlsx-sheet__row`)
		if rowIndex == headerRowIndex {
			builder.WriteString(` office-xlsx-sheet__row--header`)
		}
		if _, ok := summaryRows[rowIndex]; ok {
			builder.WriteString(` office-xlsx-sheet__row--summary`)
		}
		builder.WriteString(`">`)
		builder.WriteString(`<th scope="row" class="office-xlsx-sheet__axis office-xlsx-sheet__axis--row">`)
		builder.WriteString(strconv.Itoa(rowIndex))
		builder.WriteString(`</th>`)
		for columnIndex := 1; columnIndex <= maxCol; columnIndex++ {
			cellKey := buildXLSXCellKey(rowIndex, columnIndex)
			if _, covered := coveredCells[cellKey]; covered {
				continue
			}
			value := resolveXLSXRowValue(rows, rowIndex, columnIndex)
			cellRef := buildXLSXCellReference(rowIndex, columnIndex)
			trimmedValue := strings.TrimSpace(value)
			cellSemantics := classifyXLSXCellSemantics(trimmedValue)
			if span, ok := mergeStarts[cellKey]; ok {
				if trimmedValue == "" {
					trimmedValue = span.Value
					cellSemantics = classifyXLSXCellSemantics(trimmedValue)
				}
				builder.WriteString(`<td class="office-xlsx-sheet__cell office-xlsx-sheet__cell--merged`)
				if rowIndex == headerRowIndex {
					builder.WriteString(` office-xlsx-sheet__cell--header`)
				}
				if _, ok := summaryRows[rowIndex]; ok {
					builder.WriteString(` office-xlsx-sheet__cell--summary`)
				}
				for _, semanticClass := range cellSemantics {
					builder.WriteByte(' ')
					builder.WriteString(semanticClass)
				}
				builder.WriteString(`" data-cell-ref="`)
				builder.WriteString(html.EscapeString(cellRef))
				builder.WriteString(`"`)
				if span.Rowspan > 1 {
					builder.WriteString(` rowspan="`)
					builder.WriteString(strconv.Itoa(span.Rowspan))
					builder.WriteString(`"`)
				}
				if span.Colspan > 1 {
					builder.WriteString(` colspan="`)
					builder.WriteString(strconv.Itoa(span.Colspan))
					builder.WriteString(`"`)
				}
				builder.WriteString(`>`)
				builder.WriteString(html.EscapeString(trimmedValue))
				builder.WriteString(`</td>`)
				hasVisibleCell = true
				continue
			}
			builder.WriteString(`<td class="office-xlsx-sheet__cell`)
			if trimmedValue == "" {
				builder.WriteString(` office-xlsx-sheet__cell--empty`)
			}
			if rowIndex == headerRowIndex {
				builder.WriteString(` office-xlsx-sheet__cell--header`)
			}
			if _, ok := summaryRows[rowIndex]; ok {
				builder.WriteString(` office-xlsx-sheet__cell--summary`)
			}
			for _, semanticClass := range cellSemantics {
				builder.WriteByte(' ')
				builder.WriteString(semanticClass)
			}
			builder.WriteString(`" data-cell-ref="`)
			builder.WriteString(html.EscapeString(cellRef))
			builder.WriteString(`">`)
			builder.WriteString(html.EscapeString(trimmedValue))
			builder.WriteString(`</td>`)
			if trimmedValue != "" {
				hasVisibleCell = true
			}
		}
		builder.WriteString(`</tr>`)
	}
	builder.WriteString(`</tbody></table></div>`)
	return builder.String(), hasVisibleCell, nil
}

func buildXLSXMergeLayout(
	workbook *excelize.File,
	sheetName string,
) (map[string]xlsxMergeSpan, map[string]struct{}, int, int, error) {
	mergeStarts := make(map[string]xlsxMergeSpan)
	coveredCells := make(map[string]struct{})
	mergeCells, err := workbook.GetMergeCells(sheetName, true)
	if err != nil {
		return nil, nil, 0, 0, err
	}

	maxRow := 0
	maxCol := 0
	for _, mergeCell := range mergeCells {
		startCol, startRow, err := excelize.CellNameToCoordinates(mergeCell.GetStartAxis())
		if err != nil {
			return nil, nil, 0, 0, err
		}
		endCol, endRow, err := excelize.CellNameToCoordinates(mergeCell.GetEndAxis())
		if err != nil {
			return nil, nil, 0, 0, err
		}
		if endRow > maxRow {
			maxRow = endRow
		}
		if endCol > maxCol {
			maxCol = endCol
		}
		mergeStarts[buildXLSXCellKey(startRow, startCol)] = xlsxMergeSpan{
			Rowspan: endRow - startRow + 1,
			Colspan: endCol - startCol + 1,
			Value:   strings.TrimSpace(mergeCell.GetCellValue()),
		}
		for rowIndex := startRow; rowIndex <= endRow; rowIndex++ {
			for columnIndex := startCol; columnIndex <= endCol; columnIndex++ {
				if rowIndex == startRow && columnIndex == startCol {
					continue
				}
				coveredCells[buildXLSXCellKey(rowIndex, columnIndex)] = struct{}{}
			}
		}
	}
	return mergeStarts, coveredCells, maxRow, maxCol, nil
}

func resolveXLSXRowValue(rows [][]string, rowIndex int, columnIndex int) string {
	if rowIndex <= 0 || columnIndex <= 0 || rowIndex > len(rows) {
		return ""
	}
	row := rows[rowIndex-1]
	if columnIndex > len(row) {
		return ""
	}
	return row[columnIndex-1]
}

func buildXLSXCellKey(rowIndex int, columnIndex int) string {
	return strconv.Itoa(rowIndex) + ":" + strconv.Itoa(columnIndex)
}

func buildXLSXCellReference(rowIndex int, columnIndex int) string {
	columnLabel := buildXLSXColumnLabel(columnIndex)
	if columnLabel == "" {
		columnLabel = strconv.Itoa(columnIndex)
	}
	return columnLabel + strconv.Itoa(rowIndex)
}

func buildXLSXColumnLabel(columnIndex int) string {
	if columnIndex <= 0 {
		return ""
	}
	columnLabel, err := excelize.ColumnNumberToName(columnIndex)
	if err != nil {
		return strconv.Itoa(columnIndex)
	}
	return columnLabel
}

func buildXLSXColumnRenderMetas(
	rows [][]string,
	mergeStarts map[string]xlsxMergeSpan,
	coveredCells map[string]struct{},
	maxRow int,
	maxCol int,
) []xlsxColumnRenderMeta {
	columnMetas := make([]xlsxColumnRenderMeta, 0, maxCol)
	for columnIndex := 1; columnIndex <= maxCol; columnIndex++ {
		maxDisplayWidth := 8
		columnLabel := buildXLSXColumnLabel(columnIndex)
		if columnLabel == "" {
			columnLabel = strconv.Itoa(columnIndex)
		}
		if width := estimateXLSXDisplayWidth(columnLabel); width > maxDisplayWidth {
			maxDisplayWidth = width
		}
		for rowIndex := 1; rowIndex <= maxRow; rowIndex++ {
			cellKey := buildXLSXCellKey(rowIndex, columnIndex)
			if _, covered := coveredCells[cellKey]; covered {
				continue
			}
			value := strings.TrimSpace(resolveXLSXRowValue(rows, rowIndex, columnIndex))
			if span, ok := mergeStarts[cellKey]; ok && value == "" {
				value = strings.TrimSpace(span.Value)
			}
			if value == "" {
				continue
			}
			displayWidth := estimateXLSXDisplayWidth(value)
			if span, ok := mergeStarts[cellKey]; ok && span.Colspan > 1 {
				displayWidth = max(4, displayWidth/span.Colspan)
			}
			if displayWidth > maxDisplayWidth {
				maxDisplayWidth = displayWidth
			}
		}
		columnMetas = append(columnMetas, xlsxColumnRenderMeta{
			Label:   columnLabel,
			WidthPX: clampXLSXColumnWidth(maxDisplayWidth*13 + 28),
		})
	}
	return columnMetas
}

func detectXLSXHeaderRowIndex(
	rows [][]string,
	mergeStarts map[string]xlsxMergeSpan,
	coveredCells map[string]struct{},
	maxRow int,
	maxCol int,
) int {
	maxInspectRow := min(maxRow, 6)
	for rowIndex := 1; rowIndex <= maxInspectRow; rowIndex++ {
		nonEmptyCount := 0
		textCount := 0
		numericCount := 0
		for columnIndex := 1; columnIndex <= maxCol; columnIndex++ {
			cellKey := buildXLSXCellKey(rowIndex, columnIndex)
			if _, covered := coveredCells[cellKey]; covered {
				continue
			}
			value := strings.TrimSpace(resolveXLSXRowValue(rows, rowIndex, columnIndex))
			if span, ok := mergeStarts[cellKey]; ok && value == "" {
				value = strings.TrimSpace(span.Value)
			}
			if value == "" {
				continue
			}
			nonEmptyCount++
			if isProbablyNumericXLSXCellValue(value) {
				numericCount++
			} else {
				textCount++
			}
		}
		if nonEmptyCount >= 2 && textCount >= numericCount && textCount > 0 {
			return rowIndex
		}
	}
	return 0
}

func detectXLSXSummaryRows(
	rows [][]string,
	mergeStarts map[string]xlsxMergeSpan,
	coveredCells map[string]struct{},
	maxRow int,
	maxCol int,
	headerRowIndex int,
) map[int]struct{} {
	summaryRows := make(map[int]struct{})
	for rowIndex := 1; rowIndex <= maxRow; rowIndex++ {
		if rowIndex == headerRowIndex {
			continue
		}
		firstValue := strings.TrimSpace(resolveXLSXRowValue(rows, rowIndex, 1))
		firstKey := buildXLSXCellKey(rowIndex, 1)
		if span, ok := mergeStarts[firstKey]; ok && firstValue == "" {
			firstValue = strings.TrimSpace(span.Value)
		}
		if !isLikelyXLSXSummaryLabel(firstValue) {
			continue
		}
		numericCount := 0
		for columnIndex := 2; columnIndex <= maxCol; columnIndex++ {
			cellKey := buildXLSXCellKey(rowIndex, columnIndex)
			if _, covered := coveredCells[cellKey]; covered {
				continue
			}
			value := strings.TrimSpace(resolveXLSXRowValue(rows, rowIndex, columnIndex))
			if span, ok := mergeStarts[cellKey]; ok && value == "" {
				value = strings.TrimSpace(span.Value)
			}
			if isProbablyNumericXLSXCellValue(value) {
				numericCount++
			}
		}
		if numericCount > 0 {
			summaryRows[rowIndex] = struct{}{}
		}
	}
	return summaryRows
}

func estimateXLSXDisplayWidth(value string) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}
	width := 0
	for _, char := range trimmed {
		switch {
		case char <= 0x7f:
			width++
		case char >= 0x4e00 && char <= 0x9fff:
			width += 2
		default:
			width += 2
		}
	}
	return width
}

func classifyXLSXCellSemantics(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	classes := make([]string, 0, 3)
	if isProbablyNumericXLSXCellValue(trimmed) {
		classes = append(classes, "office-xlsx-sheet__cell--numeric")
	}
	if isProbablyCurrencyXLSXCellValue(trimmed) {
		classes = append(classes, "office-xlsx-sheet__cell--currency")
	}
	if isProbablyPercentXLSXCellValue(trimmed) {
		classes = append(classes, "office-xlsx-sheet__cell--percent")
	}
	if isProbablyDateXLSXCellValue(trimmed) {
		classes = append(classes, "office-xlsx-sheet__cell--date")
	}
	return classes
}

func clampXLSXColumnWidth(width int) int {
	if width < 108 {
		return 108
	}
	if width > 320 {
		return 320
	}
	return width
}

func isProbablyNumericXLSXCellValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	normalized := normalizeXLSXNumericCandidate(trimmed)
	if normalized == "" {
		return false
	}
	if _, err := strconv.ParseFloat(normalized, 64); err == nil {
		return true
	}
	return false
}

func normalizeXLSXNumericCandidate(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	normalized := strings.NewReplacer(
		",", "",
		" ", "",
		"\u00a0", "",
		"%", "",
		"¥", "",
		"￥", "",
		"$", "",
		"€", "",
		"£", "",
	).Replace(trimmed)
	if strings.HasPrefix(normalized, "(") && strings.HasSuffix(normalized, ")") {
		normalized = "-" + strings.TrimSuffix(strings.TrimPrefix(normalized, "("), ")")
	}
	return normalized
}

func isProbablyCurrencyXLSXCellValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if !(strings.ContainsAny(trimmed, "¥￥$€£") ||
		strings.HasPrefix(strings.ToUpper(trimmed), "USD") ||
		strings.HasPrefix(strings.ToUpper(trimmed), "CNY") ||
		strings.HasPrefix(strings.ToUpper(trimmed), "RMB")) {
		return false
	}
	return isProbablyNumericXLSXCellValue(trimmed)
}

func isProbablyPercentXLSXCellValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || !strings.Contains(trimmed, "%") {
		return false
	}
	return isProbablyNumericXLSXCellValue(trimmed)
}

func isProbablyDateXLSXCellValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	dateLayouts := []string{
		"2006-01-02",
		"2006/01/02",
		"2006.01.02",
		"2006-1-2",
		"2006/1/2",
		"2006.1.2",
		"2006-01-02 15:04",
		"2006/01/02 15:04",
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"01/02/2006",
		"1/2/2006",
		"01-02-2006",
		"1-2-2006",
	}
	normalized := strings.NewReplacer("年", "-", "月", "-", "日", "", "T", " ").Replace(trimmed)
	for _, layout := range dateLayouts {
		if _, err := time.Parse(layout, normalized); err == nil {
			return true
		}
	}
	return false
}

func isLikelyXLSXSummaryLabel(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return false
	}
	for _, candidate := range []string{
		"合计",
		"总计",
		"小计",
		"汇总",
		"总额",
		"总数",
		"实收",
		"应收",
		"结余",
		"balance",
		"subtotal",
		"total",
		"sum",
	} {
		if strings.Contains(normalized, candidate) {
			return true
		}
	}
	return false
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func defaultIfBlank(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed != "" {
		return trimmed
	}
	return fallback
}

func (s *OfficeHTMLRenderService) ensureImageBlobForContent(
	ctx context.Context,
	content []byte,
	contentType string,
	fileName string,
	spaceID string,
	documentID string,
	now time.Time,
) (*models.DocumentAttachmentBlob, error) {
	if s == nil || s.documentAttachmentRepo == nil {
		return nil, errors.New("document attachment repository is nil")
	}
	if len(content) == 0 {
		return nil, errors.New("image content is empty")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	config := DefaultImageHostingConfig()
	if s.imageHostingService != nil {
		loadedConfig, err := s.imageHostingService.GetConfig(ctx)
		if err != nil {
			return nil, err
		}
		config = loadedConfig
	}
	targetProvider := config.DefaultProvider
	if targetProvider == "" {
		targetProvider = ImageHostingProviderLocal
	}

	hashValue := sha256.Sum256(content)
	contentHash := hex.EncodeToString(hashValue[:])
	contentSize := int64(len(content))
	blob, err := s.documentAttachmentRepo.FindBlobByHash(
		ctx,
		string(targetProvider),
		"sha256",
		contentHash,
		contentSize,
	)
	if err == nil {
		return blob, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	objectKey, err := buildOfficeRenderObjectKey(
		fileName,
		contentType,
		spaceID,
		documentID,
		now,
		config.UploadPathTemplate(targetProvider),
	)
	if err != nil {
		return nil, err
	}
	objectURL, savedTargetPath, err := s.uploadRawContentToProvider(
		ctx,
		content,
		contentType,
		objectKey,
		targetProvider,
		config,
	)
	if err != nil {
		return nil, err
	}

	blobCandidate := &models.DocumentAttachmentBlob{
		BlobID:          strings.ToLower(ulid.Make().String()),
		StorageProvider: string(targetProvider),
		ObjectKey:       objectKey,
		ObjectURL:       strings.TrimSpace(objectURL),
		MimeType:        strings.TrimSpace(contentType),
		SizeBytes:       contentSize,
		ContentHashAlgo: "sha256",
		ContentHash:     contentHash,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if blobCandidate.MimeType == "" {
		blobCandidate.MimeType = "application/octet-stream"
	}

	if err := s.documentAttachmentRepo.CreateBlob(ctx, blobCandidate); err != nil {
		if savedTargetPath != "" {
			if cleanupErr := os.Remove(savedTargetPath); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
				return nil, cleanupErr
			}
		}
		if !isLikelyUniqueConstraintError(err) {
			return nil, err
		}
		existingBlob, lookupErr := s.documentAttachmentRepo.FindBlobByHash(
			ctx,
			string(targetProvider),
			"sha256",
			contentHash,
			contentSize,
		)
		if lookupErr != nil {
			return nil, lookupErr
		}
		return existingBlob, nil
	}

	return blobCandidate, nil
}

func (s *OfficeHTMLRenderService) uploadRawContentToProvider(
	ctx context.Context,
	fileContent []byte,
	contentType string,
	objectKey string,
	provider ImageHostingProvider,
	config ImageHostingConfig,
) (string, string, error) {
	if len(fileContent) == 0 {
		return "", "", errors.New("attachment content is empty")
	}

	switch provider {
	case ImageHostingProviderLocal:
		targetPath, pathErr := s.resolveLocalAttachmentTargetPath(objectKey)
		if pathErr != nil {
			return "", "", pathErr
		}
		targetDir := filepath.Dir(targetPath)
		if mkdirErr := os.MkdirAll(targetDir, 0o755); mkdirErr != nil {
			return "", "", mkdirErr
		}
		if saveErr := os.WriteFile(targetPath, fileContent, 0o644); saveErr != nil {
			return "", "", saveErr
		}
		return resolveOfficeRenderPublicURL(config.Local.PublicBaseURL, objectKey, "/uploads"), targetPath, nil
	case ImageHostingProviderCloudflareR2:
		uploadedURL, uploadErr := uploadOfficeRenderImageToCloudflareR2(ctx, fileContent, contentType, objectKey, config)
		return uploadedURL, "", uploadErr
	case ImageHostingProviderAliyunOSS:
		uploadedURL, uploadErr := uploadOfficeRenderImageToAliyunOSS(fileContent, contentType, objectKey, config)
		return uploadedURL, "", uploadErr
	default:
		return "", "", errors.New("unsupported attachment storage provider")
	}
}

func (s *OfficeHTMLRenderService) resolveLocalAttachmentTargetPath(objectKey string) (string, error) {
	normalizedObjectKey := strings.TrimSpace(strings.TrimPrefix(objectKey, "/"))
	if normalizedObjectKey == "" {
		return "", errors.New("attachment object key is empty")
	}
	cleanObjectKey := path.Clean(normalizedObjectKey)
	if cleanObjectKey == "." || cleanObjectKey == "/" || strings.HasPrefix(cleanObjectKey, "../") {
		return "", errors.New("attachment object key is invalid")
	}
	localRootDir := strings.TrimSpace(s.localRootDir)
	if localRootDir == "" {
		localRootDir = "uploads"
	}
	targetPath := filepath.Join(localRootDir, filepath.FromSlash(cleanObjectKey))
	targetAbsPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", err
	}
	rootAbsPath, err := filepath.Abs(localRootDir)
	if err != nil {
		return "", err
	}
	if !isOfficeRenderPathWithinRoot(rootAbsPath, targetAbsPath) {
		return "", errors.New("attachment object path is out of root dir")
	}
	return targetPath, nil
}

func buildOfficeRenderObjectKey(
	fileName string,
	contentType string,
	spaceID string,
	documentID string,
	now time.Time,
	uploadPathTemplate string,
) (string, error) {
	extension := sanitizeOfficeRenderPathSegment(resolveOfficeRenderFileExtension(fileName, contentType), "bin")
	assetID := strings.ToLower(ulid.Make().String())
	replaced, err := RenderImageHostingUploadPathTemplate(uploadPathTemplate, map[string]string{
		"spaceId":    sanitizeOfficeRenderPathSegment(spaceID, "space"),
		"docId":      sanitizeOfficeRenderPathSegment(documentID, "doc"),
		"yyyy":       fmt.Sprintf("%04d", now.Year()),
		"mm":         fmt.Sprintf("%02d", int(now.Month())),
		"dd":         fmt.Sprintf("%02d", now.Day()),
		"hh":         fmt.Sprintf("%02d", now.Hour()),
		"assetId":    sanitizeOfficeRenderPathSegment(assetID, "asset"),
		"origName":   sanitizeOfficeRenderPathSegment(resolveOfficeRenderOriginName(fileName), "file"),
		"ext":        extension,
		"uploaderId": "office-render",
	})
	if err != nil {
		return "", err
	}

	normalizedObjectKey := strings.TrimSpace(strings.TrimPrefix(replaced, "/"))
	if normalizedObjectKey == "" {
		return "", errors.New("attachment object key is empty")
	}
	cleanObjectKey := path.Clean(normalizedObjectKey)
	if cleanObjectKey == "." || cleanObjectKey == "/" || strings.HasPrefix(cleanObjectKey, "../") {
		return "", errors.New("attachment object key is invalid")
	}
	if !strings.HasPrefix(cleanObjectKey, "images/") {
		return "", errors.New("attachment object key must start with images/")
	}
	if len(cleanObjectKey) > 512 {
		return "", errors.New("attachment object key is too long")
	}
	return cleanObjectKey, nil
}

func resolveOfficeRenderFileExtension(fileName string, contentType string) string {
	extension := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(path.Ext(fileName)), "."))
	if isOfficeRenderSafeFileExtension(extension) {
		return extension
	}
	extensions, err := mime.ExtensionsByType(strings.TrimSpace(contentType))
	if err == nil {
		for _, item := range extensions {
			candidate := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(item), "."))
			if isOfficeRenderSafeFileExtension(candidate) {
				return candidate
			}
		}
	}
	return "bin"
}

func isOfficeRenderSafeFileExtension(extension string) bool {
	if extension == "" {
		return false
	}
	for _, char := range extension {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			continue
		}
		return false
	}
	return true
}

func resolveOfficeRenderOriginName(fileName string) string {
	trimmed := strings.TrimSpace(fileName)
	if trimmed == "" {
		return "file"
	}
	baseName := strings.TrimSpace(path.Base(trimmed))
	if baseName == "" || baseName == "." || baseName == "/" {
		return "file"
	}
	extension := path.Ext(baseName)
	if extension != "" {
		baseName = strings.TrimSuffix(baseName, extension)
	}
	return baseName
}

func sanitizeOfficeRenderPathSegment(rawValue string, fallback string) string {
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return fallback
	}

	var builder strings.Builder
	builder.Grow(len(trimmed))
	for _, char := range trimmed {
		switch {
		case (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_' || char == '.':
			builder.WriteRune(char)
		default:
			builder.WriteByte('-')
		}
	}

	normalized := strings.Trim(builder.String(), "-._")
	if normalized == "" {
		return fallback
	}
	return normalized
}

func uploadOfficeRenderImageToCloudflareR2(
	ctx context.Context,
	content []byte,
	contentType string,
	objectKey string,
	config ImageHostingConfig,
) (string, error) {
	if ctx == nil {
		return "", errors.New("request context is nil")
	}
	accountID := strings.TrimSpace(config.CloudflareR2.AccountID)
	bucket := strings.TrimSpace(config.CloudflareR2.Bucket)
	accessKeyID := strings.TrimSpace(config.CloudflareR2.AccessKeyID)
	secretAccessKey := strings.TrimSpace(config.CloudflareR2.SecretAccessKey)
	if accountID == "" || bucket == "" || accessKeyID == "" || secretAccessKey == "" {
		return "", errors.New("cloudflare r2 config is incomplete")
	}
	endpoint := resolveOfficeRenderCloudflareR2Endpoint(accountID)
	if endpoint == "" {
		return "", errors.New("cloudflare r2 endpoint is empty")
	}

	awsConfig := aws.Config{
		Region: "auto",
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
			accessKeyID,
			secretAccessKey,
			"",
		)),
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(
			func(service string, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               endpoint,
					SigningRegion:     "auto",
					HostnameImmutable: true,
				}, nil
			},
		),
	}
	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.UsePathStyle = true
	})
	_, putErr := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(content),
		ContentType: aws.String(contentType),
	})
	if putErr != nil {
		return "", putErr
	}

	resolvedURL := resolveOfficeRenderObjectStoragePublicURL(config.CloudflareR2.PublicBaseURL, objectKey)
	if resolvedURL == "" {
		resolvedURL = strings.TrimRight(endpoint, "/") + "/" + strings.TrimLeft(bucket+"/"+objectKey, "/")
	}
	return resolvedURL, nil
}

func uploadOfficeRenderImageToAliyunOSS(
	content []byte,
	contentType string,
	objectKey string,
	config ImageHostingConfig,
) (string, error) {
	bucket := strings.TrimSpace(config.AliyunOSS.Bucket)
	accessKeyID := strings.TrimSpace(config.AliyunOSS.AccessKeyID)
	accessKeySecret := strings.TrimSpace(config.AliyunOSS.AccessKeySecret)
	if bucket == "" || accessKeyID == "" || accessKeySecret == "" {
		return "", errors.New("aliyun oss config is incomplete")
	}

	endpoint := resolveOfficeRenderAliyunOSSEndpoint(config.AliyunOSS.Endpoint, config.AliyunOSS.Region)
	if endpoint == "" {
		return "", errors.New("aliyun oss endpoint is empty")
	}

	client, err := oss.New(endpoint, accessKeyID, accessKeySecret)
	if err != nil {
		return "", err
	}
	bucketClient, err := client.Bucket(bucket)
	if err != nil {
		return "", err
	}

	if putErr := bucketClient.PutObject(objectKey, bytes.NewReader(content), oss.ContentType(contentType)); putErr != nil {
		return "", putErr
	}

	resolvedURL := resolveOfficeRenderObjectStoragePublicURL(config.AliyunOSS.PublicBaseURL, objectKey)
	if resolvedURL == "" {
		baseURL := buildOfficeRenderAliyunOSSFallbackPublicBaseURL(endpoint, bucket)
		resolvedURL = resolveOfficeRenderObjectStoragePublicURL(baseURL, objectKey)
	}
	return resolvedURL, nil
}

func resolveOfficeRenderCloudflareR2Endpoint(accountID string) string {
	normalizedAccountID := strings.TrimSpace(accountID)
	if normalizedAccountID == "" {
		return ""
	}
	if strings.HasPrefix(normalizedAccountID, "https://") || strings.HasPrefix(normalizedAccountID, "http://") {
		return strings.TrimRight(normalizedAccountID, "/")
	}
	return "https://" + normalizedAccountID + ".r2.cloudflarestorage.com"
}

func resolveOfficeRenderAliyunOSSEndpoint(endpoint string, region string) string {
	normalizedEndpoint := strings.TrimSpace(endpoint)
	if normalizedEndpoint != "" {
		if strings.HasPrefix(normalizedEndpoint, "https://") || strings.HasPrefix(normalizedEndpoint, "http://") {
			return strings.TrimRight(normalizedEndpoint, "/")
		}
		return "https://" + strings.TrimRight(normalizedEndpoint, "/")
	}
	normalizedRegion := strings.TrimSpace(region)
	if normalizedRegion == "" {
		return ""
	}
	return "https://oss-" + normalizedRegion + ".aliyuncs.com"
}

func resolveOfficeRenderObjectStoragePublicURL(baseURL string, objectKey string) string {
	normalizedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	normalizedObjectKey := strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	if normalizedBaseURL == "" || normalizedObjectKey == "" {
		return ""
	}
	return normalizedBaseURL + "/" + normalizedObjectKey
}

func buildOfficeRenderAliyunOSSFallbackPublicBaseURL(endpoint string, bucket string) string {
	normalizedEndpoint := strings.TrimSpace(endpoint)
	normalizedBucket := strings.TrimSpace(bucket)
	if normalizedEndpoint == "" || normalizedBucket == "" {
		return ""
	}
	parsedURL, err := url.Parse(normalizedEndpoint)
	if err != nil || parsedURL.Host == "" {
		return ""
	}
	host := parsedURL.Hostname()
	if host == "" {
		return ""
	}
	port := parsedURL.Port()
	scheme := parsedURL.Scheme
	if scheme == "" {
		scheme = "https"
	}
	bucketHost := host
	if !strings.HasPrefix(host, normalizedBucket+".") {
		bucketHost = normalizedBucket + "." + host
	}
	if port != "" {
		return scheme + "://" + bucketHost + ":" + port
	}
	return scheme + "://" + bucketHost
}

func resolveOfficeRenderPublicURL(baseURL string, objectPath string, fallbackBaseURL string) string {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = strings.TrimSpace(fallbackBaseURL)
	}
	if base == "" {
		base = "/uploads"
	}
	if strings.EqualFold(strings.TrimRight(base, "/"), "/api/uploads/local") {
		base = "/uploads"
	}
	if strings.EqualFold(strings.TrimRight(base, "/"), "/uploads/local") {
		base = "/uploads"
	}
	if strings.HasPrefix(base, "/api/uploads/") {
		base = strings.TrimPrefix(base, "/api")
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(objectPath, "/")
}

func isOfficeRenderPathWithinRoot(rootAbsPath string, targetAbsPath string) bool {
	if rootAbsPath == targetAbsPath {
		return true
	}
	prefix := rootAbsPath + string(filepath.Separator)
	return strings.HasPrefix(targetAbsPath, prefix)
}

func isLikelyUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "duplicated")
}
