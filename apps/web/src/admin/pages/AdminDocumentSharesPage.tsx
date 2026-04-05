import { ChevronDown, Clock3, Copy, LoaderCircle, Lock, LockOpen, RefreshCw, Search, Trash2, User, Users, type LucideIcon } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEventHandler } from "react";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "../../components/ui/dropdown-menu";
import { Input } from "../../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { showToast } from "../../components/ui/toast";
import { type AdminDocumentShare, type AdminDocumentShareListResult, type AdminRole, type DataGateway } from "../../data-access";
import { formatError } from "../../editor/status-utils";
import { useAdminDialogs } from "../components/AdminDialogs";
import { AdminPageCard, AdminPaginationFooter, AdminTableContainer, AdminToolbarActions } from "../components/AdminPageLayout";

const DEFAULT_PAGE_SIZE = 20;

interface AdminDocumentSharesPageProps {
  dataGateway: DataGateway;
  currentUserRoles: AdminRole[];
  currentUserID: string;
}

interface AdminDocumentSharesState {
  items: AdminDocumentShare[];
  pagination: AdminDocumentShareListResult["pagination"];
}

type ShareCenterMenuKey = "mine" | "all";

interface ShareCenterMenuItem {
  key: ShareCenterMenuKey;
  label: string;
  description: string;
  view: "mine" | "all";
  icon: LucideIcon;
}

function emptyDocumentSharesState(): AdminDocumentSharesState {
  return {
    items: [],
    pagination: {
      page: 1,
      pageSize: DEFAULT_PAGE_SIZE,
      total: 0
    }
  };
}

function formatDateTime(value: string | null | undefined): string {
  const rawValue = typeof value === "string" ? value.trim() : "";
  if (!rawValue) {
    return "永久";
  }
  const date = new Date(rawValue);
  if (Number.isNaN(date.getTime())) {
    return rawValue;
  }
  return date.toLocaleString("zh-CN", { hour12: false });
}

function renderModeLabel(mode: AdminDocumentShare["mode"]): string {
  return mode === "password" ? "密码分享" : "公开分享";
}

function renderModeBadgeClass(mode: AdminDocumentShare["mode"]): string {
  return mode === "password"
    ? "border-amber-200 bg-amber-50 text-amber-700"
    : "border-emerald-200 bg-emerald-50 text-emerald-700";
}

function buildShareReaderPath(item: AdminDocumentShare): string {
  const spaceID = (item.spaceId ?? "").trim();
  const routeKey = (item.documentRouteKey ?? "").trim() || (item.documentId ?? "").trim();
  if (!spaceID || !routeKey) {
    return "";
  }
  return `/s/${encodeURIComponent(spaceID)}/${encodeURIComponent(routeKey)}`;
}

function resolveAbsoluteUrl(raw: string): string {
  const value = raw.trim();
  if (!value) {
    return "";
  }
  if (/^(https?:)?\/\//i.test(value)) {
    return value;
  }
  if (typeof window === "undefined") {
    return value;
  }
  try {
    return new URL(value, window.location.origin).toString();
  } catch {
    return value;
  }
}

function buildShareCenterMenus(canManageAllShares: boolean): ShareCenterMenuItem[] {
  // 普通用户也可以进入分享中心，但只允许停留在“我的分享”视图。
  const menus: ShareCenterMenuItem[] = [
    {
      key: "mine",
      label: "我的分享",
      description: "查看当前登录人创建的分享记录",
      view: "mine",
      icon: User
    }
  ];
  if (canManageAllShares) {
    menus.push({
      key: "all",
      label: "分享管理",
      description: "查看并管理权限范围内全部分享记录",
      view: "all",
      icon: Users
    });
  }
  return menus;
}

export function AdminDocumentSharesPage({ dataGateway, currentUserRoles, currentUserID }: AdminDocumentSharesPageProps) {
  const { confirm, prompt, dialogs } = useAdminDialogs();

  const canManageAllShares = useMemo(() => {
    return currentUserRoles.includes("platform_admin") || currentUserRoles.includes("space_admin");
  }, [currentUserRoles]);
  const menuItems = useMemo(() => buildShareCenterMenus(canManageAllShares), [canManageAllShares]);
  const [selectedMenuKey, setSelectedMenuKey] = useState<ShareCenterMenuKey>("mine");
  const [modeFilter, setModeFilter] = useState<"" | "all" | "public" | "password">("");
  const [expiredFilter, setExpiredFilter] = useState<"" | "all" | "yes" | "no">("");
  const [keywordInput, setKeywordInput] = useState("");
  const [keyword, setKeyword] = useState("");
  const [spaceIdInput, setSpaceIdInput] = useState("");
  const [spaceId, setSpaceId] = useState("");
  const [page, setPage] = useState(1);

  const [sharesState, setSharesState] = useState<AdminDocumentSharesState>(() => emptyDocumentSharesState());
  const [loading, setLoading] = useState(false);
  const [actioningShareID, setActioningShareID] = useState<string | null>(null);

  const selectedMenu = useMemo(() => {
    return menuItems.find((item) => item.key === selectedMenuKey) ?? menuItems[0];
  }, [menuItems, selectedMenuKey]);

  const openToast = useCallback((message: string, variant: "success" | "info" | "error" = "error") => {
    showToast(message, variant);
  }, []);

  useEffect(() => {
    if (menuItems.some((item) => item.key === selectedMenuKey)) {
      return;
    }
    setSelectedMenuKey(menuItems[0]?.key ?? "mine");
  }, [menuItems, selectedMenuKey]);

  const loadShares = useCallback(async () => {
    setLoading(true);
    try {
      const payload = await dataGateway.admin.listDocumentShares({
        view: selectedMenu.view,
        keyword,
        spaceId,
        mode: modeFilter || undefined,
        expired: expiredFilter || undefined,
        page,
        pageSize: DEFAULT_PAGE_SIZE
      });
      setSharesState(payload);
    } catch (error) {
      openToast(`加载分享列表失败：${formatError(error)}`);
      setSharesState(emptyDocumentSharesState());
    } finally {
      setLoading(false);
    }
  }, [dataGateway.admin, expiredFilter, keyword, modeFilter, openToast, page, selectedMenu.view, spaceId]);

  useEffect(() => {
    void loadShares();
  }, [loadShares]);

  const totalPages = useMemo(() => {
    const total = sharesState.pagination.total;
    const pageSize = sharesState.pagination.pageSize || DEFAULT_PAGE_SIZE;
    return Math.max(1, Math.ceil(total / pageSize));
  }, [sharesState.pagination.pageSize, sharesState.pagination.total]);

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
    setModeFilter("");
    setExpiredFilter("");
    setKeywordInput("");
    setKeyword("");
    setSpaceIdInput("");
    setSpaceId("");
    setPage(1);
  }, []);

  const runShareAction = useCallback(
    async (shareID: string, callback: () => Promise<void>) => {
      setActioningShareID(shareID);
      try {
        await callback();
        await loadShares();
      } catch (error) {
        openToast(`执行分享操作失败：${formatError(error)}`);
      } finally {
        setActioningShareID(null);
      }
    },
    [loadShares, openToast]
  );

  const handleDisableShare = useCallback(
    async (item: AdminDocumentShare) => {
      const confirmed = await confirm({
        title: `取消分享：${item.documentTitle || item.documentId}`,
        description: "取消后，当前分享链接将立即失效。",
        confirmText: "确认取消",
        tone: "danger"
      });
      if (!confirmed) {
        return;
      }
      await runShareAction(item.shareId, async () => {
        await dataGateway.admin.deleteDocumentShare(item.shareId);
      });
      openToast("已取消分享", "success");
    },
    [confirm, dataGateway.admin, openToast, runShareAction]
  );

  const handleSwitchToPublic = useCallback(
    async (item: AdminDocumentShare) => {
      await runShareAction(item.shareId, async () => {
        await dataGateway.admin.updateDocumentShare({
          shareId: item.shareId,
          mode: "public"
        });
      });
      openToast("已切换为公开分享", "success");
    },
    [dataGateway.admin, openToast, runShareAction]
  );

  const handleSwitchToPassword = useCallback(
    async (item: AdminDocumentShare) => {
      const promptResult = await prompt({
        title: `设置密码分享：${item.documentTitle || item.documentId}`,
        description: "密码长度至少 6 位，可选填写密码提示。",
        confirmText: "保存密码",
        fields: [
          {
            key: "password",
            label: "访问密码",
            type: "password",
            required: true
          },
          {
            key: "passwordHint",
            label: "密码提示",
            type: "text",
            defaultValue: item.passwordHint ?? ""
          }
        ]
      });
      if (!promptResult) {
        return;
      }
      const password = (promptResult.password ?? "").trim();
      if (password.length < 6) {
        openToast("访问密码至少 6 位");
        return;
      }

      await runShareAction(item.shareId, async () => {
        await dataGateway.admin.updateDocumentShare({
          shareId: item.shareId,
          mode: "password",
          password,
          passwordHint: promptResult.passwordHint ?? ""
        });
      });
      openToast("已切换为密码分享", "success");
    },
    [dataGateway.admin, openToast, prompt, runShareAction]
  );

  const handleChangePassword = useCallback(
    async (item: AdminDocumentShare) => {
      const promptResult = await prompt({
        title: `修改分享密码：${item.documentTitle || item.documentId}`,
        description: "密码长度至少 6 位，可选填写密码提示。",
        confirmText: "更新密码",
        fields: [
          {
            key: "password",
            label: "新密码",
            type: "password",
            required: true
          },
          {
            key: "passwordHint",
            label: "密码提示",
            type: "text",
            defaultValue: item.passwordHint ?? ""
          }
        ]
      });
      if (!promptResult) {
        return;
      }
      const password = (promptResult.password ?? "").trim();
      if (password.length < 6) {
        openToast("访问密码至少 6 位");
        return;
      }

      await runShareAction(item.shareId, async () => {
        await dataGateway.admin.updateDocumentShare({
          shareId: item.shareId,
          mode: "password",
          password,
          passwordHint: promptResult.passwordHint ?? ""
        });
      });
      openToast("分享密码已更新", "success");
    },
    [dataGateway.admin, openToast, prompt, runShareAction]
  );

  const handleExtend7Days = useCallback(
    async (item: AdminDocumentShare) => {
      const now = new Date();
      const baseDate = item.expiresAt ? new Date(item.expiresAt) : now;
      const startDate = Number.isNaN(baseDate.getTime()) || baseDate.getTime() < now.getTime() ? now : baseDate;
      const nextExpiresAt = new Date(startDate.getTime() + 7 * 24 * 60 * 60 * 1000);

      await runShareAction(item.shareId, async () => {
        await dataGateway.admin.updateDocumentShare({
          shareId: item.shareId,
          expiresAt: nextExpiresAt.toISOString()
        });
      });
      openToast("分享有效期已延长 7 天", "success");
    },
    [dataGateway.admin, openToast, runShareAction]
  );

  const handleSetPermanent = useCallback(
    async (item: AdminDocumentShare) => {
      await runShareAction(item.shareId, async () => {
        await dataGateway.admin.updateDocumentShare({
          shareId: item.shareId,
          expiresAt: null
        });
      });
      openToast("已设置为永久分享", "success");
    },
    [dataGateway.admin, openToast, runShareAction]
  );

  const handleCopyShareLink = useCallback(
    async (sharePath: string) => {
      const absoluteUrl = resolveAbsoluteUrl(sharePath);
      if (!absoluteUrl) {
        openToast("分享链接为空");
        return;
      }
      try {
        if (navigator.clipboard?.writeText) {
          await navigator.clipboard.writeText(absoluteUrl);
        } else {
          const input = document.createElement("input");
          input.value = absoluteUrl;
          input.setAttribute("readonly", "true");
          input.style.position = "absolute";
          input.style.left = "-9999px";
          document.body.appendChild(input);
          input.select();
          const copied = document.execCommand("copy");
          document.body.removeChild(input);
          if (!copied) {
            throw new Error("复制失败");
          }
        }
        openToast("分享链接已复制", "success");
      } catch (error) {
        openToast(`复制分享链接失败：${formatError(error)}`);
      }
    },
    [openToast]
  );

  const handleChangeMenu = useCallback(
    (nextMenuKey: ShareCenterMenuKey) => {
      if (nextMenuKey === selectedMenuKey) {
        return;
      }
      setSelectedMenuKey(nextMenuKey);
      setPage(1);
    },
    [selectedMenuKey]
  );

  const SelectedMenuIcon = selectedMenu.icon;

  return (
    <AdminPageCard>
      {dialogs}
      <div className="overflow-hidden rounded-md border border-slate-200 bg-white">
        <div className="grid min-h-[560px] gap-0 md:grid-cols-[240px_minmax(0,1fr)]">
          <aside className="hidden border-r border-slate-200 bg-transparent p-2 md:block">
            <nav className="space-y-1" aria-label="分享中心菜单">
              {menuItems.map((menu) => {
                const isActive = menu.key === selectedMenu.key;
                const MenuIcon = menu.icon;
                return (
                  <button
                    key={menu.key}
                    type="button"
                    className={`w-full appearance-none rounded-lg border border-transparent px-3 py-2.5 text-left shadow-none outline-none transition focus-visible:ring-2 focus-visible:ring-sky-200 ${isActive ? "bg-slate-200 text-slate-900" : "text-slate-700 hover:bg-slate-200/70"
                      }`}
                    onClick={() => handleChangeMenu(menu.key)}
                    disabled={loading}
                  >
                    <p className="flex items-center gap-2.5 text-sm font-medium">
                      <MenuIcon size={16} />
                      <span>{menu.label}</span>
                    </p>
                    <p className="mt-0.5 pl-[26px] text-xs text-slate-500">{menu.description}</p>
                  </button>
                );
              })}
            </nav>
          </aside>

          <main className="flex min-h-[560px] flex-col">
            <header className="flex h-16 shrink-0 items-center justify-between gap-3 border-b border-slate-200 px-4">
              <div>
                <h3 className="flex items-center gap-2 text-base font-semibold text-slate-800">
                  <SelectedMenuIcon size={16} />
                  {selectedMenu.label}
                </h3>
                <p className="mt-0.5 text-xs text-slate-500">{selectedMenu.description}</p>
              </div>

              {menuItems.length > 1 ? (
                <div className="md:hidden">
                  <Select
                    value={selectedMenu.key}
                    onValueChange={(value) => {
                      handleChangeMenu(value as ShareCenterMenuKey);
                    }}
                  >
                    <SelectTrigger className="h-8 w-[140px] text-xs">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {menuItems.map((menu) => (
                        <SelectItem key={menu.key} value={menu.key}>
                          {menu.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              ) : null}
            </header>

            <div className="flex-1 space-y-4 p-4">
              <form className="grid gap-3 rounded-sm border border-slate-200/80 bg-slate-50/60 p-3" onSubmit={handleSearchSubmit}>
                <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,220px)]">
                  <div className="space-y-1.5">
                    <label htmlFor="admin-document-shares-search" className="text-xs font-medium text-slate-600">
                      关键词
                    </label>
                    <Input
                      id="admin-document-shares-search"
                      value={keywordInput}
                      onChange={(event) => setKeywordInput(event.target.value)}
                      placeholder="文档标题"
                      autoComplete="off"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <label htmlFor="admin-document-shares-space-id" className="text-xs font-medium text-slate-600">
                      空间 ID
                    </label>
                    <Input
                      id="admin-document-shares-space-id"
                      value={spaceIdInput}
                      onChange={(event) => setSpaceIdInput(event.target.value)}
                      placeholder="输入空间 ID（可选）"
                      autoComplete="off"
                    />
                  </div>
                </div>
                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="space-y-1.5">
                    <label className="text-xs font-medium text-slate-600">模式</label>
                    <Select value={modeFilter || "all"} onValueChange={(value) => setModeFilter(value === "all" ? "" : (value as "public" | "password"))}>
                      <SelectTrigger className="h-9 bg-white">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="all">全部模式</SelectItem>
                        <SelectItem value="public">公开分享</SelectItem>
                        <SelectItem value="password">密码分享</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-1.5">
                    <label className="text-xs font-medium text-slate-600">过期状态</label>
                    <Select value={expiredFilter || "all"} onValueChange={(value) => setExpiredFilter(value === "all" ? "" : (value as "yes" | "no"))}>
                      <SelectTrigger className="h-9 bg-white">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="all">全部</SelectItem>
                        <SelectItem value="no">有效</SelectItem>
                        <SelectItem value="yes">已过期</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </div>
                <AdminToolbarActions className="justify-end" actionsClassName="justify-end" showGuideLabel={false}>
                  <Button type="button" variant="outline" size="sm" onClick={handleReset} disabled={loading}>
                    重置
                  </Button>
                  <Button type="submit" size="sm" disabled={loading}>
                    {loading ? <LoaderCircle size={14} className="mr-1.5 animate-spin" /> : <Search size={14} className="mr-1.5" />}
                    搜索
                  </Button>
                  <Button type="button" variant="outline" size="sm" onClick={() => void loadShares()} disabled={loading}>
                    <RefreshCw size={14} className="mr-1.5" />
                    刷新
                  </Button>
                </AdminToolbarActions>
              </form>

              <AdminTableContainer>
                <table className="w-full min-w-[1240px] border-collapse text-left text-sm">
                  <thead className="bg-slate-100/70 text-xs uppercase tracking-wide text-slate-600">
                    <tr>
                      <th className="px-3 py-2.5 font-semibold">文档</th>
                      <th className="px-3 py-2.5 font-semibold">空间</th>
                      <th className="px-3 py-2.5 font-semibold">模式</th>
                      <th className="px-3 py-2.5 font-semibold">创建人</th>
                      <th className="px-3 py-2.5 font-semibold">有效期</th>
                      <th className="px-3 py-2.5 font-semibold">更新时间</th>
                      <th className="px-3 py-2.5 font-semibold text-right">操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {loading ? (
                      <tr>
                        <td colSpan={7} className="px-3 py-12 text-center text-sm text-slate-500">
                          <span className="inline-flex items-center gap-2">
                            <LoaderCircle size={16} className="animate-spin" />
                            正在加载分享列表...
                          </span>
                        </td>
                      </tr>
                    ) : sharesState.items.length === 0 ? (
                      <tr>
                        <td colSpan={7} className="px-3 py-12 text-center text-sm text-slate-500">
                          暂无匹配的分享记录
                        </td>
                      </tr>
                    ) : (
                      sharesState.items.map((item) => {
                        const shareReaderPath = buildShareReaderPath(item);
                        const rowActioning = actioningShareID === item.shareId;
                        const canManageRow = canManageAllShares || (item.createdByUserId ?? "").trim() === currentUserID.trim();
                        const copyButtonClassName = canManageRow
                          ? "h-8 rounded-r-none border-r-0 px-2 text-xs"
                          : "h-8 rounded-md px-2 text-xs";
                        return (
                          <tr key={item.shareId} className="border-t border-slate-200/70 align-top text-sm text-slate-700">
                            <td className="px-3 py-3">
                              <div className="space-y-1">
                                {shareReaderPath ? (
                                  <a
                                    href={shareReaderPath}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    className="inline-flex max-w-full truncate font-medium text-slate-900 hover:text-blue-700 hover:underline"
                                  >
                                    {item.documentTitle || "(未命名文档)"}
                                  </a>
                                ) : (
                                  <p className="font-medium text-slate-900">{item.documentTitle || "(未命名文档)"}</p>
                                )}
                                <p className="text-xs text-slate-500">
                                  shareId: <span className="font-mono">{item.shareId}</span>
                                </p>
                              </div>
                            </td>
                            <td className="px-3 py-3 text-xs">
                              <p className="font-medium text-slate-900">{item.spaceName || item.spaceId}</p>
                              <p className="mt-1 text-slate-500">owner: {item.spaceOwnerName || item.spaceOwnerUserId}</p>
                            </td>
                            <td className="px-3 py-3">
                              <div className="space-y-2">
                                <Badge variant="outline" className={renderModeBadgeClass(item.mode)}>
                                  {item.mode === "password" ? <Lock size={12} className="mr-1" /> : <LockOpen size={12} className="mr-1" />}
                                  {renderModeLabel(item.mode)}
                                </Badge>
                                <p className="text-xs text-slate-500">{item.hasPassword ? "已设置密码" : "无密码"}</p>
                                {item.passwordHint ? <p className="text-xs text-slate-500">提示：{item.passwordHint}</p> : null}
                              </div>
                            </td>
                            <td className="px-3 py-3 text-xs">
                              <p className="font-medium text-slate-900">{item.createdByName || item.createdByUserId || "-"}</p>
                              <p className="mt-1 text-slate-500">{item.createdByEmail || "-"}</p>
                            </td>
                            <td className="px-3 py-3 text-xs">
                              <p className="inline-flex items-center gap-1 text-slate-900">
                                <Clock3 size={12} />
                                {formatDateTime(item.expiresAt)}
                              </p>
                              <p className={`mt-1 ${item.isExpired ? "text-rose-600" : "text-emerald-600"}`}>
                                {item.isExpired ? "已过期" : "有效"}
                              </p>
                            </td>
                            <td className="px-3 py-3 text-xs text-slate-500">{formatDateTime(item.updatedAt)}</td>
                            <td className="px-3 py-3">
                              <div className="flex justify-end">
                                <div className={`inline-flex ${canManageRow ? "overflow-hidden rounded-md" : "rounded-md"}`}>
                                  <Button
                                    type="button"
                                    size="sm"
                                    variant="outline"
                                    className={copyButtonClassName}
                                    disabled={rowActioning || !shareReaderPath}
                                    onClick={() => {
                                      void handleCopyShareLink(shareReaderPath);
                                    }}
                                  >
                                      {rowActioning ? (
                                        <LoaderCircle size={12} className="mr-1.5 animate-spin" />
                                      ) : (
                                        <Copy size={12} className="mr-1.5" />
                                      )}
                                      复制链接
                                  </Button>
                                  {/* 普通用户只保留自己创建的分享行内操作，管理员仍可管理全部记录。 */}
                                  {canManageRow ? (
                                    <DropdownMenu modal={false}>
                                      <DropdownMenuTrigger asChild>
                                        <Button
                                          type="button"
                                          size="sm"
                                          variant="outline"
                                          className="h-8 w-8 rounded-l-none px-0"
                                          disabled={rowActioning}
                                          aria-label="展开分享操作"
                                        >
                                          <ChevronDown size={12} />
                                        </Button>
                                      </DropdownMenuTrigger>
                                      <DropdownMenuContent align="end" className="w-44">
                                        <DropdownMenuItem
                                          disabled={rowActioning}
                                          onSelect={() => {
                                            void (item.mode === "public" ? handleSwitchToPassword(item) : handleSwitchToPublic(item));
                                          }}
                                        >
                                          {item.mode === "public" ? <Lock size={14} className="mr-2" /> : <LockOpen size={14} className="mr-2" />}
                                          <span>{item.mode === "public" ? "设密码分享" : "设公开分享"}</span>
                                        </DropdownMenuItem>
                                        {item.hasPassword ? (
                                          <DropdownMenuItem
                                            disabled={rowActioning}
                                            onSelect={() => {
                                              void handleChangePassword(item);
                                            }}
                                          >
                                            <Lock size={14} className="mr-2" />
                                            <span>修改密码</span>
                                          </DropdownMenuItem>
                                        ) : null}
                                        <DropdownMenuItem
                                          disabled={rowActioning}
                                          onSelect={() => {
                                            void handleExtend7Days(item);
                                          }}
                                        >
                                          <Clock3 size={14} className="mr-2" />
                                          <span>延期 7 天</span>
                                        </DropdownMenuItem>
                                        <DropdownMenuItem
                                          disabled={rowActioning}
                                          onSelect={() => {
                                            void handleSetPermanent(item);
                                          }}
                                        >
                                          <RefreshCw size={14} className="mr-2" />
                                          <span>设为永久</span>
                                        </DropdownMenuItem>
                                        <DropdownMenuItem
                                          disabled={rowActioning}
                                          onSelect={() => {
                                            void handleDisableShare(item);
                                          }}
                                          className="text-rose-600 focus:text-rose-600"
                                        >
                                          <Trash2 size={14} className="mr-2" />
                                          <span>取消分享</span>
                                        </DropdownMenuItem>
                                      </DropdownMenuContent>
                                    </DropdownMenu>
                                  ) : null}
                                </div>
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
                    当前第 {sharesState.pagination.page} / {totalPages} 页，共 {sharesState.pagination.total} 条
                  </>
                }
                previousDisabled={loading || sharesState.pagination.page <= 1}
                nextDisabled={loading || sharesState.pagination.page >= totalPages}
                onPrevious={() => setPage((previousPage) => Math.max(1, previousPage - 1))}
                onNext={() => setPage((previousPage) => Math.min(totalPages, previousPage + 1))}
              />
            </div>
          </main>
        </div>
      </div>
    </AdminPageCard>
  );
}
