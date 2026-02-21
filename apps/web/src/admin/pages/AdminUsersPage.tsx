import { LoaderCircle, PencilLine, RefreshCw, Search, ShieldBan, ShieldCheck, Trash2, UserPlus } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEventHandler } from "react";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { showToast } from "../../components/ui/toast";
import { type AdminUser, type AdminUserListResult, type DataGateway } from "../../data-access";
import { useAdminDialogs } from "../components/AdminDialogs";
import { AdminPageCard, AdminPaginationFooter, AdminTableContainer, AdminToolbarActions } from "../components/AdminPageLayout";
import { formatError } from "../../editor/status-utils";

const DEFAULT_PAGE_SIZE = 20;
const USER_ROLE_OPTIONS = [
  { value: "user", label: "普通用户" },
  { value: "space_admin", label: "空间管理员" },
  { value: "platform_admin", label: "全站管理员" }
] as const;

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

function renderStatusBadgeClass(status: AdminUser["status"]): string {
  switch (status) {
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

function normalizeUserRole(value: string | null | undefined): "user" | "space_admin" | "platform_admin" {
  const normalized = (value ?? "").trim();
  if (normalized === "platform_admin") {
    return "platform_admin";
  }
  if (normalized === "space_admin") {
    return "space_admin";
  }
  return "user";
}

function renderRoleLabel(role: AdminUser["role"]): string {
  const normalizedRole = normalizeUserRole(role);
  if (normalizedRole === "platform_admin") {
    return "全站管理员";
  }
  if (normalizedRole === "space_admin") {
    return "空间管理员";
  }
  return "普通用户";
}

function renderRoleBadgeClass(role: AdminUser["role"]): string {
  const normalizedRole = normalizeUserRole(role);
  if (normalizedRole === "platform_admin") {
    return "border-violet-200 bg-violet-50 text-violet-700";
  }
  if (normalizedRole === "space_admin") {
    return "border-cyan-200 bg-cyan-50 text-cyan-700";
  }
  return "border-slate-200 bg-slate-100 text-slate-600";
}

export function AdminUsersPage({ currentUserID, dataGateway }: AdminUsersPageProps) {
  const { confirm, prompt, dialogs } = useAdminDialogs();

  const [keywordInput, setKeywordInput] = useState("");
  const [keyword, setKeyword] = useState("");
  const [statusFilter, setStatusFilter] = useState<"" | "all" | "active" | "banned" | "deleted">("");
  const [page, setPage] = useState(1);

  const [usersState, setUsersState] = useState<AdminUsersState>(() => emptyUsersState());
  const [loading, setLoading] = useState(false);
  const [creatingUser, setCreatingUser] = useState(false);
  const [actioningUserID, setActioningUserID] = useState<string | null>(null);

  const openToast = useCallback((message: string, variant: "success" | "info" | "error" = "error") => {
    showToast(message, variant);
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

  const handleSearchSubmit = useCallback<FormEventHandler<HTMLFormElement>>(
    (event) => {
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

  const handleCreateUser = useCallback(async () => {
    const promptResult = await prompt({
      title: "新增用户",
      description: "创建一个新用户账号，创建后可直接使用邮箱和密码登录。",
      confirmText: "确认创建",
      fields: [
        {
          key: "name",
          label: "昵称",
          required: true,
          placeholder: "输入用户昵称"
        },
        {
          key: "email",
          label: "邮箱",
          required: true,
          placeholder: "name@example.com"
        },
        {
          key: "password",
          label: "密码",
          type: "password",
          required: true,
          placeholder: "至少 6 位"
        },
        {
          key: "confirmPassword",
          label: "确认密码",
          type: "password",
          required: true,
          placeholder: "再次输入密码"
        },
        {
          key: "role",
          label: "角色",
          type: "select",
          required: true,
          defaultValue: "user",
          options: USER_ROLE_OPTIONS.map((option) => ({
            value: option.value,
            label: option.label
          }))
        }
      ]
    });
    if (!promptResult) {
      return;
    }

    const name = (promptResult.name ?? "").trim();
    const email = (promptResult.email ?? "").trim();
    const password = promptResult.password ?? "";
    const confirmPassword = promptResult.confirmPassword ?? "";
    const role = normalizeUserRole(promptResult.role);

    if (!name) {
      openToast("昵称不能为空");
      return;
    }
    if (!email) {
      openToast("邮箱不能为空");
      return;
    }
    if (password.length < 6) {
      openToast("密码长度至少为 6 位");
      return;
    }
    if (password !== confirmPassword) {
      openToast("两次输入的密码不一致");
      return;
    }

    setCreatingUser(true);
    try {
      const createdUser = await dataGateway.admin.createUser({
        email,
        name,
        password,
        role
      });
      openToast(`用户已创建：${createdUser.email}（${renderRoleLabel(createdUser.role)}）`, "success");
      const shouldReloadImmediately = page === 1;
      setPage(1);
      if (shouldReloadImmediately) {
        await loadUsers();
      }
    } catch (error) {
      openToast(`创建用户失败：${formatError(error)}`);
    } finally {
      setCreatingUser(false);
    }
  }, [dataGateway.admin, loadUsers, openToast, page, prompt]);

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

  const handleUpdateRole = useCallback(
    async (user: AdminUser) => {
      if (!user.canEditRole) {
        openToast("不能编辑比自己角色高的用户，或不能编辑当前登录账号");
        return;
      }

      const promptResult = await prompt({
        title: `编辑角色：${user.email}`,
        description: "修改用户的后台管理角色。",
        confirmText: "保存角色",
        fields: [
          {
            key: "role",
            label: "用户角色",
            type: "select",
            required: true,
            defaultValue: normalizeUserRole(user.role),
            options: USER_ROLE_OPTIONS.map((option) => ({
              value: option.value,
              label: option.label
            }))
          }
        ]
      });
      if (!promptResult) {
        return;
      }

      const nextRole = normalizeUserRole(promptResult.role);
      if (nextRole === normalizeUserRole(user.role)) {
        openToast("角色未变更", "info");
        return;
      }

      await runUserAction(user.userId, async () => {
        await dataGateway.admin.updateUserRole({
          userId: user.userId,
          role: nextRole
        });
      });
    },
    [dataGateway.admin, openToast, prompt, runUserAction]
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
    <section aria-label="用户管理">
      {dialogs}
      <AdminPageCard>
          <form className="grid gap-3 md:grid-cols-[minmax(0,1fr)_180px_auto]" onSubmit={handleSearchSubmit}>
            <label className="space-y-1.5">
              <span className="text-xs font-semibold tracking-wide text-slate-600">关键词</span>
              <Input
                type="search"
                value={keywordInput}
                placeholder="邮箱 / 用户 ID / 名称"
                onChange={(event) => setKeywordInput(event.target.value)}
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
            <AdminToolbarActions className="self-stretch">
                <Button type="submit" disabled={loading || creatingUser}>
                  <Search size={14} />
                  <span>查询</span>
                </Button>
                <Button type="button" variant="outline" disabled={loading || creatingUser} onClick={handleReset}>
                  重置
                </Button>
                <Button type="button" variant="outline" disabled={loading || creatingUser} onClick={() => void loadUsers()}>
                  <RefreshCw size={14} />
                  <span>刷新</span>
                </Button>
                <Button type="button" disabled={loading || creatingUser} onClick={() => void handleCreateUser()}>
                  <UserPlus size={14} />
                  <span>{creatingUser ? "创建中..." : "新增用户"}</span>
                </Button>
            </AdminToolbarActions>
          </form>

          <AdminTableContainer>
              <table className="w-full min-w-[860px] border-collapse text-left text-sm">
                <thead className="sticky top-0 z-10 bg-slate-50/95 backdrop-blur">
                  <tr className="text-xs uppercase tracking-wide text-slate-600">
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">用户</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">角色</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">状态</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">封禁原因</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">更新时间</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">操作</th>
                  </tr>
                </thead>
                <tbody>
                  {loading ? (
                    <tr>
                      <td colSpan={6} className="px-3 py-12">
                        <div className="flex items-center justify-center gap-2 text-sm text-slate-500">
                          <LoaderCircle size={15} className="animate-spin" />
                          <span>正在加载用户列表...</span>
                        </div>
                      </td>
                    </tr>
                  ) : usersState.items.length === 0 ? (
                    <tr>
                      <td colSpan={6} className="px-3 py-12 text-center text-sm text-slate-500">
                        暂无符合条件的数据
                      </td>
                    </tr>
                  ) : (
                    usersState.items.map((user) => {
                      const isActioning = actioningUserID === user.userId;
                      return (
                        <tr key={user.userId} className="border-b border-slate-100 align-top text-slate-700">
                          <td className="px-3 py-3">
                            <div className="grid gap-1">
                              <strong className="text-sm font-semibold text-slate-900">{user.name || "未命名用户"}</strong>
                              <span className="text-xs text-slate-600">{user.email}</span>
                              <code className="w-fit rounded border border-sky-200 bg-sky-50 px-1.5 py-0.5 text-xs text-sky-700">
                                {user.userId}
                              </code>
                            </div>
                          </td>
                          <td className="px-3 py-3">
                            <Badge variant="outline" className={renderRoleBadgeClass(user.role)}>
                              {renderRoleLabel(user.role)}
                            </Badge>
                          </td>
                          <td className="px-3 py-3">
                            <Badge variant="outline" className={renderStatusBadgeClass(user.status)}>
                              {renderStatusLabel(user.status)}
                            </Badge>
                          </td>
                          <td className="px-3 py-3 text-xs text-slate-600">{user.bannedReason || "-"}</td>
                          <td className="px-3 py-3">
                            <div className="grid gap-1 text-xs text-slate-500">
                              <span>更新: {formatDateTime(user.updatedAt)}</span>
                              <span>删除: {formatDateTime(user.deletedAt)}</span>
                            </div>
                          </td>
                          <td className="px-3 py-3">
                            <div className="flex flex-wrap items-center gap-2">
                              <Button
                                type="button"
                                size="sm"
                                variant="outline"
                                disabled={isActioning || !user.canEditRole}
                                onClick={() => void handleUpdateRole(user)}
                              >
                                <PencilLine size={14} />
                                <span>角色</span>
                              </Button>
                              {user.status === "banned" ? (
                                <Button type="button" size="sm" variant="secondary" disabled={isActioning} onClick={() => void handleUnban(user)}>
                                  <ShieldCheck size={14} />
                                  <span>解封</span>
                                </Button>
                              ) : user.status === "active" ? (
                                <Button
                                  type="button"
                                  size="sm"
                                  variant="outline"
                                  className="border-amber-200 bg-amber-50 text-amber-700 hover:bg-amber-100"
                                  disabled={isActioning}
                                  onClick={() => void handleBan(user)}
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
                                disabled={isActioning || user.status === "deleted"}
                                onClick={() => void handleDelete(user)}
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
              当前第 {usersState.pagination.page} / {totalPages} 页，共 {usersState.pagination.total} 条
              </>
            }
            previousDisabled={loading || usersState.pagination.page <= 1}
            nextDisabled={loading || usersState.pagination.page >= totalPages}
            onPrevious={() => setPage((value) => Math.max(1, value - 1))}
            onNext={() => setPage((value) => Math.min(totalPages, value + 1))}
          />
      </AdminPageCard>
    </section>
  );
}
