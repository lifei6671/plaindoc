package server

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestRouter_ImportDocuments_DecodesGB18030TextIntoMarkdown(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "import-gb18030-owner@example.com")
	spaceID := "01h1importgb18030space000001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	encodedContent, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte("导入内容"))
	if err != nil {
		t.Fatalf("encode gb18030 content failed: %v", err)
	}

	body, contentType := buildWorkspaceImportMultipartBody(t, "", map[string][]byte{
		"导入文档.txt": encodedContent,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/imports", body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected import status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	payload := decodeJSONResultData[struct {
		SuccessCount int `json:"successCount"`
		FailedCount  int `json:"failedCount"`
	}](t, rec.Body.Bytes())
	if payload.SuccessCount != 1 || payload.FailedCount != 0 {
		t.Fatalf("unexpected import counts: %+v body=%s", payload, rec.Body.String())
	}

	var persistedDocument struct {
		Title    string `gorm:"column:title"`
		Format   string `gorm:"column:format"`
		Content  string `gorm:"column:content_md"`
		SpaceID  string `gorm:"column:space_id"`
		Document string `gorm:"column:document_id"`
	}
	if err := database.ORM.Table("documents").
		Select("documents.title", "documents.format", "documents.content_md", "nodes.space_id", "documents.document_id").
		Joins("JOIN nodes ON nodes.node_id = documents.node_id").
		Where("nodes.space_id = ?", spaceID).
		Take(&persistedDocument).Error; err != nil {
		t.Fatalf("query imported document failed: %v", err)
	}
	if persistedDocument.Title != "导入文档" {
		t.Fatalf("expected imported title 导入文档, got %q", persistedDocument.Title)
	}
	if persistedDocument.Format != "markdown" {
		t.Fatalf("expected imported format markdown, got %q", persistedDocument.Format)
	}
	if persistedDocument.Content != "导入内容" {
		t.Fatalf("expected imported content to be utf-8 decoded text, got %q", persistedDocument.Content)
	}
}

func TestRouter_ImportDocuments_AutoExtractsTitlesByFileType(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "import-auto-title-owner@example.com")
	spaceID := "01h1importautotitle000001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	body, contentType := buildWorkspaceImportMultipartBodyWithOptions(t, "", map[string][]byte{
		"title-priority.html": []byte(`<html><head><title>HTML 标题</title></head><body><h1>HTML H1</h1></body></html>`),
		"h1-only.html":        []byte(`<html><body><h1>来自 H1 的标题</h1><p>content</p></body></html>`),
		"fallback.html":       []byte(`<html><body><p>content</p></body></html>`),
		"heading.md":          []byte("# Markdown 一级标题\n\n正文"),
		"fallback.md":         []byte("普通正文\n无一级标题"),
		"plain.txt":           []byte("普通文本"),
	}, map[string]string{
		"autoExtractTitle": "true",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/imports", body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected import status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	expectedTitles := map[string]string{
		"title-priority.html": "HTML 标题",
		"h1-only.html":        "来自 H1 的标题",
		"fallback.html":       "fallback",
		"heading.md":          "Markdown 一级标题",
		"fallback.md":         "fallback",
		"plain.txt":           "plain",
	}
	for sourceName, expectedTitle := range expectedTitles {
		var persistedDocument struct {
			Title string `gorm:"column:title"`
		}
		if err := database.ORM.Table("documents").
			Select("documents.title").
			Joins("JOIN nodes ON nodes.node_id = documents.node_id").
			Where("nodes.space_id = ? AND nodes.title = ?", spaceID, expectedTitle).
			Take(&persistedDocument).Error; err != nil {
			t.Fatalf("query imported document for %s failed: %v", sourceName, err)
		}
		if persistedDocument.Title != expectedTitle {
			t.Fatalf("expected imported title %q for %s, got %q", expectedTitle, sourceName, persistedDocument.Title)
		}
	}
}

func TestRouter_ImportDocuments_ImportsHTMLFromZIPAndLocalImages(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "import-zip-html-owner@example.com")
	spaceID := "01h1importziphtmlspace00001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	zipContent := buildWorkspaceImportZIP(t, map[string][]byte{
		"guide/index.html": []byte(`<html><body><h1>指南</h1><p><img src="./cover.png" alt="封面" /></p></body></html>`),
		"guide/cover.png":  decodeBase64WorkspaceImportAsset(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+XlN8AAAAASUVORK5CYII="),
	})

	body, contentType := buildWorkspaceImportMultipartBody(t, "", map[string][]byte{
		"资料.zip": zipContent,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/imports", body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected import status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	payload := decodeJSONResultData[struct {
		SuccessCount int `json:"successCount"`
		FailedCount  int `json:"failedCount"`
		Items        []struct {
			SourcePath string `json:"sourcePath"`
			Status     string `json:"status"`
		} `json:"items"`
	}](t, rec.Body.Bytes())
	if payload.SuccessCount != 1 || payload.FailedCount != 0 {
		t.Fatalf("unexpected import counts: %+v body=%s", payload, rec.Body.String())
	}
	for _, item := range payload.Items {
		if strings.HasSuffix(item.SourcePath, ".png") && item.Status != "skipped" {
			t.Fatalf("expected local html asset image to be skipped, got %+v", item)
		}
	}

	var guideFolderCount int64
	if err := database.ORM.Table("nodes").
		Where("space_id = ? AND type = ? AND title = ?", spaceID, "folder", "guide").
		Count(&guideFolderCount).Error; err != nil {
		t.Fatalf("count imported guide folder failed: %v", err)
	}
	if guideFolderCount != 0 {
		t.Fatalf("expected guide wrapper folder to be trimmed, got %d", guideFolderCount)
	}

	var persistedDocument struct {
		Title   string `gorm:"column:title"`
		Format  string `gorm:"column:format"`
		Content string `gorm:"column:content_md"`
	}
	if err := database.ORM.Table("documents").
		Select("documents.title", "documents.format", "documents.content_md").
		Joins("JOIN nodes ON nodes.node_id = documents.node_id").
		Where("nodes.space_id = ? AND documents.title = ?", spaceID, "index").
		Take(&persistedDocument).Error; err != nil {
		t.Fatalf("query imported html document failed: %v", err)
	}
	if persistedDocument.Format != "markdown" {
		t.Fatalf("expected imported html format markdown, got %q", persistedDocument.Format)
	}
	if !strings.Contains(persistedDocument.Content, "# 指南") {
		t.Fatalf("expected imported html markdown to contain heading, got %q", persistedDocument.Content)
	}
	if !strings.Contains(persistedDocument.Content, "![封面](/uploads/") {
		t.Fatalf("expected imported html markdown to contain localized image, got %q", persistedDocument.Content)
	}
}

func TestRouter_ImportDocuments_UsesReadmeAsParentAndSkipsEmptyAssetFolders(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "import-readme-owner@example.com")
	spaceID := "01h1importreadmeparent00001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	zipContent := buildWorkspaceImportZIP(t, map[string][]byte{
		"guide/README.md":                   []byte("# 指南首页"),
		"guide/chapter-1.md":                []byte("# 第一章"),
		"guide/community/post.md":           []byte("# 社区帖子"),
		"guide/gocn/README.md":              []byte("# GoCN 首页"),
		"guide/gocn/intro.md":               []byte("# GoCN 介绍"),
		"guide/gocn/examples/example-1.md":  []byte("# 示例一"),
		"guide/assets/cover.png":            decodeBase64WorkspaceImportAsset(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+XlN8AAAAASUVORK5CYII="),
		"guide/community/assets/banner.png": decodeBase64WorkspaceImportAsset(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+XlN8AAAAASUVORK5CYII="),
		"guide/gocn/examples/assets/1.png":  decodeBase64WorkspaceImportAsset(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+XlN8AAAAASUVORK5CYII="),
	})

	body, contentType := buildWorkspaceImportMultipartBody(t, "", map[string][]byte{
		"guide.zip": zipContent,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/imports", body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected import status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var readmeNode struct {
		NodeID string `gorm:"column:node_id"`
		Title  string `gorm:"column:title"`
		Type   string `gorm:"column:type"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.title", "nodes.type").
		Where("space_id = ? AND title = ? AND parent_node_id IS NULL", spaceID, "README").
		Take(&readmeNode).Error; err != nil {
		t.Fatalf("query readme parent node failed: %v", err)
	}
	if readmeNode.Type != "doc" {
		t.Fatalf("expected guide readme node to be imported as doc parent, got %q", readmeNode.Type)
	}

	var childNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
		Title        string  `gorm:"column:title"`
		Type         string  `gorm:"column:type"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id", "nodes.title", "nodes.type").
		Where("space_id = ? AND title = ?", spaceID, "chapter-1").
		Take(&childNode).Error; err != nil {
		t.Fatalf("query readme child node failed: %v", err)
	}
	if childNode.ParentNodeID == nil || strings.TrimSpace(*childNode.ParentNodeID) != readmeNode.NodeID {
		t.Fatalf("expected chapter-1 to be attached under readme doc, got parent=%+v want=%s", childNode.ParentNodeID, readmeNode.NodeID)
	}

	var communityPostNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
		Title        string  `gorm:"column:title"`
		Type         string  `gorm:"column:type"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id", "nodes.title", "nodes.type").
		Where("space_id = ? AND title = ?", spaceID, "post").
		Take(&communityPostNode).Error; err != nil {
		t.Fatalf("query inherited readme child node failed: %v", err)
	}
	var communityFolderNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
		Type         string  `gorm:"column:type"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id", "nodes.type").
		Where("space_id = ? AND title = ? AND type = ?", spaceID, "community", "folder").
		Take(&communityFolderNode).Error; err != nil {
		t.Fatalf("query community folder node failed: %v", err)
	}
	if communityFolderNode.ParentNodeID != nil {
		t.Fatalf("expected community folder to be sibling of guide readme, got parent=%+v", communityFolderNode.ParentNodeID)
	}
	if communityPostNode.ParentNodeID == nil || strings.TrimSpace(*communityPostNode.ParentNodeID) != communityFolderNode.NodeID {
		t.Fatalf("expected community post under community folder, got parent=%+v want=%s", communityPostNode.ParentNodeID, communityFolderNode.NodeID)
	}

	var gocnReadmeNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
		Title        string  `gorm:"column:title"`
		Type         string  `gorm:"column:type"`
	}
	var gocnFolderNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
		Type         string  `gorm:"column:type"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id", "nodes.type").
		Where("space_id = ? AND title = ? AND type = ?", spaceID, "gocn", "folder").
		Take(&gocnFolderNode).Error; err != nil {
		t.Fatalf("query gocn folder node failed: %v", err)
	}
	if gocnFolderNode.ParentNodeID != nil {
		t.Fatalf("expected gocn folder to be sibling of guide readme, got parent=%+v", gocnFolderNode.ParentNodeID)
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id", "nodes.title", "nodes.type").
		Where("space_id = ? AND title = ? AND parent_node_id = ?", spaceID, "README", gocnFolderNode.NodeID).
		Take(&gocnReadmeNode).Error; err != nil {
		t.Fatalf("query nested readme node failed: %v", err)
	}
	if gocnReadmeNode.Type != "doc" {
		t.Fatalf("expected gocn readme to be imported as doc parent, got %q", gocnReadmeNode.Type)
	}
	if gocnReadmeNode.ParentNodeID == nil || strings.TrimSpace(*gocnReadmeNode.ParentNodeID) != gocnFolderNode.NodeID {
		t.Fatalf("expected gocn readme under gocn folder, got parent=%+v want=%s", gocnReadmeNode.ParentNodeID, gocnFolderNode.NodeID)
	}

	var gocnIntroNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
		Title        string  `gorm:"column:title"`
		Type         string  `gorm:"column:type"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id", "nodes.title", "nodes.type").
		Where("space_id = ? AND title = ?", spaceID, "intro").
		Take(&gocnIntroNode).Error; err != nil {
		t.Fatalf("query nested readme child node failed: %v", err)
	}
	if gocnIntroNode.ParentNodeID == nil || strings.TrimSpace(*gocnIntroNode.ParentNodeID) != gocnReadmeNode.NodeID {
		t.Fatalf("expected intro to attach under gocn readme, got parent=%+v want=%s", gocnIntroNode.ParentNodeID, gocnReadmeNode.NodeID)
	}

	var nestedExampleNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
		Title        string  `gorm:"column:title"`
		Type         string  `gorm:"column:type"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id", "nodes.title", "nodes.type").
		Where("space_id = ? AND title = ?", spaceID, "example-1").
		Take(&nestedExampleNode).Error; err != nil {
		t.Fatalf("query nested descendant node failed: %v", err)
	}
	if nestedExampleNode.ParentNodeID == nil || strings.TrimSpace(*nestedExampleNode.ParentNodeID) != gocnReadmeNode.NodeID {
		t.Fatalf("expected example-1 to inherit gocn readme parent, got parent=%+v want=%s", nestedExampleNode.ParentNodeID, gocnReadmeNode.NodeID)
	}

	var guideFolderCount int64
	if err := database.ORM.Table("nodes").
		Where("space_id = ? AND title = ? AND type = ?", spaceID, "guide", "folder").
		Count(&guideFolderCount).Error; err != nil {
		t.Fatalf("count guide folders failed: %v", err)
	}
	if guideFolderCount != 0 {
		t.Fatalf("expected no guide folder when readme acts as parent, got %d", guideFolderCount)
	}

	var assetsFolderCount int64
	if err := database.ORM.Table("nodes").
		Where("space_id = ? AND title = ? AND type = ?", spaceID, "assets", "folder").
		Count(&assetsFolderCount).Error; err != nil {
		t.Fatalf("count empty asset folders failed: %v", err)
	}
	if assetsFolderCount != 0 {
		t.Fatalf("expected no assets folder for non-importable-only subtree, got %d", assetsFolderCount)
	}

	var examplesFolderCount int64
	if err := database.ORM.Table("nodes").
		Where("space_id = ? AND title = ? AND type = ?", spaceID, "examples", "folder").
		Count(&examplesFolderCount).Error; err != nil {
		t.Fatalf("count nested descendant folders failed: %v", err)
	}
	if examplesFolderCount != 0 {
		t.Fatalf("expected no examples folder when nested readme subtree has only one child directory, got %d", examplesFolderCount)
	}
}

func TestRouter_ImportDocuments_LocalizesMarkdownImagesFromRemoteAndZIPPaths(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(decodeBase64WorkspaceImportAsset(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+XlN8AAAAASUVORK5CYII="))
	}))
	defer imageServer.Close()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "import-markdown-image-owner@example.com")
	spaceID := "01h1importmarkdownimage00001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	zipContent := buildWorkspaceImportZIP(t, map[string][]byte{
		"docs/article.md": []byte(strings.Join([]string{
			"# 图文导入",
			"",
			"![远程图](" + imageServer.URL + "/remote.png)",
			"![根目录图](/assets/root.png)",
			"![相对图](./images/local.png)",
		}, "\n")),
		"assets/root.png":       decodeBase64WorkspaceImportAsset(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+XlN8AAAAASUVORK5CYII="),
		"docs/images/local.png": decodeBase64WorkspaceImportAsset(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+XlN8AAAAASUVORK5CYII="),
	})

	body, contentType := buildWorkspaceImportMultipartBody(t, "", map[string][]byte{
		"images.zip": zipContent,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/imports", body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected import status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var persistedDocument struct {
		Title   string `gorm:"column:title"`
		Format  string `gorm:"column:format"`
		Content string `gorm:"column:content_md"`
	}
	if err := database.ORM.Table("documents").
		Select("documents.title", "documents.format", "documents.content_md").
		Joins("JOIN nodes ON nodes.node_id = documents.node_id").
		Where("nodes.space_id = ? AND documents.title = ?", spaceID, "article").
		Take(&persistedDocument).Error; err != nil {
		t.Fatalf("query imported markdown document failed: %v", err)
	}
	if persistedDocument.Format != "markdown" {
		t.Fatalf("expected imported markdown format markdown, got %q", persistedDocument.Format)
	}
	if strings.Contains(persistedDocument.Content, imageServer.URL) {
		t.Fatalf("expected remote image url to be localized, got %q", persistedDocument.Content)
	}
	if strings.Contains(persistedDocument.Content, "/assets/root.png") {
		t.Fatalf("expected root absolute image path to be localized, got %q", persistedDocument.Content)
	}
	if strings.Contains(persistedDocument.Content, "./images/local.png") {
		t.Fatalf("expected relative image path to be localized, got %q", persistedDocument.Content)
	}
	if strings.Count(persistedDocument.Content, "(/uploads/") < 3 {
		t.Fatalf("expected all markdown images to be localized into uploads urls, got %q", persistedDocument.Content)
	}
}

func TestRouter_ImportDocuments_RootReadmeOwnsDescendantsUntilNestedReadmeOverrides(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "import-root-readme-owner@example.com")
	spaceID := "01h1importrootreadme0000001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	zipContent := buildWorkspaceImportZIP(t, map[string][]byte{
		"README.md":             []byte("# 根说明"),
		"docs/intro.md":         []byte("# 介绍"),
		"gocn/README.md":        []byte("# GoCN 根文档"),
		"gocn/post.md":          []byte("# 帖子"),
		"gocn/examples/a.md":    []byte("# 示例 A"),
		"assets/ignored.png":    decodeBase64WorkspaceImportAsset(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+XlN8AAAAASUVORK5CYII="),
		"docs/assets/cover.png": decodeBase64WorkspaceImportAsset(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+XlN8AAAAASUVORK5CYII="),
	})

	body, contentType := buildWorkspaceImportMultipartBody(t, "", map[string][]byte{
		"root-readme.zip": zipContent,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/imports", body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected import status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var rootReadmeNode struct {
		NodeID string `gorm:"column:node_id"`
		Title  string `gorm:"column:title"`
		Type   string `gorm:"column:type"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.title", "nodes.type").
		Where("space_id = ? AND title = ? AND parent_node_id IS NULL", spaceID, "README").
		Take(&rootReadmeNode).Error; err != nil {
		t.Fatalf("query root readme node failed: %v", err)
	}
	if rootReadmeNode.Type != "doc" {
		t.Fatalf("expected root readme to be imported as doc parent, got %q", rootReadmeNode.Type)
	}

	assertParentNode := func(title string, expectedParentNodeID string) {
		t.Helper()
		var node struct {
			ParentNodeID *string `gorm:"column:parent_node_id"`
		}
		if err := database.ORM.Table("nodes").
			Select("nodes.parent_node_id").
			Where("space_id = ? AND title = ?", spaceID, title).
			Take(&node).Error; err != nil {
			t.Fatalf("query node %s failed: %v", title, err)
		}
		if node.ParentNodeID == nil || strings.TrimSpace(*node.ParentNodeID) != expectedParentNodeID {
			t.Fatalf("expected %s parent=%s, got %+v", title, expectedParentNodeID, node.ParentNodeID)
		}
	}

	var gocnReadmeNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
		Type         string  `gorm:"column:type"`
	}
	var docsFolderNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
		Type         string  `gorm:"column:type"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id", "nodes.type").
		Where("space_id = ? AND title = ? AND type = ?", spaceID, "docs", "folder").
		Take(&docsFolderNode).Error; err != nil {
		t.Fatalf("query docs folder failed: %v", err)
	}
	if docsFolderNode.ParentNodeID != nil {
		t.Fatalf("expected docs folder to be sibling of root readme, got %+v", docsFolderNode.ParentNodeID)
	}
	assertParentNode("intro", docsFolderNode.NodeID)

	var gocnFolderNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
		Type         string  `gorm:"column:type"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id", "nodes.type").
		Where("space_id = ? AND title = ? AND type = ?", spaceID, "gocn", "folder").
		Take(&gocnFolderNode).Error; err != nil {
		t.Fatalf("query gocn folder failed: %v", err)
	}
	if gocnFolderNode.ParentNodeID != nil {
		t.Fatalf("expected gocn folder to be sibling of root readme, got %+v", gocnFolderNode.ParentNodeID)
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id", "nodes.type").
		Where("space_id = ? AND title = ? AND parent_node_id = ?", spaceID, "README", gocnFolderNode.NodeID).
		Take(&gocnReadmeNode).Error; err != nil {
		t.Fatalf("query gocn readme node failed: %v", err)
	}
	if gocnReadmeNode.Type != "doc" {
		t.Fatalf("expected gocn readme to be imported as doc parent, got %q", gocnReadmeNode.Type)
	}
	if gocnReadmeNode.ParentNodeID == nil || strings.TrimSpace(*gocnReadmeNode.ParentNodeID) != gocnFolderNode.NodeID {
		t.Fatalf("expected gocn readme to attach under gocn folder, got %+v", gocnReadmeNode.ParentNodeID)
	}

	assertParentNode("post", gocnReadmeNode.NodeID)
	assertParentNode("a", gocnReadmeNode.NodeID)

	for _, folderTitle := range []string{"examples", "assets"} {
		var folderCount int64
		if err := database.ORM.Table("nodes").
			Where("space_id = ? AND title = ? AND type = ?", spaceID, folderTitle, "folder").
			Count(&folderCount).Error; err != nil {
			t.Fatalf("count folder %s failed: %v", folderTitle, err)
		}
		if folderCount != 0 {
			t.Fatalf("expected no folder node for %s when readme acts as parent, got %d", folderTitle, folderCount)
		}
	}
}

func TestRouter_ImportDocuments_NestedReadmeKeepsSameDirectoryFilesAsChildren(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "import-nested-readme-owner@example.com")
	spaceID := "01h1importnestedreadme000001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	zipContent := buildWorkspaceImportZIP(t, map[string][]byte{
		"README.md":         []byte("# 根 README"),
		"READMD/README.md":  []byte("# 二级 README"),
		"READMD/read-1.md":  []byte("# read-1"),
		"READMD/read-10.md": []byte("# read-10"),
		"READMD/read-11.md": []byte("# read-11"),
		"READMD/read-12.md": []byte("# read-12"),
	})

	body, contentType := buildWorkspaceImportMultipartBody(t, "", map[string][]byte{
		"nested-readme.zip": zipContent,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/imports", body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected import status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var rootReadmeNode struct {
		NodeID string `gorm:"column:node_id"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id").
		Where("space_id = ? AND title = ? AND parent_node_id IS NULL", spaceID, "README").
		Take(&rootReadmeNode).Error; err != nil {
		t.Fatalf("query root readme node failed: %v", err)
	}

	var nestedReadmeNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id").
		Where("space_id = ? AND title = ? AND parent_node_id = ?", spaceID, "README", rootReadmeNode.NodeID).
		Take(&nestedReadmeNode).Error; err != nil {
		t.Fatalf("query nested readme node failed: %v", err)
	}
	if nestedReadmeNode.ParentNodeID == nil || strings.TrimSpace(*nestedReadmeNode.ParentNodeID) != rootReadmeNode.NodeID {
		t.Fatalf("expected nested readme parent=%s, got %+v", rootReadmeNode.NodeID, nestedReadmeNode.ParentNodeID)
	}

	for _, title := range []string{"read-1", "read-10", "read-11", "read-12"} {
		var node struct {
			ParentNodeID *string `gorm:"column:parent_node_id"`
		}
		if err := database.ORM.Table("nodes").
			Select("nodes.parent_node_id").
			Where("space_id = ? AND title = ?", spaceID, title).
			Take(&node).Error; err != nil {
			t.Fatalf("query nested child %s failed: %v", title, err)
		}
		if node.ParentNodeID == nil || strings.TrimSpace(*node.ParentNodeID) != nestedReadmeNode.NodeID {
			t.Fatalf("expected %s parent=%s, got %+v", title, nestedReadmeNode.NodeID, node.ParentNodeID)
		}
	}
}

func TestRouter_ImportDocuments_NestedReadmeCollapsesSingleGrandchildDirectory(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "import-nested-readme-grandchild-owner@example.com")
	spaceID := "01h1importnestedreadmegrand001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	zipContent := buildWorkspaceImportZIP(t, map[string][]byte{
		"README.md":           []byte("# 根 README"),
		"gocn/README.md":      []byte("# GoCN README"),
		"gocn/2018/read-1.md": []byte("# read-1"),
		"gocn/2018/read-2.md": []byte("# read-2"),
	})

	body, contentType := buildWorkspaceImportMultipartBody(t, "", map[string][]byte{
		"nested-readme-grandchild.zip": zipContent,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/imports", body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected import status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var rootReadmeNode struct {
		NodeID string `gorm:"column:node_id"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id").
		Where("space_id = ? AND title = ? AND parent_node_id IS NULL", spaceID, "README").
		Take(&rootReadmeNode).Error; err != nil {
		t.Fatalf("query root readme node failed: %v", err)
	}

	var nestedReadmeNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id").
		Where("space_id = ? AND title = ? AND parent_node_id = ?", spaceID, "README", rootReadmeNode.NodeID).
		Take(&nestedReadmeNode).Error; err != nil {
		t.Fatalf("query nested readme node failed: %v", err)
	}
	if nestedReadmeNode.ParentNodeID == nil || strings.TrimSpace(*nestedReadmeNode.ParentNodeID) != rootReadmeNode.NodeID {
		t.Fatalf("expected nested readme parent=%s, got %+v", rootReadmeNode.NodeID, nestedReadmeNode.ParentNodeID)
	}

	for _, title := range []string{"read-1", "read-2"} {
		var node struct {
			ParentNodeID *string `gorm:"column:parent_node_id"`
		}
		if err := database.ORM.Table("nodes").
			Select("nodes.parent_node_id").
			Where("space_id = ? AND title = ?", spaceID, title).
			Take(&node).Error; err != nil {
			t.Fatalf("query nested grandchild %s failed: %v", title, err)
		}
		if node.ParentNodeID == nil || strings.TrimSpace(*node.ParentNodeID) != nestedReadmeNode.NodeID {
			t.Fatalf("expected %s parent=%s, got %+v", title, nestedReadmeNode.NodeID, node.ParentNodeID)
		}
	}

	var yearFolderCount int64
	if err := database.ORM.Table("nodes").
		Where("space_id = ? AND title = ? AND type = ?", spaceID, "2018", "folder").
		Count(&yearFolderCount).Error; err != nil {
		t.Fatalf("count 2018 folder failed: %v", err)
	}
	if yearFolderCount != 0 {
		t.Fatalf("expected single grandchild directory 2018 to collapse under nested readme, got %d", yearFolderCount)
	}
}

func TestRouter_ImportDocuments_ReadmdAliasBehavesLikeNestedReadme(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "import-readmd-alias-owner@example.com")
	spaceID := "01h1importreadmdalias000001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	zipContent := buildWorkspaceImportZIP(t, map[string][]byte{
		"README.md":                 []byte("# 根 README"),
		"gocn/READMD.md":            []byte("# GoCN README Alias"),
		"gocn/2018-03/read-1.md":    []byte("# read-1"),
		"gocn/2018-03/read-10.md":   []byte("# read-10"),
		"gocn/2018-03/read-11.md":   []byte("# read-11"),
		"gocn/2018-03/images/a.png": decodeBase64WorkspaceImportAsset(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+XlN8AAAAASUVORK5CYII="),
	})

	body, contentType := buildWorkspaceImportMultipartBody(t, "", map[string][]byte{
		"readmd-alias.zip": zipContent,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/imports", body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected import status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	payload := decodeJSONResultData[struct {
		CreatedNodes []struct {
			NodeID   string  `json:"nodeId"`
			ParentID *string `json:"parentId"`
			Title    string  `json:"title"`
			Type     string  `json:"type"`
		} `json:"createdNodes"`
	}](t, rec.Body.Bytes())

	var rootReadmeNode struct {
		NodeID string `gorm:"column:node_id"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id").
		Where("space_id = ? AND title = ? AND parent_node_id IS NULL", spaceID, "README").
		Take(&rootReadmeNode).Error; err != nil {
		t.Fatalf("query root readme node failed: %v", err)
	}

	var nestedReadmeNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id").
		Where("space_id = ? AND title = ? AND parent_node_id = ?", spaceID, "README", rootReadmeNode.NodeID).
		Take(&nestedReadmeNode).Error; err != nil {
		t.Fatalf("query nested alias readme node failed: %v created=%+v", err, payload.CreatedNodes)
	}
	if nestedReadmeNode.ParentNodeID == nil || strings.TrimSpace(*nestedReadmeNode.ParentNodeID) != rootReadmeNode.NodeID {
		t.Fatalf("expected nested alias readme parent=%s, got %+v", rootReadmeNode.NodeID, nestedReadmeNode.ParentNodeID)
	}

	for _, title := range []string{"read-1", "read-10", "read-11"} {
		var node struct {
			ParentNodeID *string `gorm:"column:parent_node_id"`
		}
		if err := database.ORM.Table("nodes").
			Select("nodes.parent_node_id").
			Where("space_id = ? AND title = ?", spaceID, title).
			Take(&node).Error; err != nil {
			t.Fatalf("query alias nested child %s failed: %v", title, err)
		}
		if node.ParentNodeID == nil || strings.TrimSpace(*node.ParentNodeID) != nestedReadmeNode.NodeID {
			t.Fatalf("expected %s parent=%s, got %+v", title, nestedReadmeNode.NodeID, node.ParentNodeID)
		}
	}

	var preservedFolderCount int64
	if err := database.ORM.Table("nodes").
		Where("space_id = ? AND title IN ? AND type = ?", spaceID, []string{"gocn", "2018-03", "images"}, "folder").
		Count(&preservedFolderCount).Error; err != nil {
		t.Fatalf("count collapsed alias folders failed: %v", err)
	}
	if preservedFolderCount != 0 {
		t.Fatalf("expected readmd alias subtree single-chain folders to collapse, got %d", preservedFolderCount)
	}
}

func TestRouter_ImportDocuments_ReadmePreservesSiblingDirectoriesWhenMultipleBranchesExist(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "import-readme-branch-owner@example.com")
	spaceID := "01h1importreadmebranch000001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	zipContent := buildWorkspaceImportZIP(t, map[string][]byte{
		"README.md":        []byte("# 根 README"),
		"gocn/README.md":   []byte("# GoCN README"),
		"gocn/1.md":        []byte("# 文档 1"),
		"gocn/2.md":        []byte("# 文档 2"),
		"gocn1/1.md":       []byte("# 分支 1"),
		"gocn1/2.md":       []byte("# 分支 2"),
		"gocn1/assets.png": decodeBase64WorkspaceImportAsset(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+XlN8AAAAASUVORK5CYII="),
	})

	body, contentType := buildWorkspaceImportMultipartBody(t, "", map[string][]byte{
		"readme-branches.zip": zipContent,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/imports", body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected import status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var rootReadmeNode struct {
		NodeID string `gorm:"column:node_id"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id").
		Where("space_id = ? AND title = ? AND parent_node_id IS NULL", spaceID, "README").
		Take(&rootReadmeNode).Error; err != nil {
		t.Fatalf("query root readme failed: %v", err)
	}

	var gocnFolderNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id").
		Where("space_id = ? AND title = ? AND type = ?", spaceID, "gocn", "folder").
		Take(&gocnFolderNode).Error; err != nil {
		t.Fatalf("query gocn folder failed: %v", err)
	}
	if gocnFolderNode.ParentNodeID != nil {
		t.Fatalf("expected gocn folder to be sibling of root readme, got %+v", gocnFolderNode.ParentNodeID)
	}

	var gocn1FolderNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id").
		Where("space_id = ? AND title = ? AND type = ?", spaceID, "gocn1", "folder").
		Take(&gocn1FolderNode).Error; err != nil {
		t.Fatalf("query gocn1 folder failed: %v", err)
	}
	if gocn1FolderNode.ParentNodeID != nil {
		t.Fatalf("expected gocn1 folder to be sibling of root readme, got %+v", gocn1FolderNode.ParentNodeID)
	}

	var nestedReadmeNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id").
		Where("space_id = ? AND title = ? AND parent_node_id = ?", spaceID, "README", gocnFolderNode.NodeID).
		Take(&nestedReadmeNode).Error; err != nil {
		t.Fatalf("query nested gocn readme failed: %v", err)
	}
	if nestedReadmeNode.ParentNodeID == nil || strings.TrimSpace(*nestedReadmeNode.ParentNodeID) != gocnFolderNode.NodeID {
		t.Fatalf("expected nested gocn readme parent=%s, got %+v", gocnFolderNode.NodeID, nestedReadmeNode.ParentNodeID)
	}

	for _, title := range []string{"1", "2"} {
		var node struct {
			ParentNodeID *string `gorm:"column:parent_node_id"`
		}
		if err := database.ORM.Table("nodes").
			Select("nodes.parent_node_id").
			Where("space_id = ? AND title = ? AND parent_node_id = ?", spaceID, title, nestedReadmeNode.NodeID).
			Take(&node).Error; err != nil {
			t.Fatalf("query gocn child %s failed: %v", title, err)
		}
	}

	for _, title := range []string{"1", "2"} {
		var node struct {
			ParentNodeID *string `gorm:"column:parent_node_id"`
		}
		if err := database.ORM.Table("nodes").
			Select("nodes.parent_node_id").
			Where("space_id = ? AND title = ? AND parent_node_id = ?", spaceID, title, gocn1FolderNode.NodeID).
			Take(&node).Error; err != nil {
			t.Fatalf("query gocn1 child %s failed: %v", title, err)
		}
	}
}

func TestRouter_ImportDocuments_ReadmePeersWithMultipleChildDirsAndPreservesFileIdentifiers(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "import-readme-peer-ident-owner@example.com")
	spaceID := "01h1importreadmepeerident001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	zipContent := buildWorkspaceImportZIP(t, map[string][]byte{
		"README.md":               []byte("# 根 README"),
		"gocn/README.md":          []byte("# GoCN README"),
		"gocn/2018-04/read-1.md":  []byte("# 2018-04 read-1"),
		"gocn/2018-03/read-1.md":  []byte("# 2018-03 read-1"),
		"gocn/2018-03/read-10.md": []byte("# 2018-03 read-10"),
	})

	body, contentType := buildWorkspaceImportMultipartBody(t, "", map[string][]byte{
		"readme-peer-ident.zip": zipContent,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/imports", body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected import status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var rootReadmeNode struct {
		NodeID     string  `gorm:"column:node_id"`
		ReaderSlug *string `gorm:"column:reader_slug"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.reader_slug").
		Where("space_id = ? AND title = ? AND parent_node_id IS NULL", spaceID, "README").
		Take(&rootReadmeNode).Error; err != nil {
		t.Fatalf("query root readme failed: %v", err)
	}
	if rootReadmeNode.ReaderSlug == nil || strings.TrimSpace(*rootReadmeNode.ReaderSlug) != "gocn" {
		t.Fatalf("expected root readme identifier gocn, got %+v", rootReadmeNode.ReaderSlug)
	}

	var nestedReadmeNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
		ReaderSlug   *string `gorm:"column:reader_slug"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id", "nodes.reader_slug").
		Where("space_id = ? AND title = ? AND parent_node_id = ?", spaceID, "README", rootReadmeNode.NodeID).
		Take(&nestedReadmeNode).Error; err != nil {
		t.Fatalf("query nested readme failed: %v", err)
	}
	if nestedReadmeNode.ParentNodeID == nil || strings.TrimSpace(*nestedReadmeNode.ParentNodeID) != rootReadmeNode.NodeID {
		t.Fatalf("expected nested readme sibling under root readme, got %+v", nestedReadmeNode.ParentNodeID)
	}
	if nestedReadmeNode.ReaderSlug == nil || strings.TrimSpace(*nestedReadmeNode.ReaderSlug) == "" {
		t.Fatalf("expected nested readme random identifier, got %+v", nestedReadmeNode.ReaderSlug)
	}
	if identifier := strings.TrimSpace(*nestedReadmeNode.ReaderSlug); identifier == "2018-03" || identifier == "2018-04" {
		t.Fatalf("expected nested readme identifier to be randomized when multiple child dirs exist, got %q", identifier)
	}

	var folder201803 struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id").
		Where("space_id = ? AND title = ? AND type = ?", spaceID, "2018-03", "folder").
		Take(&folder201803).Error; err != nil {
		t.Fatalf("query 2018-03 folder failed: %v", err)
	}
	if folder201803.ParentNodeID == nil || strings.TrimSpace(*folder201803.ParentNodeID) != rootReadmeNode.NodeID {
		t.Fatalf("expected 2018-03 folder sibling under root readme, got %+v", folder201803.ParentNodeID)
	}

	var folder201804 struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id").
		Where("space_id = ? AND title = ? AND type = ?", spaceID, "2018-04", "folder").
		Take(&folder201804).Error; err != nil {
		t.Fatalf("query 2018-04 folder failed: %v", err)
	}
	if folder201804.ParentNodeID == nil || strings.TrimSpace(*folder201804.ParentNodeID) != rootReadmeNode.NodeID {
		t.Fatalf("expected 2018-04 folder sibling under root readme, got %+v", folder201804.ParentNodeID)
	}

	var read201803 struct {
		ReaderSlug *string `gorm:"column:reader_slug"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.reader_slug").
		Where("space_id = ? AND title = ? AND parent_node_id = ?", spaceID, "read-1", folder201803.NodeID).
		Take(&read201803).Error; err != nil {
		t.Fatalf("query 2018-03 read-1 failed: %v", err)
	}
	if read201803.ReaderSlug == nil || strings.TrimSpace(*read201803.ReaderSlug) != "read-1.md" {
		t.Fatalf("expected 2018-03 read-1 identifier read-1.md, got %+v", read201803.ReaderSlug)
	}

	var read201804 struct {
		ReaderSlug *string `gorm:"column:reader_slug"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.reader_slug").
		Where("space_id = ? AND title = ? AND parent_node_id = ?", spaceID, "read-1", folder201804.NodeID).
		Take(&read201804).Error; err != nil {
		t.Fatalf("query 2018-04 read-1 failed: %v", err)
	}
	if read201804.ReaderSlug == nil || strings.TrimSpace(*read201804.ReaderSlug) == "" {
		t.Fatalf("expected 2018-04 read-1 randomized identifier, got %+v", read201804.ReaderSlug)
	}
	if strings.TrimSpace(*read201804.ReaderSlug) == "read-1.md" {
		t.Fatalf("expected duplicate read-1 identifier to be randomized, got %+v", read201804.ReaderSlug)
	}

	var read10 struct {
		ReaderSlug *string `gorm:"column:reader_slug"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.reader_slug").
		Where("space_id = ? AND title = ? AND parent_node_id = ?", spaceID, "read-10", folder201803.NodeID).
		Take(&read10).Error; err != nil {
		t.Fatalf("query 2018-03 read-10 failed: %v", err)
	}
	if read10.ReaderSlug == nil || strings.TrimSpace(*read10.ReaderSlug) != "read-10.md" {
		t.Fatalf("expected read-10 identifier read-10.md, got %+v", read10.ReaderSlug)
	}
}

func TestRouter_ImportDocuments_TrimsLeadingEmptyZipDirectoriesToFirstImportableRoot(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "import-zip-root-trim-owner@example.com")
	spaceID := "01h1importziproottrim000001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	zipContent := buildWorkspaceImportZIP(t, map[string][]byte{
		"a/b/c/README.md": []byte("# C README"),
		"a/b/c/1.md":      []byte("# 文档 1"),
		"a/b/d/2.md":      []byte("# 文档 2"),
	})

	body, contentType := buildWorkspaceImportMultipartBody(t, "", map[string][]byte{
		"trim-root.zip": zipContent,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/imports", body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected import status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var aFolderCount int64
	if err := database.ORM.Table("nodes").
		Where("space_id = ? AND title = ? AND type = ?", spaceID, "a", "folder").
		Count(&aFolderCount).Error; err != nil {
		t.Fatalf("count a folder failed: %v", err)
	}
	if aFolderCount != 0 {
		t.Fatalf("expected wrapper directory a to be trimmed, got %d", aFolderCount)
	}

	var bFolderCount int64
	if err := database.ORM.Table("nodes").
		Where("space_id = ? AND title = ? AND type = ?", spaceID, "b", "folder").
		Count(&bFolderCount).Error; err != nil {
		t.Fatalf("count b folder failed: %v", err)
	}
	if bFolderCount != 0 {
		t.Fatalf("expected wrapper directory b to be trimmed, got %d", bFolderCount)
	}

	var cFolderNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id").
		Where("space_id = ? AND title = ? AND type = ? AND parent_node_id IS NULL", spaceID, "c", "folder").
		Take(&cFolderNode).Error; err != nil {
		t.Fatalf("query c folder failed: %v", err)
	}

	var dFolderNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id").
		Where("space_id = ? AND title = ? AND type = ? AND parent_node_id IS NULL", spaceID, "d", "folder").
		Take(&dFolderNode).Error; err != nil {
		t.Fatalf("query d folder failed: %v", err)
	}

	var nestedReadmeNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id").
		Where("space_id = ? AND title = ? AND parent_node_id = ?", spaceID, "README", cFolderNode.NodeID).
		Take(&nestedReadmeNode).Error; err != nil {
		t.Fatalf("query nested readme in c failed: %v", err)
	}
	if nestedReadmeNode.ParentNodeID == nil || strings.TrimSpace(*nestedReadmeNode.ParentNodeID) != cFolderNode.NodeID {
		t.Fatalf("expected nested readme under c folder, got %+v", nestedReadmeNode.ParentNodeID)
	}
}

func TestRouter_ImportDocuments_UsesTargetDocumentAsParentWhenImportingFromDocumentMenu(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "import-target-doc-parent-owner@example.com")
	spaceID := "01h1importtargetdocparent001"
	targetNodeID := "01h1importtargetdocnode00001"
	targetDocumentID := "01h1importtargetdocument0001"
	seedSpaceAndDocumentForAccess(t, database, ownerUserID, spaceID, targetNodeID, targetDocumentID, "member", "member")

	zipContent := buildWorkspaceImportZIP(t, map[string][]byte{
		"README.md":          []byte("# 根 README"),
		"folder/readme-1.md": []byte("# readme-1"),
	})

	body, contentType := buildWorkspaceImportMultipartBody(t, targetNodeID, map[string][]byte{
		"document-menu.zip": zipContent,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/imports", body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected import status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var rootReadmeNode struct {
		NodeID       string  `gorm:"column:node_id"`
		ParentNodeID *string `gorm:"column:parent_node_id"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.node_id", "nodes.parent_node_id").
		Where("space_id = ? AND title = ?", spaceID, "README").
		Take(&rootReadmeNode).Error; err != nil {
		t.Fatalf("query imported root readme node failed: %v", err)
	}
	if rootReadmeNode.ParentNodeID == nil || strings.TrimSpace(*rootReadmeNode.ParentNodeID) != targetNodeID {
		t.Fatalf("expected imported root readme parent=%s, got %+v", targetNodeID, rootReadmeNode.ParentNodeID)
	}

	var nestedDocumentNode struct {
		ParentNodeID *string `gorm:"column:parent_node_id"`
	}
	if err := database.ORM.Table("nodes").
		Select("nodes.parent_node_id").
		Where("space_id = ? AND title = ?", spaceID, "readme-1").
		Take(&nestedDocumentNode).Error; err != nil {
		t.Fatalf("query imported nested document failed: %v", err)
	}
	if nestedDocumentNode.ParentNodeID == nil || strings.TrimSpace(*nestedDocumentNode.ParentNodeID) != rootReadmeNode.NodeID {
		t.Fatalf("expected imported nested document parent=%s, got %+v", rootReadmeNode.NodeID, nestedDocumentNode.ParentNodeID)
	}
}

func TestRouter_ImportDocuments_PreservesOfficeFormatsWhenOnlyOfficeEnabled(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "import-office-enabled-owner@example.com")
	spaceID := "01h1importofficeenabled00001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")
	seedOnlyOfficeEnabledConfig(t, database)

	body, contentType := buildWorkspaceImportMultipartBody(t, "", map[string][]byte{
		"季度报告.docx": buildMinimalImportDOCX(t, "导入 Word 正文"),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/imports", body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected import status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var persistedDocument struct {
		Title          string  `gorm:"column:title"`
		Format         string  `gorm:"column:format"`
		SourceBlobID   *string `gorm:"column:source_blob_id"`
		SourceFileName *string `gorm:"column:source_file_name"`
	}
	if err := database.ORM.Table("documents").
		Select("documents.title", "documents.format", "documents.source_blob_id", "documents.source_file_name").
		Joins("JOIN nodes ON nodes.node_id = documents.node_id").
		Where("nodes.space_id = ?", spaceID).
		Take(&persistedDocument).Error; err != nil {
		t.Fatalf("query imported office document failed: %v", err)
	}
	if persistedDocument.Format != "docx" {
		t.Fatalf("expected office import format docx, got %q", persistedDocument.Format)
	}
	if persistedDocument.SourceBlobID == nil || strings.TrimSpace(*persistedDocument.SourceBlobID) == "" {
		t.Fatalf("expected source blob id for imported office document")
	}
	if persistedDocument.SourceFileName == nil || *persistedDocument.SourceFileName != "季度报告.docx" {
		t.Fatalf("expected source file name 季度报告.docx, got %+v", persistedDocument.SourceFileName)
	}
}

func TestRouter_ImportDocuments_ConvertsXLSXToMarkdownWhenOnlyOfficeDisabled(t *testing.T) {
	database, serve := setupAuthTestRouter(t)
	defer func() {
		_ = database.Close()
	}()

	ownerUserID, _, ownerToken := registerAccessUser(t, serve, "import-xlsx-markdown-owner@example.com")
	spaceID := "01h1importxlsxmarkdown00001"
	seedSpaceForWorkspaceCreateNode(t, database, ownerUserID, spaceID, "member")

	workbook := excelize.NewFile()
	if err := workbook.SetCellValue("Sheet1", "A1", "预算"); err != nil {
		t.Fatalf("set xlsx A1 failed: %v", err)
	}
	if err := workbook.SetCellValue("Sheet1", "B1", 1280); err != nil {
		t.Fatalf("set xlsx B1 failed: %v", err)
	}
	var buffer bytes.Buffer
	if err := workbook.Write(&buffer); err != nil {
		t.Fatalf("write xlsx workbook failed: %v", err)
	}
	if err := workbook.Close(); err != nil {
		t.Fatalf("close xlsx workbook failed: %v", err)
	}

	body, contentType := buildWorkspaceImportMultipartBody(t, "", map[string][]byte{
		"预算表.xlsx": buffer.Bytes(),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/spaces/"+spaceID+"/imports", body)
	req.Header.Set("Authorization", "Bearer "+ownerToken)
	req.Header.Set("Content-Type", contentType)
	rec := serve(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected import status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var persistedDocument struct {
		Format  string `gorm:"column:format"`
		Content string `gorm:"column:content_md"`
	}
	if err := database.ORM.Table("documents").
		Select("documents.format", "documents.content_md").
		Joins("JOIN nodes ON nodes.node_id = documents.node_id").
		Where("nodes.space_id = ?", spaceID).
		Take(&persistedDocument).Error; err != nil {
		t.Fatalf("query imported xlsx markdown document failed: %v", err)
	}
	if persistedDocument.Format != "markdown" {
		t.Fatalf("expected imported xlsx format markdown, got %q", persistedDocument.Format)
	}
	if !strings.Contains(persistedDocument.Content, "预算") {
		t.Fatalf("expected imported xlsx markdown to contain worksheet text, got %q", persistedDocument.Content)
	}
}

func buildWorkspaceImportMultipartBody(
	t *testing.T,
	targetNodeID string,
	files map[string][]byte,
) (*bytes.Buffer, string) {
	return buildWorkspaceImportMultipartBodyWithOptions(t, targetNodeID, files, map[string]string{})
}

func buildWorkspaceImportMultipartBodyWithOptions(
	t *testing.T,
	targetNodeID string,
	files map[string][]byte,
	fields map[string]string,
) (*bytes.Buffer, string) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if strings.TrimSpace(targetNodeID) != "" {
		if err := writer.WriteField("targetNodeId", targetNodeID); err != nil {
			t.Fatalf("write targetNodeId field failed: %v", err)
		}
	}
	for fieldName, fieldValue := range fields {
		if err := writer.WriteField(fieldName, fieldValue); err != nil {
			t.Fatalf("write field %s failed: %v", fieldName, err)
		}
	}
	for fileName, content := range files {
		part, err := writer.CreateFormFile("files", fileName)
		if err != nil {
			t.Fatalf("create multipart file %s failed: %v", fileName, err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatalf("write multipart file %s failed: %v", fileName, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer failed: %v", err)
	}
	return &body, writer.FormDataContentType()
}

func buildWorkspaceImportZIP(t *testing.T, files map[string][]byte) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for filePath, content := range files {
		entry, err := writer.Create(filePath)
		if err != nil {
			t.Fatalf("create zip entry %s failed: %v", filePath, err)
		}
		if _, err := io.Copy(entry, bytes.NewReader(content)); err != nil {
			t.Fatalf("write zip entry %s failed: %v", filePath, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer failed: %v", err)
	}
	return buffer.Bytes()
}

func decodeBase64WorkspaceImportAsset(t *testing.T, rawBase64 string) []byte {
	t.Helper()

	content, err := base64.StdEncoding.DecodeString(strings.TrimSpace(rawBase64))
	if err != nil {
		t.Fatalf("decode base64 asset failed: %v", err)
	}
	return content
}

func buildMinimalImportDOCX(t *testing.T, text string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>` + text + `</w:t></w:r></w:p>
  </w:body>
</w:document>`,
	}

	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create docx entry %s failed: %v", name, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("write docx entry %s failed: %v", name, err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close docx archive failed: %v", err)
	}
	return buffer.Bytes()
}
