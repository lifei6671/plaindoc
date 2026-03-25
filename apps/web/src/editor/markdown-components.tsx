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
import {
  isExternalHTTPLink,
  mergeExternalLinkRel,
  normalizeLinkRequestOrigin
} from "./external-link";
import {
  buildMermaidCacheKey,
  MERMAID_RENDER_CACHE_MAX_ENTRIES,
  type MermaidRenderCacheEntry,
  type MermaidRenderResult
} from "./mermaid-shared";
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
  requestOrigin?: string;
  preRenderedMermaidMap?: ReadonlyMap<string, MermaidRenderResult>;
  includeMermaidSourcePayload?: boolean;
}

interface MermaidBlockProps {
  code: string;
  anchorDataAttributes: Record<string, string>;
  preRenderedResult?: MermaidRenderResult;
  includeSourcePayload?: boolean;
}

type MermaidModule = typeof import("mermaid")["default"];

interface CodeBlockCopyButtonProps {
  codeText: string;
}

const CODE_COPY_SUCCESS_FEEDBACK_MS = 1800;

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

async function copyTextToClipboard(text: string): Promise<void> {
  if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }
  if (typeof document === "undefined") {
    throw new Error("clipboard unavailable");
  }

  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "true");
  textarea.style.position = "fixed";
  textarea.style.top = "-9999px";
  textarea.style.left = "-9999px";
  document.body.appendChild(textarea);
  textarea.select();
  textarea.setSelectionRange(0, textarea.value.length);
  const copied = document.execCommand("copy");
  document.body.removeChild(textarea);
  if (!copied) {
    throw new Error("copy failed");
  }
}

function CopyIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <rect x="5" y="3.5" width="7" height="9" rx="1.5" />
      <path d="M3.5 10.5V5A1.5 1.5 0 0 1 5 3.5" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true">
      <path d="m3.5 8.5 2.6 2.6 6.4-6.6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function CodeBlockCopyButton({ codeText }: CodeBlockCopyButtonProps) {
  const [copyState, setCopyState] = useState<"idle" | "success">("idle");
  const resetTimerRef = useRef<number | null>(null);

  useEffect(() => {
    return () => {
      if (resetTimerRef.current !== null) {
        window.clearTimeout(resetTimerRef.current);
      }
    };
  }, []);

  const handleClick = async () => {
    if (!codeText) {
      return;
    }
    try {
      await copyTextToClipboard(codeText);
      setCopyState("success");
      if (resetTimerRef.current !== null) {
        window.clearTimeout(resetTimerRef.current);
      }
      resetTimerRef.current = window.setTimeout(() => {
        setCopyState("idle");
        resetTimerRef.current = null;
      }, CODE_COPY_SUCCESS_FEEDBACK_MS);
    } catch {
      setCopyState("idle");
    }
  };

  const isSuccess = copyState === "success";

  return (
    <button
      type="button"
      className="code-block-copy-button"
      data-code-copy-button="1"
      data-copy-state={copyState}
      aria-label={isSuccess ? "复制成功" : "复制代码"}
      onClick={handleClick}
    >
      <span className="code-block-copy-button__icon code-block-copy-button__icon--copy">
        <CopyIcon />
      </span>
      <span className="code-block-copy-button__icon code-block-copy-button__icon--success">
        <CheckIcon />
      </span>
      <span className="code-block-copy-button__label code-block-copy-button__label--idle">复制</span>
      <span className="code-block-copy-button__label code-block-copy-button__label--success">
        复制成功
      </span>
    </button>
  );
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
function MermaidBlock({
  code,
  anchorDataAttributes,
  preRenderedResult,
  includeSourcePayload = false
}: MermaidBlockProps) {
  const source = code.trim();
  const cacheKey = buildMermaidCacheKey(anchorDataAttributes, source);
  const cachedEntry = source ? (preRenderedResult ?? mermaidRenderCache.get(cacheKey)) : undefined;
  const [renderedSvg, setRenderedSvg] = useState(cachedEntry?.svg ?? "");
  const [renderError, setRenderError] = useState<string | null>(cachedEntry?.errorMessage ?? null);
  const renderTicketRef = useRef(0);
  const mermaidDataAttributes = {
    ...anchorDataAttributes,
    "data-reader-hook": "mermaid",
    "data-reader-mermaid-status": renderError ? "error" : renderedSvg ? "ready" : "loading"
  };
  const mermaidSourceNode = includeSourcePayload ? (
    <pre hidden data-reader-mermaid-source="1">
      <code>{code}</code>
    </pre>
  ) : null;

  useEffect(() => {
    if (!source) {
      setRenderedSvg("");
      setRenderError("Mermaid 代码块为空，无法渲染图表。");
      return;
    }

    if (preRenderedResult) {
      setRenderedSvg(preRenderedResult.svg);
      setRenderError(preRenderedResult.errorMessage);
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
  }, [cacheKey, preRenderedResult, source]);

  if (renderError) {
    return (
      <div className="mermaid-block mermaid-block--error" {...mermaidDataAttributes}>
        {mermaidSourceNode}
        <p className="mermaid-block__error-message" data-reader-mermaid-message="1">
          {renderError}
        </p>
        <pre className="mermaid-block__fallback" data-reader-mermaid-fallback="1">
          <code>{code}</code>
        </pre>
      </div>
    );
  }

  if (!renderedSvg) {
    return (
      <div className="mermaid-block mermaid-block--loading" {...mermaidDataAttributes}>
        {mermaidSourceNode}
        <p data-reader-mermaid-message="1">Mermaid 图渲染中...</p>
      </div>
    );
  }

  return (
    <div className="mermaid-block" {...mermaidDataAttributes}>
      {mermaidSourceNode}
      <div
        className="mermaid-block__diagram"
        data-reader-mermaid-diagram="1"
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
  onTocNavigate,
  requestOrigin,
  preRenderedMermaidMap,
  includeMermaidSourcePayload = false
}: BuildMarkdownComponentsOptions): Components {
  const syntaxTheme =
    PREVIEW_SYNTAX_THEMES[activePreviewTheme.syntaxTheme] ?? PREVIEW_SYNTAX_THEMES["one-light"];
  const normalizedRequestOrigin = normalizeLinkRequestOrigin(requestOrigin);

  return {
    // 标题统一渲染为 prefix/content/suffix 结构，便于主题扩展。
    h1: ({ node: _node, children, ...props }) => renderDecoratedHeading("h1", children, props),
    h2: ({ node: _node, children, ...props }) => renderDecoratedHeading("h2", children, props),
    h3: ({ node: _node, children, ...props }) => renderDecoratedHeading("h3", children, props),
    h4: ({ node: _node, children, ...props }) => renderDecoratedHeading("h4", children, props),
    h5: ({ node: _node, children, ...props }) => renderDecoratedHeading("h5", children, props),
    h6: ({ node: _node, children, ...props }) => renderDecoratedHeading("h6", children, props),
    a: ({ node: _node, href, rel, target, children, ...props }) => {
      const normalizedHref = typeof href === "string" ? href : "";
      const shouldOpenInNewWindow = isExternalHTTPLink(normalizedHref, normalizedRequestOrigin);
      const resolvedRel = shouldOpenInNewWindow
        ? mergeExternalLinkRel(typeof rel === "string" ? rel : "")
        : rel;
      return (
        <a
          href={href}
          rel={resolvedRel}
          target={shouldOpenInNewWindow ? "_blank" : target}
          {...props}
        >
          {children}
        </a>
      );
    },
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
        const mermaidCacheKey = buildMermaidCacheKey(anchorDataAttributes, codeText.trim());
        return (
          <MermaidBlock
            code={codeText}
            anchorDataAttributes={anchorDataAttributes}
            preRenderedResult={preRenderedMermaidMap?.get(mermaidCacheKey)}
            includeSourcePayload={includeMermaidSourcePayload}
          />
        );
      }

      // 自定义 PreTag：统一挂样式类，并把源码文本标记给阅读页脚本做复制。
      const PreTag = ({
        children: preChildren,
        style: preStyle,
        ...preTagProps
      }: ComponentPropsWithoutRef<"pre">) => (
        <pre
          {...preTagProps}
          className={[
            "code-block-copy-shell__surface",
            typeof preTagProps.className === "string" ? preTagProps.className : ""
          ]
            .filter(Boolean)
            .join(" ")}
          style={{
            ...(preStyle ?? {}),
            ...activePreviewTheme.codeBlockStyle
          }}
        >
          {preChildren}
        </pre>
      );
      const codeTagProps = {
        className: codeClassName,
        style: activePreviewTheme.codeBlockCodeStyle,
        "data-code-copy-source": "1"
      };

      return (
        <div className="code-block-copy-shell" {...anchorDataAttributes}>
          <CodeBlockCopyButton codeText={codeText} />
          <SyntaxHighlighter
            language={language}
            style={syntaxTheme}
            PreTag={PreTag}
            useInlineStyles
            wrapLongLines
            codeTagProps={codeTagProps as unknown as ComponentPropsWithoutRef<"code">}
          >
            {codeText}
          </SyntaxHighlighter>
        </div>
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
