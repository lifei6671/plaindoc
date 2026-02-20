import { LoaderCircle, PencilLine, RefreshCw, Search, ShieldBan, ShieldCheck, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEventHandler } from "react";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { Checkbox } from "../../components/ui/checkbox";
import { Input } from "../../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { showToast } from "../../components/ui/toast";
import { type AdminSpace, type AdminSpaceListResult, type DataGateway, type Visibility } from "../../data-access";
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

interface AdminSpacesPageProps {
  dataGateway: DataGateway;
}

interface AdminSpacesState {
  items: AdminSpace[];
  pagination: AdminSpaceListResult["pagination"];
}

function emptySpacesState(): AdminSpacesState {
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

function renderStatusLabel(value: AdminSpace["status"]): string {
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

function renderStatusBadgeClass(value: AdminSpace["status"]): string {
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

function toVisibilityValue(raw: string): Visibility | null {
  const value = raw.trim().toLowerCase();
  if (value === "public" || value === "authenticated" || value === "member") {
    return value;
  }
  return null;
}

export function AdminSpacesPage({ dataGateway }: AdminSpacesPageProps) {
  const { confirm, prompt, dialogs } = useAdminDialogs();

  const [keywordInput, setKeywordInput] = useState("");
  const [keyword, setKeyword] = useState("");
  const [statusFilter, setStatusFilter] = useState<"" | "all" | "active" | "banned" | "deleted">("");
  const [visibilityFilter, setVisibilityFilter] = useState<"" | "all" | "public" | "authenticated" | "member">("");
  const [page, setPage] = useState(1);
  const [selectedSpaceIDs, setSelectedSpaceIDs] = useState<string[]>([]);

  const [spacesState, setSpacesState] = useState<AdminSpacesState>(() => emptySpacesState());
  const [loading, setLoading] = useState(false);

  const [actioningSpaceID, setActioningSpaceID] = useState<string | null>(null);
  const [batchActioning, setBatchActioning] = useState(false);

  const openToast = useCallback((message: string, variant: "success" | "info" | "error" = "error") => {
    showToast(message, variant);
  }, []);

  const loadSpaces = useCallback(async () => {
    setLoading(true);
    try {
      const payload = await dataGateway.admin.listSpaces({
        keyword,
        status: statusFilter || undefined,
        visibility: visibilityFilter || undefined,
        page,
        pageSize: DEFAULT_PAGE_SIZE
      });
      setSpacesState(payload);
    } catch (error) {
      openToast(`加载空间列表失败：${formatError(error)}`);
      setSpacesState(emptySpacesState());
    } finally {
      setLoading(false);
    }
  }, [dataGateway, keyword, openToast, page, statusFilter, visibilityFilter]);

  useEffect(() => {
    void loadSpaces();
  }, [loadSpaces]);

  useEffect(() => {
    setSelectedSpaceIDs((previous) =>
      previous.filter((spaceID) => spacesState.items.some((item) => item.spaceId === spaceID && item.status !== "deleted"))
    );
  }, [spacesState.items]);

  const selectedSpaceSet = useMemo(() => new Set(selectedSpaceIDs), [selectedSpaceIDs]);
  const selectableSpaceIDs = useMemo(
    () => spacesState.items.filter((item) => item.status !== "deleted").map((item) => item.spaceId),
    [spacesState.items]
  );
  const allSelectableChecked = useMemo(
    () => selectableSpaceIDs.length > 0 && selectableSpaceIDs.every((spaceID) => selectedSpaceSet.has(spaceID)),
    [selectableSpaceIDs, selectedSpaceSet]
  );

  const totalPages = useMemo(() => {
    const total = spacesState.pagination.total;
    const pageSize = spacesState.pagination.pageSize || DEFAULT_PAGE_SIZE;
    return Math.max(1, Math.ceil(total / pageSize));
  }, [spacesState.pagination.pageSize, spacesState.pagination.total]);

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
    setVisibilityFilter("");
    setPage(1);
  }, []);

  const runSpaceAction = useCallback(
    async (spaceID: string, callback: () => Promise<void>) => {
      setActioningSpaceID(spaceID);
      try {
        await callback();
        await loadSpaces();
      } catch (error) {
        openToast(`执行操作失败：${formatError(error)}`);
      } finally {
        setActioningSpaceID(null);
      }
    },
    [loadSpaces, openToast]
  );

  const runBatchSpaceAction = useCallback(
    async (
      actionName: string,
      filter: (space: AdminSpace) => boolean,
      callback: (space: AdminSpace) => Promise<void>
    ) => {
      const targets = spacesState.items.filter((space) => selectedSpaceSet.has(space.spaceId) && filter(space));
      if (targets.length === 0) {
        openToast("请先选择可操作的空间");
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
            failures.push(`${target.name || target.spaceId}: ${formatError(error)}`);
          }
        }
        await loadSpaces();
        setSelectedSpaceIDs([]);
        if (failures.length > 0) {
          openToast(`批量${actionName}完成：成功 ${successCount}，失败 ${failures.length}。首个失败：${failures[0]}`);
        }
      } finally {
        setBatchActioning(false);
      }
    },
    [loadSpaces, openToast, selectedSpaceSet, spacesState.items]
  );

  const handleUpdateMetadata = useCallback(
    async (space: AdminSpace) => {
      const promptResult = await prompt({
        title: `更新空间：${space.name}`,
        description: "可修改空间名称和可见性策略。",
        confirmText: "保存变更",
        fields: [
          {
            key: "name",
            label: "空间名称",
            required: true,
            defaultValue: space.name,
            placeholder: "请输入空间名称"
          },
          {
            key: "visibility",
            label: "可见性",
            type: "select",
            required: true,
            defaultValue: space.visibility,
            options: [
              { value: "public", label: "完全公开（public）" },
              { value: "authenticated", label: "登录可见（authenticated）" },
              { value: "member", label: "成员可见（member）" }
            ]
          }
        ]
      });
      if (!promptResult) {
        return;
      }
      const normalizedName = (promptResult.name ?? "").trim();
      if (!normalizedName) {
        openToast("空间名称不能为空");
        return;
      }
      const nextVisibility = toVisibilityValue(promptResult.visibility ?? "");
      if (!nextVisibility) {
        openToast("可见性仅支持 public/authenticated/member");
        return;
      }

      await runSpaceAction(space.spaceId, async () => {
        await dataGateway.admin.updateSpaceMetadata({
          spaceId: space.spaceId,
          name: normalizedName,
          visibility: nextVisibility
        });
      });
    },
    [dataGateway.admin, openToast, prompt, runSpaceAction]
  );

  const handleDelete = useCallback(
    async (space: AdminSpace) => {
      const confirmed = await confirm({
        title: `删除空间：${space.name}`,
        description: "该操作为软删除，空间与文档将不可继续访问。",
        confirmText: "确认删除",
        tone: "danger"
      });
      if (!confirmed) {
        return;
      }

      await runSpaceAction(space.spaceId, async () => {
        await dataGateway.admin.deleteSpace(space.spaceId);
      });
    },
    [confirm, dataGateway.admin, runSpaceAction]
  );

  const handleUpdateStatus = useCallback(
    async (space: AdminSpace, status: "active" | "banned") => {
      if (status === "banned") {
        const promptResult = await prompt({
          title: `封禁空间：${space.name}`,
          description: "请输入封禁原因，封禁后空间将不可访问。",
          confirmText: "确认封禁",
          tone: "danger",
          fields: [
            {
              key: "reason",
              label: "封禁原因",
              required: true,
              defaultValue: space.bannedReason || "",
              placeholder: "例如：违规内容治理"
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

        await runSpaceAction(space.spaceId, async () => {
          await dataGateway.admin.updateSpaceStatus({
            spaceId: space.spaceId,
            status: "banned",
            reason
          });
        });
        return;
      }

      const confirmed = await confirm({
        title: `解封空间：${space.name}`,
        description: "确认后空间状态将恢复为正常。",
        confirmText: "确认解封"
      });
      if (!confirmed) {
        return;
      }

      await runSpaceAction(space.spaceId, async () => {
        await dataGateway.admin.updateSpaceStatus({
          spaceId: space.spaceId,
          status: "active"
        });
      });
    },
    [confirm, dataGateway.admin, openToast, prompt, runSpaceAction]
  );

  const handleToggleSelectAll = useCallback(
    (checked: boolean) => {
      setSelectedSpaceIDs(checked ? selectableSpaceIDs : []);
    },
    [selectableSpaceIDs]
  );

  const handleToggleSelectOne = useCallback((spaceID: string, checked: boolean) => {
    setSelectedSpaceIDs((previous) => {
      if (checked) {
        if (previous.includes(spaceID)) {
          return previous;
        }
        return [...previous, spaceID];
      }
      return previous.filter((value) => value !== spaceID);
    });
  }, []);

  const handleBatchBan = useCallback(async () => {
    const promptResult = await prompt({
      title: "批量封禁空间",
      description: "封禁原因会应用到当前所有选中空间。",
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

    await runBatchSpaceAction(
      "封禁",
      (space) => space.status === "active",
      async (space) => {
        await dataGateway.admin.updateSpaceStatus({
          spaceId: space.spaceId,
          status: "banned",
          reason
        });
      }
    );
  }, [dataGateway.admin, openToast, prompt, runBatchSpaceAction]);

  const handleBatchUnban = useCallback(async () => {
    const confirmed = await confirm({
      title: "批量解封空间",
      description: "确认后将解封所有已选中且当前为封禁状态的空间。",
      confirmText: "确认解封"
    });
    if (!confirmed) {
      return;
    }

    await runBatchSpaceAction(
      "解封",
      (space) => space.status === "banned",
      async (space) => {
        await dataGateway.admin.updateSpaceStatus({
          spaceId: space.spaceId,
          status: "active"
        });
      }
    );
  }, [confirm, dataGateway.admin, runBatchSpaceAction]);

  const handleBatchDelete = useCallback(async () => {
    const confirmed = await confirm({
      title: "批量删除空间",
      description: "该操作为软删除，确认后将删除所有选中空间。",
      confirmText: "确认删除",
      tone: "danger"
    });
    if (!confirmed) {
      return;
    }

    await runBatchSpaceAction(
      "删除",
      (space) => space.status !== "deleted",
      async (space) => {
        await dataGateway.admin.deleteSpace(space.spaceId);
      }
    );
  }, [confirm, dataGateway.admin, runBatchSpaceAction]);

  const selectionDisabled = loading || batchActioning || actioningSpaceID !== null;

  return (
    <section aria-label="空间管理">
      {dialogs}
      <AdminPageCard>
          <form className="grid gap-3 xl:grid-cols-[minmax(0,1fr)_170px_190px_auto]" onSubmit={handleSearchSubmit}>
            <label className="space-y-1.5">
              <span className="text-xs font-semibold tracking-wide text-slate-600">关键词</span>
              <Input
                type="search"
                value={keywordInput}
                placeholder="空间名称 / 空间 ID / 创建者"
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
                  onClick={() => void loadSpaces()}
                >
                  <RefreshCw size={14} />
                  <span>刷新</span>
                </Button>
            </AdminToolbarActions>
          </form>

          <AdminBulkActionBar selectedCount={selectedSpaceIDs.length}>
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="border-amber-200 bg-amber-50 text-amber-700 hover:bg-amber-100"
                disabled={selectionDisabled || selectedSpaceIDs.length === 0}
                onClick={() => void handleBatchBan()}
              >
                批量封禁
              </Button>
              <Button
                type="button"
                size="sm"
                variant="secondary"
                disabled={selectionDisabled || selectedSpaceIDs.length === 0}
                onClick={() => void handleBatchUnban()}
              >
                批量解封
              </Button>
              <Button
                type="button"
                size="sm"
                variant="destructive"
                disabled={selectionDisabled || selectedSpaceIDs.length === 0}
                onClick={() => void handleBatchDelete()}
              >
                批量删除
              </Button>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={selectionDisabled || selectedSpaceIDs.length === 0}
                onClick={() => setSelectedSpaceIDs([])}
              >
                清空选择
              </Button>
          </AdminBulkActionBar>

          <AdminTableContainer>
              <table className="w-full min-w-[1120px] border-collapse text-left text-sm">
                <thead className="sticky top-0 z-10 bg-slate-50/95 backdrop-blur">
                  <tr className="text-xs uppercase tracking-wide text-slate-600">
                    <th className="w-10 border-b border-slate-200 px-3 py-2 font-semibold">
                      <Checkbox
                        checked={allSelectableChecked}
                        disabled={selectionDisabled || selectableSpaceIDs.length === 0}
                        onCheckedChange={(checked) => handleToggleSelectAll(checked === true)}
                        aria-label="全选空间"
                      />
                    </th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">空间信息</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">创建者</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">可见性</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">状态</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">创建时间</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">更新时间</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">操作</th>
                  </tr>
                </thead>
                <tbody>
                  {loading ? (
                    <tr>
                      <td colSpan={8} className="px-3 py-12">
                        <div className="flex items-center justify-center gap-2 text-sm text-slate-500">
                          <LoaderCircle size={15} className="animate-spin" />
                          <span>正在加载空间列表...</span>
                        </div>
                      </td>
                    </tr>
                  ) : spacesState.items.length === 0 ? (
                    <tr>
                      <td colSpan={8} className="px-3 py-12 text-center text-sm text-slate-500">
                        暂无符合条件的数据
                      </td>
                    </tr>
                  ) : (
                    spacesState.items.map((space) => {
                      const isActioning = actioningSpaceID === space.spaceId || batchActioning;
                      const isDeleted = space.status === "deleted";
                      return (
                        <tr key={space.spaceId} className="border-b border-slate-100 align-top text-slate-700">
                          <td className="px-3 py-3">
                            <Checkbox
                              checked={selectedSpaceSet.has(space.spaceId)}
                              disabled={selectionDisabled || isDeleted}
                              onCheckedChange={(checked) => handleToggleSelectOne(space.spaceId, checked === true)}
                              aria-label={`选择空间 ${space.name || space.spaceId}`}
                            />
                          </td>
                          <td className="px-3 py-3">
                            <div className="grid gap-1">
                              <strong className="text-sm font-semibold text-slate-900">{space.name}</strong>
                              <code className="w-fit rounded border border-sky-200 bg-sky-50 px-1.5 py-0.5 text-xs text-sky-700">
                                {space.spaceId}
                              </code>
                            </div>
                          </td>
                          <td className="px-3 py-3">
                            <div className="grid gap-1 text-xs">
                              <strong className="font-semibold text-slate-800">{space.ownerName || "-"}</strong>
                              <span className="text-slate-600">{space.ownerEmail || "-"}</span>
                            </div>
                          </td>
                          <td className="px-3 py-3">
                            <Badge variant="outline" className={renderVisibilityBadgeClass(space.visibility)}>
                              {renderVisibilityLabel(space.visibility)}
                            </Badge>
                          </td>
                          <td className="px-3 py-3">
                            <div className="grid gap-1.5">
                              <Badge variant="outline" className={renderStatusBadgeClass(space.status)}>
                                {renderStatusLabel(space.status)}
                              </Badge>
                              {space.status === "banned" && space.bannedReason ? (
                                <small className="text-xs text-rose-600">{space.bannedReason}</small>
                              ) : null}
                            </div>
                          </td>
                          <td className="px-3 py-3 text-xs text-slate-600">{formatDateTime(space.createdAt)}</td>
                          <td className="px-3 py-3 text-xs text-slate-600">{formatDateTime(space.updatedAt)}</td>
                          <td className="px-3 py-3">
                            <div className="flex flex-wrap items-center gap-2">
                              {space.status === "banned" ? (
                                <Button
                                  type="button"
                                  size="sm"
                                  variant="secondary"
                                  disabled={isActioning || isDeleted}
                                  onClick={() => void handleUpdateStatus(space, "active")}
                                >
                                  <ShieldCheck size={14} />
                                  <span>解封</span>
                                </Button>
                              ) : (
                                <Button
                                  type="button"
                                  size="sm"
                                  variant="outline"
                                  className="border-amber-200 bg-amber-50 text-amber-700 hover:bg-amber-100"
                                  disabled={isActioning || isDeleted}
                                  onClick={() => void handleUpdateStatus(space, "banned")}
                                >
                                  <ShieldBan size={14} />
                                  <span>封禁</span>
                                </Button>
                              )}
                              <Button
                                type="button"
                                size="sm"
                                variant="outline"
                                disabled={isActioning || isDeleted}
                                onClick={() => void handleUpdateMetadata(space)}
                              >
                                <PencilLine size={14} />
                                <span>元数据</span>
                              </Button>
                              <Button
                                type="button"
                                size="sm"
                                variant="destructive"
                                disabled={isActioning || isDeleted}
                                onClick={() => void handleDelete(space)}
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
              当前第 {spacesState.pagination.page} / {totalPages} 页，共 {spacesState.pagination.total} 条
              </>
            }
            previousDisabled={loading || spacesState.pagination.page <= 1}
            nextDisabled={loading || spacesState.pagination.page >= totalPages}
            onPrevious={() => setPage((value) => Math.max(1, value - 1))}
            onNext={() => setPage((value) => Math.min(totalPages, value + 1))}
          />
      </AdminPageCard>
    </section>
  );
}
