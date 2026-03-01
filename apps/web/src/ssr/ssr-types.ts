export interface ReaderTreeNode {
  id: string;
  documentId?: string;
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
  themeId: string;
  visibility: "public" | "authenticated" | "member";
  title: string;
  contentMd: string;
  version: number;
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

export interface ReaderPagePayload {
  space: ReaderSpacePayload;
  document: ReaderDocumentPayload;
  attachments: ReaderDocumentAttachmentPayload[];
  tree: ReaderTreeNode[];
  activeDocId: string;
  requestOrigin?: string;
  viewer: ReaderViewerPayload;
  access?: ReaderAccessPayload;
}
