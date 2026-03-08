import { LoaderCircle, RefreshCw, Search, ShieldBan, ShieldCheck, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEventHandler } from "react";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { Checkbox } from "../../components/ui/checkbox";
import { Input } from "../../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { showToast } from "../../components/ui/toast";
import { type AdminDocument, type AdminDocumentListResult, type DataGateway, type Visibility } from "../../data-access";
import { useAdminDialogs } from "../components/AdminDialogs";
import {
  AdminBulkActionBar,
  AdminPageCard,
  AdminPaginationFooter,
  AdminTableContainer,
  AdminToolbarActions
} from "../components/AdminPageLayout";
import { formatError } from "../../editor/status-utils";

const DEFAULT_PAGE_SIZE = 20;

interface AdminDocumentsPageProps {
  dataGateway: DataGateway;
}

interface AdminDocumentsState {
  items: AdminDocument[];
  pagination: AdminDocumentListResult["pagination"];
}

function emptyDocumentsState(): AdminDocumentsState {
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

function renderStatusLabel(value: AdminDocument["status"]): string {
  switch (value) {
    case "active":
      return "正常";
    case "banned":
      return "已封禁";
    case "deleted":
      return "已删除";
    default:
      return value;
  }
}

function renderStatusBadgeClass(value: AdminDocument["status"]): string {
  switch (value) {
    case "active":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "banned":
      return "border-rose-200 bg-rose-50 text-rose-700";
    case "deleted":
      return "border-slate-200 bg-slate-100 text-slate-600";
    default:
      return "border-slate-200 bg-slate-100 text-slate-600";
  }
}

function renderVisibilityBadgeClass(value: Visibility): string {
  switch (value) {
    case "public":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "authenticated":
      return "border-sky-200 bg-sky-50 text-sky-700";
    case "member":
      return "border-slate-200 bg-slate-100 text-slate-700";
    default:
      return "border-slate-200 bg-slate-100 text-slate-700";
  }
}

function renderFormatLabel(value?: AdminDocument["format"]): string {
  switch (value) {
    case "docx":
      return "Word";
    case "xlsx":
      return "Excel";
    default:
      return "Markdown";
  }
}

function renderFormatBadgeClass(value?: AdminDocument["format"]): string {
  switch (value) {
    case "docx":
      return "border-sky-200 bg-sky-50 text-sky-700";
    case "xlsx":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    default:
      return "border-amber-200 bg-amber-50 text-amber-700";
  }
}

function buildSpaceReaderPath(spaceId: string): string {
  return `/r/${encodeURIComponent(spaceId)}`;
}

function buildDocumentReaderPath(spaceId: string, documentRouteKey: string): string {
  return `${buildSpaceReaderPath(spaceId)}/${encodeURIComponent(documentRouteKey)}`;
}

export function AdminDocumentsPage({ dataGateway }: AdminDocumentsPageProps) {
  const { confirm, prompt, dialogs } = useAdminDialogs();

  const [keywordInput, setKeywordInput] = useState("");
  const [keyword, setKeyword] = useState("");
  const [spaceIdInput, setSpaceIdInput] = useState("");
  const [spaceId, setSpaceId] = useState("");
  const [statusFilter, setStatusFilter] = useState<"" | "all" | "active" | "banned" | "deleted">("");
  const [visibilityFilter, setVisibilityFilter] = useState<"" | "all" | "public" | "authenticated" | "member">("");
  const [formatFilter, setFormatFilter] = useState<"" | "all" | "markdown" | "docx" | "xlsx">("");
  const [page, setPage] = useState(1);
  const [selectedDocumentIDs, setSelectedDocumentIDs] = useState<string[]>([]);

  const [documentsState, setDocumentsState] = useState<AdminDocumentsState>(() => emptyDocumentsState());
  const [loading, setLoading] = useState(false);

  const [actioningDocumentID, setActioningDocumentID] = useState<string | null>(null);
  const [batchActioning, setBatchActioning] = useState(false);

  const openToast = useCallback((message: string, variant: "success" | "info" | "error" = "error") => {
    showToast(message, variant);
  }, []);

  const loadDocuments = useCallback(async () => {
    setLoading(true);
    try {
      const payload = await dataGateway.admin.listDocuments({
        keyword,
        spaceId,
        status: statusFilter || undefined,
        visibility: visibilityFilter || undefined,
        format: formatFilter || undefined,
        page,
        pageSize: DEFAULT_PAGE_SIZE
      });
      setDocumentsState(payload);
    } catch (error) {
      openToast(`加载文档列表失败：${formatError(error)}`);
      setDocumentsState(emptyDocumentsState());
    } finally {
      setLoading(false);
    }
  }, [dataGateway, formatFilter, keyword, openToast, page, spaceId, statusFilter, visibilityFilter]);

  useEffect(() => {
    void loadDocuments();
  }, [loadDocuments]);

  useEffect(() => {
    setSelectedDocumentIDs((previous) =>
      previous.filter((documentID) =>
        documentsState.items.some((item) => item.documentId === documentID && item.status !== "deleted")
      )
    );
  }, [documentsState.items]);

  const selectedDocumentSet = useMemo(() => new Set(selectedDocumentIDs), [selectedDocumentIDs]);
  const selectableDocumentIDs = useMemo(
    () => documentsState.items.filter((item) => item.status !== "deleted").map((item) => item.documentId),
    [documentsState.items]
  );
  const allSelectableChecked = useMemo(
    () => selectableDocumentIDs.length > 0 && selectableDocumentIDs.every((documentID) => selectedDocumentSet.has(documentID)),
    [selectableDocumentIDs, selectedDocumentSet]
  );

  const totalPages = useMemo(() => {
    const total = documentsState.pagination.total;
    const pageSize = documentsState.pagination.pageSize || DEFAULT_PAGE_SIZE;
    return Math.max(1, Math.ceil(total / pageSize));
  }, [documentsState.pagination.pageSize, documentsState.pagination.total]);

  const handleSearchSubmit = useCallback<FormEventHandler<HTMLFormElement>>(
    (event) => {
      event.preventDefault();
      setPage(1);
      setKeyword(keywordInput.trim());
      setSpaceId(spaceIdInput.trim());
    },
    [keywordInput, spaceIdInput]
  );

  const handleReset = useCallback(() => {
    setKeywordInput("");
    setKeyword("");
    setSpaceIdInput("");
    setSpaceId("");
    setStatusFilter("");
    setVisibilityFilter("");
    setFormatFilter("");
    setPage(1);
  }, []);

  const runDocumentAction = useCallback(
    async (documentID: string, callback: () => Promise<void>) => {
      setActioningDocumentID(documentID);
      try {
        await callback();
        await loadDocuments();
      } catch (error) {
        openToast(`执行操作失败：${formatError(error)}`);
      } finally {
        setActioningDocumentID(null);
      }
    },
    [loadDocuments, openToast]
  );

  const runBatchDocumentAction = useCallback(
    async (
      actionName: string,
      filter: (document: AdminDocument) => boolean,
      callback: (document: AdminDocument) => Promise<void>
    ) => {
      const targets = documentsState.items.filter((document) => selectedDocumentSet.has(document.documentId) && filter(document));
      if (targets.length === 0) {
        openToast("请先选择可操作的文档");
        return;
      }

      setBatchActioning(true);
      let successCount = 0;
      const failures: string[] = [];
      try {
        for (const target of targets) {
          try {
            await callback(target);
            successCount += 1;
          } catch (error) {
            failures.push(`${target.title || target.documentId}: ${formatError(error)}`);
          }
        }
        await loadDocuments();
        setSelectedDocumentIDs([]);
        if (failures.length > 0) {
          openToast(`批量${actionName}完成：成功 ${successCount}，失败 ${failures.length}。首个失败：${failures[0]}`);
        }
      } finally {
        setBatchActioning(false);
      }
    },
    [documentsState.items, loadDocuments, openToast, selectedDocumentSet]
  );

  const handleUpdateStatus = useCallback(
    async (document: AdminDocument, status: "active" | "banned") => {
      if (status === "banned") {
        const promptResult = await prompt({
          title: `封禁文档：${document.title || document.documentId}`,
          description: "请输入封禁原因，封禁后文档不可访问。",
          confirmText: "确认封禁",
          tone: "danger",
          fields: [
            {
              key: "reason",
              label: "封禁原因",
              required: true,
              defaultValue: document.bannedReason || "",
              placeholder: "例如：内容违规"
            }
          ]
        });
        if (!promptResult) {
          return;
        }
        const reason = (promptResult.reason ?? "").trim();
        if (!reason) {
          openToast("封禁原因不能为空");
          return;
        }
        await runDocumentAction(document.documentId, async () => {
          await dataGateway.admin.updateDocumentStatus({
            documentId: document.documentId,
            status: "banned",
            reason
          });
        });
        return;
      }

      const confirmed = await confirm({
        title: `解封文档：${document.title || document.documentId}`,
        description: "确认后文档状态将恢复为正常。",
        confirmText: "确认解封"
      });
      if (!confirmed) {
        return;
      }
      await runDocumentAction(document.documentId, async () => {
        await dataGateway.admin.updateDocumentStatus({
          documentId: document.documentId,
          status: "active"
        });
      });
    },
    [confirm, dataGateway.admin, openToast, prompt, runDocumentAction]
  );

  const handleDelete = useCallback(
    async (document: AdminDocument) => {
      const confirmed = await confirm({
        title: `删除文档：${document.title || document.documentId}`,
        description: "该操作为软删除，确认后文档不可继续访问。",
        confirmText: "确认删除",
        tone: "danger"
      });
      if (!confirmed) {
        return;
      }
      await runDocumentAction(document.documentId, async () => {
        await dataGateway.admin.deleteDocument(document.documentId);
      });
    },
    [confirm, dataGateway.admin, runDocumentAction]
  );

  const handleToggleSelectAll = useCallback(
    (checked: boolean) => {
      setSelectedDocumentIDs(checked ? selectableDocumentIDs : []);
    },
    [selectableDocumentIDs]
  );

  const handleToggleSelectOne = useCallback((documentID: string, checked: boolean) => {
    setSelectedDocumentIDs((previous) => {
      if (checked) {
        if (previous.includes(documentID)) {
          return previous;
        }
        return [...previous, documentID];
      }
      return previous.filter((value) => value !== documentID);
    });
  }, []);

  const handleBatchBan = useCallback(async () => {
    const promptResult = await prompt({
      title: "批量封禁文档",
      description: "封禁原因会应用到当前所有选中文档。",
      confirmText: "确认封禁",
      tone: "danger",
      fields: [
        {
          key: "reason",
          label: "封禁原因",
          required: true,
          defaultValue: "",
          placeholder: "请输入统一封禁原因"
        }
      ]
    });
    if (!promptResult) {
      return;
    }
    const reason = (promptResult.reason ?? "").trim();
    if (!reason) {
      openToast("封禁原因不能为空");
      return;
    }

    await runBatchDocumentAction(
      "封禁",
      (document) => document.status === "active",
      async (document) => {
        await dataGateway.admin.updateDocumentStatus({
          documentId: document.documentId,
          status: "banned",
          reason
        });
      }
    );
  }, [dataGateway.admin, openToast, prompt, runBatchDocumentAction]);

  const handleBatchUnban = useCallback(async () => {
    const confirmed = await confirm({
      title: "批量解封文档",
      description: "确认后将解封所有已选中且当前为封禁状态的文档。",
      confirmText: "确认解封"
    });
    if (!confirmed) {
      return;
    }
    await runBatchDocumentAction(
      "解封",
      (document) => document.status === "banned",
      async (document) => {
        await dataGateway.admin.updateDocumentStatus({
          documentId: document.documentId,
          status: "active"
        });
      }
    );
  }, [confirm, dataGateway.admin, runBatchDocumentAction]);

  const handleBatchDelete = useCallback(async () => {
    const confirmed = await confirm({
      title: "批量删除文档",
      description: "该操作为软删除，确认后将删除所有选中文档。",
      confirmText: "确认删除",
      tone: "danger"
    });
    if (!confirmed) {
      return;
    }
    await runBatchDocumentAction(
      "删除",
      (document) => document.status !== "deleted",
      async (document) => {
        await dataGateway.admin.deleteDocument(document.documentId);
      }
    );
  }, [confirm, dataGateway.admin, runBatchDocumentAction]);

  const selectionDisabled = loading || batchActioning || actioningDocumentID !== null;

  return (
    <section aria-label="文档管理">
      {dialogs}
      <AdminPageCard>
          <form className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_220px_160px_180px_170px_auto]" onSubmit={handleSearchSubmit}>
            <label className="space-y-1.5">
              <span className="text-xs font-semibold tracking-wide text-slate-600">关键词</span>
              <Input
                type="search"
                value={keywordInput}
                placeholder="文档标题 / 文档 ID / 节点 ID"
                onChange={(event) => setKeywordInput(event.target.value)}
              />
            </label>
            <label className="space-y-1.5">
              <span className="text-xs font-semibold tracking-wide text-slate-600">空间 ID</span>
              <Input
                type="search"
                value={spaceIdInput}
                placeholder="按空间过滤"
                onChange={(event) => setSpaceIdInput(event.target.value)}
              />
            </label>
            <label className="space-y-1.5">
              <span className="text-xs font-semibold tracking-wide text-slate-600">状态</span>
              <Select
                value={statusFilter || "default"}
                onValueChange={(value) => {
                  setStatusFilter((value === "default" ? "" : value) as "" | "all" | "active" | "banned" | "deleted");
                  setPage(1);
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="default">未删除（默认）</SelectItem>
                  <SelectItem value="all">全部</SelectItem>
                  <SelectItem value="active">正常</SelectItem>
                  <SelectItem value="banned">封禁</SelectItem>
                  <SelectItem value="deleted">已删除</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label className="space-y-1.5">
              <span className="text-xs font-semibold tracking-wide text-slate-600">可见性</span>
              <Select
                value={visibilityFilter || "default"}
                onValueChange={(value) => {
                  setVisibilityFilter((value === "default" ? "" : value) as "" | "all" | "public" | "authenticated" | "member");
                  setPage(1);
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="default">全部可见性（默认）</SelectItem>
                  <SelectItem value="all">全部</SelectItem>
                  <SelectItem value="public">完全公开</SelectItem>
                  <SelectItem value="authenticated">登录可见</SelectItem>
                  <SelectItem value="member">成员可见</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label className="space-y-1.5">
              <span className="text-xs font-semibold tracking-wide text-slate-600">格式</span>
              <Select
                value={formatFilter || "default"}
                onValueChange={(value) => {
                  setFormatFilter((value === "default" ? "" : value) as "" | "all" | "markdown" | "docx" | "xlsx");
                  setPage(1);
                }}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="default">全部格式（默认）</SelectItem>
                  <SelectItem value="all">全部</SelectItem>
                  <SelectItem value="markdown">Markdown</SelectItem>
                  <SelectItem value="docx">Word</SelectItem>
                  <SelectItem value="xlsx">Excel</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <AdminToolbarActions>
                <Button type="submit" disabled={loading || batchActioning}>
                  <Search size={14} />
                  <span>查询</span>
                </Button>
                <Button type="button" variant="outline" disabled={loading || batchActioning} onClick={handleReset}>
                  重置
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  disabled={loading || batchActioning}
                  onClick={() => void loadDocuments()}
                >
                  <RefreshCw size={14} />
                  <span>刷新</span>
                </Button>
            </AdminToolbarActions>
          </form>

          <AdminBulkActionBar selectedCount={selectedDocumentIDs.length}>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="border-amber-200 bg-amber-50 text-amber-700 shadow-none hover:bg-amber-100"
                disabled={selectionDisabled || selectedDocumentIDs.length === 0}
                onClick={() => void handleBatchBan()}
              >
                批量封禁
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="border-slate-300 bg-white text-slate-700 shadow-none hover:bg-slate-50"
                disabled={selectionDisabled || selectedDocumentIDs.length === 0}
                onClick={() => void handleBatchUnban()}
              >
                批量解封
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="border-rose-200 bg-rose-50 text-rose-700 shadow-none hover:bg-rose-100"
                disabled={selectionDisabled || selectedDocumentIDs.length === 0}
                onClick={() => void handleBatchDelete()}
              >
                批量删除
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="border-slate-300 bg-white text-slate-700 shadow-none hover:bg-slate-50"
                disabled={selectionDisabled || selectedDocumentIDs.length === 0}
                onClick={() => setSelectedDocumentIDs([])}
              >
                清空选择
              </Button>
          </AdminBulkActionBar>

          <AdminTableContainer>
              <table className="w-full min-w-[1240px] table-fixed border-collapse text-left text-sm">
                <thead className="sticky top-0 z-10 bg-slate-50/95 backdrop-blur">
                  <tr className="text-xs uppercase tracking-wide text-slate-600">
                    <th className="w-10 border-b border-slate-200 px-3 py-2 font-semibold">
                      <Checkbox
                        checked={allSelectableChecked}
                        disabled={selectionDisabled || selectableDocumentIDs.length === 0}
                        onCheckedChange={(checked) => handleToggleSelectAll(checked === true)}
                        aria-label="全选文档"
                      />
                    </th>
                    <th className="w-[360px] border-b border-slate-200 px-3 py-2 font-semibold">文档信息</th>
                    <th className="w-[300px] border-b border-slate-200 px-3 py-2 font-semibold">所属空间</th>
                    <th className="w-[120px] border-b border-slate-200 px-3 py-2 font-semibold">可见性</th>
                    <th className="w-[140px] border-b border-slate-200 px-3 py-2 font-semibold">状态</th>
                    <th className="w-[170px] border-b border-slate-200 px-3 py-2 font-semibold">更新时间</th>
                    <th className="w-[240px] border-b border-slate-200 px-3 py-2 font-semibold">操作</th>
                  </tr>
                </thead>
                <tbody>
                  {loading ? (
                    <tr>
                      <td colSpan={7} className="px-3 py-12">
                        <div className="flex items-center justify-center gap-2 text-sm text-slate-500">
                          <LoaderCircle size={15} className="animate-spin" />
                          <span>正在加载文档列表...</span>
                        </div>
                      </td>
                    </tr>
                  ) : documentsState.items.length === 0 ? (
                    <tr>
                      <td colSpan={7} className="px-3 py-12 text-center text-sm text-slate-500">
                        暂无符合条件的数据
                      </td>
                    </tr>
                  ) : (
                    documentsState.items.map((document) => {
                      const isActioning = actioningDocumentID === document.documentId || batchActioning;
                      const isDeleted = document.status === "deleted";
                      const spaceReaderPath = buildSpaceReaderPath(document.spaceId);
                      const documentRouteKey = (document.documentRouteKey || document.documentId || "").trim();
                      const documentReaderPath = buildDocumentReaderPath(document.spaceId, documentRouteKey);
                      return (
                        <tr key={document.documentId} className="border-b border-slate-100 align-top text-slate-700">
                          <td className="min-w-0 px-3 py-3">
                            <Checkbox
                              checked={selectedDocumentSet.has(document.documentId)}
                              disabled={selectionDisabled || isDeleted}
                              onCheckedChange={(checked) => handleToggleSelectOne(document.documentId, checked === true)}
                              aria-label={`选择文档 ${document.title || document.documentId}`}
                            />
                          </td>
                          <td className="min-w-0 px-3 py-3">
                            <div className="grid gap-1">
                              <a
                                className={`w-fit text-sm font-semibold no-underline transition-colors hover:no-underline ${
                                  isDeleted
                                    ? "pointer-events-none text-slate-400"
                                    : "text-slate-900 hover:text-sky-700"
                                }`}
                                href={documentReaderPath}
                                target="_blank"
                                rel="noopener noreferrer"
                                title={isDeleted ? "已删除文档不可访问" : "打开文档阅读页"}
                                aria-disabled={isDeleted}
                              >
                                {document.title || "未命名文档"}
                              </a>
                              <div className="flex min-w-0 items-center gap-1.5 text-xs">
                                <Badge variant="outline" className={renderFormatBadgeClass(document.format)}>
                                  {renderFormatLabel(document.format)}
                                </Badge>
                              </div>
                              <div className="flex min-w-0 items-center gap-1.5 text-xs">
                                <span className="shrink-0 text-slate-500">文档 ID</span>
                                <code
                                  className="max-w-[280px] truncate rounded border border-sky-200 bg-sky-50 px-1.5 py-0.5 text-sky-700"
                                  title={document.documentId}
                                >
                                  {document.documentId}
                                </code>
                              </div>
                              <div className="flex min-w-0 items-center gap-1.5 text-xs">
                                <span className="shrink-0 text-slate-500">节点 ID</span>
                                <code
                                  className="max-w-[280px] truncate rounded border border-slate-200 bg-slate-100 px-1.5 py-0.5 text-slate-600"
                                  title={document.nodeId}
                                >
                                  {document.nodeId}
                                </code>
                              </div>
                            </div>
                          </td>
                          <td className="px-3 py-3">
                            <div className="grid gap-1 text-xs text-slate-600">
                              <a
                                className={`w-fit font-semibold no-underline transition-colors hover:no-underline ${
                                  isDeleted
                                    ? "pointer-events-none text-slate-400"
                                    : "text-slate-800 hover:text-sky-700"
                                }`}
                                href={spaceReaderPath}
                                target="_blank"
                                rel="noopener noreferrer"
                                title={isDeleted ? "已删除文档所属空间不可访问" : "打开空间阅读页"}
                                aria-disabled={isDeleted}
                              >
                                {document.spaceName || "-"}
                              </a>
                              <span>
                                {document.spaceOwnerName || "-"} / {document.spaceOwnerEmail || "-"}
                              </span>
                              <span>{document.spaceId}</span>
                            </div>
                          </td>
                          <td className="px-3 py-3">
                            <Badge variant="outline" className={renderVisibilityBadgeClass(document.visibility)}>
                              {renderVisibilityLabel(document.visibility)}
                            </Badge>
                          </td>
                          <td className="px-3 py-3">
                            <div className="grid gap-1.5">
                              <Badge variant="outline" className={renderStatusBadgeClass(document.status)}>
                                {renderStatusLabel(document.status)}
                              </Badge>
                              {document.status === "banned" && document.bannedReason ? (
                                <small className="text-xs text-rose-600">{document.bannedReason}</small>
                              ) : null}
                            </div>
                          </td>
                          <td className="px-3 py-3 text-xs text-slate-600">{formatDateTime(document.updatedAt)}</td>
                          <td className="px-3 py-3">
                            <div className="flex flex-wrap items-center gap-2">
                              {document.status === "banned" ? (
                                <Button
                                  type="button"
                                  size="sm"
                                  variant="secondary"
                                  disabled={isActioning || isDeleted}
                                  onClick={() => void handleUpdateStatus(document, "active")}
                                >
                                  <ShieldCheck size={14} />
                                  <span>解封</span>
                                </Button>
                              ) : document.status === "active" ? (
                                <Button
                                  type="button"
                                  size="sm"
                                  variant="outline"
                                  className="border-amber-200 bg-amber-50 text-amber-700 hover:bg-amber-100"
                                  disabled={isActioning || isDeleted}
                                  onClick={() => void handleUpdateStatus(document, "banned")}
                                >
                                  <ShieldBan size={14} />
                                  <span>封禁</span>
                                </Button>
                              ) : (
                                <span className="text-xs font-medium text-slate-400">-</span>
                              )}
                              <Button
                                type="button"
                                size="sm"
                                variant="destructive"
                                disabled={isActioning || isDeleted}
                                onClick={() => void handleDelete(document)}
                              >
                                <Trash2 size={14} />
                                <span>删除</span>
                              </Button>
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
              <>
              当前第 {documentsState.pagination.page} / {totalPages} 页，共 {documentsState.pagination.total} 条
              </>
            }
            previousDisabled={loading || documentsState.pagination.page <= 1}
            nextDisabled={loading || documentsState.pagination.page >= totalPages}
            onPrevious={() => setPage((value) => Math.max(1, value - 1))}
            onNext={() => setPage((value) => Math.min(totalPages, value + 1))}
          />
      </AdminPageCard>
    </section>
  );
}
