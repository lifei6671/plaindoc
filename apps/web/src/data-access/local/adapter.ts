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
import { createIndexedDbUserConfigGateway } from "../user-config/indexeddb-gateway";
import {
  LOCAL_SESSION_USER_META_KEY,
  buildTree,
  createLocalId,
  mapLocalDocument,
  mapLocalRevision,
  mapLocalSpace,
  mapLocalUser,
  useDatabase,
  useDatabaseTransaction
} from "./store";

function nowIso(): string {
  return new Date().toISOString();
}

const authGateway: AuthGateway = {
  async getSession() {
    return useDatabase(async (database) => {
      const sessionMeta = await database.metaTable.get(LOCAL_SESSION_USER_META_KEY);
      if (!sessionMeta?.value) {
        return { user: null };
      }
      const user = await database.usersTable.where("ulid").equals(sessionMeta.value).first();
      if (!user) {
        return { user: null };
      }
      return { user: mapLocalUser(user) };
    });
  },

  async login(input) {
    return useDatabaseTransaction("rw", async (database) => {
      const user = await database.usersTable.where("email").equals(input.email).first();
      if (!user || user.password !== input.password) {
        throw new Error("账号或密码错误");
      }
      await database.metaTable.put({
        key: LOCAL_SESSION_USER_META_KEY,
        value: user.ulid,
        updatedAt: nowIso()
      });
      return { user: mapLocalUser(user) };
    });
  },

  async register(input) {
    const ulid = await createLocalId("user");
    return useDatabaseTransaction("rw", async (database) => {
      const exists = await database.usersTable.where("email").equals(input.email).count();
      if (exists > 0) {
        throw new Error("邮箱已被使用");
      }

      const now = nowIso();
      await database.usersTable.add({
        ulid,
        email: input.email,
        name: input.name,
        password: input.password,
        createdAt: now,
        updatedAt: now
      });
      await database.metaTable.put({
        key: LOCAL_SESSION_USER_META_KEY,
        value: ulid,
        updatedAt: now
      });

      return {
        user: {
          id: ulid,
          email: input.email,
          name: input.name
        }
      };
    });
  },

  async logout() {
    await useDatabaseTransaction("rw", async (database) => {
      await database.metaTable.delete(LOCAL_SESSION_USER_META_KEY);
    });
  }
};

const workspaceGateway: WorkspaceGateway = {
  async listSpaces(): Promise<Space[]> {
    return useDatabase(async (database) => {
      const spaces = await database.spacesTable.toArray();
      return spaces
        .map(mapLocalSpace)
        .sort((left, right) => right.updatedAt.localeCompare(left.updatedAt));
    });
  },

  async createSpace(input: CreateSpaceInput): Promise<Space> {
    const ulid = await createLocalId("space");
    return useDatabaseTransaction("rw", async (database) => {
      const now = nowIso();
      const record = {
        ulid,
        name: input.name,
        createdAt: now,
        updatedAt: now
      };
      await database.spacesTable.add(record);
      return mapLocalSpace(record);
    });
  },

  async getTree(spaceId: string): Promise<TreeNode[]> {
    return useDatabase(async (database) => {
      const nodes = await database.nodesTable.where("spaceUlid").equals(spaceId).toArray();
      return buildTree(nodes, null);
    });
  },

  async createNode(input: CreateNodeInput): Promise<CreateNodeResult> {
    const nodeUlid = await createLocalId("node");
    const revisionUlid = input.type === "doc" ? await createLocalId("revision") : null;
    return useDatabaseTransaction("rw", async (database) => {
      if (input.parentId) {
        const parentNode = await database.nodesTable.where("ulid").equals(input.parentId).first();
        if (!parentNode) {
          throw new Error("父级目录节点不存在");
        }
        if (parentNode.spaceUlid !== input.spaceId) {
          throw new Error("父级目录节点不在当前空间");
        }
      }

      const siblings = (await database.nodesTable.where("spaceUlid").equals(input.spaceId).toArray()).filter(
        (node) => node.parentUlid === input.parentId
      );
      const maxSort = siblings.reduce((max, item) => Math.max(max, item.sort), 0);

      const now = nowIso();
      await database.nodesTable.add({
        ulid: nodeUlid,
        spaceUlid: input.spaceId,
        parentUlid: input.parentId,
        type: input.type,
        title: input.title,
        sort: maxSort + 1,
        createdAt: now,
        updatedAt: now
      });

      const space = await database.spacesTable.where("ulid").equals(input.spaceId).first();
      if (typeof space?.id === "number") {
        await database.spacesTable.update(space.id, { updatedAt: now });
      }

      if (input.type === "doc") {
        if (!revisionUlid) {
          throw new Error("文档修订 ID 生成失败");
        }
        await database.documentsTable.add({
          ulid: nodeUlid,
          nodeUlid,
          title: input.title,
          contentMd: "",
          version: 1,
          createdAt: now,
          updatedAt: now
        });
        await database.revisionsTable.add({
          ulid: revisionUlid,
          documentUlid: nodeUlid,
          version: 1,
          contentMd: "",
          baseVersion: 0,
          createdAt: now,
          source: "local"
        });
        return { nodeId: nodeUlid, docId: nodeUlid };
      }

      return { nodeId: nodeUlid };
    });
  },

  async updateNode(input: UpdateNodeInput): Promise<void> {
    await useDatabaseTransaction("rw", async (database) => {
      const node = await database.nodesTable.where("ulid").equals(input.nodeId).first();
      if (!node || typeof node.id !== "number") {
        throw new Error("目录节点不存在");
      }

      if ("parentId" in input && input.parentId) {
        const parentNode = await database.nodesTable.where("ulid").equals(input.parentId).first();
        if (!parentNode) {
          throw new Error("父级目录节点不存在");
        }
        if (parentNode.spaceUlid !== node.spaceUlid) {
          throw new Error("父级目录节点不在当前空间");
        }
      }

      const now = nowIso();
      await database.nodesTable.update(node.id, {
        title: typeof input.title === "string" ? input.title : node.title,
        parentUlid: "parentId" in input ? input.parentId ?? null : node.parentUlid,
        sort: typeof input.sort === "number" ? input.sort : node.sort,
        updatedAt: now
      });

      if (node.type === "doc" && typeof input.title === "string") {
        const document = await database.documentsTable.where("ulid").equals(node.ulid).first();
        if (typeof document?.id === "number") {
          await database.documentsTable.update(document.id, {
            title: input.title,
            updatedAt: now
          });
        }
      }

      const space = await database.spacesTable.where("ulid").equals(node.spaceUlid).first();
      if (typeof space?.id === "number") {
        await database.spacesTable.update(space.id, { updatedAt: now });
      }
    });
  },

  async deleteNode(nodeId: string): Promise<void> {
    await useDatabaseTransaction("rw", async (database) => {
      let affectedSpaceId: string | null = null;

      const removeSubtree = async (currentNodeId: string): Promise<void> => {
        const current = await database.nodesTable.where("ulid").equals(currentNodeId).first();
        if (!current) {
          return;
        }
        affectedSpaceId = current.spaceUlid;

        const children = await database.nodesTable.where("parentUlid").equals(currentNodeId).toArray();
        for (const child of children) {
          await removeSubtree(child.ulid);
        }

        if (current.type === "doc") {
          await database.documentsTable.where("ulid").equals(currentNodeId).delete();
          await database.revisionsTable.where("documentUlid").equals(currentNodeId).delete();
        }
        await database.nodesTable.where("ulid").equals(currentNodeId).delete();
      };

      await removeSubtree(nodeId);

      if (affectedSpaceId) {
        const space = await database.spacesTable.where("ulid").equals(affectedSpaceId).first();
        if (typeof space?.id === "number") {
          await database.spacesTable.update(space.id, { updatedAt: nowIso() });
        }
      }
    });
  }
};

const documentGateway: DocumentGateway = {
  async getDocument(docId: string): Promise<Document> {
    return useDatabase(async (database) => {
      const document = await database.documentsTable.where("ulid").equals(docId).first();
      if (!document) {
        throw new Error("文档不存在");
      }
      return mapLocalDocument(document);
    });
  },

  async saveDocument(input: SaveDocumentInput): Promise<SaveDocumentResult> {
    const revisionUlid = await createLocalId("revision");
    return useDatabaseTransaction("rw", async (database) => {
      const document = await database.documentsTable.where("ulid").equals(input.docId).first();
      if (!document || typeof document.id !== "number") {
        throw new Error("文档不存在");
      }

      if (document.version !== input.baseVersion) {
        throw new ConflictError(mapLocalDocument(document));
      }

      const now = nowIso();
      const nextVersion = document.version + 1;
      await database.documentsTable.update(document.id, {
        contentMd: input.contentMd,
        version: nextVersion,
        updatedAt: now
      });

      const node = await database.nodesTable.where("ulid").equals(document.nodeUlid).first();
      if (typeof node?.id === "number") {
        await database.nodesTable.update(node.id, { updatedAt: now });
        const space = await database.spacesTable.where("ulid").equals(node.spaceUlid).first();
        if (typeof space?.id === "number") {
          await database.spacesTable.update(space.id, { updatedAt: now });
        }
      }

      await database.revisionsTable.add({
        ulid: revisionUlid,
        documentUlid: document.ulid,
        version: nextVersion,
        contentMd: input.contentMd,
        baseVersion: input.baseVersion,
        createdAt: now,
        source: "local"
      });

      const allRevisions = await database.revisionsTable.where("documentUlid").equals(document.ulid).toArray();
      if (allRevisions.length > 100) {
        const staleRevisionIds = allRevisions
          .sort((left, right) => right.version - left.version || right.createdAt.localeCompare(left.createdAt))
          .slice(100)
          .map((revision) => revision.id)
          .filter((id): id is number => typeof id === "number");
        if (staleRevisionIds.length > 0) {
          await database.revisionsTable.bulkDelete(staleRevisionIds);
        }
      }

      const latest = await database.documentsTable.where("ulid").equals(input.docId).first();
      if (!latest) {
        throw new Error("文档不存在");
      }
      return {
        document: mapLocalDocument(latest)
      };
    });
  },

  async listRevisions(docId: string): Promise<DocumentRevision[]> {
    return useDatabase(async (database) => {
      const revisions = await database.revisionsTable.where("documentUlid").equals(docId).toArray();
      return revisions
        .sort((left, right) => right.version - left.version || right.createdAt.localeCompare(left.createdAt))
        .map(mapLocalRevision);
    });
  }
};

// 本地模式下，用户配置统一使用 IndexedDB user_config 表。
const userConfigGateway = createIndexedDbUserConfigGateway();

export function createLocalAdapter(): DataGateway {
  return {
    auth: authGateway,
    workspace: workspaceGateway,
    document: documentGateway,
    userConfig: userConfigGateway
  };
}
