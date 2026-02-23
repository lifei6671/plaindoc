import katexStyleText from "katex/dist/katex.min.css?inline";
import { ChevronDown } from "lucide-react";
import { renderToStaticMarkup } from "react-dom/server";
import ReactMarkdown from "react-markdown";
import appStyleText from "../styles.css?inline";
import { PREVIEW_BODY_CLASS, PREVIEW_BODY_ID } from "../editor/constants";
import { buildPreviewThemeStyleText, getPreviewThemeClassName } from "../editor/preview-style";
import {
  BUILTIN_PREVIEW_THEME_TEMPLATES,
  DEFAULT_PREVIEW_THEME_TEMPLATE,
  resolvePreviewTheme
} from "../preview-themes";
import { buildReaderMarkdownRenderer } from "./markdown-shared";
import type { ReaderPagePayload, ReaderTreeNode } from "./ssr-types";

const READER_BASE_STYLE = `
:root {
  color-scheme: light;
}
* {
  box-sizing: border-box;
}
body {
  margin: 0;
  color: #1f2937;
  background: #f5f7fb;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", sans-serif;
  overflow: hidden;
}
a {
  color: inherit;
}
.reader-layout {
  display: grid;
  grid-template-columns: 280px minmax(0, 1fr);
  height: 100vh;
  overflow: hidden;
}
.reader-sidebar {
  border-right: 1px solid #dbe2ea;
  background: #fff;
  padding: 18px 14px;
  position: sticky;
  top: 0;
  align-self: stretch;
  height: 100vh;
  overflow-y: auto;
  overflow-x: hidden;
}
.reader-sidebar__title {
  margin: 0 0 14px;
  font-size: 15px;
  font-weight: 700;
  color: #111827;
}
.reader-tree,
.reader-tree__children {
  margin: 0;
  padding: 0;
  list-style: none;
}
.reader-tree__children {
  margin-top: 1px;
}
.reader-tree__item {
  display: block;
  margin: 0;
  padding: 0;
}
.reader-tree__details {
  margin: 0;
  padding: 0;
}
.reader-tree__summary {
  display: block;
  list-style: none;
  cursor: pointer;
}
.reader-tree__summary::-webkit-details-marker {
  display: none;
}
.reader-tree__row {
  display: flex;
  min-height: 36px;
  width: 100%;
  align-items: center;
  border-radius: 10px;
  padding-right: 8px;
  color: #2f2f30;
  text-decoration: none;
  font-size: 14px;
}
.reader-tree__row:not(.reader-tree__row--active):hover {
  background: #e8e8ea;
}
.reader-tree__row--active {
  background: #d9dade;
}
.reader-tree__arrow {
  display: inline-flex;
  height: 18px;
  width: 18px;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  color: #727679;
}
.reader-tree__arrow--expandable {
  transition: transform 0.15s ease;
}
.reader-tree__details:not([open]) .reader-tree__arrow--expandable {
  transform: rotate(-90deg);
}
.reader-tree__arrow--empty {
  opacity: 0;
}
.reader-tree__label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  line-height: 1.3;
  color: #2f2f30;
}
.reader-tree__label-link {
  text-decoration: none;
  color: inherit;
}
.reader-tree__label-link:hover {
  text-decoration: none;
}
.reader-tree__label--folder {
  font-weight: 600;
}
.reader-tree__label--active {
  font-weight: 600;
  color: #1d4ed8;
}
.reader-main {
  min-width: 0;
  height: 100vh;
  padding: 26px;
  overflow-y: auto;
  overflow-x: hidden;
}
.reader-article-shell {
  max-width: 900px;
  margin: 0 auto;
  border: 1px solid #dbe2ea;
  border-radius: 16px;
  background: #fff;
  box-shadow: 0 10px 28px rgba(15, 23, 42, 0.06);
}
.reader-article-header {
  border-bottom: 1px solid #e5e7eb;
  padding: 22px 24px 12px;
}
.reader-article-title {
  margin: 0;
  font-size: 30px;
  line-height: 1.2;
  color: #0f172a;
}
.reader-article-meta {
  margin-top: 8px;
  color: #64748b;
  font-size: 12px;
}
#${PREVIEW_BODY_ID} {
  padding: 20px 24px 30px;
}
@media (max-width: 1024px) {
  body {
    overflow: auto;
  }
  .reader-layout {
    grid-template-columns: 1fr;
    height: auto;
    overflow: visible;
  }
  .reader-sidebar {
    border-right: 0;
    border-bottom: 1px solid #dbe2ea;
    position: static;
    height: auto;
    max-height: 38vh;
  }
  .reader-main {
    height: auto;
    overflow: visible;
    padding: 16px;
  }
  .reader-article-title {
    font-size: 24px;
  }
}
`;

interface SpaceReaderRenderHead {
  title: string;
  canonical: string;
}

interface SpaceReaderRenderMetrics {
  renderMs: number;
  payloadBytes: number;
}

export interface SpaceReaderRenderResult {
  html: string;
  head: SpaceReaderRenderHead;
  metrics: SpaceReaderRenderMetrics;
}

interface ReaderTreeProps {
  nodes: ReaderTreeNode[];
  spaceId: string;
  activeDocId: string;
  depth?: number;
}

const MINUTE_MS = 60 * 1000;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;
const WEEK_MS = 7 * DAY_MS;
const MONTH_MS = 30 * DAY_MS;
const YEAR_MS = 365 * DAY_MS;

function padToTwoDigits(value: number): string {
  return value < 10 ? `0${value}` : String(value);
}

function parseUpdatedAt(value: string): Date | null {
  const normalized = value.trim();
  if (!normalized) {
    return null;
  }
  const parsedDate = new Date(normalized);
  if (Number.isNaN(parsedDate.getTime())) {
    return null;
  }
  return parsedDate;
}

function formatAbsoluteDateTime(date: Date): string {
  return `${date.getFullYear()}-${padToTwoDigits(date.getMonth() + 1)}-${padToTwoDigits(date.getDate())} ${padToTwoDigits(date.getHours())}:${padToTwoDigits(date.getMinutes())}`;
}

function formatRelativeTime(date: Date, now: Date): string {
  const diffMs = Math.max(0, now.getTime() - date.getTime());
  if (diffMs < MINUTE_MS) {
    return "刚刚";
  }
  if (diffMs < HOUR_MS) {
    return `${Math.floor(diffMs / MINUTE_MS)}分钟前`;
  }
  if (diffMs < DAY_MS) {
    return `${Math.floor(diffMs / HOUR_MS)}小时前`;
  }
  if (diffMs < WEEK_MS) {
    return `${Math.floor(diffMs / DAY_MS)}天前`;
  }
  if (diffMs < MONTH_MS) {
    return `${Math.floor(diffMs / WEEK_MS)}周前`;
  }
  if (diffMs < YEAR_MS) {
    return `${Math.floor(diffMs / MONTH_MS)}个月前`;
  }
  return `${Math.floor(diffMs / YEAR_MS)}年前`;
}

function formatUpdatedMeta(updatedAtRaw: string, now: Date): string {
  const updatedAt = parseUpdatedAt(updatedAtRaw);
  if (!updatedAt) {
    return "最后编辑：时间未知";
  }
  const relative = formatRelativeTime(updatedAt, now);
  const absolute = formatAbsoluteDateTime(updatedAt);
  return `最后编辑：${relative}（${absolute}）`;
}

function hasActiveDocument(nodes: ReaderTreeNode[], activeDocId: string): boolean {
  return nodes.some((node) => {
    if (node.type === "doc" && node.id === activeDocId) {
      return true;
    }
    if (!node.children.length) {
      return false;
    }
    return hasActiveDocument(node.children, activeDocId);
  });
}

function normalizeTreeTitle(title: string, fallback: string): string {
  const trimmed = title.trim();
  return trimmed || fallback;
}

function ReaderTree({ nodes, spaceId, activeDocId, depth = 0 }: ReaderTreeProps) {
  if (!nodes.length) {
    return <p className="reader-tree__label reader-tree__label--folder">暂无目录</p>;
  }

  return (
    <ul className={depth > 0 ? "reader-tree__children" : "reader-tree"}>
      {nodes.map((node) => {
        const isActive = node.id === activeDocId;
        const isFolderNode = node.type === "folder";
        const hasChildren = node.children.length > 0;
        const isExpandable = isFolderNode || hasChildren;
        const isOpenByDefault = hasChildren && hasActiveDocument(node.children, activeDocId);
        const title = normalizeTreeTitle(node.title, isFolderNode ? "未命名目录" : "未命名文档");
        const rowStyle = {
          paddingLeft: `${8 + depth * 20}px`
        };
        const rowClassName = `reader-tree__row${isActive ? " reader-tree__row--active" : ""}`;
        const arrowClassName = `reader-tree__arrow${isExpandable ? " reader-tree__arrow--expandable" : " reader-tree__arrow--empty"}`;
        const labelClassName = `reader-tree__label${isFolderNode ? " reader-tree__label--folder" : ""}${isActive ? " reader-tree__label--active" : ""}`;
        const staticLabel = (
          <span className={labelClassName} title={title}>
            {title}
          </span>
        );
        const linkLabel = (
          <a
            className={`${labelClassName} reader-tree__label-link`}
            title={title}
            href={`/r/${encodeURIComponent(spaceId)}/${encodeURIComponent(node.id)}`}
          >
            {title}
          </a>
        );
        const rowContent = (
          <>
            <span className={arrowClassName} aria-hidden="true">
              {isExpandable ? <ChevronDown size={15} /> : null}
            </span>
            {isExpandable && !isFolderNode && !isActive ? linkLabel : staticLabel}
          </>
        );

        if (isExpandable) {
          return (
            <li key={node.id} className="reader-tree__item">
              <details className="reader-tree__details" open={isOpenByDefault}>
                <summary className="reader-tree__summary">
                  <div className={rowClassName} style={rowStyle}>
                    {rowContent}
                  </div>
                </summary>
                {hasChildren ? (
                  <ReaderTree nodes={node.children} spaceId={spaceId} activeDocId={activeDocId} depth={depth + 1} />
                ) : null}
              </details>
            </li>
          );
        }

        return (
          <li key={node.id} className="reader-tree__item">
            {isActive ? (
              <div className={rowClassName} style={rowStyle}>
                {rowContent}
              </div>
            ) : (
              <a className={rowClassName} style={rowStyle} href={`/r/${encodeURIComponent(spaceId)}/${encodeURIComponent(node.id)}`}>
                {rowContent}
              </a>
            )}
          </li>
        );
      })}
    </ul>
  );
}

function escapeJSONForScript(rawJSON: string): string {
  return rawJSON.replace(/</g, "\\u003c").replace(/>/g, "\\u003e").replace(/&/g, "\\u0026");
}

// renderSpaceReader 将阅读页 payload 渲染为完整 HTML 文档字符串。
export function renderSpaceReader(payload: ReaderPagePayload): SpaceReaderRenderResult {
  const startedAt = Date.now();
  const renderedAt = new Date(startedAt);
  const payloadBytes = new TextEncoder().encode(JSON.stringify(payload)).length;
  const canonicalPath = `/r/${encodeURIComponent(payload.space.id)}/${encodeURIComponent(payload.document.id)}`;

  const resolvedTheme = resolvePreviewTheme(
    payload.document.themeId || DEFAULT_PREVIEW_THEME_TEMPLATE.id,
    BUILTIN_PREVIEW_THEME_TEMPLATES
  );
  const previewThemeClassName = getPreviewThemeClassName(resolvedTheme.id);
  const previewThemeStyleText = buildPreviewThemeStyleText(resolvedTheme);
  const previewThemeCustomStyleText = (resolvedTheme.customCss ?? "").trim();
  const markdownRenderer = buildReaderMarkdownRenderer(payload.document.contentMd, resolvedTheme);

  const articleTitle = payload.document.title.trim() || "未命名文档";
  const spaceTitle = payload.space.name.trim() || "未命名空间";
  const updatedMeta = formatUpdatedMeta(payload.document.updatedAt, renderedAt);
  const documentMeta = `空间：${spaceTitle} · 版本：v${payload.document.version} · ${updatedMeta}`;

  const appMarkup = renderToStaticMarkup(
    <html lang="zh-CN">
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>{payload.space.title || articleTitle}</title>
        <link rel="canonical" href={canonicalPath} />
        <style>{appStyleText}</style>
        <style>{READER_BASE_STYLE}</style>
        <style>{katexStyleText}</style>
        {previewThemeStyleText ? <style>{previewThemeStyleText}</style> : null}
        {previewThemeCustomStyleText ? <style>{previewThemeCustomStyleText}</style> : null}
      </head>
      <body>
        <div className="reader-layout">
          <aside className="reader-sidebar">
            <h2 className="reader-sidebar__title">{spaceTitle}</h2>
            <ReaderTree
              nodes={payload.tree}
              spaceId={payload.space.id}
              activeDocId={payload.activeDocId || payload.document.id}
            />
          </aside>
          <main className="reader-main">
            <div className="reader-article-shell">
              <header className="reader-article-header">
                <h1 className="reader-article-title">{articleTitle}</h1>
                <p className="reader-article-meta">{documentMeta}</p>
              </header>
              <article
                id={PREVIEW_BODY_ID}
                className={`markdown-body ${PREVIEW_BODY_CLASS} ${previewThemeClassName}`}
              >
                <ReactMarkdown
                  remarkPlugins={markdownRenderer.remarkPlugins}
                  rehypePlugins={markdownRenderer.rehypePlugins}
                  components={markdownRenderer.components}
                >
                  {payload.document.contentMd}
                </ReactMarkdown>
              </article>
            </div>
          </main>
        </div>
        <script
          id="plaindoc-reader-state"
          type="application/json"
          dangerouslySetInnerHTML={{
            __html: escapeJSONForScript(JSON.stringify(payload))
          }}
        />
      </body>
    </html>
  );

  const html = `<!doctype html>${appMarkup}`;
  return {
    html,
    head: {
      title: payload.space.title || articleTitle,
      canonical: canonicalPath
    },
    metrics: {
      renderMs: Date.now() - startedAt,
      payloadBytes
    }
  };
}
