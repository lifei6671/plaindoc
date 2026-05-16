import { Archive, Download, FileArchive, LoaderCircle, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { Checkbox } from "../../components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { showToast } from "../../components/ui/toast";
import {
  type AdminSpace,
  type AdminSpaceExportFormat,
  type AdminSpaceTransferEvent,
  type AdminSpaceTransferSubscription,
  type DataGateway
} from "../../data-access";
import { formatError } from "../../editor/status-utils";

interface AdminSpaceExportDialogProps {
  open: boolean;
  space: AdminSpace | null;
  dataGateway: DataGateway;
  onOpenChange(open: boolean): void;
}

function renderExportFormatLabel(format: AdminSpaceExportFormat): string {
  switch (format) {
    case "markdown_zip":
      return "Markdown ZIP";
    case "source_zip":
      return "PlainDoc 包";
    case "epub":
      return "EPUB";
    default:
      return format;
  }
}

function getExportFormatDescription(format: AdminSpaceExportFormat): {
  summary: string;
  includes: string[];
  bestFor: string;
  note: string;
} {
  switch (format) {
    case "source_zip":
      return {
        summary: "PlainDoc 包是可导入的完整空间交换包，用于迁移、备份和跨环境恢复。",
        includes: ["目录树和空间元数据", "空间封面", "Markdown 文档", "Office 源文件", "附件与校验信息"],
        bestFor: "后续还要通过“导入空间”恢复为新空间。",
        note: "文件后缀为 .plaindoc，本质是机器可读交换包，不建议手动编辑后再导入。"
      };
    case "markdown_zip":
      return {
        summary: "Markdown ZIP 是内容归档包，用于离线查看或交给外部系统处理。",
        includes: ["按目录树展开的 Markdown 文件", "可选附件", "基础 manifest 信息"],
        bestFor: "只需要 Markdown 内容备份或人工阅读。",
        note: "Office 文档不会转换成 Markdown，也不保证可完整导回。"
      };
    case "epub":
      return {
        summary: "EPUB 是电子书阅读包，用于离线阅读和分发。",
        includes: ["空间标题页", "树状目录", "SSR 渲染后的 Markdown", "Office 纯 HTML 章节"],
        bestFor: "在阅读器、平板或手机上查看空间内容。",
        note: "EPUB 是阅读产物，不能作为空间导入包。"
      };
    default:
      return {
        summary: "请选择导出格式。",
        includes: [],
        bestFor: "",
        note: ""
      };
  }
}

function defaultDownloadFileName(spaceID: string, format: AdminSpaceExportFormat): string {
  const baseName = spaceID.trim() || "space-export";
  if (format === "source_zip") {
    return `${baseName}.plaindoc`;
  }
  return format === "epub" ? `${baseName}.epub` : `${baseName}.zip`;
}

function normalizeDownloadFileName(fileName: string | undefined, spaceID: string, format: AdminSpaceExportFormat): string {
  const fallback = defaultDownloadFileName(spaceID, format);
  const normalized = fileName?.trim() || fallback;
  if (format === "epub" && !normalized.toLowerCase().endsWith(".epub")) {
    return normalized.replace(/\.[^.]+$/, "") + ".epub";
  }
  if (format === "source_zip" && !normalized.toLowerCase().endsWith(".plaindoc")) {
    return normalized.replace(/\.[^.]+$/, "") + ".plaindoc";
  }
  if (format === "markdown_zip" && !normalized.toLowerCase().endsWith(".zip")) {
    return normalized.replace(/\.[^.]+$/, "") + ".zip";
  }
  return normalized;
}

function triggerDownload(downloadURL: string, fileName: string): boolean {
  if (typeof document === "undefined") {
    return false;
  }
  const normalizedURL = downloadURL.trim();
  if (!normalizedURL) {
    return false;
  }
  try {
    const anchor = document.createElement("a");
    anchor.href = normalizedURL;
    anchor.download = fileName.trim();
    anchor.rel = "noopener noreferrer";
    anchor.style.display = "none";
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    return true;
  } catch {
    return false;
  }
}

export function AdminSpaceExportDialog({ open, space, dataGateway, onOpenChange }: AdminSpaceExportDialogProps) {
  const [format, setFormat] = useState<AdminSpaceExportFormat>("source_zip");
  const [includeAttachments, setIncludeAttachments] = useState(true);
  const [includeOfficeSources, setIncludeOfficeSources] = useState(true);
  const [starting, setStarting] = useState(false);
  const [preparingDownload, setPreparingDownload] = useState(false);
  const [latestEvent, setLatestEvent] = useState<AdminSpaceTransferEvent | null>(null);
  const [completedExportFormat, setCompletedExportFormat] = useState<AdminSpaceExportFormat | null>(null);
  const [completedExportStreamURL, setCompletedExportStreamURL] = useState("");
  const subscriptionRef = useRef<AdminSpaceTransferSubscription | null>(null);
  const downloadRefreshSubscriptionRef = useRef<AdminSpaceTransferSubscription | null>(null);

  const targetSpaceID = space?.spaceId.trim() ?? "";
  const lockedFullPackageOptions = format === "source_zip";
  const lockedOfficeSourceOptions = format === "source_zip" || format === "epub";
  const effectiveIncludeAttachments = lockedFullPackageOptions ? true : includeAttachments;
  const effectiveIncludeOfficeSources = lockedOfficeSourceOptions ? true : includeOfficeSources;
  const running = starting || latestEvent?.type === "progress";
  const completedEvent = latestEvent?.type === "completed" ? latestEvent : null;
  const failedEvent = latestEvent?.type === "failed" ? latestEvent : null;
  const downloadURL = completedEvent?.downloadUrl?.trim() || "";
  const completedDownloadFormat = completedExportFormat ?? format;
  const downloadFileName = useMemo(
    () => normalizeDownloadFileName(completedEvent?.fileName, targetSpaceID, completedDownloadFormat),
    [completedDownloadFormat, completedEvent?.fileName, targetSpaceID]
  );
  const formatDescription = useMemo(() => getExportFormatDescription(format), [format]);

  const closeSubscription = useCallback(() => {
    subscriptionRef.current?.close();
    subscriptionRef.current = null;
  }, []);

  const closeDownloadRefreshSubscription = useCallback(() => {
    downloadRefreshSubscriptionRef.current?.close();
    downloadRefreshSubscriptionRef.current = null;
  }, []);

  const resetDialog = useCallback(() => {
    closeSubscription();
    closeDownloadRefreshSubscription();
    setFormat("source_zip");
    setIncludeAttachments(true);
    setIncludeOfficeSources(true);
    setStarting(false);
    setPreparingDownload(false);
    setLatestEvent(null);
    setCompletedExportFormat(null);
    setCompletedExportStreamURL("");
  }, [closeDownloadRefreshSubscription, closeSubscription]);

  const closeDialog = useCallback(() => {
    if (running) {
      return;
    }
    resetDialog();
    onOpenChange(false);
  }, [onOpenChange, resetDialog, running]);

  useEffect(() => {
    if (!open) {
      resetDialog();
    }
  }, [open, resetDialog]);

  useEffect(() => {
    return () => {
      closeSubscription();
      closeDownloadRefreshSubscription();
    };
  }, [closeDownloadRefreshSubscription, closeSubscription]);

  useEffect(() => {
    if (!open) {
      return;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        closeDialog();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [closeDialog, open]);

  useEffect(() => {
    if (!open) {
      return;
    }
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = previousOverflow;
    };
  }, [open]);

  const handleStartExport = useCallback(async () => {
    if (!targetSpaceID || starting) {
      return;
    }
    closeSubscription();
    const requestedFormat = format;
    setStarting(true);
    setLatestEvent({ type: "progress", stage: "queued", progress: 0, message: "正在创建导出任务" });
    setCompletedExportFormat(null);
    setCompletedExportStreamURL("");
    try {
      const result = await dataGateway.admin.startSpaceExport({
        spaceId: targetSpaceID,
        format,
        includeAttachments: effectiveIncludeAttachments,
        includeOfficeSources: effectiveIncludeOfficeSources
      });
      setCompletedExportStreamURL(result.streamUrl);
      subscriptionRef.current = dataGateway.admin.subscribeSpaceExport({
        streamUrl: result.streamUrl,
        onEvent(event) {
          setLatestEvent(event);
          if (event.type === "completed" || event.type === "failed") {
            if (event.type === "completed") {
              setCompletedExportFormat(requestedFormat);
            }
            closeSubscription();
          }
        },
        onError() {
          closeSubscription();
          setLatestEvent({
            type: "failed",
            stage: "stream",
            progress: 0,
            message: "导出事件连接异常，请稍后重试"
          });
          showToast("导出事件连接异常，请稍后重试");
        }
      });
      showToast("导出任务已创建", "success");
    } catch (error) {
      setLatestEvent(null);
      showToast(`创建导出任务失败：${formatError(error)}`);
    } finally {
      setStarting(false);
    }
  }, [closeSubscription, dataGateway.admin, effectiveIncludeAttachments, effectiveIncludeOfficeSources, format, starting, targetSpaceID]);

  const refreshCompletedDownload = useCallback(() => {
    const streamURL = completedExportStreamURL.trim();
    if (!streamURL) {
      return Promise.resolve({
        downloadUrl: downloadURL,
        fileName: completedEvent?.fileName
      });
    }
    closeDownloadRefreshSubscription();
    return new Promise<{ downloadUrl: string; fileName?: string }>((resolve, reject) => {
      let settled = false;
      let subscription: AdminSpaceTransferSubscription | null = null;
      const settle = (callback: () => void) => {
        if (settled) {
          return;
        }
        settled = true;
        subscription?.close();
        if (downloadRefreshSubscriptionRef.current === subscription) {
          downloadRefreshSubscriptionRef.current = null;
        }
        callback();
      };
      subscription = dataGateway.admin.subscribeSpaceExport({
        streamUrl: streamURL,
        onEvent(event) {
          if (event.type === "completed") {
            const freshDownloadURL = event.downloadUrl?.trim() || "";
            if (!freshDownloadURL) {
              settle(() => reject(new Error("导出完成事件缺少下载链接")));
              return;
            }
            setLatestEvent(event);
            settle(() => resolve({
              downloadUrl: freshDownloadURL,
              fileName: event.fileName
            }));
            return;
          }
          if (event.type === "failed") {
            setLatestEvent(event);
            settle(() => reject(new Error(event.message || "导出任务失败")));
          }
        },
        onError() {
          settle(() => reject(new Error("导出下载链接刷新失败")));
        }
      });
      downloadRefreshSubscriptionRef.current = subscription;
    });
  }, [closeDownloadRefreshSubscription, completedEvent?.fileName, completedExportStreamURL, dataGateway.admin, downloadURL]);

  const handleManualDownload = useCallback(async () => {
    if (!downloadURL || preparingDownload) {
      return;
    }
    setPreparingDownload(true);
    try {
      const freshDownload = await refreshCompletedDownload();
      const fileName = normalizeDownloadFileName(freshDownload.fileName, targetSpaceID, completedDownloadFormat);
      if (!triggerDownload(freshDownload.downloadUrl, fileName)) {
        showToast("打开下载失败，请稍后重试");
      }
    } catch (error) {
      showToast(`刷新下载链接失败：${formatError(error)}`);
    } finally {
      setPreparingDownload(false);
    }
  }, [completedDownloadFormat, downloadURL, preparingDownload, refreshCompletedDownload, targetSpaceID]);

  if (!open || !space) {
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
      <section className="grid max-h-[92vh] w-full max-w-[760px] grid-rows-[auto_minmax(0,1fr)_auto] overflow-hidden rounded-xl border border-slate-200 bg-white shadow-[0_30px_80px_rgba(15,23,42,0.25)]">
        <header className="flex items-start justify-between border-b border-slate-200 bg-slate-50 px-5 py-4">
          <div className="space-y-1">
            <h3 className="text-lg font-semibold text-slate-900">导出空间</h3>
            <p className="text-xs text-slate-600">{space.name || targetSpaceID}</p>
          </div>
          <Button type="button" size="icon" variant="ghost" className="h-8 w-8" disabled={running} onClick={closeDialog}>
            <X size={16} />
          </Button>
        </header>

        <div className="grid min-h-0 gap-4 overflow-y-auto px-5 py-4 lg:grid-cols-[minmax(0,1fr)_260px]">
          <div className="space-y-5">
            <label className="block space-y-1.5">
              <span className="text-xs font-semibold text-slate-700">导出格式</span>
              <Select value={format} onValueChange={(value) => setFormat(value as AdminSpaceExportFormat)}>
                <SelectTrigger disabled={running}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className="z-[2900]">
                  <SelectItem value="source_zip">PlainDoc 包</SelectItem>
                  <SelectItem value="markdown_zip">Markdown ZIP</SelectItem>
                  <SelectItem value="epub">EPUB</SelectItem>
                </SelectContent>
              </Select>
            </label>

            <div className="grid gap-3 pt-1">
              <label className="flex items-center gap-2 rounded-md border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700">
                <Checkbox
                  checked={effectiveIncludeAttachments}
                  disabled={running || lockedFullPackageOptions}
                  onCheckedChange={(checked) => setIncludeAttachments(checked === true)}
                  aria-label="包含附件"
                />
                <span>包含附件</span>
              </label>
              <label className="flex items-center gap-2 rounded-md border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700">
                <Checkbox
                  checked={effectiveIncludeOfficeSources}
                  disabled={running || lockedOfficeSourceOptions}
                  onCheckedChange={(checked) => setIncludeOfficeSources(checked === true)}
                  aria-label="包含 Office 源文件"
                />
                <span>包含 Office 源文件</span>
              </label>
            </div>

            {latestEvent ? (
              <div className="rounded-lg border border-slate-200 bg-white p-4">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <p className="text-xs font-medium text-slate-500">任务状态</p>
                    <p className="mt-1 text-sm font-semibold text-slate-900">
                      {latestEvent.message || (completedEvent ? "导出完成" : failedEvent ? "导出失败" : "正在导出")}
                    </p>
                  </div>
                  <Badge
                    variant="outline"
                    className={
                      completedEvent
                        ? "border-emerald-200 bg-emerald-50 text-emerald-700"
                        : failedEvent
                          ? "border-rose-200 bg-rose-50 text-rose-700"
                          : "border-sky-200 bg-sky-50 text-sky-700"
                    }
                  >
                    {completedEvent ? "completed" : failedEvent ? "failed" : latestEvent.stage || "progress"}
                  </Badge>
                </div>
                <div className="mt-3 h-2 overflow-hidden rounded-full bg-slate-100">
                  <div
                    className="h-full rounded-full bg-slate-900 transition-all"
                    style={{ width: `${Math.max(0, Math.min(100, latestEvent.progress ?? (completedEvent ? 100 : 0)))}%` }}
                  />
                </div>
                {completedEvent ? (
                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    <Button type="button" size="sm" disabled={!downloadURL || preparingDownload} onClick={() => void handleManualDownload()}>
                      {preparingDownload ? <LoaderCircle size={14} className="animate-spin" /> : <Download size={14} />}
                      下载文件
                    </Button>
                    <span className="text-xs text-slate-500">{downloadFileName}</span>
                  </div>
                ) : null}
              </div>
            ) : null}
          </div>

          <aside className="space-y-3">
            <div className="rounded-lg border border-slate-200 bg-slate-50 p-3">
              <div className="flex items-center gap-2 text-xs font-semibold text-slate-700">
                {format === "epub" ? <Archive size={15} /> : <FileArchive size={15} />}
                <span>{renderExportFormatLabel(format)}</span>
              </div>
              <p className="mt-2 text-xs leading-5 text-slate-600">{formatDescription.summary}</p>
              <div className="mt-3 space-y-2 text-xs leading-5 text-slate-600">
                <div>
                  <p className="font-semibold text-slate-700">包含内容</p>
                  <ul className="mt-1 list-disc space-y-1 pl-4">
                    {formatDescription.includes.map((item) => (
                      <li key={item}>{item}</li>
                    ))}
                  </ul>
                </div>
                <div>
                  <p className="font-semibold text-slate-700">适合场景</p>
                  <p className="mt-1">{formatDescription.bestFor}</p>
                </div>
                <div>
                  <p className="font-semibold text-slate-700">注意</p>
                  <p className="mt-1">{formatDescription.note}</p>
                </div>
              </div>
            </div>
          </aside>
        </div>

        <footer className="flex flex-wrap items-center justify-end gap-2 border-t border-slate-200 px-5 py-3">
          {running ? (
            <div className="mr-auto inline-flex items-center gap-1.5 rounded-md bg-slate-100 px-2.5 py-1 text-xs text-slate-600">
              <LoaderCircle size={14} className="animate-spin" />
              <span>导出中...</span>
            </div>
          ) : null}
          <Button type="button" variant="outline" disabled={running} onClick={closeDialog}>
            关闭
          </Button>
          <Button type="button" disabled={running || !targetSpaceID} onClick={() => void handleStartExport()}>
            {starting ? <LoaderCircle size={15} className="animate-spin" /> : <Download size={15} />}
            开始导出
          </Button>
        </footer>
      </section>
    </div>,
    document.body
  );
}
