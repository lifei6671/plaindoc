import type { ReactNode } from "react";
import { isValidElement } from "react";
import MarkdownIt from "markdown-it";
import {
  BLOCK_NODE_TYPES,
  PREVIEW_BODY_SELECTOR,
  TOC_MARKER_PATTERN,
  TOC_MAX_DEPTH
} from "./constants";
import type { MarkdownNode, MarkdownToken, TocItem, TocParseResult } from "./types";

// 提取代码语言名（language-xxx）。
export function resolveCodeLanguage(className: string | undefined): string {
  if (!className) {
    return "text";
  }
  const matched = /language-([\w-]+)/.exec(className);
  return matched?.[1] ?? "text";
}

// 将 ReactNode 递归还原成纯文本代码。
export function extractCodeText(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") {
    return String(node);
  }
  if (Array.isArray(node)) {
    return node.map((item) => extractCodeText(item)).join("");
  }
  if (isValidElement(node)) {
    const elementChildren = (node.props as { children?: ReactNode }).children;
    return extractCodeText(elementChildren);
  }
  return "";
}

// 从代码节点 props 中抽取锚点属性，供同步滚动映射继续使用。
export function pickAnchorDataAttributes(
  props: Record<string, unknown>
): Record<string, string> {
  const anchors: Record<string, string> = {};
  const sourceLine = props["data-source-line"];
  const sourceOffset = props["data-source-offset"];
  const sourceEndLine = props["data-source-end-line"];
  const sourceEndOffset = props["data-source-end-offset"];
  const anchorIndex = props["data-anchor-index"];
  if (typeof sourceLine === "string") {
    anchors["data-source-line"] = sourceLine;
  }
  if (typeof sourceOffset === "string") {
    anchors["data-source-offset"] = sourceOffset;
  }
  if (typeof sourceEndLine === "string") {
    anchors["data-source-end-line"] = sourceEndLine;
  }
  if (typeof sourceEndOffset === "string") {
    anchors["data-source-end-offset"] = sourceEndOffset;
  }
  if (typeof anchorIndex === "string") {
    anchors["data-anchor-index"] = anchorIndex;
  }
  return anchors;
}

// 将 Markdown 渲染为 HTML 后提取纯文本，去除语法标记影响。
export function extractPlainTextFromMarkdown(markdownContent: string, parser: MarkdownIt): string {
  // 统计字数时忽略 [TOC] 标记，避免语法指令被计入正文。
  const sanitizedMarkdown = markdownContent.replace(/^\s*\[toc\]\s*$/gim, "");
  const renderedHtml = parser.render(sanitizedMarkdown);
  if (typeof DOMParser === "undefined") {
    return sanitizedMarkdown;
  }
  const htmlDocument = new DOMParser().parseFromString(renderedHtml, "text/html");
  return htmlDocument.body.textContent ?? "";
}

// 判断文本是否为 [TOC] 语法标记（大小写不敏感）。
export function isTocMarkerText(rawText: string): boolean {
  return TOC_MARKER_PATTERN.test(rawText.trim());
}

// 从 markdown-it inline token 中提取纯文本，避免目录显示 Markdown 标记。
function extractInlineTextFromToken(token: MarkdownToken): string {
  if (!token.children?.length) {
    return token.content?.trim() ?? "";
  }
  return token.children
    .map((child) => {
      if (child.type === "text" || child.type === "code_inline" || child.type === "emoji") {
        return child.content ?? "";
      }
      if (child.type === "image") {
        return child.content ?? "";
      }
      return "";
    })
    .join("")
    .trim();
}

// 基于 markdown-it token 解析 TOC：提取标题目录并识别 [TOC] 标记。
export function parseTocFromMarkdown(markdownContent: string, parser: MarkdownIt): TocParseResult {
  if (!markdownContent) {
    return {
      items: [],
      hasMarker: false
    };
  }
  const tokens = parser.parse(markdownContent, {}) as MarkdownToken[];
  const items: TocItem[] = [];
  let hasMarker = false;

  for (let index = 0; index < tokens.length; index += 1) {
    const token = tokens[index];
    // 仅在段落上下文中识别 [TOC]，避免误伤代码块和普通文本。
    if (token.type === "inline") {
      const previousToken = tokens[index - 1];
      const nextToken = tokens[index + 1];
      if (
        previousToken?.type === "paragraph_open" &&
        nextToken?.type === "paragraph_close" &&
        isTocMarkerText(token.content ?? "")
      ) {
        hasMarker = true;
      }
    }

    if (token.type !== "heading_open") {
      continue;
    }
    const tag = token.tag ?? "";
    if (!tag.startsWith("h")) {
      continue;
    }
    const level = Number(tag.slice(1));
    if (!Number.isFinite(level) || level < 1 || level > TOC_MAX_DEPTH) {
      continue;
    }
    const inlineToken = tokens[index + 1];
    if (!inlineToken || inlineToken.type !== "inline") {
      continue;
    }
    const text = extractInlineTextFromToken(inlineToken);
    if (!text) {
      continue;
    }
    const sourceLine = Array.isArray(token.map) ? token.map[0] + 1 : null;
    if (!sourceLine || !Number.isFinite(sourceLine)) {
      continue;
    }
    items.push({
      level,
      text,
      sourceLine
    });
  }

  return {
    items,
    hasMarker
  };
}

// 将 TOC 条目滚动定位到预览区目标标题。
export function scrollPreviewToTocItem(previewElement: HTMLElement, item: TocItem): void {
  const selector = `${PREVIEW_BODY_SELECTOR} h${item.level}[data-source-line="${item.sourceLine}"]`;
  const targetHeading = previewElement.querySelector<HTMLElement>(selector);
  if (!targetHeading) {
    return;
  }
  const previewRect = previewElement.getBoundingClientRect();
  const targetTop =
    targetHeading.getBoundingClientRect().top - previewRect.top + previewElement.scrollTop;
  previewElement.scrollTo({ top: targetTop, behavior: "smooth" });
}

// 为 block 节点注入 source offset，供同步滚动映射使用。
export function remarkBlockAnchorPlugin() {
  return (tree: MarkdownNode) => {
    // 额外写入序号，便于调试锚点顺序问题。
    let anchorIndex = 0;

    const walk = (node: MarkdownNode): void => {
      // 仅在 block 节点上打锚点，避免过密且不稳定的 inline 锚点。
      if (BLOCK_NODE_TYPES.has(node.type)) {
        const sourceLine = node.position?.start?.line;
        const sourceOffset = node.position?.start?.offset;
        const sourceEndLine = node.position?.end?.line;
        const sourceEndOffset = node.position?.end?.offset;
        const hasSourceLine = typeof sourceLine === "number" && Number.isFinite(sourceLine);
        const hasSourceOffset = typeof sourceOffset === "number" && Number.isFinite(sourceOffset);
        const hasSourceEndLine = typeof sourceEndLine === "number" && Number.isFinite(sourceEndLine);
        const hasSourceEndOffset =
          typeof sourceEndOffset === "number" && Number.isFinite(sourceEndOffset);
        if (hasSourceLine || hasSourceOffset) {
          if (!node.data) {
            node.data = {};
          }
          const hProperties = (node.data.hProperties ??= {});
          if (hasSourceLine) {
            hProperties["data-source-line"] = String(Math.max(1, Math.floor(sourceLine)));
          }
          if (hasSourceOffset) {
            hProperties["data-source-offset"] = String(Math.max(0, Math.floor(sourceOffset)));
          }
          // 同步记录 block 结束位置信息，便于长公式/长代码块生成区间锚点。
          if (hasSourceEndLine) {
            hProperties["data-source-end-line"] = String(Math.max(1, Math.floor(sourceEndLine)));
          }
          if (hasSourceEndOffset) {
            hProperties["data-source-end-offset"] = String(Math.max(0, Math.floor(sourceEndOffset)));
          }
          hProperties["data-anchor-index"] = String(anchorIndex);
          anchorIndex += 1;
        }
      }

      // 深度优先遍历 AST。
      if (!node.children?.length) {
        return;
      }
      for (const child of node.children) {
        walk(child);
      }
    };

    walk(tree);
  };
}
