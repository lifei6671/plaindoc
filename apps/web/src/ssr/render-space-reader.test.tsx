import { describe, expect, it } from "vitest";
import { renderSpaceReader } from "./render-space-reader";
import type { ReaderPagePayload } from "./ssr-types";

function createPayload(
  format: "markdown" | "docx" | "xlsx",
  options?: {
    shareEnabled?: boolean;
    officeRendering?: ReaderPagePayload["officeRendering"];
    officeHtml?: string;
    officeRenderStatus?: "idle" | "pending" | "success" | "failed";
    officeRenderError?: string;
  }
): ReaderPagePayload {
  const shareEnabled = options?.shareEnabled === true;
  return {
    space: {
      id: "space-reader-test",
      name: "测试空间",
      title: "测试文档"
    },
    document: {
      id: "doc-reader-test",
      nodeId: "node-reader-test",
      routeKey: "doc-reader-test",
      themeId: "default",
      format,
      visibility: "public",
      title: format === "xlsx" ? "预算表" : "项目说明",
      contentMd: format === "markdown" ? "# 标题\n\n正文" : options?.officeHtml ?? "",
      renderStatus: format === "markdown" ? undefined : options?.officeRenderStatus ?? "idle",
      renderError: format === "markdown" ? undefined : options?.officeRenderError ?? "",
      version: 1,
      sourceBlobId: format === "markdown" ? undefined : "blob-reader-test",
      sourceFileName: format === "xlsx" ? "预算表.xlsx" : format === "docx" ? "项目说明.docx" : undefined,
      sourceMimeType:
        format === "xlsx"
          ? "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
          : format === "docx"
            ? "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
            : undefined,
      contentVersion: 1,
      authorNickname: "测试作者",
      updatedAt: "2026-03-06T08:00:00Z"
    },
    share: shareEnabled
      ? {
          enabled: true,
          shareId: "share-reader-test",
          spaceId: "space-reader-test",
          documentRouteKey: "doc-reader-test",
          basePath: "/s/space-reader-test",
          attachmentBasePath: "/api/shares/space-reader-test/doc-reader-test/attachments"
        }
      : undefined,
    attachments: [],
    tree: [],
    activeDocId: "doc-reader-test",
    officeRendering: options?.officeRendering,
    viewer: {
      authenticated: false
    }
  };
}

describe("renderSpaceReader", () => {
  it("renders office reader shell with noindex and without markdown export actions", () => {
    const result = renderSpaceReader(createPayload("docx"));

    expect(result.html).toContain('meta name="robots" content="noindex, nofollow"');
    expect(result.html).toContain('data-reader-office-editor="1"');
    expect(result.html).toContain('data-reader-office-download="1"');
    expect(result.html).not.toContain('data-reader-export-action="markdown"');
    expect(result.html).not.toContain('data-reader-export-action="pdf"');
  });

  it("keeps markdown export actions for markdown documents", () => {
    const result = renderSpaceReader(createPayload("markdown"));

    expect(result.html).not.toContain('meta name="robots" content="noindex, nofollow"');
    expect(result.html).toContain('data-reader-export-action="markdown"');
    expect(result.html).toContain('data-reader-export-action="pdf"');
    expect(result.html).not.toContain('data-reader-office-editor="1"');
  });

  it("renders share page with office read-only shell and no markdown export actions", () => {
    const result = renderSpaceReader(createPayload("xlsx", { shareEnabled: true }));

    expect(result.html).toContain('meta name="robots" content="noindex, nofollow"');
    expect(result.html).toContain('data-reader-office-editor="1"');
    expect(result.html).not.toContain('data-reader-export-action="markdown"');
    expect(result.html).not.toContain('data-reader-export-action="pdf"');
  });

  it("renders local office html when independent rendering is enabled and succeeded", () => {
    const result = renderSpaceReader(
      createPayload("docx", {
        officeRendering: {
          independentRenderEnabled: true,
          fallbackToOnlyOfficeOnRenderFailure: true
        },
        officeRenderStatus: "success",
        officeHtml: "<h1>转换结果</h1><p>Office HTML</p>"
      })
    );

    expect(result.html).toContain("转换结果");
    expect(result.html).toContain("Office HTML");
    expect(result.html).not.toContain('data-reader-office-editor="1"');
  });

  it("renders office failure state when independent rendering fails without fallback", () => {
    const result = renderSpaceReader(
      createPayload("docx", {
        officeRendering: {
          independentRenderEnabled: true,
          fallbackToOnlyOfficeOnRenderFailure: false
        },
        officeRenderStatus: "failed",
        officeRenderError: "render failed"
      })
    );

    expect(result.html).toContain('data-reader-office-status="failed"');
    expect(result.html).toContain("render failed");
    expect(result.html).not.toContain('data-reader-office-editor="1"');
  });

  it("renders copy button for fenced code blocks", () => {
    const payload = createPayload("markdown");
    payload.document.contentMd = "```ts\nconst answer = 42;\n```";

    const result = renderSpaceReader(payload);

    expect(result.html).toContain('data-code-copy-button="1"');
    expect(result.html).toContain('data-code-copy-source="1"');
    expect(result.html).toContain("复制成功");
  });
});
