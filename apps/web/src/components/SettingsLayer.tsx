import { memo, useCallback, useEffect, useState, type FormEvent } from "react";
import {
  cloneImageHostingConfig,
  type AliyunOssConfig,
  type CloudflareR2Config,
  type ImageHostingConfig,
  type ImageHostingProvider
} from "../settings/image-hosting";

interface SettingsLayerProps {
  open: boolean;
  initialImageHostingConfig: ImageHostingConfig;
  isLoading: boolean;
  isSaving: boolean;
  errorMessage: string | null;
  onClose: () => void;
  onSaveImageHostingConfig: (config: ImageHostingConfig) => Promise<void>;
}

// 通用文本输入组件：统一图床配置字段的标签、占位文案与输入样式。
interface SettingsTextFieldProps {
  label: string;
  value: string;
  placeholder: string;
  onChange: (value: string) => void;
  type?: "text" | "password";
}

function SettingsTextField({
  label,
  value,
  placeholder,
  onChange,
  type = "text"
}: SettingsTextFieldProps) {
  return (
    <label className="settings-form__field">
      <span className="settings-form__label">{label}</span>
      <input
        type={type}
        value={value}
        placeholder={placeholder}
        autoComplete="off"
        onChange={(event) => onChange(event.target.value)}
      />
    </label>
  );
}

export const SettingsLayer = memo(function SettingsLayer({
  open,
  initialImageHostingConfig,
  isLoading,
  isSaving,
  errorMessage,
  onClose,
  onSaveImageHostingConfig
}: SettingsLayerProps) {
  // 当前正在编辑的厂商 tab；仅影响右侧表单可见区，不等于“默认图床”。
  const [activeProviderTab, setActiveProviderTab] = useState<ImageHostingProvider>(
    initialImageHostingConfig.defaultProvider
  );
  // 图床配置草稿：只在设置面板内部维护，点击保存后再提交到数据层。
  const [draftImageHostingConfig, setDraftImageHostingConfig] =
    useState<ImageHostingConfig>(initialImageHostingConfig);

  // 面板打开时重置草稿，避免上次未保存的输入污染下一次打开。
  useEffect(() => {
    if (!open) {
      return;
    }
    const clonedConfig = cloneImageHostingConfig(initialImageHostingConfig);
    setDraftImageHostingConfig(clonedConfig);
    setActiveProviderTab(clonedConfig.defaultProvider);
  }, [open, initialImageHostingConfig]);

  // 浮层打开时监听 ESC，提升键盘操作可达性。
  useEffect(() => {
    if (!open) {
      return;
    }

    const onWindowKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };

    window.addEventListener("keydown", onWindowKeyDown);
    return () => {
      window.removeEventListener("keydown", onWindowKeyDown);
    };
  }, [open, onClose]);

  const setCloudflareField = useCallback((field: keyof CloudflareR2Config, value: string) => {
    setDraftImageHostingConfig((previousConfig) => ({
      ...previousConfig,
      cloudflareR2: {
        ...previousConfig.cloudflareR2,
        [field]: value
      }
    }));
  }, []);

  const setAliyunField = useCallback((field: keyof AliyunOssConfig, value: string) => {
    setDraftImageHostingConfig((previousConfig) => ({
      ...previousConfig,
      aliyunOss: {
        ...previousConfig.aliyunOss,
        [field]: value
      }
    }));
  }, []);

  // 仅切换“当前编辑中的厂商”。
  const switchProviderTab = useCallback((provider: ImageHostingProvider) => {
    setActiveProviderTab(provider);
  }, []);

  // 设为默认图床：用于粘贴图片自动上传时选择目标厂商。
  const setDefaultProvider = useCallback((provider: ImageHostingProvider) => {
    setDraftImageHostingConfig((previousConfig) => ({
      ...previousConfig,
      defaultProvider: provider
    }));
  }, []);

  // 提交保存：将当前草稿交给父组件持久化。
  const handleSave = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      await onSaveImageHostingConfig(draftImageHostingConfig);
    },
    [draftImageHostingConfig, onSaveImageHostingConfig]
  );

  if (!open) {
    return null;
  }

  return (
    <div className="settings-layer" role="dialog" aria-modal="true" aria-label="设置">
      <button type="button" className="settings-layer__backdrop" aria-label="关闭设置" onClick={onClose} />
      <section className="settings-panel">
        <header className="settings-panel__header">
          <div className="settings-panel__title-wrap">
            <h2>设置</h2>
            <p>统一管理图床配置，保存后将用于后续图片上传流程。</p>
          </div>
          <button type="button" className="settings-panel__close" onClick={onClose}>
            关闭
          </button>
        </header>
        <div className="settings-panel__content">
          <nav className="settings-section-tabs" aria-label="设置分类">
            <button
              type="button"
              className="settings-section-tabs__item settings-section-tabs__item--active"
              aria-current="page"
            >
              图床设置
            </button>
          </nav>

          <div className="settings-panel__main">
            <div className="settings-provider-tabs" role="tablist" aria-label="图床厂商">
              <button
                type="button"
                role="tab"
                aria-selected={activeProviderTab === "cloudflare-r2"}
                className={`settings-provider-tabs__item ${
                  activeProviderTab === "cloudflare-r2" ? "settings-provider-tabs__item--active" : ""
                }`}
                onClick={() => switchProviderTab("cloudflare-r2")}
              >
                Cloudflare R2
                {draftImageHostingConfig.defaultProvider === "cloudflare-r2" ? (
                  <span className="settings-provider-tabs__badge">默认</span>
                ) : null}
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={activeProviderTab === "aliyun-oss"}
                className={`settings-provider-tabs__item ${
                  activeProviderTab === "aliyun-oss" ? "settings-provider-tabs__item--active" : ""
                }`}
                onClick={() => switchProviderTab("aliyun-oss")}
              >
                阿里云 OSS
                {draftImageHostingConfig.defaultProvider === "aliyun-oss" ? (
                  <span className="settings-provider-tabs__badge">默认</span>
                ) : null}
              </button>
            </div>

            {isLoading ? (
              <p className="settings-form__hint">正在加载配置...</p>
            ) : (
              <form className="settings-form" onSubmit={(event) => void handleSave(event)}>
                <div className="settings-form__default-provider">
                  <span>默认图床</span>
                  <button
                    type="button"
                    className="settings-form__default-button"
                    onClick={() => setDefaultProvider(activeProviderTab)}
                    disabled={draftImageHostingConfig.defaultProvider === activeProviderTab}
                  >
                    {draftImageHostingConfig.defaultProvider === activeProviderTab
                      ? "当前厂商已是默认图床"
                      : `设为默认图床（${activeProviderTab === "cloudflare-r2" ? "Cloudflare R2" : "阿里云 OSS"}）`}
                  </button>
                </div>

                {activeProviderTab === "cloudflare-r2" ? (
                  <div className="settings-form__grid" role="tabpanel" aria-label="Cloudflare R2 配置">
                    <SettingsTextField
                      label="Account ID"
                      value={draftImageHostingConfig.cloudflareR2.accountId}
                      placeholder="例如：4d2a1c..."
                      onChange={(value) => setCloudflareField("accountId", value)}
                    />
                    <SettingsTextField
                      label="Bucket"
                      value={draftImageHostingConfig.cloudflareR2.bucket}
                      placeholder="例如：plaindoc-assets"
                      onChange={(value) => setCloudflareField("bucket", value)}
                    />
                    <SettingsTextField
                      label="Access Key ID"
                      value={draftImageHostingConfig.cloudflareR2.accessKeyId}
                      placeholder="例如：R2XXXX..."
                      onChange={(value) => setCloudflareField("accessKeyId", value)}
                    />
                    <SettingsTextField
                      label="Secret Access Key"
                      type="password"
                      value={draftImageHostingConfig.cloudflareR2.secretAccessKey}
                      placeholder="输入 Secret Access Key"
                      onChange={(value) => setCloudflareField("secretAccessKey", value)}
                    />
                    <SettingsTextField
                      label="公网访问域名"
                      value={draftImageHostingConfig.cloudflareR2.publicBaseUrl}
                      placeholder="例如：https://img.example.com"
                      onChange={(value) => setCloudflareField("publicBaseUrl", value)}
                    />
                  </div>
                ) : (
                  <div className="settings-form__grid" role="tabpanel" aria-label="阿里云 OSS 配置">
                    <SettingsTextField
                      label="Region"
                      value={draftImageHostingConfig.aliyunOss.region}
                      placeholder="例如：oss-cn-hangzhou"
                      onChange={(value) => setAliyunField("region", value)}
                    />
                    <SettingsTextField
                      label="Bucket"
                      value={draftImageHostingConfig.aliyunOss.bucket}
                      placeholder="例如：plaindoc-assets"
                      onChange={(value) => setAliyunField("bucket", value)}
                    />
                    <SettingsTextField
                      label="Endpoint"
                      value={draftImageHostingConfig.aliyunOss.endpoint}
                      placeholder="例如：https://oss-cn-hangzhou.aliyuncs.com"
                      onChange={(value) => setAliyunField("endpoint", value)}
                    />
                    <SettingsTextField
                      label="Access Key ID"
                      value={draftImageHostingConfig.aliyunOss.accessKeyId}
                      placeholder="例如：LTAI..."
                      onChange={(value) => setAliyunField("accessKeyId", value)}
                    />
                    <SettingsTextField
                      label="Access Key Secret"
                      type="password"
                      value={draftImageHostingConfig.aliyunOss.accessKeySecret}
                      placeholder="输入 Access Key Secret"
                      onChange={(value) => setAliyunField("accessKeySecret", value)}
                    />
                    <SettingsTextField
                      label="公网访问域名"
                      value={draftImageHostingConfig.aliyunOss.publicBaseUrl}
                      placeholder="例如：https://img.example.com"
                      onChange={(value) => setAliyunField("publicBaseUrl", value)}
                    />
                  </div>
                )}

                {errorMessage ? <p className="settings-form__error">{errorMessage}</p> : null}

                <div className="settings-form__actions">
                  <button type="button" className="settings-form__cancel" onClick={onClose}>
                    取消
                  </button>
                  <button type="submit" className="settings-form__submit" disabled={isSaving}>
                    {isSaving ? "保存中..." : "保存配置"}
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      </section>
    </div>
  );
});
