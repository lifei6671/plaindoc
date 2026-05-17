import { FileArchive, LoaderCircle, UploadCloud, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { showToast } from "../../components/ui/toast";
import {
  type AdminSpaceCategory,
  type AdminSpaceImportInspectResult,
  type DataGateway,
  type Visibility
} from "../../data-access";
import { formatError } from "../../editor/status-utils";
import { useAdminSpaceTransferTasks } from "../space-transfer/useAdminSpaceTransferTasks";

const NO_CATEGORY_VALUE = "__PD_NO_CATEGORY__";
const PLAINDOC_ACCEPT = ".plaindoc";

interface AdminSpaceImportDialogProps {
  open: boolean;
  dataGateway: DataGateway;
  categoryOptions: AdminSpaceCategory[];
  defaultVisibility: Visibility;
  onOpenChange(open: boolean): void;
  onImportCompleted(): Promise<void> | void;
}

function formatDateTime(value: string): string {
  if (!value.trim()) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return date.toLocaleString("zh-CN", { hour12: false });
}

function renderVisibilityLabel(value: Visibility): string {
  switch (value) {
    case "public":
      return "完全公开";
    case "authenticated":
      return "登录可见";
    case "member":
      return "成员可见";
    default:
      return value;
  }
}

function normalizeVisibility(value: string | undefined, fallback: Visibility): Visibility {
  if (value === "public" || value === "authenticated" || value === "member") {
    return value;
  }
  return fallback;
}

function isPlaindocFile(file: File): boolean {
  const fileName = file.name.trim().toLowerCase();
  return fileName.endsWith(".plaindoc");
}

export function AdminSpaceImportDialog({
  open,
  dataGateway,
  categoryOptions,
  defaultVisibility,
  onOpenChange,
  onImportCompleted
}: AdminSpaceImportDialogProps) {
  const { trackImportTask } = useAdminSpaceTransferTasks();
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<AdminSpaceImportInspectResult | null>(null);
  const [spaceName, setSpaceName] = useState("");
  const [categoryID, setCategoryID] = useState(NO_CATEGORY_VALUE);
  const [visibility, setVisibility] = useState<Visibility>(defaultVisibility);
  const [inspecting, setInspecting] = useState(false);
  const [committing, setCommitting] = useState(false);
  const importLocked = inspecting || committing;

  const resetDialog = useCallback(() => {
    setSelectedFile(null);
    setPreview(null);
    setSpaceName("");
    setCategoryID(NO_CATEGORY_VALUE);
    setVisibility(defaultVisibility);
    setInspecting(false);
    setCommitting(false);
  }, [defaultVisibility]);

  const closeDialog = useCallback(() => {
    if (importLocked) {
      return;
    }
    resetDialog();
    onOpenChange(false);
  }, [importLocked, onOpenChange, resetDialog]);

  useEffect(() => {
    if (!open) {
      resetDialog();
      return;
    }
    setVisibility(defaultVisibility);
  }, [defaultVisibility, open, resetDialog]);

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
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
    };
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

  const normalizedCategoryOptions = useMemo(
    () =>
      categoryOptions
        .filter((category) => category.categoryId.trim() && category.name.trim())
        .map((category) => ({
          categoryId: category.categoryId.trim(),
          name: category.name.trim()
        })),
    [categoryOptions]
  );

  const handleFileChange = useCallback(
    async (fileList: FileList | null) => {
      const file = fileList?.[0] ?? null;
      setSelectedFile(file);
      setPreview(null);
      setSpaceName("");
      if (!file) {
        return;
      }
      if (!isPlaindocFile(file)) {
        showToast("请选择 .plaindoc 空间交换包");
        return;
      }

      setInspecting(true);
      try {
        const result = await dataGateway.admin.inspectSpaceImport({ file });
        setPreview(result);
        setSpaceName(result.space.name.trim() || "导入空间");
        setVisibility(normalizeVisibility(result.space.visibility, defaultVisibility));
        const importedCategoryID = result.space.categoryId?.trim() || "";
        setCategoryID(
          importedCategoryID && normalizedCategoryOptions.some((option) => option.categoryId === importedCategoryID)
            ? importedCategoryID
            : NO_CATEGORY_VALUE
        );
      } catch (error) {
        showToast(`解析导入包失败：${formatError(error)}`);
      } finally {
        setInspecting(false);
      }
    },
    [dataGateway.admin, defaultVisibility, normalizedCategoryOptions]
  );

  const handleCommit = useCallback(async () => {
    if (!preview || !preview.importable || committing) {
      return;
    }
    const normalizedName = spaceName.trim();
    if (!normalizedName) {
      showToast("新空间名称不能为空");
      return;
    }

    setCommitting(true);
    try {
      const result = await dataGateway.admin.commitSpaceImport({
        importId: preview.importId,
        spaceName: normalizedName,
        categoryId: categoryID === NO_CATEGORY_VALUE ? undefined : categoryID,
        visibility
      });
      trackImportTask({
        jobId: result.jobId,
        streamUrl: result.streamUrl,
        importId: preview.importId,
        spaceName: normalizedName,
        needsDefaultCover: !preview.space.hasCover,
        onCompleted: () => onImportCompleted()
      });
      showToast("导入任务已创建", "success");
      resetDialog();
      onOpenChange(false);
    } catch (error) {
      showToast(`创建导入任务失败：${formatError(error)}`);
    } finally {
      setCommitting(false);
    }
  }, [
    categoryID,
    committing,
    dataGateway.admin,
    onImportCompleted,
    onOpenChange,
    preview,
    resetDialog,
    spaceName,
    trackImportTask,
    visibility
  ]);

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
      <section className="grid max-h-[92vh] w-full max-w-[860px] grid-rows-[auto_minmax(0,1fr)_auto] overflow-hidden rounded-xl border border-slate-200 bg-white shadow-[0_30px_80px_rgba(15,23,42,0.25)]">
        <header className="flex items-start justify-between border-b border-slate-200 bg-slate-50 px-5 py-4">
          <div className="space-y-1">
            <h3 className="text-lg font-semibold text-slate-900">导入空间</h3>
            <p className="text-xs text-slate-600">上传 PlainDoc 空间交换包，先解析预览，再创建导入任务。</p>
          </div>
          <Button type="button" size="icon" variant="ghost" className="h-8 w-8" disabled={importLocked} onClick={closeDialog}>
            <X size={16} />
          </Button>
        </header>

        <div className="grid min-h-0 gap-4 overflow-y-auto px-5 py-4 lg:grid-cols-[minmax(0,1fr)_300px]">
          <div className="space-y-4">
            <label className="block rounded-lg border border-dashed border-slate-300 bg-slate-50 p-4 transition-colors hover:border-sky-300 hover:bg-sky-50/40">
              <span className="flex items-center gap-2 text-sm font-semibold text-slate-800">
                <FileArchive size={16} />
                空间交换包
              </span>
              <span className="mt-1 block text-xs text-slate-500">仅支持 `.plaindoc`，ZIP 和 EPUB 阅读包不能导入。</span>
              <input
                className="mt-3 block w-full rounded-md border border-slate-300 bg-white text-sm text-slate-700 file:mr-3 file:h-9 file:border-0 file:bg-slate-900 file:px-3 file:text-sm file:font-medium file:text-white"
                type="file"
                aria-label="空间交换包"
                accept={PLAINDOC_ACCEPT}
                disabled={importLocked}
                onChange={(event) => {
                  void handleFileChange(event.target.files);
                }}
              />
            </label>

            {selectedFile ? (
              <div className="rounded-md border border-slate-200 bg-white p-3 text-xs text-slate-600">
                <p className="font-medium text-slate-800">{selectedFile.name}</p>
                <p>大小：{Math.max(1, Math.ceil(selectedFile.size / 1024))} KB</p>
              </div>
            ) : null}

            {inspecting ? (
              <div className="inline-flex items-center gap-2 rounded-md bg-slate-100 px-3 py-2 text-sm text-slate-600">
                <LoaderCircle size={15} className="animate-spin" />
                <span>正在解析导入包...</span>
              </div>
            ) : null}

            {preview ? (
              <div className="space-y-4">
                <div className="rounded-lg border border-slate-200 bg-white p-4">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div>
                      <p className="text-xs font-medium text-slate-500">原空间</p>
                      <h4 className="mt-1 text-base font-semibold text-slate-900">{preview.space.name || preview.space.spaceId}</h4>
                    </div>
                    <Badge
                      variant="outline"
                      className={
                        preview.importable
                          ? "border-emerald-200 bg-emerald-50 text-emerald-700"
                          : "border-amber-200 bg-amber-50 text-amber-700"
                      }
                    >
                      {preview.importable ? "可导入" : "仅可预览"}
                    </Badge>
                  </div>
                  <div className="mt-3 grid gap-2 text-xs text-slate-600 sm:grid-cols-2">
                    <p>空间 ID：<code className="rounded bg-slate-100 px-1.5 py-0.5 text-slate-700">{preview.space.spaceId}</code></p>
                    <p>导出时间：{formatDateTime(preview.exportedAt)}</p>
                    <p>包类型：{preview.packageType}</p>
                    <p>版本：v{preview.packageVersion}</p>
                  </div>
                </div>

                <div className="grid gap-2 sm:grid-cols-4">
                  <span className="rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-xs font-medium text-slate-700">
                    目录 {preview.summary.folderCount}
                  </span>
                  <span className="rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-xs font-medium text-slate-700">
                    文档 {preview.summary.documentCount}
                  </span>
                  <span className="rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-xs font-medium text-slate-700">
                    附件 {preview.summary.attachmentCount}
                  </span>
                  <span className="rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-xs font-medium text-slate-700">
                    Office 源文件 {preview.summary.officeSourceCount}
                  </span>
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  <label className="space-y-1.5 sm:col-span-2">
                    <span className="text-xs font-semibold text-slate-700">新空间名称</span>
                    <Input
                      value={spaceName}
                      maxLength={120}
                      disabled={importLocked}
                      onChange={(event) => setSpaceName(event.target.value)}
                    />
                  </label>
                  <label className="space-y-1.5">
                    <span className="text-xs font-semibold text-slate-700">可见性</span>
                    <Select value={visibility} onValueChange={(value) => setVisibility(normalizeVisibility(value, defaultVisibility))}>
                      <SelectTrigger disabled={importLocked}>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent className="z-[2900]">
                        <SelectItem value="public">{renderVisibilityLabel("public")}</SelectItem>
                        <SelectItem value="authenticated">{renderVisibilityLabel("authenticated")}</SelectItem>
                        <SelectItem value="member">{renderVisibilityLabel("member")}</SelectItem>
                      </SelectContent>
                    </Select>
                  </label>
                  <label className="space-y-1.5">
                    <span className="text-xs font-semibold text-slate-700">分类</span>
                    <Select value={categoryID} onValueChange={(value) => setCategoryID(value)}>
                      <SelectTrigger disabled={importLocked}>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent className="z-[2900]">
                        <SelectItem value={NO_CATEGORY_VALUE}>未分类</SelectItem>
                        {normalizedCategoryOptions.map((option) => (
                          <SelectItem key={option.categoryId} value={option.categoryId}>
                            {option.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </label>
                </div>
              </div>
            ) : null}
          </div>

          <aside className="space-y-3">
            <div className="rounded-lg border border-slate-200 bg-white p-3">
              <p className="text-xs font-semibold text-slate-700">解析状态</p>
              <div className="mt-2 flex min-h-[140px] items-center justify-center rounded-md bg-slate-50 px-4 text-center text-xs text-slate-500">
                {preview ? "导入包已解析，确认前可调整新空间名称、分类和可见性。" : "选择 .plaindoc 后会在这里显示预览结果。"}
              </div>
            </div>
            {preview?.warnings.length ? (
              <div className="rounded-lg border border-amber-200 bg-amber-50 p-3">
                <p className="text-xs font-semibold text-amber-800">Warnings</p>
                <ul className="mt-2 list-disc space-y-1 pl-4 text-xs text-amber-800">
                  {preview.warnings.map((warning) => (
                    <li key={warning}>{warning}</li>
                  ))}
                </ul>
              </div>
            ) : null}
          </aside>
        </div>

        <footer className="flex flex-wrap items-center justify-end gap-2 border-t border-slate-200 px-5 py-3">
          {committing ? (
            <div className="mr-auto inline-flex items-center gap-1.5 rounded-md bg-slate-100 px-2.5 py-1 text-xs text-slate-600">
              <LoaderCircle size={14} className="animate-spin" />
              <span>正在创建导入任务...</span>
            </div>
          ) : null}
          <Button type="button" variant="outline" disabled={importLocked} onClick={closeDialog}>
            取消
          </Button>
          <Button
            type="button"
            disabled={!preview || !preview.importable || inspecting || committing || !spaceName.trim()}
            onClick={() => void handleCommit()}
          >
            {committing ? <LoaderCircle size={15} className="animate-spin" /> : <UploadCloud size={15} />}
            确认导入
          </Button>
        </footer>
      </section>
    </div>,
    document.body
  );
}
