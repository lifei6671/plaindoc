import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ForgotPasswordPanel } from "./ForgotPasswordPanel";

describe("ForgotPasswordPanel", () => {
  it("submits trimmed email and shows success message", async () => {
    const onSubmit = vi.fn<((email: string) => Promise<void>)>().mockResolvedValue(undefined);
    const user = userEvent.setup();

    render(
      <ForgotPasswordPanel
        submitting={false}
        errorMessage={null}
        loginPath="/login"
        onSubmit={onSubmit}
      />
    );

    const submitButton = screen.getByRole("button", { name: "发送重置邮件" });
    expect(submitButton).toBeDisabled();

    await user.type(screen.getByPlaceholderText("name@example.com"), "  test@example.com  ");
    expect(submitButton).not.toBeDisabled();

    await user.click(submitButton);

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledTimes(1);
      expect(onSubmit).toHaveBeenCalledWith("test@example.com");
    });
    expect(screen.getByText("如果邮箱存在，系统将发送重置链接，请注意查收。")).toBeInTheDocument();
  });

  it("renders error and login link", () => {
    const onSubmit = vi.fn<((email: string) => Promise<void>)>().mockResolvedValue(undefined);

    render(
      <ForgotPasswordPanel
        submitting={false}
        errorMessage="请求失败"
        loginPath="/login"
        onSubmit={onSubmit}
      />
    );

    expect(screen.getByText("请求失败")).toBeInTheDocument();
    const loginLink = screen.getByRole("link", { name: "返回登录" });
    expect(loginLink).toHaveAttribute("href", "/login");
  });
});
