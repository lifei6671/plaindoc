export type Role = "owner" | "collaborator" | "reader";
export type NodeType = "folder" | "doc";
export type DocumentFormat = "markdown" | "docx" | "xlsx";
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
  passwordResetEnabled: boolean;
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

export interface AuthPasswordResetRequestInput {
  email: string;
}

export interface AuthPasswordResetVerifyInput {
  token: string;
}

export interface AuthPasswordResetVerifyResult {
  valid: boolean;
  expiresAt: string;
}

export interface AuthPasswordResetConfirmInput {
  token: string;
  newPassword: string;
  confirmPassword: string;
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
  documentIdentifier?: string;
  documentRouteKey?: string;
  documentFormat?: DocumentFormat;
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
  format?: DocumentFormat;
  title: string;
  contentMd: string;
  version: number;
  sourceBlobId?: string;
  sourceFileName?: string;
  sourceMimeType?: string;
  contentVersion?: number;
  updatedAt: string;
  visibility?: Visibility;
}

export type DocumentRevisionFormat = "markdown" | "docx" | "xlsx";

export interface DocumentRevisionEditorUser {
  userId: string;
  displayName: string;
}

export interface DocumentRevisionSummary {
  id: string;
  documentId: string;
  version: number;
  baseVersion: number;
  createdAt: string;
  source: "local" | "remote";
  format: DocumentRevisionFormat;
  fileName?: string;
  mimeType?: string;
  editorUser?: DocumentRevisionEditorUser | null;
}

export interface DocumentRevisionFileMetadata {
  fileName?: string;
  mimeType?: string;
  blobId?: string;
}

export interface DocumentRevisionDetail extends DocumentRevisionSummary {
  // 历史版本列表只返回摘要；仅详情接口会携带 Markdown 正文。
  contentMd?: string;
  // Office 历史版本没有 Markdown 正文，详情接口返回文件版本元数据供预览/恢复使用。
  file?: DocumentRevisionFileMetadata | null;
}

export type DocumentRevision = DocumentRevisionDetail;

export interface ListDocumentRevisionsOptions {
  page?: number;
  pageSize?: number;
}

export type DocumentAttachmentPreviewKind = "none" | "image" | "pdf" | "office" | "text";
export type DocumentShareMode = "public" | "password";

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

export interface DocumentShareConfig {
  enabled: boolean;
  shareId: string;
  documentId: string;
  spaceId: string;
  mode: DocumentShareMode;
  passwordHint: string;
  hasPassword: boolean;
  expiresAt?: string | null;
  disabledAt?: string | null;
  accessVersion: number;
  createdAt: string;
  updatedAt: string;
}

export interface UpdateDocumentShareInput {
  enabled: boolean;
  mode?: DocumentShareMode;
  password?: string;
  passwordHint?: string;
  expiresAt?: string | null;
}

export interface CreateSpaceInput {
  name: string;
}

export interface CreateNodeInput {
  spaceId: string;
  parentId: string | null;
  type: NodeType;
  title: string;
  documentIdentifier?: string;
  templateId?: string;
  format?: DocumentFormat;
}

export interface CreateNodeResult {
  nodeId: string;
  docId?: string;
}

export interface ImportWorkspaceDocumentsInput {
  spaceId: string;
  targetNodeId: string | null;
  files: File[];
  autoExtractTitle: boolean;
}

export interface ImportedWorkspaceNode {
  nodeId: string;
  documentId?: string;
  parentId?: string | null;
  title: string;
  type: NodeType;
  format?: DocumentFormat;
}

export interface ImportWorkspaceItemResult {
  sourceName: string;
  sourcePath: string;
  detectedType: string;
  status: "success" | "failed" | "skipped";
  stage: string;
  errorMessage?: string;
  createdNodeId?: string;
  createdDocumentId?: string;
}

export interface ImportWorkspaceDocumentsResult {
  totalCount: number;
  successCount: number;
  failedCount: number;
  items: ImportWorkspaceItemResult[];
  failureItems: ImportWorkspaceItemResult[];
  createdNodes: ImportedWorkspaceNode[];
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

export interface RestoreDocumentRevisionInput {
  docId: string;
  revisionId: string;
  // 前端调用方应始终传当前 Document.version，后端据此做并发版本校验。
  baseVersion: number;
}

export interface RestoreDocumentRevisionResult extends SaveDocumentResult {
  restoredFromRevision: DocumentRevisionSummary;
}

export interface OnlyOfficeEditConfig {
  documentServerUrl: string;
  config: Record<string, unknown>;
}

export interface UpdateDocumentIdentifierResult {
  documentId: string;
  identifier: string | null;
  readerUrl: string;
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
  requestPasswordReset(input: AuthPasswordResetRequestInput): Promise<void>;
  verifyPasswordResetToken(input: AuthPasswordResetVerifyInput): Promise<AuthPasswordResetVerifyResult>;
  confirmPasswordReset(input: AuthPasswordResetConfirmInput): Promise<void>;
  logout(): Promise<void>;
}

export interface WorkspaceGateway {
  listSpaces(): Promise<Space[]>;
  getSpace(spaceId: string): Promise<Space>;
  createSpace(input: CreateSpaceInput): Promise<Space>;
  getTree(spaceId: string): Promise<TreeNode[]>;
  createNode(input: CreateNodeInput): Promise<CreateNodeResult>;
  importDocuments(input: ImportWorkspaceDocumentsInput): Promise<ImportWorkspaceDocumentsResult>;
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
  getOnlyOfficeEditConfig(docId: string): Promise<OnlyOfficeEditConfig>;
  updateDocumentIdentifier(docId: string, identifier: string | null): Promise<UpdateDocumentIdentifierResult>;
  getShareConfig(docId: string): Promise<DocumentShareConfig>;
  updateShareConfig(docId: string, input: UpdateDocumentShareInput): Promise<DocumentShareConfig>;
  disableShare(docId: string): Promise<void>;
  localizeRemoteImages(input: LocalizeRemoteImagesInput): Promise<LocalizeRemoteImagesResult>;
  listRevisions(docId: string, options?: ListDocumentRevisionsOptions): Promise<DocumentRevisionSummary[]>;
  getRevisionDetail(docId: string, revisionId: string): Promise<DocumentRevisionDetail>;
  restoreRevision(input: RestoreDocumentRevisionInput): Promise<RestoreDocumentRevisionResult>;
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

export interface DocumentTemplateSummary {
  templateId: string;
  sceneKey: string;
  sceneName: string;
  name: string;
  description: string;
  defaultTitle: string;
  sort: number;
  builtin: boolean;
  enabled: boolean;
  updatedAt: string;
}

export interface DocumentTemplateListResult {
  items: DocumentTemplateSummary[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
  };
}

export interface DocumentTemplateDetail {
  templateId: string;
  sceneKey: string;
  sceneName: string;
  name: string;
  description: string;
  defaultTitle: string;
  contentMd: string;
  sort: number;
  builtin: boolean;
  enabled: boolean;
  createdByUserId?: string;
  updatedByUserId?: string;
  createdAt: string;
  updatedAt: string;
}

export interface DocumentTemplateGateway {
  listTemplates(input?: {
    sceneKey?: string;
    keyword?: string;
    page?: number;
    pageSize?: number;
  }): Promise<DocumentTemplateListResult>;
  getTemplate(templateId: string): Promise<DocumentTemplateDetail>;
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
  capabilities: AdminAccessCapabilities;
}

export interface AdminAccessCapabilities {
  canViewSpaceManagement: boolean;
  canManageSpace: boolean;
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

export type AdminSpaceExportFormat = "markdown_zip" | "source_zip" | "epub";

export interface AdminSpaceExportStartInput {
  spaceId: string;
  format: AdminSpaceExportFormat;
  includeAttachments?: boolean;
  includeOfficeSources?: boolean;
}

export interface AdminSpaceExportStartResult {
  jobId: string;
  streamUrl: string;
}

export type AdminSpaceTransferEventType = "progress" | "completed" | "failed";

export interface AdminSpaceTransferEvent {
  type: AdminSpaceTransferEventType;
  stage?: string;
  progress?: number;
  message?: string;
  downloadUrl?: string;
  fileName?: string;
  sizeBytes?: number;
  spaceId?: string;
  spaceName?: string;
  newSpaceId?: string;
}

export interface AdminSpaceTransferSubscription {
  close(): void;
}

export interface AdminSpaceTransferSubscribeInput {
  streamUrl: string;
  onEvent(event: AdminSpaceTransferEvent): void;
  onError?(error: Event): void;
}

export type AdminSpaceTransferTaskKind = "space_export" | "space_import";
export type AdminSpaceTransferTaskStatus = "queued" | "running" | "completed" | "failed";
export type AdminSpaceTransferTaskStatusFilter = "active" | AdminSpaceTransferTaskStatus;

export interface AdminSpaceTransferTask {
  jobId: string;
  kind: AdminSpaceTransferTaskKind;
  status: AdminSpaceTransferTaskStatus;
  stage?: string;
  progress: number;
  message?: string;
  spaceId?: string;
  spaceName?: string;
  format?: AdminSpaceExportFormat | string;
  importId?: string;
  fileName?: string;
  sizeBytes?: number;
  newSpaceId?: string;
  errorMessage?: string;
  createdAt: string;
  updatedAt: string;
  expiresAt: string;
}

export interface AdminSpaceTransferTaskListInput {
  status?: AdminSpaceTransferTaskStatusFilter;
  limit?: number;
}

export interface AdminSpaceTransferTaskListResult {
  tasks: AdminSpaceTransferTask[];
}

export interface AdminSpaceTransferTaskResult {
  task: AdminSpaceTransferTask;
}

export interface AdminSpaceTransferTokenInput {
  kind: AdminSpaceTransferTaskKind;
  jobId: string;
}

export interface AdminSpaceTransferStreamTokenResult {
  streamUrl: string;
}

export interface AdminSpaceTransferDownloadTokenResult {
  downloadUrl: string;
}

export type AdminSpaceImportPackageType = "plaindoc-space" | "plaindoc" | "epub";

export interface AdminSpaceImportInspectResult {
  importId: string;
  packageVersion: number;
  packageType: AdminSpaceImportPackageType;
  exportedAt?: string;
  sourcePublishedAt?: string;
  sourceAuthors?: string[];
  importable: boolean;
  space: {
    spaceId: string;
    name: string;
    categoryId?: string;
    visibility: Visibility;
    hasCover?: boolean;
  };
  summary: {
    folderCount: number;
    documentCount: number;
    attachmentCount: number;
    officeSourceCount: number;
    imageCount?: number;
    maxDepth?: number;
  };
  warnings: string[];
}

export interface AdminSpaceImportCommitInput {
  importId: string;
  spaceName: string;
  spaceId?: string;
  categoryId?: string;
  visibility?: Visibility;
}

export interface AdminSpaceImportStartResult {
  jobId: string;
  streamUrl: string;
}

export type AdminDocumentStatusFilter = "all" | "active" | "banned" | "deleted";
export type AdminDocumentVisibilityFilter = "all" | "public" | "authenticated" | "member";
export type AdminDocumentFormatFilter = "all" | DocumentFormat;

export interface AdminDocument {
  documentId: string;
  documentRouteKey?: string;
  nodeId: string;
  format?: DocumentFormat;
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

export type AdminDocumentShareView = "all" | "mine";

export interface AdminDocumentShare {
  shareId: string;
  documentId: string;
  documentRouteKey?: string;
  documentTitle: string;
  spaceId: string;
  spaceName: string;
  spaceOwnerUserId: string;
  spaceOwnerName: string;
  spaceOwnerEmail: string;
  mode: DocumentShareMode;
  passwordHint: string;
  hasPassword: boolean;
  expiresAt?: string | null;
  accessVersion: number;
  createdByUserId?: string | null;
  createdByName: string;
  createdByEmail: string;
  updatedByUserId?: string | null;
  createdAt: string;
  updatedAt: string;
  isExpired: boolean;
}

export interface AdminDocumentShareListInput {
  view?: AdminDocumentShareView;
  keyword?: string;
  spaceId?: string;
  mode?: "all" | DocumentShareMode;
  expired?: "all" | "yes" | "no";
  page?: number;
  pageSize?: number;
}

export interface AdminDocumentShareListResult {
  items: AdminDocumentShare[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
  };
}

export interface AdminDocumentListInput {
  keyword?: string;
  spaceId?: string;
  status?: AdminDocumentStatusFilter;
  visibility?: AdminDocumentVisibilityFilter;
  format?: AdminDocumentFormatFilter;
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
  documentRouteKey?: string;
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

export type AdminDocumentImageAssetStatusFilter = "all" | "active" | "pending_cleanup" | "deleted";
export type AdminDocumentImageAssetStorageProviderFilter = "all" | "local" | "cloudflare-r2" | "aliyun-oss";

export interface AdminDocumentImageAsset {
  imageAssetId: string;
  documentId: string;
  documentRouteKey?: string;
  documentTitle: string;
  documentStatus: EntityStatus;
  spaceId: string;
  spaceName: string;
  spaceOwnerUserId: string;
  spaceOwnerName: string;
  spaceOwnerEmail: string;
  storageProvider: string;
  objectKey: string;
  objectUrl: string;
  status: "active" | "pending_cleanup" | "deleted";
  lastReferencedAt: string;
  pendingCleanupAt: string | null;
  deletedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface AdminDocumentImageAssetListInput {
  keyword?: string;
  spaceId?: string;
  documentId?: string;
  status?: AdminDocumentImageAssetStatusFilter;
  storageProvider?: AdminDocumentImageAssetStorageProviderFilter;
  page?: number;
  pageSize?: number;
}

export interface AdminDocumentImageAssetListResult {
  items: AdminDocumentImageAsset[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
  };
}

export interface AdminDocumentImageAssetDeleteReference {
  imageAssetId: string;
  documentId: string;
  documentTitle: string;
  spaceId: string;
  spaceName: string;
}

export interface AdminDocumentImageAssetDeleteResult {
  imageAssetId: string;
  documentId: string;
  spaceId: string;
  physicalDeleteRequested: boolean;
  physicalDeleteExecuted: boolean;
  softDeleted: boolean;
  hardDeleted: boolean;
  sharedReferenceCount: number;
  sharedReferences: AdminDocumentImageAssetDeleteReference[];
  confirmationRequired: boolean;
  confirmationReason: string;
  physicalDeleteError: string;
}

export interface AdminDocumentTemplate {
  templateId: string;
  sceneKey: string;
  sceneName: string;
  name: string;
  description: string;
  defaultTitle: string;
  contentMd: string;
  sort: number;
  builtin: boolean;
  enabled: boolean;
  createdByUserId?: string;
  updatedByUserId?: string;
  createdAt: string;
  updatedAt: string;
}

export interface AdminDocumentTemplateListInput {
  sceneKey?: string;
  keyword?: string;
  page?: number;
  pageSize?: number;
}

export interface AdminDocumentTemplateListResult {
  items: Array<{
    templateId: string;
    sceneKey: string;
    sceneName: string;
    name: string;
    description: string;
    defaultTitle: string;
    sort: number;
    builtin: boolean;
    enabled: boolean;
    updatedAt: string;
  }>;
  pagination: {
    page: number;
    pageSize: number;
    total: number;
  };
}

export interface AdminDocumentTemplateScene {
  sceneKey: string;
  sceneName: string;
  description: string;
  sort: number;
  builtin: boolean;
  templateCount: number;
  createdByUserId?: string;
  updatedByUserId?: string;
  createdAt: string;
  updatedAt: string;
}

export interface AdminDocumentTemplateSceneListInput {
  keyword?: string;
  page?: number;
  pageSize?: number;
}

export interface AdminDocumentTemplateSceneListResult {
  items: Array<{
    sceneKey: string;
    sceneName: string;
    description: string;
    sort: number;
    builtin: boolean;
    templateCount: number;
    updatedAt: string;
  }>;
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

export type AdminSearchAnalyzerName = "jieba";
export type AdminSearchAnalyzerMode = "query" | "index";
export type AdminSearchAnalyzerDictStatus = "active" | "deleted";

export interface AdminSearchAnalyzerRecord {
  name: string;
  enabled: boolean;
  active: boolean;
  dictVersion: string;
  supportsUserDict: boolean;
  supportsHotReload: boolean;
  supportsPhraseHint: boolean;
  supportsStopwords: boolean;
  supportsSynonyms: boolean;
}

export interface AdminSearchAnalyzerDictEntry {
  id: number;
  analyzer: string;
  term: string;
  weight: number | null;
  tag: string;
  status: AdminSearchAnalyzerDictStatus;
  createdByUserId: string | null;
  updatedByUserId: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface AdminSearchAnalyzerDictListResult {
  items: AdminSearchAnalyzerDictEntry[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
  };
}

export interface AdminSearchAnalyzerReloadResult {
  analyzer: string;
  dictVersion: string;
  sourceVersion: number;
  loadedAt: string;
  activeAnalyzer: string;
}

export interface AdminSearchAnalyzerAnalyzePreviewResult {
  analyzer: string;
  mode: AdminSearchAnalyzerMode;
  tokens: string[];
  normalizedText: string;
  tokenCount: number;
  dictVersion: string;
}

export interface AdminDataRetentionCleanupPolicy {
  enabled: boolean;
  scheduleMinutes: number;
  cleanupBatchSize: number;
  cleanupTables: string[];
  auditLogRetentionDays: number;
  authCaptchaRetentionHours: number;
  authRiskStateRetentionDays: number;
  userSessionRetentionDays: number;
  documentRevisionRetentionCount: number;
}

export interface AdminDataRetentionCleanupResult {
  policy: AdminDataRetentionCleanupPolicy;
  startedAt: string;
  finishedAt: string;
  deletedAuditLogs: number;
  deletedAuthCaptchaChallenges: number;
  deletedAuthRiskStates: number;
  deletedUserSessions: number;
  deletedDocumentAttachments: number;
  deletedAttachmentBlobs: number;
  deletedDocumentImageAssets: number;
  deletedDocumentRevisions: number;
  totalDeleted: number;
}

export interface AdminSearchIndexRebuildResult {
  provider: string;
  indexedDocuments: number;
}

export interface AdminSearchIndexStatusResult {
  enabled: boolean;
  activeProvider: string;
  effectiveProvider: string;
  fallbackPolicy: string;
  activeAnalyzer: string;
  rebuildInProgress: boolean;
  providerHealthy: boolean;
  providerMessage: string;
  supportsDocCount: boolean;
  indexedDocuments: number;
  lastRebuildAt?: string | null;
  lastRebuildSource: string;
  lastRebuildIndexedDocuments: number;
}

export type AdminAuditModule =
  | "user"
  | "space"
  | "document"
  | "document_template"
  | "document_template_scene"
  | "theme"
  | "system_config";
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
  sendUserPasswordResetEmail(input: { userId: string }): Promise<void>;
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
  startSpaceExport(input: AdminSpaceExportStartInput): Promise<AdminSpaceExportStartResult>;
  subscribeSpaceExport(input: AdminSpaceTransferSubscribeInput): AdminSpaceTransferSubscription;
  listSpaceTransferTasks(input?: AdminSpaceTransferTaskListInput): Promise<AdminSpaceTransferTaskListResult>;
  getSpaceTransferTask(input: AdminSpaceTransferTokenInput): Promise<AdminSpaceTransferTaskResult>;
  issueSpaceTransferStreamToken(input: AdminSpaceTransferTokenInput): Promise<AdminSpaceTransferStreamTokenResult>;
  issueSpaceTransferDownloadToken(input: AdminSpaceTransferTokenInput): Promise<AdminSpaceTransferDownloadTokenResult>;
  inspectSpaceImport(input: { file: File }): Promise<AdminSpaceImportInspectResult>;
  commitSpaceImport(input: AdminSpaceImportCommitInput): Promise<AdminSpaceImportStartResult>;
  subscribeSpaceImport(input: AdminSpaceTransferSubscribeInput): AdminSpaceTransferSubscription;
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
  listDocumentShares(input?: AdminDocumentShareListInput): Promise<AdminDocumentShareListResult>;
  updateDocumentShare(input: {
    shareId: string;
    mode?: DocumentShareMode;
    password?: string;
    passwordHint?: string;
    expiresAt?: string | null;
  }): Promise<DocumentShareConfig>;
  deleteDocumentShare(shareId: string): Promise<void>;
  updateDocumentStatus(input: { documentId: string; status: "active" | "banned"; reason?: string }): Promise<AdminDocument>;
  deleteDocument(documentId: string): Promise<void>;
  listDocumentAttachments(input?: AdminDocumentAttachmentListInput): Promise<AdminDocumentAttachmentListResult>;
  deleteDocumentAttachment(input: {
    attachmentId: string;
    physicalDelete?: boolean;
    forcePhysicalDeleteOnShare?: boolean;
  }): Promise<AdminDocumentAttachmentDeleteResult>;
  listDocumentImages(input?: AdminDocumentImageAssetListInput): Promise<AdminDocumentImageAssetListResult>;
  deleteDocumentImage(input: {
    imageAssetId: string;
    physicalDelete?: boolean;
    forcePhysicalDeleteOnShare?: boolean;
  }): Promise<AdminDocumentImageAssetDeleteResult>;
  listDocumentTemplates(input?: AdminDocumentTemplateListInput): Promise<AdminDocumentTemplateListResult>;
  getDocumentTemplate(templateId: string): Promise<AdminDocumentTemplate>;
  listDocumentTemplateScenes(input?: AdminDocumentTemplateSceneListInput): Promise<AdminDocumentTemplateSceneListResult>;
  getDocumentTemplateScene(sceneKey: string): Promise<AdminDocumentTemplateScene>;
  createDocumentTemplateScene(input: {
    sceneKey: string;
    sceneName: string;
    description?: string;
    sort?: number;
  }): Promise<AdminDocumentTemplateScene>;
  updateDocumentTemplateScene(input: {
    sceneKey: string;
    sceneName?: string;
    description?: string;
    sort?: number;
  }): Promise<AdminDocumentTemplateScene>;
  deleteDocumentTemplateScene(sceneKey: string): Promise<void>;
  createDocumentTemplate(input: {
    templateId: string;
    sceneKey: string;
    name: string;
    description?: string;
    defaultTitle?: string;
    contentMd?: string;
    sort?: number;
    enabled?: boolean;
  }): Promise<AdminDocumentTemplate>;
  updateDocumentTemplate(input: {
    templateId: string;
    name?: string;
    description?: string;
    defaultTitle?: string;
    contentMd?: string;
    sort?: number;
    enabled?: boolean;
  }): Promise<AdminDocumentTemplate>;
  deleteDocumentTemplate(templateId: string): Promise<void>;
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
    configKey:
      | "site"
      | "editor"
      | "security"
      | "search"
      | "auth"
      | "email"
      | "onlyoffice"
      | "onlyoffice-integration"
      | "office-rendering"
      | "image-hosting"
      | "sitemap"
      | "data-retention";
    value: Record<string, unknown>;
    expectedVersion?: number;
  }): Promise<AdminSystemConfig>;
  listSearchAnalyzers(): Promise<AdminSearchAnalyzerRecord[]>;
  listSearchAnalyzerDictEntries(input: {
    analyzer: AdminSearchAnalyzerName;
    status?: "all" | AdminSearchAnalyzerDictStatus;
    page?: number;
    pageSize?: number;
  }): Promise<AdminSearchAnalyzerDictListResult>;
  createSearchAnalyzerDictEntry(input: {
    analyzer: AdminSearchAnalyzerName;
    term: string;
    weight?: number;
    tag?: string;
  }): Promise<AdminSearchAnalyzerDictEntry>;
  updateSearchAnalyzerDictEntry(input: {
    analyzer: AdminSearchAnalyzerName;
    entryId: number;
    term: string;
    weight?: number;
    tag?: string;
  }): Promise<AdminSearchAnalyzerDictEntry>;
  deleteSearchAnalyzerDictEntry(input: {
    analyzer: AdminSearchAnalyzerName;
    entryId: number;
  }): Promise<AdminSearchAnalyzerDictEntry>;
  reloadSearchAnalyzer(input: {
    analyzer: AdminSearchAnalyzerName;
  }): Promise<AdminSearchAnalyzerReloadResult>;
  analyzeSearchAnalyzerPreview(input: {
    analyzer: AdminSearchAnalyzerName;
    text: string;
    mode?: AdminSearchAnalyzerMode;
    language?: string;
    spaceId?: string;
  }): Promise<AdminSearchAnalyzerAnalyzePreviewResult>;
  runDataRetentionCleanup(): Promise<AdminDataRetentionCleanupResult>;
  runSearchIndexRebuild(): Promise<AdminSearchIndexRebuildResult>;
  getSearchIndexStatus(): Promise<AdminSearchIndexStatusResult>;
  testAuthLDAPConnection(input: {
    value: Record<string, unknown>;
    providerId?: string;
  }): Promise<{ ok: boolean }>;
  testSystemEmailSend(input: {
    value: Record<string, unknown>;
    toEmail: string;
  }): Promise<{ ok: boolean }>;
  listAudits(input?: AdminAuditListInput): Promise<AdminAuditListResult>;
}

export interface UploadLocalImageResult {
  key: string;
  url: string;
}

export interface IssueImageObjectKeyResult {
  provider: "cloudflare-r2" | "aliyun-oss" | "local";
  key: string;
}

export interface ImageHostingGateway {
  // 获取当前生效的图床配置（由后端系统配置统一管理）。
  getConfig(): Promise<Record<string, unknown>>;
  // 由后端统一分配图片对象 key（所有 provider 文件名均由后端生成）。
  issueObjectKey(input: {
    provider?: "cloudflare-r2" | "aliyun-oss" | "local";
    spaceId: string;
    docId?: string | null;
    fileName?: string;
    contentType?: string;
  }): Promise<IssueImageObjectKeyResult>;
  // 上传到本地图片存储（由后端接收 multipart/form-data 文件）。
  uploadLocalImage(
    file: File,
    uploadEndpoint?: string,
    options?: {
      spaceId?: string | null;
      docId?: string | null;
    }
  ): Promise<UploadLocalImageResult>;
}

export interface OnlyOfficeClientConfig {
  enabled: boolean;
}

export interface OnlyOfficeGateway {
  getConfig(): Promise<OnlyOfficeClientConfig>;
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
  documentTemplate: DocumentTemplateGateway;
  theme: ThemeGateway;
  admin: AdminGateway;
  imageHosting: ImageHostingGateway;
  onlyOffice: OnlyOfficeGateway;
  userConfig: UserConfigGateway;
}
