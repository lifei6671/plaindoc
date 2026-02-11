import { PutObjectCommand, S3Client } from "@aws-sdk/client-s3";
import type { ImageHostingConfig, ImageHostingProvider } from "./image-hosting";

// 粘贴图片上传结果：用于在编辑器中回填 Markdown 图片链接。
export interface UploadImageResult {
  provider: ImageHostingProvider;
  key: string;
  url: string;
}

interface UploadContext {
  config: ImageHostingConfig;
  file: File;
  objectKey: string;
}

// 入口函数：按“默认图床”路由到对应上传实现。
export async function uploadImageToDefaultHosting(
  config: ImageHostingConfig,
  file: File
): Promise<UploadImageResult> {
  try {
    if (!file.type.startsWith("image/")) {
      throw new Error("仅支持上传图片类型文件");
    }

    const objectKey = buildObjectKey(file);
    const context: UploadContext = {
      config,
      file,
      objectKey
    };

    if (config.defaultProvider === "cloudflare-r2") {
      return uploadToCloudflareR2(context);
    }

    return uploadToAliyunOss(context);
  } catch (error) {
    console.error("[image-upload] 默认图床上传失败", {
      provider: config.defaultProvider,
      fileName: file.name || "未命名图片",
      fileType: file.type,
      error
    });
    throw error;
  }
}

// Cloudflare R2 上传：使用 S3 兼容 API 进行 PUT Object。
async function uploadToCloudflareR2(context: UploadContext): Promise<UploadImageResult> {
  try {
    const { cloudflareR2 } = context.config;
    if (
      !cloudflareR2.accountId ||
      !cloudflareR2.accessKeyId ||
      !cloudflareR2.secretAccessKey ||
      !cloudflareR2.bucket
    ) {
      throw new Error("Cloudflare R2 配置不完整，请检查 Account ID、Access Key、Secret 与 Bucket");
    }

    const endpoint = `https://${cloudflareR2.accountId}.r2.cloudflarestorage.com`;
    const s3Client = new S3Client({
      region: "auto",
      endpoint,
      forcePathStyle: true,
      credentials: {
        accessKeyId: cloudflareR2.accessKeyId,
        secretAccessKey: cloudflareR2.secretAccessKey
      }
    });

    // 浏览器端直接传 File/Blob 会被 SDK 判定为流式 body，
    // 从而触发 aws-chunked 编码分支；部分环境下该分支会出现
    // readableStream.getReader 兼容异常，因此这里转成 Uint8Array
    // 强制走非流式上传路径。
    const fileBytes = new Uint8Array(await context.file.arrayBuffer());

    await s3Client.send(
      new PutObjectCommand({
        Bucket: cloudflareR2.bucket,
        Key: context.objectKey,
        Body: fileBytes,
        ContentType: context.file.type || "application/octet-stream"
      })
    );

    return {
      provider: "cloudflare-r2",
      key: context.objectKey,
      // 自定义公网域名通常已绑定到 bucket 根路径，此时只拼接 object key。
      url: cloudflareR2.publicBaseUrl.trim()
        ? resolvePublicUrl(cloudflareR2.publicBaseUrl, context.objectKey, endpoint)
        : resolvePublicUrl("", `${cloudflareR2.bucket}/${context.objectKey}`, endpoint)
    };
  } catch (error) {
    console.error("[image-upload][cloudflare-r2] 上传失败", {
      key: context.objectKey,
      bucket: context.config.cloudflareR2.bucket,
      error
    });
    throw error;
  }
}

// 阿里云 OSS 上传：浏览器端生成签名并发起 PUT 请求。
async function uploadToAliyunOss(context: UploadContext): Promise<UploadImageResult> {
  try {
    const { aliyunOss } = context.config;
    if (!aliyunOss.accessKeyId || !aliyunOss.accessKeySecret || !aliyunOss.bucket) {
      throw new Error("阿里云 OSS 配置不完整，请检查 Access Key、Secret 与 Bucket");
    }

    const endpointUrl = resolveAliyunEndpointUrl(aliyunOss.endpoint, aliyunOss.region);
    const uploadBaseUrl = resolveAliyunUploadBaseUrl(endpointUrl, aliyunOss.bucket);
    const encodedObjectKey = encodeObjectKey(context.objectKey);
    const uploadUrl = `${uploadBaseUrl}/${encodedObjectKey}`;
    const date = new Date().toUTCString();
    const contentType = context.file.type || "application/octet-stream";
    const stringToSign = `PUT\n\n${contentType}\n${date}\n/${aliyunOss.bucket}/${context.objectKey}`;
    const signature = await signWithHmacSha1(aliyunOss.accessKeySecret, stringToSign);

    const response = await fetch(uploadUrl, {
      method: "PUT",
      headers: {
        "Content-Type": contentType,
        Date: date,
        Authorization: `OSS ${aliyunOss.accessKeyId}:${signature}`
      },
      body: context.file
    });

    if (!response.ok) {
      const responseText = await response.text();
      throw new Error(`阿里云 OSS 上传失败（${response.status}）：${responseText || "未知错误"}`);
    }

    return {
      provider: "aliyun-oss",
      key: context.objectKey,
      url: resolvePublicUrl(aliyunOss.publicBaseUrl, context.objectKey, uploadBaseUrl)
    };
  } catch (error) {
    console.error("[image-upload][aliyun-oss] 上传失败", {
      key: context.objectKey,
      bucket: context.config.aliyunOss.bucket,
      endpoint: context.config.aliyunOss.endpoint,
      region: context.config.aliyunOss.region,
      error
    });
    throw error;
  }
}

// 生成对象 key：按日期分层，避免单目录对象过多。
function buildObjectKey(file: File): string {
  const date = new Date();
  const yyyy = String(date.getFullYear());
  const mm = String(date.getMonth() + 1).padStart(2, "0");
  const dd = String(date.getDate()).padStart(2, "0");
  const random = Math.random().toString(36).slice(2, 10);
  const extension = resolveFileExtension(file);
  return `plaindoc/${yyyy}/${mm}/${dd}/${Date.now()}-${random}.${extension}`;
}

function resolveFileExtension(file: File): string {
  const nameMatch = file.name.match(/\.([a-zA-Z0-9]+)$/);
  if (nameMatch && nameMatch[1]) {
    return nameMatch[1].toLowerCase();
  }

  const mimeToExtension: Record<string, string> = {
    "image/jpeg": "jpg",
    "image/png": "png",
    "image/gif": "gif",
    "image/webp": "webp",
    "image/svg+xml": "svg",
    "image/bmp": "bmp",
    "image/tiff": "tif"
  };
  return mimeToExtension[file.type] ?? "png";
}

function encodeObjectKey(objectKey: string): string {
  return objectKey.split("/").map((segment) => encodeURIComponent(segment)).join("/");
}

function resolvePublicUrl(baseUrl: string, objectPath: string, fallbackBaseUrl: string): string {
  const base = baseUrl.trim() ? baseUrl.trim() : fallbackBaseUrl;
  return `${base.replace(/\/+$/, "")}/${objectPath.replace(/^\/+/, "")}`;
}

function resolveAliyunEndpointUrl(endpoint: string, region: string): URL {
  const normalizedEndpoint = endpoint.trim();
  if (normalizedEndpoint) {
    const withProtocol = /^https?:\/\//i.test(normalizedEndpoint)
      ? normalizedEndpoint
      : `https://${normalizedEndpoint}`;
    return new URL(withProtocol);
  }

  if (!region.trim()) {
    throw new Error("阿里云 OSS 需要配置 Endpoint 或 Region");
  }

  return new URL(`https://${region.trim()}.aliyuncs.com`);
}

function resolveAliyunUploadBaseUrl(endpointUrl: URL, bucket: string): string {
  const protocol = endpointUrl.protocol || "https:";
  const hostname = endpointUrl.hostname;
  const port = endpointUrl.port ? `:${endpointUrl.port}` : "";
  const bucketPrefix = `${bucket}.`;
  const host = hostname.startsWith(bucketPrefix) ? hostname : `${bucket}.${hostname}`;
  return `${protocol}//${host}${port}`;
}

async function signWithHmacSha1(secret: string, payload: string): Promise<string> {
  if (!globalThis.crypto?.subtle) {
    throw new Error("当前浏览器不支持 WebCrypto，无法生成 OSS 上传签名");
  }

  const textEncoder = new TextEncoder();
  const cryptoKey = await globalThis.crypto.subtle.importKey(
    "raw",
    textEncoder.encode(secret),
    {
      name: "HMAC",
      hash: "SHA-1"
    },
    false,
    ["sign"]
  );

  const signatureBuffer = await globalThis.crypto.subtle.sign("HMAC", cryptoKey, textEncoder.encode(payload));
  return encodeBase64(new Uint8Array(signatureBuffer));
}

function encodeBase64(bytes: Uint8Array): string {
  let binary = "";
  const chunkSize = 0x8000;
  for (let offset = 0; offset < bytes.length; offset += chunkSize) {
    const chunk = bytes.subarray(offset, offset + chunkSize);
    binary += String.fromCharCode(...chunk);
  }
  return btoa(binary);
}
