import { LoaderCircle, PencilLine, Plus, RefreshCw, Search, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEventHandler } from "react";
import { Input } from "../../components/ui/input";
import { showToast } from "../../components/ui/toast";
import { type AdminDocumentTemplate, type AdminDocumentTemplateListResult, type DataGateway } from "../../data-access";
import { formatError } from "../../editor/status-utils";
import { useAdminDialogs } from "../components/AdminDialogs";
import { AdminPageCard, AdminTableContainer, AdminToolbarActions } from "../components/AdminPageLayout";

interface AdminDocumentTemplatesPageProps {
  dataGateway: DataGateway;
}

function formatDateTime(value: string | null): string {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return date.toLocaleString("zh-CN", { hour12: false });
}

function parseBooleanInput(value: string, fallback: boolean): boolean {
  const normalized = value.trim().toLowerCase();
  if (normalized === "true" || normalized === "1") {
    return true;
  }
  if (normalized === "false" || normalized === "0") {
    return false;
  }
  return fallback;
}

export function AdminDocumentTemplatesPage({ dataGateway }: AdminDocumentTemplatesPageProps) {
  const { confirm, prompt, dialogs } = useAdminDialogs();

  const [keywordInput, setKeywordInput] = useState("");
  const [keyword, setKeyword] = useState("");
  const [templates, setTemplates] = useState<AdminDocumentTemplateListResult["items"]>([]);
  const [loading, setLoading] = useState(false);
  const [actioningTemplateID, setActioningTemplateID] = useState<string | null>(null);

  const openToast = useCallback((message: string, variant: "success" | "info" | "error" = "error") => {
    showToast(message, variant);
  }, []);

  const loadTemplates = useCallback(async () => {
    setLoading(true);
    try {
      const payload = await dataGateway.admin.listDocumentTemplates({
        keyword,
        page: 1,
        pageSize: 200
      });
      setTemplates(payload.items);
    } catch (error) {
      openToast(`加载文档模板失败：${formatError(error)}`);
      setTemplates([]);
    } finally {
      setLoading(false);
    }
  }, [dataGateway.admin, keyword, openToast]);

  useEffect(() => {
    void loadTemplates();
  }, [loadTemplates]);

  const sortedTemplates = useMemo(() => {
    return [...templates].sort((left, right) => {
      if (left.sceneKey !== right.sceneKey) {
        return left.sceneKey.localeCompare(right.sceneKey);
      }
      if (left.sort !== right.sort) {
        return left.sort - right.sort;
      }
      return left.templateId.localeCompare(right.templateId);
    });
  }, [templates]);

  const runTemplateAction = useCallback(
    async (templateID: string, task: () => Promise<void>) => {
      setActioningTemplateID(templateID);
      try {
        await task();
        await loadTemplates();
      } catch (error) {
        openToast(`操作失败：${formatError(error)}`);
      } finally {
        setActioningTemplateID(null);
      }
    },
    [loadTemplates, openToast]
  );

  const handleSearchSubmit = useCallback<FormEventHandler<HTMLFormElement>>(
    (event) => {
      event.preventDefault();
      setKeyword(keywordInput.trim());
    },
    [keywordInput]
  );

  const handleResetSearch = useCallback(() => {
    setKeywordInput("");
    setKeyword("");
  }, []);

  const handleCreateTemplate = useCallback(async () => {
    const promptResult = await prompt({
      title: "新建文档模板",
      description: "模板 ID 与场景标识仅支持小写字母、数字、下划线和短横线。",
      confirmText: "创建模板",
      fields: [
        { key: "templateId", label: "模板 ID", required: true, defaultValue: "meeting-template" },
        { key: "sceneKey", label: "场景标识", required: true, defaultValue: "meeting" },
        { key: "sceneName", label: "场景名称", required: true, defaultValue: "会议纪要" },
        { key: "name", label: "模板名称", required: true, defaultValue: "会议模板" },
        { key: "description", label: "模板描述", type: "textarea", rows: 3, defaultValue: "" },
        { key: "defaultTitle", label: "默认标题", defaultValue: "" },
        { key: "sort", label: "排序值", defaultValue: "0" },
        {
          key: "enabled",
          label: "启用状态",
          type: "select",
          required: true,
          defaultValue: "true",
          options: [
            { value: "true", label: "启用" },
            { value: "false", label: "停用" }
          ]
        },
        { key: "contentMd", label: "模板内容", type: "textarea", rows: 10, defaultValue: "" }
      ]
    });
    if (!promptResult) {
      return;
    }

    const sort = Number.parseInt(promptResult.sort ?? "0", 10);
    await runTemplateAction((promptResult.templateId ?? "").trim(), async () => {
      await dataGateway.admin.createDocumentTemplate({
        templateId: (promptResult.templateId ?? "").trim(),
        sceneKey: (promptResult.sceneKey ?? "").trim(),
        sceneName: (promptResult.sceneName ?? "").trim(),
        name: (promptResult.name ?? "").trim(),
        description: (promptResult.description ?? "").trim(),
        defaultTitle: (promptResult.defaultTitle ?? "").trim(),
        contentMd: promptResult.contentMd ?? "",
        sort: Number.isFinite(sort) ? sort : 0,
        enabled: parseBooleanInput(promptResult.enabled ?? "true", true)
      });
      openToast("文档模板创建成功", "success");
    });
  }, [dataGateway.admin, openToast, prompt, runTemplateAction]);

  const handleEditTemplate = useCallback(
    async (template: AdminDocumentTemplateListResult["items"][number]) => {
      let detail: AdminDocumentTemplate;
      try {
        detail = await dataGateway.admin.getDocumentTemplate(template.templateId);
      } catch (error) {
        openToast(`加载模板详情失败：${formatError(error)}`);
        return;
      }
      const promptResult = await prompt({
        title: `编辑模板：${detail.name}`,
        description: detail.builtin ? "内置模板不可编辑。" : "更新模板字段并保存。",
        confirmText: "保存修改",
        fields: [
          { key: "sceneKey", label: "场景标识", required: true, defaultValue: detail.sceneKey },
          { key: "sceneName", label: "场景名称", required: true, defaultValue: detail.sceneName },
          { key: "name", label: "模板名称", required: true, defaultValue: detail.name },
          { key: "description", label: "模板描述", type: "textarea", rows: 3, defaultValue: detail.description ?? "" },
          { key: "defaultTitle", label: "默认标题", defaultValue: detail.defaultTitle ?? "" },
          { key: "sort", label: "排序值", defaultValue: String(detail.sort ?? 0) },
          {
            key: "enabled",
            label: "启用状态",
            type: "select",
            required: true,
            defaultValue: detail.enabled ? "true" : "false",
            options: [
              { value: "true", label: "启用" },
              { value: "false", label: "停用" }
            ]
          },
          { key: "contentMd", label: "模板内容", type: "textarea", rows: 10, defaultValue: detail.contentMd ?? "" }
        ]
      });
      if (!promptResult) {
        return;
      }

      const sort = Number.parseInt(promptResult.sort ?? "0", 10);
      await runTemplateAction(detail.templateId, async () => {
        await dataGateway.admin.updateDocumentTemplate({
          templateId: detail.templateId,
          sceneKey: (promptResult.sceneKey ?? "").trim(),
          sceneName: (promptResult.sceneName ?? "").trim(),
          name: (promptResult.name ?? "").trim(),
          description: (promptResult.description ?? "").trim(),
          defaultTitle: (promptResult.defaultTitle ?? "").trim(),
          contentMd: promptResult.contentMd ?? "",
          sort: Number.isFinite(sort) ? sort : 0,
          enabled: parseBooleanInput(promptResult.enabled ?? "true", detail.enabled)
        });
        openToast("文档模板更新成功", "success");
      });
    },
    [dataGateway.admin, openToast, prompt, runTemplateAction]
  );

  const handleDeleteTemplate = useCallback(
    async (template: AdminDocumentTemplateListResult["items"][number]) => {
      if (template.builtin) {
        openToast("内置模板不支持删除");
        return;
      }
      const confirmed = await confirm({
        title: "删除文档模板",
        description: `确认删除模板「${template.name}」吗？该操作不可撤销。`,
        confirmText: "确认删除",
        tone: "danger"
      });
      if (!confirmed) {
        return;
      }
      await runTemplateAction(template.templateId, async () => {
        await dataGateway.admin.deleteDocumentTemplate(template.templateId);
        openToast("文档模板已删除", "success");
      });
    },
    [confirm, dataGateway.admin, openToast, runTemplateAction]
  );

  return (
    <>
      {dialogs}
      <AdminPageCard>
        <div className="flex flex-wrap items-end justify-between gap-3">
          <form className="flex min-w-[260px] flex-1 items-center gap-2" onSubmit={handleSearchSubmit}>
            <label className="sr-only" htmlFor="admin-document-template-search">
              搜索文档模板
            </label>
            <Input
              id="admin-document-template-search"
              value={keywordInput}
              onChange={(event) => setKeywordInput(event.target.value)}
              placeholder="按模板 ID / 名称 / 场景搜索"
              className="h-9"
            />
            <button
              type="submit"
              className="inline-flex h-9 items-center justify-center rounded-sm border border-slate-200 px-3 text-sm text-slate-700 hover:bg-slate-50"
            >
              <Search className="mr-1 h-3.5 w-3.5" />
              搜索
            </button>
            <button
              type="button"
              className="inline-flex h-9 items-center justify-center rounded-sm border border-slate-200 px-3 text-sm text-slate-700 hover:bg-slate-50"
              onClick={handleResetSearch}
            >
              重置
            </button>
          </form>

          <AdminToolbarActions>
            <button
              type="button"
              className="inline-flex h-9 items-center justify-center rounded-sm border border-slate-200 px-3 text-sm text-slate-700 hover:bg-slate-50"
              onClick={() => {
                void loadTemplates();
              }}
              disabled={loading}
            >
              <RefreshCw className={`mr-1 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
              刷新
            </button>
            <button
              type="button"
              className="inline-flex h-9 items-center justify-center rounded-sm bg-slate-900 px-3 text-sm text-white hover:bg-slate-800"
              onClick={() => {
                void handleCreateTemplate();
              }}
            >
              <Plus className="mr-1 h-3.5 w-3.5" />
              新建模板
            </button>
          </AdminToolbarActions>
        </div>

        <AdminTableContainer>
          <table className="min-w-full border-collapse text-left text-sm">
            <thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-3 py-2">模板</th>
                <th className="px-3 py-2">场景</th>
                <th className="px-3 py-2">状态</th>
                <th className="px-3 py-2">排序</th>
                <th className="px-3 py-2">更新时间</th>
                <th className="px-3 py-2 text-right">操作</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr>
                  <td colSpan={6} className="px-3 py-10 text-center text-slate-500">
                    <span className="inline-flex items-center gap-2">
                      <LoaderCircle className="h-4 w-4 animate-spin" />
                      加载中...
                    </span>
                  </td>
                </tr>
              ) : sortedTemplates.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-3 py-10 text-center text-slate-500">
                    暂无模板
                  </td>
                </tr>
              ) : (
                sortedTemplates.map((template) => {
                  const isActioning = actioningTemplateID === template.templateId;
                  return (
                    <tr key={template.templateId} className="border-t border-slate-100">
                      <td className="px-3 py-2 align-top">
                        <p className="font-medium text-slate-800">{template.name}</p>
                        <p className="mt-0.5 text-xs text-slate-500">{template.templateId}</p>
                      </td>
                      <td className="px-3 py-2 align-top">
                        <p className="text-slate-700">{template.sceneName}</p>
                        <p className="mt-0.5 text-xs text-slate-500">{template.sceneKey}</p>
                      </td>
                      <td className="px-3 py-2 align-top">
                        <p className="text-slate-700">{template.enabled ? "启用" : "停用"}</p>
                        <p className="mt-0.5 text-xs text-slate-500">{template.builtin ? "内置" : "自定义"}</p>
                      </td>
                      <td className="px-3 py-2 align-top text-slate-700">{template.sort}</td>
                      <td className="px-3 py-2 align-top text-slate-600">{formatDateTime(template.updatedAt)}</td>
                      <td className="px-3 py-2 align-top">
                        <div className="flex items-center justify-end gap-2">
                          <button
                            type="button"
                            className="inline-flex h-8 items-center rounded-sm border border-slate-200 px-2 text-xs text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
                            disabled={isActioning || template.builtin}
                            onClick={() => {
                              void handleEditTemplate(template);
                            }}
                          >
                            <PencilLine className="mr-1 h-3.5 w-3.5" />
                            编辑
                          </button>
                          <button
                            type="button"
                            className="inline-flex h-8 items-center rounded-sm border border-rose-200 px-2 text-xs text-rose-700 hover:bg-rose-50 disabled:cursor-not-allowed disabled:opacity-60"
                            disabled={isActioning || template.builtin}
                            onClick={() => {
                              void handleDeleteTemplate(template);
                            }}
                          >
                            <Trash2 className="mr-1 h-3.5 w-3.5" />
                            删除
                          </button>
                        </div>
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </AdminTableContainer>
      </AdminPageCard>
    </>
  );
}
