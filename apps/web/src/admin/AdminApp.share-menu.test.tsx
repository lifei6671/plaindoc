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

describe("AdminApp share menu", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("shows share center for normal users", async () => {
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
    expect(screen.getByRole("button", { name: "分享中心" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "概览" })).toBeNull();
    expect(screen.getByText("普通用户")).toBeInTheDocument();
  });
});

