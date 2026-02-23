export interface ReaderTreeNode {
  id: string;
  parentId?: string | null;
  type: "folder" | "doc";
  title: string;
  sort: number;
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

export interface ReaderPagePayload {
  space: ReaderSpacePayload;
  document: ReaderDocumentPayload;
  tree: ReaderTreeNode[];
  activeDocId: string;
  viewer: ReaderViewerPayload;
}
