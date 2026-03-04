import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { DataGateway } from "../../data-access";
import { AdminDocumentTemplatesPage } from "./AdminDocumentTemplatesPage";

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

describe("AdminDocumentTemplatesPage", () => {
  beforeEach(() => {
    confirmMock.mockReset();
    promptMock.mockReset();
    showToastMock.mockReset();
  });

  it("creates template from prompt result and refreshes list", async () => {
    const listDocumentTemplates = vi
      .fn()
      .mockResolvedValueOnce({
        items: [],
        pagination: { page: 1, pageSize: 200, total: 0 }
      })
      .mockResolvedValue({
        items: [
          {
            templateId: "meeting-template",
            sceneKey: "meeting",
            sceneName: "会议",
            name: "会议模板",
            description: "",
            defaultTitle: "",
            sort: 0,
            builtin: false,
            enabled: true,
            updatedAt: "2026-03-04T00:00:00Z"
          }
        ],
        pagination: { page: 1, pageSize: 200, total: 1 }
      });
    const createDocumentTemplate = vi.fn().mockResolvedValue({
      templateId: "meeting-template",
      sceneKey: "meeting",
      sceneName: "会议",
      name: "会议模板",
      description: "",
      defaultTitle: "",
      contentMd: "# 模板正文",
      sort: 7,
      builtin: false,
      enabled: true,
      createdAt: "2026-03-04T00:00:00Z",
      updatedAt: "2026-03-04T00:00:00Z"
    });
    promptMock.mockResolvedValue({
      templateId: " meeting-template ",
      sceneKey: " meeting ",
      sceneName: " 会议 ",
      name: " 会议模板 ",
      description: " 描述 ",
      defaultTitle: " 默认标题 ",
      sort: "7",
      enabled: "true",
      contentMd: "# 模板正文"
    });

    const dataGateway = {
      admin: {
        listDocumentTemplates,
        createDocumentTemplate
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    render(<AdminDocumentTemplatesPage dataGateway={dataGateway} />);

    await screen.findByText("暂无模板");
    await user.click(screen.getByRole("button", { name: "新建模板" }));

    await waitFor(() => {
      expect(promptMock).toHaveBeenCalledTimes(1);
      expect(createDocumentTemplate).toHaveBeenCalledTimes(1);
    });
    expect(createDocumentTemplate).toHaveBeenCalledWith({
      templateId: "meeting-template",
      sceneKey: "meeting",
      sceneName: "会议",
      name: "会议模板",
      description: "描述",
      defaultTitle: "默认标题",
      contentMd: "# 模板正文",
      sort: 7,
      enabled: true
    });
    expect(showToastMock).toHaveBeenCalledWith("文档模板创建成功", "success");
    expect(listDocumentTemplates.mock.calls.length).toBeGreaterThanOrEqual(2);
  });

  it("edits template after loading detail", async () => {
    const listDocumentTemplates = vi.fn().mockResolvedValue({
      items: [
        {
          templateId: "tpl-weekly",
          sceneKey: "report",
          sceneName: "报告",
          name: "周报模板",
          description: "",
          defaultTitle: "",
          sort: 10,
          builtin: false,
          enabled: true,
          updatedAt: "2026-03-04T00:00:00Z"
        }
      ],
      pagination: { page: 1, pageSize: 200, total: 1 }
    });
    const getDocumentTemplate = vi.fn().mockResolvedValue({
      templateId: "tpl-weekly",
      sceneKey: "report",
      sceneName: "报告",
      name: "周报模板",
      description: "旧描述",
      defaultTitle: "周报",
      contentMd: "# old",
      sort: 10,
      builtin: false,
      enabled: true,
      createdAt: "2026-03-04T00:00:00Z",
      updatedAt: "2026-03-04T00:00:00Z"
    });
    const updateDocumentTemplate = vi.fn().mockResolvedValue({
      templateId: "tpl-weekly",
      sceneKey: "report",
      sceneName: "报告",
      name: "周报模板（新）",
      description: "新描述",
      defaultTitle: "周报",
      contentMd: "# new",
      sort: 11,
      builtin: false,
      enabled: false,
      createdAt: "2026-03-04T00:00:00Z",
      updatedAt: "2026-03-04T00:00:00Z"
    });
    promptMock.mockResolvedValue({
      sceneKey: "report",
      sceneName: "报告",
      name: "周报模板（新）",
      description: "新描述",
      defaultTitle: "周报",
      contentMd: "# new",
      sort: "11",
      enabled: "false"
    });

    const dataGateway = {
      admin: {
        listDocumentTemplates,
        getDocumentTemplate,
        updateDocumentTemplate
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    render(<AdminDocumentTemplatesPage dataGateway={dataGateway} />);

    const templateName = await screen.findByText("周报模板");
    const row = templateName.closest("tr");
    expect(row).not.toBeNull();
    await user.click(within(row as HTMLTableRowElement).getByRole("button", { name: "编辑" }));

    await waitFor(() => {
      expect(getDocumentTemplate).toHaveBeenCalledWith("tpl-weekly");
      expect(promptMock).toHaveBeenCalledTimes(1);
      expect(updateDocumentTemplate).toHaveBeenCalledTimes(1);
    });
    expect(updateDocumentTemplate).toHaveBeenCalledWith({
      templateId: "tpl-weekly",
      sceneKey: "report",
      sceneName: "报告",
      name: "周报模板（新）",
      description: "新描述",
      defaultTitle: "周报",
      contentMd: "# new",
      sort: 11,
      enabled: false
    });
    expect(showToastMock).toHaveBeenCalledWith("文档模板更新成功", "success");
  });

  it("deletes template after confirm", async () => {
    const listDocumentTemplates = vi.fn().mockResolvedValue({
      items: [
        {
          templateId: "tpl-meeting",
          sceneKey: "meeting",
          sceneName: "会议",
          name: "会议模板",
          description: "",
          defaultTitle: "",
          sort: 0,
          builtin: false,
          enabled: true,
          updatedAt: "2026-03-04T00:00:00Z"
        }
      ],
      pagination: { page: 1, pageSize: 200, total: 1 }
    });
    const deleteDocumentTemplate = vi.fn().mockResolvedValue(undefined);
    confirmMock.mockResolvedValue(true);

    const dataGateway = {
      admin: {
        listDocumentTemplates,
        deleteDocumentTemplate
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    render(<AdminDocumentTemplatesPage dataGateway={dataGateway} />);

    const templateName = await screen.findByText("会议模板");
    const row = templateName.closest("tr");
    expect(row).not.toBeNull();
    await user.click(within(row as HTMLTableRowElement).getByRole("button", { name: "删除" }));

    await waitFor(() => {
      expect(confirmMock).toHaveBeenCalledTimes(1);
      expect(deleteDocumentTemplate).toHaveBeenCalledWith("tpl-meeting");
    });
    expect(showToastMock).toHaveBeenCalledWith("文档模板已删除", "success");
  });

  it("disables actions for builtin template", async () => {
    const listDocumentTemplates = vi.fn().mockResolvedValue({
      items: [
        {
          templateId: "tpl-builtin",
          sceneKey: "meeting",
          sceneName: "会议",
          name: "内置模板",
          description: "",
          defaultTitle: "",
          sort: 0,
          builtin: true,
          enabled: true,
          updatedAt: "2026-03-04T00:00:00Z"
        }
      ],
      pagination: { page: 1, pageSize: 200, total: 1 }
    });

    const dataGateway = {
      admin: {
        listDocumentTemplates
      }
    } as unknown as DataGateway;

    render(<AdminDocumentTemplatesPage dataGateway={dataGateway} />);

    const templateName = await screen.findByText("内置模板");
    const row = templateName.closest("tr");
    expect(row).not.toBeNull();

    expect(within(row as HTMLTableRowElement).getByRole("button", { name: "编辑" })).toBeDisabled();
    expect(within(row as HTMLTableRowElement).getByRole("button", { name: "删除" })).toBeDisabled();
    expect(confirmMock).not.toHaveBeenCalled();
    expect(promptMock).not.toHaveBeenCalled();
  });

  it("searches, refreshes and resets with expected keyword", async () => {
    const listDocumentTemplates = vi.fn().mockImplementation(async (input?: { keyword?: string }) => {
      const keyword = (input?.keyword ?? "").trim();
      if (keyword === "meeting") {
        return {
          items: [
            {
              templateId: "tpl-meeting",
              sceneKey: "meeting",
              sceneName: "会议",
              name: "会议模板",
              description: "",
              defaultTitle: "",
              sort: 0,
              builtin: false,
              enabled: true,
              updatedAt: "2026-03-04T00:00:00Z"
            }
          ],
          pagination: { page: 1, pageSize: 200, total: 1 }
        };
      }
      return {
        items: [
          {
            templateId: "tpl-meeting",
            sceneKey: "meeting",
            sceneName: "会议",
            name: "会议模板",
            description: "",
            defaultTitle: "",
            sort: 0,
            builtin: false,
            enabled: true,
            updatedAt: "2026-03-04T00:00:00Z"
          },
          {
            templateId: "tpl-weekly",
            sceneKey: "report",
            sceneName: "报告",
            name: "周报模板",
            description: "",
            defaultTitle: "",
            sort: 10,
            builtin: false,
            enabled: true,
            updatedAt: "2026-03-04T00:00:00Z"
          }
        ],
        pagination: { page: 1, pageSize: 200, total: 2 }
      };
    });

    const dataGateway = {
      admin: {
        listDocumentTemplates
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    render(<AdminDocumentTemplatesPage dataGateway={dataGateway} />);

    await screen.findByText("周报模板");
    expect(listDocumentTemplates).toHaveBeenNthCalledWith(1, {
      keyword: "",
      page: 1,
      pageSize: 200
    });

    await user.type(screen.getByPlaceholderText("按模板 ID / 名称 / 场景搜索"), "  meeting  ");
    await user.click(screen.getByRole("button", { name: "搜索" }));

    await waitFor(() => {
      expect(listDocumentTemplates).toHaveBeenCalledTimes(2);
    });
    expect(listDocumentTemplates).toHaveBeenNthCalledWith(2, {
      keyword: "meeting",
      page: 1,
      pageSize: 200
    });
    await waitFor(() => {
      expect(screen.queryByText("周报模板")).not.toBeInTheDocument();
    });
    expect(screen.getByText("会议模板")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "刷新" }));
    await waitFor(() => {
      expect(listDocumentTemplates).toHaveBeenCalledTimes(3);
    });
    expect(listDocumentTemplates).toHaveBeenNthCalledWith(3, {
      keyword: "meeting",
      page: 1,
      pageSize: 200
    });

    await user.click(screen.getByRole("button", { name: "重置" }));
    await waitFor(() => {
      expect(listDocumentTemplates).toHaveBeenCalledTimes(4);
    });
    expect(listDocumentTemplates).toHaveBeenNthCalledWith(4, {
      keyword: "",
      page: 1,
      pageSize: 200
    });
    expect(screen.getByPlaceholderText("按模板 ID / 名称 / 场景搜索")).toHaveValue("");
    expect(screen.getByText("周报模板")).toBeInTheDocument();
  });
});
