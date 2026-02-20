import { LoaderCircle, PencilLine, Plus, Power, PowerOff, RefreshCw, Search, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { type AdminTheme, type DataGateway } from "../../data-access";
import { useAdminDialogs } from "../components/AdminDialogs";
import { TopToast, type TopToastVariant } from "../../components/TopToast";
import { formatError } from "../../editor/status-utils";

interface AdminThemesPageProps {
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

function renderSyntaxLabel(value: AdminTheme["syntaxTheme"]): string {
  if (value === "one-dark") {
    return "深色语法";
  }
  return "浅色语法";
}

export function AdminThemesPage({ dataGateway }: AdminThemesPageProps) {
  const { confirm, prompt, dialogs } = useAdminDialogs();

  const [keywordInput, setKeywordInput] = useState("");
  const [keyword, setKeyword] = useState("");

  const [themes, setThemes] = useState<AdminTheme[]>([]);
  const [loading, setLoading] = useState(false);

  const [actioningThemeID, setActioningThemeID] = useState<string | null>(null);
  const [toastState, setToastState] = useState<{
    open: boolean;
    message: string;
    variant: TopToastVariant;
    triggerKey: number;
  }>({
    open: false,
    message: "",
    variant: "error",
    triggerKey: 0
  });

  const openToast = useCallback((message: string, variant: TopToastVariant = "error") => {
    const normalizedMessage = message.trim();
    if (!normalizedMessage) {
      return;
    }
    setToastState((previousState) => ({
      open: true,
      message: normalizedMessage,
      variant,
      triggerKey: previousState.triggerKey + 1
    }));
  }, []);

  const closeToast = useCallback(() => {
    setToastState((previousState) => {
      if (!previousState.open) {
        return previousState;
      }
      return {
        ...previousState,
        open: false
      };
    });
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

  const handleSearchSubmit = useCallback(
    (event: FormEvent<HTMLFormElement>) => {
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
            defaultValue: theme.syntaxTheme,
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
    <section className="admin-spaces-panel" aria-label="主题管理">
      <TopToast
        open={toastState.open}
        message={toastState.message}
        variant={toastState.variant}
        triggerKey={toastState.triggerKey}
        durationMs={2800}
        onClose={closeToast}
      />
      {dialogs}
      <form className="admin-spaces-toolbar admin-themes-toolbar" onSubmit={handleSearchSubmit}>
        <label className="admin-spaces-toolbar__field">
          <span>关键词</span>
          <input
            type="search"
            value={keywordInput}
            placeholder="主题 ID / 名称 / 描述"
            onChange={(event) => setKeywordInput(event.target.value)}
          />
        </label>
        <div className="admin-spaces-toolbar__actions">
          <button type="submit" disabled={loading}>
            <Search size={14} />
            <span>查询</span>
          </button>
          <button type="button" disabled={loading} onClick={handleResetSearch}>
            重置
          </button>
          <button type="button" disabled={loading} onClick={() => void loadThemes()}>
            <RefreshCw size={14} />
            <span>刷新</span>
          </button>
          <button type="button" disabled={loading} onClick={() => void handleCreateTheme()}>
            <Plus size={14} />
            <span>新建主题</span>
          </button>
        </div>
      </form>

      <div className="admin-spaces-table-wrap">
        <table className="admin-spaces-table admin-themes-table">
          <thead>
            <tr>
              <th>主题</th>
              <th>语法</th>
              <th>类型/状态</th>
              <th>更新时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={5}>
                  <div className="admin-spaces-loading-row">
                    <LoaderCircle size={15} className="admin-spaces-loading-row__icon" />
                    <span>正在加载主题列表...</span>
                  </div>
                </td>
              </tr>
            ) : filteredThemes.length === 0 ? (
              <tr>
                <td colSpan={5}>
                  <div className="admin-spaces-empty-row">暂无符合条件的数据</div>
                </td>
              </tr>
            ) : (
              filteredThemes.map((theme) => {
                const isActioning = actioningThemeID === theme.themeId;
                return (
                  <tr key={theme.themeId}>
                    <td>
                      <div className="admin-theme-cell">
                        <strong>{theme.name}</strong>
                        <code>{theme.themeId}</code>
                        <span>{theme.description || "-"}</span>
                      </div>
                    </td>
                    <td>
                      <span className="admin-spaces-visibility">{renderSyntaxLabel(theme.syntaxTheme)}</span>
                    </td>
                    <td>
                      <div className="admin-theme-status-cell">
                        <span className={`admin-theme-kind ${theme.builtin ? "admin-theme-kind--builtin" : "admin-theme-kind--custom"}`}>
                          {theme.builtin ? "内置" : "自定义"}
                        </span>
                        <span className={`admin-spaces-status ${theme.enabled ? "admin-spaces-status--active" : "admin-spaces-status--deleted"}`}>
                          {theme.enabled ? "已启用" : "已停用"}
                        </span>
                      </div>
                    </td>
                    <td>{formatDateTime(theme.updatedAt)}</td>
                    <td>
                      <div className="admin-spaces-actions">
                        <button
                          type="button"
                          disabled={isActioning}
                          onClick={() => void handleEditTheme(theme)}
                        >
                          <PencilLine size={13} />
                          <span>编辑</span>
                        </button>
                        <button
                          type="button"
                          disabled={isActioning}
                          className="warning"
                          onClick={() => void handleToggleTheme(theme)}
                        >
                          {theme.enabled ? <PowerOff size={13} /> : <Power size={13} />}
                          <span>{theme.enabled ? "停用" : "启用"}</span>
                        </button>
                        <button
                          type="button"
                          className="danger"
                          disabled={isActioning}
                          onClick={() => void handleDeleteTheme(theme)}
                        >
                          <Trash2 size={13} />
                          <span>删除</span>
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      <footer className="admin-spaces-footer">
        <p>共 {filteredThemes.length} 条（总计 {themes.length} 条）</p>
      </footer>
    </section>
  );
}
