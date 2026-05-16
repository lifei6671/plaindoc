import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { AdminSpace, AdminSpaceTransferSubscribeInput, DataGateway } from "../../data-access";
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

describe("AdminSpacesPage", () => {
  beforeEach(() => {
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

    render(<AdminSpacesPage dataGateway={dataGateway} mode="member" />);

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
    render(<AdminSpacesPage dataGateway={dataGateway} mode="admin" />);

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
    render(<AdminSpacesPage dataGateway={dataGateway} mode="admin" />);

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
    render(<AdminSpacesPage dataGateway={dataGateway} mode="admin" />);

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
    render(<AdminSpacesPage dataGateway={dataGateway} mode="admin" />);

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
    expect(screen.getByRole("button", { name: "下载文件" })).toBeInTheDocument();
    expect(screen.getByText("管理空间.epub")).toBeInTheDocument();

    await user.selectOptions(screen.getByDisplayValue("EPUB"), "source_zip");
    expect(screen.getByText("管理空间.epub")).toBeInTheDocument();
    expect(screen.queryByText("管理空间.plaindoc")).toBeNull();

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

    await user.click(screen.getByRole("button", { name: "下载文件" }));
    await waitFor(() => expect(subscribeSpaceExport).toHaveBeenCalledTimes(2));
    act(() => {
      exportSubscriptionInput?.onEvent({
        type: "completed",
        progress: 100,
        message: "导出完成",
        downloadUrl: "/api/admin/space-exports/export-job/download?token=two",
        fileName: "管理空间.zip"
      });
    });
    await waitFor(() => expect(anchorClick).toHaveBeenCalledTimes(1));
    expect(closeExport).toHaveBeenCalledTimes(2);
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
    render(<AdminSpacesPage dataGateway={dataGateway} mode="admin" />);

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

    await user.click(screen.getByRole("button", { name: "下载文件" }));
    await waitFor(() => expect(subscribeSpaceExport).toHaveBeenCalledTimes(2));
    act(() => {
      exportSubscriptionInputs[1]?.onEvent({
        type: "completed",
        progress: 100,
        message: "导出完成",
        downloadUrl: "/api/admin/space-exports/export-job/download?token=two",
        fileName: "管理空间.epub"
      });
    });

    await waitFor(() => expect(clickedHrefs).toHaveLength(1));
    expect(clickedHrefs[0]).toContain("token=two");
    expect(closeExports[1]).toHaveBeenCalledTimes(1);

    await user.click(screen.getByRole("button", { name: "下载文件" }));
    await waitFor(() => expect(subscribeSpaceExport).toHaveBeenCalledTimes(3));
    act(() => {
      exportSubscriptionInputs[2]?.onEvent({
        type: "completed",
        progress: 100,
        message: "导出完成",
        downloadUrl: "/api/admin/space-exports/export-job/download?token=three",
        fileName: "管理空间.epub"
      });
    });

    await waitFor(() => expect(clickedHrefs).toHaveLength(2));
    expect(clickedHrefs[1]).toContain("token=three");
    expect(closeExports[2]).toHaveBeenCalledTimes(1);
    anchorClick.mockRestore();
  });

  it("closes a pending export download refresh subscription when the dialog closes", async () => {
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
        subscribeSpaceExport
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    render(<AdminSpacesPage dataGateway={dataGateway} mode="admin" />);

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

    await user.click(screen.getByRole("button", { name: "下载文件" }));
    await waitFor(() => expect(subscribeSpaceExport).toHaveBeenCalledTimes(2));

    await user.click(screen.getByRole("button", { name: "关闭" }));

    expect(closeExports[1]).toHaveBeenCalledTimes(1);
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
    render(<AdminSpacesPage dataGateway={dataGateway} mode="admin" />);

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
    expect(await screen.findByText("正在导出文档")).toBeInTheDocument();
    expect(screen.getByText("documents")).toBeInTheDocument();

    act(() => {
      exportSubscriptionInput?.onEvent({
        type: "failed",
        stage: "zip",
        progress: 45,
        message: "导出失败"
      });
    });

    expect(await screen.findByText("导出失败")).toBeInTheDocument();
    expect(closeExport).toHaveBeenCalledTimes(1);
  });

  it("keeps the export event stream while the task is running", async () => {
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
    render(<AdminSpacesPage dataGateway={dataGateway} mode="admin" />);

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

    const closeButton = screen.getByRole("button", { name: "关闭" });
    expect(closeButton).toBeDisabled();
    await user.click(closeButton);

    expect(closeExport).not.toHaveBeenCalled();
    expect(screen.getByText("正在导出文档")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "开始导出" })).toBeInTheDocument();
  });

  it("unlocks the export dialog when the event stream errors", async () => {
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
    render(<AdminSpacesPage dataGateway={dataGateway} mode="admin" />);

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

    expect(screen.getByRole("button", { name: "关闭" })).toBeDisabled();

    act(() => {
      exportSubscriptionInput?.onError?.(new Event("error"));
    });

    expect(await screen.findByText("导出事件连接异常，请稍后重试")).toBeInTheDocument();
    expect(closeExport).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: "关闭" })).not.toBeDisabled();
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
    render(<AdminSpacesPage dataGateway={dataGateway} mode="admin" />);

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
    render(<AdminSpacesPage dataGateway={dataGateway} mode="admin" />);

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
    const cancelButton = screen.getByRole("button", { name: "取消" });
    expect(cancelButton).toBeDisabled();
    await user.click(cancelButton);

    expect(closeImport).not.toHaveBeenCalled();
    expect(screen.getByText("正在导入")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "确认导入" })).toBeInTheDocument();
  });

  it("unlocks the import dialog when the event stream errors", async () => {
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
    render(<AdminSpacesPage dataGateway={dataGateway} mode="admin" />);

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
    expect(screen.getByRole("button", { name: "取消" })).toBeDisabled();

    act(() => {
      importSubscriptionInput?.onError?.(new Event("error"));
    });

    expect(closeImport).toHaveBeenCalledTimes(1);
    expect(screen.getByText("导入事件连接异常，请稍后重试")).toBeInTheDocument();
    expect(screen.getByText("failed")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "取消" })).toBeEnabled();
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
    render(<AdminSpacesPage dataGateway={dataGateway} mode="admin" />);

    await user.click(await screen.findByRole("button", { name: "导入空间" }));
    const fileInput = screen.getByLabelText("空间交换包") as HTMLInputElement;
    expect(fileInput.accept).toContain(".plaindoc");
    expect(fileInput.accept).not.toContain(".zip");
	    expect(fileInput.accept).not.toContain("epub");

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
    render(<AdminSpacesPage dataGateway={dataGateway} mode="admin" />);

    await user.click(await screen.findByRole("button", { name: "导入空间" }));
    const fileInput = screen.getByLabelText("空间交换包") as HTMLInputElement;
    const file = new File(["zip"], "space.plaindoc", { type: "application/octet-stream" });
    await user.upload(fileInput, file);

    expect(await screen.findByText("仅可预览")).toBeInTheDocument();
    expect(screen.getByText("缺少 source 文件")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "确认导入" })).toBeDisabled();
  });
});
