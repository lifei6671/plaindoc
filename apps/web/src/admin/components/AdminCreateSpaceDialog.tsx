import { ImagePlus, LoaderCircle, WandSparkles, X } from "lucide-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ChangeEvent,
  type PointerEvent as ReactPointerEvent,
  type WheelEvent as ReactWheelEvent
} from "react";
import { createPortal } from "react-dom";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../../components/ui/tabs";
import { Textarea } from "../../components/ui/textarea";
import type { AdminSpaceCategory, DataGateway, Visibility } from "../../data-access";
import { formatError } from "../../editor/status-utils";
import { showToast } from "../../components/ui/toast";

const COVER_FRAME_WIDTH = 260;
const COVER_FRAME_HEIGHT = 416;
const COVER_STAGE_WIDTH = 340;
const COVER_STAGE_HEIGHT = 540;
const COVER_MAX_WIDTH = 1600;
const COVER_MAX_HEIGHT = 2560;
const COVER_MAX_TITLE_LINES = 3;
const COVER_MAX_TITLE_UNITS_PER_LINE = 10.8;
const SPACE_ID_MAX_LENGTH = 26;
const SYSTEM_COVER_TEXT_MARGIN = 128;
const SYSTEM_COVER_FONT_FAMILY = "PingFang SC, Microsoft YaHei, Noto Sans SC, Source Han Sans SC, sans-serif";
const SYSTEM_COVER_LINE_HEIGHT_RATIO = 1.16;
const SYSTEM_COVER_BG_COLOR = "#d9e3f2";
const SYSTEM_COVER_TEXT_COLOR = "#2f3f5f";
const SYSTEM_COVER_TITLE_BASELINE_RATIO = 0.31;
const SPACE_ID_PATTERN = /^[A-Za-z0-9_-]+$/;

interface LoadedImageData {
  element: HTMLImageElement;
  width: number;
  height: number;
  objectURL: string;
  fileType: string;
  fileName: string;
}

// 封面仅做前端预览，点击“创建空间”时再统一上传到后端。
interface PendingCoverPreview {
  sourceLabel: "user_upload" | "system_generated";
  file: File;
  width: number;
  height: number;
  mimeType: string;
  previewUrl: string;
}

interface AdminCreateSpaceDialogProps {
  open: boolean;
  dataGateway: DataGateway;
  categoryOptions: AdminSpaceCategory[];
  onOpenChange: (open: boolean) => void;
  onCreated: () => Promise<void> | void;
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}

function computeFrameScale(image: LoadedImageData | null, zoom: number) {
  if (!image) {
    return {
      scale: 1,
      displayWidth: COVER_FRAME_WIDTH,
      displayHeight: COVER_FRAME_HEIGHT
    };
  }
  const baseScale = Math.max(COVER_FRAME_WIDTH / image.width, COVER_FRAME_HEIGHT / image.height);
  const scale = baseScale * zoom;
  return {
    scale,
    displayWidth: image.width * scale,
    displayHeight: image.height * scale
  };
}

function computeOffsetBounds(image: LoadedImageData | null, zoom: number) {
  const { displayWidth, displayHeight } = computeFrameScale(image, zoom);
  const maxOffsetX = Math.max(0, (displayWidth - COVER_FRAME_WIDTH) / 2);
  const maxOffsetY = Math.max(0, (displayHeight - COVER_FRAME_HEIGHT) / 2);
  return {
    minX: -maxOffsetX,
    maxX: maxOffsetX,
    minY: -maxOffsetY,
    maxY: maxOffsetY
  };
}

function normalizeVisibility(rawValue: string): Visibility {
  const value = rawValue.trim().toLowerCase();
  if (value === "public" || value === "authenticated" || value === "member") {
    return value;
  }
  return "member";
}

function normalizeSpaceID(rawValue: string): string {
  return rawValue.trim().toLowerCase();
}

function resolveSpaceIDValidationMessage(rawValue: string): string | null {
  const normalized = normalizeSpaceID(rawValue);
  if (!normalized) {
    return null;
  }
  if (normalized.length > SPACE_ID_MAX_LENGTH) {
    return `空间 ID 最多 ${SPACE_ID_MAX_LENGTH} 个字符`;
  }
  if (!SPACE_ID_PATTERN.test(normalized)) {
    return "空间 ID 仅支持英文字母、数字、下划线(_)和中横线(-)";
  }
  return null;
}

function estimateCoverTitleUnits(text: string): number {
  let units = 0;
  for (const char of text) {
    if (char === " ") {
      units += 0.4;
      continue;
    }
    if (/[\x20-\x7E]/.test(char)) {
      units += 0.58;
      continue;
    }
    units += 1;
  }
  return units;
}

function trimCoverTitleWithEllipsis(text: string): string {
  const runes = [...text];
  if (runes.length <= 1) {
    return "…";
  }
  return `${runes.slice(0, -1).join("")}…`;
}

function splitSystemCoverTitleLines(rawTitle: string): string[] {
  const normalized = rawTitle.trim().replace(/\s+/g, " ");
  if (!normalized) {
    return ["未命名空间"];
  }

  const tokens = normalized.match(/[A-Za-z0-9._-]+|\s+|./g) ?? [normalized];
  const lines: string[] = [];
  let current = "";

  const pushCurrent = () => {
    const value = current.trim();
    if (!value) {
      return;
    }
    lines.push(value);
  };

  for (let index = 0; index < tokens.length; index += 1) {
    const token = tokens[index];
    if (!token.trim()) {
      if (!current || current.endsWith(" ")) {
        continue;
      }
    }

    const next = current + token;
    if (estimateCoverTitleUnits(next) <= COVER_MAX_TITLE_UNITS_PER_LINE) {
      current = next;
      continue;
    }

    if (!current.trim()) {
      current = token.trim();
    }

    if (lines.length >= COVER_MAX_TITLE_LINES - 1) {
      const rest = `${current}${tokens.slice(index).join("")}`.trim();
      let candidate = rest;
      while (estimateCoverTitleUnits(candidate) > COVER_MAX_TITLE_UNITS_PER_LINE && candidate.length > 1) {
        candidate = trimCoverTitleWithEllipsis(candidate);
      }
      if (!candidate.endsWith("…")) {
        candidate = `${candidate}…`;
      }
      lines.push(candidate);
      return lines;
    }

    pushCurrent();
    current = token.trimStart();
  }

  pushCurrent();
  if (lines.length === 0) {
    return ["未命名空间"];
  }
  return lines.slice(0, COVER_MAX_TITLE_LINES);
}

function escapeSVGText(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&apos;");
}

let systemCoverMeasureContext: CanvasRenderingContext2D | null = null;

function getSystemCoverMeasureContext(): CanvasRenderingContext2D | null {
  if (typeof document === "undefined") {
    return null;
  }
  if (!systemCoverMeasureContext) {
    const canvas = document.createElement("canvas");
    systemCoverMeasureContext = canvas.getContext("2d");
  }
  return systemCoverMeasureContext;
}

function measureSystemCoverTextWidth(text: string, fontSize: number): number {
  // 关键函数：用真实字体测量文本宽度，避免换行后右侧被截断。
  const context = getSystemCoverMeasureContext();
  if (!context) {
    return estimateCoverTitleUnits(text) * fontSize;
  }
  context.font = `${fontSize}px ${SYSTEM_COVER_FONT_FAMILY}`;
  return context.measureText(text).width;
}

function buildSystemCoverSVG(spaceName: string): string {
  // 系统封面标题排版：先估算分行，再用真实测量缩放字号，保证不会溢出画布。
  const lines = splitSystemCoverTitleLines(spaceName);
  const maxUnits = Math.max(...lines.map((line) => estimateCoverTitleUnits(line)));
  let fontSize = maxUnits > 11 ? 148 : maxUnits > 9 ? 164 : 178;
  const maxTextWidth = COVER_MAX_WIDTH - SYSTEM_COVER_TEXT_MARGIN * 2;
  const measuredMaxWidth = Math.max(...lines.map((line) => measureSystemCoverTextWidth(line, fontSize)));
  if (measuredMaxWidth > maxTextWidth) {
    // 若真实测量超出画布宽度，则按比例缩小字号，避免右侧被裁切。
    const scale = maxTextWidth / measuredMaxWidth;
    fontSize = Math.max(1, Math.floor(fontSize * scale));
  }
  const lineHeight = Math.round(fontSize * SYSTEM_COVER_LINE_HEIGHT_RATIO);
  const startY = Math.round(COVER_MAX_HEIGHT * SYSTEM_COVER_TITLE_BASELINE_RATIO);

  const tspans = lines
    .map((line, index) => {
      const dy = index === 0 ? 0 : lineHeight;
      return `<tspan x="${SYSTEM_COVER_TEXT_MARGIN}" dy="${dy}">${escapeSVGText(line)}</tspan>`;
    })
    .join("");

  return `<svg xmlns="http://www.w3.org/2000/svg" width="${COVER_MAX_WIDTH}" height="${COVER_MAX_HEIGHT}" viewBox="0 0 ${COVER_MAX_WIDTH} ${COVER_MAX_HEIGHT}">
<rect width="100%" height="100%" fill="${SYSTEM_COVER_BG_COLOR}"/>
<g opacity="0.3">
  <ellipse cx="300" cy="2610" rx="880" ry="470" fill="#edf2fb"/>
</g>
<g opacity="0.52">
  <path d="M -260 2230 C 180 1880 670 1910 1070 2205 C 1230 2320 1360 2442 1470 2560 L -260 2560 Z" fill="#cad8ee"/>
</g>
<g opacity="0.64">
  <path d="M -230 1770 C 90 1600 420 1710 728 1988 C 938 2174 1098 2348 1222 2560 L 852 2560 C 733 2364 614 2226 472 2096 C 248 1888 18 1794 -230 1846 Z" fill="#b4caec"/>
</g>
<g opacity="0.68">
  <path d="M -320 1870 C -52 1722 190 1782 392 1944 C 584 2102 698 2280 768 2560 L 444 2560 C 382 2388 300 2260 185 2154 C 68 2042 -78 1972 -320 2000 Z" fill="#a2bcec"/>
</g>
<text x="${SYSTEM_COVER_TEXT_MARGIN}" y="${startY}" fill="${SYSTEM_COVER_TEXT_COLOR}" font-size="${fontSize}" font-weight="600" font-family="${SYSTEM_COVER_FONT_FAMILY}">
${tspans}
</text>
</svg>`;
}

async function exportSystemGeneratedWebP(
  spaceName: string,
  quality = 0.9
): Promise<{ file: File; width: number; height: number }> {
  const svg = buildSystemCoverSVG(spaceName);
  const svgBlob = new Blob([svg], { type: "image/svg+xml;charset=utf-8" });
  const svgURL = URL.createObjectURL(svgBlob);

  try {
    const image = await new Promise<HTMLImageElement>((resolve, reject) => {
      const value = new Image();
      value.onload = () => resolve(value);
      value.onerror = () => reject(new Error("系统封面渲染失败"));
      value.src = svgURL;
    });

    const canvas = document.createElement("canvas");
    canvas.width = COVER_MAX_WIDTH;
    canvas.height = COVER_MAX_HEIGHT;
    const context = canvas.getContext("2d");
    if (!context) {
      throw new Error("浏览器不支持 Canvas 2D，无法生成系统封面");
    }
    context.imageSmoothingEnabled = true;
    context.imageSmoothingQuality = "high";
    context.drawImage(image, 0, 0, COVER_MAX_WIDTH, COVER_MAX_HEIGHT);

    const webpBlob = await new Promise<Blob>((resolve, reject) => {
      canvas.toBlob(
        (value) => {
          if (!value) {
            reject(new Error("系统封面导出失败"));
            return;
          }
          resolve(value);
        },
        "image/webp",
        quality
      );
    });

    return {
      file: new File([webpBlob], `space-cover-${Date.now()}.webp`, {
        type: "image/webp",
        lastModified: Date.now()
      }),
      width: COVER_MAX_WIDTH,
      height: COVER_MAX_HEIGHT
    };
  } finally {
    URL.revokeObjectURL(svgURL);
  }
}

function readImageFile(file: File): Promise<LoadedImageData> {
  return new Promise((resolve, reject) => {
    const objectURL = URL.createObjectURL(file);
    const image = new Image();
    image.onload = () => {
      resolve({
        element: image,
        width: image.naturalWidth,
        height: image.naturalHeight,
        objectURL,
        fileType: file.type || "image/*",
        fileName: file.name || "cover"
      });
    };
    image.onerror = () => {
      URL.revokeObjectURL(objectURL);
      reject(new Error("读取图片失败，请更换图片重试"));
    };
    image.src = objectURL;
  });
}

async function exportCroppedWebP(
  image: LoadedImageData,
  zoom: number,
  offsetX: number,
  offsetY: number,
  quality = 0.86
): Promise<{ file: File; width: number; height: number }> {
  const { scale, displayWidth, displayHeight } = computeFrameScale(image, zoom);
  const imageLeft = COVER_FRAME_WIDTH / 2 + offsetX - displayWidth / 2;
  const imageTop = COVER_FRAME_HEIGHT / 2 + offsetY - displayHeight / 2;

  const sourceX = clamp((-imageLeft) / scale, 0, image.width);
  const sourceY = clamp((-imageTop) / scale, 0, image.height);
  const sourceWidth = clamp(COVER_FRAME_WIDTH / scale, 1, image.width-sourceX);
  const sourceHeight = clamp(COVER_FRAME_HEIGHT / scale, 1, image.height-sourceY);

  const downScale = Math.min(1, COVER_MAX_WIDTH / sourceWidth, COVER_MAX_HEIGHT / sourceHeight);
  const outputWidth = Math.max(1, Math.round(sourceWidth * downScale));
  const outputHeight = Math.max(1, Math.round(sourceHeight * downScale));

  const canvas = document.createElement("canvas");
  canvas.width = outputWidth;
  canvas.height = outputHeight;
  const context = canvas.getContext("2d");
  if (!context) {
    throw new Error("浏览器不支持 Canvas 2D，上载封面失败");
  }
  context.imageSmoothingEnabled = true;
  context.imageSmoothingQuality = "high";
  context.drawImage(
    image.element,
    sourceX,
    sourceY,
    sourceWidth,
    sourceHeight,
    0,
    0,
    outputWidth,
    outputHeight
  );

  const blob = await new Promise<Blob>((resolve, reject) => {
    canvas.toBlob(
      (value) => {
        if (!value) {
          reject(new Error("封面导出失败，请重试"));
          return;
        }
        resolve(value);
      },
      "image/webp",
      quality
    );
  });

  const safeName = image.fileName.replace(/\.[^.]+$/, "") || "space-cover";
  const file = new File([blob], `${safeName}.webp`, {
    type: "image/webp",
    lastModified: Date.now()
  });
  return {
    file,
    width: outputWidth,
    height: outputHeight
  };
}

export function AdminCreateSpaceDialog({ open, dataGateway, categoryOptions, onOpenChange, onCreated }: AdminCreateSpaceDialogProps) {
  // 管理后台新建空间弹窗：负责表单、封面本地预览与创建提交流程。
  const [spaceID, setSpaceID] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [visibility, setVisibility] = useState<Visibility>("member");
  const [categoryID, setCategoryID] = useState("");
  const [coverMode, setCoverMode] = useState<"user_upload" | "system_generated">("user_upload");

  const [loadedImage, setLoadedImage] = useState<LoadedImageData | null>(null);
  const [zoom, setZoom] = useState(1);
  const [offsetX, setOffsetX] = useState(0);
  const [offsetY, setOffsetY] = useState(0);

  const [uploadingCover, setUploadingCover] = useState(false);
  const [creatingSpace, setCreatingSpace] = useState(false);
  const [createProgressMessage, setCreateProgressMessage] = useState("");
  const [pendingCover, setPendingCover] = useState<PendingCoverPreview | null>(null);

  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const dragStartRef = useRef<{ pointerX: number; pointerY: number; offsetX: number; offsetY: number } | null>(null);
  const createSpaceInFlightRef = useRef(false);

  useEffect(() => {
    if (!open) {
      return;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        if (uploadingCover || creatingSpace) {
          return;
        }
        onOpenChange(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      document.body.style.overflow = previousOverflow;
    };
  }, [creatingSpace, onOpenChange, open, uploadingCover]);

  useEffect(() => {
    return () => {
      if (loadedImage?.objectURL) {
        URL.revokeObjectURL(loadedImage.objectURL);
      }
    };
  }, [loadedImage?.objectURL]);

  const frameInfo = useMemo(() => computeFrameScale(loadedImage, zoom), [loadedImage, zoom]);
  const normalizedCategoryOptions = useMemo(
    () =>
      categoryOptions
        .map((item) => ({
          categoryId: (item.categoryId ?? "").trim(),
          name: (item.name ?? "").trim(),
          isDefault: item.isDefault === true
        }))
        .filter((item) => item.categoryId.length > 0 && item.name.length > 0)
        .filter((item, index, items) => items.findIndex((current) => current.categoryId === item.categoryId) === index),
    [categoryOptions]
  );
  const defaultCategoryID = useMemo(
    () => normalizedCategoryOptions.find((item) => item.isDefault)?.categoryId ?? normalizedCategoryOptions[0]?.categoryId ?? "",
    [normalizedCategoryOptions]
  );
  const coverPreviewUrl = pendingCover?.previewUrl?.trim() || "";
  const spaceIDValidationMessage = useMemo(() => resolveSpaceIDValidationMessage(spaceID), [spaceID]);

  useEffect(() => {
    if (!open) {
      return;
    }
    if (categoryID || !defaultCategoryID) {
      return;
    }
    setCategoryID(defaultCategoryID);
  }, [categoryID, defaultCategoryID, open]);

  const clearPendingCover = useCallback(() => {
    setPendingCover((previous) => {
      if (previous?.previewUrl) {
        URL.revokeObjectURL(previous.previewUrl);
      }
      return null;
    });
  }, []);

  const resetDialog = useCallback(() => {
    setSpaceID("");
    setName("");
    setDescription("");
    setVisibility("member");
    setCategoryID(defaultCategoryID);
    setCoverMode("user_upload");
    setZoom(1);
    setOffsetX(0);
    setOffsetY(0);
    clearPendingCover();
    setUploadingCover(false);
    setCreatingSpace(false);
    setCreateProgressMessage("");
    createSpaceInFlightRef.current = false;
    setLoadedImage((previous) => {
      if (previous?.objectURL) {
        URL.revokeObjectURL(previous.objectURL);
      }
      return null;
    });
  }, [defaultCategoryID]);

  const closeDialog = useCallback(() => {
    if (uploadingCover || creatingSpace) {
      return;
    }
    resetDialog();
    onOpenChange(false);
  }, [creatingSpace, onOpenChange, resetDialog, uploadingCover]);

  const applyClampedOffsets = useCallback(
    (nextX: number, nextY: number, nextZoom: number = zoom) => {
      const bounds = computeOffsetBounds(loadedImage, nextZoom);
      setOffsetX(clamp(nextX, bounds.minX, bounds.maxX));
      setOffsetY(clamp(nextY, bounds.minY, bounds.maxY));
    },
    [loadedImage, zoom]
  );

  const handleZoomChange = useCallback(
    (event: ChangeEvent<HTMLInputElement>) => {
      const nextZoom = Number.parseFloat(event.target.value);
      if (!Number.isFinite(nextZoom)) {
        return;
      }
      setZoom(nextZoom);
      applyClampedOffsets(offsetX, offsetY, nextZoom);
    },
    [applyClampedOffsets, offsetX, offsetY]
  );

  const handlePickFile = useCallback(() => {
    fileInputRef.current?.click();
  }, []);

  const handleFileChange = useCallback(async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }
    try {
      clearPendingCover();
      const image = await readImageFile(file);
      setLoadedImage((previous) => {
        if (previous?.objectURL) {
          URL.revokeObjectURL(previous.objectURL);
        }
        return image;
      });
      setZoom(1);
      setOffsetX(0);
      setOffsetY(0);
    } catch (error) {
      showToast(`读取图片失败：${formatError(error)}`);
    } finally {
      event.target.value = "";
    }
  }, [clearPendingCover]);

  const handleFramePointerDown = useCallback((event: ReactPointerEvent<HTMLDivElement>) => {
    if (!loadedImage) {
      return;
    }
    dragStartRef.current = {
      pointerX: event.clientX,
      pointerY: event.clientY,
      offsetX,
      offsetY
    };
    event.currentTarget.setPointerCapture(event.pointerId);
  }, [loadedImage, offsetX, offsetY]);

  const handleFramePointerMove = useCallback((event: ReactPointerEvent<HTMLDivElement>) => {
    const dragState = dragStartRef.current;
    if (!dragState || !loadedImage) {
      return;
    }
    const deltaX = event.clientX - dragState.pointerX;
    const deltaY = event.clientY - dragState.pointerY;
    applyClampedOffsets(dragState.offsetX + deltaX, dragState.offsetY + deltaY);
  }, [applyClampedOffsets, loadedImage]);

  const handleFramePointerUp = useCallback((event: ReactPointerEvent<HTMLDivElement>) => {
    dragStartRef.current = null;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  }, []);

  const handleFrameWheel = useCallback((event: ReactWheelEvent<HTMLDivElement>) => {
    if (!loadedImage) {
      return;
    }
    event.preventDefault();
    const delta = event.deltaY < 0 ? 0.08 : -0.08;
    const nextZoom = clamp(Number((zoom + delta).toFixed(2)), 1, 3);
    if (nextZoom === zoom) {
      return;
    }
    setZoom(nextZoom);
    applyClampedOffsets(offsetX, offsetY, nextZoom);
  }, [applyClampedOffsets, loadedImage, offsetX, offsetY, zoom]);

  const handleUploadProcessedCover = useCallback(async () => {
    if (!loadedImage) {
      showToast("请先选择并裁剪图片");
      return;
    }
    setUploadingCover(true);
    try {
      // 仅生成本地预览文件，不在此处上传。
      const exported = await exportCroppedWebP(loadedImage, zoom, offsetX, offsetY);
      const previewUrl = URL.createObjectURL(exported.file);
      setPendingCover((previous) => {
        if (previous?.previewUrl) {
          URL.revokeObjectURL(previous.previewUrl);
        }
        return {
          sourceLabel: "user_upload",
          file: exported.file,
          width: exported.width,
          height: exported.height,
          mimeType: exported.file.type || "image/webp",
          previewUrl
        };
      });
      showToast("封面预览已生成", "success");
    } catch (error) {
      showToast(`封面预览失败：${formatError(error)}`);
    } finally {
      setUploadingCover(false);
    }
  }, [loadedImage, offsetX, offsetY, zoom]);

  const handleGenerateSystemCover = useCallback(async () => {
    const trimmedName = name.trim();
    if (!trimmedName) {
      showToast("请先填写空间名称");
      return;
    }
    setUploadingCover(true);
    try {
      // 系统封面在前端生成预览，创建时再统一上传。
      const generated = await exportSystemGeneratedWebP(trimmedName);
      const previewUrl = URL.createObjectURL(generated.file);
      setPendingCover((previous) => {
        if (previous?.previewUrl) {
          URL.revokeObjectURL(previous.previewUrl);
        }
        return {
          sourceLabel: "system_generated",
          file: generated.file,
          width: generated.width,
          height: generated.height,
          mimeType: generated.file.type || "image/webp",
          previewUrl
        };
      });
      showToast("系统封面预览已生成", "success");
    } catch (error) {
      showToast(`系统封面生成失败：${formatError(error)}`);
    } finally {
      setUploadingCover(false);
    }
  }, [name]);

  const handleCreateSpace = useCallback(async () => {
    // 同步锁：防止 React 状态刷新前的快速连点触发重复提交。
    if (createSpaceInFlightRef.current) {
      return;
    }

    const normalizedSpaceID = normalizeSpaceID(spaceID);
    const spaceIDValidationMessage = resolveSpaceIDValidationMessage(normalizedSpaceID);
    if (spaceIDValidationMessage) {
      showToast(spaceIDValidationMessage);
      return;
    }

    const trimmedName = name.trim();
    if (!trimmedName) {
      showToast("空间名称不能为空");
      return;
    }
    if (trimmedName.length > 120) {
      showToast("空间名称最多 120 个字符");
      return;
    }
    if (description.trim().length > 280) {
      showToast("空间简介最多 280 个字符");
      return;
    }

    createSpaceInFlightRef.current = true;
    setCreatingSpace(true);
    setCreateProgressMessage("正在准备创建...");
    try {
      // 封面创建策略：
      // 1) 用户已生成/上传封面预览：沿用该预览文件上传；
      // 2) 未提供封面：前端自动生成系统封面，再继续创建空间。
      let preparedCover = pendingCover;
      if (!preparedCover) {
        setCreateProgressMessage("正在自动生成封面...");
        const generated = await exportSystemGeneratedWebP(trimmedName);
        preparedCover = {
          sourceLabel: "system_generated",
          file: generated.file,
          width: generated.width,
          height: generated.height,
          mimeType: generated.file.type || "image/webp",
          previewUrl: ""
        };
      }

      let coverAssetId: string | undefined;
      setCreateProgressMessage("正在上传封面...");
      const payload = await dataGateway.admin.createSpaceCoverAsset({
        source: "user_upload",
        file: preparedCover.file,
        clientWidth: preparedCover.width,
        clientHeight: preparedCover.height,
        clientMimeType: preparedCover.mimeType,
        clientProcessed: true
      });
      coverAssetId = payload.assetId;

      setCreateProgressMessage("正在创建空间...");
      await dataGateway.admin.createSpace({
        spaceId: normalizedSpaceID || undefined,
        name: trimmedName,
        description,
        categoryId: categoryID,
        visibility: normalizeVisibility(visibility),
        coverAssetId
      });
      showToast("空间创建成功", "success");
      await onCreated();
      resetDialog();
      onOpenChange(false);
    } catch (error) {
      showToast(`创建空间失败：${formatError(error)}`);
    } finally {
      createSpaceInFlightRef.current = false;
      setCreatingSpace(false);
      setCreateProgressMessage("");
    }
  }, [categoryID, dataGateway.admin, description, name, onCreated, onOpenChange, pendingCover, resetDialog, spaceID, visibility]);

  if (!open) {
    return null;
  }

  return createPortal(
    <div
      className="fixed inset-0 z-[2750] flex items-center justify-center bg-slate-900/45 p-4 backdrop-blur-sm"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) {
          closeDialog();
        }
      }}
    >
      <section className="grid max-h-[92vh] w-full max-w-[980px] grid-rows-[auto_minmax(0,1fr)_auto] overflow-hidden rounded-xl border border-slate-200 bg-white shadow-[0_30px_80px_rgba(15,23,42,0.25)]">
        <header className="flex items-start justify-between border-b border-slate-200 bg-gradient-to-r from-slate-50 to-sky-50 px-5 py-4">
          <div className="space-y-1">
            <h3 className="text-lg font-semibold text-slate-900">新建空间</h3>
            <p className="text-xs text-slate-600">支持封面上传裁剪或系统自动生成，创建后立即生效。</p>
          </div>
          <Button type="button" size="icon" variant="ghost" className="h-8 w-8" disabled={uploadingCover || creatingSpace} onClick={closeDialog}>
            <X size={16} />
          </Button>
        </header>

        <div className="grid min-h-0 gap-5 overflow-y-auto px-5 py-4 lg:grid-cols-[minmax(0,1.1fr)_320px]">
          <div className="space-y-4">
            <div className="grid gap-3 md:grid-cols-2">
              <label className="space-y-1.5 md:col-span-2">
                <span className="text-xs font-semibold text-slate-700">空间 ID（可选）</span>
                <Input
                  value={spaceID}
                  maxLength={SPACE_ID_MAX_LENGTH}
                  placeholder="留空自动生成，例如：linux_docs"
                  onChange={(event) => setSpaceID(event.target.value)}
                />
                <small className={`block text-[11px] ${spaceIDValidationMessage ? "text-rose-600" : "text-slate-500"}`}>
                  {spaceIDValidationMessage ?? "仅支持英文字母、数字、下划线(_)和中横线(-)，创建后不可修改"}
                </small>
              </label>
              <label className="space-y-1.5 md:col-span-2">
                <span className="text-xs font-semibold text-slate-700">空间名称</span>
                <Input value={name} maxLength={120} placeholder="例如：品牌设计协作空间" onChange={(event) => setName(event.target.value)} />
                <small className="block text-[11px] text-slate-500">{name.trim().length}/120</small>
              </label>
              <label className="space-y-1.5 md:col-span-2">
                <span className="text-xs font-semibold text-slate-700">空间简介</span>
                <Textarea
                  rows={4}
                  value={description}
                  maxLength={280}
                  placeholder="可选，用于在空间列表中展示该空间用途。"
                  onChange={(event) => setDescription(event.target.value)}
                />
                <small className="block text-[11px] text-slate-500">{description.trim().length}/280</small>
              </label>
              <label className="space-y-1.5">
                <span className="text-xs font-semibold text-slate-700">可见性</span>
                <Select value={visibility} onValueChange={(value) => setVisibility(normalizeVisibility(value))}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent className="z-[2900]">
                    <SelectItem value="public">完全公开（public）</SelectItem>
                    <SelectItem value="authenticated">登录可见（authenticated）</SelectItem>
                    <SelectItem value="member">成员可见（member）</SelectItem>
                  </SelectContent>
                </Select>
              </label>
              <label className="space-y-1.5">
                <span className="text-xs font-semibold text-slate-700">分类</span>
                <Select
                  value={categoryID}
                  onValueChange={(value) => setCategoryID(value)}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="未分类" />
                  </SelectTrigger>
                  <SelectContent className="z-[2900]">
                    {normalizedCategoryOptions.length === 0 ? (
                      <SelectItem value="__PD_NO_CATEGORY_OPTIONS__" disabled>
                        暂无可选分类
                      </SelectItem>
                    ) : (
                      normalizedCategoryOptions.map((option) => (
                        <SelectItem key={option.categoryId} value={option.categoryId}>
                          {option.name}
                        </SelectItem>
                      ))
                    )}
                  </SelectContent>
                </Select>
              </label>
            </div>

            <div className="rounded-lg border border-slate-200 bg-slate-50/60 p-3">
              <Tabs value={coverMode} onValueChange={(value) => setCoverMode(value as "user_upload" | "system_generated")}>
                <TabsList className="h-auto gap-1.5 rounded-md bg-white p-1">
                  <TabsTrigger value="user_upload" className="rounded-md px-3 py-1.5 data-[state=active]:bg-slate-900 data-[state=active]:text-white">
                    上传封面
                  </TabsTrigger>
                  <TabsTrigger value="system_generated" className="rounded-md px-3 py-1.5 data-[state=active]:bg-slate-900 data-[state=active]:text-white">
                    系统生成
                  </TabsTrigger>
                </TabsList>

                <TabsContent value="user_upload" className="space-y-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <input ref={fileInputRef} type="file" accept="image/*" className="hidden" onChange={handleFileChange} />
                    <Button type="button" variant="outline" onClick={handlePickFile} disabled={uploadingCover || creatingSpace}>
                      <ImagePlus size={15} />
                      选择图片
                    </Button>
                    <Button
                      type="button"
                      variant="default"
                      disabled={!loadedImage || uploadingCover || creatingSpace}
                      onClick={() => void handleUploadProcessedCover()}
                    >
                      {uploadingCover ? <LoaderCircle size={15} className="animate-spin" /> : <ImagePlus size={15} />}
                      裁剪并预览
                    </Button>
                    <span className="text-xs text-slate-500">目标比例 5:8，前端导出 WebP 仅用于预览，创建时上传</span>
                  </div>

                  <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_200px]">
                    <div className="space-y-2 rounded-md border border-dashed border-slate-300 bg-white p-3">
                      <p className="text-xs font-medium text-slate-700">裁剪预览（拖拽平移，滚轮缩放）</p>
                      <div
                        className="relative mx-auto cursor-grab overflow-hidden rounded-md border border-slate-300 bg-slate-100 shadow-inner active:cursor-grabbing"
                        style={{
                          width: `${COVER_STAGE_WIDTH}px`,
                          height: `${COVER_STAGE_HEIGHT}px`,
                          touchAction: "none"
                        }}
                        onPointerDown={handleFramePointerDown}
                        onPointerMove={handleFramePointerMove}
                        onPointerUp={handleFramePointerUp}
                        onPointerCancel={handleFramePointerUp}
                        onWheel={handleFrameWheel}
                      >
                        {loadedImage ? (
                          <>
                            <img
                              src={loadedImage.objectURL}
                              alt="crop-preview"
                              draggable={false}
                              className="pointer-events-none absolute left-1/2 top-1/2 select-none"
                              style={{
                                width: `${frameInfo.displayWidth}px`,
                                height: `${frameInfo.displayHeight}px`,
                                transform: `translate(-50%, -50%) translate(${offsetX}px, ${offsetY}px)`
                              }}
                            />
                            <div
                              className="pointer-events-none absolute left-1/2 top-1/2 rounded-md border border-white/90 shadow-[0_0_0_9999px_rgba(15,23,42,0.45)]"
                              style={{
                                width: `${COVER_FRAME_WIDTH}px`,
                                height: `${COVER_FRAME_HEIGHT}px`,
                                transform: "translate(-50%, -50%)"
                              }}
                            />
                          </>
                        ) : (
                          <div className="flex h-full items-center justify-center text-xs text-slate-500">请先选择图片</div>
                        )}
                      </div>
                      <label className="grid gap-1.5">
                        <span className="text-xs font-medium text-slate-700">缩放</span>
                        <input
                          type="range"
                          min={1}
                          max={3}
                          step={0.01}
                          value={zoom}
                          disabled={!loadedImage}
                          onChange={handleZoomChange}
                        />
                      </label>
                    </div>
                    <div className="rounded-md border border-slate-200 bg-white p-3 text-xs text-slate-600">
                      <p className="font-medium text-slate-800">操作说明</p>
                      <ul className="mt-2 list-disc space-y-1 pl-4">
                        <li>支持 `jpg/png/webp` 上传</li>
                        <li>拖拽平移，滚轮或滑杆缩放</li>
                        <li>裁剪框外区域为半透明蒙层</li>
                        <li>前端导出 WebP，后端再次校验与规范化</li>
                      </ul>
                    </div>
                  </div>
                </TabsContent>

                <TabsContent value="system_generated" className="space-y-3">
                  <p className="text-xs text-slate-600">系统将基于当前“空间名称”在前端生成简约风 SVG，再转 Canvas 导出 WebP 作为预览，创建时上传。</p>
                  <Button
                    type="button"
                    disabled={uploadingCover || creatingSpace || !name.trim()}
                    onClick={() => void handleGenerateSystemCover()}
                  >
                    {uploadingCover ? <LoaderCircle size={15} className="animate-spin" /> : <WandSparkles size={15} />}
                    生成并预览
                  </Button>
                </TabsContent>
              </Tabs>
            </div>
          </div>

          <aside className="space-y-3">
            <div className="rounded-lg border border-slate-200 bg-white p-3">
              <p className="text-xs font-semibold text-slate-700">封面预览</p>
              <div className="mt-2 flex justify-center rounded-md border border-slate-200 bg-slate-50 p-2">
                <div className="relative h-[320px] w-[200px] overflow-hidden rounded-md bg-slate-200">
                  {coverPreviewUrl ? (
                    <img src={coverPreviewUrl} alt="space-cover-preview" className="h-full w-full object-cover" />
                  ) : (
                    <div className="flex h-full items-center justify-center px-4 text-center text-xs text-slate-500">
                      尚未设置封面，创建后可继续编辑
                    </div>
                  )}
                </div>
              </div>
              {pendingCover ? (
                <div className="mt-2 rounded-md bg-slate-50 p-2 text-[11px] text-slate-600">
                  <p>来源：{pendingCover.sourceLabel === "system_generated" ? "系统生成" : "用户上传"}</p>
                  <p>尺寸：{pendingCover.width} × {pendingCover.height}</p>
                  <p>格式：{pendingCover.mimeType}</p>
                </div>
              ) : null}
            </div>
          </aside>
        </div>

        <footer className="flex items-center justify-end gap-2 border-t border-slate-200 px-5 py-3">
          {creatingSpace ? (
            <div className="mr-auto inline-flex items-center gap-1.5 rounded-md bg-slate-100 px-2.5 py-1 text-xs text-slate-600">
              <LoaderCircle size={14} className="animate-spin" />
              <span>{createProgressMessage || "处理中..."}</span>
            </div>
          ) : null}
          <Button type="button" variant="outline" disabled={uploadingCover || creatingSpace} onClick={closeDialog}>
            取消
          </Button>
          <Button
            type="button"
            disabled={creatingSpace || uploadingCover}
            aria-busy={creatingSpace}
            className="min-w-[112px] transition-all duration-200"
            onClick={() => void handleCreateSpace()}
          >
            {creatingSpace ? <LoaderCircle size={15} className="animate-spin" /> : null}
            {creatingSpace ? "处理中..." : "创建空间"}
          </Button>
        </footer>
      </section>
    </div>,
    document.body
  );
}
