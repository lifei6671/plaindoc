import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AdminProfile, DataGateway } from "../../data-access";
import { AdminProfilePage } from "./AdminProfilePage";

const { showToastMock } = vi.hoisted(() => ({
  showToastMock: vi.fn()
}));

vi.mock("../../components/ui/toast", () => {
  return {
    showToast: showToastMock
  };
});

describe("AdminProfilePage", () => {
  beforeEach(() => {
    showToastMock.mockReset();
  });

  it("allows a logged-in normal user to update personal profile information", async () => {
    const profile: AdminProfile = {
      userId: "user-1",
      email: "user@example.com",
      name: "普通用户",
      avatarUrl: "",
      roles: [],
      createdAt: "2026-04-05T00:00:00Z",
      updatedAt: "2026-04-05T00:00:00Z"
    };
    const updatedProfile: AdminProfile = {
      ...profile,
      name: "普通用户已更新"
    };

    const getProfile = vi.fn().mockResolvedValue(profile);
    const updateProfile = vi.fn().mockResolvedValue(updatedProfile);
    const updatePassword = vi.fn().mockResolvedValue(undefined);
    const uploadAvatar = vi.fn().mockResolvedValue(updatedProfile);
    const onProfileUpdated = vi.fn();

    const dataGateway = {
      admin: {
        getProfile,
        updateProfile,
        updatePassword,
        uploadAvatar
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    render(<AdminProfilePage dataGateway={dataGateway} onProfileUpdated={onProfileUpdated} />);

    const nameInput = (await screen.findByPlaceholderText("输入昵称")) as HTMLInputElement;
    await user.clear(nameInput);
    await user.type(nameInput, "  普通用户已更新  ");
    await user.click(screen.getByRole("button", { name: "保存个人信息" }));

    await waitFor(() => {
      expect(updateProfile).toHaveBeenCalledWith({ name: "普通用户已更新" });
    });
    expect(onProfileUpdated).toHaveBeenCalledWith(updatedProfile);
    expect(showToastMock).toHaveBeenCalledWith("个人信息已更新", "success");
  });

  it("allows a logged-in normal user to update password", async () => {
    const profile: AdminProfile = {
      userId: "user-2",
      email: "user2@example.com",
      name: "普通用户二号",
      avatarUrl: "",
      roles: [],
      createdAt: "2026-04-05T00:00:00Z",
      updatedAt: "2026-04-05T00:00:00Z"
    };

    const getProfile = vi.fn().mockResolvedValue(profile);
    const updateProfile = vi.fn().mockResolvedValue(profile);
    const updatePassword = vi.fn().mockResolvedValue(undefined);
    const uploadAvatar = vi.fn().mockResolvedValue(profile);

    const dataGateway = {
      admin: {
        getProfile,
        updateProfile,
        updatePassword,
        uploadAvatar
      }
    } as unknown as DataGateway;

    const user = userEvent.setup();
    render(<AdminProfilePage dataGateway={dataGateway} />);

    await screen.findByDisplayValue("普通用户二号");

    await user.type(screen.getByPlaceholderText("输入当前密码"), "123456");
    await user.type(screen.getByPlaceholderText("至少 6 位"), "newpass123");
    await user.type(screen.getByPlaceholderText("再次输入新密码"), "newpass123");
    await user.click(screen.getByRole("button", { name: "更新密码" }));

    await waitFor(() => {
      expect(updatePassword).toHaveBeenCalledWith({
        currentPassword: "123456",
        newPassword: "newpass123",
        confirmPassword: "newpass123"
      });
    });
    expect(showToastMock).toHaveBeenCalledWith("密码修改成功", "success");
  });
});
