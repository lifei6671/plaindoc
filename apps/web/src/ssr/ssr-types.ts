export interface ReaderTreeNode {
  id: string;
  documentId?: string;
  documentIdentifier?: string;
  documentRouteKey?: string;
  documentFormat?: "markdown" | "docx" | "xlsx";
  parentId?: string | null;
  type: "folder" | "doc";
  title: string;
  sort: number;
  visibility?: "public" | "authenticated" | "member";
  children: ReaderTreeNode[];
}

export interface ReaderSpacePayload {
  id: string;
  name: string;
  title: string;
}

export interface ReaderDocumentPayload {
  id: string;
  nodeId: string;
  identifier?: string;
  routeKey: string;
  themeId: string;
  format?: "markdown" | "docx" | "xlsx";
  visibility: "public" | "authenticated" | "member";
  title: string;
  contentMd: string;
  renderStatus?: "idle" | "pending" | "success" | "failed";
  renderError?: string;
  renderedAt?: string;
  version: number;
  sourceBlobId?: string;
  sourceFileName?: string;
  sourceMimeType?: string;
  contentVersion?: number;
  authorNickname: string;
  updatedAt: string;
}

export interface ReaderDocumentAttachmentPayload {
  attachmentId: string;
  documentId: string;
  fileName: string;
  mimeType: string;
  sizeBytes: number;
  previewKind: "none" | "image" | "pdf" | "office" | "text";
  previewSupported: boolean;
}

export interface ReaderViewerPayload {
  userId?: string;
  name?: string;
  authenticated: boolean;
}

export interface ReaderAccessPayload {
  code: string;
  title: string;
  description: string;
  requiresLogin?: boolean;
}

export interface ReaderSharePayload {
  enabled: boolean;
  shareId?: string;
  spaceId?: string;
  documentRouteKey?: string;
  basePath?: string;
  attachmentBasePath?: string;
}

export interface ReaderPagePayload {
  space: ReaderSpacePayload;
  document: ReaderDocumentPayload;
  attachments: ReaderDocumentAttachmentPayload[];
  tree: ReaderTreeNode[];
  activeDocId: string;
  requestOrigin?: string;
  viewer: ReaderViewerPayload;
  officeRendering?: {
    independentRenderEnabled: boolean;
    fallbackToOnlyOfficeOnRenderFailure: boolean;
  };
  access?: ReaderAccessPayload;
  share?: ReaderSharePayload;
}
