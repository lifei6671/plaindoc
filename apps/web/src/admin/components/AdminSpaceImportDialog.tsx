import { BookOpen, ExternalLink, FileArchive, FileText, LoaderCircle, UploadCloud, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { showToast } from "../../components/ui/toast";
import {
  type AdminSpaceCategory,
  type AdminSpaceImportInspectResult,
  type AdminSpaceTransferEvent,
  type AdminSpaceTransferSubscription,
  type DataGateway,
  type Visibility
} from "../../data-access";
import { formatError } from "../../editor/status-utils";
import { exportSystemGeneratedWebP } from "./spaceCoverDefault";

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

function buildEditorPath(spaceID: string): string {
  return `/editor/${encodeURIComponent(spaceID)}`;
}

function buildReaderPath(spaceID: string): string {
  return `/r/${encodeURIComponent(spaceID)}`;
}

function openPathInNewTab(path: string): void {
  if (typeof window === "undefined") {
    return;
  }
  window.open(path, "_blank", "noopener,noreferrer");
}

export function AdminSpaceImportDialog({
  open,
  dataGateway,
  categoryOptions,
  defaultVisibility,
  onOpenChange,
  onImportCompleted
}: AdminSpaceImportDialogProps) {
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<AdminSpaceImportInspectResult | null>(null);
  const [spaceName, setSpaceName] = useState("");
  const [categoryID, setCategoryID] = useState(NO_CATEGORY_VALUE);
  const [visibility, setVisibility] = useState<Visibility>(defaultVisibility);
  const [inspecting, setInspecting] = useState(false);
  const [committing, setCommitting] = useState(false);
  const [latestEvent, setLatestEvent] = useState<AdminSpaceTransferEvent | null>(null);
  const [completedSpaceID, setCompletedSpaceID] = useState("");
  const subscriptionRef = useRef<AdminSpaceTransferSubscription | null>(null);
  const completed = latestEvent?.type === "completed";
  const failed = latestEvent?.type === "failed";
  const running = latestEvent?.type === "progress";
  const importLocked = inspecting || committing || running;

  const closeSubscription = useCallback(() => {
    subscriptionRef.current?.close();
    subscriptionRef.current = null;
  }, []);

  const resetDialog = useCallback(() => {
    closeSubscription();
    setSelectedFile(null);
    setPreview(null);
    setSpaceName("");
    setCategoryID(NO_CATEGORY_VALUE);
    setVisibility(defaultVisibility);
    setInspecting(false);
    setCommitting(false);
    setLatestEvent(null);
    setCompletedSpaceID("");
  }, [closeSubscription, defaultVisibility]);

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
    return () => closeSubscription();
  }, [closeSubscription]);

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
      setLatestEvent(null);
      setCompletedSpaceID("");
      closeSubscription();
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
    [closeSubscription, dataGateway.admin, defaultVisibility, normalizedCategoryOptions]
  );

  const attachDefaultCoverToImportedSpace = useCallback(
    async (spaceID: string, name: string) => {
      const generated = await exportSystemGeneratedWebP(name);
      const cover = await dataGateway.admin.createSpaceCoverAsset({
        source: "user_upload",
        file: generated.file,
        clientWidth: generated.width,
        clientHeight: generated.height,
        clientMimeType: generated.file.type,
        clientProcessed: true
      });
      if (!cover.assetId) {
        throw new Error("默认封面上传未返回 assetId");
      }
      await dataGateway.admin.updateSpaceMetadata({
        spaceId: spaceID,
        coverAssetId: cover.assetId
      });
    },
    [dataGateway.admin]
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
      setLatestEvent({ type: "progress", stage: "queued", progress: 0, message: "导入任务已创建" });
      subscriptionRef.current = dataGateway.admin.subscribeSpaceImport({
        streamUrl: result.streamUrl,
        onEvent(event) {
          setLatestEvent(event);
          if (event.type === "completed") {
            closeSubscription();
            const nextSpaceID = event.spaceId?.trim() || "";
            setCompletedSpaceID(nextSpaceID);
            if (nextSpaceID && !preview.space.hasCover) {
              void (async () => {
                try {
                  await attachDefaultCoverToImportedSpace(nextSpaceID, normalizedName);
                } catch (error) {
                  showToast(`导入完成，但默认封面生成失败：${formatError(error)}`);
                }
                showToast("导入完成", "success");
                void onImportCompleted();
              })();
            } else {
              showToast("导入完成", "success");
              void onImportCompleted();
            }
            return;
          }
          if (event.type === "failed") {
            closeSubscription();
            showToast(event.message?.trim() || "导入失败");
          }
        },
        onError() {
          closeSubscription();
          setLatestEvent({
            type: "failed",
            stage: "stream",
            progress: 0,
            message: "导入事件连接异常，请稍后重试"
          });
          showToast("导入事件连接异常，请稍后重试");
        }
      });
      showToast("导入任务已创建", "success");
    } catch (error) {
      showToast(`创建导入任务失败：${formatError(error)}`);
    } finally {
      setCommitting(false);
    }
  }, [attachDefaultCoverToImportedSpace, categoryID, closeSubscription, committing, dataGateway.admin, onImportCompleted, preview, spaceName, visibility]);

  const handleOpenEditor = useCallback(() => {
    if (completedSpaceID) {
      openPathInNewTab(buildEditorPath(completedSpaceID));
    }
  }, [completedSpaceID]);

  const handleOpenReader = useCallback(() => {
    if (completedSpaceID) {
      openPathInNewTab(buildReaderPath(completedSpaceID));
    }
  }, [completedSpaceID]);

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

            {latestEvent ? (
              <div className="rounded-lg border border-slate-200 bg-white p-4">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <p className="text-xs font-medium text-slate-500">任务状态</p>
                    <p className="mt-1 text-sm font-semibold text-slate-900">
                      {latestEvent.message || (completed ? "导入完成" : failed ? "导入失败" : "正在导入")}
                    </p>
                  </div>
                  <Badge
                    variant="outline"
                    className={
                      completed
                        ? "border-emerald-200 bg-emerald-50 text-emerald-700"
                        : failed
                          ? "border-rose-200 bg-rose-50 text-rose-700"
                          : "border-sky-200 bg-sky-50 text-sky-700"
                    }
                  >
                    {completed ? "completed" : failed ? "failed" : latestEvent.stage || "progress"}
                  </Badge>
                </div>
                <div className="mt-3 h-2 overflow-hidden rounded-full bg-slate-100">
                  <div
                    className="h-full rounded-full bg-slate-900 transition-all"
                    style={{ width: `${Math.max(0, Math.min(100, latestEvent.progress ?? (completed ? 100 : 0)))}%` }}
                  />
                </div>
                {completed && completedSpaceID ? (
                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    <Button type="button" size="sm" onClick={handleOpenEditor}>
                      <FileText size={14} />
                      打开编辑器
                    </Button>
                    <Button type="button" size="sm" variant="outline" onClick={handleOpenReader}>
                      <BookOpen size={14} />
                      打开阅读页
                    </Button>
                    <span className="inline-flex items-center gap-1 text-xs text-slate-500">
                      <ExternalLink size={13} />
                      {completedSpaceID}
                    </span>
                  </div>
                ) : null}
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
                {completed ? "导入已完成，可打开新空间继续编辑或阅读。" : preview ? "导入包已解析，确认前可调整新空间名称、分类和可见性。" : "选择 .plaindoc 后会在这里显示预览结果。"}
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
          {committing || running ? (
            <div className="mr-auto inline-flex items-center gap-1.5 rounded-md bg-slate-100 px-2.5 py-1 text-xs text-slate-600">
              <LoaderCircle size={14} className="animate-spin" />
              <span>{committing ? "正在创建导入任务..." : "正在导入..."}</span>
            </div>
          ) : null}
          <Button type="button" variant="outline" disabled={importLocked} onClick={closeDialog}>
            {completed ? "关闭" : "取消"}
          </Button>
          <Button
            type="button"
            disabled={!preview || !preview.importable || inspecting || committing || running || completed || !spaceName.trim()}
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
