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
  uploadLocalImage?: (file: File) => Promise<{ key: string; url: string }>;
}

interface UploadImageToDefaultHostingOptions {
  // 统一上传回调：由后端接收文件并按服务端配置落盘/入对象存储。
  uploadLocalImage?: (file: File) => Promise<{ key: string; url: string }>;
}

// 入口函数：仅走后端上传，前端不直接调用任何对象存储 SDK 或签名接口。
export async function uploadImageToDefaultHosting(
  config: ImageHostingConfig,
  file: File,
  options: UploadImageToDefaultHostingOptions = {}
): Promise<UploadImageResult> {
  try {
    if (!file.type.startsWith("image/")) {
      throw new Error("仅支持上传图片类型文件");
    }
    const context: UploadContext = {
      config,
      file,
      uploadLocalImage: options.uploadLocalImage
    };
    return uploadToBackend(context);
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

// 所有 provider 统一经后端上传，前端只负责发送文件并回填 URL。
async function uploadToBackend(context: UploadContext): Promise<UploadImageResult> {
  if (!context.uploadLocalImage) {
    throw new Error("未配置后端上传能力，请检查 HTTP 数据驱动与上传接口");
  }
  const uploaded = await context.uploadLocalImage(context.file);
  const objectKey = uploaded.key?.trim();
  const imageURL = uploaded.url.trim();
  if (!objectKey) {
    throw new Error("后端上传返回的对象 key 为空");
  }
  if (!imageURL) {
    throw new Error("后端上传返回的图片地址为空");
  }
  return {
    provider: context.config.defaultProvider as ImageHostingProvider,
    key: objectKey,
    url: imageURL
  };
}
