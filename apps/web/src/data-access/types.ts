export type Role = "owner" | "collaborator" | "reader";
export type NodeType = "folder" | "doc";
export type AdminRole = "platform_admin" | "space_admin";
export type EntityStatus = "active" | "banned" | "deleted";
export type Visibility = "public" | "authenticated" | "member";

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
  themeId: string;
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

// 目录节点移动参数：用于后续拖拽排序扩展点。
export interface MoveNodeInput {
  nodeId: string;
  parentId: string | null;
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

// 用户配置键值读写参数：用于 user_config 表抽象。
export type UserConfigUserId = string | number;

export interface UserConfigGetInput {
  userId: UserConfigUserId;
  key: string;
}

export interface UserConfigSetInput {
  userId: UserConfigUserId;
  key: string;
  value: unknown;
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
  // 目录移动（扩展点）：本期可不实现，后续用于拖拽排序。
  moveNode?(input: MoveNodeInput): Promise<void>;
  // 空间删除（扩展点）：本期可不实现，后续用于空间管理。
  deleteSpace?(spaceId: string): Promise<void>;
}

export interface DocumentGateway {
  getDocument(docId: string): Promise<Document>;
  saveDocument(input: SaveDocumentInput): Promise<SaveDocumentResult>;
  listRevisions(docId: string): Promise<DocumentRevision[]>;
  setDocumentTheme(docId: string, themeId: string): Promise<Document>;
}

export interface Theme {
  id: string;
  name: string;
  description: string;
  variables: Record<string, string>;
  syntaxTheme: "one-light" | "one-dark";
  codeBlockStyle: Record<string, string | number>;
  codeBlockCodeStyle: Record<string, string | number>;
  inlineCodeStyle: Record<string, string | number>;
  customCss?: string;
  builtin?: boolean;
}

export interface ThemeGateway {
  listThemes(): Promise<Theme[]>;
}

export interface AdminIdentity {
  userId: string;
  roles: AdminRole[];
}

export type AdminUserStatusFilter = "all" | "active" | "banned" | "deleted";

export interface AdminUser {
  userId: string;
  email: string;
  name: string;
  status: EntityStatus;
  bannedReason: string;
  bannedAt: string | null;
  deletedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface AdminUserListInput {
  keyword?: string;
  status?: AdminUserStatusFilter;
  page?: number;
  pageSize?: number;
}

export interface AdminUserListResult {
  items: AdminUser[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
  };
}

export type AdminSpaceStatusFilter = "all" | "active" | "banned" | "deleted";
export type AdminSpaceVisibilityFilter = "all" | "public" | "authenticated" | "member";

export interface AdminSpace {
  spaceId: string;
  name: string;
  ownerUserId: string;
  ownerName: string;
  ownerEmail: string;
  visibility: Visibility;
  status: EntityStatus;
  bannedReason: string;
  bannedAt: string | null;
  deletedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface AdminSpaceListInput {
  keyword?: string;
  status?: AdminSpaceStatusFilter;
  visibility?: AdminSpaceVisibilityFilter;
  page?: number;
  pageSize?: number;
}

export interface AdminSpaceListResult {
  items: AdminSpace[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
  };
}

export type AdminDocumentStatusFilter = "all" | "active" | "banned" | "deleted";
export type AdminDocumentVisibilityFilter = "all" | "public" | "authenticated" | "member";

export interface AdminDocument {
  documentId: string;
  nodeId: string;
  title: string;
  spaceId: string;
  spaceName: string;
  spaceOwnerUserId: string;
  spaceOwnerName: string;
  spaceOwnerEmail: string;
  visibility: Visibility;
  status: EntityStatus;
  bannedReason: string;
  bannedAt: string | null;
  deletedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface AdminDocumentListInput {
  keyword?: string;
  spaceId?: string;
  status?: AdminDocumentStatusFilter;
  visibility?: AdminDocumentVisibilityFilter;
  page?: number;
  pageSize?: number;
}

export interface AdminDocumentListResult {
  items: AdminDocument[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
  };
}

export interface AdminTheme {
  themeId: string;
  name: string;
  description: string;
  variables: Record<string, string>;
  syntaxTheme: "one-light" | "one-dark";
  codeBlockStyle: Record<string, string | number>;
  codeBlockCodeStyle: Record<string, string | number>;
  inlineCodeStyle: Record<string, string | number>;
  customCss: string;
  builtin: boolean;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface AdminSystemConfig {
  configKey: string;
  value: Record<string, unknown>;
  version: number;
  updatedByUserId: string | null;
  createdAt: string;
  updatedAt: string;
}

export type AdminAuditModule = "user" | "space" | "document" | "theme" | "system_config";
export type AdminAuditAction = "create" | "update" | "delete";

export interface AdminAuditLog {
  id: number;
  actorUserId: string | null;
  actorName: string;
  actorEmail: string;
  module: AdminAuditModule;
  action: AdminAuditAction;
  targetType: string;
  targetId: string;
  summary: string;
  detail: Record<string, unknown>;
  requestId: string;
  createdAt: string;
}

export interface AdminAuditListInput {
  keyword?: string;
  module?: "all" | AdminAuditModule;
  action?: "all" | AdminAuditAction;
  actorUserId?: string;
  targetType?: string;
  targetId?: string;
  requestId?: string;
  from?: string;
  to?: string;
  page?: number;
  pageSize?: number;
}

export interface AdminAuditListResult {
  items: AdminAuditLog[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
  };
}

export interface AdminGateway {
  getMe(): Promise<AdminIdentity>;
  canManageSpace(spaceId: string): Promise<boolean>;
  listUsers(input?: AdminUserListInput): Promise<AdminUserListResult>;
  updateUserStatus(input: { userId: string; status: "active" | "banned"; reason?: string }): Promise<AdminUser>;
  deleteUser(userId: string): Promise<void>;
  listSpaces(input?: AdminSpaceListInput): Promise<AdminSpaceListResult>;
  updateSpaceStatus(input: { spaceId: string; status: "active" | "banned"; reason?: string }): Promise<AdminSpace>;
  updateSpaceMetadata(input: { spaceId: string; name?: string; visibility?: Visibility }): Promise<AdminSpace>;
  deleteSpace(spaceId: string): Promise<void>;
  listDocuments(input?: AdminDocumentListInput): Promise<AdminDocumentListResult>;
  updateDocumentStatus(input: { documentId: string; status: "active" | "banned"; reason?: string }): Promise<AdminDocument>;
  deleteDocument(documentId: string): Promise<void>;
  listThemes(): Promise<AdminTheme[]>;
  createTheme(input: {
    themeId: string;
    name: string;
    description?: string;
    variables?: Record<string, string>;
    syntaxTheme?: "one-light" | "one-dark";
    codeBlockStyle?: Record<string, string | number>;
    codeBlockCodeStyle?: Record<string, string | number>;
    inlineCodeStyle?: Record<string, string | number>;
    customCss?: string;
    enabled?: boolean;
  }): Promise<AdminTheme>;
  updateTheme(input: {
    themeId: string;
    name?: string;
    description?: string;
    variables?: Record<string, string>;
    syntaxTheme?: "one-light" | "one-dark";
    codeBlockStyle?: Record<string, string | number>;
    codeBlockCodeStyle?: Record<string, string | number>;
    inlineCodeStyle?: Record<string, string | number>;
    customCss?: string;
    enabled?: boolean;
  }): Promise<AdminTheme>;
  deleteTheme(themeId: string): Promise<void>;
  listSystemConfigs(): Promise<AdminSystemConfig[]>;
  upsertSystemConfig(input: {
    configKey: "site" | "editor" | "security";
    value: Record<string, unknown>;
    expectedVersion?: number;
  }): Promise<AdminSystemConfig>;
  listAudits(input?: AdminAuditListInput): Promise<AdminAuditListResult>;
}

export interface UserConfigGateway {
  // 读取配置：不存在时返回 null。
  getValue<T = unknown>(input: UserConfigGetInput): Promise<T | null>;
  // 写入配置：按 userId + key 覆盖或新增。
  setValue<T = unknown>(input: UserConfigSetInput & { value: T }): Promise<void>;
}

export interface DataGateway {
  auth: AuthGateway;
  workspace: WorkspaceGateway;
  document: DocumentGateway;
  theme: ThemeGateway;
  admin: AdminGateway;
  userConfig: UserConfigGateway;
}
