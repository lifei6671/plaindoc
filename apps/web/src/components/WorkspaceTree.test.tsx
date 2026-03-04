import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  CreateNodeResult,
  DocumentTemplateDetail,
  DocumentTemplateSummary,
  NodeType,
  Visibility
} from "../data-access";
import { WorkspaceTree } from "./WorkspaceTree";

const { toastErrorMock } = vi.hoisted(() => ({
  toastErrorMock: vi.fn()
}));

vi.mock("sonner", () => ({
  toast: {
    error: toastErrorMock
  }
}));

function buildTemplateSummary(input: Partial<DocumentTemplateSummary>): DocumentTemplateSummary {
  return {
    templateId: input.templateId ?? "tpl-default",
    sceneKey: input.sceneKey ?? "default",
    sceneName: input.sceneName ?? "默认场景",
    name: input.name ?? "默认模板",
    description: input.description ?? "",
    defaultTitle: input.defaultTitle ?? "",
    sort: input.sort ?? 0,
    builtin: input.builtin ?? false,
    enabled: input.enabled ?? true,
    updatedAt: input.updatedAt ?? "2026-03-04T00:00:00Z"
  };
}

function buildTemplateDetail(input: Partial<DocumentTemplateDetail>): DocumentTemplateDetail {
  return {
    templateId: input.templateId ?? "tpl-default",
    sceneKey: input.sceneKey ?? "default",
    sceneName: input.sceneName ?? "默认场景",
    name: input.name ?? "默认模板",
    description: input.description ?? "",
    defaultTitle: input.defaultTitle ?? "",
    contentMd: input.contentMd ?? "",
    sort: input.sort ?? 0,
    builtin: input.builtin ?? false,
    enabled: input.enabled ?? true,
    createdByUserId: input.createdByUserId,
    updatedByUserId: input.updatedByUserId,
    createdAt: input.createdAt ?? "2026-03-04T00:00:00Z",
    updatedAt: input.updatedAt ?? "2026-03-04T00:00:00Z"
  };
}

function createProps(overrides?: {
  onCreateNode?: (input: {
    parentId: string | null;
    type: NodeType;
    title: string;
    documentIdentifier?: string;
    templateId?: string;
  }) => Promise<CreateNodeResult>;
  onListDocumentTemplates?: () => Promise<DocumentTemplateSummary[]>;
  onGetDocumentTemplate?: (templateId: string) => Promise<DocumentTemplateDetail>;
}) {
  const onOpenDocument = vi.fn<(docId: string) => Promise<void>>().mockResolvedValue(undefined);
  const onCreateNode =
    overrides?.onCreateNode ??
    vi.fn<
      (input: {
        parentId: string | null;
        type: NodeType;
        title: string;
        documentIdentifier?: string;
        templateId?: string;
      }) => Promise<CreateNodeResult>
    >().mockResolvedValue({
      nodeId: "node-created",
      docId: "doc-created"
    });
  const onListDocumentTemplates =
    overrides?.onListDocumentTemplates ??
    vi
      .fn<() => Promise<DocumentTemplateSummary[]>>()
      .mockResolvedValue([buildTemplateSummary({ templateId: "tpl-default" })]);
  const onGetDocumentTemplate =
    overrides?.onGetDocumentTemplate ??
    vi
      .fn<(templateId: string) => Promise<DocumentTemplateDetail>>()
      .mockResolvedValue(buildTemplateDetail({ templateId: "tpl-default", contentMd: "# default" }));
  const onUpdateDocumentIdentifier = vi
    .fn<(docId: string, identifier: string | null) => Promise<void>>()
    .mockResolvedValue(undefined);
  const onUpdateDocumentVisibility = vi
    .fn<(docId: string, visibility: Visibility) => Promise<void>>()
    .mockResolvedValue(undefined);
  const onRenameNode = vi.fn<(nodeId: string, title: string) => Promise<void>>().mockResolvedValue(undefined);
  const onDeleteNode = vi.fn<(nodeId: string) => Promise<void>>().mockResolvedValue(undefined);
  const onMoveNode = vi
    .fn<(input: { nodeId: string; targetParentId: string | null; targetIndex: number }) => Promise<void>>()
    .mockResolvedValue(undefined);

  return {
    nodes: [],
    activeDocId: null,
    onOpenDocument,
    onCreateNode,
    onListDocumentTemplates,
    onGetDocumentTemplate,
    onUpdateDocumentIdentifier,
    onUpdateDocumentVisibility,
    onRenameNode,
    onDeleteNode,
    onMoveNode
  };
}

describe("WorkspaceTree", () => {
  beforeEach(() => {
    toastErrorMock.mockReset();
  });

  it("loads templates, previews selected template, and forwards templateId on create", async () => {
    const templates: DocumentTemplateSummary[] = [
      buildTemplateSummary({
        templateId: "tpl-weekly",
        sceneKey: "report",
        sceneName: "报告",
        name: "周报模板",
        sort: 20
      }),
      buildTemplateSummary({
        templateId: "tpl-meeting",
        sceneKey: "meeting",
        sceneName: "会议",
        name: "会议纪要模板",
        defaultTitle: "会议纪要",
        sort: 10
      })
    ];
    const details: Record<string, DocumentTemplateDetail> = {
      "tpl-meeting": buildTemplateDetail({
        templateId: "tpl-meeting",
        sceneKey: "meeting",
        sceneName: "会议",
        name: "会议纪要模板",
        defaultTitle: "会议纪要",
        contentMd: "# 会议目标\n\n- 事项 A"
      }),
      "tpl-weekly": buildTemplateDetail({
        templateId: "tpl-weekly",
        sceneKey: "report",
        sceneName: "报告",
        name: "周报模板",
        contentMd: "# 本周进展"
      })
    };

    const onListDocumentTemplates = vi.fn<() => Promise<DocumentTemplateSummary[]>>().mockResolvedValue(templates);
    const onGetDocumentTemplate = vi
      .fn<(templateId: string) => Promise<DocumentTemplateDetail>>()
      .mockImplementation(async (templateId: string) => details[templateId]);
    const onCreateNode = vi
      .fn<
        (input: {
          parentId: string | null;
          type: NodeType;
          title: string;
          documentIdentifier?: string;
          templateId?: string;
        }) => Promise<CreateNodeResult>
      >()
      .mockResolvedValue({
        nodeId: "node-new",
        docId: "doc-new"
      });

    const props = createProps({
      onCreateNode,
      onListDocumentTemplates,
      onGetDocumentTemplate
    });
    const user = userEvent.setup();
    render(<WorkspaceTree {...props} />);

    await user.click(screen.getByRole("button", { name: "新建第一篇文档" }));

    await screen.findByRole("heading", { name: "新建文档" });
    await waitFor(() => {
      expect(onListDocumentTemplates).toHaveBeenCalledTimes(1);
    });
    expect(screen.getByRole("option", { name: "会议纪要模板 (tpl-meeting)" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "周报模板 (tpl-weekly)" })).toBeInTheDocument();

    const templateSelect = screen.getByLabelText("模板（可选）");
    await user.selectOptions(templateSelect, "tpl-meeting");

    await waitFor(() => {
      expect(onGetDocumentTemplate).toHaveBeenCalledWith("tpl-meeting");
    });
    expect(await screen.findByText("会议纪要模板")).toBeInTheDocument();
    expect(screen.getByText("默认标题：会议纪要")).toBeInTheDocument();
    expect(screen.getByText(/会议目标/)).toBeInTheDocument();
    expect(screen.getByText(/事项 A/)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "创建" }));

    await waitFor(() => {
      expect(onCreateNode).toHaveBeenCalledTimes(1);
    });
    expect(onCreateNode).toHaveBeenCalledWith({
      parentId: null,
      type: "doc",
      title: "未命名文档",
      documentIdentifier: undefined,
      templateId: "tpl-meeting"
    });
    await waitFor(() => {
      expect(props.onOpenDocument).toHaveBeenCalledWith("doc-new");
    });
  });

  it("retries preview loading after template detail request fails", async () => {
    const onGetDocumentTemplate = vi
      .fn<(templateId: string) => Promise<DocumentTemplateDetail>>()
      .mockRejectedValueOnce(new Error("network unavailable"))
      .mockResolvedValue(
        buildTemplateDetail({
          templateId: "tpl-postmortem",
          sceneKey: "incident",
          sceneName: "故障复盘",
          name: "复盘模板",
          contentMd: "# 事故时间线"
        })
      );
    const onListDocumentTemplates = vi
      .fn<() => Promise<DocumentTemplateSummary[]>>()
      .mockResolvedValue([
        buildTemplateSummary({
          templateId: "tpl-postmortem",
          sceneKey: "incident",
          sceneName: "故障复盘",
          name: "复盘模板"
        })
      ]);

    const props = createProps({
      onListDocumentTemplates,
      onGetDocumentTemplate
    });
    const user = userEvent.setup();
    render(<WorkspaceTree {...props} />);

    await user.click(screen.getByRole("button", { name: "新建第一篇文档" }));
    await screen.findByRole("heading", { name: "新建文档" });

    await user.selectOptions(screen.getByLabelText("模板（可选）"), "tpl-postmortem");

    expect(await screen.findByText("模板预览加载失败：network unavailable")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "重试" }));

    await waitFor(() => {
      expect(onGetDocumentTemplate).toHaveBeenCalledTimes(2);
    });
    expect(await screen.findByText("复盘模板")).toBeInTheDocument();
    expect(screen.getByText("# 事故时间线")).toBeInTheDocument();
  });
});
