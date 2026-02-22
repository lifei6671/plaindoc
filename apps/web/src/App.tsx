import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import { languages } from "@codemirror/language-data";
import { EditorSelection } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import CodeMirror from "@uiw/react-codemirror";
import {
  AlertCircle,
  CheckCircle2,
  GripVertical,
  Link2,
  LoaderCircle,
  LogOut,
  Monitor,
  PanelLeftClose,
  PanelLeftOpen,
  Smartphone
} from "lucide-react";
import MarkdownIt from "markdown-it";
// KaTeX mhchem 扩展：支持 `\\ce{}` 化学公式语法。
import "katex/contrib/mhchem";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type PointerEvent as ReactPointerEvent
} from "react";
import ReactMarkdown from "react-markdown";
import { useLocation, useNavigate } from "react-router-dom";
import rehypeKatex from "rehype-katex";
import rehypeRaw from "rehype-raw";
import rehypeSanitize from "rehype-sanitize";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import { AuthPanel } from "./components/AuthPanel";
import { EditorAccessErrorPage } from "./components/EditorAccessErrorPage";
import { WorkspaceSidebar } from "./components/WorkspaceSidebar";
import { ThemeMenu } from "./components/ThemeMenu";
import { TocMenu } from "./components/TocMenu";
import { useConfirmDialog } from "./components/ConfirmDialog";
import { Toaster } from "./components/ui/sonner";
import { AdminApp } from "./admin/AdminApp";
import { ADMIN_LOGIN_ROUTE_PATH, ADMIN_ROUTE_BASE_PATH } from "./admin/routes";
import { ConflictError, getDataGateway, type AuthSession, type CreateNodeResult } from "./data-access";
import {
  DEFAULT_PREVIEW_THEME_ID,
  FALLBACK_CONTENT,
  PREVIEW_BODY_CLASS,
  PREVIEW_BODY_ID,
  PREVIEW_CUSTOM_STYLE_EVENT,
  PREVIEW_CUSTOM_STYLE_STORAGE_KEY,
  PREVIEW_PANE_CLASS,
  PREVIEW_PANE_ID,
  PREVIEW_VIEWPORT_MODE_STORAGE_KEY
} from "./editor/constants";
import { buildMarkdownComponents } from "./editor/markdown-components";
import {
  extractPlainTextFromMarkdown,
  parseTocFromMarkdown,
  remarkBlockAnchorPlugin
} from "./editor/markdown-utils";
import {
  PREVIEW_HTML_SANITIZE_SCHEMA,
  PREVIEW_MARKDOWN_REHYPE_OPTIONS
} from "./editor/markdown-sanitize";
import { remarkReferenceFootnotePlugin } from "./editor/remark-reference-footnotes";
import { remarkStyledSpanContainerPlugin } from "./editor/remark-styled-span-container";
import {
  buildPreviewThemeStyleText,
  getPreviewThemeClassName,
  normalizePreviewStyleText
} from "./editor/preview-style";
import {
  formatError,
  formatSavedTime,
  resolveSaveIndicatorVariant
} from "./editor/status-utils";
import type { PreviewLinkRenderMode, PreviewViewportMode, SaveStatus } from "./editor/types";
import { useScrollSync } from "./editor/use-scroll-sync";
import { copyPreviewToWechat } from "./editor/wechat-export";
import {
  DEFAULT_PREVIEW_THEME_TEMPLATE,
  resolvePreviewTheme,
  toPreviewThemeTemplate
} from "./preview-themes";
import { toast } from "sonner";
import {
  DEFAULT_IMAGE_HOSTING_CONFIG,
  normalizeImageHostingConfig,
  type ImageHostingConfig
} from "./settings/image-hosting";
import { uploadImageToDefaultHosting } from "./settings/image-hosting-upload";
import { useWorkspace } from "./workspace/use-workspace";

// 扩展 window 类型，支持外部注入预览样式字符串。
declare global {
  interface Window {
    __PLAINDOC_PREVIEW_STYLE__?: string;
  }
}

const WORKSPACE_SIDEBAR_MIN_WIDTH = 240;
const WORKSPACE_SIDEBAR_MAX_WIDTH = 560;
const WORKSPACE_SIDEBAR_DEFAULT_WIDTH = 320;
const WORKSPACE_SIDEBAR_WIDTH_STORAGE_KEY = "workspace.sidebar.width";
const WORKSPACE_SIDEBAR_COLLAPSED_STORAGE_KEY = "workspace.sidebar.collapsed";
const WORKSPACE_ACTIVE_SPACE_ID_STORAGE_KEY = "workspace.activeSpaceId";
const WORKSPACE_ACTIVE_DOC_ID_STORAGE_KEY = "workspace.activeDocId";
const PREVIEW_LINK_RENDER_MODE_STORAGE_KEY = "plaindoc.preview.link-render-mode";
const LOGIN_ROUTE_PATH = "/login";
const REGISTER_ROUTE_PATH = "/register";
const EDITOR_ROUTE_BASE_PATH = "/editor";

export type AppRoute =
  | { kind: "login" }
  | { kind: "register" }
  | { kind: "editor-root" }
  | { kind: "editor-space"; spaceId: string }
  | { kind: "editor-doc"; spaceId: string; docId: string }
  | { kind: "admin-login" }
  | { kind: "admin-root" }
  | { kind: "admin-page"; pagePath: string }
  | { kind: "unknown" };

interface EditorAccessErrorState {
  spaceId: string | null;
  description: string;
  technicalMessage?: string | null;
}

function normalizeRoutePath(pathname: string): string {
  const normalized = pathname.replace(/\/+$/, "");
  return normalized || "/";
}

function decodeRoutePart(value: string): string | null {
  try {
    return decodeURIComponent(value);
  } catch {
    return null;
  }
}

function parseAppRoute(pathname: string): AppRoute {
  const normalizedPathname = normalizeRoutePath(pathname);
  if (normalizedPathname === LOGIN_ROUTE_PATH) {
    return { kind: "login" };
  }
  if (normalizedPathname === REGISTER_ROUTE_PATH) {
    return { kind: "register" };
  }
  if (normalizedPathname === ADMIN_LOGIN_ROUTE_PATH) {
    return { kind: "admin-login" };
  }
  if (normalizedPathname === ADMIN_ROUTE_BASE_PATH) {
    return { kind: "admin-root" };
  }
  if (normalizedPathname.startsWith(`${ADMIN_ROUTE_BASE_PATH}/`)) {
    return { kind: "admin-page", pagePath: normalizedPathname.slice(ADMIN_ROUTE_BASE_PATH.length + 1) };
  }
  if (normalizedPathname === EDITOR_ROUTE_BASE_PATH) {
    return { kind: "editor-root" };
  }
  if (!normalizedPathname.startsWith(`${EDITOR_ROUTE_BASE_PATH}/`)) {
    return { kind: "unknown" };
  }

  const routeParts = normalizedPathname.split("/").filter(Boolean);
  if (routeParts.length === 2) {
    const spaceId = decodeRoutePart(routeParts[1]);
    return spaceId ? { kind: "editor-space", spaceId } : { kind: "unknown" };
  }
  if (routeParts.length === 3) {
    const spaceId = decodeRoutePart(routeParts[1]);
    const docId = decodeRoutePart(routeParts[2]);
    if (!spaceId || !docId) {
      return { kind: "unknown" };
    }
    return { kind: "editor-doc", spaceId, docId };
  }
  return { kind: "unknown" };
}

function buildEditorRoutePath(spaceId: string | null, docId: string | null): string {
  if (!spaceId) {
    return EDITOR_ROUTE_BASE_PATH;
  }
  const encodedSpaceID = encodeURIComponent(spaceId);
  if (!docId) {
    return `${EDITOR_ROUTE_BASE_PATH}/${encodedSpaceID}`;
  }
  return `${EDITOR_ROUTE_BASE_PATH}/${encodedSpaceID}/${encodeURIComponent(docId)}`;
}

function resolveAuthRedirectTarget(rawValue: string | null): string | null {
  const trimmedValue = rawValue?.trim() ?? "";
  if (!trimmedValue) {
    return null;
  }
  if (trimmedValue.startsWith("/")) {
    return trimmedValue;
  }

  try {
    const parsedURL = new URL(trimmedValue);
    if (parsedURL.protocol !== "http:" && parsedURL.protocol !== "https:") {
      return null;
    }
    return parsedURL.toString();
  } catch {
    return null;
  }
}

// 根据后端错误文本生成可展示给用户的空间访问失败说明。
function resolveEditorAccessDescription(rawMessage: string): string {
  const normalizedMessage = rawMessage.trim();
  if (!normalizedMessage) {
    return "当前无法验证空间访问权限，请稍后重试。";
  }
  if (
    normalizedMessage.includes("不存在") ||
    normalizedMessage.includes("无访问权限") ||
    normalizedMessage.includes("无权访问") ||
    normalizedMessage.includes("forbidden") ||
    normalizedMessage.includes("not found")
  ) {
    return "该空间不存在，或你暂无访问权限。请联系空间管理员确认权限。";
  }
  return "空间访问校验失败，请稍后重试。";
}

// 统一钳制侧栏宽度，避免本地脏值导致布局异常。
function clampWorkspaceSidebarWidth(width: number): number {
  return Math.min(WORKSPACE_SIDEBAR_MAX_WIDTH, Math.max(WORKSPACE_SIDEBAR_MIN_WIDTH, width));
}

// 读取侧栏宽度缓存：解析失败时回退默认宽度。
function readStoredWorkspaceSidebarWidth(): number {
  try {
    const rawWidth = globalThis.localStorage.getItem(WORKSPACE_SIDEBAR_WIDTH_STORAGE_KEY);
    if (!rawWidth) {
      return WORKSPACE_SIDEBAR_DEFAULT_WIDTH;
    }
    const parsedWidth = Number(rawWidth);
    if (!Number.isFinite(parsedWidth)) {
      return WORKSPACE_SIDEBAR_DEFAULT_WIDTH;
    }
    return clampWorkspaceSidebarWidth(parsedWidth);
  } catch {
    return WORKSPACE_SIDEBAR_DEFAULT_WIDTH;
  }
}

// 读取侧栏折叠态缓存：仅 "1" 代表折叠。
function readStoredWorkspaceSidebarCollapsed(): boolean {
  try {
    return globalThis.localStorage.getItem(WORKSPACE_SIDEBAR_COLLAPSED_STORAGE_KEY) === "1";
  } catch {
    return false;
  }
}

// 读取上次激活空间：用于刷新后恢复目录上下文。
function readStoredWorkspaceActiveSpaceId(): string | null {
  try {
    const value = window.localStorage.getItem(WORKSPACE_ACTIVE_SPACE_ID_STORAGE_KEY);
    return value && value.trim() ? value : null;
  } catch {
    return null;
  }
}

// 读取上次激活文档：用于刷新后恢复选中态和编辑内容。
function readStoredWorkspaceActiveDocId(): string | null {
  try {
    const value = window.localStorage.getItem(WORKSPACE_ACTIVE_DOC_ID_STORAGE_KEY);
    return value && value.trim() ? value : null;
  } catch {
    return null;
  }
}

// 读取链接渲染模式：默认保持原始链接，避免影响已有阅读习惯。
function readStoredPreviewLinkRenderMode(): PreviewLinkRenderMode {
  try {
    const value = window.localStorage.getItem(PREVIEW_LINK_RENDER_MODE_STORAGE_KEY);
    return value === "footnote" ? "footnote" : "link";
  } catch {
    return "link";
  }
}

// 从剪贴板事件中提取图片文件列表：优先 items，兜底 files。
function extractImageFilesFromClipboard(event: ClipboardEvent): File[] {
  const clipboardData = event.clipboardData;
  if (!clipboardData) {
    return [];
  }

  const imageFilesFromItems = Array.from(clipboardData.items)
    .filter((item) => item.kind === "file" && item.type.startsWith("image/"))
    .map((item) => item.getAsFile())
    .filter((file): file is File => file instanceof File);
  if (imageFilesFromItems.length) {
    return imageFilesFromItems;
  }

  return Array.from(clipboardData.files).filter((file) => file.type.startsWith("image/"));
}

// 生成 Markdown 图片文案：优先复用文件名，缺失时回退到 image-x。
function buildImageMarkdownLine(file: File, url: string, index: number): string {
  const rawName = file.name.replace(/\.[^.]+$/, "").trim();
  const altText = rawName || `image-${index + 1}`;
  return `![${altText}](${url})`;
}

// 在当前选区插入图片 Markdown，自动补齐前后换行避免粘连原文本。
function insertImageMarkdown(view: EditorView, markdownLines: string[]): void {
  const selectedRange = view.state.selection.main;
  const markdownBlock = markdownLines.join("\n");
  const docLength = view.state.doc.length;
  // 保证图片块前后都至少保留一个空行（即两侧至少有两个换行符）。
  const beforeContext = view.state.doc.sliceString(Math.max(0, selectedRange.from - 2), selectedRange.from);
  const afterContext = view.state.doc.sliceString(selectedRange.to, Math.min(docLength, selectedRange.to + 2));

  const prefix =
    selectedRange.from === 0 ? "" : beforeContext.endsWith("\n\n") ? "" : beforeContext.endsWith("\n") ? "\n" : "\n\n";
  const suffix =
    selectedRange.to === docLength
      ? ""
      : afterContext.startsWith("\n\n")
        ? ""
        : afterContext.startsWith("\n")
          ? "\n"
          : "\n\n";

  const insertText = `${prefix}${markdownBlock}${suffix}`;

  const cursor = selectedRange.from + insertText.length;
  view.dispatch({
    changes: {
      from: selectedRange.from,
      to: selectedRange.to,
      insert: insertText
    },
    selection: EditorSelection.cursor(cursor),
    scrollIntoView: true
  });
}

export default function App() {
  // 数据网关单例。
  const dataGateway = useMemo(() => getDataGateway(), []);
  const location = useLocation();
  const navigate = useNavigate();
  const route = useMemo(() => parseAppRoute(location.pathname), [location.pathname]);
  const isEditorRoute =
    route.kind === "editor-root" || route.kind === "editor-space" || route.kind === "editor-doc";
  const isAdminRoute =
    route.kind === "admin-login" || route.kind === "admin-root" || route.kind === "admin-page";
  const routeSpaceId = route.kind === "editor-space" || route.kind === "editor-doc" ? route.spaceId : null;
  const routeDocId = route.kind === "editor-doc" ? route.docId : null;
  const authRedirectTarget = useMemo(() => {
    if (route.kind !== "login" && route.kind !== "register") {
      return null;
    }
    return resolveAuthRedirectTarget(new URLSearchParams(location.search).get("redirect"));
  }, [location.search, route.kind]);
  // 会话状态：登录态用户、校验中状态与提交中状态。
  const [authSession, setAuthSession] = useState<AuthSession>({ user: null });
  const [isAuthChecking, setIsAuthChecking] = useState(true);
  const [isAuthSubmitting, setIsAuthSubmitting] = useState(false);
  const [authErrorMessage, setAuthErrorMessage] = useState<string | null>(null);
  const activeUser = authSession.user;
  // 工作区状态层：统一管理空间/目录树/文档加载，减少 App 根组件职责。
  const {
    activeSpaceId,
    activeSpaceName,
    workspaceTree,
    activeDocumentTitle,
    activeDocumentThemeId,
    activeDocId,
    content,
    baseVersion,
    lastSavedContent,
    lastSavedAt,
    bootstrapWorkspace,
    createNode,
    renameNode,
    deleteNode,
    openDocument,
    setContent,
    setBaseVersion,
    setActiveDocumentTitle,
    setActiveDocumentThemeId,
    setLastSavedContent,
    setLastSavedAt
  } = useWorkspace({
    dataGateway,
    initialContent: FALLBACK_CONTENT
  });
  // 保存状态。
  const [saveStatus, setSaveStatus] = useState<SaveStatus>("loading");
  // 页头状态文案。
  const [statusMessage, setStatusMessage] = useState("初始化中...");
  // 当前可用主题列表：统一从数据层读取（local/http 均走 DataGateway）。
  const [previewThemes, setPreviewThemes] = useState([DEFAULT_PREVIEW_THEME_TEMPLATE]);
  // 当前生效的预览主题 ID。
  const [activePreviewThemeId, setActivePreviewThemeId] = useState(DEFAULT_PREVIEW_THEME_ID);
  // 外部注入的预览样式文本；为空时仅使用内置主题。
  const [customPreviewStyleText, setCustomPreviewStyleText] = useState("");
  // 预览视口模式：desktop 保持现状，mobile 模拟窄屏阅读。
  const [previewViewportMode, setPreviewViewportMode] = useState<PreviewViewportMode>("desktop");
  // 链接渲染模式：支持“原始链接”与“脚注+角标”双模式切换。
  const [previewLinkRenderMode, setPreviewLinkRenderMode] = useState<PreviewLinkRenderMode>(
    readStoredPreviewLinkRenderMode
  );
  // 复制到公众号时的进行中状态：防止重复点击触发并发复制。
  const [isWechatCopying, setIsWechatCopying] = useState(false);
  // 粘贴图片上传状态：用于防止重复触发并展示状态文案。
  const [isImageUploading, setIsImageUploading] = useState(false);
  // 当前上传任务总数与已处理数量：用于展示实时上传进度。
  const [imageUploadTotalCount, setImageUploadTotalCount] = useState(0);
  const [imageUploadCompletedCount, setImageUploadCompletedCount] = useState(0);
  const { confirm: confirmByModal, dialog: confirmDialog } = useConfirmDialog();
  // 图床配置读取状态。
  const [isImageHostingConfigLoading, setIsImageHostingConfigLoading] = useState(true);
  // 图床配置错误文案。
  const [imageHostingConfigError, setImageHostingConfigError] = useState<string | null>(null);
  // 图床配置数据。
  const [imageHostingConfig, setImageHostingConfig] = useState<ImageHostingConfig>(
    DEFAULT_IMAGE_HOSTING_CONFIG
  );
  // 图床配置引用：供异步粘贴上传逻辑读取最新值，避免闭包拿到旧配置。
  const imageHostingConfigRef = useRef(imageHostingConfig);
  const isImageHostingConfigLoadingRef = useRef(isImageHostingConfigLoading);
  const imageHostingConfigErrorRef = useRef(imageHostingConfigError);
  // 上传中引用：用于 paste 事件同步分支判断，避免并发上传。
  const isImageUploadingRef = useRef(isImageUploading);
  // 工作区宽度与折叠状态：支持侧栏拖拽调宽与隐藏。
  const [workspaceSidebarWidth, setWorkspaceSidebarWidth] = useState(readStoredWorkspaceSidebarWidth);
  const [isWorkspaceSidebarCollapsed, setIsWorkspaceSidebarCollapsed] = useState(
    readStoredWorkspaceSidebarCollapsed
  );
  // 编辑器空间访问失败态：用于展示“空间不存在/无权限”简约错误页。
  const [editorAccessError, setEditorAccessError] = useState<EditorAccessErrorState | null>(null);
  // 手动重试计数：点击“重新校验”时递增，触发路由同步 effect 重新执行。
  const [editorAccessRetryCount, setEditorAccessRetryCount] = useState(0);
  const workspaceSidebarResizeStateRef = useRef<{ startX: number; startWidth: number } | null>(null);
  // 记录“由本地动作触发的目标文档路由”，避免路由 effect 在状态尚未同步时重复请求。
  const pendingRouteDocumentIDRef = useRef<{ docId: string; startedAt: number } | null>(null);

  // 当前生效主题对象，用于渲染菜单高亮和生成样式。
  const activePreviewTheme = useMemo(
    () => resolvePreviewTheme(activePreviewThemeId, previewThemes),
    [activePreviewThemeId, previewThemes]
  );
  // 预览区主题类名：挂到正文 article 上参与主题变量匹配。
  const activePreviewThemeClassName = useMemo(
    () => getPreviewThemeClassName(activePreviewTheme.id),
    [activePreviewTheme.id]
  );

  // 滚动同步 Hook：封装编辑区/预览区双向同步与锚点重建逻辑。
  const { handleEditorPaneRef, handlePreviewScrollerRef, handleEditorCreate, handleTocNavigate } =
    useScrollSync({
      content,
      previewThemeClassName: activePreviewThemeClassName,
      customPreviewStyleText,
      previewViewportMode
    });

  const routeDocExistsInTree = useMemo(() => {
    if (!routeDocId) {
      return false;
    }
    const stack = [...workspaceTree];
    while (stack.length) {
      const node = stack.pop();
      if (!node) {
        continue;
      }
      if (node.id === routeDocId && node.type === "doc") {
        return true;
      }
      if (node.children.length) {
        stack.push(...node.children);
      }
    }
    return false;
  }, [routeDocId, workspaceTree]);

  // 启动时先校验会话，避免未登录就触发工作区加载请求。
  useEffect(() => {
    let cancelled = false;

    const checkSession = async () => {
      setIsAuthChecking(true);
      setAuthErrorMessage(null);
      try {
        const session = await dataGateway.auth.getSession();
        if (cancelled) {
          return;
        }
        setAuthSession(session);
      } catch (error) {
        if (cancelled) {
          return;
        }
        console.error("[auth] 会话检查失败", error);
        setAuthSession({ user: null });
        setAuthErrorMessage(`会话检查失败：${formatError(error)}`);
      } finally {
        if (!cancelled) {
          setIsAuthChecking(false);
        }
      }
    };

    void checkSession();
    return () => {
      cancelled = true;
    };
  }, [dataGateway]);

  // 登录态与路由守卫：未登录固定到 /login，已登录固定到 /editor 系列路由。
  useEffect(() => {
    if (isAuthChecking) {
      return;
    }

    if (!activeUser) {
      if (isAdminRoute) {
        if (route.kind !== "admin-login") {
          navigate(ADMIN_LOGIN_ROUTE_PATH, { replace: true });
        }
        return;
      }
      if (route.kind !== "login" && route.kind !== "register") {
        navigate(LOGIN_ROUTE_PATH, { replace: true });
      }
      return;
    }

    if ((route.kind === "login" || route.kind === "register") && authRedirectTarget) {
      window.location.replace(authRedirectTarget);
      return;
    }

    // 编辑器必须绑定具体空间，禁止直接打开 /editor 根路径。
    if (route.kind === "editor-root") {
      window.location.replace("/");
      return;
    }

    if (!isEditorRoute && !isAdminRoute) {
      if (location.pathname !== "/") {
        window.location.replace("/");
      }
    }
  }, [
    activeUser,
    authRedirectTarget,
    isAuthChecking,
    isAdminRoute,
    isEditorRoute,
    location.pathname,
    route.kind
  ]);

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

  // 首次加载主题模板：统一从数据层读取，保证 local/http 一致。
  useEffect(() => {
    let cancelled = false;

    const loadThemes = async () => {
      try {
        const themes = await dataGateway.theme.listThemes();
        if (cancelled) {
          return;
        }
        if (!themes.length) {
          setPreviewThemes([DEFAULT_PREVIEW_THEME_TEMPLATE]);
          return;
        }
        setPreviewThemes(themes.map(toPreviewThemeTemplate));
      } catch (error) {
        if (cancelled) {
          return;
        }
        console.error("[theme] 加载主题列表失败", error);
        setPreviewThemes([DEFAULT_PREVIEW_THEME_TEMPLATE]);
      }
    };

    void loadThemes();
    return () => {
      cancelled = true;
    };
  }, [dataGateway]);

  // 当前文档变化时，自动切换到文档绑定的主题。
  useEffect(() => {
    const targetTheme = resolvePreviewTheme(activeDocumentThemeId, previewThemes);
    setActivePreviewThemeId((previousThemeID) =>
      previousThemeID === targetTheme.id ? previousThemeID : targetTheme.id
    );
  }, [activeDocumentThemeId, previewThemes]);

  // 首次加载时恢复上次选择的预览视口模式（PC / 移动端）。
  useEffect(() => {
    try {
      const storedPreviewViewportMode = window.localStorage.getItem(PREVIEW_VIEWPORT_MODE_STORAGE_KEY);
      if (storedPreviewViewportMode === "desktop" || storedPreviewViewportMode === "mobile") {
        setPreviewViewportMode(storedPreviewViewportMode);
      }
    } catch {
      // localStorage 不可用时保持默认 PC 预览模式。
    }
  }, []);

  // 预览模式变化时写入本地缓存，便于下次启动直接恢复。
  useEffect(() => {
    try {
      window.localStorage.setItem(PREVIEW_VIEWPORT_MODE_STORAGE_KEY, previewViewportMode);
    } catch {
      // localStorage 失败时忽略持久化，不影响当前显示。
    }
  }, [previewViewportMode]);

  // 链接渲染模式变化时写入本地缓存，便于下次启动直接恢复。
  useEffect(() => {
    try {
      window.localStorage.setItem(PREVIEW_LINK_RENDER_MODE_STORAGE_KEY, previewLinkRenderMode);
    } catch {
      // localStorage 失败时忽略持久化，不影响当前显示。
    }
  }, [previewLinkRenderMode]);

  // 同步配置引用，确保粘贴上传始终使用最新“默认图床 + 凭据”。
  useEffect(() => {
    imageHostingConfigRef.current = imageHostingConfig;
  }, [imageHostingConfig]);

  // 同步上传状态引用，避免在 paste 事件中读取到过期状态。
  useEffect(() => {
    isImageUploadingRef.current = isImageUploading;
  }, [isImageUploading]);

  // 同步图床配置加载状态引用，避免 paste 事件读取过期值。
  useEffect(() => {
    isImageHostingConfigLoadingRef.current = isImageHostingConfigLoading;
  }, [isImageHostingConfigLoading]);

  // 同步图床配置错误引用，便于上传时给出即时提示。
  useEffect(() => {
    imageHostingConfigErrorRef.current = imageHostingConfigError;
  }, [imageHostingConfigError]);

  // 登录后加载后台图床配置：由管理后台统一维护。
  useEffect(() => {
    if (!activeUser) {
      setImageHostingConfig(DEFAULT_IMAGE_HOSTING_CONFIG);
      setImageHostingConfigError(null);
      setIsImageHostingConfigLoading(false);
      return;
    }

    let cancelled = false;

    const loadImageHostingConfig = async () => {
      setIsImageHostingConfigLoading(true);
      setImageHostingConfigError(null);
      try {
        const systemConfig = await dataGateway.imageHosting.getConfig();
        if (cancelled) {
          return;
        }
        setImageHostingConfig(normalizeImageHostingConfig(systemConfig));
      } catch (error) {
        if (cancelled) {
          return;
        }
        console.error("[settings][image-hosting] 读取后台图床配置失败", error);
        setImageHostingConfig(DEFAULT_IMAGE_HOSTING_CONFIG);
        setImageHostingConfigError(`读取后台图床配置失败：${formatError(error)}`);
      } finally {
        if (!cancelled) {
          setIsImageHostingConfigLoading(false);
        }
      }
    };

    void loadImageHostingConfig();

    return () => {
      cancelled = true;
    };
  }, [activeUser, dataGateway]);

  const extensions = useMemo(
    () => [
      // 编辑器软换行，避免横向滚动影响同步体验。
      EditorView.lineWrapping,
      // 拦截粘贴图片：自动上传到默认图床并回填 Markdown 图片链接。
      EditorView.domEventHandlers({
        paste: (event, view) => {
          const imageFiles = extractImageFilesFromClipboard(event);
          if (!imageFiles.length) {
            return false;
          }

          event.preventDefault();
          void (async () => {
            if (isImageHostingConfigLoadingRef.current) {
              setStatusMessage("图床配置加载中，请稍后再试");
              return;
            }
            if (imageHostingConfigErrorRef.current) {
              setStatusMessage(imageHostingConfigErrorRef.current);
              toast.error(imageHostingConfigErrorRef.current);
              return;
            }
            if (isImageUploadingRef.current) {
              setStatusMessage("图片上传中，请稍候...");
              return;
            }

            isImageUploadingRef.current = true;
            setIsImageUploading(true);
            setImageUploadTotalCount(imageFiles.length);
            setImageUploadCompletedCount(0);
            setStatusMessage(`正在上传 ${imageFiles.length} 张图片...`);
            const successMarkdownLines: string[] = [];
            const failedMessages: string[] = [];

            try {
              for (const [index, imageFile] of imageFiles.entries()) {
                try {
                  const uploadedImage = await uploadImageToDefaultHosting(
                    imageHostingConfigRef.current,
                    imageFile,
                    {
                      uploadLocalImage: (file) =>
                        dataGateway.imageHosting.uploadLocalImage(
                          file,
                          imageHostingConfigRef.current.local.uploadEndpoint
                        )
                    }
                  );
                  successMarkdownLines.push(buildImageMarkdownLine(imageFile, uploadedImage.url, index));
                } catch (error) {
                  console.error("[editor][paste-image] 单张图片上传失败", {
                    fileName: imageFile.name || "未命名图片",
                    provider: imageHostingConfigRef.current.defaultProvider,
                    error
                  });
                  failedMessages.push(`${imageFile.name || "未命名图片"}：${formatError(error)}`);
                } finally {
                  setImageUploadCompletedCount((previousCount) => previousCount + 1);
                }
              }

              if (successMarkdownLines.length) {
                insertImageMarkdown(view, successMarkdownLines);
                setStatusMessage(`已上传 ${successMarkdownLines.length} 张图片并插入链接`);
                toast.success(`图片上传成功（${successMarkdownLines.length}/${imageFiles.length}）`);
              }

              if (failedMessages.length) {
                const firstError = failedMessages[0];
                console.error("[editor][paste-image] 部分图片上传失败", {
                  failedCount: failedMessages.length,
                  errors: failedMessages
                });
                setStatusMessage(`图片上传失败：${firstError}`);
                toast.error(`部分图片上传失败：${firstError}`);
              }
            } catch (error) {
              console.error("[editor][paste-image] 粘贴图片上传流程异常", error);
              setStatusMessage(`图片上传异常：${formatError(error)}`);
              toast.error(`图片上传异常：${formatError(error)}`);
            } finally {
              isImageUploadingRef.current = false;
              setIsImageUploading(false);
              setImageUploadTotalCount(0);
              setImageUploadCompletedCount(0);
            }
          })();
          return true;
        }
      }),
      markdown({
        // 启用 Markdown 语言与代码块语言支持。
        base: markdownLanguage,
        codeLanguages: languages
      })
    ],
    [dataGateway]
  );
  // remark 插件顺序：先 GFM/数学公式，再规整样式 span 容器，再按链接模式处理脚注，最后注入锚点属性。
  const remarkPlugins = useMemo(() => {
    const referenceFootnotePlugin: [typeof remarkReferenceFootnotePlugin, { mode: PreviewLinkRenderMode }] = [
      remarkReferenceFootnotePlugin,
      { mode: previewLinkRenderMode }
    ];
    return [remarkGfm, remarkMath, remarkStyledSpanContainerPlugin, referenceFootnotePlugin, remarkBlockAnchorPlugin];
  }, [previewLinkRenderMode]);
  // rehype 插件顺序：先解析内嵌 HTML，再做白名单清洗，最后渲染 KaTeX。
  const rehypePlugins = useMemo(() => {
    const sanitizePlugin: [typeof rehypeSanitize, typeof PREVIEW_HTML_SANITIZE_SCHEMA] = [
      rehypeSanitize,
      PREVIEW_HTML_SANITIZE_SCHEMA
    ];
    return [rehypeRaw, sanitizePlugin, rehypeKatex];
  }, []);
  // markdown-it 仅用于“去语法后的文字统计”和 TOC 语法解析。
  const markdownTextParser = useMemo(
    () =>
      new MarkdownIt({
        html: false,
        linkify: true,
        typographer: false
      }),
    []
  );
  // 解析文档标题与 [TOC] 标记，供目录菜单与语法渲染共用。
  const tocParseResult = useMemo(
    () => parseTocFromMarkdown(content, markdownTextParser),
    [content, markdownTextParser]
  );
  // TOC 标题列表。
  const tocItems = tocParseResult.items;
  // 当前文档是否声明了 [TOC] 语法标记。
  const hasTocMarker = tocParseResult.hasMarker;

  // 自定义 Markdown 渲染器。
  const markdownComponents = useMemo(
    () =>
      buildMarkdownComponents({
        activePreviewTheme,
        tocItems,
        onTocNavigate: handleTocNavigate
      }),
    [activePreviewTheme, handleTocNavigate, tocItems]
  );
  // 提取 Markdown 对应的纯文本内容。
  const plainTextContent = useMemo(
    () => extractPlainTextFromMarkdown(content, markdownTextParser),
    [content, markdownTextParser]
  );
  // 统计非空白字符数量，作为字数展示。
  const plainTextCount = useMemo(() => plainTextContent.replace(/\s+/g, "").length, [plainTextContent]);
  // 将最后保存时间格式化为状态栏文案。
  const lastSavedTimeLabel = useMemo(() => formatSavedTime(lastSavedAt), [lastSavedAt]);
  // 根据保存状态生成状态栏图标展示类型。
  const saveIndicatorVariant = useMemo(() => resolveSaveIndicatorVariant(saveStatus), [saveStatus]);
  // 当前主题对应的变量样式文本：通过 style 标签注入。
  const activePreviewThemeStyleText = useMemo(
    () => buildPreviewThemeStyleText(activePreviewTheme),
    [activePreviewTheme]
  );
  // 主题自定义 CSS：由主题数据直接下发，作为统一主题数据源的一部分。
  const activePreviewThemeCustomStyleText = useMemo(
    () => normalizePreviewStyleText(activePreviewTheme.customCss ?? ""),
    [activePreviewTheme.customCss]
  );
  // 图片上传中的顶部提示文案：展示 x/y 进度以降低等待焦虑。
  const imageUploadLoadingMessage = useMemo(() => {
    if (!isImageUploading) {
      return "";
    }
    if (imageUploadTotalCount <= 0) {
      return "图片上传中...";
    }
    return `图片上传中（${Math.min(imageUploadCompletedCount, imageUploadTotalCount)}/${imageUploadTotalCount}）...`;
  }, [imageUploadCompletedCount, imageUploadTotalCount, isImageUploading]);

  // 粘贴图片上传期间展示 sonner loading，结束后自动关闭。
  useEffect(() => {
    const toastID = "plaindoc-image-upload-progress";
    if (isImageUploading) {
      toast.loading(imageUploadLoadingMessage || "图片上传中...", {
        id: toastID,
        duration: Infinity
      });
      return;
    }
    toast.dismiss(toastID);
  }, [imageUploadLoadingMessage, isImageUploading]);

  // 持久化当前激活空间：刷新后尽量恢复到上次知识本上下文。
  useEffect(() => {
    try {
      if (activeSpaceId) {
        window.localStorage.setItem(WORKSPACE_ACTIVE_SPACE_ID_STORAGE_KEY, activeSpaceId);
      } else {
        window.localStorage.removeItem(WORKSPACE_ACTIVE_SPACE_ID_STORAGE_KEY);
      }
    } catch {
      // localStorage 不可用时仅忽略持久化，不影响当前会话。
    }
  }, [activeSpaceId]);

  // 持久化当前激活文档：刷新后恢复树节点高亮和文档加载。
  useEffect(() => {
    try {
      if (activeDocId) {
        window.localStorage.setItem(WORKSPACE_ACTIVE_DOC_ID_STORAGE_KEY, activeDocId);
      } else {
        window.localStorage.removeItem(WORKSPACE_ACTIVE_DOC_ID_STORAGE_KEY);
      }
    } catch {
      // localStorage 不可用时仅忽略持久化，不影响当前会话。
    }
  }, [activeDocId]);

  // 在离开当前文档前确认未保存修改，避免目录切换导致内容丢失。
  const confirmLeaveForDocumentSwitch = useCallback(async (): Promise<boolean> => {
    const hasUnsavedChanges =
      content !== lastSavedContent && saveStatus !== "loading" && saveStatus !== "saving";
    if (!hasUnsavedChanges) {
      return true;
    }
    return confirmByModal({
      title: "切换文档确认",
      description: "当前文档仍有未保存修改，切换后这些修改将丢失。",
      confirmText: "继续切换",
      tone: "warning"
    });
  }, [confirmByModal, content, lastSavedContent, saveStatus]);

  // 从侧边栏打开目标文档：保持保存状态与状态栏提示一致。
  const handleOpenWorkspaceDocument = useCallback(
    async (docId: string): Promise<void> => {
      if (docId === activeDocId) {
        return;
      }
      if (!(await confirmLeaveForDocumentSwitch())) {
        return;
      }
      setSaveStatus("loading");
      setStatusMessage("切换文档中...");
      try {
        const result = await openDocument(docId);
        setSaveStatus("ready");
        setStatusMessage(`已切换文档 v${result.version}`);
        if (activeSpaceId) {
          const targetPath = buildEditorRoutePath(activeSpaceId, result.id);
          if (location.pathname !== targetPath) {
            pendingRouteDocumentIDRef.current = {
              docId: result.id,
              startedAt: Date.now()
            };
            navigate(targetPath);
          }
        }
      } catch (error) {
        pendingRouteDocumentIDRef.current = null;
        setSaveStatus("error");
        setStatusMessage(`切换文档失败：${formatError(error)}`);
        throw error;
      }
    },
    [activeDocId, activeSpaceId, confirmLeaveForDocumentSwitch, location.pathname, navigate, openDocument]
  );

  // 路由同步 effect 使用 ref 读取最新打开文档函数，避免因依赖抖动触发重复初始化请求。
  const handleOpenWorkspaceDocumentRef = useRef(handleOpenWorkspaceDocument);
  useEffect(() => {
    handleOpenWorkspaceDocumentRef.current = handleOpenWorkspaceDocument;
  }, [handleOpenWorkspaceDocument]);

  // 路由与工作区同步：支持 /editor、/editor/:spaceId、/editor/:spaceId/:docId 深链接。
  useEffect(() => {
    if (isAuthChecking || !activeUser || !isEditorRoute) {
      return;
    }

    let cancelled = false;
    setEditorAccessError(null);

    const syncWorkspaceByRoute = async () => {
      try {
        const pendingRouteDocument = pendingRouteDocumentIDRef.current;
        if (pendingRouteDocument) {
          const pendingElapsed = Date.now() - pendingRouteDocument.startedAt;
          const isPendingExpired = pendingElapsed > 1500;
          if (routeDocId !== pendingRouteDocument.docId) {
            if (!isPendingExpired) {
              return;
            }
            pendingRouteDocumentIDRef.current = null;
          } else if (activeDocId !== pendingRouteDocument.docId) {
            if (!isPendingExpired) {
              return;
            }
            pendingRouteDocumentIDRef.current = null;
          } else {
            pendingRouteDocumentIDRef.current = null;
          }
        }

        // 未初始化或目标空间变更时，按路由参数重建工作区上下文。
        if (!activeSpaceId || (routeSpaceId && routeSpaceId !== activeSpaceId)) {
          setSaveStatus("loading");
          setStatusMessage("加载文档中...");

          const bootstrapResult = await bootstrapWorkspace({
            preferredSpaceId: routeSpaceId ?? readStoredWorkspaceActiveSpaceId(),
            preferredDocId: routeSpaceId ? routeDocId : readStoredWorkspaceActiveDocId(),
            strictPreferredSpace: Boolean(routeSpaceId)
          });
          if (cancelled) {
            return;
          }

          setSaveStatus("ready");
          setStatusMessage(`已加载文档 v${bootstrapResult.documentVersion}`);
          const targetPath = buildEditorRoutePath(bootstrapResult.spaceId, bootstrapResult.docId);
          if (location.pathname !== targetPath) {
            navigate(targetPath, { replace: true });
          }
          return;
        }

        // 同空间下，URL 显式指定了新文档时，按 URL 切文档（支持浏览器前进/后退）。
        if (routeDocId && routeDocId !== activeDocId) {
          // 本地动作触发的路由切换：等待 activeDocId 与路由同步，避免二次请求。
          if (pendingRouteDocumentIDRef.current?.docId === routeDocId) {
            return;
          }
          // 若 URL 指向已失效文档（例如被删除），回退到当前激活文档路径。
          if (activeDocId && !routeDocExistsInTree) {
            const fallbackPath = buildEditorRoutePath(activeSpaceId, activeDocId);
            if (location.pathname !== fallbackPath) {
              navigate(fallbackPath, { replace: true });
            }
            return;
          }
          await handleOpenWorkspaceDocumentRef.current(routeDocId);
          return;
        }

        // /editor 或 /editor/:spaceId 路由统一规范化到带 docId 的可分享链接。
        if (activeSpaceId && activeDocId && (!routeSpaceId || !routeDocId)) {
          const targetPath = buildEditorRoutePath(activeSpaceId, activeDocId);
          if (location.pathname !== targetPath) {
            navigate(targetPath, { replace: true });
          }
        }
      } catch (error) {
        if (cancelled) {
          return;
        }
        const message = formatError(error);
        setSaveStatus("error");
        setStatusMessage(`加载失败：${message}`);
        if (routeSpaceId) {
          setEditorAccessError({
            spaceId: routeSpaceId,
            description: resolveEditorAccessDescription(message),
            technicalMessage: message
          });
        }
      }
    };

    void syncWorkspaceByRoute();
    return () => {
      cancelled = true;
    };
  }, [
    activeDocId,
    activeSpaceId,
    activeUser,
    bootstrapWorkspace,
    isAuthChecking,
    isEditorRoute,
    location.pathname,
    navigate,
    routeDocExistsInTree,
    routeDocId,
    routeSpaceId,
    editorAccessRetryCount
  ]);

  // 重新校验空间访问权限：触发一次路由同步 effect 即可。
  const retryEditorAccessCheck = useCallback(() => {
    setEditorAccessError(null);
    setEditorAccessRetryCount((value) => value + 1);
  }, []);

  // 返回首页：当空间不可访问时提供明确出口，避免停留在无效路由。
  const backToHomepage = useCallback(() => {
    window.location.assign("/");
  }, []);

  // 目录树菜单动作：创建节点后若为文档则自动打开，保持编辑流连续。
  const handleCreateWorkspaceNode = useCallback(
    async (input: {
      parentId: string | null;
      type: "folder" | "doc";
      title: string;
    }): Promise<CreateNodeResult> => {
      try {
        const created = await createNode(input);
        if (created.docId) {
          await handleOpenWorkspaceDocument(created.docId);
          return created;
        }
        setStatusMessage(input.type === "folder" ? "目录创建成功" : "文档创建成功");
        return created;
      } catch (error) {
        setStatusMessage(`创建失败：${formatError(error)}`);
        throw error;
      }
    },
    [createNode, handleOpenWorkspaceDocument]
  );

  // 节点重命名动作：失败时回写状态栏，便于用户定位问题。
  const handleRenameWorkspaceNode = useCallback(
    async (nodeId: string, title: string): Promise<void> => {
      try {
        await renameNode(nodeId, title);
        setStatusMessage("重命名成功");
      } catch (error) {
        setStatusMessage(`重命名失败：${formatError(error)}`);
        throw error;
      }
    },
    [renameNode]
  );

  // 节点删除动作：由工作区层负责文档兜底切换，页面只承接状态提示。
  const handleDeleteWorkspaceNode = useCallback(
    async (nodeId: string): Promise<void> => {
      try {
        await deleteNode(nodeId);
        setStatusMessage("删除成功");
      } catch (error) {
        setStatusMessage(`删除失败：${formatError(error)}`);
        throw error;
      }
    },
    [deleteNode]
  );

  // 侧栏拖拽移动：按起始宽度和鼠标偏移计算目标宽度。
  const handleWorkspaceSidebarResizeMove = useCallback((event: PointerEvent) => {
    const resizeState = workspaceSidebarResizeStateRef.current;
    if (!resizeState) {
      return;
    }
    const deltaX = event.clientX - resizeState.startX;
    const nextWidth = clampWorkspaceSidebarWidth(resizeState.startWidth + deltaX);
    setWorkspaceSidebarWidth(nextWidth);
  }, []);

  // 结束侧栏拖拽：统一移除全局监听，避免残留事件导致内存泄漏。
  const finishWorkspaceSidebarResize = useCallback(() => {
    workspaceSidebarResizeStateRef.current = null;
    window.removeEventListener("pointermove", handleWorkspaceSidebarResizeMove);
    window.removeEventListener("pointerup", finishWorkspaceSidebarResize);
    window.removeEventListener("pointercancel", finishWorkspaceSidebarResize);
  }, [handleWorkspaceSidebarResizeMove]);

  // 开始侧栏拖拽：仅在“未折叠”状态下启用拖拽调宽。
  const handleWorkspaceSidebarResizeStart = useCallback(
    (event: ReactPointerEvent<HTMLButtonElement>) => {
      if (isWorkspaceSidebarCollapsed) {
        return;
      }
      event.preventDefault();
      workspaceSidebarResizeStateRef.current = {
        startX: event.clientX,
        startWidth: workspaceSidebarWidth
      };
      window.addEventListener("pointermove", handleWorkspaceSidebarResizeMove);
      window.addEventListener("pointerup", finishWorkspaceSidebarResize);
      window.addEventListener("pointercancel", finishWorkspaceSidebarResize);
    },
    [
      finishWorkspaceSidebarResize,
      handleWorkspaceSidebarResizeMove,
      isWorkspaceSidebarCollapsed,
      workspaceSidebarWidth
    ]
  );

  // 工作区显隐切换：折叠后保留已调节宽度，展开时恢复该宽度。
  const toggleWorkspaceSidebar = useCallback(() => {
    setIsWorkspaceSidebarCollapsed((collapsed) => !collapsed);
  }, []);

  // 页面卸载时兜底清理拖拽监听，确保不会残留全局事件。
  useEffect(() => {
    return () => {
      finishWorkspaceSidebarResize();
    };
  }, [finishWorkspaceSidebarResize]);

  // 持久化侧栏宽度：刷新后保持上次调节结果。
  useEffect(() => {
    try {
      window.localStorage.setItem(
        WORKSPACE_SIDEBAR_WIDTH_STORAGE_KEY,
        String(clampWorkspaceSidebarWidth(workspaceSidebarWidth))
      );
    } catch {
      // localStorage 不可用时仅忽略持久化，不影响当前会话。
    }
  }, [workspaceSidebarWidth]);

  // 持久化侧栏折叠态：刷新后保留用户显隐偏好。
  useEffect(() => {
    try {
      window.localStorage.setItem(
        WORKSPACE_SIDEBAR_COLLAPSED_STORAGE_KEY,
        isWorkspaceSidebarCollapsed ? "1" : "0"
      );
    } catch {
      // localStorage 不可用时仅忽略持久化，不影响当前会话。
    }
  }, [isWorkspaceSidebarCollapsed]);

  const workspaceLayoutStyle = useMemo<CSSProperties>(
    () =>
      ({
        "--workspace-sidebar-width": `${isWorkspaceSidebarCollapsed ? 0 : workspaceSidebarWidth}px`
      }) as CSSProperties,
    [isWorkspaceSidebarCollapsed, workspaceSidebarWidth]
  );

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
        setActiveDocumentThemeId(result.document.themeId || DEFAULT_PREVIEW_THEME_ID);
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

  // 应用选中的主题：更新文档主题绑定并同步预览区渲染。
  const handleThemeChange = useCallback(
    (themeId: string) => {
      if (!activeDocId) {
        setStatusMessage("当前未打开文档，无法切换主题");
        return;
      }
      const targetTheme = resolvePreviewTheme(themeId, previewThemes);
      if (targetTheme.id === activeDocumentThemeId) {
        return;
      }

      void (async () => {
        try {
          const updatedDocument = await dataGateway.document.setDocumentTheme(activeDocId, targetTheme.id);
          setActiveDocumentThemeId(updatedDocument.themeId);
          setActivePreviewThemeId(updatedDocument.themeId);
          setLastSavedAt(updatedDocument.updatedAt);
          setStatusMessage(`主题已切换：${targetTheme.name}`);
        } catch (error) {
          setStatusMessage(`切换主题失败：${formatError(error)}`);
        }
      })();
    },
    [activeDocId, activeDocumentThemeId, dataGateway, previewThemes, setActiveDocumentThemeId, setLastSavedAt]
  );

  // 切换预览视口：desktop <-> mobile。
  const togglePreviewViewportMode = useCallback(() => {
    setPreviewViewportMode((previousMode) =>
      previousMode === "desktop" ? "mobile" : "desktop"
    );
  }, []);

  // 切换链接渲染模式：原始链接 <-> 脚注角标。
  const togglePreviewLinkRenderMode = useCallback(() => {
    setPreviewLinkRenderMode((previousMode) =>
      previousMode === "link" ? "footnote" : "link"
    );
  }, []);

  // 导出预览区为内联样式 HTML，并写入剪贴板供公众号编辑器粘贴。
  const handleCopyToWechat = useCallback(async () => {
    if (isWechatCopying) {
      return;
    }
    setIsWechatCopying(true);
    try {
      await copyPreviewToWechat({
        linkRenderMode: previewLinkRenderMode
      });
      setStatusMessage("已复制预览内容，可直接粘贴到微信公众号编辑器");
      toast.success("复制成功，可直接粘贴到微信公众号编辑器");
    } catch (error) {
      setStatusMessage(`复制失败：${formatError(error)}`);
    } finally {
      setIsWechatCopying(false);
    }
  }, [isWechatCopying, previewLinkRenderMode]);

  // 手动同步到最新版本，用于冲突后的回拉。
  const syncLatestVersion = async () => {
    if (!activeDocId) {
      return;
    }
    try {
      const latestDocument = await openDocument(activeDocId);
      setSaveStatus("ready");
      setStatusMessage(`已同步到最新版本 v${latestDocument.version}`);
    } catch (error) {
      setSaveStatus("error");
      setStatusMessage(`同步失败：${formatError(error)}`);
    }
  };

  // 登录动作：认证成功后切换到工作区，并触发工作区启动流程。
  const handleAuthLogin = useCallback(
    async (input: { email: string; password: string }) => {
      setIsAuthSubmitting(true);
      setAuthErrorMessage(null);
      try {
        const session = await dataGateway.auth.login(input);
        if (!session.user) {
          setAuthErrorMessage("登录失败：服务端未返回用户信息");
          return;
        }
        setAuthSession(session);
        setSaveStatus("loading");
        setStatusMessage(`欢迎回来，${session.user.name}`);
        if (authRedirectTarget) {
          window.location.assign(authRedirectTarget);
        }
      } catch (error) {
        setAuthErrorMessage(`登录失败：${formatError(error)}`);
      } finally {
        setIsAuthSubmitting(false);
      }
    },
    [authRedirectTarget, dataGateway]
  );

  // 注册动作：注册成功后直接进入登录态。
  const handleAuthRegister = useCallback(
    async (input: { name: string; email: string; password: string }) => {
      setIsAuthSubmitting(true);
      setAuthErrorMessage(null);
      try {
        const session = await dataGateway.auth.register(input);
        if (!session.user) {
          setAuthErrorMessage("注册失败：服务端未返回用户信息");
          return;
        }
        setAuthSession(session);
        setSaveStatus("loading");
        setStatusMessage(`欢迎使用，${session.user.name}`);
        if (authRedirectTarget) {
          window.location.assign(authRedirectTarget);
        }
      } catch (error) {
        setAuthErrorMessage(`注册失败：${formatError(error)}`);
      } finally {
        setIsAuthSubmitting(false);
      }
    },
    [authRedirectTarget, dataGateway]
  );

  // 退出登录：清除会话并返回登录页。
  const handleAuthLogout = useCallback(async () => {
    setIsAuthSubmitting(true);
    setAuthErrorMessage(null);
    try {
      await dataGateway.auth.logout();
      setAuthSession({ user: null });
      setSaveStatus("loading");
      setStatusMessage("已退出登录");
      setLastSavedAt(null);
    } catch (error) {
      setAuthErrorMessage(`退出失败：${formatError(error)}`);
    } finally {
      setIsAuthSubmitting(false);
    }
  }, [dataGateway, setLastSavedAt]);

  if (isAdminRoute) {
    return (
      <>
        <Toaster />
        <AdminApp
          authSession={authSession}
          checking={isAuthChecking}
          submitting={isAuthSubmitting}
          errorMessage={authErrorMessage}
          dataGateway={dataGateway}
          onLogin={handleAuthLogin}
          onLogout={handleAuthLogout}
        />
      </>
    );
  }

  // 登录前只展示认证面板，不渲染编辑器布局。
  if (isAuthChecking || !activeUser) {
    return (
      <>
        <Toaster />
        <AuthPanel
          mode={route.kind === "register" ? "register" : "login"}
          switchPath={route.kind === "register" ? LOGIN_ROUTE_PATH : REGISTER_ROUTE_PATH}
          redirectTarget={authRedirectTarget}
          checking={isAuthChecking}
          submitting={isAuthSubmitting}
          errorMessage={authErrorMessage}
          onLogin={handleAuthLogin}
          onRegister={handleAuthRegister}
        />
      </>
    );
  }

  // 编辑器路由在空间校验失败时显示简约错误页，避免继续渲染无效工作区状态。
  if (isEditorRoute && editorAccessError) {
    return (
      <>
        <Toaster />
        <EditorAccessErrorPage
          spaceId={editorAccessError.spaceId}
          description={editorAccessError.description}
          technicalMessage={editorAccessError.technicalMessage}
          onRetry={retryEditorAccessCheck}
          onBackHome={backToHomepage}
        />
      </>
    );
  }

  return (
    // 主页面容器。
    <div className="page">
      <Toaster />
      {confirmDialog}
      {/* 当前主题样式：先注入内置模板变量，后续允许外部样式继续覆盖。 */}
      {activePreviewThemeStyleText ? (
        <style id="plaindoc-preview-theme-style">{activePreviewThemeStyleText}</style>
      ) : null}
      {/* 主题表内的自定义 CSS：与主题元数据一并维护。 */}
      {activePreviewThemeCustomStyleText ? (
        <style id="plaindoc-preview-theme-custom-style">{activePreviewThemeCustomStyleText}</style>
      ) : null}
      {/* 外部自定义预览样式：存在时插入到页面末端，确保覆盖内置主题。 */}
      {customPreviewStyleText ? (
        <style id="plaindoc-preview-custom-style">{customPreviewStyleText}</style>
      ) : null}
      {/* 顶部状态栏。 */}
      <header className="header">
        <h1>PlainDoc</h1>
        <div className="header-actions">
          <span className="auth-user-pill" title={activeUser.email}>
            {activeUser.name}
          </span>
          <button
            type="button"
            className="auth-logout-button"
            onClick={() => void handleAuthLogout()}
            disabled={isAuthSubmitting}
            title="退出当前账号"
            aria-label="退出当前账号"
          >
            <LogOut size={14} />
            <span>{isAuthSubmitting ? "退出中..." : "退出"}</span>
          </button>
          {/* 目录菜单：展示标题结构并支持快速跳转。 */}
          {hasTocMarker ? <TocMenu items={tocItems} onSelectItem={handleTocNavigate} /> : null}
          {/* 复制到公众号：将当前预览导出为内联样式 HTML。 */}
          <button
            type="button"
            className="wechat-copy-button"
            onClick={() => void handleCopyToWechat()}
            disabled={isWechatCopying}
            title="复制当前预览为公众号可粘贴内容"
            aria-label="复制当前预览为公众号可粘贴内容"
          >
            {isWechatCopying ? "复制中..." : "复制到公众号"}
          </button>
          {/* 预览模式切换：在 PC 与移动端窄屏模拟之间切换。 */}
          <button
            type="button"
            className={`preview-mode-toggle preview-mode-toggle--${previewViewportMode}`}
            onClick={togglePreviewViewportMode}
            title={previewViewportMode === "desktop" ? "切换到移动端预览" : "切换到 PC 预览"}
            aria-label={previewViewportMode === "desktop" ? "切换到移动端预览" : "切换到 PC 预览"}
          >
            {previewViewportMode === "desktop" ? <Monitor size={14} /> : <Smartphone size={14} />}
            <span className="preview-mode-toggle__label">
              {previewViewportMode === "desktop" ? "PC 预览" : "移动预览"}
            </span>
          </button>
          {/* 链接渲染开关：原始链接 / 脚注角标。 */}
          <button
            type="button"
            className={`preview-mode-toggle ${
              previewLinkRenderMode === "footnote" ? "preview-mode-toggle--footnote" : ""
            }`}
            onClick={togglePreviewLinkRenderMode}
            title={
              previewLinkRenderMode === "link"
                ? "切换为脚注+角标渲染"
                : "切换为原始链接渲染"
            }
            aria-label={
              previewLinkRenderMode === "link"
                ? "切换为脚注+角标渲染"
                : "切换为原始链接渲染"
            }
          >
            <Link2 size={14} />
            <span className="preview-mode-toggle__label">
              {previewLinkRenderMode === "link" ? "链接渲染" : "脚注渲染"}
            </span>
          </button>
          {/* 主题菜单：展开/收起只更新菜单组件自身。 */}
          <ThemeMenu
            themes={previewThemes}
            activeThemeId={activePreviewTheme.id}
            onSelectTheme={handleThemeChange}
            customPreviewStyleText={customPreviewStyleText}
          />
        </div>
      </header>
      {/* 工作区主区域：左侧边栏 + 中间编辑器 + 右侧预览。 */}
      <main
        className={`workspace ${isWorkspaceSidebarCollapsed ? "workspace--sidebar-collapsed" : ""}`}
        style={workspaceLayoutStyle}
      >
        <div className="workspace-sidebar-slot">
          <WorkspaceSidebar
            activeSpaceName={activeSpaceName}
            activeDocId={activeDocId}
            workspaceTree={workspaceTree}
            onOpenDocument={handleOpenWorkspaceDocument}
            onCreateNode={handleCreateWorkspaceNode}
            onRenameNode={handleRenameWorkspaceNode}
            onDeleteNode={handleDeleteWorkspaceNode}
          />
        </div>
        <div className="workspace-sidebar-resizer" role="separator" aria-orientation="vertical" aria-label="工作区宽度调节">
          <button
            type="button"
            className="workspace-sidebar-resizer__toggle"
            onClick={toggleWorkspaceSidebar}
            title={isWorkspaceSidebarCollapsed ? "展开工作区目录" : "隐藏工作区目录"}
            aria-label={isWorkspaceSidebarCollapsed ? "展开工作区目录" : "隐藏工作区目录"}
          >
            {isWorkspaceSidebarCollapsed ? <PanelLeftOpen size={14} /> : <PanelLeftClose size={14} />}
          </button>
          <button
            type="button"
            className="workspace-sidebar-resizer__handle"
            onPointerDown={handleWorkspaceSidebarResizeStart}
            disabled={isWorkspaceSidebarCollapsed}
            title={
              isWorkspaceSidebarCollapsed ? "请先展开工作区目录" : "拖拽调整工作区目录宽度"
            }
            aria-label={
              isWorkspaceSidebarCollapsed ? "请先展开工作区目录" : "拖拽调整工作区目录宽度"
            }
          >
            <GripVertical size={14} />
          </button>
        </div>
        <section className="pane editor-pane" ref={handleEditorPaneRef}>
          <CodeMirror
            value={content}
            extensions={extensions}
            height="100%"
            onCreateEditor={handleEditorCreate}
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
          className={`pane preview-pane preview-pane--${previewViewportMode} ${PREVIEW_PANE_CLASS}`}
          // 使用稳定 ref 回调，保证滚动监听不会被重复拆装。
          ref={handlePreviewScrollerRef}
        >
          <div className={`preview-viewport preview-viewport--${previewViewportMode}`}>
            <article
              id={PREVIEW_BODY_ID}
              className={`markdown-body ${PREVIEW_BODY_CLASS} preview-body--${previewViewportMode} ${activePreviewThemeClassName}`}
            >
              {/* 使用 remark 插件渲染 Markdown 并写入 block 锚点。 */}
              <ReactMarkdown
                remarkPlugins={remarkPlugins}
                // 开启 Markdown 内嵌 HTML 解析，安全边界由 rehype-sanitize 白名单控制。
                remarkRehypeOptions={PREVIEW_MARKDOWN_REHYPE_OPTIONS}
                rehypePlugins={rehypePlugins}
                components={markdownComponents}
              >
                {content}
              </ReactMarkdown>
            </article>
          </div>
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
          <span style={{ fontWeight: 600 }}>文档位置：</span>
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
          <span>
            <span style={{ fontWeight: 600 }}>最后保存时间：</span>
            {lastSavedTimeLabel}
          </span>
          <span>
            <span style={{ fontWeight: 600 }}>字数统计：</span>
            {plainTextCount}
          </span>
        </div>
      </footer>
    </div>
  );
}
