import { LoaderCircle, PencilLine, RefreshCw, Search, ShieldBan, ShieldCheck, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { type AdminSpace, type AdminSpaceListResult, type DataGateway, type Visibility } from "../../data-access";
import { useAdminDialogs } from "../components/AdminDialogs";
import { TopToast, type TopToastVariant } from "../../components/TopToast";
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
    <section className="admin-spaces-panel" aria-label="空间管理">
      <TopToast
        open={toastState.open}
        message={toastState.message}
        variant={toastState.variant}
        triggerKey={toastState.triggerKey}
        durationMs={2800}
        onClose={closeToast}
      />
      {dialogs}
      <form className="admin-spaces-toolbar" onSubmit={handleSearchSubmit}>
        <label className="admin-spaces-toolbar__field admin-spaces-toolbar__field--keyword">
          <span>关键词</span>
          <input
            type="search"
            value={keywordInput}
            placeholder="空间名称 / 空间 ID / 创建者"
            onChange={(event) => setKeywordInput(event.target.value)}
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
          <button type="button" disabled={loading || batchActioning} onClick={() => void loadSpaces()}>
            <RefreshCw size={14} />
            <span>刷新</span>
          </button>
        </div>
      </form>

      <div className="admin-bulk-bar">
        <p>已选 {selectedSpaceIDs.length} 项</p>
        <div className="admin-bulk-bar__actions">
          <button type="button" disabled={selectionDisabled || selectedSpaceIDs.length === 0} onClick={() => void handleBatchBan()}>
            批量封禁
          </button>
          <button type="button" disabled={selectionDisabled || selectedSpaceIDs.length === 0} onClick={() => void handleBatchUnban()}>
            批量解封
          </button>
          <button
            type="button"
            className="danger"
            disabled={selectionDisabled || selectedSpaceIDs.length === 0}
            onClick={() => void handleBatchDelete()}
          >
            批量删除
          </button>
          <button type="button" disabled={selectionDisabled || selectedSpaceIDs.length === 0} onClick={() => setSelectedSpaceIDs([])}>
            清空选择
          </button>
        </div>
      </div>

      <div className="admin-spaces-table-wrap">
        <table className="admin-spaces-table">
          <thead>
            <tr>
              <th className="admin-select-cell">
                <input
                  type="checkbox"
                  checked={allSelectableChecked}
                  disabled={selectionDisabled || selectableSpaceIDs.length === 0}
                  onChange={(event) => handleToggleSelectAll(event.target.checked)}
                  aria-label="全选空间"
                />
              </th>
              <th>空间信息</th>
              <th>创建者</th>
              <th>可见性</th>
              <th>状态</th>
              <th>创建时间</th>
              <th>更新时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={8}>
                  <div className="admin-spaces-loading-row">
                    <LoaderCircle size={15} className="admin-spaces-loading-row__icon" />
                    <span>正在加载空间列表...</span>
                  </div>
                </td>
              </tr>
            ) : spacesState.items.length === 0 ? (
              <tr>
                <td colSpan={8}>
                  <div className="admin-spaces-empty-row">暂无符合条件的数据</div>
                </td>
              </tr>
            ) : (
              spacesState.items.map((space) => {
                const isActioning = actioningSpaceID === space.spaceId || batchActioning;
                const isDeleted = space.status === "deleted";
                return (
                  <tr key={space.spaceId}>
                    <td className="admin-select-cell">
                      <input
                        type="checkbox"
                        checked={selectedSpaceSet.has(space.spaceId)}
                        disabled={selectionDisabled || isDeleted}
                        onChange={(event) => handleToggleSelectOne(space.spaceId, event.target.checked)}
                        aria-label={`选择空间 ${space.name || space.spaceId}`}
                      />
                    </td>
                    <td>
                      <div className="admin-spaces-space-cell">
                        <strong>{space.name}</strong>
                        <code>{space.spaceId}</code>
                      </div>
                    </td>
                    <td>
                      <div className="admin-spaces-owner-cell">
                        <strong>{space.ownerName || "-"}</strong>
                        <span>{space.ownerEmail || "-"}</span>
                      </div>
                    </td>
                    <td>
                      <span className="admin-spaces-visibility">{renderVisibilityLabel(space.visibility)}</span>
                    </td>
                    <td>
                      <div className="admin-spaces-status-cell">
                        <span className={`admin-spaces-status admin-spaces-status--${space.status}`}>
                          {renderStatusLabel(space.status)}
                        </span>
                        {space.status === "banned" && space.bannedReason ? (
                          <small className="admin-spaces-ban-reason">{space.bannedReason}</small>
                        ) : null}
                      </div>
                    </td>
                    <td>{formatDateTime(space.createdAt)}</td>
                    <td>{formatDateTime(space.updatedAt)}</td>
                    <td>
                      <div className="admin-spaces-actions">
                        {space.status === "banned" ? (
                          <button
                            type="button"
                            disabled={isActioning || isDeleted}
                            onClick={() => void handleUpdateStatus(space, "active")}
                          >
                            <ShieldCheck size={14} />
                            <span>解封</span>
                          </button>
                        ) : (
                          <button
                            type="button"
                            className="warning"
                            disabled={isActioning || isDeleted}
                            onClick={() => void handleUpdateStatus(space, "banned")}
                          >
                            <ShieldBan size={14} />
                            <span>封禁</span>
                          </button>
                        )}
                        <button
                          type="button"
                          disabled={isActioning || isDeleted}
                          onClick={() => void handleUpdateMetadata(space)}
                        >
                          <PencilLine size={14} />
                          <span>元数据</span>
                        </button>
                        <button
                          type="button"
                          className="danger"
                          disabled={isActioning || isDeleted}
                          onClick={() => void handleDelete(space)}
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
          当前第 {spacesState.pagination.page} / {totalPages} 页，共 {spacesState.pagination.total} 条
        </p>
        <div className="admin-spaces-pagination">
          <button
            type="button"
            onClick={() => setPage((value) => Math.max(1, value - 1))}
            disabled={loading || spacesState.pagination.page <= 1}
          >
            上一页
          </button>
          <button
            type="button"
            onClick={() => setPage((value) => Math.min(totalPages, value + 1))}
            disabled={loading || spacesState.pagination.page >= totalPages}
          >
            下一页
          </button>
        </div>
      </footer>
    </section>
  );
}
