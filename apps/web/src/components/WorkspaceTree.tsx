import {
  Check,
  ChevronDown,
  ChevronRight,
  FilePlus2,
  FolderPlus,
  Globe,
  Link2,
  Lock,
  LockOpen,
  LoaderCircle,
  PencilLine,
  Plus,
  Trash2,
  Users
} from "lucide-react";
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
  type DraggingPosition,
  InteractionMode,
  Tree,
  type TreeInformation,
  type TreeItem,
  type TreeItemRenderContext,
  type TreeViewState
} from "react-complex-tree";
import type {
  CreateNodeResult,
  DocumentTemplateDetail,
  DocumentTemplateSummary,
  NodeType,
  TreeNode,
  Visibility
} from "../data-access";
import { formatError } from "../editor/status-utils";
import { useConfirmDialog } from "./ConfirmDialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger
} from "./ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "./ui/tooltip";
import { toast } from "sonner";

const WORKSPACE_TREE_ID = "workspace-doc-tree";
const WORKSPACE_TREE_ROOT_ID = "__workspace_doc_tree_root__";
const DEFAULT_DOCUMENT_TITLE = "未命名文档";
const DEFAULT_FOLDER_TITLE = "未命名目录";
const MAX_DOCUMENT_IDENTIFIER_LENGTH = 80;
const DOCUMENT_IDENTIFIER_PATTERN = /^[a-z0-9-]+$/;
const RESERVED_DOCUMENT_IDENTIFIERS = new Set(["admin", "api", "explore", "login", "register", "search"]);

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
    documentIdentifier?: string;
    templateId?: string;
  }) => Promise<CreateNodeResult>;
  onListDocumentTemplates: () => Promise<DocumentTemplateSummary[]>;
  onGetDocumentTemplate: (templateId: string) => Promise<DocumentTemplateDetail>;
  onUpdateDocumentIdentifier: (docId: string, identifier: string | null) => Promise<void>;
  onUpdateDocumentVisibility: (docId: string, visibility: Visibility) => Promise<void>;
  onRenameNode: (nodeId: string, title: string) => Promise<void>;
  onDeleteNode: (nodeId: string) => Promise<void>;
  onMoveNode: (input: { nodeId: string; targetParentId: string | null; targetIndex: number }) => Promise<void>;
}

interface PendingCreateDraftNode {
  nodeId: string;
  parentId: string | null;
  type: NodeType;
  title: string;
  documentIdentifier?: string;
  templateId?: string;
}

interface CreateNodeDialogState {
  parentId: string | null;
  type: NodeType;
  title: string;
  documentIdentifier: string;
  templateId: string;
}

function mergeClassNames(...classNames: Array<string | false | null | undefined>): string {
  return classNames.filter(Boolean).join(" ");
}

function validateDocumentIdentifier(rawValue: string, options: { allowEmpty: boolean }): {
  value: string | null;
  error: string | null;
} {
  const normalizedValue = rawValue.trim().toLowerCase();
  if (!normalizedValue) {
    if (options.allowEmpty) {
      return { value: null, error: null };
    }
    return { value: null, error: "文档标识不能为空" };
  }
  if (normalizedValue.length > MAX_DOCUMENT_IDENTIFIER_LENGTH) {
    return { value: null, error: `文档标识长度不能超过 ${MAX_DOCUMENT_IDENTIFIER_LENGTH} 个字符` };
  }
  if (!DOCUMENT_IDENTIFIER_PATTERN.test(normalizedValue)) {
    return { value: null, error: "文档标识仅支持小写字母、数字和连字符（-）" };
  }
  if (normalizedValue.startsWith("-") || normalizedValue.endsWith("-")) {
    return { value: null, error: "文档标识不能以连字符（-）开头或结尾" };
  }
  if (RESERVED_DOCUMENT_IDENTIFIERS.has(normalizedValue)) {
    return { value: null, error: "该标识为系统保留词，请更换其他标识" };
  }
  return { value: normalizedValue, error: null };
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

function collectAncestorNodeIds(
  nodeId: string,
  parentNodeIdByNodeID: Map<string, string | null>
): string[] {
  const ancestorNodeIds: string[] = [];
  const visitedNodeIds = new Set<string>();
  let currentNodeID = parentNodeIdByNodeID.get(nodeId) ?? null;

  while (currentNodeID && !visitedNodeIds.has(currentNodeID)) {
    ancestorNodeIds.push(currentNodeID);
    visitedNodeIds.add(currentNodeID);
    currentNodeID = parentNodeIdByNodeID.get(currentNodeID) ?? null;
  }

  return ancestorNodeIds;
}

function collectDescendantNodeIds(nodeId: string, nodeById: Map<string, TreeNode>): string[] {
  const rootNode = nodeById.get(nodeId);
  if (!rootNode || rootNode.children.length === 0) {
    return [];
  }

  const descendantNodeIds: string[] = [];
  const pendingNodes: TreeNode[] = [...rootNode.children];
  while (pendingNodes.length > 0) {
    const currentNode = pendingNodes.pop();
    if (!currentNode) {
      continue;
    }
    descendantNodeIds.push(currentNode.id);
    if (currentNode.children.length > 0) {
      pendingNodes.push(...currentNode.children);
    }
  }
  return descendantNodeIds;
}

interface WorkspaceNodeMoveTarget {
  targetParentId: string | null;
  targetIndex: number;
}

function clampWorkspaceMoveIndex(targetIndex: number, maxIndex: number): number {
  if (targetIndex < 0) {
    return 0;
  }
  if (targetIndex > maxIndex) {
    return maxIndex;
  }
  return targetIndex;
}

function resolveWorkspaceNodeMoveTarget(
  target: DraggingPosition,
  nodeById: Map<string, TreeNode>,
  rootChildrenCount: number
): WorkspaceNodeMoveTarget | null {
  if (target.targetType === "root") {
    return {
      targetParentId: null,
      targetIndex: rootChildrenCount
    };
  }

  if (target.targetType === "item") {
    const targetNodeID = String(target.targetItem);
    if (targetNodeID === WORKSPACE_TREE_ROOT_ID) {
      return {
        targetParentId: null,
        targetIndex: rootChildrenCount
      };
    }
    const targetNode = nodeById.get(targetNodeID);
    if (!targetNode) {
      return null;
    }
    return {
      targetParentId: targetNodeID,
      targetIndex: targetNode.children.length
    };
  }

  const parentNodeID = String(target.parentItem);
  const targetParentId = parentNodeID === WORKSPACE_TREE_ROOT_ID ? null : parentNodeID;
  const targetParentNode = targetParentId ? nodeById.get(targetParentId) : null;
  if (targetParentId && !targetParentNode) {
    return null;
  }
  const maxIndex = targetParentNode ? targetParentNode.children.length : rootChildrenCount;
  return {
    targetParentId,
    targetIndex: clampWorkspaceMoveIndex(target.childIndex, maxIndex)
  };
}

function findWorkspaceSiblingIndex(
  nodeID: string,
  nodeById: Map<string, TreeNode>,
  rootNodes: TreeNode[]
): number {
  const node = nodeById.get(nodeID);
  if (!node) {
    return -1;
  }
  if (!node.parentId) {
    return rootNodes.findIndex((candidateNode) => candidateNode.id === nodeID);
  }
  const parentNode = nodeById.get(node.parentId);
  if (!parentNode) {
    return -1;
  }
  return parentNode.children.findIndex((candidateNode) => candidateNode.id === nodeID);
}

function resolveVisibilityMarkerConfig(
  visibilityInput: Visibility | undefined
): { label: string; variant: "public" | "authenticated" | "member"; className: string } {
  const visibility = visibilityInput ?? "member";
  if (visibility === "public") {
    return {
      label: "公开",
      variant: "public",
      className: "text-[#166534]"
    };
  }
  if (visibility === "authenticated") {
    return {
      label: "登录可见",
      variant: "authenticated",
      className: "text-[#1d4ed8]"
    };
  }
  return {
    label: "成员可见",
    variant: "member",
    className: "text-[#334155]"
  };
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
  onListDocumentTemplates,
  onGetDocumentTemplate,
  onUpdateDocumentIdentifier,
  onUpdateDocumentVisibility,
  onRenameNode,
  onDeleteNode,
  onMoveNode
}: WorkspaceTreeProps) {
  const { confirm: confirmByModal, dialog: confirmDialog } = useConfirmDialog();
  const [draftNodes, setDraftNodes] = useState<PendingCreateDraftNode[]>([]);
  const mergedNodes = useMemo(
    () => mergeDraftNodesIntoTree(nodes, draftNodes),
    [draftNodes, nodes]
  );
  const { items, nodeById } = useMemo(() => buildTreeItems(mergedNodes), [mergedNodes]);
  const activeTreeItemId = useMemo(() => {
    if (!activeDocId) {
      return null;
    }
    if (items[activeDocId]) {
      return activeDocId;
    }
    for (const [nodeId, node] of nodeById.entries()) {
      if (node.type !== "doc") {
        continue;
      }
      if ((node.documentId ?? node.id) === activeDocId) {
        return nodeId;
      }
    }
    return null;
  }, [activeDocId, items, nodeById]);
  const parentNodeIdByNodeID = useMemo(() => {
    const mappedParentNodeIDs = new Map<string, string | null>();
    for (const [nodeID, node] of nodeById.entries()) {
      const normalizedParentNodeID = (node.parentId ?? "").trim();
      mappedParentNodeIDs.set(nodeID, normalizedParentNodeID || null);
    }
    return mappedParentNodeIDs;
  }, [nodeById]);
  const expandableNodeIds = useMemo(() => collectExpandableNodeIds(mergedNodes), [mergedNodes]);
  const expandableNodeIdSet = useMemo(() => new Set(expandableNodeIds), [expandableNodeIds]);
  const draftNodeByID = useMemo(() => {
    const mappedDraftNodes = new Map<string, PendingCreateDraftNode>();
    for (const draftNode of draftNodes) {
      mappedDraftNodes.set(draftNode.nodeId, draftNode);
    }
    return mappedDraftNodes;
  }, [draftNodes]);
  const hasInitialAutoExpandedRef = useRef(false);
  const lastAutoScrolledActiveTreeItemIDRef = useRef<string | null>(null);
  const actionMenuRootRef = useRef<HTMLDivElement | null>(null);
  const inlineEditInputRef = useRef<HTMLInputElement | null>(null);
  const pendingInlineEditFocusNodeIdRef = useRef<string | null>(null);
  const isCommittingInlineEditRef = useRef(false);
  const documentTemplateDetailsRef = useRef<Record<string, DocumentTemplateDetail>>({});
  const templatePreviewRequestIDRef = useRef(0);
  // 默认全折叠：首次进入目录树时不自动展开任何节点。
  const [manuallyExpandedNodeIds, setManuallyExpandedNodeIds] = useState<string[]>([]);
  const expandedNodeIds = manuallyExpandedNodeIds;
  const [openActionNodeId, setOpenActionNodeId] = useState<string | null>(null);
  const [editingNodeId, setEditingNodeId] = useState<string | null>(null);
  const [editingNodeTitle, setEditingNodeTitle] = useState("");
  const [creatingDraftNodeIds, setCreatingDraftNodeIds] = useState<string[]>([]);
  const [createNodeDialog, setCreateNodeDialog] = useState<CreateNodeDialogState | null>(null);
  const [isCreateNodeDialogSubmitting, setIsCreateNodeDialogSubmitting] = useState(false);
  const [documentTemplates, setDocumentTemplates] = useState<DocumentTemplateSummary[]>([]);
  const [isDocumentTemplatesLoading, setIsDocumentTemplatesLoading] = useState(false);
  const [documentTemplatesLoaded, setDocumentTemplatesLoaded] = useState(false);
  const [documentTemplatesError, setDocumentTemplatesError] = useState<string | null>(null);
  const [documentTemplateDetails, setDocumentTemplateDetails] = useState<Record<string, DocumentTemplateDetail>>({});
  const [isTemplatePreviewLoading, setIsTemplatePreviewLoading] = useState(false);
  const [templatePreviewError, setTemplatePreviewError] = useState<string | null>(null);
  const [editIdentifierDialogNodeID, setEditIdentifierDialogNodeID] = useState<string | null>(null);
  const [editIdentifierDialogDocumentID, setEditIdentifierDialogDocumentID] = useState("");
  const [editIdentifierDialogTitle, setEditIdentifierDialogTitle] = useState(DEFAULT_DOCUMENT_TITLE);
  const [editIdentifierDialogValue, setEditIdentifierDialogValue] = useState("");
  // 文档可见性更新中的节点：用于在树节点上展示细粒度 loading 状态。
  const [updatingVisibilityNodeIds, setUpdatingVisibilityNodeIds] = useState<string[]>([]);
  const [updatingIdentifierNodeIds, setUpdatingIdentifierNodeIds] = useState<string[]>([]);
  const [isDesktopDragEnabled, setIsDesktopDragEnabled] = useState(false);
  const creatingDraftNodeIdSet = useMemo(() => new Set(creatingDraftNodeIds), [creatingDraftNodeIds]);
  const updatingVisibilityNodeIdSet = useMemo(
    () => new Set(updatingVisibilityNodeIds),
    [updatingVisibilityNodeIds]
  );
  const updatingIdentifierNodeIdSet = useMemo(
    () => new Set(updatingIdentifierNodeIds),
    [updatingIdentifierNodeIds]
  );
  const documentTemplateOptions = useMemo(
    () =>
      [...documentTemplates].sort((left, right) => {
        if (left.sceneKey !== right.sceneKey) {
          return left.sceneKey.localeCompare(right.sceneKey);
        }
        if (left.sort !== right.sort) {
          return left.sort - right.sort;
        }
        return left.name.localeCompare(right.name);
      }),
    [documentTemplates]
  );
  const groupedDocumentTemplateOptions = useMemo(() => {
    const grouped = new Map<
      string,
      { sceneKey: string; sceneLabel: string; options: DocumentTemplateSummary[] }
    >();
    for (const item of documentTemplateOptions) {
      const sceneKey = item.sceneKey || "default";
      const sceneLabel = item.sceneName || item.sceneKey || "未分类场景";
      const existing = grouped.get(sceneKey);
      if (existing) {
        existing.options.push(item);
        continue;
      }
      grouped.set(sceneKey, { sceneKey, sceneLabel, options: [item] });
    }
    return Array.from(grouped.values());
  }, [documentTemplateOptions]);
  const selectedTemplateID = useMemo(() => {
    if (!createNodeDialog || createNodeDialog.type !== "doc") {
      return "";
    }
    return createNodeDialog.templateId.trim();
  }, [createNodeDialog?.templateId, createNodeDialog?.type]);
  const selectedTemplateDetail = useMemo(() => {
    if (!selectedTemplateID) {
      return null;
    }
    return documentTemplateDetails[selectedTemplateID] ?? null;
  }, [documentTemplateDetails, selectedTemplateID]);
  const selectedTemplatePreviewText = useMemo(() => {
    const content = selectedTemplateDetail?.contentMd ?? "";
    const normalized = content.trim();
    if (!normalized) {
      return "(该模板正文为空)";
    }
    const lines = normalized.split("\n").slice(0, 16);
    return lines.join("\n");
  }, [selectedTemplateDetail?.contentMd]);

  const loadDocumentTemplates = useCallback(
    async (forceReload = false): Promise<void> => {
      if (!forceReload && (documentTemplatesLoaded || isDocumentTemplatesLoading)) {
        return;
      }
      setIsDocumentTemplatesLoading(true);
      try {
        const items = await onListDocumentTemplates();
        setDocumentTemplates(items);
        setDocumentTemplatesLoaded(true);
        setDocumentTemplatesError(null);
      } catch (error) {
        const message = formatError(error);
        setDocumentTemplatesError(message);
        toast.error(`加载模板失败：${message}`);
      } finally {
        setIsDocumentTemplatesLoading(false);
      }
    },
    [documentTemplatesLoaded, isDocumentTemplatesLoading, onListDocumentTemplates]
  );

  const loadDocumentTemplateDetail = useCallback(
    async (templateId: string, options?: { forceReload?: boolean }): Promise<void> => {
      const normalizedTemplateID = templateId.trim();
      if (!normalizedTemplateID) {
        templatePreviewRequestIDRef.current += 1;
        setTemplatePreviewError(null);
        setIsTemplatePreviewLoading(false);
        return;
      }
      if (!options?.forceReload && documentTemplateDetailsRef.current[normalizedTemplateID]) {
        templatePreviewRequestIDRef.current += 1;
        setTemplatePreviewError(null);
        setIsTemplatePreviewLoading(false);
        return;
      }
      const requestID = templatePreviewRequestIDRef.current + 1;
      templatePreviewRequestIDRef.current = requestID;
      setIsTemplatePreviewLoading(true);
      setTemplatePreviewError(null);
      try {
        const detail = await onGetDocumentTemplate(normalizedTemplateID);
        setDocumentTemplateDetails((previousDetails) => ({
          ...previousDetails,
          [normalizedTemplateID]: detail
        }));
      } catch (error) {
        if (templatePreviewRequestIDRef.current !== requestID) {
          return;
        }
        setTemplatePreviewError(formatError(error));
      } finally {
        if (templatePreviewRequestIDRef.current === requestID) {
          setIsTemplatePreviewLoading(false);
        }
      }
    },
    [onGetDocumentTemplate]
  );

  useEffect(() => {
    documentTemplateDetailsRef.current = documentTemplateDetails;
  }, [documentTemplateDetails]);

  useEffect(() => {
    void loadDocumentTemplateDetail(selectedTemplateID);
  }, [loadDocumentTemplateDetail, selectedTemplateID]);

  // 拖拽排序仅在桌面端启用：依赖 hover + fine pointer 能力判断。
  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
      return;
    }
    const mediaQuery = window.matchMedia(
      "((hover: hover) and (pointer: fine)) or ((any-hover: hover) and (any-pointer: fine))"
    );
    const applyCapability = () => {
      setIsDesktopDragEnabled(mediaQuery.matches);
    };
    applyCapability();

    if (typeof mediaQuery.addEventListener === "function") {
      mediaQuery.addEventListener("change", applyCapability);
      return () => {
        mediaQuery.removeEventListener("change", applyCapability);
      };
    }

    mediaQuery.addListener(applyCapability);
    return () => {
      mediaQuery.removeListener(applyCapability);
    };
  }, []);

  // 树结构变化时仅保留仍然存在的展开节点，不自动展开新节点。
  // 这样可以保持“默认全折叠”与“用户手动展开优先”。
  useEffect(() => {
    const currentExpandableNodeIdSet = new Set(expandableNodeIds);
    setManuallyExpandedNodeIds((previousExpandedNodeIds) => {
      return previousExpandedNodeIds.filter((nodeId) => currentExpandableNodeIdSet.has(nodeId));
    });
  }, [expandableNodeIds]);

  // 仅在页面首次进入且已拿到激活文档时自动展开一次祖先链路。
  // 后续异步切换文档不再改动折叠状态，避免覆盖用户手动展开/收起操作。
  useEffect(() => {
    if (hasInitialAutoExpandedRef.current) {
      return;
    }
    if (!activeTreeItemId) {
      return;
    }
    const ancestorNodeIds = collectAncestorNodeIds(activeTreeItemId, parentNodeIdByNodeID).filter((nodeID) =>
      expandableNodeIdSet.has(nodeID)
    );
    setManuallyExpandedNodeIds(ancestorNodeIds);
    hasInitialAutoExpandedRef.current = true;
  }, [activeTreeItemId, expandableNodeIdSet, parentNodeIdByNodeID]);

  // 自动滚动到当前激活文档，避免刷新后需要手动在树里二次定位。
  useEffect(() => {
    if (!activeTreeItemId) {
      return;
    }
    if (lastAutoScrolledActiveTreeItemIDRef.current === activeTreeItemId) {
      return;
    }

    const targetElementID = `workspace-tree-item-${activeTreeItemId}`;
    const rafID = window.requestAnimationFrame(() => {
      const targetElement = document.getElementById(targetElementID);
      if (!(targetElement instanceof HTMLElement)) {
        return;
      }
      targetElement.scrollIntoView({
        block: "nearest",
        inline: "nearest"
      });
      lastAutoScrolledActiveTreeItemIDRef.current = activeTreeItemId;
    });

    return () => {
      window.cancelAnimationFrame(rafID);
    };
  }, [activeTreeItemId, expandedNodeIds]);

  useEffect(() => {
    if (activeTreeItemId) {
      return;
    }
    lastAutoScrolledActiveTreeItemIDRef.current = null;
  }, [activeTreeItemId]);

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
    const focusedItem = editingNodeId ? undefined : activeTreeItemId ?? undefined;
    return {
      [WORKSPACE_TREE_ID]: {
        expandedItems: expandedNodeIds,
        selectedItems: activeTreeItemId ? [activeTreeItemId] : [],
        focusedItem
      }
    };
  }, [activeTreeItemId, editingNodeId, expandedNodeIds]);

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
      toast.error(`操作失败：${formatError(error)}`);
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
          title: normalizedTitle,
          documentIdentifier: editingDraftNode.documentIdentifier,
          templateId: editingDraftNode.templateId
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

  const handleExpandNode = useCallback((item: TreeItem<WorkspaceTreeItemData>) => {
    const nodeId = String(item.index);
    const descendantNodeIdSet = new Set(collectDescendantNodeIds(nodeId, nodeById));
    setManuallyExpandedNodeIds((previousExpandedNodeIds) => {
      const filteredExpandedNodeIds = previousExpandedNodeIds.filter(
        (expandedNodeID) => !descendantNodeIdSet.has(expandedNodeID)
      );
      if (filteredExpandedNodeIds.includes(nodeId)) {
        return filteredExpandedNodeIds;
      }
      return [...filteredExpandedNodeIds, nodeId];
    });
  }, [nodeById]);

  const handleCollapseNode = useCallback((item: TreeItem<WorkspaceTreeItemData>) => {
    const nodeId = String(item.index);
    const collapsedNodeIdSet = new Set([nodeId, ...collectDescendantNodeIds(nodeId, nodeById)]);
    setManuallyExpandedNodeIds((previousExpandedNodeIds) =>
      previousExpandedNodeIds.filter((expandedNodeId) => !collapsedNodeIdSet.has(expandedNodeId))
    );
  }, [nodeById]);

  // 主操作仅用于打开文档；目录展开收起交给箭头交互管理。
  const handlePrimaryAction = useCallback(
    (item: TreeItem<WorkspaceTreeItemData>) => {
      if (item.data.type !== "doc" || !item.data.nodeId) {
        return;
      }
      const currentNode = nodeById.get(item.data.nodeId);
      if (!currentNode || currentNode.type !== "doc") {
        return;
      }
      if (draftNodeByID.has(currentNode.id)) {
        return;
      }
      const documentID = (currentNode.documentId ?? currentNode.id ?? "").trim();
      if (!documentID) {
        return;
      }
      setOpenActionNodeId(null);
      void onOpenDocument(documentID).catch(() => {
        // 上层会统一更新状态与路由，这里吞掉 Promise rejection 避免控制台噪音。
      });
    },
    [draftNodeByID, nodeById, onOpenDocument]
  );

  const handleCreateChildDocument = useCallback(
    async (nodeId: string): Promise<void> => {
      setCreateNodeDialog({
        parentId: nodeId,
        type: "doc",
        title: DEFAULT_DOCUMENT_TITLE,
        documentIdentifier: "",
        templateId: ""
      });
      void loadDocumentTemplates();
      setOpenActionNodeId(null);
    },
    [loadDocumentTemplates]
  );

  const handleCreateSiblingDocument = useCallback(
    async (nodeId: string): Promise<void> => {
      const currentNode = nodeById.get(nodeId);
      if (!currentNode) {
        throw new Error("目录节点不存在");
      }
      setCreateNodeDialog({
        parentId: currentNode.parentId,
        type: "doc",
        title: DEFAULT_DOCUMENT_TITLE,
        documentIdentifier: "",
        templateId: ""
      });
      void loadDocumentTemplates();
      setOpenActionNodeId(null);
    },
    [loadDocumentTemplates, nodeById]
  );

  const handleCreateChildFolder = useCallback(
    async (nodeId: string): Promise<void> => {
      setCreateNodeDialog({
        parentId: nodeId,
        type: "folder",
        title: DEFAULT_FOLDER_TITLE,
        documentIdentifier: "",
        templateId: ""
      });
      setOpenActionNodeId(null);
    },
    []
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

  const handleUpdateNodeVisibility = useCallback(
    async (nodeId: string, visibility: Visibility): Promise<void> => {
      if (updatingVisibilityNodeIdSet.has(nodeId)) {
        return;
      }
      const currentNode = nodeById.get(nodeId);
      if (!currentNode) {
        throw new Error("目录节点不存在");
      }
      if (currentNode.type !== "doc") {
        throw new Error("仅文档支持可见性设置");
      }
      const documentID = (currentNode.documentId ?? currentNode.id ?? "").trim();
      if (!documentID) {
        throw new Error("文档 ID 不存在");
      }
      setUpdatingVisibilityNodeIds((previousNodeIds) =>
        previousNodeIds.includes(nodeId) ? previousNodeIds : [...previousNodeIds, nodeId]
      );
      try {
        await onUpdateDocumentVisibility(documentID, visibility);
      } finally {
        setUpdatingVisibilityNodeIds((previousNodeIds) =>
          previousNodeIds.filter((candidateNodeID) => candidateNodeID !== nodeId)
        );
      }
    },
    [nodeById, onUpdateDocumentVisibility, updatingVisibilityNodeIdSet]
  );

  const handleCreateRootDocument = useCallback(async () => {
    setCreateNodeDialog({
      parentId: null,
      type: "doc",
      title: DEFAULT_DOCUMENT_TITLE,
      documentIdentifier: "",
      templateId: ""
    });
    void loadDocumentTemplates();
  }, [loadDocumentTemplates]);

  const closeCreateNodeDialog = useCallback(() => {
    if (isCreateNodeDialogSubmitting) {
      return;
    }
    setCreateNodeDialog(null);
  }, [isCreateNodeDialogSubmitting]);

  const handleCreateNodeByDialog = useCallback(async () => {
    if (!createNodeDialog || isCreateNodeDialogSubmitting) {
      return;
    }
    const fallbackTitle = createNodeDialog.type === "folder" ? DEFAULT_FOLDER_TITLE : DEFAULT_DOCUMENT_TITLE;
    const normalizedTitle = createNodeDialog.title.trim() || fallbackTitle;

    let normalizedIdentifier: string | undefined;
    if (createNodeDialog.type === "doc") {
      const identifierValidation = validateDocumentIdentifier(createNodeDialog.documentIdentifier, { allowEmpty: true });
      if (identifierValidation.error) {
        toast.error(identifierValidation.error);
        return;
      }
      normalizedIdentifier = identifierValidation.value ?? undefined;
    }

    setIsCreateNodeDialogSubmitting(true);
    try {
      const created = await onCreateNode({
        parentId: createNodeDialog.parentId,
        type: createNodeDialog.type,
        title: normalizedTitle,
        documentIdentifier: normalizedIdentifier,
        templateId: createNodeDialog.templateId || undefined
      });
      if (created.docId) {
        void onOpenDocument(created.docId).catch(() => {
          // 上层会统一更新状态与路由，这里吞掉 Promise rejection 避免控制台噪音。
        });
      }
      setCreateNodeDialog(null);
    } catch (error) {
      toast.error(`创建失败：${formatError(error)}`);
    } finally {
      setIsCreateNodeDialogSubmitting(false);
    }
  }, [createNodeDialog, isCreateNodeDialogSubmitting, onCreateNode, onOpenDocument]);

  const openEditDocumentIdentifierDialog = useCallback(
    (nodeId: string) => {
      const currentNode = nodeById.get(nodeId);
      if (!currentNode || currentNode.type !== "doc") {
        toast.error("仅文档支持设置标识");
        return;
      }
      const documentID = (currentNode.documentId ?? currentNode.id ?? "").trim();
      if (!documentID) {
        toast.error("文档 ID 不存在");
        return;
      }
      setEditIdentifierDialogNodeID(nodeId);
      setEditIdentifierDialogDocumentID(documentID);
      setEditIdentifierDialogTitle(currentNode.title?.trim() || DEFAULT_DOCUMENT_TITLE);
      setEditIdentifierDialogValue((currentNode.documentIdentifier ?? "").trim());
      setOpenActionNodeId(null);
    },
    [nodeById]
  );

  const closeEditDocumentIdentifierDialog = useCallback(() => {
    setEditIdentifierDialogNodeID(null);
    setEditIdentifierDialogDocumentID("");
    setEditIdentifierDialogTitle(DEFAULT_DOCUMENT_TITLE);
    setEditIdentifierDialogValue("");
  }, []);

  const handleUpdateDocumentIdentifier = useCallback(async () => {
    const nodeID = (editIdentifierDialogNodeID ?? "").trim();
    const documentID = editIdentifierDialogDocumentID.trim();
    if (!nodeID || !documentID) {
      return;
    }
    if (updatingIdentifierNodeIdSet.has(nodeID)) {
      return;
    }
    const identifierValidation = validateDocumentIdentifier(editIdentifierDialogValue, { allowEmpty: true });
    if (identifierValidation.error) {
      toast.error(identifierValidation.error);
      return;
    }

    setUpdatingIdentifierNodeIds((previousNodeIDs) =>
      previousNodeIDs.includes(nodeID) ? previousNodeIDs : [...previousNodeIDs, nodeID]
    );
    try {
      await onUpdateDocumentIdentifier(documentID, identifierValidation.value);
      closeEditDocumentIdentifierDialog();
    } finally {
      setUpdatingIdentifierNodeIds((previousNodeIDs) =>
        previousNodeIDs.filter((candidateNodeID) => candidateNodeID !== nodeID)
      );
    }
  }, [
    closeEditDocumentIdentifierDialog,
    editIdentifierDialogDocumentID,
    editIdentifierDialogNodeID,
    editIdentifierDialogValue,
    onUpdateDocumentIdentifier,
    updatingIdentifierNodeIdSet
  ]);

  const canDragItems = useCallback(
    (draggingItems: TreeItem<WorkspaceTreeItemData>[]): boolean => {
      if (!isDesktopDragEnabled || editingNodeId !== null) {
        return false;
      }
      if (draggingItems.length !== 1) {
        return false;
      }

      const draggingNodeID = String(draggingItems[0]?.index ?? "");
      if (!draggingNodeID || draggingNodeID === WORKSPACE_TREE_ROOT_ID) {
        return false;
      }
      if (!nodeById.has(draggingNodeID)) {
        return false;
      }
      if (draftNodeByID.has(draggingNodeID) || creatingDraftNodeIdSet.has(draggingNodeID)) {
        return false;
      }
      return true;
    },
    [creatingDraftNodeIdSet, draftNodeByID, editingNodeId, isDesktopDragEnabled, nodeById]
  );

  const canDropAt = useCallback(
    (draggingItems: TreeItem<WorkspaceTreeItemData>[], target: DraggingPosition): boolean => {
      if (!isDesktopDragEnabled || draggingItems.length !== 1) {
        return false;
      }
      const draggingNodeID = String(draggingItems[0]?.index ?? "");
      if (!draggingNodeID || draggingNodeID === WORKSPACE_TREE_ROOT_ID) {
        return false;
      }
      if (!nodeById.has(draggingNodeID) || draftNodeByID.has(draggingNodeID)) {
        return false;
      }

      const moveTarget = resolveWorkspaceNodeMoveTarget(target, nodeById, mergedNodes.length);
      if (!moveTarget) {
        return false;
      }
      if (moveTarget.targetParentId === draggingNodeID) {
        return false;
      }
      if (!moveTarget.targetParentId) {
        return true;
      }

      const descendantNodeIdSet = new Set(collectDescendantNodeIds(draggingNodeID, nodeById));
      return !descendantNodeIdSet.has(moveTarget.targetParentId);
    },
    [draftNodeByID, isDesktopDragEnabled, mergedNodes.length, nodeById]
  );

  const handleDropNodes = useCallback(
    (draggingItems: TreeItem<WorkspaceTreeItemData>[], target: DraggingPosition) => {
      if (draggingItems.length !== 1) {
        return;
      }
      const draggingNodeID = String(draggingItems[0]?.index ?? "");
      if (!draggingNodeID || draggingNodeID === WORKSPACE_TREE_ROOT_ID) {
        return;
      }

      const moveTarget = resolveWorkspaceNodeMoveTarget(target, nodeById, mergedNodes.length);
      if (!moveTarget || moveTarget.targetParentId === draggingNodeID) {
        return;
      }
      if (moveTarget.targetParentId) {
        const descendantNodeIdSet = new Set(collectDescendantNodeIds(draggingNodeID, nodeById));
        if (descendantNodeIdSet.has(moveTarget.targetParentId)) {
          return;
        }
      }

      let targetIndex = moveTarget.targetIndex;
      // 同父级重排时，目标 childIndex 是基于“含源节点”的序列，需要扣除原位偏移。
      if (target.targetType === "between-items") {
        const draggingNode = nodeById.get(draggingNodeID);
        if (draggingNode) {
          const currentParentId = draggingNode.parentId ?? null;
          if (currentParentId === moveTarget.targetParentId) {
            const currentSiblingIndex = findWorkspaceSiblingIndex(draggingNodeID, nodeById, mergedNodes);
            if (currentSiblingIndex >= 0 && currentSiblingIndex < targetIndex) {
              targetIndex -= 1;
            }
          }
        }
      }
      targetIndex = clampWorkspaceMoveIndex(targetIndex, Number.MAX_SAFE_INTEGER);
      const autoExpandParentNodeID =
        target.targetType === "item" && moveTarget.targetParentId ? moveTarget.targetParentId : null;

      void onMoveNode({
        nodeId: draggingNodeID,
        targetParentId: moveTarget.targetParentId,
        targetIndex
      })
        .then(() => {
          if (!autoExpandParentNodeID) {
            return;
          }
          setManuallyExpandedNodeIds((previousExpandedNodeIds) => {
            if (previousExpandedNodeIds.includes(autoExpandParentNodeID)) {
              return previousExpandedNodeIds;
            }
            return [...previousExpandedNodeIds, autoExpandParentNodeID];
          });
        })
        .catch((error) => {
          toast.error(`拖拽排序失败：${formatError(error)}`);
        });
    },
    [mergedNodes, mergedNodes.length, nodeById, onMoveNode]
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
      const currentNode = nodeById.get(nodeId) ?? null;
      const currentDocumentID =
        currentNode?.type === "doc" ? (currentNode.documentId ?? currentNode.id ?? "").trim() : "";
      const isFolder = item.data.type === "folder" || item.isFolder;
      const isActive = nodeId === activeDocId || (currentDocumentID !== "" && currentDocumentID === activeDocId);
      const isActionMenuOpen = openActionNodeId === nodeId;
      const isInlineEditing = editingNodeId === nodeId;
      const isDraftNode = draftNodeByID.has(nodeId);
      const isCreatingDraftNode = creatingDraftNodeIdSet.has(nodeId);
      const isUpdatingVisibility = updatingVisibilityNodeIdSet.has(nodeId);
      const isUpdatingIdentifier = updatingIdentifierNodeIdSet.has(nodeId);
      const currentDocumentVisibility: Visibility =
        currentNode?.type === "doc" ? currentNode.visibility ?? "member" : "member";
      const nodeTitleText = (currentNode?.title ?? item.data.title ?? "").trim() || "未命名文档";
      const rowStyle = {
        ...(context.itemContainerWithoutChildrenProps.style ?? {}),
        paddingLeft: `${8 + depth * 20}px`,
        cursor: "pointer"
      };
      const childDropLineStyle = {
        left: `${8 + (depth + 1) * 20}px`,
        right: "8px"
      };
      const interactiveType = context.isRenaming || isInlineEditing ? undefined : "button";
      const InteractiveComponent = context.isRenaming || isInlineEditing ? "div" : "button";
      const treeInteractiveElementProps = !isInlineEditing ? ((context.interactiveElementProps as any) ?? {}) : {};

      const openCurrentDocument = () => {
        if (!currentDocumentID || isDraftNode) {
          return;
        }
        setOpenActionNodeId(null);
        void onOpenDocument(currentDocumentID).catch(() => {
          // 上层会统一更新状态与路由，这里吞掉 Promise rejection 避免控制台噪音。
        });
      };

      return (
        <li {...(context.itemContainerWithChildrenProps as any)} className="m-0 p-0">
          <div
            {...(context.itemContainerWithoutChildrenProps as any)}
            id={`workspace-tree-item-${nodeId}`}
            className={mergeClassNames(
              "group relative flex min-h-[36px] w-full cursor-pointer items-center rounded-[10px] pr-2 text-[14px] text-[#2f2f30]",
              isActive ? "bg-[#d9dade]" : "bg-transparent hover:bg-[#e8e8ea]",
              context.isDraggingOver && "bg-[#d5e5ff]",
              context.isDraggingOverParent && "bg-[#e6f0ff]",
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
                aria-hidden={isFolder ? undefined : true}
              >
              {isFolder ? context.isExpanded ? <ChevronDown size={15} /> : <ChevronRight size={15} /> : null}
            </span>
            <InteractiveComponent
              type={interactiveType}
              {...treeInteractiveElementProps}
              onClick={
                !isInlineEditing && currentNode?.type === "doc"
                  ? (event: MouseEvent<HTMLElement>) => {
                      event.preventDefault();
                      event.stopPropagation();
                      openCurrentDocument();
                    }
                  : treeInteractiveElementProps.onClick
              }
              onKeyDown={
                !isInlineEditing && currentNode?.type === "doc"
                  ? (event: ReactKeyboardEvent<HTMLElement>) => {
                      if (event.key !== "Enter" && event.key !== " ") {
                        return;
                      }
                      event.preventDefault();
                      event.stopPropagation();
                      openCurrentDocument();
                    }
                  : treeInteractiveElementProps.onKeyDown
              }
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
                  {currentNode?.type === "doc" ? (
                    (() => {
                      const marker = resolveVisibilityMarkerConfig(currentDocumentVisibility);
                      if (isUpdatingVisibility) {
                        return (
                          <Tooltip delayDuration={120}>
                            <TooltipTrigger asChild>
                              <span
                                className="mr-1 inline-flex h-[18px] w-[18px] shrink-0 items-center justify-center text-[#64748b]"
                                aria-label="正在更新可见性"
                              >
                                <LoaderCircle size={14} className="animate-spin" />
                              </span>
                            </TooltipTrigger>
                            <TooltipContent side="top">正在更新可见性...</TooltipContent>
                          </Tooltip>
                        );
                      }
                      return (
                        <Tooltip delayDuration={120}>
                          <TooltipTrigger asChild>
                            <span
                              className={mergeClassNames(
                                "mr-1 inline-flex h-[18px] w-[18px] shrink-0 items-center justify-center",
                                marker.className
                              )}
                              aria-label={`可见性：${marker.label}`}
                            >
                              {marker.variant === "member" ? <Lock size={14} /> : null}
                              {marker.variant === "authenticated" ? <LockOpen size={14} /> : null}
                              {marker.variant === "public" ? (
                                <span className="relative inline-flex h-[14px] w-[14px] items-center justify-center">
                                  <Lock size={14} />
                                  <span className="pointer-events-none absolute h-[2px] w-[15px] rotate-[-35deg] rounded-full bg-current" />
                                </span>
                              ) : null}
                            </span>
                          </TooltipTrigger>
                          <TooltipContent side="top">可见性：{marker.label}</TooltipContent>
                        </Tooltip>
                      );
                    })()
                  ) : null}
                  <Tooltip delayDuration={120}>
                    <TooltipTrigger asChild>
                      <span
                        className={mergeClassNames(
                          "min-w-0 truncate leading-[1.3]",
                          isActive && "font-semibold"
                        )}
                      >
                        {title}
                      </span>
                    </TooltipTrigger>
                    <TooltipContent side="top" align="start" className="max-w-[320px] whitespace-pre-wrap break-words">
                      {nodeTitleText}
                    </TooltipContent>
                  </Tooltip>
                  {isUpdatingVisibility ? (
                    <span className="ml-2 shrink-0 text-[11px] text-[#64748b]">权限更新中...</span>
                  ) : null}
                  {isUpdatingIdentifier ? (
                    <span className="ml-2 shrink-0 text-[11px] text-[#64748b]">标识更新中...</span>
                  ) : null}
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
                    {currentNode?.type === "doc" ? (
                      <>
                        <div className="my-1 h-px bg-[#eceff3]" />
                        <button
                          type="button"
                          className={mergeClassNames(
                            "inline-flex min-h-[34px] w-full items-center gap-2 rounded-[8px] border-0 bg-transparent px-2.5 text-left text-[13px] text-[#2f2f30] focus-visible:outline-none",
                            isUpdatingIdentifier ? "cursor-not-allowed opacity-60" : "hover:bg-[#f0f2f4]"
                          )}
                          role="menuitem"
                          disabled={isUpdatingIdentifier}
                          onMouseDown={stopTreeItemEvent}
                          onClick={(event) => {
                            stopTreeItemEvent(event);
                            openEditDocumentIdentifierDialog(nodeId);
                          }}
                        >
                          <Link2 size={14} />
                          <span>设置文档标识</span>
                        </button>
                        <button
                          type="button"
                          className={mergeClassNames(
                            "inline-flex min-h-[34px] w-full items-center gap-2 rounded-[8px] border-0 bg-transparent px-2.5 text-left text-[13px] text-[#2f2f30] focus-visible:outline-none",
                            isUpdatingVisibility ? "cursor-not-allowed opacity-60" : "hover:bg-[#f0f2f4]"
                          )}
                          role="menuitemradio"
                          aria-checked={currentDocumentVisibility === "public"}
                          disabled={isUpdatingVisibility}
                          onMouseDown={stopTreeItemEvent}
                          onClick={(event) => {
                            stopTreeItemEvent(event);
                            void runActionMenuTask(() => handleUpdateNodeVisibility(nodeId, "public"));
                          }}
                        >
                          <Globe size={14} />
                          <span className="flex min-w-0 flex-1 items-center justify-between gap-2">
                            <span>设为完全公开</span>
                            {currentDocumentVisibility === "public" ? <Check size={13} /> : null}
                          </span>
                        </button>
                        <button
                          type="button"
                          className={mergeClassNames(
                            "inline-flex min-h-[34px] w-full items-center gap-2 rounded-[8px] border-0 bg-transparent px-2.5 text-left text-[13px] text-[#2f2f30] focus-visible:outline-none",
                            isUpdatingVisibility ? "cursor-not-allowed opacity-60" : "hover:bg-[#f0f2f4]"
                          )}
                          role="menuitemradio"
                          aria-checked={currentDocumentVisibility === "authenticated"}
                          disabled={isUpdatingVisibility}
                          onMouseDown={stopTreeItemEvent}
                          onClick={(event) => {
                            stopTreeItemEvent(event);
                            void runActionMenuTask(() => handleUpdateNodeVisibility(nodeId, "authenticated"));
                          }}
                        >
                          <Lock size={14} />
                          <span className="flex min-w-0 flex-1 items-center justify-between gap-2">
                            <span>设为登录可见</span>
                            {currentDocumentVisibility === "authenticated" ? <Check size={13} /> : null}
                          </span>
                        </button>
                        <button
                          type="button"
                          className={mergeClassNames(
                            "inline-flex min-h-[34px] w-full items-center gap-2 rounded-[8px] border-0 bg-transparent px-2.5 text-left text-[13px] text-[#2f2f30] focus-visible:outline-none",
                            isUpdatingVisibility ? "cursor-not-allowed opacity-60" : "hover:bg-[#f0f2f4]"
                          )}
                          role="menuitemradio"
                          aria-checked={currentDocumentVisibility === "member"}
                          disabled={isUpdatingVisibility}
                          onMouseDown={stopTreeItemEvent}
                          onClick={(event) => {
                            stopTreeItemEvent(event);
                            void runActionMenuTask(() => handleUpdateNodeVisibility(nodeId, "member"));
                          }}
                        >
                          <Users size={14} />
                          <span className="flex min-w-0 flex-1 items-center justify-between gap-2">
                            <span>设为成员可见</span>
                            {currentDocumentVisibility === "member" ? <Check size={13} /> : null}
                          </span>
                        </button>
                      </>
                    ) : null}
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
            {context.isDraggingOver ? (
              <div
                aria-hidden
                className="pointer-events-none absolute bottom-0 border-t-2 border-[#4a76d1]"
                style={childDropLineStyle}
              />
            ) : null}
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
      openEditDocumentIdentifierDialog,
      handleRenameNode,
      handleUpdateNodeVisibility,
      cancelInlineEdit,
      commitInlineEdit,
      editingNodeId,
      nodeById,
      removeDraftNode,
      editingNodeTitle,
      openActionNodeId,
      runActionMenuTask,
      stopTreeItemPropagation,
      stopTreeItemEvent,
      updatingIdentifierNodeIdSet,
      updatingVisibilityNodeIdSet
    ]
  );

  return (
    <TooltipProvider delayDuration={120}>
      {confirmDialog}
      {createNodeDialog !== null ? (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/35 px-4"
          onMouseDown={(event) => {
            if (event.target !== event.currentTarget) {
              return;
            }
            closeCreateNodeDialog();
          }}
        >
          <div
            className="w-full max-w-[440px] rounded-[14px] bg-white p-5 shadow-[0_22px_48px_rgba(15,23,42,0.28)]"
            onMouseDown={(event) => {
              event.stopPropagation();
            }}
          >
            <div className="mb-4">
              <h3 className="m-0 text-[16px] font-semibold text-[#1f2328]">
                {createNodeDialog.type === "folder" ? "新建目录" : "新建文档"}
              </h3>
              <p className="mt-1 mb-0 text-[13px] leading-[1.55] text-[#5f6468]">
                {createNodeDialog.type === "folder"
                  ? "目录创建后会直接出现在当前节点下。"
                  : "可选文档标识将用于阅读页 URL，例如 /r/space/quick-start。"}
              </p>
            </div>
            <label className={mergeClassNames("block", createNodeDialog.type === "doc" && "mb-3")}>
              <span className="mb-1.5 block text-[13px] font-medium text-[#1f2328]">
                {createNodeDialog.type === "folder" ? "目录标题" : "文档标题"}
              </span>
              <input
                value={createNodeDialog.title}
                className="h-9 w-full rounded-[9px] border border-[#ccd2d8] bg-white px-3 text-[13px] text-[#1f2328] outline-none transition-colors focus:border-[#8ea8c4]"
                disabled={isCreateNodeDialogSubmitting}
                onChange={(event) => {
                  const nextTitle = event.target.value;
                  setCreateNodeDialog((previousDialog) =>
                    previousDialog ? { ...previousDialog, title: nextTitle } : previousDialog
                  );
                }}
                onKeyDown={(event) => {
                  if (event.key === "Escape") {
                    event.preventDefault();
                    closeCreateNodeDialog();
                  }
                }}
              />
            </label>
            {createNodeDialog.type === "doc" ? (
              <>
                <label className="mb-3 block">
                  <span className="mb-1.5 block text-[13px] font-medium text-[#1f2328]">模板（可选）</span>
                  <select
                    value={createNodeDialog.templateId}
                    className="h-9 w-full rounded-[9px] border border-[#ccd2d8] bg-white px-3 text-[13px] text-[#1f2328] outline-none transition-colors focus:border-[#8ea8c4]"
                    disabled={isCreateNodeDialogSubmitting || isDocumentTemplatesLoading}
                    onChange={(event) => {
                      const nextTemplateID = event.target.value;
                      setCreateNodeDialog((previousDialog) =>
                        previousDialog
                          ? {
                              ...previousDialog,
                              templateId: nextTemplateID
                            }
                          : previousDialog
                      );
                    }}
                    onKeyDown={(event) => {
                      if (event.key === "Escape") {
                        event.preventDefault();
                        closeCreateNodeDialog();
                      }
                    }}
                  >
                    <option value="">不使用模板</option>
                    {groupedDocumentTemplateOptions.map((group) => (
                      <optgroup key={group.sceneKey} label={group.sceneLabel}>
                        {group.options.map((item) => (
                          <option key={item.templateId} value={item.templateId}>
                            {item.name} ({item.templateId})
                          </option>
                        ))}
                      </optgroup>
                    ))}
                  </select>
                </label>
                <p className="mt-0 mb-3 text-[12px] text-[#6b7280]">
                  {isDocumentTemplatesLoading
                    ? "模板加载中..."
                    : documentTemplatesError
                      ? `模板加载失败：${documentTemplatesError}`
                      : documentTemplatesLoaded
                        ? documentTemplateOptions.length > 0
                          ? `当前可用模板 ${documentTemplateOptions.length} 个，按场景分组展示。`
                          : "当前暂无可用模板，可联系管理员在后台「模板管理」中创建并启用模板。"
                        : "将按需加载模板列表。"}
                  {documentTemplatesError ? (
                    <button
                      type="button"
                      className="ml-2 rounded border border-[#cdd5df] bg-white px-1.5 py-0.5 text-[11px] text-[#334155] hover:bg-[#f8fafc]"
                      onClick={() => {
                        void loadDocumentTemplates(true);
                      }}
                      disabled={isDocumentTemplatesLoading || isCreateNodeDialogSubmitting}
                    >
                      重试
                    </button>
                  ) : null}
                </p>
                {createNodeDialog.templateId ? (
                  <div className="mb-3 rounded-[10px] border border-[#e5e7eb] bg-[#f8fafc] px-3 py-2.5">
                    {isTemplatePreviewLoading ? (
                      <p className="m-0 text-[12px] text-[#475467]">模板预览加载中...</p>
                    ) : templatePreviewError ? (
                      <p className="m-0 text-[12px] text-[#b42318]">
                        模板预览加载失败：{templatePreviewError}
                        <button
                          type="button"
                          className="ml-2 rounded border border-[#fecdd3] bg-white px-1.5 py-0.5 text-[11px] text-[#b42318] hover:bg-[#fff1f2]"
                          onClick={() => {
                            void loadDocumentTemplateDetail(selectedTemplateID, { forceReload: true });
                          }}
                          disabled={isCreateNodeDialogSubmitting}
                        >
                          重试
                        </button>
                      </p>
                    ) : selectedTemplateDetail ? (
                      <>
                        <div className="mb-2 flex items-center justify-between gap-2">
                          <span className="text-[12px] font-medium text-[#0f172a]">{selectedTemplateDetail.name}</span>
                          <span className="text-[11px] text-[#64748b]">{selectedTemplateDetail.sceneName}</span>
                        </div>
                        {selectedTemplateDetail.defaultTitle ? (
                          <p className="mt-0 mb-2 text-[11px] text-[#475569]">
                            默认标题：{selectedTemplateDetail.defaultTitle}
                          </p>
                        ) : null}
                        <pre className="m-0 max-h-[132px] overflow-auto whitespace-pre-wrap break-words rounded-[8px] bg-white px-2.5 py-2 text-[11px] leading-[1.5] text-[#334155]">
                          {selectedTemplatePreviewText}
                        </pre>
                      </>
                    ) : (
                      <p className="m-0 text-[12px] text-[#475467]">模板预览不可用。</p>
                    )}
                  </div>
                ) : null}
                <label className="block">
                  <span className="mb-1.5 block text-[13px] font-medium text-[#1f2328]">文档标识（可空）</span>
                  <input
                    value={createNodeDialog.documentIdentifier}
                    className="h-9 w-full rounded-[9px] border border-[#ccd2d8] bg-white px-3 text-[13px] text-[#1f2328] outline-none transition-colors focus:border-[#8ea8c4]"
                    placeholder="留空则使用 documentId"
                    disabled={isCreateNodeDialogSubmitting}
                    onChange={(event) => {
                      const nextIdentifier = event.target.value;
                      setCreateNodeDialog((previousDialog) =>
                        previousDialog
                          ? {
                              ...previousDialog,
                              documentIdentifier: nextIdentifier
                            }
                          : previousDialog
                      );
                    }}
                    onKeyDown={(event) => {
                      if (event.key === "Enter") {
                        event.preventDefault();
                        void handleCreateNodeByDialog();
                        return;
                      }
                      if (event.key === "Escape") {
                        event.preventDefault();
                        closeCreateNodeDialog();
                      }
                    }}
                  />
                </label>
                <p className="mt-2 mb-0 text-[12px] text-[#6b7280]">
                  仅支持小写字母、数字和连字符（-），并在同一空间内保持唯一。
                </p>
              </>
            ) : null}
            <div className="mt-5 flex items-center justify-end gap-2">
              <button
                type="button"
                className="inline-flex h-8 items-center rounded-[8px] border border-[#d0d5db] bg-white px-3 text-[13px] text-[#344054] hover:bg-[#f8fafc] focus-visible:outline-none"
                onClick={closeCreateNodeDialog}
                disabled={isCreateNodeDialogSubmitting}
              >
                取消
              </button>
              <button
                type="button"
                className="inline-flex h-8 items-center rounded-[8px] border-0 bg-[#3b82f6] px-3 text-[13px] font-medium text-white hover:bg-[#2563eb] focus-visible:outline-none"
                onClick={() => {
                  void handleCreateNodeByDialog();
                }}
                disabled={isCreateNodeDialogSubmitting}
              >
                {isCreateNodeDialogSubmitting ? "创建中..." : "创建"}
              </button>
            </div>
          </div>
        </div>
      ) : null}
      {editIdentifierDialogNodeID !== null ? (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/35 px-4"
          onMouseDown={(event) => {
            if (event.target !== event.currentTarget) {
              return;
            }
            closeEditDocumentIdentifierDialog();
          }}
        >
          <div
            className="w-full max-w-[440px] rounded-[14px] bg-white p-5 shadow-[0_22px_48px_rgba(15,23,42,0.28)]"
            onMouseDown={(event) => {
              event.stopPropagation();
            }}
          >
            <div className="mb-4">
              <h3 className="m-0 text-[16px] font-semibold text-[#1f2328]">设置文档标识</h3>
              <p className="mt-1 mb-0 text-[13px] leading-[1.55] text-[#5f6468]">
                当前文档：<span className="font-medium text-[#1f2328]">{editIdentifierDialogTitle}</span>
              </p>
            </div>
            <label className="block">
              <span className="mb-1.5 block text-[13px] font-medium text-[#1f2328]">文档标识</span>
              <input
                value={editIdentifierDialogValue}
                className="h-9 w-full rounded-[9px] border border-[#ccd2d8] bg-white px-3 text-[13px] text-[#1f2328] outline-none transition-colors focus:border-[#8ea8c4]"
                placeholder="留空表示清空标识并回退到文档 ID"
                onChange={(event) => {
                  setEditIdentifierDialogValue(event.target.value);
                }}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    void handleUpdateDocumentIdentifier();
                    return;
                  }
                  if (event.key === "Escape") {
                    event.preventDefault();
                    closeEditDocumentIdentifierDialog();
                  }
                }}
              />
            </label>
            <p className="mt-2 mb-0 text-[12px] text-[#6b7280]">
              仅支持小写字母、数字和连字符（-），同一空间内唯一。
            </p>
            <div className="mt-5 flex items-center justify-end gap-2">
              <button
                type="button"
                className="inline-flex h-8 items-center rounded-[8px] border border-[#d0d5db] bg-white px-3 text-[13px] text-[#344054] hover:bg-[#f8fafc] focus-visible:outline-none"
                onClick={closeEditDocumentIdentifierDialog}
              >
                取消
              </button>
              <button
                type="button"
                className="inline-flex h-8 items-center rounded-[8px] border-0 bg-[#3b82f6] px-3 text-[13px] font-medium text-white hover:bg-[#2563eb] focus-visible:outline-none"
                onClick={() => {
                  void handleUpdateDocumentIdentifier();
                }}
              >
                保存
              </button>
            </div>
          </div>
        </div>
      ) : null}
      <div className="mb-2 flex h-11 items-center justify-between border-b border-[#d9dade] px-2">
        <span className="text-[18px] font-semibold text-[#1f2328]">目录</span>
        <DropdownMenu modal={false}>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              className="inline-flex h-8 w-8 cursor-pointer items-center justify-center rounded-[8px] border-0 bg-transparent text-[#1f2328] transition-colors hover:bg-[#e7e8ea] data-[state=open]:bg-[#e7e8ea] focus:outline-none focus-visible:outline-none"
              aria-label="打开目录快捷菜单"
              disabled={isCreateNodeDialogSubmitting}
            >
              <Plus size={18} />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            align="end"
            className="min-w-[148px]"
            onCloseAutoFocus={(event) => {
              // 弹窗创建关闭后不强制把焦点归回触发按钮，避免焦点跳动。
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
            disabled={isCreateNodeDialogSubmitting}
          >
            <FilePlus2 size={14} />
            <span>{isCreateNodeDialogSubmitting ? "创建中..." : "新建第一篇文档"}</span>
          </button>
        </div>
      ) : (
        <ControlledTreeEnvironment<WorkspaceTreeItemData>
          items={items}
          getItemTitle={(item) => item.data.title}
          viewState={viewState}
          autoFocus={false}
          defaultInteractionMode={InteractionMode.ClickArrowToExpand}
          canDragAndDrop={isDesktopDragEnabled}
          canDropOnFolder
          canDropOnNonFolder
          canReorderItems
          canDropBelowOpenFolders
          canDrag={canDragItems}
          canDropAt={canDropAt}
          canSearch={false}
          canRename={false}
          onDrop={handleDropNodes}
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
          renderDragBetweenLine={({ lineProps }) => (
            <div
              {...lineProps}
              className={mergeClassNames("rounded-full border-t-2 border-[#4a76d1]", lineProps.className)}
            />
          )}
          renderItem={renderTreeItem}
        >
          <Tree treeId={WORKSPACE_TREE_ID} rootItem={WORKSPACE_TREE_ROOT_ID} treeLabel="工作区目录树" />
        </ControlledTreeEnvironment>
      )}
    </TooltipProvider>
  );
});
