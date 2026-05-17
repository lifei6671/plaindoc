import { createContext, useContext } from "react";
import type {
  AdminSpaceExportFormat,
  AdminSpaceTransferTask,
  AdminSpaceTransferTaskKind
} from "../../data-access";

export const ADMIN_SPACE_IMPORT_COMPLETED_EVENT = "plaindoc:admin-space-import-completed";

export interface TrackAdminSpaceExportTaskInput {
  jobId: string;
  streamUrl: string;
  spaceId?: string;
  spaceName?: string;
  format?: AdminSpaceExportFormat;
}

export interface TrackAdminSpaceImportTaskInput {
  jobId: string;
  streamUrl: string;
  importId?: string;
  spaceName?: string;
  needsDefaultCover?: boolean;
  onCompleted?(spaceId: string): Promise<void> | void;
}

export interface AdminSpaceTransferTaskView extends AdminSpaceTransferTask {
  downloadUrl?: string;
  streamUrl?: string;
}

export interface AdminSpaceTransferTaskContextValue {
  tasks: AdminSpaceTransferTaskView[];
  trackExportTask(input: TrackAdminSpaceExportTaskInput): void;
  trackImportTask(input: TrackAdminSpaceImportTaskInput): void;
  removeTask(kind: AdminSpaceTransferTaskKind, jobId: string): void;
}

const noop = () => undefined;

export const AdminSpaceTransferTaskContext = createContext<AdminSpaceTransferTaskContextValue>({
  tasks: [],
  trackExportTask: noop,
  trackImportTask: noop,
  removeTask: noop
});

export function useAdminSpaceTransferTasks(): AdminSpaceTransferTaskContextValue {
  return useContext(AdminSpaceTransferTaskContext);
}
