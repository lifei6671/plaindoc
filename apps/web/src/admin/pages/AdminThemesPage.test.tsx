import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { DataGateway } from "../../data-access";
import { AdminThemesPage } from "./AdminThemesPage";

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

describe("AdminThemesPage", () => {
  beforeEach(() => {
    confirmMock.mockReset();
    promptMock.mockReset();
    showToastMock.mockReset();
  });

  it("hides edit button for builtin themes while keeping custom theme edit action", async () => {
    const listThemes = vi.fn().mockResolvedValue([
      {
        themeId: "default",
        name: "内置默认",
        description: "系统内置主题",
        variables: {},
        syntaxTheme: "one-light",
        codeBlockStyle: {},
        codeBlockCodeStyle: {},
        inlineCodeStyle: {},
        customCss: "",
        builtin: true,
        enabled: true,
        createdAt: "2026-04-05T00:00:00Z",
        updatedAt: "2026-04-05T00:00:00Z"
      },
      {
        themeId: "custom_theme",
        name: "自定义主题",
        description: "用户创建主题",
        variables: {},
        syntaxTheme: "one-dark",
        codeBlockStyle: {},
        codeBlockCodeStyle: {},
        inlineCodeStyle: {},
        customCss: "",
        builtin: false,
        enabled: true,
        createdAt: "2026-04-05T00:00:00Z",
        updatedAt: "2026-04-05T00:00:00Z"
      }
    ]);

    const dataGateway = {
      admin: {
        listThemes
      }
    } as unknown as DataGateway;

    render(<AdminThemesPage dataGateway={dataGateway} />);

    const builtinThemeName = await screen.findByText("内置默认");
    const builtinRow = builtinThemeName.closest("tr");
    expect(builtinRow).not.toBeNull();
    expect(within(builtinRow as HTMLTableRowElement).queryByRole("button", { name: "编辑" })).toBeNull();
    expect(within(builtinRow as HTMLTableRowElement).queryByRole("button", { name: "停用" })).toBeNull();
    expect(within(builtinRow as HTMLTableRowElement).queryByRole("button", { name: "删除" })).toBeNull();

    const customThemeName = await screen.findByText("自定义主题");
    const customRow = customThemeName.closest("tr");
    expect(customRow).not.toBeNull();
    expect(within(customRow as HTMLTableRowElement).getByRole("button", { name: "编辑" })).toBeInTheDocument();
    expect(within(customRow as HTMLTableRowElement).getByRole("button", { name: "停用" })).toBeInTheDocument();
    expect(within(customRow as HTMLTableRowElement).getByRole("button", { name: "删除" })).toBeInTheDocument();
  });
});
