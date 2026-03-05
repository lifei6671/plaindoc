package handler

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

type documentAttachmentPreviewPageData struct {
	DocumentID     string
	AttachmentID   string
	AccessLinkPath string
}

var documentAttachmentPreviewPageTemplate = template.Must(
	template.New("document_attachment_preview_page").Parse(`<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>附件在线预览</title>
    <style>
      :root {
        color-scheme: light;
        font-family: "Noto Sans SC","PingFang SC","Microsoft YaHei",sans-serif;
        --preview-border: #d4d4d8;
        --preview-muted: #64748b;
        --preview-bg: #f8fafc;
        --preview-surface: #ffffff;
        --preview-text: #111827;
        --preview-accent: #1d4ed8;
        --preview-danger: #dc2626;
      }
      * {
        box-sizing: border-box;
      }
      body {
        margin: 0;
        color: var(--preview-text);
        background: radial-gradient(circle at top right, #e0f2fe 0, #f8fafc 42%, #f8fafc 100%);
      }
      .preview-page {
        min-height: 100vh;
        display: grid;
        grid-template-rows: auto 1fr;
      }
      .preview-toolbar {
        display: flex;
        flex-wrap: wrap;
        gap: 12px;
        align-items: center;
        justify-content: space-between;
        padding: 12px 20px;
        border-bottom: 1px solid var(--preview-border);
        background: rgba(255, 255, 255, 0.9);
        backdrop-filter: blur(4px);
      }
      .preview-title {
        margin: 0;
        font-size: 15px;
        font-weight: 700;
      }
      .preview-meta {
        margin: 2px 0 0;
        color: var(--preview-muted);
        font-size: 12px;
      }
      .preview-toast-viewport {
        position: fixed;
        top: 12px;
        left: 50%;
        transform: translateX(-50%);
        width: min(560px, calc(100vw - 24px));
        z-index: 120;
        pointer-events: none;
      }
      .preview-toast {
        display: flex;
        align-items: center;
        gap: 10px;
        border-radius: 12px;
        border: 1px solid #cbd5e1;
        background: rgba(255, 255, 255, 0.98);
        color: #0f172a;
        box-shadow: 0 12px 26px rgba(2, 8, 23, 0.12);
        padding: 10px 12px;
        opacity: 0;
        transform: translateY(-16px) scale(0.98);
        transition: transform 220ms cubic-bezier(0.21, 1.02, 0.73, 1), opacity 180ms ease;
      }
      .preview-toast--visible {
        opacity: 1;
        transform: translateY(0) scale(1);
      }
      .preview-toast--leaving {
        opacity: 0;
        transform: translateY(-8px) scale(0.98);
      }
      .preview-toast--error {
        border-color: #fecaca;
        background: #fff1f2;
        color: #b91c1c;
      }
      .preview-toast__dot {
        width: 8px;
        height: 8px;
        border-radius: 999px;
        background: #2563eb;
        flex: 0 0 auto;
      }
      .preview-toast--error .preview-toast__dot {
        background: #dc2626;
      }
      .preview-toast__text {
        margin: 0;
        font-size: 13px;
        line-height: 1.35;
        font-weight: 600;
      }
      .preview-actions {
        display: inline-flex;
        align-items: center;
        gap: 8px;
      }
      .preview-button {
        border: 1px solid var(--preview-border);
        border-radius: 8px;
        background: var(--preview-surface);
        color: var(--preview-text);
        padding: 8px 12px;
        font-size: 12px;
        font-weight: 600;
        cursor: pointer;
      }
      .preview-button:disabled {
        opacity: 0.6;
        cursor: wait;
      }
      .preview-button--primary {
        border-color: var(--preview-accent);
        color: #fff;
        background: linear-gradient(135deg, #1d4ed8 0%, #2563eb 100%);
      }
      .preview-container {
        padding: 16px 20px 20px;
      }
      .preview-content {
        min-height: calc(100vh - 152px);
        border: 1px solid var(--preview-border);
        border-radius: 12px;
        background: var(--preview-surface);
        overflow: hidden;
      }
      .preview-iframe {
        border: 0;
        width: 100%;
        min-height: calc(100vh - 154px);
      }
      .preview-image {
        width: 100%;
        display: block;
        object-fit: contain;
        background: var(--preview-bg);
      }
      .preview-placeholder {
        min-height: 240px;
        padding: 28px;
        display: grid;
        gap: 12px;
        align-content: start;
      }
      .preview-placeholder p {
        margin: 0;
        color: var(--preview-muted);
        line-height: 1.5;
      }
      @media (max-width: 768px) {
        .preview-toolbar {
          padding: 10px 12px;
        }
        .preview-container {
          padding: 10px 12px 14px;
        }
      }
    </style>
  </head>
  <body data-doc-id="{{.DocumentID}}" data-attachment-id="{{.AttachmentID}}" data-access-link-path="{{.AccessLinkPath}}">
    <div class="preview-toast-viewport" id="preview-toast-viewport" aria-live="polite" aria-atomic="true"></div>
    <div class="preview-page">
      <header class="preview-toolbar">
        <div>
          <h1 class="preview-title">附件在线预览</h1>
          <p class="preview-meta" id="preview-meta">准备加载附件...</p>
        </div>
        <div class="preview-actions">
          <button type="button" class="preview-button" id="retry-btn">重试</button>
          <button type="button" class="preview-button preview-button--primary" id="download-btn">下载</button>
        </div>
      </header>
      <main class="preview-container">
        <section class="preview-content" id="preview-content"></section>
      </main>
    </div>
    <script>
      (() => {
        const body = document.body;
        const documentID = (body.getAttribute("data-doc-id") || "").trim();
        const attachmentID = (body.getAttribute("data-attachment-id") || "").trim();
        const accessLinkPath = (body.getAttribute("data-access-link-path") || "").trim();
        const toastViewport = document.getElementById("preview-toast-viewport");
        const contentNode = document.getElementById("preview-content");
        const metaNode = document.getElementById("preview-meta");
        const retryButton = document.getElementById("retry-btn");
        const downloadButton = document.getElementById("download-btn");
        let toastTimer = 0;
        let activeToast = null;

        const PREVIEW_KIND_LABELS = {
          none: "未知类型",
          image: "图片",
          pdf: "PDF 文档",
          office: "Office 文档",
          text: "文本文件"
        };

        const normalizePreviewKind = (value) => {
          const normalized = typeof value === "string" ? value.trim().toLowerCase() : "";
          if (normalized === "image" || normalized === "pdf" || normalized === "office" || normalized === "text") {
            return normalized;
          }
          return "none";
        };

        const resolveAbsoluteURL = (value) => {
          const normalized = typeof value === "string" ? value.trim() : "";
          if (!normalized) {
            return "";
          }
          if (/^(https?:)?\/\//i.test(normalized) || /^blob:|^data:/i.test(normalized)) {
            return normalized;
          }
          return new URL(normalized, window.location.origin).toString();
        };

        const readEnvelopeData = (payload) => {
          if (!payload || typeof payload !== "object") {
            return null;
          }
          if ("data" in payload && payload.data && typeof payload.data === "object") {
            return payload.data;
          }
          return payload;
        };

        const parseJSON = (rawText) => {
          const text = typeof rawText === "string" ? rawText.trim() : "";
          if (!text) {
            return null;
          }
          try {
            return JSON.parse(text);
          } catch {
            return null;
          }
        };

        const clearToast = () => {
          if (toastTimer) {
            window.clearTimeout(toastTimer);
            toastTimer = 0;
          }
          if (!(activeToast instanceof HTMLElement)) {
            return;
          }
          activeToast.classList.remove("preview-toast--visible");
          activeToast.classList.add("preview-toast--leaving");
          const targetToast = activeToast;
          activeToast = null;
          window.setTimeout(() => {
            if (targetToast.parentElement) {
              targetToast.parentElement.removeChild(targetToast);
            }
          }, 220);
        };

        const setStatus = (message, isError, durationMs) => {
          if (!(toastViewport instanceof HTMLElement)) {
            return;
          }
          const text = typeof message === "string" ? message.trim() : "";
          if (!text) {
            return;
          }

          clearToast();
          const toast = document.createElement("div");
          toast.className = "preview-toast" + (isError === true ? " preview-toast--error" : "");

          const dot = document.createElement("span");
          dot.className = "preview-toast__dot";
          toast.appendChild(dot);

          const textNode = document.createElement("p");
          textNode.className = "preview-toast__text";
          textNode.textContent = text;
          toast.appendChild(textNode);

          toastViewport.appendChild(toast);
          activeToast = toast;
          window.requestAnimationFrame(() => {
            toast.classList.add("preview-toast--visible");
          });

          const resolvedDuration = Number.isFinite(durationMs) && durationMs > 0
            ? durationMs
            : (isError === true ? 2600 : 1400);
          toastTimer = window.setTimeout(() => {
            clearToast();
          }, resolvedDuration);
        };

        const clearPreviewContent = () => {
          if (!(contentNode instanceof HTMLElement)) {
            return;
          }
          contentNode.innerHTML = "";
        };

        const renderPlaceholder = (message) => {
          if (!(contentNode instanceof HTMLElement)) {
            return;
          }
          clearPreviewContent();
          const box = document.createElement("div");
          box.className = "preview-placeholder";
          const textNode = document.createElement("p");
          textNode.textContent = message;
          box.appendChild(textNode);
          contentNode.appendChild(box);
        };

        const renderImage = (resourceURL) => {
          if (!(contentNode instanceof HTMLElement)) {
            return;
          }
          clearPreviewContent();
          const image = document.createElement("img");
          image.className = "preview-image";
          image.alt = "附件预览";
          image.src = resourceURL;
          contentNode.appendChild(image);
        };

        const renderFrame = (resourceURL) => {
          if (!(contentNode instanceof HTMLElement)) {
            return;
          }
          clearPreviewContent();
          const frame = document.createElement("iframe");
          frame.className = "preview-iframe";
          frame.src = resourceURL;
          frame.loading = "eager";
          frame.referrerPolicy = "same-origin";
          contentNode.appendChild(frame);
        };

        const renderOffice = (resourceURL, requiresAuth) => {
          if (requiresAuth) {
            renderPlaceholder("当前附件需要鉴权，Office 在线预览服务无法直接访问该链接，请点击右上角下载按钮查看。");
            return;
          }
          const officeRenderers = window.__PLAINDOC_ATTACHMENT_PREVIEW_RENDERERS__;
          if (officeRenderers && typeof officeRenderers === "object") {
            const customOfficeRenderer = officeRenderers.office;
            if (typeof customOfficeRenderer === "function") {
              const handled = customOfficeRenderer({
                container: contentNode,
                sourceURL: resourceURL
              });
              if (handled === true) {
                return;
              }
            }
          }
          const officeViewerURL = "https://view.officeapps.live.com/op/embed.aspx?src=" + encodeURIComponent(resourceURL);
          renderFrame(officeViewerURL);
        };

        const requestAttachmentAccessLink = async (purpose) => {
          const queryText = "purpose=" + encodeURIComponent(purpose);
          const requestPath = accessLinkPath
            ? accessLinkPath + (accessLinkPath.indexOf("?") >= 0 ? "&" : "?") + queryText
            : "/api/docs/" +
              encodeURIComponent(documentID) +
              "/attachments/" +
              encodeURIComponent(attachmentID) +
              "/access-link?" +
              queryText;
          const response = await fetch(requestPath, {
            method: "POST",
            credentials: "include",
            headers: {
              Accept: "application/json",
              "X-Requested-With": "plaindoc-attachment-preview-page"
            }
          });
          const rawResponseText = await response.text();
          const payload = parseJSON(rawResponseText);
          const resultData = readEnvelopeData(payload);
          const resultCode =
            payload && typeof payload === "object" && typeof payload.code === "number"
              ? payload.code
              : 0;
          const resultMessage =
            payload && typeof payload === "object" && typeof payload.message === "string"
              ? payload.message.trim()
              : "";
          const accessURL =
            resultData && typeof resultData === "object" && typeof resultData.url === "string"
              ? resolveAbsoluteURL(resultData.url)
              : "";
          if (!response.ok || resultCode !== 0 || !accessURL) {
            throw new Error(resultMessage || "附件访问链接生成失败");
          }
          return {
            url: accessURL,
            previewKind: normalizePreviewKind(resultData.previewKind),
            requiresAuth: resultData.requiresAuth === true
          };
        };

        const openDownload = async () => {
          if (!(downloadButton instanceof HTMLButtonElement)) {
            return;
          }
          downloadButton.disabled = true;
          try {
            setStatus("正在生成下载链接...", false, 1200);
            const accessLink = await requestAttachmentAccessLink("download");
            window.open(accessLink.url, "_blank", "noopener,noreferrer");
            setStatus("已开始下载。", false, 1500);
          } catch (error) {
            const message =
              error && typeof error === "object" && "message" in error && typeof error.message === "string"
                ? error.message
                : "生成下载链接失败，请稍后重试。";
            setStatus(message, true, 2600);
          } finally {
            downloadButton.disabled = false;
          }
        };

        const loadPreview = async () => {
          if (!documentID || !attachmentID) {
            setStatus("附件参数无效，请关闭后重新打开预览。", true, 2600);
            renderPlaceholder("附件参数无效，无法继续预览。");
            return;
          }
          if (retryButton instanceof HTMLButtonElement) {
            retryButton.disabled = true;
          }
          setStatus("正在生成预览链接...", false, 1200);
          try {
            const accessLink = await requestAttachmentAccessLink("preview");
            if (metaNode instanceof HTMLElement) {
              const previewKindLabel = PREVIEW_KIND_LABELS[accessLink.previewKind] || PREVIEW_KIND_LABELS.none;
              metaNode.textContent = "预览类型：" + previewKindLabel;
            }
            if (accessLink.previewKind === "image") {
              renderImage(accessLink.url);
            } else if (accessLink.previewKind === "pdf" || accessLink.previewKind === "text") {
              renderFrame(accessLink.url);
            } else if (accessLink.previewKind === "office") {
              renderOffice(accessLink.url, accessLink.requiresAuth);
            } else {
              renderPlaceholder("当前文件类型暂不支持在线预览，请点击右上角下载按钮查看。");
            }
            setStatus("预览已就绪。", false, 1400);
          } catch (error) {
            const message =
              error && typeof error === "object" && "message" in error && typeof error.message === "string"
                ? error.message
                : "加载预览失败，请稍后重试。";
            setStatus(message, true, 2800);
            renderPlaceholder("预览加载失败，请点击“重试”或使用下载功能。");
          } finally {
            if (retryButton instanceof HTMLButtonElement) {
              retryButton.disabled = false;
            }
          }
        };

        if (retryButton instanceof HTMLButtonElement) {
          retryButton.addEventListener("click", () => {
            loadPreview();
          });
        }
        if (downloadButton instanceof HTMLButtonElement) {
          downloadButton.addEventListener("click", () => {
            openDownload();
          });
        }

        loadPreview();
      })();
    </script>
  </body>
</html>`),
)

const documentAttachmentPreviewPageUnavailableHTML = `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"/><title>附件预览暂不可用</title></head><body><h1>附件预览暂不可用</h1><p>请稍后重试。</p></body></html>`

// DocumentAttachmentPreviewPage 渲染文档附件在线预览页。
func (h *workspaceHandler) DocumentAttachmentPreviewPage(c *gin.Context) {
	if c == nil {
		return
	}
	if h == nil {
		setRequestErrmsgText(c, "附件预览页初始化失败: workspace handler is nil")
		c.Data(http.StatusInternalServerError, "text/html; charset=utf-8", []byte(documentAttachmentPreviewPageUnavailableHTML))
		return
	}

	documentID := strings.TrimSpace(c.Param("docId"))
	if documentID == "" {
		setRequestErrmsgText(c, "附件预览页参数错误: docId is empty")
		c.String(http.StatusBadRequest, "document id is required")
		return
	}
	attachmentID := strings.TrimSpace(c.Param("attachmentId"))
	if attachmentID == "" {
		setRequestErrmsgText(c, "附件预览页参数错误: attachmentId is empty")
		c.String(http.StatusBadRequest, "attachment id is required")
		return
	}

	appendVaryHeader(c, "Authorization")
	appendVaryHeader(c, "Cookie")
	c.Header("Cache-Control", "private, no-store, max-age=0")

	pageHTML, err := buildDocumentAttachmentPreviewPageHTML(documentAttachmentPreviewPageData{
		DocumentID:     documentID,
		AttachmentID:   attachmentID,
		AccessLinkPath: buildWorkspaceDocumentAttachmentAccessLinkPath(documentID, attachmentID),
	})
	if err != nil {
		setRequestErrmsg(c, err, "渲染附件预览页失败")
		c.Data(http.StatusInternalServerError, "text/html; charset=utf-8", []byte(documentAttachmentPreviewPageUnavailableHTML))
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(pageHTML))
}

func buildDocumentAttachmentPreviewPageHTML(data documentAttachmentPreviewPageData) (string, error) {
	if strings.TrimSpace(data.DocumentID) == "" || strings.TrimSpace(data.AttachmentID) == "" {
		return "", errors.New("document id or attachment id is empty")
	}
	accessLinkPath := strings.TrimSpace(data.AccessLinkPath)
	if accessLinkPath == "" {
		accessLinkPath = buildWorkspaceDocumentAttachmentAccessLinkPath(data.DocumentID, data.AttachmentID)
	}
	data.AccessLinkPath = accessLinkPath
	var builder bytes.Buffer
	if err := documentAttachmentPreviewPageTemplate.Execute(&builder, data); err != nil {
		return "", err
	}
	return builder.String(), nil
}

func buildWorkspaceDocumentAttachmentAccessLinkPath(documentID string, attachmentID string) string {
	return "/api/docs/" +
		url.PathEscape(strings.TrimSpace(documentID)) +
		"/attachments/" +
		url.PathEscape(strings.TrimSpace(attachmentID)) +
		"/access-link"
}
