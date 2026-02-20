import { LoaderCircle, RefreshCw, Search, ShieldBan, ShieldCheck, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { type AdminUser, type AdminUserListResult, type DataGateway } from "../../data-access";
import { useAdminDialogs } from "../components/AdminDialogs";
import { TopToast, type TopToastVariant } from "../../components/TopToast";
import { formatError } from "../../editor/status-utils";

const DEFAULT_PAGE_SIZE = 20;

interface AdminUsersPageProps {
  currentUserID: string;
  dataGateway: DataGateway;
}

interface AdminUsersState {
  items: AdminUser[];
  pagination: AdminUserListResult["pagination"];
}

function emptyUsersState(): AdminUsersState {
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

function renderStatusLabel(status: AdminUser["status"]): string {
  switch (status) {
    case "active":
      return "正常";
    case "banned":
      return "已封禁";
    case "deleted":
      return "已删除";
    default:
      return status;
  }
}

export function AdminUsersPage({ currentUserID, dataGateway }: AdminUsersPageProps) {
  const { confirm, prompt, dialogs } = useAdminDialogs();

  const [keywordInput, setKeywordInput] = useState("");
  const [keyword, setKeyword] = useState("");
  const [statusFilter, setStatusFilter] = useState<"" | "all" | "active" | "banned" | "deleted">("");
  const [page, setPage] = useState(1);

  const [usersState, setUsersState] = useState<AdminUsersState>(() => emptyUsersState());
  const [loading, setLoading] = useState(false);
  const [actioningUserID, setActioningUserID] = useState<string | null>(null);
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

  const loadUsers = useCallback(async () => {
    setLoading(true);
    try {
      const payload = await dataGateway.admin.listUsers({
        keyword,
        status: statusFilter || undefined,
        page,
        pageSize: DEFAULT_PAGE_SIZE
      });
      setUsersState(payload);
    } catch (error) {
      openToast(`加载用户列表失败：${formatError(error)}`);
      setUsersState(emptyUsersState());
    } finally {
      setLoading(false);
    }
  }, [dataGateway, keyword, openToast, page, statusFilter]);

  useEffect(() => {
    void loadUsers();
  }, [loadUsers]);

  const totalPages = useMemo(() => {
    const total = usersState.pagination.total;
    const pageSize = usersState.pagination.pageSize || DEFAULT_PAGE_SIZE;
    return Math.max(1, Math.ceil(total / pageSize));
  }, [usersState.pagination.pageSize, usersState.pagination.total]);

  const handleSearchSubmit = useCallback(
    (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      setPage(1);
      setKeyword(keywordInput.trim());
    },
    [keywordInput]
  );

  const handleReset = useCallback(() => {
    setKeywordInput("");
    setKeyword("");
    setStatusFilter("");
    setPage(1);
  }, []);

  const runUserAction = useCallback(
    async (targetUserID: string, callback: () => Promise<void>) => {
      setActioningUserID(targetUserID);
      try {
        await callback();
        await loadUsers();
      } catch (error) {
        openToast(`执行操作失败：${formatError(error)}`);
      } finally {
        setActioningUserID(null);
      }
    },
    [loadUsers, openToast]
  );

  const handleBan = useCallback(
    async (user: AdminUser) => {
      if (user.userId === currentUserID) {
        openToast("不允许封禁当前登录管理员账号");
        return;
      }

      const promptResult = await prompt({
        title: `封禁用户：${user.email}`,
        description: "请输入封禁原因，提交后该用户将无法正常访问系统。",
        confirmText: "确认封禁",
        tone: "danger",
        fields: [
          {
            key: "reason",
            label: "封禁原因",
            required: true,
            defaultValue: user.bannedReason || "",
            placeholder: "例如：多次触发风控策略"
          }
        ]
      });
      if (!promptResult) {
        return;
      }
      const normalizedReason = (promptResult.reason ?? "").trim();
      if (!normalizedReason) {
        openToast("封禁原因不能为空");
        return;
      }

      await runUserAction(user.userId, async () => {
        await dataGateway.admin.updateUserStatus({
          userId: user.userId,
          status: "banned",
          reason: normalizedReason
        });
      });
    },
    [currentUserID, dataGateway.admin, openToast, prompt, runUserAction]
  );

  const handleUnban = useCallback(
    async (user: AdminUser) => {
      const confirmed = await confirm({
        title: `解封用户：${user.email}`,
        description: "确认后该用户状态将恢复为正常。",
        confirmText: "确认解封"
      });
      if (!confirmed) {
        return;
      }
      await runUserAction(user.userId, async () => {
        await dataGateway.admin.updateUserStatus({
          userId: user.userId,
          status: "active"
        });
      });
    },
    [confirm, dataGateway.admin, runUserAction]
  );

  const handleDelete = useCallback(
    async (user: AdminUser) => {
      if (user.userId === currentUserID) {
        openToast("不允许删除当前登录管理员账号");
        return;
      }
      const confirmed = await confirm({
        title: `删除用户：${user.email}`,
        description: "该操作为软删除，用户将不可继续登录。",
        confirmText: "确认删除",
        tone: "danger"
      });
      if (!confirmed) {
        return;
      }

      await runUserAction(user.userId, async () => {
        await dataGateway.admin.deleteUser(user.userId);
      });
    },
    [confirm, currentUserID, dataGateway.admin, openToast, runUserAction]
  );

  return (
    <section className="admin-users-panel" aria-label="用户管理">
      <TopToast
        open={toastState.open}
        message={toastState.message}
        variant={toastState.variant}
        triggerKey={toastState.triggerKey}
        durationMs={2800}
        onClose={closeToast}
      />
      {dialogs}
      <form className="admin-users-toolbar" onSubmit={handleSearchSubmit}>
        <label className="admin-users-toolbar__field admin-users-toolbar__field--keyword">
          <span>关键词</span>
          <input
            type="search"
            value={keywordInput}
            placeholder="邮箱 / 用户 ID / 名称"
            onChange={(event) => setKeywordInput(event.target.value)}
          />
        </label>
        <label className="admin-users-toolbar__field admin-users-toolbar__field--status">
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
        <div className="admin-users-toolbar__actions">
          <button type="submit" disabled={loading}>
            <Search size={14} />
            <span>查询</span>
          </button>
          <button type="button" disabled={loading} onClick={handleReset}>
            重置
          </button>
          <button type="button" disabled={loading} onClick={() => void loadUsers()}>
            <RefreshCw size={14} />
            <span>刷新</span>
          </button>
        </div>
      </form>

      <div className="admin-users-table-wrap">
        <table className="admin-users-table">
          <thead>
            <tr>
              <th>用户</th>
              <th>状态</th>
              <th>封禁原因</th>
              <th>更新时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={5}>
                  <div className="admin-users-loading-row">
                    <LoaderCircle size={15} className="admin-users-loading-row__icon" />
                    <span>正在加载用户列表...</span>
                  </div>
                </td>
              </tr>
            ) : usersState.items.length === 0 ? (
              <tr>
                <td colSpan={5}>
                  <div className="admin-users-empty-row">暂无符合条件的数据</div>
                </td>
              </tr>
            ) : (
              usersState.items.map((user) => {
                const isActioning = actioningUserID === user.userId;
                const statusClassName = `admin-users-status admin-users-status--${user.status}`;
                return (
                  <tr key={user.userId}>
                    <td>
                      <div className="admin-users-user-cell">
                        <strong>{user.name || "未命名用户"}</strong>
                        <span>{user.email}</span>
                        <code>{user.userId}</code>
                      </div>
                    </td>
                    <td>
                      <span className={statusClassName}>{renderStatusLabel(user.status)}</span>
                    </td>
                    <td>
                      <p className="admin-users-ban-reason">{user.bannedReason || "-"}</p>
                    </td>
                    <td>
                      <div className="admin-users-time-cell">
                        <span>更新: {formatDateTime(user.updatedAt)}</span>
                        <span>删除: {formatDateTime(user.deletedAt)}</span>
                      </div>
                    </td>
                    <td>
                      <div className="admin-users-actions">
                        {user.status === "banned" ? (
                          <button type="button" disabled={isActioning} onClick={() => void handleUnban(user)}>
                            <ShieldCheck size={14} />
                            <span>解封</span>
                          </button>
                        ) : user.status === "active" ? (
                          <button type="button" disabled={isActioning} onClick={() => void handleBan(user)}>
                            <ShieldBan size={14} />
                            <span>封禁</span>
                          </button>
                        ) : (
                          <span className="admin-users-actions__disabled">-</span>
                        )}
                        <button
                          type="button"
                          className="danger"
                          disabled={isActioning || user.status === "deleted"}
                          onClick={() => void handleDelete(user)}
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

      <footer className="admin-users-footer">
        <p>
          当前第 {usersState.pagination.page} / {totalPages} 页，共 {usersState.pagination.total} 条
        </p>
        <div className="admin-users-pagination">
          <button
            type="button"
            onClick={() => setPage((value) => Math.max(1, value - 1))}
            disabled={loading || usersState.pagination.page <= 1}
          >
            上一页
          </button>
          <button
            type="button"
            onClick={() => setPage((value) => Math.min(totalPages, value + 1))}
            disabled={loading || usersState.pagination.page >= totalPages}
          >
            下一页
          </button>
        </div>
      </footer>
    </section>
  );
}
