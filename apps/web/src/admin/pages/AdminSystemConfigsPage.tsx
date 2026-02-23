import {
  Home,
  ImageIcon,
  Keyboard,
  LoaderCircle,
  Lock,
  RefreshCw,
  Save,
  type LucideIcon
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Button } from "../../components/ui/button";
import { Checkbox } from "../../components/ui/checkbox";
import { Input } from "../../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../../components/ui/tabs";
import { showToast } from "../../components/ui/toast";
import { type AdminSystemConfig, type DataGateway } from "../../data-access";
import { AdminPageCard, AdminToolbarActions } from "../components/AdminPageLayout";
import { formatError } from "../../editor/status-utils";
import {
  cloneImageHostingConfig,
  DEFAULT_IMAGE_HOSTING_CONFIG,
  normalizeImageHostingConfig,
  type AliyunOssConfig,
  type CloudflareR2Config,
  type ImageHostingConfig,
  type ImageHostingProvider,
  type LocalImageHostingConfig
} from "../../settings/image-hosting";

type SystemConfigKey = "site" | "editor" | "security" | "image-hosting";
type SpaceVisibility = "public" | "authenticated" | "member";

interface SiteSystemConfigValue {
  allowRegistration: boolean;
  defaultSpaceVisibility: SpaceVisibility;
}

interface EditorSystemConfigValue {
  autosaveIntervalSeconds: number;
  maxDocumentSizeKB: number;
}

interface SecuritySystemConfigValue {
  accessTokenTTLMinutes: number;
  refreshTokenTTLMinutes: number;
}

interface SystemConfigTabItem {
  key: SystemConfigKey;
  label: string;
  description: string;
  icon: LucideIcon;
}

const SYSTEM_CONFIG_TABS: SystemConfigTabItem[] = [
  {
    key: "site",
    label: "站点设置",
    description: "注册与默认可见性",
    icon: Home
  },
  {
    key: "editor",
    label: "编辑器设置",
    description: "自动保存与文档大小",
    icon: Keyboard
  },
  {
    key: "security",
    label: "安全设置",
    description: "Token 生命周期",
    icon: Lock
  },
  {
    key: "image-hosting",
    label: "图床设置",
    description: "本地 / R2 / OSS",
    icon: ImageIcon
  }
];

const SPACE_VISIBILITY_OPTIONS: Array<{ value: SpaceVisibility; label: string }> = [
  { value: "public", label: "完全公开（未登录可见）" },
  { value: "authenticated", label: "登录可见（需登录）" },
  { value: "member", label: "成员可见（阅读者及以上）" }
];

const IMAGE_HOSTING_PROVIDER_OPTIONS: Array<{ value: ImageHostingProvider; label: string }> = [
  { value: "local", label: "本地存储" },
  { value: "cloudflare-r2", label: "Cloudflare R2" },
  { value: "aliyun-oss", label: "阿里云 OSS" }
];

const SITE_TEMPLATE: SiteSystemConfigValue = {
  allowRegistration: true,
  defaultSpaceVisibility: "member"
};

const EDITOR_TEMPLATE: EditorSystemConfigValue = {
  autosaveIntervalSeconds: 15,
  maxDocumentSizeKB: 1024
};

const SECURITY_TEMPLATE: SecuritySystemConfigValue = {
  accessTokenTTLMinutes: 120,
  refreshTokenTTLMinutes: 10080
};

const IMAGE_HOSTING_TEMPLATE = cloneImageHostingConfig(DEFAULT_IMAGE_HOSTING_CONFIG);

interface AdminSystemConfigsPageProps {
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

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
}

function parseInteger(value: unknown, fallback: number): number {
  if (typeof value === "number" && Number.isFinite(value)) {
    return Math.trunc(value);
  }
  return fallback;
}

function parseString(value: unknown, fallback: string): string {
  if (typeof value !== "string") {
    return fallback;
  }
  const trimmedValue = value.trim();
  return trimmedValue || fallback;
}

function parseSiteConfig(value: unknown): SiteSystemConfigValue {
  const payload = asRecord(value);
  if (!payload) {
    return { ...SITE_TEMPLATE };
  }

  const allowRegistration =
    typeof payload.allowRegistration === "boolean"
      ? payload.allowRegistration
      : SITE_TEMPLATE.allowRegistration;
  const defaultVisibility = payload.defaultSpaceVisibility;
  const defaultSpaceVisibility: SpaceVisibility =
    defaultVisibility === "public" || defaultVisibility === "authenticated" || defaultVisibility === "member"
      ? defaultVisibility
      : SITE_TEMPLATE.defaultSpaceVisibility;

  return {
    allowRegistration,
    defaultSpaceVisibility
  };
}

function parseEditorConfig(value: unknown): EditorSystemConfigValue {
  const payload = asRecord(value);
  if (!payload) {
    return { ...EDITOR_TEMPLATE };
  }

  return {
    autosaveIntervalSeconds: parseInteger(payload.autosaveIntervalSeconds, EDITOR_TEMPLATE.autosaveIntervalSeconds),
    maxDocumentSizeKB: parseInteger(payload.maxDocumentSizeKB, EDITOR_TEMPLATE.maxDocumentSizeKB)
  };
}

function parseSecurityConfig(value: unknown): SecuritySystemConfigValue {
  const payload = asRecord(value);
  if (!payload) {
    return { ...SECURITY_TEMPLATE };
  }

  return {
    accessTokenTTLMinutes: parseInteger(payload.accessTokenTTLMinutes, SECURITY_TEMPLATE.accessTokenTTLMinutes),
    refreshTokenTTLMinutes: parseInteger(payload.refreshTokenTTLMinutes, SECURITY_TEMPLATE.refreshTokenTTLMinutes)
  };
}

function parseImageHostingConfig(value: unknown): ImageHostingConfig {
  return normalizeImageHostingConfig(value);
}

function normalizeIntegerInput(rawValue: string, fallbackValue: number): number {
  const parsedValue = Number.parseInt(rawValue, 10);
  if (!Number.isFinite(parsedValue)) {
    return fallbackValue;
  }
  return Math.trunc(parsedValue);
}

export function AdminSystemConfigsPage({ dataGateway }: AdminSystemConfigsPageProps) {
  const [configs, setConfigs] = useState<AdminSystemConfig[]>([]);
  const [selectedKey, setSelectedKey] = useState<SystemConfigKey>("site");
  const [imageHostingProviderTab, setImageHostingProviderTab] = useState<ImageHostingProvider>("local");

  const [siteDraft, setSiteDraft] = useState<SiteSystemConfigValue>({ ...SITE_TEMPLATE });
  const [editorDraft, setEditorDraft] = useState<EditorSystemConfigValue>({ ...EDITOR_TEMPLATE });
  const [securityDraft, setSecurityDraft] = useState<SecuritySystemConfigValue>({ ...SECURITY_TEMPLATE });
  const [imageHostingDraft, setImageHostingDraft] = useState<ImageHostingConfig>(
    cloneImageHostingConfig(IMAGE_HOSTING_TEMPLATE)
  );

  const [dirtyKeys, setDirtyKeys] = useState<Record<SystemConfigKey, boolean>>({
    site: false,
    editor: false,
    security: false,
    "image-hosting": false
  });
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  const openToast = useCallback((message: string, variant: "success" | "info" | "error" = "error") => {
    showToast(message, variant);
  }, []);

  const tabMap = useMemo(
    () =>
      SYSTEM_CONFIG_TABS.reduce<Record<SystemConfigKey, SystemConfigTabItem>>((accumulator, item) => {
        accumulator[item.key] = item;
        return accumulator;
      }, {} as Record<SystemConfigKey, SystemConfigTabItem>),
    []
  );
  const selectedTab = tabMap[selectedKey];

  const selectedConfig = useMemo(
    () => configs.find((item) => item.configKey === selectedKey) ?? null,
    [configs, selectedKey]
  );

  const findConfigValue = useCallback(
    (key: SystemConfigKey): Record<string, unknown> | null => {
      const item = configs.find((entry) => entry.configKey === key);
      return item?.value ?? null;
    },
    [configs]
  );

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

  useEffect(() => {
    if (!dirtyKeys.site) {
      setSiteDraft(parseSiteConfig(findConfigValue("site")));
    }
    if (!dirtyKeys.editor) {
      setEditorDraft(parseEditorConfig(findConfigValue("editor")));
    }
    if (!dirtyKeys.security) {
      setSecurityDraft(parseSecurityConfig(findConfigValue("security")));
    }
    if (!dirtyKeys["image-hosting"]) {
      const parsedConfig = parseImageHostingConfig(findConfigValue("image-hosting"));
      setImageHostingDraft(parsedConfig);
      setImageHostingProviderTab(parsedConfig.defaultProvider);
    }
  }, [dirtyKeys, findConfigValue]);

  const markDirty = useCallback((key: SystemConfigKey) => {
    setDirtyKeys((previous) => ({
      ...previous,
      [key]: true
    }));
  }, []);

  const clearDirty = useCallback((key: SystemConfigKey) => {
    setDirtyKeys((previous) => ({
      ...previous,
      [key]: false
    }));
  }, []);

  const isSelectedDirty = dirtyKeys[selectedKey];

  const handleResetTemplate = useCallback(() => {
    switch (selectedKey) {
      case "site":
        setSiteDraft({ ...SITE_TEMPLATE });
        markDirty("site");
        return;
      case "editor":
        setEditorDraft({ ...EDITOR_TEMPLATE });
        markDirty("editor");
        return;
      case "security":
        setSecurityDraft({ ...SECURITY_TEMPLATE });
        markDirty("security");
        return;
      case "image-hosting":
        setImageHostingDraft(cloneImageHostingConfig(IMAGE_HOSTING_TEMPLATE));
        setImageHostingProviderTab(IMAGE_HOSTING_TEMPLATE.defaultProvider);
        markDirty("image-hosting");
        return;
      default:
        return;
    }
  }, [markDirty, selectedKey]);

  const handleLoadCurrent = useCallback(() => {
    switch (selectedKey) {
      case "site":
        setSiteDraft(parseSiteConfig(findConfigValue("site")));
        clearDirty("site");
        return;
      case "editor":
        setEditorDraft(parseEditorConfig(findConfigValue("editor")));
        clearDirty("editor");
        return;
      case "security":
        setSecurityDraft(parseSecurityConfig(findConfigValue("security")));
        clearDirty("security");
        return;
      case "image-hosting": {
        const parsedConfig = parseImageHostingConfig(findConfigValue("image-hosting"));
        setImageHostingDraft(parsedConfig);
        setImageHostingProviderTab(parsedConfig.defaultProvider);
        clearDirty("image-hosting");
        return;
      }
      default:
        return;
    }
  }, [clearDirty, findConfigValue, selectedKey]);

  const buildSelectedPayload = useCallback((): Record<string, unknown> => {
    switch (selectedKey) {
      case "site":
        return {
          allowRegistration: siteDraft.allowRegistration,
          defaultSpaceVisibility: siteDraft.defaultSpaceVisibility
        };
      case "editor":
        return {
          autosaveIntervalSeconds: editorDraft.autosaveIntervalSeconds,
          maxDocumentSizeKB: editorDraft.maxDocumentSizeKB
        };
      case "security":
        return {
          accessTokenTTLMinutes: securityDraft.accessTokenTTLMinutes,
          refreshTokenTTLMinutes: securityDraft.refreshTokenTTLMinutes
        };
      case "image-hosting":
        return cloneImageHostingConfig(imageHostingDraft) as unknown as Record<string, unknown>;
      default:
        return {};
    }
  }, [editorDraft, imageHostingDraft, securityDraft, selectedKey, siteDraft]);

  const handleSave = useCallback(async () => {
    const payload = buildSelectedPayload();
    setSaving(true);
    try {
      const result = await dataGateway.admin.upsertSystemConfig({
        configKey: selectedKey,
        value: payload,
        expectedVersion: selectedConfig?.version ?? 0
      });
      clearDirty(selectedKey);
      openToast(`配置已保存：${selectedTab.label}（v${result.version}）`, "success");
      await loadConfigs();
    } catch (error) {
      openToast(`保存系统配置失败：${formatError(error)}`);
    } finally {
      setSaving(false);
    }
  }, [buildSelectedPayload, clearDirty, dataGateway.admin, loadConfigs, openToast, selectedConfig?.version, selectedKey, selectedTab.label]);

  const setCloudflareField = useCallback((field: keyof CloudflareR2Config, value: string) => {
    setImageHostingDraft((previousConfig) => ({
      ...previousConfig,
      cloudflareR2: {
        ...previousConfig.cloudflareR2,
        [field]: parseString(value, "")
      }
    }));
    markDirty("image-hosting");
  }, [markDirty]);

  const setAliyunField = useCallback((field: keyof AliyunOssConfig, value: string) => {
    setImageHostingDraft((previousConfig) => ({
      ...previousConfig,
      aliyunOss: {
        ...previousConfig.aliyunOss,
        [field]: parseString(value, "")
      }
    }));
    markDirty("image-hosting");
  }, [markDirty]);

  const setLocalField = useCallback((field: keyof LocalImageHostingConfig, value: string) => {
    setImageHostingDraft((previousConfig) => ({
      ...previousConfig,
      local: {
        ...previousConfig.local,
        [field]: parseString(value, "")
      }
    }));
    markDirty("image-hosting");
  }, [markDirty]);

  return (
    <section aria-label="系统配置管理">
      <AdminPageCard>
        <div className="overflow-hidden rounded-md border border-slate-200 bg-white">
          <div className="grid min-h-[560px] gap-0 md:grid-cols-[240px_minmax(0,1fr)]">
            <aside className="hidden bg-transparent p-2 md:block">
            <nav className="space-y-1" aria-label="配置分组">
              {SYSTEM_CONFIG_TABS.map((tab) => {
                const isActive = tab.key === selectedKey;
                const TabIcon = tab.icon;
                return (
                  <button
                    key={tab.key}
                    type="button"
                    className={`w-full appearance-none rounded-lg border border-transparent px-3 py-2.5 text-left shadow-none outline-none transition focus-visible:ring-2 focus-visible:ring-sky-200 ${
                      isActive
                        ? "bg-slate-200 text-slate-900"
                        : "text-slate-700 hover:bg-slate-200/70"
                    }`}
                    onClick={() => setSelectedKey(tab.key)}
                    disabled={saving}
                  >
                    <p className="flex items-center gap-2.5 text-sm font-medium">
                      <TabIcon size={16} />
                      <span>{tab.label}</span>
                    </p>
                    <p className="mt-0.5 pl-[26px] text-xs text-slate-500">{tab.description}</p>
                  </button>
                );
              })}
            </nav>
          </aside>

          <main className="flex min-h-[560px] flex-col">
            <header className="flex h-16 shrink-0 items-center justify-between gap-3 border-b border-slate-200 px-4">
              <div>
                <h3 className="text-base font-semibold text-slate-800">{selectedTab.label}</h3>
              </div>
              <AdminToolbarActions>
                <Button type="button" variant="outline" disabled={loading || saving} onClick={handleResetTemplate}>
                  模板填充
                </Button>
                <Button type="button" variant="outline" disabled={loading || saving} onClick={handleLoadCurrent}>
                  载入线上值
                </Button>
                <Button type="button" variant="outline" disabled={loading || saving} onClick={() => void loadConfigs()}>
                  <RefreshCw size={14} />
                  <span>{loading ? "刷新中..." : "刷新"}</span>
                </Button>
                <Button type="button" disabled={loading || saving || !isSelectedDirty} onClick={() => void handleSave()}>
                  <Save size={14} />
                  <span>{saving ? "保存中..." : "保存配置"}</span>
                </Button>
              </AdminToolbarActions>
            </header>

            {loading ? (
              <div className="mx-4 mt-4 flex items-center gap-2 rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-600">
                <LoaderCircle size={15} className="animate-spin" />
                <span>正在加载系统配置...</span>
              </div>
            ) : null}

            <div className="mx-4 mt-4 rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-600">
              配置键：<code>{selectedKey}</code>
              {selectedConfig ? `，版本：v${selectedConfig.version}` : "，版本：未创建"}
              {selectedConfig ? `，更新时间：${formatDateTime(selectedConfig.updatedAt)}` : ""}
              {selectedConfig?.updatedByUserId ? `，更新人：${selectedConfig.updatedByUserId}` : ""}
            </div>

            <div className="flex-1 overflow-y-auto p-4">

            {selectedKey === "site" ? (
              <div className="rounded-md border border-slate-200 bg-white p-4">
                <div className="grid gap-4 sm:grid-cols-2">
                  <label className="flex items-center gap-2.5 rounded-md border border-slate-200 bg-slate-50 px-3 py-3">
                    <Checkbox
                      checked={siteDraft.allowRegistration}
                      onCheckedChange={(checked) => {
                        setSiteDraft((previous) => ({
                          ...previous,
                          allowRegistration: checked === true
                        }));
                        markDirty("site");
                      }}
                      disabled={saving}
                    />
                    <div className="space-y-0.5">
                      <span className="text-sm font-medium text-slate-700">允许新用户注册</span>
                      <p className="text-xs text-slate-500">关闭后，前台注册接口会拒绝请求。</p>
                    </div>
                  </label>
                  <label className="space-y-1.5">
                    <span className="text-xs font-semibold tracking-wide text-slate-600">默认空间可见性</span>
                    <Select
                      value={siteDraft.defaultSpaceVisibility}
                      onValueChange={(value) => {
                        setSiteDraft((previous) => ({
                          ...previous,
                          defaultSpaceVisibility: value as SpaceVisibility
                        }));
                        markDirty("site");
                      }}
                      disabled={saving}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {SPACE_VISIBILITY_OPTIONS.map((option) => (
                          <SelectItem key={option.value} value={option.value}>
                            {option.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </label>
                </div>
              </div>
            ) : null}

            {selectedKey === "editor" ? (
              <div className="rounded-md border border-slate-200 bg-white p-4">
                <div className="grid gap-4 sm:grid-cols-2">
                  <label className="space-y-1.5">
                    <span className="text-xs font-semibold tracking-wide text-slate-600">自动保存间隔（秒）</span>
                    <Input
                      type="number"
                      min={5}
                      max={600}
                      value={String(editorDraft.autosaveIntervalSeconds)}
                      onChange={(event) => {
                        setEditorDraft((previous) => ({
                          ...previous,
                          autosaveIntervalSeconds: normalizeIntegerInput(
                            event.target.value,
                            previous.autosaveIntervalSeconds
                          )
                        }));
                        markDirty("editor");
                      }}
                      disabled={saving}
                    />
                  </label>
                  <label className="space-y-1.5">
                    <span className="text-xs font-semibold tracking-wide text-slate-600">文档最大体积（KB）</span>
                    <Input
                      type="number"
                      min={64}
                      max={4096}
                      value={String(editorDraft.maxDocumentSizeKB)}
                      onChange={(event) => {
                        setEditorDraft((previous) => ({
                          ...previous,
                          maxDocumentSizeKB: normalizeIntegerInput(event.target.value, previous.maxDocumentSizeKB)
                        }));
                        markDirty("editor");
                      }}
                      disabled={saving}
                    />
                  </label>
                </div>
              </div>
            ) : null}

            {selectedKey === "security" ? (
              <div className="rounded-md border border-slate-200 bg-white p-4">
                <div className="grid gap-4 sm:grid-cols-2">
                  <label className="space-y-1.5">
                    <span className="text-xs font-semibold tracking-wide text-slate-600">Access Token 时长（分钟）</span>
                    <Input
                      type="number"
                      min={5}
                      max={1440}
                      value={String(securityDraft.accessTokenTTLMinutes)}
                      onChange={(event) => {
                        setSecurityDraft((previous) => ({
                          ...previous,
                          accessTokenTTLMinutes: normalizeIntegerInput(
                            event.target.value,
                            previous.accessTokenTTLMinutes
                          )
                        }));
                        markDirty("security");
                      }}
                      disabled={saving}
                    />
                  </label>
                  <label className="space-y-1.5">
                    <span className="text-xs font-semibold tracking-wide text-slate-600">Refresh Token 时长（分钟）</span>
                    <Input
                      type="number"
                      min={60}
                      max={43200}
                      value={String(securityDraft.refreshTokenTTLMinutes)}
                      onChange={(event) => {
                        setSecurityDraft((previous) => ({
                          ...previous,
                          refreshTokenTTLMinutes: normalizeIntegerInput(
                            event.target.value,
                            previous.refreshTokenTTLMinutes
                          )
                        }));
                        markDirty("security");
                      }}
                      disabled={saving}
                    />
                  </label>
                </div>
              </div>
            ) : null}

            {selectedKey === "image-hosting" ? (
              <div className="rounded-md border border-slate-200 bg-white p-4">
                <div className="grid gap-4 sm:grid-cols-2">
                  <label className="space-y-1.5">
                    <span className="text-xs font-semibold tracking-wide text-slate-600">默认图床</span>
                    <Select
                      value={imageHostingDraft.defaultProvider}
                      onValueChange={(value) => {
                        setImageHostingDraft((previousConfig) => ({
                          ...previousConfig,
                          defaultProvider: value as ImageHostingProvider
                        }));
                        markDirty("image-hosting");
                      }}
                      disabled={saving}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {IMAGE_HOSTING_PROVIDER_OPTIONS.map((option) => (
                          <SelectItem key={option.value} value={option.value}>
                            {option.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </label>
                  <div className="rounded-md border border-slate-200 bg-slate-50 px-3 py-3 text-xs text-slate-600">
                    当前默认图床：{
                      IMAGE_HOSTING_PROVIDER_OPTIONS.find((option) => option.value === imageHostingDraft.defaultProvider)
                        ?.label
                    }
                  </div>
                </div>

                <Tabs
                  className="mt-4"
                  value={imageHostingProviderTab}
                  onValueChange={(value) => setImageHostingProviderTab(value as ImageHostingProvider)}
                >
                  <p className="mb-2 text-xs font-semibold tracking-wide text-slate-600">图床配置项</p>
                  <TabsList className="h-auto w-full justify-start gap-2 overflow-x-auto border-0 bg-transparent p-0">
                    {IMAGE_HOSTING_PROVIDER_OPTIONS.map((option) => (
                      <TabsTrigger
                        key={option.value}
                        value={option.value}
                        disabled={saving}
                        className="rounded-md border border-slate-200 bg-white px-4 py-2 text-base font-medium text-slate-700 shadow-sm hover:bg-slate-50 data-[state=active]:border-sky-300 data-[state=active]:bg-sky-50 data-[state=active]:text-sky-700"
                      >
                        {option.label}
                      </TabsTrigger>
                    ))}
                  </TabsList>

                  <TabsContent value="local" className="grid gap-4 sm:grid-cols-2">
                    <label className="space-y-1.5">
                      <span className="text-xs font-semibold tracking-wide text-slate-600">上传接口地址</span>
                      <Input
                        placeholder="/api/uploads/images"
                        value={imageHostingDraft.local.uploadEndpoint}
                        onChange={(event) => setLocalField("uploadEndpoint", event.target.value)}
                        disabled={saving}
                      />
                    </label>
                    <label className="space-y-1.5">
                      <span className="text-xs font-semibold tracking-wide text-slate-600">公网访问前缀</span>
                      <Input
                        placeholder="/uploads"
                        value={imageHostingDraft.local.publicBaseUrl}
                        onChange={(event) => setLocalField("publicBaseUrl", event.target.value)}
                        disabled={saving}
                      />
                    </label>
                  </TabsContent>

                  <TabsContent value="cloudflare-r2" className="grid gap-4 sm:grid-cols-2">
                    <label className="space-y-1.5">
                      <span className="text-xs font-semibold tracking-wide text-slate-600">Account ID</span>
                      <Input
                        placeholder="4d2a1c..."
                        value={imageHostingDraft.cloudflareR2.accountId}
                        onChange={(event) => setCloudflareField("accountId", event.target.value)}
                        disabled={saving}
                      />
                    </label>
                    <label className="space-y-1.5">
                      <span className="text-xs font-semibold tracking-wide text-slate-600">Bucket</span>
                      <Input
                        placeholder="plaindoc-assets"
                        value={imageHostingDraft.cloudflareR2.bucket}
                        onChange={(event) => setCloudflareField("bucket", event.target.value)}
                        disabled={saving}
                      />
                    </label>
                    <label className="space-y-1.5">
                      <span className="text-xs font-semibold tracking-wide text-slate-600">Access Key ID</span>
                      <Input
                        placeholder="R2XXXX..."
                        value={imageHostingDraft.cloudflareR2.accessKeyId}
                        onChange={(event) => setCloudflareField("accessKeyId", event.target.value)}
                        disabled={saving}
                      />
                    </label>
                    <label className="space-y-1.5">
                      <span className="text-xs font-semibold tracking-wide text-slate-600">Secret Access Key</span>
                      <Input
                        type="password"
                        placeholder="输入 Secret Access Key"
                        value={imageHostingDraft.cloudflareR2.secretAccessKey}
                        onChange={(event) => setCloudflareField("secretAccessKey", event.target.value)}
                        disabled={saving}
                      />
                    </label>
                    <label className="space-y-1.5 sm:col-span-2">
                      <span className="text-xs font-semibold tracking-wide text-slate-600">公网访问域名</span>
                      <Input
                        placeholder="https://img.example.com"
                        value={imageHostingDraft.cloudflareR2.publicBaseUrl}
                        onChange={(event) => setCloudflareField("publicBaseUrl", event.target.value)}
                        disabled={saving}
                      />
                    </label>
                  </TabsContent>

                  <TabsContent value="aliyun-oss" className="grid gap-4 sm:grid-cols-2">
                    <label className="space-y-1.5">
                      <span className="text-xs font-semibold tracking-wide text-slate-600">Region（可选）</span>
                      <Input
                        placeholder="oss-cn-hangzhou"
                        value={imageHostingDraft.aliyunOss.region}
                        onChange={(event) => setAliyunField("region", event.target.value)}
                        disabled={saving}
                      />
                    </label>
                    <label className="space-y-1.5">
                      <span className="text-xs font-semibold tracking-wide text-slate-600">Bucket</span>
                      <Input
                        placeholder="plaindoc-assets"
                        value={imageHostingDraft.aliyunOss.bucket}
                        onChange={(event) => setAliyunField("bucket", event.target.value)}
                        disabled={saving}
                      />
                    </label>
                    <label className="space-y-1.5">
                      <span className="text-xs font-semibold tracking-wide text-slate-600">Endpoint（可选）</span>
                      <Input
                        placeholder="https://oss-cn-hangzhou.aliyuncs.com"
                        value={imageHostingDraft.aliyunOss.endpoint}
                        onChange={(event) => setAliyunField("endpoint", event.target.value)}
                        disabled={saving}
                      />
                    </label>
                    <label className="space-y-1.5">
                      <span className="text-xs font-semibold tracking-wide text-slate-600">Access Key ID</span>
                      <Input
                        placeholder="LTAI..."
                        value={imageHostingDraft.aliyunOss.accessKeyId}
                        onChange={(event) => setAliyunField("accessKeyId", event.target.value)}
                        disabled={saving}
                      />
                    </label>
                    <label className="space-y-1.5">
                      <span className="text-xs font-semibold tracking-wide text-slate-600">Access Key Secret</span>
                      <Input
                        type="password"
                        placeholder="输入 Access Key Secret"
                        value={imageHostingDraft.aliyunOss.accessKeySecret}
                        onChange={(event) => setAliyunField("accessKeySecret", event.target.value)}
                        disabled={saving}
                      />
                    </label>
                    <label className="space-y-1.5 sm:col-span-2">
                      <span className="text-xs font-semibold tracking-wide text-slate-600">公网访问域名</span>
                      <Input
                        placeholder="https://img.example.com"
                        value={imageHostingDraft.aliyunOss.publicBaseUrl}
                        onChange={(event) => setAliyunField("publicBaseUrl", event.target.value)}
                        disabled={saving}
                      />
                    </label>
                  </TabsContent>
                </Tabs>
              </div>
            ) : null}
            </div>
          </main>
        </div>
      </div>
      </AdminPageCard>
    </section>
  );
}
