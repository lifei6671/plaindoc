import {
  type AdminAuditListInput,
  type AdminAuditListResult,
  type AdminGateway,
  type AdminSystemConfig,
  type AdminTheme,
  type AdminDocument,
  type AdminDocumentListInput,
  type AdminDocumentListResult,
  type AdminIdentity,
  type AdminSpace,
  type AdminSpaceCover,
  type AdminSpaceListInput,
  type AdminSpaceListResult,
  type AdminRole,
  type AdminUser,
  type AdminUserListInput,
  type AdminUserListResult,
  type AuthSession,
  type AuthGateway,
  ConflictError,
  type CreateNodeInput,
  type CreateNodeResult,
  type CreateSpaceInput,
  type DataGateway,
  type Document,
  type DocumentGateway,
  type DocumentRevision,
  type ImageHostingGateway,
  type SaveDocumentInput,
  type SaveDocumentResult,
  type Space,
  type Theme,
  type ThemeGateway,
  type TreeNode,
  type UploadLocalImageResult,
  type UpdateNodeInput,
  type WorkspaceGateway
} from "../types";
import { createIndexedDbUserConfigGateway } from "../user-config/indexeddb-gateway";

interface HttpAdapterOptions {
  baseUrl: string;
}

const ACCESS_TOKEN_STORAGE_KEY = "plaindoc.auth.access-token";
const REFRESH_TOKEN_STORAGE_KEY = "plaindoc.auth.refresh-token";

interface HttpAuthSession extends AuthSession {
  refreshToken?: string;
}

class HttpRequestError extends Error {
  readonly status: number;
  readonly body: string;

  constructor(status: number, body: string) {
    super(body || `Request failed: ${status}`);
    this.name = "HttpRequestError";
    this.status = status;
    this.body = body;
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

function toRequestError(status: number, body: string): HttpRequestError {
  return new HttpRequestError(status, body);
}

function isAbsoluteUrl(value: string): boolean {
  return /^https?:\/\//i.test(value.trim());
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

export function createHttpAdapter(options: HttpAdapterOptions): DataGateway {
  let accessToken: string | null = readStoredValue(ACCESS_TOKEN_STORAGE_KEY);
  let refreshToken: string | null = readStoredValue(REFRESH_TOKEN_STORAGE_KEY);

  const clearStoredTokens = () => {
    accessToken = null;
    refreshToken = null;
    removeStoredValue(ACCESS_TOKEN_STORAGE_KEY);
    removeStoredValue(REFRESH_TOKEN_STORAGE_KEY);
  };

  const saveSessionTokens = (session: HttpAuthSession) => {
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
      headers
    });

    if (response.status === 409) {
      const payload = (await response.json()) as { latestDocument: Document };
      throw new ConflictError(payload.latestDocument);
    }

    if (response.status === 401 && !skipAuth && retryOnUnauthorized) {
      const refreshed = await refreshSession();
      if (refreshed) {
        return request<T>(path, init, {
          skipAuth: false,
          retryOnUnauthorized: false
        });
      }
    }

    if (!response.ok) {
      const message = await response.text();
      throw toRequestError(response.status, message);
    }

    if (response.status === 204) {
      return undefined as T;
    }

    return (await response.json()) as T;
  };

  const refreshSession = async (): Promise<boolean> => {
    if (!refreshToken) {
      return false;
    }
    try {
      const session = await request<HttpAuthSession>(
        "/auth/refresh",
        {
          method: "POST",
          body: JSON.stringify({ refreshToken })
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
    async getSession() {
      if (!accessToken) {
        return { user: null };
      }

      try {
        const session = await request<HttpAuthSession>("/auth/me");
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
        if (error instanceof HttpRequestError && error.status === 401) {
          clearStoredTokens();
          return { user: null };
        }
        throw error;
      }
    },
    async login(input) {
      const session = await request<HttpAuthSession>("/auth/login", {
        method: "POST",
        body: JSON.stringify(input)
      });
      saveSessionTokens(session);
      return {
        user: session.user,
        token: session.token
      };
    },
    async register(input) {
      const session = await request<HttpAuthSession>("/auth/register", {
        method: "POST",
        body: JSON.stringify(input)
      });
      saveSessionTokens(session);
      return {
        user: session.user,
        token: session.token
      };
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
    async listRevisions(docId: string) {
      return request<DocumentRevision[]>(`/docs/${docId}/revisions`);
    },
    async setDocumentTheme(docId: string, themeId: string) {
      return request<Document>(`/docs/${docId}/theme`, {
        method: "PUT",
        body: JSON.stringify({ themeId })
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
    async createSpace(input: { name: string; description?: string; visibility?: "public" | "authenticated" | "member"; coverAssetId?: string }) {
      const name = input.name.trim();
      if (!name) {
        throw new Error("空间名称不能为空");
      }
      return request<AdminSpace>("/admin/spaces", {
        method: "POST",
        body: JSON.stringify({
          name,
          description: input.description ?? "",
          visibility: input.visibility ?? "member",
          coverAssetId: input.coverAssetId ?? ""
        })
      });
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

      return request<AdminSpaceCover>("/admin/spaces/cover-assets", {
        method: "POST",
        body: formData
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
      return request<AdminSpaceListResult>(path);
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

      return request<AdminSpace>(
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
    },
    async updateSpaceMetadata(input: { spaceId: string; name?: string; visibility?: "public" | "authenticated" | "member" }) {
      const targetSpaceID = input.spaceId.trim();
      if (!targetSpaceID) {
        throw new Error("空间 ID 不能为空");
      }

      const payload: Record<string, string> = {};
      if (typeof input.name === "string") {
        payload.name = input.name.trim();
      }
      if (typeof input.visibility === "string" && input.visibility.trim()) {
        payload.visibility = input.visibility;
      }

      return request<AdminSpace>(
        `/admin/spaces/${encodeURIComponent(targetSpaceID)}/metadata`,
        {
          method: "PATCH",
          body: JSON.stringify(payload)
        }
      );
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
      configKey: "site" | "editor" | "security" | "image-hosting";
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
    async uploadLocalImage(file: File, uploadEndpoint?: string) {
      const formData = new FormData();
      formData.append("file", file);
      return request<UploadLocalImageResult>(
        normalizeUploadEndpoint(uploadEndpoint, options.baseUrl),
        {
          method: "POST",
          body: formData
        }
      );
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
