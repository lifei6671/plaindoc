import {
  Children,
  isValidElement,
  useEffect,
  useRef,
  useState,
  type ComponentPropsWithoutRef,
  type ReactNode
} from "react";
import type { Components } from "react-markdown";
import SyntaxHighlighter from "react-syntax-highlighter/dist/esm/prism-light";
import prismBash from "react-syntax-highlighter/dist/esm/languages/prism/bash";
import prismC from "react-syntax-highlighter/dist/esm/languages/prism/c";
import prismClike from "react-syntax-highlighter/dist/esm/languages/prism/clike";
import prismCpp from "react-syntax-highlighter/dist/esm/languages/prism/cpp";
import prismCss from "react-syntax-highlighter/dist/esm/languages/prism/css";
import prismDiff from "react-syntax-highlighter/dist/esm/languages/prism/diff";
import prismDocker from "react-syntax-highlighter/dist/esm/languages/prism/docker";
import prismGo from "react-syntax-highlighter/dist/esm/languages/prism/go";
import prismIni from "react-syntax-highlighter/dist/esm/languages/prism/ini";
import prismJava from "react-syntax-highlighter/dist/esm/languages/prism/java";
import prismJavascript from "react-syntax-highlighter/dist/esm/languages/prism/javascript";
import prismJson from "react-syntax-highlighter/dist/esm/languages/prism/json";
import prismJsx from "react-syntax-highlighter/dist/esm/languages/prism/jsx";
import prismMarkdown from "react-syntax-highlighter/dist/esm/languages/prism/markdown";
import prismMarkup from "react-syntax-highlighter/dist/esm/languages/prism/markup";
import prismPython from "react-syntax-highlighter/dist/esm/languages/prism/python";
import prismRust from "react-syntax-highlighter/dist/esm/languages/prism/rust";
import prismShellSession from "react-syntax-highlighter/dist/esm/languages/prism/shell-session";
import prismSql from "react-syntax-highlighter/dist/esm/languages/prism/sql";
import prismToml from "react-syntax-highlighter/dist/esm/languages/prism/toml";
import prismTsx from "react-syntax-highlighter/dist/esm/languages/prism/tsx";
import prismTypescript from "react-syntax-highlighter/dist/esm/languages/prism/typescript";
import prismYaml from "react-syntax-highlighter/dist/esm/languages/prism/yaml";
import {
  extractCodeText,
  isTocMarkerText,
  pickAnchorDataAttributes,
  resolveCodeLanguage
} from "./markdown-utils";
import type { TocItem } from "./types";
import {
  PREVIEW_SYNTAX_THEMES,
  type PreviewThemeTemplate
} from "../preview-themes";

// 使用 Prism light 版本按需注册语言，避免触发 refractor/all 的全量语言加载。
const PRISM_LANGUAGE_REGISTRATIONS = [
  ["markup", prismMarkup],
  ["markdown", prismMarkdown],
  ["bash", prismBash],
  ["shell-session", prismShellSession],
  ["diff", prismDiff],
  ["json", prismJson],
  ["yaml", prismYaml],
  ["toml", prismToml],
  ["ini", prismIni],
  ["sql", prismSql],
  ["css", prismCss],
  ["clike", prismClike],
  ["javascript", prismJavascript],
  ["jsx", prismJsx],
  ["typescript", prismTypescript],
  ["tsx", prismTsx],
  ["c", prismC],
  ["cpp", prismCpp],
  ["go", prismGo],
  ["java", prismJava],
  ["python", prismPython],
  ["rust", prismRust],
  ["docker", prismDocker]
] as const;

let hasRegisteredPrismLanguages = false;

function ensurePrismLanguagesRegistered() {
  if (hasRegisteredPrismLanguages) {
    return;
  }

  for (const [name, language] of PRISM_LANGUAGE_REGISTRATIONS) {
    SyntaxHighlighter.registerLanguage(name, language);
  }

  // 常见 fenced code block 别名，统一映射到已注册语言。
  SyntaxHighlighter.alias("markup", ["html", "xml", "svg"]);
  SyntaxHighlighter.alias("javascript", ["js"]);
  SyntaxHighlighter.alias("typescript", ["ts"]);
  SyntaxHighlighter.alias("bash", ["sh", "shell", "zsh"]);
  SyntaxHighlighter.alias("yaml", ["yml"]);
  SyntaxHighlighter.alias("markdown", ["md"]);

  hasRegisteredPrismLanguages = true;
}

ensurePrismLanguagesRegistered();

interface BuildMarkdownComponentsOptions {
  activePreviewTheme: PreviewThemeTemplate;
  tocItems: TocItem[];
  onTocNavigate: (item: TocItem) => void;
}

interface MermaidBlockProps {
  code: string;
  anchorDataAttributes: Record<string, string>;
}

type MermaidModule = typeof import("mermaid")["default"];

interface MermaidRenderResult {
  errorMessage: string | null;
  svg: string;
}

interface MermaidRenderCacheEntry extends MermaidRenderResult {
  pendingPromise?: Promise<MermaidRenderResult>;
}

const MERMAID_RENDER_CACHE_MAX_ENTRIES = 200;

let mermaidModulePromise: Promise<MermaidModule> | null = null;
let mermaidRenderSequence = 0;

const mermaidRenderCache = new Map<string, MermaidRenderCacheEntry>();

function setMermaidRenderCacheEntry(cacheKey: string, entry: MermaidRenderCacheEntry) {
  if (mermaidRenderCache.has(cacheKey)) {
    mermaidRenderCache.delete(cacheKey);
  }
  mermaidRenderCache.set(cacheKey, entry);
  while (mermaidRenderCache.size > MERMAID_RENDER_CACHE_MAX_ENTRIES) {
    const oldestKey = mermaidRenderCache.keys().next().value;
    if (!oldestKey) {
      break;
    }
    mermaidRenderCache.delete(oldestKey);
  }
}

async function loadMermaidModule(): Promise<MermaidModule> {
  if (!mermaidModulePromise) {
    mermaidModulePromise = import("mermaid")
      .then((module) => {
        module.default.initialize({
          startOnLoad: false,
          securityLevel: "strict",
          suppressErrorRendering: true
        });
        return module.default;
      })
      .catch((error) => {
        // 动态导入失败时清空模块级 promise，允许后续重新尝试加载。
        mermaidModulePromise = null;
        throw error;
      });
  }
  return mermaidModulePromise;
}

function buildMermaidCacheKey(
  anchorDataAttributes: Record<string, string>,
  source: string
): string {
  const anchorIndex = anchorDataAttributes["data-anchor-index"]?.trim() || "mermaid";
  return `${anchorIndex}:${source}`;
}

async function renderMermaidWithCache(
  cacheKey: string,
  source: string
): Promise<MermaidRenderResult> {
  const cachedEntry = mermaidRenderCache.get(cacheKey);
  if (cachedEntry?.pendingPromise) {
    return cachedEntry.pendingPromise;
  }
  if (cachedEntry && (cachedEntry.svg || cachedEntry.errorMessage)) {
    return {
      errorMessage: cachedEntry.errorMessage,
      svg: cachedEntry.svg
    };
  }

  const pendingPromise = (async () => {
    try {
      const mermaid = await loadMermaidModule();
      mermaidRenderSequence += 1;
      const renderResult = await mermaid.render(
        `plaindoc-mermaid-${mermaidRenderSequence}`,
        source
      );
      const successResult: MermaidRenderResult = {
        errorMessage: null,
        svg: renderResult.svg
      };
      setMermaidRenderCacheEntry(cacheKey, successResult);
      return successResult;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      // 渲染失败不写入长期缓存，避免一次瞬时错误把同源图表永久锁死。
      mermaidRenderCache.delete(cacheKey);
      const failureResult: MermaidRenderResult = {
        errorMessage: `Mermaid 渲染失败：${message}`,
        svg: ""
      };
      return failureResult;
    }
  })();

  setMermaidRenderCacheEntry(cacheKey, {
    errorMessage: null,
    svg: "",
    pendingPromise
  });
  return pendingPromise;
}

// Mermaid 代码块渲染器：将 fenced mermaid 文本编译为 SVG，并保留锚点属性。
function MermaidBlock({ code, anchorDataAttributes }: MermaidBlockProps) {
  const source = code.trim();
  const cacheKey = buildMermaidCacheKey(anchorDataAttributes, source);
  const cachedEntry = source ? mermaidRenderCache.get(cacheKey) : undefined;
  const [renderedSvg, setRenderedSvg] = useState(cachedEntry?.svg ?? "");
  const [renderError, setRenderError] = useState<string | null>(cachedEntry?.errorMessage ?? null);
  const renderTicketRef = useRef(0);

  useEffect(() => {
    if (!source) {
      setRenderedSvg("");
      setRenderError("Mermaid 代码块为空，无法渲染图表。");
      return;
    }

    const currentCachedEntry = mermaidRenderCache.get(cacheKey);
    if (currentCachedEntry?.svg) {
      setRenderedSvg(currentCachedEntry.svg);
      setRenderError(currentCachedEntry.errorMessage);
      return;
    }
    if (currentCachedEntry?.errorMessage) {
      setRenderedSvg("");
      setRenderError(currentCachedEntry.errorMessage);
      return;
    }

    renderTicketRef.current += 1;
    const renderTicket = renderTicketRef.current;
    // 重新渲染期间保留旧 SVG，避免预览高度先塌缩再撑开导致抖动。
    setRenderError(null);

    void renderMermaidWithCache(cacheKey, source).then((renderResult) => {
      if (renderTicketRef.current !== renderTicket) {
        return;
      }
      setRenderedSvg(renderResult.svg);
      setRenderError(renderResult.errorMessage);
    });
  }, [cacheKey, source]);

  if (renderError) {
    return (
      <div className="mermaid-block mermaid-block--error" {...anchorDataAttributes}>
        <p className="mermaid-block__error-message">{renderError}</p>
        <pre className="mermaid-block__fallback">
          <code>{code}</code>
        </pre>
      </div>
    );
  }

  if (!renderedSvg) {
    return (
      <div className="mermaid-block mermaid-block--loading" {...anchorDataAttributes}>
        Mermaid 图渲染中...
      </div>
    );
  }

  return (
    <div className="mermaid-block" {...anchorDataAttributes}>
      <div
        className="mermaid-block__diagram"
        // Mermaid 官方渲染结果为可信 SVG 字符串；此处按需注入到预览 DOM。
        dangerouslySetInnerHTML={{ __html: renderedSvg }}
      />
    </div>
  );
}

// 统一渲染标题结构，便于主题使用 prefix/content/suffix 三段式样式。
function renderDecoratedHeading(
  Tag: "h1" | "h2" | "h3" | "h4" | "h5" | "h6",
  children: ReactNode,
  props: Record<string, unknown>
) {
  return (
    <Tag {...props}>
      <span className="prefix" aria-hidden="true" />
      <span className="content">{children}</span>
      <span className="suffix" aria-hidden="true" />
    </Tag>
  );
}

// 构建 Markdown 渲染组件：拆分出 App，降低主文件复杂度。
export function buildMarkdownComponents({
  activePreviewTheme,
  tocItems,
  onTocNavigate
}: BuildMarkdownComponentsOptions): Components {
  const syntaxTheme =
    PREVIEW_SYNTAX_THEMES[activePreviewTheme.syntaxTheme] ?? PREVIEW_SYNTAX_THEMES["one-light"];

  return {
    // 标题统一渲染为 prefix/content/suffix 结构，便于主题扩展。
    h1: ({ node: _node, children, ...props }) => renderDecoratedHeading("h1", children, props),
    h2: ({ node: _node, children, ...props }) => renderDecoratedHeading("h2", children, props),
    h3: ({ node: _node, children, ...props }) => renderDecoratedHeading("h3", children, props),
    h4: ({ node: _node, children, ...props }) => renderDecoratedHeading("h4", children, props),
    h5: ({ node: _node, children, ...props }) => renderDecoratedHeading("h5", children, props),
    h6: ({ node: _node, children, ...props }) => renderDecoratedHeading("h6", children, props),
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
                      onClick={() => onTocNavigate(item)}
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

      if (language.toLowerCase() === "mermaid") {
        return <MermaidBlock code={codeText} anchorDataAttributes={anchorDataAttributes} />;
      }

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
            ...activePreviewTheme.codeBlockStyle
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
            style: activePreviewTheme.codeBlockCodeStyle
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
            ...activePreviewTheme.inlineCodeStyle,
            ...(style ?? {})
          }}
          {...props}
        >
          {children}
        </code>
      );
    }
  };
}
