import { useCallback, useState } from "react";
import type { CreateNodeResult, Document, MoveNodeInput, Space, TreeNode } from "../data-access";
import { findFirstDocId, formatError } from "../editor/status-utils";
import type {
  UseWorkspaceOptions,
  UseWorkspaceResult,
  WorkspaceBootstrapResult,
  WorkspaceCreateNodeInput,
  WorkspaceMoveNodeInput
} from "./types";

const DEFAULT_SPACE_NAME = "默认空间";
const DEFAULT_ACTIVE_SPACE_NAME = "未命名空间";
const DEFAULT_DOCUMENT_TITLE = "未命名文档";

// 规范化文档标题：空字符串或全空白时回退默认值。
function resolveDocumentTitle(title: string, fallbackTitle: string): string {
  const normalizedTitle = title.trim();
  return normalizedTitle ? normalizedTitle : fallbackTitle;
}

// 统一排序空间列表：按最近更新时间倒序，保证切换器展示稳定。
function sortSpaces(spaces: Space[]): Space[] {
  return [...spaces].sort((left, right) => right.updatedAt.localeCompare(left.updatedAt));
}

// 合并空间列表并按空间 ID 去重：保证本地快照不会出现重复项。
function mergeUniqueSpaces(...spaceGroups: Space[][]): Space[] {
  const mergedMap = new Map<string, Space>();
  for (const group of spaceGroups) {
    for (const space of group) {
      mergedMap.set(space.id, space);
    }
  }
  return sortSpaces(Array.from(mergedMap.values()));
}

// 判断目标节点是否仍在目录树中：用于删除后校验当前激活文档是否有效。
function containsNode(nodes: TreeNode[], nodeId: string): boolean {
  for (const node of nodes) {
    if (node.id === nodeId) {
      return true;
    }
    if (node.children.length > 0 && containsNode(node.children, nodeId)) {
      return true;
    }
  }
  return false;
}

// 根据节点 ID 查找节点快照：用于按类型决定默认命名文案。
function findNodeById(nodes: TreeNode[], nodeId: string): TreeNode | null {
  for (const node of nodes) {
    if (node.id === nodeId) {
      return node;
    }
    if (node.children.length > 0) {
      const childMatched = findNodeById(node.children, nodeId);
      if (childMatched) {
        return childMatched;
      }
    }
  }
  return null;
}

// 工作区状态 Hook：集中管理空间、目录树与当前文档装载流程。
export function useWorkspace(options: UseWorkspaceOptions): UseWorkspaceResult {
  const {
    dataGateway,
    initialContent,
    initialSpaceName = DEFAULT_ACTIVE_SPACE_NAME,
    initialDocumentTitle = DEFAULT_DOCUMENT_TITLE,
    defaultSpaceName = DEFAULT_SPACE_NAME,
    defaultDocumentTitle = DEFAULT_DOCUMENT_TITLE
  } = options;

  // 空间列表快照：供侧边栏空间切换器消费。
  const [spaces, setSpaces] = useState<Space[]>([]);
  // 当前工作区空间 ID 与名称。
  const [activeSpaceId, setActiveSpaceId] = useState<string | null>(null);
  const [activeSpaceName, setActiveSpaceName] = useState(initialSpaceName);
  // 当前空间目录树快照：后续侧边栏树结构会直接消费该状态。
  const [workspaceTree, setWorkspaceTree] = useState<TreeNode[]>([]);
  // 当前打开文档的身份与内容快照。
  const [activeDocId, setActiveDocId] = useState<string | null>(null);
  const [activeDocumentTitle, setActiveDocumentTitle] = useState(initialDocumentTitle);
  const [content, setContent] = useState(initialContent);
  const [baseVersion, setBaseVersion] = useState(0);
  const [lastSavedContent, setLastSavedContent] = useState(initialContent);
  const [lastSavedAt, setLastSavedAt] = useState<string | null>(null);
  // 启动与异常状态：用于上层决定提示文案与兜底行为。
  const [isWorkspaceBootstrapping, setIsWorkspaceBootstrapping] = useState(false);
  const [workspaceErrorMessage, setWorkspaceErrorMessage] = useState<string | null>(null);

  // 读取并应用文档快照：切换文档与启动加载都复用该流程。
  const openDocument = useCallback(
    async (docId: string): Promise<Document> => {
      try {
        const document = await dataGateway.document.getDocument(docId);
        setActiveDocId(document.id);
        setActiveDocumentTitle(resolveDocumentTitle(document.title, defaultDocumentTitle));
        setContent(document.contentMd);
        setBaseVersion(document.version);
        setLastSavedContent(document.contentMd);
        setLastSavedAt(document.updatedAt);
        setWorkspaceErrorMessage(null);
        return document;
      } catch (error) {
        setWorkspaceErrorMessage(formatError(error));
        throw error;
      }
    },
    [dataGateway, defaultDocumentTitle]
  );

  // 重新拉取当前空间目录树：供后续 CRUD 及空间切换复用。
  const reloadTree = useCallback(
    async (spaceId?: string): Promise<TreeNode[]> => {
      const targetSpaceId = spaceId ?? activeSpaceId;
      if (!targetSpaceId) {
        setWorkspaceTree([]);
        return [];
      }
      const tree = await dataGateway.workspace.getTree(targetSpaceId);
      setWorkspaceTree(tree);
      return tree;
    },
    [activeSpaceId, dataGateway]
  );

  // 刷新空间列表：供初始化与后续空间操作复用。
  const refreshSpaces = useCallback(async (): Promise<Space[]> => {
    const listedSpaces = await dataGateway.workspace.listSpaces();
    const sortedSpaces = sortSpaces(listedSpaces);
    setSpaces(sortedSpaces);
    return sortedSpaces;
  }, [dataGateway.workspace]);

  // 确保目标空间存在可编辑文档：不存在时自动创建默认文档。
  const ensureSpaceReady = useCallback(
    async (space: Space): Promise<WorkspaceBootstrapResult> => {
      setActiveSpaceId(space.id);
      setActiveSpaceName(space.name);

      let tree = await dataGateway.workspace.getTree(space.id);
      setWorkspaceTree(tree);

      let docId = findFirstDocId(tree);
      if (!docId) {
        const created = await dataGateway.workspace.createNode({
          spaceId: space.id,
          parentId: null,
          type: "doc",
          title: defaultDocumentTitle
        });
        docId = created.docId ?? null;
        if (!docId) {
          throw new Error("无法创建初始化文档");
        }
        // 创建文档后刷新目录树，确保树状态与数据层一致。
        tree = await dataGateway.workspace.getTree(space.id);
        setWorkspaceTree(tree);
      }

      const document = await openDocument(docId);
      return {
        spaceId: space.id,
        spaceName: space.name,
        docId: document.id,
        documentVersion: document.version
      };
    },
    [dataGateway.workspace, defaultDocumentTitle, openDocument]
  );

  // 根据空间 ID 定位空间对象：优先使用本地快照，兜底刷新列表。
  const findSpaceById = useCallback(
    async (spaceId: string): Promise<Space> => {
      const localMatched = spaces.find((item) => item.id === spaceId);
      if (localMatched) {
        return localMatched;
      }
      const latestSpaces = await refreshSpaces();
      const latestMatched = latestSpaces.find((item) => item.id === spaceId);
      if (!latestMatched) {
        throw new Error("目标空间不存在");
      }
      return latestMatched;
    },
    [refreshSpaces, spaces]
  );

  // 切换当前空间并自动打开该空间首篇文档。
  const switchSpace = useCallback(
    async (spaceId: string): Promise<WorkspaceBootstrapResult> => {
      setWorkspaceErrorMessage(null);
      try {
        const targetSpace = await findSpaceById(spaceId);
        return await ensureSpaceReady(targetSpace);
      } catch (error) {
        setWorkspaceErrorMessage(formatError(error));
        throw error;
      }
    },
    [ensureSpaceReady, findSpaceById]
  );

  // 创建空间并自动切换：保证用户创建后立即进入可编辑状态。
  const createSpace = useCallback(
    async (spaceName: string): Promise<WorkspaceBootstrapResult> => {
      const normalizedSpaceName = spaceName.trim() || defaultSpaceName;
      setWorkspaceErrorMessage(null);
      try {
        const createdSpace = await dataGateway.workspace.createSpace({
          name: normalizedSpaceName
        });
        setSpaces((previousSpaces) => mergeUniqueSpaces([createdSpace], previousSpaces));
        return await ensureSpaceReady(createdSpace);
      } catch (error) {
        setWorkspaceErrorMessage(formatError(error));
        throw error;
      }
    },
    [dataGateway.workspace, defaultSpaceName, ensureSpaceReady]
  );

  // 新增目录节点：支持文档/目录创建，完成后刷新当前树快照。
  const createNode = useCallback(
    async (input: WorkspaceCreateNodeInput): Promise<CreateNodeResult> => {
      const targetSpaceId = activeSpaceId;
      if (!targetSpaceId) {
        throw new Error("当前未激活空间，无法创建目录节点。");
      }
      const fallbackTitle = input.type === "folder" ? "未命名目录" : defaultDocumentTitle;
      const normalizedTitle = resolveDocumentTitle(input.title, fallbackTitle);
      setWorkspaceErrorMessage(null);
      try {
        const created = await dataGateway.workspace.createNode({
          spaceId: targetSpaceId,
          parentId: input.parentId,
          type: input.type,
          title: normalizedTitle
        });
        await reloadTree(targetSpaceId);
        return created;
      } catch (error) {
        setWorkspaceErrorMessage(formatError(error));
        throw error;
      }
    },
    [activeSpaceId, dataGateway.workspace, defaultDocumentTitle, reloadTree]
  );

  // 重命名目录节点：文档节点重命名后同步更新当前标题状态。
  const renameNode = useCallback(
    async (nodeId: string, title: string): Promise<void> => {
      const node = findNodeById(workspaceTree, nodeId);
      const fallbackTitle = node?.type === "folder" ? "未命名目录" : defaultDocumentTitle;
      const normalizedTitle = resolveDocumentTitle(title, fallbackTitle);
      setWorkspaceErrorMessage(null);
      try {
        await dataGateway.workspace.updateNode({
          nodeId,
          title: normalizedTitle
        });
        await reloadTree();
        if (activeDocId === nodeId) {
          setActiveDocumentTitle(normalizedTitle);
        }
      } catch (error) {
        setWorkspaceErrorMessage(formatError(error));
        throw error;
      }
    },
    [activeDocId, dataGateway.workspace, defaultDocumentTitle, reloadTree, workspaceTree]
  );

  // 删除目录节点：若当前文档被删，自动回退到可用文档；无文档时补建一篇兜底文档。
  const deleteNode = useCallback(
    async (nodeId: string): Promise<void> => {
      const targetSpaceId = activeSpaceId;
      if (!targetSpaceId) {
        throw new Error("当前未激活空间，无法删除目录节点。");
      }
      setWorkspaceErrorMessage(null);
      try {
        await dataGateway.workspace.deleteNode(nodeId);
        let latestTree = await reloadTree(targetSpaceId);

        const canKeepCurrentDoc = activeDocId ? containsNode(latestTree, activeDocId) : false;
        if (canKeepCurrentDoc) {
          return;
        }

        let nextDocId = findFirstDocId(latestTree);
        if (!nextDocId) {
          const created = await dataGateway.workspace.createNode({
            spaceId: targetSpaceId,
            parentId: null,
            type: "doc",
            title: defaultDocumentTitle
          });
          nextDocId = created.docId ?? null;
          latestTree = await reloadTree(targetSpaceId);
          if (!nextDocId) {
            nextDocId = findFirstDocId(latestTree);
          }
        }

        if (nextDocId) {
          await openDocument(nextDocId);
          return;
        }

        // 极端兜底：若仍找不到文档，清空当前激活态避免残留脏 ID。
        setActiveDocId(null);
        setActiveDocumentTitle(defaultDocumentTitle);
        setContent(initialContent);
        setBaseVersion(0);
        setLastSavedContent(initialContent);
        setLastSavedAt(null);
      } catch (error) {
        setWorkspaceErrorMessage(formatError(error));
        throw error;
      }
    },
    [
      activeDocId,
      activeSpaceId,
      dataGateway.workspace,
      defaultDocumentTitle,
      initialContent,
      openDocument,
      reloadTree
    ]
  );

  // 启动工作区：确保至少有一个空间和一篇可编辑文档。
  const bootstrapWorkspace = useCallback(async (): Promise<WorkspaceBootstrapResult> => {
    setIsWorkspaceBootstrapping(true);
    setWorkspaceErrorMessage(null);
    try {
      const listedSpaces = await dataGateway.workspace.listSpaces();
      const sortedSpaces = sortSpaces(listedSpaces);
      const targetSpace =
        sortedSpaces[0] ??
        (await dataGateway.workspace.createSpace({
          name: defaultSpaceName
        }));
      setSpaces(mergeUniqueSpaces([targetSpace], sortedSpaces));
      return await ensureSpaceReady(targetSpace);
    } catch (error) {
      setWorkspaceErrorMessage(formatError(error));
      throw error;
    } finally {
      setIsWorkspaceBootstrapping(false);
    }
  }, [dataGateway.workspace, defaultSpaceName, ensureSpaceReady]);

  // 目录拖拽排序扩展点：本期默认未实现，仅保留调用入口。
  const moveNode = useCallback(
    async (input: WorkspaceMoveNodeInput): Promise<void> => {
      const moveHandler = dataGateway.workspace.moveNode;
      if (!moveHandler) {
        throw new Error("moveNode 尚未实现：该能力保留给后续拖拽排序阶段。");
      }
      const moveInput: MoveNodeInput = {
        nodeId: input.nodeId,
        parentId: input.targetParentId,
        sort: input.targetSort
      };
      await moveHandler(moveInput);
      await reloadTree();
    },
    [dataGateway.workspace, reloadTree]
  );

  // 空间删除扩展点：本期默认未实现，仅保留能力位。
  const deleteSpace = useCallback(
    async (spaceId: string): Promise<void> => {
      const deleteHandler = dataGateway.workspace.deleteSpace;
      if (!deleteHandler) {
        throw new Error("deleteSpace 尚未实现：该能力保留给后续空间管理阶段。");
      }
      await deleteHandler(spaceId);
      // 若删除的是当前空间，重置为初始展示态，避免残留脏状态。
      if (spaceId === activeSpaceId) {
        setActiveSpaceId(null);
        setActiveSpaceName(initialSpaceName);
        setWorkspaceTree([]);
        setActiveDocId(null);
        setActiveDocumentTitle(initialDocumentTitle);
        setContent(initialContent);
        setBaseVersion(0);
        setLastSavedContent(initialContent);
        setLastSavedAt(null);
      }
    },
    [activeSpaceId, dataGateway.workspace, initialContent, initialDocumentTitle, initialSpaceName]
  );

  return {
    spaces,
    activeSpaceId,
    activeSpaceName,
    workspaceTree,
    activeDocId,
    activeDocumentTitle,
    content,
    baseVersion,
    lastSavedContent,
    lastSavedAt,
    isWorkspaceBootstrapping,
    workspaceErrorMessage,
    bootstrapWorkspace,
    refreshSpaces,
    switchSpace,
    createSpace,
    createNode,
    renameNode,
    deleteNode,
    openDocument,
    reloadTree,
    moveNode,
    deleteSpace,
    setActiveSpaceName,
    setActiveDocumentTitle,
    setContent,
    setBaseVersion,
    setLastSavedContent,
    setLastSavedAt
  };
}
