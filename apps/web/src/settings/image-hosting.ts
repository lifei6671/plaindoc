export type ImageHostingProvider = "cloudflare-r2" | "aliyun-oss" | "local";
export type ImageHostingImageProcessingMode = "to_webp" | "same_format";
export type ImageHostingImageQualityPreset = "original" | "high" | "standard" | "saver";

export interface ImageHostingImageProcessingConfig {
  mode: ImageHostingImageProcessingMode;
  qualityPreset: ImageHostingImageQualityPreset;
  maxWidth: number;
  maxHeight: number;
  skipAnimated: boolean;
}

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
  imageProcessing: ImageHostingImageProcessingConfig;
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
  },
  imageProcessing: {
    mode: "same_format",
    qualityPreset: "standard",
    maxWidth: 8000,
    maxHeight: 5000,
    skipAnimated: true
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

function readInteger(record: Record<string, unknown> | null, key: string, fallback: number): number {
  if (!record) {
    return fallback;
  }
  const value = record[key];
  if (typeof value === "number" && Number.isFinite(value)) {
    return Math.trunc(value);
  }
  if (typeof value === "string") {
    const parsed = Number.parseInt(value, 10);
    if (Number.isFinite(parsed)) {
      return parsed;
    }
  }
  return fallback;
}

function readBoolean(record: Record<string, unknown> | null, key: string, fallback: boolean): boolean {
  if (!record) {
    return fallback;
  }
  const value = record[key];
  return typeof value === "boolean" ? value : fallback;
}

function normalizeImageProcessingMode(input: unknown): ImageHostingImageProcessingMode {
  return input === "to_webp" ? "to_webp" : "same_format";
}

function normalizeImageQualityPreset(input: unknown): ImageHostingImageQualityPreset {
  switch (input) {
    case "original":
      return "original";
    case "high":
      return "high";
    case "saver":
      return "saver";
    case "standard":
    default:
      return "standard";
  }
}

function normalizeImageMaxWidth(value: number): number {
  if (!Number.isFinite(value)) {
    return DEFAULT_IMAGE_HOSTING_CONFIG.imageProcessing.maxWidth;
  }
  const normalized = Math.trunc(value);
  if (normalized < 256 || normalized > 20000) {
    return DEFAULT_IMAGE_HOSTING_CONFIG.imageProcessing.maxWidth;
  }
  return normalized;
}

function normalizeImageMaxHeight(value: number): number {
  if (!Number.isFinite(value)) {
    return DEFAULT_IMAGE_HOSTING_CONFIG.imageProcessing.maxHeight;
  }
  const normalized = Math.trunc(value);
  if (normalized < 256 || normalized > 20000) {
    return DEFAULT_IMAGE_HOSTING_CONFIG.imageProcessing.maxHeight;
  }
  return normalized;
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
    },
    imageProcessing: {
      ...config.imageProcessing
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
  const imageProcessing = asRecord(root?.imageProcessing);
  const hasMaxWidth = imageProcessing !== null && Object.prototype.hasOwnProperty.call(imageProcessing, "maxWidth");
  const hasMaxHeight = imageProcessing !== null && Object.prototype.hasOwnProperty.call(imageProcessing, "maxHeight");
  const legacyMaxPixels = readInteger(imageProcessing, "maxPixels", 0);
  const derivedLegacyDimension =
    legacyMaxPixels > 0 ? Math.max(256, Math.min(20000, Math.floor(Math.sqrt(legacyMaxPixels)))) : 0;

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
    },
    imageProcessing: {
      mode: normalizeImageProcessingMode(readString(imageProcessing, "mode")),
      qualityPreset: normalizeImageQualityPreset(readString(imageProcessing, "qualityPreset")),
      maxWidth: normalizeImageMaxWidth(
        readInteger(
          imageProcessing,
          "maxWidth",
          hasMaxWidth
            ? DEFAULT_IMAGE_HOSTING_CONFIG.imageProcessing.maxWidth
            : derivedLegacyDimension || DEFAULT_IMAGE_HOSTING_CONFIG.imageProcessing.maxWidth
        )
      ),
      maxHeight: normalizeImageMaxHeight(
        readInteger(
          imageProcessing,
          "maxHeight",
          hasMaxHeight
            ? DEFAULT_IMAGE_HOSTING_CONFIG.imageProcessing.maxHeight
            : derivedLegacyDimension || DEFAULT_IMAGE_HOSTING_CONFIG.imageProcessing.maxHeight
        )
      ),
      skipAnimated: readBoolean(
        imageProcessing,
        "skipAnimated",
        DEFAULT_IMAGE_HOSTING_CONFIG.imageProcessing.skipAnimated
      )
    }
  };
}
