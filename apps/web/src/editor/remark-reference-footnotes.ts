import type { PreviewLinkRenderMode } from "./types";

interface FootnoteMarkdownNode {
  type: string;
  value?: string;
  url?: string;
  title?: string | null;
  identifier?: string;
  label?: string;
  alt?: string;
  data?: {
    hProperties?: Record<string, unknown>;
  };
  children?: FootnoteMarkdownNode[];
}

interface DefinitionMeta {
  url: string;
  title: string | null;
  label: string;
}

interface ExternalFootnoteEntry {
  index: number;
  url: string;
  title: string | null;
}

interface RemarkReferenceFootnotePluginOptions {
  mode?: PreviewLinkRenderMode;
}

const EXTERNAL_URL_PATTERN = /^(https?:|mailto:|tel:)/i;

// 判断是否为外链：仅处理 http/https/mailto/tel 与协议相对地址。
function isExternalUrl(rawUrl: string): boolean {
  const normalizedUrl = rawUrl.trim();
  if (!normalizedUrl || normalizedUrl.startsWith("#")) {
    return false;
  }
  if (normalizedUrl.startsWith("//")) {
    return true;
  }
  return EXTERNAL_URL_PATTERN.test(normalizedUrl);
}

// 归一化引用标识：与 CommonMark 引用匹配规则保持一致（大小写不敏感，连续空白折叠）。
function normalizeReferenceIdentifier(identifier: string): string {
  return identifier.trim().replace(/\s+/g, " ").toLowerCase();
}

// 递归提取节点可见文本，作为“脚注模式”下外链的正文展示文本。
function extractNodePlainText(node: FootnoteMarkdownNode): string {
  if (node.type === "text" || node.type === "inlineCode") {
    return node.value ?? "";
  }
  if (node.type === "image") {
    return node.alt ?? "";
  }
  if (!node.children?.length) {
    return "";
  }
  return node.children.map((childNode) => extractNodePlainText(childNode)).join("");
}

// 转义 HTML 特殊字符，避免 URL 和标题被误解析为标签。
function escapeHtml(rawText: string): string {
  return rawText
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

// 生成正文角标节点：通过 footnote-ref 类复用主题颜色，并保持右上角排版语义。
function createInlineFootnoteMarkerNode(index: number): FootnoteMarkdownNode {
  return {
    type: "html",
    value: `<span class="footnote-ref preview-link-footnote-ref">[${index}]</span>`
  };
}

// 构造文末脚注段落：输出 `[n] url "title"` 的可读结构（纯文本 URL，不是可点击链接）。
function createFootnoteLineNode(entry: ExternalFootnoteEntry): FootnoteMarkdownNode {
  const lineParts: string[] = [
    `<span class="footnote-ref">[${entry.index}]</span>`,
    `<span class="footnote-word">${escapeHtml(entry.url)}</span>`
  ];
  if (entry.title?.trim()) {
    lineParts.push(`<span class="reference-footnote-note">"${escapeHtml(entry.title.trim())}"</span>`);
  }

  return {
    type: "paragraph",
    data: {
      hProperties: {
        className: "reference-footnote-line"
      }
    },
    children: [
      {
        type: "html",
        value: lineParts.join(" ")
      }
    ]
  };
}

// 收集文档中的 definition 节点，便于 linkReference 在脚注模式下解析真实 URL。
function collectDefinitionMap(rootNode: FootnoteMarkdownNode): Map<string, DefinitionMeta> {
  const definitionMap = new Map<string, DefinitionMeta>();
  const walk = (node: FootnoteMarkdownNode): void => {
    if (node.type === "definition" && typeof node.identifier === "string" && typeof node.url === "string") {
      const normalizedIdentifier = normalizeReferenceIdentifier(node.identifier);
      if (normalizedIdentifier && !definitionMap.has(normalizedIdentifier)) {
        definitionMap.set(normalizedIdentifier, {
          url: node.url,
          title: typeof node.title === "string" ? node.title : null,
          label:
            typeof node.label === "string" && node.label.trim()
              ? node.label.trim()
              : node.identifier
        });
      }
    }
    if (!node.children?.length) {
      return;
    }
    for (const childNode of node.children) {
      walk(childNode);
    }
  };

  walk(rootNode);
  return definitionMap;
}

// 统一分配脚注编号：同一 URL 复用同一编号，避免正文重复外链产生多条脚注。
function ensureExternalFootnoteIndex(
  url: string,
  title: string | null,
  indexByUrl: Map<string, number>,
  footnotes: ExternalFootnoteEntry[]
): number {
  const normalizedUrl = url.trim();
  const existingIndex = indexByUrl.get(normalizedUrl);
  if (existingIndex) {
    if (title?.trim()) {
      const existingEntry = footnotes[existingIndex - 1];
      if (existingEntry && !existingEntry.title) {
        existingEntry.title = title.trim();
      }
    }
    return existingIndex;
  }

  const nextIndex = footnotes.length + 1;
  indexByUrl.set(normalizedUrl, nextIndex);
  footnotes.push({
    index: nextIndex,
    url: normalizedUrl,
    title: title?.trim() || null
  });
  return nextIndex;
}

// 脚注模式插件：外链转“正文角标 + 文末脚注”，链接模式则不改动。
export function remarkReferenceFootnotePlugin(options: RemarkReferenceFootnotePluginOptions = {}) {
  const mode = options.mode ?? "link";

  return (tree: FootnoteMarkdownNode) => {
    if (mode !== "footnote" || !Array.isArray(tree.children) || !tree.children.length) {
      return;
    }

    const definitionMap = collectDefinitionMap(tree);
    const indexByUrl = new Map<string, number>();
    const externalFootnotes: ExternalFootnoteEntry[] = [];

    const transformChildren = (parentNode: FootnoteMarkdownNode): void => {
      if (!parentNode.children?.length) {
        return;
      }

      const transformedChildren: FootnoteMarkdownNode[] = [];
      for (const childNode of parentNode.children) {
        if (childNode.type === "link" && typeof childNode.url === "string" && isExternalUrl(childNode.url)) {
          const visibleText = extractNodePlainText(childNode).trim() || childNode.url.trim();
          const footnoteIndex = ensureExternalFootnoteIndex(
            childNode.url,
            typeof childNode.title === "string" ? childNode.title : null,
            indexByUrl,
            externalFootnotes
          );
          transformedChildren.push({
            type: "text",
            value: visibleText
          });
          transformedChildren.push(createInlineFootnoteMarkerNode(footnoteIndex));
          continue;
        }

        if (childNode.type === "linkReference" && typeof childNode.identifier === "string") {
          const normalizedIdentifier = normalizeReferenceIdentifier(childNode.identifier);
          const definition = definitionMap.get(normalizedIdentifier);
          if (definition && isExternalUrl(definition.url)) {
            const visibleText = extractNodePlainText(childNode).trim() || definition.label || definition.url;
            const footnoteIndex = ensureExternalFootnoteIndex(
              definition.url,
              definition.title,
              indexByUrl,
              externalFootnotes
            );
            transformedChildren.push({
              type: "text",
              value: visibleText
            });
            transformedChildren.push(createInlineFootnoteMarkerNode(footnoteIndex));
            continue;
          }
        }

        transformChildren(childNode);
        transformedChildren.push(childNode);
      }
      parentNode.children = transformedChildren;
    };

    transformChildren(tree);

    if (!externalFootnotes.length) {
      return;
    }

    tree.children.push(
      {
        type: "thematicBreak"
      },
      {
        type: "paragraph",
        data: {
          hProperties: {
            className: "reference-footnote-title"
          }
        },
        children: [
          {
            type: "text",
            value: "参考链接"
          }
        ]
      }
    );

    externalFootnotes.forEach((footnoteEntry) => {
      tree.children?.push(createFootnoteLineNode(footnoteEntry));
    });
  };
}

