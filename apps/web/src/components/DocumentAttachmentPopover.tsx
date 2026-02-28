import {
  Download,
  Eye,
  FileText,
  LoaderCircle,
  Paperclip,
  RefreshCw,
  Trash2,
  Upload
} from "lucide-react";
import { memo, useMemo, useRef, type ChangeEvent } from "react";
import type { DocumentAttachment } from "../data-access";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger
} from "./ui/tooltip";
import { Popover, PopoverContent, PopoverTrigger } from "./ui/popover";

interface PendingAttachmentAction {
  attachmentId: string;
  action: "download" | "preview" | "delete";
}

interface DocumentAttachmentPopoverProps {
  attachments: DocumentAttachment[];
  disabled?: boolean;
  loading?: boolean;
  uploading?: boolean;
  pendingAction?: PendingAttachmentAction | null;
  onUploadFiles: (files: File[]) => void;
  onRefresh: () => void;
  onDownload: (attachment: DocumentAttachment) => void;
  onPreview: (attachment: DocumentAttachment) => void;
  onDelete: (attachment: DocumentAttachment) => void;
}

function formatFileSize(sizeBytes: number): string {
  const size = Number(sizeBytes);
  if (!Number.isFinite(size) || size <= 0) {
    return "0 B";
  }
  const units = ["B", "KB", "MB", "GB", "TB"];
  let currentValue = size;
  let unitIndex = 0;
  while (currentValue >= 1024 && unitIndex < units.length - 1) {
    currentValue /= 1024;
    unitIndex += 1;
  }
  const precision = currentValue >= 100 || unitIndex === 0 ? 0 : 1;
  return `${currentValue.toFixed(precision)} ${units[unitIndex]}`;
}

export const DocumentAttachmentPopover = memo(function DocumentAttachmentPopover({
  attachments,
  disabled = false,
  loading = false,
  uploading = false,
  pendingAction = null,
  onUploadFiles,
  onRefresh,
  onDownload,
  onPreview,
  onDelete
}: DocumentAttachmentPopoverProps) {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const hasAttachments = attachments.length > 0;

  const summaryText = useMemo(() => {
    if (loading) {
      return "附件加载中...";
    }
    if (!hasAttachments) {
      return "当前文档暂无附件";
    }
    return `共 ${attachments.length} 个附件`;
  }, [attachments.length, hasAttachments, loading]);

  const triggerUploadPicker = () => {
    if (disabled || uploading) {
      return;
    }
    const input = fileInputRef.current;
    if (!input) {
      return;
    }
    input.value = "";
    input.click();
  };

  const handleFileInputChange = (event: ChangeEvent<HTMLInputElement>) => {
    const selectedFiles = event.target.files ? Array.from(event.target.files) : [];
    event.currentTarget.value = "";
    if (!selectedFiles.length) {
      return;
    }
    onUploadFiles(selectedFiles);
  };

  const isAttachmentActionBusy = (attachmentID: string): boolean => {
    return Boolean(pendingAction && pendingAction.attachmentId === attachmentID);
  };

  return (
    <>
      <Popover>
        <TooltipProvider delayDuration={120}>
          <Tooltip>
            <TooltipTrigger asChild>
              <PopoverTrigger asChild>
                <button
                  type="button"
                  className="attachment-menu__trigger"
                  aria-label="文档附件"
                  disabled={disabled}
                >
                  <Paperclip size={15} />
                </button>
              </PopoverTrigger>
            </TooltipTrigger>
            <TooltipContent side="bottom">文档附件</TooltipContent>
          </Tooltip>
        </TooltipProvider>
        <PopoverContent className="attachment-menu" align="end" sideOffset={10}>
          <div className="attachment-menu__header">
            <div>
              <p className="attachment-menu__title">文档附件</p>
              <p className="attachment-menu__summary">{summaryText}</p>
            </div>
            <div className="attachment-menu__header-actions">
              <button
                type="button"
                className="attachment-menu__header-button"
                onClick={onRefresh}
                disabled={loading || uploading || disabled}
                aria-label="刷新附件列表"
              >
                <RefreshCw size={14} />
              </button>
              <button
                type="button"
                className="attachment-menu__header-button attachment-menu__header-button--primary"
                onClick={triggerUploadPicker}
                disabled={uploading || disabled}
              >
                {uploading ? <LoaderCircle size={14} className="attachment-menu__spin" /> : <Upload size={14} />}
                <span>{uploading ? "上传中" : "上传"}</span>
              </button>
            </div>
          </div>

          <div className="attachment-menu__body">
            {loading ? (
              <div className="attachment-menu__loading">
                <LoaderCircle size={14} className="attachment-menu__spin" />
                <span>附件列表加载中...</span>
              </div>
            ) : null}
            {!loading && !hasAttachments ? (
              <div className="attachment-menu__empty">
                <FileText size={14} />
                <span>暂无附件，点击“上传”添加文件</span>
              </div>
            ) : null}
            {!loading && hasAttachments ? (
              <ul className="attachment-menu__list">
                {attachments.map((attachment) => {
                  const isBusy = isAttachmentActionBusy(attachment.attachmentId);
                  const busyAction = isBusy ? pendingAction?.action : null;
                  return (
                    <li key={attachment.attachmentId} className="attachment-menu__item">
                      <div className="attachment-menu__meta">
                        <p className="attachment-menu__name" title={attachment.fileName}>
                          {attachment.fileName}
                        </p>
                        <p className="attachment-menu__info">
                          <span>{formatFileSize(attachment.sizeBytes)}</span>
                          <span>·</span>
                          <span>{attachment.storageProvider}</span>
                          {attachment.requiresAuthDownload ? (
                            <>
                              <span>·</span>
                              <span className="attachment-menu__badge">鉴权</span>
                            </>
                          ) : null}
                        </p>
                      </div>
                      <div className="attachment-menu__actions">
                        {attachment.previewSupported ? (
                          <button
                            type="button"
                            className="attachment-menu__action"
                            onClick={() => onPreview(attachment)}
                            disabled={uploading || isBusy || disabled}
                            title="在线预览"
                            aria-label={`预览附件 ${attachment.fileName}`}
                          >
                            {busyAction === "preview" ? (
                              <LoaderCircle size={13} className="attachment-menu__spin" />
                            ) : (
                              <Eye size={13} />
                            )}
                          </button>
                        ) : null}
                        <button
                          type="button"
                          className="attachment-menu__action"
                          onClick={() => onDownload(attachment)}
                          disabled={uploading || isBusy || disabled}
                          title="下载文件"
                          aria-label={`下载附件 ${attachment.fileName}`}
                        >
                          {busyAction === "download" ? (
                            <LoaderCircle size={13} className="attachment-menu__spin" />
                          ) : (
                            <Download size={13} />
                          )}
                        </button>
                        <button
                          type="button"
                          className="attachment-menu__action attachment-menu__action--danger"
                          onClick={() => onDelete(attachment)}
                          disabled={uploading || isBusy || disabled}
                          title="删除附件"
                          aria-label={`删除附件 ${attachment.fileName}`}
                        >
                          {busyAction === "delete" ? (
                            <LoaderCircle size={13} className="attachment-menu__spin" />
                          ) : (
                            <Trash2 size={13} />
                          )}
                        </button>
                      </div>
                    </li>
                  );
                })}
              </ul>
            ) : null}
          </div>
        </PopoverContent>
      </Popover>
      <input
        ref={fileInputRef}
        type="file"
        multiple
        className="attachment-menu__file-input"
        tabIndex={-1}
        onChange={handleFileInputChange}
      />
    </>
  );
});
