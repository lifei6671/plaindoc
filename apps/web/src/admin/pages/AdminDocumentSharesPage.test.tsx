import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AdminDocumentShareListResult, DataGateway } from "../../data-access";
import { AdminDocumentSharesPage } from "./AdminDocumentSharesPage";

describe("AdminDocumentSharesPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("only shows my shares menu and read-only actions for normal users", async () => {
    const listDocumentShares = vi.fn().mockResolvedValue({
      items: [
        {
          shareId: "share-1",
          documentId: "doc-1",
          documentRouteKey: "doc-route-1",
          documentTitle: "测试文档",
          spaceId: "space-1",
          spaceName: "测试空间",
          spaceOwnerUserId: "owner-1",
          spaceOwnerName: "空间负责人",
          spaceOwnerEmail: "owner@example.com",
          mode: "public",
          passwordHint: "",
          hasPassword: false,
          expiresAt: null,
          accessVersion: 1,
          createdByUserId: "user-1",
          createdByName: "普通用户",
          createdByEmail: "user@example.com",
          updatedByUserId: "user-1",
          createdAt: "2026-04-05T12:00:00.000Z",
          updatedAt: "2026-04-05T12:00:00.000Z",
          isExpired: false
        }
      ],
      pagination: {
        page: 1,
        pageSize: 20,
        total: 1
      }
    } satisfies AdminDocumentShareListResult);

    const dataGateway = {
      admin: {
        listDocumentShares
      }
    } as unknown as DataGateway;

    render(<AdminDocumentSharesPage dataGateway={dataGateway} currentUserRoles={[]} currentUserID="user-1" />);

    await waitFor(() => {
      expect(listDocumentShares).toHaveBeenCalledTimes(1);
    });

    expect(screen.getByRole("button", { name: /我的分享/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /分享管理/ })).toBeNull();
    expect(screen.getByRole("button", { name: "复制链接" })).toHaveClass("rounded-r-none");
    expect(screen.getByLabelText("展开分享操作")).toHaveClass("rounded-l-none");
  });
});
