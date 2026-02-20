import {
  FileText,
  FolderKanban,
  History,
  LayoutDashboard,
  LoaderCircle,
  LogOut,
  type LucideIcon,
  Palette,
  Settings2,
  ShieldAlert,
  Users
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { type AdminRole, type AdminIdentity, type AuthSession, type DataGateway } from "../data-access";
import { formatError } from "../editor/status-utils";
import { AdminAuthPanel } from "../components/AdminAuthPanel";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { ADMIN_LOGIN_ROUTE_PATH, ADMIN_ROUTE_BASE_PATH } from "./routes";
import { AdminUsersPage } from "./pages/AdminUsersPage";
import { AdminSpacesPage } from "./pages/AdminSpacesPage";
import { AdminDocumentsPage } from "./pages/AdminDocumentsPage";
import { AdminThemesPage } from "./pages/AdminThemesPage";
import { AdminSystemConfigsPage } from "./pages/AdminSystemConfigsPage";
import { AdminAuditsPage } from "./pages/AdminAuditsPage";

type AdminMenuKey = "dashboard" | "users" | "spaces" | "documents" | "themes" | "system" | "audits";

interface AdminMenuItem {
  key: AdminMenuKey;
  label: string;
  description: string;
  path: string;
  icon: LucideIcon;
}

interface AdminAppProps {
  authSession: AuthSession;
  checking: boolean;
  submitting: boolean;
  errorMessage: string | null;
  dataGateway: DataGateway;
  onLogin: (input: { email: string; password: string }) => Promise<void>;
  onLogout: () => Promise<void>;
}

function buildAdminMenu(roles: AdminRole[]): AdminMenuItem[] {
  const hasPlatformAdminRole = roles.includes("platform_admin");
  const hasSpaceAdminRole = roles.includes("space_admin");
  const hasAnyAdminRole = hasPlatformAdminRole || hasSpaceAdminRole;
  if (!hasAnyAdminRole) {
    return [];
  }

  const items: AdminMenuItem[] = [
    {
      key: "dashboard",
      label: "概览",
      description: "查看后台整体运行与风险概况",
      path: ADMIN_ROUTE_BASE_PATH,
      icon: LayoutDashboard
    }
  ];

  if (hasPlatformAdminRole) {
    items.push({
      key: "users",
      label: "用户管理",
      description: "管理用户状态、封禁与删除",
      path: "/admin/users",
      icon: Users
    });
  }

  items.push(
    {
      key: "spaces",
      label: "空间管理",
      description: "查看空间状态并执行封禁/删除",
      path: "/admin/spaces",
      icon: FolderKanban
    },
    {
      key: "documents",
      label: "文档管理",
      description: "筛选文档并处理违规内容",
      path: "/admin/documents",
      icon: FileText
    },
    {
      key: "themes",
      label: "主题管理",
      description: "维护全站主题模板与 CSS 变量",
      path: "/admin/themes",
      icon: Palette
    }
  );

  if (hasPlatformAdminRole) {
    items.push({
      key: "system",
      label: "系统配置",
      description: "维护系统级配置参数",
      path: "/admin/system-configs",
      icon: Settings2
    });
  }

  items.push({
    key: "audits",
    label: "审计日志",
    description: "检索关键管理操作轨迹",
    path: "/admin/audits",
    icon: History
  });

  return items;
}

function resolveActiveMenu(menuItems: AdminMenuItem[], pathname: string): AdminMenuItem | null {
  const normalizedPathname = pathname.endsWith("/") && pathname !== "/" ? pathname.slice(0, -1) : pathname;

  // 优先精确匹配，避免 `/admin` 抢占 `/admin/users` 等子路由。
  const exactMatchedItem = menuItems.find((item) => normalizedPathname === item.path);
  if (exactMatchedItem) {
    return exactMatchedItem;
  }

  // 再按路径深度匹配前缀，保证 `/admin/users/:id` 命中 `users` 菜单。
  const prefixCandidates = menuItems
    .filter((item) => normalizedPathname.startsWith(`${item.path}/`))
    .sort((left, right) => right.path.length - left.path.length);
  return prefixCandidates[0] ?? null;
}

function renderRoleLabel(role: AdminRole): string {
  switch (role) {
    case "platform_admin":
      return "全站管理员";
    case "space_admin":
      return "空间管理员";
    default:
      return role;
  }
}

function renderPlaceholderContent(activeMenuKey: AdminMenuKey): { title: string; description: string; todo: string[] } {
  switch (activeMenuKey) {
    case "dashboard":
      return {
        title: "后台概览",
        description: "展示关键指标、风险概况与待处理事项。",
        todo: ["接入统计接口", "展示异常告警", "汇总今日高风险操作"]
      };
    case "users":
      return {
        title: "用户管理",
        description: "用户列表、搜索、封禁/解封、软删除。",
        todo: ["实现列表与筛选", "支持批量封禁", "写入审计日志"]
      };
    case "spaces":
      return {
        title: "空间管理",
        description: "空间状态治理与权限范围内操作。",
        todo: ["实现空间列表", "支持状态变更", "支持删除确认流程"]
      };
    case "documents":
      return {
        title: "文档管理",
        description: "文档检索、状态变更和内容治理。",
        todo: ["实现多条件搜索", "支持封禁/删除", "支持按空间过滤"]
      };
    case "themes":
      return {
        title: "主题管理",
        description: "主题模板维护与生效范围管理。",
        todo: ["实现主题 CRUD", "区分内置与自定义主题", "支持变更审计"]
      };
    case "system":
      return {
        title: "系统配置",
        description: "系统级参数配置与校验。",
        todo: ["配置项列表", "JSON schema 校验", "配置变更审计"]
      };
    case "audits":
      return {
        title: "审计日志",
        description: "检索操作轨迹并支持筛选导出。",
        todo: ["按操作者/目标筛选", "按时间区间查询", "导出审计报表"]
      };
    default:
      return {
        title: "待开发",
        description: "该模块将在下一阶段实现。",
        todo: ["补齐模块接口", "完成页面联调", "补充测试用例"]
      };
  }
}

export function AdminApp({
  authSession,
  checking,
  submitting,
  errorMessage,
  dataGateway,
  onLogin,
  onLogout
}: AdminAppProps) {
  const location = useLocation();
  const navigate = useNavigate();
  const activeUser = authSession.user;

  const [adminIdentity, setAdminIdentity] = useState<AdminIdentity | null>(null);
  const [isAdminIdentityLoading, setIsAdminIdentityLoading] = useState(false);
  const [adminIdentityError, setAdminIdentityError] = useState<string | null>(null);

  useEffect(() => {
    if (checking || !activeUser) {
      setAdminIdentity(null);
      setAdminIdentityError(null);
      return;
    }

    let cancelled = false;
    const loadAdminIdentity = async () => {
      setIsAdminIdentityLoading(true);
      setAdminIdentityError(null);
      try {
        const identity = await dataGateway.admin.getMe();
        if (cancelled) {
          return;
        }
        setAdminIdentity(identity);
      } catch (error) {
        if (cancelled) {
          return;
        }
        setAdminIdentity(null);
        setAdminIdentityError(`管理员权限校验失败：${formatError(error)}`);
      } finally {
        if (!cancelled) {
          setIsAdminIdentityLoading(false);
        }
      }
    };

    void loadAdminIdentity();
    return () => {
      cancelled = true;
    };
  }, [activeUser, checking, dataGateway]);

  const adminMenuItems = useMemo(
    () => buildAdminMenu(adminIdentity?.roles ?? []),
    [adminIdentity?.roles]
  );
  const activeMenuItem = useMemo(
    () => resolveActiveMenu(adminMenuItems, location.pathname),
    [adminMenuItems, location.pathname]
  );

  useEffect(() => {
    if (!adminIdentity || !adminMenuItems.length) {
      return;
    }
    if (location.pathname === ADMIN_LOGIN_ROUTE_PATH) {
      navigate(ADMIN_ROUTE_BASE_PATH, { replace: true });
      return;
    }
    if (!activeMenuItem) {
      navigate(adminMenuItems[0].path, { replace: true });
    }
  }, [activeMenuItem, adminIdentity, adminMenuItems, location.pathname, navigate]);

  const handleNavigateMenu = useCallback(
    (path: string) => {
      if (location.pathname !== path) {
        navigate(path);
      }
    },
    [location.pathname, navigate]
  );

  if (checking || isAdminIdentityLoading) {
    return (
      <div className="admin-loading-page bg-[radial-gradient(circle_at_top_left,#dbeafe_0%,transparent_42%),radial-gradient(circle_at_top_right,#cffafe_0%,transparent_38%),linear-gradient(180deg,#f8fafc_0%,#eef2ff_100%)]">
        <Card className="admin-loading-card border-slate-200 shadow-xl">
          <CardContent className="flex min-h-[88px] items-center justify-center gap-2 p-6 text-slate-600">
            <LoaderCircle className="admin-loading-card__icon" size={18} />
            <span className="text-sm font-medium">正在加载管理后台...</span>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (!activeUser) {
    return (
      <AdminAuthPanel
        checking={checking}
        submitting={submitting}
        errorMessage={errorMessage}
        onLogin={onLogin}
      />
    );
  }

  if (adminIdentityError || !adminIdentity || adminMenuItems.length === 0) {
    return (
      <div className="admin-forbidden-page">
        <Card className="admin-forbidden-card border-rose-100 shadow-xl">
          <CardHeader className="pb-2">
            <div className="admin-forbidden-card__icon">
              <ShieldAlert size={18} />
            </div>
            <CardTitle className="mt-3 text-2xl">无管理后台权限</CardTitle>
            <CardDescription className="mt-2 text-sm text-slate-600">
              {adminIdentityError ?? "当前账号未配置管理员角色，请联系平台管理员授权。"}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="admin-forbidden-card__actions">
              <Button type="button" variant="outline" onClick={() => navigate("/editor", { replace: true })}>
                返回编辑器
              </Button>
              <Button type="button" variant="destructive" onClick={() => void onLogout()}>
                切换账号
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  const activeContent = renderPlaceholderContent(activeMenuItem?.key ?? adminMenuItems[0].key);

  return (
    <div className="admin-shell bg-[radial-gradient(circle_at_top_left,#e0f2fe_0%,transparent_35%),radial-gradient(circle_at_95%_5%,#dcfce7_0%,transparent_28%),linear-gradient(180deg,#f8fafc_0%,#f1f5f9_100%)]">
      <aside className="admin-shell__sidebar border-r border-slate-200/80 bg-white/80 backdrop-blur">
        <Card className="border-slate-200 bg-white/80 shadow-sm">
          <CardHeader className="pb-4">
            <p className="admin-brand text-xs font-semibold uppercase tracking-[0.18em] text-slate-500">PlainDoc</p>
            <CardTitle className="text-2xl tracking-tight">管理后台</CardTitle>
            <CardDescription className="text-xs text-slate-500">Admin Console • shadcn/ui</CardDescription>
          </CardHeader>
        </Card>
        <nav className="admin-menu" aria-label="后台菜单">
          {adminMenuItems.map((item) => {
            const ItemIcon = item.icon;
            const isActive = item.key === (activeMenuItem?.key ?? adminMenuItems[0].key);
            return (
              <Button
                key={item.key}
                type="button"
                variant={isActive ? "secondary" : "ghost"}
                className="admin-menu__item h-auto min-h-14 w-full justify-start border border-slate-200 px-3 py-2 text-left hover:border-slate-300 hover:bg-slate-50"
                onClick={() => handleNavigateMenu(item.path)}
              >
                <span className="admin-menu__item-icon">
                  <ItemIcon size={16} />
                </span>
                <span className="admin-menu__item-labels">
                  <span>{item.label}</span>
                  <small>{item.description}</small>
                </span>
              </Button>
            );
          })}
        </nav>
      </aside>
      <section className="admin-shell__main">
        <header className="admin-header border-b border-slate-200/80 bg-white/70 backdrop-blur">
          <div className="admin-header__title">
            <h2>{activeContent.title}</h2>
            <p>{activeContent.description}</p>
          </div>
          <div className="admin-header__actions">
            <div className="admin-role-badges">
              {adminIdentity.roles.map((role) => (
                <Badge key={role} variant="outline" className="admin-role-badge border-cyan-200 bg-cyan-50 text-cyan-700">
                  {renderRoleLabel(role)}
                </Badge>
              ))}
            </div>
            <Button type="button" variant="outline" className="admin-logout-button border-rose-200 text-rose-700 hover:bg-rose-50" onClick={() => void onLogout()}>
              <LogOut size={14} />
              <span>退出</span>
            </Button>
          </div>
        </header>
        <main className="admin-content">
          {activeMenuItem?.key === "users" ? (
            <AdminUsersPage currentUserID={adminIdentity.userId} dataGateway={dataGateway} />
          ) : activeMenuItem?.key === "spaces" ? (
            <AdminSpacesPage dataGateway={dataGateway} />
          ) : activeMenuItem?.key === "documents" ? (
            <AdminDocumentsPage dataGateway={dataGateway} />
          ) : activeMenuItem?.key === "themes" ? (
            <AdminThemesPage dataGateway={dataGateway} />
          ) : activeMenuItem?.key === "system" ? (
            <AdminSystemConfigsPage dataGateway={dataGateway} />
          ) : activeMenuItem?.key === "audits" ? (
            <AdminAuditsPage dataGateway={dataGateway} />
          ) : (
            <Card className="admin-placeholder-card border-slate-200 shadow-sm">
              <CardHeader className="pb-3">
                <CardTitle>{activeContent.title}</CardTitle>
                <CardDescription>{activeContent.description}</CardDescription>
              </CardHeader>
              <CardContent>
                <ul>
                  {activeContent.todo.map((todo) => (
                    <li key={todo}>{todo}</li>
                  ))}
                </ul>
              </CardContent>
            </Card>
          )}
        </main>
      </section>
    </div>
  );
}
