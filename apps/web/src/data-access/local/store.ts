import type { Document, DocumentRevision, NodeType, Space, TreeNode, User } from "../types";

const STORAGE_KEY = "plaindoc.local-db.v1";

type LocalNode = {
  id: string;
  spaceId: string;
  parentId: string | null;
  type: NodeType;
  title: string;
  sort: number;
  createdAt: string;
  updatedAt: string;
};

type LocalUser = User & {
  password: string;
};

interface LocalDatabase {
  users: Record<string, LocalUser>;
  sessionUserId: string | null;
  spaces: Record<string, Space>;
  nodes: Record<string, LocalNode>;
  documents: Record<string, Document>;
  revisions: Record<string, DocumentRevision[]>;
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

function nowIso(): string {
  return new Date().toISOString();
}

function createId(prefix: string): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return `${prefix}_${crypto.randomUUID()}`;
  }
  return `${prefix}_${Date.now()}_${Math.random().toString(16).slice(2)}`;
}

function defaultDatabase(): LocalDatabase {
  const now = nowIso();
  const userId = createId("user");
  const spaceId = createId("space");
  const docId = createId("doc");

  const user: LocalUser = {
    id: userId,
    email: "local@plaindoc.dev",
    name: "Local User",
    password: "local"
  };

  const space: Space = {
    id: spaceId,
    name: "默认空间",
    createdAt: now,
    updatedAt: now
  };

  const node: LocalNode = {
    id: docId,
    spaceId,
    parentId: null,
    type: "doc",
    title: "欢迎文档",
    sort: 1,
    createdAt: now,
    updatedAt: now
  };

  const document: Document = {
    id: docId,
    nodeId: docId,
    title: "欢迎文档",
    contentMd: WELCOME_CONTENT,
    version: 1,
    updatedAt: now
  };

  const revision: DocumentRevision = {
    id: createId("rev"),
    documentId: docId,
    version: 1,
    contentMd: WELCOME_CONTENT,
    baseVersion: 0,
    createdAt: now,
    source: "local"
  };

  return {
    users: { [userId]: user },
    sessionUserId: userId,
    spaces: { [spaceId]: space },
    nodes: { [node.id]: node },
    documents: { [document.id]: document },
    revisions: { [document.id]: [revision] }
  };
}

function getStorage(): Storage | null {
  try {
    return globalThis.localStorage;
  } catch {
    return null;
  }
}

let inMemoryDb: LocalDatabase | null = null;

function readDatabase(): LocalDatabase {
  const storage = getStorage();
  if (storage) {
    const raw = storage.getItem(STORAGE_KEY);
    if (raw) {
      try {
        return JSON.parse(raw) as LocalDatabase;
      } catch {
        const fallback = defaultDatabase();
        storage.setItem(STORAGE_KEY, JSON.stringify(fallback));
        return fallback;
      }
    }
    const seeded = defaultDatabase();
    storage.setItem(STORAGE_KEY, JSON.stringify(seeded));
    return seeded;
  }

  if (!inMemoryDb) {
    inMemoryDb = defaultDatabase();
  }
  return inMemoryDb;
}

function writeDatabase(db: LocalDatabase): void {
  const storage = getStorage();
  if (storage) {
    storage.setItem(STORAGE_KEY, JSON.stringify(db));
    return;
  }
  inMemoryDb = db;
}

export function useDatabase<T>(worker: (db: LocalDatabase) => T): T {
  const database = readDatabase();
  const result = worker(database);
  writeDatabase(database);
  return result;
}

export function createLocalId(prefix: string): string {
  return createId(prefix);
}

export function buildTree(nodes: LocalNode[], parentId: string | null): TreeNode[] {
  return nodes
    .filter((node) => node.parentId === parentId)
    .sort((left, right) => left.sort - right.sort || left.title.localeCompare(right.title))
    .map((node) => ({
      id: node.id,
      spaceId: node.spaceId,
      parentId: node.parentId,
      type: node.type,
      title: node.title,
      sort: node.sort,
      children: buildTree(nodes, node.id)
    }));
}
