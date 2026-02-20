import { LoaderCircle, RefreshCw, Search, ShieldBan, ShieldCheck, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { type AdminDocument, type AdminDocumentListResult, type DataGateway, type Visibility } from "../../data-access";
import { useAdminDialogs } from "../components/AdminDialogs";
import { TopToast, type TopToastVariant } from "../../components/TopToast";
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

export function AdminDocumentsPage({ dataGateway }: AdminDocumentsPageProps) {
  const { confirm, prompt, dialogs } = useAdminDialogs();

  const [keywordInput, setKeywordInput] = useState("");
  const [keyword, setKeyword] = useState("");
  const [spaceIdInput, setSpaceIdInput] = useState("");
  const [spaceId, setSpaceId] = useState("");
  const [statusFilter, setStatusFilter] = useState<"" | "all" | "active" | "banned" | "deleted">("");
  const [visibilityFilter, setVisibilityFilter] = useState<"" | "all" | "public" | "authenticated" | "member">("");
  const [page, setPage] = useState(1);
  const [selectedDocumentIDs, setSelectedDocumentIDs] = useState<string[]>([]);

  const [documentsState, setDocumentsState] = useState<AdminDocumentsState>(() => emptyDocumentsState());
  const [loading, setLoading] = useState(false);

  const [actioningDocumentID, setActioningDocumentID] = useState<string | null>(null);
  const [batchActioning, setBatchActioning] = useState(false);
  const [toastState, setToastState] = useState<{
    open: boolean;
    message: string;
    variant: TopToastVariant;
    triggerKey: number;
  }>({
    open: false,
    message: "",
    variant: "error",
    triggerKey: 0
  });

  const openToast = useCallback((message: string, variant: TopToastVariant = "error") => {
    const normalizedMessage = message.trim();
    if (!normalizedMessage) {
      return;
    }
    setToastState((previousState) => ({
      open: true,
      message: normalizedMessage,
      variant,
      triggerKey: previousState.triggerKey + 1
    }));
  }, []);

  const closeToast = useCallback(() => {
    setToastState((previousState) => {
      if (!previousState.open) {
        return previousState;
      }
      return {
        ...previousState,
        open: false
      };
    });
  }, []);

  const loadDocuments = useCallback(async () => {
    setLoading(true);
    try {
      const payload = await dataGateway.admin.listDocuments({
        keyword,
        spaceId,
        status: statusFilter || undefined,
        visibility: visibilityFilter || undefined,
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
  }, [dataGateway, keyword, openToast, page, spaceId, statusFilter, visibilityFilter]);

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

  const handleSearchSubmit = useCallback(
    (event: FormEvent<HTMLFormElement>) => {
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
    <section className="admin-spaces-panel" aria-label="文档管理">
      <TopToast
        open={toastState.open}
        message={toastState.message}
        variant={toastState.variant}
        triggerKey={toastState.triggerKey}
        durationMs={2800}
        onClose={closeToast}
      />
      {dialogs}
      <form className="admin-spaces-toolbar admin-documents-toolbar" onSubmit={handleSearchSubmit}>
        <label className="admin-spaces-toolbar__field">
          <span>关键词</span>
          <input
            type="search"
            value={keywordInput}
            placeholder="文档标题 / 文档 ID / 节点 ID"
            onChange={(event) => setKeywordInput(event.target.value)}
          />
        </label>
        <label className="admin-spaces-toolbar__field">
          <span>空间 ID</span>
          <input
            type="search"
            value={spaceIdInput}
            placeholder="按空间过滤"
            onChange={(event) => setSpaceIdInput(event.target.value)}
          />
        </label>
        <label className="admin-spaces-toolbar__field">
          <span>状态</span>
          <select
            value={statusFilter}
            onChange={(event) => {
              setStatusFilter(event.target.value as "" | "all" | "active" | "banned" | "deleted");
              setPage(1);
            }}
          >
            <option value="">未删除（默认）</option>
            <option value="all">全部</option>
            <option value="active">正常</option>
            <option value="banned">封禁</option>
            <option value="deleted">已删除</option>
          </select>
        </label>
        <label className="admin-spaces-toolbar__field">
          <span>可见性</span>
          <select
            value={visibilityFilter}
            onChange={(event) => {
              setVisibilityFilter(event.target.value as "" | "all" | "public" | "authenticated" | "member");
              setPage(1);
            }}
          >
            <option value="">全部可见性（默认）</option>
            <option value="all">全部</option>
            <option value="public">完全公开</option>
            <option value="authenticated">登录可见</option>
            <option value="member">成员可见</option>
          </select>
        </label>
        <div className="admin-spaces-toolbar__actions">
          <button type="submit" disabled={loading || batchActioning}>
            <Search size={14} />
            <span>查询</span>
          </button>
          <button type="button" disabled={loading || batchActioning} onClick={handleReset}>
            重置
          </button>
          <button type="button" disabled={loading || batchActioning} onClick={() => void loadDocuments()}>
            <RefreshCw size={14} />
            <span>刷新</span>
          </button>
        </div>
      </form>

      <div className="admin-bulk-bar">
        <p>已选 {selectedDocumentIDs.length} 项</p>
        <div className="admin-bulk-bar__actions">
          <button
            type="button"
            disabled={selectionDisabled || selectedDocumentIDs.length === 0}
            onClick={() => void handleBatchBan()}
          >
            批量封禁
          </button>
          <button
            type="button"
            disabled={selectionDisabled || selectedDocumentIDs.length === 0}
            onClick={() => void handleBatchUnban()}
          >
            批量解封
          </button>
          <button
            type="button"
            className="danger"
            disabled={selectionDisabled || selectedDocumentIDs.length === 0}
            onClick={() => void handleBatchDelete()}
          >
            批量删除
          </button>
          <button
            type="button"
            disabled={selectionDisabled || selectedDocumentIDs.length === 0}
            onClick={() => setSelectedDocumentIDs([])}
          >
            清空选择
          </button>
        </div>
      </div>

      <div className="admin-spaces-table-wrap">
        <table className="admin-spaces-table admin-documents-table">
          <thead>
            <tr>
              <th className="admin-select-cell">
                <input
                  type="checkbox"
                  checked={allSelectableChecked}
                  disabled={selectionDisabled || selectableDocumentIDs.length === 0}
                  onChange={(event) => handleToggleSelectAll(event.target.checked)}
                  aria-label="全选文档"
                />
              </th>
              <th>文档信息</th>
              <th>所属空间</th>
              <th>可见性</th>
              <th>状态</th>
              <th>更新时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={7}>
                  <div className="admin-spaces-loading-row">
                    <LoaderCircle size={15} className="admin-spaces-loading-row__icon" />
                    <span>正在加载文档列表...</span>
                  </div>
                </td>
              </tr>
            ) : documentsState.items.length === 0 ? (
              <tr>
                <td colSpan={7}>
                  <div className="admin-spaces-empty-row">暂无符合条件的数据</div>
                </td>
              </tr>
            ) : (
              documentsState.items.map((document) => {
                const isActioning = actioningDocumentID === document.documentId || batchActioning;
                const isDeleted = document.status === "deleted";
                return (
                  <tr key={document.documentId}>
                    <td className="admin-select-cell">
                      <input
                        type="checkbox"
                        checked={selectedDocumentSet.has(document.documentId)}
                        disabled={selectionDisabled || isDeleted}
                        onChange={(event) => handleToggleSelectOne(document.documentId, event.target.checked)}
                        aria-label={`选择文档 ${document.title || document.documentId}`}
                      />
                    </td>
                    <td>
                      <div className="admin-spaces-space-cell">
                        <strong>{document.title || "未命名文档"}</strong>
                        <code>{document.documentId}</code>
                        <code>{document.nodeId}</code>
                      </div>
                    </td>
                    <td>
                      <div className="admin-spaces-owner-cell">
                        <strong>{document.spaceName || "-"}</strong>
                        <span>{document.spaceOwnerName || "-"} / {document.spaceOwnerEmail || "-"}</span>
                        <span>{document.spaceId}</span>
                      </div>
                    </td>
                    <td>
                      <span className="admin-spaces-visibility">{renderVisibilityLabel(document.visibility)}</span>
                    </td>
                    <td>
                      <div className="admin-spaces-status-cell">
                        <span className={`admin-spaces-status admin-spaces-status--${document.status}`}>
                          {renderStatusLabel(document.status)}
                        </span>
                        {document.status === "banned" && document.bannedReason ? (
                          <small className="admin-spaces-ban-reason">{document.bannedReason}</small>
                        ) : null}
                      </div>
                    </td>
                    <td>{formatDateTime(document.updatedAt)}</td>
                    <td>
                      <div className="admin-spaces-actions">
                        {document.status === "banned" ? (
                          <button
                            type="button"
                            disabled={isActioning || isDeleted}
                            onClick={() => void handleUpdateStatus(document, "active")}
                          >
                            <ShieldCheck size={14} />
                            <span>解封</span>
                          </button>
                        ) : document.status === "active" ? (
                          <button
                            type="button"
                            className="warning"
                            disabled={isActioning || isDeleted}
                            onClick={() => void handleUpdateStatus(document, "banned")}
                          >
                            <ShieldBan size={14} />
                            <span>封禁</span>
                          </button>
                        ) : (
                          <span className="admin-users-actions__disabled">-</span>
                        )}
                        <button
                          type="button"
                          className="danger"
                          disabled={isActioning || isDeleted}
                          onClick={() => void handleDelete(document)}
                        >
                          <Trash2 size={14} />
                          <span>删除</span>
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      <footer className="admin-spaces-footer">
        <p>
          当前第 {documentsState.pagination.page} / {totalPages} 页，共 {documentsState.pagination.total} 条
        </p>
        <div className="admin-spaces-pagination">
          <button
            type="button"
            onClick={() => setPage((value) => Math.max(1, value - 1))}
            disabled={loading || documentsState.pagination.page <= 1}
          >
            上一页
          </button>
          <button
            type="button"
            onClick={() => setPage((value) => Math.min(totalPages, value + 1))}
            disabled={loading || documentsState.pagination.page >= totalPages}
          >
            下一页
          </button>
        </div>
      </footer>
    </section>
  );
}
