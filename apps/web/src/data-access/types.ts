export type Role = "owner" | "collaborator" | "reader";
export type NodeType = "folder" | "doc";

export interface User {
  id: string;
  email: string;
  name: string;
}

export interface AuthSession {
  user: User | null;
  token?: string;
}

export interface Space {
  id: string;
  name: string;
  createdAt: string;
  updatedAt: string;
}

export interface TreeNode {
  id: string;
  spaceId: string;
  parentId: string | null;
  type: NodeType;
  title: string;
  sort: number;
  children: TreeNode[];
}

export interface Document {
  id: string;
  nodeId: string;
  title: string;
  contentMd: string;
  version: number;
  updatedAt: string;
}

export interface DocumentRevision {
  id: string;
  documentId: string;
  version: number;
  contentMd: string;
  baseVersion: number;
  createdAt: string;
  source: "local" | "remote";
}

export interface CreateSpaceInput {
  name: string;
}

export interface CreateNodeInput {
  spaceId: string;
  parentId: string | null;
  type: NodeType;
  title: string;
}

export interface CreateNodeResult {
  nodeId: string;
  docId?: string;
}

export interface UpdateNodeInput {
  nodeId: string;
  title?: string;
  parentId?: string | null;
  sort?: number;
}

export interface SaveDocumentInput {
  docId: string;
  contentMd: string;
  baseVersion: number;
}

export interface SaveDocumentResult {
  document: Document;
}

export class ConflictError extends Error {
  readonly latestDocument: Document;

  constructor(latestDocument: Document) {
    super("Document version conflict");
    this.name = "ConflictError";
    this.latestDocument = latestDocument;
  }
}

export interface AuthGateway {
  getSession(): Promise<AuthSession>;
  login(input: { email: string; password: string }): Promise<AuthSession>;
  register(input: { email: string; password: string; name: string }): Promise<AuthSession>;
  logout(): Promise<void>;
}

export interface WorkspaceGateway {
  listSpaces(): Promise<Space[]>;
  createSpace(input: CreateSpaceInput): Promise<Space>;
  getTree(spaceId: string): Promise<TreeNode[]>;
  createNode(input: CreateNodeInput): Promise<CreateNodeResult>;
  updateNode(input: UpdateNodeInput): Promise<void>;
  deleteNode(nodeId: string): Promise<void>;
}

export interface DocumentGateway {
  getDocument(docId: string): Promise<Document>;
  saveDocument(input: SaveDocumentInput): Promise<SaveDocumentResult>;
  listRevisions(docId: string): Promise<DocumentRevision[]>;
}

export interface DataGateway {
  auth: AuthGateway;
  workspace: WorkspaceGateway;
  document: DocumentGateway;
}
