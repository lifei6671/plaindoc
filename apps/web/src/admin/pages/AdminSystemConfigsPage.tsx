import { LoaderCircle, RefreshCw, Save } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { type AdminSystemConfig, type DataGateway } from "../../data-access";
import { TopToast, type TopToastVariant } from "../../components/TopToast";
import { formatError } from "../../editor/status-utils";

type SystemConfigKey = "site" | "editor" | "security";

interface AdminSystemConfigsPageProps {
  dataGateway: DataGateway;
}

const SYSTEM_CONFIG_TEMPLATES: Record<SystemConfigKey, Record<string, unknown>> = {
  site: {
    allowRegistration: true,
    defaultSpaceVisibility: "member"
  },
  editor: {
    autosaveIntervalSeconds: 15,
    maxDocumentSizeKB: 1024
  },
  security: {
    accessTokenTTLMinutes: 120,
    refreshTokenTTLMinutes: 10080
  }
};

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

function stringifyConfigValue(value: Record<string, unknown>): string {
  return JSON.stringify(value, null, 2);
}

export function AdminSystemConfigsPage({ dataGateway }: AdminSystemConfigsPageProps) {
  const [configs, setConfigs] = useState<AdminSystemConfig[]>([]);
  const [selectedKey, setSelectedKey] = useState<SystemConfigKey>("site");
  const [editorText, setEditorText] = useState("{}");
  const [isEditorDirty, setIsEditorDirty] = useState(false);

  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
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

  const loadConfigs = useCallback(async () => {
    setLoading(true);
    try {
      const payload = await dataGateway.admin.listSystemConfigs();
      setConfigs(payload);
    } catch (error) {
      openToast(`加载系统配置失败：${formatError(error)}`);
      setConfigs([]);
    } finally {
      setLoading(false);
    }
  }, [dataGateway, openToast]);

  useEffect(() => {
    void loadConfigs();
  }, [loadConfigs]);

  const selectedConfig = useMemo(
    () => configs.find((item) => item.configKey === selectedKey) ?? null,
    [configs, selectedKey]
  );

  useEffect(() => {
    if (isEditorDirty) {
      return;
    }
    if (selectedConfig) {
      setEditorText(stringifyConfigValue(selectedConfig.value));
      return;
    }
    setEditorText(stringifyConfigValue(SYSTEM_CONFIG_TEMPLATES[selectedKey]));
  }, [isEditorDirty, selectedConfig, selectedKey]);

  const handleResetTemplate = useCallback(() => {
    setEditorText(stringifyConfigValue(SYSTEM_CONFIG_TEMPLATES[selectedKey]));
    setIsEditorDirty(true);
  }, [selectedKey]);

  const handleLoadCurrent = useCallback(() => {
    if (selectedConfig) {
      setEditorText(stringifyConfigValue(selectedConfig.value));
    } else {
      setEditorText(stringifyConfigValue(SYSTEM_CONFIG_TEMPLATES[selectedKey]));
    }
    setIsEditorDirty(false);
  }, [selectedConfig, selectedKey]);

  const handleSave = useCallback(async () => {
    let parsedValue: unknown;
    try {
      parsedValue = JSON.parse(editorText);
    } catch {
      openToast("JSON 格式不合法，请先修正");
      return;
    }
    if (!parsedValue || Array.isArray(parsedValue) || typeof parsedValue !== "object") {
      openToast("配置值必须是 JSON 对象");
      return;
    }

    setSaving(true);
    try {
      const result = await dataGateway.admin.upsertSystemConfig({
        configKey: selectedKey,
        value: parsedValue as Record<string, unknown>,
        expectedVersion: selectedConfig?.version ?? 0
      });
      openToast(`配置已保存：${result.configKey}（version=${result.version}）`, "success");
      setIsEditorDirty(false);
      await loadConfigs();
    } catch (error) {
      openToast(`保存系统配置失败：${formatError(error)}`);
    } finally {
      setSaving(false);
    }
  }, [dataGateway.admin, editorText, loadConfigs, openToast, selectedConfig?.version, selectedKey]);

  return (
    <section className="admin-spaces-panel" aria-label="系统配置管理">
      <TopToast
        open={toastState.open}
        message={toastState.message}
        variant={toastState.variant}
        triggerKey={toastState.triggerKey}
        durationMs={2800}
        onClose={closeToast}
      />
      <div className="admin-system-config-toolbar">
        <label className="admin-spaces-toolbar__field">
          <span>配置键</span>
          <select
            value={selectedKey}
            onChange={(event) => {
              setSelectedKey(event.target.value as SystemConfigKey);
              setIsEditorDirty(false);
            }}
            disabled={loading || saving}
          >
            <option value="site">site</option>
            <option value="editor">editor</option>
            <option value="security">security</option>
          </select>
        </label>
        <div className="admin-spaces-toolbar__actions">
          <button type="button" disabled={loading || saving} onClick={handleResetTemplate}>
            模板填充
          </button>
          <button type="button" disabled={loading || saving} onClick={handleLoadCurrent}>
            载入线上值
          </button>
          <button type="button" disabled={loading || saving} onClick={() => void loadConfigs()}>
            <RefreshCw size={14} />
            <span>刷新</span>
          </button>
          <button type="button" disabled={loading || saving} onClick={() => void handleSave()}>
            <Save size={14} />
            <span>{saving ? "保存中..." : "保存配置"}</span>
          </button>
        </div>
      </div>

      <div className="admin-system-config-editor-wrap">
        <p className="admin-system-config-editor__meta">
          当前键：<strong>{selectedKey}</strong>
          {selectedConfig ? `，当前版本：v${selectedConfig.version}` : "，当前版本：未创建"}
        </p>
        <textarea
          className="admin-system-config-editor"
          value={editorText}
          onChange={(event) => {
            setEditorText(event.target.value);
            setIsEditorDirty(true);
          }}
          spellCheck={false}
          disabled={loading || saving}
        />
      </div>

      <div className="admin-spaces-table-wrap">
        <table className="admin-spaces-table admin-system-config-table">
          <thead>
            <tr>
              <th>配置键</th>
              <th>版本</th>
              <th>更新人</th>
              <th>更新时间</th>
              <th>配置值</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={5}>
                  <div className="admin-spaces-loading-row">
                    <LoaderCircle size={15} className="admin-spaces-loading-row__icon" />
                    <span>正在加载系统配置...</span>
                  </div>
                </td>
              </tr>
            ) : configs.length === 0 ? (
              <tr>
                <td colSpan={5}>
                  <div className="admin-spaces-empty-row">暂无系统配置</div>
                </td>
              </tr>
            ) : (
              configs.map((item) => (
                <tr key={item.configKey}>
                  <td>
                    <code>{item.configKey}</code>
                  </td>
                  <td>v{item.version}</td>
                  <td>{item.updatedByUserId || "-"}</td>
                  <td>{formatDateTime(item.updatedAt)}</td>
                  <td>
                    <pre className="admin-system-config-value">{stringifyConfigValue(item.value)}</pre>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <footer className="admin-spaces-footer">
        <p>共 {configs.length} 条配置</p>
      </footer>
    </section>
  );
}
