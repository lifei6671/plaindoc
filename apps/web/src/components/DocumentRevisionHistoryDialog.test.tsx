import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  ConflictError,
  type DataGateway,
  type Document,
  type DocumentRevisionDetail,
  type DocumentRevisionSummary,
  type RestoreDocumentRevisionResult
} from "../data-access";
import {
  DocumentRevisionHistoryDialog,
  DocumentRevisionHistoryTrigger
} from "./DocumentRevisionHistoryDialog";
import { TooltipProvider } from "./ui/tooltip";

const mergeViewMock = vi.hoisted(() => {
  const instances: Array<{ config: Record<string, unknown>; destroy: ReturnType<typeof vi.fn> }> = [];
  const MergeView = vi.fn(function MockMergeView(
    this: { config: Record<string, unknown>; destroy: ReturnType<typeof vi.fn> },
    config: Record<string, unknown>
  ) {
    this.config = config;
    this.destroy = vi.fn();
    instances.push(this);
  });
  return { MergeView, instances };
});

vi.mock("@codemirror/merge", () => ({
  MergeView: mergeViewMock.MergeView
}));

beforeEach(() => {
  vi.spyOn(console, "info").mockImplementation(() => undefined);
  vi.spyOn(console, "error").mockImplementation(() => undefined);
  mergeViewMock.MergeView.mockClear();
  mergeViewMock.instances.length = 0;
});

afterEach(() => {
  vi.restoreAllMocks();
});

function createRevision(overrides: Partial<DocumentRevisionSummary>): DocumentRevisionSummary {
  return {
    id: "revision-a",
    documentId: "doc-1",
    version: 1,
    baseVersion: 0,
    createdAt: "2026-05-17T00:00:00Z",
    source: "local",
    format: "markdown",
    editorUser: { userId: "user-a", displayName: "作者甲" },
    ...overrides
  };
}

function createRevisionDetail(overrides: Partial<DocumentRevisionDetail>): DocumentRevisionDetail {
  return {
    ...createRevision(overrides),
    contentMd: "# 历史正文",
    ...overrides
  };
}

function createDocument(overrides: Partial<Document>): Document {
  return {
    id: "doc-1",
    nodeId: "node-1",
    themeId: "default",
    format: "markdown",
    title: "产品说明",
    contentMd: "# 恢复后的正文",
    version: 8,
    updatedAt: "2026-05-17T02:00:00Z",
    ...overrides
  };
}

function createGateway(input: {
  listRevisions: DataGateway["document"]["listRevisions"];
  getRevisionDetail?: DataGateway["document"]["getRevisionDetail"];
  restoreRevision?: DataGateway["document"]["restoreRevision"];
}): DataGateway {
  return {
    document: {
      listRevisions: input.listRevisions,
      getRevisionDetail: input.getRevisionDetail ?? vi.fn(async (_docId: string, revisionId: string) => (
        createRevisionDetail({ id: revisionId, format: "docx", contentMd: undefined })
      )),
      restoreRevision: input.restoreRevision ?? vi.fn()
    }
  } as unknown as DataGateway;
}

describe("DocumentRevisionHistoryTrigger", () => {
  it("opens history dialog only when an active document exists", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();

    const { rerender } = render(
      <TooltipProvider>
        <DocumentRevisionHistoryTrigger disabled onOpen={onOpen} />
      </TooltipProvider>
    );

    const disabledButton = screen.getByRole("button", { name: "历史版本" });
    expect(disabledButton).toBeDisabled();
    await user.click(disabledButton);
    expect(onOpen).not.toHaveBeenCalled();

    rerender(
      <TooltipProvider>
        <DocumentRevisionHistoryTrigger disabled={false} onOpen={onOpen} />
      </TooltipProvider>
    );

    await user.click(screen.getByRole("button", { name: "历史版本" }));
    expect(onOpen).toHaveBeenCalledTimes(1);
  });
});

describe("DocumentRevisionHistoryDialog", () => {
  it("renders a controlled dialog shell for the selected document", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();

    render(
      <DocumentRevisionHistoryDialog
        open
        documentId="doc-1"
        documentTitle="产品说明"
        currentContent="# 当前正文"
        currentDocumentVersion={6}
        hasUnsavedChanges={false}
        dataGateway={createGateway({ listRevisions: vi.fn(async () => []) })}
        onOpenChange={onOpenChange}
        onRestoreSuccess={vi.fn()}
      />
    );

    expect(screen.getByRole("dialog", { name: "历史版本" })).toBeInTheDocument();
    expect(screen.getByText("产品说明")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "关闭历史版本" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("loads the first 30 revision summaries and renders creator fallback", async () => {
    const listRevisions = vi.fn(async () => [
      createRevision({
        id: "revision-2",
        version: 2,
        createdAt: "2026-05-17T01:00:00Z",
        editorUser: null
      }),
      createRevision({
        id: "revision-1",
        version: 1,
        createdAt: "2026-05-17T00:00:00Z"
      })
    ]);

    render(
      <DocumentRevisionHistoryDialog
        open
        documentId="doc-1"
        documentTitle="产品说明"
        currentContent="# 当前正文"
        currentDocumentVersion={6}
        hasUnsavedChanges={false}
        dataGateway={createGateway({ listRevisions })}
        onOpenChange={vi.fn()}
        onRestoreSuccess={vi.fn()}
      />
    );

    expect(screen.getByText("正在加载历史版本...")).toBeInTheDocument();

    expect(await screen.findByRole("button", { name: /版本 2/ })).toBeInTheDocument();
    expect(screen.getAllByText("未知创建人").length).toBeGreaterThan(0);
    expect(screen.getByRole("region", { name: "历史版本详情" })).toBeInTheDocument();
    expect(listRevisions).toHaveBeenCalledWith("doc-1", { page: 1, pageSize: 30 });
  });

  it("loads next page without duplicated revision items", async () => {
    const user = userEvent.setup();
    const firstPageRevisions = [
      createRevision({ id: "revision-3", version: 3 }),
      createRevision({ id: "revision-2", version: 2 }),
      ...Array.from({ length: 28 }, (_, index) => createRevision({
        id: `revision-filler-${index}`,
        version: 100 + index
      }))
    ];
    const listRevisions = vi
      .fn<DataGateway["document"]["listRevisions"]>()
      .mockResolvedValueOnce(firstPageRevisions)
      .mockResolvedValueOnce([
        createRevision({ id: "revision-2", version: 2 }),
        createRevision({ id: "revision-1", version: 1 })
      ]);

    render(
      <DocumentRevisionHistoryDialog
        open
        documentId="doc-1"
        documentTitle="产品说明"
        currentContent="# 当前正文"
        currentDocumentVersion={6}
        hasUnsavedChanges={false}
        dataGateway={createGateway({ listRevisions })}
        onOpenChange={vi.fn()}
        onRestoreSuccess={vi.fn()}
      />
    );

    await screen.findByRole("button", { name: /版本 3/ });
    await user.click(screen.getByRole("button", { name: "加载更多" }));
    await screen.findByRole("button", { name: "版本 1，作者甲" });

    expect(screen.getAllByRole("button", { name: /版本 / })).toHaveLength(31);
    expect(listRevisions).toHaveBeenNthCalledWith(1, "doc-1", { page: 1, pageSize: 30 });
    expect(listRevisions).toHaveBeenNthCalledWith(2, "doc-1", { page: 2, pageSize: 30 });
  });

  it("ignores stale revision list responses after switching documents", async () => {
    let resolveFirstList: (revisions: DocumentRevisionSummary[]) => void = () => undefined;
    const firstListPromise = new Promise<DocumentRevisionSummary[]>((resolve) => {
      resolveFirstList = resolve;
    });
    const listRevisions = vi.fn<DataGateway["document"]["listRevisions"]>((docId) => {
      if (docId === "doc-1") {
        return firstListPromise;
      }
      return Promise.resolve([
        createRevision({ id: "revision-doc-2", documentId: "doc-2", version: 2 })
      ]);
    });

    const { rerender } = render(
      <DocumentRevisionHistoryDialog
        open
        documentId="doc-1"
        documentTitle="产品说明"
        currentContent="# 当前正文"
        currentDocumentVersion={6}
        hasUnsavedChanges={false}
        dataGateway={createGateway({ listRevisions })}
        onOpenChange={vi.fn()}
        onRestoreSuccess={vi.fn()}
      />
    );

    await waitFor(() => {
      expect(listRevisions).toHaveBeenCalledWith("doc-1", { page: 1, pageSize: 30 });
    });

    rerender(
      <DocumentRevisionHistoryDialog
        open
        documentId="doc-2"
        documentTitle="产品说明"
        currentContent="# 当前正文"
        currentDocumentVersion={6}
        hasUnsavedChanges={false}
        dataGateway={createGateway({ listRevisions })}
        onOpenChange={vi.fn()}
        onRestoreSuccess={vi.fn()}
      />
    );

    expect(await screen.findByRole("button", { name: "版本 2，作者甲" })).toBeInTheDocument();

    await act(async () => {
      resolveFirstList([
        createRevision({ id: "revision-doc-1", documentId: "doc-1", version: 1 })
      ]);
      await firstListPromise;
    });

    await waitFor(() => {
      expect(screen.queryByRole("button", { name: "版本 1，作者甲" })).not.toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "版本 2，作者甲" })).toBeInTheDocument();
  });

  it("renders empty state and retries after a loading error", async () => {
    const user = userEvent.setup();
    const listRevisions = vi
      .fn<DataGateway["document"]["listRevisions"]>()
      .mockRejectedValueOnce(new Error("网络异常"))
      .mockResolvedValueOnce([]);

    render(
      <DocumentRevisionHistoryDialog
        open
        documentId="doc-1"
        documentTitle="产品说明"
        currentContent="# 当前正文"
        currentDocumentVersion={6}
        hasUnsavedChanges={false}
        dataGateway={createGateway({ listRevisions })}
        onOpenChange={vi.fn()}
        onRestoreSuccess={vi.fn()}
      />
    );

    expect(await screen.findByText("网络异常")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "重试" }));

    await waitFor(() => {
      expect(listRevisions).toHaveBeenCalledTimes(2);
    });
    expect(await screen.findByText("暂无历史版本")).toBeInTheDocument();
  });

  it("renders a readonly markdown diff against the current editor content", async () => {
    const listRevisions = vi.fn(async () => [
      createRevision({ id: "revision-1", version: 1, format: "markdown" })
    ]);
    const getRevisionDetail = vi.fn(async () => createRevisionDetail({
      id: "revision-1",
      version: 1,
      contentMd: "# 历史正文\n\n旧内容"
    }));

    render(
      <DocumentRevisionHistoryDialog
        open
        documentId="doc-1"
        documentTitle="产品说明"
        currentContent={"# 当前正文\n\n新内容"}
        currentDocumentVersion={6}
        hasUnsavedChanges={false}
        dataGateway={createGateway({ listRevisions, getRevisionDetail })}
        onOpenChange={vi.fn()}
        onRestoreSuccess={vi.fn()}
      />
    );

    const diffRegion = await screen.findByRole("region", { name: "Markdown 差异视图" });
    expect(diffRegion).toBeInTheDocument();
    expect(within(diffRegion).getByText("历史版本")).toBeInTheDocument();
    expect(within(diffRegion).getByText("当前内容")).toBeInTheDocument();
    expect(getRevisionDetail).toHaveBeenCalledWith("doc-1", "revision-1");
    expect(mergeViewMock.MergeView).toHaveBeenCalledTimes(1);
    expect((mergeViewMock.instances[0].config.a as { doc: string }).doc).toBe("# 历史正文\n\n旧内容");
    expect((mergeViewMock.instances[0].config.b as { doc: string }).doc).toBe("# 当前正文\n\n新内容");
  });

  it("ignores stale detail requests when switching selected revisions", async () => {
    const user = userEvent.setup();
    let resolveFirstDetail: (detail: DocumentRevisionDetail) => void = () => undefined;
    const firstDetailPromise = new Promise<DocumentRevisionDetail>((resolve) => {
      resolveFirstDetail = resolve;
    });
    const listRevisions = vi.fn(async () => [
      createRevision({ id: "revision-2", version: 2, format: "markdown" }),
      createRevision({ id: "revision-1", version: 1, format: "markdown" })
    ]);
    const getRevisionDetail = vi.fn((_: string, revisionId: string) => {
      if (revisionId === "revision-2") {
        return firstDetailPromise;
      }
      return Promise.resolve(createRevisionDetail({
        id: "revision-1",
        version: 1,
        contentMd: "# v1 历史正文"
      }));
    });

    render(
      <DocumentRevisionHistoryDialog
        open
        documentId="doc-1"
        documentTitle="产品说明"
        currentContent="# 当前正文"
        currentDocumentVersion={6}
        hasUnsavedChanges={false}
        dataGateway={createGateway({ listRevisions, getRevisionDetail })}
        onOpenChange={vi.fn()}
        onRestoreSuccess={vi.fn()}
      />
    );

    await screen.findByRole("button", { name: "版本 2，作者甲" });
    await user.click(screen.getByRole("button", { name: "版本 1，作者甲" }));
    expect(await screen.findByRole("region", { name: "Markdown 差异视图" })).toBeInTheDocument();

    resolveFirstDetail(createRevisionDetail({
      id: "revision-2",
      version: 2,
      contentMd: "# v2 过期正文"
    }));
    await waitFor(() => {
      const latestMergeView = mergeViewMock.instances[mergeViewMock.instances.length - 1];
      expect((latestMergeView.config.a as { doc: string }).doc).toBe("# v1 历史正文");
    });
  });

  it("keeps diff visible but disables restore when the current document has unsaved changes", async () => {
    const listRevisions = vi.fn(async () => [
      createRevision({ id: "revision-1", version: 1, format: "markdown" })
    ]);
    const getRevisionDetail = vi.fn(async () => createRevisionDetail({
      id: "revision-1",
      version: 1,
      contentMd: "# 历史正文"
    }));

    render(
      <DocumentRevisionHistoryDialog
        open
        documentId="doc-1"
        documentTitle="产品说明"
        currentContent="# 未保存的当前正文"
        currentDocumentVersion={6}
        hasUnsavedChanges
        dataGateway={createGateway({ listRevisions, getRevisionDetail })}
        onOpenChange={vi.fn()}
        onRestoreSuccess={vi.fn()}
      />
    );

    expect(await screen.findByRole("region", { name: "Markdown 差异视图" })).toBeInTheDocument();
    expect(screen.getByText("存在未保存修改，恢复前请先保存或放弃当前编辑。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "恢复到此版本" })).toBeDisabled();
    expect((mergeViewMock.instances[0].config.b as { doc: string }).doc).toBe("# 未保存的当前正文");
  });

  it("renders office revision metadata without attempting a text diff", async () => {
    const listRevisions = vi.fn(async () => [
      createRevision({
        id: "revision-office-1",
        version: 7,
        format: "docx",
        fileName: "方案说明.docx",
        mimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
      })
    ]);
    const getRevisionDetail = vi.fn(async () => createRevisionDetail({
      id: "revision-office-1",
      version: 7,
      format: "docx",
      contentMd: undefined,
      fileName: "方案说明.docx",
      mimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
      file: {
        fileName: "方案说明.docx",
        mimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
        blobId: "blob-office-1"
      }
    }));

    render(
      <DocumentRevisionHistoryDialog
        open
        documentId="doc-1"
        documentTitle="产品说明"
        currentContent="# 当前正文"
        currentDocumentVersion={6}
        hasUnsavedChanges={false}
        dataGateway={createGateway({ listRevisions, getRevisionDetail })}
        onOpenChange={vi.fn()}
        onRestoreSuccess={vi.fn()}
      />
    );

    const metadataRegion = await screen.findByRole("region", { name: "Office 历史版本元数据" });
    expect(metadataRegion).toBeInTheDocument();
    expect(within(metadataRegion).getByText("方案说明.docx")).toBeInTheDocument();
    expect(
      within(metadataRegion).getByText("application/vnd.openxmlformats-officedocument.wordprocessingml.document")
    ).toBeInTheDocument();
    expect(within(metadataRegion).getByText("v7")).toBeInTheDocument();
    expect(within(metadataRegion).getByText("作者甲")).toBeInTheDocument();
    expect(within(metadataRegion).getByText("Office 文档暂不支持二进制差异预览。")).toBeInTheDocument();
    expect(
      screen.getByText("恢复前会要求二次确认，确认后会切换当前 Office 源文件版本。")
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "恢复到此版本" })).toBeEnabled();
    expect(getRevisionDetail).toHaveBeenCalledWith("doc-1", "revision-office-1");
    expect(mergeViewMock.MergeView).not.toHaveBeenCalled();
  });

  it("confirms and restores a markdown revision with the current document version", async () => {
    const user = userEvent.setup();
    const restoredResult: RestoreDocumentRevisionResult = {
      document: createDocument({ version: 8, contentMd: "# 恢复后的正文" }),
      restoredFromRevision: createRevision({ id: "revision-1", version: 1, format: "markdown" })
    };
    const listRevisions = vi
      .fn<DataGateway["document"]["listRevisions"]>()
      .mockResolvedValueOnce([
        createRevision({ id: "revision-1", version: 1, format: "markdown" })
      ])
      .mockResolvedValueOnce([
        createRevision({ id: "revision-8", version: 8, format: "markdown" }),
        createRevision({ id: "revision-1", version: 1, format: "markdown" })
      ]);
    const getRevisionDetail = vi.fn(async () => createRevisionDetail({
      id: "revision-1",
      version: 1,
      contentMd: "# 历史正文"
    }));
    const restoreRevision = vi.fn(async () => restoredResult);
    const onRestoreSuccess = vi.fn();

    render(
      <DocumentRevisionHistoryDialog
        open
        documentId="doc-1"
        documentTitle="产品说明"
        currentContent="# 当前正文"
        currentDocumentVersion={7}
        hasUnsavedChanges={false}
        dataGateway={createGateway({ listRevisions, getRevisionDetail, restoreRevision })}
        onOpenChange={vi.fn()}
        onRestoreSuccess={onRestoreSuccess}
      />
    );

    await screen.findByRole("region", { name: "Markdown 差异视图" });
    await user.click(screen.getByRole("button", { name: "恢复到此版本" }));

    const confirmDialog = screen.getByRole("alertdialog", { name: "确认恢复历史版本" });
    expect(confirmDialog).toBeInTheDocument();
    expect(within(confirmDialog).getByText(/即将恢复到 v1/)).toBeInTheDocument();
    expect(within(confirmDialog).getByText(/作者甲/)).toBeInTheDocument();
    expect(screen.getAllByText("2026/05/17 08:00").length).toBeGreaterThan(0);

    await user.click(screen.getByRole("button", { name: "确认恢复" }));

    await waitFor(() => {
      expect(restoreRevision).toHaveBeenCalledWith({
        docId: "doc-1",
        revisionId: "revision-1",
        baseVersion: 7
      });
    });
    expect(onRestoreSuccess).toHaveBeenCalledWith(restoredResult);
    expect(listRevisions).toHaveBeenCalledTimes(2);
    expect(await screen.findByText("已恢复到 v1，当前文档版本 v8。")).toBeInTheDocument();
  });

  it("keeps the dialog open and explains revision conflict failures", async () => {
    const user = userEvent.setup();
    const listRevisions = vi.fn(async () => [
      createRevision({ id: "revision-1", version: 1, format: "markdown" })
    ]);
    const getRevisionDetail = vi.fn(async () => createRevisionDetail({
      id: "revision-1",
      version: 1,
      contentMd: "# 历史正文"
    }));
    const restoreRevision = vi.fn(async () => {
      throw new ConflictError(createDocument({ version: 9 }));
    });

    render(
      <DocumentRevisionHistoryDialog
        open
        documentId="doc-1"
        documentTitle="产品说明"
        currentContent="# 当前正文"
        currentDocumentVersion={7}
        hasUnsavedChanges={false}
        dataGateway={createGateway({ listRevisions, getRevisionDetail, restoreRevision })}
        onOpenChange={vi.fn()}
        onRestoreSuccess={vi.fn()}
      />
    );

    await screen.findByRole("region", { name: "Markdown 差异视图" });
    await user.click(screen.getByRole("button", { name: "恢复到此版本" }));
    await user.click(screen.getByRole("button", { name: "确认恢复" }));

    expect(await screen.findByText(/版本冲突/)).toBeInTheDocument();
    expect(screen.getByText(/请刷新或重新选择历史版本后再试/)).toBeInTheDocument();
    expect(screen.getByRole("dialog", { name: "历史版本" })).toBeInTheDocument();
  });
});
