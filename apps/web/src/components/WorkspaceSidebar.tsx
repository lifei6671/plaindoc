import { memo } from "react";
import type { NodeType, TreeNode } from "../data-access";
import { WorkspaceTree } from "./WorkspaceTree";

// 工作区侧边栏入参：仅管理“当前空间”文档，不提供空间维护能力。
interface WorkspaceSidebarProps {
  activeSpaceName: string;
  workspaceTree: TreeNode[];
  onOpenDocument: (docId: string) => Promise<void>;
  onCreateNode: (input: {
    parentId: string | null;
    type: NodeType;
    title: string;
  }) => Promise<void>;
  onRenameNode: (nodeId: string, title: string) => Promise<void>;
  onDeleteNode: (nodeId: string) => Promise<void>;
  activeDocId: string | null;
}

// 工作区侧边栏：仅承载当前空间目录树，不含顶部空间管理头部。
export const WorkspaceSidebar = memo(function WorkspaceSidebar({
  activeSpaceName,
  workspaceTree,
  onOpenDocument,
  onCreateNode,
  onRenameNode,
  onDeleteNode,
  activeDocId
}: WorkspaceSidebarProps) {
  return (
    <aside
      className="flex h-full min-h-0 max-h-[230px] flex-col overflow-auto bg-[#f3f3f4] p-3 transition-opacity duration-150 md:max-h-none md:px-[10px] md:pt-2 md:pb-3"
      aria-label={`${activeSpaceName} 目录`}
    >
      <WorkspaceTree
        nodes={workspaceTree}
        activeDocId={activeDocId}
        onOpenDocument={onOpenDocument}
        onCreateNode={onCreateNode}
        onRenameNode={onRenameNode}
        onDeleteNode={onDeleteNode}
      />
    </aside>
  );
});
