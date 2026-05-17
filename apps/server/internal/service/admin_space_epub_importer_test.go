package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func TestPlanAdminSpaceEPUBImportTree_BuildsFoldersDocsFragmentsAndReferences(t *testing.T) {
	t.Parallel()

	items := []adminSpaceEPUBNavItem{
		{
			Title: "Part 1",
			Children: []adminSpaceEPUBNavItem{
				{Title: "Chapter 1", Href: "chapters/chapter1.xhtml#intro"},
				{Title: "Chapter 1", Href: "chapters/chapter1.xhtml#intro"},
			},
		},
		{
			Title: "Part 2",
			Href:  "chapters/chapter2.xhtml",
			Children: []adminSpaceEPUBNavItem{
				{Title: "正文", Href: "chapters/chapter2.xhtml#body"},
			},
		},
		{Title: "Missing Fragment", Href: "chapters/chapter3.xhtml#missing"},
	}

	plan, warnings := planAdminSpaceEPUBImportTree(adminSpaceEPUBPlanInput{
		OPFRoot: "OPS",
		Items:   items,
		ChapterHTMLByCanonicalHref: map[string][]byte{
			"OPS/chapters/chapter1.xhtml": []byte(`<html><body><h1 id="intro">Intro</h1></body></html>`),
			"OPS/chapters/chapter2.xhtml": []byte(`<html><body><h1 id="body">Body</h1></body></html>`),
			"OPS/chapters/chapter3.xhtml": []byte(`<html><body><h1>Missing</h1></body></html>`),
		},
	})

	if len(plan.Root) != 3 {
		t.Fatalf("expected three root nodes, got %#v", plan.Root)
	}
	part1 := plan.Root[0]
	if part1.Type != adminSpaceEPUBPlannedNodeTypeFolder || len(part1.Children) != 2 {
		t.Fatalf("expected Part 1 folder with two children, got %#v", part1)
	}
	chapter := part1.Children[0]
	if chapter.Type != adminSpaceEPUBPlannedNodeTypeDoc ||
		chapter.CanonicalHref != "OPS/chapters/chapter1.xhtml" ||
		chapter.Fragment != "intro" ||
		chapter.TargetKey != "OPS/chapters/chapter1.xhtml#intro" {
		t.Fatalf("unexpected chapter plan: %#v", chapter)
	}
	reference := part1.Children[1]
	if !reference.Reference || !strings.Contains(reference.ContentMD, "> 本章节内容见：") {
		t.Fatalf("expected duplicate target to become reference doc, got %#v", reference)
	}
	if reference.ReferenceTargetKey != chapter.TargetKey {
		t.Fatalf("expected reference target %q, got %q", chapter.TargetKey, reference.ReferenceTargetKey)
	}

	part2 := plan.Root[1]
	if part2.Type != adminSpaceEPUBPlannedNodeTypeFolder || len(part2.Children) != 2 {
		t.Fatalf("expected Part 2 folder with body doc and child doc, got %#v", part2)
	}
	if part2.Children[0].Title != "正文" || part2.Children[0].TargetKey != "OPS/chapters/chapter2.xhtml" {
		t.Fatalf("expected folder body doc, got %#v", part2.Children[0])
	}
	if part2.Children[1].Title != "正文 2" {
		t.Fatalf("expected sibling title conflict to be uniqued, got %#v", part2.Children[1])
	}

	missing := plan.Root[2]
	if missing.TargetKey != "OPS/chapters/chapter3.xhtml" || missing.Fragment != "" {
		t.Fatalf("expected missing fragment fallback to canonical href, got %#v", missing)
	}
	if len(warnings) == 0 {
		t.Fatal("expected warning for missing fragment fallback")
	}
	if plan.Targets["OPS/chapters/chapter1.xhtml#intro"].DocumentID == "" ||
		plan.Targets["OPS/chapters/chapter2.xhtml"].ReaderURL == "" {
		t.Fatalf("expected target mappings with document id and reader url, got %#v", plan.Targets)
	}
	if plan.CanonicalTargets["OPS/chapters/chapter1.xhtml"].ReaderURL != chapter.ReaderURL {
		t.Fatalf("expected canonical target to point at primary chapter, got %#v", plan.CanonicalTargets)
	}
}

func TestNormalizeAdminSpaceEPUBHref(t *testing.T) {
	t.Parallel()

	target, ok := normalizeAdminSpaceEPUBHref("OPS", "chapters/../chapter.xhtml#Intro")
	if !ok {
		t.Fatal("expected href to normalize")
	}
	if target.CanonicalHref != "OPS/chapter.xhtml" || target.Fragment != "Intro" || target.TargetKey != "OPS/chapter.xhtml#Intro" {
		t.Fatalf("unexpected normalized target: %#v", target)
	}

	if _, ok := normalizeAdminSpaceEPUBHref("OPS", "javascript:alert(1)"); ok {
		t.Fatal("expected dangerous protocol to be rejected")
	}
}

func TestRewriteAdminSpaceEPUBInternalLinks_RewritesAndDegradesTargets(t *testing.T) {
	t.Parallel()

	plan, _ := planAdminSpaceEPUBImportTree(adminSpaceEPUBPlanInput{
		OPFRoot: "OPS",
		Items: []adminSpaceEPUBNavItem{
			{Title: "Chapter 1 Intro", Href: "chapters/chapter1.xhtml#intro"},
			{Title: "Chapter 2", Href: "chapters/chapter2.xhtml"},
		},
		ChapterHTMLByCanonicalHref: map[string][]byte{
			"OPS/chapters/chapter1.xhtml": []byte(`<html><body><h1 id="intro">Intro</h1></body></html>`),
			"OPS/chapters/chapter2.xhtml": []byte(`<html><body><h1>Chapter 2</h1></body></html>`),
		},
	})

	rewritten, warnings, err := rewriteAdminSpaceEPUBInternalLinks(adminSpaceEPUBLinkRewriteInput{
		SourceKey:           "OPS/chapters/chapter1.xhtml",
		SourceCanonicalHref: "OPS/chapters/chapter1.xhtml",
		HTML: []byte(`<body>
			<a href="#intro">同文件 fragment</a>
			<a href="chapter2.xhtml">跨文件</a>
			<a href="chapter2.xhtml#missing">fragment 降级</a>
			<a href="missing.xhtml">缺失目标</a>
			<a href="https://example.com/ref">外链</a>
		</body>`),
		Plan: plan,
	})
	if err != nil {
		t.Fatalf("rewriteAdminSpaceEPUBInternalLinks returned error: %v", err)
	}

	if !strings.Contains(rewritten, `<a href="/read/doc-001">同文件 fragment</a>`) {
		t.Fatalf("expected same-file fragment to rewrite to first reader url, got %q", rewritten)
	}
	if !strings.Contains(rewritten, `<a href="/read/doc-002">跨文件</a>`) {
		t.Fatalf("expected cross-file link to rewrite to chapter 2, got %q", rewritten)
	}
	if !strings.Contains(rewritten, `<a href="/read/doc-002">fragment 降级</a>`) {
		t.Fatalf("expected missing fragment to degrade to canonical target, got %q", rewritten)
	}
	if strings.Contains(rewritten, "missing.xhtml") {
		t.Fatalf("expected missing target href to be removed, got %q", rewritten)
	}
	if !strings.Contains(rewritten, ">缺失目标</a>") {
		t.Fatalf("expected missing target text to be retained, got %q", rewritten)
	}
	if !strings.Contains(rewritten, `<a href="https://example.com/ref">外链</a>`) {
		t.Fatalf("expected external link to stay unchanged, got %q", rewritten)
	}
	if len(warnings) != 2 {
		t.Fatalf("expected two warnings for canonical fallback and missing target, got %#v", warnings)
	}
	for _, warning := range warnings {
		if !strings.Contains(warning, "OPS/chapters/chapter1.xhtml") {
			t.Fatalf("expected warning to include source key, got %q", warning)
		}
	}
}

func TestBuildAdminSpaceEPUBReferenceMarkdown(t *testing.T) {
	t.Parallel()

	markdown := buildAdminSpaceEPUBReferenceMarkdown("第一章", "/read/doc-001")
	if markdown != "> 本章节内容见：[第一章](/read/doc-001)" {
		t.Fatalf("unexpected reference markdown: %q", markdown)
	}
}

func TestRewriteAdminSpaceEPUBInternalLinks_ConvertedMarkdownHasNoEPUBRelativeLinks(t *testing.T) {
	t.Parallel()

	plan, _ := planAdminSpaceEPUBImportTree(adminSpaceEPUBPlanInput{
		OPFRoot: "OPS",
		Items: []adminSpaceEPUBNavItem{
			{Title: "Chapter 1", Href: "chapter1.xhtml"},
			{Title: "Chapter 2", Href: "chapter2.xhtml"},
		},
		ChapterHTMLByCanonicalHref: map[string][]byte{
			"OPS/chapter1.xhtml": []byte(`<html><body><h1>Chapter 1</h1></body></html>`),
			"OPS/chapter2.xhtml": []byte(`<html><body><h1>Chapter 2</h1></body></html>`),
		},
	})
	rewritten, warnings, err := rewriteAdminSpaceEPUBInternalLinks(adminSpaceEPUBLinkRewriteInput{
		SourceKey:           "OPS/chapter1.xhtml",
		SourceCanonicalHref: "OPS/chapter1.xhtml",
		HTML:                []byte(`<body><p><a href="chapter2.xhtml">下一章</a> <a href="missing.xhtml">缺失章</a></p></body>`),
		Plan:                plan,
	})
	if err != nil {
		t.Fatalf("rewriteAdminSpaceEPUBInternalLinks returned error: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected one warning for missing target, got %#v", warnings)
	}

	result, err := NewHTMLMarkdownConverter().Convert(context.Background(), ConvertHTMLMarkdownInput{
		SourceKey:    "OPS/chapter1.xhtml",
		PlainDocMode: true,
		HTML:         rewritten,
	})
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}
	if strings.Contains(result.Markdown, "chapter2.xhtml") || strings.Contains(result.Markdown, "missing.xhtml") {
		t.Fatalf("expected markdown without epub relative links, got %q", result.Markdown)
	}
	if !strings.Contains(result.Markdown, "[下一章](/read/doc-002)") || !strings.Contains(result.Markdown, "缺失章") {
		t.Fatalf("expected rewritten link and retained missing text, got %q", result.Markdown)
	}
}

func TestLocalizeAdminSpaceEPUBChapterImages_LocalizesRelativeAndDataImages(t *testing.T) {
	t.Parallel()

	entries := collectAdminSpaceEPUBEntriesForImageTest(t, map[string][]byte{
		"OPS/images/cover.png": []byte("png-payload"),
		"OPS/images/icon.svg":  []byte(`<svg viewBox="0 0 10 10"><path d="M0 0h10v10z"/></svg>`),
	})
	var saved []adminSpaceEPUBImageAsset
	rewritten, warnings, err := localizeAdminSpaceEPUBChapterImages(adminSpaceEPUBImageLocalizeInput{
		SourceKey:           "OPS/chapters/chapter1.xhtml",
		SourceCanonicalHref: "OPS/chapters/chapter1.xhtml",
		HTML: []byte(`<body>
			<img src="../images/cover.png" alt="封面">
			<img src="../images/icon.svg" alt="图标">
			<img src="data:image/png;base64,` + base64.StdEncoding.EncodeToString([]byte("inline-png")) + `" alt="内联图">
		</body>`),
		Entries: entries,
		Localize: func(asset adminSpaceEPUBImageAsset) (string, error) {
			saved = append(saved, asset)
			return "/uploads/" + asset.FileName, nil
		},
	})
	if err != nil {
		t.Fatalf("localizeAdminSpaceEPUBChapterImages returned error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if len(saved) != 3 {
		t.Fatalf("expected two localized images, got %#v", saved)
	}
	if saved[0].CanonicalHref != "OPS/images/cover.png" || saved[0].ContentType != "image/png" {
		t.Fatalf("unexpected relative image asset: %#v", saved[0])
	}
	if saved[1].CanonicalHref != "OPS/images/icon.svg" || saved[1].ContentType != "image/svg+xml" {
		t.Fatalf("unexpected svg image asset: %#v", saved[1])
	}
	if saved[2].CanonicalHref != "" || saved[2].ContentType != "image/png" {
		t.Fatalf("unexpected data image asset: %#v", saved[2])
	}
	if strings.Contains(rewritten, "../images/cover.png") || strings.Contains(rewritten, "data:image/") {
		t.Fatalf("expected image src to be localized, got %q", rewritten)
	}
	if !strings.Contains(rewritten, `src="/uploads/cover.png"`) ||
		!strings.Contains(rewritten, `src="/uploads/icon.svg"`) ||
		!strings.Contains(rewritten, `src="/uploads/inline.png"`) {
		t.Fatalf("expected localized upload urls, got %q", rewritten)
	}
}

func TestLocalizeAdminSpaceEPUBChapterImages_DegradesOversizedDangerousAndFailedImages(t *testing.T) {
	t.Parallel()

	entries := collectAdminSpaceEPUBEntriesForImageTest(t, map[string][]byte{
		"OPS/images/huge.png":          bytes.Repeat([]byte("x"), int(maxAdminSpaceEPUBImageBytes)+1),
		"OPS/images/dangerous.svg":     []byte(`<svg><script>alert(1)</script></svg>`),
		"OPS/images/storage-error.png": []byte("png-payload"),
	})
	rewritten, warnings, err := localizeAdminSpaceEPUBChapterImages(adminSpaceEPUBImageLocalizeInput{
		SourceKey:           "OPS/chapters/chapter1.xhtml",
		SourceCanonicalHref: "OPS/chapters/chapter1.xhtml",
		HTML: []byte(`<body>
			<img src="../images/huge.png" alt="超大图">
			<img src="../images/dangerous.svg" alt="危险 SVG">
			<img src="../images/storage-error.png" alt="存储失败图">
		</body>`),
		Entries: entries,
		Localize: func(asset adminSpaceEPUBImageAsset) (string, error) {
			return "", fmt.Errorf("模拟存储失败: %s", asset.FileName)
		},
	})
	if err != nil {
		t.Fatalf("localizeAdminSpaceEPUBChapterImages returned error: %v", err)
	}
	for _, unexpected := range []string{"huge.png", "dangerous.svg", "storage-error.png", "<script"} {
		if strings.Contains(rewritten, unexpected) {
			t.Fatalf("expected unsafe image source %q to be removed, got %q", unexpected, rewritten)
		}
	}
	for _, want := range []string{"超大图", "危险 SVG", "存储失败图"} {
		if !strings.Contains(rewritten, want) {
			t.Fatalf("expected degraded alt text %q, got %q", want, rewritten)
		}
	}
	if len(warnings) != 3 {
		t.Fatalf("expected three warnings, got %#v", warnings)
	}
	for _, warning := range warnings {
		if !strings.Contains(warning, "OPS/chapters/chapter1.xhtml") {
			t.Fatalf("expected warning to include source key, got %q", warning)
		}
	}
}

func TestAdminSpaceEPUBImageContentTypeSupportsAllowedTypes(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"cover.png":  "image/png",
		"cover.jpg":  "image/jpeg",
		"cover.jpeg": "image/jpeg",
		"cover.gif":  "image/gif",
		"cover.webp": "image/webp",
		"cover.svg":  "image/svg+xml",
	}
	for fileName, want := range cases {
		fileName := fileName
		want := want
		t.Run(fileName, func(t *testing.T) {
			t.Parallel()
			if got := adminSpaceEPUBImageContentType(fileName); got != want {
				t.Fatalf("expected %s, got %s", want, got)
			}
		})
	}
}

func TestParseAdminSpaceEPUBDataImageAssetRejectsOversizedBeforeDecode(t *testing.T) {
	t.Parallel()

	oversizedPayload := strings.Repeat("A", int(maxAdminSpaceEPUBImageBytes)*2)
	_, err := parseAdminSpaceEPUBDataImageAsset("data:image/png;base64," + oversizedPayload)
	if err == nil {
		t.Fatal("expected oversized data image to fail before decode")
	}
}

func collectAdminSpaceEPUBEntriesForImageTest(t *testing.T, files map[string][]byte) map[string]*zip.File {
	t.Helper()

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	for name, payload := range files {
		writer, err := zipWriter.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := writer.Write(payload); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		t.Fatalf("open zip reader: %v", err)
	}
	entries, err := collectAdminSpaceEPUBEntries(reader)
	if err != nil {
		t.Fatalf("collect epub entries: %v", err)
	}
	return entries
}

func TestSanitizeAdminSpaceEPUBChapterHTML_RemovesDangerousHTML(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		input           adminSpaceEPUBHTMLSanitizeInput
		wantContains    []string
		wantNotContains []string
		wantWarnings    int
	}{
		{
			name: "keeps body content and removes blocked tags and event attributes",
			input: adminSpaceEPUBHTMLSanitizeInput{
				SourceKey: "OPS/chapter.xhtml",
				Title:     "第一章",
				HTML: []byte(`<!doctype html><html><head><title>Hidden</title></head><body>
					<h1 onclick="alert(1)">正文</h1>
					<script>alert(1)</script>
					<style>body{display:none}</style>
					<form><input value="secret"><button>提交</button></form>
					<p data-keep="yes">段落</p>
				</body></html>`),
			},
			wantContains: []string{
				"<h1>正文</h1>",
				`<p data-keep="yes">段落</p>`,
			},
			wantNotContains: []string{
				"Hidden",
				"<script",
				"<style",
				"<form",
				"<input",
				"<button",
				"onclick",
			},
			wantWarnings: 4,
		},
		{
			name: "keeps external and internal links but unwraps dangerous links",
			input: adminSpaceEPUBHTMLSanitizeInput{
				SourceKey: "OPS/chapter.xhtml",
				Title:     "第二章",
				HTML: []byte(`<body>
					<a href="https://example.com/ref">外链</a>
					<a href="chapter2.xhtml#part">内链</a>
					<a href="javascript:alert(1)">危险链接</a>
					<a href="file:///etc/passwd">文件链接</a>
					<a href="/etc/passwd">本机路径</a>
					<a href="data:text/html;base64,PHNjcmlwdA==">危险 data</a>
				</body>`),
			},
			wantContains: []string{
				`<a href="https://example.com/ref">外链</a>`,
				`<a href="chapter2.xhtml#part">内链</a>`,
				"危险链接",
				"文件链接",
				"本机路径",
				"危险 data",
			},
			wantNotContains: []string{
				"javascript:",
				"file:",
				`href="/etc/passwd"`,
				"data:text/html",
			},
			wantWarnings: 4,
		},
		{
			name: "keeps data image for later localization and replaces dangerous image src with alt text",
			input: adminSpaceEPUBHTMLSanitizeInput{
				SourceKey: "OPS/chapter.xhtml",
				Title:     "第三章",
				HTML: []byte(`<body>
					<img src="data:image/png;base64,AAAA" alt="封面">
					<img src="data:text/html;base64,PHNjcmlwdA==" alt="危险图">
					<img src="C:\Users\me\secret.png" alt="本机图">
				</body>`),
			},
			wantContains: []string{
				`<img src="data:image/png;base64,AAAA" alt="封面"/>`,
				"危险图",
				"本机图",
			},
			wantNotContains: []string{
				"data:text/html",
				`C:\Users\me\secret.png`,
			},
			wantWarnings: 2,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, warnings, err := sanitizeAdminSpaceEPUBChapterHTML(tc.input)
			if err != nil {
				t.Fatalf("sanitizeAdminSpaceEPUBChapterHTML returned error: %v", err)
			}
			for _, want := range tc.wantContains {
				if !strings.Contains(got, want) {
					t.Fatalf("expected sanitized html to contain %q, got %q", want, got)
				}
			}
			for _, unwanted := range tc.wantNotContains {
				if strings.Contains(got, unwanted) {
					t.Fatalf("expected sanitized html not to contain %q, got %q", unwanted, got)
				}
			}
			if len(warnings) != tc.wantWarnings {
				t.Fatalf("expected %d warnings, got %d: %#v", tc.wantWarnings, len(warnings), warnings)
			}
			for _, warning := range warnings {
				if !strings.Contains(warning, tc.input.SourceKey) || !strings.Contains(warning, tc.input.Title) {
					t.Fatalf("expected warning to include source key and title, got %q", warning)
				}
			}
		})
	}
}
