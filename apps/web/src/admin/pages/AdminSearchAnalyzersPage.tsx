import { LoaderCircle, Pencil, RefreshCw, Search, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEventHandler } from "react";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { Textarea } from "../../components/ui/textarea";
import { showToast } from "../../components/ui/toast";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "../../components/ui/tooltip";
import {
  type AdminSearchAnalyzerAnalyzePreviewResult,
  type AdminSearchAnalyzerDictEntry,
  type AdminSearchAnalyzerDictListResult,
  type AdminSearchAnalyzerMode,
  type AdminSearchAnalyzerName,
  type AdminSearchAnalyzerRecord,
  type DataGateway
} from "../../data-access";
import { formatError } from "../../editor/status-utils";
import {
  AdminPageCard,
  AdminPaginationFooter,
  AdminTableContainer,
  AdminToolbarActions
} from "../components/AdminPageLayout";

const DEFAULT_PAGE_SIZE = 20;
const JIEBA_WEIGHT_PRESETS: Array<{ value: number; label: string; description: string }> = [
  { value: 10, label: "10（轻）", description: "普通业务词，先做基础补充。" },
  { value: 50, label: "50（中）", description: "常见术语，推荐默认档。" },
  { value: 200, label: "200（高）", description: "品牌词/核心专有名词。" },
  { value: 500, label: "500（极高）", description: "非常确定且应优先保留的短语。" }
];
const JIEBA_POS_TAG_OPTIONS: Array<{ value: string; label: string; description: string }> = [
  { value: "n", label: "n", description: "普通名词" },
  { value: "nr", label: "nr", description: "人名" },
  { value: "ns", label: "ns", description: "地名" },
  { value: "nt", label: "nt", description: "机构团体" },
  { value: "nz", label: "nz", description: "其他专有名词" },
  { value: "v", label: "v", description: "动词" },
  { value: "vn", label: "vn", description: "名动词" },
  { value: "a", label: "a", description: "形容词" },
  { value: "eng", label: "eng", description: "英文词" }
];

interface AdminSearchAnalyzersPageProps {
  dataGateway: DataGateway;
}

interface DictState {
  items: AdminSearchAnalyzerDictEntry[];
  pagination: AdminSearchAnalyzerDictListResult["pagination"];
}

function emptyDictState(): DictState {
  return {
    items: [],
    pagination: {
      page: 1,
      pageSize: DEFAULT_PAGE_SIZE,
      total: 0
    }
  };
}

function formatDateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return date.toLocaleString("zh-CN", { hour12: false });
}

function parseOptionalWeight(rawValue: string): { value?: number; error?: string } {
  const normalized = rawValue.trim();
  if (!normalized) {
    return {};
  }
  const parsed = Number(normalized);
  if (!Number.isFinite(parsed) || parsed <= 0 || !Number.isInteger(parsed)) {
    return { error: "词频权重必须是正整数" };
  }
  return { value: Math.trunc(parsed) };
}

function normalizeAnalyzerName(value: string): string {
  return value.trim().toLowerCase();
}

function renderDictStatusBadge(status: string): { label: string; className: string } {
  if (status === "active") {
    return {
      label: "生效",
      className: "border-emerald-200 bg-emerald-50 text-emerald-700"
    };
  }
  return {
    label: "已删除",
    className: "border-slate-200 bg-slate-100 text-slate-600"
  };
}

interface AnalyzerCapabilityBadge {
  key: "user_dict" | "hot_reload" | "phrase_hint" | "stopwords" | "synonyms";
  label: string;
  tooltip: string;
  enabled: boolean;
}

function renderCapabilityBadges(analyzer: AdminSearchAnalyzerRecord | null): AnalyzerCapabilityBadge[] {
  if (!analyzer) {
    return [];
  }
  return [
    {
      key: "user_dict",
      label: "用户词典",
      tooltip: "user_dict：支持自定义词典词条，可手动维护行业词、品牌词等并参与分词。",
      enabled: analyzer.supportsUserDict
    },
    {
      key: "hot_reload",
      label: "热重载",
      tooltip: "hot_reload：支持词典热重载，更新词典后无需重启服务即可生效。",
      enabled: analyzer.supportsHotReload
    },
    {
      key: "phrase_hint",
      label: "短语提示",
      tooltip: "phrase_hint：支持短语级提示能力，用于提升固定短语/术语的分词与匹配效果。",
      enabled: analyzer.supportsPhraseHint
    },
    {
      key: "stopwords",
      label: "停用词",
      tooltip: "stopwords：支持停用词过滤，可忽略“的、了、and”等低信息量词。",
      enabled: analyzer.supportsStopwords
    },
    {
      key: "synonyms",
      label: "同义词",
      tooltip: "synonyms：支持同义词扩展（L2 能力），可将近义词映射到同一检索语义。",
      enabled: analyzer.supportsSynonyms
    }
  ];
}

export function AdminSearchAnalyzersPage({ dataGateway }: AdminSearchAnalyzersPageProps) {
  const [analyzers, setAnalyzers] = useState<AdminSearchAnalyzerRecord[]>([]);
  const managedAnalyzer: AdminSearchAnalyzerName = "jieba";
  const [loadingAnalyzers, setLoadingAnalyzers] = useState(false);
  const [loadingDict, setLoadingDict] = useState(false);
  const [statusFilter, setStatusFilter] = useState<"all" | "active" | "deleted">("active");
  const [page, setPage] = useState(1);
  const [dictState, setDictState] = useState<DictState>(() => emptyDictState());

  const [termInput, setTermInput] = useState("");
  const [weightInput, setWeightInput] = useState("");
  const [tagInput, setTagInput] = useState("");
  const [creating, setCreating] = useState(false);

  const [editingEntryID, setEditingEntryID] = useState<number | null>(null);
  const [updating, setUpdating] = useState(false);
  const [deletingEntryID, setDeletingEntryID] = useState<number | null>(null);

  const [reloading, setReloading] = useState(false);

  const [previewText, setPreviewText] = useState("");
  const [previewMode, setPreviewMode] = useState<AdminSearchAnalyzerMode>("query");
  const [previewLanguage, setPreviewLanguage] = useState("zh-CN");
  const [previewSpaceID, setPreviewSpaceID] = useState("");
  const [previewing, setPreviewing] = useState(false);
  const [previewResult, setPreviewResult] = useState<AdminSearchAnalyzerAnalyzePreviewResult | null>(null);

  const selectedAnalyzer = useMemo(
    () => analyzers.find((item) => normalizeAnalyzerName(item.name) === managedAnalyzer) ?? null,
    [managedAnalyzer, analyzers]
  );
  const runtimeActiveAnalyzer = useMemo(
    () => analyzers.find((item) => item.active) ?? null,
    [analyzers]
  );

  const capabilityBadges = useMemo(() => renderCapabilityBadges(selectedAnalyzer), [selectedAnalyzer]);
  const supportedCapabilityBadges = useMemo(
    () => capabilityBadges.filter((item) => item.enabled),
    [capabilityBadges]
  );
  const unsupportedCapabilityBadges = useMemo(
    () => capabilityBadges.filter((item) => !item.enabled),
    [capabilityBadges]
  );
  const isManagedAnalyzerActive = useMemo(() => {
    if (!runtimeActiveAnalyzer) {
      return false;
    }
    return normalizeAnalyzerName(runtimeActiveAnalyzer.name) === managedAnalyzer;
  }, [managedAnalyzer, runtimeActiveAnalyzer]);

  const totalPages = useMemo(() => {
    const total = dictState.pagination.total;
    const pageSize = dictState.pagination.pageSize || DEFAULT_PAGE_SIZE;
    return Math.max(1, Math.ceil(total / pageSize));
  }, [dictState.pagination.pageSize, dictState.pagination.total]);

  const openToast = useCallback((message: string, variant: "success" | "info" | "error" = "error") => {
    showToast(message, variant);
  }, []);

  const resetEditForm = useCallback(() => {
    setEditingEntryID(null);
    setTermInput("");
    setWeightInput("");
    setTagInput("");
  }, []);

  const loadAnalyzers = useCallback(async () => {
    setLoadingAnalyzers(true);
    try {
      const payload = await dataGateway.admin.listSearchAnalyzers();
      setAnalyzers(payload);
      if (!payload.some((item) => normalizeAnalyzerName(item.name) === managedAnalyzer)) {
        openToast("当前未发现 jieba 分词器配置，页面将继续按 jieba 目标调用接口。", "info");
      }
    } catch (error) {
      openToast(`加载分词器状态失败：${formatError(error)}`);
      setAnalyzers([]);
    } finally {
      setLoadingAnalyzers(false);
    }
  }, [dataGateway.admin, managedAnalyzer, openToast]);

  const loadDictEntries = useCallback(async () => {
    setLoadingDict(true);
    try {
      const payload = await dataGateway.admin.listSearchAnalyzerDictEntries({
        analyzer: managedAnalyzer,
        status: statusFilter,
        page,
        pageSize: DEFAULT_PAGE_SIZE
      });
      setDictState(payload);
    } catch (error) {
      openToast(`加载词典列表失败：${formatError(error)}`);
      setDictState(emptyDictState());
    } finally {
      setLoadingDict(false);
    }
  }, [dataGateway.admin, managedAnalyzer, openToast, page, statusFilter]);

  useEffect(() => {
    void loadAnalyzers();
  }, [loadAnalyzers]);

  useEffect(() => {
    void loadDictEntries();
  }, [loadDictEntries]);

  const handleRefresh = useCallback(async () => {
    await Promise.all([loadAnalyzers(), loadDictEntries()]);
  }, [loadAnalyzers, loadDictEntries]);

  const handleCreate: FormEventHandler<HTMLFormElement> = useCallback(
    async (event) => {
      event.preventDefault();
      const term = termInput.trim();
      if (!term) {
        openToast("词条不能为空");
        return;
      }
      const parsedWeight = parseOptionalWeight(weightInput);
      if (parsedWeight.error) {
        openToast(parsedWeight.error);
        return;
      }

      setCreating(true);
      try {
        const created = await dataGateway.admin.createSearchAnalyzerDictEntry({
          analyzer: managedAnalyzer,
          term,
          weight: parsedWeight.value,
          tag: tagInput.trim() || undefined
        });
        openToast(`词条已新增：${created.term}`, "success");
        resetEditForm();
        setPage(1);
        if (page === 1) {
          await loadDictEntries();
        }
      } catch (error) {
        openToast(`新增词条失败：${formatError(error)}`);
      } finally {
        setCreating(false);
      }
    },
    [dataGateway.admin, loadDictEntries, managedAnalyzer, openToast, page, resetEditForm, tagInput, termInput, weightInput]
  );

  const handleStartEdit = useCallback((entry: AdminSearchAnalyzerDictEntry) => {
    setEditingEntryID(entry.id);
    setTermInput(entry.term);
    setWeightInput(typeof entry.weight === "number" && entry.weight > 0 ? String(entry.weight) : "");
    setTagInput(entry.tag ?? "");
  }, []);

  const handleUpdate = useCallback(async () => {
    if (!editingEntryID) {
      openToast("请先选择要编辑的词条");
      return;
    }
    const term = termInput.trim();
    if (!term) {
      openToast("词条不能为空");
      return;
    }
    const parsedWeight = parseOptionalWeight(weightInput);
    if (parsedWeight.error) {
      openToast(parsedWeight.error);
      return;
    }

    setUpdating(true);
    try {
      const updated = await dataGateway.admin.updateSearchAnalyzerDictEntry({
        analyzer: managedAnalyzer,
        entryId: editingEntryID,
        term,
        weight: parsedWeight.value,
        tag: tagInput.trim() || undefined
      });
      openToast(`词条已更新：${updated.term}`, "success");
      resetEditForm();
      await loadDictEntries();
    } catch (error) {
      openToast(`更新词条失败：${formatError(error)}`);
    } finally {
      setUpdating(false);
    }
  }, [
    dataGateway.admin,
    editingEntryID,
    loadDictEntries,
    managedAnalyzer,
    openToast,
    resetEditForm,
    tagInput,
    termInput,
    weightInput
  ]);

  const handleDelete = useCallback(
    async (entry: AdminSearchAnalyzerDictEntry) => {
      if (!window.confirm(`确认删除词条「${entry.term}」吗？`)) {
        return;
      }
      setDeletingEntryID(entry.id);
      try {
        await dataGateway.admin.deleteSearchAnalyzerDictEntry({
          analyzer: managedAnalyzer,
          entryId: entry.id
        });
        openToast(`词条已删除：${entry.term}`, "success");
        await loadDictEntries();
      } catch (error) {
        openToast(`删除词条失败：${formatError(error)}`);
      } finally {
        setDeletingEntryID(null);
      }
    },
    [dataGateway.admin, loadDictEntries, managedAnalyzer, openToast]
  );

  const handleReload = useCallback(async () => {
    setReloading(true);
    try {
      const result = await dataGateway.admin.reloadSearchAnalyzer({ analyzer: managedAnalyzer });
      openToast(`分词器已重载：${result.analyzer} / ${result.dictVersion}`, "success");
      await loadAnalyzers();
    } catch (error) {
      openToast(`重载分词器失败：${formatError(error)}`);
    } finally {
      setReloading(false);
    }
  }, [dataGateway.admin, loadAnalyzers, managedAnalyzer, openToast]);

  const handleAnalyzePreview = useCallback(async () => {
    setPreviewing(true);
    try {
      const result = await dataGateway.admin.analyzeSearchAnalyzerPreview({
        analyzer: managedAnalyzer,
        text: previewText,
        mode: previewMode,
        language: previewLanguage.trim() || undefined,
        spaceId: previewSpaceID.trim() || undefined
      });
      setPreviewResult(result);
      openToast(`分词完成，共 ${result.tokenCount} 个 token`, "success");
    } catch (error) {
      openToast(`分词预览失败：${formatError(error)}`);
    } finally {
      setPreviewing(false);
    }
  }, [dataGateway.admin, managedAnalyzer, openToast, previewLanguage, previewMode, previewSpaceID, previewText]);

  return (
    <section aria-label="分词治理">
      <AdminPageCard contentClassName="space-y-5 px-2 pb-5 pt-2 lg:px-0">
        <div className="rounded-sm border border-slate-200/80 bg-slate-50 p-3">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="space-y-2">
              <p className="text-sm font-semibold text-slate-900">分词器状态与平台能力</p>
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant="outline" className="border-slate-200 bg-white text-slate-700">
                  治理对象：{selectedAnalyzer?.name ?? managedAnalyzer}
                </Badge>
                <Badge
                  variant="outline"
                  className={runtimeActiveAnalyzer
                    ? "border-emerald-200 bg-emerald-50 text-emerald-700"
                    : "border-slate-200 bg-slate-100 text-slate-600"}
                >
                  系统活跃：{runtimeActiveAnalyzer?.name ?? "未知"}
                </Badge>
                <Badge
                  variant="outline"
                  className={isManagedAnalyzerActive
                    ? "border-emerald-200 bg-emerald-50 text-emerald-700"
                    : "border-amber-200 bg-amber-50 text-amber-700"}
                >
                  治理对象状态：{isManagedAnalyzerActive ? "已活跃" : "未活跃"}
                </Badge>
                <Badge variant="outline" className="border-slate-200 bg-white text-slate-700">
                  词典版本：{selectedAnalyzer?.dictVersion ?? "default"}
                </Badge>
              </div>
              <p className="text-xs text-slate-500">
                下方标签是 PlainDoc 平台定义的分词能力位，不是随机标签。蓝色表示治理对象已支持，灰色表示当前未支持或规划中。
              </p>
              {!isManagedAnalyzerActive && runtimeActiveAnalyzer ? (
                <p className="rounded-sm border border-amber-200 bg-amber-50 px-2 py-1.5 text-xs text-amber-700">
                  当前系统活跃分词器是 <code>{runtimeActiveAnalyzer.name}</code>，不是 <code>{managedAnalyzer}</code>。
                  此页面的词典治理变更不会影响线上检索，需要先在“系统配置 / 全文检索”切换活跃分词器。
                </p>
              ) : null}
              <TooltipProvider delayDuration={120}>
                <div className="space-y-2">
                  <div className="flex flex-wrap items-center gap-1.5">
                    <span className="text-xs font-semibold tracking-wide text-slate-600">已支持</span>
                    {supportedCapabilityBadges.map((item) => (
                      <Tooltip key={item.key}>
                        <TooltipTrigger asChild>
                          <Badge variant="outline" className="cursor-help border-sky-200 bg-sky-50 text-sky-700">
                            {item.label}
                          </Badge>
                        </TooltipTrigger>
                        <TooltipContent side="top" className="max-w-[320px] whitespace-pre-wrap break-words">
                          {item.tooltip}
                        </TooltipContent>
                      </Tooltip>
                    ))}
                    {supportedCapabilityBadges.length === 0 ? (
                      <span className="text-xs text-slate-500">暂无</span>
                    ) : null}
                  </div>
                  <div className="flex flex-wrap items-center gap-1.5">
                    <span className="text-xs font-semibold tracking-wide text-slate-600">未支持 / 规划中</span>
                    {unsupportedCapabilityBadges.map((item) => (
                      <Tooltip key={item.key}>
                        <TooltipTrigger asChild>
                          <Badge variant="outline" className="cursor-help border-slate-200 bg-slate-100 text-slate-500">
                            {item.label}
                          </Badge>
                        </TooltipTrigger>
                        <TooltipContent side="top" className="max-w-[320px] whitespace-pre-wrap break-words">
                          {item.tooltip}
                        </TooltipContent>
                      </Tooltip>
                    ))}
                    {unsupportedCapabilityBadges.length === 0 ? (
                      <span className="text-xs text-slate-500">暂无</span>
                    ) : null}
                  </div>
                </div>
              </TooltipProvider>
            </div>
            <AdminToolbarActions className="space-y-0">
              <Button type="button" variant="outline" size="sm" disabled={loadingAnalyzers || loadingDict} onClick={handleRefresh}>
                {loadingAnalyzers || loadingDict ? <LoaderCircle className="mr-1 animate-spin" size={14} /> : <RefreshCw className="mr-1" size={14} />}
                刷新
              </Button>
              <Button type="button" variant="outline" size="sm" disabled={reloading || loadingAnalyzers} onClick={() => void handleReload()}>
                {reloading ? <LoaderCircle className="mr-1 animate-spin" size={14} /> : <RefreshCw className="mr-1" size={14} />}
                重载词典
              </Button>
            </AdminToolbarActions>
          </div>
        </div>

        <div className="grid gap-4 xl:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]">
          <div className="space-y-3 rounded-sm border border-slate-200/80 bg-white p-3">
            <div className="space-y-1">
              <h3 className="text-base font-semibold text-slate-900">词典治理</h3>
              <p className="text-xs text-slate-500">
                支持新增、编辑、删除词条；变更后可使用上方“重载词典”使其生效。
              </p>
            </div>

            <form className="grid gap-2 md:grid-cols-[minmax(0,2fr)_130px_130px_auto]" onSubmit={handleCreate}>
              <Input
                value={termInput}
                onChange={(event) => setTermInput(event.target.value)}
                placeholder="词条（必填）"
                disabled={creating || updating}
              />
              <Input
                value={weightInput}
                onChange={(event) => setWeightInput(event.target.value)}
                placeholder="权重（可选）"
                disabled={creating || updating}
              />
              <Input
                value={tagInput}
                onChange={(event) => setTagInput(event.target.value)}
                placeholder="词性（可选）"
                disabled={creating || updating}
              />
              <div className="flex gap-2">
                <Button type="submit" size="sm" disabled={creating || updating}>
                  {creating ? <LoaderCircle className="mr-1 animate-spin" size={14} /> : null}
                  新增
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={!editingEntryID || creating || updating}
                  onClick={() => void handleUpdate()}
                >
                  {updating ? <LoaderCircle className="mr-1 animate-spin" size={14} /> : <Pencil className="mr-1" size={14} />}
                  更新
                </Button>
                <Button type="button" size="sm" variant="ghost" disabled={!editingEntryID || creating || updating} onClick={resetEditForm}>
                  取消
                </Button>
              </div>
            </form>

            <TooltipProvider delayDuration={120}>
              <div className="space-y-2 rounded-sm border border-slate-200 bg-slate-50 p-2.5">
                <p className="text-xs text-slate-600">
                  字段说明：`weight` 是词频权重（正整数，可选），`tag` 是词性标记（可选）。不确定时可以留空。
                </p>
                <div className="flex flex-wrap items-center gap-1.5">
                  <span className="text-xs font-semibold tracking-wide text-slate-600">权重推荐</span>
                  {JIEBA_WEIGHT_PRESETS.map((item) => (
                    <Tooltip key={item.value}>
                      <TooltipTrigger asChild>
                        <button
                          type="button"
                          className="rounded border border-slate-200 bg-white px-2 py-0.5 text-xs text-slate-700 transition hover:bg-slate-100"
                          disabled={creating || updating}
                          onClick={() => setWeightInput(String(item.value))}
                        >
                          {item.label}
                        </button>
                      </TooltipTrigger>
                      <TooltipContent side="top">{item.description}</TooltipContent>
                    </Tooltip>
                  ))}
                  <button
                    type="button"
                    className="rounded border border-slate-200 bg-white px-2 py-0.5 text-xs text-slate-700 transition hover:bg-slate-100"
                    disabled={creating || updating}
                    onClick={() => setWeightInput("")}
                  >
                    留空
                  </button>
                </div>
                <div className="flex flex-wrap items-center gap-1.5">
                  <span className="text-xs font-semibold tracking-wide text-slate-600">词性可选项</span>
                  {JIEBA_POS_TAG_OPTIONS.map((item) => (
                    <Tooltip key={item.value}>
                      <TooltipTrigger asChild>
                        <button
                          type="button"
                          className="rounded border border-slate-200 bg-white px-2 py-0.5 text-xs text-slate-700 transition hover:bg-slate-100"
                          disabled={creating || updating}
                          onClick={() => setTagInput(item.value)}
                        >
                          {item.label}
                        </button>
                      </TooltipTrigger>
                      <TooltipContent side="top">
                        {item.value}：{item.description}
                      </TooltipContent>
                    </Tooltip>
                  ))}
                  <button
                    type="button"
                    className="rounded border border-slate-200 bg-white px-2 py-0.5 text-xs text-slate-700 transition hover:bg-slate-100"
                    disabled={creating || updating}
                    onClick={() => setTagInput("")}
                  >
                    清空
                  </button>
                </div>
                <p className="text-[11px] text-slate-500">
                  当前版本说明：检索主路径主要按 `term` 生效，`weight/tag` 已保留为 jieba 词典兼容字段与后续增强能力预留。
                </p>
              </div>
            </TooltipProvider>

            {editingEntryID ? (
              <p className="text-xs text-amber-700">正在编辑词条 ID #{editingEntryID}，点击“更新”提交。</p>
            ) : null}

            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <span className="text-xs font-semibold tracking-wide text-slate-600">状态筛选</span>
                <Select
                  value={statusFilter}
                  onValueChange={(value) => {
                    setStatusFilter(value as typeof statusFilter);
                    setPage(1);
                  }}
                  disabled={loadingDict}
                >
                  <SelectTrigger className="h-8 w-32">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">全部</SelectItem>
                    <SelectItem value="active">生效中</SelectItem>
                    <SelectItem value="deleted">已删除</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <p className="text-xs text-slate-500">
                共 {dictState.pagination.total} 条，页码 {dictState.pagination.page}/{totalPages}
              </p>
            </div>

            <AdminTableContainer>
              <table className="min-w-[760px] w-full border-collapse text-left text-sm">
                <thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-600">
                  <tr>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">词条</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">权重</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">标签</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">状态</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold">更新时间</th>
                    <th className="border-b border-slate-200 px-3 py-2 font-semibold text-right">操作</th>
                  </tr>
                </thead>
                <tbody>
                  {loadingDict ? (
                    <tr>
                      <td colSpan={6} className="px-3 py-8 text-center text-sm text-slate-500">
                        <span className="inline-flex items-center gap-2">
                          <LoaderCircle size={16} className="animate-spin" />
                          正在加载词典...
                        </span>
                      </td>
                    </tr>
                  ) : dictState.items.length === 0 ? (
                    <tr>
                      <td colSpan={6} className="px-3 py-8 text-center text-sm text-slate-500">
                        当前筛选条件下暂无词条
                      </td>
                    </tr>
                  ) : (
                    dictState.items.map((entry) => {
                      const badge = renderDictStatusBadge(entry.status);
                      return (
                        <tr key={entry.id} className="border-b border-slate-100 text-slate-700">
                          <td className="px-3 py-2.5 font-medium text-slate-900">{entry.term}</td>
                          <td className="px-3 py-2.5 text-xs text-slate-600">{entry.weight ?? "-"}</td>
                          <td className="px-3 py-2.5 text-xs text-slate-600">{entry.tag || "-"}</td>
                          <td className="px-3 py-2.5">
                            <Badge variant="outline" className={badge.className}>
                              {badge.label}
                            </Badge>
                          </td>
                          <td className="px-3 py-2.5 text-xs text-slate-600">{formatDateTime(entry.updatedAt)}</td>
                          <td className="px-3 py-2.5">
                            <div className="flex justify-end gap-2">
                              <Button
                                type="button"
                                size="sm"
                                variant="outline"
                                disabled={creating || updating || deletingEntryID !== null}
                                onClick={() => handleStartEdit(entry)}
                              >
                                <Pencil className="mr-1" size={14} />
                                编辑
                              </Button>
                              <Button
                                type="button"
                                size="sm"
                                variant="outline"
                                className="border-rose-200 text-rose-600 hover:bg-rose-50"
                                disabled={deletingEntryID === entry.id || creating || updating}
                                onClick={() => void handleDelete(entry)}
                              >
                                {deletingEntryID === entry.id ? (
                                  <LoaderCircle className="mr-1 animate-spin" size={14} />
                                ) : (
                                  <Trash2 className="mr-1" size={14} />
                                )}
                                删除
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
              summary={`共 ${dictState.pagination.total} 条记录`}
              previousDisabled={loadingDict || page <= 1}
              nextDisabled={loadingDict || page >= totalPages}
              onPrevious={() => setPage((previous) => Math.max(1, previous - 1))}
              onNext={() => setPage((previous) => Math.min(totalPages, previous + 1))}
            />
          </div>

          <div className="space-y-3 rounded-sm border border-slate-200/80 bg-white p-3">
            <div className="space-y-1">
              <h3 className="text-base font-semibold text-slate-900">分词预览</h3>
              <p className="text-xs text-slate-500">
                调用 `analyze-preview` 实时查看分词结果，便于验证词典效果。
              </p>
            </div>

            <div className="grid gap-2 sm:grid-cols-2">
              <label className="space-y-1.5">
                <span className="text-xs font-semibold tracking-wide text-slate-600">模式</span>
                <Select
                  value={previewMode}
                  onValueChange={(value) => setPreviewMode(value as AdminSearchAnalyzerMode)}
                  disabled={previewing}
                >
                  <SelectTrigger className="h-8">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="query">query</SelectItem>
                    <SelectItem value="index">index</SelectItem>
                  </SelectContent>
                </Select>
              </label>
              <label className="space-y-1.5">
                <span className="text-xs font-semibold tracking-wide text-slate-600">language</span>
                <Input
                  value={previewLanguage}
                  onChange={(event) => setPreviewLanguage(event.target.value)}
                  placeholder="zh-CN"
                  disabled={previewing}
                />
              </label>
            </div>

            <label className="space-y-1.5">
              <span className="text-xs font-semibold tracking-wide text-slate-600">spaceId（可选）</span>
              <Input
                value={previewSpaceID}
                onChange={(event) => setPreviewSpaceID(event.target.value)}
                placeholder="space-id"
                disabled={previewing}
              />
            </label>

            <label className="space-y-1.5">
              <span className="text-xs font-semibold tracking-wide text-slate-600">输入文本</span>
              <Textarea
                value={previewText}
                onChange={(event) => setPreviewText(event.target.value)}
                placeholder="输入 Markdown 文本，代码块/公式/mermaid 会在分词前清洗。"
                className="min-h-[140px]"
                disabled={previewing}
              />
            </label>

            <Button type="button" size="sm" className="w-full" disabled={previewing} onClick={() => void handleAnalyzePreview()}>
              {previewing ? <LoaderCircle className="mr-1 animate-spin" size={14} /> : <Search className="mr-1" size={14} />}
              开始分词
            </Button>

            {previewResult ? (
              <div className="space-y-2 rounded-sm border border-slate-200 bg-slate-50 p-3">
                <p className="text-xs text-slate-600">
                  analyzer: <span className="font-medium text-slate-800">{previewResult.analyzer}</span> / mode:{" "}
                  <span className="font-medium text-slate-800">{previewResult.mode}</span> / dictVersion:{" "}
                  <span className="font-medium text-slate-800">{previewResult.dictVersion}</span>
                </p>
                <p className="text-xs text-slate-600">tokenCount: {previewResult.tokenCount}</p>
                <div className="flex flex-wrap gap-1.5">
                  {previewResult.tokens.length === 0 ? (
                    <span className="text-xs text-slate-500">无 token 输出</span>
                  ) : (
                    previewResult.tokens.map((token, index) => (
                      <Badge key={`${token}-${index}`} variant="outline" className="border-slate-200 bg-white text-slate-700">
                        {token}
                      </Badge>
                    ))
                  )}
                </div>
                <div className="rounded-sm border border-slate-200 bg-white p-2">
                  <p className="mb-1 text-xs font-semibold text-slate-600">Normalized Text</p>
                  <p className="whitespace-pre-wrap break-words text-xs text-slate-700">
                    {previewResult.normalizedText || "-"}
                  </p>
                </div>
              </div>
            ) : null}
          </div>
        </div>
      </AdminPageCard>
    </section>
  );
}
