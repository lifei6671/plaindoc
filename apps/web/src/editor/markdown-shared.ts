import type { Components } from "react-markdown";
import rehypeKatex from "rehype-katex";
import rehypeRaw from "rehype-raw";
import rehypeSanitize from "rehype-sanitize";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import type { MermaidRenderResult } from "./mermaid-shared";
import type { PreviewThemeTemplate } from "../preview-themes";
import { buildMarkdownComponents } from "./markdown-components";
import { PREVIEW_HTML_SANITIZE_SCHEMA } from "./markdown-sanitize";
import { remarkBlockAnchorPlugin } from "./markdown-utils";
import { remarkReferenceFootnotePlugin } from "./remark-reference-footnotes";
import { remarkStyledSpanContainerPlugin } from "./remark-styled-span-container";
import type { PreviewLinkRenderMode, TocItem } from "./types";

interface CreateMarkdownComponentsInput {
  activePreviewTheme: PreviewThemeTemplate;
  tocItems: TocItem[];
  onTocNavigate: (item: TocItem) => void;
  requestOrigin?: string;
  preRenderedMermaidMap?: ReadonlyMap<string, MermaidRenderResult>;
  includeMermaidSourcePayload?: boolean;
}

interface CreateSharedRemarkPluginsOptions {
  includeBlockAnchors?: boolean;
}

// createSharedRemarkPlugins 统一产出 markdown -> mdast 阶段插件链路。
export function createSharedRemarkPlugins(
  linkRenderMode: PreviewLinkRenderMode,
  options: CreateSharedRemarkPluginsOptions = {}
) {
  const referenceFootnotePlugin: [
    typeof remarkReferenceFootnotePlugin,
    { mode: PreviewLinkRenderMode }
  ] = [remarkReferenceFootnotePlugin, { mode: linkRenderMode }];
  const includeBlockAnchors = options.includeBlockAnchors ?? true;

  return includeBlockAnchors
    ? [
        remarkGfm,
        remarkMath,
        remarkStyledSpanContainerPlugin,
        referenceFootnotePlugin,
        remarkBlockAnchorPlugin
      ]
    : [remarkGfm, remarkMath, remarkStyledSpanContainerPlugin, referenceFootnotePlugin];
}

// createSharedRehypePlugins 统一产出 hast -> react 阶段插件链路。
export function createSharedRehypePlugins() {
  const sanitizePlugin: [typeof rehypeSanitize, typeof PREVIEW_HTML_SANITIZE_SCHEMA] = [
    rehypeSanitize,
    PREVIEW_HTML_SANITIZE_SCHEMA
  ];

  return [rehypeRaw, sanitizePlugin, rehypeKatex];
}

// createSharedMarkdownComponents 统一产出 Markdown 渲染组件映射。
export function createSharedMarkdownComponents(input: CreateMarkdownComponentsInput): Components {
  return buildMarkdownComponents({
    activePreviewTheme: input.activePreviewTheme,
    tocItems: input.tocItems,
    onTocNavigate: input.onTocNavigate,
    requestOrigin: input.requestOrigin,
    preRenderedMermaidMap: input.preRenderedMermaidMap,
    includeMermaidSourcePayload: input.includeMermaidSourcePayload
  });
}
