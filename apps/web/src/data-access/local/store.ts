import Dexie, { type Table } from "dexie";
import type { Document, DocumentRevision, NodeType, Space, TreeNode, User } from "../types";
import { generateLowercaseUlid } from "./ulid";

const LOCAL_DB_NAME = "plaindoc_local_workspace";
export const LOCAL_SESSION_USER_META_KEY = "session_user_ulid";

type LocalIdScope = "user" | "space" | "node" | "revision";

export interface LocalNode {
  id?: number;
  ulid: string;
  spaceUlid: string;
  parentUlid: string | null;
  type: NodeType;
  title: string;
  sort: number;
  createdAt: string;
  updatedAt: string;
}

export interface LocalUser {
  id?: number;
  ulid: string;
  email: string;
  name: string;
  password: string;
  createdAt: string;
  updatedAt: string;
}

export interface LocalSpace {
  id?: number;
  ulid: string;
  name: string;
  createdAt: string;
  updatedAt: string;
}

export interface LocalDocument {
  id?: number;
  ulid: string;
  nodeUlid: string;
  title: string;
  contentMd: string;
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface LocalDocumentRevision {
  id?: number;
  ulid: string;
  documentUlid: string;
  version: number;
  contentMd: string;
  baseVersion: number;
  createdAt: string;
  source: "local" | "remote";
}

export interface LocalMeta {
  key: string;
  value: string;
  updatedAt: string;
}

class LocalWorkspaceDatabase extends Dexie {
  readonly usersTable: Table<LocalUser, number>;
  readonly metaTable: Table<LocalMeta, string>;
  readonly spacesTable: Table<LocalSpace, number>;
  readonly nodesTable: Table<LocalNode, number>;
  readonly documentsTable: Table<LocalDocument, number>;
  readonly revisionsTable: Table<LocalDocumentRevision, number>;

  constructor() {
    super(LOCAL_DB_NAME);

    // 数据模型约定：
    // - id: 自增技术主键（IndexedDB 内部使用）
    // - ulid: 业务主键（对外引用，全部小写）
    this.version(1).stores({
      users: "++id,&ulid,&email,updatedAt,createdAt",
      meta: "&key,updatedAt",
      spaces: "++id,&ulid,updatedAt,createdAt,name",
      nodes:
        "++id,&ulid,spaceUlid,parentUlid,type,sort,updatedAt,[spaceUlid+parentUlid],[spaceUlid+parentUlid+sort]",
      documents: "++id,&ulid,&nodeUlid,updatedAt,version",
      revisions: "++id,&ulid,documentUlid,version,createdAt,[documentUlid+version],[documentUlid+createdAt]"
    });

    this.usersTable = this.table("users");
    this.metaTable = this.table("meta");
    this.spacesTable = this.table("spaces");
    this.nodesTable = this.table("nodes");
    this.documentsTable = this.table("documents");
    this.revisionsTable = this.table("revisions");
  }
}

export const WELCOME_CONTENT = `# PlainDoc

这是本地模式（localAdapter）初始化文档。

## 当前能力

- 双栏实时预览
- 本地空间 / 目录 / 文档存储
- 自动保存与版本号递增

\`\`\`mermaid
flowchart TD
  A[编辑器] --> B[Gateway 抽象]
  B --> C[localAdapter]
\`\`\`
`;

let singletonDatabase: LocalWorkspaceDatabase | null = null;
let ensureReadyPromise: Promise<void> | null = null;

function nowIso(): string {
  return new Date().toISOString();
}

function getDatabase(): LocalWorkspaceDatabase {
  if (!singletonDatabase) {
    singletonDatabase = new LocalWorkspaceDatabase();
  }
  return singletonDatabase;
}

function getAllTables(database: LocalWorkspaceDatabase): Array<Table<any, any>> {
  return [
    database.usersTable,
    database.metaTable,
    database.spacesTable,
    database.nodesTable,
    database.documentsTable,
    database.revisionsTable
  ];
}

function getIdScopeTable(
  database: LocalWorkspaceDatabase,
  scope: LocalIdScope
): Table<{ ulid: string }, number> {
  switch (scope) {
    case "user":
      return database.usersTable as unknown as Table<{ ulid: string }, number>;
    case "space":
      return database.spacesTable as unknown as Table<{ ulid: string }, number>;
    case "node":
      return database.nodesTable as unknown as Table<{ ulid: string }, number>;
    case "revision":
      return database.revisionsTable as unknown as Table<{ ulid: string }, number>;
    default:
      throw new Error("未知的本地 ID 作用域");
  }
}

async function createUniqueUlid(
  database: LocalWorkspaceDatabase,
  scope: LocalIdScope
): Promise<string> {
  const table = getIdScopeTable(database, scope);
  return generateLowercaseUlid({
    exists: async (candidate) => (await table.where("ulid").equals(candidate).count()) > 0
  });
}

async function ensureSeeded(): Promise<void> {
  const database = getDatabase();
  await database.transaction("rw", getAllTables(database), async () => {
    const spaceCount = await database.spacesTable.count();
    if (spaceCount > 0) {
      return;
    }

    const now = nowIso();
    let owner = await database.usersTable.orderBy("createdAt").first();
    if (!owner) {
      owner = {
        ulid: await createUniqueUlid(database, "user"),
        email: "local@plaindoc.dev",
        name: "Local User",
        password: "local",
        createdAt: now,
        updatedAt: now
      };
      await database.usersTable.add(owner);
    }

    const spaceUlid = await createUniqueUlid(database, "space");
    const nodeUlid = await createUniqueUlid(database, "node");
    const revisionUlid = await createUniqueUlid(database, "revision");

    await database.spacesTable.add({
      ulid: spaceUlid,
      name: "默认空间",
      createdAt: now,
      updatedAt: now
    });

    await database.nodesTable.add({
      ulid: nodeUlid,
      spaceUlid,
      parentUlid: null,
      type: "doc",
      title: "欢迎文档",
      sort: 1,
      createdAt: now,
      updatedAt: now
    });

    // 兼容现有前端调用链：文档节点 ID 与文档 ID 保持一致。
    await database.documentsTable.add({
      ulid: nodeUlid,
      nodeUlid,
      title: "欢迎文档",
      contentMd: WELCOME_CONTENT,
      version: 1,
      createdAt: now,
      updatedAt: now
    });

    await database.revisionsTable.add({
      ulid: revisionUlid,
      documentUlid: nodeUlid,
      version: 1,
      contentMd: WELCOME_CONTENT,
      baseVersion: 0,
      createdAt: now,
      source: "local"
    });

    await database.metaTable.put({
      key: LOCAL_SESSION_USER_META_KEY,
      value: owner.ulid,
      updatedAt: now
    });
  });
}

export async function ensureLocalDatabaseReady(): Promise<void> {
  if (!ensureReadyPromise) {
    ensureReadyPromise = ensureSeeded().catch((error) => {
      ensureReadyPromise = null;
      throw error;
    });
  }
  await ensureReadyPromise;
}

export async function useDatabase<T>(
  worker: (database: LocalWorkspaceDatabase) => Promise<T> | T
): Promise<T> {
  await ensureLocalDatabaseReady();
  return worker(getDatabase());
}

export async function useDatabaseTransaction<T>(
  mode: "r" | "rw",
  worker: (database: LocalWorkspaceDatabase) => Promise<T> | T
): Promise<T> {
  await ensureLocalDatabaseReady();
  const database = getDatabase();
  return database.transaction(mode, getAllTables(database), () => worker(database));
}

// 为不同实体分配 ULID：所有输出均为 26 位小写字符串。
export async function createLocalId(scope: LocalIdScope): Promise<string> {
  await ensureLocalDatabaseReady();
  return createUniqueUlid(getDatabase(), scope);
}

export function mapLocalUser(record: LocalUser): User {
  return {
    id: record.ulid,
    email: record.email,
    name: record.name
  };
}

export function mapLocalSpace(record: LocalSpace): Space {
  return {
    id: record.ulid,
    name: record.name,
    createdAt: record.createdAt,
    updatedAt: record.updatedAt
  };
}

export function mapLocalDocument(record: LocalDocument): Document {
  return {
    id: record.ulid,
    nodeId: record.nodeUlid,
    title: record.title,
    contentMd: record.contentMd,
    version: record.version,
    updatedAt: record.updatedAt
  };
}

export function mapLocalRevision(record: LocalDocumentRevision): DocumentRevision {
  return {
    id: record.ulid,
    documentId: record.documentUlid,
    version: record.version,
    contentMd: record.contentMd,
    baseVersion: record.baseVersion,
    createdAt: record.createdAt,
    source: record.source
  };
}

export function buildTree(nodes: LocalNode[], parentId: string | null): TreeNode[] {
  return nodes
    .filter((node) => node.parentUlid === parentId)
    .sort((left, right) => left.sort - right.sort || left.title.localeCompare(right.title))
    .map((node) => ({
      id: node.ulid,
      spaceId: node.spaceUlid,
      parentId: node.parentUlid,
      type: node.type,
      title: node.title,
      sort: node.sort,
      children: buildTree(nodes, node.ulid)
    }));
}
