import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { DataGateway } from "../data-access";
import { AdminApp } from "./AdminApp";

vi.mock("./pages/AdminProfilePage", () => {
  return {
    AdminProfilePage: () => <div data-testid="admin-profile-page">个人信息页</div>
  };
});

vi.mock("./pages/AdminSpacesPage", () => {
  return {
    AdminSpacesPage: () => <div data-testid="admin-spaces-page">空间管理页</div>
  };
});

vi.mock("./pages/AdminUsersPage", () => ({ AdminUsersPage: () => <div /> }));
vi.mock("./pages/AdminDocumentsPage", () => ({ AdminDocumentsPage: () => <div /> }));
vi.mock("./pages/AdminDocumentSharesPage", () => ({ AdminDocumentSharesPage: () => <div /> }));
vi.mock("./pages/AdminDocumentAttachmentsPage", () => ({ AdminDocumentAttachmentsPage: () => <div /> }));
vi.mock("./pages/AdminDocumentImagesPage", () => ({ AdminDocumentImagesPage: () => <div /> }));
vi.mock("./pages/AdminDocumentTemplatesPage", () => ({ AdminDocumentTemplatesPage: () => <div /> }));
vi.mock("./pages/AdminThemesPage", () => ({ AdminThemesPage: () => <div /> }));
vi.mock("./pages/AdminSystemConfigsPage", () => ({ AdminSystemConfigsPage: () => <div /> }));
vi.mock("./pages/AdminAuditsPage", () => ({ AdminAuditsPage: () => <div /> }));
vi.mock("./pages/AdminSearchAnalyzersPage", () => ({ AdminSearchAnalyzersPage: () => <div /> }));

describe("AdminApp", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("only shows profile menu for a logged-in user without space membership", async () => {
    const getMe = vi.fn().mockResolvedValue({
      userId: "user-1",
      email: "user@example.com",
      name: "用户A",
      avatarUrl: "",
      roles: [],
      capabilities: {
        canViewSpaceManagement: false,
        canManageSpace: false
      }
    });

    const dataGateway = {
      admin: {
        getMe
      }
    } as unknown as DataGateway;

    render(
      <MemoryRouter initialEntries={["/admin/profile"]}>
        <AdminApp
          authSession={{ user: { id: "user-1", email: "user@example.com", name: "用户A" } }}
          checking={false}
          submitting={false}
          errorMessage={null}
          authChallenge={null}
          dataGateway={dataGateway}
          onLogin={vi.fn()}
          onRefreshCaptcha={vi.fn()}
          onLogout={vi.fn()}
        />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId("admin-profile-page")).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "个人信息" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "空间管理" })).toBeNull();
    expect(screen.queryByRole("button", { name: "概览" })).toBeNull();
    expect(screen.getByText("普通用户")).toBeInTheDocument();
  });

  it("shows space management menu for a logged-in space member", async () => {
    const getMe = vi.fn().mockResolvedValue({
      userId: "member-1",
      email: "member@example.com",
      name: "成员A",
      avatarUrl: "",
      roles: ["space_admin"],
      capabilities: {
        canViewSpaceManagement: true,
        canManageSpace: true
      }
    });

    const dataGateway = {
      admin: {
        getMe
      }
    } as unknown as DataGateway;

    render(
      <MemoryRouter initialEntries={["/admin/profile"]}>
        <AdminApp
          authSession={{ user: { id: "member-1", email: "member@example.com", name: "成员A" } }}
          checking={false}
          submitting={false}
          errorMessage={null}
          authChallenge={null}
          dataGateway={dataGateway}
          onLogin={vi.fn()}
          onRefreshCaptcha={vi.fn()}
          onLogout={vi.fn()}
        />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId("admin-profile-page")).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "个人信息" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "空间管理" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "概览" })).toBeInTheDocument();
    expect(screen.getByText("空间管理员")).toBeInTheDocument();
  });

  it("shows platform admin badge for privileged admin users", async () => {
    const getMe = vi.fn().mockResolvedValue({
      userId: "platform-1",
      email: "platform@example.com",
      name: "平台A",
      avatarUrl: "",
      roles: ["platform_admin"],
      capabilities: {
        canViewSpaceManagement: true,
        canManageSpace: true
      }
    });

    const dataGateway = {
      admin: {
        getMe
      }
    } as unknown as DataGateway;

    render(
      <MemoryRouter initialEntries={["/admin/profile"]}>
        <AdminApp
          authSession={{ user: { id: "platform-1", email: "platform@example.com", name: "平台A" } }}
          checking={false}
          submitting={false}
          errorMessage={null}
          authChallenge={null}
          dataGateway={dataGateway}
          onLogin={vi.fn()}
          onRefreshCaptcha={vi.fn()}
          onLogout={vi.fn()}
        />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId("admin-profile-page")).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "概览" })).toBeInTheDocument();
    expect(screen.getByText("平台管理员")).toBeInTheDocument();
  });
});
