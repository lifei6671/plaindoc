import { AUTH_UNAUTHORIZED_EVENT, type AuthUnauthorizedEventDetail } from "../auth-events";
import {
  type AdminDataRetentionCleanupResult,
  type AdminAuditListInput,
  type AdminAuditListResult,
  type AdminDocument,
  type AdminDocumentAttachmentDeleteResult,
  type AdminDocumentAttachmentListInput,
  type AdminDocumentAttachmentListResult,
  type AdminDocumentListInput,
  type AdminDocumentListResult,
  type AdminGateway,
  type AdminIdentity,
  type AdminProfile,
  type AdminRole,
  type AdminSpace,
  type AdminSpaceCategory,
  type AdminSpaceCover,
  type AdminSpaceListInput,
  type AdminSpaceListResult,
  type AdminSpaceMember,
  type AdminSystemConfig,
  type AdminTheme,
  type AdminUser,
  type AdminUserListInput,
  type AdminUserListResult,
  type AuthCaptchaChallenge,
  type AuthCaptchaRefreshInput,
  type AuthGateway,
  type AuthLoginInput,
  type AuthLoginMode,
  type AuthLoginOptions,
  type AuthLoginProviderOption,
  type AuthRegisterInput,
  type AuthSession,
  ConflictError,
  type CreateNodeInput,
  type CreateNodeResult,
  type CreateSpaceInput,
  type DataGateway,
  type Document,
  type DocumentAttachment,
  type DocumentAttachmentAccessLink,
  type DocumentGateway,
  type DocumentRevision,
  type ImageHostingGateway,
  type LocalizeRemoteImagesInput,
  type LocalizeRemoteImagesResult,
  type MoveNodeInput,
  type SaveDocumentInput,
  type SaveDocumentResult,
  type Space,
  type Theme,
  type ThemeGateway,
  type TreeNode,
  type UpdateNodeInput,
  type UploadLocalImageResult,
  type WorkspaceGateway
} from "../types";
import { createIndexedDbUserConfigGateway } from "../user-config/indexeddb-gateway";

interface HttpAdapterOptions {
  baseUrl: string;
}

const ACCESS_TOKEN_STORAGE_KEY = "plaindoc.auth.access-token";
const REFRESH_TOKEN_STORAGE_KEY = "plaindoc.auth.refresh-token";
const JSON_RESULT_SUCCESS_CODE = 0;
const JSON_RESULT_UNAUTHORIZED_CODE = 1001;
const JSON_RESULT_CONFIG_VERSION_CONFLICT_CODE = 4002;
const JSON_RESULT_DOCUMENT_VERSION_CONFLICT_CODE = 4010;

interface HttpAuthSession extends AuthSession {
  refreshToken?: string;
}

class HttpRequestError extends Error {
  readonly status: number;
  readonly body: string;
  readonly code?: number;
  readonly data?: unknown;

  constructor(status: number, body: string, code?: number, data?: unknown) {
    super(body || `Request failed: ${status}`);
    this.name = "HttpRequestError";
    this.status = status;
    this.body = body;
    this.code = code;
    this.data = data;
  }
}

function readStoredValue(key: string): string | null {
  try {
    const value = window.localStorage.getItem(key);
    return value && value.trim() ? value : null;
  } catch {
    return null;
  }
}

function writeStoredValue(key: string, value: string): void {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    // localStorage 不可用时忽略持久化。
  }
}

function removeStoredValue(key: string): void {
  try {
    window.localStorage.removeItem(key);
  } catch {
    // localStorage 不可用时忽略持久化。
  }
}

function toRequestError(status: number, body: string, code?: number, data?: unknown): HttpRequestError {
  return new HttpRequestError(status, body, code, data);
}

interface JsonResultEnvelope<T> {
  code?: number;
  message?: string;
  requestId?: string;
  data?: T;
}

const DEFAULT_AUTH_LOGIN_OPTIONS: AuthLoginOptions = {
  loginMode: "local_only",
  defaultProviderId: "local",
  allowUserRegister: true,
  providers: []
};

function isJsonResultEnvelope(value: unknown): value is JsonResultEnvelope<unknown> {
  if (!value || typeof value !== "object") {
    return false;
  }
  const record = value as Record<string, unknown>;
  return "code" in record && "data" in record;
}

function normalizeAuthLoginMode(value: unknown): AuthLoginMode {
  if (value === "local_only" || value === "ldap_only" || value === "mixed") {
    return value;
  }
  return DEFAULT_AUTH_LOGIN_OPTIONS.loginMode;
}

function normalizeAuthLoginOptions(value: unknown): AuthLoginOptions {
  if (!value || typeof value !== "object") {
    return DEFAULT_AUTH_LOGIN_OPTIONS;
  }
  const record = value as Record<string, unknown>;
  const providersRaw = Array.isArray(record.providers) ? record.providers : [];
  const providers: AuthLoginProviderOption[] = providersRaw
    .map((provider) => {
      if (!provider || typeof provider !== "object") {
        return null;
      }
      const providerRecord = provider as Record<string, unknown>;
      const id = typeof providerRecord.id === "string" ? providerRecord.id.trim() : "";
      if (!id) {
        return null;
      }
      const name = typeof providerRecord.name === "string" ? providerRecord.name.trim() : id;
      const type = typeof providerRecord.type === "string" ? providerRecord.type.trim().toLowerCase() : "";
      const priorityValue = Number(providerRecord.priority);
      return {
        id,
        name: name || id,
        type: type || "ldap",
        priority: Number.isFinite(priorityValue) ? Math.trunc(priorityValue) : 0
      };
    })
    .filter((provider): provider is AuthLoginProviderOption => provider !== null)
    .sort((left, right) => right.priority - left.priority || left.id.localeCompare(right.id));

  const defaultProviderIdRaw =
    typeof record.defaultProviderId === "string" ? record.defaultProviderId.trim() : "";
  const defaultProviderId = defaultProviderIdRaw || DEFAULT_AUTH_LOGIN_OPTIONS.defaultProviderId;
  const allowUserRegister =
    typeof record.allowUserRegister === "boolean"
      ? record.allowUserRegister
      : DEFAULT_AUTH_LOGIN_OPTIONS.allowUserRegister;

  return {
    loginMode: normalizeAuthLoginMode(record.loginMode),
    defaultProviderId,
    allowUserRegister,
    providers
  };
}

function normalizeAuthCaptchaChallenge(value: unknown): AuthCaptchaChallenge {
  if (!value || typeof value !== "object") {
    throw new Error("验证码会话数据格式不正确");
  }
  const record = value as Record<string, unknown>;
  const captchaId = typeof record.captchaId === "string" ? record.captchaId.trim() : "";
  const captchaImageDataUrl =
    typeof record.captchaImageDataUrl === "string" ? record.captchaImageDataUrl.trim() : "";
  const level = Number(record.level);
  const expiresInSeconds = Number(record.expiresInSeconds);
  if (!captchaId || !captchaImageDataUrl || !Number.isFinite(level) || !Number.isFinite(expiresInSeconds)) {
    throw new Error("验证码会话数据不完整");
  }
  return {
    captchaId,
    captchaImageDataUrl,
    level: Math.trunc(level),
    expiresInSeconds: Math.trunc(expiresInSeconds)
  };
}

function parseResponsePayload(rawBody: string): unknown {
  const text = rawBody.trim();
  if (!text) {
    return undefined;
  }
  try {
    return JSON.parse(text);
  } catch {
    return rawBody;
  }
}

function extractErrorMessage(status: number, payload: unknown, fallbackBody: string): string {
  if (typeof payload === "string" && payload.trim()) {
    return payload;
  }
  if (isJsonResultEnvelope(payload) && typeof payload.message === "string" && payload.message.trim()) {
    return payload.message.trim();
  }
  if (payload && typeof payload === "object") {
    const record = payload as Record<string, unknown>;
    if (typeof record.message === "string" && record.message.trim()) {
      return record.message.trim();
    }
  }
  const text = fallbackBody.trim();
  if (text) {
    return text;
  }
  return `Request failed: ${status}`;
}

function isAbsoluteUrl(value: string): boolean {
  return /^https?:\/\//i.test(value.trim());
}

// 关键函数：补全后端资源的绝对 URL，避免前后端端口不同导致相对路径失效。
function resolveBackendPublicUrl(rawUrl: string, apiBaseUrl: string): string {
  const trimmed = rawUrl.trim();
  if (!trimmed) {
    return rawUrl;
  }
  if (isAbsoluteUrl(trimmed) || /^data:|^blob:/i.test(trimmed)) {
    return trimmed;
  }
  if (!isAbsoluteUrl(apiBaseUrl)) {
    return trimmed;
  }

  try {
    const apiOrigin = new URL(apiBaseUrl).origin;
    const path = trimmed.startsWith("/") ? trimmed : `/${trimmed}`;
    return `${apiOrigin}${path}`;
  } catch {
    return trimmed;
  }
}

// 空间封面统一补全后端域名，避免列表页渲染失败。
function normalizeAdminSpaceCover(
  cover: AdminSpaceCover | null | undefined,
  apiBaseUrl: string
): AdminSpaceCover | null | undefined {
  if (!cover) {
    return cover;
  }
  if (!cover.url || !cover.url.trim()) {
    return cover;
  }
  const normalizedUrl = resolveBackendPublicUrl(cover.url, apiBaseUrl);
  if (normalizedUrl === cover.url) {
    return cover;
  }
  return {
    ...cover,
    url: normalizedUrl
  };
}

function normalizeUploadEndpoint(uploadEndpoint: string | undefined, apiBaseUrl: string): string {
  const raw = (uploadEndpoint ?? "").trim();
  if (!raw) {
    return "/uploads/images";
  }
  if (isAbsoluteUrl(raw)) {
    return raw;
  }

  const endpointPath = raw.startsWith("/") ? raw : `/${raw}`;

  try {
    const basePath = isAbsoluteUrl(apiBaseUrl)
      ? new URL(apiBaseUrl).pathname
      : apiBaseUrl.startsWith("/")
        ? apiBaseUrl
        : `/${apiBaseUrl}`;
    const normalizedBasePath = basePath.replace(/\/+$/, "");
    if (
      normalizedBasePath &&
      normalizedBasePath !== "/" &&
      endpointPath.startsWith(`${normalizedBasePath}/`)
    ) {
      return endpointPath.slice(normalizedBasePath.length) || "/";
    }
  } catch {
    // 忽略解析失败，保留原始路径。
  }

  return endpointPath;
}

// 模块职责：HTTP 数据驱动，负责与后端 API 交互并做必要的客户端归一化处理。
export function createHttpAdapter(options: HttpAdapterOptions): DataGateway {
  let accessToken: string | null = readStoredValue(ACCESS_TOKEN_STORAGE_KEY);
  let refreshToken: string | null = readStoredValue(REFRESH_TOKEN_STORAGE_KEY);
  let hasNotifiedUnauthorized = false;

  const notifyUnauthorized = (status: number, code?: number, message?: string) => {
    if (hasNotifiedUnauthorized) {
      return;
    }
    hasNotifiedUnauthorized = true;
    clearStoredTokens();
    if (
      typeof window === "undefined" ||
      typeof window.dispatchEvent !== "function" ||
      typeof CustomEvent !== "function"
    ) {
      return;
    }
    const detail: AuthUnauthorizedEventDetail = { status, code, message };
    window.dispatchEvent(new CustomEvent<AuthUnauthorizedEventDetail>(AUTH_UNAUTHORIZED_EVENT, { detail }));
  };

  const clearStoredTokens = () => {
    accessToken = null;
    refreshToken = null;
    removeStoredValue(ACCESS_TOKEN_STORAGE_KEY);
    removeStoredValue(REFRESH_TOKEN_STORAGE_KEY);
  };

  const saveSessionTokens = (session: HttpAuthSession) => {
    hasNotifiedUnauthorized = false;
    if (typeof session.token === "string") {
      accessToken = session.token.trim() || null;
    }
    if (typeof session.refreshToken === "string") {
      refreshToken = session.refreshToken.trim() || null;
    }
    if (accessToken) {
      writeStoredValue(ACCESS_TOKEN_STORAGE_KEY, accessToken);
    } else {
      removeStoredValue(ACCESS_TOKEN_STORAGE_KEY);
    }
    if (refreshToken) {
      writeStoredValue(REFRESH_TOKEN_STORAGE_KEY, refreshToken);
    } else {
      removeStoredValue(REFRESH_TOKEN_STORAGE_KEY);
    }
  };

  const request = async <T>(
    path: string,
    init?: RequestInit,
    optionsOverride?: {
      skipAuth?: boolean;
      retryOnUnauthorized?: boolean;
    }
  ): Promise<T> => {
    const skipAuth = optionsOverride?.skipAuth ?? false;
    const retryOnUnauthorized = optionsOverride?.retryOnUnauthorized ?? true;
    const headers = new Headers(init?.headers);
    const isFormDataBody =
      typeof FormData !== "undefined" && init?.body instanceof FormData;
    if (!isFormDataBody && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
    if (!skipAuth && accessToken) {
      headers.set("Authorization", `Bearer ${accessToken}`);
    }

    const requestUrl = isAbsoluteUrl(path) ? path : `${options.baseUrl}${path}`;
    const response = await fetch(requestUrl, {
      ...init,
      headers,
      credentials: init?.credentials ?? "include"
    });
    const rawBody = await response.text();
    const payload = parseResponsePayload(rawBody);
    const envelope = isJsonResultEnvelope(payload) ? payload : null;
    const resultCode = typeof envelope?.code === "number" ? envelope.code : undefined;

    if (
      response.status === 403 &&
      resultCode === JSON_RESULT_UNAUTHORIZED_CODE &&
      !skipAuth &&
      retryOnUnauthorized
    ) {
      const refreshed = await refreshSession();
      if (refreshed) {
        return request<T>(path, init, {
          skipAuth: false,
          retryOnUnauthorized: false
        });
      }
    }

    if (
      resultCode === JSON_RESULT_CONFIG_VERSION_CONFLICT_CODE ||
      resultCode === JSON_RESULT_DOCUMENT_VERSION_CONFLICT_CODE
    ) {
      const resultData = envelope?.data as { latestDocument?: Document } | undefined;
      if (resultData?.latestDocument) {
        throw new ConflictError(resultData.latestDocument);
      }
    } else if (response.status === 409 && payload && typeof payload === "object") {
      const value = payload as { latestDocument?: Document };
      if (value.latestDocument) {
        throw new ConflictError(value.latestDocument);
      }
    }

    if (!response.ok) {
      const message = extractErrorMessage(response.status, payload, rawBody);
      if (response.status === 403 && resultCode === JSON_RESULT_UNAUTHORIZED_CODE) {
        notifyUnauthorized(response.status, resultCode, message);
      }
      throw toRequestError(response.status, message, resultCode, envelope?.data);
    }

    if (
      envelope &&
      typeof envelope.code === "number" &&
      envelope.code !== JSON_RESULT_SUCCESS_CODE
    ) {
      const message = extractErrorMessage(response.status, payload, rawBody);
      throw toRequestError(response.status, message, envelope.code, envelope.data);
    }

    if (response.status === 204) {
      return undefined as T;
    }

    if (envelope) {
      return envelope.data as T;
    }
    return payload as T;
  };

  const refreshSession = async (): Promise<boolean> => {
    try {
      const refreshPayload = refreshToken ? JSON.stringify({ refreshToken }) : undefined;
      const session = await request<HttpAuthSession>(
        "/auth/refresh",
        {
          method: "POST",
          body: refreshPayload
        },
        {
          skipAuth: true,
          retryOnUnauthorized: false
        }
      );
      if (!session.user || !session.token) {
        clearStoredTokens();
        return false;
      }
      saveSessionTokens(session);
      return true;
    } catch {
      clearStoredTokens();
      return false;
    }
  };

  const auth: AuthGateway = {
    async getLoginOptions() {
      const options = await request<unknown>(
        "/auth/options",
        undefined,
        {
          skipAuth: true,
          retryOnUnauthorized: false
        }
      );
      return normalizeAuthLoginOptions(options);
    },
    async getSession() {
      try {
        const session = await request<HttpAuthSession>("/auth/me", undefined, {
          skipAuth: true
        });
        if (!session.user) {
          clearStoredTokens();
          return { user: null };
        }
        if (session.token || session.refreshToken) {
          saveSessionTokens(session);
        }
        return {
          user: session.user,
          token: accessToken ?? undefined
        };
      } catch (error) {
        if (
          error instanceof HttpRequestError &&
          (error.code === JSON_RESULT_UNAUTHORIZED_CODE || error.status === 403)
        ) {
          clearStoredTokens();
          return { user: null };
        }
        throw error;
      }
    },
    async login(input) {
      const payload: AuthLoginInput = {
        password: input.password
      };
      const identifier = typeof input.identifier === "string" ? input.identifier.trim() : "";
      const email = typeof input.email === "string" ? input.email.trim() : "";
      const provider = typeof input.provider === "string" ? input.provider.trim() : "";
      if (identifier) {
        payload.identifier = identifier;
      } else if (email) {
        payload.email = email;
      }
      if (provider) {
        payload.provider = provider;
      }
      const captchaId = typeof input.captchaId === "string" ? input.captchaId.trim() : "";
      const captchaAnswer = typeof input.captchaAnswer === "string" ? input.captchaAnswer.trim() : "";
      if (captchaId) {
        payload.captchaId = captchaId;
      }
      if (captchaAnswer) {
        payload.captchaAnswer = captchaAnswer;
      }
      const session = await request<HttpAuthSession>("/auth/login", {
        method: "POST",
        body: JSON.stringify(payload)
      });
      saveSessionTokens(session);
      return {
        user: session.user,
        token: session.token
      };
    },
    async register(input: AuthRegisterInput) {
      const payload: AuthRegisterInput = {
        email: input.email,
        password: input.password,
        name: input.name
      };
      const captchaId = typeof input.captchaId === "string" ? input.captchaId.trim() : "";
      const captchaAnswer = typeof input.captchaAnswer === "string" ? input.captchaAnswer.trim() : "";
      if (captchaId) {
        payload.captchaId = captchaId;
      }
      if (captchaAnswer) {
        payload.captchaAnswer = captchaAnswer;
      }
      const session = await request<HttpAuthSession>("/auth/register", {
        method: "POST",
        body: JSON.stringify(payload)
      });
      saveSessionTokens(session);
      return {
        user: session.user,
        token: session.token
      };
    },
    async refreshCaptcha(input: AuthCaptchaRefreshInput) {
      const scene = typeof input.scene === "string" ? input.scene.trim() : "";
      if (scene !== "login" && scene !== "register") {
        throw new Error("验证码刷新场景无效");
      }
      const identifier = typeof input.identifier === "string" ? input.identifier.trim() : "";
      if (!identifier) {
        throw new Error("验证码刷新账号不能为空");
      }
      const payload: AuthCaptchaRefreshInput = {
        scene,
        identifier
      };
      const captchaId = typeof input.captchaId === "string" ? input.captchaId.trim() : "";
      if (captchaId) {
        payload.captchaId = captchaId;
      }
      const challenge = await request<unknown>(
        "/auth/captcha/refresh",
        {
          method: "POST",
          body: JSON.stringify(payload)
        },
        {
          skipAuth: true,
          retryOnUnauthorized: false
        }
      );
      return normalizeAuthCaptchaChallenge(challenge);
    },
    async logout() {
      try {
        await request<void>("/auth/logout", {
          method: "POST"
        });
      } finally {
        clearStoredTokens();
      }
    }
  };

  const workspace: WorkspaceGateway = {
    async listSpaces() {
      return request<Space[]>("/spaces");
    },
    async getSpace(spaceId: string) {
      const targetSpaceID = spaceId.trim();
      if (!targetSpaceID) {
        throw new Error("空间 ID 不能为空");
      }
      return request<Space>(`/spaces/${targetSpaceID}`);
    },
    async createSpace(input: CreateSpaceInput) {
      return request<Space>("/spaces", {
        method: "POST",
        body: JSON.stringify(input)
      });
    },
    async getTree(spaceId: string) {
      return request<TreeNode[]>(`/spaces/${spaceId}/tree`);
    },
    async createNode(input: CreateNodeInput) {
      return request<CreateNodeResult>(`/spaces/${input.spaceId}/nodes`, {
        method: "POST",
        body: JSON.stringify(input)
      });
    },
    async updateNode(input: UpdateNodeInput) {
      await request<void>(`/nodes/${input.nodeId}`, {
        method: "PATCH",
        body: JSON.stringify(input)
      });
    },
    async deleteNode(nodeId: string) {
      await request<void>(`/nodes/${nodeId}`, {
        method: "DELETE"
      });
    },
    async moveNode(input: MoveNodeInput) {
      await request<void>(`/nodes/${input.nodeId}/move`, {
        method: "POST",
        body: JSON.stringify({
          targetParentId: input.targetParentId,
          targetIndex: input.targetIndex
        })
      });
    }
  };

  const document: DocumentGateway = {
    async getDocument(docId: string) {
      return request<Document>(`/docs/${docId}`);
    },
    async saveDocument(input: SaveDocumentInput) {
      return request<SaveDocumentResult>(`/docs/${input.docId}`, {
        method: "PUT",
        body: JSON.stringify(input)
      });
    },
    async localizeRemoteImages(input: LocalizeRemoteImagesInput) {
      const documentID = input.docId.trim();
      if (!documentID) {
        throw new Error("文档 ID 不能为空");
      }
      const imageURLs = Array.isArray(input.imageUrls)
        ? input.imageUrls
          .map((item) => (typeof item === "string" ? item.trim() : ""))
          .filter((item) => item.length > 0)
        : [];
      if (!imageURLs.length) {
        return { localizedUrls: {} };
      }
      const payload = await request<LocalizeRemoteImagesResult>(
        `/docs/${documentID}/remote-images/localize`,
        {
          method: "POST",
          body: JSON.stringify({
            imageUrls: imageURLs
          })
        }
      );
      const localizedURLsRecord =
        payload.localizedUrls && typeof payload.localizedUrls === "object"
          ? payload.localizedUrls
          : {};
      const normalizedLocalizedURLs: Record<string, string> = {};
      for (const [rawSourceURL, rawLocalizedURL] of Object.entries(localizedURLsRecord)) {
        const sourceURL = rawSourceURL.trim();
        const localizedURL = typeof rawLocalizedURL === "string" ? rawLocalizedURL.trim() : "";
        if (!sourceURL || !localizedURL) {
          continue;
        }
        normalizedLocalizedURLs[sourceURL] = resolveBackendPublicUrl(localizedURL, options.baseUrl);
      }
      return { localizedUrls: normalizedLocalizedURLs };
    },
    async listRevisions(docId: string) {
      return request<DocumentRevision[]>(`/docs/${docId}/revisions`);
    },
    async listAttachments(docId: string) {
      const documentID = docId.trim();
      if (!documentID) {
        throw new Error("文档 ID 不能为空");
      }
      const payload = await request<{ items: DocumentAttachment[] }>(`/docs/${encodeURIComponent(documentID)}/attachments`);
      if (!Array.isArray(payload.items)) {
        return [];
      }
      return payload.items.map((item) => ({
        ...item,
        publicDownloadUrl:
          typeof item.publicDownloadUrl === "string" && item.publicDownloadUrl.trim()
            ? resolveBackendPublicUrl(item.publicDownloadUrl, options.baseUrl)
            : item.publicDownloadUrl
      }));
    },
    async uploadAttachment(input: { docId: string; file: File }) {
      const documentID = input.docId.trim();
      if (!documentID) {
        throw new Error("文档 ID 不能为空");
      }
      if (!input.file) {
        throw new Error("附件文件不能为空");
      }
      const formData = new FormData();
      formData.append("file", input.file);
      return request<DocumentAttachment>(`/docs/${encodeURIComponent(documentID)}/attachments`, {
        method: "POST",
        body: formData
      });
    },
    async deleteAttachment(input: { docId: string; attachmentId: string; physicalDelete?: boolean }) {
      const documentID = input.docId.trim();
      const attachmentID = input.attachmentId.trim();
      if (!documentID) {
        throw new Error("文档 ID 不能为空");
      }
      if (!attachmentID) {
        throw new Error("附件 ID 不能为空");
      }
      const query = new URLSearchParams();
      if (input.physicalDelete === true) {
        query.set("physicalDelete", "true");
      }
      const queryText = query.toString();
      const path =
        `/docs/${encodeURIComponent(documentID)}/attachments/${encodeURIComponent(attachmentID)}` +
        (queryText ? `?${queryText}` : "");
      await request<void>(path, {
        method: "DELETE"
      });
    },
    async createAttachmentAccessLink(input: {
      docId: string;
      attachmentId: string;
      purpose?: "download" | "preview";
    }) {
      const documentID = input.docId.trim();
      const attachmentID = input.attachmentId.trim();
      if (!documentID) {
        throw new Error("文档 ID 不能为空");
      }
      if (!attachmentID) {
        throw new Error("附件 ID 不能为空");
      }
      const query = new URLSearchParams();
      if (input.purpose === "preview" || input.purpose === "download") {
        query.set("purpose", input.purpose);
      }
      const queryText = query.toString();
      const path =
        `/docs/${encodeURIComponent(documentID)}/attachments/${encodeURIComponent(attachmentID)}/access-link` +
        (queryText ? `?${queryText}` : "");
      const accessLink = await request<DocumentAttachmentAccessLink>(path, {
        method: "POST"
      });
      return {
        ...accessLink,
        url: resolveBackendPublicUrl(accessLink.url ?? "", options.baseUrl)
      };
    },
    async setDocumentTheme(docId: string, themeId: string) {
      return request<Document>(`/docs/${docId}/theme`, {
        method: "PUT",
        body: JSON.stringify({ themeId })
      });
    },
    async updateDocumentVisibility(docId: string, visibility: "public" | "authenticated" | "member") {
      return request<Document>(`/docs/${docId}/visibility`, {
        method: "PUT",
        body: JSON.stringify({ visibility })
      });
    }
  };

  const theme: ThemeGateway = {
    async listThemes() {
      return request<Theme[]>("/themes");
    }
  };

  const issueAdminOperationToken = async (input: {
    operation: string;
    targetType?: string;
    targetId?: string;
  }): Promise<string> => {
    const payload = await request<{ token: string }>("/admin/operation-tokens", {
      method: "POST",
      body: JSON.stringify({
        operation: input.operation,
        targetType: input.targetType ?? "",
        targetId: input.targetId ?? ""
      })
    });
    const token = typeof payload.token === "string" ? payload.token.trim() : "";
    if (!token) {
      throw new Error("高风险操作令牌签发失败");
    }
    return token;
  };

  const buildAdminOperationTokenHeaders = (operationToken: string): Headers => {
    const headers = new Headers();
    headers.set("X-Admin-Operation-Token", operationToken);
    return headers;
  };

  const admin: AdminGateway = {
    async getMe() {
      return request<AdminIdentity>("/admin/me");
    },
    async getProfile() {
      return request<AdminProfile>("/admin/profile");
    },
    async updateProfile(input: { name?: string; avatarUrl?: string }) {
      const payload: Record<string, string> = {};
      if (typeof input.name === "string") {
        payload.name = input.name.trim();
      }
      if (typeof input.avatarUrl === "string") {
        payload.avatarUrl = input.avatarUrl.trim();
      }
      return request<AdminProfile>("/admin/profile", {
        method: "PATCH",
        body: JSON.stringify(payload)
      });
    },
    async updatePassword(input: { currentPassword: string; newPassword: string; confirmPassword: string }) {
      const currentPassword = input.currentPassword ?? "";
      const newPassword = input.newPassword ?? "";
      const confirmPassword = input.confirmPassword ?? "";
      if (!currentPassword) {
        throw new Error("当前密码不能为空");
      }
      if (!newPassword) {
        throw new Error("新密码不能为空");
      }
      await request<void>("/admin/profile/password", {
        method: "PUT",
        body: JSON.stringify({
          currentPassword,
          newPassword,
          confirmPassword
        })
      });
    },
    async uploadAvatar(file: File) {
      if (!file) {
        throw new Error("请选择头像文件");
      }
      const formData = new FormData();
      formData.append("file", file);
      return request<AdminProfile>("/admin/profile/avatar", {
        method: "POST",
        body: formData
      });
    },
    async canManageSpace(spaceId: string) {
      const payload = await request<{ canManage: boolean }>(`/admin/spaces/${spaceId}/check`);
      return payload.canManage;
    },
    async listUsers(input: AdminUserListInput = {}) {
      const query = new URLSearchParams();
      if (typeof input.keyword === "string" && input.keyword.trim()) {
        query.set("keyword", input.keyword.trim());
      }
      if (typeof input.status === "string" && input.status.trim()) {
        query.set("status", input.status);
      }
      if (typeof input.page === "number" && Number.isFinite(input.page) && input.page > 0) {
        query.set("page", String(Math.trunc(input.page)));
      }
      if (typeof input.pageSize === "number" && Number.isFinite(input.pageSize) && input.pageSize > 0) {
        query.set("pageSize", String(Math.trunc(input.pageSize)));
      }

      const queryText = query.toString();
      const path = queryText ? `/admin/users?${queryText}` : "/admin/users";
      return request<AdminUserListResult>(path);
    },
    async createUser(input: { email: string; name: string; password: string; role: "user" | AdminRole }) {
      const email = input.email.trim();
      const name = input.name.trim();
      const role = input.role;
      if (!email) {
        throw new Error("邮箱不能为空");
      }
      if (!name) {
        throw new Error("昵称不能为空");
      }
      if (!input.password) {
        throw new Error("密码不能为空");
      }
      if (role !== "user" && role !== "space_admin" && role !== "platform_admin") {
        throw new Error("用户角色非法");
      }
      return request<AdminUser>("/admin/users", {
        method: "POST",
        body: JSON.stringify({
          email,
          name,
          password: input.password,
          role
        })
      });
    },
    async updateUserRole(input: { userId: string; role: "user" | AdminRole }) {
      const targetUserID = input.userId.trim();
      if (!targetUserID) {
        throw new Error("用户 ID 不能为空");
      }
      const role = input.role;
      if (role !== "user" && role !== "space_admin" && role !== "platform_admin") {
        throw new Error("用户角色非法");
      }
      const operationToken = await issueAdminOperationToken({
        operation: "user.update_role",
        targetType: "user",
        targetId: targetUserID
      });
      return request<AdminUser>(`/admin/users/${encodeURIComponent(targetUserID)}/role`, {
        method: "PATCH",
        headers: buildAdminOperationTokenHeaders(operationToken),
        body: JSON.stringify({
          role
        })
      });
    },
    async updateUserStatus(input: { userId: string; status: "active" | "banned"; reason?: string }) {
      const targetUserID = input.userId.trim();
      if (!targetUserID) {
        throw new Error("用户 ID 不能为空");
      }
      const operationToken = await issueAdminOperationToken({
        operation: "user.update_status",
        targetType: "user",
        targetId: targetUserID
      });
      return request<AdminUser>(`/admin/users/${encodeURIComponent(targetUserID)}/status`, {
        method: "PATCH",
        headers: buildAdminOperationTokenHeaders(operationToken),
        body: JSON.stringify({
          status: input.status,
          reason: input.reason ?? ""
        })
      });
    },
    async deleteUser(userId: string) {
      const targetUserID = userId.trim();
      if (!targetUserID) {
        throw new Error("用户 ID 不能为空");
      }
      const operationToken = await issueAdminOperationToken({
        operation: "user.delete",
        targetType: "user",
        targetId: targetUserID
      });
      await request<void>(`/admin/users/${encodeURIComponent(targetUserID)}`, {
        method: "DELETE",
        headers: buildAdminOperationTokenHeaders(operationToken)
      });
    },
    async createSpace(input: {
      spaceId?: string;
      name: string;
      description?: string;
      categoryId?: string;
      visibility?: "public" | "authenticated" | "member";
      coverAssetId?: string;
    }) {
      const name = input.name.trim();
      if (!name) {
        throw new Error("空间名称不能为空");
      }
      const spaceId = (input.spaceId ?? "").trim();
      const space = await request<AdminSpace>("/admin/spaces", {
        method: "POST",
        body: JSON.stringify({
          spaceId,
          name,
          description: input.description ?? "",
          categoryId: input.categoryId ?? "",
          visibility: input.visibility ?? "member",
          coverAssetId: input.coverAssetId ?? ""
        })
      });
      return {
        ...space,
        cover: normalizeAdminSpaceCover(space.cover, options.baseUrl)
      };
    },
    async createSpaceCoverAsset(input: {
      source: "user_upload" | "system_generated";
      file?: File;
      spaceName?: string;
      clientWidth?: number;
      clientHeight?: number;
      clientMimeType?: string;
      clientProcessed?: boolean;
    }) {
      const source = input.source;
      if (source !== "user_upload" && source !== "system_generated") {
        throw new Error("封面来源不合法");
      }

      const formData = new FormData();
      formData.append("source", source);

      if (source === "user_upload") {
        if (!input.file) {
          throw new Error("请先选择要上传的封面文件");
        }
        formData.append("file", input.file);
      } else {
        const spaceName = (input.spaceName ?? "").trim();
        if (!spaceName) {
          throw new Error("空间名称不能为空");
        }
        formData.append("spaceName", spaceName);
      }

      if (typeof input.clientWidth === "number" && Number.isFinite(input.clientWidth) && input.clientWidth > 0) {
        formData.append("clientWidth", String(Math.trunc(input.clientWidth)));
      }
      if (typeof input.clientHeight === "number" && Number.isFinite(input.clientHeight) && input.clientHeight > 0) {
        formData.append("clientHeight", String(Math.trunc(input.clientHeight)));
      }
      if (typeof input.clientMimeType === "string" && input.clientMimeType.trim()) {
        formData.append("clientMimeType", input.clientMimeType.trim());
      }
      if (typeof input.clientProcessed === "boolean") {
        formData.append("clientProcessed", input.clientProcessed ? "true" : "false");
      }

      const cover = await request<AdminSpaceCover>("/admin/spaces/cover-assets", {
        method: "POST",
        body: formData
      });
      return {
        ...cover,
        url: resolveBackendPublicUrl(cover.url, options.baseUrl)
      };
    },
    async listSpaceCategories() {
      const payload = await request<{ items: AdminSpaceCategory[] }>("/admin/spaces/categories");
      if (!Array.isArray(payload.items)) {
        return [];
      }
      return payload.items
        .map((item) => ({
          categoryId: typeof item?.categoryId === "string" ? item.categoryId.trim() : "",
          name: typeof item?.name === "string" ? item.name.trim() : "",
          isDefault: item?.isDefault === true,
          createdAt: typeof item?.createdAt === "string" ? item.createdAt : "",
          updatedAt: typeof item?.updatedAt === "string" ? item.updatedAt : ""
        }))
        .filter((item) => item.categoryId.length > 0 && item.name.length > 0)
        .filter((item, index, items) => items.findIndex((current) => current.categoryId === item.categoryId) === index);
    },
    async createSpaceCategory(input: { name: string }) {
      const name = input.name.trim();
      if (!name) {
        throw new Error("分类名称不能为空");
      }
      return request<AdminSpaceCategory>("/admin/spaces/categories", {
        method: "POST",
        body: JSON.stringify({ name })
      });
    },
    async renameSpaceCategory(input: { categoryId: string; name: string }) {
      const categoryId = input.categoryId.trim();
      const name = input.name.trim();
      if (!categoryId) {
        throw new Error("分类 ID 不能为空");
      }
      if (!name) {
        throw new Error("分类名称不能为空");
      }
      return request<AdminSpaceCategory>(`/admin/spaces/categories/${encodeURIComponent(categoryId)}`, {
        method: "PATCH",
        body: JSON.stringify({ name })
      });
    },
    async deleteSpaceCategory(input: { categoryId: string }) {
      const categoryId = input.categoryId.trim();
      if (!categoryId) {
        throw new Error("分类 ID 不能为空");
      }
      return request<{
        categoryId: string;
        reassignedToCategoryId: string;
        reassignedToName: string;
        movedSpaceCount: number;
      }>(`/admin/spaces/categories/${encodeURIComponent(categoryId)}`, {
        method: "DELETE"
      });
    },
    async listSpaces(input: AdminSpaceListInput = {}) {
      const query = new URLSearchParams();
      if (typeof input.keyword === "string" && input.keyword.trim()) {
        query.set("keyword", input.keyword.trim());
      }
      if (typeof input.status === "string" && input.status.trim()) {
        query.set("status", input.status);
      }
      if (typeof input.visibility === "string" && input.visibility.trim()) {
        query.set("visibility", input.visibility);
      }
      if (typeof input.page === "number" && Number.isFinite(input.page) && input.page > 0) {
        query.set("page", String(Math.trunc(input.page)));
      }
      if (typeof input.pageSize === "number" && Number.isFinite(input.pageSize) && input.pageSize > 0) {
        query.set("pageSize", String(Math.trunc(input.pageSize)));
      }
      const queryText = query.toString();
      const path = queryText ? `/admin/spaces?${queryText}` : "/admin/spaces";
      const result = await request<AdminSpaceListResult>(path);
      return {
        ...result,
        items: result.items.map((space) => ({
          ...space,
          // 关键分支：列表页封面 URL 可能是相对路径，这里统一补全。
          cover: normalizeAdminSpaceCover(space.cover, options.baseUrl)
        }))
      };
    },
    async updateSpaceStatus(input: { spaceId: string; status: "active" | "banned"; reason?: string }) {
      const targetSpaceID = input.spaceId.trim();
      if (!targetSpaceID) {
        throw new Error("空间 ID 不能为空");
      }
      const operationToken = await issueAdminOperationToken({
        operation: "space.update_status",
        targetType: "space",
        targetId: targetSpaceID
      });

      const space = await request<AdminSpace>(
        `/admin/spaces/${encodeURIComponent(targetSpaceID)}/status`,
        {
          method: "PATCH",
          headers: buildAdminOperationTokenHeaders(operationToken),
          body: JSON.stringify({
            status: input.status,
            reason: input.reason ?? ""
          })
        }
      );
      return {
        ...space,
        cover: normalizeAdminSpaceCover(space.cover, options.baseUrl)
      };
    },
    async updateSpaceMetadata(input: {
      spaceId: string;
      name?: string;
      description?: string;
      categoryId?: string;
      visibility?: "public" | "authenticated" | "member";
      coverAssetId?: string;
    }) {
      const targetSpaceID = input.spaceId.trim();
      if (!targetSpaceID) {
        throw new Error("空间 ID 不能为空");
      }

      const payload: Record<string, string> = {};
      if (typeof input.name === "string") {
        payload.name = input.name.trim();
      }
      // 允许传空字符串用于清空简介或封面。
      if (typeof input.description === "string") {
        payload.description = input.description.trim();
      }
      if (typeof input.categoryId === "string") {
        payload.categoryId = input.categoryId.trim();
      }
      if (typeof input.visibility === "string" && input.visibility.trim()) {
        payload.visibility = input.visibility;
      }
      if (typeof input.coverAssetId === "string") {
        payload.coverAssetId = input.coverAssetId.trim();
      }

      const space = await request<AdminSpace>(
        `/admin/spaces/${encodeURIComponent(targetSpaceID)}/metadata`,
        {
          method: "PATCH",
          body: JSON.stringify(payload)
        }
      );
      return {
        ...space,
        cover: normalizeAdminSpaceCover(space.cover, options.baseUrl)
      };
    },
    async listSpaceMembers(spaceId: string) {
      const targetSpaceID = spaceId.trim();
      if (!targetSpaceID) {
        throw new Error("空间 ID 不能为空");
      }
      const payload = await request<{ items: AdminSpaceMember[] }>(
        `/admin/spaces/${encodeURIComponent(targetSpaceID)}/members`
      );
      return Array.isArray(payload.items) ? payload.items : [];
    },
    async addSpaceMember(input: {
      spaceId: string;
      targetUserId?: string;
      targetEmail?: string;
      role: "collaborator" | "reader";
    }) {
      const targetSpaceID = input.spaceId.trim();
      if (!targetSpaceID) {
        throw new Error("空间 ID 不能为空");
      }
      const role = input.role;
      if (role !== "collaborator" && role !== "reader") {
        throw new Error("成员角色非法");
      }

      const targetUserID = (input.targetUserId ?? "").trim();
      const targetEmail = (input.targetEmail ?? "").trim();
      if (!targetUserID && !targetEmail) {
        throw new Error("成员目标不能为空");
      }

      return request<AdminSpaceMember>(`/admin/spaces/${encodeURIComponent(targetSpaceID)}/members`, {
        method: "POST",
        body: JSON.stringify({
          targetUserId: targetUserID,
          targetEmail,
          role
        })
      });
    },
    async updateSpaceMemberRole(input: {
      spaceId: string;
      userId: string;
      role: "collaborator" | "reader";
    }) {
      const targetSpaceID = input.spaceId.trim();
      if (!targetSpaceID) {
        throw new Error("空间 ID 不能为空");
      }
      const memberUserID = input.userId.trim();
      if (!memberUserID) {
        throw new Error("成员用户 ID 不能为空");
      }
      const role = input.role;
      if (role !== "collaborator" && role !== "reader") {
        throw new Error("成员角色非法");
      }

      return request<AdminSpaceMember>(
        `/admin/spaces/${encodeURIComponent(targetSpaceID)}/members/${encodeURIComponent(memberUserID)}`,
        {
          method: "PATCH",
          body: JSON.stringify({ role })
        }
      );
    },
    async removeSpaceMember(input: { spaceId: string; userId: string }) {
      const targetSpaceID = input.spaceId.trim();
      if (!targetSpaceID) {
        throw new Error("空间 ID 不能为空");
      }
      const memberUserID = input.userId.trim();
      if (!memberUserID) {
        throw new Error("成员用户 ID 不能为空");
      }

      await request<void>(
        `/admin/spaces/${encodeURIComponent(targetSpaceID)}/members/${encodeURIComponent(memberUserID)}`,
        {
          method: "DELETE"
        }
      );
    },
    async transferSpaceOwnership(input: { spaceId: string; targetUserId?: string; targetEmail?: string }) {
      const targetSpaceID = input.spaceId.trim();
      if (!targetSpaceID) {
        throw new Error("空间 ID 不能为空");
      }
      // 高风险操作：转让空间归属，需要签发一次性操作令牌。
      const targetUserId = (input.targetUserId ?? "").trim();
      const targetEmail = (input.targetEmail ?? "").trim();
      if (!targetUserId && !targetEmail) {
        throw new Error("转让目标不能为空");
      }
      const operationToken = await issueAdminOperationToken({
        operation: "space.transfer",
        targetType: "space",
        targetId: targetSpaceID
      });
      const payload: Record<string, string> = {};
      if (targetUserId) {
        payload.targetUserId = targetUserId;
      }
      if (targetEmail) {
        payload.targetEmail = targetEmail;
      }
      const space = await request<AdminSpace>(
        `/admin/spaces/${encodeURIComponent(targetSpaceID)}/transfer`,
        {
          method: "POST",
          headers: buildAdminOperationTokenHeaders(operationToken),
          body: JSON.stringify(payload)
        }
      );
      return {
        ...space,
        cover: normalizeAdminSpaceCover(space.cover, options.baseUrl)
      };
    },
    async deleteSpace(spaceId: string) {
      const targetSpaceID = spaceId.trim();
      if (!targetSpaceID) {
        throw new Error("空间 ID 不能为空");
      }
      const operationToken = await issueAdminOperationToken({
        operation: "space.delete",
        targetType: "space",
        targetId: targetSpaceID
      });
      await request<void>(`/admin/spaces/${encodeURIComponent(targetSpaceID)}`, {
        method: "DELETE",
        headers: buildAdminOperationTokenHeaders(operationToken)
      });
    },
    async listDocuments(input: AdminDocumentListInput = {}) {
      const query = new URLSearchParams();
      if (typeof input.keyword === "string" && input.keyword.trim()) {
        query.set("keyword", input.keyword.trim());
      }
      if (typeof input.spaceId === "string" && input.spaceId.trim()) {
        query.set("spaceId", input.spaceId.trim());
      }
      if (typeof input.status === "string" && input.status.trim()) {
        query.set("status", input.status);
      }
      if (typeof input.visibility === "string" && input.visibility.trim()) {
        query.set("visibility", input.visibility);
      }
      if (typeof input.page === "number" && Number.isFinite(input.page) && input.page > 0) {
        query.set("page", String(Math.trunc(input.page)));
      }
      if (typeof input.pageSize === "number" && Number.isFinite(input.pageSize) && input.pageSize > 0) {
        query.set("pageSize", String(Math.trunc(input.pageSize)));
      }

      const queryText = query.toString();
      const path = queryText ? `/admin/documents?${queryText}` : "/admin/documents";
      return request<AdminDocumentListResult>(path);
    },
    async updateDocumentStatus(input: { documentId: string; status: "active" | "banned"; reason?: string }) {
      const targetDocumentID = input.documentId.trim();
      if (!targetDocumentID) {
        throw new Error("文档 ID 不能为空");
      }
      const operationToken = await issueAdminOperationToken({
        operation: "document.update_status",
        targetType: "document",
        targetId: targetDocumentID
      });

      return request<AdminDocument>(
        `/admin/documents/${encodeURIComponent(targetDocumentID)}/status`,
        {
          method: "PATCH",
          headers: buildAdminOperationTokenHeaders(operationToken),
          body: JSON.stringify({
            status: input.status,
            reason: input.reason ?? ""
          })
        }
      );
    },
    async deleteDocument(documentId: string) {
      const targetDocumentID = documentId.trim();
      if (!targetDocumentID) {
        throw new Error("文档 ID 不能为空");
      }
      const operationToken = await issueAdminOperationToken({
        operation: "document.delete",
        targetType: "document",
        targetId: targetDocumentID
      });
      await request<void>(`/admin/documents/${encodeURIComponent(targetDocumentID)}`, {
        method: "DELETE",
        headers: buildAdminOperationTokenHeaders(operationToken)
      });
    },
    async listDocumentAttachments(input: AdminDocumentAttachmentListInput = {}) {
      const query = new URLSearchParams();
      if (typeof input.keyword === "string" && input.keyword.trim()) {
        query.set("keyword", input.keyword.trim());
      }
      if (typeof input.spaceId === "string" && input.spaceId.trim()) {
        query.set("spaceId", input.spaceId.trim());
      }
      if (typeof input.documentId === "string" && input.documentId.trim()) {
        query.set("documentId", input.documentId.trim());
      }
      if (typeof input.status === "string" && input.status.trim()) {
        query.set("status", input.status);
      }
      if (typeof input.storageProvider === "string" && input.storageProvider.trim()) {
        query.set("storageProvider", input.storageProvider);
      }
      if (typeof input.page === "number" && Number.isFinite(input.page) && input.page > 0) {
        query.set("page", String(Math.trunc(input.page)));
      }
      if (typeof input.pageSize === "number" && Number.isFinite(input.pageSize) && input.pageSize > 0) {
        query.set("pageSize", String(Math.trunc(input.pageSize)));
      }

      const queryText = query.toString();
      const path = queryText ? `/admin/document-attachments?${queryText}` : "/admin/document-attachments";
      return request<AdminDocumentAttachmentListResult>(path);
    },
    async deleteDocumentAttachment(input: {
      attachmentId: string;
      physicalDelete?: boolean;
      forcePhysicalDeleteOnShare?: boolean;
    }) {
      const targetAttachmentID = input.attachmentId.trim();
      if (!targetAttachmentID) {
        throw new Error("附件 ID 不能为空");
      }
      const operationToken = await issueAdminOperationToken({
        operation: "document_attachment.delete",
        targetType: "document_attachment",
        targetId: targetAttachmentID
      });
      const query = new URLSearchParams();
      if (input.physicalDelete === true) {
        query.set("physicalDelete", "true");
      }
      if (input.forcePhysicalDeleteOnShare === true) {
        query.set("forcePhysicalDeleteOnShare", "true");
      }
      const queryText = query.toString();
      const path =
        `/admin/document-attachments/${encodeURIComponent(targetAttachmentID)}` + (queryText ? `?${queryText}` : "");
      return request<AdminDocumentAttachmentDeleteResult>(path, {
        method: "DELETE",
        headers: buildAdminOperationTokenHeaders(operationToken)
      });
    },
    async listThemes() {
      return request<AdminTheme[]>("/admin/themes");
    },
    async createTheme(input) {
      const themeID = input.themeId.trim();
      if (!themeID) {
        throw new Error("主题 ID 不能为空");
      }
      const themeName = input.name.trim();
      if (!themeName) {
        throw new Error("主题名称不能为空");
      }

      return request<AdminTheme>("/admin/themes", {
        method: "POST",
        body: JSON.stringify({
          themeId: themeID,
          name: themeName,
          description: input.description ?? "",
          variables: input.variables ?? {},
          syntaxTheme: input.syntaxTheme ?? "one-light",
          codeBlockStyle: input.codeBlockStyle ?? {},
          codeBlockCodeStyle: input.codeBlockCodeStyle ?? {},
          inlineCodeStyle: input.inlineCodeStyle ?? {},
          customCss: input.customCss ?? "",
          enabled: input.enabled ?? true
        })
      });
    },
    async updateTheme(input) {
      const themeID = input.themeId.trim();
      if (!themeID) {
        throw new Error("主题 ID 不能为空");
      }
      const operationToken = await issueAdminOperationToken({
        operation: "theme.update",
        targetType: "theme",
        targetId: themeID
      });
      const payload: Record<string, unknown> = {};
      if (typeof input.name === "string") {
        payload.name = input.name.trim();
      }
      if (typeof input.description === "string") {
        payload.description = input.description;
      }
      if (input.variables && typeof input.variables === "object") {
        payload.variables = input.variables;
      }
      if (typeof input.syntaxTheme === "string") {
        payload.syntaxTheme = input.syntaxTheme;
      }
      if (input.codeBlockStyle && typeof input.codeBlockStyle === "object") {
        payload.codeBlockStyle = input.codeBlockStyle;
      }
      if (input.codeBlockCodeStyle && typeof input.codeBlockCodeStyle === "object") {
        payload.codeBlockCodeStyle = input.codeBlockCodeStyle;
      }
      if (input.inlineCodeStyle && typeof input.inlineCodeStyle === "object") {
        payload.inlineCodeStyle = input.inlineCodeStyle;
      }
      if (typeof input.customCss === "string") {
        payload.customCss = input.customCss;
      }
      if (typeof input.enabled === "boolean") {
        payload.enabled = input.enabled;
      }
      return request<AdminTheme>(`/admin/themes/${encodeURIComponent(themeID)}`, {
        method: "PUT",
        headers: buildAdminOperationTokenHeaders(operationToken),
        body: JSON.stringify(payload)
      });
    },
    async deleteTheme(themeId: string) {
      const themeID = themeId.trim();
      if (!themeID) {
        throw new Error("主题 ID 不能为空");
      }
      const operationToken = await issueAdminOperationToken({
        operation: "theme.delete",
        targetType: "theme",
        targetId: themeID
      });
      await request<void>(`/admin/themes/${encodeURIComponent(themeID)}`, {
        method: "DELETE",
        headers: buildAdminOperationTokenHeaders(operationToken)
      });
    },
    async listSystemConfigs() {
      return request<AdminSystemConfig[]>("/admin/system-configs");
    },
    async upsertSystemConfig(input: {
      configKey: "site" | "editor" | "security" | "auth" | "image-hosting" | "sitemap" | "data-retention";
      value: Record<string, unknown>;
      expectedVersion?: number;
    }) {
      const configKey = input.configKey.trim();
      if (!configKey) {
        throw new Error("配置键不能为空");
      }
      const operationToken = await issueAdminOperationToken({
        operation: "system_config.upsert",
        targetType: "system_config",
        targetId: configKey
      });
      const payload: {
        value: Record<string, unknown>;
        expectedVersion?: number;
      } = { value: input.value ?? {} };
      if (
        typeof input.expectedVersion === "number" &&
        Number.isFinite(input.expectedVersion) &&
        input.expectedVersion >= 0
      ) {
        payload.expectedVersion = Math.trunc(input.expectedVersion);
      }
      return request<AdminSystemConfig>(`/admin/system-configs/${encodeURIComponent(configKey)}`, {
        method: "PUT",
        headers: buildAdminOperationTokenHeaders(operationToken),
        body: JSON.stringify(payload)
      });
    },
    async runDataRetentionCleanup() {
      const operationToken = await issueAdminOperationToken({
        operation: "system_config.cleanup",
        targetType: "system_config",
        targetId: "data-retention"
      });
      return request<AdminDataRetentionCleanupResult>("/admin/system-configs/data-retention/cleanup/run", {
        method: "POST",
        headers: buildAdminOperationTokenHeaders(operationToken)
      });
    },
    async testAuthLDAPConnection(input: { value: Record<string, unknown>; providerId?: string }) {
      const payload: { value: Record<string, unknown>; providerId?: string } = {
        value: input.value ?? {}
      };
      if (typeof input.providerId === "string" && input.providerId.trim()) {
        payload.providerId = input.providerId.trim();
      }
      return request<{ ok: boolean }>("/admin/system-configs/auth/providers/ldap/test", {
        method: "POST",
        body: JSON.stringify(payload)
      });
    },
    async listAudits(input: AdminAuditListInput = {}) {
      const query = new URLSearchParams();
      if (typeof input.keyword === "string" && input.keyword.trim()) {
        query.set("keyword", input.keyword.trim());
      }
      if (typeof input.module === "string" && input.module !== "all" && input.module.trim()) {
        query.set("module", input.module.trim());
      }
      if (typeof input.action === "string" && input.action !== "all" && input.action.trim()) {
        query.set("action", input.action.trim());
      }
      if (typeof input.actorUserId === "string" && input.actorUserId.trim()) {
        query.set("actorUserId", input.actorUserId.trim());
      }
      if (typeof input.targetType === "string" && input.targetType.trim()) {
        query.set("targetType", input.targetType.trim());
      }
      if (typeof input.targetId === "string" && input.targetId.trim()) {
        query.set("targetId", input.targetId.trim());
      }
      if (typeof input.requestId === "string" && input.requestId.trim()) {
        query.set("requestId", input.requestId.trim());
      }
      if (typeof input.from === "string" && input.from.trim()) {
        query.set("from", input.from.trim());
      }
      if (typeof input.to === "string" && input.to.trim()) {
        query.set("to", input.to.trim());
      }
      if (typeof input.page === "number" && Number.isFinite(input.page) && input.page > 0) {
        query.set("page", String(Math.trunc(input.page)));
      }
      if (typeof input.pageSize === "number" && Number.isFinite(input.pageSize) && input.pageSize > 0) {
        query.set("pageSize", String(Math.trunc(input.pageSize)));
      }
      const queryText = query.toString();
      const path = queryText ? `/admin/audits?${queryText}` : "/admin/audits";
      return request<AdminAuditListResult>(path);
    }
  };

  const imageHosting: ImageHostingGateway = {
    async getConfig() {
      return request<Record<string, unknown>>("/image-hosting");
    },
    async uploadLocalImage(
      file: File,
      uploadEndpoint?: string,
      uploadOptions?: { spaceId?: string | null }
    ) {
      const formData = new FormData();
      formData.append("file", file);
      const spaceID = typeof uploadOptions?.spaceId === "string" ? uploadOptions.spaceId.trim() : "";
      if (spaceID) {
        formData.append("spaceId", spaceID);
      }
      const uploaded = await request<UploadLocalImageResult>(
        normalizeUploadEndpoint(uploadEndpoint, options.baseUrl),
        {
          method: "POST",
          body: formData
        }
      );
      const rawUrl = typeof uploaded.url === "string" ? uploaded.url.trim() : "";
      return {
        ...uploaded,
        url: resolveBackendPublicUrl(rawUrl, options.baseUrl)
      };
    }
  };

  // 远端驱动下仍复用本地 user_config：用于保存本机偏好配置。
  const userConfig = createIndexedDbUserConfigGateway();

  return {
    auth,
    workspace,
    document,
    theme,
    admin,
    imageHosting,
    userConfig
  };
}
