import {
  type AuthSession,
  ConflictError,
  type AuthGateway,
  type CreateNodeInput,
  type CreateNodeResult,
  type CreateSpaceInput,
  type DataGateway,
  type Document,
  type DocumentGateway,
  type DocumentRevision,
  type SaveDocumentInput,
  type SaveDocumentResult,
  type Space,
  type Theme,
  type ThemeGateway,
  type TreeNode,
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
    headers.set("Content-Type", "application/json");
    if (!skipAuth && accessToken) {
      headers.set("Authorization", `Bearer ${accessToken}`);
    }

    const response = await fetch(`${options.baseUrl}${path}`, {
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

  // 远端驱动下仍复用本地 user_config：用于保存本机偏好配置（如图床参数）。
  const userConfig = createIndexedDbUserConfigGateway();

  return {
    auth,
    workspace,
    document,
    theme,
    userConfig
  };
}
