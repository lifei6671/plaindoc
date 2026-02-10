import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import { languages } from "@codemirror/language-data";
import { EditorView } from "@codemirror/view";
import CodeMirror from "@uiw/react-codemirror";
import { AlertCircle, CheckCircle2, LoaderCircle } from "lucide-react";
import MarkdownIt from "markdown-it";
// KaTeX 样式：用于行内/块级公式排版。
import "katex/dist/katex.min.css";
// KaTeX mhchem 扩展：支持 `\\ce{}` 化学公式语法。
import "katex/contrib/mhchem";
import {
  Children,
  isValidElement,
  memo,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type ComponentPropsWithoutRef,
  type ReactNode
} from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import rehypeKatex from "rehype-katex";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import { ConflictError, getDataGateway, type TreeNode } from "./data-access";
import {
  PREVIEW_SYNTAX_THEMES,
  PREVIEW_THEME_TEMPLATES,
  resolvePreviewTheme,
  type PreviewThemeTemplate
} from "./preview-themes";

// 扩展 window 类型，支持外部注入预览样式字符串。
declare global {
  interface Window {
    __PLAINDOC_PREVIEW_STYLE__?: string;
  }
}

// 编辑器初始化时使用的兜底内容。
const fallbackContent = `# PlainDoc

加载中...
`;

// 文档保存状态机。
type SaveStatus = "loading" | "ready" | "saving" | "saved" | "conflict" | "error";
// 状态栏保存图标类型。
type SaveIndicatorVariant = "unsaved" | "saving" | "saved";
// 当前滚动事件的来源。
type ScrollSource = "editor" | "preview";

// 编辑区与预览区的单个锚点映射。
interface ScrollAnchor {
  editorY: number;
  previewY: number;
}

// 单向映射锚点（sourceY -> targetY）。
interface DirectionAnchor {
  sourceY: number;
  targetY: number;
}

// TOC 单条目录项信息。
interface TocItem {
  level: number;
  text: string;
  sourceLine: number;
}

// TOC 解析结果：同时返回标题目录与是否存在 [TOC] 语法标记。
interface TocParseResult {
  items: TocItem[];
  hasMarker: boolean;
}

// 仅包含本模块需要访问的 Markdown AST 字段。
interface MarkdownNode {
  type: string;
  position?: {
    start?: {
      line?: number;
      offset?: number;
    };
    end?: {
      line?: number;
      offset?: number;
    };
  };
  data?: {
    hProperties?: Record<string, unknown>;
  };
  children?: MarkdownNode[];
}

// 仅包含 TOC 解析所需的 markdown-it token 字段。
interface MarkdownToken {
  type: string;
  tag?: string;
  map?: number[] | null;
  content?: string;
  children?: MarkdownToken[] | null;
}

// 参与锚点映射的 block 级节点类型。
const BLOCK_NODE_TYPES = new Set([
  "paragraph",
  "heading",
  "blockquote",
  "code",
  // 块级公式同样需要参与锚点映射，避免长公式场景同步漂移。
  "math",
  "table",
  "thematicBreak",
  "html"
]);

// 预览区锚点节点选择器：仅采集 remark 注入的 block 级锚点，避免子节点噪声干扰。
const BLOCK_ANCHOR_SELECTOR = "[data-anchor-index]";
// TOC 最大显示层级，默认覆盖 h1~h6 标题。
const TOC_MAX_DEPTH = 6;
// TOC 语法标记匹配规则，仅匹配完整段落的 [TOC]。
const TOC_MARKER_PATTERN = /^\[toc\]$/i;
// 同步滚动单帧最大步进，避免 TOC 高度失配时出现“瞬移”。
const MAX_SYNC_STEP_PER_FRAME = 120;
// 同步收敛阈值，小于该值直接视为对齐完成。
const SYNC_SETTLE_THRESHOLD = 0.5;

// 预览区固定根节点 ID，供外部样式覆盖时稳定选择。
const PREVIEW_PANE_ID = "plaindoc-preview-pane";
// 预览区固定根节点类名，便于 class 级样式覆盖。
const PREVIEW_PANE_CLASS = "plaindoc-preview-pane";
// 预览区主题类名前缀，实际类名为 `plaindoc-preview-pane--{themeId}`。
const PREVIEW_PANE_THEME_CLASS_PREFIX = "plaindoc-preview-pane--";
// 默认主题 ID。
const DEFAULT_PREVIEW_THEME_ID = "default";
// 预览内容容器类名，外部样式可直接作用于该容器。
const PREVIEW_BODY_CLASS = "plaindoc-preview-body";
// 预览内容容器查询选择器（DOM 观测使用）。
const PREVIEW_BODY_SELECTOR = `.${PREVIEW_BODY_CLASS}`;
// 本地持久化自定义样式的键名。
const PREVIEW_CUSTOM_STYLE_STORAGE_KEY = "plaindoc.preview.custom-style";
// 本地持久化主题模板的键名。
const PREVIEW_THEME_STORAGE_KEY = "plaindoc.preview.theme-template";
// 外部通知预览样式变更的自定义事件名。
const PREVIEW_CUSTOM_STYLE_EVENT = "plaindoc:preview-style-change";

// 提取代码语言名（language-xxx）。
function resolveCodeLanguage(className: string | undefined): string {
  if (!className) {
    return "text";
  }
  const matched = /language-([\w-]+)/.exec(className);
  return matched?.[1] ?? "text";
}

// 将 ReactNode 递归还原成纯文本代码。
function extractCodeText(node: ReactNode): string {
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
function pickAnchorDataAttributes(props: Record<string, unknown>): Record<string, string> {
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

// 规范化外部传入的样式文本，统一裁剪空白并兜底为空字符串。
function normalizePreviewStyleText(styleText: unknown): string {
  if (typeof styleText !== "string") {
    return "";
  }
  return styleText.trim();
}

// 根据主题 ID 生成预览容器类名。
function getPreviewThemeClassName(themeId: string): string {
  return `${PREVIEW_PANE_THEME_CLASS_PREFIX}${themeId}`;
}

// 将主题变量序列化为 style 标签文本，便于动态注入。
function buildPreviewThemeStyleText(theme: PreviewThemeTemplate): string {
  const declarations = Object.entries(theme.variables)
    .map(([variableName, variableValue]) => `  ${variableName}: ${variableValue};`)
    .join("\n");
  if (!declarations) {
    return "";
  }
  const themeSelector = `#${PREVIEW_PANE_ID}.${getPreviewThemeClassName(theme.id)}`;
  return `${themeSelector} {\n${declarations}\n}`;
}

// 为 block 节点注入 source offset，供同步滚动映射使用。
function remarkBlockAnchorPlugin() {
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

// 数值钳制，避免 scrollTop 越界。
function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

// 获取元素可滚动高度（scrollHeight - clientHeight）。
function getMaxScrollable(element: HTMLElement): number {
  return Math.max(0, element.scrollHeight - element.clientHeight);
}

// 构建单向映射锚点表：同一 sourceY 取最大 targetY，并保证 target 单调不减。
function buildDirectionAnchors(
  points: DirectionAnchor[],
  sourceMaxScrollable: number,
  targetMaxScrollable: number
): DirectionAnchor[] {
  const sanitizedPoints = points
    .filter(
      (point) =>
        Number.isFinite(point.sourceY) &&
        Number.isFinite(point.targetY) &&
        point.sourceY >= 0 &&
        point.targetY >= 0
    )
    .map((point) => ({
      sourceY: clamp(point.sourceY, 0, sourceMaxScrollable),
      targetY: clamp(point.targetY, 0, targetMaxScrollable)
    }));

  // 头尾边界锚点保证全区间可映射。
  sanitizedPoints.push({ sourceY: 0, targetY: 0 });
  sanitizedPoints.push({ sourceY: sourceMaxScrollable, targetY: targetMaxScrollable });
  sanitizedPoints.sort((left, right) => left.sourceY - right.sourceY || left.targetY - right.targetY);

  const groupedAnchors: DirectionAnchor[] = [];
  let index = 0;
  while (index < sanitizedPoints.length) {
    const sourceY = sanitizedPoints[index].sourceY;
    let targetY = sanitizedPoints[index].targetY;
    index += 1;
    while (index < sanitizedPoints.length && sanitizedPoints[index].sourceY === sourceY) {
      targetY = Math.max(targetY, sanitizedPoints[index].targetY);
      index += 1;
    }
    groupedAnchors.push({ sourceY, targetY });
  }

  // target 轴做前缀最大化，确保单调，防止插值反向。
  for (let anchorIndex = 1; anchorIndex < groupedAnchors.length; anchorIndex += 1) {
    if (groupedAnchors[anchorIndex].targetY < groupedAnchors[anchorIndex - 1].targetY) {
      groupedAnchors[anchorIndex].targetY = groupedAnchors[anchorIndex - 1].targetY;
    }
  }

  // 固定顶部边界：source=0 必须严格映射到 target=0，避免首屏出现“还能再向上滚一点”的错位。
  const topBoundaryAnchor = groupedAnchors.find((anchor) => anchor.sourceY === 0);
  if (topBoundaryAnchor) {
    topBoundaryAnchor.targetY = 0;
  } else {
    groupedAnchors.unshift({ sourceY: 0, targetY: 0 });
  }

  // 固定底部边界：source=max 必须映射到 target=max，避免尾部无法对齐到底部。
  const bottomBoundaryAnchor = groupedAnchors.find(
    (anchor) => anchor.sourceY === sourceMaxScrollable
  );
  if (bottomBoundaryAnchor) {
    bottomBoundaryAnchor.targetY = targetMaxScrollable;
  } else {
    groupedAnchors.push({
      sourceY: sourceMaxScrollable,
      targetY: targetMaxScrollable
    });
  }

  // 重新按 source 排序并再做一次单调修正，保证插值阶段始终可用。
  groupedAnchors.sort((left, right) => left.sourceY - right.sourceY || left.targetY - right.targetY);
  for (let anchorIndex = 1; anchorIndex < groupedAnchors.length; anchorIndex += 1) {
    if (groupedAnchors[anchorIndex].targetY < groupedAnchors[anchorIndex - 1].targetY) {
      groupedAnchors[anchorIndex].targetY = groupedAnchors[anchorIndex - 1].targetY;
    }
  }

  return groupedAnchors;
}

// 在文档树中找到首个文档节点。
function findFirstDocId(nodes: TreeNode[]): string | null {
  for (const node of nodes) {
    if (node.type === "doc") {
      return node.id;
    }
    const childDocId = findFirstDocId(node.children);
    if (childDocId) {
      return childDocId;
    }
  }
  return null;
}

// 将 unknown 错误转换为可展示文案。
function formatError(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return "未知错误";
}

// 将 Markdown 渲染为 HTML 后提取纯文本，去除语法标记影响。
function extractPlainTextFromMarkdown(markdownContent: string, parser: MarkdownIt): string {
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
function isTocMarkerText(rawText: string): boolean {
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
function parseTocFromMarkdown(markdownContent: string, parser: MarkdownIt): TocParseResult {
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
function scrollPreviewToTocItem(previewElement: HTMLElement, item: TocItem): void {
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

// 将 ISO 时间格式化为“时:分:秒”。
function formatSavedTime(isoTime: string | null): string {
  if (!isoTime) {
    return "未保存";
  }
  const parsed = new Date(isoTime);
  if (Number.isNaN(parsed.getTime())) {
    return "未保存";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    hour12: false,
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit"
  }).format(parsed);
}

// 将保存状态映射为状态栏图标的展示类型。
function resolveSaveIndicatorVariant(saveStatus: SaveStatus): SaveIndicatorVariant {
  if (saveStatus === "saved") {
    return "saved";
  }
  if (saveStatus === "saving" || saveStatus === "loading") {
    return "saving";
  }
  return "unsaved";
}

// 抽屉样式键值对：用于统一展示当前生效样式。
interface StyleDetailEntry {
  property: string;
  value: string;
}

// 样式详情抽屉入参：由父组件传入当前主题与外部覆盖样式。
interface StyleDetailsDrawerProps {
  theme: PreviewThemeTemplate | null;
  customPreviewStyleText: string;
  onClose: () => void;
}

// 将驼峰样式键转换为 kebab-case，便于用户直观查看 CSS 属性名。
function toKebabCaseStyleProperty(styleProperty: string): string {
  if (!styleProperty) {
    return styleProperty;
  }
  if (styleProperty.startsWith("--")) {
    return styleProperty;
  }
  return styleProperty.replace(/[A-Z]/g, (matched) => `-${matched.toLowerCase()}`);
}

// 统一格式化 CSSProperties 的值，便于在抽屉中输出。
function formatStyleDetailValue(styleValue: unknown): string {
  if (typeof styleValue === "string" || typeof styleValue === "number") {
    return String(styleValue);
  }
  if (styleValue === null || styleValue === undefined) {
    return "";
  }
  return String(styleValue);
}

// 将 CSSProperties 转成可展示的键值数组，并过滤空值。
function buildStyleDetailEntries(styleObject: CSSProperties): StyleDetailEntry[] {
  return Object.entries(styleObject as Record<string, unknown>)
    .map(([property, value]) => ({
      property: toKebabCaseStyleProperty(property),
      value: formatStyleDetailValue(value)
    }))
    .filter((entry) => entry.property && entry.value)
    .sort((left, right) => left.property.localeCompare(right.property));
}

// 将样式条目序列化为 CSS declaration 文本。
function buildCssDeclarationsSource(entries: StyleDetailEntry[]): string {
  if (!entries.length) {
    return "  /* 无样式声明 */";
  }
  return entries.map((entry) => `  ${entry.property}: ${entry.value};`).join("\n");
}

// 生成当前主题可复制的 CSS 模板（包含注释说明）。
function buildThemeCssTemplate(theme: PreviewThemeTemplate): string {
  const previewPaneSelector = `#${PREVIEW_PANE_ID}.${getPreviewThemeClassName(theme.id)}`;
  const previewBodySelector = `${previewPaneSelector} .${PREVIEW_BODY_CLASS}`;
  const sortedVariables = Object.entries(theme.variables).sort(([left], [right]) =>
    left.localeCompare(right)
  );
  const variableSource =
    sortedVariables.length > 0
      ? sortedVariables.map(([name, value]) => `  ${name}: ${value};`).join("\n")
      : "  /* 无变量声明 */";
  const codeBlockSource = buildCssDeclarationsSource(buildStyleDetailEntries(theme.codeBlockStyle));
  const codeBlockCodeSource = buildCssDeclarationsSource(
    buildStyleDetailEntries(theme.codeBlockCodeStyle)
  );
  const inlineCodeSource = buildCssDeclarationsSource(buildStyleDetailEntries(theme.inlineCodeStyle));

  return `/* PlainDoc 主题样式模板（可复制后直接修改） 
 * 主题名称：${theme.name}
 * 主题 ID：${theme.id}
 * 语法高亮：${theme.syntaxTheme}
 */

/* 预览区基础变量 */
${previewPaneSelector} {
${variableSource}
}

/* 代码块容器（fenced code -> pre） */
${previewBodySelector} pre {
${codeBlockSource}
}

/* 代码块文本（pre > code） */
${previewBodySelector} pre code {
${codeBlockCodeSource}
}

/* 行内代码（p/li/table 内 code） */
${previewBodySelector} p code,
${previewBodySelector} li code,
${previewBodySelector} table code {
${inlineCodeSource}
}`;
}

// 主题菜单组件入参：由父组件提供当前主题和切换回调。
interface ThemeMenuProps {
  themes: PreviewThemeTemplate[];
  activeThemeId: string;
  onSelectTheme: (themeId: string) => void;
  customPreviewStyleText: string;
}

// 目录菜单组件入参：由父组件提供 TOC 数据和跳转行为。
interface TocMenuProps {
  items: TocItem[];
  onSelectItem: (item: TocItem) => void;
}

// 独立主题菜单：开关状态内聚在子组件中，避免影响整页渲染。
const ThemeMenu = memo(function ThemeMenu({
  themes,
  activeThemeId,
  onSelectTheme,
  customPreviewStyleText
}: ThemeMenuProps) {
  // 菜单展开状态仅影响当前子树，不触发父组件重渲染。
  const [isThemeMenuOpen, setIsThemeMenuOpen] = useState(false);
  // 样式详情抽屉对应的主题 ID；为空表示抽屉关闭。
  const [detailsThemeId, setDetailsThemeId] = useState<string | null>(null);
  // 菜单根节点引用：用于判断点击是否发生在菜单外部。
  const themeMenuRef = useRef<HTMLDivElement | null>(null);

  // 当前主题文案显示。
  const activeTheme = useMemo(() => {
    const foundTheme = themes.find((theme) => theme.id === activeThemeId);
    return foundTheme ?? themes[0];
  }, [themes, activeThemeId]);

  // 切换主题菜单显示状态。
  const toggleThemeMenu = useCallback(() => {
    setIsThemeMenuOpen((previous) => !previous);
  }, []);

  // 应用选中的主题并关闭菜单。
  const applyTheme = useCallback(
    (themeId: string) => {
      onSelectTheme(themeId);
      setIsThemeMenuOpen(false);
    },
    [onSelectTheme]
  );

  // 当前抽屉展示的主题对象。
  const detailsTheme = useMemo(
    () => (detailsThemeId ? resolvePreviewTheme(detailsThemeId) : null),
    [detailsThemeId]
  );

  // 打开指定主题的样式详情抽屉。
  const openThemeDetails = useCallback(
    (themeId: string) => {
      const targetTheme = resolvePreviewTheme(themeId);
      setDetailsThemeId(targetTheme.id);
      setIsThemeMenuOpen(false);
    },
    []
  );

  // 关闭样式详情抽屉。
  const closeStyleDetailsDrawer = useCallback(() => {
    setDetailsThemeId(null);
  }, []);

  // 主题菜单弹出时监听外部点击与 ESC，提升交互可控性。
  useEffect(() => {
    if (!isThemeMenuOpen) {
      return;
    }

    const onWindowMouseDown = (event: MouseEvent) => {
      const menuRootElement = themeMenuRef.current;
      if (!menuRootElement) {
        return;
      }
      if (event.target instanceof Node && menuRootElement.contains(event.target)) {
        return;
      }
      setIsThemeMenuOpen(false);
    };

    const onWindowKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setIsThemeMenuOpen(false);
      }
    };

    window.addEventListener("mousedown", onWindowMouseDown);
    window.addEventListener("keydown", onWindowKeyDown);
    return () => {
      window.removeEventListener("mousedown", onWindowMouseDown);
      window.removeEventListener("keydown", onWindowKeyDown);
    };
  }, [isThemeMenuOpen]);

  return (
    <>
      <div className="theme-menu" ref={themeMenuRef}>
        <button
          type="button"
          className="theme-menu__trigger"
          aria-label="选择预览主题"
          aria-haspopup="listbox"
          aria-expanded={isThemeMenuOpen}
          onClick={toggleThemeMenu}
        >
          <span className="theme-menu__trigger-label">主题</span>
          <span className="theme-menu__trigger-value">{activeTheme.name}</span>
        </button>
        {isThemeMenuOpen ? (
          <ul className="theme-menu__dropdown" role="listbox" aria-label="预览主题列表">
            {themes.map((themeTemplate) => {
              const isActiveTheme = themeTemplate.id === activeTheme.id;
              return (
                <li key={themeTemplate.id} className="theme-menu__item-row">
                  <button
                    type="button"
                    role="option"
                    aria-selected={isActiveTheme}
                    className={`theme-menu__item ${isActiveTheme ? "theme-menu__item--active" : ""}`}
                    onClick={() => applyTheme(themeTemplate.id)}
                  >
                    <span className="theme-menu__item-name">{themeTemplate.name}</span>
                    <span className="theme-menu__item-description">{themeTemplate.description}</span>
                  </button>
                  <button
                    type="button"
                    className="theme-menu__details-button"
                    aria-label={`查看 ${themeTemplate.name} 样式详情`}
                    onClick={() => openThemeDetails(themeTemplate.id)}
                  >
                    查看
                  </button>
                </li>
              );
            })}
          </ul>
        ) : null}
      </div>
      <StyleDetailsDrawer
        theme={detailsTheme}
        customPreviewStyleText={customPreviewStyleText}
        onClose={closeStyleDetailsDrawer}
      />
    </>
  );
});

// 目录菜单：仅控制自身开关状态，目录点击后导航到对应标题。
const TocMenu = memo(function TocMenu({ items, onSelectItem }: TocMenuProps) {
  // 目录菜单展开状态独立维护，避免影响主视图。
  const [isTocMenuOpen, setIsTocMenuOpen] = useState(false);
  // 菜单根节点引用：用于判断是否点击了菜单外部。
  const tocMenuRef = useRef<HTMLDivElement | null>(null);
  // 目录是否为空。
  const hasItems = items.length > 0;

  // 切换目录菜单显示状态。
  const toggleTocMenu = useCallback(() => {
    setIsTocMenuOpen((previous) => !previous);
  }, []);

  // 选择目录条目并关闭菜单。
  const handleSelectItem = useCallback(
    (item: TocItem) => {
      onSelectItem(item);
      setIsTocMenuOpen(false);
    },
    [onSelectItem]
  );

  // 目录菜单弹出时监听外部点击与 ESC，保证交互一致性。
  useEffect(() => {
    if (!isTocMenuOpen) {
      return;
    }

    const onWindowMouseDown = (event: MouseEvent) => {
      const menuRootElement = tocMenuRef.current;
      if (!menuRootElement) {
        return;
      }
      if (event.target instanceof Node && menuRootElement.contains(event.target)) {
        return;
      }
      setIsTocMenuOpen(false);
    };

    const onWindowKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setIsTocMenuOpen(false);
      }
    };

    window.addEventListener("mousedown", onWindowMouseDown);
    window.addEventListener("keydown", onWindowKeyDown);
    return () => {
      window.removeEventListener("mousedown", onWindowMouseDown);
      window.removeEventListener("keydown", onWindowKeyDown);
    };
  }, [isTocMenuOpen]);

  return (
    <div className="toc-menu" ref={tocMenuRef}>
      <button
        type="button"
        className="toc-menu__trigger"
        aria-label="打开目录"
        aria-haspopup="listbox"
        aria-expanded={isTocMenuOpen}
        onClick={toggleTocMenu}
      >
        <span className="toc-menu__trigger-label">目录</span>
        <span className="toc-menu__trigger-value">{hasItems ? `${items.length} 项` : "暂无"}</span>
      </button>
      {isTocMenuOpen ? (
        <div className="toc-menu__dropdown">
          {hasItems ? (
            <ul className="toc-menu__list" role="listbox" aria-label="目录列表">
              {items.map((item) => (
                <li key={`${item.sourceLine}-${item.level}`} className="toc-menu__item-row">
                  <button
                    type="button"
                    role="option"
                    className="toc-menu__item"
                    title={item.text}
                    // 根据标题层级做视觉缩进，强化目录结构层次。
                    style={{ paddingLeft: `${10 + (item.level - 1) * 14}px` }}
                    onClick={() => handleSelectItem(item)}
                  >
                    {item.text}
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <p className="toc-menu__empty">当前文档暂无标题。</p>
          )}
        </div>
      ) : null}
    </div>
  );
});

// 右侧样式详情抽屉：用于查看当前主题与覆盖样式细节。
const StyleDetailsDrawer = memo(function StyleDetailsDrawer({
  theme,
  customPreviewStyleText,
  onClose
}: StyleDetailsDrawerProps) {
  // 仅当存在主题时才展示抽屉。
  const isOpen = Boolean(theme);
  // 当前主题 CSS 模板：用于复制后快速二次修改。
  const themeCssTemplate = useMemo(
    () => (theme ? buildThemeCssTemplate(theme) : ""),
    [theme]
  );

  // 抽屉打开时支持 ESC 快捷关闭。
  useEffect(() => {
    if (!isOpen) {
      return;
    }
    const onWindowKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };
    window.addEventListener("keydown", onWindowKeyDown);
    return () => {
      window.removeEventListener("keydown", onWindowKeyDown);
    };
  }, [isOpen, onClose]);

  if (!theme) {
    return null;
  }

  return (
    <div className="style-drawer-layer" role="dialog" aria-modal="true" aria-label="当前样式详情">
      <button
        type="button"
        className="style-drawer-backdrop"
        aria-label="关闭样式详情抽屉"
        onClick={onClose}
      />
      <aside className="style-drawer">
        <header className="style-drawer__header">
          <div className="style-drawer__header-copy">
            <h2>当前生效样式</h2>
            <p>已生成带注释 CSS 模板，可直接复制后修改。</p>
          </div>
          <button type="button" className="style-drawer__close" onClick={onClose}>
            关闭
          </button>
        </header>
        <div className="style-drawer__body">
          <section className="style-drawer-section">
            <h3>主题信息</h3>
            <dl className="style-drawer-kv">
              <dt>主题 ID</dt>
              <dd>{theme.id}</dd>
              <dt>主题名称</dt>
              <dd>{theme.name}</dd>
              <dt>主题描述</dt>
              <dd>{theme.description}</dd>
              <dt>高亮主题</dt>
              <dd>{theme.syntaxTheme}</dd>
            </dl>
          </section>

          <section className="style-drawer-section">
            <h3>主题 CSS 源码（含注释，可复制）</h3>
            <pre className="style-drawer-code">{themeCssTemplate}</pre>
          </section>

          <section className="style-drawer-section">
            <h3>外部覆盖样式</h3>
            {customPreviewStyleText ? (
              <pre className="style-drawer-code">{customPreviewStyleText}</pre>
            ) : (
              <p className="style-drawer-empty">当前没有外部覆盖样式。</p>
            )}
          </section>
        </div>
      </aside>
    </div>
  );
});

export default function App() {
  // 数据网关单例。
  const dataGateway = useMemo(() => getDataGateway(), []);
  // 当前编辑内容。
  const [content, setContent] = useState(fallbackContent);
  // 当前打开文档 ID。
  const [activeDocId, setActiveDocId] = useState<string | null>(null);
  // 当前保存基线版本。
  const [baseVersion, setBaseVersion] = useState(0);
  // 最近一次成功保存的内容。
  const [lastSavedContent, setLastSavedContent] = useState(fallbackContent);
  // 保存状态。
  const [saveStatus, setSaveStatus] = useState<SaveStatus>("loading");
  // 页头状态文案。
  const [statusMessage, setStatusMessage] = useState("初始化中...");
  // 当前文档所属空间名。
  const [activeSpaceName, setActiveSpaceName] = useState("未命名空间");
  // 当前文档名称。
  const [activeDocumentTitle, setActiveDocumentTitle] = useState("未命名文档");
  // 最近一次成功保存时间。
  const [lastSavedAt, setLastSavedAt] = useState<string | null>(null);
  // 当前生效的预览主题 ID。
  const [activePreviewThemeId, setActivePreviewThemeId] = useState(DEFAULT_PREVIEW_THEME_ID);
  // 外部注入的预览样式文本；为空时仅使用内置主题。
  const [customPreviewStyleText, setCustomPreviewStyleText] = useState("");
  // 编辑器面板根节点，用于在 StrictMode 下追踪滚动容器的重建。
  const [editorPaneElement, setEditorPaneElement] = useState<HTMLElement | null>(null);
  // 编辑区滚动容器（state 版本），用于保证监听器绑定时机稳定。
  const [editorScrollerElement, setEditorScrollerElement] = useState<HTMLElement | null>(null);
  // 预览区滚动容器（state 版本），用于保证监听器绑定时机稳定。
  const [previewScrollerElement, setPreviewScrollerElement] = useState<HTMLElement | null>(null);
  // CodeMirror 滚动容器。
  const editorScrollerRef = useRef<HTMLElement | null>(null);
  // 编辑器面板节点（ref 版本），用于在回调里兜底查询最新滚动容器。
  const editorPaneRef = useRef<HTMLElement | null>(null);
  // CodeMirror 实例，用于文档偏移 -> 像素位置换算。
  const editorViewRef = useRef<EditorView | null>(null);
  // 预览区滚动容器。
  const previewScrollerRef = useRef<HTMLElement | null>(null);
  // 编辑区 -> 预览区锚点映射表。
  const editorToPreviewAnchorsRef = useRef<DirectionAnchor[]>([]);
  // 预览区 -> 编辑区锚点映射表。
  const previewToEditorAnchorsRef = useRef<DirectionAnchor[]>([]);
  // 映射重建调度句柄（rAF）。
  const rebuildMapRafRef = useRef<number | null>(null);
  // 同步滚动补帧句柄：用于分帧追平大跨度映射。
  const syncFollowRafRef = useRef<number | null>(null);
  // 延迟重建定时器集合：用于处理粘贴/批量改动后的异步布局收敛。
  const delayedRebuildTimersRef = useRef<number[]>([]);
  // 上一次内容快照：用于判断本次改动是否属于“大幅变更”（如整段粘贴）。
  const previousContentSnapshotRef = useRef(fallbackContent);
  // 防止双向同步引发循环滚动。
  const syncingRef = useRef(false);
  // 记录最近一次主动滚动来源，用于重算后回对齐。
  const lastScrollSourceRef = useRef<ScrollSource>("editor");

  // 统一更新编辑区滚动容器引用，避免重复 setState 触发不必要重渲染。
  const setEditorScrollerNode = useCallback((node: HTMLElement | null) => {
    editorScrollerRef.current = node;
    setEditorScrollerElement((previous) => (previous === node ? previous : node));
  }, []);

  // 编辑器面板 ref：用于感知 CodeMirror 在 StrictMode/HMR 下的重挂载。
  const handleEditorPaneRef = useCallback(
    (node: HTMLElement | null) => {
      // 维护 ref 版本，便于在 useCallback 闭包中读取最新面板节点。
      editorPaneRef.current = node;
      setEditorPaneElement((previous) => (previous === node ? previous : node));
      // 面板卸载时清空滚动容器引用，防止监听器挂在旧节点上。
      if (!node) {
        setEditorScrollerNode(null);
      }
    },
    [setEditorScrollerNode]
  );

  // 预览区 ref 采用稳定回调，避免每次渲染都触发 null -> node 抖动。
  const handlePreviewScrollerRef = useCallback((node: HTMLElement | null) => {
    previewScrollerRef.current = node;
    setPreviewScrollerElement((previous) => (previous === node ? previous : node));
  }, []);

  // 获取“当前仍可用”的编辑区滚动容器：优先用 ref，其次回退到编辑器面板查询。
  const resolveLiveEditorScroller = useCallback((): HTMLElement | null => {
    const currentScroller = editorScrollerRef.current;
    if (currentScroller && currentScroller.isConnected) {
      return currentScroller;
    }
    const fallbackScroller = editorPaneRef.current?.querySelector<HTMLElement>(".cm-scroller") ?? null;
    if (fallbackScroller && fallbackScroller.isConnected) {
      // 回写最新节点，避免后续逻辑继续使用过期引用。
      setEditorScrollerNode(fallbackScroller);
      return fallbackScroller;
    }
    return null;
  }, [setEditorScrollerNode]);

  // 获取“当前仍可用”的 EditorView：优先使用已有实例，失效时从 DOM 反查恢复。
  const resolveLiveEditorView = useCallback((editorScroller: HTMLElement): EditorView | null => {
    const currentEditorView = editorViewRef.current;
    if (currentEditorView && currentEditorView.dom.isConnected) {
      return currentEditorView;
    }
    const recoveredEditorView = EditorView.findFromDOM(editorScroller);
    if (recoveredEditorView && recoveredEditorView.dom.isConnected) {
      // 回写当前实例，避免映射重建持续失败。
      editorViewRef.current = recoveredEditorView;
      return recoveredEditorView;
    }
    return null;
  }, []);

  // 加载并监听外部自定义样式：支持 window 变量、localStorage 与自定义事件三种入口。
  useEffect(() => {
    // 读取初始样式：window 注入优先，其次回退到 localStorage。
    const readInitialCustomStyleText = (): string => {
      const styleFromWindow = normalizePreviewStyleText(window.__PLAINDOC_PREVIEW_STYLE__);
      if (styleFromWindow) {
        return styleFromWindow;
      }
      try {
        return normalizePreviewStyleText(
          window.localStorage.getItem(PREVIEW_CUSTOM_STYLE_STORAGE_KEY)
        );
      } catch {
        return "";
      }
    };

    setCustomPreviewStyleText(readInitialCustomStyleText());

    // 响应外部样式更新事件，并同步持久化到 localStorage。
    const onCustomStyleChanged = (event: Event) => {
      const detail = (event as CustomEvent<string>).detail;
      const normalizedStyleText = normalizePreviewStyleText(detail);
      setCustomPreviewStyleText(normalizedStyleText);
      try {
        if (normalizedStyleText) {
          window.localStorage.setItem(PREVIEW_CUSTOM_STYLE_STORAGE_KEY, normalizedStyleText);
        } else {
          window.localStorage.removeItem(PREVIEW_CUSTOM_STYLE_STORAGE_KEY);
        }
      } catch {
        // localStorage 失败时仅忽略持久化，不影响当前会话样式。
      }
    };

    window.addEventListener(PREVIEW_CUSTOM_STYLE_EVENT, onCustomStyleChanged);
    return () => {
      window.removeEventListener(PREVIEW_CUSTOM_STYLE_EVENT, onCustomStyleChanged);
    };
  }, []);

  // 首次加载时恢复上次选择的主题模板。
  useEffect(() => {
    try {
      const storedThemeId = window.localStorage.getItem(PREVIEW_THEME_STORAGE_KEY);
      if (!storedThemeId) {
        return;
      }
      const restoredTheme = resolvePreviewTheme(storedThemeId);
      setActivePreviewThemeId(restoredTheme.id);
    } catch {
      // localStorage 不可用时保持默认主题。
    }
  }, []);

  // 主题变化时写入本地缓存，便于下次启动直接恢复。
  useEffect(() => {
    try {
      window.localStorage.setItem(PREVIEW_THEME_STORAGE_KEY, activePreviewThemeId);
    } catch {
      // localStorage 失败时忽略持久化，不影响当前显示。
    }
  }, [activePreviewThemeId]);

  // 将当前滚动位置转换为滚动比例，用于锚点不足时兜底。
  const getRatio = (element: HTMLElement): number => {
    const maxScrollable = getMaxScrollable(element);
    if (maxScrollable <= 0) {
      return 0;
    }
    return element.scrollTop / maxScrollable;
  };

  // 在单向锚点表上做二分查找 + 分段线性插值。
  const mapScrollWithDirectionAnchors = (
    anchors: DirectionAnchor[],
    sourceY: number
  ): number => {
    if (anchors.length === 0) {
      return 0;
    }
    if (anchors.length === 1) {
      return anchors[0].targetY;
    }

    // 超出边界时直接钳到首尾锚点。
    const first = anchors[0];
    const last = anchors[anchors.length - 1];

    if (sourceY <= first.sourceY) {
      return first.targetY;
    }
    if (sourceY >= last.sourceY) {
      return last.targetY;
    }

    // 二分定位 sourceY 所在区间。
    let left = 0;
    let right = anchors.length - 1;
    while (left + 1 < right) {
      const middle = (left + right) >> 1;
      if (anchors[middle].sourceY <= sourceY) {
        left = middle;
      } else {
        right = middle;
      }
    }

    const start = anchors[left];
    const end = anchors[right];
    const sourceDistance = end.sourceY - start.sourceY;
    // source 轴重合时退到区间终点，保证尾部重合锚点可被命中。
    if (sourceDistance <= 0) {
      return end.targetY;
    }
    // 线性插值计算目标滚动位置。
    const progress = (sourceY - start.sourceY) / sourceDistance;
    return start.targetY + progress * (end.targetY - start.targetY);
  };

  // 根据当前来源区域，计算目标区域应设置的 scrollTop。
  const getMappedTargetScrollTop = useCallback(
    (sourceName: ScrollSource): number => {
      const editorElement = resolveLiveEditorScroller();
      const previewElement = previewScrollerRef.current;
      if (!editorElement || !previewElement) {
        return 0;
      }

      if (sourceName === "editor") {
        const previewMaxScrollable = getMaxScrollable(previewElement);
        const anchors = editorToPreviewAnchorsRef.current;
        // 锚点充足时优先走插值。
        if (anchors.length >= 2) {
          return clamp(mapScrollWithDirectionAnchors(anchors, editorElement.scrollTop), 0, previewMaxScrollable);
        }
        // 锚点不足时退回比例映射。
        return clamp(getRatio(editorElement) * previewMaxScrollable, 0, previewMaxScrollable);
      }

      const editorMaxScrollable = getMaxScrollable(editorElement);
      const anchors = previewToEditorAnchorsRef.current;
      // 预览 -> 编辑同理。
      if (anchors.length >= 2) {
        return clamp(mapScrollWithDirectionAnchors(anchors, previewElement.scrollTop), 0, editorMaxScrollable);
      }
      return clamp(getRatio(previewElement) * editorMaxScrollable, 0, editorMaxScrollable);
    },
    [resolveLiveEditorScroller]
  );

  // 清理同步补帧任务，避免旧来源任务持续写入滚动位置。
  const clearSyncFollowRaf = useCallback(() => {
    if (syncFollowRafRef.current !== null) {
      window.cancelAnimationFrame(syncFollowRafRef.current);
      syncFollowRafRef.current = null;
    }
  }, []);

  // 单帧同步：限制最大步进，避免高斜率区间（如展开 TOC）瞬间跨越。
  const applySyncStep = useCallback(
    (sourceName: ScrollSource): boolean => {
      const editorElement = resolveLiveEditorScroller();
      const previewElement = previewScrollerRef.current;
      if (!editorElement || !previewElement) {
        return false;
      }

      const targetElement = sourceName === "editor" ? previewElement : editorElement;
      const mappedTarget = getMappedTargetScrollTop(sourceName);
      const delta = mappedTarget - targetElement.scrollTop;
      if (Math.abs(delta) <= SYNC_SETTLE_THRESHOLD) {
        // 误差足够小时直接吸附到目标，避免小数误差抖动。
        targetElement.scrollTop = mappedTarget;
        return false;
      }

      const limitedStep = clamp(delta, -MAX_SYNC_STEP_PER_FRAME, MAX_SYNC_STEP_PER_FRAME);
      const nextTop = clamp(
        targetElement.scrollTop + limitedStep,
        0,
        getMaxScrollable(targetElement)
      );
      targetElement.scrollTop = nextTop;
      return Math.abs(delta) > MAX_SYNC_STEP_PER_FRAME;
    },
    [getMappedTargetScrollTop, resolveLiveEditorScroller]
  );

  // 若目标位移过大，继续按帧追平，保证视觉连续而不是一次跳跃。
  const scheduleFollowSync = useCallback(
    (sourceName: ScrollSource) => {
      if (syncFollowRafRef.current !== null) {
        return;
      }

      const follow = () => {
        syncFollowRafRef.current = null;
        if (syncingRef.current) {
          syncFollowRafRef.current = window.requestAnimationFrame(follow);
          return;
        }
        // 来源已变化时停止旧任务，避免跟当前用户输入“打架”。
        if (lastScrollSourceRef.current !== sourceName) {
          return;
        }

        syncingRef.current = true;
        const shouldContinue = applySyncStep(sourceName);
        window.requestAnimationFrame(() => {
          syncingRef.current = false;
          if (shouldContinue && lastScrollSourceRef.current === sourceName) {
            syncFollowRafRef.current = window.requestAnimationFrame(follow);
          }
        });
      };

      syncFollowRafRef.current = window.requestAnimationFrame(follow);
    },
    [applySyncStep]
  );

  // 执行一次单向同步，并用锁避免对端 scroll 反向触发。
  const syncFromSource = useCallback(
    (sourceName: ScrollSource) => {
      if (syncingRef.current) {
        return;
      }
      const editorElement = resolveLiveEditorScroller();
      const previewElement = previewScrollerRef.current;
      if (!editorElement || !previewElement) {
        return;
      }

      // 新输入到来时取消旧补帧任务，优先响应当前滚动来源。
      clearSyncFollowRaf();
      // 同步阶段写入锁并记录来源。
      syncingRef.current = true;
      lastScrollSourceRef.current = sourceName;
      const shouldFollow = applySyncStep(sourceName);
      window.requestAnimationFrame(() => {
        syncingRef.current = false;
        if (shouldFollow) {
          scheduleFollowSync(sourceName);
        }
      });
    },
    [applySyncStep, clearSyncFollowRaf, resolveLiveEditorScroller, scheduleFollowSync]
  );

  // 映射表重建后，按最近来源做一次回对齐。
  const resyncFromLastSource = useCallback(() => {
    if (syncingRef.current) {
      return;
    }
    const editorElement = resolveLiveEditorScroller();
    const previewElement = previewScrollerRef.current;
    if (!editorElement || !previewElement) {
      return;
    }
    syncingRef.current = true;
    const sourceName = lastScrollSourceRef.current;
    const shouldFollow = applySyncStep(sourceName);
    window.requestAnimationFrame(() => {
      syncingRef.current = false;
      if (shouldFollow) {
        scheduleFollowSync(sourceName);
      }
    });
  }, [applySyncStep, resolveLiveEditorScroller, scheduleFollowSync]);

  // 重建 block 级锚点映射表：source offset -> editorY 与 previewY。
  const rebuildScrollAnchors = useCallback(() => {
    const editorElement = resolveLiveEditorScroller();
    const previewElement = previewScrollerRef.current;
    const editorView = editorElement ? resolveLiveEditorView(editorElement) : null;
    // editorView 可能在 StrictMode 旧实例卸载后短暂失效，需等待新实例就绪。
    if (!editorElement || !previewElement || !editorView || !editorView.dom.isConnected) {
      editorToPreviewAnchorsRef.current = [];
      previewToEditorAnchorsRef.current = [];
      return;
    }

    const editorMaxScrollable = getMaxScrollable(editorElement);
    const previewMaxScrollable = getMaxScrollable(previewElement);
    const docLength = editorView.state.doc.length;
    const previewRect = previewElement.getBoundingClientRect();
    // 读取所有被 remark 注入的锚点节点。
    const anchorNodes = previewElement.querySelectorAll<HTMLElement>(BLOCK_ANCHOR_SELECTOR);
    // 先收集原始锚点，再分别构建双向映射表。
    const rawAnchors: ScrollAnchor[] = [];
    // 已处理锚点序号集合：用于去重被渲染器复制到子节点的重复锚点。
    const seenAnchorIndices = new Set<string>();

    // 统一解析 block 的起止源码位置，映射为编辑区像素坐标。
    const resolveEditorAnchorY = (
      rawLine: string | undefined,
      rawOffset: string | undefined,
      anchorEdge: "start" | "end"
    ): number | null => {
      if (rawLine) {
        const parsedLine = Number(rawLine);
        if (Number.isFinite(parsedLine)) {
          const lineNumber = clamp(Math.floor(parsedLine), 1, editorView.state.doc.lines);
          const lineFrom = editorView.state.doc.line(lineNumber).from;
          const lineBlock = editorView.lineBlockAt(lineFrom);
          return clamp(
            anchorEdge === "end" ? lineBlock.bottom : lineBlock.top,
            0,
            editorMaxScrollable
          );
        }
      }
      if (!rawOffset) {
        return null;
      }
      const parsedOffset = Number(rawOffset);
      if (!Number.isFinite(parsedOffset)) {
        return null;
      }
      const normalizedOffset = clamp(Math.floor(parsedOffset), 0, docLength);
      // end offset 在 AST 语义上通常指向“块后一个字符”，这里回退 1 以命中块末尾行。
      const anchorOffset =
        anchorEdge === "end" ? clamp(normalizedOffset - 1, 0, docLength) : normalizedOffset;
      const lineBlock = editorView.lineBlockAt(anchorOffset);
      return clamp(
        anchorEdge === "end" ? lineBlock.bottom : lineBlock.top,
        0,
        editorMaxScrollable
      );
    };

    for (const node of anchorNodes) {
      const anchorIndex = node.dataset.anchorIndex;
      // 仅使用插件生成的锚点，避免误采集到渲染库内部节点。
      if (!anchorIndex) {
        continue;
      }
      if (seenAnchorIndices.has(anchorIndex)) {
        continue;
      }
      seenAnchorIndices.add(anchorIndex);

      const rawStartLine = node.dataset.sourceLine;
      const rawStartOffset = node.dataset.sourceOffset;
      const rawEndLine = node.dataset.sourceEndLine;
      const rawEndOffset = node.dataset.sourceEndOffset;

      const editorStartY = resolveEditorAnchorY(rawStartLine, rawStartOffset, "start");
      if (editorStartY === null) {
        continue;
      }
      const editorEndYCandidate = resolveEditorAnchorY(rawEndLine, rawEndOffset, "end");
      const editorEndY =
        editorEndYCandidate === null ? editorStartY : Math.max(editorStartY, editorEndYCandidate);

      // 将节点视口坐标转换为容器内容坐标，并为块底部补一组锚点。
      const nodeRect = node.getBoundingClientRect();
      const previewStartY = clamp(
        nodeRect.top - previewRect.top + previewElement.scrollTop,
        0,
        previewMaxScrollable
      );
      const previewEndY = clamp(
        nodeRect.bottom - previewRect.top + previewElement.scrollTop,
        0,
        previewMaxScrollable
      );

      rawAnchors.push({
        editorY: editorStartY,
        previewY: previewStartY
      });
      // 对可见高度大于一行的 block（如块状公式）补充底部锚点，避免滚动跨越。
      if (
        editorEndYCandidate !== null &&
        editorEndY > editorStartY &&
        previewEndY > previewStartY
      ) {
        rawAnchors.push({
          editorY: editorEndY,
          previewY: Math.max(previewStartY, previewEndY)
        });
      }
    }

    // 构建编辑区 -> 预览区映射：同 editorY 聚合到最远 previewY。
    editorToPreviewAnchorsRef.current = buildDirectionAnchors(
      rawAnchors.map((anchor) => ({
        sourceY: anchor.editorY,
        targetY: anchor.previewY
      })),
      editorMaxScrollable,
      previewMaxScrollable
    );

    // 构建预览区 -> 编辑区映射：同 previewY 聚合到最远 editorY。
    previewToEditorAnchorsRef.current = buildDirectionAnchors(
      rawAnchors.map((anchor) => ({
        sourceY: anchor.previewY,
        targetY: anchor.editorY
      })),
      previewMaxScrollable,
      editorMaxScrollable
    );
  }, [resolveLiveEditorScroller, resolveLiveEditorView]);

  // 用 requestAnimationFrame 合并多次重建请求，降低重排频率。
  const scheduleRebuildScrollAnchors = useCallback(() => {
    if (rebuildMapRafRef.current !== null) {
      return;
    }

    rebuildMapRafRef.current = window.requestAnimationFrame(() => {
      rebuildMapRafRef.current = null;
      rebuildScrollAnchors();
      resyncFromLastSource();
    });
  }, [rebuildScrollAnchors, resyncFromLastSource]);

  // 清理所有延迟重建任务，避免重复排队造成无效重建。
  const clearDelayedRebuildTimers = useCallback(() => {
    for (const timerId of delayedRebuildTimersRef.current) {
      window.clearTimeout(timerId);
    }
    delayedRebuildTimersRef.current = [];
  }, []);

  // 追加多次延迟重建：覆盖粘贴后编辑器/预览异步布局更新窗口。
  const scheduleDelayedRebuilds = useCallback(
    (delays: number[]) => {
      clearDelayedRebuildTimers();
      delayedRebuildTimersRef.current = delays.map((delay) => {
        const scheduledTimerId = window.setTimeout(() => {
          scheduleRebuildScrollAnchors();
          // 执行后移出记录，避免数组持续增长。
          delayedRebuildTimersRef.current = delayedRebuildTimersRef.current.filter(
            (timerId) => timerId !== scheduledTimerId
          );
        }, delay);
        return scheduledTimerId;
      });
    },
    [clearDelayedRebuildTimers, scheduleRebuildScrollAnchors]
  );

  const extensions = useMemo(
    () => [
      // 编辑器软换行，避免横向滚动影响同步体验。
      EditorView.lineWrapping,
      markdown({
        // 启用 Markdown 语言与代码块语言支持。
        base: markdownLanguage,
        codeLanguages: languages
      })
    ],
    []
  );
  // remark 插件顺序：先 GFM，再解析数学公式，最后注入锚点属性。
  const remarkPlugins = useMemo(() => [remarkGfm, remarkMath, remarkBlockAnchorPlugin], []);
  // rehype 插件：将 Math AST 渲染为 KaTeX HTML。
  const rehypePlugins = useMemo(() => [rehypeKatex], []);
  // markdown-it 仅用于“去语法后的文字统计”和 TOC 语法解析。
  const markdownTextParser = useMemo(
    () =>
      new MarkdownIt({
        html: false,
        linkify: true,
        typographer: false
      }),
    []
  );
  // 解析文档标题与 [TOC] 标记，供目录菜单与语法渲染共用。
  const tocParseResult = useMemo(
    () => parseTocFromMarkdown(content, markdownTextParser),
    [content, markdownTextParser]
  );
  // TOC 标题列表。
  const tocItems = tocParseResult.items;
  // 当前文档是否声明了 [TOC] 语法标记。
  const hasTocMarker = tocParseResult.hasMarker;

  // 根据目录条目滚动预览区，让同步滚动机制继续驱动编辑区对齐。
  const handleTocNavigate = useCallback((item: TocItem) => {
    const previewElement = previewScrollerRef.current;
    if (!previewElement) {
      return;
    }
    scrollPreviewToTocItem(previewElement, item);
  }, []);

  // 自定义 Markdown 渲染器：代码块走高亮组件，行内代码走轻量内联样式。
  const markdownComponents = useMemo<Components>(
    () => {
      // 读取当前主题下的代码渲染配置。
      const activeTheme = resolvePreviewTheme(activePreviewThemeId);
      const syntaxTheme =
        PREVIEW_SYNTAX_THEMES[activeTheme.syntaxTheme] ?? PREVIEW_SYNTAX_THEMES["one-light"];

      return {
        // 标题统一渲染为 prefix/content/suffix 结构，便于主题做伪元素以外的装饰扩展。
        h1: ({ node: _node, children, ...props }) => (
          <h1 {...props}>
            <span className="prefix" aria-hidden="true" />
            <span className="content">{children}</span>
            <span className="suffix" aria-hidden="true" />
          </h1>
        ),
        // 二级标题同样输出固定结构，兼容用户自定义 CSS 选择器。
        h2: ({ node: _node, children, ...props }) => (
          <h2 {...props}>
            <span className="prefix" aria-hidden="true" />
            <span className="content">{children}</span>
            <span className="suffix" aria-hidden="true" />
          </h2>
        ),
        // 三级标题同样支持前后缀装饰节点。
        h3: ({ node: _node, children, ...props }) => (
          <h3 {...props}>
            <span className="prefix" aria-hidden="true" />
            <span className="content">{children}</span>
            <span className="suffix" aria-hidden="true" />
          </h3>
        ),
        // 四级标题保持一致结构，避免不同层级样式能力不一致。
        h4: ({ node: _node, children, ...props }) => (
          <h4 {...props}>
            <span className="prefix" aria-hidden="true" />
            <span className="content">{children}</span>
            <span className="suffix" aria-hidden="true" />
          </h4>
        ),
        // 五级标题保持一致结构，便于主题批量复用样式规则。
        h5: ({ node: _node, children, ...props }) => (
          <h5 {...props}>
            <span className="prefix" aria-hidden="true" />
            <span className="content">{children}</span>
            <span className="suffix" aria-hidden="true" />
          </h5>
        ),
        // 六级标题保持一致结构，确保所有标题层级都可被统一定制。
        h6: ({ node: _node, children, ...props }) => (
          <h6 {...props}>
            <span className="prefix" aria-hidden="true" />
            <span className="content">{children}</span>
            <span className="suffix" aria-hidden="true" />
          </h6>
        ),
        // 识别独占段落 [TOC] 标记，并在文档内渲染可点击目录菜单。
        p: ({ node: _node, className, children, ...props }) => {
          const paragraphText = extractCodeText(children).trim();
          if (!isTocMarkerText(paragraphText)) {
            return (
              <p className={className} {...props}>
                {children}
              </p>
            );
          }
          // TOC 标记块只透传锚点属性，避免段落 ref 类型与 details 冲突。
          const tocAnchorDataAttributes = pickAnchorDataAttributes(props as Record<string, unknown>);

          return (
            <details
              className={["toc-inline", className].filter(Boolean).join(" ")}
              aria-label="文档目录"
              open
              {...tocAnchorDataAttributes}
            >
              <summary className="toc-inline__summary">
                <span className="toc-inline__title">文档目录</span>
                <span className="toc-inline__meta">{tocItems.length} 项</span>
              </summary>
              {tocItems.length ? (
                <div className="toc-inline__body">
                  <ol className="toc-inline__list">
                    {tocItems.map((item) => (
                      <li key={`${item.sourceLine}-${item.level}`} className="toc-inline__item">
                        <button
                          type="button"
                          className="toc-inline__button"
                          // 根据标题层级做视觉缩进，强化文档结构层次。
                          style={{ paddingLeft: `${10 + (item.level - 1) * 14}px` }}
                          onClick={() => handleTocNavigate(item)}
                        >
                          {item.text}
                        </button>
                      </li>
                    ))}
                  </ol>
                </div>
              ) : (
                <p className="toc-inline__empty">当前文档暂无可用标题。</p>
              )}
            </details>
          );
        },
        pre: ({ node: _node, children, ...props }) => {
          const childNodes = Children.toArray(children);
          const codeElement = childNodes[0];
          if (!isValidElement(codeElement)) {
            return <pre {...props}>{children}</pre>;
          }

          const codeElementProps = codeElement.props as Record<string, unknown>;
          const codeClassName =
            typeof codeElementProps.className === "string" ? codeElementProps.className : undefined;
          const language = resolveCodeLanguage(codeClassName);
          const anchorDataAttributes = pickAnchorDataAttributes(codeElementProps);
          const codeText = extractCodeText(codeElementProps.children as ReactNode).replace(/\n$/, "");

          // 自定义 PreTag：把 source 锚点挂回代码块根节点，保证滚动映射不丢失。
          const PreTag = ({
            children: preChildren,
            style: preStyle,
            ...preTagProps
          }: ComponentPropsWithoutRef<"pre">) => (
            <pre
              {...preTagProps}
              {...anchorDataAttributes}
              style={{
                ...(preStyle ?? {}),
                ...activeTheme.codeBlockStyle
              }}
            >
              {preChildren}
            </pre>
          );

          return (
            <SyntaxHighlighter
              language={language}
              style={syntaxTheme}
              PreTag={PreTag}
              useInlineStyles
              wrapLongLines
              codeTagProps={{
                className: codeClassName,
                style: activeTheme.codeBlockCodeStyle
              }}
            >
              {codeText}
            </SyntaxHighlighter>
          );
        },
        code: ({ node: _node, className, style, children, ...props }) => {
          const dataSourceLine = (props as Record<string, unknown>)["data-source-line"];
          const dataSourceOffset = (props as Record<string, unknown>)["data-source-offset"];
          const isBlockCode = typeof dataSourceLine === "string" || typeof dataSourceOffset === "string";
          // block code 由 pre 渲染器统一处理，code 节点只做透传，避免重复包裹。
          if (isBlockCode) {
            return (
              <code className={className} style={style} {...props}>
                {children}
              </code>
            );
          }

          return (
            <code
              className={className}
              style={{
                ...activeTheme.inlineCodeStyle,
                ...(style ?? {})
              }}
              {...props}
            >
              {children}
            </code>
          );
        }
      };
    },
    [activePreviewThemeId, handleTocNavigate, tocItems]
  );
  // 提取 Markdown 对应的纯文本内容。
  const plainTextContent = useMemo(
    () => extractPlainTextFromMarkdown(content, markdownTextParser),
    [content, markdownTextParser]
  );
  // 统计非空白字符数量，作为字数展示。
  const plainTextCount = useMemo(
    () => plainTextContent.replace(/\s+/g, "").length,
    [plainTextContent]
  );
  // 将最后保存时间格式化为状态栏文案。
  const lastSavedTimeLabel = useMemo(() => formatSavedTime(lastSavedAt), [lastSavedAt]);
  // 根据保存状态生成状态栏图标展示类型。
  const saveIndicatorVariant = useMemo(
    () => resolveSaveIndicatorVariant(saveStatus),
    [saveStatus]
  );
  // 当前生效主题对象，用于渲染菜单高亮和生成样式。
  const activePreviewTheme = useMemo(
    () => resolvePreviewTheme(activePreviewThemeId),
    [activePreviewThemeId]
  );
  // 预览区主题类名：挂到预览容器上参与选择器匹配。
  const activePreviewThemeClassName = useMemo(
    () => getPreviewThemeClassName(activePreviewTheme.id),
    [activePreviewTheme.id]
  );
  // 当前主题对应的变量样式文本：通过 style 标签注入。
  const activePreviewThemeStyleText = useMemo(
    () => buildPreviewThemeStyleText(activePreviewTheme),
    [activePreviewTheme]
  );

  // 首次启动：加载空间、文档树与默认文档内容。
  useEffect(() => {
    let cancelled = false;

    const bootstrap = async () => {
      try {
        const spaces = await dataGateway.workspace.listSpaces();
        const space =
          spaces[0] ??
          (await dataGateway.workspace.createSpace({
            name: "默认空间"
          }));
        const tree = await dataGateway.workspace.getTree(space.id);
        const existingDocId = findFirstDocId(tree);
        const docId =
          existingDocId ??
          (
            await dataGateway.workspace.createNode({
              spaceId: space.id,
              parentId: null,
              type: "doc",
              title: "未命名文档"
            })
          ).docId;

        if (!docId) {
          throw new Error("无法创建初始化文档");
        }

        const document = await dataGateway.document.getDocument(docId);
        if (cancelled) {
          return;
        }

        // 初始化编辑状态与保存基线。
        setActiveSpaceName(space.name);
        setActiveDocumentTitle(document.title || "未命名文档");
        setLastSavedAt(document.updatedAt);
        setActiveDocId(document.id);
        setBaseVersion(document.version);
        setContent(document.contentMd);
        setLastSavedContent(document.contentMd);
        setSaveStatus("ready");
        setStatusMessage(`已加载文档 v${document.version}`);
      } catch (error) {
        if (cancelled) {
          return;
        }
        setSaveStatus("error");
        setStatusMessage(`加载失败：${formatError(error)}`);
      }
    };

    void bootstrap();

    return () => {
      cancelled = true;
    };
  }, [dataGateway]);

  // 自动保存：内容变化后延迟提交，处理版本冲突与失败状态。
  useEffect(() => {
    if (
      !activeDocId ||
      content === lastSavedContent ||
      saveStatus === "loading" ||
      saveStatus === "saving"
    ) {
      return;
    }

    const timer = window.setTimeout(async () => {
      setSaveStatus("saving");
      setStatusMessage("保存中...");
      try {
        const result = await dataGateway.document.saveDocument({
          docId: activeDocId,
          contentMd: content,
          baseVersion
        });
        setBaseVersion(result.document.version);
        setActiveDocumentTitle(result.document.title || "未命名文档");
        setLastSavedAt(result.document.updatedAt);
        setLastSavedContent(result.document.contentMd);
        setSaveStatus("saved");
        setStatusMessage(`已保存 v${result.document.version}`);
      } catch (error) {
        if (error instanceof ConflictError) {
          setSaveStatus("conflict");
          setStatusMessage(
            `检测到冲突：当前基线 v${baseVersion}，最新版本 v${error.latestDocument.version}`
          );
          return;
        }
        setSaveStatus("error");
        setStatusMessage(`保存失败：${formatError(error)}`);
      }
    }, 800);

    // 输入持续变化时清理上一次保存定时器。
    return () => {
      window.clearTimeout(timer);
    };
  }, [activeDocId, baseVersion, content, dataGateway, lastSavedContent, saveStatus]);

  // 当滚动容器就绪后重建一次映射，避免首屏阶段因时序问题拿到空锚点。
  useEffect(() => {
    if (!editorScrollerElement || !previewScrollerElement) {
      return;
    }
    scheduleRebuildScrollAnchors();
  }, [editorScrollerElement, previewScrollerElement, scheduleRebuildScrollAnchors]);

  // 监听编辑器面板中的 DOM 变化，确保滚动容器引用始终指向“当前活跃实例”。
  useEffect(() => {
    if (!editorPaneElement) {
      return;
    }

    const refreshEditorScroller = () => {
      const currentScroller = editorPaneElement.querySelector<HTMLElement>(".cm-scroller");
      // 仅接受仍在文档中的节点，避免绑定到已销毁实例。
      if (currentScroller && currentScroller.isConnected) {
        setEditorScrollerNode(currentScroller);
        return;
      }
      setEditorScrollerNode(null);
    };

    refreshEditorScroller();

    let mutationObserver: MutationObserver | null = null;
    if (typeof MutationObserver !== "undefined") {
      mutationObserver = new MutationObserver(() => {
        refreshEditorScroller();
      });
      mutationObserver.observe(editorPaneElement, {
        childList: true,
        subtree: true
      });
    }

    return () => {
      mutationObserver?.disconnect();
    };
  }, [editorPaneElement, setEditorScrollerNode]);

  // 绑定编辑区与预览区滚动事件，触发单向同步。
  useEffect(() => {
    if (!editorScrollerElement || !previewScrollerElement) {
      return;
    }

    const onEditorScroll = () => {
      syncFromSource("editor");
    };

    const onPreviewScroll = () => {
      syncFromSource("preview");
    };

    editorScrollerElement.addEventListener("scroll", onEditorScroll, { passive: true });
    previewScrollerElement.addEventListener("scroll", onPreviewScroll, { passive: true });

    return () => {
      editorScrollerElement.removeEventListener("scroll", onEditorScroll);
      previewScrollerElement.removeEventListener("scroll", onPreviewScroll);
    };
  }, [editorScrollerElement, previewScrollerElement, syncFromSource]);

  // 内容变更后需要重建锚点映射。
  useEffect(() => {
    // 判断是否为批量变更（例如一次性粘贴长文）；批量变更时增加延迟重建兜底。
    const previousContent = previousContentSnapshotRef.current;
    const currentContent = content;
    const characterDelta = Math.abs(currentContent.length - previousContent.length);
    const isBulkContentChange = characterDelta >= 120;
    previousContentSnapshotRef.current = currentContent;

    scheduleRebuildScrollAnchors();
    if (isBulkContentChange) {
      // 多轮重建用于覆盖 CodeMirror 重排、图片加载与高亮渲染的延迟窗口。
      scheduleDelayedRebuilds([80, 240, 520]);
    }
  }, [content, scheduleDelayedRebuilds, scheduleRebuildScrollAnchors]);

  // 处理“大段粘贴”场景：粘贴后追加延迟重建，覆盖图片与布局异步更新窗口。
  useEffect(() => {
    const editorElement = editorScrollerElement;
    if (!editorElement) {
      return;
    }

    // 记录本次粘贴触发的延迟任务，组件卸载或重复粘贴时统一清理。
    const pendingTimers = new Set<number>();

    const onPaste = () => {
      // 第一次重建：尽快刷新映射，保证初次滚动就可同步。
      scheduleRebuildScrollAnchors();
      // 第二次重建：等待 CodeMirror 完成一轮布局更新。
      const timerAfterLayout = window.setTimeout(() => {
        pendingTimers.delete(timerAfterLayout);
        scheduleRebuildScrollAnchors();
      }, 60);
      pendingTimers.add(timerAfterLayout);
      // 第三次重建：兜底等待图片尺寸与高亮等异步渲染完成。
      const timerAfterAsyncRender = window.setTimeout(() => {
        pendingTimers.delete(timerAfterAsyncRender);
        scheduleRebuildScrollAnchors();
      }, 220);
      pendingTimers.add(timerAfterAsyncRender);
    };

    // 使用捕获阶段监听 paste，规避 CodeMirror 在冒泡阶段拦截事件导致监听不到。
    editorElement.addEventListener("paste", onPaste, true);
    return () => {
      editorElement.removeEventListener("paste", onPaste, true);
      for (const timerId of pendingTimers) {
        window.clearTimeout(timerId);
      }
      pendingTimers.clear();
    };
  }, [editorScrollerElement, scheduleRebuildScrollAnchors]);

  // 主题样式或外部覆盖样式变化后，主动重建锚点映射，避免滚动同步漂移。
  useEffect(() => {
    scheduleRebuildScrollAnchors();
  }, [activePreviewThemeClassName, customPreviewStyleText, scheduleRebuildScrollAnchors]);

  // 监听图片异步加载与容器尺寸变化，保障长图场景下映射实时更新。
  useEffect(() => {
    const previewElement = previewScrollerElement;
    const editorElement = editorScrollerElement;
    if (!previewElement || !editorElement) {
      return;
    }

    // 单图事件处理器：任意图片完成/失败都触发锚点重建。
    const onImageEvent = () => {
      scheduleRebuildScrollAnchors();
    };

    // 维护已绑定的图片集合，避免重复绑定监听器。
    const boundImages = new Set<HTMLImageElement>();

    // 为当前预览区图片绑定监听；若图片已完成加载则立即触发一次重建。
    const refreshImageBindings = () => {
      const currentImages = Array.from(previewElement.querySelectorAll<HTMLImageElement>("img"));
      const currentImageSet = new Set(currentImages);

      for (const image of currentImages) {
        if (boundImages.has(image)) {
          continue;
        }
        image.addEventListener("load", onImageEvent);
        image.addEventListener("error", onImageEvent);
        boundImages.add(image);
        // 处理缓存命中场景：已完成图片不会再次触发 load，需要主动重建映射。
        if (image.complete) {
          scheduleRebuildScrollAnchors();
        }
      }

      for (const image of boundImages) {
        if (currentImageSet.has(image)) {
          continue;
        }
        image.removeEventListener("load", onImageEvent);
        image.removeEventListener("error", onImageEvent);
        boundImages.delete(image);
      }
    };

    refreshImageBindings();

    let resizeObserver: ResizeObserver | null = null;
    if (typeof ResizeObserver !== "undefined") {
      resizeObserver = new ResizeObserver(() => {
        // 任意尺寸变化都触发一次合并后的重建。
        scheduleRebuildScrollAnchors();
      });
      resizeObserver.observe(editorElement);
      // 直接观察预览滚动容器尺寸变化。
      resizeObserver.observe(previewElement);
      const markdownBody = previewElement.querySelector<HTMLElement>(PREVIEW_BODY_SELECTOR);
      if (markdownBody) {
        resizeObserver.observe(markdownBody);
      }
    }

    let mutationObserver: MutationObserver | null = null;
    if (typeof MutationObserver !== "undefined") {
      mutationObserver = new MutationObserver(() => {
        // 预览 DOM 变更（例如图片节点替换）后刷新监听并重建映射。
        refreshImageBindings();
        scheduleRebuildScrollAnchors();
      });
      mutationObserver.observe(previewElement, {
        childList: true,
        subtree: true
      });
    }

    return () => {
      for (const image of boundImages) {
        image.removeEventListener("load", onImageEvent);
        image.removeEventListener("error", onImageEvent);
      }
      resizeObserver?.disconnect();
      mutationObserver?.disconnect();
    };
  }, [editorScrollerElement, previewScrollerElement, scheduleRebuildScrollAnchors]);

  // 卸载时取消未执行的重建任务，避免悬挂回调。
  useEffect(() => {
    return () => {
      if (rebuildMapRafRef.current !== null) {
        window.cancelAnimationFrame(rebuildMapRafRef.current);
      }
      clearSyncFollowRaf();
      clearDelayedRebuildTimers();
    };
  }, [clearDelayedRebuildTimers, clearSyncFollowRaf]);

  // 应用选中的主题：仅在主题真正变化时更新父组件状态。
  const handleThemeChange = useCallback((themeId: string) => {
    const targetTheme = resolvePreviewTheme(themeId);
    setActivePreviewThemeId((previousThemeId) =>
      previousThemeId === targetTheme.id ? previousThemeId : targetTheme.id
    );
  }, []);

  // 手动同步到最新版本，用于冲突后的回拉。
  const syncLatestVersion = async () => {
    if (!activeDocId) {
      return;
    }
    try {
      const latestDocument = await dataGateway.document.getDocument(activeDocId);
      setActiveDocumentTitle(latestDocument.title || "未命名文档");
      setLastSavedAt(latestDocument.updatedAt);
      setContent(latestDocument.contentMd);
      setBaseVersion(latestDocument.version);
      setLastSavedContent(latestDocument.contentMd);
      setSaveStatus("ready");
      setStatusMessage(`已同步到最新版本 v${latestDocument.version}`);
    } catch (error) {
      setSaveStatus("error");
      setStatusMessage(`同步失败：${formatError(error)}`);
    }
  };

  return (
    // 主页面容器。
    <div className="page">
      {/* 当前主题样式：先注入内置模板变量，后续允许外部样式继续覆盖。 */}
      {activePreviewThemeStyleText ? (
        <style id="plaindoc-preview-theme-style">{activePreviewThemeStyleText}</style>
      ) : null}
      {/* 外部自定义预览样式：存在时插入到页面末端，确保覆盖内置主题。 */}
      {customPreviewStyleText ? (
        <style id="plaindoc-preview-custom-style">{customPreviewStyleText}</style>
      ) : null}
      {/* 顶部状态栏。 */}
      <header className="header">
        <h1>PlainDoc</h1>
        <div className="header-actions">
          {/* 目录菜单：展示标题结构并支持快速跳转。 */}
          {hasTocMarker ? <TocMenu items={tocItems} onSelectItem={handleTocNavigate} /> : null}
          {/* 主题菜单：展开/收起只更新菜单组件自身。 */}
          <ThemeMenu
            themes={PREVIEW_THEME_TEMPLATES}
            activeThemeId={activePreviewTheme.id}
            onSelectTheme={handleThemeChange}
            customPreviewStyleText={customPreviewStyleText}
          />
        </div>
      </header>
      {/* 双栏工作区：左编辑、右预览。 */}
      <main className="workspace">
        <section className="pane editor-pane" ref={handleEditorPaneRef}>
          <CodeMirror
            value={content}
            extensions={extensions}
            height="100%"
            onCreateEditor={(view) => {
              // 保存编辑器实例与滚动容器引用，供映射计算使用。
              editorViewRef.current = view;
              setEditorScrollerNode(view.scrollDOM);
              scheduleRebuildScrollAnchors();
            }}
            onChange={(value) => {
              // 录入编辑内容，并将状态切回可保存。
              setContent(value);
              if (saveStatus !== "loading") {
                setSaveStatus("ready");
              }
            }}
            basicSetup={{
              lineNumbers: false,
              foldGutter: false
            }}
          />
        </section>
        <section
          id={PREVIEW_PANE_ID}
          className={`pane preview-pane ${PREVIEW_PANE_CLASS} ${activePreviewThemeClassName}`}
          // 使用稳定 ref 回调，保证滚动监听不会被重复拆装。
          ref={handlePreviewScrollerRef}
        >
          <article className={`markdown-body ${PREVIEW_BODY_CLASS}`}>
            {/* 使用 remark 插件渲染 Markdown 并写入 block 锚点。 */}
            <ReactMarkdown
              remarkPlugins={remarkPlugins}
              rehypePlugins={rehypePlugins}
              components={markdownComponents}
            >
              {content}
            </ReactMarkdown>
          </article>
        </section>
      </main>
      {/* 冲突提示与手动同步入口。 */}
      {saveStatus === "conflict" ? (
        <footer className="conflict-footer">
          <span>当前文档存在版本冲突，请先同步最新版本后再手动合并。</span>
          <button type="button" onClick={() => void syncLatestVersion()}>
            同步最新版本
          </button>
        </footer>
      ) : null}
      {/* 固定底部状态栏：左侧空间/文件，右侧保存时间/字数。 */}
      <footer className="status-bar">
        <div className="status-bar__side status-bar__side--left">
          <span style={{ fontWeight: 600 }}>文档位置：</span>
          <span className="status-pill" title={activeSpaceName}>
            {activeSpaceName}
          </span>
          <span className="status-separator">/</span>
          <span className="status-pill" title={activeDocumentTitle}>
            {activeDocumentTitle}
          </span>
        </div>
        <div className="status-bar__side status-bar__side--right">
          {/* 保存状态图标：未保存=黄色，保存中=旋转，已保存=绿色。 */}
          <span
            className={`status-save-indicator status-save-indicator--${saveIndicatorVariant}`}
            title={statusMessage}
            aria-label={statusMessage}
          >
            {saveIndicatorVariant === "saving" ? (
              <LoaderCircle className="status-save-icon status-save-icon--spin" size={14} />
            ) : null}
            {saveIndicatorVariant === "saved" ? (
              <CheckCircle2 className="status-save-icon" size={14} />
            ) : null}
            {saveIndicatorVariant === "unsaved" ? (
              <AlertCircle className="status-save-icon" size={14} />
            ) : null}
          </span>
          <span><span style={{ fontWeight: 600 }}>最后保存时间：</span>{lastSavedTimeLabel}</span>
          <span><span style={{ fontWeight: 600 }}>字数统计：</span>{plainTextCount}</span>
        </div>
      </footer>
    </div>
  );
}
