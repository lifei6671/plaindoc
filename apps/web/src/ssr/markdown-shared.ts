import MarkdownIt from "markdown-it";
import type { Components } from "react-markdown";
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

// buildReaderMarkdownRenderer 产出阅读页 SSR 与编辑器预览共享的 markdown 渲染配置。
export function buildReaderMarkdownRenderer(
  content: string,
  activePreviewTheme: PreviewThemeTemplate
): BuildReaderMarkdownRendererResult {
  const tocItems = parseTocFromMarkdown(content, markdownTextParser).items;
  return {
    remarkPlugins: createSharedRemarkPlugins("link", { includeBlockAnchors: false }),
    rehypePlugins: createSharedRehypePlugins(),
    components: createSharedMarkdownComponents({
      activePreviewTheme,
      tocItems,
      onTocNavigate: () => {}
    })
  };
}
