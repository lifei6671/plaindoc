import { LoaderCircle, RefreshCw, Search } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import {
  type AdminAuditAction,
  type AdminAuditListResult,
  type AdminAuditLog,
  type AdminAuditModule,
  type DataGateway
} from "../../data-access";
import { TopToast, type TopToastVariant } from "../../components/TopToast";
import { formatError } from "../../editor/status-utils";

const DEFAULT_PAGE_SIZE = 20;

interface AdminAuditsPageProps {
  dataGateway: DataGateway;
}

interface AdminAuditsState {
  items: AdminAuditLog[];
  pagination: AdminAuditListResult["pagination"];
}

function emptyAuditsState(): AdminAuditsState {
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

function toRFC3339(value: string): string | undefined {
  const normalizedValue = value.trim();
  if (!normalizedValue) {
    return undefined;
  }
  const date = new Date(normalizedValue);
  if (Number.isNaN(date.getTime())) {
    return undefined;
  }
  return date.toISOString();
}

function renderModuleLabel(value: AdminAuditModule): string {
  switch (value) {
    case "user":
      return "用户管理";
    case "space":
      return "空间管理";
    case "document":
      return "文档管理";
    case "theme":
      return "主题管理";
    case "system_config":
      return "系统配置";
    default:
      return value;
  }
}

function renderActionLabel(value: AdminAuditAction): string {
  switch (value) {
    case "create":
      return "创建";
    case "update":
      return "更新";
    case "delete":
      return "删除";
    default:
      return value;
  }
}

function formatActorIdentity(audit: AdminAuditLog): string {
  const name = (audit.actorName ?? "").trim();
  const email = (audit.actorEmail ?? "").trim();
  const actorUserID = (audit.actorUserId ?? "").trim();
  if (name && email) {
    return `${name} (${email})`;
  }
  if (email) {
    return email;
  }
  if (name) {
    return name;
  }
  if (actorUserID) {
    return actorUserID;
  }
  return "-";
}

export function AdminAuditsPage({ dataGateway }: AdminAuditsPageProps) {
  const [keywordInput, setKeywordInput] = useState("");
  const [keyword, setKeyword] = useState("");
  const [moduleFilter, setModuleFilter] = useState<"" | "all" | AdminAuditModule>("");
  const [actionFilter, setActionFilter] = useState<"" | "all" | AdminAuditAction>("");

  const [actorUserIDInput, setActorUserIDInput] = useState("");
  const [actorUserID, setActorUserID] = useState("");
  const [targetIDInput, setTargetIDInput] = useState("");
  const [targetID, setTargetID] = useState("");
  const [requestIDInput, setRequestIDInput] = useState("");
  const [requestID, setRequestID] = useState("");

  const [fromInput, setFromInput] = useState("");
  const [fromTime, setFromTime] = useState("");
  const [toInput, setToInput] = useState("");
  const [toTime, setToTime] = useState("");

  const [page, setPage] = useState(1);
  const [expandedAuditID, setExpandedAuditID] = useState<number | null>(null);

  const [auditsState, setAuditsState] = useState<AdminAuditsState>(() => emptyAuditsState());
  const [loading, setLoading] = useState(false);
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

  const loadAudits = useCallback(async () => {
    setLoading(true);
    try {
      const payload = await dataGateway.admin.listAudits({
        keyword,
        module: moduleFilter || undefined,
        action: actionFilter || undefined,
        actorUserId: actorUserID || undefined,
        targetId: targetID || undefined,
        requestId: requestID || undefined,
        from: fromTime || undefined,
        to: toTime || undefined,
        page,
        pageSize: DEFAULT_PAGE_SIZE
      });
      setAuditsState(payload);
    } catch (error) {
      openToast(`加载审计日志失败：${formatError(error)}`);
      setAuditsState(emptyAuditsState());
    } finally {
      setLoading(false);
    }
  }, [actionFilter, actorUserID, dataGateway.admin, fromTime, keyword, moduleFilter, openToast, page, requestID, targetID, toTime]);

  useEffect(() => {
    void loadAudits();
  }, [loadAudits]);

  useEffect(() => {
    if (!expandedAuditID) {
      return;
    }
    if (!auditsState.items.some((item) => item.id === expandedAuditID)) {
      setExpandedAuditID(null);
    }
  }, [auditsState.items, expandedAuditID]);

  const totalPages = useMemo(() => {
    const total = auditsState.pagination.total;
    const pageSize = auditsState.pagination.pageSize || DEFAULT_PAGE_SIZE;
    return Math.max(1, Math.ceil(total / pageSize));
  }, [auditsState.pagination.pageSize, auditsState.pagination.total]);

  const handleSearchSubmit = useCallback(
    (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();

      const parsedFrom = toRFC3339(fromInput);
      const parsedTo = toRFC3339(toInput);
      if (fromInput.trim() && !parsedFrom) {
        openToast("开始时间格式无效，请重新选择");
        return;
      }
      if (toInput.trim() && !parsedTo) {
        openToast("结束时间格式无效，请重新选择");
        return;
      }
      if (parsedFrom && parsedTo && new Date(parsedFrom).getTime() > new Date(parsedTo).getTime()) {
        openToast("开始时间不能晚于结束时间");
        return;
      }

      setPage(1);
      setKeyword(keywordInput.trim());
      setActorUserID(actorUserIDInput.trim());
      setTargetID(targetIDInput.trim());
      setRequestID(requestIDInput.trim());
      setFromTime(parsedFrom ?? "");
      setToTime(parsedTo ?? "");
    },
    [actorUserIDInput, fromInput, keywordInput, openToast, requestIDInput, targetIDInput, toInput]
  );

  const handleReset = useCallback(() => {
    setKeywordInput("");
    setKeyword("");
    setModuleFilter("");
    setActionFilter("");

    setActorUserIDInput("");
    setActorUserID("");
    setTargetIDInput("");
    setTargetID("");
    setRequestIDInput("");
    setRequestID("");

    setFromInput("");
    setFromTime("");
    setToInput("");
    setToTime("");

    setExpandedAuditID(null);
    setPage(1);
  }, []);

  return (
    <section className="admin-spaces-panel" aria-label="审计日志查询">
      <TopToast
        open={toastState.open}
        message={toastState.message}
        variant={toastState.variant}
        triggerKey={toastState.triggerKey}
        durationMs={2800}
        onClose={closeToast}
      />

      <form className="admin-spaces-toolbar admin-audits-toolbar" onSubmit={handleSearchSubmit}>
        <label className="admin-spaces-toolbar__field">
          <span>关键字</span>
          <input
            value={keywordInput}
            onChange={(event) => setKeywordInput(event.target.value)}
            placeholder="摘要/目标 ID/操作者"
            disabled={loading}
          />
        </label>

        <label className="admin-spaces-toolbar__field">
          <span>模块</span>
          <select value={moduleFilter} onChange={(event) => setModuleFilter(event.target.value as typeof moduleFilter)} disabled={loading}>
            <option value="">全部模块</option>
            <option value="user">user</option>
            <option value="space">space</option>
            <option value="document">document</option>
            <option value="theme">theme</option>
            <option value="system_config">system_config</option>
          </select>
        </label>

        <label className="admin-spaces-toolbar__field">
          <span>动作</span>
          <select value={actionFilter} onChange={(event) => setActionFilter(event.target.value as typeof actionFilter)} disabled={loading}>
            <option value="">全部动作</option>
            <option value="create">create</option>
            <option value="update">update</option>
            <option value="delete">delete</option>
          </select>
        </label>

        <label className="admin-spaces-toolbar__field">
          <span>操作者 ID</span>
          <input value={actorUserIDInput} onChange={(event) => setActorUserIDInput(event.target.value)} disabled={loading} />
        </label>

        <label className="admin-spaces-toolbar__field">
          <span>目标 ID</span>
          <input value={targetIDInput} onChange={(event) => setTargetIDInput(event.target.value)} disabled={loading} />
        </label>

        <label className="admin-spaces-toolbar__field">
          <span>请求 ID</span>
          <input value={requestIDInput} onChange={(event) => setRequestIDInput(event.target.value)} disabled={loading} />
        </label>

        <label className="admin-spaces-toolbar__field">
          <span>开始时间</span>
          <input
            type="datetime-local"
            value={fromInput}
            onChange={(event) => setFromInput(event.target.value)}
            disabled={loading}
          />
        </label>

        <label className="admin-spaces-toolbar__field">
          <span>结束时间</span>
          <input type="datetime-local" value={toInput} onChange={(event) => setToInput(event.target.value)} disabled={loading} />
        </label>

        <div className="admin-spaces-toolbar__actions">
          <button type="submit" disabled={loading}>
            <Search size={14} />
            <span>查询</span>
          </button>
          <button type="button" disabled={loading} onClick={handleReset}>
            重置
          </button>
          <button type="button" disabled={loading} onClick={() => void loadAudits()}>
            <RefreshCw size={14} />
            <span>刷新</span>
          </button>
        </div>
      </form>

      <div className="admin-spaces-table-wrap">
        <table className="admin-spaces-table admin-audits-table">
          <thead>
            <tr>
              <th>时间</th>
              <th>模块/动作</th>
              <th>目标</th>
              <th>操作者</th>
              <th>摘要</th>
              <th>请求 ID</th>
              <th>详情</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={7}>
                  <div className="admin-spaces-loading-row">
                    <LoaderCircle size={15} className="admin-spaces-loading-row__icon" />
                    <span>正在加载审计日志...</span>
                  </div>
                </td>
              </tr>
            ) : auditsState.items.length === 0 ? (
              <tr>
                <td colSpan={7}>
                  <div className="admin-spaces-empty-row">暂无审计日志</div>
                </td>
              </tr>
            ) : (
              auditsState.items.map((item) => {
                const isExpanded = expandedAuditID === item.id;
                return (
                  <tr key={item.id}>
                    <td>
                      <div className="admin-users-time-cell">
                        <span>{formatDateTime(item.createdAt)}</span>
                        <code className="admin-audits-id">#{item.id}</code>
                      </div>
                    </td>
                    <td>
                      <div className="admin-audits-module-cell">
                        <span className="admin-theme-kind admin-theme-kind--custom">{renderModuleLabel(item.module)}</span>
                        <span className="admin-spaces-visibility">{renderActionLabel(item.action)}</span>
                      </div>
                    </td>
                    <td>
                      <div className="admin-theme-cell">
                        <strong>{item.targetType || "-"}</strong>
                        <code>{item.targetId || "-"}</code>
                      </div>
                    </td>
                    <td>
                      <div className="admin-spaces-owner-cell">
                        <strong>{formatActorIdentity(item)}</strong>
                        <span>{item.actorUserId || "-"}</span>
                      </div>
                    </td>
                    <td>
                      <p className="admin-audits-summary">{item.summary || "-"}</p>
                    </td>
                    <td>
                      <code className="admin-audits-request-id">{item.requestId || "-"}</code>
                    </td>
                    <td>
                      <button
                        type="button"
                        className="admin-audits-detail-button"
                        onClick={() => setExpandedAuditID(isExpanded ? null : item.id)}
                      >
                        {isExpanded ? "收起" : "查看"}
                      </button>
                      {isExpanded ? (
                        <pre className="admin-system-config-value admin-audits-detail-json">
                          {JSON.stringify(item.detail ?? {}, null, 2)}
                        </pre>
                      ) : null}
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
          共 {auditsState.pagination.total} 条，当前第 {auditsState.pagination.page} / {totalPages} 页
        </p>
        <div className="admin-spaces-pagination">
          <button type="button" disabled={loading || page <= 1} onClick={() => setPage((previousPage) => Math.max(1, previousPage - 1))}>
            上一页
          </button>
          <button
            type="button"
            disabled={loading || page >= totalPages}
            onClick={() => setPage((previousPage) => Math.min(totalPages, previousPage + 1))}
          >
            下一页
          </button>
        </div>
      </footer>
    </section>
  );
}
