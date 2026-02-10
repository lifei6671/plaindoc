import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import { languages } from "@codemirror/language-data";
import { EditorView } from "@codemirror/view";
import CodeMirror from "@uiw/react-codemirror";
import { AlertCircle, CheckCircle2, LoaderCircle } from "lucide-react";
import MarkdownIt from "markdown-it";
import {
  Children,
  isValidElement,
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
import { oneLight } from "react-syntax-highlighter/dist/esm/styles/prism";
import remarkGfm from "remark-gfm";
import { ConflictError, getDataGateway, type TreeNode } from "./data-access";

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

// 代码块行内样式：复制到第三方平台时可保留视觉表现。
const INLINE_CODE_BLOCK_STYLE: CSSProperties = {
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

// 代码块 code 标签样式：统一字体并提升可读性。
const INLINE_CODE_BLOCK_CODE_STYLE: CSSProperties = {
  fontFamily: "\"SFMono-Regular\", Menlo, Monaco, Consolas, \"Liberation Mono\", \"Courier New\", monospace"
};

// 行内代码样式：确保没有 fenced block 时也有可视化区分。
const INLINE_CODE_STYLE: CSSProperties = {
  padding: "1px 6px",
  borderRadius: "5px",
  border: "1px solid #dbe2ea",
  background: "#f1f5f9",
  color: "#0f172a",
  fontSize: "0.92em",
  fontFamily: "\"SFMono-Regular\", Menlo, Monaco, Consolas, \"Liberation Mono\", \"Courier New\", monospace"
};

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
    () => ({
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
              ...INLINE_CODE_BLOCK_STYLE
            }}
          >
            {preChildren}
          </pre>
        );

        return (
          <SyntaxHighlighter
            language={language}
            style={oneLight}
            PreTag={PreTag}
            useInlineStyles
            wrapLongLines
            codeTagProps={{
              className: codeClassName,
              style: INLINE_CODE_BLOCK_CODE_STYLE
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
              ...INLINE_CODE_STYLE,
              ...(style ?? {})
            }}
            {...props}
          >
            {children}
          </code>
        );
      }
    }),
    []
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
      const markdownBody = previewElement.querySelector<HTMLElement>(".markdown-body");
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
      {/* 顶部状态栏。 */}
      <header className="header">
        <h1>PlainDoc</h1>
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
          className="pane preview-pane"
          // 使用稳定 ref 回调，保证滚动监听不会被重复拆装。
          ref={handlePreviewScrollerRef}
        >
          <article className="markdown-body">
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
