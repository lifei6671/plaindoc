interface StyledSpanMarkdownNode {
  type: string;
  value?: string;
  depth?: number;
  children?: StyledSpanMarkdownNode[];
  data?: {
    hName?: string;
    hProperties?: Record<string, unknown>;
  };
  position?: {
    start?: {
      line?: number;
      column?: number;
      offset?: number;
    };
    end?: {
      line?: number;
      column?: number;
      offset?: number;
    };
  };
}

interface SpanOpenTagMeta {
  styleText: string | null;
  classNames: string[];
}

interface CloseSpanExtractResult {
  hasCloseTag: boolean;
  normalizedNode: StyledSpanMarkdownNode | null;
  closeNode: StyledSpanMarkdownNode | null;
}

interface HtmlMarkdownNode extends StyledSpanMarkdownNode {
  value: string;
}

const SPAN_OPEN_TAG_PATTERN = /^<span\b([^>]*)>$/i;
const SPAN_CLOSE_TAG_PATTERN = /^<\/span\s*>$/i;
const STYLE_UNSAFE_VALUE_PATTERN = /(expression\s*\(|url\s*\(|javascript:|@import)/i;
const CLASS_NAME_TOKEN_PATTERN = /^[A-Za-z0-9_-]+$/;
const STYLED_SPAN_CONTAINER_CLASS_NAME = "plaindoc-styled-span-container";
const ALLOWED_INLINE_STYLE_PROPERTIES = new Set([
  "display",
  "color",
  "background",
  "background-color",
  "font-size",
  "font-weight",
  "font-style",
  "line-height",
  "text-align",
  "text-indent",
  "padding",
  "padding-top",
  "padding-right",
  "padding-bottom",
  "padding-left",
  "margin",
  "margin-top",
  "margin-right",
  "margin-bottom",
  "margin-left",
  "border",
  "border-top",
  "border-right",
  "border-bottom",
  "border-left",
  "border-radius",
  "box-sizing",
  "white-space"
]);

// 提取 HTML 标签属性：仅支持单双引号值，满足当前编辑器输入场景。
function readHtmlAttributeValue(rawAttributeText: string, attributeName: string): string | null {
  const attributePattern = new RegExp(
    `(?:^|\\s)${attributeName}\\s*=\\s*(\"([^\"]*)\"|'([^']*)')`,
    "i"
  );
  const matched = rawAttributeText.match(attributePattern);
  if (!matched) {
    return null;
  }
  return (matched[2] ?? matched[3] ?? "").trim() || null;
}

// 过滤 class token，避免注入非法字符影响后续渲染与选择器匹配。
function sanitizeClassNames(rawClassNameText: string | null): string[] {
  if (!rawClassNameText) {
    return [];
  }
  return rawClassNameText
    .split(/\s+/)
    .map((token) => token.trim())
    .filter((token) => token.length > 0 && CLASS_NAME_TOKEN_PATTERN.test(token));
}

// 过滤 style 声明：仅放行常用排版属性，并拦截 url()/expression 等危险值。
function sanitizeInlineStyleText(rawStyleText: string | null): string | null {
  if (!rawStyleText) {
    return null;
  }

  const sanitizedDeclarations: string[] = [];
  rawStyleText.split(";").forEach((declarationText) => {
    const normalizedDeclarationText = declarationText.trim();
    if (!normalizedDeclarationText) {
      return;
    }
    const separatorIndex = normalizedDeclarationText.indexOf(":");
    if (separatorIndex <= 0) {
      return;
    }
    const propertyName = normalizedDeclarationText.slice(0, separatorIndex).trim().toLowerCase();
    const propertyValue = normalizedDeclarationText.slice(separatorIndex + 1).trim();
    if (!propertyValue || !ALLOWED_INLINE_STYLE_PROPERTIES.has(propertyName)) {
      return;
    }
    if (STYLE_UNSAFE_VALUE_PATTERN.test(propertyValue) || /[<>]/.test(propertyValue)) {
      return;
    }
    sanitizedDeclarations.push(`${propertyName}: ${propertyValue}`);
  });

  return sanitizedDeclarations.length ? sanitizedDeclarations.join("; ") : null;
}

function isHtmlNode(node: StyledSpanMarkdownNode): node is HtmlMarkdownNode {
  return node.type === "html" && typeof node.value === "string";
}

function isSpanCloseTagNode(node: StyledSpanMarkdownNode): boolean {
  return isHtmlNode(node) && SPAN_CLOSE_TAG_PATTERN.test(node.value.trim());
}

// 识别 `<span ...>` 开标签，并提取受控 style/class 元信息。
function parseSpanOpenTagNode(node: StyledSpanMarkdownNode): SpanOpenTagMeta | null {
  if (!isHtmlNode(node)) {
    return null;
  }
  const normalizedTagText = node.value.trim();
  if (normalizedTagText.endsWith("/>") || SPAN_CLOSE_TAG_PATTERN.test(normalizedTagText)) {
    return null;
  }
  const matched = normalizedTagText.match(SPAN_OPEN_TAG_PATTERN);
  if (!matched) {
    return null;
  }

  const rawAttributeText = matched[1] ?? "";
  const rawClassNameText =
    readHtmlAttributeValue(rawAttributeText, "class") ??
    readHtmlAttributeValue(rawAttributeText, "className");
  return {
    classNames: sanitizeClassNames(rawClassNameText),
    styleText: sanitizeInlineStyleText(readHtmlAttributeValue(rawAttributeText, "style"))
  };
}

// 构造“块级容器”节点：统一输出 div，确保内部列表/段落在 HTML 结构上合法。
function createStyledContainerNode(
  openTagMeta: SpanOpenTagMeta,
  children: StyledSpanMarkdownNode[],
  openTagNode: StyledSpanMarkdownNode,
  closeTagNode: StyledSpanMarkdownNode
): StyledSpanMarkdownNode {
  const hProperties: Record<string, unknown> = {};
  const containerClassNames = [STYLED_SPAN_CONTAINER_CLASS_NAME, ...openTagMeta.classNames];
  hProperties.className = containerClassNames.join(" ");
  if (openTagMeta.styleText) {
    hProperties.style = openTagMeta.styleText;
  }

  const containerNode: StyledSpanMarkdownNode = {
    type: "plaindocStyledSpanContainer",
    data: {
      hName: "div",
      hProperties
    },
    children
  };
  if (openTagNode.position || closeTagNode.position) {
    containerNode.position = {
      start: openTagNode.position?.start,
      end: closeTagNode.position?.end ?? openTagNode.position?.end
    };
  }
  return containerNode;
}

// 在同级节点里查找与开标签匹配的 `</span>`，支持简单嵌套深度计数。
function findMatchingCloseSpanIndex(children: StyledSpanMarkdownNode[], startIndex: number): number {
  let depth = 1;
  for (let index = startIndex; index < children.length; index += 1) {
    const currentNode = children[index];
    if (parseSpanOpenTagNode(currentNode)) {
      depth += 1;
      continue;
    }
    if (isSpanCloseTagNode(currentNode)) {
      depth -= 1;
      if (depth === 0) {
        return index;
      }
    }
  }
  return -1;
}

// 处理“段落包裹的 span 容器”：如 `<span ...>文本 **强调** </span>` 的单行写法。
function rewriteParagraphWrappedStyledSpan(
  paragraphNode: StyledSpanMarkdownNode
): StyledSpanMarkdownNode | null {
  if (paragraphNode.type !== "paragraph" || !Array.isArray(paragraphNode.children) || !paragraphNode.children.length) {
    return null;
  }

  const paragraphChildren = paragraphNode.children;
  const openTagNode = paragraphChildren[0];
  const openTagMeta = parseSpanOpenTagNode(openTagNode);
  if (!openTagMeta) {
    return null;
  }

  const closeIndex = findMatchingCloseSpanIndex(paragraphChildren, 1);
  if (closeIndex !== paragraphChildren.length - 1) {
    return null;
  }

  const closeTagNode = paragraphChildren[closeIndex];
  const innerNodes = paragraphChildren.slice(1, closeIndex);
  return createStyledContainerNode(openTagMeta, innerNodes, openTagNode, closeTagNode);
}

// 处理 setext 误判：`<span ...>...</span>` 紧跟 `---` 时会被 markdown 解析成 h2，这里还原为容器 + 分隔线。
function rewriteSetextHeadingWrappedStyledSpan(
  headingNode: StyledSpanMarkdownNode
): StyledSpanMarkdownNode[] | null {
  if (
    headingNode.type !== "heading" ||
    headingNode.depth !== 2 ||
    !Array.isArray(headingNode.children) ||
    !headingNode.children.length
  ) {
    return null;
  }

  // 仅处理 setext（跨两行）场景，避免影响用户主动写的 `##` 标题。
  const headingStartLine = headingNode.position?.start?.line;
  const headingEndLine = headingNode.position?.end?.line;
  if (
    typeof headingStartLine !== "number" ||
    typeof headingEndLine !== "number" ||
    headingEndLine <= headingStartLine
  ) {
    return null;
  }

  const headingChildren = headingNode.children;
  const openTagNode = headingChildren[0];
  const openTagMeta = parseSpanOpenTagNode(openTagNode);
  if (!openTagMeta) {
    return null;
  }

  const closeIndex = findMatchingCloseSpanIndex(headingChildren, 1);
  if (closeIndex !== headingChildren.length - 1) {
    return null;
  }

  const closeTagNode = headingChildren[closeIndex];
  const innerNodes = headingChildren.slice(1, closeIndex);
  const containerNode = createStyledContainerNode(openTagMeta, innerNodes, openTagNode, closeTagNode);

  return [
    containerNode,
    {
      type: "thematicBreak"
    }
  ];
}

// 把“节点尾部的 `</span>`”剥离出来，便于跨 blockquote 合并时复用现有 markdown 结构。
function extractTrailingCloseSpanFromNode(node: StyledSpanMarkdownNode): CloseSpanExtractResult {
  if (isSpanCloseTagNode(node)) {
    return {
      hasCloseTag: true,
      normalizedNode: null,
      closeNode: node
    };
  }
  if (!Array.isArray(node.children) || !node.children.length) {
    return {
      hasCloseTag: false,
      normalizedNode: node,
      closeNode: null
    };
  }

  const trailingChildNode = node.children[node.children.length - 1];
  if (!isSpanCloseTagNode(trailingChildNode)) {
    return {
      hasCloseTag: false,
      normalizedNode: node,
      closeNode: null
    };
  }

  const remainingChildren = node.children.slice(0, -1);
  if (!remainingChildren.length && node.type === "paragraph") {
    return {
      hasCloseTag: true,
      normalizedNode: null,
      closeNode: trailingChildNode
    };
  }

  return {
    hasCloseTag: true,
    normalizedNode: {
      ...node,
      children: remainingChildren
    },
    closeNode: trailingChildNode
  };
}

// 修复 `> <span ...>` 后续行漏写 `>` 的场景：把后续块重新并回同一个 blockquote。
function mergeCrossBlockquoteStyledSpan(
  siblingNodes: StyledSpanMarkdownNode[],
  blockquoteIndex: number
): boolean {
  const blockquoteNode = siblingNodes[blockquoteIndex];
  if (blockquoteNode.type !== "blockquote" || !Array.isArray(blockquoteNode.children) || !blockquoteNode.children.length) {
    return false;
  }

  const trailingNode = blockquoteNode.children[blockquoteNode.children.length - 1];
  const openTagMeta = parseSpanOpenTagNode(trailingNode);
  if (!openTagMeta) {
    return false;
  }

  const collectedInnerNodes: StyledSpanMarkdownNode[] = [];
  let consumedEndIndex = -1;
  let closeNode: StyledSpanMarkdownNode | null = null;

  for (let index = blockquoteIndex + 1; index < siblingNodes.length; index += 1) {
    const siblingNode = siblingNodes[index];
    const extractedResult = extractTrailingCloseSpanFromNode(siblingNode);
    if (extractedResult.hasCloseTag) {
      if (extractedResult.normalizedNode) {
        collectedInnerNodes.push(extractedResult.normalizedNode);
      }
      consumedEndIndex = index;
      closeNode = extractedResult.closeNode;
      break;
    }
    collectedInnerNodes.push(siblingNode);
  }

  if (consumedEndIndex < 0 || !closeNode) {
    return false;
  }

  const preservedChildren = blockquoteNode.children.slice(0, -1);
  preservedChildren.push(createStyledContainerNode(openTagMeta, collectedInnerNodes, trailingNode, closeNode));
  blockquoteNode.children = preservedChildren;
  if (blockquoteNode.position?.start && closeNode.position?.end) {
    blockquoteNode.position = {
      ...blockquoteNode.position,
      end: closeNode.position.end
    };
  }

  // 删除已并入 blockquote 的后续兄弟节点（含闭标签所在节点）。
  siblingNodes.splice(blockquoteIndex + 1, consumedEndIndex - blockquoteIndex);
  return true;
}

// 递归重写 `<span style="..."> ... </span>` 的跨块容器语法，保证渲染结构稳定。
function rewriteStyledSpanContainers(parentNode: StyledSpanMarkdownNode): void {
  if (!Array.isArray(parentNode.children) || !parentNode.children.length) {
    return;
  }

  const siblingNodes = parentNode.children;
  for (let index = 0; index < siblingNodes.length; ) {
    const currentNode = siblingNodes[index];

    // 修复 setext 标题误判：把“样式 span + ---”恢复为普通容器块。
    if (currentNode.type === "heading") {
      const rewrittenHeadingNodes = rewriteSetextHeadingWrappedStyledSpan(currentNode);
      if (rewrittenHeadingNodes) {
        siblingNodes.splice(index, 1, ...rewrittenHeadingNodes);
        rewrittenHeadingNodes.forEach((rewrittenNode) => {
          if (rewrittenNode.type !== "thematicBreak") {
            rewriteStyledSpanContainers(rewrittenNode);
          }
        });
        index += rewrittenHeadingNodes.length;
        continue;
      }
    }

    // 把段落内“首尾 span 容器”提升为块级容器，避免被 paragraph 结构裹挟导致渲染不稳定。
    if (currentNode.type === "paragraph") {
      const rewrittenContainerNode = rewriteParagraphWrappedStyledSpan(currentNode);
      if (rewrittenContainerNode) {
        siblingNodes.splice(index, 1, rewrittenContainerNode);
        rewriteStyledSpanContainers(rewrittenContainerNode);
        index += 1;
        continue;
      }
    }

    if (currentNode.type === "blockquote" && mergeCrossBlockquoteStyledSpan(siblingNodes, index)) {
      rewriteStyledSpanContainers(siblingNodes[index]);
      index += 1;
      continue;
    }

    // 仅在“非段落层级”收敛 span 容器，避免误伤行内 `<span>文本</span>` 语义。
    const openTagMeta = parentNode.type === "paragraph" ? null : parseSpanOpenTagNode(currentNode);
    if (openTagMeta) {
      const closeIndex = findMatchingCloseSpanIndex(siblingNodes, index + 1);
      if (closeIndex >= 0) {
        const closeNode = siblingNodes[closeIndex];
        const innerNodes = siblingNodes.slice(index + 1, closeIndex);
        const containerNode = createStyledContainerNode(openTagMeta, innerNodes, currentNode, closeNode);
        siblingNodes.splice(index, closeIndex - index + 1, containerNode);
        rewriteStyledSpanContainers(containerNode);
        index += 1;
        continue;
      }
    }

    rewriteStyledSpanContainers(currentNode);
    index += 1;
  }
}

// remark 插件入口：把“样式 span 跨块包裹”统一规整为可渲染的容器节点。
export function remarkStyledSpanContainerPlugin() {
  return (tree: StyledSpanMarkdownNode) => {
    rewriteStyledSpanContainers(tree);
  };
}
