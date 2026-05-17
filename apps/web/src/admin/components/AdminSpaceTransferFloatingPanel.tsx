import {
  ArrowDownToLine,
  BookOpen,
  ChevronDown,
  ChevronUp,
  Download,
  ExternalLink,
  FileText,
  LoaderCircle,
  Trash2,
  UploadCloud,
} from "lucide-react";
import { useCallback, useMemo, useState } from "react";
import { showToast } from "../../components/ui/toast";
import type { AdminSpaceTransferTaskKind, DataGateway } from "../../data-access";
import { formatError } from "../../editor/status-utils";
import type { AdminSpaceTransferTaskView } from "../space-transfer/useAdminSpaceTransferTasks";

interface AdminSpaceTransferFloatingPanelProps {
  tasks: AdminSpaceTransferTaskView[];
  dataGateway: DataGateway;
  onRemoveTask(kind: AdminSpaceTransferTaskKind, jobId: string): void;
}

function isActiveTask(task: AdminSpaceTransferTaskView): boolean {
  return task.status === "queued" || task.status === "running";
}

function renderTaskName(task: AdminSpaceTransferTaskView): string {
  return task.spaceName?.trim() || task.fileName?.trim().replace(/\.(plaindoc|zip|epub)$/i, "") || task.spaceId?.trim() || task.importId?.trim() || task.jobId;
}

function renderStatusLabel(status: AdminSpaceTransferTaskView["status"]): string {
  switch (status) {
    case "queued":
      return "等待中";
    case "running":
      return "进行中";
    case "completed":
      return "已完成";
    case "failed":
      return "失败";
    default:
      return status;
  }
}

function renderStatusClassName(status: AdminSpaceTransferTaskView["status"]): string {
  switch (status) {
    case "completed":
      return "text-emerald-600";
    case "failed":
      return "text-rose-600";
    case "queued":
      return "text-sky-600";
    case "running":
      return "text-blue-600";
    default:
      return "text-slate-700";
  }
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

export function AdminSpaceTransferFloatingPanel({
  tasks,
  dataGateway,
  onRemoveTask
}: AdminSpaceTransferFloatingPanelProps) {
  const [expanded, setExpanded] = useState(false);
  const [downloadingJobID, setDownloadingJobID] = useState("");
  const visibleTasks = useMemo(
    () => tasks.filter((task) => task.jobId.trim()).slice(0, 8),
    [tasks]
  );
  const activeCount = visibleTasks.filter(isActiveTask).length;

  const handleDownload = useCallback(
    async (task: AdminSpaceTransferTaskView) => {
      if (task.kind !== "space_export" || task.status !== "completed" || downloadingJobID) {
        return;
      }
      setDownloadingJobID(task.jobId);
      try {
        const result = await dataGateway.admin.issueSpaceTransferDownloadToken({
          kind: task.kind,
          jobId: task.jobId
        });
        const fileName = task.fileName?.trim() || `${task.jobId}.plaindoc`;
        if (!triggerDownload(result.downloadUrl, fileName)) {
          showToast("打开下载失败，请稍后重试");
        }
      } catch (error) {
        showToast(`刷新下载链接失败：${formatError(error)}`);
      } finally {
        setDownloadingJobID("");
      }
    },
    [dataGateway.admin, downloadingJobID]
  );

  if (!visibleTasks.length) {
    return null;
  }

  return (
    <section className="fixed bottom-4 right-4 z-[2600] w-[min(430px,calc(100vw-24px))]">
      <div className="overflow-hidden rounded-[22px] border border-slate-200 bg-white shadow-[0_18px_60px_rgba(15,23,42,0.18)]">
        <div className="flex h-16 items-center gap-3 px-5">
          <div className="relative flex h-10 w-10 shrink-0 items-center justify-center rounded-full border-4 border-slate-200 bg-white text-emerald-500">
            {activeCount > 0 ? (
              <span className="absolute inset-[-4px] rounded-full border-4 border-transparent border-t-emerald-500" />
            ) : null}
            <ArrowDownToLine size={20} />
          </div>
          <div className="min-w-0 flex-1">
            <p className="truncate text-lg font-semibold text-slate-950">剩余{activeCount}个</p>
            <p className="sr-only">导入导出任务</p>
          </div>
          <div className="flex shrink-0 items-center gap-1">
            <button
              type="button"
              className="inline-flex h-8 w-8 cursor-pointer items-center justify-center rounded-md border-0 bg-transparent text-slate-600 transition-colors hover:bg-slate-100 hover:text-slate-950 focus:outline-none focus:ring-2 focus:ring-slate-300"
              aria-label={expanded ? "最小化任务中心" : "展开任务中心"}
              onClick={() => setExpanded((previousState) => !previousState)}
            >
              {expanded ? <ChevronUp size={19} /> : <ChevronDown size={19} />}
            </button>
          </div>
        </div>

        {expanded ? (
          <div className="border-t border-slate-100">
            <div className="max-h-[320px] overflow-y-auto">
              {visibleTasks.length > 0 ? (
                visibleTasks.map((task) => {
                  const progress = Math.max(0, Math.min(100, task.progress ?? (task.status === "completed" ? 100 : 0)));
                  const taskName = renderTaskName(task);
                  const newSpaceID = task.newSpaceId?.trim() || task.spaceId?.trim() || "";
                  const activeTask = isActiveTask(task);
                  return (
                    <article
                      key={`${task.kind}:${task.jobId}`}
                      className="group border-t border-slate-100 px-5 py-3 transition-colors hover:bg-slate-50 focus-within:bg-slate-50"
                    >
                      <div className="flex items-center gap-3">
                        <span className={task.kind === "space_import" ? "text-sky-500" : "text-emerald-500"}>
                          {task.kind === "space_import" ? <UploadCloud size={20} /> : <ArrowDownToLine size={20} />}
                        </span>
                        <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-slate-100 text-slate-500">
                          <FileText size={17} />
                        </span>
                        <div className="min-w-0 flex-1">
                          <p className="truncate text-sm font-medium text-slate-950">{taskName}</p>
                        </div>
                        {activeTask ? (
                          <span className={`shrink-0 text-sm tabular-nums ${renderStatusClassName(task.status)}`}>{progress}%</span>
                        ) : (
                          <div className="relative h-8 w-[104px] shrink-0">
                            <span
                              className={`absolute right-0 top-1/2 -translate-y-1/2 text-sm font-medium tabular-nums transition-opacity group-hover:opacity-0 group-focus-within:opacity-0 ${renderStatusClassName(task.status)}`}
                            >
                              {renderStatusLabel(task.status)}
                            </span>
                            <div className="absolute right-0 top-0 flex items-center justify-end gap-1 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
                            {task.kind === "space_export" && task.status === "completed" ? (
                              <button
                                type="button"
                                className="inline-flex h-8 w-8 cursor-pointer appearance-none items-center justify-center rounded-md border-0 bg-transparent p-0 text-slate-600 shadow-none transition-colors hover:bg-slate-100 hover:text-slate-950 focus:outline-none focus:ring-2 focus:ring-slate-300 disabled:cursor-not-allowed disabled:opacity-40"
                                aria-label="下载文件"
                                title="下载文件"
                                disabled={downloadingJobID === task.jobId}
                                onClick={() => void handleDownload(task)}
                              >
                                {downloadingJobID === task.jobId ? <LoaderCircle size={16} className="animate-spin" /> : <Download size={16} />}
                              </button>
                            ) : null}
                            {task.kind === "space_import" && task.status === "completed" && newSpaceID ? (
                              <>
                                <button
                                  type="button"
                                  className="inline-flex h-8 w-8 cursor-pointer appearance-none items-center justify-center rounded-md border-0 bg-transparent p-0 text-slate-600 shadow-none transition-colors hover:bg-slate-100 hover:text-slate-950 focus:outline-none focus:ring-2 focus:ring-slate-300"
                                  aria-label="打开编辑器"
                                  title="打开编辑器"
                                  onClick={() => openPathInNewTab(buildEditorPath(newSpaceID))}
                                >
                                  <ExternalLink size={16} />
                                </button>
                                <button
                                  type="button"
                                  className="inline-flex h-8 w-8 cursor-pointer appearance-none items-center justify-center rounded-md border-0 bg-transparent p-0 text-slate-600 shadow-none transition-colors hover:bg-slate-100 hover:text-slate-950 focus:outline-none focus:ring-2 focus:ring-slate-300"
                                  aria-label="打开阅读页"
                                  title="打开阅读页"
                                  onClick={() => openPathInNewTab(buildReaderPath(newSpaceID))}
                                >
                                  <BookOpen size={16} />
                                </button>
                              </>
                            ) : null}
                              <button
                                type="button"
                                className="inline-flex h-8 w-8 cursor-pointer appearance-none items-center justify-center rounded-md border-0 bg-transparent p-0 text-slate-600 shadow-none transition-colors hover:bg-slate-100 hover:text-slate-950 focus:outline-none focus:ring-2 focus:ring-slate-300"
                                aria-label="清除"
                                title="清除"
                                onClick={() => onRemoveTask(task.kind, task.jobId)}
                              >
                                <Trash2 size={16} />
                              </button>
                            </div>
                          </div>
                        )}
                      </div>
                      {activeTask ? (
                        <div className="mt-3 h-0.5 overflow-hidden rounded-full bg-slate-200">
                          <div className="h-full rounded-full bg-emerald-500 transition-all" style={{ width: `${progress}%` }} />
                        </div>
                      ) : null}
                    </article>
                  );
                })
              ) : (
                <div className="border-t border-slate-100 px-5 py-8 text-center text-sm text-slate-500">暂无导入导出任务</div>
              )}
            </div>
          </div>
        ) : null}
      </div>
    </section>
  );
}
