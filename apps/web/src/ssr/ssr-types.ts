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
  updatedAt: string;
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
  tree: ReaderTreeNode[];
  activeDocId: string;
  viewer: ReaderViewerPayload;
  access?: ReaderAccessPayload;
}
