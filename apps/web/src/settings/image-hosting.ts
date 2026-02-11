export type ImageHostingProvider = "cloudflare-r2" | "aliyun-oss";

export interface CloudflareR2Config {
  accountId: string;
  accessKeyId: string;
  secretAccessKey: string;
  bucket: string;
  publicBaseUrl: string;
}

export interface AliyunOssConfig {
  region: string;
  accessKeyId: string;
  accessKeySecret: string;
  bucket: string;
  endpoint: string;
  publicBaseUrl: string;
}

export interface ImageHostingConfig {
  // 默认图床提供商：粘贴图片时按该设置自动上传。
  defaultProvider: ImageHostingProvider;
  cloudflareR2: CloudflareR2Config;
  aliyunOss: AliyunOssConfig;
}

export const DEFAULT_IMAGE_HOSTING_CONFIG: ImageHostingConfig = {
  defaultProvider: "cloudflare-r2",
  cloudflareR2: {
    accountId: "",
    accessKeyId: "",
    secretAccessKey: "",
    bucket: "",
    publicBaseUrl: ""
  },
  aliyunOss: {
    region: "",
    accessKeyId: "",
    accessKeySecret: "",
    bucket: "",
    endpoint: "",
    publicBaseUrl: ""
  }
};

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
}

function readString(record: Record<string, unknown> | null, key: string): string {
  if (!record) {
    return "";
  }
  const value = record[key];
  return typeof value === "string" ? value : "";
}

export function cloneImageHostingConfig(config: ImageHostingConfig): ImageHostingConfig {
  return {
    defaultProvider: config.defaultProvider,
    cloudflareR2: {
      ...config.cloudflareR2
    },
    aliyunOss: {
      ...config.aliyunOss
    }
  };
}

export function normalizeImageHostingConfig(input: unknown): ImageHostingConfig {
  const root = asRecord(input);
  // 向后兼容旧字段 activeProvider：优先读取 defaultProvider，缺失时回退到 activeProvider。
  const defaultProviderRaw = root?.defaultProvider;
  const activeProviderRaw = root?.activeProvider;
  const defaultProviderCandidate =
    defaultProviderRaw === "cloudflare-r2" || defaultProviderRaw === "aliyun-oss"
      ? defaultProviderRaw
      : activeProviderRaw;
  const defaultProvider: ImageHostingProvider =
    defaultProviderCandidate === "cloudflare-r2" || defaultProviderCandidate === "aliyun-oss"
      ? defaultProviderCandidate
      : "cloudflare-r2";

  const cloudflareR2 = asRecord(root?.cloudflareR2);
  const aliyunOss = asRecord(root?.aliyunOss);

  return {
    defaultProvider,
    cloudflareR2: {
      accountId: readString(cloudflareR2, "accountId"),
      accessKeyId: readString(cloudflareR2, "accessKeyId"),
      secretAccessKey: readString(cloudflareR2, "secretAccessKey"),
      bucket: readString(cloudflareR2, "bucket"),
      publicBaseUrl: readString(cloudflareR2, "publicBaseUrl")
    },
    aliyunOss: {
      region: readString(aliyunOss, "region"),
      accessKeyId: readString(aliyunOss, "accessKeyId"),
      accessKeySecret: readString(aliyunOss, "accessKeySecret"),
      bucket: readString(aliyunOss, "bucket"),
      endpoint: readString(aliyunOss, "endpoint"),
      publicBaseUrl: readString(aliyunOss, "publicBaseUrl")
    }
  };
}
