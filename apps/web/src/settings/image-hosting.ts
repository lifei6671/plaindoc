export type ImageHostingProvider = "cloudflare-r2" | "aliyun-oss" | "local";

export interface CloudflareR2Config {
  accountId: string;
  accessKeyId: string;
  secretAccessKey: string;
  bucket: string;
  publicBaseUrl: string;
  uploadPathTemplate: string;
}

export interface AliyunOssConfig {
  region: string;
  accessKeyId: string;
  accessKeySecret: string;
  bucket: string;
  endpoint: string;
  publicBaseUrl: string;
  uploadPathTemplate: string;
}

export interface LocalImageHostingConfig {
  uploadEndpoint: string;
  publicBaseUrl: string;
  uploadPathTemplate: string;
}

export interface ImageHostingConfig {
  // 默认图床提供商：粘贴图片时按该设置自动上传。
  defaultProvider: ImageHostingProvider;
  cloudflareR2: CloudflareR2Config;
  aliyunOss: AliyunOssConfig;
  local: LocalImageHostingConfig;
}

export const DEFAULT_IMAGE_HOSTING_CONFIG: ImageHostingConfig = {
  defaultProvider: "local",
  cloudflareR2: {
    accountId: "",
    accessKeyId: "",
    secretAccessKey: "",
    bucket: "",
    publicBaseUrl: "",
    uploadPathTemplate: "images/{spaceId}/{docId}/{yyyy}/{mm}/{dd}/{assetId}.{ext}"
  },
  aliyunOss: {
    region: "",
    accessKeyId: "",
    accessKeySecret: "",
    bucket: "",
    endpoint: "",
    publicBaseUrl: "",
    uploadPathTemplate: "images/{spaceId}/{docId}/{yyyy}/{mm}/{dd}/{assetId}.{ext}"
  },
  local: {
    uploadEndpoint: "/api/uploads/images",
    publicBaseUrl: "/uploads",
    uploadPathTemplate: "images/{spaceId}/{docId}/{yyyy}/{mm}/{dd}/{assetId}.{ext}"
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
    },
    local: {
      ...config.local
    }
  };
}

export function normalizeImageHostingConfig(input: unknown): ImageHostingConfig {
  const root = asRecord(input);
  // 向后兼容旧字段 activeProvider：优先读取 defaultProvider，缺失时回退到 activeProvider。
  const defaultProviderRaw = root?.defaultProvider;
  const activeProviderRaw = root?.activeProvider;
  const defaultProviderCandidate =
    defaultProviderRaw === "cloudflare-r2" ||
    defaultProviderRaw === "aliyun-oss" ||
    defaultProviderRaw === "local"
      ? defaultProviderRaw
      : activeProviderRaw;
  const defaultProvider: ImageHostingProvider =
    defaultProviderCandidate === "cloudflare-r2" ||
    defaultProviderCandidate === "aliyun-oss" ||
    defaultProviderCandidate === "local"
      ? defaultProviderCandidate
      : "local";

  const cloudflareR2 = asRecord(root?.cloudflareR2);
  const aliyunOss = asRecord(root?.aliyunOss);
  const local = asRecord(root?.local);

  return {
    defaultProvider,
    cloudflareR2: {
      accountId: readString(cloudflareR2, "accountId"),
      accessKeyId: readString(cloudflareR2, "accessKeyId"),
      secretAccessKey: readString(cloudflareR2, "secretAccessKey"),
      bucket: readString(cloudflareR2, "bucket"),
      publicBaseUrl: readString(cloudflareR2, "publicBaseUrl"),
      uploadPathTemplate:
        readString(cloudflareR2, "uploadPathTemplate") ||
        DEFAULT_IMAGE_HOSTING_CONFIG.cloudflareR2.uploadPathTemplate
    },
    aliyunOss: {
      region: readString(aliyunOss, "region"),
      accessKeyId: readString(aliyunOss, "accessKeyId"),
      accessKeySecret: readString(aliyunOss, "accessKeySecret"),
      bucket: readString(aliyunOss, "bucket"),
      endpoint: readString(aliyunOss, "endpoint"),
      publicBaseUrl: readString(aliyunOss, "publicBaseUrl"),
      uploadPathTemplate:
        readString(aliyunOss, "uploadPathTemplate") || DEFAULT_IMAGE_HOSTING_CONFIG.aliyunOss.uploadPathTemplate
    },
    local: {
      uploadEndpoint:
        readString(local, "uploadEndpoint") || DEFAULT_IMAGE_HOSTING_CONFIG.local.uploadEndpoint,
      publicBaseUrl:
        readString(local, "publicBaseUrl") || DEFAULT_IMAGE_HOSTING_CONFIG.local.publicBaseUrl,
      uploadPathTemplate:
        readString(local, "uploadPathTemplate") || DEFAULT_IMAGE_HOSTING_CONFIG.local.uploadPathTemplate
    }
  };
}
