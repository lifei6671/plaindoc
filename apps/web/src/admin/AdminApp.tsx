import {
  ChevronRight,
  FileText,
  FolderKanban,
  History,
  LayoutDashboard,
  LoaderCircle,
  LogOut,
  PanelLeftClose,
  PanelLeftOpen,
  type LucideIcon,
  Palette,
  Settings2,
  ShieldAlert,
  User,
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../components/ui/select";
import { ADMIN_BRAND_LOGO_SRC } from "./brand";
import { ADMIN_LOGIN_ROUTE_PATH, ADMIN_ROUTE_BASE_PATH } from "./routes";
import { AdminUsersPage } from "./pages/AdminUsersPage";
import { AdminSpacesPage } from "./pages/AdminSpacesPage";
import { AdminDocumentsPage } from "./pages/AdminDocumentsPage";
import { AdminThemesPage } from "./pages/AdminThemesPage";
import { AdminSystemConfigsPage } from "./pages/AdminSystemConfigsPage";
import { AdminAuditsPage } from "./pages/AdminAuditsPage";
import { AdminProfilePage } from "./pages/AdminProfilePage";

type AdminMenuKey = "dashboard" | "profile" | "users" | "spaces" | "documents" | "themes" | "system" | "audits";

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
    },
    {
      key: "profile",
      label: "个人信息",
      description: "维护昵称、头像与密码",
      path: "/admin/profile",
      icon: User
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
    }
  );

  if (hasPlatformAdminRole) {
    items.push({
      key: "themes",
      label: "主题管理",
      description: "维护全站主题模板与 CSS 变量",
      path: "/admin/themes",
      icon: Palette
    });
  }

  if (hasPlatformAdminRole) {
    items.push({
      key: "system",
      label: "系统配置",
      description: "维护系统级配置参数",
      path: "/admin/system-configs",
      icon: Settings2
    });
  }

  if (hasPlatformAdminRole) {
    items.push({
      key: "audits",
      label: "审计日志",
      description: "检索关键管理操作轨迹",
      path: "/admin/audits",
      icon: History
    });
  }

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
    case "profile":
      return {
        title: "个人信息",
        description: "维护昵称、头像与密码。",
        todo: ["昵称修改", "头像上传", "密码更新"]
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

interface DashboardMetric {
  label: string;
  value: string;
  delta: string;
  detail: string;
  tone: "positive" | "negative" | "neutral";
}

interface DashboardTimelinePoint {
  label: string;
  primary: number;
  secondary: number;
}

interface DashboardTodo {
  item: string;
  module: string;
  owner: string;
  status: "processing" | "done";
}

interface AdminMenuGroup {
  label: string;
  keys: AdminMenuKey[];
}

const DASHBOARD_METRICS: readonly DashboardMetric[] = [
  {
    label: "平台用户",
    value: "1,264",
    delta: "+12%",
    detail: "较上周新增 136 人",
    tone: "positive"
  },
  {
    label: "空间数量",
    value: "328",
    delta: "+6%",
    detail: "新增 21 个协作空间",
    tone: "positive"
  },
  {
    label: "文档总量",
    value: "9,842",
    delta: "+18%",
    detail: "近 7 天新增 1,532 篇",
    tone: "positive"
  },
  {
    label: "高风险审计",
    value: "17",
    delta: "-9%",
    detail: "已较昨日下降 2 条",
    tone: "negative"
  }
];

const DASHBOARD_TREND: readonly DashboardTimelinePoint[] = [
  { label: "Mon", primary: 42, secondary: 35 },
  { label: "Tue", primary: 54, secondary: 41 },
  { label: "Wed", primary: 38, secondary: 33 },
  { label: "Thu", primary: 67, secondary: 49 },
  { label: "Fri", primary: 58, secondary: 45 },
  { label: "Sat", primary: 48, secondary: 37 },
  { label: "Sun", primary: 63, secondary: 47 }
];

const DASHBOARD_TODO: readonly DashboardTodo[] = [
  { item: "完成用户封禁策略校验", module: "用户管理", owner: "平台管理员", status: "processing" },
  { item: "处理违规空间申诉", module: "空间管理", owner: "空间管理员", status: "processing" },
  { item: "同步主题变量到前端", module: "主题管理", owner: "设计系统", status: "done" },
  { item: "更新 access token 过期策略", module: "系统配置", owner: "安全负责人", status: "done" }
];

const ADMIN_MENU_GROUPS: readonly AdminMenuGroup[] = [
  { label: "总览", keys: ["dashboard"] },
  { label: "账号", keys: ["profile"] },
  { label: "内容管理", keys: ["users", "spaces", "documents"] },
  { label: "系统治理", keys: ["themes", "system", "audits"] }
];
const ADMIN_PAGE_BACKGROUND = "lab(98.26% 0 0)";
const ADMIN_TITLE_EXTRA_METADATA = "PlainDoc - 一个适合中小团队文档在线管理系统";
const ADMIN_DEFAULT_PAGE_TITLE = "管理后台";

function composeAdminBrowserTitle(pageTitle: string): string {
  const normalizedTitle = pageTitle.trim() || ADMIN_DEFAULT_PAGE_TITLE;
  return `${normalizedTitle} - ${ADMIN_TITLE_EXTRA_METADATA}`;
}

function buildMenuGroups(menuItems: AdminMenuItem[]): Array<{ label: string; items: AdminMenuItem[] }> {
  return ADMIN_MENU_GROUPS
    .map((group) => ({
      label: group.label,
      items: group.keys
        .map((key) => menuItems.find((item) => item.key === key))
        .filter((item): item is AdminMenuItem => item !== undefined)
    }))
    .filter((group) => group.items.length > 0);
}

function renderMetricToneClass(tone: DashboardMetric["tone"]): string {
  switch (tone) {
    case "positive":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "negative":
      return "border-amber-200 bg-amber-50 text-amber-700";
    default:
      return "border-slate-200 bg-slate-100 text-slate-600";
  }
}

function AdminDashboardPreview({
  menuItems,
  onNavigate
}: {
  menuItems: AdminMenuItem[];
  onNavigate: (path: string) => void;
}) {
  const quickEntries = menuItems.filter((item) => item.key !== "dashboard").slice(0, 6);

  return (
    <div className="space-y-5">
      <div className="grid gap-4 sm:grid-cols-2 2xl:grid-cols-4">
        {DASHBOARD_METRICS.map((metric) => (
          <Card key={metric.label} className="border-slate-200 bg-white shadow-sm">
            <CardHeader className="space-y-2 pb-2">
              <CardDescription className="text-xs font-medium uppercase tracking-[0.08em] text-slate-500">
                {metric.label}
              </CardDescription>
              <CardTitle className="text-3xl font-semibold tracking-tight">{metric.value}</CardTitle>
            </CardHeader>
            <CardContent className="flex items-center justify-between gap-2 pt-0">
              <p className="text-xs text-slate-500">{metric.detail}</p>
              <Badge variant="outline" className={renderMetricToneClass(metric.tone)}>
                {metric.delta}
              </Badge>
            </CardContent>
          </Card>
        ))}
      </div>

      <div className="grid gap-5 xl:grid-cols-[minmax(0,1.8fr)_minmax(0,1fr)]">
        <Card className="border-slate-200 bg-white shadow-sm">
          <CardHeader>
            <CardTitle className="text-xl">最近 7 天内容趋势</CardTitle>
            <CardDescription>主色柱为文档更新量，浅色柱为审计动作量</CardDescription>
          </CardHeader>
          <CardContent className="pt-0">
            <div className="rounded-xl border border-slate-200 bg-slate-50 p-4">
              <div className="flex h-44 items-end gap-2">
                {DASHBOARD_TREND.map((point) => (
                  <div key={point.label} className="flex flex-1 flex-col items-center gap-1">
                    <div className="relative w-full">
                      <div
                        className="absolute bottom-0 left-0 right-0 rounded-t bg-slate-300"
                        style={{ height: `${point.secondary}%` }}
                      />
                      <div
                        className="absolute bottom-0 left-0 right-0 rounded-t bg-slate-600/85"
                        style={{ height: `${point.primary}%` }}
                      />
                      <div className="h-36" />
                    </div>
                    <span className="text-[11px] text-slate-500">{point.label}</span>
                  </div>
                ))}
              </div>
            </div>
          </CardContent>
        </Card>

        <Card className="border-slate-200 bg-white shadow-sm">
          <CardHeader>
            <CardTitle className="text-xl">快捷入口</CardTitle>
            <CardDescription>快速进入常用管理模块</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2 pt-0">
            {quickEntries.map((entry) => {
              const EntryIcon = entry.icon;
              return (
                <Button
                  key={entry.key}
                  type="button"
                  variant="ghost"
                  className="h-auto w-full items-center justify-between rounded-sm border border-slate-200 bg-slate-50 px-3 py-2.5 text-left font-normal transition-colors hover:bg-white"
                  onClick={() => onNavigate(entry.path)}
                >
                  <span className="flex min-w-0 items-center gap-2">
                    <EntryIcon size={15} className="text-slate-600" />
                    <span className="truncate text-sm text-slate-800">{entry.label}</span>
                  </span>
                  <ChevronRight size={15} className="text-slate-400" />
                </Button>
              );
            })}
          </CardContent>
        </Card>
      </div>

      <Card className="border-slate-200 bg-white shadow-sm">
        <CardHeader className="flex-row items-center justify-between gap-3">
          <div>
            <CardTitle className="text-xl">待处理事项</CardTitle>
            <CardDescription>来自用户、空间、主题与配置模块的待办</CardDescription>
          </div>
          <div className="hidden sm:flex items-center gap-2">
            <Badge variant="secondary">处理中</Badge>
            <Badge variant="outline">已完成</Badge>
          </div>
        </CardHeader>
        <CardContent className="pt-0">
          <div className="overflow-hidden rounded-sm border border-slate-200">
            <table className="w-full border-collapse text-left text-sm">
              <thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-600">
                <tr>
                  <th className="border-b border-slate-200 px-3 py-2 font-semibold">事项</th>
                  <th className="border-b border-slate-200 px-3 py-2 font-semibold">模块</th>
                  <th className="border-b border-slate-200 px-3 py-2 font-semibold">负责人</th>
                  <th className="border-b border-slate-200 px-3 py-2 font-semibold">状态</th>
                </tr>
              </thead>
              <tbody>
                {DASHBOARD_TODO.map((todo) => (
                  <tr key={todo.item} className="border-b border-slate-100 text-slate-700">
                    <td className="px-3 py-2.5 text-sm font-medium text-slate-900">{todo.item}</td>
                    <td className="px-3 py-2.5 text-xs text-slate-600">{todo.module}</td>
                    <td className="px-3 py-2.5 text-xs text-slate-600">{todo.owner}</td>
                    <td className="px-3 py-2.5">
                      <Badge
                        variant="outline"
                        className={todo.status === "done"
                          ? "border-emerald-200 bg-emerald-50 text-emerald-700"
                          : "border-amber-200 bg-amber-50 text-amber-700"}
                      >
                        {todo.status === "done" ? "已完成" : "处理中"}
                      </Badge>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>
  );
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
  const [isSidebarCollapsed, setIsSidebarCollapsed] = useState(false);

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
  const currentAdminPageTitle = useMemo(() => {
    if (!activeUser) {
      return "管理后台登录";
    }
    if (checking || isAdminIdentityLoading) {
      return ADMIN_DEFAULT_PAGE_TITLE;
    }
    if (adminIdentityError || !adminIdentity || adminMenuItems.length === 0) {
      return "无管理后台权限";
    }
    const activeMenuKey = activeMenuItem?.key ?? adminMenuItems[0]?.key ?? "dashboard";
    return renderPlaceholderContent(activeMenuKey).title;
  }, [
    activeMenuItem,
    activeUser,
    adminIdentity,
    adminIdentityError,
    adminMenuItems,
    checking,
    isAdminIdentityLoading
  ]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    window.document.title = composeAdminBrowserTitle(currentAdminPageTitle);
  }, [currentAdminPageTitle]);

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
  const handleBackHome = useCallback(() => {
    // 返回首页需触发整页导航，交由后端 SSR 渲染首页，而不是停留在前端 SPA 路由。
    window.location.assign("/");
  }, []);
  const handleProfileUpdated = useCallback(
    (payload: { name: string; email: string; avatarUrl: string }) => {
      setAdminIdentity((previousState) => {
        if (!previousState) {
          return previousState;
        }
        return {
          ...previousState,
          name: payload.name,
          email: payload.email,
          avatarUrl: payload.avatarUrl
        };
      });
    },
    []
  );

  if (checking || isAdminIdentityLoading) {
    return (
      <div className="flex min-h-dvh items-center justify-center p-6" style={{ backgroundColor: ADMIN_PAGE_BACKGROUND }}>
        <Card className="w-full max-w-md border-slate-200 shadow-sm">
          <CardContent className="flex min-h-[88px] items-center justify-center gap-2 p-6 text-slate-600">
            <LoaderCircle className="animate-spin" size={18} />
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
      <div className="flex min-h-dvh items-center justify-center p-6" style={{ backgroundColor: ADMIN_PAGE_BACKGROUND }}>
        <Card className="w-full max-w-lg border-rose-200 shadow-sm">
          <CardHeader className="pb-2">
            <div className="inline-flex h-10 w-10 items-center justify-center rounded-md bg-rose-50 text-rose-600">
              <ShieldAlert size={18} />
            </div>
            <CardTitle className="mt-3 text-2xl">无管理后台权限</CardTitle>
            <CardDescription className="mt-2 text-sm text-slate-600">
              {adminIdentityError ?? "当前账号未配置管理员角色，请联系平台管理员授权。"}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap gap-2">
              <Button type="button" variant="outline" onClick={handleBackHome}>
                返回首页
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
  const currentMenuPath = activeMenuItem?.path ?? adminMenuItems[0].path;
  const groupedMenuItems = buildMenuGroups(adminMenuItems);

  return (
    <div className="h-dvh overflow-hidden text-slate-900" style={{ backgroundColor: ADMIN_PAGE_BACKGROUND }}>
      <div className="flex h-full">
        <aside
          className={`hidden shrink-0 flex-col bg-transparent transition-[width,padding,opacity] duration-200 lg:flex ${
            isSidebarCollapsed ? "w-0 overflow-hidden p-0 opacity-0 pointer-events-none" : "w-62 p-2"
          }`}
        >
          <div className="flex items-center gap-2 px-2 py-1">
            <span className="flex h-7 w-7 shrink-0 items-center justify-center overflow-hidden rounded-md border border-slate-200/80 bg-white shadow-sm">
              <img src={ADMIN_BRAND_LOGO_SRC} alt="PlainDoc logo" className="h-full w-full object-cover" />
            </span>
            <p className="text-sm font-semibold tracking-tight">PlainDoc 管理后台</p>
          </div>

          <div className="mt-6 flex-1 space-y-5 overflow-auto px-1">
            {groupedMenuItems.map((group) => (
              <section key={group.label}>
                <p className="px-2 text-[11px] font-semibold uppercase tracking-[0.1em] text-slate-500">
                  {group.label}
                </p>
                <nav className="mt-2 space-y-0.5" aria-label={`${group.label}菜单`}>
                  {group.items.map((item) => {
                    const ItemIcon = item.icon;
                    const isActive = item.key === (activeMenuItem?.key ?? adminMenuItems[0].key);
                    return (
                      <Button
                        key={item.key}
                        type="button"
                        variant="ghost"
                        className={
                          isActive
                            ? "admin-nav-item-button admin-nav-item-button--active h-auto w-full items-center justify-start gap-3 rounded-sm px-3 py-1.5 text-left text-sm font-medium text-slate-50"
                            : "admin-nav-item-button h-auto w-full items-center justify-start gap-3 rounded-sm px-3 py-1.5 text-left text-sm text-slate-700"
                        }
                        onClick={() => handleNavigateMenu(item.path)}
                      >
                        <ItemIcon size={15} />
                        <span className="truncate">{item.label}</span>
                      </Button>
                    );
                  })}
                </nav>
              </section>
            ))}
          </div>

          <div className="mt-4 border-t border-slate-200/80 px-2 pt-3">
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0 space-y-1">
                <p className="truncate text-sm font-semibold text-slate-900">
                  {adminIdentity.name || activeUser.name || "管理员"}
                </p>
                <p className="truncate text-xs text-slate-500">{adminIdentity.email || activeUser.email}</p>
              </div>
              <Badge variant="outline" className="border-slate-200 bg-slate-50 text-[10px] text-slate-600">
                Admin
              </Badge>
            </div>
            <div className="mt-3 flex items-center gap-2">
              <Button
                type="button"
                size="sm"
                variant="ghost"
                className="admin-sidebar-footer-button flex-1 justify-start text-slate-700 hover:bg-slate-50"
                onClick={handleBackHome}
              >
                返回首页
              </Button>
              <Button
                type="button"
                size="sm"
                variant="ghost"
                className="admin-sidebar-footer-button admin-sidebar-footer-button--icon text-slate-700 hover:bg-slate-50"
                onClick={() => void onLogout()}
              >
                <LogOut size={14} />
                <span className="sr-only">退出</span>
              </Button>
            </div>
          </div>
        </aside>

        <section className="min-w-0 flex-1 p-2 lg:p-2">
          <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border border-slate-200/80 bg-white shadow-sm">
            <header className="flex h-14 items-center px-4">
              <div className="flex min-w-0 items-center gap-3">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="admin-header-sidebar-toggle h-8 w-8 rounded-sm p-0 text-slate-500 hover:bg-transparent hover:text-slate-700"
                  onClick={() => setIsSidebarCollapsed((previousState) => !previousState)}
                  aria-label={isSidebarCollapsed ? "展开侧边栏" : "收起侧边栏"}
                >
                  {isSidebarCollapsed ? <PanelLeftOpen size={16} /> : <PanelLeftClose size={16} />}
                </Button>
                <div className="h-5 w-px bg-slate-200" />
                <h2 className="truncate text-xl font-semibold tracking-tight">{activeContent.title}</h2>
                <div className="lg:hidden">
                  <Select
                    value={currentMenuPath}
                    onValueChange={handleNavigateMenu}
                  >
                    <SelectTrigger className="h-8 w-[148px] text-xs">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {adminMenuItems.map((item) => (
                        <SelectItem key={item.key} value={item.path}>
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
            </header>

            <div className="border-t border-slate-200/80" />

            <main className="min-h-0 flex-1 overflow-auto overscroll-none px-4 pb-4 pt-2 lg:px-6 lg:pb-6 lg:pt-3">
              <div className="mx-auto max-w-[1500px]">
              {activeMenuItem?.key === "users" ? (
                <AdminUsersPage currentUserID={adminIdentity.userId} dataGateway={dataGateway} />
              ) : activeMenuItem?.key === "profile" ? (
                <AdminProfilePage dataGateway={dataGateway} onProfileUpdated={handleProfileUpdated} />
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
              ) : activeMenuItem?.key === "dashboard" ? (
                <AdminDashboardPreview menuItems={adminMenuItems} onNavigate={handleNavigateMenu} />
              ) : (
                <Card className="border-slate-200 bg-white shadow-sm">
                  <CardHeader className="pb-3">
                    <CardTitle>{activeContent.title}</CardTitle>
                    <CardDescription>{activeContent.description}</CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-2 text-sm text-slate-600">
                    {activeContent.todo.map((todo) => (
                      <p key={todo}>- {todo}</p>
                    ))}
                  </CardContent>
                </Card>
              )}
              </div>
            </main>
          </div>
        </section>
      </div>
    </div>
  );
}
