import { ChevronDown, ExternalLink, LoaderCircle, RefreshCw, Search, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEventHandler } from "react";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { Checkbox } from "../../components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger
} from "../../components/ui/dropdown-menu";
import { Input } from "../../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { showToast } from "../../components/ui/toast";
import {
  type AdminDocumentAttachment,
  type AdminDocumentAttachmentDeleteResult,
  type AdminDocumentAttachmentListResult,
  type DataGateway,
  type DocumentAttachmentPreviewKind
} from "../../data-access";
import { formatError } from "../../editor/status-utils";
import { useAdminDialogs } from "../components/AdminDialogs";
import {
  AdminBulkActionBar,
  AdminPageCard,
  AdminPaginationFooter,
  AdminTableContainer,
  AdminToolbarActions
} from "../components/AdminPageLayout";

const DEFAULT_PAGE_SIZE = 20;

interface AdminDocumentAttachmentsPageProps {
  dataGateway: DataGateway;
}

interface AdminDocumentAttachmentsState {
  items: AdminDocumentAttachment[];
  pagination: AdminDocumentAttachmentListResult["pagination"];
}

function emptyDocumentAttachmentsState(): AdminDocumentAttachmentsState {
  return {
    items: [],
    pagination: {
      page: 1,
      pageSize: DEFAULT_PAGE_SIZE,
      total: 0
    }
  };
}

function formatDateTime(value: string | null): string {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return date.toLocaleString("zh-CN", { hour12: false });
}

function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value <= 0) {
    return "0 B";
  }
  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = value;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }
  if (unitIndex === 0) {
    return `${Math.trunc(size)} ${units[unitIndex]}`;
  }
  return `${size.toFixed(2)} ${units[unitIndex]}`;
}

function renderStatusLabel(value: AdminDocumentAttachment["status"]): string {
  switch (value) {
    case "active":
      return "正常";
    case "deleted":
      return "已删除";
    default:
      return value;
  }
}

function renderStatusBadgeClass(value: AdminDocumentAttachment["status"]): string {
  switch (value) {
    case "active":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "deleted":
      return "border-slate-200 bg-slate-100 text-slate-600";
    default:
      return "border-slate-200 bg-slate-100 text-slate-600";
  }
}

function renderDocumentStatusLabel(value: AdminDocumentAttachment["documentStatus"]): string {
  switch (value) {
    case "active":
      return "文档正常";
    case "banned":
      return "文档封禁";
    case "deleted":
      return "文档删除";
    default:
      return value;
  }
}

function renderPreviewKindLabel(value: DocumentAttachmentPreviewKind): string {
  switch (value) {
    case "image":
      return "图片";
    case "pdf":
      return "PDF";
    case "office":
      return "Office";
    case "text":
      return "文本";
    default:
      return "未知";
  }
}

function renderStorageProviderLabel(value: string): string {
  switch (value) {
    case "local":
      return "本地";
    case "cloudflare-r2":
      return "Cloudflare R2";
    case "aliyun-oss":
      return "阿里云 OSS";
    default:
      return value || "-";
  }
}

function buildDocumentReaderPath(spaceID: string, documentID: string): string {
  return `/r/${encodeURIComponent(spaceID)}/${encodeURIComponent(documentID)}`;
}

function buildAttachmentPreviewPath(documentID: string, attachmentID: string): string {
  return `/preview/docs/${encodeURIComponent(documentID)}/attachments/${encodeURIComponent(attachmentID)}`;
}

function openPathInNewTab(path: string): void {
  if (typeof window === "undefined") {
    return;
  }
  window.open(path, "_blank", "noopener,noreferrer");
}

function buildDeleteAttachmentToastMessage(result: AdminDocumentAttachmentDeleteResult): {
  message: string;
  variant: "success" | "info";
} {
  if (!result.physicalDeleteRequested) {
    return {
      message: "附件删除成功（逻辑删除）",
      variant: "success"
    };
  }

  if (result.physicalDeleteExecuted) {
    return {
      message: "附件删除成功（记录与物理文件均已删除）",
      variant: "success"
    };
  }

  const normalizedDeleteError = result.physicalDeleteError.trim();
  if (normalizedDeleteError) {
    return {
      message: `已删除附件记录，但物理文件未删除：${normalizedDeleteError}`,
      variant: "info"
    };
  }

  if (result.sharedReferenceCount > 0) {
    return {
      message: `已删除附件记录；物理文件仍有 ${result.sharedReferenceCount} 个活跃引用，未执行物理删除。`,
      variant: "info"
    };
  }

  return {
    message: "附件删除成功",
    variant: "success"
  };
}

export function AdminDocumentAttachmentsPage({ dataGateway }: AdminDocumentAttachmentsPageProps) {
  const { confirm, prompt, dialogs } = useAdminDialogs();

  const [keywordInput, setKeywordInput] = useState("");
  const [keyword, setKeyword] = useState("");
  const [spaceIdInput, setSpaceIdInput] = useState("");
  const [spaceId, setSpaceId] = useState("");
  const [documentIdInput, setDocumentIdInput] = useState("");
  const [documentId, setDocumentId] = useState("");
  const [statusFilter, setStatusFilter] = useState<"" | "all" | "active" | "deleted">("");
  const [storageProviderFilter, setStorageProviderFilter] = useState<"" | "all" | "local" | "cloudflare-r2" | "aliyun-oss">("");
  const [page, setPage] = useState(1);
  const [selectedAttachmentIDs, setSelectedAttachmentIDs] = useState<string[]>([]);

  const [attachmentsState, setAttachmentsState] = useState<AdminDocumentAttachmentsState>(() =>
    emptyDocumentAttachmentsState()
  );
  const [loading, setLoading] = useState(false);
  const [actioningAttachmentID, setActioningAttachmentID] = useState<string | null>(null);
  const [batchActioning, setBatchActioning] = useState(false);

  const openToast = useCallback((message: string, variant: "success" | "info" | "error" = "error") => {
    showToast(message, variant);
  }, []);

  const loadAttachments = useCallback(async () => {
    setLoading(true);
    try {
      const payload = await dataGateway.admin.listDocumentAttachments({
        keyword,
        spaceId,
        documentId,
        status: statusFilter || undefined,
        storageProvider: storageProviderFilter || undefined,
        page,
        pageSize: DEFAULT_PAGE_SIZE
      });
      setAttachmentsState(payload);
    } catch (error) {
      openToast(`加载附件列表失败：${formatError(error)}`);
      setAttachmentsState(emptyDocumentAttachmentsState());
    } finally {
      setLoading(false);
    }
  }, [dataGateway.admin, documentId, keyword, openToast, page, spaceId, statusFilter, storageProviderFilter]);

  useEffect(() => {
    void loadAttachments();
  }, [loadAttachments]);

  useEffect(() => {
    setSelectedAttachmentIDs((previous) =>
      previous.filter((attachmentID) => attachmentsState.items.some((item) => item.attachmentId === attachmentID))
    );
  }, [attachmentsState.items]);

  const selectedAttachmentSet = useMemo(() => new Set(selectedAttachmentIDs), [selectedAttachmentIDs]);
  const selectableAttachmentIDs = useMemo(
    () => attachmentsState.items.map((item) => item.attachmentId),
    [attachmentsState.items]
  );
  const allSelectableChecked = useMemo(
    () =>
      selectableAttachmentIDs.length > 0 &&
      selectableAttachmentIDs.every((attachmentID) => selectedAttachmentSet.has(attachmentID)),
    [selectableAttachmentIDs, selectedAttachmentSet]
  );

  const totalPages = useMemo(() => {
    const total = attachmentsState.pagination.total;
    const pageSize = attachmentsState.pagination.pageSize || DEFAULT_PAGE_SIZE;
    return Math.max(1, Math.ceil(total / pageSize));
  }, [attachmentsState.pagination.pageSize, attachmentsState.pagination.total]);

  const handleSearchSubmit = useCallback<FormEventHandler<HTMLFormElement>>(
    (event) => {
      event.preventDefault();
      setPage(1);
      setKeyword(keywordInput.trim());
      setSpaceId(spaceIdInput.trim());
      setDocumentId(documentIdInput.trim());
    },
    [documentIdInput, keywordInput, spaceIdInput]
  );

  const handleReset = useCallback(() => {
    setKeywordInput("");
    setKeyword("");
    setSpaceIdInput("");
    setSpaceId("");
    setDocumentIdInput("");
    setDocumentId("");
    setStatusFilter("");
    setStorageProviderFilter("");
    setPage(1);
  }, []);

  const handleDelete = useCallback(
    async (attachment: AdminDocumentAttachment) => {
      const promptResult = await prompt({
        title: `删除附件：${attachment.fileName || attachment.attachmentId}`,
        description:
          "逻辑删除仅标记为删除且保留文件；物理删除会删除附件记录并删除本地物理文件。",
        confirmText: "确认删除",
        tone: "danger",
        fields: [
          {
            key: "physicalDelete",
            label: "删除方式",
            type: "select",
            required: true,
            defaultValue: "false",
            options: [
              { value: "false", label: "仅逻辑删除" },
              { value: "true", label: "物理删除（删文件+删记录）" }
            ]
          }
        ]
      });
      if (!promptResult) {
        return;
      }

      const physicalDelete = (promptResult.physicalDelete ?? "false").trim() === "true";
      setActioningAttachmentID(attachment.attachmentId);
      try {
        let deleteResult = await dataGateway.admin.deleteDocumentAttachment({
          attachmentId: attachment.attachmentId,
          physicalDelete
        });

        if (physicalDelete && deleteResult.confirmationRequired) {
          const sampleRefs = deleteResult.sharedReferences
            .slice(0, 3)
            .map((item) => item.documentTitle || item.documentId)
            .filter((item) => item && item.trim());
          const confirmationDescription = [
            deleteResult.confirmationReason || "该物理文件仍被多个文档引用。",
            `当前活跃引用数：${deleteResult.sharedReferenceCount}`,
            sampleRefs.length > 0 ? `示例文档：${sampleRefs.join("、")}` : "",
            "继续后仅删除当前附件记录，不会删除物理文件。"
          ]
            .filter((item) => item.trim())
            .join("\n");
          const confirmed = await confirm({
            title: "检测到共享文件引用",
            description: confirmationDescription,
            confirmText: "继续删除当前记录",
            cancelText: "取消",
            tone: "warning"
          });
          if (!confirmed) {
            return;
          }
          deleteResult = await dataGateway.admin.deleteDocumentAttachment({
            attachmentId: attachment.attachmentId,
            physicalDelete: true,
            forcePhysicalDeleteOnShare: true
          });
        }

        const toast = buildDeleteAttachmentToastMessage(deleteResult);
        openToast(toast.message, toast.variant);
        await loadAttachments();
      } catch (error) {
        openToast(`删除附件失败：${formatError(error)}`);
      } finally {
        setActioningAttachmentID(null);
      }
    },
    [confirm, dataGateway.admin, loadAttachments, openToast, prompt]
  );

  const handleToggleSelectAll = useCallback(
    (checked: boolean) => {
      setSelectedAttachmentIDs(checked ? selectableAttachmentIDs : []);
    },
    [selectableAttachmentIDs]
  );

  const handleToggleSelectOne = useCallback((attachmentID: string, checked: boolean) => {
    setSelectedAttachmentIDs((previous) => {
      if (checked) {
        if (previous.includes(attachmentID)) {
          return previous;
        }
        return [...previous, attachmentID];
      }
      return previous.filter((value) => value !== attachmentID);
    });
  }, []);

  const handleBatchDelete = useCallback(async () => {
    const selectedItems = attachmentsState.items.filter((item) => selectedAttachmentSet.has(item.attachmentId));
    if (selectedItems.length === 0) {
      openToast("请先选择需要删除的附件");
      return;
    }

    const promptResult = await prompt({
      title: `批量删除附件（${selectedItems.length} 项）`,
      description:
        "请选择删除方式。逻辑删除仅标记为删除并保留文件；物理删除会删除附件记录，并在无引用时删除物理文件（不可恢复）。",
      confirmText: "继续",
      tone: "danger",
      fields: [
        {
          key: "physicalDelete",
          label: "删除方式",
          type: "select",
          required: true,
          defaultValue: "false",
          options: [
            { value: "false", label: "仅逻辑删除" },
            { value: "true", label: "物理删除（不可恢复）" }
          ]
        }
      ]
    });
    if (!promptResult) {
      return;
    }

    const physicalDelete = (promptResult.physicalDelete ?? "false").trim() === "true";
    if (physicalDelete) {
      const confirmed = await confirm({
        title: "确认批量物理删除",
        description:
          "物理删除后不可恢复。系统会按引用关系删除记录；若文件仍被其他文档引用，则不会删除物理文件。",
        confirmText: "确认物理删除",
        cancelText: "取消",
        tone: "danger"
      });
      if (!confirmed) {
        return;
      }
    }

    setBatchActioning(true);
    let successCount = 0;
    const failures: string[] = [];
    try {
      for (const item of selectedItems) {
        try {
          let deleteResult = await dataGateway.admin.deleteDocumentAttachment({
            attachmentId: item.attachmentId,
            physicalDelete
          });
          if (physicalDelete && deleteResult.confirmationRequired) {
            deleteResult = await dataGateway.admin.deleteDocumentAttachment({
              attachmentId: item.attachmentId,
              physicalDelete: true,
              forcePhysicalDeleteOnShare: true
            });
          }
          const toast = buildDeleteAttachmentToastMessage(deleteResult);
          if (toast.variant === "info") {
            failures.push(`${item.fileName || item.attachmentId}: ${toast.message}`);
          } else {
            successCount += 1;
          }
        } catch (error) {
          failures.push(`${item.fileName || item.attachmentId}: ${formatError(error)}`);
        }
      }

      await loadAttachments();
      setSelectedAttachmentIDs([]);
      if (failures.length > 0) {
        openToast(`批量删除完成：成功 ${successCount}，异常 ${failures.length}。首个异常：${failures[0]}`, "info");
      } else {
        openToast(`批量删除成功：共 ${successCount} 条`, "success");
      }
    } finally {
      setBatchActioning(false);
    }
  }, [attachmentsState.items, confirm, dataGateway.admin, loadAttachments, openToast, prompt, selectedAttachmentSet]);

  const selectionDisabled = loading || batchActioning || actioningAttachmentID !== null;

  return (
    <AdminPageCard className="border border-slate-200 bg-white shadow-sm" contentClassName="space-y-4 px-5 pb-5 pt-4">
      <form className="grid gap-3 xl:grid-cols-[minmax(0,2fr)_minmax(220px,1fr)_minmax(220px,1fr)_180px_190px_auto]" onSubmit={handleSearchSubmit}>
        <label className="space-y-1.5">
          <span className="text-xs font-medium text-slate-600">关键词</span>
          <div className="relative">
            <Search className="pointer-events-none absolute left-2 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <Input
              value={keywordInput}
              onChange={(event) => setKeywordInput(event.target.value)}
              placeholder="附件名 / 附件ID / 文档名 / 空间名"
              className="h-9 pl-8"
            />
          </div>
        </label>

        <label className="space-y-1.5">
          <span className="text-xs font-medium text-slate-600">空间 ID</span>
          <Input
            value={spaceIdInput}
            onChange={(event) => setSpaceIdInput(event.target.value)}
            placeholder="按空间过滤"
            className="h-9"
          />
        </label>

        <label className="space-y-1.5">
          <span className="text-xs font-medium text-slate-600">文档 ID</span>
          <Input
            value={documentIdInput}
            onChange={(event) => setDocumentIdInput(event.target.value)}
            placeholder="按文档过滤"
            className="h-9"
          />
        </label>

        <label className="space-y-1.5">
          <span className="text-xs font-medium text-slate-600">状态</span>
          <Select value={statusFilter || "__EMPTY__"} onValueChange={(value) => setStatusFilter(value === "__EMPTY__" ? "" : value as typeof statusFilter)}>
            <SelectTrigger className="h-9">
              <SelectValue placeholder="全部状态" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__EMPTY__">全部状态</SelectItem>
              <SelectItem value="all">全部（含已删）</SelectItem>
              <SelectItem value="active">仅正常</SelectItem>
              <SelectItem value="deleted">仅已删除</SelectItem>
            </SelectContent>
          </Select>
        </label>

        <label className="space-y-1.5">
          <span className="text-xs font-medium text-slate-600">存储提供商</span>
          <Select
            value={storageProviderFilter || "__EMPTY__"}
            onValueChange={(value) => setStorageProviderFilter(value === "__EMPTY__" ? "" : value as typeof storageProviderFilter)}
          >
            <SelectTrigger className="h-9">
              <SelectValue placeholder="全部提供商" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__EMPTY__">全部提供商</SelectItem>
              <SelectItem value="all">全部</SelectItem>
              <SelectItem value="local">本地</SelectItem>
              <SelectItem value="cloudflare-r2">Cloudflare R2</SelectItem>
              <SelectItem value="aliyun-oss">阿里云 OSS</SelectItem>
            </SelectContent>
          </Select>
        </label>

        <AdminToolbarActions className="xl:justify-self-end">
          <Button type="submit" size="sm" className="h-9">
            查询
          </Button>
          <Button type="button" size="sm" variant="outline" className="h-9" onClick={handleReset}>
            重置
          </Button>
          <Button type="button" size="icon" variant="outline" className="h-9 w-9" onClick={() => void loadAttachments()} disabled={loading}>
            {loading ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
            <span className="sr-only">刷新</span>
          </Button>
        </AdminToolbarActions>
      </form>

      <AdminBulkActionBar selectedCount={selectedAttachmentIDs.length}>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="border-rose-200 bg-rose-50 text-rose-700 shadow-none hover:bg-rose-100"
          disabled={selectionDisabled || selectedAttachmentIDs.length === 0}
          onClick={() => void handleBatchDelete()}
        >
          批量删除
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="border-slate-300 bg-white text-slate-700 shadow-none hover:bg-slate-50"
          disabled={selectionDisabled || selectedAttachmentIDs.length === 0}
          onClick={() => setSelectedAttachmentIDs([])}
        >
          清空选择
        </Button>
      </AdminBulkActionBar>

      <AdminTableContainer>
        <table className="min-w-full table-fixed border-collapse text-sm">
          <thead className="bg-slate-50/80 text-left text-xs uppercase tracking-wide text-slate-500">
            <tr>
              <th className="w-10 px-3 py-2 font-semibold">
                <Checkbox
                  checked={allSelectableChecked}
                  disabled={selectionDisabled || selectableAttachmentIDs.length === 0}
                  onCheckedChange={(checked) => handleToggleSelectAll(checked === true)}
                  aria-label="全选附件"
                />
              </th>
              <th className="w-[260px] px-3 py-2 font-semibold">附件</th>
              <th className="w-[300px] px-3 py-2 font-semibold">所属文档</th>
              <th className="w-[240px] px-3 py-2 font-semibold">所属空间</th>
              <th className="w-[150px] px-3 py-2 font-semibold">存储</th>
              <th className="w-[130px] px-3 py-2 font-semibold">状态</th>
              <th className="w-[190px] px-3 py-2 font-semibold">创建时间</th>
              <th className="w-[190px] px-3 py-2 font-semibold">更新时间</th>
              <th className="w-[220px] px-3 py-2 font-semibold">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100 text-slate-700">
            {attachmentsState.items.length === 0 ? (
              <tr>
                <td className="px-3 py-9 text-center text-sm text-slate-500" colSpan={9}>
                  {loading ? "正在加载附件..." : "暂无匹配的附件记录"}
                </td>
              </tr>
            ) : (
              attachmentsState.items.map((attachment) => {
                const actioning = actioningAttachmentID === attachment.attachmentId || batchActioning;
                const isDeleted = attachment.status === "deleted";
                return (
                  <tr key={attachment.attachmentId} className="align-top hover:bg-slate-50/60">
                    <td className="px-3 py-2.5">
                      <Checkbox
                        checked={selectedAttachmentSet.has(attachment.attachmentId)}
                        disabled={selectionDisabled}
                        onCheckedChange={(checked) => handleToggleSelectOne(attachment.attachmentId, checked === true)}
                        aria-label={`选择附件 ${attachment.fileName || attachment.attachmentId}`}
                      />
                    </td>
                    <td className="px-3 py-2.5">
                      <div className="space-y-1">
                        <p className="line-clamp-2 break-all font-medium text-slate-900">{attachment.fileName || attachment.attachmentId}</p>
                        <p className="break-all text-xs text-slate-500">{attachment.attachmentId}</p>
                        <p className="text-xs text-slate-500">{attachment.mimeType || "application/octet-stream"}</p>
                        <p className="text-xs text-slate-500">
                          {formatBytes(attachment.sizeBytes)} · {renderPreviewKindLabel(attachment.previewKind)}
                        </p>
                      </div>
                    </td>
                    <td className="px-3 py-2.5">
                      <div className="space-y-1">
                        <p className="line-clamp-2 break-all text-sm font-medium text-slate-900">
                          {attachment.documentTitle || attachment.documentId}
                        </p>
                        <p className="break-all text-xs text-slate-500">{attachment.documentId}</p>
                        <p className="text-xs text-slate-500">{renderDocumentStatusLabel(attachment.documentStatus)}</p>
                      </div>
                    </td>
                    <td className="px-3 py-2.5">
                      <div className="space-y-1">
                        <p className="line-clamp-2 break-all text-sm font-medium text-slate-900">
                          {attachment.spaceName || attachment.spaceId}
                        </p>
                        <p className="break-all text-xs text-slate-500">{attachment.spaceId}</p>
                        <p className="line-clamp-1 text-xs text-slate-500">
                          所有者：{attachment.spaceOwnerName || attachment.spaceOwnerEmail || attachment.spaceOwnerUserId}
                        </p>
                      </div>
                    </td>
                    <td className="px-3 py-2.5">
                      <Badge variant="outline" className="border-slate-200 bg-slate-50 text-slate-700">
                        {renderStorageProviderLabel(attachment.storageProvider)}
                      </Badge>
                    </td>
                    <td className="px-3 py-2.5">
                      <Badge variant="outline" className={renderStatusBadgeClass(attachment.status)}>
                        {renderStatusLabel(attachment.status)}
                      </Badge>
                    </td>
                    <td className="px-3 py-2.5 text-xs text-slate-600">{formatDateTime(attachment.createdAt)}</td>
                    <td className="px-3 py-2.5 text-xs text-slate-600">{formatDateTime(attachment.updatedAt)}</td>
                    <td className="px-3 py-2.5">
                      <div className="inline-flex rounded-md">
                        <Button
                          type="button"
                          size="sm"
                          variant="outline"
                          className="h-7 rounded-r-none border-r-0 px-2 text-xs"
                          disabled={actioning || isDeleted}
                          onClick={() => openPathInNewTab(buildAttachmentPreviewPath(attachment.documentId, attachment.attachmentId))}
                        >
                          <ExternalLink className="mr-1 h-3.5 w-3.5" />
                          查看
                        </Button>
                        <DropdownMenu modal={false}>
                          <DropdownMenuTrigger asChild>
                            <Button
                              type="button"
                              size="sm"
                              variant="outline"
                              className="h-7 w-8 rounded-l-none px-0"
                              disabled={actioning}
                              aria-label="展开更多操作"
                            >
                              <ChevronDown size={14} />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end" className="w-40">
                            <DropdownMenuItem
                              disabled={isDeleted}
                              onSelect={() => openPathInNewTab(buildDocumentReaderPath(attachment.spaceId, attachment.documentId))}
                            >
                              <ExternalLink size={14} className="mr-2" />
                              <span>查看文档</span>
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              disabled={actioning}
                              onSelect={() => void handleDelete(attachment)}
                              className="text-rose-600 focus:text-rose-600"
                            >
                              {actioning ? (
                                <LoaderCircle size={14} className="mr-2 animate-spin" />
                              ) : (
                                <Trash2 size={14} className="mr-2" />
                              )}
                              <span>删除</span>
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </div>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </AdminTableContainer>

      <AdminPaginationFooter
        summary={
          loading
            ? "正在加载附件列表..."
            : `共 ${attachmentsState.pagination.total} 条，当前第 ${attachmentsState.pagination.page} / ${totalPages} 页`
        }
        previousDisabled={loading || page <= 1}
        nextDisabled={loading || page >= totalPages}
        onPrevious={() => setPage((previousPage) => Math.max(1, previousPage - 1))}
        onNext={() => setPage((previousPage) => Math.min(totalPages, previousPage + 1))}
      />

      {dialogs}
    </AdminPageCard>
  );
}
