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
import { buildTree, createLocalId, useDatabase } from "./store";

function nowIso(): string {
  return new Date().toISOString();
}

const authGateway: AuthGateway = {
  async getSession() {
    return useDatabase((db) => {
      const user = db.sessionUserId ? db.users[db.sessionUserId] : null;
      if (!user) {
        return { user: null };
      }
      return {
        user: {
          id: user.id,
          email: user.email,
          name: user.name
        }
      };
    });
  },
  async login(input) {
    return useDatabase((db) => {
      const user = Object.values(db.users).find(
        (entry) => entry.email === input.email && entry.password === input.password
      );
      if (!user) {
        throw new Error("账号或密码错误");
      }
      db.sessionUserId = user.id;
      return {
        user: {
          id: user.id,
          email: user.email,
          name: user.name
        }
      };
    });
  },
  async register(input) {
    return useDatabase((db) => {
      const exists = Object.values(db.users).some((entry) => entry.email === input.email);
      if (exists) {
        throw new Error("邮箱已被使用");
      }
      const id = createLocalId("user");
      db.users[id] = {
        id,
        email: input.email,
        name: input.name,
        password: input.password
      };
      db.sessionUserId = id;
      return {
        user: {
          id,
          email: input.email,
          name: input.name
        }
      };
    });
  },
  async logout() {
    useDatabase((db) => {
      db.sessionUserId = null;
    });
  }
};

const workspaceGateway: WorkspaceGateway = {
  async listSpaces(): Promise<Space[]> {
    return useDatabase((db) =>
      Object.values(db.spaces).sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))
    );
  },

  async createSpace(input: CreateSpaceInput): Promise<Space> {
    return useDatabase((db) => {
      const now = nowIso();
      const id = createLocalId("space");
      const space: Space = {
        id,
        name: input.name,
        createdAt: now,
        updatedAt: now
      };
      db.spaces[id] = space;
      return space;
    });
  },

  async getTree(spaceId: string): Promise<TreeNode[]> {
    return useDatabase((db) => {
      const nodes = Object.values(db.nodes).filter((node) => node.spaceId === spaceId);
      return buildTree(nodes, null);
    });
  },

  async createNode(input: CreateNodeInput): Promise<CreateNodeResult> {
    return useDatabase((db) => {
      const siblings = Object.values(db.nodes).filter(
        (node) => node.spaceId === input.spaceId && node.parentId === input.parentId
      );
      const maxSort = siblings.reduce((max, item) => Math.max(max, item.sort), 0);
      const id = createLocalId(input.type === "doc" ? "doc" : "node");
      const now = nowIso();

      db.nodes[id] = {
        id,
        spaceId: input.spaceId,
        parentId: input.parentId,
        type: input.type,
        title: input.title,
        sort: maxSort + 1,
        createdAt: now,
        updatedAt: now
      };

      const space = db.spaces[input.spaceId];
      if (space) {
        space.updatedAt = now;
      }

      if (input.type === "doc") {
        const document: Document = {
          id,
          nodeId: id,
          title: input.title,
          contentMd: "",
          version: 1,
          updatedAt: now
        };
        const revision: DocumentRevision = {
          id: createLocalId("rev"),
          documentId: id,
          version: 1,
          contentMd: "",
          baseVersion: 0,
          createdAt: now,
          source: "local"
        };
        db.documents[id] = document;
        db.revisions[id] = [revision];
        return { nodeId: id, docId: id };
      }

      return { nodeId: id };
    });
  },

  async updateNode(input: UpdateNodeInput): Promise<void> {
    useDatabase((db) => {
      const node = db.nodes[input.nodeId];
      if (!node) {
        throw new Error("目录节点不存在");
      }
      if (typeof input.title === "string") {
        node.title = input.title;
      }
      if ("parentId" in input) {
        node.parentId = input.parentId ?? null;
      }
      if (typeof input.sort === "number") {
        node.sort = input.sort;
      }
      node.updatedAt = nowIso();

      if (node.type === "doc" && db.documents[node.id] && typeof input.title === "string") {
        db.documents[node.id].title = input.title;
      }
    });
  },

  async deleteNode(nodeId: string): Promise<void> {
    useDatabase((db) => {
      const removeSubtree = (id: string) => {
        const children = Object.values(db.nodes).filter((node) => node.parentId === id);
        for (const child of children) {
          removeSubtree(child.id);
        }
        const current = db.nodes[id];
        if (!current) {
          return;
        }
        if (current.type === "doc") {
          delete db.documents[id];
          delete db.revisions[id];
        }
        delete db.nodes[id];
      };
      removeSubtree(nodeId);
    });
  }
};

const documentGateway: DocumentGateway = {
  async getDocument(docId: string): Promise<Document> {
    return useDatabase((db) => {
      const document = db.documents[docId];
      if (!document) {
        throw new Error("文档不存在");
      }
      return { ...document };
    });
  },

  async saveDocument(input: SaveDocumentInput): Promise<SaveDocumentResult> {
    return useDatabase((db) => {
      const doc = db.documents[input.docId];
      if (!doc) {
        throw new Error("文档不存在");
      }
      if (doc.version !== input.baseVersion) {
        throw new ConflictError({ ...doc });
      }

      const now = nowIso();
      const nextVersion = doc.version + 1;
      doc.contentMd = input.contentMd;
      doc.version = nextVersion;
      doc.updatedAt = now;

      const node = db.nodes[input.docId];
      if (node) {
        node.updatedAt = now;
      }

      const revisions = db.revisions[input.docId] ?? [];
      revisions.unshift({
        id: createLocalId("rev"),
        documentId: input.docId,
        version: nextVersion,
        contentMd: input.contentMd,
        baseVersion: input.baseVersion,
        createdAt: now,
        source: "local"
      });
      db.revisions[input.docId] = revisions.slice(0, 100);

      return {
        document: { ...doc }
      };
    });
  },

  async listRevisions(docId: string): Promise<DocumentRevision[]> {
    return useDatabase((db) => {
      const revisions = db.revisions[docId] ?? [];
      return revisions.map((item) => ({ ...item }));
    });
  }
};

export function createLocalAdapter(): DataGateway {
  return {
    auth: authGateway,
    workspace: workspaceGateway,
    document: documentGateway
  };
}
