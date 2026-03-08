import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { DataGateway } from "../../data-access";
import { AdminDocumentsPage } from "./AdminDocumentsPage";

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

describe("AdminDocumentsPage", () => {
  beforeEach(() => {
    confirmMock.mockReset();
    promptMock.mockReset();
    showToastMock.mockReset();
  });

  it("renders document format badge and forwards format filter", async () => {
    const listDocuments = vi
      .fn()
      .mockResolvedValueOnce({
        items: [
          {
            documentId: "doc-word-1",
            documentRouteKey: "doc-word-1",
            nodeId: "node-word-1",
            format: "docx",
            title: "合同文档",
            spaceId: "space-1",
            spaceName: "销售空间",
            spaceOwnerUserId: "owner-1",
            spaceOwnerName: "张三",
            spaceOwnerEmail: "zhangsan@example.com",
            visibility: "public",
            status: "active",
            bannedReason: "",
            bannedAt: null,
            deletedAt: null,
            createdAt: "2026-03-07T00:00:00Z",
            updatedAt: "2026-03-07T00:00:00Z"
          }
        ],
        pagination: { page: 1, pageSize: 20, total: 1 }
      })
      .mockResolvedValueOnce({
        items: [],
        pagination: { page: 1, pageSize: 20, total: 0 }
      });

    const dataGateway = {
      admin: {
        listDocuments
      }
    } as unknown as DataGateway;

    render(<AdminDocumentsPage dataGateway={dataGateway} />);

    const titleLink = await screen.findByText("合同文档");
    expect(titleLink).toBeInTheDocument();
    const row = titleLink.closest("tr");
    if (!row) {
      throw new Error("document row not found");
    }
    expect(within(row).getByText("Word")).toBeInTheDocument();

    const formatField = screen.getByText("格式").closest("label");
    if (!formatField) {
      throw new Error("format filter field not found");
    }

    const nativeSelect = formatField.querySelector("select");
    if (!(nativeSelect instanceof HTMLSelectElement)) {
      throw new Error("format native select not found");
    }
    fireEvent.change(nativeSelect, { target: { value: "xlsx" } });

    await waitFor(() => {
      expect(listDocuments).toHaveBeenLastCalledWith(
        expect.objectContaining({
          format: "xlsx"
        })
      );
    });
  });
});
