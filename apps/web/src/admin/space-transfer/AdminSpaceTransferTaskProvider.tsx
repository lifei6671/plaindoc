import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { showToast } from "../../components/ui/toast";
import {
  type AdminGateway,
  type AdminSpace,
  type AdminSpaceTransferEvent,
  type AdminSpaceTransferSubscribeInput,
  type AdminSpaceTransferSubscription,
  type AdminSpaceTransferTask,
  type AdminSpaceTransferTaskKind,
  type DataGateway
} from "../../data-access";
import { formatError } from "../../editor/status-utils";
import { AdminSpaceTransferFloatingPanel } from "../components/AdminSpaceTransferFloatingPanel";
import { exportSystemGeneratedWebP } from "../components/spaceCoverDefault";
import {
  ADMIN_SPACE_IMPORT_COMPLETED_EVENT,
  AdminSpaceTransferTaskContext,
  type AdminSpaceTransferTaskView,
  type TrackAdminSpaceExportTaskInput,
  type TrackAdminSpaceImportTaskInput
} from "./useAdminSpaceTransferTasks";

interface AdminSpaceTransferTaskProviderProps {
  dataGateway: DataGateway;
  children: ReactNode;
}

type AdminGatewayRuntime = Partial<AdminGateway>;

const dismissedTransferTaskStorageKey = "plaindoc:admin-space-transfer-dismissed-tasks:v1";
const maxDismissedTransferTaskKeys = 200;

interface ImportTaskRuntimeMeta {
  needsDefaultCover?: boolean;
  spaceName?: string;
  onCompleted?(spaceId: string): Promise<void> | void;
}

function taskKey(kind: AdminSpaceTransferTaskKind, jobID: string): string {
  return `${kind}:${jobID}`;
}

function isTerminalTransferTask(task: AdminSpaceTransferTaskView): boolean {
  return task.status === "completed" || task.status === "failed";
}

function readDismissedTransferTaskKeys(): Set<string> {
  if (typeof window === "undefined") {
    return new Set();
  }
  try {
    const rawValue = window.localStorage.getItem(dismissedTransferTaskStorageKey);
    if (!rawValue) {
      return new Set();
    }
    const parsedValue = JSON.parse(rawValue);
    if (!Array.isArray(parsedValue)) {
      return new Set();
    }
    return new Set(
      parsedValue
        .filter((value): value is string => typeof value === "string" && value.trim() !== "")
        .slice(-maxDismissedTransferTaskKeys)
    );
  } catch {
    return new Set();
  }
}

function writeDismissedTransferTaskKeys(keys: Set<string>): void {
  if (typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.setItem(
      dismissedTransferTaskStorageKey,
      JSON.stringify(Array.from(keys).slice(-maxDismissedTransferTaskKeys))
    );
  } catch {
    // localStorage 可能被禁用；此时退化为当前会话内隐藏。
  }
}

function statusFromEvent(event: AdminSpaceTransferEvent): AdminSpaceTransferTaskView["status"] {
  if (event.type === "completed") {
    return "completed";
  }
  if (event.type === "failed") {
    return "failed";
  }
  return "running";
}

function mergeEventIntoTask(task: AdminSpaceTransferTaskView, event: AdminSpaceTransferEvent): AdminSpaceTransferTaskView {
  return {
    ...task,
    status: statusFromEvent(event),
    stage: event.stage ?? task.stage,
    progress: event.progress ?? (event.type === "completed" ? 100 : task.progress),
    message: event.message ?? task.message,
    downloadUrl: event.downloadUrl ?? task.downloadUrl,
    fileName: event.fileName ?? task.fileName,
    sizeBytes: event.sizeBytes ?? task.sizeBytes,
    spaceId: event.spaceId ?? task.spaceId,
    spaceName: event.spaceName ?? task.spaceName,
    newSpaceId: event.newSpaceId ?? event.spaceId ?? task.newSpaceId,
    updatedAt: new Date().toISOString()
  };
}

function eventFromTaskSnapshot(task: AdminSpaceTransferTask): AdminSpaceTransferEvent {
  return {
    type: task.status === "completed" || task.status === "failed" ? task.status : "progress",
    stage: task.stage,
    progress: task.progress,
    message: task.message,
    fileName: task.fileName,
    sizeBytes: task.sizeBytes,
    spaceId: task.spaceId,
    spaceName: task.spaceName,
    newSpaceId: task.newSpaceId
  };
}

function markTaskStreamInterrupted(task: AdminSpaceTransferTaskView): AdminSpaceTransferTaskView {
  if (isTerminalTransferTask(task)) {
    return task;
  }
  return {
    ...task,
    status: task.status === "queued" ? "queued" : "running",
    stage: "stream",
    // SSE 只负责传递进度事件，连接断开不等于后端任务失败；保留已有进度，避免大文件导入被前端误判为失败。
    message: "任务事件连接异常，后台仍在继续处理，请稍后刷新查看最新进度",
    updatedAt: new Date().toISOString()
  };
}

function closeSubscription(subscriptions: Map<string, AdminSpaceTransferSubscription>, key: string): void {
  subscriptions.get(key)?.close();
  subscriptions.delete(key);
}

async function resolveRecoveredImportNeedsDefaultCover(admin: AdminGatewayRuntime, spaceID: string): Promise<boolean> {
  const trimmedSpaceID = spaceID.trim();
  if (!trimmedSpaceID || !admin.listSpaces) {
    return false;
  }
  const result = await admin.listSpaces({ keyword: trimmedSpaceID, page: 1, pageSize: 1 });
  const matchedSpace = result.items.find((space: AdminSpace) => space.spaceId.trim() === trimmedSpaceID);
  return Boolean(matchedSpace && !matchedSpace.cover);
}

function dispatchAdminSpaceImportCompleted(spaceID: string): void {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent(ADMIN_SPACE_IMPORT_COMPLETED_EVENT, { detail: { spaceId: spaceID } }));
}

export function AdminSpaceTransferTaskProvider({
  dataGateway,
  children
}: AdminSpaceTransferTaskProviderProps) {
  const [tasks, setTasks] = useState<AdminSpaceTransferTaskView[]>([]);
  const subscriptionsRef = useRef(new Map<string, AdminSpaceTransferSubscription>());
  const subscribeTaskRef = useRef<(kind: AdminSpaceTransferTaskKind, jobID: string, streamUrl: string) => void>(() => undefined);
  const importTaskMetaRef = useRef(new Map<string, ImportTaskRuntimeMeta>());
  const handledImportCompletionsRef = useRef(new Set<string>());
  const dismissedTaskKeysRef = useRef(readDismissedTransferTaskKeys());

  const upsertTask = useCallback((task: AdminSpaceTransferTaskView) => {
    setTasks((previousTasks) => {
      const key = taskKey(task.kind, task.jobId);
      if (dismissedTaskKeysRef.current.has(key)) {
        return previousTasks;
      }
      const nextTasks = previousTasks.filter((existingTask) => taskKey(existingTask.kind, existingTask.jobId) !== key);
      return [task, ...nextTasks].slice(0, 12);
    });
  }, []);

  const updateTaskFromEvent = useCallback((kind: AdminSpaceTransferTaskKind, jobID: string, event: AdminSpaceTransferEvent) => {
    const key = taskKey(kind, jobID);
    setTasks((previousTasks) =>
      previousTasks.map((task) => {
        if (taskKey(task.kind, task.jobId) !== key) {
          return task;
        }
        return mergeEventIntoTask(task, event);
      })
    );
  }, []);

  const markStreamInterrupted = useCallback((kind: AdminSpaceTransferTaskKind, jobID: string) => {
    const key = taskKey(kind, jobID);
    setTasks((previousTasks) =>
      previousTasks.map((task) => {
        if (taskKey(task.kind, task.jobId) !== key) {
          return task;
        }
        return markTaskStreamInterrupted(task);
      })
    );
  }, []);

  const handleImportCompleted = useCallback(
    async (jobID: string, event: AdminSpaceTransferEvent) => {
      const key = taskKey("space_import", jobID);
      if (handledImportCompletionsRef.current.has(key)) {
        return;
      }
      handledImportCompletionsRef.current.add(key);

      const meta = importTaskMetaRef.current.get(key);
      const spaceID = event.newSpaceId?.trim() || event.spaceId?.trim() || "";
      let needsDefaultCover = meta?.needsDefaultCover === true;
      if (!meta && spaceID) {
        try {
          needsDefaultCover = await resolveRecoveredImportNeedsDefaultCover(dataGateway.admin as AdminGatewayRuntime, spaceID);
        } catch (error) {
          showToast(`导入完成，但默认封面状态确认失败：${formatError(error)}`);
        }
      }
      if (spaceID && needsDefaultCover) {
        try {
          const generated = await exportSystemGeneratedWebP(meta?.spaceName?.trim() || event.spaceName?.trim() || "导入空间");
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
        } catch (error) {
          showToast(`导入完成，但默认封面生成失败：${formatError(error)}`);
        }
      }

      showToast("导入完成", "success");
      if (meta?.onCompleted) {
        await meta.onCompleted(spaceID);
      } else {
        dispatchAdminSpaceImportCompleted(spaceID);
      }
    },
    [dataGateway.admin]
  );

  const recoverInterruptedTask = useCallback(
    async (kind: AdminSpaceTransferTaskKind, jobID: string) => {
      const admin = dataGateway.admin as AdminGatewayRuntime;
      if (!admin.getSpaceTransferTask) {
        return;
      }
      try {
        const result = await admin.getSpaceTransferTask({ kind, jobId: jobID });
        const task = result?.task;
        if (!task || dismissedTaskKeysRef.current.has(taskKey(kind, jobID))) {
          return;
        }
        // SSE 长连接只负责推送事件，断线后必须主动刷新一次后端任务快照，
        // 否则大 EPUB 后端已经完成时，前端会停留在最后一次进度。
        upsertTask(task);
        if (kind === "space_import" && task.status === "completed") {
          await handleImportCompleted(jobID, eventFromTaskSnapshot(task));
          return;
        }
        if (task.status !== "queued" && task.status !== "running") {
          return;
        }
        if (!admin.issueSpaceTransferStreamToken) {
          return;
        }
        const tokenResult = await admin.issueSpaceTransferStreamToken({ kind, jobId: jobID });
        if (tokenResult?.streamUrl) {
          subscribeTaskRef.current(kind, jobID, tokenResult.streamUrl);
        }
      } catch (error) {
        showToast(`刷新任务状态失败：${formatError(error)}`);
      }
    },
    [dataGateway.admin, handleImportCompleted, upsertTask]
  );

  const subscribeTask = useCallback(
    (kind: AdminSpaceTransferTaskKind, jobID: string, streamUrl: string) => {
      const admin = dataGateway.admin as AdminGatewayRuntime;
      const key = taskKey(kind, jobID);
      closeSubscription(subscriptionsRef.current, key);
      const input: AdminSpaceTransferSubscribeInput = {
        streamUrl,
        onEvent(event) {
          updateTaskFromEvent(kind, jobID, event);
          if (kind === "space_import" && event.type === "completed") {
            void handleImportCompleted(jobID, event);
          }
          if (event.type === "completed" || event.type === "failed") {
            closeSubscription(subscriptionsRef.current, key);
          }
        },
        onError() {
          markStreamInterrupted(kind, jobID);
          closeSubscription(subscriptionsRef.current, key);
          void recoverInterruptedTask(kind, jobID);
        }
      };
      const subscription =
        kind === "space_export"
          ? admin.subscribeSpaceExport?.(input)
          : admin.subscribeSpaceImport?.(input);
      if (subscription) {
        subscriptionsRef.current.set(key, subscription);
      }
    },
    [dataGateway.admin, handleImportCompleted, markStreamInterrupted, recoverInterruptedTask, updateTaskFromEvent]
  );

  useEffect(() => {
    subscribeTaskRef.current = subscribeTask;
  }, [subscribeTask]);

  useEffect(() => {
    const admin = dataGateway.admin as AdminGatewayRuntime;
    if (!admin.listSpaceTransferTasks || !admin.issueSpaceTransferStreamToken) {
      return;
    }
    let cancelled = false;
    const recoverTasks = async () => {
      try {
        const result = await admin.listSpaceTransferTasks?.({ limit: 12 });
        if (cancelled || !result?.tasks.length) {
          return;
        }
        setTasks((previousTasks) => {
          const existingKeys = new Set(previousTasks.map((task) => taskKey(task.kind, task.jobId)));
          const recovered = result.tasks.filter((task) => {
            const key = taskKey(task.kind, task.jobId);
            return !existingKeys.has(key) && !dismissedTaskKeysRef.current.has(key);
          });
          return [...recovered, ...previousTasks].slice(0, 12);
        });
        const activeTasks = result.tasks.filter((task) => {
          const key = taskKey(task.kind, task.jobId);
          return !dismissedTaskKeysRef.current.has(key) && (task.status === "queued" || task.status === "running");
        });
        await Promise.all(activeTasks.map(async (task) => {
          try {
            const tokenResult = await admin.issueSpaceTransferStreamToken?.({
              kind: task.kind,
              jobId: task.jobId
            });
            if (!cancelled && tokenResult?.streamUrl) {
              subscribeTask(task.kind, task.jobId, tokenResult.streamUrl);
            }
          } catch (error) {
            showToast(`恢复任务订阅失败：${formatError(error)}`);
          }
        }));
      } catch (error) {
        showToast(`恢复导入导出任务失败：${formatError(error)}`);
      }
    };
    void recoverTasks();
    return () => {
      cancelled = true;
    };
  }, [dataGateway.admin, subscribeTask]);

  useEffect(() => {
    const subscriptions = subscriptionsRef.current;
    return () => {
      for (const subscription of subscriptions.values()) {
        subscription.close();
      }
      subscriptions.clear();
    };
  }, []);

  const trackExportTask = useCallback(
    (input: TrackAdminSpaceExportTaskInput) => {
      const now = new Date().toISOString();
      upsertTask({
        jobId: input.jobId,
        kind: "space_export",
        status: "queued",
        stage: "queued",
        progress: 0,
        message: "导出任务已创建",
        spaceId: input.spaceId,
        spaceName: input.spaceName,
        format: input.format,
        streamUrl: input.streamUrl,
        createdAt: now,
        updatedAt: now,
        expiresAt: ""
      });
      subscribeTask("space_export", input.jobId, input.streamUrl);
    },
    [subscribeTask, upsertTask]
  );

  const trackImportTask = useCallback(
    (input: TrackAdminSpaceImportTaskInput) => {
      const now = new Date().toISOString();
      importTaskMetaRef.current.set(taskKey("space_import", input.jobId), {
        needsDefaultCover: input.needsDefaultCover,
        spaceName: input.spaceName,
        onCompleted: input.onCompleted
      });
      upsertTask({
        jobId: input.jobId,
        kind: "space_import",
        status: "queued",
        stage: "queued",
        progress: 0,
        message: "导入任务已创建",
        importId: input.importId,
        spaceName: input.spaceName,
        streamUrl: input.streamUrl,
        createdAt: now,
        updatedAt: now,
        expiresAt: ""
      });
      subscribeTask("space_import", input.jobId, input.streamUrl);
    },
    [subscribeTask, upsertTask]
  );

  const removeTask = useCallback((kind: AdminSpaceTransferTaskKind, jobID: string) => {
    const key = taskKey(kind, jobID);
    closeSubscription(subscriptionsRef.current, key);
    importTaskMetaRef.current.delete(key);
    handledImportCompletionsRef.current.delete(key);
    setTasks((previousTasks) => {
      const removedTask = previousTasks.find((task) => taskKey(task.kind, task.jobId) === key);
      if (removedTask && isTerminalTransferTask(removedTask)) {
        dismissedTaskKeysRef.current.add(key);
        writeDismissedTransferTaskKeys(dismissedTaskKeysRef.current);
      }
      return previousTasks.filter((task) => taskKey(task.kind, task.jobId) !== key);
    });
  }, []);

  const contextValue = useMemo(
    () => ({
      tasks,
      trackExportTask,
      trackImportTask,
      removeTask
    }),
    [removeTask, tasks, trackExportTask, trackImportTask]
  );

  return (
    <AdminSpaceTransferTaskContext.Provider value={contextValue}>
      {children}
      <AdminSpaceTransferFloatingPanel
        tasks={tasks}
        dataGateway={dataGateway}
        onRemoveTask={removeTask}
      />
    </AdminSpaceTransferTaskContext.Provider>
  );
}
