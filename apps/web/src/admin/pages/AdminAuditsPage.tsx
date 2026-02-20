import { LoaderCircle, RefreshCw, Search } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { Card, CardContent } from "../../components/ui/card";
import { Input } from "../../components/ui/input";
import { Select } from "../../components/ui/select";
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

function renderModuleBadgeClass(value: AdminAuditModule): string {
  switch (value) {
    case "user":
      return "border-sky-200 bg-sky-50 text-sky-700";
    case "space":
      return "border-indigo-200 bg-indigo-50 text-indigo-700";
    case "document":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
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
    <section aria-label="审计日志查询">
      <TopToast
        open={toastState.open}
        message={toastState.message}
        variant={toastState.variant}
        triggerKey={toastState.triggerKey}
        durationMs={2800}
        onClose={closeToast}
      />
      <Card className="border-slate-200/80 shadow-sm">
        <CardContent className="space-y-4 p-5">
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
              <Select value={moduleFilter} onChange={(event) => setModuleFilter(event.target.value as typeof moduleFilter)} disabled={loading}>
                <option value="">全部模块</option>
                <option value="user">user</option>
                <option value="space">space</option>
                <option value="document">document</option>
                <option value="theme">theme</option>
                <option value="system_config">system_config</option>
              </Select>
            </label>

            <label className="space-y-1.5">
              <span className="text-xs font-semibold tracking-wide text-slate-600">动作</span>
              <Select value={actionFilter} onChange={(event) => setActionFilter(event.target.value as typeof actionFilter)} disabled={loading}>
                <option value="">全部动作</option>
                <option value="create">create</option>
                <option value="update">update</option>
                <option value="delete">delete</option>
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

            <div className="flex flex-wrap items-end gap-2 xl:col-span-3">
              <Button type="submit" size="sm" disabled={loading}>
                <Search size={14} />
                <span>查询</span>
              </Button>
              <Button type="button" size="sm" variant="outline" disabled={loading} onClick={handleReset}>
                重置
              </Button>
              <Button type="button" size="sm" variant="outline" disabled={loading} onClick={() => void loadAudits()}>
                <RefreshCw size={14} />
                <span>刷新</span>
              </Button>
            </div>
          </form>

          <div className="overflow-hidden rounded-lg border border-slate-200">
            <div className="max-h-[56vh] overflow-auto">
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
                      const isExpanded = expandedAuditID === item.id;
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
                            <Button
                              type="button"
                              size="sm"
                              variant="outline"
                              onClick={() => setExpandedAuditID(isExpanded ? null : item.id)}
                            >
                              {isExpanded ? "收起" : "查看"}
                            </Button>
                            {isExpanded ? (
                              <pre className="mt-2 max-h-44 overflow-auto rounded border border-slate-200 bg-slate-50 p-2 text-xs leading-relaxed text-slate-700">
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
          </div>

          <footer className="flex flex-wrap items-center justify-between gap-3">
            <p className="text-xs text-slate-600">
              共 {auditsState.pagination.total} 条，当前第 {auditsState.pagination.page} / {totalPages} 页
            </p>
            <div className="flex gap-2">
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={loading || page <= 1}
                onClick={() => setPage((previousPage) => Math.max(1, previousPage - 1))}
              >
                上一页
              </Button>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={loading || page >= totalPages}
                onClick={() => setPage((previousPage) => Math.min(totalPages, previousPage + 1))}
              >
                下一页
              </Button>
            </div>
          </footer>
        </CardContent>
      </Card>
    </section>
  );
}
