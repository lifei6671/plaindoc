import { LoaderCircle, PencilLine, Plus, Power, PowerOff, RefreshCw, Search, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEventHandler } from "react";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { showToast } from "../../components/ui/toast";
import { type AdminTheme, type DataGateway } from "../../data-access";
import { useAdminDialogs } from "../components/AdminDialogs";
import { AdminPageCard, AdminTableContainer, AdminToolbarActions } from "../components/AdminPageLayout";
import { formatError } from "../../editor/status-utils";

interface AdminThemesPageProps {
  dataGateway: DataGateway;
}

function normalizeSyntaxTheme(value: string | null | undefined): "one-light" | "one-dark" {
  const normalized = (value ?? "").trim().toLowerCase();
  if (
    normalized === "one-dark" ||
    normalized === "one_dark" ||
    normalized === "dark" ||
    normalized.includes("dark")
  ) {
    return "one-dark";
  }
  return "one-light";
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

function renderSyntaxLabel(value: AdminTheme["syntaxTheme"]): string {
  if (normalizeSyntaxTheme(value) === "one-dark") {
    return "深色语法";
  }
  return "浅色语法";
}

function renderSyntaxBadgeClass(value: AdminTheme["syntaxTheme"]): string {
  if (normalizeSyntaxTheme(value) === "one-dark") {
    return "border-indigo-200 bg-indigo-50 text-indigo-700";
  }
  return "border-sky-200 bg-sky-50 text-sky-700";
}

export function AdminThemesPage({ dataGateway }: AdminThemesPageProps) {
  const { confirm, prompt, dialogs } = useAdminDialogs();

  const [keywordInput, setKeywordInput] = useState("");
  const [keyword, setKeyword] = useState("");

  const [themes, setThemes] = useState<AdminTheme[]>([]);
  const [loading, setLoading] = useState(false);

  const [actioningThemeID, setActioningThemeID] = useState<string | null>(null);

  const openToast = useCallback((message: string, variant: "success" | "info" | "error" = "error") => {
    showToast(message, variant);
  }, []);

  const loadThemes = useCallback(async () => {
    setLoading(true);
    try {
      const payload = await dataGateway.admin.listThemes();
      setThemes(payload);
    } catch (error) {
      openToast(`加载主题列表失败：${formatError(error)}`);
      setThemes([]);
    } finally {
      setLoading(false);
    }
  }, [dataGateway, openToast]);

  useEffect(() => {
    void loadThemes();
  }, [loadThemes]);

  const filteredThemes = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLowerCase();
    if (!normalizedKeyword) {
      return themes;
    }
    return themes.filter((item) =>
      [item.themeId, item.name, item.description].some((value) =>
        value.toLowerCase().includes(normalizedKeyword)
      )
    );
  }, [keyword, themes]);

  const runThemeAction = useCallback(
    async (themeID: string, callback: () => Promise<void>) => {
      setActioningThemeID(themeID);
      try {
        await callback();
        await loadThemes();
      } catch (error) {
        openToast(`执行操作失败：${formatError(error)}`);
      } finally {
        setActioningThemeID(null);
      }
    },
    [loadThemes, openToast]
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

  const handleCreateTheme = useCallback(async () => {
    const promptResult = await prompt({
      title: "新建主题",
      description: "创建后可继续编辑变量和自定义 CSS。",
      confirmText: "创建主题",
      fields: [
        {
          key: "themeId",
          label: "主题 ID",
          required: true,
          defaultValue: "custom_theme",
          placeholder: "小写字母/数字/_/-"
        },
        {
          key: "name",
          label: "主题名称",
          required: true,
          defaultValue: "自定义主题"
        },
        {
          key: "description",
          label: "主题描述",
          type: "textarea",
          rows: 4,
          defaultValue: ""
        },
        {
          key: "syntaxTheme",
          label: "语法主题",
          type: "select",
          required: true,
          defaultValue: "one-light",
          options: [
            { value: "one-light", label: "one-light（浅色）" },
            { value: "one-dark", label: "one-dark（深色）" }
          ]
        }
      ]
    });
    if (!promptResult) {
      return;
    }

    const themeID = (promptResult.themeId ?? "").trim().toLowerCase();
    if (!themeID) {
      openToast("主题 ID 不能为空");
      return;
    }
    const themeName = (promptResult.name ?? "").trim();
    if (!themeName) {
      openToast("主题名称不能为空");
      return;
    }
    const syntaxTheme = (promptResult.syntaxTheme ?? "one-light").trim() === "one-dark" ? "one-dark" : "one-light";

    await runThemeAction(themeID, async () => {
      await dataGateway.admin.createTheme({
        themeId: themeID,
        name: themeName,
        description: (promptResult.description ?? "").trim(),
        syntaxTheme,
        variables: {},
        codeBlockStyle: {},
        codeBlockCodeStyle: {},
        inlineCodeStyle: {},
        customCss: "",
        enabled: true
      });
    });
  }, [dataGateway.admin, openToast, prompt, runThemeAction]);

  const handleEditTheme = useCallback(
    async (theme: AdminTheme) => {
      if (theme.builtin) {
        openToast("内置主题不支持修改");
        return;
      }

      const promptResult = await prompt({
        title: `编辑主题：${theme.name}`,
        description: "可修改名称、描述、语法主题与自定义 CSS。",
        confirmText: "保存修改",
        fields: [
          {
            key: "name",
            label: "主题名称",
            required: true,
            defaultValue: theme.name
          },
          {
            key: "description",
            label: "主题描述",
            type: "textarea",
            rows: 4,
            defaultValue: theme.description ?? ""
          },
          {
            key: "syntaxTheme",
            label: "语法主题",
            type: "select",
            required: true,
            defaultValue: normalizeSyntaxTheme(theme.syntaxTheme),
            options: [
              { value: "one-light", label: "one-light（浅色）" },
              { value: "one-dark", label: "one-dark（深色）" }
            ]
          },
          {
            key: "customCss",
            label: "自定义 CSS",
            type: "textarea",
            rows: 8,
            defaultValue: theme.customCss || ""
          }
        ]
      });
      if (!promptResult) {
        return;
      }
      const themeName = (promptResult.name ?? "").trim();
      if (!themeName) {
        openToast("主题名称不能为空");
        return;
      }
      const normalizedSyntaxTheme = (promptResult.syntaxTheme ?? "").trim();
      if (normalizedSyntaxTheme !== "one-light" && normalizedSyntaxTheme !== "one-dark") {
        openToast("语法主题仅支持 one-light / one-dark");
        return;
      }

      await runThemeAction(theme.themeId, async () => {
        await dataGateway.admin.updateTheme({
          themeId: theme.themeId,
          name: themeName,
          description: promptResult.description ?? "",
          syntaxTheme: normalizedSyntaxTheme,
          customCss: promptResult.customCss ?? ""
        });
      });
    },
    [dataGateway.admin, openToast, prompt, runThemeAction]
  );

  const handleToggleTheme = useCallback(
    async (theme: AdminTheme) => {
      if (theme.builtin) {
        openToast("内置主题不支持启停");
        return;
      }

      const nextEnabled = !theme.enabled;
      const actionLabel = nextEnabled ? "启用" : "停用";
      const confirmed = await confirm({
        title: `${actionLabel}主题：${theme.name}`,
        description: `确认后将${actionLabel}此主题。`,
        confirmText: `确认${actionLabel}`,
        tone: nextEnabled ? "default" : "warning"
      });
      if (!confirmed) {
        return;
      }

      await runThemeAction(theme.themeId, async () => {
        await dataGateway.admin.updateTheme({
          themeId: theme.themeId,
          enabled: nextEnabled
        });
      });
    },
    [confirm, dataGateway.admin, openToast, runThemeAction]
  );

  const handleDeleteTheme = useCallback(
    async (theme: AdminTheme) => {
      if (theme.builtin) {
        openToast("内置主题不支持删除");
        return;
      }
      const confirmed = await confirm({
        title: `删除主题：${theme.name}`,
        description: "确认后主题将被删除，已引用位置请提前确认。",
        confirmText: "确认删除",
        tone: "danger"
      });
      if (!confirmed) {
        return;
      }
      await runThemeAction(theme.themeId, async () => {
        await dataGateway.admin.deleteTheme(theme.themeId);
      });
    },
    [confirm, dataGateway.admin, openToast, runThemeAction]
  );

  return (
    <section aria-label="主题管理">
      {dialogs}
      <AdminPageCard>
          <form className="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto]" onSubmit={handleSearchSubmit}>
            <label className="space-y-1.5">
              <span className="text-xs font-semibold tracking-wide text-slate-600">关键词</span>
              <Input
                type="search"
                value={keywordInput}
                placeholder="主题 ID / 名称 / 描述"
                onChange={(event) => setKeywordInput(event.target.value)}
              />
            </label>
            <AdminToolbarActions>
                <Button type="submit" disabled={loading}>
                  <Search size={14} />
                  <span>查询</span>
                </Button>
                <Button type="button" variant="outline" disabled={loading} onClick={handleResetSearch}>
                  重置
                </Button>
                <Button type="button" variant="outline" disabled={loading} onClick={() => void loadThemes()}>
                  <RefreshCw size={14} />
                  <span>刷新</span>
                </Button>
                <Button type="button" onClick={() => void handleCreateTheme()} disabled={loading}>
                  <Plus size={14} />
                  <span>新建主题</span>
                </Button>
            </AdminToolbarActions>
          </form>

          <AdminTableContainer>
              <table className="w-full min-w-[980px] border-collapse text-left text-sm">
                <thead className="sticky top-0 z-10 bg-slate-50/95 backdrop-blur">
                  <tr className="text-xs uppercase tracking-wide text-slate-600">
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">主题</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">语法</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">类型/状态</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">更新时间</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">操作</th>
                  </tr>
                </thead>
                <tbody>
                  {loading ? (
                    <tr>
                      <td colSpan={5} className="px-3 py-12">
                        <div className="flex items-center justify-center gap-2 text-sm text-slate-500">
                          <LoaderCircle size={15} className="animate-spin" />
                          <span>正在加载主题列表...</span>
                        </div>
                      </td>
                    </tr>
                  ) : filteredThemes.length === 0 ? (
                    <tr>
                      <td colSpan={5} className="px-3 py-12 text-center text-sm text-slate-500">
                        暂无符合条件的数据
                      </td>
                    </tr>
                  ) : (
                    filteredThemes.map((theme) => {
                      const isActioning = actioningThemeID === theme.themeId;
                      return (
                        <tr key={theme.themeId} className="border-b border-slate-100 align-top text-slate-700">
                          <td className="px-3 py-3">
                            <div className="grid gap-1">
                              <strong className="text-sm font-semibold text-slate-900">{theme.name}</strong>
                              <code className="w-fit rounded border border-sky-200 bg-sky-50 px-1.5 py-0.5 text-xs text-sky-700">
                                {theme.themeId}
                              </code>
                              <span className="text-xs text-slate-600">{theme.description || "-"}</span>
                            </div>
                          </td>
                          <td className="px-3 py-3">
                            <Badge variant="outline" className={renderSyntaxBadgeClass(theme.syntaxTheme)}>
                              {renderSyntaxLabel(theme.syntaxTheme)}
                            </Badge>
                          </td>
                          <td className="px-3 py-3">
                            <div className="flex flex-wrap items-center gap-2">
                              <Badge
                                variant="outline"
                                className={theme.builtin ? "border-purple-200 bg-purple-50 text-purple-700" : "border-slate-200 bg-slate-100 text-slate-700"}
                              >
                                {theme.builtin ? "内置" : "自定义"}
                              </Badge>
                              <Badge
                                variant="outline"
                                className={theme.enabled ? "border-emerald-200 bg-emerald-50 text-emerald-700" : "border-slate-200 bg-slate-100 text-slate-600"}
                              >
                                {theme.enabled ? "已启用" : "已停用"}
                              </Badge>
                            </div>
                          </td>
                          <td className="px-3 py-3 text-xs text-slate-600">{formatDateTime(theme.updatedAt)}</td>
                          <td className="px-3 py-3">
                            <div className="flex flex-wrap items-center gap-2">
                              {theme.builtin ? null : (
                                <Button type="button" size="sm" variant="outline" disabled={isActioning} onClick={() => void handleEditTheme(theme)}>
                                  <PencilLine size={13} />
                                  <span>编辑</span>
                                </Button>
                              )}
                              {theme.builtin ? null : (
                                <>
                                  <Button
                                    type="button"
                                    size="sm"
                                    variant="outline"
                                    className={
                                      theme.enabled
                                        ? "border-amber-200 bg-amber-50 text-amber-700 hover:bg-amber-100"
                                        : "border-emerald-200 bg-emerald-50 text-emerald-700 hover:bg-emerald-100"
                                    }
                                    disabled={isActioning}
                                    onClick={() => void handleToggleTheme(theme)}
                                  >
                                    {theme.enabled ? <PowerOff size={13} /> : <Power size={13} />}
                                    <span>{theme.enabled ? "停用" : "启用"}</span>
                                  </Button>
                                  <Button type="button" size="sm" variant="destructive" disabled={isActioning} onClick={() => void handleDeleteTheme(theme)}>
                                    <Trash2 size={13} />
                                    <span>删除</span>
                                  </Button>
                                </>
                              )}
                            </div>
                          </td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>
          </AdminTableContainer>

          <footer className="text-xs text-slate-600">共 {filteredThemes.length} 条（总计 {themes.length} 条）</footer>
      </AdminPageCard>
    </section>
  );
}
