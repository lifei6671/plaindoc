import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import { languages } from "@codemirror/language-data";
import { EditorView } from "@codemirror/view";
import CodeMirror from "@uiw/react-codemirror";
import { AlertCircle, CheckCircle2, LoaderCircle } from "lucide-react";
import MarkdownIt from "markdown-it";
import {
  Children,
  isValidElement,
  memo,
  type CSSProperties,
  type ComponentPropsWithoutRef,
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState
} from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import { oneDark, oneLight } from "react-syntax-highlighter/dist/esm/styles/prism";
import remarkGfm from "remark-gfm";
import { ConflictError, getDataGateway, type TreeNode } from "./data-access";

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

// 仅包含本模块需要访问的 Markdown AST 字段。
interface MarkdownNode {
  type: string;
  position?: {
    start?: {
      line?: number;
      offset?: number;
    };
  };
  data?: {
    hProperties?: Record<string, unknown>;
  };
  children?: MarkdownNode[];
}

// 内置主题模板定义：用于管理可切换的预览风格。
interface PreviewThemeTemplate {
  id: string;
  name: string;
  description: string;
  variables: Record<string, string>;
  syntaxTheme: PreviewSyntaxThemeId;
  codeBlockStyle: CSSProperties;
  codeBlockCodeStyle: CSSProperties;
  inlineCodeStyle: CSSProperties;
}

// 语法高亮主题标识。
type PreviewSyntaxThemeId = "one-light" | "one-dark";

// 参与锚点映射的 block 级节点类型。
const BLOCK_NODE_TYPES = new Set([
  "paragraph",
  "heading",
  "blockquote",
  "code",
  "table",
  "thematicBreak",
  "html"
]);

// 预览区锚点节点选择器。
const BLOCK_ANCHOR_SELECTOR = "[data-source-line], [data-source-offset]";

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

// 代码高亮主题映射表：用于在主题模板里切换高亮配色。
const PREVIEW_SYNTAX_THEMES: Record<PreviewSyntaxThemeId, Record<string, CSSProperties>> = {
  "one-light": oneLight as Record<string, CSSProperties>,
  "one-dark": oneDark as Record<string, CSSProperties>
};

// 默认代码块容器样式：复制到第三方平台时可保留视觉表现。
const DEFAULT_CODE_BLOCK_STYLE: CSSProperties = {
  margin: "16px 0",
  padding: "14px 16px",
  borderRadius: "10px",
  border: "1px solid #dbe2ea",
  boxShadow: "0 1px 2px rgba(15, 23, 42, 0.06)",
  overflowX: "auto",
  fontSize: "13px",
  lineHeight: 1.65,
  background: "#f8fafc"
};

// 默认代码块 code 标签样式：统一字体并提升可读性。
const DEFAULT_CODE_BLOCK_CODE_STYLE: CSSProperties = {
  fontFamily: "\"SFMono-Regular\", Menlo, Monaco, Consolas, \"Liberation Mono\", \"Courier New\", monospace"
};

// 默认行内代码样式：确保没有 fenced block 时也有可视化区分。
const DEFAULT_INLINE_CODE_STYLE: CSSProperties = {
  padding: "1px 6px",
  borderRadius: "5px",
  border: "1px solid #dbe2ea",
  background: "#f1f5f9",
  color: "#0f172a",
  fontSize: "0.92em",
  fontFamily: "\"SFMono-Regular\", Menlo, Monaco, Consolas, \"Liberation Mono\", \"Courier New\", monospace"
};

// 内置主题模板列表：支持从菜单直接切换。
const PREVIEW_THEME_TEMPLATES: PreviewThemeTemplate[] = [
  {
    id: "default",
    name: "内置默认",
    description: "通用文档风格",
    variables: {
      "--pd-preview-padding": "30px",
      "--pd-preview-font-family":
        "Optima-Regular, Optima, PingFangSC-light, PingFangTC-light, \"PingFang SC\", Cambria, Cochin, Georgia, Times, \"Times New Roman\", serif",
      "--pd-preview-text-color": "rgb(89, 89, 89)",
      "--pd-preview-link-color": "rgb(71, 193, 168)",
      "--pd-preview-inline-code-color": "rgb(71, 193, 168)",
      "--pd-preview-font-size": "16px",
      "--pd-preview-line-height": "26px",
      "--pd-preview-word-spacing": "3px",
      "--pd-preview-letter-spacing": "0.02em",
      "--pd-preview-paragraph-margin-top": "5px",
      "--pd-preview-paragraph-margin-bottom": "5px",
      "--pd-preview-paragraph-indent": "2em",
      "--pd-preview-title-color": "rgb(89, 89, 89)",
      "--pd-preview-h2-border-color": "rgb(89, 89, 89)",
      "--pd-preview-blockquote-text-color": "#666666",
      "--pd-preview-blockquote-mark-color": "#555555",
      "--pd-preview-blockquote-background": "#f8fafc",
      "--pd-preview-blockquote-border-color": "#cbd5e1",
      "--pd-preview-strong-color": "rgb(71, 193, 168)",
      "--pd-preview-em-color": "rgb(71, 193, 168)",
      "--pd-preview-hr-color": "#cbd5e1",
      "--pd-preview-image-width": "100%",
      "--pd-preview-table-font-size": "14px",
      "--pd-preview-table-border-color": "#dbe2ea",
      "--pd-preview-table-cell-padding": "10px 12px"
    },
    syntaxTheme: "one-light",
    codeBlockStyle: { ...DEFAULT_CODE_BLOCK_STYLE },
    codeBlockCodeStyle: { ...DEFAULT_CODE_BLOCK_CODE_STYLE },
    inlineCodeStyle: { ...DEFAULT_INLINE_CODE_STYLE }
  },
  {
    id: "newspaper",
    name: "报刊主题",
    description: "更适合长文阅读",
    variables: {
      "--pd-preview-padding": "34px",
      "--pd-preview-font-family":
        "\"Noto Serif SC\", \"Source Han Serif SC\", Songti SC, SimSun, Georgia, serif",
      "--pd-preview-text-color": "#334155",
      "--pd-preview-link-color": "#0f766e",
      "--pd-preview-inline-code-color": "#0f766e",
      "--pd-preview-font-size": "16px",
      "--pd-preview-line-height": "30px",
      "--pd-preview-word-spacing": "1px",
      "--pd-preview-letter-spacing": "0.01em",
      "--pd-preview-paragraph-margin-top": "8px",
      "--pd-preview-paragraph-margin-bottom": "8px",
      "--pd-preview-paragraph-indent": "2em",
      "--pd-preview-title-color": "#0f172a",
      "--pd-preview-h2-border-color": "#334155",
      "--pd-preview-blockquote-text-color": "#475569",
      "--pd-preview-blockquote-mark-color": "#334155",
      "--pd-preview-blockquote-background": "#f8fafc",
      "--pd-preview-blockquote-border-color": "#94a3b8",
      "--pd-preview-strong-color": "#0f766e",
      "--pd-preview-em-color": "#0f766e",
      "--pd-preview-hr-color": "#94a3b8",
      "--pd-preview-image-width": "100%",
      "--pd-preview-table-font-size": "14px",
      "--pd-preview-table-border-color": "#cbd5e1",
      "--pd-preview-table-cell-padding": "10px 12px"
    },
    syntaxTheme: "one-light",
    codeBlockStyle: {
      ...DEFAULT_CODE_BLOCK_STYLE,
      borderRadius: "8px",
      border: "1px solid #cbd5e1",
      background: "#f8fafc"
    },
    codeBlockCodeStyle: {
      ...DEFAULT_CODE_BLOCK_CODE_STYLE,
      fontFamily: "\"Source Code Pro\", \"SFMono-Regular\", Menlo, Monaco, Consolas, monospace"
    },
    inlineCodeStyle: {
      ...DEFAULT_INLINE_CODE_STYLE,
      background: "#ecfeff",
      border: "1px solid #99f6e4",
      color: "#115e59"
    }
  },
  {
    id: "clean-tech",
    name: "清爽技术",
    description: "偏开发文档排版",
    variables: {
      "--pd-preview-padding": "26px",
      "--pd-preview-font-family":
        "\"JetBrains Mono\", \"SFMono-Regular\", Menlo, Monaco, Consolas, \"Courier New\", monospace",
      "--pd-preview-text-color": "#334155",
      "--pd-preview-link-color": "#0ea5e9",
      "--pd-preview-inline-code-color": "#0284c7",
      "--pd-preview-font-size": "14px",
      "--pd-preview-line-height": "24px",
      "--pd-preview-word-spacing": "0",
      "--pd-preview-letter-spacing": "0",
      "--pd-preview-paragraph-margin-top": "6px",
      "--pd-preview-paragraph-margin-bottom": "6px",
      "--pd-preview-paragraph-indent": "0",
      "--pd-preview-title-color": "#0f172a",
      "--pd-preview-h2-border-color": "#475569",
      "--pd-preview-blockquote-text-color": "#475569",
      "--pd-preview-blockquote-mark-color": "#64748b",
      "--pd-preview-blockquote-background": "#f1f5f9",
      "--pd-preview-blockquote-border-color": "#cbd5e1",
      "--pd-preview-strong-color": "#0369a1",
      "--pd-preview-em-color": "#0284c7",
      "--pd-preview-hr-color": "#cbd5e1",
      "--pd-preview-image-width": "100%",
      "--pd-preview-table-font-size": "13px",
      "--pd-preview-table-border-color": "#cbd5e1",
      "--pd-preview-table-cell-padding": "8px 10px"
    },
    syntaxTheme: "one-dark",
    codeBlockStyle: {
      ...DEFAULT_CODE_BLOCK_STYLE,
      borderRadius: "12px",
      border: "1px solid #1e293b",
      boxShadow: "0 8px 18px rgba(2, 6, 23, 0.2)",
      background: "#0f172a"
    },
    codeBlockCodeStyle: {
      ...DEFAULT_CODE_BLOCK_CODE_STYLE,
      color: "#dbeafe"
    },
    inlineCodeStyle: {
      ...DEFAULT_INLINE_CODE_STYLE,
      background: "#0f172a",
      border: "1px solid #334155",
      color: "#7dd3fc"
    }
  }
];

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
  const anchorIndex = props["data-anchor-index"];
  if (typeof sourceLine === "string") {
    anchors["data-source-line"] = sourceLine;
  }
  if (typeof sourceOffset === "string") {
    anchors["data-source-offset"] = sourceOffset;
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

// 按主题 ID 返回主题模板；找不到时回退默认主题。
function resolvePreviewTheme(themeId: string): PreviewThemeTemplate {
  const foundTheme = PREVIEW_THEME_TEMPLATES.find((theme) => theme.id === themeId);
  return foundTheme ?? PREVIEW_THEME_TEMPLATES[0];
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
        const hasSourceLine = typeof sourceLine === "number" && Number.isFinite(sourceLine);
        const hasSourceOffset = typeof sourceOffset === "number" && Number.isFinite(sourceOffset);
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
  const renderedHtml = parser.render(markdownContent);
  if (typeof DOMParser === "undefined") {
    return markdownContent;
  }
  const htmlDocument = new DOMParser().parseFromString(renderedHtml, "text/html");
  return htmlDocument.body.textContent ?? "";
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
      const editorElement = editorScrollerRef.current;
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
    []
  );

  // 执行一次单向同步，并用锁避免对端 scroll 反向触发。
  const syncFromSource = useCallback(
    (sourceName: ScrollSource) => {
      if (syncingRef.current) {
        return;
      }
      const editorElement = editorScrollerRef.current;
      const previewElement = previewScrollerRef.current;
      if (!editorElement || !previewElement) {
        return;
      }

      // 同步阶段写入锁并记录来源。
      syncingRef.current = true;
      lastScrollSourceRef.current = sourceName;
      if (sourceName === "editor") {
        previewElement.scrollTop = getMappedTargetScrollTop("editor");
      } else {
        editorElement.scrollTop = getMappedTargetScrollTop("preview");
      }
      window.requestAnimationFrame(() => {
        syncingRef.current = false;
      });
    },
    [getMappedTargetScrollTop]
  );

  // 映射表重建后，按最近来源做一次回对齐。
  const resyncFromLastSource = useCallback(() => {
    if (syncingRef.current) {
      return;
    }
    const editorElement = editorScrollerRef.current;
    const previewElement = previewScrollerRef.current;
    if (!editorElement || !previewElement) {
      return;
    }
    syncingRef.current = true;
    // 回对齐时不改 lastScrollSourceRef，保持最近来源语义不变。
    if (lastScrollSourceRef.current === "editor") {
      previewElement.scrollTop = getMappedTargetScrollTop("editor");
    } else {
      editorElement.scrollTop = getMappedTargetScrollTop("preview");
    }
    window.requestAnimationFrame(() => {
      syncingRef.current = false;
    });
  }, [getMappedTargetScrollTop]);

  // 重建 block 级锚点映射表：source offset -> editorY 与 previewY。
  const rebuildScrollAnchors = useCallback(() => {
    const editorElement = editorScrollerRef.current;
    const previewElement = previewScrollerRef.current;
    const editorView = editorViewRef.current;
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

    for (const node of anchorNodes) {
      const rawLine = node.dataset.sourceLine;
      const rawOffset = node.dataset.sourceOffset;
      let editorY: number | null = null;
      if (rawLine) {
        const parsedLine = Number(rawLine);
        if (Number.isFinite(parsedLine)) {
          const lineNumber = clamp(Math.floor(parsedLine), 1, editorView.state.doc.lines);
          const lineFrom = editorView.state.doc.line(lineNumber).from;
          // 优先按源码行号映射，避免 offset 误差导致锚点错位。
          editorY = clamp(editorView.lineBlockAt(lineFrom).top, 0, editorMaxScrollable);
        }
      }
      if (editorY === null) {
        if (!rawOffset) {
          continue;
        }
        const parsedOffset = Number(rawOffset);
        if (!Number.isFinite(parsedOffset)) {
          continue;
        }
        const sourceOffset = clamp(Math.floor(parsedOffset), 0, docLength);
        // 无行号时退回 offset 映射。
        editorY = clamp(editorView.lineBlockAt(sourceOffset).top, 0, editorMaxScrollable);
      }

      // 将节点视口坐标转换为容器内容坐标。
      const previewY = clamp(
        node.getBoundingClientRect().top - previewRect.top + previewElement.scrollTop,
        0,
        previewMaxScrollable
      );

      rawAnchors.push({ editorY, previewY });
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
  }, []);

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
  // remark 插件顺序：先 GFM，再注入锚点属性。
  const remarkPlugins = useMemo(() => [remarkGfm, remarkBlockAnchorPlugin], []);
  // 自定义 Markdown 渲染器：代码块走高亮组件，行内代码走轻量内联样式。
  const markdownComponents = useMemo<Components>(
    () => {
      // 读取当前主题下的代码渲染配置。
      const activeTheme = resolvePreviewTheme(activePreviewThemeId);
      const syntaxTheme =
        PREVIEW_SYNTAX_THEMES[activeTheme.syntaxTheme] ?? PREVIEW_SYNTAX_THEMES["one-light"];

      return {
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
    [activePreviewThemeId]
  );
  // markdown-it 仅用于“去语法后的文字统计”。
  const markdownTextParser = useMemo(
    () =>
      new MarkdownIt({
        html: false,
        linkify: true,
        typographer: false
      }),
    []
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
    scheduleRebuildScrollAnchors();
  }, [content, scheduleRebuildScrollAnchors]);

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
    };
  }, []);

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
            <ReactMarkdown remarkPlugins={remarkPlugins} components={markdownComponents}>
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
          <span>最后保存时间：{lastSavedTimeLabel}</span>
          <span>字数：{plainTextCount}</span>
        </div>
      </footer>
    </div>
  );
}
