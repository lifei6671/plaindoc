package service

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestHTMLMarkdownConverter_ConvertsPlainDocEditableMarkdown(t *testing.T) {
	t.Parallel()

	converter := NewHTMLMarkdownConverter()
	result, err := converter.Convert(context.Background(), ConvertHTMLMarkdownInput{
		SourceKey:    "OPS/chapter.xhtml",
		PlainDocMode: true,
		HTML: `<h1>标题</h1>
<p>正文 <strong>加粗</strong> <em>强调</em></p>
<ul><li>项目一</li><li>项目二</li></ul>
<pre><code class="language-go">fmt.Println("hello")
</code></pre>
<table><tr><th>名称</th><th>数量</th></tr><tr><td>苹果</td><td>3</td></tr></table>
<p>H<sub>2</sub>O E=mc<sup>2</sup><span> span 文本</span></p>
<p><a href="https://example.com/ref">外链</a> <a href="/read/doc-001">内链</a></p>
<p><img src="/uploads/space/doc/image.png" alt="封面"></p>`,
	})
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}
	markdown := result.Markdown
	for _, want := range []string{
		"# 标题",
		"**加粗**",
		"_强调_",
		"- 项目一",
		"```go",
		"| 名称",
		"H~2~O E=mc^2^",
		"[外链](https://example.com/ref)",
		"[内链](/read/doc-001)",
		"![封面](/uploads/space/doc/image.png)",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("expected markdown to contain %q, got %q", want, markdown)
		}
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", result.Warnings)
	}
}

func TestHTMLMarkdownConverter_DegradesUnsafeURLs(t *testing.T) {
	t.Parallel()

	converter := NewHTMLMarkdownConverter()
	result, err := converter.Convert(context.Background(), ConvertHTMLMarkdownInput{
		SourceKey:    "OPS/chapter.xhtml",
		PlainDocMode: true,
		HTML: `<p>
<a href="chapter2.xhtml#part">未重写内链</a>
<a href="javascript:alert(1)">危险链接</a>
<img src="data:image/png;base64,AAAA" alt="未本地化图片">
<img src="https://cdn.example.com/raw.png" alt="外部图片">
</p>`,
	})
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}
	markdown := result.Markdown
	for _, unwanted := range []string{"chapter2.xhtml", "javascript:", "data:image", "https://cdn.example.com/raw.png"} {
		if strings.Contains(markdown, unwanted) {
			t.Fatalf("expected markdown not to contain %q, got %q", unwanted, markdown)
		}
	}
	for _, want := range []string{"未重写内链", "危险链接", "未本地化图片", "外部图片"} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("expected markdown to retain text %q, got %q", want, markdown)
		}
	}
	if len(result.Warnings) != 4 {
		t.Fatalf("expected 3 warnings, got %#v", result.Warnings)
	}
	for _, warning := range result.Warnings {
		if !strings.Contains(warning, "OPS/chapter.xhtml") {
			t.Fatalf("expected warning to include source key, got %q", warning)
		}
	}
}

func TestHTMLMarkdownConverter_DegradesComplexTables(t *testing.T) {
	t.Parallel()

	converter := NewHTMLMarkdownConverter()
	result, err := converter.Convert(context.Background(), ConvertHTMLMarkdownInput{
		SourceKey:    "OPS/table.xhtml",
		PlainDocMode: true,
		HTML: `<table>
<tr><th colspan="2">复杂表头</th></tr>
<tr><td>左侧</td><td>右侧</td></tr>
</table>`,
	})
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}
	if strings.Contains(result.Markdown, "|") {
		t.Fatalf("expected complex table to degrade to plain text, got %q", result.Markdown)
	}
	for _, want := range []string{"复杂表头", "左侧", "右侧"} {
		if !strings.Contains(result.Markdown, want) {
			t.Fatalf("expected degraded table text to contain %q, got %q", want, result.Markdown)
		}
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "OPS/table.xhtml") {
		t.Fatalf("expected table warning with source key, got %#v", result.Warnings)
	}
}

func TestHTMLMarkdownConverter_RejectsEmptyInput(t *testing.T) {
	t.Parallel()

	converter := NewHTMLMarkdownConverter()
	if _, err := converter.Convert(context.Background(), ConvertHTMLMarkdownInput{}); err == nil {
		t.Fatal("expected empty input to fail")
	}
}

func BenchmarkHTMLMarkdownConverter_SerialChapters(b *testing.B) {
	chapterCounts := []int{20, 50, 100}
	chapterSizes := []int{10 << 10, 100 << 10, 1 << 20}

	for _, chapterCount := range chapterCounts {
		chapterCount := chapterCount
		for _, chapterSize := range chapterSizes {
			chapterSize := chapterSize
			b.Run(fmt.Sprintf("chapters_%03d_size_%dKiB", chapterCount, chapterSize>>10), func(b *testing.B) {
				converter := NewHTMLMarkdownConverter()
				chapters := buildBenchmarkHTMLMarkdownChapters(chapterCount, chapterSize)
				b.ReportAllocs()
				b.ResetTimer()

				var lastDurations []time.Duration
				var lastWarnings int
				var lastPeakHeapBytes uint64
				for i := 0; i < b.N; i++ {
					runtime.GC()
					start := time.Now()
					durations, warningCount, peakHeapBytes := convertBenchmarkHTMLMarkdownChapters(b, converter, chapters)
					elapsed := time.Since(start)
					lastDurations = durations
					lastWarnings = warningCount
					lastPeakHeapBytes = peakHeapBytes
					b.ReportMetric(float64(elapsed.Milliseconds()), "serial_ms/op")
				}

				if len(lastDurations) > 0 {
					b.ReportMetric(float64(percentileDuration(lastDurations, 0.95).Milliseconds()), "chapter_p95_ms")
				}
				b.ReportMetric(float64(lastWarnings), "warnings/op")
				b.ReportMetric(float64(lastPeakHeapBytes)/(1<<20), "heap_alloc_mib/op")
			})
		}
	}
}

func buildBenchmarkHTMLMarkdownChapters(chapterCount int, targetSize int) []string {
	chapters := make([]string, 0, chapterCount)
	for index := 0; index < chapterCount; index++ {
		chapters = append(chapters, buildBenchmarkHTMLMarkdownChapter(index+1, targetSize))
	}
	return chapters
}

func buildBenchmarkHTMLMarkdownChapter(chapterIndex int, targetSize int) string {
	paragraph := `<p>这是用于 EPUB HTML 转 Markdown 性能基准的段落，包含 <strong>加粗</strong>、<em>强调</em>、<a href="/read/doc-001">安全内链</a>、<a href="chapter.xhtml#raw">未重写链接</a>、<img src="/uploads/space/doc/image.png" alt="图片"> 和 <img src="data:image/png;base64,AAAA" alt="未本地化图片">。</p>`
	tableHTML := `<table><tr><th>名称</th><th>数量</th></tr><tr><td>苹果</td><td>3</td></tr><tr><td>香蕉</td><td>5</td></tr></table>`
	codeHTML := `<pre><code class="language-go">fmt.Println("hello")
</code></pre>`

	var builder strings.Builder
	builder.Grow(targetSize + 512)
	builder.WriteString(fmt.Sprintf(`<h1>章节 %03d</h1>`, chapterIndex))
	builder.WriteString(tableHTML)
	builder.WriteString(codeHTML)
	for builder.Len() < targetSize {
		builder.WriteString(paragraph)
	}
	return builder.String()
}

func convertBenchmarkHTMLMarkdownChapters(
	b *testing.B,
	converter HTMLMarkdownConverter,
	chapters []string,
) ([]time.Duration, int, uint64) {
	b.Helper()

	durations := make([]time.Duration, 0, len(chapters))
	warningCount := 0
	var peakHeapBytes uint64
	for index, chapterHTML := range chapters {
		start := time.Now()
		result, err := converter.Convert(context.Background(), ConvertHTMLMarkdownInput{
			SourceKey:    fmt.Sprintf("OPS/chapter-%03d.xhtml", index+1),
			PlainDocMode: true,
			HTML:         chapterHTML,
		})
		if err != nil {
			b.Fatalf("Convert returned error: %v", err)
		}
		durations = append(durations, time.Since(start))
		warningCount += len(result.Warnings)
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		if memStats.Alloc > peakHeapBytes {
			peakHeapBytes = memStats.Alloc
		}
	}
	return durations, warningCount, peakHeapBytes
}

func percentileDuration(values []time.Duration, percentile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sortedValues := append([]time.Duration(nil), values...)
	sort.Slice(sortedValues, func(i int, j int) bool {
		return sortedValues[i] < sortedValues[j]
	})
	index := int(float64(len(sortedValues)-1) * percentile)
	return sortedValues[index]
}
