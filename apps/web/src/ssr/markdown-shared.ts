import MarkdownIt from "markdown-it";
import { createElement } from "react";
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
const READER_EXTERNAL_LINK_REL_TOKENS = ["noopener", "noreferrer", "nofollow"] as const;
const READER_HTTP_PROTOCOLS = new Set(["http:", "https:"]);

interface BuildReaderMarkdownRendererResult {
  remarkPlugins: ReturnType<typeof createSharedRemarkPlugins>;
  rehypePlugins: ReturnType<typeof createSharedRehypePlugins>;
  components: Components;
}

interface BuildReaderMarkdownRendererOptions {
  requestOrigin?: string;
}

function normalizeRequestOrigin(value: string | undefined): string {
  const normalized = (value ?? "").trim();
  if (!normalized) {
    return "";
  }
  try {
    const parsedURL = new URL(normalized);
    if (!READER_HTTP_PROTOCOLS.has(parsedURL.protocol)) {
      return "";
    }
    return parsedURL.origin;
  } catch {
    return "";
  }
}

function shouldAppendExternalLinkRel(rawHref: string, requestOrigin: string): boolean {
  const normalizedHref = rawHref.trim();
  if (!normalizedHref || normalizedHref.startsWith("#") || !requestOrigin) {
    return false;
  }
  try {
    const resolvedURL = new URL(normalizedHref, requestOrigin);
    if (!READER_HTTP_PROTOCOLS.has(resolvedURL.protocol)) {
      return false;
    }
    return resolvedURL.origin !== requestOrigin;
  } catch {
    return false;
  }
}

function mergeExternalRelTokens(rawRel: string): string {
  const tokenSet = new Set<string>();
  for (const token of rawRel.split(/\s+/)) {
    const normalizedToken = token.trim().toLowerCase();
    if (normalizedToken) {
      tokenSet.add(normalizedToken);
    }
  }
  for (const requiredToken of READER_EXTERNAL_LINK_REL_TOKENS) {
    tokenSet.add(requiredToken);
  }
  return Array.from(tokenSet).join(" ");
}

// buildReaderMarkdownRenderer 产出阅读页 SSR 与编辑器预览共享的 markdown 渲染配置。
export function buildReaderMarkdownRenderer(
  content: string,
  activePreviewTheme: PreviewThemeTemplate,
  options: BuildReaderMarkdownRendererOptions = {}
): BuildReaderMarkdownRendererResult {
  const tocItems = parseTocFromMarkdown(content, markdownTextParser).items;
  const requestOrigin = normalizeRequestOrigin(options.requestOrigin);
  const baseComponents = createSharedMarkdownComponents({
    activePreviewTheme,
    tocItems,
    onTocNavigate: () => {}
  });
  const baseAnchorRenderer = baseComponents.a;
  const renderAnchorElement = (anchorProps: Record<string, unknown>) => {
    if (baseAnchorRenderer) {
      return createElement(baseAnchorRenderer as never, anchorProps as never);
    }
    const { node: _node, ...nativeAnchorProps } = anchorProps;
    return createElement("a", nativeAnchorProps);
  };
  const components: Components = {
    ...baseComponents,
    a: (anchorProps) => {
      const href = typeof anchorProps.href === "string" ? anchorProps.href : "";
      if (!shouldAppendExternalLinkRel(href, requestOrigin)) {
        return renderAnchorElement(anchorProps as Record<string, unknown>);
      }
      const rel = mergeExternalRelTokens(
        typeof anchorProps.rel === "string" ? anchorProps.rel : ""
      );
      return renderAnchorElement({
        ...(anchorProps as Record<string, unknown>),
        rel
      });
    }
  };

  return {
    remarkPlugins: createSharedRemarkPlugins("link", { includeBlockAnchors: false }),
    rehypePlugins: createSharedRehypePlugins(),
    components
  };
}
