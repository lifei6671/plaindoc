import MarkdownIt from "markdown-it";
import type { Components } from "react-markdown";
import type { MermaidRenderResult } from "../editor/mermaid-shared";
import type { PreviewThemeTemplate } from "../preview-themes";
import {
  createSharedMarkdownComponents,
  createSharedRehypePlugins,
  createSharedRemarkPlugins
} from "../editor/markdown-shared";
import { parseTocFromMarkdown } from "../editor/markdown-utils";

const markdownTextParser = new MarkdownIt({
  html: false,
  linkify: true,
  typographer: false
});

interface BuildReaderMarkdownRendererResult {
  remarkPlugins: ReturnType<typeof createSharedRemarkPlugins>;
  rehypePlugins: ReturnType<typeof createSharedRehypePlugins>;
  components: Components;
}

interface BuildReaderMarkdownRendererOptions {
  requestOrigin?: string;
  preRenderedMermaidMap?: ReadonlyMap<string, MermaidRenderResult>;
}

// buildReaderMarkdownRenderer 产出阅读页 SSR 与编辑器预览共享的 markdown 渲染配置。
export function buildReaderMarkdownRenderer(
  content: string,
  activePreviewTheme: PreviewThemeTemplate,
  options: BuildReaderMarkdownRendererOptions = {}
): BuildReaderMarkdownRendererResult {
  const tocItems = parseTocFromMarkdown(content, markdownTextParser).items;
  const components = createSharedMarkdownComponents({
    activePreviewTheme,
    tocItems,
    onTocNavigate: () => {},
    requestOrigin: options.requestOrigin,
    preRenderedMermaidMap: options.preRenderedMermaidMap,
    includeMermaidSourcePayload: true
  });

  return {
    remarkPlugins: createSharedRemarkPlugins("link", { includeBlockAnchors: false }),
    rehypePlugins: createSharedRehypePlugins(),
    components
  };
}
