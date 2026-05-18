package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"golang.org/x/net/html"
)

// HTMLMarkdownConverter 是 EPUB/Office HTML 转 Markdown 的 service 层边界。
// 调用方必须先完成 HTML 清洗、内部链接重写和图片本地化，转换器只负责生成可编辑 Markdown。
type HTMLMarkdownConverter interface {
	Convert(ctx context.Context, input ConvertHTMLMarkdownInput) (ConvertHTMLMarkdownResult, error)
}

// ConvertHTMLMarkdownInput 描述一次 HTML 到 Markdown 转换请求。
// PlainDocMode 为后续差异化规则预留，当前默认按 PlainDoc 可编辑 Markdown 输出。
type ConvertHTMLMarkdownInput struct {
	HTML         string
	SourceKey    string
	PlainDocMode bool
}

// ConvertHTMLMarkdownResult 返回转换后的 Markdown 以及可降级问题。
// Warnings 用于导入任务日志展示，避免单个链接或图片异常中断整本 EPUB。
type ConvertHTMLMarkdownResult struct {
	Markdown string
	Warnings []string
}

type htmlMarkdownConverter struct{}

// NewHTMLMarkdownConverter 创建默认 HTML 转 Markdown 转换器。
func NewHTMLMarkdownConverter() HTMLMarkdownConverter {
	return htmlMarkdownConverter{}
}

func (c htmlMarkdownConverter) Convert(ctx context.Context, input ConvertHTMLMarkdownInput) (ConvertHTMLMarkdownResult, error) {
	if err := ctx.Err(); err != nil {
		return ConvertHTMLMarkdownResult{}, fmt.Errorf("转换 HTML 为 Markdown 前检查上下文: %w", err)
	}
	if strings.TrimSpace(input.HTML) == "" {
		return ConvertHTMLMarkdownResult{}, fmt.Errorf("转换 HTML 为 Markdown: HTML 内容为空")
	}

	warnings := []string{}
	conv := newPlainDocHTMLMarkdownConverter(input, &warnings)
	markdown, err := conv.ConvertString(input.HTML, converter.WithContext(ctx))
	if err != nil {
		return ConvertHTMLMarkdownResult{}, fmt.Errorf("转换 HTML 为 Markdown: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return ConvertHTMLMarkdownResult{}, fmt.Errorf("转换 HTML 为 Markdown 后检查上下文: %w", err)
	}
	return ConvertHTMLMarkdownResult{
		Markdown: strings.TrimSpace(markdown),
		Warnings: warnings,
	}, nil
}

func newPlainDocHTMLMarkdownConverter(input ConvertHTMLMarkdownInput, warnings *[]string) *converter.Converter {
	return converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(
				commonmark.WithHeadingStyle(commonmark.HeadingStyleATX),
				commonmark.WithBulletListMarker("-"),
				commonmark.WithEmDelimiter("_"),
				commonmark.WithStrongDelimiter("**"),
				commonmark.WithCodeBlockFence("```"),
			),
			table.NewTablePlugin(
				table.WithHeaderPromotion(true),
				table.WithSpanCellBehavior(table.SpanBehaviorMirror),
				table.WithCellPaddingBehavior(table.CellPaddingBehaviorAligned),
				table.WithSkipEmptyRows(true),
				table.WithNewlineBehavior(table.NewlineBehaviorPreserve),
			),
			&plainDocMarkdownPlugin{input: input, warnings: warnings},
		),
		converter.WithEscapeMode(converter.EscapeModeSmart),
	)
}

type plainDocMarkdownPlugin struct {
	input    ConvertHTMLMarkdownInput
	warnings *[]string
}

func (p *plainDocMarkdownPlugin) Name() string {
	return "plaindoc-markdown"
}

func (p *plainDocMarkdownPlugin) Init(conv *converter.Converter) error {
	// 图片和链接必须早于 commonmark 默认规则执行，确保不会把未本地化或未重写的 URL 输出到 Markdown。
	conv.Register.RendererFor("img", converter.TagTypeInline, p.renderImage, converter.PriorityEarly)
	conv.Register.RendererFor("a", converter.TagTypeInline, p.renderLink, converter.PriorityEarly)
	conv.Register.RendererFor("sup", converter.TagTypeInline, renderWrappedInline("^"), converter.PriorityEarly)
	conv.Register.RendererFor("sub", converter.TagTypeInline, renderWrappedInline("~"), converter.PriorityEarly)
	conv.Register.RendererFor("span", converter.TagTypeInline, renderChildrenOnly, converter.PriorityEarly)
	conv.Register.Renderer(p.renderComplexTable, converter.PriorityEarly)
	return nil
}

func (p *plainDocMarkdownPlugin) renderImage(_ converter.Context, w converter.Writer, node *html.Node) converter.RenderStatus {
	src := strings.TrimSpace(htmlAttribute(node, "src"))
	alt := strings.TrimSpace(htmlAttribute(node, "alt"))
	if src == "" {
		return converter.RenderSuccess
	}
	if !isSafePlainDocMarkdownImageURL(src) {
		p.warn("已降级未本地化或不安全图片 " + src)
		if alt != "" {
			if err := writeMarkdownString(w, alt); err != nil {
				p.warn(err.Error())
			}
		}
		return converter.RenderSuccess
	}
	if err := writeMarkdownString(w, "!["+escapeMarkdownImageAlt(alt)+"]("+src+")"); err != nil {
		p.warn(err.Error())
	}
	return converter.RenderSuccess
}

func (p *plainDocMarkdownPlugin) renderLink(ctx converter.Context, w converter.Writer, node *html.Node) converter.RenderStatus {
	href := strings.TrimSpace(htmlAttribute(node, "href"))
	if href == "" {
		ctx.RenderChildNodes(ctx, w, node)
		return converter.RenderSuccess
	}
	if !isSafePlainDocMarkdownLinkURL(href) {
		p.warn("已降级未重写或不安全链接 " + href)
		ctx.RenderChildNodes(ctx, w, node)
		return converter.RenderSuccess
	}
	if err := writeMarkdownString(w, "["); err != nil {
		p.warn(err.Error())
		return converter.RenderSuccess
	}
	ctx.RenderChildNodes(ctx, w, node)
	if err := writeMarkdownString(w, "]("+href+")"); err != nil {
		p.warn(err.Error())
	}
	return converter.RenderSuccess
}

func (p *plainDocMarkdownPlugin) renderComplexTable(ctx converter.Context, w converter.Writer, node *html.Node) converter.RenderStatus {
	if node == nil || node.Type != html.ElementNode || !strings.EqualFold(node.Data, "table") {
		return converter.RenderTryNext
	}
	if !hasAdminSpaceHTMLTableSpan(node) {
		return converter.RenderTryNext
	}
	p.warn("复杂表格包含 rowspan/colspan，已降级为纯文本")
	text := strings.TrimSpace(collectHTMLText(node))
	if text != "" {
		if err := writeMarkdownString(w, "\n\n"+text+"\n\n"); err != nil {
			p.warn(err.Error())
		}
	}
	_ = ctx
	return converter.RenderSuccess
}

func (p *plainDocMarkdownPlugin) warn(detail string) {
	if p == nil || p.warnings == nil {
		return
	}
	sourceKey := strings.TrimSpace(p.input.SourceKey)
	if sourceKey == "" {
		sourceKey = "unknown"
	}
	*p.warnings = append(*p.warnings, "HTML 转 Markdown "+sourceKey+"："+detail)
}

func renderWrappedInline(wrapper string) converter.HandleRenderFunc {
	return func(ctx converter.Context, w converter.Writer, node *html.Node) converter.RenderStatus {
		if err := writeMarkdownString(w, wrapper); err != nil {
			return converter.RenderSuccess
		}
		ctx.RenderChildNodes(ctx, w, node)
		if err := writeMarkdownString(w, wrapper); err != nil {
			return converter.RenderSuccess
		}
		return converter.RenderSuccess
	}
}

func renderChildrenOnly(ctx converter.Context, w converter.Writer, node *html.Node) converter.RenderStatus {
	ctx.RenderChildNodes(ctx, w, node)
	return converter.RenderSuccess
}

// writeMarkdownString 统一处理 converter.Writer 的写入错误。
// Renderer 接口不能向上返回 error，只能把可恢复写入异常降级为 warning 或提前结束当前节点渲染。
func writeMarkdownString(w converter.Writer, value string) error {
	if _, err := w.WriteString(value); err != nil {
		return fmt.Errorf("写入 Markdown 片段: %w", err)
	}
	return nil
}

func htmlAttribute(node *html.Node, key string) string {
	if node == nil {
		return ""
	}
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, key) {
			return attr.Val
		}
	}
	return ""
}

func isSafePlainDocMarkdownImageURL(rawURL string) bool {
	trimmed := strings.TrimSpace(rawURL)
	// EPUB 图片进入转换器前必须已经本地化，外部 http/https 图片在这里降级为 alt 文本。
	return strings.HasPrefix(trimmed, "/uploads/") ||
		strings.HasPrefix(trimmed, "/api/uploads/")
}

func isSafePlainDocMarkdownLinkURL(rawURL string) bool {
	trimmed := strings.TrimSpace(rawURL)
	lowerURL := strings.ToLower(trimmed)
	return strings.HasPrefix(trimmed, "/r/") ||
		strings.HasPrefix(trimmed, "/share/") ||
		strings.HasPrefix(trimmed, "#") ||
		strings.HasPrefix(lowerURL, "http://") ||
		strings.HasPrefix(lowerURL, "https://")
}

func escapeMarkdownImageAlt(alt string) string {
	return strings.ReplaceAll(strings.TrimSpace(alt), "]", "\\]")
}

func hasAdminSpaceHTMLTableSpan(node *html.Node) bool {
	if node == nil {
		return false
	}
	if node.Type == html.ElementNode {
		if strings.TrimSpace(htmlAttribute(node, "rowspan")) != "" || strings.TrimSpace(htmlAttribute(node, "colspan")) != "" {
			return true
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if hasAdminSpaceHTMLTableSpan(child) {
			return true
		}
	}
	return false
}

func collectHTMLText(node *html.Node) string {
	if node == nil {
		return ""
	}
	if node.Type == html.TextNode {
		return strings.TrimSpace(node.Data)
	}
	parts := []string{}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if text := strings.TrimSpace(collectHTMLText(child)); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " ")
}
