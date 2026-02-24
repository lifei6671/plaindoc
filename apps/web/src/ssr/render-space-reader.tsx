import katexStyleText from "katex/dist/katex.min.css?inline";
import { ChevronDown, Lock, LockOpen } from "lucide-react";
import { renderToStaticMarkup } from "react-dom/server";
import ReactMarkdown from "react-markdown";
import { PREVIEW_BODY_CLASS, PREVIEW_BODY_ID } from "../editor/constants";
import { buildPreviewThemeStyleText, getPreviewThemeClassName } from "../editor/preview-style";
import {
  BUILTIN_PREVIEW_THEME_TEMPLATES,
  DEFAULT_PREVIEW_THEME_TEMPLATE,
  resolvePreviewTheme
} from "../preview-themes";
import appStyleText from "../styles.css?inline";
import { buildReaderMarkdownRenderer } from "./markdown-shared";
import type { ReaderPagePayload, ReaderTreeNode } from "./ssr-types";

const SEO_TITLE_SUFFIX = "PlainDoc - 一个适合中小团队文档在线管理系统";

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
  z-index: 60;
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
.reader-tree__row:not([data-reader-active="1"]):hover {
  background: #e8e8ea;
}
.reader-tree__row--active,
.reader-tree__row[data-reader-active="1"] {
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
  display: inline-flex;
  flex: 1;
  min-width: 0;
  align-items: center;
  gap: 8px;
  line-height: 1.3;
  color: #2f2f30;
}
.reader-tree__label-text {
  display: block;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.reader-tree__title-tooltip {
  display: inline-flex;
  min-width: 0;
  flex: 1;
}
.reader-tree__title-tooltip::before,
.reader-tree__title-tooltip::after {
  content: none;
}
.reader-floating-tooltip {
  position: fixed;
  left: -9999px;
  top: -9999px;
  z-index: 2147483647;
  pointer-events: none;
  border-radius: 8px;
  border: 1px solid #1e293b;
  background: #0f172a;
  color: #f8fafc;
  font-size: 12px;
  line-height: 1.4;
  padding: 8px 10px;
  box-shadow: 0 12px 24px rgba(2, 6, 23, 0.28);
  max-width: min(80vw, 960px);
  white-space: pre-wrap;
  word-break: break-word;
  overflow-wrap: anywhere;
  opacity: 0;
  transform: translateY(4px);
  transition: opacity 0.12s ease, transform 0.12s ease;
}
.reader-floating-tooltip--visible {
  opacity: 1;
  transform: translateY(0);
}
.reader-tree__label-link {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  text-decoration: none;
  color: inherit;
}
.reader-tree__label-link:hover {
  text-decoration: none;
}
.reader-tree__label--folder {
  font-weight: 600;
}
.reader-tree__label--active,
.reader-tree__label[data-reader-label-active="1"] {
  font-weight: 600;
  color: #1d4ed8;
}
.reader-visibility-marker {
  display: inline-flex;
  height: 18px;
  width: 18px;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
}
.reader-visibility-marker--public {
  color: #166534;
}
.reader-visibility-marker--authenticated {
  color: #1d4ed8;
}
.reader-visibility-marker--member {
  color: #334155;
}
.reader-visibility-marker__no-lock {
  position: relative;
  display: inline-flex;
  width: 14px;
  height: 14px;
  align-items: center;
  justify-content: center;
}
.reader-visibility-marker__no-lock::after {
  content: "";
  position: absolute;
  width: 15px;
  height: 2px;
  border-radius: 999px;
  background: currentColor;
  transform: rotate(-35deg);
}
.reader-tooltip {
  position: relative;
  display: inline-flex;
}
.reader-tooltip:not(.reader-tree__title-tooltip)::before {
  content: attr(data-tooltip);
  position: absolute;
  left: 50%;
  bottom: calc(100% + 8px);
  transform: translateX(-50%) translateY(4px);
  opacity: 0;
  pointer-events: none;
  white-space: nowrap;
  z-index: 5000;
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  border-radius: 8px;
  border: 1px solid #1e293b;
  background: #0f172a;
  color: #f8fafc;
  font-size: 12px;
  line-height: 1.3;
  padding: 6px 8px;
  transition: opacity 0.12s ease, transform 0.12s ease;
}
.reader-tooltip:not(.reader-tree__title-tooltip)::after {
  content: "";
  position: absolute;
  left: 50%;
  bottom: calc(100% + 2px);
  transform: translateX(-50%) translateY(4px);
  opacity: 0;
  pointer-events: none;
  width: 8px;
  height: 8px;
  background: #0f172a;
  border-right: 1px solid #1e293b;
  border-bottom: 1px solid #1e293b;
  rotate: 45deg;
  z-index: 5000;
  transition: opacity 0.12s ease, transform 0.12s ease;
}
.reader-tooltip:not(.reader-tree__title-tooltip):hover::before,
.reader-tooltip:not(.reader-tree__title-tooltip):hover::after,
.reader-tooltip:not(.reader-tree__title-tooltip):focus-within::before,
.reader-tooltip:not(.reader-tree__title-tooltip):focus-within::after {
  opacity: 1;
  transform: translateX(-50%) translateY(0);
}
.reader-main {
  position: relative;
  min-width: 0;
  height: 100vh;
  padding: 26px;
  overflow-y: auto;
  overflow-x: hidden;
}
.reader-progress {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 9800;
  height: 2px;
  margin: 0;
  overflow: hidden;
  pointer-events: none;
}
.reader-progress__bar {
  display: block;
  width: var(--reader-progress, 0%);
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, #2563eb, #0ea5e9);
  opacity: 0;
  transition: width 0.18s ease, opacity 0.18s ease;
}
.reader-progress--visible .reader-progress__bar {
  opacity: 0.95;
}
.reader-article-shell {
  max-width: 1024px;
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
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.reader-article-meta__text {
  display: inline-flex;
  align-items: center;
}
.reader-access-denied {
  padding: 30px 24px 34px;
}
.reader-access-denied__panel {
  border: 1px solid #e2e8f0;
  background: #f8fafc;
  border-radius: 14px;
  padding: 20px;
}
.reader-access-denied__icon {
  display: inline-flex;
  width: 36px;
  height: 36px;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: #e2e8f0;
  color: #334155;
}
.reader-access-denied__title {
  margin: 12px 0 0;
  font-size: 20px;
  line-height: 1.4;
  color: #0f172a;
}
.reader-access-denied__desc {
  margin: 10px 0 0;
  color: #475569;
  font-size: 14px;
  line-height: 1.75;
}
.reader-access-denied__hint {
  margin: 10px 0 0;
  color: #64748b;
  font-size: 13px;
  line-height: 1.6;
}
.reader-access-denied__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 14px;
}
.reader-access-denied__action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 36px;
  border-radius: 9px;
  border: 1px solid #cbd5e1;
  background: #fff;
  color: #334155;
  text-decoration: none;
  font-size: 13px;
  font-weight: 600;
  padding: 0 14px;
}
.reader-access-denied__action:hover {
  background: #f1f5f9;
}
.reader-access-denied__action--primary {
  border-color: #0f172a;
  background: #0f172a;
  color: #fff;
}
.reader-access-denied__action--primary:hover {
  background: #1e293b;
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

type ReaderVisibility = "public" | "authenticated" | "member";

const MINUTE_MS = 60 * 1000;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;
const WEEK_MS = 7 * DAY_MS;
const MONTH_MS = 30 * DAY_MS;
const YEAR_MS = 365 * DAY_MS;
const SECOND_MS = 1000;
const JUST_NOW_THRESHOLD_MS = 5 * SECOND_MS;

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
  const diffMs = now.getTime() - date.getTime();
  const isFuture = diffMs < 0;
  const deltaMs = Math.abs(diffMs);
  if (deltaMs <= JUST_NOW_THRESHOLD_MS) {
    return "刚刚";
  }
  if (deltaMs < MINUTE_MS) {
    const seconds = Math.max(1, Math.floor(deltaMs / SECOND_MS));
    return isFuture ? `${seconds}秒后` : `${seconds}秒前`;
  }
  if (deltaMs < HOUR_MS) {
    const minutes = Math.max(1, Math.floor(deltaMs / MINUTE_MS));
    return isFuture ? `${minutes}分钟后` : `${minutes}分钟前`;
  }
  if (deltaMs < DAY_MS) {
    const hours = Math.max(1, Math.floor(deltaMs / HOUR_MS));
    return isFuture ? `${hours}小时后` : `${hours}小时前`;
  }
  if (deltaMs < WEEK_MS) {
    const days = Math.max(1, Math.floor(deltaMs / DAY_MS));
    return isFuture ? `${days}天后` : `${days}天前`;
  }
  if (deltaMs < MONTH_MS) {
    const weeks = Math.max(1, Math.floor(deltaMs / WEEK_MS));
    return isFuture ? `${weeks}周后` : `${weeks}周前`;
  }
  if (deltaMs < YEAR_MS) {
    const months = Math.max(1, Math.floor(deltaMs / MONTH_MS));
    return isFuture ? `${months}个月后` : `${months}个月前`;
  }
  const years = Math.max(1, Math.floor(deltaMs / YEAR_MS));
  return isFuture ? `${years}年后` : `${years}年前`;
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

function normalizeTreeTitle(title: string, fallback: string): string {
  const trimmed = title.trim();
  return trimmed || fallback;
}

function normalizeReaderVisibility(value: unknown): ReaderVisibility {
  if (value === "public" || value === "authenticated" || value === "member") {
    return value;
  }
  return "member";
}

function renderVisibilityMarker(visibilityInput: unknown) {
  const visibility = normalizeReaderVisibility(visibilityInput);
  if (visibility === "public") {
    return (
      <span
        className="reader-tooltip reader-visibility-marker reader-visibility-marker--public"
        data-tooltip="可见性：公开"
        aria-label="可见性：公开"
      >
        <span className="reader-visibility-marker__no-lock">
          <Lock size={14} />
        </span>
      </span>
    );
  }
  if (visibility === "authenticated") {
    return (
      <span
        className="reader-tooltip reader-visibility-marker reader-visibility-marker--authenticated"
        data-tooltip="可见性：登录可见"
        aria-label="可见性：登录可见"
      >
        <LockOpen size={14} />
      </span>
    );
  }
  return (
    <span
      className="reader-tooltip reader-visibility-marker reader-visibility-marker--member"
      data-tooltip="可见性：成员可见"
      aria-label="可见性：成员可见"
    >
      <Lock size={14} />
    </span>
  );
}

function ReaderTree({ nodes, spaceId, activeDocId, depth = 0 }: ReaderTreeProps) {
  if (!nodes.length) {
    return <p className="reader-tree__label reader-tree__label--folder">暂无目录</p>;
  }

  return (
    <ul className={depth > 0 ? "reader-tree__children" : "reader-tree"}>
      {nodes.map((node) => {
        const isDocumentNode = node.type === "doc";
        const resolvedDocumentID = isDocumentNode
          ? (node.documentId?.trim() || node.id.trim())
          : node.id.trim();
        const isActive = isDocumentNode ? resolvedDocumentID === activeDocId : false;
        const isFolderNode = node.type === "folder";
        const hasChildren = node.children.length > 0;
        const isExpandable = isFolderNode || hasChildren;
        const title = normalizeTreeTitle(node.title, isFolderNode ? "未命名目录" : "未命名文档");
        const rowStyle = {
          paddingLeft: `${8 + depth * 20}px`
        };
        const rowClassName = `reader-tree__row${isActive ? " reader-tree__row--active" : ""}`;
        const arrowClassName = `reader-tree__arrow${isExpandable ? " reader-tree__arrow--expandable" : " reader-tree__arrow--empty"}`;
        const labelClassName = `reader-tree__label${isFolderNode ? " reader-tree__label--folder" : ""}${isActive ? " reader-tree__label--active" : ""}`;
        const visibilityMarker = isDocumentNode ? renderVisibilityMarker(node.visibility) : null;
        const staticLabel = (
          <span
            className={labelClassName}
            data-reader-hook="tree-label"
            data-reader-label-active={isActive ? "1" : undefined}
          >
            {visibilityMarker}
            <span
              className="reader-tooltip reader-tree__title-tooltip"
              data-reader-hook="tree-title-tooltip"
              data-tooltip={title}
            >
              <span className="reader-tree__label-text">{title}</span>
            </span>
          </span>
        );
        const linkLabel = (
          <a
            className={`${labelClassName} reader-tree__label-link`}
            data-reader-hook="tree-label"
            data-reader-label-active={isActive ? "1" : undefined}
            data-reader-doc-link={isDocumentNode ? "1" : undefined}
            data-reader-doc-id={isDocumentNode ? resolvedDocumentID : undefined}
            href={`/r/${encodeURIComponent(spaceId)}/${encodeURIComponent(resolvedDocumentID)}`}
          >
            {visibilityMarker}
            <span
              className="reader-tooltip reader-tree__title-tooltip"
              data-reader-hook="tree-title-tooltip"
              data-tooltip={title}
            >
              <span className="reader-tree__label-text">{title}</span>
            </span>
          </a>
        );
        const rowContent = (
          <>
            <span className={arrowClassName} data-reader-hook={isExpandable ? "tree-arrow" : undefined} aria-hidden="true">
              {isExpandable ? <ChevronDown size={15} /> : null}
            </span>
            {isDocumentNode ? (isExpandable ? linkLabel : staticLabel) : staticLabel}
          </>
        );

        if (isExpandable) {
          return (
            <li key={node.id} className="reader-tree__item">
              <details className="reader-tree__details" data-reader-hook="tree-details">
                <summary className="reader-tree__summary" data-reader-hook="tree-summary">
                  <div
                    className={rowClassName}
                    data-reader-hook="tree-row"
                    data-reader-active={isActive ? "1" : undefined}
                    style={rowStyle}
                  >
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
            {isDocumentNode ? (
              <a
                className={rowClassName}
                data-reader-hook="tree-row"
                data-reader-active={isActive ? "1" : undefined}
                data-reader-doc-link="1"
                data-reader-doc-id={resolvedDocumentID}
                style={rowStyle}
                href={`/r/${encodeURIComponent(spaceId)}/${encodeURIComponent(resolvedDocumentID)}`}
              >
                {rowContent}
              </a>
            ) : (
              <div className={rowClassName} style={rowStyle}>
                {rowContent}
              </div>
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

function composeSEOTitle(title: string): string {
  const normalizedTitle = title.trim();
  if (!normalizedTitle) {
    return SEO_TITLE_SUFFIX;
  }
  return `${normalizedTitle} - ${SEO_TITLE_SUFFIX}`;
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
  const seoTitle = composeSEOTitle(payload.space.title || articleTitle);
  const hasDeniedAccess = Boolean(payload.access?.code?.trim());
  const updatedMeta = hasDeniedAccess ? "" : formatUpdatedMeta(payload.document.updatedAt, renderedAt);
  const documentMeta = `空间：${spaceTitle} · 版本：v${payload.document.version} · ${updatedMeta}`;
  const documentVisibilityMarker = hasDeniedAccess ? null : renderVisibilityMarker(payload.document.visibility);
  const spaceLandingPath = `/r/${encodeURIComponent(payload.space.id)}`;
  const loginPath = `/login?redirect=${encodeURIComponent(canonicalPath)}`;

  const appMarkup = renderToStaticMarkup(
    <html lang="zh-CN">
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>{seoTitle}</title>
        <link rel="canonical" href={canonicalPath} />
        <style>{appStyleText}</style>
        <style>{READER_BASE_STYLE}</style>
        <style>{katexStyleText}</style>
        {previewThemeStyleText ? <style id="plaindoc-reader-theme-style">{previewThemeStyleText}</style> : null}
        {previewThemeCustomStyleText ? (
          <style id="plaindoc-reader-theme-custom-style">{previewThemeCustomStyleText}</style>
        ) : null}
      </head>
      <body>
        <div
          id="plaindoc-reader-progress"
          className="reader-progress"
          data-reader-hook="progress"
          aria-hidden="true"
        >
          <span className="reader-progress__bar" />
        </div>
        <div className="reader-layout">
          <aside className="reader-sidebar" data-reader-hook="sidebar">
            <h2 className="reader-sidebar__title">{spaceTitle}</h2>
            <ReaderTree
              nodes={payload.tree}
              spaceId={payload.space.id}
              activeDocId={payload.activeDocId || payload.document.id}
            />
          </aside>
          <main className="reader-main" data-reader-hook="main">
            <div
              id="plaindoc-reader-article-shell"
              className="reader-article-shell"
              data-reader-hook="article-shell"
            >
              {!hasDeniedAccess ? (
                <header className="reader-article-header">
                  <h1 className="reader-article-title">{articleTitle}</h1>
                  <div className="reader-article-meta">
                    {documentVisibilityMarker}
                    <span className="reader-article-meta__text">{documentMeta}</span>
                  </div>
                </header>
              ) : null}
              {hasDeniedAccess ? (
                <section className="reader-access-denied">
                  <div className="reader-access-denied__panel">
                    <span className="reader-access-denied__icon" aria-hidden="true">
                      <Lock size={18} />
                    </span>
                    <h2 className="reader-access-denied__title">{payload.access?.title?.trim() || "暂无访问权限"}</h2>
                    <p className="reader-access-denied__desc">
                      {payload.access?.description?.trim() || "你没有权限访问该文档，请联系空间管理员。"}
                    </p>
                    <p className="reader-access-denied__hint">你可以返回空间浏览其它文档，或联系管理员开通权限。</p>
                    <div className="reader-access-denied__actions">
                      {payload.access?.requiresLogin ? (
                        <a className="reader-access-denied__action reader-access-denied__action--primary" href={loginPath}>
                          去登录
                        </a>
                      ) : null}
                      <a className="reader-access-denied__action" href={spaceLandingPath}>
                        返回空间
                      </a>
                    </div>
                  </div>
                </section>
              ) : (
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
              )}
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
        <script
          dangerouslySetInnerHTML={{
            __html: `(() => {
  const DOC_LINK_SELECTOR = "a[data-reader-doc-link='1']";
  const TREE_DETAILS_SELECTOR = "[data-reader-hook='tree-details']";
  const TREE_SUMMARY_SELECTOR = "[data-reader-hook='tree-summary']";
  const TREE_ARROW_SELECTOR = "[data-reader-hook='tree-arrow']";
  const TREE_ROW_SELECTOR = "[data-reader-hook='tree-row']";
  const TREE_LABEL_SELECTOR = "[data-reader-hook='tree-label']";
  const TREE_ROW_ACTIVE_SELECTOR = "[data-reader-hook='tree-row'][data-reader-active='1']";
  const TREE_LABEL_ACTIVE_SELECTOR = "[data-reader-hook='tree-label'][data-reader-label-active='1']";
  const ARTICLE_SHELL_SELECTOR = "[data-reader-hook='article-shell']";
  const MAIN_SELECTOR = "[data-reader-hook='main']";
  const TREE_TITLE_TOOLTIP_SELECTOR = "[data-reader-hook='tree-title-tooltip'][data-tooltip]";
  const TREE_ROW_ACTIVE_CLASS = "reader-tree__row--active";
  const TREE_LABEL_ACTIVE_CLASS = "reader-tree__label--active";
  const STATE_SCRIPT_SELECTOR = "#plaindoc-reader-state";
  const PROGRESS_SELECTOR = "[data-reader-hook='progress']";
  const PROGRESS_VISIBLE_CLASS = "reader-progress--visible";
  const THEME_STYLE_ID = "plaindoc-reader-theme-style";
  const THEME_CUSTOM_STYLE_ID = "plaindoc-reader-theme-custom-style";
  const queryActiveRow = () => document.querySelector(TREE_ROW_ACTIVE_SELECTOR);

  const normalizePathname = (pathname) => pathname.replace(/\\/+$/, "") || "/";
  const isModifiedClick = (event) =>
    !(event instanceof MouseEvent) ||
    event.button !== 0 ||
    event.metaKey ||
    event.ctrlKey ||
    event.shiftKey ||
    event.altKey;

  const toSameOriginURL = (rawHref) => {
    try {
      const parsedURL = new URL(rawHref, window.location.href);
      if (parsedURL.origin !== window.location.origin) {
        return null;
      }
      return parsedURL;
    } catch {
      return null;
    }
  };

  const expandAncestorsForActiveRow = () => {
    const allDetails = document.querySelectorAll(TREE_DETAILS_SELECTOR);
    for (const detailNode of allDetails) {
      if (detailNode instanceof HTMLDetailsElement) {
        detailNode.open = false;
      }
    }
    const activeRow = queryActiveRow();
    if (!(activeRow instanceof HTMLElement)) {
      return;
    }

    const activeDetails = activeRow.closest(TREE_DETAILS_SELECTOR);
    const activeSummaryElement = activeRow.closest(TREE_SUMMARY_SELECTOR);
    const isActiveInsideOwnSummary =
      activeDetails instanceof HTMLDetailsElement &&
      activeSummaryElement instanceof HTMLElement &&
      activeDetails.firstElementChild === activeSummaryElement;
    let parentElement =
      activeDetails instanceof HTMLDetailsElement
        ? isActiveInsideOwnSummary
          ? activeDetails.parentElement
          : activeDetails
        : activeRow.parentElement;
    while (parentElement) {
      if (parentElement instanceof HTMLDetailsElement) {
        parentElement.open = true;
      }
      parentElement = parentElement.parentElement;
    }
    activeRow.scrollIntoView({ block: "nearest", inline: "nearest" });
  };

  const progressRoot = document.querySelector(PROGRESS_SELECTOR);
  let progressTimer = 0;
  let progressValue = 0;
  const setProgress = (value) => {
    if (!(progressRoot instanceof HTMLElement)) {
      return;
    }
    progressValue = Math.max(0, Math.min(100, value));
    progressRoot.style.setProperty("--reader-progress", progressValue.toFixed(2) + "%");
  };
  const startProgress = () => {
    if (!(progressRoot instanceof HTMLElement)) {
      return;
    }
    if (progressTimer) {
      window.clearInterval(progressTimer);
      progressTimer = 0;
    }
    progressRoot.classList.add(PROGRESS_VISIBLE_CLASS);
    setProgress(8);
    progressTimer = window.setInterval(() => {
      if (progressValue >= 90) {
        return;
      }
      const increment = Math.max(1.2, (90 - progressValue) * 0.16);
      setProgress(progressValue + increment);
    }, 120);
  };
  const finishProgress = () => {
    if (!(progressRoot instanceof HTMLElement)) {
      return;
    }
    if (progressTimer) {
      window.clearInterval(progressTimer);
      progressTimer = 0;
    }
    setProgress(100);
    window.setTimeout(() => {
      progressRoot.classList.remove(PROGRESS_VISIBLE_CLASS);
      setProgress(0);
    }, 220);
  };

  const syncHeadStyleByID = (styleID, nextDocument) => {
    const nextStyleNode = nextDocument.getElementById(styleID);
    const nextStyleText = nextStyleNode ? nextStyleNode.textContent || "" : "";
    const currentStyleNode = document.getElementById(styleID);
    if (!nextStyleText.trim()) {
      if (currentStyleNode && currentStyleNode.parentNode) {
        currentStyleNode.parentNode.removeChild(currentStyleNode);
      }
      return;
    }
    if (currentStyleNode instanceof HTMLStyleElement) {
      currentStyleNode.textContent = nextStyleText;
      return;
    }
    const createdStyleNode = document.createElement("style");
    createdStyleNode.id = styleID;
    createdStyleNode.textContent = nextStyleText;
    document.head.appendChild(createdStyleNode);
  };

  const syncReaderStateScript = (nextDocument) => {
    const nextStateNode = nextDocument.querySelector(STATE_SCRIPT_SELECTOR);
    const currentStateNode = document.querySelector(STATE_SCRIPT_SELECTOR);
    if (!(nextStateNode instanceof HTMLScriptElement) || !(currentStateNode instanceof HTMLScriptElement)) {
      return;
    }
    currentStateNode.textContent = nextStateNode.textContent || "{}";
  };

  const syncDocumentHead = (nextDocument, targetURL) => {
    if (typeof nextDocument.title === "string" && nextDocument.title.trim()) {
      document.title = nextDocument.title.trim();
    }
    const nextCanonicalNode = nextDocument.querySelector("link[rel='canonical']");
    let currentCanonicalNode = document.querySelector("link[rel='canonical']");
    if (!(currentCanonicalNode instanceof HTMLLinkElement)) {
      currentCanonicalNode = document.createElement("link");
      currentCanonicalNode.setAttribute("rel", "canonical");
      document.head.appendChild(currentCanonicalNode);
    }
    if (nextCanonicalNode instanceof HTMLLinkElement && nextCanonicalNode.href) {
      currentCanonicalNode.href = nextCanonicalNode.href;
    } else {
      currentCanonicalNode.href = targetURL.toString();
    }
  };

  const replaceArticleShell = (nextDocument) => {
    const currentArticleShell = document.querySelector(ARTICLE_SHELL_SELECTOR);
    const nextArticleShell =
      nextDocument.querySelector(ARTICLE_SHELL_SELECTOR) || nextDocument.querySelector(".reader-article-shell");
    if (!(currentArticleShell instanceof HTMLElement) || !(nextArticleShell instanceof HTMLElement)) {
      return false;
    }
    const importedArticleShell = document.importNode(nextArticleShell, true);
    if (importedArticleShell instanceof HTMLElement) {
      importedArticleShell.id = "plaindoc-reader-article-shell";
    }
    currentArticleShell.replaceWith(importedArticleShell);
    return true;
  };

  const markActiveTreeItemByPathname = (pathname) => {
    const normalizedPathname = normalizePathname(pathname);
    for (const rowNode of document.querySelectorAll(TREE_ROW_SELECTOR)) {
      if (!(rowNode instanceof HTMLElement)) {
        continue;
      }
      rowNode.removeAttribute("data-reader-active");
      rowNode.classList.remove(TREE_ROW_ACTIVE_CLASS);
    }
    for (const labelNode of document.querySelectorAll(TREE_LABEL_SELECTOR)) {
      if (!(labelNode instanceof HTMLElement)) {
        continue;
      }
      labelNode.removeAttribute("data-reader-label-active");
      labelNode.classList.remove(TREE_LABEL_ACTIVE_CLASS);
    }

    let matchedLink = null;
    for (const linkNode of document.querySelectorAll(DOC_LINK_SELECTOR)) {
      if (!(linkNode instanceof HTMLAnchorElement)) {
        continue;
      }
      const linkURL = toSameOriginURL(linkNode.href);
      if (!linkURL) {
        continue;
      }
      if (normalizePathname(linkURL.pathname) === normalizedPathname) {
        matchedLink = linkNode;
        break;
      }
    }
    if (!(matchedLink instanceof HTMLAnchorElement)) {
      return;
    }

    const rowElement = matchedLink.closest(TREE_ROW_SELECTOR);
    if (rowElement instanceof HTMLElement) {
      rowElement.setAttribute("data-reader-active", "1");
      rowElement.classList.add(TREE_ROW_ACTIVE_CLASS);
      rowElement.scrollIntoView({ block: "nearest", inline: "nearest" });
    }
    const labelElement =
      matchedLink.matches(TREE_LABEL_SELECTOR)
        ? matchedLink
        : matchedLink.querySelector(TREE_LABEL_SELECTOR);
    if (labelElement instanceof HTMLElement) {
      labelElement.setAttribute("data-reader-label-active", "1");
      labelElement.classList.add(TREE_LABEL_ACTIVE_CLASS);
    }

    const activeDetails = rowElement instanceof HTMLElement ? rowElement.closest(TREE_DETAILS_SELECTOR) : null;
    const activeSummaryElement = rowElement instanceof HTMLElement ? rowElement.closest(TREE_SUMMARY_SELECTOR) : null;
    const isActiveInsideOwnSummary =
      activeDetails instanceof HTMLDetailsElement &&
      activeSummaryElement instanceof HTMLElement &&
      activeDetails.firstElementChild === activeSummaryElement;
    let parentNode =
      activeDetails instanceof HTMLDetailsElement
        ? isActiveInsideOwnSummary
          ? activeDetails.parentElement
          : activeDetails
        : matchedLink.parentElement;
    while (parentNode) {
      if (parentNode instanceof HTMLDetailsElement) {
        parentNode.open = true;
      }
      parentNode = parentNode.parentElement;
    }
  };

  let inflightController = null;
  let inflightSeq = 0;
  const loadReaderPage = async (targetURL, pushHistory) => {
    if (!(targetURL instanceof URL)) {
      return;
    }
    const targetPathname = normalizePathname(targetURL.pathname);
    if (pushHistory && targetPathname === normalizePathname(window.location.pathname)) {
      return;
    }

    inflightSeq += 1;
    const currentSeq = inflightSeq;
    if (inflightController instanceof AbortController) {
      inflightController.abort();
    }
    inflightController = new AbortController();
    startProgress();

    try {
      const response = await fetch(targetURL.toString(), {
        method: "GET",
        credentials: "include",
        signal: inflightController.signal,
        headers: {
          Accept: "text/html",
          "X-Requested-With": "plaindoc-reader-async"
        }
      });
      if (currentSeq !== inflightSeq) {
        return;
      }
      if (!response.ok) {
        window.location.assign(targetURL.toString());
        return;
      }
      const htmlText = await response.text();
      if (currentSeq !== inflightSeq) {
        return;
      }
      const parsedDocument = new DOMParser().parseFromString(htmlText, "text/html");
      if (!replaceArticleShell(parsedDocument)) {
        window.location.assign(targetURL.toString());
        return;
      }

      syncDocumentHead(parsedDocument, targetURL);
      syncHeadStyleByID(THEME_STYLE_ID, parsedDocument);
      syncHeadStyleByID(THEME_CUSTOM_STYLE_ID, parsedDocument);
      syncReaderStateScript(parsedDocument);
      markActiveTreeItemByPathname(targetPathname);

      const readerMain = document.querySelector(MAIN_SELECTOR);
      if (readerMain instanceof HTMLElement) {
        readerMain.scrollTop = 0;
      }
      if (pushHistory) {
        window.history.pushState({ reader: true }, "", targetURL.toString());
      }
    } catch (error) {
      if (
        error &&
        typeof error === "object" &&
        "name" in error &&
        (error).name === "AbortError"
      ) {
        return;
      }
      window.location.assign(targetURL.toString());
    } finally {
      if (currentSeq === inflightSeq) {
        finishProgress();
      }
    }
  };

  try {
    expandAncestorsForActiveRow();
    markActiveTreeItemByPathname(window.location.pathname);
  } catch {
    // no-op: initialization enhancement should never block rendering.
  }

  try {
    document.addEventListener(
      "click",
      (event) => {
        if (!(event.target instanceof Element)) {
          return;
        }

        const docLink = event.target.closest(DOC_LINK_SELECTOR);
        if (docLink instanceof HTMLAnchorElement) {
          if (event.defaultPrevented || isModifiedClick(event)) {
            return;
          }
          const targetURL = toSameOriginURL(docLink.getAttribute("href") || docLink.href);
          if (!targetURL) {
            return;
          }
          event.preventDefault();
          event.stopPropagation();
          void loadReaderPage(targetURL, true);
          return;
        }

        const summaryElement = event.target.closest(TREE_SUMMARY_SELECTOR);
        if (!(summaryElement instanceof HTMLElement)) {
          return;
        }
        if (event.target.closest(TREE_ARROW_SELECTOR)) {
          const detailsElement = summaryElement.parentElement;
          if (detailsElement instanceof HTMLDetailsElement) {
            window.requestAnimationFrame(() => {
              if (!detailsElement.open) {
                return;
              }
              const nestedDetails = detailsElement.querySelectorAll(TREE_DETAILS_SELECTOR);
              const activeRow = queryActiveRow();
              for (const nestedDetail of nestedDetails) {
                if (!(nestedDetail instanceof HTMLDetailsElement) || nestedDetail === detailsElement) {
                  continue;
                }
                if (activeRow instanceof HTMLElement && nestedDetail.contains(activeRow)) {
                  nestedDetail.open = true;
                  continue;
                }
                nestedDetail.open = false;
              }
            });
          }
          return;
        }

        event.preventDefault();
        event.stopPropagation();
      },
      true
    );
  } catch {
    // no-op: click behavior enhancement should never block rendering.
  }

  try {
    window.addEventListener("popstate", () => {
      const targetURL = toSameOriginURL(window.location.href);
      if (!targetURL) {
        return;
      }
      void loadReaderPage(targetURL, false);
    });
  } catch {
    // no-op: history enhancement should never block rendering.
  }

  try {
    const tooltipTargetSelector = TREE_TITLE_TOOLTIP_SELECTOR;
    const tooltipClassName = "reader-floating-tooltip";
    const tooltipVisibleClassName = "reader-floating-tooltip--visible";
    const viewportPadding = 10;
    const tooltipOffset = 8;
    let activeTarget = null;
    let tooltipNode = null;
    let rafId = 0;

    const ensureTooltipNode = () => {
      if (tooltipNode instanceof HTMLElement) {
        return tooltipNode;
      }
      const createdNode = document.createElement("div");
      createdNode.className = tooltipClassName;
      createdNode.setAttribute("role", "tooltip");
      createdNode.setAttribute("aria-hidden", "true");
      document.body.appendChild(createdNode);
      tooltipNode = createdNode;
      return createdNode;
    };

    const updateTooltipPosition = () => {
      rafId = 0;
      if (!(activeTarget instanceof HTMLElement) || !(tooltipNode instanceof HTMLElement)) {
        return;
      }
      const anchorRect = activeTarget.getBoundingClientRect();
      tooltipNode.style.left = viewportPadding + "px";
      tooltipNode.style.top = "-9999px";

      const tooltipRect = tooltipNode.getBoundingClientRect();
      let left = anchorRect.left;
      const maxLeft = window.innerWidth - viewportPadding - tooltipRect.width;
      if (left > maxLeft) {
        left = maxLeft;
      }
      if (left < viewportPadding) {
        left = viewportPadding;
      }

      let top = anchorRect.top - tooltipRect.height - tooltipOffset;
      if (top < viewportPadding) {
        top = anchorRect.bottom + tooltipOffset;
      }
      const maxTop = window.innerHeight - viewportPadding - tooltipRect.height;
      if (top > maxTop) {
        top = maxTop;
      }
      if (top < viewportPadding) {
        top = viewportPadding;
      }

      tooltipNode.style.left = Math.round(left) + "px";
      tooltipNode.style.top = Math.round(top) + "px";
    };

    const scheduleTooltipPositionUpdate = () => {
      if (rafId) {
        window.cancelAnimationFrame(rafId);
      }
      rafId = window.requestAnimationFrame(updateTooltipPosition);
    };

    const showTooltip = (target) => {
      const tooltipText = (target.getAttribute("data-tooltip") || "").trim();
      if (!tooltipText) {
        return;
      }
      activeTarget = target;
      const ensuredTooltipNode = ensureTooltipNode();
      ensuredTooltipNode.textContent = tooltipText;
      ensuredTooltipNode.setAttribute("aria-hidden", "false");
      ensuredTooltipNode.classList.add(tooltipVisibleClassName);
      scheduleTooltipPositionUpdate();
    };

    const hideTooltip = () => {
      activeTarget = null;
      if (rafId) {
        window.cancelAnimationFrame(rafId);
        rafId = 0;
      }
      if (!(tooltipNode instanceof HTMLElement)) {
        return;
      }
      tooltipNode.classList.remove(tooltipVisibleClassName);
      tooltipNode.setAttribute("aria-hidden", "true");
      tooltipNode.style.left = "-9999px";
      tooltipNode.style.top = "-9999px";
    };

    const tooltipTargets = document.querySelectorAll(tooltipTargetSelector);
    for (const targetNode of tooltipTargets) {
      if (!(targetNode instanceof HTMLElement)) {
        continue;
      }
      targetNode.addEventListener("mouseenter", () => showTooltip(targetNode));
      targetNode.addEventListener("mouseleave", hideTooltip);
      targetNode.addEventListener("focusin", () => showTooltip(targetNode));
      targetNode.addEventListener("focusout", hideTooltip);
    }

    window.addEventListener(
      "scroll",
      () => {
        if (activeTarget) {
          scheduleTooltipPositionUpdate();
        }
      },
      true
    );
    window.addEventListener("resize", () => {
      if (activeTarget) {
        scheduleTooltipPositionUpdate();
      }
    });
  } catch {
    // no-op: tooltip enhancement should never block rendering.
  }
})();`
          }}
        />
      </body>
    </html>
  );

  const html = `<!doctype html>${appMarkup}`;
  return {
    html,
    head: {
      title: seoTitle,
      canonical: canonicalPath
    },
    metrics: {
      renderMs: Date.now() - startedAt,
      payloadBytes
    }
  };
}
