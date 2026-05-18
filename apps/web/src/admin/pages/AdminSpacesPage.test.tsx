import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { AdminSpace, AdminSpaceTransferSubscribeInput, DataGateway } from "../../data-access";
import { AdminSpaceTransferTaskProvider } from "../space-transfer/AdminSpaceTransferTaskProvider";
import { AdminSpacesPage } from "./AdminSpacesPage";

const { confirmMock, promptMock, showToastMock } = vi.hoisted(() => ({
  confirmMock: vi.fn(),
  promptMock: vi.fn(),
  showToastMock: vi.fn()
}));

vi.mock("../components/AdminDialogs", () => {
  return {
    useAdminDialogs: () => ({
      confirm: confirmMock,
      prompt: promptMock,
      dialogs: null
    })
  };
});

vi.mock("../../components/ui/toast", () => {
  return {
    showToast: showToastMock
  };
});

function createAdminSpace(overrides: Partial<AdminSpace> = {}): AdminSpace {
  return {
    spaceId: "space-admin",
    name: "管理空间",
    description: "管理员视图空间",
    categoryId: "",
    category: "",
    categoryIsDefault: false,
    ownerUserId: "owner-1",
    ownerName: "空间拥有者",
    ownerEmail: "owner@example.com",
    visibility: "member",
    cover: null,
    status: "active",
    bannedReason: "",
    bannedAt: null,
    deletedAt: null,
    createdAt: "2026-04-05T00:00:00Z",
    updatedAt: "2026-04-05T00:00:00Z",
    ...overrides
  };
}

function renderAdminSpacesPage(dataGateway: DataGateway, mode: "admin" | "member") {
  return render(
    <AdminSpaceTransferTaskProvider dataGateway={dataGateway}>
      <AdminSpacesPage dataGateway={dataGateway} mode={mode} />
    </AdminSpaceTransferTaskProvider>
  );
}

describe("AdminSpacesPage", () => {
  beforeEach(() => {
    window.localStorage.clear();
    confirmMock.mockReset();
    promptMock.mockReset();
    showToastMock.mockReset();
  });

  afterEach(() => {
    cleanup();
    document.body.style.overflow = "";
    vi.restoreAllMocks();
  });

  it("shows only the edit document action in member mode", async () => {
    const listSpaces = vi.fn().mockResolvedValue({
      items: [
        createAdminSpace({
          spaceId: "space-member",
          name: "成员空间",
          description: "成员视图空间"
        })
      ],
      pagination: {
        page: 1,
        pageSize: 20,
        total: 1
      }
    });

    const dataGateway = {
      admin: {
        listSpaces
      }
    } as unknown as DataGateway;

    renderAdminSpacesPage(dataGateway, "member");

    const rowTitle = await screen.findByText("成员空间");
    const row = rowTitle.closest("tr");
    if (!row) {
      throw new Error("member row not found");
    }

    expect(within(row).getByRole("button", { name: "编辑文档" })).toBeInTheDocument();
    expect(within(row).queryByRole("button", { name: "设置" })).toBeNull();
    expect(within(row).queryByText("成员管理")).toBeNull();
    expect(within(row).queryByText("删除空间")).toBeNull();
    expect(screen.queryByText("批量封禁")).toBeNull();
    expect(screen.queryByRole("checkbox", { name: "全选空间" })).toBeNull();
  });

  it("keeps admin actions in the settings menu for admin mode", async () => {
    const listSpaces = vi.fn().mockResolvedValue({
      items: [
        createAdminSpace()
      ],
      pagination: {
        page: 1,
        pageSize: 20,
        total: 1
      }
    });
    const listSpaceCategories = vi.fn().mockResolvedValue([]);
    const listSystemConfigs = vi.fn().mockResolvedValue([
      {
        configKey: "site",
        value: {
          defaultSpaceVisibility: "member"
        }
      }
    ]);

    const dataGateway = {
      admin: {
        listSpaces,
        listSpaceCategories,
        listSystemConfigs
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    renderAdminSpacesPage(dataGateway, "admin");

    const rowTitle = await screen.findByText("管理空间");
    const row = rowTitle.closest("tr");
    if (!row) {
      throw new Error("admin row not found");
    }

    expect(within(row).queryByRole("button", { name: "编辑文档" })).toBeNull();
    const settingsButton = within(row).getByRole("button", { name: "展开更多操作" });
    await user.click(settingsButton);

    expect(screen.getByText("编辑文档")).toBeInTheDocument();
    expect(screen.getByText("空间设置")).toBeInTheDocument();
    expect(screen.getByText("导出空间")).toBeInTheDocument();
    expect(screen.getByText("成员管理")).toBeInTheDocument();
    expect(screen.getByText("删除空间")).toBeInTheDocument();
  });

  it("explains the selected export format in the side panel", async () => {
    const listSpaces = vi.fn().mockResolvedValue({
      items: [createAdminSpace()],
      pagination: {
        page: 1,
        pageSize: 20,
        total: 1
      }
    });
    const listSpaceCategories = vi.fn().mockResolvedValue([]);
    const listSystemConfigs = vi.fn().mockResolvedValue([
      {
        configKey: "site",
        value: {
          defaultSpaceVisibility: "member"
        }
      }
    ]);

    const dataGateway = {
      admin: {
        listSpaces,
        listSpaceCategories,
        listSystemConfigs
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    renderAdminSpacesPage(dataGateway, "admin");

    const rowTitle = await screen.findByText("管理空间");
    const row = rowTitle.closest("tr");
    if (!row) {
      throw new Error("admin row not found");
    }

    await user.click(within(row).getByRole("button", { name: "展开更多操作" }));
    await user.click(screen.getByText("导出空间"));

    expect(screen.getByText("PlainDoc 包是可导入的完整空间交换包，用于迁移、备份和跨环境恢复。")).toBeInTheDocument();
	    expect(screen.getByText("目录树和空间元数据")).toBeInTheDocument();
    expect(screen.getByText("空间封面")).toBeInTheDocument();
	    expect(screen.getByText("后续还要通过“导入空间”恢复为新空间。")).toBeInTheDocument();

    await user.selectOptions(screen.getByDisplayValue("PlainDoc 包"), "markdown_zip");
    expect(screen.getByText("Markdown ZIP 是内容归档包，用于离线查看或交给外部系统处理。")).toBeInTheDocument();
    expect(screen.getByText("Office 文档不会转换成 Markdown，也不保证可完整导回。")).toBeInTheDocument();

    await user.selectOptions(screen.getByDisplayValue("Markdown ZIP"), "epub");
    expect(screen.getByText("EPUB 是电子书阅读包，用于离线阅读和分发。")).toBeInTheDocument();
    expect(screen.getByText("SSR 渲染后的 Markdown")).toBeInTheDocument();
    expect(screen.getByText("EPUB 是阅读产物，不能作为空间导入包。")).toBeInTheDocument();
  });

  it("forces PlainDoc package exports to keep importable options", async () => {
    const listSpaces = vi.fn().mockResolvedValue({
      items: [createAdminSpace()],
      pagination: {
        page: 1,
        pageSize: 20,
        total: 1
      }
    });
    const listSpaceCategories = vi.fn().mockResolvedValue([]);
    const listSystemConfigs = vi.fn().mockResolvedValue([
      {
        configKey: "site",
        value: {
          defaultSpaceVisibility: "member"
        }
      }
    ]);
    const startSpaceExport = vi.fn().mockResolvedValue({
      jobId: "export-job",
      streamUrl: "/api/admin/space-exports/export-job/events"
    });
    const subscribeSpaceExport = vi.fn(() => ({ close: vi.fn() }));

    const dataGateway = {
      admin: {
        listSpaces,
        listSpaceCategories,
        listSystemConfigs,
        startSpaceExport,
        subscribeSpaceExport
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    renderAdminSpacesPage(dataGateway, "admin");

    const rowTitle = await screen.findByText("管理空间");
    const row = rowTitle.closest("tr");
    if (!row) {
      throw new Error("admin row not found");
    }

    await user.click(within(row).getByRole("button", { name: "展开更多操作" }));
    await user.click(screen.getByText("导出空间"));

    expect(screen.getByRole("checkbox", { name: "包含附件" })).toBeDisabled();
    expect(screen.getByRole("checkbox", { name: "包含 Office 源文件" })).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "开始导出" }));

    await waitFor(() => expect(startSpaceExport).toHaveBeenCalledWith({
      spaceId: "space-admin",
      format: "source_zip",
      includeAttachments: true,
      includeOfficeSources: true
    }));
  });

  it("keeps a completed export download manual for the single-use token", async () => {
    let exportSubscriptionInput: AdminSpaceTransferSubscribeInput | null = null;
    const closeExport = vi.fn();
    const anchorClick = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => undefined);
    const listSpaces = vi.fn().mockResolvedValue({
      items: [createAdminSpace()],
      pagination: {
        page: 1,
        pageSize: 20,
        total: 1
      }
    });
    const listSpaceCategories = vi.fn().mockResolvedValue([]);
    const listSystemConfigs = vi.fn().mockResolvedValue([
      {
        configKey: "site",
        value: {
          defaultSpaceVisibility: "member"
        }
      }
    ]);
    const startSpaceExport = vi.fn().mockResolvedValue({
      jobId: "export-job",
      streamUrl: "/api/admin/space-exports/export-job/events"
    });
    const subscribeSpaceExport = vi.fn((input: AdminSpaceTransferSubscribeInput) => {
      exportSubscriptionInput = input;
      return { close: closeExport };
    });
    const issueSpaceTransferDownloadToken = vi.fn().mockResolvedValue({
      downloadUrl: "/api/admin/space-exports/export-job/download?token=two"
    });

    const dataGateway = {
      admin: {
        listSpaces,
        listSpaceCategories,
        listSystemConfigs,
        startSpaceExport,
        subscribeSpaceExport,
        issueSpaceTransferDownloadToken
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    renderAdminSpacesPage(dataGateway, "admin");

    const rowTitle = await screen.findByText("管理空间");
    const row = rowTitle.closest("tr");
    if (!row) {
      throw new Error("admin row not found");
    }

    await user.click(within(row).getByRole("button", { name: "展开更多操作" }));
    await user.click(screen.getByText("导出空间"));
    await user.selectOptions(screen.getByDisplayValue("PlainDoc 包"), "epub");
    await user.click(screen.getByRole("button", { name: "开始导出" }));

    await waitFor(() => expect(startSpaceExport).toHaveBeenCalledWith({
      spaceId: "space-admin",
      format: "epub",
      includeAttachments: true,
      includeOfficeSources: true
    }));
    await waitFor(() => expect(subscribeSpaceExport).toHaveBeenCalledTimes(1));

    act(() => {
      exportSubscriptionInput?.onEvent({
        type: "completed",
        progress: 100,
        message: "导出完成",
        downloadUrl: "/api/admin/space-exports/export-job/download?token=one",
        fileName: "管理空间.zip"
      });
    });

    expect(anchorClick).not.toHaveBeenCalled();
    expect(closeExport).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("button", { name: "开始导出" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "展开任务中心" }));
    expect(screen.getByText("已完成")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "下载文件" }));
    await waitFor(() => {
      expect(issueSpaceTransferDownloadToken).toHaveBeenCalledWith({
        kind: "space_export",
        jobId: "export-job"
      });
    });
    await waitFor(() => expect(anchorClick).toHaveBeenCalledTimes(1));
    anchorClick.mockRestore();
  });

  it("refreshes the completed export download token before each manual download", async () => {
    const exportSubscriptionInputs: AdminSpaceTransferSubscribeInput[] = [];
    const clickedHrefs: string[] = [];
    const closeExports: Array<ReturnType<typeof vi.fn>> = [];
    const anchorClick = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (this: HTMLAnchorElement) {
      clickedHrefs.push(this.href);
    });
    const listSpaces = vi.fn().mockResolvedValue({
      items: [createAdminSpace()],
      pagination: {
        page: 1,
        pageSize: 20,
        total: 1
      }
    });
    const listSpaceCategories = vi.fn().mockResolvedValue([]);
    const listSystemConfigs = vi.fn().mockResolvedValue([
      {
        configKey: "site",
        value: {
          defaultSpaceVisibility: "member"
        }
      }
    ]);
    const startSpaceExport = vi.fn().mockResolvedValue({
      jobId: "export-job",
      streamUrl: "/api/admin/space-exports/export-job/events"
    });
    const subscribeSpaceExport = vi.fn((input: AdminSpaceTransferSubscribeInput) => {
      exportSubscriptionInputs.push(input);
      const closeExport = vi.fn();
      closeExports.push(closeExport);
      return { close: closeExport };
    });
    const issueSpaceTransferDownloadToken = vi.fn()
      .mockResolvedValueOnce({
        downloadUrl: "/api/admin/space-exports/export-job/download?token=two"
      })
      .mockResolvedValueOnce({
        downloadUrl: "/api/admin/space-exports/export-job/download?token=three"
      });

    const dataGateway = {
      admin: {
        listSpaces,
        listSpaceCategories,
        listSystemConfigs,
        startSpaceExport,
        subscribeSpaceExport,
        issueSpaceTransferDownloadToken
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    renderAdminSpacesPage(dataGateway, "admin");

    const rowTitle = await screen.findByText("管理空间");
    const row = rowTitle.closest("tr");
    if (!row) {
      throw new Error("admin row not found");
    }

    await user.click(within(row).getByRole("button", { name: "展开更多操作" }));
    await user.click(screen.getByText("导出空间"));
    await user.selectOptions(screen.getByDisplayValue("PlainDoc 包"), "epub");
    await user.click(screen.getByRole("button", { name: "开始导出" }));
    await waitFor(() => expect(subscribeSpaceExport).toHaveBeenCalledTimes(1));

    act(() => {
      exportSubscriptionInputs[0]?.onEvent({
        type: "completed",
        progress: 100,
        message: "导出完成",
        downloadUrl: "/api/admin/space-exports/export-job/download?token=one",
        fileName: "管理空间.epub"
      });
    });

    await user.click(screen.getByRole("button", { name: "展开任务中心" }));
    await user.click(screen.getByRole("button", { name: "下载文件" }));
    await waitFor(() => expect(clickedHrefs).toHaveLength(1));
    expect(clickedHrefs[0]).toContain("token=two");
    expect(closeExports[0]).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "下载文件" }));
    await waitFor(() => expect(clickedHrefs).toHaveLength(2));
    expect(clickedHrefs[1]).toContain("token=three");
    expect(issueSpaceTransferDownloadToken).toHaveBeenCalledTimes(2);
    anchorClick.mockRestore();
  });

  it("removes a completed export task from the floating panel", async () => {
    const exportSubscriptionInputs: AdminSpaceTransferSubscribeInput[] = [];
    const closeExports: Array<ReturnType<typeof vi.fn>> = [];
    const listSpaces = vi.fn().mockResolvedValue({
      items: [createAdminSpace()],
      pagination: {
        page: 1,
        pageSize: 20,
        total: 1
      }
    });
    const listSpaceCategories = vi.fn().mockResolvedValue([]);
    const listSystemConfigs = vi.fn().mockResolvedValue([
      {
        configKey: "site",
        value: {
          defaultSpaceVisibility: "member"
        }
      }
    ]);
    const startSpaceExport = vi.fn().mockResolvedValue({
      jobId: "export-job",
      streamUrl: "/api/admin/space-exports/export-job/events"
    });
    const subscribeSpaceExport = vi.fn((input: AdminSpaceTransferSubscribeInput) => {
      exportSubscriptionInputs.push(input);
      const closeExport = vi.fn();
      closeExports.push(closeExport);
      return { close: closeExport };
    });

    const dataGateway = {
      admin: {
        listSpaces,
        listSpaceCategories,
        listSystemConfigs,
        startSpaceExport,
        subscribeSpaceExport,
        issueSpaceTransferDownloadToken: vi.fn().mockResolvedValue({
          downloadUrl: "/api/admin/space-exports/export-job/download?token=two"
        })
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    renderAdminSpacesPage(dataGateway, "admin");

    const rowTitle = await screen.findByText("管理空间");
    const row = rowTitle.closest("tr");
    if (!row) {
      throw new Error("admin row not found");
    }

    await user.click(within(row).getByRole("button", { name: "展开更多操作" }));
    await user.click(screen.getByText("导出空间"));
    await user.click(screen.getByRole("button", { name: "开始导出" }));
    await waitFor(() => expect(subscribeSpaceExport).toHaveBeenCalledTimes(1));

    act(() => {
      exportSubscriptionInputs[0]?.onEvent({
        type: "completed",
        progress: 100,
        message: "导出完成",
        downloadUrl: "/api/admin/space-exports/export-job/download?token=one",
        fileName: "管理空间.plaindoc"
      });
    });

    await user.click(screen.getByRole("button", { name: "展开任务中心" }));
    expect(screen.getByText("已完成")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "清除" }));
    expect(screen.queryByText("已完成")).toBeNull();
    expect(closeExports[0]).toHaveBeenCalledTimes(1);
  });

  it("updates export progress and shows failed events", async () => {
    let exportSubscriptionInput: AdminSpaceTransferSubscribeInput | null = null;
    const closeExport = vi.fn();
    const listSpaces = vi.fn().mockResolvedValue({
      items: [createAdminSpace()],
      pagination: {
        page: 1,
        pageSize: 20,
        total: 1
      }
    });
    const listSpaceCategories = vi.fn().mockResolvedValue([]);
    const listSystemConfigs = vi.fn().mockResolvedValue([
      {
        configKey: "site",
        value: {
          defaultSpaceVisibility: "member"
        }
      }
    ]);
    const startSpaceExport = vi.fn().mockResolvedValue({
      jobId: "export-job",
      streamUrl: "/api/admin/space-exports/export-job/events"
    });
    const subscribeSpaceExport = vi.fn((input: AdminSpaceTransferSubscribeInput) => {
      exportSubscriptionInput = input;
      return { close: closeExport };
    });

    const dataGateway = {
      admin: {
        listSpaces,
        listSpaceCategories,
        listSystemConfigs,
        startSpaceExport,
        subscribeSpaceExport
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    renderAdminSpacesPage(dataGateway, "admin");

    const rowTitle = await screen.findByText("管理空间");
    const row = rowTitle.closest("tr");
    if (!row) {
      throw new Error("admin row not found");
    }

    await user.click(within(row).getByRole("button", { name: "展开更多操作" }));
    await user.click(screen.getByText("导出空间"));
    await user.click(screen.getByRole("button", { name: "开始导出" }));
    await waitFor(() => expect(subscribeSpaceExport).toHaveBeenCalledTimes(1));

    act(() => {
      exportSubscriptionInput?.onEvent({
        type: "progress",
        stage: "documents",
        progress: 45,
        message: "正在导出文档"
      });
    });
    await user.click(screen.getByRole("button", { name: "展开任务中心" }));
    expect(screen.getByText("45%")).toBeInTheDocument();

    act(() => {
      exportSubscriptionInput?.onEvent({
        type: "failed",
        stage: "zip",
        progress: 45,
        message: "导出失败"
      });
    });

    expect(await screen.findByText("失败")).toBeInTheDocument();
    expect(closeExport).toHaveBeenCalledTimes(1);
  });

  it("keeps the export event stream in the floating panel after the dialog closes", async () => {
    let exportSubscriptionInput: AdminSpaceTransferSubscribeInput | null = null;
    const closeExport = vi.fn();
    const listSpaces = vi.fn().mockResolvedValue({
      items: [createAdminSpace()],
      pagination: {
        page: 1,
        pageSize: 20,
        total: 1
      }
    });
    const listSpaceCategories = vi.fn().mockResolvedValue([]);
    const listSystemConfigs = vi.fn().mockResolvedValue([
      {
        configKey: "site",
        value: {
          defaultSpaceVisibility: "member"
        }
      }
    ]);
    const startSpaceExport = vi.fn().mockResolvedValue({
      jobId: "export-job",
      streamUrl: "/api/admin/space-exports/export-job/events"
    });
    const subscribeSpaceExport = vi.fn((input: AdminSpaceTransferSubscribeInput) => {
      exportSubscriptionInput = input;
      return { close: closeExport };
    });

    const dataGateway = {
      admin: {
        listSpaces,
        listSpaceCategories,
        listSystemConfigs,
        startSpaceExport,
        subscribeSpaceExport
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    renderAdminSpacesPage(dataGateway, "admin");

    const rowTitle = await screen.findByText("管理空间");
    const row = rowTitle.closest("tr");
    if (!row) {
      throw new Error("admin row not found");
    }

    await user.click(within(row).getByRole("button", { name: "展开更多操作" }));
    await user.click(screen.getByText("导出空间"));
    await user.click(screen.getByRole("button", { name: "开始导出" }));
    await waitFor(() => expect(subscribeSpaceExport).toHaveBeenCalledTimes(1));
    expect(screen.queryByRole("button", { name: "开始导出" })).toBeNull();

    act(() => {
      exportSubscriptionInput?.onEvent({
        type: "progress",
        stage: "documents",
        progress: 45,
        message: "正在导出文档"
      });
    });

    await user.click(screen.getByRole("button", { name: "展开任务中心" }));
    expect(closeExport).not.toHaveBeenCalled();
    expect(screen.getByText("45%")).toBeInTheDocument();
  });

  it("keeps an export task running when the event stream errors", async () => {
    let exportSubscriptionInput: AdminSpaceTransferSubscribeInput | null = null;
    const closeExport = vi.fn();
    const listSpaces = vi.fn().mockResolvedValue({
      items: [createAdminSpace()],
      pagination: {
        page: 1,
        pageSize: 20,
        total: 1
      }
    });
    const listSpaceCategories = vi.fn().mockResolvedValue([]);
    const listSystemConfigs = vi.fn().mockResolvedValue([
      {
        configKey: "site",
        value: {
          defaultSpaceVisibility: "member"
        }
      }
    ]);
    const startSpaceExport = vi.fn().mockResolvedValue({
      jobId: "export-job",
      streamUrl: "/api/admin/space-exports/export-job/events"
    });
    const subscribeSpaceExport = vi.fn((input: AdminSpaceTransferSubscribeInput) => {
      exportSubscriptionInput = input;
      return { close: closeExport };
    });

    const dataGateway = {
      admin: {
        listSpaces,
        listSpaceCategories,
        listSystemConfigs,
        startSpaceExport,
        subscribeSpaceExport
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    renderAdminSpacesPage(dataGateway, "admin");

    const rowTitle = await screen.findByText("管理空间");
    const row = rowTitle.closest("tr");
    if (!row) {
      throw new Error("admin row not found");
    }

    await user.click(within(row).getByRole("button", { name: "展开更多操作" }));
    await user.click(screen.getByText("导出空间"));
    await user.click(screen.getByRole("button", { name: "开始导出" }));
    await waitFor(() => expect(subscribeSpaceExport).toHaveBeenCalledTimes(1));

    act(() => {
      exportSubscriptionInput?.onEvent({
        type: "progress",
        stage: "documents",
        progress: 45,
        message: "正在导出文档"
      });
    });

    act(() => {
      exportSubscriptionInput?.onError?.(new Event("error"));
    });

    await user.click(screen.getByRole("button", { name: "展开任务中心" }));
    expect(closeExport).toHaveBeenCalledTimes(1);
    expect(screen.getByText("45%")).toBeInTheDocument();
    expect(screen.queryByText("失败")).toBeNull();
  });

  it("refreshes the list after import completion and exposes editor and reader shortcuts", async () => {
    let importSubscriptionInput: AdminSpaceTransferSubscribeInput | null = null;
    const closeImport = vi.fn();
    const openWindow = vi.spyOn(window, "open").mockImplementation(() => null);
    const listSpaces = vi
      .fn()
      .mockResolvedValueOnce({
        items: [],
        pagination: {
          page: 1,
          pageSize: 20,
          total: 0
        }
      })
      .mockResolvedValue({
        items: [
          createAdminSpace({
            spaceId: "imported-space",
            name: "导入完成空间"
          })
        ],
        pagination: {
          page: 1,
          pageSize: 20,
          total: 1
        }
      });
    const listSpaceCategories = vi.fn().mockResolvedValue([]);
    const listSystemConfigs = vi.fn().mockResolvedValue([
      {
        configKey: "site",
        value: {
          defaultSpaceVisibility: "member"
        }
      }
    ]);
    const inspectSpaceImport = vi.fn().mockResolvedValue({
      importId: "01spaceimportpreview",
      packageVersion: 1,
      packageType: "plaindoc-space",
      exportedAt: "2026-05-16T00:00:00Z",
      importable: true,
	      space: {
	        spaceId: "space-source",
	        name: "源空间",
	        visibility: "member",
	        hasCover: true
	      },
      summary: {
        folderCount: 2,
        documentCount: 3,
        attachmentCount: 4,
        officeSourceCount: 1
      },
      warnings: []
    });
    const commitSpaceImport = vi.fn().mockResolvedValue({
      jobId: "import-job",
      streamUrl: "/api/admin/space-imports/import-job/events"
    });
    const subscribeSpaceImport = vi.fn((input: AdminSpaceTransferSubscribeInput) => {
      importSubscriptionInput = input;
      return { close: closeImport };
    });

    const dataGateway = {
      admin: {
        listSpaces,
        listSpaceCategories,
        listSystemConfigs,
        inspectSpaceImport,
        commitSpaceImport,
        subscribeSpaceImport
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    renderAdminSpacesPage(dataGateway, "admin");

    await user.click(await screen.findByRole("button", { name: "导入空间" }));
    const fileInput = screen.getByLabelText("空间交换包") as HTMLInputElement;
    const file = new File(["zip"], "space.plaindoc", { type: "application/octet-stream" });
    await user.upload(fileInput, file);
    await screen.findByText("源空间");
    await user.click(screen.getByRole("button", { name: "确认导入" }));

    await waitFor(() => expect(subscribeSpaceImport).toHaveBeenCalledTimes(1));
    act(() => {
      importSubscriptionInput?.onEvent({
        type: "completed",
        progress: 100,
        message: "导入完成",
        spaceId: "imported-space",
        spaceName: "导入完成空间"
      });
    });

    await waitFor(() => expect(listSpaces).toHaveBeenCalledTimes(2));
    expect(closeImport).toHaveBeenCalledTimes(1);
    expect(await screen.findByText("导入完成空间")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "展开任务中心" }));
    await user.click(screen.getByRole("button", { name: "打开编辑器" }));
    await user.click(screen.getByRole("button", { name: "打开阅读页" }));
    expect(openWindow).toHaveBeenNthCalledWith(1, "/editor/imported-space", "_blank", "noopener,noreferrer");
    expect(openWindow).toHaveBeenNthCalledWith(2, "/r/imported-space", "_blank", "noopener,noreferrer");
    openWindow.mockRestore();
  });

  it("keeps the import event stream while the task is running", async () => {
    let importSubscriptionInput: AdminSpaceTransferSubscribeInput | null = null;
    const closeImport = vi.fn();
    const listSpaces = vi.fn().mockResolvedValue({
      items: [],
      pagination: {
        page: 1,
        pageSize: 20,
        total: 0
      }
    });
    const listSpaceCategories = vi.fn().mockResolvedValue([]);
    const listSystemConfigs = vi.fn().mockResolvedValue([
      {
        configKey: "site",
        value: {
          defaultSpaceVisibility: "member"
        }
      }
    ]);
    const inspectSpaceImport = vi.fn().mockResolvedValue({
      importId: "01spaceimportpreview",
      packageVersion: 1,
      packageType: "plaindoc-space",
      exportedAt: "2026-05-16T00:00:00Z",
      importable: true,
      space: {
        spaceId: "space-source",
        name: "源空间",
        visibility: "member",
        hasCover: true
      },
      summary: {
        folderCount: 0,
        documentCount: 1,
        attachmentCount: 0,
        officeSourceCount: 0
      },
      warnings: []
    });
    const commitSpaceImport = vi.fn().mockResolvedValue({
      jobId: "import-job",
      streamUrl: "/api/admin/space-imports/import-job/events"
    });
    const subscribeSpaceImport = vi.fn((input: AdminSpaceTransferSubscribeInput) => {
      importSubscriptionInput = input;
      return { close: closeImport };
    });

    const dataGateway = {
      admin: {
        listSpaces,
        listSpaceCategories,
        listSystemConfigs,
        inspectSpaceImport,
        commitSpaceImport,
        subscribeSpaceImport
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    renderAdminSpacesPage(dataGateway, "admin");

    await user.click(await screen.findByRole("button", { name: "导入空间" }));
    const fileInput = screen.getByLabelText("空间交换包") as HTMLInputElement;
    await user.upload(fileInput, new File(["zip"], "space.plaindoc", { type: "application/octet-stream" }));
    await screen.findByText("源空间");
    await user.click(screen.getByRole("button", { name: "确认导入" }));
    await waitFor(() => expect(subscribeSpaceImport).toHaveBeenCalledTimes(1));

    act(() => {
      importSubscriptionInput?.onEvent({
        type: "progress",
        stage: "importing",
        progress: 42,
        message: "正在导入"
      });
    });

    expect(screen.queryByRole("button", { name: "确认导入" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "展开任务中心" }));
    expect(closeImport).not.toHaveBeenCalled();
    expect(screen.getByText("42%")).toBeInTheDocument();
  });

  it("keeps an import task running when the event stream errors", async () => {
    let importSubscriptionInput: AdminSpaceTransferSubscribeInput | null = null;
    const closeImport = vi.fn();
    const listSpaces = vi.fn().mockResolvedValue({
      items: [],
      pagination: {
        page: 1,
        pageSize: 20,
        total: 0
      }
    });
    const listSpaceCategories = vi.fn().mockResolvedValue([]);
    const listSystemConfigs = vi.fn().mockResolvedValue([
      {
        configKey: "site",
        value: {
          defaultSpaceVisibility: "member"
        }
      }
    ]);
    const inspectSpaceImport = vi.fn().mockResolvedValue({
      importId: "01spaceimportpreview",
      packageVersion: 1,
      packageType: "plaindoc-space",
      exportedAt: "2026-05-16T00:00:00Z",
      importable: true,
      space: {
        spaceId: "space-source",
        name: "源空间",
        visibility: "member",
        hasCover: true
      },
      summary: {
        folderCount: 0,
        documentCount: 1,
        attachmentCount: 0,
        officeSourceCount: 0
      },
      warnings: []
    });
    const commitSpaceImport = vi.fn().mockResolvedValue({
      jobId: "import-job",
      streamUrl: "/api/admin/space-imports/import-job/events"
    });
    const subscribeSpaceImport = vi.fn((input: AdminSpaceTransferSubscribeInput) => {
      importSubscriptionInput = input;
      return { close: closeImport };
    });

    const dataGateway = {
      admin: {
        listSpaces,
        listSpaceCategories,
        listSystemConfigs,
        inspectSpaceImport,
        commitSpaceImport,
        subscribeSpaceImport
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    renderAdminSpacesPage(dataGateway, "admin");

    await user.click(await screen.findByRole("button", { name: "导入空间" }));
    const fileInput = screen.getByLabelText("空间交换包") as HTMLInputElement;
    await user.upload(fileInput, new File(["zip"], "space.plaindoc", { type: "application/octet-stream" }));
    await screen.findByText("源空间");
    await user.click(screen.getByRole("button", { name: "确认导入" }));
    await waitFor(() => expect(subscribeSpaceImport).toHaveBeenCalledTimes(1));

    act(() => {
      importSubscriptionInput?.onEvent({
        type: "progress",
        stage: "importing",
        progress: 42,
        message: "正在导入"
      });
    });

    act(() => {
      importSubscriptionInput?.onError?.(new Event("error"));
    });

    await user.click(screen.getByRole("button", { name: "展开任务中心" }));
    expect(closeImport).toHaveBeenCalledTimes(1);
    expect(screen.getByText("42%")).toBeInTheDocument();
    expect(screen.queryByText("失败")).toBeNull();
  });

  it("inspects a selected PlainDoc package and shows the import preview", async () => {
    const listSpaces = vi.fn().mockResolvedValue({
      items: [],
      pagination: {
        page: 1,
        pageSize: 20,
        total: 0
      }
    });
    const listSpaceCategories = vi.fn().mockResolvedValue([]);
    const listSystemConfigs = vi.fn().mockResolvedValue([
      {
        configKey: "site",
        value: {
          defaultSpaceVisibility: "member"
        }
      }
    ]);
    const inspectSpaceImport = vi.fn().mockResolvedValue({
      importId: "01spaceimportpreview",
      packageVersion: 1,
      packageType: "plaindoc-space",
      exportedAt: "2026-05-16T00:00:00Z",
      importable: true,
      space: {
        spaceId: "space-source",
        name: "源空间",
        visibility: "member",
        hasCover: true
      },
      summary: {
        folderCount: 2,
        documentCount: 3,
        attachmentCount: 4,
        officeSourceCount: 1
      },
      warnings: []
    });

    const dataGateway = {
      admin: {
        listSpaces,
        listSpaceCategories,
        listSystemConfigs,
        inspectSpaceImport
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    renderAdminSpacesPage(dataGateway, "admin");

    await user.click(await screen.findByRole("button", { name: "导入空间" }));
    const fileInput = screen.getByLabelText("空间交换包") as HTMLInputElement;
    expect(fileInput.accept).toContain(".plaindoc");
    expect(fileInput.accept).toContain(".epub");
    expect(fileInput.accept).not.toContain(".zip");

    const file = new File(["zip"], "space.plaindoc", { type: "application/octet-stream" });
    await user.upload(fileInput, file);

    await waitFor(() => expect(inspectSpaceImport).toHaveBeenCalledWith({ file }));
    expect(await screen.findByText("源空间")).toBeInTheDocument();
    expect(screen.getByText("space-source")).toBeInTheDocument();
    expect(screen.getByDisplayValue("源空间")).toBeInTheDocument();
    expect(screen.getByText("目录 2")).toBeInTheDocument();
    expect(screen.getByText("文档 3")).toBeInTheDocument();
    expect(screen.getByText("附件 4")).toBeInTheDocument();
    expect(screen.getByText("Office 源文件 1")).toBeInTheDocument();
    expect(screen.queryByText("图片 0")).toBeNull();
    expect(screen.queryByText("层级 0")).toBeNull();
    expect(screen.getByText("包类型：plaindoc-space")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "确认导入" })).toBeEnabled();
  });

  it("inspects a selected EPUB package and shows chapter image depth warnings", async () => {
    const listSpaces = vi.fn().mockResolvedValue({
      items: [],
      pagination: {
        page: 1,
        pageSize: 20,
        total: 0
      }
    });
    const listSpaceCategories = vi.fn().mockResolvedValue([]);
    const listSystemConfigs = vi.fn().mockResolvedValue([
      {
        configKey: "site",
        value: {
          defaultSpaceVisibility: "member"
        }
      }
    ]);
    const inspectSpaceImport = vi.fn().mockResolvedValue({
      importId: "01epubimportpreview",
      packageVersion: 1,
      packageType: "epub",
      sourcePublishedAt: "2026-05-17T08:00:00Z",
      sourceAuthors: ["张三", "李四"],
      importable: true,
      space: {
        spaceId: "epub-source",
        name: "EPUB 电子书",
        visibility: "member",
        hasCover: false
      },
      summary: {
        folderCount: 2,
        documentCount: 5,
        attachmentCount: 3,
        officeSourceCount: 0,
        imageCount: 7,
        maxDepth: 4
      },
      warnings: ["章节 item-3 未出现在目录中，已按阅读顺序追加"]
    });

    const dataGateway = {
      admin: {
        listSpaces,
        listSpaceCategories,
        listSystemConfigs,
        inspectSpaceImport
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    renderAdminSpacesPage(dataGateway, "admin");

    await user.click(await screen.findByRole("button", { name: "导入空间" }));
    const fileInput = screen.getByLabelText("空间交换包") as HTMLInputElement;
    const file = new File(["epub"], "book.epub", { type: "application/epub+zip" });
    await user.upload(fileInput, file);

    await waitFor(() => expect(inspectSpaceImport).toHaveBeenCalledWith({ file }));
    expect(await screen.findByText("EPUB 电子书")).toBeInTheDocument();
    expect(screen.getByText("epub-source")).toBeInTheDocument();
    expect(screen.getByDisplayValue("EPUB 电子书")).toBeInTheDocument();
    expect(screen.getByText("作者：张三、李四")).toBeInTheDocument();
    expect(screen.getByText(/^出版日期：/)).toBeInTheDocument();
    expect(screen.getByText("包类型：epub")).toBeInTheDocument();
    expect(screen.getByText("目录 2")).toBeInTheDocument();
    expect(screen.getByText("文档 5")).toBeInTheDocument();
    expect(screen.getByText("附件 3")).toBeInTheDocument();
    expect(screen.getByText("图片 7")).toBeInTheDocument();
    expect(screen.getByText("层级 4")).toBeInTheDocument();
    expect(screen.getByText("Warnings")).toBeInTheDocument();
    expect(screen.getByText("章节 item-3 未出现在目录中，已按阅读顺序追加")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "确认导入" })).toBeEnabled();
  });

  it("keeps the EPUB preview usable when backend returns null warnings", async () => {
    const listSpaces = vi.fn().mockResolvedValue({
      items: [],
      pagination: {
        page: 1,
        pageSize: 20,
        total: 0
      }
    });
    const listSpaceCategories = vi.fn().mockResolvedValue([]);
    const listSystemConfigs = vi.fn().mockResolvedValue([
      {
        configKey: "site",
        value: {
          defaultSpaceVisibility: "member"
        }
      }
    ]);
    const inspectSpaceImport = vi.fn().mockResolvedValue({
      importId: "01epubimportpreview",
      packageVersion: 1,
      packageType: "epub",
      sourceAuthors: [],
      importable: true,
      space: {
        spaceId: "epub-source",
        name: "无警告 EPUB",
        visibility: "member",
        hasCover: false
      },
      summary: {
        folderCount: 1,
        documentCount: 2,
        attachmentCount: 0,
        officeSourceCount: 0,
        imageCount: 0,
        maxDepth: 1
      },
      warnings: null as unknown as string[]
    });

    const dataGateway = {
      admin: {
        listSpaces,
        listSpaceCategories,
        listSystemConfigs,
        inspectSpaceImport
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    renderAdminSpacesPage(dataGateway, "admin");

    await user.click(await screen.findByRole("button", { name: "导入空间" }));
    const fileInput = screen.getByLabelText("空间交换包") as HTMLInputElement;
    await user.upload(fileInput, new File(["epub"], "book.epub", { type: "application/epub+zip" }));

    expect(await screen.findByText("无警告 EPUB")).toBeInTheDocument();
    expect(screen.queryByText("Warnings")).toBeNull();
    expect(screen.getByRole("button", { name: "确认导入" })).toBeEnabled();
  });

  it("disables import confirmation for non-importable packages", async () => {
    const listSpaces = vi.fn().mockResolvedValue({
      items: [],
      pagination: {
        page: 1,
        pageSize: 20,
        total: 0
      }
    });
    const listSpaceCategories = vi.fn().mockResolvedValue([]);
    const listSystemConfigs = vi.fn().mockResolvedValue([
      {
        configKey: "site",
        value: {
          defaultSpaceVisibility: "member"
        }
      }
    ]);
    const inspectSpaceImport = vi.fn().mockResolvedValue({
      importId: "01spaceimportpreview",
      packageVersion: 1,
      packageType: "plaindoc-space",
      exportedAt: "2026-05-16T00:00:00Z",
      importable: false,
      space: {
        spaceId: "space-source",
        name: "源空间",
        visibility: "member",
        hasCover: true
      },
      summary: {
        folderCount: 0,
        documentCount: 1,
        attachmentCount: 0,
        officeSourceCount: 0
      },
      warnings: ["缺少 source 文件"]
    });

    const dataGateway = {
      admin: {
        listSpaces,
        listSpaceCategories,
        listSystemConfigs,
        inspectSpaceImport
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    renderAdminSpacesPage(dataGateway, "admin");

    await user.click(await screen.findByRole("button", { name: "导入空间" }));
    const fileInput = screen.getByLabelText("空间交换包") as HTMLInputElement;
    const file = new File(["zip"], "space.plaindoc", { type: "application/octet-stream" });
    await user.upload(fileInput, file);

    expect(await screen.findByText("仅可预览")).toBeInTheDocument();
    expect(screen.getByText("缺少 source 文件")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "确认导入" })).toBeDisabled();
  });
});
