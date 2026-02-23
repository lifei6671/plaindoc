import { ChevronDown, ChevronRight, FilePlus2, FolderPlus, PencilLine, Plus, Trash2 } from "lucide-react";
import {
  memo,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type FocusEvent as ReactFocusEvent,
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
import { useConfirmDialog } from "./ConfirmDialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger
} from "./ui/dropdown-menu";

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

interface PendingCreateDraftNode {
  nodeId: string;
  parentId: string | null;
  type: NodeType;
  title: string;
}

function mergeClassNames(...classNames: Array<string | false | null | undefined>): string {
  return classNames.filter(Boolean).join(" ");
}

// 收集可展开节点：用于在树结构刷新后过滤无效展开项。
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

function cloneTreeNodes(nodes: TreeNode[]): TreeNode[] {
  return nodes.map((node) => ({
    ...node,
    children: cloneTreeNodes(node.children)
  }));
}

function findNodeByID(nodes: TreeNode[], targetNodeID: string): TreeNode | null {
  for (const node of nodes) {
    if (node.id === targetNodeID) {
      return node;
    }
    if (node.children.length > 0) {
      const matchedChildNode = findNodeByID(node.children, targetNodeID);
      if (matchedChildNode) {
        return matchedChildNode;
      }
    }
  }
  return null;
}

function buildDraftNodeID(): string {
  const randomValue = Math.random().toString(36).slice(2, 10);
  return `draft-${Date.now().toString(36)}-${randomValue}`;
}

function mergeDraftNodesIntoTree(
  baseNodes: TreeNode[],
  draftNodes: PendingCreateDraftNode[]
): TreeNode[] {
  if (draftNodes.length === 0) {
    return baseNodes;
  }

  const mergedNodes = cloneTreeNodes(baseNodes);
  for (const draftNode of draftNodes) {
    let spaceID = "";
    if (draftNode.parentId) {
      const parentNode = findNodeByID(mergedNodes, draftNode.parentId);
      if (parentNode) {
        spaceID = parentNode.spaceId;
      }
    }
    if (!spaceID && mergedNodes.length > 0) {
      spaceID = mergedNodes[0].spaceId;
    }

    const nextDraftNode: TreeNode = {
      id: draftNode.nodeId,
      spaceId: spaceID,
      parentId: draftNode.parentId,
      type: draftNode.type,
      title: draftNode.title,
      sort: Number.MAX_SAFE_INTEGER,
      children: []
    };

    if (!draftNode.parentId) {
      mergedNodes.push(nextDraftNode);
      continue;
    }
    const parentNode = findNodeByID(mergedNodes, draftNode.parentId);
    if (!parentNode) {
      mergedNodes.push(nextDraftNode);
      continue;
    }
    parentNode.children.push(nextDraftNode);
  }

  return mergedNodes;
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
  const { confirm: confirmByModal, dialog: confirmDialog } = useConfirmDialog();
  const [draftNodes, setDraftNodes] = useState<PendingCreateDraftNode[]>([]);
  const mergedNodes = useMemo(
    () => mergeDraftNodesIntoTree(nodes, draftNodes),
    [draftNodes, nodes]
  );
  const { items, nodeById } = useMemo(() => buildTreeItems(mergedNodes), [mergedNodes]);
  const expandableNodeIds = useMemo(() => collectExpandableNodeIds(mergedNodes), [mergedNodes]);
  const draftNodeByID = useMemo(() => {
    const mappedDraftNodes = new Map<string, PendingCreateDraftNode>();
    for (const draftNode of draftNodes) {
      mappedDraftNodes.set(draftNode.nodeId, draftNode);
    }
    return mappedDraftNodes;
  }, [draftNodes]);
  const knownExpandableNodeIdsRef = useRef<Set<string>>(new Set());
  const actionMenuRootRef = useRef<HTMLDivElement | null>(null);
  const inlineEditInputRef = useRef<HTMLInputElement | null>(null);
  const pendingInlineEditFocusNodeIdRef = useRef<string | null>(null);
  const isCommittingInlineEditRef = useRef(false);
  // 默认全折叠：首次进入目录树时不自动展开任何节点。
  const [expandedNodeIds, setExpandedNodeIds] = useState<string[]>([]);
  const [openActionNodeId, setOpenActionNodeId] = useState<string | null>(null);
  const [editingNodeId, setEditingNodeId] = useState<string | null>(null);
  const [editingNodeTitle, setEditingNodeTitle] = useState("");
  const [creatingDraftNodeIds, setCreatingDraftNodeIds] = useState<string[]>([]);
  const [isCreatingFirstDocument, setIsCreatingFirstDocument] = useState(false);
  const creatingDraftNodeIdSet = useMemo(() => new Set(creatingDraftNodeIds), [creatingDraftNodeIds]);

  // 树结构变化时仅保留仍然存在的展开节点，不自动展开新节点。
  // 这样可以保持“默认全折叠”与“用户手动展开优先”。
  useEffect(() => {
    const currentExpandableNodeIdSet = new Set(expandableNodeIds);
    setExpandedNodeIds((previousExpandedNodeIds) => {
      return previousExpandedNodeIds.filter((nodeId) => currentExpandableNodeIdSet.has(nodeId));
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
    let canceled = false;
    let timeoutId: number | null = null;
    const rafIds: number[] = [];
    let attemptCount = 0;

    const runFocusAttempt = () => {
      if (canceled) {
        return;
      }
      const inputElement = inlineEditInputRef.current;
      if (inputElement) {
        inputElement.focus();
        inputElement.select();
        pendingInlineEditFocusNodeIdRef.current = null;
        return;
      }

      attemptCount += 1;
      if (attemptCount >= 8) {
        return;
      }
      timeoutId = window.setTimeout(() => {
        rafIds.push(window.requestAnimationFrame(runFocusAttempt));
      }, 16);
    };

    rafIds.push(window.requestAnimationFrame(runFocusAttempt));
    return () => {
      canceled = true;
      for (const rafId of rafIds) {
        window.cancelAnimationFrame(rafId);
      }
      if (timeoutId !== null) {
        window.clearTimeout(timeoutId);
      }
    };
  }, [editingNodeId, items]);

  const viewState = useMemo<TreeViewState>(() => {
    // 行内编辑期间不让树组件接管焦点，避免输入框因焦点切换触发 onBlur 误提交。
    const focusedItem = editingNodeId
      ? undefined
      : activeDocId && items[activeDocId]
        ? activeDocId
        : undefined;
    return {
      [WORKSPACE_TREE_ID]: {
        expandedItems: expandedNodeIds,
        selectedItems: activeDocId ? [activeDocId] : [],
        focusedItem
      }
    };
  }, [activeDocId, editingNodeId, expandedNodeIds, items]);

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

  const removeDraftNode = useCallback((nodeId: string) => {
    setDraftNodes((previousDraftNodes) =>
      previousDraftNodes.filter((draftNode) => draftNode.nodeId !== nodeId)
    );
    setCreatingDraftNodeIds((previousNodeIds) =>
      previousNodeIds.filter((draftNodeID) => draftNodeID !== nodeId)
    );
  }, []);

  const commitInlineEdit = useCallback(async () => {
    if (!editingNodeId || isCommittingInlineEditRef.current) {
      return;
    }

    const editingDraftNode = draftNodeByID.get(editingNodeId);
    if (editingDraftNode) {
      const fallbackTitle =
        editingDraftNode.type === "folder" ? DEFAULT_FOLDER_TITLE : DEFAULT_DOCUMENT_TITLE;
      const normalizedTitle = editingNodeTitle.trim() || fallbackTitle;
      const draftNodeID = editingNodeId;

      isCommittingInlineEditRef.current = true;
      try {
        // 先退出输入态并保留草稿节点占位，避免“节点消失后再出现”的闪烁。
        setDraftNodes((previousDraftNodes) =>
          previousDraftNodes.map((draftNode) =>
            draftNode.nodeId === draftNodeID ? { ...draftNode, title: normalizedTitle } : draftNode
          )
        );
        setCreatingDraftNodeIds((previousNodeIds) =>
          previousNodeIds.includes(draftNodeID) ? previousNodeIds : [...previousNodeIds, draftNodeID]
        );
        cancelInlineEdit();
        await onCreateNode({
          parentId: editingDraftNode.parentId,
          type: editingDraftNode.type,
          title: normalizedTitle
        });
        removeDraftNode(draftNodeID);
      } catch (error) {
        setCreatingDraftNodeIds((previousNodeIds) =>
          previousNodeIds.filter((candidateNodeID) => candidateNodeID !== draftNodeID)
        );
        beginInlineEdit(draftNodeID, normalizedTitle);
        window.alert(`创建失败：${formatError(error)}`);
      } finally {
        isCommittingInlineEditRef.current = false;
      }
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
  }, [
    beginInlineEdit,
    cancelInlineEdit,
    draftNodeByID,
    editingNodeId,
    editingNodeTitle,
    nodeById,
    onCreateNode,
    onRenameNode,
    removeDraftNode
  ]);

  const stageDraftNodeAndEnterInlineEdit = useCallback(
    (input: { parentId: string | null; type: NodeType; title: string }) => {
      const draftNodeID = buildDraftNodeID();
      setDraftNodes((previousDraftNodes) => [
        ...previousDraftNodes,
        {
          nodeId: draftNodeID,
          parentId: input.parentId,
          type: input.type,
          title: input.title
        }
      ]);
      if (input.parentId) {
        setExpandedNodeIds((previousExpandedNodeIds) => {
          if (previousExpandedNodeIds.includes(input.parentId!)) {
            return previousExpandedNodeIds;
          }
          return [...previousExpandedNodeIds, input.parentId!];
        });
      }
      setOpenActionNodeId(null);
      beginInlineEdit(draftNodeID, input.title);
    },
    [beginInlineEdit]
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
      if (draftNodeByID.has(item.data.nodeId)) {
        return;
      }
      setOpenActionNodeId(null);
      void onOpenDocument(item.data.nodeId).catch(() => {
        // 上层会统一更新状态与路由，这里吞掉 Promise rejection 避免控制台噪音。
      });
    },
    [draftNodeByID, onOpenDocument]
  );

  const handleCreateChildDocument = useCallback(
    async (nodeId: string): Promise<void> => {
      stageDraftNodeAndEnterInlineEdit({
        parentId: nodeId,
        type: "doc",
        title: DEFAULT_DOCUMENT_TITLE
      });
    },
    [stageDraftNodeAndEnterInlineEdit]
  );

  const handleCreateSiblingDocument = useCallback(
    async (nodeId: string): Promise<void> => {
      const currentNode = nodeById.get(nodeId);
      if (!currentNode) {
        throw new Error("目录节点不存在");
      }
      stageDraftNodeAndEnterInlineEdit({
        parentId: currentNode.parentId,
        type: "doc",
        title: DEFAULT_DOCUMENT_TITLE
      });
    },
    [nodeById, stageDraftNodeAndEnterInlineEdit]
  );

  const handleCreateChildFolder = useCallback(
    async (nodeId: string): Promise<void> => {
      stageDraftNodeAndEnterInlineEdit({
        parentId: nodeId,
        type: "folder",
        title: DEFAULT_FOLDER_TITLE
      });
    },
    [stageDraftNodeAndEnterInlineEdit]
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
      const confirmed = await confirmByModal({
        title: "删除节点确认",
        description: confirmMessage,
        confirmText: "确认删除",
        tone: "danger"
      });
      if (!confirmed) {
        return;
      }
      await onDeleteNode(nodeId);
    },
    [confirmByModal, nodeById, onDeleteNode]
  );

  const handleCreateRootDocument = useCallback(async () => {
    if (isCreatingFirstDocument) {
      return;
    }
    setIsCreatingFirstDocument(true);
    try {
      stageDraftNodeAndEnterInlineEdit({
        parentId: null,
        type: "doc",
        title: DEFAULT_DOCUMENT_TITLE
      });
    } finally {
      setIsCreatingFirstDocument(false);
    }
  }, [isCreatingFirstDocument, stageDraftNodeAndEnterInlineEdit]);

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
      const isDraftNode = draftNodeByID.has(nodeId);
      const isCreatingDraftNode = creatingDraftNodeIdSet.has(nodeId);
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
              isActive ? "bg-[#d9dade]" : "bg-transparent hover:bg-[#e8e8ea]",
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
                      if (editingNodeId && draftNodeByID.has(editingNodeId)) {
                        removeDraftNode(editingNodeId);
                      }
                      cancelInlineEdit();
                    }
                  }}
                  onBlur={(event: ReactFocusEvent<HTMLInputElement>) => {
                    if (
                      event.relatedTarget instanceof HTMLElement &&
                      event.currentTarget.parentElement?.contains(event.relatedTarget)
                    ) {
                      return;
                    }
                    void commitInlineEdit();
                  }}
                />
              ) : (
                <>
                  <span
                    className={mergeClassNames(
                      "min-w-0 truncate leading-[1.3]",
                      isActive && "font-semibold"
                    )}
                    title={item.data.title}
                  >
                    {title}
                  </span>
                  {isDraftNode && isCreatingDraftNode ? (
                    <span className="ml-2 shrink-0 text-[11px] text-[#8a8d90]">创建中...</span>
                  ) : null}
                </>
              )}
            </InteractiveComponent>
            {isInlineEditing || isDraftNode ? null : (
              <div className="relative ml-1.5 inline-flex items-center" ref={isActionMenuOpen ? actionMenuRootRef : undefined}>
                <button
                  type="button"
                  className={mergeClassNames(
                    "inline-flex h-[26px] w-[26px] items-center justify-center rounded-[8px] border-0 bg-transparent text-[#71767a]",
                    "transition-[opacity,background-color,color] duration-100",
                    "hover:bg-[#dde0e4] hover:text-[#3e4247] focus-visible:outline-none",
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
                      className="inline-flex min-h-[34px] w-full items-center gap-2 rounded-[8px] border-0 bg-transparent px-2.5 text-left text-[13px] text-[#2f2f30] hover:bg-[#f0f2f4] focus-visible:outline-none"
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
                      className="inline-flex min-h-[34px] w-full items-center gap-2 rounded-[8px] border-0 bg-transparent px-2.5 text-left text-[13px] text-[#2f2f30] hover:bg-[#f0f2f4] focus-visible:outline-none"
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
                      className="inline-flex min-h-[34px] w-full items-center gap-2 rounded-[8px] border-0 bg-transparent px-2.5 text-left text-[13px] text-[#2f2f30] hover:bg-[#f0f2f4] focus-visible:outline-none"
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
                      className="inline-flex min-h-[34px] w-full items-center gap-2 rounded-[8px] border-0 bg-transparent px-2.5 text-left text-[13px] text-[#2f2f30] hover:bg-[#f0f2f4] focus-visible:outline-none"
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
                      className="inline-flex min-h-[34px] w-full items-center gap-2 rounded-[8px] border-0 bg-transparent px-2.5 text-left text-[13px] text-[#b42318] hover:bg-[#fff0ef] focus-visible:outline-none"
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
      creatingDraftNodeIdSet,
      draftNodeByID,
      handleCreateChildDocument,
      handleCreateChildFolder,
      handleCreateSiblingDocument,
      handleDeleteNode,
      handleRenameNode,
      cancelInlineEdit,
      commitInlineEdit,
      editingNodeId,
      removeDraftNode,
      editingNodeTitle,
      openActionNodeId,
      runActionMenuTask,
      stopTreeItemPropagation,
      stopTreeItemEvent
    ]
  );

  return (
    <>
      {confirmDialog}
      <div className="mb-2 flex h-11 items-center justify-between border-b border-[#d9dade] px-2">
        <span className="text-[18px] font-semibold text-[#1f2328]">目录</span>
        <DropdownMenu modal={false}>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              className="inline-flex h-8 w-8 cursor-pointer items-center justify-center rounded-[8px] border-0 bg-transparent text-[#1f2328] transition-colors hover:bg-[#e7e8ea] data-[state=open]:bg-[#e7e8ea] focus:outline-none focus-visible:outline-none"
              aria-label="打开目录快捷菜单"
              disabled={isCreatingFirstDocument}
            >
              <Plus size={18} />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            align="end"
            className="min-w-[148px]"
            onCloseAutoFocus={(event) => {
              // 新建后焦点应交给行内输入框，而不是回到触发按钮。
              event.preventDefault();
            }}
          >
            <DropdownMenuItem
              className="cursor-pointer"
              onSelect={() => {
                void handleCreateRootDocument();
              }}
            >
              <FilePlus2 size={14} className="mr-2" />
              <span>新建文档</span>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      {mergedNodes.length === 0 ? (
        <div className="mt-2.5 mr-2 mb-0 ml-2 flex min-h-[168px] flex-col items-center justify-end gap-3 pb-5 text-center">
          <p className="m-0 text-[14px] text-[#8a8d90]">当前空间暂无文档。</p>
          <button
            type="button"
            className="inline-flex h-9 w-fit items-center gap-1.5 rounded-full border-0 bg-[#e1e4e8] px-4 text-[13px] font-medium text-[#2f2f30] transition-colors hover:bg-[#cfd4da] focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-60"
            onClick={() => {
              void handleCreateRootDocument();
            }}
            disabled={isCreatingFirstDocument}
          >
            <FilePlus2 size={14} />
            <span>{isCreatingFirstDocument ? "创建中..." : "新建第一篇文档"}</span>
          </button>
        </div>
      ) : (
        <ControlledTreeEnvironment<WorkspaceTreeItemData>
          items={items}
          getItemTitle={(item) => item.data.title}
          viewState={viewState}
          autoFocus={false}
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
      )}
    </>
  );
});
