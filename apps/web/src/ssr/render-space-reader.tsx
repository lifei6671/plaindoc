import katexStyleText from "katex/dist/katex.min.css?inline";
import {
  ChevronDown,
  Download,
  Eye,
  FileSpreadsheet,
  FileText,
  LoaderCircle,
  Lock,
  LockOpen,
  Paperclip,
  Printer
} from "lucide-react";
import MarkdownIt from "markdown-it";
import { renderToStaticMarkup } from "react-dom/server";
import ReactMarkdown from "react-markdown";
import { PREVIEW_BODY_CLASS, PREVIEW_BODY_ID } from "../editor/constants";
import { parseTocFromMarkdown } from "../editor/markdown-utils";
import { buildPreviewThemeStyleText, getPreviewThemeClassName } from "../editor/preview-style";
import {
  BUILTIN_PREVIEW_THEME_TEMPLATES,
  DEFAULT_PREVIEW_THEME_TEMPLATE,
  resolvePreviewTheme
} from "../preview-themes";
import appStyleText from "../styles.css?inline";
import readerBaseStyleText from "./render-space-reader.base.css?inline";
import { ReaderImageViewerShell } from "./ReaderImageViewerShell";
import { READER_ASYNC_ENHANCEMENT_SCRIPT } from "./render-space-reader.async-script";
import readerGoogleSansCodeStyleText from "./render-space-reader.font.css?inline";
import { buildReaderMarkdownRenderer } from "./markdown-shared";
import type { ReaderDocumentAttachmentPayload, ReaderPagePayload, ReaderTreeNode } from "./ssr-types";

const SEO_TITLE_SUFFIX = "PlainDoc - 一个适合中小团队文档在线管理系统";
const readerTocParser = new MarkdownIt({
  html: false,
  linkify: true,
  typographer: false
});

interface ReaderOutlineItem {
  index: number;
  level: number;
  text: string;
}

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
  linkBasePath: string;
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
const READER_HTTP_PROTOCOLS = new Set(["http:", "https:"]);

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

function isOfficeDocumentFormat(value: unknown): value is "docx" | "xlsx" {
  return value === "docx" || value === "xlsx";
}

function resolveOfficeDocumentLabel(value: unknown): string {
  return value === "xlsx" ? "Excel 文档" : "Word 文档";
}

function renderOfficeDocumentIcon(value: unknown) {
  if (value === "xlsx") {
    return <FileSpreadsheet size={20} />;
  }
  return <FileText size={20} />;
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

function ReaderTree({ nodes, spaceId, activeDocId, linkBasePath, depth = 0 }: ReaderTreeProps) {
  if (!nodes.length) {
    return <p className="reader-tree__label reader-tree__label--folder">暂无目录</p>;
  }

  return (
    <ul className={depth > 0 ? "reader-tree__children" : "reader-tree"}>
      {nodes.map((node) => {
        const isDocumentNode = node.type === "doc";
        const resolvedDocumentID = isDocumentNode ? (node.documentId?.trim() || node.id.trim()) : node.id.trim();
        const resolvedDocumentRouteKey = isDocumentNode
          ? (node.documentRouteKey?.trim() || resolvedDocumentID)
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
            href={`${linkBasePath}/${encodeURIComponent(resolvedDocumentRouteKey)}`}
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
                  <ReaderTree
                    nodes={node.children}
                    spaceId={spaceId}
                    activeDocId={activeDocId}
                    linkBasePath={linkBasePath}
                    depth={depth + 1}
                  />
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
                href={`${linkBasePath}/${encodeURIComponent(resolvedDocumentRouteKey)}`}
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

function formatAttachmentSize(sizeBytes: number): string {
  if (!Number.isFinite(sizeBytes) || sizeBytes <= 0) {
    return "未知大小";
  }
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = sizeBytes;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  if (unitIndex === 0) {
    return `${Math.round(value)} ${units[unitIndex]}`;
  }
  return `${value.toFixed(value >= 10 ? 1 : 2)} ${units[unitIndex]}`;
}

function normalizeAttachmentPreviewKind(value: unknown): ReaderDocumentAttachmentPayload["previewKind"] {
  if (value === "image" || value === "pdf" || value === "office" || value === "text") {
    return value;
  }
  return "none";
}

function normalizeReaderRequestOrigin(value: string | undefined): string {
  const normalized = (value ?? "").trim();
  if (!normalized) {
    return "";
  }
  try {
    const parsedURL = new URL(normalized);
    if (!READER_HTTP_PROTOCOLS.has(parsedURL.protocol)) {
      return "";
    }
    return parsedURL.origin;
  } catch {
    return "";
  }
}

function normalizeReaderPathPrefix(value: string | undefined, fallbackValue: string): string {
  const fallback = fallbackValue.trim();
  const normalized = (value ?? "").trim();
  if (!normalized) {
    return fallback;
  }
  if (!normalized.startsWith("/")) {
    return fallback;
  }
  const trimmed = normalized.replace(/\/+$/, "");
  if (trimmed) {
    return trimmed;
  }
  return fallback || "/";
}

function resolveShareAttachmentBasePath(payload: ReaderPagePayload): string | null {
  if (!payload.share || payload.share.enabled !== true) {
    return null;
  }
  const normalizedBasePath = normalizeReaderPathPrefix(payload.share.attachmentBasePath, "");
  return normalizedBasePath ? normalizedBasePath : null;
}

function buildAttachmentDownloadHref(
  payload: ReaderPagePayload,
  documentID: string,
  attachmentID: string,
  requestOrigin: string | undefined
): string {
  const normalizedAttachmentID = attachmentID.trim();
  if (!normalizedAttachmentID) {
    return "";
  }
  const shareAttachmentBasePath = resolveShareAttachmentBasePath(payload);
  const downloadPath = shareAttachmentBasePath
    ? `${shareAttachmentBasePath}/${encodeURIComponent(normalizedAttachmentID)}/download`
    : "/api/docs/" +
      encodeURIComponent(documentID.trim()) +
      "/attachments/" +
      encodeURIComponent(normalizedAttachmentID) +
      "/download";
  const origin = normalizeReaderRequestOrigin(requestOrigin);
  return origin ? `${origin}${downloadPath}` : downloadPath;
}

function buildAttachmentAccessLinkPath(
  payload: ReaderPagePayload,
  documentID: string,
  attachmentID: string,
  purpose: "download" | "preview"
): string {
  const normalizedAttachmentID = attachmentID.trim();
  if (!normalizedAttachmentID) {
    return "";
  }
  const queryText = `purpose=${encodeURIComponent(purpose)}`;
  const shareAttachmentBasePath = resolveShareAttachmentBasePath(payload);
  if (shareAttachmentBasePath) {
    return `${shareAttachmentBasePath}/${encodeURIComponent(normalizedAttachmentID)}/access-link?${queryText}`;
  }
  const normalizedDocumentID = documentID.trim();
  if (!normalizedDocumentID) {
    return "";
  }
  return (
    "/api/docs/" +
    encodeURIComponent(normalizedDocumentID) +
    "/attachments/" +
    encodeURIComponent(normalizedAttachmentID) +
    "/access-link?" +
    queryText
  );
}

function buildAttachmentPreviewPagePath(
  payload: ReaderPagePayload,
  documentID: string,
  attachmentID: string
): string {
  const normalizedAttachmentID = attachmentID.trim();
  if (!normalizedAttachmentID) {
    return "";
  }
  if (payload.share?.enabled === true) {
    const shareSpaceID = (payload.share.spaceId ?? payload.space.id ?? "").trim();
    const shareDocKey = (payload.share.documentRouteKey ?? payload.document.routeKey ?? payload.document.id ?? "").trim();
    if (!shareSpaceID || !shareDocKey) {
      return "";
    }
    return (
      "/preview/shares/" +
      encodeURIComponent(shareSpaceID) +
      "/" +
      encodeURIComponent(shareDocKey) +
      "/attachments/" +
      encodeURIComponent(normalizedAttachmentID)
    );
  }
  const normalizedDocumentID = documentID.trim();
  if (!normalizedDocumentID) {
    return "";
  }
  return (
    "/preview/docs/" +
    encodeURIComponent(normalizedDocumentID) +
    "/attachments/" +
    encodeURIComponent(normalizedAttachmentID)
  );
}

function resolveAttachmentPreviewLabel(previewKind: ReaderDocumentAttachmentPayload["previewKind"]): string {
  switch (previewKind) {
    case "image":
      return "图片";
    case "pdf":
      return "PDF";
    case "office":
      return "Office";
    case "text":
      return "文本";
    default:
      return "文件";
  }
}

function buildReaderOutlineItems(markdownContent: string): ReaderOutlineItem[] {
  const parsedItems = parseTocFromMarkdown(markdownContent, readerTocParser).items;
  const outlineItems: ReaderOutlineItem[] = [];
  for (let index = 0; index < parsedItems.length; index += 1) {
    const item = parsedItems[index];
    const headingText = item.text.trim();
    if (!headingText) {
      continue;
    }
    outlineItems.push({
      index: outlineItems.length,
      level: Math.min(6, Math.max(1, item.level)),
      text: headingText
    });
  }
  return outlineItems;
}

// renderSpaceReader 将阅读页 payload 渲染为完整 HTML 文档字符串。
export function renderSpaceReader(payload: ReaderPagePayload): SpaceReaderRenderResult {
  const startedAt = Date.now();
  const renderedAt = new Date(startedAt);
  const payloadBytes = new TextEncoder().encode(JSON.stringify(payload)).length;
  const isOfficeDocument = isOfficeDocumentFormat(payload.document.format);
  const officeRendering = payload.officeRendering ?? {
    independentRenderEnabled: false,
    fallbackToOnlyOfficeOnRenderFailure: true
  };
  const officeRenderStatus = payload.document.renderStatus ?? "idle";
  const officeContentHtml = typeof payload.document.contentMd === "string" ? payload.document.contentMd.trim() : "";
  const useIndependentOfficeRender = isOfficeDocument && officeRendering.independentRenderEnabled === true;
  const shouldFallbackToOnlyOffice =
    useIndependentOfficeRender &&
    officeRenderStatus === "failed" &&
    officeRendering.fallbackToOnlyOfficeOnRenderFailure === true;
  const shouldRenderOnlyOffice = isOfficeDocument && (!useIndependentOfficeRender || shouldFallbackToOnlyOffice);
  const shouldRenderLocalOfficeHtml =
    useIndependentOfficeRender && officeRenderStatus === "success" && officeContentHtml.length > 0;
  const shouldRenderOfficeStatus =
    isOfficeDocument && useIndependentOfficeRender && !shouldRenderLocalOfficeHtml && !shouldFallbackToOnlyOffice;
  const documentRouteKey = (payload.document.routeKey || payload.document.id || "").trim() || payload.document.id;
  const shareEnabled = payload.share?.enabled === true;
  const readerBasePath = shareEnabled
    ? normalizeReaderPathPrefix(payload.share?.basePath, `/s/${encodeURIComponent(payload.space.id)}`)
    : `/r/${encodeURIComponent(payload.space.id)}`;
  const canonicalPath = `${readerBasePath}/${encodeURIComponent(documentRouteKey)}`;

  const resolvedTheme = resolvePreviewTheme(
    payload.document.themeId || DEFAULT_PREVIEW_THEME_TEMPLATE.id,
    BUILTIN_PREVIEW_THEME_TEMPLATES
  );
  const previewThemeClassName = getPreviewThemeClassName(resolvedTheme.id);
  const previewThemeStyleText = buildPreviewThemeStyleText(resolvedTheme);
  const previewThemeCustomStyleText = (resolvedTheme.customCss ?? "").trim();
  const articleTitle = payload.document.title.trim() || "未命名文档";
  const spaceTitle = payload.space.name.trim() || "未命名空间";
  const seoTitle = composeSEOTitle(payload.space.title || articleTitle);
  const hasDeniedAccess = Boolean(payload.access?.code?.trim());
  const markdownRenderer = buildReaderMarkdownRenderer(payload.document.contentMd, resolvedTheme, {
    requestOrigin: payload.requestOrigin
  });
  const updatedMeta = hasDeniedAccess ? "" : formatUpdatedMeta(payload.document.updatedAt, renderedAt);
  const authorNickname = payload.document.authorNickname.trim() || "未知作者";
  const documentMeta = `空间：${spaceTitle} · 作者：${authorNickname} · ${updatedMeta}`;
  const documentVisibilityMarker = hasDeniedAccess ? null : renderVisibilityMarker(payload.document.visibility);
  const readerOutlineItems = hasDeniedAccess || isOfficeDocument ? [] : buildReaderOutlineItems(payload.document.contentMd);
  const hasReaderOutline = readerOutlineItems.length > 0;
  const documentAttachments = hasDeniedAccess ? [] : (Array.isArray(payload.attachments) ? payload.attachments : []);
  const showSidebarTree = !shareEnabled;
  const spaceLandingPath = shareEnabled ? canonicalPath : `/r/${encodeURIComponent(payload.space.id)}`;
  const loginPath = `/login?redirect=${encodeURIComponent(canonicalPath)}`;
  const shouldNoIndex = isOfficeDocument;

  const appMarkup = renderToStaticMarkup(
    <html lang="zh-CN">
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>{seoTitle}</title>
        <link rel="canonical" href={canonicalPath} />
        {shouldNoIndex ? <meta name="robots" content="noindex, nofollow" /> : null}
        {/* 字体样式最先注入，确保后续 body/code 字体声明可即时命中 Google Sans Code。 */}
        <style id="plaindoc-reader-google-sans-code-style">{readerGoogleSansCodeStyleText}</style>
        <style id="plaindoc-reader-app-style">{appStyleText}</style>
        <style id="plaindoc-reader-base-style">{readerBaseStyleText}</style>
        <style id="plaindoc-reader-katex-style">{katexStyleText}</style>
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
        <div className={`reader-layout${showSidebarTree ? "" : " reader-layout--without-sidebar"}`}>
          {showSidebarTree ? (
            <aside
              id="plaindoc-reader-sidebar"
              className="reader-sidebar"
              data-reader-hook="sidebar"
              aria-label={`${spaceTitle} 文档目录`}
            >
              <div className="reader-sidebar__header">
                <div className="reader-sidebar__header-row">
                  <h2 className="reader-sidebar__title">{spaceTitle}</h2>
                  <button
                    type="button"
                    className="reader-sidebar__close"
                    data-reader-hook="mobile-sidebar-close"
                    aria-label="关闭目录"
                  >
                    ×
                  </button>
                </div>
              </div>
              <div className="reader-sidebar__tree-scroll">
                <ReaderTree
                  nodes={payload.tree}
                  spaceId={payload.space.id}
                  activeDocId={payload.activeDocId || payload.document.id}
                  linkBasePath={readerBasePath}
                />
              </div>
              <div className="reader-sidebar__footer" role="note" aria-label="发布信息">
                本文档使用{" "}
                <a
                  href="https://github.com/lifei6671/plaindoc"
                  title="PlainDoc"
                  target="_blank"
                  rel="noopener noreferrer nofollow"
                >
                  PlainDoc
                </a>{" "}
                发布
              </div>
            </aside>
          ) : null}
          <main
            className="reader-main"
            data-reader-hook="main"
            data-reader-outline={hasReaderOutline ? "1" : undefined}
          >
            {showSidebarTree ? (
              <div className="reader-mobile-bar" data-reader-hook="mobile-bar">
                <button
                  type="button"
                  className="reader-mobile-bar__toggle"
                  data-reader-hook="mobile-sidebar-open"
                  aria-controls="plaindoc-reader-sidebar"
                  aria-expanded="false"
                >
                  目录
                </button>
                <span className="reader-mobile-bar__title" data-reader-hook="mobile-bar-title" title={articleTitle}>
                  {articleTitle}
                </span>
              </div>
            ) : null}
            {hasReaderOutline ? (
              <aside className="reader-outline" data-reader-hook="outline" aria-label="文档大纲">
                <div className="reader-outline__title">文档大纲</div>
                <ol className="reader-outline__list">
                  {readerOutlineItems.map((item) => (
                    <li key={`${item.index}-${item.level}`} className="reader-outline__item">
                      <button
                        type="button"
                        className="reader-outline__link"
                        data-reader-hook="outline-link"
                        data-outline-index={item.index}
                        style={{ paddingLeft: `${12 + (item.level - 1) * 12}px` }}
                      >
                        {item.text}
                      </button>
                    </li>
                  ))}
                </ol>
              </aside>
            ) : null}
            <div
              id="plaindoc-reader-article-shell"
              className={`reader-article-shell${isOfficeDocument ? " reader-article-shell--office" : ""}`}
              data-reader-hook="article-shell"
            >
              {!hasDeniedAccess ? (
                <header className="reader-article-header">
                  <h1 className="reader-article-title">{articleTitle}</h1>
                  <div className="reader-article-meta">
                    {documentVisibilityMarker}
                    <span className="reader-article-meta__text">{documentMeta}</span>
                  </div>
                  <div className="reader-article-actions" role="group" aria-label={isOfficeDocument ? "文档操作" : "导出操作"}>
                    {isOfficeDocument ? (
                      <a
                        className="reader-article-action reader-article-action--primary"
                        data-reader-office-download="1"
                        aria-disabled="true"
                      >
                        <Download size={14} aria-hidden="true" />
                        <span>下载原文件</span>
                      </a>
                    ) : (
                      <>
                        <button
                          type="button"
                          className="reader-article-action"
                          data-reader-export-action="markdown"
                        >
                          <FileText size={14} aria-hidden="true" />
                          <span>导出 Markdown</span>
                        </button>
                        <button
                          type="button"
                          className="reader-article-action reader-article-action--primary"
                          data-reader-export-action="pdf"
                        >
                          <Printer size={14} aria-hidden="true" />
                          <span>导出 PDF</span>
                        </button>
                      </>
                    )}
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
                <>
                  {shouldRenderOnlyOffice ? (
                    <section className="office-pane reader-office-pane" aria-label={resolveOfficeDocumentLabel(payload.document.format)}>
                      <div className="office-pane__surface">
                        <div
                          id="plaindoc-reader-onlyoffice-editor"
                          className="office-pane__editor office-pane__editor--hidden"
                          data-reader-office-editor="1"
                          aria-label={resolveOfficeDocumentLabel(payload.document.format)}
                        />
                        <div
                          className="office-pane__placeholder"
                          data-reader-office-placeholder="1"
                          data-reader-office-status="loading"
                        >
                          <div className="office-pane__placeholder-icon" aria-hidden="true">
                            {renderOfficeDocumentIcon(payload.document.format)}
                            <LoaderCircle size={14} className="office-pane__spinner" />
                          </div>
                          <h2 className="office-pane__placeholder-title" data-reader-office-title="1">
                            {resolveOfficeDocumentLabel(payload.document.format)}
                          </h2>
                          <p className="office-pane__placeholder-description" data-reader-office-message="1">
                            {shouldFallbackToOnlyOffice ? "本地渲染失败，正在回退 ONLYOFFICE 阅读器..." : "正在加载 ONLYOFFICE 阅读器..."}
                          </p>
                        </div>
                      </div>
                    </section>
                  ) : shouldRenderLocalOfficeHtml ? (
                    <article
                      id={PREVIEW_BODY_ID}
                      className={`markdown-body ${PREVIEW_BODY_CLASS} ${previewThemeClassName}`}
                      dangerouslySetInnerHTML={{ __html: payload.document.contentMd }}
                    />
                  ) : shouldRenderOfficeStatus ? (
                    <section className="office-pane reader-office-pane" aria-label={resolveOfficeDocumentLabel(payload.document.format)}>
                      <div className="office-pane__surface">
                        <div
                          className={`office-pane__placeholder${officeRenderStatus === "failed" ? " office-pane__placeholder--error" : ""}`}
                          data-reader-office-placeholder="1"
                          data-reader-office-status={officeRenderStatus}
                        >
                          <div className="office-pane__placeholder-icon" aria-hidden="true">
                            {renderOfficeDocumentIcon(payload.document.format)}
                            {officeRenderStatus === "pending" || officeRenderStatus === "idle" ? (
                              <LoaderCircle size={14} className="office-pane__spinner" />
                            ) : null}
                          </div>
                          <h2 className="office-pane__placeholder-title" data-reader-office-title="1">
                            {resolveOfficeDocumentLabel(payload.document.format)}
                          </h2>
                          <p className="office-pane__placeholder-description" data-reader-office-message="1">
                            {officeRenderStatus === "failed"
                              ? (payload.document.renderError?.trim() || "本地 HTML 渲染失败")
                              : "正在生成本地阅读内容..."}
                          </p>
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
                  {documentAttachments.length > 0 ? (
                    <section className="reader-attachments" aria-label="文档附件">
                      <div className="reader-attachments__header">
                        <h2 className="reader-attachments__title">
                          <Paperclip size={15} aria-hidden="true" />
                          <span>文档附件</span>
                          <span className="reader-attachments__count">({documentAttachments.length})</span>
                        </h2>
                        <p className="reader-attachments__hint" data-reader-hook="attachment-status" aria-live="polite">
                          点击附件名称可直接下载，预览会打开独立预览页。
                        </p>
                      </div>
                      <ul className="reader-attachments__list">
                        {documentAttachments.map((attachment) => {
                          const attachmentID = (attachment.attachmentId ?? "").trim();
                          if (!attachmentID) {
                            return null;
                          }
                          const fileName = (attachment.fileName ?? "").trim() || attachmentID;
                          const mimeType = (attachment.mimeType ?? "").trim() || "application/octet-stream";
                          const documentID = (attachment.documentId ?? "").trim() || payload.document.id;
                          const downloadHref = buildAttachmentDownloadHref(
                            payload,
                            documentID,
                            attachmentID,
                            payload.requestOrigin
                          );
                          const downloadAccessLinkPath = buildAttachmentAccessLinkPath(
                            payload,
                            documentID,
                            attachmentID,
                            "download"
                          );
                          const previewAccessLinkPath = buildAttachmentAccessLinkPath(
                            payload,
                            documentID,
                            attachmentID,
                            "preview"
                          );
                          const previewPagePath = buildAttachmentPreviewPagePath(payload, documentID, attachmentID);
                          const previewKind = normalizeAttachmentPreviewKind(attachment.previewKind);
                          const previewSupported = attachment.previewSupported === true;
                          return (
                            <li key={attachmentID} className="reader-attachment">
                              <div className="reader-attachment__meta">
                                {downloadHref ? (
                                  <a
                                    className="reader-attachment__name reader-attachment__name-link"
                                    href={downloadHref}
                                    data-reader-attachment-link="1"
                                    title={fileName}
                                  >
                                    {fileName}
                                  </a>
                                ) : (
                                  <div className="reader-attachment__name" title={fileName}>
                                    {fileName}
                                  </div>
                                )}
                                <div className="reader-attachment__desc">
                                  <span>{formatAttachmentSize(Number(attachment.sizeBytes))}</span>
                                  <span>{resolveAttachmentPreviewLabel(previewKind)}</span>
                                  <span>{mimeType}</span>
                                </div>
                              </div>
                              <div className="reader-attachment__actions">
                                <button
                                  type="button"
                                  className="reader-attachment__action"
                                  data-reader-attachment-action="1"
                                  data-reader-attachment-purpose="download"
                                  data-reader-attachment-id={attachmentID}
                                  data-reader-doc-id={documentID}
                                  data-reader-attachment-access-link-path={downloadAccessLinkPath || undefined}
                                >
                                  <Download size={14} aria-hidden="true" />
                                  <span>下载</span>
                                </button>
                                {previewSupported ? (
                                  <button
                                    type="button"
                                    className="reader-attachment__action reader-attachment__action--preview"
                                    data-reader-attachment-action="1"
                                    data-reader-attachment-purpose="preview"
                                    data-reader-attachment-id={attachmentID}
                                    data-reader-doc-id={documentID}
                                    data-reader-attachment-access-link-path={previewAccessLinkPath || undefined}
                                    data-reader-attachment-preview-page-path={previewPagePath || undefined}
                                  >
                                    <Eye size={14} aria-hidden="true" />
                                    <span>预览</span>
                                  </button>
                                ) : null}
                              </div>
                            </li>
                          );
                        })}
                      </ul>
                    </section>
                  ) : null}
                </>
              )}
            </div>
          </main>
        </div>
        {showSidebarTree ? (
          <button
            type="button"
            className="reader-mobile-overlay"
            data-reader-hook="mobile-overlay"
            aria-label="关闭目录"
            hidden
          />
        ) : null}
        <ReaderImageViewerShell />
        <script
          id="plaindoc-reader-state"
          type="application/json"
          dangerouslySetInnerHTML={{
            __html: escapeJSONForScript(JSON.stringify(payload))
          }}
        />
        <script
          dangerouslySetInnerHTML={{
            __html: READER_ASYNC_ENHANCEMENT_SCRIPT
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
