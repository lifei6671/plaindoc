import type { Dispatch, SetStateAction } from "react";
import type { CreateNodeResult, DataGateway, Document, NodeType, Space, TreeNode } from "../data-access";

// useWorkspace 初始化参数：由上层注入网关与默认展示文案。
export interface UseWorkspaceOptions {
  dataGateway: DataGateway;
  initialContent: string;
  initialSpaceName?: string;
  initialDocumentTitle?: string;
  defaultSpaceName?: string;
  defaultDocumentTitle?: string;
}

// 工作区核心状态：覆盖空间、目录树和当前文档快照。
export interface WorkspaceState {
  spaces: Space[];
  activeSpaceId: string | null;
  activeSpaceName: string;
  workspaceTree: TreeNode[];
  activeDocId: string | null;
  activeDocumentTitle: string;
  content: string;
  baseVersion: number;
  lastSavedContent: string;
  lastSavedAt: string | null;
}

// 工作区运行态：用于描述启动过程与错误信息。
export interface WorkspaceRuntimeState {
  isWorkspaceBootstrapping: boolean;
  workspaceErrorMessage: string | null;
}

// 启动完成返回值：供上层状态栏展示版本等信息。
export interface WorkspaceBootstrapResult {
  spaceId: string;
  spaceName: string;
  docId: string;
  documentVersion: number;
}

// 目录移动参数：作为拖拽排序扩展点的统一输入结构。
export interface WorkspaceMoveNodeInput {
  nodeId: string;
  targetParentId: string | null;
  targetSort?: number;
}

// 目录树新增参数：用于创建文档或目录节点。
export interface WorkspaceCreateNodeInput {
  parentId: string | null;
  type: NodeType;
  title: string;
}

// 工作区动作：统一封装目录树和文档加载行为。
export interface WorkspaceActions {
  bootstrapWorkspace(): Promise<WorkspaceBootstrapResult>;
  refreshSpaces(): Promise<Space[]>;
  switchSpace(spaceId: string): Promise<WorkspaceBootstrapResult>;
  createSpace(spaceName: string): Promise<WorkspaceBootstrapResult>;
  createNode(input: WorkspaceCreateNodeInput): Promise<CreateNodeResult>;
  renameNode(nodeId: string, title: string): Promise<void>;
  deleteNode(nodeId: string): Promise<void>;
  openDocument(docId: string): Promise<Document>;
  reloadTree(spaceId?: string): Promise<TreeNode[]>;
  moveNode(input: WorkspaceMoveNodeInput): Promise<void>;
  deleteSpace(spaceId: string): Promise<void>;
}

// 工作区状态写入器：供保存链路复用，避免上层重复定义状态。
export interface WorkspaceSetters {
  setActiveSpaceName: Dispatch<SetStateAction<string>>;
  setActiveDocumentTitle: Dispatch<SetStateAction<string>>;
  setContent: Dispatch<SetStateAction<string>>;
  setBaseVersion: Dispatch<SetStateAction<number>>;
  setLastSavedContent: Dispatch<SetStateAction<string>>;
  setLastSavedAt: Dispatch<SetStateAction<string | null>>;
}

// Hook 返回结构：状态 + 运行态 + 动作 + setter。
export type UseWorkspaceResult = WorkspaceState &
  WorkspaceRuntimeState &
  WorkspaceActions &
  WorkspaceSetters;
