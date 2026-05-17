import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { useState } from "react";
import type { AdminSpaceTransferSubscribeInput, DataGateway } from "../../data-access";
import { AdminSpaceTransferTaskProvider } from "./AdminSpaceTransferTaskProvider";
import { useAdminSpaceTransferTasks } from "./useAdminSpaceTransferTasks";

vi.mock("../components/spaceCoverDefault", () => ({
  exportSystemGeneratedWebP: vi.fn(async () => ({
    file: new File(["cover"], "cover.webp", { type: "image/webp" }),
    width: 800,
    height: 1200
  }))
}));

function TrackImportTaskButton({
  onCompleted
}: {
  onCompleted(spaceID: string): Promise<void> | void;
}) {
  const { trackImportTask } = useAdminSpaceTransferTasks();
  return (
    <button
      type="button"
      onClick={() =>
        trackImportTask({
          jobId: "import-job",
          streamUrl: "/api/admin/space-imports/import-job/events?token=stream",
          importId: "import-a",
          spaceName: "导入空间",
          needsDefaultCover: true,
          onCompleted
        })
      }
    >
      创建导入任务
    </button>
  );
}

function PageSwitchHarness() {
  const [page, setPage] = useState<"spaces" | "profile">("spaces");
  const { trackExportTask } = useAdminSpaceTransferTasks();
  return (
    <div>
      {page === "spaces" ? (
        <button
          type="button"
          onClick={() =>
            trackExportTask({
              jobId: "export-job",
              streamUrl: "/api/admin/space-exports/export-job/events?token=stream",
              spaceId: "space-a",
              spaceName: "切换空间",
              format: "source_zip"
            })
          }
        >
          创建导出任务
        </button>
      ) : (
        <div>个人信息页</div>
      )}
      <button type="button" onClick={() => setPage("profile")}>
        切换页面
      </button>
    </div>
  );
}

describe("AdminSpaceTransferTaskProvider", () => {
  it("recovers active tasks, subscribes, and renders the floating task panel", async () => {
    let subscriptionInput: AdminSpaceTransferSubscribeInput | null = null;
    const listSpaceTransferTasks = vi.fn().mockResolvedValue({
      tasks: [
        {
          jobId: "job-a",
          kind: "space_export",
          status: "running",
          stage: "documents",
          progress: 35,
          message: "正在导出文档",
          spaceName: "知识库",
          format: "source_zip",
          createdAt: "2026-05-17T00:00:00Z",
          updatedAt: "2026-05-17T00:01:00Z",
          expiresAt: "2026-05-17T00:11:00Z"
        }
      ]
    });
    const issueSpaceTransferStreamToken = vi.fn().mockResolvedValue({
      streamUrl: "/api/admin/spaces/space-a/exports/job-a/events?token=stream"
    });
    const issueSpaceTransferDownloadToken = vi.fn().mockResolvedValue({
      downloadUrl: "/api/admin/space-exports/job-a/download?token=fresh"
    });
    const subscribeSpaceExport = vi.fn((input: AdminSpaceTransferSubscribeInput) => {
      subscriptionInput = input;
      return { close: vi.fn() };
    });
    const dataGateway = {
      admin: {
        listSpaceTransferTasks,
        issueSpaceTransferStreamToken,
        issueSpaceTransferDownloadToken,
        subscribeSpaceExport
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    render(
      <AdminSpaceTransferTaskProvider dataGateway={dataGateway}>
        <div>后台页面</div>
      </AdminSpaceTransferTaskProvider>
    );

    expect(await screen.findByText("导入导出任务")).toBeInTheDocument();
    expect(screen.getByText("剩余1个")).toBeInTheDocument();
    await waitFor(() => expect(subscribeSpaceExport).toHaveBeenCalledTimes(1));

    await user.click(screen.getByRole("button", { name: "展开任务中心" }));
    expect(screen.getByText("知识库")).toBeInTheDocument();
    expect(screen.getByText("35%")).toBeInTheDocument();

    act(() => {
      subscriptionInput?.onEvent({
        type: "completed",
        progress: 100,
        message: "导出完成",
        downloadUrl: "/api/admin/space-exports/job-a/download?token=done",
        fileName: "知识库.plaindoc"
      });
    });

    expect(await screen.findByText("已完成")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "下载文件" }));
    expect(issueSpaceTransferDownloadToken).toHaveBeenCalledWith({
      kind: "space_export",
      jobId: "job-a"
    });
  });

  it("recovers terminal tasks without resubscribing stream tokens", async () => {
    const listSpaceTransferTasks = vi.fn().mockResolvedValue({
      tasks: [
        {
          jobId: "job-done",
          kind: "space_export",
          status: "completed",
          stage: "done",
          progress: 100,
          message: "导出完成",
          spaceName: "已完成空间",
          fileName: "已完成空间.plaindoc",
          sizeBytes: 2048,
          createdAt: "2026-05-17T00:00:00Z",
          updatedAt: "2026-05-17T00:01:00Z",
          expiresAt: "2026-05-17T00:11:00Z"
        },
        {
          jobId: "job-failed",
          kind: "space_import",
          status: "failed",
          stage: "restore",
          progress: 25,
          message: "导入失败",
          errorMessage: "导入失败",
          spaceName: "失败空间",
          createdAt: "2026-05-17T00:00:00Z",
          updatedAt: "2026-05-17T00:01:00Z",
          expiresAt: "2026-05-17T00:11:00Z"
        }
      ]
    });
    const issueSpaceTransferStreamToken = vi.fn();
    const issueSpaceTransferDownloadToken = vi.fn().mockResolvedValue({
      downloadUrl: "/api/admin/space-exports/job-done/download?token=fresh"
    });
    const dataGateway = {
      admin: {
        listSpaceTransferTasks,
        issueSpaceTransferStreamToken,
        issueSpaceTransferDownloadToken,
        subscribeSpaceExport: vi.fn(),
        subscribeSpaceImport: vi.fn()
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    render(
      <AdminSpaceTransferTaskProvider dataGateway={dataGateway}>
        <div>后台页面</div>
      </AdminSpaceTransferTaskProvider>
    );

    expect(await screen.findByText("导入导出任务")).toBeInTheDocument();
    expect(listSpaceTransferTasks).toHaveBeenCalledWith({ limit: 12 });
    expect(issueSpaceTransferStreamToken).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "展开任务中心" }));
    expect(screen.getByText("已完成空间")).toBeInTheDocument();
    expect(screen.getByText("失败空间")).toBeInTheDocument();
    expect(screen.getAllByText("已完成").length).toBeGreaterThan(0);
    expect(screen.getAllByText("失败").length).toBeGreaterThan(0);
    expect(screen.queryByText("completed")).toBeNull();
    expect(screen.queryByText("failed")).toBeNull();

    await user.click(screen.getByRole("button", { name: "下载文件" }));
    expect(issueSpaceTransferDownloadToken).toHaveBeenCalledWith({
      kind: "space_export",
      jobId: "job-done"
    });
  });

  it("keeps a cleared terminal task hidden after the provider recovers tasks again", async () => {
    window.localStorage.clear();
    const listSpaceTransferTasks = vi.fn().mockResolvedValue({
      tasks: [
        {
          jobId: "job-cleared",
          kind: "space_export",
          status: "completed",
          stage: "done",
          progress: 100,
          message: "导出完成",
          spaceName: "已清除空间",
          fileName: "已清除空间.plaindoc",
          createdAt: "2026-05-17T00:00:00Z",
          updatedAt: "2026-05-17T00:01:00Z",
          expiresAt: "2026-05-17T00:11:00Z"
        }
      ]
    });
    const dataGateway = {
      admin: {
        listSpaceTransferTasks,
        issueSpaceTransferStreamToken: vi.fn(),
        issueSpaceTransferDownloadToken: vi.fn(),
        subscribeSpaceExport: vi.fn(),
        subscribeSpaceImport: vi.fn()
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    const firstRender = render(
      <AdminSpaceTransferTaskProvider dataGateway={dataGateway}>
        <div>后台页面</div>
      </AdminSpaceTransferTaskProvider>
    );

    expect(await screen.findByText("导入导出任务")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "展开任务中心" }));
    expect(screen.getByText("已清除空间")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "清除" }));
    expect(screen.queryByText("已清除空间")).toBeNull();
    firstRender.unmount();

    render(
      <AdminSpaceTransferTaskProvider dataGateway={dataGateway}>
        <div>后台页面</div>
      </AdminSpaceTransferTaskProvider>
    );

    await waitFor(() => expect(listSpaceTransferTasks).toHaveBeenCalledTimes(2));
    expect(screen.queryByText("导入导出任务")).toBeNull();
    expect(screen.queryByText("已清除空间")).toBeNull();
  });

  it("minimizes the expanded transfer task panel", async () => {
    const dataGateway = {
      admin: {
        listSpaceTransferTasks: vi.fn().mockResolvedValue({
          tasks: [
            {
              jobId: "job-done",
              kind: "space_export",
              status: "completed",
              stage: "done",
              progress: 100,
              message: "导出完成",
              spaceName: "最小化空间",
              fileName: "最小化空间.plaindoc",
              createdAt: "2026-05-17T00:00:00Z",
              updatedAt: "2026-05-17T00:01:00Z",
              expiresAt: "2026-05-17T00:11:00Z"
            }
          ]
        }),
        issueSpaceTransferStreamToken: vi.fn(),
        issueSpaceTransferDownloadToken: vi.fn().mockResolvedValue({
          downloadUrl: "/api/admin/space-exports/job-done/download?token=fresh"
        }),
        subscribeSpaceExport: vi.fn(),
        subscribeSpaceImport: vi.fn()
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    render(
      <AdminSpaceTransferTaskProvider dataGateway={dataGateway}>
        <div>后台页面</div>
      </AdminSpaceTransferTaskProvider>
    );

    expect(await screen.findByText("导入导出任务")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "展开任务中心" }));
    expect(screen.getByText("最小化空间")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "最小化任务中心" }));
    expect(screen.queryByText("最小化空间")).toBeNull();
    expect(screen.getByRole("button", { name: "展开任务中心" })).toBeInTheDocument();
  });

  it("attaches a generated default cover after a tracked import task completes", async () => {
    let subscriptionInput: AdminSpaceTransferSubscribeInput | null = null;
    const onCompleted = vi.fn();
    const createSpaceCoverAsset = vi.fn().mockResolvedValue({
      assetId: "cover-asset"
    });
    const updateSpaceMetadata = vi.fn().mockResolvedValue(undefined);
    const subscribeSpaceImport = vi.fn((input: AdminSpaceTransferSubscribeInput) => {
      subscriptionInput = input;
      return { close: vi.fn() };
    });
    const dataGateway = {
      admin: {
        listSpaceTransferTasks: vi.fn().mockResolvedValue({ tasks: [] }),
        issueSpaceTransferStreamToken: vi.fn(),
        issueSpaceTransferDownloadToken: vi.fn(),
        subscribeSpaceImport,
        createSpaceCoverAsset,
        updateSpaceMetadata
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    render(
      <AdminSpaceTransferTaskProvider dataGateway={dataGateway}>
        <TrackImportTaskButton onCompleted={onCompleted} />
      </AdminSpaceTransferTaskProvider>
    );

    await user.click(screen.getByRole("button", { name: "创建导入任务" }));
    await user.click(await screen.findByRole("button", { name: "展开任务中心" }));
    expect(screen.getByText("导入空间")).toBeInTheDocument();

    act(() => {
      subscriptionInput?.onEvent({
        type: "completed",
        stage: "done",
        progress: 100,
        message: "导入完成",
        spaceId: "new-space",
        spaceName: "导入空间"
      });
    });

    await waitFor(() => expect(createSpaceCoverAsset).toHaveBeenCalledTimes(1));
    expect(updateSpaceMetadata).toHaveBeenCalledWith({
      spaceId: "new-space",
      coverAssetId: "cover-asset"
    });
    expect(onCompleted).toHaveBeenCalledWith("new-space");
    expect(await screen.findByRole("button", { name: "打开编辑器" })).toBeInTheDocument();
  });

  it("recovers import completion cover fallback after page refresh", async () => {
    let subscriptionInput: AdminSpaceTransferSubscribeInput | null = null;
    const listSpaceTransferTasks = vi.fn().mockResolvedValue({
      tasks: [
        {
          jobId: "import-recovered",
          kind: "space_import",
          status: "running",
          stage: "documents",
          progress: 45,
          message: "正在导入",
          spaceName: "刷新恢复空间",
          importId: "import-recovered-id",
          createdAt: "2026-05-17T00:00:00Z",
          updatedAt: "2026-05-17T00:01:00Z",
          expiresAt: "2026-05-17T00:11:00Z"
        }
      ]
    });
    const issueSpaceTransferStreamToken = vi.fn().mockResolvedValue({
      streamUrl: "/api/admin/space-imports/import-recovered/events?token=stream"
    });
    const subscribeSpaceImport = vi.fn((input: AdminSpaceTransferSubscribeInput) => {
      subscriptionInput = input;
      return { close: vi.fn() };
    });
    const listSpaces = vi.fn().mockResolvedValue({
      items: [
        {
          spaceId: "new-space",
          name: "刷新恢复空间",
          description: "",
          visibility: "member",
          status: "active",
          ownerUserId: "actor-user",
          ownerName: "Actor",
          ownerEmail: "actor@example.com",
          categoryId: "",
          categoryName: "",
          cover: null,
          memberCount: 1,
          documentCount: 0,
          createdAt: "2026-05-17T00:00:00Z",
          updatedAt: "2026-05-17T00:00:00Z",
          isOwner: true
        }
      ],
      pagination: { page: 1, pageSize: 1, total: 1 }
    });
    const createSpaceCoverAsset = vi.fn().mockResolvedValue({
      assetId: "cover-asset"
    });
    const updateSpaceMetadata = vi.fn().mockResolvedValue(undefined);
    const importCompletedListener = vi.fn();
    window.addEventListener("plaindoc:admin-space-import-completed", importCompletedListener);

    const dataGateway = {
      admin: {
        listSpaceTransferTasks,
        issueSpaceTransferStreamToken,
        issueSpaceTransferDownloadToken: vi.fn(),
        subscribeSpaceImport,
        listSpaces,
        createSpaceCoverAsset,
        updateSpaceMetadata
      }
    } as unknown as DataGateway;

    try {
      render(
        <AdminSpaceTransferTaskProvider dataGateway={dataGateway}>
          <div>后台页面</div>
        </AdminSpaceTransferTaskProvider>
      );

      await waitFor(() => expect(subscribeSpaceImport).toHaveBeenCalledTimes(1));

      act(() => {
        subscriptionInput?.onEvent({
          type: "completed",
          stage: "done",
          progress: 100,
          message: "导入完成",
          spaceId: "new-space",
          spaceName: "刷新恢复空间"
        });
      });

      await waitFor(() => expect(createSpaceCoverAsset).toHaveBeenCalledTimes(1));
      expect(listSpaces).toHaveBeenCalledWith({ keyword: "new-space", page: 1, pageSize: 1 });
      expect(updateSpaceMetadata).toHaveBeenCalledWith({
        spaceId: "new-space",
        coverAssetId: "cover-asset"
      });
      await waitFor(() => expect(importCompletedListener).toHaveBeenCalledTimes(1));
    } finally {
      window.removeEventListener("plaindoc:admin-space-import-completed", importCompletedListener);
    }
  });

  it("keeps a tracked task subscription after switching admin pages", async () => {
    let subscriptionInput: AdminSpaceTransferSubscribeInput | null = null;
    const closeExport = vi.fn();
    const subscribeSpaceExport = vi.fn((input: AdminSpaceTransferSubscribeInput) => {
      subscriptionInput = input;
      return { close: closeExport };
    });
    const dataGateway = {
      admin: {
        listSpaceTransferTasks: vi.fn().mockResolvedValue({ tasks: [] }),
        issueSpaceTransferStreamToken: vi.fn(),
        issueSpaceTransferDownloadToken: vi.fn(),
        subscribeSpaceExport
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    render(
      <AdminSpaceTransferTaskProvider dataGateway={dataGateway}>
        <PageSwitchHarness />
      </AdminSpaceTransferTaskProvider>
    );

    await user.click(screen.getByRole("button", { name: "创建导出任务" }));
    await waitFor(() => expect(subscribeSpaceExport).toHaveBeenCalledTimes(1));
    await user.click(screen.getByRole("button", { name: "切换页面" }));
    expect(await screen.findByText("个人信息页")).toBeInTheDocument();

    act(() => {
      subscriptionInput?.onEvent({
        type: "progress",
        stage: "documents",
        progress: 55,
        message: "切换后仍在导出"
      });
    });

    await user.click(screen.getByRole("button", { name: "展开任务中心" }));
    expect(screen.getByText("切换空间")).toBeInTheDocument();
    expect(screen.getByText("55%")).toBeInTheDocument();
    expect(closeExport).not.toHaveBeenCalled();
  });
});
