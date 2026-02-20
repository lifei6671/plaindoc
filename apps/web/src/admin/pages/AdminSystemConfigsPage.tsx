import { LoaderCircle, RefreshCw, Save } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Button } from "../../components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { Textarea } from "../../components/ui/textarea";
import { showToast } from "../../components/ui/toast";
import { type AdminSystemConfig, type DataGateway } from "../../data-access";
import { AdminPageCard, AdminTableContainer, AdminToolbarActions } from "../components/AdminPageLayout";
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

  const openToast = useCallback((message: string, variant: "success" | "info" | "error" = "error") => {
    showToast(message, variant);
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
    <section aria-label="系统配置管理">
      <AdminPageCard>
          <div className="flex flex-wrap items-end gap-3">
            <label className="w-full space-y-1.5 sm:w-[220px]">
              <span className="text-xs font-semibold tracking-wide text-slate-600">配置键</span>
              <Select
                value={selectedKey}
                onValueChange={(value) => {
                  setSelectedKey(value as SystemConfigKey);
                  setIsEditorDirty(false);
                }}
                disabled={loading || saving}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="site">site</SelectItem>
                  <SelectItem value="editor">editor</SelectItem>
                  <SelectItem value="security">security</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <AdminToolbarActions>
                <Button type="button" variant="outline" disabled={loading || saving} onClick={handleResetTemplate}>
                  模板填充
                </Button>
                <Button type="button" variant="outline" disabled={loading || saving} onClick={handleLoadCurrent}>
                  载入线上值
                </Button>
                <Button type="button" variant="outline" disabled={loading || saving} onClick={() => void loadConfigs()}>
                  <RefreshCw size={14} />
                  <span>刷新</span>
                </Button>
                <Button type="button" disabled={loading || saving} onClick={() => void handleSave()}>
                  <Save size={14} />
                  <span>{saving ? "保存中..." : "保存配置"}</span>
                </Button>
            </AdminToolbarActions>
          </div>

          <div className="rounded-sm bg-slate-50 p-3">
            <p className="mb-2 text-xs text-slate-600">
              当前键：<strong>{selectedKey}</strong>
              {selectedConfig ? `，当前版本：v${selectedConfig.version}` : "，当前版本：未创建"}
            </p>
            <Textarea
              className="min-h-[320px] font-mono text-xs leading-relaxed"
              value={editorText}
              onChange={(event) => {
                setEditorText(event.target.value);
                setIsEditorDirty(true);
              }}
              spellCheck={false}
              disabled={loading || saving}
            />
          </div>

          <AdminTableContainer>
              <table className="w-full min-w-[920px] border-collapse text-left text-sm">
                <thead className="sticky top-0 z-10 bg-slate-50/95 backdrop-blur">
                  <tr className="text-xs uppercase tracking-wide text-slate-600">
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">配置键</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">版本</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">更新人</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">更新时间</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">配置值</th>
                  </tr>
                </thead>
                <tbody>
                  {loading ? (
                    <tr>
                      <td colSpan={5} className="px-3 py-12">
                        <div className="flex items-center justify-center gap-2 text-sm text-slate-500">
                          <LoaderCircle size={15} className="animate-spin" />
                          <span>正在加载系统配置...</span>
                        </div>
                      </td>
                    </tr>
                  ) : configs.length === 0 ? (
                    <tr>
                      <td colSpan={5} className="px-3 py-12 text-center text-sm text-slate-500">
                        暂无系统配置
                      </td>
                    </tr>
                  ) : (
                    configs.map((item) => (
                      <tr key={item.configKey} className="border-b border-slate-100 align-top text-slate-700">
                        <td className="px-3 py-3">
                          <code className="rounded border border-sky-200 bg-sky-50 px-1.5 py-0.5 text-xs text-sky-700">
                            {item.configKey}
                          </code>
                        </td>
                        <td className="px-3 py-3 text-xs text-slate-600">v{item.version}</td>
                        <td className="px-3 py-3 text-xs text-slate-600">{item.updatedByUserId || "-"}</td>
                        <td className="px-3 py-3 text-xs text-slate-600">{formatDateTime(item.updatedAt)}</td>
                        <td className="px-3 py-3">
                          <pre className="max-h-40 overflow-auto rounded border border-slate-200 bg-slate-50 p-2 text-xs leading-relaxed text-slate-700">
                            {stringifyConfigValue(item.value)}
                          </pre>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
          </AdminTableContainer>

          <footer className="text-xs text-slate-600">共 {configs.length} 条配置</footer>
      </AdminPageCard>
    </section>
  );
}
