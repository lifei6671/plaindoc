import { ArrowLeftRight, ChevronDown, Copy, LoaderCircle, Plus, RefreshCw, Search, Settings, ShieldBan, ShieldCheck, Tags, Trash2, UserPlus } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEventHandler } from "react";
import { useNavigate } from "react-router-dom";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { Checkbox } from "../../components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger
} from "../../components/ui/dropdown-menu";
import { Input } from "../../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "../../components/ui/tooltip";
import { showToast } from "../../components/ui/toast";
import { type AdminSpace, type AdminSpaceCategory, type AdminSpaceListResult, type DataGateway, type Visibility } from "../../data-access";
import { formatError } from "../../editor/status-utils";
import { AdminSpaceCategoriesDialog } from "../components/AdminSpaceCategoriesDialog";
import { AdminCreateSpaceDialog } from "../components/AdminCreateSpaceDialog";
import { AdminSpaceMembersDialog } from "../components/AdminSpaceMembersDialog";
import { AdminSpaceSettingsDialog } from "../components/AdminSpaceSettingsDialog";
import { useAdminDialogs } from "../components/AdminDialogs";
import {
  AdminBulkActionBar,
  AdminPageCard,
  AdminPaginationFooter,
  AdminTableContainer,
  AdminToolbarActions
} from "../components/AdminPageLayout";

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
  // 服务端可能返回零时间或无效时间，前端兜底显示为占位符。
  if (value.startsWith("0001-01-01")) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  if (date.getFullYear() <= 1) {
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

// 空间管理页面：承载列表筛选、批量操作与空间设置入口。
export function AdminSpacesPage({ dataGateway }: AdminSpacesPageProps) {
  const { confirm, prompt, dialogs } = useAdminDialogs();
  const navigate = useNavigate();

  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [categoriesDialogOpen, setCategoriesDialogOpen] = useState(false);
  const [settingsDialogOpen, setSettingsDialogOpen] = useState(false);
  const [editingSpace, setEditingSpace] = useState<AdminSpace | null>(null);
  const [membersDialogOpen, setMembersDialogOpen] = useState(false);
  const [managingMembersSpace, setManagingMembersSpace] = useState<AdminSpace | null>(null);

  const [keywordInput, setKeywordInput] = useState("");
  const [keyword, setKeyword] = useState("");
  const [statusFilter, setStatusFilter] = useState<"" | "all" | "active" | "banned" | "deleted">("");
  const [visibilityFilter, setVisibilityFilter] = useState<"" | "all" | "public" | "authenticated" | "member">("");
  const [page, setPage] = useState(1);
  const [selectedSpaceIDs, setSelectedSpaceIDs] = useState<string[]>([]);

  const [spacesState, setSpacesState] = useState<AdminSpacesState>(() => emptySpacesState());
  const [spaceCategories, setSpaceCategories] = useState<AdminSpaceCategory[]>([]);
  const [loading, setLoading] = useState(false);
  const [actioningSpaceID, setActioningSpaceID] = useState<string | null>(null);
  const [batchActioning, setBatchActioning] = useState(false);
  const [updatingCategories, setUpdatingCategories] = useState(false);

  const openToast = useCallback((message: string, variant: "success" | "info" | "error" = "error") => {
    showToast(message, variant);
  }, []);

  const handleOpenSpaceEditor = useCallback(
    (space: AdminSpace) => {
      const targetSpaceID = space.spaceId.trim();
      if (!targetSpaceID) {
        return;
      }
      navigate(`/editor/${encodeURIComponent(targetSpaceID)}`);
    },
    [navigate]
  );

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
  }, [dataGateway.admin, keyword, openToast, page, statusFilter, visibilityFilter]);

  const loadSpaceCategories = useCallback(async () => {
    try {
      const items = await dataGateway.admin.listSpaceCategories();
      setSpaceCategories(items);
    } catch (error) {
      openToast(`加载空间分类失败：${formatError(error)}`);
      setSpaceCategories([]);
    }
  }, [dataGateway.admin, openToast]);

  useEffect(() => {
    void loadSpaces();
  }, [loadSpaces]);

  useEffect(() => {
    void loadSpaceCategories();
  }, [loadSpaceCategories]);

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

  const handleCopySpaceId = useCallback(
    async (spaceId: string) => {
      const value = spaceId.trim();
      if (!value) {
        openToast("空间 ID 为空");
        return;
      }
      try {
        // 复制空间 ID，便于管理员快速定位与粘贴。
        if (navigator.clipboard?.writeText) {
          await navigator.clipboard.writeText(value);
        } else {
          const input = document.createElement("input");
          input.value = value;
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
        openToast("空间 ID 已复制", "success");
      } catch (error) {
        openToast(`复制失败：${formatError(error)}`);
      }
    },
    [openToast]
  );

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

  const handleTransferSpace = useCallback(
    async (space: AdminSpace) => {
      const promptResult = await prompt({
        title: `转让空间：${space.name}`,
        description: "仅支持转让给该空间成员，转让后当前所有者将变为协作者，目标成员成为空间拥有者并获得空间管理员权限。",
        confirmText: "确认转让",
        tone: "danger",
        fields: [
          {
            key: "target",
            label: "目标成员（用户 ID 或邮箱）",
            required: true,
            placeholder: "请输入用户 ID 或邮箱"
          }
        ]
      });
      if (!promptResult) {
        return;
      }
      const targetRaw = (promptResult.target ?? "").trim();
      if (!targetRaw) {
        openToast("转让目标不能为空");
        return;
      }
      if (targetRaw === space.ownerUserId || targetRaw === space.ownerEmail) {
        openToast("不能转让给当前所有者");
        return;
      }

      await runSpaceAction(space.spaceId, async () => {
        if (targetRaw.includes("@")) {
          await dataGateway.admin.transferSpaceOwnership({
            spaceId: space.spaceId,
            targetEmail: targetRaw
          });
          return;
        }
        await dataGateway.admin.transferSpaceOwnership({
          spaceId: space.spaceId,
          targetUserId: targetRaw
        });
      });
    },
    [dataGateway.admin, openToast, prompt, runSpaceAction]
  );

  const handleOpenSettings = useCallback((space: AdminSpace) => {
    // 打开空间设置时同步回填当前记录。
    setEditingSpace(space);
    setSettingsDialogOpen(true);
  }, []);

  const handleOpenMembers = useCallback((space: AdminSpace) => {
    setManagingMembersSpace(space);
    setMembersDialogOpen(true);
  }, []);

  const handleSettingsOpenChange = useCallback((open: boolean) => {
    setSettingsDialogOpen(open);
    if (!open) {
      setEditingSpace(null);
    }
  }, []);

  const handleMembersOpenChange = useCallback((open: boolean) => {
    setMembersDialogOpen(open);
    if (!open) {
      setManagingMembersSpace(null);
    }
  }, []);

  const runBatchSpaceAction = useCallback(
    async (actionName: string, filter: (space: AdminSpace) => boolean, callback: (space: AdminSpace) => Promise<void>) => {
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
        } else {
          openToast(`批量${actionName}成功：共 ${successCount} 条`, "success");
        }
      } finally {
        setBatchActioning(false);
      }
    },
    [loadSpaces, openToast, selectedSpaceSet, spacesState.items]
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

    await runBatchSpaceAction("封禁", (space) => space.status === "active", async (space) => {
      await dataGateway.admin.updateSpaceStatus({
        spaceId: space.spaceId,
        status: "banned",
        reason
      });
    });
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

    await runBatchSpaceAction("解封", (space) => space.status === "banned", async (space) => {
      await dataGateway.admin.updateSpaceStatus({
        spaceId: space.spaceId,
        status: "active"
      });
    });
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

    await runBatchSpaceAction("删除", (space) => space.status !== "deleted", async (space) => {
      await dataGateway.admin.deleteSpace(space.spaceId);
    });
  }, [confirm, dataGateway.admin, runBatchSpaceAction]);

  const handleSpaceCreated = useCallback(async () => {
    setSelectedSpaceIDs([]);
    await loadSpaces();
  }, [loadSpaces]);

  const handleCreateCategory = useCallback(
    async (name: string) => {
      setUpdatingCategories(true);
      try {
        await dataGateway.admin.createSpaceCategory({ name });
        await loadSpaceCategories();
        openToast("分类创建成功", "success");
      } catch (error) {
        openToast(`新增分类失败：${formatError(error)}`);
      } finally {
        setUpdatingCategories(false);
      }
    },
    [dataGateway.admin, loadSpaceCategories, openToast]
  );

  const handleRenameCategory = useCallback(
    async (category: AdminSpaceCategory, name: string) => {
      setUpdatingCategories(true);
      try {
        await dataGateway.admin.renameSpaceCategory({
          categoryId: category.categoryId,
          name
        });
        await loadSpaceCategories();
        await loadSpaces();
        openToast("分类已重命名", "success");
      } catch (error) {
        openToast(`重命名分类失败：${formatError(error)}`);
      } finally {
        setUpdatingCategories(false);
      }
    },
    [dataGateway.admin, loadSpaceCategories, loadSpaces, openToast]
  );

  const handleDeleteCategory = useCallback(
    async (category: AdminSpaceCategory) => {
      const confirmed = await confirm({
        title: `删除分类：${category.name}`,
        description: "删除后，该分类下空间将自动迁移到“未分类”。",
        confirmText: "确认删除",
        tone: "danger"
      });
      if (!confirmed) {
        return;
      }

      setUpdatingCategories(true);
      try {
        const result = await dataGateway.admin.deleteSpaceCategory({
          categoryId: category.categoryId
        });
        await loadSpaceCategories();
        await loadSpaces();
        openToast(`分类已删除，已迁移 ${result.movedSpaceCount} 个空间`, "success");
      } catch (error) {
        openToast(`删除分类失败：${formatError(error)}`);
      } finally {
        setUpdatingCategories(false);
      }
    },
    [confirm, dataGateway.admin, loadSpaceCategories, loadSpaces, openToast]
  );

  const selectionDisabled = loading || batchActioning || actioningSpaceID !== null;

  return (
    <section aria-label="空间管理">
      {dialogs}
      <AdminSpaceCategoriesDialog
        open={categoriesDialogOpen}
        loading={updatingCategories}
        categories={spaceCategories}
        onOpenChange={setCategoriesDialogOpen}
        onCreate={handleCreateCategory}
        onRename={handleRenameCategory}
        onDelete={handleDeleteCategory}
      />
      <AdminCreateSpaceDialog
        open={createDialogOpen}
        dataGateway={dataGateway}
        categoryOptions={spaceCategories}
        onOpenChange={setCreateDialogOpen}
        onCreated={handleSpaceCreated}
      />
      <AdminSpaceSettingsDialog
        open={settingsDialogOpen}
        space={editingSpace}
        dataGateway={dataGateway}
        categoryOptions={spaceCategories}
        onOpenChange={handleSettingsOpenChange}
        onUpdated={handleSpaceCreated}
      />
      <AdminSpaceMembersDialog
        open={membersDialogOpen}
        space={managingMembersSpace}
        dataGateway={dataGateway}
        onOpenChange={handleMembersOpenChange}
      />

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
          <AdminToolbarActions actionsClassName="h-auto flex-wrap">
            <Button type="submit" disabled={loading || batchActioning}>
              <Search size={14} />
              <span>查询</span>
            </Button>
            <Button type="button" variant="outline" disabled={loading || batchActioning} onClick={handleReset}>
              重置
            </Button>
            <Button type="button" variant="outline" disabled={loading || batchActioning} onClick={() => void loadSpaces()}>
              <RefreshCw size={14} />
              <span>刷新</span>
            </Button>
            <Button type="button" variant="outline" disabled={loading || batchActioning || updatingCategories} onClick={() => setCategoriesDialogOpen(true)}>
              <Tags size={14} />
              <span>分类管理</span>
            </Button>
            <Button type="button" disabled={loading || batchActioning} onClick={() => setCreateDialogOpen(true)}>
              <Plus size={14} />
              <span>新建空间</span>
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

        <TooltipProvider delayDuration={200}>
          <AdminTableContainer>
            <table className="w-full min-w-[1180px] table-fixed border-collapse text-left text-sm">
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
                  <th className="min-w-[360px] border-b border-slate-200 px-3 py-2 font-semibold">空间信息</th>
                  <th className="w-[160px] border-b border-slate-200 px-3 py-2 font-semibold">创建者</th>
                  <th className="w-[110px] border-b border-slate-200 px-3 py-2 font-semibold">可见性</th>
                  <th className="w-[110px] border-b border-slate-200 px-3 py-2 font-semibold">状态</th>
                  <th className="w-[130px] border-b border-slate-200 px-3 py-2 font-semibold">创建时间</th>
                  <th className="w-[130px] border-b border-slate-200 px-3 py-2 font-semibold">更新时间</th>
                  <th className="w-[220px] border-b border-slate-200 px-3 py-2 font-semibold">操作</th>
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
                      <td className="min-w-0 px-3 py-3">
                        <div className="flex items-start gap-3">
                          <button
                            type="button"
                            className="h-16 w-10 shrink-0 overflow-hidden rounded-md border-0 bg-transparent p-0 transition-opacity hover:opacity-90 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-60"
                            disabled={isDeleted}
                            onClick={() => handleOpenSpaceEditor(space)}
                            title={isDeleted ? "已删除空间不可编辑" : `打开空间：${space.name || space.spaceId}`}
                          >
                            {space.cover?.url ? (
                              <img src={space.cover.url} alt={`${space.name}-cover`} className="h-full w-full object-cover" />
                            ) : <span className="sr-only">打开空间编辑器</span>}
                          </button>
                          <div className="grid min-w-0 gap-1">
                            <button
                              type="button"
                              className="w-fit max-w-full truncate border-0 bg-transparent p-0 text-left text-sm font-semibold text-slate-900 transition-colors hover:text-sky-700 focus-visible:outline-none disabled:cursor-not-allowed disabled:text-slate-400"
                              disabled={isDeleted}
                              onClick={() => handleOpenSpaceEditor(space)}
                              title={isDeleted ? "已删除空间不可编辑" : `打开空间：${space.name || space.spaceId}`}
                            >
                              {space.name || space.spaceId}
                            </button>
                            <div className="flex min-w-0 items-center gap-1.5">
                              <code
                                className="max-w-[220px] truncate rounded border border-sky-200 bg-sky-50 px-1.5 py-0.5 text-xs text-sky-700"
                                title={space.spaceId}
                              >
                                {space.spaceId}
                              </code>
                              <Button
                                type="button"
                                size="icon"
                                variant="ghost"
                                className="h-6 w-6 text-slate-500 hover:text-slate-700"
                                onClick={() => void handleCopySpaceId(space.spaceId)}
                                aria-label="复制空间 ID"
                                title="复制空间 ID"
                              >
                                <Copy size={14} />
                              </Button>
                            </div>
                            {space.description ? (
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <p className="line-clamp-2 break-words text-xs leading-5 text-slate-600" tabIndex={0}>
                                    {space.description}
                                  </p>
                                </TooltipTrigger>
                                <TooltipContent side="top" align="start" className="max-w-[360px] whitespace-pre-wrap break-words">
                                  {space.description}
                                </TooltipContent>
                              </Tooltip>
                            ) : (
                              <p className="text-xs text-slate-400">暂无简介</p>
                            )}
                            <div className="flex items-center gap-1.5">
                              <span className="text-xs text-slate-500">分类：</span>
                              {space.category ? (
                                <Badge variant="outline" className="border-indigo-200 bg-indigo-50 text-indigo-700">
                                  {space.category}
                                </Badge>
                              ) : (
                                <span className="text-xs text-slate-400">未分类</span>
                              )}
                            </div>
                          </div>
                        </div>
                      </td>
                      <td className="px-3 py-3">
                        <div className="grid gap-1 text-xs">
                          <strong className="font-semibold text-slate-800">{space.ownerName || "-"}</strong>
                          <span className="text-slate-600">{space.ownerEmail || "-"}</span>
                        </div>
                      </td>
                      <td className="px-3 py-3">
                        <Badge
                          variant="outline"
                          className={`${renderVisibilityBadgeClass(space.visibility)} w-fit whitespace-nowrap`}
                        >
                          {renderVisibilityLabel(space.visibility)}
                        </Badge>
                      </td>
                      <td className="px-3 py-3">
                        <div className="grid justify-items-start gap-1.5">
                          <Badge variant="outline" className={`${renderStatusBadgeClass(space.status)} w-fit whitespace-nowrap`}>
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
                        <div className="inline-flex rounded-md">
                          <Button
                            type="button"
                            size="sm"
                            variant="outline"
                            className="rounded-r-none border-r-0"
                            disabled={isActioning || isDeleted}
                            onClick={() => handleOpenSettings(space)}
                          >
                            <Settings size={14} />
                            <span>设置</span>
                          </Button>
                          <DropdownMenu modal={false}>
                            <DropdownMenuTrigger asChild>
                              <Button
                                type="button"
                                size="sm"
                                variant="outline"
                                className="w-8 rounded-l-none px-0"
                                disabled={isActioning || isDeleted}
                                aria-label="展开更多操作"
                              >
                                <ChevronDown size={14} />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end" className="w-40">
                              {space.status === "banned" ? (
                                <DropdownMenuItem
                                  disabled={isActioning || isDeleted}
                                  onSelect={() => void handleUpdateStatus(space, "active")}
                                >
                                  <ShieldCheck size={14} className="mr-2" />
                                  <span>解封空间</span>
                                </DropdownMenuItem>
                              ) : (
                                <DropdownMenuItem
                                  disabled={isActioning || isDeleted}
                                  onSelect={() => void handleUpdateStatus(space, "banned")}
                                  className="text-amber-700 focus:text-amber-700"
                                >
                                  <ShieldBan size={14} className="mr-2" />
                                  <span>封禁空间</span>
                                </DropdownMenuItem>
                              )}
                              <DropdownMenuItem
                                disabled={isActioning || isDeleted}
                                onSelect={() => handleOpenMembers(space)}
                                className="text-sky-700 focus:text-sky-700"
                              >
                                <UserPlus size={14} className="mr-2" />
                                <span>成员管理</span>
                              </DropdownMenuItem>
                              <DropdownMenuItem
                                disabled={isActioning || isDeleted}
                                onSelect={() => void handleTransferSpace(space)}
                                className="text-indigo-700 focus:text-indigo-700"
                              >
                                <ArrowLeftRight size={14} className="mr-2" />
                                <span>转让空间</span>
                              </DropdownMenuItem>
                              <DropdownMenuSeparator />
                              <DropdownMenuItem
                                disabled={isActioning || isDeleted}
                                onSelect={() => void handleDelete(space)}
                                className="text-rose-600 focus:text-rose-600"
                              >
                                <Trash2 size={14} className="mr-2" />
                                <span>删除空间</span>
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
        </TooltipProvider>

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
