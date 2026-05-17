import { Archive, Download, FileArchive, LoaderCircle, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { Button } from "../../components/ui/button";
import { Checkbox } from "../../components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { showToast } from "../../components/ui/toast";
import {
  type AdminSpace,
  type AdminSpaceExportFormat,
  type DataGateway
} from "../../data-access";
import { formatError } from "../../editor/status-utils";
import { useAdminSpaceTransferTasks } from "../space-transfer/useAdminSpaceTransferTasks";

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

export function AdminSpaceExportDialog({ open, space, dataGateway, onOpenChange }: AdminSpaceExportDialogProps) {
  const { trackExportTask } = useAdminSpaceTransferTasks();
  const [format, setFormat] = useState<AdminSpaceExportFormat>("source_zip");
  const [includeAttachments, setIncludeAttachments] = useState(true);
  const [includeOfficeSources, setIncludeOfficeSources] = useState(true);
  const [starting, setStarting] = useState(false);

  const targetSpaceID = space?.spaceId.trim() ?? "";
  const lockedFullPackageOptions = format === "source_zip";
  const lockedOfficeSourceOptions = format === "source_zip" || format === "epub";
  const effectiveIncludeAttachments = lockedFullPackageOptions ? true : includeAttachments;
  const effectiveIncludeOfficeSources = lockedOfficeSourceOptions ? true : includeOfficeSources;
  const formatDescription = useMemo(() => getExportFormatDescription(format), [format]);

  const resetDialog = useCallback(() => {
    setFormat("source_zip");
    setIncludeAttachments(true);
    setIncludeOfficeSources(true);
    setStarting(false);
  }, []);

  const closeDialog = useCallback(() => {
    if (starting) {
      return;
    }
    resetDialog();
    onOpenChange(false);
  }, [onOpenChange, resetDialog, starting]);

  useEffect(() => {
    if (!open) {
      resetDialog();
    }
  }, [open, resetDialog]);

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
    const requestedFormat = format;
    setStarting(true);
    try {
      const result = await dataGateway.admin.startSpaceExport({
        spaceId: targetSpaceID,
        format,
        includeAttachments: effectiveIncludeAttachments,
        includeOfficeSources: effectiveIncludeOfficeSources
      });
      trackExportTask({
        jobId: result.jobId,
        streamUrl: result.streamUrl,
        spaceId: targetSpaceID,
        spaceName: space?.name,
        format: requestedFormat
      });
      showToast("导出任务已创建", "success");
      resetDialog();
      onOpenChange(false);
    } catch (error) {
      showToast(`创建导出任务失败：${formatError(error)}`);
    } finally {
      setStarting(false);
    }
  }, [
    dataGateway.admin,
    effectiveIncludeAttachments,
    effectiveIncludeOfficeSources,
    format,
    space?.name,
    starting,
    targetSpaceID,
    trackExportTask,
    resetDialog,
    onOpenChange
  ]);

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
          <Button type="button" size="icon" variant="ghost" className="h-8 w-8" disabled={starting} onClick={closeDialog}>
            <X size={16} />
          </Button>
        </header>

        <div className="grid min-h-0 gap-4 overflow-y-auto px-5 py-4 lg:grid-cols-[minmax(0,1fr)_260px]">
          <div className="space-y-5">
            <label className="block space-y-1.5">
              <span className="text-xs font-semibold text-slate-700">导出格式</span>
              <Select value={format} onValueChange={(value) => setFormat(value as AdminSpaceExportFormat)}>
                <SelectTrigger disabled={starting}>
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
                  disabled={starting || lockedFullPackageOptions}
                  onCheckedChange={(checked) => setIncludeAttachments(checked === true)}
                  aria-label="包含附件"
                />
                <span>包含附件</span>
              </label>
              <label className="flex items-center gap-2 rounded-md border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700">
                <Checkbox
                  checked={effectiveIncludeOfficeSources}
                  disabled={starting || lockedOfficeSourceOptions}
                  onCheckedChange={(checked) => setIncludeOfficeSources(checked === true)}
                  aria-label="包含 Office 源文件"
                />
                <span>包含 Office 源文件</span>
              </label>
            </div>
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
          {starting ? (
            <div className="mr-auto inline-flex items-center gap-1.5 rounded-md bg-slate-100 px-2.5 py-1 text-xs text-slate-600">
              <LoaderCircle size={14} className="animate-spin" />
              <span>正在创建导出任务...</span>
            </div>
          ) : null}
          <Button type="button" variant="outline" disabled={starting} onClick={closeDialog}>
            关闭
          </Button>
          <Button type="button" disabled={starting || !targetSpaceID} onClick={() => void handleStartExport()}>
            {starting ? <LoaderCircle size={15} className="animate-spin" /> : <Download size={15} />}
            开始导出
          </Button>
        </footer>
      </section>
    </div>,
    document.body
  );
}
