export type Role = "owner" | "collaborator" | "reader";
export type NodeType = "folder" | "doc";
export type AdminRole = "platform_admin" | "space_admin";
export type EntityStatus = "active" | "banned" | "deleted";
export type Visibility = "public" | "authenticated" | "member";

export interface User {
  id: string;
  email: string;
  name: string;
  avatarUrl?: string;
}

export interface AuthSession {
  user: User | null;
  token?: string;
}

export type AuthLoginMode = "local_only" | "ldap_only" | "mixed";

export interface AuthLoginProviderOption {
  id: string;
  name: string;
  type: string;
  priority: number;
}

export interface AuthLoginOptions {
  loginMode: AuthLoginMode;
  defaultProviderId: string;
  allowUserRegister: boolean;
  providers: AuthLoginProviderOption[];
}

export interface AuthLoginInput {
  identifier?: string;
  email?: string;
  provider?: string;
  password: string;
  captchaId?: string;
  captchaAnswer?: string;
}

export interface AuthRegisterInput {
  email: string;
  password: string;
  name: string;
  captchaId?: string;
  captchaAnswer?: string;
}

export interface AuthCaptchaChallenge {
  captchaId: string;
  captchaImageDataUrl: string;
  level: number;
  expiresInSeconds: number;
}

export interface AuthCaptchaRefreshInput {
  scene: "login" | "register";
  identifier: string;
  captchaId?: string;
}

export interface Space {
  id: string;
  name: string;
  createdAt: string;
  updatedAt: string;
}

export interface TreeNode {
  id: string;
  documentId?: string;
  spaceId: string;
  parentId: string | null;
  type: NodeType;
  title: string;
  sort: number;
  visibility?: Visibility;
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
  visibility?: Visibility;
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

export type DocumentAttachmentPreviewKind = "none" | "image" | "pdf" | "office" | "text";

export interface DocumentAttachment {
  attachmentId: string;
  documentId: string;
  spaceId: string;
  fileName: string;
  mimeType: string;
  sizeBytes: number;
  storageProvider: string;
  previewKind: DocumentAttachmentPreviewKind;
  previewSupported: boolean;
  requiresAuthDownload: boolean;
  publicDownloadUrl?: string;
  createdAt: string;
  updatedAt: string;
  deletedAt?: string | null;
}

export interface DocumentAttachmentAccessLink {
  url: string;
  purpose: "download" | "preview";
  previewKind: DocumentAttachmentPreviewKind;
  requiresAuth: boolean;
  expiresAt?: string;
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
  targetParentId: string | null;
  targetIndex: number;
}

export interface SaveDocumentInput {
  docId: string;
  contentMd: string;
  baseVersion: number;
}

export interface SaveDocumentResult {
  document: Document;
}

export interface LocalizeRemoteImagesInput {
  docId: string;
  imageUrls: string[];
}

export interface LocalizeRemoteImagesResult {
  localizedUrls: Record<string, string>;
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
  getLoginOptions(): Promise<AuthLoginOptions>;
  getSession(): Promise<AuthSession>;
  login(input: AuthLoginInput): Promise<AuthSession>;
  register(input: AuthRegisterInput): Promise<AuthSession>;
  refreshCaptcha(input: AuthCaptchaRefreshInput): Promise<AuthCaptchaChallenge>;
  logout(): Promise<void>;
}

export interface WorkspaceGateway {
  listSpaces(): Promise<Space[]>;
  getSpace(spaceId: string): Promise<Space>;
  createSpace(input: CreateSpaceInput): Promise<Space>;
  getTree(spaceId: string): Promise<TreeNode[]>;
  createNode(input: CreateNodeInput): Promise<CreateNodeResult>;
  updateNode(input: UpdateNodeInput): Promise<void>;
  deleteNode(nodeId: string): Promise<void>;
  // 目录移动：用于目录树拖拽排序（同级重排 + 跨父级移动）。
  moveNode(input: MoveNodeInput): Promise<void>;
  // 空间删除（扩展点）：本期可不实现，后续用于空间管理。
  deleteSpace?(spaceId: string): Promise<void>;
}

export interface DocumentGateway {
  getDocument(docId: string): Promise<Document>;
  saveDocument(input: SaveDocumentInput): Promise<SaveDocumentResult>;
  localizeRemoteImages(input: LocalizeRemoteImagesInput): Promise<LocalizeRemoteImagesResult>;
  listRevisions(docId: string): Promise<DocumentRevision[]>;
  listAttachments(docId: string): Promise<DocumentAttachment[]>;
  uploadAttachment(input: { docId: string; file: File }): Promise<DocumentAttachment>;
  deleteAttachment(input: { docId: string; attachmentId: string; physicalDelete?: boolean }): Promise<void>;
  createAttachmentAccessLink(input: {
    docId: string;
    attachmentId: string;
    purpose?: "download" | "preview";
  }): Promise<DocumentAttachmentAccessLink>;
  setDocumentTheme(docId: string, themeId: string): Promise<Document>;
  updateDocumentVisibility(docId: string, visibility: Visibility): Promise<Document>;
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
  email: string;
  name: string;
  avatarUrl: string;
  roles: AdminRole[];
}

export interface AdminProfile {
  userId: string;
  email: string;
  name: string;
  avatarUrl: string;
  roles: AdminRole[];
  createdAt: string;
  updatedAt: string;
}

export type AdminUserStatusFilter = "all" | "active" | "banned" | "deleted";

export interface AdminUser {
  userId: string;
  email: string;
  name: string;
  role: "user" | AdminRole;
  canEditRole: boolean;
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
export type AdminSpaceCoverSource = "user_upload" | "system_generated";

export interface AdminSpaceCover {
  assetId?: string;
  key: string;
  url: string;
  width: number;
  height: number;
  mimeType: string;
  sizeBytes?: number;
  normalized: boolean;
  source: AdminSpaceCoverSource;
}

export interface AdminSpace {
  spaceId: string;
  name: string;
  description: string;
  categoryId: string;
  category: string;
  categoryIsDefault: boolean;
  ownerUserId: string;
  ownerName: string;
  ownerEmail: string;
  visibility: Visibility;
  cover?: AdminSpaceCover | null;
  status: EntityStatus;
  bannedReason: string;
  bannedAt: string | null;
  deletedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface AdminSpaceMember {
  userId: string;
  email: string;
  name: string;
  role: Role;
  isOwner: boolean;
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

export interface AdminSpaceCategory {
  categoryId: string;
  name: string;
  isDefault: boolean;
  createdAt: string;
  updatedAt: string;
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

export type AdminDocumentAttachmentStatusFilter = "all" | "active" | "deleted";
export type AdminDocumentAttachmentStorageProviderFilter = "all" | "local" | "cloudflare-r2" | "aliyun-oss";

export interface AdminDocumentAttachment {
  attachmentId: string;
  documentId: string;
  documentTitle: string;
  documentStatus: EntityStatus;
  spaceId: string;
  spaceName: string;
  spaceOwnerUserId: string;
  spaceOwnerName: string;
  spaceOwnerEmail: string;
  fileName: string;
  mimeType: string;
  sizeBytes: number;
  storageProvider: string;
  previewKind: DocumentAttachmentPreviewKind;
  status: EntityStatus;
  createdByUserId: string | null;
  createdByName: string;
  createdByEmail: string;
  deletedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface AdminDocumentAttachmentListInput {
  keyword?: string;
  spaceId?: string;
  documentId?: string;
  status?: AdminDocumentAttachmentStatusFilter;
  storageProvider?: AdminDocumentAttachmentStorageProviderFilter;
  page?: number;
  pageSize?: number;
}

export interface AdminDocumentAttachmentListResult {
  items: AdminDocumentAttachment[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
  };
}

export interface AdminDocumentAttachmentDeleteReference {
  attachmentId: string;
  documentId: string;
  documentTitle: string;
  spaceId: string;
  spaceName: string;
  fileName: string;
}

export interface AdminDocumentAttachmentDeleteResult {
  attachmentId: string;
  documentId: string;
  spaceId: string;
  physicalDeleteRequested: boolean;
  physicalDeleteExecuted: boolean;
  softDeleted: boolean;
  hardDeleted: boolean;
  sharedReferenceCount: number;
  sharedReferences: AdminDocumentAttachmentDeleteReference[];
  confirmationRequired: boolean;
  confirmationReason: string;
  physicalDeleteError: string;
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

export interface AdminDataRetentionCleanupPolicy {
  enabled: boolean;
  scheduleMinutes: number;
  cleanupBatchSize: number;
  auditLogRetentionDays: number;
  authCaptchaRetentionHours: number;
  authRiskStateRetentionDays: number;
  userSessionRetentionDays: number;
}

export interface AdminDataRetentionCleanupResult {
  policy: AdminDataRetentionCleanupPolicy;
  startedAt: string;
  finishedAt: string;
  deletedAuditLogs: number;
  deletedAuthCaptchaChallenges: number;
  deletedAuthRiskStates: number;
  deletedUserSessions: number;
  totalDeleted: number;
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
  getProfile(): Promise<AdminProfile>;
  updateProfile(input: { name?: string; avatarUrl?: string }): Promise<AdminProfile>;
  updatePassword(input: { currentPassword: string; newPassword: string; confirmPassword: string }): Promise<void>;
  uploadAvatar(file: File): Promise<AdminProfile>;
  canManageSpace(spaceId: string): Promise<boolean>;
  listUsers(input?: AdminUserListInput): Promise<AdminUserListResult>;
  createUser(input: { email: string; name: string; password: string; role: "user" | AdminRole }): Promise<AdminUser>;
  updateUserRole(input: { userId: string; role: "user" | AdminRole }): Promise<AdminUser>;
  updateUserStatus(input: { userId: string; status: "active" | "banned"; reason?: string }): Promise<AdminUser>;
  deleteUser(userId: string): Promise<void>;
  createSpace(input: {
    spaceId?: string;
    name: string;
    description?: string;
    categoryId?: string;
    visibility?: Visibility;
    coverAssetId?: string;
  }): Promise<AdminSpace>;
  createSpaceCoverAsset(input: {
    source: AdminSpaceCoverSource;
    file?: File;
    spaceName?: string;
    clientWidth?: number;
    clientHeight?: number;
    clientMimeType?: string;
    clientProcessed?: boolean;
  }): Promise<AdminSpaceCover>;
  listSpaceCategories(): Promise<AdminSpaceCategory[]>;
  createSpaceCategory(input: { name: string }): Promise<AdminSpaceCategory>;
  renameSpaceCategory(input: { categoryId: string; name: string }): Promise<AdminSpaceCategory>;
  deleteSpaceCategory(input: { categoryId: string }): Promise<{
    categoryId: string;
    reassignedToCategoryId: string;
    reassignedToName: string;
    movedSpaceCount: number;
  }>;
  listSpaces(input?: AdminSpaceListInput): Promise<AdminSpaceListResult>;
  updateSpaceStatus(input: { spaceId: string; status: "active" | "banned"; reason?: string }): Promise<AdminSpace>;
  updateSpaceMetadata(input: {
    spaceId: string;
    name?: string;
    description?: string;
    categoryId?: string;
    visibility?: Visibility;
    coverAssetId?: string;
  }): Promise<AdminSpace>;
  listSpaceMembers(spaceId: string): Promise<AdminSpaceMember[]>;
  addSpaceMember(input: {
    spaceId: string;
    targetUserId?: string;
    targetEmail?: string;
    role: "collaborator" | "reader";
  }): Promise<AdminSpaceMember>;
  updateSpaceMemberRole(input: {
    spaceId: string;
    userId: string;
    role: "collaborator" | "reader";
  }): Promise<AdminSpaceMember>;
  removeSpaceMember(input: { spaceId: string; userId: string }): Promise<void>;
  // 空间管理：转让空间归属（管理员操作）。
  transferSpaceOwnership(input: { spaceId: string; targetUserId?: string; targetEmail?: string }): Promise<AdminSpace>;
  deleteSpace(spaceId: string): Promise<void>;
  listDocuments(input?: AdminDocumentListInput): Promise<AdminDocumentListResult>;
  updateDocumentStatus(input: { documentId: string; status: "active" | "banned"; reason?: string }): Promise<AdminDocument>;
  deleteDocument(documentId: string): Promise<void>;
  listDocumentAttachments(input?: AdminDocumentAttachmentListInput): Promise<AdminDocumentAttachmentListResult>;
  deleteDocumentAttachment(input: {
    attachmentId: string;
    physicalDelete?: boolean;
    forcePhysicalDeleteOnShare?: boolean;
  }): Promise<AdminDocumentAttachmentDeleteResult>;
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
    configKey: "site" | "editor" | "security" | "auth" | "image-hosting" | "sitemap" | "data-retention";
    value: Record<string, unknown>;
    expectedVersion?: number;
  }): Promise<AdminSystemConfig>;
  runDataRetentionCleanup(): Promise<AdminDataRetentionCleanupResult>;
  testAuthLDAPConnection(input: {
    value: Record<string, unknown>;
    providerId?: string;
  }): Promise<{ ok: boolean }>;
  listAudits(input?: AdminAuditListInput): Promise<AdminAuditListResult>;
}

export interface UploadLocalImageResult {
  key: string;
  url: string;
}

export interface ImageHostingGateway {
  // 获取当前生效的图床配置（由后端系统配置统一管理）。
  getConfig(): Promise<Record<string, unknown>>;
  // 上传到本地图片存储（由后端接收 multipart/form-data 文件）。
  uploadLocalImage(
    file: File,
    uploadEndpoint?: string,
    options?: {
      spaceId?: string | null;
    }
  ): Promise<UploadLocalImageResult>;
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
  imageHosting: ImageHostingGateway;
  userConfig: UserConfigGateway;
}
