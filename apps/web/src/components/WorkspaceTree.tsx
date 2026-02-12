import { ChevronDown, ChevronRight, FilePlus2, FolderPlus, PencilLine, Plus, Trash2 } from "lucide-react";
import {
  memo,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
  type MouseEvent,
  type ReactNode
} from "react";
import {
  ControlledTreeEnvironment,
  InteractionMode,
  Tree,
  type TreeInformation,
  type TreeItem,
  type TreeItemRenderContext,
  type TreeViewState
} from "react-complex-tree";
import type { CreateNodeResult, NodeType, TreeNode } from "../data-access";
import { formatError } from "../editor/status-utils";

const WORKSPACE_TREE_ID = "workspace-doc-tree";
const WORKSPACE_TREE_ROOT_ID = "__workspace_doc_tree_root__";
const DEFAULT_DOCUMENT_TITLE = "未命名文档";
const DEFAULT_FOLDER_TITLE = "未命名目录";

interface WorkspaceTreeItemData {
  nodeId: string | null;
  type: NodeType | "root";
  title: string;
}

interface WorkspaceTreeRenderItemProps {
  item: TreeItem<WorkspaceTreeItemData>;
  depth: number;
  children: ReactNode | null;
  title: ReactNode;
  arrow: ReactNode;
  context: TreeItemRenderContext;
  info: TreeInformation;
}

// 目录树组件入参：接收当前空间树结构与节点操作动作。
interface WorkspaceTreeProps {
  nodes: TreeNode[];
  activeDocId: string | null;
  onOpenDocument: (docId: string) => Promise<void>;
  onCreateNode: (input: {
    parentId: string | null;
    type: NodeType;
    title: string;
  }) => Promise<CreateNodeResult>;
  onRenameNode: (nodeId: string, title: string) => Promise<void>;
  onDeleteNode: (nodeId: string) => Promise<void>;
}

function mergeClassNames(...classNames: Array<string | false | null | undefined>): string {
  return classNames.filter(Boolean).join(" ");
}

// 收集可展开节点：初次渲染时默认展开全部目录类节点。
function collectExpandableNodeIds(nodes: TreeNode[]): string[] {
  const expandableNodeIds: string[] = [];
  const walk = (currentNodes: TreeNode[]) => {
    for (const node of currentNodes) {
      if (node.type === "folder" || node.children.length > 0) {
        expandableNodeIds.push(node.id);
      }
      if (node.children.length > 0) {
        walk(node.children);
      }
    }
  };
  walk(nodes);
  return expandableNodeIds;
}

// 构建 react-complex-tree 数据源：统一映射根节点与目录树索引。
function buildTreeItems(nodes: TreeNode[]): {
  items: Record<string, TreeItem<WorkspaceTreeItemData>>;
  nodeById: Map<string, TreeNode>;
} {
  const items: Record<string, TreeItem<WorkspaceTreeItemData>> = {
    [WORKSPACE_TREE_ROOT_ID]: {
      index: WORKSPACE_TREE_ROOT_ID,
      isFolder: true,
      children: nodes.map((node) => node.id),
      data: {
        nodeId: null,
        type: "root",
        title: "root"
      }
    }
  };
  const nodeById = new Map<string, TreeNode>();

  const walk = (currentNodes: TreeNode[]) => {
    for (const node of currentNodes) {
      nodeById.set(node.id, node);
      items[node.id] = {
        index: node.id,
        isFolder: node.type === "folder" || node.children.length > 0,
        canRename: false,
        children: node.children.map((childNode) => childNode.id),
        data: {
          nodeId: node.id,
          type: node.type,
          title: node.title
        }
      };
      if (node.children.length > 0) {
        walk(node.children);
      }
    }
  };

  walk(nodes);
  return { items, nodeById };
}

// 统计子树规模：删除确认时用于提示联动删除范围。
function countDescendants(node: TreeNode): number {
  let descendantCount = 0;
  const walk = (currentNode: TreeNode) => {
    for (const childNode of currentNode.children) {
      descendantCount += 1;
      walk(childNode);
    }
  };
  walk(node);
  return descendantCount;
}

// 目录树容器：使用 React Complex Tree 承载交互和可扩展能力。
export const WorkspaceTree = memo(function WorkspaceTree({
  nodes,
  activeDocId,
  onOpenDocument,
  onCreateNode,
  onRenameNode,
  onDeleteNode
}: WorkspaceTreeProps) {
  const { items, nodeById } = useMemo(() => buildTreeItems(nodes), [nodes]);
  const expandableNodeIds = useMemo(() => collectExpandableNodeIds(nodes), [nodes]);
  const knownExpandableNodeIdsRef = useRef<Set<string>>(new Set());
  const actionMenuRootRef = useRef<HTMLDivElement | null>(null);
  const inlineEditInputRef = useRef<HTMLInputElement | null>(null);
  const pendingInlineEditFocusNodeIdRef = useRef<string | null>(null);
  const isCommittingInlineEditRef = useRef(false);
  const [expandedNodeIds, setExpandedNodeIds] = useState<string[]>(expandableNodeIds);
  const [openActionNodeId, setOpenActionNodeId] = useState<string | null>(null);
  const [editingNodeId, setEditingNodeId] = useState<string | null>(null);
  const [editingNodeTitle, setEditingNodeTitle] = useState("");

  // 树结构变化时更新展开状态：保留用户折叠选择，仅默认展开新出现的目录。
  useEffect(() => {
    const currentExpandableNodeIdSet = new Set(expandableNodeIds);
    const newNodeIds = expandableNodeIds.filter((nodeId) => !knownExpandableNodeIdsRef.current.has(nodeId));
    setExpandedNodeIds((previousExpandedNodeIds) => {
      const remainingNodeIds = previousExpandedNodeIds.filter((nodeId) =>
        currentExpandableNodeIdSet.has(nodeId)
      );
      return [...remainingNodeIds, ...newNodeIds];
    });
    knownExpandableNodeIdsRef.current = currentExpandableNodeIdSet;
  }, [expandableNodeIds]);

  // 目录刷新后若菜单目标已不存在，自动关闭动作菜单。
  useEffect(() => {
    if (!openActionNodeId) {
      return;
    }
    if (!nodeById.has(openActionNodeId)) {
      setOpenActionNodeId(null);
    }
  }, [nodeById, openActionNodeId]);

  // 菜单展开后监听外部点击与 Esc：确保菜单能被快速收起。
  useEffect(() => {
    if (!openActionNodeId) {
      return;
    }
    const handleWindowPointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) {
        setOpenActionNodeId(null);
        return;
      }
      if (actionMenuRootRef.current?.contains(target)) {
        return;
      }
      setOpenActionNodeId(null);
    };
    const handleWindowKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpenActionNodeId(null);
      }
    };

    window.addEventListener("pointerdown", handleWindowPointerDown);
    window.addEventListener("keydown", handleWindowKeyDown);
    return () => {
      window.removeEventListener("pointerdown", handleWindowPointerDown);
      window.removeEventListener("keydown", handleWindowKeyDown);
    };
  }, [openActionNodeId]);

  // 若正在编辑的节点被删除或不可见，自动退出编辑态，避免残留脏状态。
  useEffect(() => {
    if (!editingNodeId) {
      return;
    }
    if (pendingInlineEditFocusNodeIdRef.current === editingNodeId) {
      return;
    }
    if (!nodeById.has(editingNodeId)) {
      setEditingNodeId(null);
      setEditingNodeTitle("");
    }
  }, [editingNodeId, nodeById]);

  // 进入编辑态后自动聚焦并选中文本，保证“创建即改名”流程顺滑。
  useEffect(() => {
    if (!editingNodeId) {
      return;
    }
    if (pendingInlineEditFocusNodeIdRef.current !== editingNodeId) {
      return;
    }
    const frameId = window.requestAnimationFrame(() => {
      const inputElement = inlineEditInputRef.current;
      if (!inputElement) {
        return;
      }
      inputElement.focus();
      inputElement.select();
      pendingInlineEditFocusNodeIdRef.current = null;
    });
    return () => {
      window.cancelAnimationFrame(frameId);
    };
  }, [editingNodeId]);

  const viewState = useMemo<TreeViewState>(() => {
    return {
      [WORKSPACE_TREE_ID]: {
        expandedItems: expandedNodeIds,
        selectedItems: activeDocId ? [activeDocId] : [],
        focusedItem: activeDocId ?? undefined
      }
    };
  }, [activeDocId, expandedNodeIds]);

  // 阻止菜单按钮冒泡到树项主操作，避免误触发文档打开。
  const stopTreeItemEvent = useCallback((event: MouseEvent<HTMLElement>) => {
    event.preventDefault();
    event.stopPropagation();
  }, []);

  // 输入框仅阻止冒泡，保留默认行为（聚焦、文本选择等）。
  const stopTreeItemPropagation = useCallback((event: MouseEvent<HTMLElement>) => {
    event.stopPropagation();
  }, []);

  // 封装菜单动作执行：统一处理错误提示和菜单收起行为。
  const runActionMenuTask = useCallback(async (task: () => Promise<void>) => {
    try {
      await task();
      setOpenActionNodeId(null);
    } catch (error) {
      window.alert(`操作失败：${formatError(error)}`);
    }
  }, []);

  const beginInlineEdit = useCallback((nodeId: string, initialTitle: string) => {
    pendingInlineEditFocusNodeIdRef.current = nodeId;
    setEditingNodeId(nodeId);
    setEditingNodeTitle(initialTitle);
  }, []);

  const cancelInlineEdit = useCallback(() => {
    pendingInlineEditFocusNodeIdRef.current = null;
    setEditingNodeId(null);
    setEditingNodeTitle("");
  }, []);

  const commitInlineEdit = useCallback(async () => {
    if (!editingNodeId || isCommittingInlineEditRef.current) {
      return;
    }

    const currentNode = nodeById.get(editingNodeId);
    if (!currentNode) {
      cancelInlineEdit();
      return;
    }

    const fallbackTitle = currentNode.type === "folder" ? DEFAULT_FOLDER_TITLE : DEFAULT_DOCUMENT_TITLE;
    const normalizedTitle = editingNodeTitle.trim() || fallbackTitle;

    if (normalizedTitle === currentNode.title) {
      cancelInlineEdit();
      return;
    }

    isCommittingInlineEditRef.current = true;
    try {
      await onRenameNode(editingNodeId, normalizedTitle);
    } catch (error) {
      window.alert(`重命名失败：${formatError(error)}`);
    } finally {
      isCommittingInlineEditRef.current = false;
      cancelInlineEdit();
    }
  }, [cancelInlineEdit, editingNodeId, editingNodeTitle, nodeById, onRenameNode]);

  const createNodeAndEnterInlineEdit = useCallback(
    async (input: { parentId: string | null; type: NodeType; title: string }) => {
      try {
        const created = await onCreateNode(input);
        if (input.parentId) {
          setExpandedNodeIds((previousExpandedNodeIds) => {
            if (previousExpandedNodeIds.includes(input.parentId!)) {
              return previousExpandedNodeIds;
            }
            return [...previousExpandedNodeIds, input.parentId!];
          });
        }
        setOpenActionNodeId(null);
        beginInlineEdit(created.nodeId, input.title);
      } catch (error) {
        window.alert(`操作失败：${formatError(error)}`);
      }
    },
    [beginInlineEdit, onCreateNode]
  );

  const handleExpandNode = useCallback((item: TreeItem<WorkspaceTreeItemData>) => {
    const nodeId = String(item.index);
    setExpandedNodeIds((previousExpandedNodeIds) => {
      if (previousExpandedNodeIds.includes(nodeId)) {
        return previousExpandedNodeIds;
      }
      return [...previousExpandedNodeIds, nodeId];
    });
  }, []);

  const handleCollapseNode = useCallback((item: TreeItem<WorkspaceTreeItemData>) => {
    const nodeId = String(item.index);
    setExpandedNodeIds((previousExpandedNodeIds) =>
      previousExpandedNodeIds.filter((expandedNodeId) => expandedNodeId !== nodeId)
    );
  }, []);

  // 主操作仅用于打开文档；目录展开收起交给箭头交互管理。
  const handlePrimaryAction = useCallback(
    (item: TreeItem<WorkspaceTreeItemData>) => {
      if (item.data.type !== "doc" || !item.data.nodeId) {
        return;
      }
      setOpenActionNodeId(null);
      void onOpenDocument(item.data.nodeId);
    },
    [onOpenDocument]
  );

  const handleCreateChildDocument = useCallback(
    async (nodeId: string): Promise<void> => {
      await createNodeAndEnterInlineEdit({
        parentId: nodeId,
        type: "doc",
        title: DEFAULT_DOCUMENT_TITLE
      });
    },
    [createNodeAndEnterInlineEdit]
  );

  const handleCreateSiblingDocument = useCallback(
    async (nodeId: string): Promise<void> => {
      const currentNode = nodeById.get(nodeId);
      if (!currentNode) {
        throw new Error("目录节点不存在");
      }
      await createNodeAndEnterInlineEdit({
        parentId: currentNode.parentId,
        type: "doc",
        title: DEFAULT_DOCUMENT_TITLE
      });
    },
    [createNodeAndEnterInlineEdit, nodeById]
  );

  const handleCreateChildFolder = useCallback(
    async (nodeId: string): Promise<void> => {
      await createNodeAndEnterInlineEdit({
        parentId: nodeId,
        type: "folder",
        title: DEFAULT_FOLDER_TITLE
      });
    },
    [createNodeAndEnterInlineEdit]
  );

  const handleRenameNode = useCallback(
    async (nodeId: string): Promise<void> => {
      const currentNode = nodeById.get(nodeId);
      if (!currentNode) {
        throw new Error("目录节点不存在");
      }
      beginInlineEdit(nodeId, currentNode.title);
    },
    [beginInlineEdit, nodeById]
  );

  const handleDeleteNode = useCallback(
    async (nodeId: string): Promise<void> => {
      const currentNode = nodeById.get(nodeId);
      if (!currentNode) {
        throw new Error("目录节点不存在");
      }
      const descendantCount = countDescendants(currentNode);
      const baseTitle = currentNode.title || DEFAULT_DOCUMENT_TITLE;
      const confirmMessage =
        descendantCount > 0
          ? `确认删除「${baseTitle}」吗？该操作会同时移除 ${descendantCount} 个子节点。`
          : `确认删除「${baseTitle}」吗？`;
      if (!window.confirm(confirmMessage)) {
        return;
      }
      await onDeleteNode(nodeId);
    },
    [nodeById, onDeleteNode]
  );

  const renderTreeItem = useCallback(
    ({
      item,
      depth,
      children,
      title,
      context
    }: WorkspaceTreeRenderItemProps) => {
      const nodeId = item.data.nodeId;
      if (!nodeId) {
        return (
          <li {...(context.itemContainerWithChildrenProps as any)} className="m-0 p-0">
            {children}
          </li>
        );
      }
      const isFolder = item.data.type === "folder" || item.isFolder;
      const isActive = nodeId === activeDocId;
      const isActionMenuOpen = openActionNodeId === nodeId;
      const isInlineEditing = editingNodeId === nodeId;
      const rowStyle = {
        ...(context.itemContainerWithoutChildrenProps.style ?? {}),
        paddingLeft: `${8 + depth * 20}px`,
        cursor: "pointer"
      };
      const interactiveType = context.isRenaming || isInlineEditing ? undefined : "button";
      const InteractiveComponent = context.isRenaming || isInlineEditing ? "div" : "button";

      return (
        <li {...(context.itemContainerWithChildrenProps as any)} className="m-0 p-0">
          <div
            {...(context.itemContainerWithoutChildrenProps as any)}
            className={mergeClassNames(
              "group relative flex min-h-[36px] w-full cursor-pointer items-center rounded-[10px] pr-2 text-[14px] text-[#2f2f30]",
              "bg-transparent hover:bg-[#e8e8ea]",
              isActive && "bg-[#d9dade]",
              context.isFocused && "outline-none"
            )}
            style={rowStyle}
          >
              <span
                {...(isFolder ? context.arrowProps : {})}
                className={mergeClassNames(
                  "inline-flex h-[18px] w-[18px] shrink-0 items-center justify-center rounded-[6px] text-[#727679]",
                  isFolder ? "!cursor-pointer hover:bg-[#dde0e4]" : "pointer-events-none opacity-0"
                )}
                aria-hidden="true"
              >
              {isFolder ? context.isExpanded ? <ChevronDown size={15} /> : <ChevronRight size={15} /> : null}
            </span>
            <InteractiveComponent
              type={interactiveType}
              {...(!isInlineEditing ? (context.interactiveElementProps as any) : {})}
              className="flex min-h-[36px] min-w-0 flex-1 !cursor-pointer items-center border-0 bg-transparent p-0 text-left text-[14px] text-[#2f2f30] focus-visible:outline-none disabled:!cursor-pointer"
            >
              {isInlineEditing ? (
                <input
                  ref={inlineEditInputRef}
                  value={editingNodeTitle}
                  className="h-[28px] w-full rounded-[8px] border border-[#c8cdd2] bg-white px-2 text-[13px] leading-[1.2] text-[#1f2328] outline-none focus:border-[#8ea8c4]"
                  aria-label="输入文档名称"
                  onMouseDown={stopTreeItemPropagation}
                  onClick={stopTreeItemPropagation}
                  onChange={(event) => {
                    setEditingNodeTitle(event.target.value);
                  }}
                  onKeyDown={(event: ReactKeyboardEvent<HTMLInputElement>) => {
                    if (event.key === "Enter") {
                      event.preventDefault();
                      void commitInlineEdit();
                      return;
                    }
                    if (event.key === "Escape") {
                      event.preventDefault();
                      cancelInlineEdit();
                    }
                  }}
                  onBlur={() => {
                    void commitInlineEdit();
                  }}
                />
              ) : (
                <span
                  className={mergeClassNames(
                    "min-w-0 truncate leading-[1.3]",
                    isActive && "font-semibold"
                  )}
                  title={item.data.title}
                >
                  {title}
                </span>
              )}
            </InteractiveComponent>
            {isInlineEditing ? null : (
              <div className="relative ml-1.5 inline-flex items-center" ref={isActionMenuOpen ? actionMenuRootRef : undefined}>
                <button
                  type="button"
                  className={mergeClassNames(
                    "inline-flex h-[26px] w-[26px] items-center justify-center rounded-[8px] border-0 bg-transparent text-[#71767a]",
                    "transition-[opacity,background-color,color] duration-100",
                    "hover:bg-[#dde0e4] hover:text-[#3e4247] focus-visible:outline-2 focus-visible:outline-[#8ea8c4] focus-visible:outline-offset-[-1px]",
                    isActionMenuOpen
                      ? "pointer-events-auto opacity-100"
                      : "pointer-events-none opacity-0 group-hover:pointer-events-auto group-hover:opacity-100 focus-visible:pointer-events-auto focus-visible:opacity-100"
                  )}
                  aria-label="打开文档操作菜单"
                  onMouseDown={stopTreeItemEvent}
                  onClick={(event) => {
                    stopTreeItemEvent(event);
                    setOpenActionNodeId((previousNodeId) => (previousNodeId === nodeId ? null : nodeId));
                  }}
                >
                  <Plus size={14} />
                </button>
                {isActionMenuOpen ? (
                  <div
                    className="absolute top-[calc(100%+6px)] right-0 z-30 flex min-w-[196px] flex-col gap-0.5 rounded-[12px] bg-white p-1.5 shadow-[0_14px_30px_rgba(15,23,42,0.16)]"
                    role="menu"
                    aria-label="文档操作菜单"
                  >
                    <button
                      type="button"
                      className="inline-flex min-h-[34px] w-full items-center gap-2 rounded-[8px] border-0 bg-transparent px-2.5 text-left text-[13px] text-[#2f2f30] hover:bg-[#f0f2f4] focus-visible:outline-2 focus-visible:outline-[#8ea8c4] focus-visible:outline-offset-[-1px]"
                      role="menuitem"
                      onMouseDown={stopTreeItemEvent}
                      onClick={(event) => {
                        stopTreeItemEvent(event);
                        void handleCreateChildDocument(nodeId);
                      }}
                    >
                      <FilePlus2 size={14} />
                      <span>新建子文档</span>
                    </button>
                    <button
                      type="button"
                      className="inline-flex min-h-[34px] w-full items-center gap-2 rounded-[8px] border-0 bg-transparent px-2.5 text-left text-[13px] text-[#2f2f30] hover:bg-[#f0f2f4] focus-visible:outline-2 focus-visible:outline-[#8ea8c4] focus-visible:outline-offset-[-1px]"
                      role="menuitem"
                      onMouseDown={stopTreeItemEvent}
                      onClick={(event) => {
                        stopTreeItemEvent(event);
                        void handleCreateSiblingDocument(nodeId);
                      }}
                    >
                      <FilePlus2 size={14} />
                      <span>新建同级文档</span>
                    </button>
                    <button
                      type="button"
                      className="inline-flex min-h-[34px] w-full items-center gap-2 rounded-[8px] border-0 bg-transparent px-2.5 text-left text-[13px] text-[#2f2f30] hover:bg-[#f0f2f4] focus-visible:outline-2 focus-visible:outline-[#8ea8c4] focus-visible:outline-offset-[-1px]"
                      role="menuitem"
                      onMouseDown={stopTreeItemEvent}
                      onClick={(event) => {
                        stopTreeItemEvent(event);
                        void handleCreateChildFolder(nodeId);
                      }}
                    >
                      <FolderPlus size={14} />
                      <span>新建子目录</span>
                    </button>
                    <button
                      type="button"
                      className="inline-flex min-h-[34px] w-full items-center gap-2 rounded-[8px] border-0 bg-transparent px-2.5 text-left text-[13px] text-[#2f2f30] hover:bg-[#f0f2f4] focus-visible:outline-2 focus-visible:outline-[#8ea8c4] focus-visible:outline-offset-[-1px]"
                      role="menuitem"
                      onMouseDown={stopTreeItemEvent}
                      onClick={(event) => {
                        stopTreeItemEvent(event);
                        void runActionMenuTask(() => handleRenameNode(nodeId));
                      }}
                    >
                      <PencilLine size={14} />
                      <span>重命名</span>
                    </button>
                    <button
                      type="button"
                      className="inline-flex min-h-[34px] w-full items-center gap-2 rounded-[8px] border-0 bg-transparent px-2.5 text-left text-[13px] text-[#b42318] hover:bg-[#fff0ef] focus-visible:outline-2 focus-visible:outline-[#8ea8c4] focus-visible:outline-offset-[-1px]"
                      role="menuitem"
                      onMouseDown={stopTreeItemEvent}
                      onClick={(event) => {
                        stopTreeItemEvent(event);
                        void runActionMenuTask(() => handleDeleteNode(nodeId));
                      }}
                    >
                      <Trash2 size={14} />
                      <span>删除</span>
                    </button>
                  </div>
                ) : null}
              </div>
            )}
          </div>
          {children}
        </li>
      );
    },
    [
      activeDocId,
      handleCreateChildDocument,
      handleCreateChildFolder,
      handleCreateSiblingDocument,
      handleDeleteNode,
      handleRenameNode,
      cancelInlineEdit,
      commitInlineEdit,
      editingNodeId,
      editingNodeTitle,
      openActionNodeId,
      runActionMenuTask,
      stopTreeItemPropagation,
      stopTreeItemEvent
    ]
  );

  if (nodes.length === 0) {
    return <p className="mt-2.5 mr-2 mb-0 ml-2 text-[14px] text-[#8a8d90]">当前空间暂无文档。</p>;
  }

  return (
    <ControlledTreeEnvironment<WorkspaceTreeItemData>
      items={items}
      getItemTitle={(item) => item.data.title}
      viewState={viewState}
      defaultInteractionMode={InteractionMode.ClickArrowToExpand}
      canDragAndDrop={false}
      canDropOnFolder={false}
      canReorderItems={false}
      canSearch={false}
      canRename={false}
      // 允许“可展开文档”点击触发主动作，避免仅叶子文档可打开。
      canInvokePrimaryActionOnItemContainer
      onExpandItem={handleExpandNode}
      onCollapseItem={handleCollapseNode}
      onPrimaryAction={handlePrimaryAction}
      renderTreeContainer={({ children, containerProps }) => (
        <div {...containerProps} className={mergeClassNames("min-h-0", containerProps.className)}>
          {children}
        </div>
      )}
      renderItemsContainer={({ children, containerProps, depth }) => (
        <ul
          {...containerProps}
          className={mergeClassNames(
            depth > 0 ? "mt-px m-0 list-none p-0" : "m-0 list-none p-0",
            containerProps.className
          )}
        >
          {children}
        </ul>
      )}
      renderItem={renderTreeItem}
    >
      <Tree treeId={WORKSPACE_TREE_ID} rootItem={WORKSPACE_TREE_ROOT_ID} treeLabel="工作区目录树" />
    </ControlledTreeEnvironment>
  );
});
