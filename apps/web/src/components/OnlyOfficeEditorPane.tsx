import { AlertCircle, FileSpreadsheet, FileText, LoaderCircle } from "lucide-react";
import { memo, useCallback, useEffect, useId, useRef, useState, type ReactNode } from "react";
import type { DocumentFormat, OnlyOfficeEditConfig } from "../data-access";

type OnlyOfficeEditorRuntimeStatus = "loading" | "ready" | "dirty" | "error";

export interface OnlyOfficeEditorPaneState {
  status: OnlyOfficeEditorRuntimeStatus;
  message: string;
  shouldRefreshMetadata?: boolean;
}

interface OnlyOfficeEditorPaneProps {
  documentId: string | null;
  documentTitle: string;
  documentFormat: DocumentFormat;
  editConfig: OnlyOfficeEditConfig | null;
  isConfigLoading: boolean;
  errorMessage?: string | null;
  onStateChange?: (state: OnlyOfficeEditorPaneState) => void;
}

interface OnlyOfficeDocEditorInstance {
  destroyEditor?: () => void;
}

interface OnlyOfficeDocsAPI {
  DocEditor: new (placeholderId: string, config: Record<string, unknown>) => OnlyOfficeDocEditorInstance;
}

type OnlyOfficeEventHandler = (...args: unknown[]) => void;

interface OnlyOfficeRuntimeEventHandlers {
  onDocumentReady(): void;
  onDocumentStateChange(hasPendingChanges: boolean): void;
  onError(message: string): void;
}

declare global {
  interface Window {
    DocsAPI?: OnlyOfficeDocsAPI;
  }
}

const onlyOfficeScriptLoaders = new Map<string, Promise<void>>();

export const OnlyOfficeEditorPane = memo(function OnlyOfficeEditorPane({
  documentId,
  documentTitle,
  documentFormat,
  editConfig,
  isConfigLoading,
  errorMessage,
  onStateChange
}: OnlyOfficeEditorPaneProps) {
  const containerId = `onlyoffice-editor-${useId().replace(/:/g, "-")}`;
  const editorInstanceRef = useRef<OnlyOfficeDocEditorInstance | null>(null);
  const onStateChangeRef = useRef(onStateChange);
  const [runtimeState, setRuntimeState] = useState<OnlyOfficeEditorPaneState>({
    status: "loading",
    message: "正在获取 ONLYOFFICE 配置..."
  });

  useEffect(() => {
    onStateChangeRef.current = onStateChange;
  }, [onStateChange]);

  const publishState = useCallback((nextState: OnlyOfficeEditorPaneState) => {
    setRuntimeState(nextState);
    onStateChangeRef.current?.(nextState);
  }, []);

  useEffect(() => {
    if (!documentId) {
      return;
    }

    if (errorMessage) {
      const nextState: OnlyOfficeEditorPaneState = {
        status: "error",
        message: normalizeOnlyOfficeErrorMessage(errorMessage)
      };
      publishState(nextState);
      return;
    }

    if (isConfigLoading || !editConfig) {
      const nextState: OnlyOfficeEditorPaneState = {
        status: "loading",
        message: "正在获取 ONLYOFFICE 配置..."
      };
      publishState(nextState);
      return;
    }

    let cancelled = false;
    const loadingState: OnlyOfficeEditorPaneState = {
      status: "loading",
      message: "正在加载 ONLYOFFICE 编辑器..."
    };
    publishState(loadingState);

    void (async () => {
      try {
        await loadOnlyOfficeApiScript(editConfig.documentServerUrl);
        if (cancelled) {
          return;
        }

        const DocEditor = window.DocsAPI?.DocEditor;
        if (typeof DocEditor !== "function") {
          throw new Error("ONLYOFFICE Docs API 未就绪");
        }

        editorInstanceRef.current?.destroyEditor?.();
        const runtimeConfig = attachOnlyOfficeRuntimeEvents(
          cloneOnlyOfficeConfigPayload(editConfig.config),
          {
            onDocumentReady: () => {
              if (cancelled) {
                return;
              }
              publishState({
                status: "ready",
                message: `${resolveOnlyOfficeFormatLabel(documentFormat)}编辑器已就绪`
              });
            },
            onDocumentStateChange: (hasPendingChanges) => {
              if (cancelled) {
                return;
              }
              if (hasPendingChanges) {
                publishState({
                  status: "dirty",
                  message: `${resolveOnlyOfficeFormatLabel(documentFormat)}文档存在未提交更改`
                });
                return;
              }
              publishState({
                status: "ready",
                message: `${resolveOnlyOfficeFormatLabel(documentFormat)}文档已同步`,
                shouldRefreshMetadata: true
              });
            },
            onError: (message) => {
              if (cancelled) {
                return;
              }
              publishState({
                status: "error",
                message: normalizeOnlyOfficeErrorMessage(message)
              });
            }
          }
        );

        editorInstanceRef.current = new DocEditor(containerId, runtimeConfig);
        publishState({
          status: "loading",
          message: "正在初始化 ONLYOFFICE 文档..."
        });
      } catch (error) {
        if (cancelled) {
          return;
        }
        const nextState: OnlyOfficeEditorPaneState = {
          status: "error",
          message: normalizeOnlyOfficeErrorMessage(
            error instanceof Error && error.message.trim()
              ? error.message
              : "加载 ONLYOFFICE 编辑器失败"
          )
        };
        publishState(nextState);
      }
    })();

    return () => {
      cancelled = true;
      editorInstanceRef.current?.destroyEditor?.();
      editorInstanceRef.current = null;
    };
  }, [
    containerId,
    documentFormat,
    documentId,
    editConfig,
    errorMessage,
    isConfigLoading,
    publishState
  ]);

  const isEditorReady = runtimeState.status === "ready" || runtimeState.status === "dirty";
  const titleText = resolveOnlyOfficeFormatLabel(documentFormat).trim() + resolveOnlyOfficeTitleSuffix(documentTitle);

  return (
    <section
      className={`pane office-pane office-pane--${runtimeState.status}`}
      aria-busy={runtimeState.status === "loading"}
    >
      <div className="office-pane__surface">
        <div
          id={containerId}
          className={`office-pane__editor ${isEditorReady ? "" : "office-pane__editor--hidden"}`}
          aria-label={titleText}
        />
        {!isEditorReady ? (
          <div className={`office-pane__placeholder office-pane__placeholder--${runtimeState.status}`}>
            <div className="office-pane__placeholder-icon" aria-hidden="true">
              {resolveOnlyOfficePlaceholderIcon(documentFormat, runtimeState.status)}
              {runtimeState.status === "loading" ? (
                <LoaderCircle size={14} className="office-pane__spinner" />
              ) : null}
            </div>
            <h2 className="office-pane__placeholder-title">{titleText}</h2>
            <p className="office-pane__placeholder-description">{runtimeState.message}</p>
          </div>
        ) : null}
      </div>
    </section>
  );
});

function resolveOnlyOfficeTitleSuffix(documentTitle: string): string {
  const normalizedTitle = documentTitle.trim();
  if (!normalizedTitle) {
    return "";
  }
  return ` · ${normalizedTitle}`;
}

function resolveOnlyOfficeFormatLabel(documentFormat: DocumentFormat): string {
  switch (documentFormat) {
    case "xlsx":
      return "Excel";
    case "docx":
      return "Word";
    default:
      return "ONLYOFFICE";
  }
}

function cloneOnlyOfficeConfigPayload(input: Record<string, unknown>): Record<string, unknown> {
  if (typeof structuredClone === "function") {
    return structuredClone(input);
  }
  return JSON.parse(JSON.stringify(input)) as Record<string, unknown>;
}

function attachOnlyOfficeRuntimeEvents(
  config: Record<string, unknown>,
  runtimeHandlers: OnlyOfficeRuntimeEventHandlers
): Record<string, unknown> {
  const existingEvents = isRecord(config.events) ? config.events : {};
  const onDocumentReady = toOnlyOfficeEventHandler(existingEvents.onDocumentReady);
  const onDocumentStateChange = toOnlyOfficeEventHandler(existingEvents.onDocumentStateChange);
  const onError = toOnlyOfficeEventHandler(existingEvents.onError);

  return {
    ...config,
    events: {
      ...existingEvents,
      onDocumentReady: (...args: unknown[]) => {
        onDocumentReady?.(...args);
        runtimeHandlers.onDocumentReady();
      },
      onDocumentStateChange: (event: unknown, ...args: unknown[]) => {
        onDocumentStateChange?.(event, ...args);
        runtimeHandlers.onDocumentStateChange(resolveOnlyOfficePendingChanges(event));
      },
      onError: (event: unknown, ...args: unknown[]) => {
        onError?.(event, ...args);
        runtimeHandlers.onError(resolveOnlyOfficeErrorMessage(event));
      }
    }
  };
}

function buildOnlyOfficeApiScriptURL(documentServerUrl: string): string {
  const normalizedBaseURL = documentServerUrl.trim().replace(/\/+$/, "");
  return `${normalizedBaseURL}/web-apps/apps/api/documents/api.js`;
}

function loadOnlyOfficeApiScript(documentServerUrl: string): Promise<void> {
  if (typeof window === "undefined" || typeof document === "undefined") {
    return Promise.reject(new Error("ONLYOFFICE 仅支持浏览器环境"));
  }
  if (typeof window.DocsAPI?.DocEditor === "function") {
    return Promise.resolve();
  }

  const scriptURL = buildOnlyOfficeApiScriptURL(documentServerUrl);
  if (!scriptURL.trim()) {
    return Promise.reject(new Error("ONLYOFFICE 服务地址为空"));
  }

  const cachedLoader = onlyOfficeScriptLoaders.get(scriptURL);
  if (cachedLoader) {
    return cachedLoader;
  }

  const existingScript = document.querySelector<HTMLScriptElement>(
    `script[data-onlyoffice-api="${scriptURL}"]`
  );

  const loader = new Promise<void>((resolve, reject) => {
    const handleLoad = () => {
      if (typeof window.DocsAPI?.DocEditor !== "function") {
        reject(new Error("ONLYOFFICE Docs API 未就绪"));
        return;
      }
      resolve();
    };
    const handleError = () => {
      onlyOfficeScriptLoaders.delete(scriptURL);
      reject(new Error("加载 ONLYOFFICE 脚本失败"));
    };

    const scriptElement = existingScript ?? document.createElement("script");
    scriptElement.addEventListener("load", handleLoad, { once: true });
    scriptElement.addEventListener("error", handleError, { once: true });

    if (!existingScript) {
      scriptElement.src = scriptURL;
      scriptElement.async = true;
      scriptElement.defer = true;
      scriptElement.dataset.onlyofficeApi = scriptURL;
      document.head.appendChild(scriptElement);
    }
  });

  onlyOfficeScriptLoaders.set(scriptURL, loader);
  return loader;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object";
}

function toOnlyOfficeEventHandler(value: unknown): OnlyOfficeEventHandler | null {
  return typeof value === "function" ? (value as OnlyOfficeEventHandler) : null;
}

function resolveOnlyOfficePendingChanges(event: unknown): boolean {
  if (!isRecord(event)) {
    return false;
  }
  return event.data === true;
}

function resolveOnlyOfficeErrorMessage(event: unknown): string {
  if (isRecord(event)) {
    const directMessage = typeof event.message === "string" ? event.message.trim() : "";
    if (directMessage) {
      return directMessage;
    }
    if (isRecord(event.data)) {
      const detail = event.data;
      const description =
        typeof detail.errorDescription === "string"
          ? detail.errorDescription.trim()
          : typeof detail.errorMessage === "string"
            ? detail.errorMessage.trim()
            : typeof detail.message === "string"
              ? detail.message.trim()
              : "";
      if (description) {
        return description;
      }
    }
  }
  return "ONLYOFFICE 编辑器发生错误";
}

function resolveOnlyOfficePlaceholderIcon(
  documentFormat: DocumentFormat,
  runtimeStatus: OnlyOfficeEditorRuntimeStatus
): ReactNode {
  if (runtimeStatus === "error") {
    return <AlertCircle size={20} />;
  }
  switch (documentFormat) {
    case "xlsx":
      return <FileSpreadsheet size={20} />;
    case "docx":
      return <FileText size={20} />;
    default:
      return <FileText size={20} />;
  }
}

function normalizeOnlyOfficeErrorMessage(message: string): string {
  const normalizedMessage = message.trim();
  if (!normalizedMessage) {
    return "加载 ONLYOFFICE 编辑器失败";
  }

  const lowerMessage = normalizedMessage.toLowerCase();
  if (
    lowerMessage.includes("401") ||
    lowerMessage.includes("unauthorized") ||
    lowerMessage.includes("token expired") ||
    lowerMessage.includes("session expired") ||
    lowerMessage.includes("会话过期") ||
    lowerMessage.includes("登录失效")
  ) {
    return "登录会话已过期，请刷新页面后重新登录。";
  }
  if (
    lowerMessage.includes("403") ||
    lowerMessage.includes("forbidden") ||
    lowerMessage.includes("permission denied") ||
    lowerMessage.includes("access denied") ||
    lowerMessage.includes("do not have rights") ||
    lowerMessage.includes("you are trying to perform an action you do not have rights") ||
    lowerMessage.includes("权限不足") ||
    lowerMessage.includes("无权限")
  ) {
    return "当前账号没有该文档的编辑权限。";
  }
  if (
    lowerMessage.includes("加载 onlyoffice 脚本失败") ||
    lowerMessage.includes("docs api 未就绪") ||
    lowerMessage.includes("script") ||
    lowerMessage.includes("network")
  ) {
    return "ONLYOFFICE 脚本加载失败，请检查 Document Server 连接后重试。";
  }
  return normalizedMessage;
}
