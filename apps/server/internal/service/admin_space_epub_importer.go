package service

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/lifei6671/plaindoc/apps/server/internal/pkg/errcode"
	"golang.org/x/net/html"
)

const (
	adminSpaceEPUBMimetype          = "application/epub+zip"
	maxAdminSpaceEPUBEntries        = 2000
	maxAdminSpaceEPUBEntryBytes     = 32 << 20
	maxAdminSpaceEPUBTotalBytes     = 128 << 20
	maxAdminSpaceEPUBMetadataBytes  = 4 << 20
	maxAdminSpaceEPUBPathDepth      = 16
	defaultAdminSpaceEPUBVisibility = "member"
)

type adminSpaceEPUBPreview struct {
	SourcePublishedAt string
	SourceAuthors     []string
	Space             AdminSpaceImportPreviewSpace
	Summary           AdminSpaceExportManifestSummary
}

type adminSpaceEPUBContainer struct {
	Rootfiles []adminSpaceEPUBRootfile `xml:"rootfiles>rootfile"`
}

type adminSpaceEPUBRootfile struct {
	FullPath string `xml:"full-path,attr"`
}

type adminSpaceEPUBOPFPackage struct {
	Metadata adminSpaceEPUBOPFMetadata  `xml:"metadata"`
	Manifest []adminSpaceEPUBOPFItem    `xml:"manifest>item"`
	Spine    []adminSpaceEPUBOPFItemRef `xml:"spine>itemref"`
}

type adminSpaceEPUBOPFMetadata struct {
	Titles       []string                `xml:"title"`
	Creators     []string                `xml:"creator"`
	Dates        []string                `xml:"date"`
	Descriptions []string                `xml:"description"`
	Meta         []adminSpaceEPUBOPFMeta `xml:"meta"`
}

type adminSpaceEPUBOPFMeta struct {
	Name    string `xml:"name,attr"`
	Content string `xml:"content,attr"`
}

type adminSpaceEPUBOPFItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

type adminSpaceEPUBOPFItemRef struct {
	IDRef string `xml:"idref,attr"`
}

// adminSpaceEPUBNavItem 是 nav.xhtml / toc.ncx / spine fallback 解析后的统一目录项。
// 这里仍保留 EPUB 原始 href，后续预分配阶段再把它规范化为 PlainDoc 可落库的目标键。
type adminSpaceEPUBNavItem struct {
	Title    string
	Href     string
	Children []adminSpaceEPUBNavItem
}

// adminSpaceEPUBPlanInput 是目录预分配阶段的输入。
// ChapterHTMLByCanonicalHref 只用于判断 fragment 是否真实存在，正文清洗和 Markdown 转换放在后续阶段处理。
type adminSpaceEPUBPlanInput struct {
	OPFRoot                    string
	Items                      []adminSpaceEPUBNavItem
	ChapterHTMLByCanonicalHref map[string][]byte
	NewNodeID                  func() string
	NewDocumentID              func() string
}

// adminSpaceEPUBPlan 保存第一段预分配结果。
// Root 表示 PlainDoc 目录树草图，Targets 用于第二段 HTML 转换时重写 EPUB 内部链接。
type adminSpaceEPUBPlan struct {
	Root             []adminSpaceEPUBPlannedNode
	Targets          map[string]adminSpaceEPUBPlannedTarget
	CanonicalTargets map[string]adminSpaceEPUBPlannedTarget
}

type adminSpaceEPUBPlannedNodeType string

const (
	adminSpaceEPUBPlannedNodeTypeFolder adminSpaceEPUBPlannedNodeType = "folder"
	adminSpaceEPUBPlannedNodeTypeDoc    adminSpaceEPUBPlannedNodeType = "doc"
)

type adminSpaceEPUBPlannedNode struct {
	Type               adminSpaceEPUBPlannedNodeType
	Title              string
	NodeID             string
	DocumentID         string
	CanonicalHref      string
	Fragment           string
	TargetKey          string
	Reference          bool
	ReferenceTargetKey string
	Children           []adminSpaceEPUBPlannedNode
}

// adminSpaceEPUBPlannedTarget 是内部链接重写需要的最小目标信息。
// 预分配阶段先生成稳定 documentID，避免后续正文转换时再依赖写库顺序。
type adminSpaceEPUBPlannedTarget struct {
	Title      string
	DocumentID string
}

// adminSpaceEPUBImportPackage 是 commit 阶段使用的 EPUB 解析结果。
// 与 inspect 摘要不同，这里保留章节正文、zip entries 和目录项，用于后续写库和图片本地化。
type adminSpaceEPUBImportPackage struct {
	closer                     io.Closer
	OPFPath                    string
	OPFRoot                    string
	Title                      string
	Authors                    []string
	PublishedAt                string
	Description                string
	CoverPath                  string
	Entries                    map[string]*zip.File
	NavItems                   []adminSpaceEPUBNavItem
	ChapterHTMLByCanonicalHref map[string][]byte
}

// adminSpaceEPUBNormalizedHref 表示 EPUB 内部链接的安全规范化结果。
// TargetKey 带 fragment，CanonicalHref 不带 fragment，分别服务精确命中和整章降级。
type adminSpaceEPUBNormalizedHref struct {
	CanonicalHref string
	Fragment      string
	TargetKey     string
}

// adminSpaceEPUBHTMLSanitizeInput 表示单个 EPUB 章节 HTML 清洗请求。
// SourceKey 和 Title 只用于 warning 定位，避免导入日志里只有难排查的原始 href。
type adminSpaceEPUBHTMLSanitizeInput struct {
	SourceKey string
	Title     string
	HTML      []byte
}

// adminSpaceEPUBLinkRewriteInput 描述单章 HTML 内部链接重写请求。
// SourceCanonicalHref 用来解析相对链接；Plan 来自第一段预分配，保证链接重写不依赖写库顺序。
type adminSpaceEPUBLinkRewriteInput struct {
	SourceKey           string
	SourceCanonicalHref string
	SpaceID             string
	HTML                []byte
	Plan                adminSpaceEPUBPlan
}

// adminSpaceEPUBImageLocalizeInput 描述单章 HTML 图片本地化请求。
// Localize 由导入服务注入，后续会复用现有附件 blob / image hosting 能力写入本地资源。
type adminSpaceEPUBImageLocalizeInput struct {
	SourceKey           string
	SourceCanonicalHref string
	HTML                []byte
	Entries             map[string]*zip.File
	Localize            func(adminSpaceEPUBImageAsset) (string, error)
}

// adminSpaceEPUBImageAsset 是等待写入 PlainDoc 本地资源的图片内容。
// CanonicalHref 为空表示来源是 data:image/*，非空表示来源是 EPUB zip entry。
type adminSpaceEPUBImageAsset struct {
	Source        string
	CanonicalHref string
	FileName      string
	ContentType   string
	Payload       []byte
}

// inspectAdminSpaceImportEPUB 只做导入前预检，不创建空间、目录或文档。
// 预检阶段故意返回摘要与 warning，底层 zip/XML 错误统一映射为导入错误，避免把内部实现细节暴露给前端。
func inspectAdminSpaceImportEPUB(payload []byte) (adminSpaceEPUBPreview, []string, error) {
	warnings := []string{}
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return adminSpaceEPUBPreview{}, nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	entries, err := collectAdminSpaceEPUBEntries(reader)
	if err != nil {
		return adminSpaceEPUBPreview{}, nil, err
	}

	mimetypePayload, err := readAdminSpaceEPUBMetadataFile(entries["mimetype"])
	if err != nil || strings.TrimSpace(string(mimetypePayload)) != adminSpaceEPUBMimetype {
		return adminSpaceEPUBPreview{}, nil, errcode.ErrAdminSpaceImportZipInvalid
	}

	containerPayload, err := readAdminSpaceEPUBMetadataFile(entries["META-INF/container.xml"])
	if err != nil {
		return adminSpaceEPUBPreview{}, nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	var container adminSpaceEPUBContainer
	if err := unmarshalAdminSpaceEPUBXML(containerPayload, &container, &warnings, "META-INF/container.xml"); err != nil {
		return adminSpaceEPUBPreview{}, nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	opfPath := firstAdminSpaceEPUBRootfilePath(container)
	if opfPath == "" {
		return adminSpaceEPUBPreview{}, nil, errcode.ErrAdminSpaceImportZipInvalid
	}

	opfPayload, err := readAdminSpaceEPUBMetadataFile(entries[opfPath])
	if err != nil {
		return adminSpaceEPUBPreview{}, nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	var opf adminSpaceEPUBOPFPackage
	if err := unmarshalAdminSpaceEPUBXML(opfPayload, &opf, &warnings, opfPath); err != nil {
		return adminSpaceEPUBPreview{}, nil, errcode.ErrAdminSpaceImportZipInvalid
	}

	manifestByID := make(map[string]adminSpaceEPUBOPFItem, len(opf.Manifest))
	imageCount := 0
	var navItem adminSpaceEPUBOPFItem
	var tocItem adminSpaceEPUBOPFItem
	for _, item := range opf.Manifest {
		id := strings.TrimSpace(item.ID)
		if id != "" {
			manifestByID[id] = item
		}
		if usesAdminSpaceEPUBExtensionFallback(item) {
			warnings = append(warnings, "资源 "+strings.TrimSpace(item.Href)+" 的 media-type 不标准，已按文件扩展名识别")
		}
		if isAdminSpaceEPUBImageItem(item) {
			imageCount++
		}
		if strings.Contains(" "+strings.TrimSpace(item.Properties)+" ", " nav ") {
			navItem = item
		}
		if isAdminSpaceEPUBTOCItem(item) {
			tocItem = item
		}
	}

	documentCount := 0
	for _, itemRef := range opf.Spine {
		item, ok := manifestByID[strings.TrimSpace(itemRef.IDRef)]
		if !ok || !isAdminSpaceEPUBDocumentItem(item) {
			continue
		}
		documentCount++
	}
	if documentCount == 0 {
		return adminSpaceEPUBPreview{}, nil, errcode.ErrAdminSpaceImportPackageNotImportable
	}

	opfRoot := path.Dir(opfPath)
	if opfRoot == "." {
		opfRoot = ""
	}
	maxDepth := 1
	if strings.TrimSpace(navItem.Href) != "" {
		navPath := cleanAdminSpaceEPUBZipHref(opfRoot, navItem.Href)
		if navPath != "" {
			if navPayload, readErr := readAdminSpaceEPUBMetadataFile(entries[navPath]); readErr == nil {
				maxDepth = max(maxDepth, inspectAdminSpaceEPUBNavDepth(navPayload))
			}
		}
	} else if strings.TrimSpace(tocItem.Href) != "" {
		tocPath := cleanAdminSpaceEPUBZipHref(opfRoot, tocItem.Href)
		if tocPath != "" {
			if tocPayload, readErr := readAdminSpaceEPUBMetadataFile(entries[tocPath]); readErr == nil {
				maxDepth = max(maxDepth, inspectAdminSpaceEPUBTOCDepth(tocPayload))
			}
		}
	}

	title := firstNonEmptyEPUBString(opf.Metadata.Titles)
	if title == "" {
		title = strings.TrimSuffix(path.Base(strings.TrimSpace(opfPath)), filepath.Ext(opfPath))
	}
	if strings.TrimSpace(title) == "" {
		title = "EPUB 导入空间"
	}

	return adminSpaceEPUBPreview{
		SourcePublishedAt: firstNonEmptyEPUBString(opf.Metadata.Dates),
		SourceAuthors:     compactAdminSpaceEPUBStrings(opf.Metadata.Creators),
		Space: AdminSpaceImportPreviewSpace{
			SpaceID:    "epub-preview",
			Name:       title,
			Visibility: defaultAdminSpaceEPUBVisibility,
			HasCover:   resolveAdminSpaceEPUBCoverPath(opfRoot, opf) != "",
		},
		Summary: AdminSpaceExportManifestSummary{
			DocumentCount: documentCount,
			ImageCount:    imageCount,
			MaxDepth:      maxDepth,
		},
	}, warnings, nil
}

// collectAdminSpaceEPUBEntries 建立 zip entry 索引，并集中执行 EPUB 容器安全限制。
// 后续读取文件时只允许通过该索引访问，避免 zip slip、重复路径和超大包绕过检查。
func collectAdminSpaceEPUBEntries(reader *zip.Reader) (map[string]*zip.File, error) {
	if reader == nil || len(reader.File) == 0 || len(reader.File) > maxAdminSpaceEPUBEntries {
		return nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	entries := make(map[string]*zip.File, len(reader.File))
	var totalUncompressedBytes uint64
	for _, file := range reader.File {
		if file == nil {
			continue
		}
		if file.UncompressedSize64 > maxAdminSpaceEPUBEntryBytes {
			return nil, errcode.ErrAdminSpaceImportZipInvalid
		}
		totalUncompressedBytes += file.UncompressedSize64
		if totalUncompressedBytes > maxAdminSpaceEPUBTotalBytes {
			return nil, errcode.ErrAdminSpaceImportZipInvalid
		}
		entryName := cleanAdminSpaceImportZipEntry(file.Name)
		if entryName == "" {
			return nil, errcode.ErrAdminSpaceImportZipInvalid
		}
		// EPUB 内部路径只作为 zip entry key 使用，提前限制层级能避免异常深路径拖垮后续映射和清理逻辑。
		if adminSpaceEPUBPathDepth(entryName) > maxAdminSpaceEPUBPathDepth {
			return nil, errcode.ErrAdminSpaceImportZipInvalid
		}
		if _, exists := entries[entryName]; exists {
			return nil, errcode.ErrAdminSpaceImportZipInvalid
		}
		entries[entryName] = file
	}
	return entries, nil
}

func adminSpaceEPUBPathDepth(entryName string) int {
	cleaned := strings.Trim(strings.TrimSpace(entryName), "/")
	if cleaned == "" {
		return 0
	}
	return len(strings.Split(cleaned, "/"))
}

func readAdminSpaceEPUBMetadataFile(file *zip.File) ([]byte, error) {
	if file == nil || file.UncompressedSize64 > maxAdminSpaceEPUBMetadataBytes {
		return nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("打开 EPUB 元数据文件: %w", err)
	}
	defer reader.Close()
	payload, err := io.ReadAll(io.LimitReader(reader, maxAdminSpaceEPUBMetadataBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取 EPUB 元数据文件: %w", err)
	}
	if len(payload) > maxAdminSpaceEPUBMetadataBytes {
		return nil, errcode.ErrAdminSpaceImportZipInvalid
	}
	return payload, nil
}

func unmarshalAdminSpaceEPUBXML(payload []byte, value any, warnings *[]string, source string) error {
	decoder := xml.NewDecoder(bytes.NewReader(payload))
	decoder.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		normalized := strings.ToLower(strings.TrimSpace(charset))
		if normalized != "" && normalized != "utf-8" && normalized != "utf8" {
			// 首期只做 ASCII 兼容：不主动转码，但保留 warning，后续样本证明需要时再引入确定的 charset 转换策略。
			*warnings = append(*warnings, "XML 文件 "+strings.TrimSpace(source)+" 声明了非 UTF-8 编码 "+charset+"，已按有限兼容模式解析")
		}
		return input, nil
	}
	return decoder.Decode(value)
}

func firstAdminSpaceEPUBRootfilePath(container adminSpaceEPUBContainer) string {
	for _, rootfile := range container.Rootfiles {
		cleanPath := cleanAdminSpaceImportZipEntry(rootfile.FullPath)
		if cleanPath != "" {
			return cleanPath
		}
	}
	return ""
}

func resolveAdminSpaceEPUBCoverPath(opfRoot string, opf adminSpaceEPUBOPFPackage) string {
	// EPUB3 通过 manifest item 的 cover-image 属性声明封面。
	for _, item := range opf.Manifest {
		if !isAdminSpaceEPUBImageItem(item) || !hasAdminSpaceEPUBManifestProperty(item, "cover-image") {
			continue
		}
		if coverPath := cleanAdminSpaceEPUBZipHref(opfRoot, item.Href); coverPath != "" {
			return coverPath
		}
	}

	// EPUB2 常见写法是 <meta name="cover" content="cover-id"/>，
	// content 指向 manifest item id。
	manifestByID := buildAdminSpaceEPUBManifestByID(opf.Manifest)
	for _, meta := range opf.Metadata.Meta {
		if !strings.EqualFold(strings.TrimSpace(meta.Name), "cover") {
			continue
		}
		item, ok := manifestByID[strings.TrimSpace(meta.Content)]
		if !ok || !isAdminSpaceEPUBImageItem(item) {
			continue
		}
		if coverPath := cleanAdminSpaceEPUBZipHref(opfRoot, item.Href); coverPath != "" {
			return coverPath
		}
	}
	return ""
}

func hasAdminSpaceEPUBManifestProperty(item adminSpaceEPUBOPFItem, property string) bool {
	normalizedProperty := strings.TrimSpace(property)
	if normalizedProperty == "" {
		return false
	}
	for _, token := range strings.Fields(strings.TrimSpace(item.Properties)) {
		if strings.EqualFold(token, normalizedProperty) {
			return true
		}
	}
	return false
}

func isAdminSpaceEPUBDocumentItem(item adminSpaceEPUBOPFItem) bool {
	mediaType := strings.ToLower(strings.TrimSpace(item.MediaType))
	if mediaType == "application/xhtml+xml" || mediaType == "text/html" {
		return true
	}
	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(item.Href)))
	return extension == ".xhtml" || extension == ".html" || extension == ".htm"
}

func isAdminSpaceEPUBImageItem(item adminSpaceEPUBOPFItem) bool {
	mediaType := strings.ToLower(strings.TrimSpace(item.MediaType))
	if strings.HasPrefix(mediaType, "image/") {
		return isSupportedAdminSpaceEPUBImageContentType(mediaType)
	}
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(item.Href))) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return true
	default:
		return false
	}
}

func isAdminSpaceEPUBTOCItem(item adminSpaceEPUBOPFItem) bool {
	mediaType := strings.ToLower(strings.TrimSpace(item.MediaType))
	if mediaType == "application/x-dtbncx+xml" {
		return true
	}
	return strings.EqualFold(filepath.Ext(strings.TrimSpace(item.Href)), ".ncx")
}

func usesAdminSpaceEPUBExtensionFallback(item adminSpaceEPUBOPFItem) bool {
	mediaType := strings.ToLower(strings.TrimSpace(item.MediaType))
	if mediaType == "" {
		return isAdminSpaceEPUBDocumentExtension(item) || isAdminSpaceEPUBImageExtension(item) || isAdminSpaceEPUBTOCItem(item)
	}
	if isAdminSpaceEPUBDocumentItem(item) && mediaType != "application/xhtml+xml" && mediaType != "text/html" {
		return true
	}
	if isAdminSpaceEPUBImageItem(item) && !strings.HasPrefix(mediaType, "image/") {
		return true
	}
	return false
}

func isAdminSpaceEPUBDocumentExtension(item adminSpaceEPUBOPFItem) bool {
	extension := strings.ToLower(filepath.Ext(strings.TrimSpace(item.Href)))
	return extension == ".xhtml" || extension == ".html" || extension == ".htm"
}

func isAdminSpaceEPUBImageExtension(item adminSpaceEPUBOPFItem) bool {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(item.Href))) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return true
	default:
		return false
	}
}

func inspectAdminSpaceEPUBNavDepth(payload []byte) int {
	root, err := html.Parse(bytes.NewReader(payload))
	if err != nil {
		return 1
	}
	maxDepth := 1
	var walk func(node *html.Node, depth int)
	walk = func(node *html.Node, depth int) {
		if node == nil {
			return
		}
		nextDepth := depth
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "li") {
			nextDepth = depth + 1
			if nextDepth > maxDepth {
				maxDepth = nextDepth
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child, nextDepth)
		}
	}
	walk(root, 0)
	return maxDepth
}

func inspectAdminSpaceEPUBTOCDepth(payload []byte) int {
	root, err := html.Parse(bytes.NewReader(payload))
	if err != nil {
		return 1
	}
	maxDepth := 1
	var walk func(node *html.Node, depth int)
	walk = func(node *html.Node, depth int) {
		if node == nil {
			return
		}
		nextDepth := depth
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "navPoint") {
			nextDepth = depth + 1
			if nextDepth > maxDepth {
				maxDepth = nextDepth
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child, nextDepth)
		}
	}
	walk(root, 0)
	return maxDepth
}

func parseAdminSpaceEPUBNavDocument(payload []byte) ([]adminSpaceEPUBNavItem, error) {
	root, err := html.Parse(bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("解析 EPUB nav: %w", err)
	}
	nav := findAdminSpaceEPUBTOCNav(root)
	if nav == nil {
		nav = root
	}
	orderedList := firstAdminSpaceEPUBDescendantElement(nav, "ol")
	if orderedList == nil {
		return nil, nil
	}
	return parseAdminSpaceEPUBNavList(orderedList), nil
}

func parseAdminSpaceEPUBTOCDocument(payload []byte) ([]adminSpaceEPUBNavItem, error) {
	decoder := xml.NewDecoder(bytes.NewReader(payload))
	type tocFrame struct {
		item   adminSpaceEPUBNavItem
		inText bool
	}
	stack := make([]tocFrame, 0)
	rootItems := make([]adminSpaceEPUBNavItem, 0)
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("解析 EPUB toc.ncx: %w", err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			switch strings.ToLower(value.Name.Local) {
			case "navpoint":
				stack = append(stack, tocFrame{})
			case "content":
				if len(stack) == 0 {
					continue
				}
				for _, attr := range value.Attr {
					if strings.EqualFold(attr.Name.Local, "src") {
						stack[len(stack)-1].item.Href = strings.TrimSpace(attr.Value)
						break
					}
				}
			case "text":
				if len(stack) > 0 {
					stack[len(stack)-1].inText = true
				}
			}
		case xml.CharData:
			if len(stack) > 0 && stack[len(stack)-1].inText {
				stack[len(stack)-1].item.Title += string(value)
			}
		case xml.EndElement:
			switch strings.ToLower(value.Name.Local) {
			case "text":
				if len(stack) > 0 {
					stack[len(stack)-1].inText = false
				}
			case "navpoint":
				if len(stack) == 0 {
					continue
				}
				frame := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				frame.item.Title = strings.TrimSpace(strings.Join(strings.Fields(frame.item.Title), " "))
				if frame.item.Title == "" && frame.item.Href != "" {
					frame.item.Title = strings.TrimSuffix(path.Base(frame.item.Href), path.Ext(frame.item.Href))
				}
				if frame.item.Title == "" && frame.item.Href == "" && len(frame.item.Children) == 0 {
					continue
				}
				if len(stack) > 0 {
					parent := &stack[len(stack)-1]
					parent.item.Children = append(parent.item.Children, frame.item)
				} else {
					rootItems = append(rootItems, frame.item)
				}
			}
		}
	}
	return rootItems, nil
}

func findAdminSpaceEPUBTOCNav(root *html.Node) *html.Node {
	if root == nil {
		return nil
	}
	if root.Type == html.ElementNode && strings.EqualFold(root.Data, "nav") {
		for _, attr := range root.Attr {
			if strings.EqualFold(attr.Key, "epub:type") || strings.EqualFold(attr.Key, "type") {
				if strings.Contains(" "+strings.ToLower(strings.TrimSpace(attr.Val))+" ", " toc ") {
					return root
				}
			}
		}
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if found := findAdminSpaceEPUBTOCNav(child); found != nil {
			return found
		}
	}
	if root.Type == html.ElementNode && strings.EqualFold(root.Data, "nav") {
		return root
	}
	return nil
}

func parseAdminSpaceEPUBNavList(list *html.Node) []adminSpaceEPUBNavItem {
	items := make([]adminSpaceEPUBNavItem, 0)
	for child := list.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode || !strings.EqualFold(child.Data, "li") {
			continue
		}
		item := parseAdminSpaceEPUBNavListItem(child)
		if strings.TrimSpace(item.Title) == "" && strings.TrimSpace(item.Href) == "" && len(item.Children) == 0 {
			continue
		}
		items = append(items, item)
	}
	return items
}

func parseAdminSpaceEPUBNavListItem(listItem *html.Node) adminSpaceEPUBNavItem {
	var item adminSpaceEPUBNavItem
	for child := listItem.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		switch strings.ToLower(child.Data) {
		case "a":
			if strings.TrimSpace(item.Title) == "" {
				item.Title = textContentAdminSpaceEPUB(child)
			}
			if strings.TrimSpace(item.Href) == "" {
				item.Href = htmlAttribute(child, "href")
			}
		case "span":
			if strings.TrimSpace(item.Title) == "" {
				item.Title = textContentAdminSpaceEPUB(child)
			}
		case "ol":
			item.Children = append(item.Children, parseAdminSpaceEPUBNavList(child)...)
		default:
			// nav.xhtml 中常见 <div><a>...</a></div> 包裹结构，这里只在当前层补齐标题和 href。
			if strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.Href) == "" {
				if link := firstAdminSpaceEPUBDescendantElement(child, "a"); link != nil {
					if strings.TrimSpace(item.Title) == "" {
						item.Title = textContentAdminSpaceEPUB(link)
					}
					if strings.TrimSpace(item.Href) == "" {
						item.Href = htmlAttribute(link, "href")
					}
				} else if span := firstAdminSpaceEPUBDescendantElement(child, "span"); span != nil && strings.TrimSpace(item.Title) == "" {
					item.Title = textContentAdminSpaceEPUB(span)
				}
			}
			if nested := firstDirectAdminSpaceEPUBChildElement(child, "ol"); nested != nil {
				item.Children = append(item.Children, parseAdminSpaceEPUBNavList(nested)...)
			}
		}
	}
	if strings.TrimSpace(item.Title) == "" && strings.TrimSpace(item.Href) != "" {
		item.Title = strings.TrimSuffix(path.Base(strings.TrimSpace(item.Href)), path.Ext(strings.TrimSpace(item.Href)))
	}
	return item
}

func firstAdminSpaceEPUBDescendantElement(root *html.Node, tagName string) *html.Node {
	if root == nil {
		return nil
	}
	if root.Type == html.ElementNode && strings.EqualFold(root.Data, tagName) {
		return root
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if found := firstAdminSpaceEPUBDescendantElement(child, tagName); found != nil {
			return found
		}
	}
	return nil
}

func firstDirectAdminSpaceEPUBChildElement(root *html.Node, tagName string) *html.Node {
	if root == nil {
		return nil
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && strings.EqualFold(child.Data, tagName) {
			return child
		}
	}
	return nil
}

func textContentAdminSpaceEPUB(root *html.Node) string {
	var builder strings.Builder
	var walk func(node *html.Node)
	walk = func(node *html.Node) {
		if node == nil {
			return
		}
		if node.Type == html.TextNode {
			builder.WriteString(node.Data)
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return strings.TrimSpace(strings.Join(strings.Fields(builder.String()), " "))
}

func firstNonEmptyEPUBString(values []string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func compactAdminSpaceEPUBStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

// sanitizeAdminSpaceEPUBChapterHTML 清洗单章 EPUB XHTML/HTML，并只返回 body 主体片段。
// 清洗阶段只负责去除危险结构和协议，内部 EPUB 相对链接会保留下来，等待 commit 阶段基于 target 映射重写。
func sanitizeAdminSpaceEPUBChapterHTML(input adminSpaceEPUBHTMLSanitizeInput) (string, []string, error) {
	root, err := html.Parse(bytes.NewReader(input.HTML))
	if err != nil {
		return "", nil, fmt.Errorf("解析 EPUB 章节 HTML: %w", err)
	}
	warnings := []string{}
	body := firstAdminSpaceEPUBHTMLElement(root, "body")
	if body == nil {
		body = root
	}
	sanitizeAdminSpaceEPUBHTMLChildren(body, input, &warnings)

	var builder strings.Builder
	for child := body.FirstChild; child != nil; child = child.NextSibling {
		if err := html.Render(&builder, child); err != nil {
			return "", warnings, fmt.Errorf("渲染 EPUB 清洗后 HTML: %w", err)
		}
	}
	return strings.TrimSpace(builder.String()), warnings, nil
}

func sanitizeAdminSpaceEPUBHTMLChildren(parent *html.Node, input adminSpaceEPUBHTMLSanitizeInput, warnings *[]string) {
	for child := parent.FirstChild; child != nil; {
		next := child.NextSibling
		sanitizeAdminSpaceEPUBHTMLNode(parent, child, input, warnings)
		child = next
	}
}

func sanitizeAdminSpaceEPUBHTMLNode(parent *html.Node, node *html.Node, input adminSpaceEPUBHTMLSanitizeInput, warnings *[]string) {
	if node == nil || parent == nil {
		return
	}
	if node.Type != html.ElementNode {
		sanitizeAdminSpaceEPUBHTMLChildren(node, input, warnings)
		return
	}

	tagName := strings.ToLower(strings.TrimSpace(node.Data))
	if isBlockedAdminSpaceEPUBHTMLTag(tagName) {
		*warnings = append(*warnings, adminSpaceEPUBHTMLSanitizeWarning(input, "已移除危险标签 <"+tagName+">"))
		parent.RemoveChild(node)
		return
	}

	removeNodeAfterChildren := false
	replaceNodeWithAltText := false
	altText := ""
	sanitizedAttrs := make([]html.Attribute, 0, len(node.Attr))
	for _, attr := range node.Attr {
		attrKey := strings.ToLower(strings.TrimSpace(attr.Key))
		if attrKey == "" {
			continue
		}
		if strings.HasPrefix(attrKey, "on") {
			*warnings = append(*warnings, adminSpaceEPUBHTMLSanitizeWarning(input, "已移除事件属性 "+attr.Key))
			continue
		}
		switch {
		case tagName == "a" && attrKey == "href":
			if adminSpaceEPUBURLNeedsTextFallback(attr.Val, false) {
				*warnings = append(*warnings, adminSpaceEPUBHTMLSanitizeWarning(input, "已降级危险链接 "+strings.TrimSpace(attr.Val)))
				removeNodeAfterChildren = true
				continue
			}
		case tagName == "img" && attrKey == "src":
			if adminSpaceEPUBURLNeedsTextFallback(attr.Val, true) {
				*warnings = append(*warnings, adminSpaceEPUBHTMLSanitizeWarning(input, "已降级危险图片 "+strings.TrimSpace(attr.Val)))
				replaceNodeWithAltText = true
				continue
			}
		case tagName == "img" && attrKey == "alt":
			altText = strings.TrimSpace(attr.Val)
		}
		sanitizedAttrs = append(sanitizedAttrs, html.Attribute{Namespace: attr.Namespace, Key: attr.Key, Val: attr.Val})
	}
	node.Attr = sanitizedAttrs

	if replaceNodeWithAltText {
		replaceAdminSpaceEPUBHTMLNodeWithText(parent, node, altText)
		return
	}

	sanitizeAdminSpaceEPUBHTMLChildren(node, input, warnings)
	if removeNodeAfterChildren {
		unwrapAdminSpaceEPUBHTMLNode(parent, node)
	}
}

func isBlockedAdminSpaceEPUBHTMLTag(tagName string) bool {
	switch tagName {
	case "script", "style", "form", "input", "button", "noscript":
		return true
	default:
		return false
	}
}

func adminSpaceEPUBURLNeedsTextFallback(rawURL string, allowDataImage bool) bool {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return false
	}
	lowerURL := strings.ToLower(trimmed)
	if strings.Contains(trimmed, "\\") || strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "~") || strings.HasPrefix(trimmed, "//") {
		return true
	}
	if len(trimmed) >= 3 && trimmed[1] == ':' && ((trimmed[0] >= 'a' && trimmed[0] <= 'z') || (trimmed[0] >= 'A' && trimmed[0] <= 'Z')) {
		return true
	}
	if strings.HasPrefix(lowerURL, "http://") || strings.HasPrefix(lowerURL, "https://") {
		return false
	}
	if strings.HasPrefix(lowerURL, "data:") {
		return !(allowDataImage && strings.HasPrefix(lowerURL, "data:image/"))
	}
	if colonIndex := strings.Index(lowerURL, ":"); colonIndex > 0 {
		slashIndex := strings.IndexAny(lowerURL, "/?#")
		if slashIndex == -1 || colonIndex < slashIndex {
			return true
		}
	}
	return false
}

func firstAdminSpaceEPUBHTMLElement(root *html.Node, tagName string) *html.Node {
	if root == nil {
		return nil
	}
	if root.Type == html.ElementNode && strings.EqualFold(root.Data, tagName) {
		return root
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if found := firstAdminSpaceEPUBHTMLElement(child, tagName); found != nil {
			return found
		}
	}
	return nil
}

func replaceAdminSpaceEPUBHTMLNodeWithText(parent *html.Node, node *html.Node, text string) {
	if strings.TrimSpace(text) != "" {
		parent.InsertBefore(&html.Node{Type: html.TextNode, Data: text}, node)
	}
	parent.RemoveChild(node)
}

func unwrapAdminSpaceEPUBHTMLNode(parent *html.Node, node *html.Node) {
	for child := node.FirstChild; child != nil; {
		next := child.NextSibling
		node.RemoveChild(child)
		parent.InsertBefore(child, node)
		child = next
	}
	parent.RemoveChild(node)
}

func adminSpaceEPUBHTMLSanitizeWarning(input adminSpaceEPUBHTMLSanitizeInput, detail string) string {
	sourceKey := strings.TrimSpace(input.SourceKey)
	if sourceKey == "" {
		sourceKey = "unknown"
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "未命名章节"
	}
	return "EPUB 章节 " + title + " (" + sourceKey + ")：" + detail
}

// rewriteAdminSpaceEPUBInternalLinks 执行第二段链接重写。
// exact targetKey 命中时写入目标阅读链接；仅 canonical 命中时降级到主文档；完全未命中时移除 href 保留文本。
func rewriteAdminSpaceEPUBInternalLinks(input adminSpaceEPUBLinkRewriteInput) (string, []string, error) {
	root, err := html.Parse(bytes.NewReader(input.HTML))
	if err != nil {
		return "", nil, fmt.Errorf("解析 EPUB 内部链接 HTML: %w", err)
	}
	warnings := []string{}
	body := firstAdminSpaceEPUBHTMLElement(root, "body")
	if body == nil {
		body = root
	}
	rewriteAdminSpaceEPUBLinkNodes(body, input, &warnings)

	var builder strings.Builder
	for child := body.FirstChild; child != nil; child = child.NextSibling {
		if err := html.Render(&builder, child); err != nil {
			return "", warnings, fmt.Errorf("渲染 EPUB 内部链接 HTML: %w", err)
		}
	}
	return strings.TrimSpace(builder.String()), warnings, nil
}

func rewriteAdminSpaceEPUBLinkNodes(node *html.Node, input adminSpaceEPUBLinkRewriteInput, warnings *[]string) {
	if node == nil {
		return
	}
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, "a") {
		rewriteAdminSpaceEPUBAnchor(node, input, warnings)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		rewriteAdminSpaceEPUBLinkNodes(child, input, warnings)
	}
}

func rewriteAdminSpaceEPUBAnchor(node *html.Node, input adminSpaceEPUBLinkRewriteInput, warnings *[]string) {
	href := strings.TrimSpace(htmlAttribute(node, "href"))
	if href == "" || isExternalAdminSpaceEPUBLink(href) {
		return
	}
	normalized, ok := normalizeAdminSpaceEPUBContentHref(input.SourceCanonicalHref, href)
	if !ok {
		removeAdminSpaceEPUBHTMLAttribute(node, "href")
		*warnings = append(*warnings, adminSpaceEPUBLinkRewriteWarning(input, "链接 "+href+" 无法安全解析，已移除 href"))
		return
	}
	if target, exists := input.Plan.Targets[normalized.TargetKey]; exists {
		setAdminSpaceEPUBHTMLAttribute(node, "href", buildAdminSpaceEPUBImportedReaderURL(input.SpaceID, target.DocumentID))
		return
	}
	if target, exists := input.Plan.CanonicalTargets[normalized.CanonicalHref]; exists {
		setAdminSpaceEPUBHTMLAttribute(node, "href", buildAdminSpaceEPUBImportedReaderURL(input.SpaceID, target.DocumentID))
		*warnings = append(*warnings, adminSpaceEPUBLinkRewriteWarning(input, "链接 "+href+" 未命中 fragment，已降级到整章文档"))
		return
	}
	removeAdminSpaceEPUBHTMLAttribute(node, "href")
	*warnings = append(*warnings, adminSpaceEPUBLinkRewriteWarning(input, "链接 "+href+" 未命中导入目标，已保留文本并移除 href"))
}

func normalizeAdminSpaceEPUBContentHref(sourceCanonicalHref string, rawHref string) (adminSpaceEPUBNormalizedHref, bool) {
	trimmed := strings.TrimSpace(rawHref)
	if trimmed == "" || strings.Contains(trimmed, "\\") {
		return adminSpaceEPUBNormalizedHref{}, false
	}
	lowerHref := strings.ToLower(trimmed)
	if strings.HasPrefix(lowerHref, "javascript:") ||
		strings.HasPrefix(lowerHref, "file:") ||
		strings.HasPrefix(lowerHref, "data:") {
		return adminSpaceEPUBNormalizedHref{}, false
	}
	hrefPath, fragment, ok := splitAdminSpaceEPUBHrefPath(trimmed)
	if !ok {
		return adminSpaceEPUBNormalizedHref{}, false
	}

	var canonicalHref string
	if strings.TrimSpace(hrefPath) == "" {
		canonicalHref = cleanAdminSpaceImportZipEntry(strings.TrimSpace(sourceCanonicalHref))
	} else {
		canonicalHref = cleanAdminSpaceImportZipEntry(path.Join(path.Dir(strings.TrimSpace(sourceCanonicalHref)), strings.TrimSpace(hrefPath)))
	}
	if canonicalHref == "" {
		return adminSpaceEPUBNormalizedHref{}, false
	}
	targetKey := canonicalHref
	if fragment != "" {
		targetKey += "#" + fragment
	}
	return adminSpaceEPUBNormalizedHref{
		CanonicalHref: canonicalHref,
		Fragment:      fragment,
		TargetKey:     targetKey,
	}, true
}

func isExternalAdminSpaceEPUBLink(rawHref string) bool {
	lowerHref := strings.ToLower(strings.TrimSpace(rawHref))
	return strings.HasPrefix(lowerHref, "http://") || strings.HasPrefix(lowerHref, "https://") || strings.HasPrefix(lowerHref, "mailto:")
}

func setAdminSpaceEPUBHTMLAttribute(node *html.Node, key string, value string) {
	if node == nil {
		return
	}
	for index, attr := range node.Attr {
		if strings.EqualFold(attr.Key, key) {
			node.Attr[index].Val = value
			return
		}
	}
	node.Attr = append(node.Attr, html.Attribute{Key: key, Val: value})
}

func removeAdminSpaceEPUBHTMLAttribute(node *html.Node, key string) {
	if node == nil {
		return
	}
	attrs := node.Attr[:0]
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, key) {
			continue
		}
		attrs = append(attrs, attr)
	}
	node.Attr = attrs
}

func adminSpaceEPUBLinkRewriteWarning(input adminSpaceEPUBLinkRewriteInput, detail string) string {
	sourceKey := strings.TrimSpace(input.SourceKey)
	if sourceKey == "" {
		sourceKey = strings.TrimSpace(input.SourceCanonicalHref)
	}
	if sourceKey == "" {
		sourceKey = "unknown"
	}
	return "EPUB 内部链接 " + sourceKey + "：" + detail
}

// localizeAdminSpaceEPUBChapterImages 将章节 HTML 中的图片改写为 PlainDoc 本地资源 URL。
// 单张图片失败时会降级为 alt 文本并记录 warning，避免一本 EPUB 因个别图片异常整体失败。
func localizeAdminSpaceEPUBChapterImages(input adminSpaceEPUBImageLocalizeInput) (string, []string, error) {
	root, err := html.Parse(bytes.NewReader(input.HTML))
	if err != nil {
		return "", nil, fmt.Errorf("解析 EPUB 图片 HTML: %w", err)
	}
	warnings := []string{}
	body := firstAdminSpaceEPUBHTMLElement(root, "body")
	if body == nil {
		body = root
	}
	localizeAdminSpaceEPUBImageNodes(body, input, &warnings)

	var builder strings.Builder
	for child := body.FirstChild; child != nil; child = child.NextSibling {
		if err := html.Render(&builder, child); err != nil {
			return "", warnings, fmt.Errorf("渲染 EPUB 图片 HTML: %w", err)
		}
	}
	return strings.TrimSpace(builder.String()), warnings, nil
}

func localizeAdminSpaceEPUBImageNodes(node *html.Node, input adminSpaceEPUBImageLocalizeInput, warnings *[]string) {
	if node == nil {
		return
	}
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, "img") {
		localizeAdminSpaceEPUBImageNode(node, input, warnings)
		return
	}
	for child := node.FirstChild; child != nil; {
		next := child.NextSibling
		localizeAdminSpaceEPUBImageNodes(child, input, warnings)
		child = next
	}
}

func localizeAdminSpaceEPUBImageNode(node *html.Node, input adminSpaceEPUBImageLocalizeInput, warnings *[]string) {
	src := strings.TrimSpace(htmlAttribute(node, "src"))
	if src == "" {
		return
	}
	asset, err := loadAdminSpaceEPUBImageAsset(input, src)
	if err != nil {
		degradeAdminSpaceEPUBImportImageNode(node, input, warnings, "图片 "+src+" 无法本地化："+err.Error())
		return
	}
	if input.Localize == nil {
		degradeAdminSpaceEPUBImportImageNode(node, input, warnings, "图片 "+src+" 缺少本地化写入器")
		return
	}
	localURL, err := input.Localize(asset)
	if err != nil || strings.TrimSpace(localURL) == "" {
		if err == nil {
			err = fmt.Errorf("本地化 URL 为空")
		}
		degradeAdminSpaceEPUBImportImageNode(node, input, warnings, "图片 "+src+" 写入本地资源失败："+err.Error())
		return
	}
	setAdminSpaceEPUBHTMLAttribute(node, "src", strings.TrimSpace(localURL))
}

func loadAdminSpaceEPUBImageAsset(input adminSpaceEPUBImageLocalizeInput, rawSrc string) (adminSpaceEPUBImageAsset, error) {
	lowerSrc := strings.ToLower(strings.TrimSpace(rawSrc))
	if strings.HasPrefix(lowerSrc, "data:image/") {
		return parseAdminSpaceEPUBDataImageAsset(rawSrc)
	}
	if isExternalAdminSpaceEPUBLink(rawSrc) || strings.HasPrefix(lowerSrc, "cid:") {
		return adminSpaceEPUBImageAsset{}, fmt.Errorf("图片不是 EPUB 内部资源")
	}
	normalized, ok := normalizeAdminSpaceEPUBContentHref(input.SourceCanonicalHref, rawSrc)
	if !ok {
		return adminSpaceEPUBImageAsset{}, fmt.Errorf("图片路径无法安全解析")
	}
	payload, err := readAdminSpaceEPUBImageEntry(input.Entries[normalized.CanonicalHref])
	if err != nil {
		return adminSpaceEPUBImageAsset{}, err
	}
	contentType := adminSpaceEPUBImageContentType(normalized.CanonicalHref)
	if contentType == "" {
		return adminSpaceEPUBImageAsset{}, fmt.Errorf("不支持的图片类型")
	}
	return adminSpaceEPUBImageAsset{
		Source:        rawSrc,
		CanonicalHref: normalized.CanonicalHref,
		FileName:      path.Base(normalized.CanonicalHref),
		ContentType:   contentType,
		Payload:       payload,
	}, nil
}

func parseAdminSpaceEPUBDataImageAsset(rawSrc string) (adminSpaceEPUBImageAsset, error) {
	commaIndex := strings.Index(rawSrc, ",")
	if commaIndex < 0 {
		return adminSpaceEPUBImageAsset{}, fmt.Errorf("data image 缺少 payload")
	}
	header := strings.ToLower(strings.TrimSpace(rawSrc[:commaIndex]))
	contentType := strings.TrimPrefix(header, "data:")
	if semicolonIndex := strings.Index(contentType, ";"); semicolonIndex >= 0 {
		contentType = contentType[:semicolonIndex]
	}
	if !isSupportedAdminSpaceEPUBImageContentType(contentType) {
		return adminSpaceEPUBImageAsset{}, fmt.Errorf("不支持的 data image 类型")
	}
	rawPayload := rawSrc[commaIndex+1:]
	var payload []byte
	var err error
	if strings.Contains(header, ";base64") {
		trimmedPayload := strings.TrimSpace(rawPayload)
		if err := validateAdminSpaceEPUBImageSize(int64(base64.StdEncoding.DecodedLen(len(trimmedPayload)))); err != nil {
			return adminSpaceEPUBImageAsset{}, err
		}
		payload, err = base64.StdEncoding.DecodeString(trimmedPayload)
	} else {
		if err := validateAdminSpaceEPUBImageSize(int64(len(rawPayload))); err != nil {
			return adminSpaceEPUBImageAsset{}, err
		}
		decoded, decodeErr := url.PathUnescape(rawPayload)
		if decodeErr != nil {
			return adminSpaceEPUBImageAsset{}, decodeErr
		}
		payload = []byte(decoded)
	}
	if err != nil {
		return adminSpaceEPUBImageAsset{}, err
	}
	if len(payload) == 0 {
		return adminSpaceEPUBImageAsset{}, fmt.Errorf("data image payload 为空")
	}
	if err := validateAdminSpaceEPUBImageSize(int64(len(payload))); err != nil {
		return adminSpaceEPUBImageAsset{}, err
	}
	return adminSpaceEPUBImageAsset{
		Source:      rawSrc,
		FileName:    "inline" + adminSpaceEPUBImageExtensionForContentType(contentType),
		ContentType: contentType,
		Payload:     payload,
	}, nil
}

func readAdminSpaceEPUBImageEntry(file *zip.File) ([]byte, error) {
	if file == nil {
		return nil, fmt.Errorf("图片 entry 不存在")
	}
	if err := validateAdminSpaceEPUBImageSize(int64(file.UncompressedSize64)); err != nil {
		return nil, err
	}
	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("打开 EPUB 图片 entry: %w", err)
	}
	defer reader.Close()
	payload, err := io.ReadAll(io.LimitReader(reader, maxAdminSpaceEPUBImageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取 EPUB 图片 entry: %w", err)
	}
	if err := validateAdminSpaceEPUBImageSize(int64(len(payload))); err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("图片 entry 为空")
	}
	return payload, nil
}

func adminSpaceEPUBImageContentType(canonicalHref string) string {
	switch strings.ToLower(path.Ext(strings.TrimSpace(canonicalHref))) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return ""
	}
}

func isSupportedAdminSpaceEPUBImageContentType(contentType string) bool {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/png", "image/jpeg", "image/jpg", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

func adminSpaceEPUBImageExtensionForContentType(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".png"
	}
}

func degradeAdminSpaceEPUBImportImageNode(node *html.Node, input adminSpaceEPUBImageLocalizeInput, warnings *[]string, detail string) {
	alt := strings.TrimSpace(htmlAttribute(node, "alt"))
	if alt == "" {
		alt = "图片无法导入"
	}
	*warnings = append(*warnings, adminSpaceEPUBImageLocalizeWarning(input, detail))
	if node.Parent != nil {
		replaceAdminSpaceEPUBHTMLNodeWithText(node.Parent, node, alt)
	}
}

func adminSpaceEPUBImageLocalizeWarning(input adminSpaceEPUBImageLocalizeInput, detail string) string {
	sourceKey := strings.TrimSpace(input.SourceKey)
	if sourceKey == "" {
		sourceKey = strings.TrimSpace(input.SourceCanonicalHref)
	}
	if sourceKey == "" {
		sourceKey = "unknown"
	}
	return "EPUB 图片 " + sourceKey + "：" + detail
}

// normalizeAdminSpaceEPUBHref 将 EPUB href 规范化为 zip 内 canonicalHref + fragment。
// 危险协议和外部 URL 在这里直接拒绝，内部相对路径才允许进入后续 target 映射和链接重写。
func normalizeAdminSpaceEPUBHref(opfRoot string, rawHref string) (adminSpaceEPUBNormalizedHref, bool) {
	trimmed := strings.TrimSpace(rawHref)
	if trimmed == "" || strings.Contains(trimmed, "\\") {
		return adminSpaceEPUBNormalizedHref{}, false
	}
	lowerHref := strings.ToLower(trimmed)
	if strings.HasPrefix(lowerHref, "javascript:") ||
		strings.HasPrefix(lowerHref, "file:") ||
		strings.HasPrefix(lowerHref, "data:") ||
		strings.HasPrefix(lowerHref, "http://") ||
		strings.HasPrefix(lowerHref, "https://") {
		return adminSpaceEPUBNormalizedHref{}, false
	}
	hrefPath, fragment, ok := splitAdminSpaceEPUBHrefPath(trimmed)
	if !ok {
		return adminSpaceEPUBNormalizedHref{}, false
	}
	canonicalHref := cleanAdminSpaceImportZipEntry(path.Join(strings.TrimSpace(opfRoot), strings.TrimSpace(hrefPath)))
	if canonicalHref == "" {
		return adminSpaceEPUBNormalizedHref{}, false
	}
	targetKey := canonicalHref
	if fragment != "" {
		targetKey += "#" + fragment
	}
	return adminSpaceEPUBNormalizedHref{
		CanonicalHref: canonicalHref,
		Fragment:      fragment,
		TargetKey:     targetKey,
	}, true
}

func cleanAdminSpaceEPUBZipHref(basePath string, rawHref string) string {
	hrefPath, _, ok := splitAdminSpaceEPUBHrefPath(rawHref)
	if !ok || strings.TrimSpace(hrefPath) == "" {
		return ""
	}
	return cleanAdminSpaceImportZipEntry(path.Join(strings.TrimSpace(basePath), strings.TrimSpace(hrefPath)))
}

func splitAdminSpaceEPUBHrefPath(rawHref string) (string, string, bool) {
	trimmed := strings.TrimSpace(rawHref)
	if trimmed == "" || strings.Contains(trimmed, "\\") {
		return "", "", false
	}
	hrefWithoutFragment := trimmed
	fragment := ""
	if hashIndex := strings.Index(hrefWithoutFragment, "#"); hashIndex >= 0 {
		fragment = strings.TrimSpace(hrefWithoutFragment[hashIndex+1:])
		hrefWithoutFragment = hrefWithoutFragment[:hashIndex]
	}
	if queryIndex := strings.Index(hrefWithoutFragment, "?"); queryIndex >= 0 {
		hrefWithoutFragment = hrefWithoutFragment[:queryIndex]
	}
	decodedPath, err := url.PathUnescape(strings.TrimSpace(hrefWithoutFragment))
	if err != nil {
		return "", "", false
	}
	if fragment != "" {
		decodedFragment, err := url.PathUnescape(fragment)
		if err != nil {
			return "", "", false
		}
		fragment = strings.TrimSpace(decodedFragment)
	}
	return strings.TrimSpace(decodedPath), fragment, true
}

// planAdminSpaceEPUBImportTree 执行第一段目录规划：只分配节点、文档和 target 映射。
// 这一阶段不读取正文、不做 Markdown 转换，保证目录结构和链接目标可以先稳定下来。
func planAdminSpaceEPUBImportTree(input adminSpaceEPUBPlanInput) (adminSpaceEPUBPlan, []string) {
	planner := adminSpaceEPUBTreePlanner{
		opfRoot:                    strings.TrimSpace(input.OPFRoot),
		chapterHTMLByCanonicalHref: input.ChapterHTMLByCanonicalHref,
		targets:                    make(map[string]adminSpaceEPUBPlannedTarget),
		canonicalTargets:           make(map[string]string),
		newNodeID:                  input.NewNodeID,
		newDocumentID:              input.NewDocumentID,
	}
	rootTitleCounts := make(map[string]int)
	root := make([]adminSpaceEPUBPlannedNode, 0, len(input.Items))
	for _, item := range input.Items {
		node := planner.planItem(item, rootTitleCounts)
		if node.Title == "" {
			continue
		}
		root = append(root, node)
	}
	return adminSpaceEPUBPlan{
		Root:             root,
		Targets:          planner.targets,
		CanonicalTargets: planner.buildCanonicalTargets(),
	}, planner.warnings
}

type adminSpaceEPUBTreePlanner struct {
	opfRoot                    string
	chapterHTMLByCanonicalHref map[string][]byte
	targets                    map[string]adminSpaceEPUBPlannedTarget
	canonicalTargets           map[string]string
	warnings                   []string
	newNodeID                  func() string
	newDocumentID              func() string
	nextNode                   int
	nextDocument               int
}

// planItem 按 EPUB 目录语义递归映射为 PlainDoc 节点。
// 有 href 且有子项时生成 folder + “正文”文档；只有 href 时生成文档；没有 href 时作为纯目录。
func (p *adminSpaceEPUBTreePlanner) planItem(item adminSpaceEPUBNavItem, siblingTitleCounts map[string]int) adminSpaceEPUBPlannedNode {
	title := uniqueAdminSpaceEPUBTitle(resolveAdminSpaceEPUBTitle(item.Title, p.nextDocument+1), siblingTitleCounts)
	normalized, hasHref := normalizeAdminSpaceEPUBHref(p.opfRoot, item.Href)
	if hasHref && normalized.Fragment != "" && !p.fragmentExists(normalized.CanonicalHref, normalized.Fragment) {
		// fragment 必须在预分配阶段降级，否则相同章节的 documentID 和链接映射会在转换阶段变得不稳定。
		p.warnings = append(p.warnings, "目录项 "+title+" 的 fragment "+normalized.Fragment+" 无法定位，已降级为整章文档")
		normalized.Fragment = ""
		normalized.TargetKey = normalized.CanonicalHref
	}

	if len(item.Children) > 0 {
		folder := adminSpaceEPUBPlannedNode{
			Type:   adminSpaceEPUBPlannedNodeTypeFolder,
			Title:  title,
			NodeID: p.nextNodeID(),
		}
		childTitleCounts := make(map[string]int)
		if hasHref {
			bodyTitle := uniqueAdminSpaceEPUBTitle("正文", childTitleCounts)
			folder.Children = append(folder.Children, p.planDocument(bodyTitle, normalized))
		}
		for _, child := range item.Children {
			childNode := p.planItem(child, childTitleCounts)
			if childNode.Title == "" {
				continue
			}
			folder.Children = append(folder.Children, childNode)
		}
		return folder
	}

	if hasHref {
		return p.planDocument(title, normalized)
	}
	return adminSpaceEPUBPlannedNode{
		Type:   adminSpaceEPUBPlannedNodeTypeFolder,
		Title:  title,
		NodeID: p.nextNodeID(),
	}
}

// planDocument 为正文 target 预分配 documentID。
// 多个目录项指向同一 target 时创建“参见”占位文档，避免复制正文导致后续链接目标不一致。
func (p *adminSpaceEPUBTreePlanner) planDocument(title string, normalized adminSpaceEPUBNormalizedHref) adminSpaceEPUBPlannedNode {
	if _, ok := p.targets[normalized.TargetKey]; ok {
		documentID := p.nextDocumentID()
		p.warnings = append(p.warnings, "目录项 "+title+" 重复指向 "+normalized.TargetKey+"，已创建参见占位文档")
		return adminSpaceEPUBPlannedNode{
			Type:               adminSpaceEPUBPlannedNodeTypeDoc,
			Title:              title,
			NodeID:             p.nextNodeID(),
			DocumentID:         documentID,
			CanonicalHref:      normalized.CanonicalHref,
			Fragment:           normalized.Fragment,
			TargetKey:          normalized.TargetKey,
			Reference:          true,
			ReferenceTargetKey: normalized.TargetKey,
		}
	}

	documentID := p.nextDocumentID()
	p.targets[normalized.TargetKey] = adminSpaceEPUBPlannedTarget{
		Title:      title,
		DocumentID: documentID,
	}
	if _, exists := p.canonicalTargets[normalized.CanonicalHref]; !exists {
		p.canonicalTargets[normalized.CanonicalHref] = normalized.TargetKey
	}
	return adminSpaceEPUBPlannedNode{
		Type:          adminSpaceEPUBPlannedNodeTypeDoc,
		Title:         title,
		NodeID:        p.nextNodeID(),
		DocumentID:    documentID,
		CanonicalHref: normalized.CanonicalHref,
		Fragment:      normalized.Fragment,
		TargetKey:     normalized.TargetKey,
	}
}

func (p *adminSpaceEPUBTreePlanner) buildCanonicalTargets() map[string]adminSpaceEPUBPlannedTarget {
	result := make(map[string]adminSpaceEPUBPlannedTarget, len(p.canonicalTargets))
	for canonicalHref, targetKey := range p.canonicalTargets {
		target, exists := p.targets[targetKey]
		if !exists {
			continue
		}
		result[canonicalHref] = target
	}
	return result
}

// fragmentExists 只做 fragment ID 可定位性判断。
// 真正的正文切片、HTML 清洗和 Markdown 转换会在后续 Phase 中基于同一个 canonicalHref 继续处理。
func (p *adminSpaceEPUBTreePlanner) fragmentExists(canonicalHref string, fragment string) bool {
	payload := p.chapterHTMLByCanonicalHref[strings.TrimSpace(canonicalHref)]
	if len(payload) == 0 {
		return false
	}
	root, err := html.Parse(bytes.NewReader(payload))
	if err != nil {
		return false
	}
	var found bool
	var walk func(node *html.Node)
	walk = func(node *html.Node) {
		if node == nil || found {
			return
		}
		if node.Type == html.ElementNode {
			for _, attr := range node.Attr {
				if strings.EqualFold(attr.Key, "id") && attr.Val == fragment {
					found = true
					return
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return found
}

func (p *adminSpaceEPUBTreePlanner) nextNodeID() string {
	if p != nil && p.newNodeID != nil {
		if id := strings.TrimSpace(p.newNodeID()); id != "" {
			return id
		}
	}
	p.nextNode++
	return fmt.Sprintf("node-%03d", p.nextNode)
}

func (p *adminSpaceEPUBTreePlanner) nextDocumentID() string {
	if p != nil && p.newDocumentID != nil {
		if id := strings.TrimSpace(p.newDocumentID()); id != "" {
			return id
		}
	}
	p.nextDocument++
	return fmt.Sprintf("doc-%03d", p.nextDocument)
}

func uniqueAdminSpaceEPUBTitle(title string, siblingTitleCounts map[string]int) string {
	base := strings.TrimSpace(title)
	if base == "" {
		base = "章节"
	}
	count := siblingTitleCounts[base] + 1
	siblingTitleCounts[base] = count
	if count == 1 {
		return base
	}
	return fmt.Sprintf("%s %d", base, count)
}

func resolveAdminSpaceEPUBTitle(title string, fallbackIndex int) string {
	if trimmed := strings.TrimSpace(title); trimmed != "" {
		return trimmed
	}
	if fallbackIndex <= 0 {
		fallbackIndex = 1
	}
	return fmt.Sprintf("章节 %03d", fallbackIndex)
}

func buildAdminSpaceEPUBImportedReaderURL(spaceID string, documentID string) string {
	normalizedSpaceID := strings.TrimSpace(spaceID)
	normalizedDocumentID := strings.TrimSpace(documentID)
	if normalizedSpaceID == "" || normalizedDocumentID == "" {
		return ""
	}
	return "/r/" + normalizedSpaceID + "/" + normalizedDocumentID
}

func buildAdminSpaceEPUBReferenceMarkdown(title string, readerURL string) string {
	return "> 本章节内容见：[" + strings.TrimSpace(title) + "](" + strings.TrimSpace(readerURL) + ")"
}
