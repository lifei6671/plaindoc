import {
  Children,
  isValidElement,
  type ComponentPropsWithoutRef,
  type ReactNode
} from "react";
import type { Components } from "react-markdown";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import {
  extractCodeText,
  isTocMarkerText,
  pickAnchorDataAttributes,
  resolveCodeLanguage
} from "./markdown-utils";
import type { TocItem } from "./types";
import {
  PREVIEW_SYNTAX_THEMES,
  resolvePreviewTheme
} from "../preview-themes";

interface BuildMarkdownComponentsOptions {
  activePreviewThemeId: string;
  tocItems: TocItem[];
  onTocNavigate: (item: TocItem) => void;
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
  activePreviewThemeId,
  tocItems,
  onTocNavigate
}: BuildMarkdownComponentsOptions): Components {
  // 读取当前主题下的代码渲染配置。
  const activeTheme = resolvePreviewTheme(activePreviewThemeId);
  const syntaxTheme =
    PREVIEW_SYNTAX_THEMES[activeTheme.syntaxTheme] ?? PREVIEW_SYNTAX_THEMES["one-light"];

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
}
