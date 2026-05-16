package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	epub "github.com/go-shiori/go-epub"
	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"github.com/lifei6671/plaindoc/apps/server/internal/storage/models"
	"gorm.io/gorm"
)

const adminSpaceEPUBBaseCSS = `
body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  line-height: 1.72;
  color: #1f2933;
}
h1, h2, h3 {
  line-height: 1.28;
}
table {
  border-collapse: collapse;
  width: 100%;
}
th, td {
  border: 1px solid #d0d7de;
  padding: 0.35rem 0.5rem;
}
img {
  max-width: 100%;
  height: auto;
}
`

const maxAdminSpaceEPUBImageBytes int64 = 20 << 20

var adminSpaceEPUBImageSrcPattern = regexp.MustCompile(`(?is)<img\b[^>]*\bsrc\s*=\s*["']([^"']*)["'][^>]*>`)
var adminSpaceReaderArticlePattern = regexp.MustCompile(`(?is)<article\b[^>]*\bid\s*=\s*["']plaindoc-preview-body["'][^>]*>.*?</article>`)
var adminSpaceReaderCodeCopyButtonPattern = regexp.MustCompile(`(?is)<button\b[^>]*\bdata-code-copy-button\s*=\s*["']1["'][^>]*>.*?</button>`)

func (s *AdminSpaceExportService) exportAdminSpaceEPUBPackage(
	ctx context.Context,
	job AdminSpaceExportJob,
) (string, string, int64, error) {
	if s == nil || s.spaceReader == nil || s.workspaceReader == nil {
		return "", "", 0, fmt.Errorf("导出服务依赖未配置")
	}

	s.PublishProgress(job.JobID, AdminSpaceTransferEvent{
		Type:     AdminSpaceTransferEventTypeProgress,
		Stage:    "metadata",
		Progress: 5,
		Message:  "正在读取空间元数据",
	})

	space, err := s.spaceReader.GetBySpaceID(ctx, job.SpaceID)
	if err != nil {
		if errorsIsRecordNotFound(err) {
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
		Message:  "正在构建 EPUB 目录",
	})

	exportedAt := s.now()
	packageJob := job
	packageJob.IncludeOfficeSources = true
	packageJob.IncludeAttachments = false
	pkg, err := s.buildAdminSpaceExportPackage(ctx, packageJob, *space, rows, exportedAt)
	if err != nil {
		return "", "", 0, err
	}

	s.PublishProgress(job.JobID, AdminSpaceTransferEvent{
		Type:     AdminSpaceTransferEventTypeProgress,
		Stage:    "render",
		Progress: 55,
		Message:  "正在渲染 EPUB 章节",
	})

	exportDir := filepath.Clean(s.exportDir)
	if err := os.MkdirAll(exportDir, 0o700); err != nil {
		return "", "", 0, err
	}
	mediaTempDir, err := os.MkdirTemp(exportDir, job.JobID+"-epub-media-*")
	if err != nil {
		return "", "", 0, err
	}
	defer os.RemoveAll(mediaTempDir)

	book, err := s.buildAdminSpaceEPUB(ctx, *space, pkg, exportedAt, mediaTempDir)
	if err != nil {
		return "", "", 0, err
	}

	fileName := buildAdminSpaceExportEPUBFileName(job.SpaceID, exportedAt)
	partPath := filepath.Join(exportDir, job.JobID+".part")
	finalPath := filepath.Join(exportDir, job.JobID+".epub")
	if err := writeAdminSpaceEPUBFile(partPath, book); err != nil {
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

func (s *AdminSpaceExportService) buildAdminSpaceEPUB(
	ctx context.Context,
	space models.Space,
	pkg adminSpaceExportPackage,
	exportedAt time.Time,
	mediaTempDir string,
) (*epub.Epub, error) {
	title := strings.TrimSpace(space.Name)
	if title == "" {
		title = strings.TrimSpace(space.SpaceID)
	}
	if title == "" {
		title = "PlainDoc Space"
	}
	book, err := epub.NewEpub(title)
	if err != nil {
		return nil, err
	}
	book.SetAuthor("PlainDoc")
	book.SetIdentifier("plaindoc:space:" + strings.TrimSpace(space.SpaceID) + ":" + exportedAt.Format(time.RFC3339))
	book.SetLang("zh-CN")
	if description := strings.TrimSpace(space.Description); description != "" {
		book.SetDescription(description)
	}

	cssPath, err := book.AddCSS(adminSpaceEPUBCSSDataURL(), "plaindoc.css")
	if err != nil {
		return nil, err
	}
	if _, err := book.AddSection(buildAdminSpaceEPUBTitlePage(space, exportedAt), title, "index.xhtml", cssPath); err != nil {
		return nil, err
	}

	usedNames := map[string]int{"index.xhtml": 1}
	imageSources := make(map[string]string)
	documentsByNodeID := make(map[string]AdminSpaceExportDocumentEntry, len(pkg.Manifest.Documents))
	for _, document := range pkg.Manifest.Documents {
		documentsByNodeID[strings.TrimSpace(document.NodeID)] = document
	}
	renderedDocumentNodes := make(map[string]struct{}, len(pkg.Manifest.Documents))
	var addTreeNodes func(parentSectionPath string, nodes []AdminSpaceExportTreeNode) error
	addTreeNodes = func(parentSectionPath string, nodes []AdminSpaceExportTreeNode) error {
		for _, node := range nodes {
			nodeID := strings.TrimSpace(node.NodeID)
			sectionTitle := adminSpaceEPUBTreeNodeTitle(node)
			internalFilename := uniqueAdminSpaceExportName(usedNames, sectionTitle, "node-"+nodeID, ".xhtml")
			var body string
			switch models.NodeType(strings.TrimSpace(node.Type)) {
			case models.NodeTypeFolder:
				body = buildAdminSpaceEPUBFolderPage(sectionTitle)
			case models.NodeTypeDoc:
				document, ok := documentsByNodeID[nodeID]
				if !ok {
					return fmt.Errorf("EPUB 目录节点缺少文档条目: %s", nodeID)
				}
				renderedBody, err := s.renderAdminSpaceEPUBDocument(ctx, book, imageSources, pkg, document, mediaTempDir)
				if err != nil {
					return err
				}
				body = renderedBody
				renderedDocumentNodes[nodeID] = struct{}{}
			default:
				if len(node.Children) == 0 {
					continue
				}
				if err := addTreeNodes(parentSectionPath, node.Children); err != nil {
					return err
				}
				continue
			}
			sectionPath, err := addAdminSpaceEPUBSection(book, parentSectionPath, body, sectionTitle, internalFilename, cssPath)
			if err != nil {
				return err
			}
			if len(node.Children) > 0 {
				if err := addTreeNodes(sectionPath, node.Children); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := addTreeNodes("", pkg.Tree.Root); err != nil {
		return nil, err
	}
	for _, document := range pkg.Manifest.Documents {
		if _, ok := renderedDocumentNodes[strings.TrimSpace(document.NodeID)]; ok {
			continue
		}
		body, err := s.renderAdminSpaceEPUBDocument(ctx, book, imageSources, pkg, document, mediaTempDir)
		if err != nil {
			return nil, err
		}
		sectionTitle := strings.TrimSpace(document.Title)
		if sectionTitle == "" {
			sectionTitle = strings.TrimSpace(document.DocumentID)
		}
		internalFilename := uniqueAdminSpaceExportName(usedNames, sectionTitle, "document-"+document.NodeID, ".xhtml")
		if _, err := book.AddSection(body, sectionTitle, internalFilename, cssPath); err != nil {
			return nil, err
		}
	}
	if len(pkg.Manifest.Documents) == 0 {
		if _, err := book.AddSection("<p></p>", title, "empty.xhtml", cssPath); err != nil {
			return nil, err
		}
	}
	return book, nil
}

func addAdminSpaceEPUBSection(
	book *epub.Epub,
	parentSectionPath string,
	body string,
	sectionTitle string,
	internalFilename string,
	cssPath string,
) (string, error) {
	if strings.TrimSpace(parentSectionPath) != "" {
		return book.AddSubSection(parentSectionPath, body, sectionTitle, internalFilename, cssPath)
	}
	return book.AddSection(body, sectionTitle, internalFilename, cssPath)
}

func adminSpaceEPUBTreeNodeTitle(node AdminSpaceExportTreeNode) string {
	title := strings.TrimSpace(node.Title)
	if title != "" {
		return title
	}
	if documentID := strings.TrimSpace(node.DocumentID); documentID != "" {
		return documentID
	}
	if nodeID := strings.TrimSpace(node.NodeID); nodeID != "" {
		return nodeID
	}
	return "未命名"
}

func buildAdminSpaceEPUBFolderPage(title string) string {
	escapedTitle := html.EscapeString(strings.TrimSpace(title))
	if escapedTitle == "" {
		escapedTitle = "未命名"
	}
	return "<h1>" + escapedTitle + "</h1>"
}

func (s *AdminSpaceExportService) renderAdminSpaceEPUBDocument(
	ctx context.Context,
	book *epub.Epub,
	imageSources map[string]string,
	pkg adminSpaceExportPackage,
	document AdminSpaceExportDocumentEntry,
	mediaTempDir string,
) (string, error) {
	switch models.NormalizeDocumentFormat(models.DocumentFormat(document.Format)) {
	case models.DocumentFormatMarkdown:
		content, ok := pkg.Files[document.Path]
		if !ok {
			return "", fmt.Errorf("EPUB 章节缺少 Markdown 内容: %s", document.DocumentID)
		}
		if s == nil || s.readerHTMLRenderer == nil {
			return "", fmt.Errorf("EPUB Markdown 渲染依赖未配置")
		}
		renderedHTML, err := s.readerHTMLRenderer.RenderMarkdownHTML(ctx, AdminSpaceExportReaderHTMLRenderInput{
			Space:      models.Space{SpaceID: pkg.Manifest.Space.SpaceID, Name: pkg.Manifest.Space.Name, Description: pkg.Manifest.Space.Description, Visibility: models.Visibility(pkg.Manifest.Space.Visibility)},
			Document:   document,
			Content:    string(content),
			Tree:       pkg.Tree,
			ExportedAt: parseAdminSpaceExportedAt(pkg.Manifest.ExportedAt),
		})
		if err != nil {
			return "", err
		}
		body, err := extractAdminSpaceReaderArticleFragment(renderedHTML)
		if err != nil {
			return "", err
		}
		return localizeAdminSpaceEPUBImages(book, imageSources, ensureAdminSpaceEPUBBody(body), mediaTempDir)
	case models.DocumentFormatDOCX, models.DocumentFormatXLSX:
		if s == nil || s.officeHTMLRenderer == nil {
			return "", fmt.Errorf("EPUB Office 渲染依赖未配置")
		}
		if document.Source == nil || !document.Source.Included {
			return "", fmt.Errorf("EPUB Office 章节缺少 source: %s", document.DocumentID)
		}
		sourceContent, ok := pkg.Files[document.Source.Path]
		if !ok {
			return "", fmt.Errorf("EPUB Office 章节 source 内容缺失: %s", document.DocumentID)
		}
		renderedHTML, err := s.officeHTMLRenderer.RenderExportHTML(
			ctx,
			models.NormalizeDocumentFormat(models.DocumentFormat(document.Format)),
			sourceContent,
			pkg.Manifest.Space.SpaceID,
			document.DocumentID,
		)
		if err != nil {
			return "", err
		}
		return localizeAdminSpaceEPUBImages(book, imageSources, ensureAdminSpaceEPUBBody(extractAdminSpaceEPUBBodyFragment(renderedHTML)), mediaTempDir)
	default:
		return "", errcode.ErrAdminSpaceExportFormatUnsupported
	}
}

func extractAdminSpaceEPUBBodyFragment(renderedHTML string) string {
	trimmed := strings.TrimSpace(renderedHTML)
	lower := strings.ToLower(trimmed)
	bodyStart := strings.Index(lower, "<body")
	if bodyStart < 0 {
		return trimmed
	}
	bodyOpenEnd := strings.Index(lower[bodyStart:], ">")
	if bodyOpenEnd < 0 {
		return trimmed
	}
	contentStart := bodyStart + bodyOpenEnd + 1
	bodyEnd := strings.LastIndex(lower, "</body>")
	if bodyEnd < contentStart {
		return trimmed[contentStart:]
	}
	return trimmed[contentStart:bodyEnd]
}

func extractAdminSpaceReaderArticleFragment(renderedHTML string) (string, error) {
	trimmed := strings.TrimSpace(renderedHTML)
	if trimmed == "" {
		return "", errors.New("reader ssr html is empty")
	}
	match := adminSpaceReaderArticlePattern.FindString(trimmed)
	if strings.TrimSpace(match) == "" {
		return "", errors.New("reader ssr html missing plaindoc preview article")
	}
	return sanitizeAdminSpaceReaderArticleForEPUB(match), nil
}

func sanitizeAdminSpaceReaderArticleForEPUB(articleHTML string) string {
	return adminSpaceReaderCodeCopyButtonPattern.ReplaceAllString(articleHTML, "")
}

func ensureAdminSpaceEPUBBody(body string) string {
	if strings.TrimSpace(body) == "" {
		return "<p></p>"
	}
	return body
}

func parseAdminSpaceExportedAt(value string) time.Time {
	if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value)); err == nil {
		return parsed
	}
	return time.Time{}
}

func adminSpaceEPUBCSSDataURL() string {
	return "data:text/css;base64," + base64.StdEncoding.EncodeToString([]byte(adminSpaceEPUBBaseCSS))
}

func localizeAdminSpaceEPUBImages(book *epub.Epub, imageSources map[string]string, body string, mediaTempDir string) (string, error) {
	if book == nil || strings.TrimSpace(body) == "" {
		return body, nil
	}
	matches := adminSpaceEPUBImageSrcPattern.FindAllStringSubmatchIndex(body, -1)
	if len(matches) == 0 {
		return body, nil
	}
	var builder strings.Builder
	lastEnd := 0
	for _, match := range matches {
		tagStart, tagEnd := match[0], match[1]
		srcStart, srcEnd := match[2], match[3]
		builder.WriteString(body[lastEnd:tagStart])
		originalTag := body[tagStart:tagEnd]
		source := strings.TrimSpace(body[srcStart:srcEnd])
		localizedSource, err := addAdminSpaceEPUBImage(book, imageSources, source, mediaTempDir)
		if err != nil {
			builder.WriteString(degradeAdminSpaceEPUBImageTag(originalTag))
			lastEnd = tagEnd
			continue
		}
		if localizedSource == "" {
			builder.WriteString(degradeAdminSpaceEPUBImageTag(originalTag))
		} else {
			relativeSrcStart := srcStart - tagStart
			relativeSrcEnd := srcEnd - tagStart
			builder.WriteString(originalTag[:relativeSrcStart])
			builder.WriteString(localizedSource)
			builder.WriteString(originalTag[relativeSrcEnd:])
		}
		lastEnd = tagEnd
	}
	builder.WriteString(body[lastEnd:])
	return builder.String(), nil
}

func buildAdminSpaceEPUBTitlePage(space models.Space, exportedAt time.Time) string {
	title := strings.TrimSpace(space.Name)
	if title == "" {
		title = strings.TrimSpace(space.SpaceID)
	}
	if title == "" {
		title = "PlainDoc Space"
	}
	var builder strings.Builder
	builder.WriteString("<section>")
	builder.WriteString("<h1>")
	builder.WriteString(escapeAdminSpaceEPUBText(title))
	builder.WriteString("</h1>")
	if description := strings.TrimSpace(space.Description); description != "" {
		builder.WriteString("<p>")
		builder.WriteString(escapeAdminSpaceEPUBText(description))
		builder.WriteString("</p>")
	}
	builder.WriteString("<p>导出时间：")
	builder.WriteString(escapeAdminSpaceEPUBText(exportedAt.Format(time.RFC3339)))
	builder.WriteString("</p>")
	builder.WriteString("</section>")
	return builder.String()
}

func degradeAdminSpaceEPUBImageTag(tag string) string {
	alt := extractAdminSpaceEPUBImageAlt(tag)
	if alt == "" {
		alt = "图片无法导出"
	}
	return `<p class="plaindoc-epub-missing-image">` + escapeAdminSpaceEPUBText(alt) + `</p>`
}

func extractAdminSpaceEPUBImageAlt(tag string) string {
	pattern := regexp.MustCompile(`(?is)\balt\s*=\s*["']([^"']*)["']`)
	match := pattern.FindStringSubmatch(strings.TrimSpace(tag))
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func escapeAdminSpaceEPUBText(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
}

func addAdminSpaceEPUBImage(book *epub.Epub, imageSources map[string]string, source string, mediaTempDir string) (string, error) {
	source = strings.TrimSpace(source)
	if book == nil || source == "" || strings.HasPrefix(strings.ToLower(source), "cid:") {
		return "", nil
	}
	if localized := imageSources[source]; localized != "" {
		return localized, nil
	}
	mediaSource, err := prepareAdminSpaceEPUBImageFile(source, mediaTempDir)
	if err != nil {
		return "", err
	}
	if mediaSource == "" {
		return "", nil
	}
	internalFilename := buildAdminSpaceEPUBImageFileName(source)
	localized, err := book.AddImage(mediaSource, internalFilename)
	if err != nil {
		return "", err
	}
	if imageSources != nil {
		imageSources[source] = localized
	}
	return localized, nil
}

func prepareAdminSpaceEPUBImageFile(source string, mediaTempDir string) (string, error) {
	source = strings.TrimSpace(source)
	lower := strings.ToLower(source)
	switch {
	case strings.HasPrefix(lower, "data:image/"):
		return writeAdminSpaceEPUBDataImageFile(source, mediaTempDir)
	case strings.HasPrefix(source, "/uploads/"):
		objectKey := adminSpaceEPUBUploadObjectKey(source)
		if objectKey == "" {
			return "", nil
		}
		localPath, err := resolveAdminSpaceExportLocalBlobPath("uploads", objectKey)
		if err != nil {
			return "", err
		}
		return copyAdminSpaceEPUBImageFile(source, localPath, mediaTempDir)
	default:
		return "", nil
	}
}

func writeAdminSpaceEPUBDataImageFile(source string, mediaTempDir string) (string, error) {
	commaIndex := strings.Index(source, ",")
	if commaIndex < 0 {
		return "", fmt.Errorf("EPUB data image missing payload")
	}
	header := strings.ToLower(strings.TrimSpace(source[:commaIndex]))
	if !strings.HasPrefix(header, "data:image/") {
		return "", nil
	}
	payload := source[commaIndex+1:]
	var data []byte
	var err error
	if strings.Contains(header, ";base64") {
		trimmedPayload := strings.TrimSpace(payload)
		if err := validateAdminSpaceEPUBImageSize(int64(base64.StdEncoding.DecodedLen(len(trimmedPayload)))); err != nil {
			return "", err
		}
		data, err = base64.StdEncoding.DecodeString(trimmedPayload)
	} else {
		if err := validateAdminSpaceEPUBImageSize(int64(len(payload))); err != nil {
			return "", err
		}
		decoded, decodeErr := url.QueryUnescape(payload)
		if decodeErr != nil {
			return "", decodeErr
		}
		data = []byte(decoded)
	}
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", fmt.Errorf("EPUB data image payload is empty")
	}
	if err := validateAdminSpaceEPUBImageSize(int64(len(data))); err != nil {
		return "", err
	}
	return writeAdminSpaceEPUBTempImageFile(source, mediaTempDir, data)
}

func copyAdminSpaceEPUBImageFile(source string, localPath string, mediaTempDir string) (string, error) {
	input, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer input.Close()
	if stat, err := input.Stat(); err != nil {
		return "", err
	} else if err := validateAdminSpaceEPUBImageSize(stat.Size()); err != nil {
		return "", err
	}
	if err := os.MkdirAll(mediaTempDir, 0o700); err != nil {
		return "", err
	}
	targetPath := filepath.Join(mediaTempDir, buildAdminSpaceEPUBImageFileName(source))
	output, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	closeOutput := true
	defer func() {
		if closeOutput {
			_ = output.Close()
		}
	}()
	written, err := io.Copy(output, io.LimitReader(input, maxAdminSpaceEPUBImageBytes+1))
	if err != nil {
		return "", err
	}
	if err := validateAdminSpaceEPUBImageSize(written); err != nil {
		_ = output.Close()
		closeOutput = false
		_ = os.Remove(targetPath)
		return "", err
	}
	if err := output.Close(); err != nil {
		closeOutput = false
		return "", err
	}
	closeOutput = false
	return targetPath, nil
}

func validateAdminSpaceEPUBImageSize(sizeBytes int64) error {
	if sizeBytes > maxAdminSpaceEPUBImageBytes {
		return fmt.Errorf("EPUB image exceeds max size %d bytes", maxAdminSpaceEPUBImageBytes)
	}
	return nil
}

func writeAdminSpaceEPUBTempImageFile(source string, mediaTempDir string, data []byte) (string, error) {
	if err := os.MkdirAll(mediaTempDir, 0o700); err != nil {
		return "", err
	}
	targetPath := filepath.Join(mediaTempDir, buildAdminSpaceEPUBImageFileName(source))
	if err := os.WriteFile(targetPath, data, 0o600); err != nil {
		return "", err
	}
	return targetPath, nil
}

func adminSpaceEPUBUploadObjectKey(source string) string {
	value := strings.TrimPrefix(strings.TrimSpace(source), "/uploads/")
	if index := strings.IndexAny(value, "?#"); index >= 0 {
		value = value[:index]
	}
	return strings.TrimSpace(value)
}

func buildAdminSpaceEPUBImageFileName(source string) string {
	sum := sha256.Sum256([]byte(source))
	extension := adminSpaceEPUBImageExtension(source)
	return "image-" + hex.EncodeToString(sum[:])[:16] + extension
}

func adminSpaceEPUBImageExtension(source string) string {
	lower := strings.ToLower(strings.TrimSpace(source))
	if strings.HasPrefix(lower, "data:") {
		header := lower
		if semicolon := strings.Index(header, ";"); semicolon >= 0 {
			header = header[:semicolon]
		}
		switch strings.TrimPrefix(header, "data:") {
		case "image/jpeg", "image/jpg":
			return ".jpg"
		case "image/gif":
			return ".gif"
		case "image/webp":
			return ".webp"
		case "image/svg+xml":
			return ".svg"
		default:
			return ".png"
		}
	}
	if ext := strings.TrimSpace(path.Ext(lower)); ext != "" && len(ext) <= 8 {
		return ext
	}
	return ".png"
}

func writeAdminSpaceEPUBFile(partPath string, book *epub.Epub) error {
	if book == nil {
		return fmt.Errorf("epub book is nil")
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
	if _, err := book.WriteTo(file); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		closeFile = false
		return err
	}
	closeFile = false
	return nil
}

func buildAdminSpaceExportEPUBFileName(spaceID string, exportedAt time.Time) string {
	return fmt.Sprintf(
		"space-%s-%s.epub",
		sanitizeAdminSpaceExportPathSegment(spaceID, "space"),
		exportedAt.Format("20060102150405"),
	)
}

func errorsIsRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
