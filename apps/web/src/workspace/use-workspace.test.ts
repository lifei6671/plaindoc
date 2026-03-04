import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { DataGateway, Document, Space, TreeNode } from "../data-access";
import { useWorkspace } from "./use-workspace";

function buildTree(spaceID: string): TreeNode[] {
  return [
    {
      id: "node-doc-1",
      documentId: "doc-1",
      documentIdentifier: "intro",
      documentRouteKey: "intro",
      spaceId: spaceID,
      parentId: null,
      type: "doc",
      title: "介绍",
      sort: 1,
      visibility: "member",
      children: []
    }
  ];
}

function buildDocument(): Document {
  return {
    id: "doc-1",
    nodeId: "node-doc-1",
    themeId: "default",
    title: "介绍",
    contentMd: "# intro",
    version: 1,
    updatedAt: "2026-03-04T09:00:00Z",
    visibility: "member"
  };
}

function createGatewayMocks() {
  const activeSpace: Space = {
    id: "space-1",
    name: "默认空间",
    createdAt: "2026-03-04T08:00:00Z",
    updatedAt: "2026-03-04T08:00:00Z"
  };
  const workspaceTree = buildTree(activeSpace.id);

  const listSpaces = vi.fn().mockResolvedValue([activeSpace]);
  const getSpace = vi.fn().mockResolvedValue(activeSpace);
  const createSpace = vi.fn();
  const getTree = vi.fn().mockResolvedValue(workspaceTree);
  const createNode = vi.fn().mockResolvedValue({
    nodeId: "node-doc-2",
    docId: "doc-2"
  });
  const updateNode = vi.fn();
  const deleteNode = vi.fn();
  const moveNode = vi.fn();

  const getDocument = vi.fn().mockResolvedValue(buildDocument());
  const saveDocument = vi.fn();
  const updateDocumentIdentifier = vi.fn();
  const localizeRemoteImages = vi.fn();
  const listRevisions = vi.fn();
  const listAttachments = vi.fn();
  const uploadAttachment = vi.fn();
  const deleteAttachment = vi.fn();
  const createAttachmentAccessLink = vi.fn();
  const setDocumentTheme = vi.fn();
  const updateDocumentVisibility = vi.fn();

  const dataGateway = {
    workspace: {
      listSpaces,
      getSpace,
      createSpace,
      getTree,
      createNode,
      updateNode,
      deleteNode,
      moveNode
    },
    document: {
      getDocument,
      saveDocument,
      updateDocumentIdentifier,
      localizeRemoteImages,
      listRevisions,
      listAttachments,
      uploadAttachment,
      deleteAttachment,
      createAttachmentAccessLink,
      setDocumentTheme,
      updateDocumentVisibility
    }
  } as unknown as DataGateway;

  return {
    dataGateway,
    activeSpace,
    mocks: {
      getTree,
      createNode
    }
  };
}

describe("useWorkspace", () => {
  it("forwards template and identifier when creating a document node", async () => {
    const { dataGateway, activeSpace, mocks } = createGatewayMocks();
    const { result } = renderHook(() =>
      useWorkspace({
        dataGateway,
        initialContent: "",
        defaultDocumentTitle: "未命名文档"
      })
    );

    await act(async () => {
      await result.current.bootstrapWorkspace();
    });

    await act(async () => {
      await result.current.createNode({
        parentId: null,
        type: "doc",
        title: "  项目规范  ",
        documentIdentifier: "project-guide",
        templateId: "tpl-project-guide"
      });
    });

    expect(mocks.createNode).toHaveBeenCalledTimes(1);
    expect(mocks.createNode).toHaveBeenCalledWith({
      spaceId: activeSpace.id,
      parentId: null,
      type: "doc",
      title: "项目规范",
      documentIdentifier: "project-guide",
      templateId: "tpl-project-guide"
    });
    expect(mocks.getTree).toHaveBeenCalledWith(activeSpace.id);
    expect(mocks.getTree).toHaveBeenCalledTimes(2);
  });
});
