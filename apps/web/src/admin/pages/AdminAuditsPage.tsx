import { Copy, LoaderCircle, RefreshCw, Search } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEventHandler } from "react";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "../../components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { showToast } from "../../components/ui/toast";
import {
  type AdminAuditAction,
  type AdminAuditListResult,
  type AdminAuditLog,
  type AdminAuditModule,
  type DataGateway
} from "../../data-access";
import { AdminPageCard, AdminPaginationFooter, AdminTableContainer, AdminToolbarActions } from "../components/AdminPageLayout";
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
    case "document_template":
      return "模板管理";
    case "document_template_scene":
      return "模板场景";
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

function renderModuleBadgeClass(value: AdminAuditModule): string {
  switch (value) {
    case "user":
      return "border-sky-200 bg-sky-50 text-sky-700";
    case "space":
      return "border-indigo-200 bg-indigo-50 text-indigo-700";
    case "document":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "document_template":
      return "border-cyan-200 bg-cyan-50 text-cyan-700";
    case "document_template_scene":
      return "border-teal-200 bg-teal-50 text-teal-700";
    case "theme":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "system_config":
      return "border-violet-200 bg-violet-50 text-violet-700";
    default:
      return "border-slate-200 bg-slate-100 text-slate-700";
  }
}

function renderActionBadgeClass(value: AdminAuditAction): string {
  switch (value) {
    case "create":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "update":
      return "border-sky-200 bg-sky-50 text-sky-700";
    case "delete":
      return "border-rose-200 bg-rose-50 text-rose-700";
    default:
      return "border-slate-200 bg-slate-100 text-slate-700";
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

  const [auditsState, setAuditsState] = useState<AdminAuditsState>(() => emptyAuditsState());
  const [loading, setLoading] = useState(false);

  const openToast = useCallback((message: string, variant: "success" | "info" | "error" = "error") => {
    showToast(message, variant);
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

  const totalPages = useMemo(() => {
    const total = auditsState.pagination.total;
    const pageSize = auditsState.pagination.pageSize || DEFAULT_PAGE_SIZE;
    return Math.max(1, Math.ceil(total / pageSize));
  }, [auditsState.pagination.pageSize, auditsState.pagination.total]);

  const handleSearchSubmit = useCallback<FormEventHandler<HTMLFormElement>>(
    (event) => {
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

    setPage(1);
  }, []);

  const handleCopyAuditDetail = useCallback(
    async (value: string) => {
      const detailText = value.trim();
      if (!detailText) {
        openToast("详情内容为空", "info");
        return;
      }
      try {
        if (navigator.clipboard?.writeText) {
          await navigator.clipboard.writeText(detailText);
        } else {
          const input = document.createElement("input");
          input.value = detailText;
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
        openToast("审计详情已复制", "success");
      } catch (error) {
        openToast(`复制失败：${formatError(error)}`);
      }
    },
    [openToast]
  );

  return (
    <section aria-label="审计日志查询">
      <AdminPageCard>
          <form className="grid gap-3 xl:grid-cols-4" onSubmit={handleSearchSubmit}>
            <label className="space-y-1.5 xl:col-span-2">
              <span className="text-xs font-semibold tracking-wide text-slate-600">关键字</span>
              <Input
                value={keywordInput}
                onChange={(event) => setKeywordInput(event.target.value)}
                placeholder="摘要/目标 ID/操作者"
                disabled={loading}
              />
            </label>

            <label className="space-y-1.5">
              <span className="text-xs font-semibold tracking-wide text-slate-600">模块</span>
              <Select
                value={moduleFilter || "default"}
                onValueChange={(value) => setModuleFilter((value === "default" ? "" : value) as typeof moduleFilter)}
                disabled={loading}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="default">全部模块</SelectItem>
                  <SelectItem value="user">user</SelectItem>
                  <SelectItem value="space">space</SelectItem>
                  <SelectItem value="document">document</SelectItem>
                  <SelectItem value="document_template">document_template</SelectItem>
                  <SelectItem value="document_template_scene">document_template_scene</SelectItem>
                  <SelectItem value="theme">theme</SelectItem>
                  <SelectItem value="system_config">system_config</SelectItem>
                </SelectContent>
              </Select>
            </label>

            <label className="space-y-1.5">
              <span className="text-xs font-semibold tracking-wide text-slate-600">动作</span>
              <Select
                value={actionFilter || "default"}
                onValueChange={(value) => setActionFilter((value === "default" ? "" : value) as typeof actionFilter)}
                disabled={loading}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="default">全部动作</SelectItem>
                  <SelectItem value="create">create</SelectItem>
                  <SelectItem value="update">update</SelectItem>
                  <SelectItem value="delete">delete</SelectItem>
                </SelectContent>
              </Select>
            </label>

            <label className="space-y-1.5">
              <span className="text-xs font-semibold tracking-wide text-slate-600">操作者 ID</span>
              <Input value={actorUserIDInput} onChange={(event) => setActorUserIDInput(event.target.value)} disabled={loading} />
            </label>

            <label className="space-y-1.5">
              <span className="text-xs font-semibold tracking-wide text-slate-600">目标 ID</span>
              <Input value={targetIDInput} onChange={(event) => setTargetIDInput(event.target.value)} disabled={loading} />
            </label>

            <label className="space-y-1.5">
              <span className="text-xs font-semibold tracking-wide text-slate-600">请求 ID</span>
              <Input value={requestIDInput} onChange={(event) => setRequestIDInput(event.target.value)} disabled={loading} />
            </label>

            <label className="space-y-1.5">
              <span className="text-xs font-semibold tracking-wide text-slate-600">开始时间</span>
              <Input
                type="datetime-local"
                value={fromInput}
                onChange={(event) => setFromInput(event.target.value)}
                disabled={loading}
              />
            </label>

            <label className="space-y-1.5">
              <span className="text-xs font-semibold tracking-wide text-slate-600">结束时间</span>
              <Input
                type="datetime-local"
                value={toInput}
                onChange={(event) => setToInput(event.target.value)}
                disabled={loading}
              />
            </label>

            <AdminToolbarActions className="xl:col-span-3">
                <Button type="submit" disabled={loading}>
                  <Search size={14} />
                  <span>查询</span>
                </Button>
                <Button type="button" variant="outline" disabled={loading} onClick={handleReset}>
                  重置
                </Button>
                <Button type="button" variant="outline" disabled={loading} onClick={() => void loadAudits()}>
                  <RefreshCw size={14} />
                  <span>刷新</span>
                </Button>
            </AdminToolbarActions>
          </form>

          <AdminTableContainer>
              <table className="w-full min-w-[1280px] border-collapse text-left text-sm">
                <thead className="sticky top-0 z-10 bg-slate-50/95 backdrop-blur">
                  <tr className="text-xs uppercase tracking-wide text-slate-600">
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">时间</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">模块/动作</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">目标</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">操作者</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">摘要</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">请求 ID</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">详情</th>
                  </tr>
                </thead>
                <tbody>
                  {loading ? (
                    <tr>
                      <td colSpan={7} className="px-3 py-12">
                        <div className="flex items-center justify-center gap-2 text-sm text-slate-500">
                          <LoaderCircle size={15} className="animate-spin" />
                          <span>正在加载审计日志...</span>
                        </div>
                      </td>
                    </tr>
                  ) : auditsState.items.length === 0 ? (
                    <tr>
                      <td colSpan={7} className="px-3 py-12 text-center text-sm text-slate-500">
                        暂无审计日志
                      </td>
                    </tr>
                  ) : (
                    auditsState.items.map((item) => {
                      const detailText = JSON.stringify(item.detail ?? {}, null, 2);
                      return (
                        <tr key={item.id} className="border-b border-slate-100 align-top text-slate-700">
                          <td className="px-3 py-3">
                            <div className="grid gap-1">
                              <span className="text-xs text-slate-600">{formatDateTime(item.createdAt)}</span>
                              <code className="w-fit rounded border border-slate-200 bg-slate-100 px-1.5 py-0.5 text-xs text-slate-600">
                                #{item.id}
                              </code>
                            </div>
                          </td>
                          <td className="px-3 py-3">
                            <div className="flex flex-wrap items-center gap-2">
                              <Badge variant="outline" className={renderModuleBadgeClass(item.module)}>
                                {renderModuleLabel(item.module)}
                              </Badge>
                              <Badge variant="outline" className={renderActionBadgeClass(item.action)}>
                                {renderActionLabel(item.action)}
                              </Badge>
                            </div>
                          </td>
                          <td className="px-3 py-3">
                            <div className="grid gap-1">
                              <strong className="text-xs font-semibold text-slate-900">{item.targetType || "-"}</strong>
                              <code className="w-fit rounded border border-sky-200 bg-sky-50 px-1.5 py-0.5 text-xs text-sky-700">
                                {item.targetId || "-"}
                              </code>
                            </div>
                          </td>
                          <td className="px-3 py-3">
                            <div className="grid gap-1 text-xs text-slate-600">
                              <strong className="font-semibold text-slate-800">{formatActorIdentity(item)}</strong>
                              <span>{item.actorUserId || "-"}</span>
                            </div>
                          </td>
                          <td className="px-3 py-3 text-xs text-slate-600">{item.summary || "-"}</td>
                          <td className="px-3 py-3">
                            <code className="rounded border border-slate-200 bg-slate-100 px-1.5 py-0.5 text-xs text-slate-600">
                              {item.requestId || "-"}
                            </code>
                          </td>
                          <td className="px-3 py-3">
                            <Popover>
                              <PopoverTrigger asChild>
                                <Button type="button" size="sm" variant="outline">
                                  查看
                                </Button>
                              </PopoverTrigger>
                              <PopoverContent side="left" align="start" className="w-[min(80vw,34rem)] p-0">
                                <div className="flex items-center justify-between gap-2 border-b border-slate-200 bg-slate-50 px-3 py-2">
                                  <span className="text-xs font-semibold tracking-wide text-slate-600">审计详情</span>
                                  <Button
                                    type="button"
                                    size="sm"
                                    variant="ghost"
                                    className="h-7 px-2 text-xs text-slate-600 hover:text-slate-900"
                                    onClick={() => void handleCopyAuditDetail(detailText)}
                                    aria-label="复制审计详情"
                                    title="复制审计详情"
                                  >
                                    <Copy size={13} />
                                    <span>复制</span>
                                  </Button>
                                </div>
                                <pre className="max-h-72 overflow-auto px-3 py-2 text-xs leading-relaxed text-slate-700">
                                  {detailText}
                                </pre>
                              </PopoverContent>
                            </Popover>
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
              共 {auditsState.pagination.total} 条，当前第 {auditsState.pagination.page} / {totalPages} 页
              </>
            }
            previousDisabled={loading || page <= 1}
            nextDisabled={loading || page >= totalPages}
            onPrevious={() => setPage((previousPage) => Math.max(1, previousPage - 1))}
            onNext={() => setPage((previousPage) => Math.min(totalPages, previousPage + 1))}
          />
      </AdminPageCard>
    </section>
  );
}
