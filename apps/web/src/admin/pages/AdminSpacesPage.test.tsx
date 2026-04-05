import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { DataGateway } from "../../data-access";
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

describe("AdminSpacesPage", () => {
  beforeEach(() => {
    confirmMock.mockReset();
    promptMock.mockReset();
    showToastMock.mockReset();
  });

  it("shows only the edit document action in member mode", async () => {
    const listSpaces = vi.fn().mockResolvedValue({
      items: [
        {
          spaceId: "space-member",
          name: "成员空间",
          description: "成员视图空间",
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
          updatedAt: "2026-04-05T00:00:00Z"
        }
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
        {
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
          updatedAt: "2026-04-05T00:00:00Z"
        }
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
    expect(screen.getByText("成员管理")).toBeInTheDocument();
    expect(screen.getByText("删除空间")).toBeInTheDocument();
  });
});
