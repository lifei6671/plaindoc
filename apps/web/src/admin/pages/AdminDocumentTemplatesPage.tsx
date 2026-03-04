import { LoaderCircle, PencilLine, Plus, RefreshCw, Search, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEventHandler } from "react";
import { Input } from "../../components/ui/input";
import { showToast } from "../../components/ui/toast";
import {
  type AdminDocumentTemplate,
  type AdminDocumentTemplateListResult,
  type AdminDocumentTemplateScene,
  type AdminDocumentTemplateSceneListResult,
  type DataGateway
} from "../../data-access";
import { formatError } from "../../editor/status-utils";
import { useAdminDialogs } from "../components/AdminDialogs";
import { AdminPageCard, AdminTableContainer } from "../components/AdminPageLayout";

interface AdminDocumentTemplatesPageProps {
  dataGateway: DataGateway;
}

type TemplateManagementMenuKey = "scenes" | "templates";

interface TemplateManagementMenuItem {
  key: TemplateManagementMenuKey;
  label: string;
  description: string;
}

const TEMPLATE_MANAGEMENT_MENUS: TemplateManagementMenuItem[] = [
  {
    key: "scenes",
    label: "场景管理",
    description: "维护模板场景与排序"
  },
  {
    key: "templates",
    label: "模板管理",
    description: "管理模板内容与启停状态"
  }
];

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
  const [sceneKeywordInput, setSceneKeywordInput] = useState("");
  const [sceneKeyword, setSceneKeyword] = useState("");
  const [selectedMenuKey, setSelectedMenuKey] = useState<TemplateManagementMenuKey>("scenes");
  const [templates, setTemplates] = useState<AdminDocumentTemplateListResult["items"]>([]);
  const [scenes, setScenes] = useState<AdminDocumentTemplateSceneListResult["items"]>([]);
  const [loading, setLoading] = useState(false);
  const [sceneLoading, setSceneLoading] = useState(false);
  const [actioningTemplateID, setActioningTemplateID] = useState<string | null>(null);
  const [actioningSceneKey, setActioningSceneKey] = useState<string | null>(null);

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

  const loadScenes = useCallback(async () => {
    setSceneLoading(true);
    try {
      const payload = await dataGateway.admin.listDocumentTemplateScenes({
        keyword: sceneKeyword,
        page: 1,
        pageSize: 200
      });
      setScenes(payload.items);
    } catch (error) {
      openToast(`加载模板场景失败：${formatError(error)}`);
      setScenes([]);
    } finally {
      setSceneLoading(false);
    }
  }, [dataGateway.admin, openToast, sceneKeyword]);

  useEffect(() => {
    void loadTemplates();
  }, [loadTemplates]);

  useEffect(() => {
    void loadScenes();
  }, [loadScenes]);

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

  const sortedScenes = useMemo(() => {
    return [...scenes].sort((left, right) => {
      if (left.sort !== right.sort) {
        return left.sort - right.sort;
      }
      return left.sceneKey.localeCompare(right.sceneKey);
    });
  }, [scenes]);

  const sceneSelectOptions = useMemo(() => {
    return sortedScenes.map((scene) => ({
      value: scene.sceneKey,
      label: `${scene.sceneName} (${scene.sceneKey})`
    }));
  }, [sortedScenes]);

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

  const runSceneAction = useCallback(
    async (sceneKey: string, task: () => Promise<void>) => {
      setActioningSceneKey(sceneKey);
      try {
        await task();
        await Promise.all([loadScenes(), loadTemplates()]);
      } catch (error) {
        openToast(`操作失败：${formatError(error)}`);
      } finally {
        setActioningSceneKey(null);
      }
    },
    [loadScenes, loadTemplates, openToast]
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

  const handleSceneSearchSubmit = useCallback<FormEventHandler<HTMLFormElement>>(
    (event) => {
      event.preventDefault();
      setSceneKeyword(sceneKeywordInput.trim());
    },
    [sceneKeywordInput]
  );

  const handleResetSceneSearch = useCallback(() => {
    setSceneKeywordInput("");
    setSceneKeyword("");
  }, []);

  const handleCreateTemplate = useCallback(async () => {
    if (sceneSelectOptions.length === 0) {
      openToast("请先创建模板场景后再新建模板");
      return;
    }
    const promptResult = await prompt({
      title: "新建文档模板",
      description: "模板 ID 仅支持小写字母、数字、下划线和短横线。",
      confirmText: "创建模板",
      fields: [
        { key: "templateId", label: "模板 ID", required: true, defaultValue: "meeting-template" },
        {
          key: "sceneKey",
          label: "模板场景",
          type: "select",
          required: true,
          defaultValue: sceneSelectOptions[0]?.value ?? "",
          options: sceneSelectOptions
        },
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
        name: (promptResult.name ?? "").trim(),
        description: (promptResult.description ?? "").trim(),
        defaultTitle: (promptResult.defaultTitle ?? "").trim(),
        contentMd: promptResult.contentMd ?? "",
        sort: Number.isFinite(sort) ? sort : 0,
        enabled: parseBooleanInput(promptResult.enabled ?? "true", true)
      });
      openToast("文档模板创建成功", "success");
    });
  }, [dataGateway.admin, openToast, prompt, runTemplateAction, sceneSelectOptions]);

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
          { key: "sceneHint", label: "所属场景", defaultValue: `${detail.sceneName} (${detail.sceneKey})` },
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

  const handleCreateScene = useCallback(async () => {
    const promptResult = await prompt({
      title: "新建模板场景",
      description: "场景标识仅支持小写字母、数字、下划线和短横线。",
      confirmText: "创建场景",
      fields: [
        { key: "sceneKey", label: "场景标识", required: true, defaultValue: "meeting" },
        { key: "sceneName", label: "场景名称", required: true, defaultValue: "会议纪要" },
        { key: "description", label: "场景描述", type: "textarea", rows: 3, defaultValue: "" },
        { key: "sort", label: "排序值", defaultValue: "0" }
      ]
    });
    if (!promptResult) {
      return;
    }

    const sceneKey = (promptResult.sceneKey ?? "").trim();
    const sort = Number.parseInt(promptResult.sort ?? "0", 10);
    await runSceneAction(sceneKey, async () => {
      await dataGateway.admin.createDocumentTemplateScene({
        sceneKey,
        sceneName: (promptResult.sceneName ?? "").trim(),
        description: (promptResult.description ?? "").trim(),
        sort: Number.isFinite(sort) ? sort : 0
      });
      openToast("模板场景创建成功", "success");
    });
  }, [dataGateway.admin, openToast, prompt, runSceneAction]);

  const handleEditScene = useCallback(
    async (scene: AdminDocumentTemplateSceneListResult["items"][number]) => {
      let detail: AdminDocumentTemplateScene;
      try {
        detail = await dataGateway.admin.getDocumentTemplateScene(scene.sceneKey);
      } catch (error) {
        openToast(`加载场景详情失败：${formatError(error)}`);
        return;
      }

      const promptResult = await prompt({
        title: `编辑场景：${detail.sceneName}`,
        description: detail.builtin ? "内置场景不可编辑。" : "更新场景字段并保存。",
        confirmText: "保存修改",
        fields: [
          { key: "sceneHint", label: "场景标识", defaultValue: detail.sceneKey },
          { key: "sceneName", label: "场景名称", required: true, defaultValue: detail.sceneName },
          { key: "description", label: "场景描述", type: "textarea", rows: 3, defaultValue: detail.description ?? "" },
          { key: "sort", label: "排序值", defaultValue: String(detail.sort ?? 0) }
        ]
      });
      if (!promptResult) {
        return;
      }

      const sort = Number.parseInt(promptResult.sort ?? "0", 10);
      await runSceneAction(detail.sceneKey, async () => {
        await dataGateway.admin.updateDocumentTemplateScene({
          sceneKey: detail.sceneKey,
          sceneName: (promptResult.sceneName ?? "").trim(),
          description: (promptResult.description ?? "").trim(),
          sort: Number.isFinite(sort) ? sort : 0
        });
        openToast("模板场景更新成功", "success");
      });
    },
    [dataGateway.admin, openToast, prompt, runSceneAction]
  );

  const handleDeleteScene = useCallback(
    async (scene: AdminDocumentTemplateSceneListResult["items"][number]) => {
      if (scene.builtin) {
        openToast("内置场景不支持删除");
        return;
      }
      if (scene.templateCount > 0) {
        openToast(`场景「${scene.sceneName}」仍有 ${scene.templateCount} 个模板引用，禁止删除`);
        return;
      }

      const confirmed = await confirm({
        title: "删除模板场景",
        description: `确认删除场景「${scene.sceneName}」吗？该操作不可撤销。`,
        confirmText: "确认删除",
        tone: "danger"
      });
      if (!confirmed) {
        return;
      }
      await runSceneAction(scene.sceneKey, async () => {
        await dataGateway.admin.deleteDocumentTemplateScene(scene.sceneKey);
        openToast("模板场景已删除", "success");
      });
    },
    [confirm, dataGateway.admin, openToast, runSceneAction]
  );

  const selectedMenu = useMemo(() => {
    return (
      TEMPLATE_MANAGEMENT_MENUS.find((item) => item.key === selectedMenuKey) ??
      TEMPLATE_MANAGEMENT_MENUS[0]
    );
  }, [selectedMenuKey]);

  const activeLoading = selectedMenuKey === "templates" ? loading : sceneLoading;

  return (
    <>
      {dialogs}
      <AdminPageCard>
        <div className="overflow-hidden rounded-md border border-slate-200 bg-white">
          <div className="grid min-h-[560px] gap-0 md:grid-cols-[240px_minmax(0,1fr)]">
            <aside className="hidden border-r border-slate-200 bg-transparent p-2 md:block">
              <nav className="space-y-1" aria-label="模板管理菜单">
                {TEMPLATE_MANAGEMENT_MENUS.map((menu) => {
                  const isActive = menu.key === selectedMenuKey;
                  return (
                    <button
                      key={menu.key}
                      type="button"
                      aria-label={menu.label}
                      className={`w-full appearance-none rounded-lg border border-transparent px-3 py-2.5 text-left shadow-none outline-none transition focus-visible:ring-2 focus-visible:ring-sky-200 ${
                        isActive ? "bg-slate-200 text-slate-900" : "text-slate-700 hover:bg-slate-200/70"
                      }`}
                      onClick={() => setSelectedMenuKey(menu.key)}
                    >
                      <p className="text-sm font-medium">{menu.label}</p>
                      <p className="mt-0.5 text-xs text-slate-500">{menu.description}</p>
                    </button>
                  );
                })}
              </nav>
            </aside>

            <main className="flex min-h-[560px] flex-col">
              <header className="flex h-16 shrink-0 items-center justify-between gap-3 border-b border-slate-200 px-4">
                <div>
                  <h3 className="text-base font-semibold text-slate-800">{selectedMenu.label}</h3>
                </div>

                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    className="inline-flex h-9 shrink-0 items-center justify-center whitespace-nowrap rounded-sm border border-slate-200 px-3 text-sm text-slate-700 hover:bg-slate-50"
                    onClick={() => {
                      if (selectedMenuKey === "templates") {
                        void loadTemplates();
                        return;
                      }
                      void loadScenes();
                    }}
                    disabled={activeLoading}
                  >
                    <RefreshCw className={`mr-1 h-3.5 w-3.5 ${activeLoading ? "animate-spin" : ""}`} />
                    刷新
                  </button>
                  {selectedMenuKey === "scenes" ? (
                    <button
                      type="button"
                      className="inline-flex h-9 shrink-0 items-center justify-center whitespace-nowrap rounded-sm bg-slate-900 px-3 text-sm text-white hover:bg-slate-800"
                      onClick={() => {
                        void handleCreateScene();
                      }}
                    >
                      <Plus className="mr-1 h-3.5 w-3.5" />
                      新建场景
                    </button>
                  ) : (
                    <button
                      type="button"
                      className="inline-flex h-9 shrink-0 items-center justify-center whitespace-nowrap rounded-sm bg-slate-900 px-3 text-sm text-white hover:bg-slate-800"
                      onClick={() => {
                        void handleCreateTemplate();
                      }}
                    >
                      <Plus className="mr-1 h-3.5 w-3.5" />
                      新建模板
                    </button>
                  )}
                </div>
              </header>

              <div className="flex-1 space-y-4 p-4">
                {selectedMenuKey === "templates" ? (
                  <>
                    <form className="flex min-w-[260px] items-center gap-2" onSubmit={handleSearchSubmit}>
                      <label className="sr-only" htmlFor="admin-document-template-search">
                        搜索文档模板
                      </label>
                      <Input
                        id="admin-document-template-search"
                        value={keywordInput}
                        onChange={(event) => setKeywordInput(event.target.value)}
                        placeholder="按模板 ID / 名称 / 场景搜索"
                        className="h-9 min-w-0 flex-1"
                      />
                      <button
                        type="submit"
                        className="inline-flex h-9 shrink-0 items-center justify-center whitespace-nowrap rounded-sm border border-slate-200 px-3 text-sm text-slate-700 hover:bg-slate-50"
                      >
                        <Search className="mr-1 h-3.5 w-3.5" />
                        搜索
                      </button>
                      <button
                        type="button"
                        className="inline-flex h-9 shrink-0 items-center justify-center whitespace-nowrap rounded-sm border border-slate-200 px-3 text-sm text-slate-700 hover:bg-slate-50"
                        onClick={handleResetSearch}
                      >
                        重置
                      </button>
                    </form>

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
                  </>
                ) : (
                  <>
                    <form className="flex min-w-[260px] items-center gap-2" onSubmit={handleSceneSearchSubmit}>
                      <label className="sr-only" htmlFor="admin-document-template-scene-search">
                        搜索模板场景
                      </label>
                      <Input
                        id="admin-document-template-scene-search"
                        value={sceneKeywordInput}
                        onChange={(event) => setSceneKeywordInput(event.target.value)}
                        placeholder="按场景标识 / 场景名称搜索"
                        className="h-9 min-w-0 flex-1"
                      />
                      <button
                        type="submit"
                        className="inline-flex h-9 shrink-0 items-center justify-center whitespace-nowrap rounded-sm border border-slate-200 px-3 text-sm text-slate-700 hover:bg-slate-50"
                      >
                        <Search className="mr-1 h-3.5 w-3.5" />
                        搜索
                      </button>
                      <button
                        type="button"
                        className="inline-flex h-9 shrink-0 items-center justify-center whitespace-nowrap rounded-sm border border-slate-200 px-3 text-sm text-slate-700 hover:bg-slate-50"
                        onClick={handleResetSceneSearch}
                      >
                        重置
                      </button>
                    </form>

                    <p className="text-xs text-slate-500">模板创建时必须先选择已维护的场景，避免重复命名。</p>

                    <AdminTableContainer>
                      <table className="min-w-full border-collapse text-left text-sm">
                        <thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
                          <tr>
                            <th className="px-3 py-2">场景</th>
                            <th className="px-3 py-2">描述</th>
                            <th className="px-3 py-2">排序</th>
                            <th className="px-3 py-2">引用模板数</th>
                            <th className="px-3 py-2">更新时间</th>
                            <th className="px-3 py-2 text-right">操作</th>
                          </tr>
                        </thead>
                        <tbody>
                          {sceneLoading ? (
                            <tr>
                              <td colSpan={6} className="px-3 py-10 text-center text-slate-500">
                                <span className="inline-flex items-center gap-2">
                                  <LoaderCircle className="h-4 w-4 animate-spin" />
                                  加载中...
                                </span>
                              </td>
                            </tr>
                          ) : sortedScenes.length === 0 ? (
                            <tr>
                              <td colSpan={6} className="px-3 py-10 text-center text-slate-500">
                                暂无场景
                              </td>
                            </tr>
                          ) : (
                            sortedScenes.map((scene) => {
                              const actioning = actioningSceneKey === scene.sceneKey;
                              const deleteDisabled = actioning || scene.builtin || scene.templateCount > 0;
                              return (
                                <tr key={scene.sceneKey} className="border-t border-slate-100">
                                  <td className="px-3 py-2 align-top">
                                    <p className="font-medium text-slate-800">{scene.sceneName}</p>
                                    <p className="mt-0.5 text-xs text-slate-500">{scene.sceneKey}</p>
                                  </td>
                                  <td className="px-3 py-2 align-top text-slate-700">{scene.description || "-"}</td>
                                  <td className="px-3 py-2 align-top text-slate-700">{scene.sort}</td>
                                  <td className="px-3 py-2 align-top text-slate-700">{scene.templateCount}</td>
                                  <td className="px-3 py-2 align-top text-slate-600">{formatDateTime(scene.updatedAt)}</td>
                                  <td className="px-3 py-2 align-top">
                                    <div className="flex items-center justify-end gap-2">
                                      <button
                                        type="button"
                                        className="inline-flex h-8 items-center rounded-sm border border-slate-200 px-2 text-xs text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
                                        disabled={actioning || scene.builtin}
                                        onClick={() => {
                                          void handleEditScene(scene);
                                        }}
                                      >
                                        <PencilLine className="mr-1 h-3.5 w-3.5" />
                                        编辑
                                      </button>
                                      <button
                                        type="button"
                                        className="inline-flex h-8 items-center rounded-sm border border-rose-200 px-2 text-xs text-rose-700 hover:bg-rose-50 disabled:cursor-not-allowed disabled:opacity-60"
                                        disabled={deleteDisabled}
                                        onClick={() => {
                                          void handleDeleteScene(scene);
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
                  </>
                )}
              </div>
            </main>
          </div>
        </div>
      </AdminPageCard>
    </>
  );
}
