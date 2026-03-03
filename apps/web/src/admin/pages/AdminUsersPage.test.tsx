import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { DataGateway } from "../../data-access";
import { AdminUsersPage } from "./AdminUsersPage";

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

describe("AdminUsersPage", () => {
  beforeEach(() => {
    confirmMock.mockReset();
    promptMock.mockReset();
    showToastMock.mockReset();
    promptMock.mockResolvedValue(null);
  });

  it("sends password reset email after confirm", async () => {
    const listUsers = vi.fn().mockResolvedValue({
      items: [
        {
          userId: "user-target",
          email: "target@example.com",
          name: "Target User",
          role: "user",
          canEditRole: true,
          status: "active",
          bannedReason: "",
          bannedAt: null,
          deletedAt: null,
          createdAt: "2026-03-01T00:00:00Z",
          updatedAt: "2026-03-01T00:00:00Z"
        }
      ],
      pagination: {
        page: 1,
        pageSize: 20,
        total: 1
      }
    });
    const sendUserPasswordResetEmail = vi.fn().mockResolvedValue(undefined);
    confirmMock.mockResolvedValue(true);

    const dataGateway = {
      admin: {
        listUsers,
        sendUserPasswordResetEmail
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    render(<AdminUsersPage currentUserID="admin-self" dataGateway={dataGateway} />);

    await screen.findByText("target@example.com");
    await user.click(screen.getByRole("button", { name: "重置邮件" }));

    await waitFor(() => {
      expect(confirmMock).toHaveBeenCalledTimes(1);
      expect(sendUserPasswordResetEmail).toHaveBeenCalledTimes(1);
      expect(sendUserPasswordResetEmail).toHaveBeenCalledWith({ userId: "user-target" });
    });
    expect(showToastMock).toHaveBeenCalledWith("重置密码邮件已发送：target@example.com", "success");
    expect(listUsers.mock.calls.length).toBeGreaterThanOrEqual(2);
  });

  it("disables reset email button for current user", async () => {
    const listUsers = vi.fn().mockResolvedValue({
      items: [
        {
          userId: "admin-self",
          email: "admin@example.com",
          name: "Admin",
          role: "platform_admin",
          canEditRole: true,
          status: "active",
          bannedReason: "",
          bannedAt: null,
          deletedAt: null,
          createdAt: "2026-03-01T00:00:00Z",
          updatedAt: "2026-03-01T00:00:00Z"
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
        listUsers
      }
    } as unknown as DataGateway;

    render(<AdminUsersPage currentUserID="admin-self" dataGateway={dataGateway} />);
    await screen.findByText("admin@example.com");

    expect(screen.getByRole("button", { name: "重置邮件" })).toBeDisabled();
    expect(confirmMock).not.toHaveBeenCalled();
  });
});
