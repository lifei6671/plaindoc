import {
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
  type TreeNode,
  type UpdateNodeInput,
  type WorkspaceGateway
} from "../types";

interface HttpAdapterOptions {
  baseUrl: string;
}

async function request<T>(baseUrl: string, path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${baseUrl}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {})
    }
  });

  if (response.status === 409) {
    const payload = (await response.json()) as { latestDocument: Document };
    throw new ConflictError(payload.latestDocument);
  }

  if (!response.ok) {
    const message = await response.text();
    throw new Error(message || `Request failed: ${response.status}`);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}

export function createHttpAdapter(options: HttpAdapterOptions): DataGateway {
  const auth: AuthGateway = {
    async getSession() {
      return request(options.baseUrl, "/auth/me");
    },
    async login(input) {
      return request(options.baseUrl, "/auth/login", {
        method: "POST",
        body: JSON.stringify(input)
      });
    },
    async register(input) {
      return request(options.baseUrl, "/auth/register", {
        method: "POST",
        body: JSON.stringify(input)
      });
    },
    async logout() {
      await request<void>(options.baseUrl, "/auth/logout", {
        method: "POST"
      });
    }
  };

  const workspace: WorkspaceGateway = {
    async listSpaces() {
      return request<Space[]>(options.baseUrl, "/spaces");
    },
    async createSpace(input: CreateSpaceInput) {
      return request<Space>(options.baseUrl, "/spaces", {
        method: "POST",
        body: JSON.stringify(input)
      });
    },
    async getTree(spaceId: string) {
      return request<TreeNode[]>(options.baseUrl, `/spaces/${spaceId}/tree`);
    },
    async createNode(input: CreateNodeInput) {
      return request<CreateNodeResult>(options.baseUrl, `/spaces/${input.spaceId}/nodes`, {
        method: "POST",
        body: JSON.stringify(input)
      });
    },
    async updateNode(input: UpdateNodeInput) {
      await request<void>(options.baseUrl, `/nodes/${input.nodeId}`, {
        method: "PATCH",
        body: JSON.stringify(input)
      });
    },
    async deleteNode(nodeId: string) {
      await request<void>(options.baseUrl, `/nodes/${nodeId}`, {
        method: "DELETE"
      });
    }
  };

  const document: DocumentGateway = {
    async getDocument(docId: string) {
      return request<Document>(options.baseUrl, `/docs/${docId}`);
    },
    async saveDocument(input: SaveDocumentInput) {
      return request<SaveDocumentResult>(options.baseUrl, `/docs/${input.docId}`, {
        method: "PUT",
        body: JSON.stringify(input)
      });
    },
    async listRevisions(docId: string) {
      return request<DocumentRevision[]>(options.baseUrl, `/docs/${docId}/revisions`);
    }
  };

  return {
    auth,
    workspace,
    document
  };
}
