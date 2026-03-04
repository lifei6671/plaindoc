import { ChevronDown, ExternalLink, LoaderCircle, RefreshCw, Search, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEventHandler } from "react";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { Checkbox } from "../../components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger
} from "../../components/ui/dropdown-menu";
import { Input } from "../../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { showToast } from "../../components/ui/toast";
import {
  type AdminDocumentImageAsset,
  type AdminDocumentImageAssetDeleteResult,
  type AdminDocumentImageAssetListResult,
  type DataGateway
} from "../../data-access";
import { formatError } from "../../editor/status-utils";
import { useAdminDialogs } from "../components/AdminDialogs";
import {
  AdminBulkActionBar,
  AdminPageCard,
  AdminPaginationFooter,
  AdminTableContainer,
  AdminToolbarActions
} from "../components/AdminPageLayout";

const DEFAULT_PAGE_SIZE = 20;

interface AdminDocumentImagesPageProps {
  dataGateway: DataGateway;
}

interface AdminDocumentImagesState {
  items: AdminDocumentImageAsset[];
  pagination: AdminDocumentImageAssetListResult["pagination"];
}

function emptyDocumentImagesState(): AdminDocumentImagesState {
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

function renderDocumentStatusLabel(value: AdminDocumentImageAsset["documentStatus"]): string {
  switch (value) {
    case "active":
      return "文档正常";
    case "banned":
      return "文档封禁";
    case "deleted":
      return "文档删除";
    default:
      return value;
  }
}

function renderImageAssetStatusLabel(value: AdminDocumentImageAsset["status"]): string {
  switch (value) {
    case "active":
      return "正常";
    case "pending_cleanup":
      return "待清理";
    case "deleted":
      return "已删除";
    default:
      return value;
  }
}

function renderImageAssetStatusBadgeClass(value: AdminDocumentImageAsset["status"]): string {
  switch (value) {
    case "active":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "pending_cleanup":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "deleted":
      return "border-slate-200 bg-slate-100 text-slate-600";
    default:
      return "border-slate-200 bg-slate-100 text-slate-600";
  }
}

function renderStorageProviderLabel(value: string): string {
  switch (value) {
    case "local":
      return "本地";
    case "cloudflare-r2":
      return "Cloudflare R2";
    case "aliyun-oss":
      return "阿里云 OSS";
    default:
      return value || "-";
  }
}

function buildDocumentReaderPath(spaceID: string, documentRouteKey: string): string {
  return `/r/${encodeURIComponent(spaceID)}/${encodeURIComponent(documentRouteKey)}`;
}

function resolveAbsoluteUrl(raw: string): string {
  const value = raw.trim();
  if (!value) {
    return "";
  }
  if (/^(https?:)?\/\//i.test(value) || /^data:|^blob:/i.test(value)) {
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

function openPathInNewTab(path: string): void {
  if (typeof window === "undefined") {
    return;
  }
  const normalizedPath = path.trim();
  if (!normalizedPath) {
    return;
  }
  window.open(normalizedPath, "_blank", "noopener,noreferrer");
}

function buildDeleteImageToastMessage(result: AdminDocumentImageAssetDeleteResult): {
  message: string;
  variant: "success" | "info";
} {
  if (!result.physicalDeleteRequested) {
    return {
      message: "图片资源删除成功（逻辑删除）",
      variant: "success"
    };
  }

  if (result.physicalDeleteExecuted) {
    return {
      message: "图片资源删除成功（记录与物理文件均已删除）",
      variant: "success"
    };
  }

  const normalizedDeleteError = result.physicalDeleteError.trim();
  if (normalizedDeleteError) {
    return {
      message: `已删除图片记录，但物理文件未删除：${normalizedDeleteError}`,
      variant: "info"
    };
  }

  if (result.sharedReferenceCount > 0) {
    return {
      message: `已删除图片记录；物理文件仍有 ${result.sharedReferenceCount} 个活跃引用，未执行物理删除。`,
      variant: "info"
    };
  }

  return {
    message: "图片资源删除成功",
    variant: "success"
  };
}

function ImageAssetThumbnail({
  previewURL,
  imageAssetID,
  onOpen
}: {
  previewURL: string;
  imageAssetID: string;
  onOpen: () => void;
}) {
  const [failed, setFailed] = useState(false);
  const canRenderImage = !!previewURL && !failed;

  return (
    <button
      type="button"
      className="group relative block h-16 w-24 overflow-hidden rounded-md border border-slate-200 bg-slate-100 text-left"
      onClick={onOpen}
      disabled={!previewURL}
      title={previewURL ? "点击查看原图" : "当前资源无可用预览地址"}
    >
      {canRenderImage ? (
        <img
          src={previewURL}
          alt={`图片资源 ${imageAssetID}`}
          className="h-full w-full object-cover transition-transform duration-200 group-hover:scale-[1.03]"
          loading="lazy"
          referrerPolicy="no-referrer"
          onError={() => setFailed(true)}
        />
      ) : (
        <div className="flex h-full w-full items-center justify-center bg-[linear-gradient(135deg,#e2e8f0_0%,#f8fafc_100%)] px-2 text-center text-[11px] text-slate-500">
          {previewURL ? "预览失败" : "无预览"}
        </div>
      )}
    </button>
  );
}

export function AdminDocumentImagesPage({ dataGateway }: AdminDocumentImagesPageProps) {
  const { confirm, prompt, dialogs } = useAdminDialogs();

  const [keywordInput, setKeywordInput] = useState("");
  const [keyword, setKeyword] = useState("");
  const [spaceIdInput, setSpaceIdInput] = useState("");
  const [spaceId, setSpaceId] = useState("");
  const [documentIdInput, setDocumentIdInput] = useState("");
  const [documentId, setDocumentId] = useState("");
  const [statusFilter, setStatusFilter] = useState<"" | "all" | "active" | "pending_cleanup" | "deleted">("");
  const [storageProviderFilter, setStorageProviderFilter] = useState<"" | "all" | "local" | "cloudflare-r2" | "aliyun-oss">("");
  const [page, setPage] = useState(1);
  const [selectedImageAssetIDs, setSelectedImageAssetIDs] = useState<string[]>([]);

  const [imagesState, setImagesState] = useState<AdminDocumentImagesState>(() => emptyDocumentImagesState());
  const [loading, setLoading] = useState(false);
  const [actioningImageAssetID, setActioningImageAssetID] = useState<string | null>(null);
  const [batchActioning, setBatchActioning] = useState(false);

  const openToast = useCallback((message: string, variant: "success" | "info" | "error" = "error") => {
    showToast(message, variant);
  }, []);

  const loadImages = useCallback(async () => {
    setLoading(true);
    try {
      const payload = await dataGateway.admin.listDocumentImages({
        keyword,
        spaceId,
        documentId,
        status: statusFilter || undefined,
        storageProvider: storageProviderFilter || undefined,
        page,
        pageSize: DEFAULT_PAGE_SIZE
      });
      setImagesState(payload);
    } catch (error) {
      openToast(`加载图片资源列表失败：${formatError(error)}`);
      setImagesState(emptyDocumentImagesState());
    } finally {
      setLoading(false);
    }
  }, [dataGateway.admin, documentId, keyword, openToast, page, spaceId, statusFilter, storageProviderFilter]);

  useEffect(() => {
    void loadImages();
  }, [loadImages]);

  useEffect(() => {
    setSelectedImageAssetIDs((previous) =>
      previous.filter((imageAssetID) => imagesState.items.some((item) => item.imageAssetId === imageAssetID))
    );
  }, [imagesState.items]);

  const selectedImageAssetSet = useMemo(() => new Set(selectedImageAssetIDs), [selectedImageAssetIDs]);
  const selectableImageAssetIDs = useMemo(
    () => imagesState.items.map((item) => item.imageAssetId),
    [imagesState.items]
  );
  const allSelectableChecked = useMemo(
    () =>
      selectableImageAssetIDs.length > 0 &&
      selectableImageAssetIDs.every((imageAssetID) => selectedImageAssetSet.has(imageAssetID)),
    [selectableImageAssetIDs, selectedImageAssetSet]
  );

  const totalPages = useMemo(() => {
    const total = imagesState.pagination.total;
    const pageSize = imagesState.pagination.pageSize || DEFAULT_PAGE_SIZE;
    return Math.max(1, Math.ceil(total / pageSize));
  }, [imagesState.pagination.pageSize, imagesState.pagination.total]);

  const handleSearchSubmit = useCallback<FormEventHandler<HTMLFormElement>>(
    (event) => {
      event.preventDefault();
      setPage(1);
      setKeyword(keywordInput.trim());
      setSpaceId(spaceIdInput.trim());
      setDocumentId(documentIdInput.trim());
    },
    [documentIdInput, keywordInput, spaceIdInput]
  );

  const handleReset = useCallback(() => {
    setKeywordInput("");
    setKeyword("");
    setSpaceIdInput("");
    setSpaceId("");
    setDocumentIdInput("");
    setDocumentId("");
    setStatusFilter("");
    setStorageProviderFilter("");
    setPage(1);
  }, []);

  const handleDelete = useCallback(
    async (imageAsset: AdminDocumentImageAsset) => {
      const promptResult = await prompt({
        title: `删除图片资源：${imageAsset.imageAssetId}`,
        description:
          "逻辑删除仅标记为删除并保留文件；物理删除会删除当前图片记录，并仅在无其他活跃引用时删除物理文件。",
        confirmText: "确认删除",
        tone: "danger",
        fields: [
          {
            key: "physicalDelete",
            label: "删除方式",
            type: "select",
            required: true,
            defaultValue: "false",
            options: [
              { value: "false", label: "仅逻辑删除" },
              { value: "true", label: "物理删除（按引用关系处理文件）" }
            ]
          }
        ]
      });
      if (!promptResult) {
        return;
      }

      const physicalDelete = (promptResult.physicalDelete ?? "false").trim() === "true";
      setActioningImageAssetID(imageAsset.imageAssetId);
      try {
        let deleteResult = await dataGateway.admin.deleteDocumentImage({
          imageAssetId: imageAsset.imageAssetId,
          physicalDelete
        });

        if (physicalDelete && deleteResult.confirmationRequired) {
          const sampleRefs = deleteResult.sharedReferences
            .slice(0, 3)
            .map((item) => item.documentTitle || item.documentId)
            .filter((item) => item && item.trim());
          const confirmationDescription = [
            deleteResult.confirmationReason || "该图片对象仍被多个文档引用。",
            `当前活跃引用数：${deleteResult.sharedReferenceCount}`,
            sampleRefs.length > 0 ? `示例文档：${sampleRefs.join("、")}` : "",
            "继续后仅删除当前图片记录，不会删除物理文件。"
          ]
            .filter((item) => item.trim())
            .join("\n");
          const confirmed = await confirm({
            title: "检测到共享图片引用",
            description: confirmationDescription,
            confirmText: "继续删除当前记录",
            cancelText: "取消",
            tone: "warning"
          });
          if (!confirmed) {
            return;
          }
          deleteResult = await dataGateway.admin.deleteDocumentImage({
            imageAssetId: imageAsset.imageAssetId,
            physicalDelete: true,
            forcePhysicalDeleteOnShare: true
          });
        }

        const toast = buildDeleteImageToastMessage(deleteResult);
        openToast(toast.message, toast.variant);
        await loadImages();
      } catch (error) {
        openToast(`删除图片资源失败：${formatError(error)}`);
      } finally {
        setActioningImageAssetID(null);
      }
    },
    [confirm, dataGateway.admin, loadImages, openToast, prompt]
  );

  const handleToggleSelectAll = useCallback(
    (checked: boolean) => {
      setSelectedImageAssetIDs(checked ? selectableImageAssetIDs : []);
    },
    [selectableImageAssetIDs]
  );

  const handleToggleSelectOne = useCallback((imageAssetID: string, checked: boolean) => {
    setSelectedImageAssetIDs((previous) => {
      if (checked) {
        if (previous.includes(imageAssetID)) {
          return previous;
        }
        return [...previous, imageAssetID];
      }
      return previous.filter((value) => value !== imageAssetID);
    });
  }, []);

  const handleBatchDelete = useCallback(async () => {
    const selectedItems = imagesState.items.filter((item) => selectedImageAssetSet.has(item.imageAssetId));
    if (selectedItems.length === 0) {
      openToast("请先选择需要删除的图片资源");
      return;
    }

    const promptResult = await prompt({
      title: `批量删除图片资源（${selectedItems.length} 项）`,
      description:
        "请选择删除方式。逻辑删除仅标记为删除并保留文件；物理删除会删除记录，并在无引用时删除物理文件（不可恢复）。",
      confirmText: "继续",
      tone: "danger",
      fields: [
        {
          key: "physicalDelete",
          label: "删除方式",
          type: "select",
          required: true,
          defaultValue: "false",
          options: [
            { value: "false", label: "仅逻辑删除" },
            { value: "true", label: "物理删除（不可恢复）" }
          ]
        }
      ]
    });
    if (!promptResult) {
      return;
    }

    const physicalDelete = (promptResult.physicalDelete ?? "false").trim() === "true";
    if (physicalDelete) {
      const confirmed = await confirm({
        title: "确认批量物理删除",
        description: "物理删除后不可恢复。若文件仍被其他文档引用，则不会删除物理文件。",
        confirmText: "确认物理删除",
        cancelText: "取消",
        tone: "danger"
      });
      if (!confirmed) {
        return;
      }
    }

    setBatchActioning(true);
    let successCount = 0;
    const failures: string[] = [];
    try {
      for (const item of selectedItems) {
        try {
          let deleteResult = await dataGateway.admin.deleteDocumentImage({
            imageAssetId: item.imageAssetId,
            physicalDelete
          });
          if (physicalDelete && deleteResult.confirmationRequired) {
            deleteResult = await dataGateway.admin.deleteDocumentImage({
              imageAssetId: item.imageAssetId,
              physicalDelete: true,
              forcePhysicalDeleteOnShare: true
            });
          }
          const toast = buildDeleteImageToastMessage(deleteResult);
          if (toast.variant === "info") {
            failures.push(`${item.imageAssetId}: ${toast.message}`);
          } else {
            successCount += 1;
          }
        } catch (error) {
          failures.push(`${item.imageAssetId}: ${formatError(error)}`);
        }
      }

      await loadImages();
      setSelectedImageAssetIDs([]);
      if (failures.length > 0) {
        openToast(`批量删除完成：成功 ${successCount}，异常 ${failures.length}。首个异常：${failures[0]}`, "info");
      } else {
        openToast(`批量删除成功：共 ${successCount} 条`, "success");
      }
    } finally {
      setBatchActioning(false);
    }
  }, [confirm, dataGateway.admin, imagesState.items, loadImages, openToast, prompt, selectedImageAssetSet]);

  const selectionDisabled = loading || batchActioning || actioningImageAssetID !== null;

  return (
    <AdminPageCard className="border border-slate-200 bg-white shadow-sm" contentClassName="space-y-4 px-5 pb-5 pt-4">
      <form className="grid gap-3 xl:grid-cols-[minmax(0,2fr)_minmax(220px,1fr)_minmax(220px,1fr)_180px_190px_auto]" onSubmit={handleSearchSubmit}>
        <label className="space-y-1.5">
          <span className="text-xs font-medium text-slate-600">关键词</span>
          <div className="relative">
            <Search className="pointer-events-none absolute left-2 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <Input
              value={keywordInput}
              onChange={(event) => setKeywordInput(event.target.value)}
              placeholder="图片资源ID / 对象键 / 文档名 / 空间名"
              className="h-9 pl-8"
            />
          </div>
        </label>

        <label className="space-y-1.5">
          <span className="text-xs font-medium text-slate-600">空间 ID</span>
          <Input
            value={spaceIdInput}
            onChange={(event) => setSpaceIdInput(event.target.value)}
            placeholder="按空间过滤"
            className="h-9"
          />
        </label>

        <label className="space-y-1.5">
          <span className="text-xs font-medium text-slate-600">文档 ID</span>
          <Input
            value={documentIdInput}
            onChange={(event) => setDocumentIdInput(event.target.value)}
            placeholder="按文档过滤"
            className="h-9"
          />
        </label>

        <label className="space-y-1.5">
          <span className="text-xs font-medium text-slate-600">状态</span>
          <Select value={statusFilter || "__EMPTY__"} onValueChange={(value) => setStatusFilter(value === "__EMPTY__" ? "" : value as typeof statusFilter)}>
            <SelectTrigger className="h-9">
              <SelectValue placeholder="全部状态" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__EMPTY__">全部状态</SelectItem>
              <SelectItem value="all">全部（含待清理/已删）</SelectItem>
              <SelectItem value="active">仅正常</SelectItem>
              <SelectItem value="pending_cleanup">仅待清理</SelectItem>
              <SelectItem value="deleted">仅已删除</SelectItem>
            </SelectContent>
          </Select>
        </label>

        <label className="space-y-1.5">
          <span className="text-xs font-medium text-slate-600">存储提供商</span>
          <Select
            value={storageProviderFilter || "__EMPTY__"}
            onValueChange={(value) => setStorageProviderFilter(value === "__EMPTY__" ? "" : value as typeof storageProviderFilter)}
          >
            <SelectTrigger className="h-9">
              <SelectValue placeholder="全部提供商" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__EMPTY__">全部提供商</SelectItem>
              <SelectItem value="all">全部</SelectItem>
              <SelectItem value="local">本地</SelectItem>
              <SelectItem value="cloudflare-r2">Cloudflare R2</SelectItem>
              <SelectItem value="aliyun-oss">阿里云 OSS</SelectItem>
            </SelectContent>
          </Select>
        </label>

        <AdminToolbarActions className="xl:justify-self-end">
          <Button type="submit" size="sm" className="h-9">
            查询
          </Button>
          <Button type="button" size="sm" variant="outline" className="h-9" onClick={handleReset}>
            重置
          </Button>
          <Button type="button" size="icon" variant="outline" className="h-9 w-9" onClick={() => void loadImages()} disabled={loading}>
            {loading ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
            <span className="sr-only">刷新</span>
          </Button>
        </AdminToolbarActions>
      </form>

      <AdminBulkActionBar selectedCount={selectedImageAssetIDs.length}>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="border-rose-200 bg-rose-50 text-rose-700 shadow-none hover:bg-rose-100"
          disabled={selectionDisabled || selectedImageAssetIDs.length === 0}
          onClick={() => void handleBatchDelete()}
        >
          批量删除
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="border-slate-300 bg-white text-slate-700 shadow-none hover:bg-slate-50"
          disabled={selectionDisabled || selectedImageAssetIDs.length === 0}
          onClick={() => setSelectedImageAssetIDs([])}
        >
          清空选择
        </Button>
      </AdminBulkActionBar>

      <AdminTableContainer>
        <table className="min-w-full table-fixed border-collapse text-sm">
          <thead className="bg-slate-50/80 text-left text-xs uppercase tracking-wide text-slate-500">
            <tr>
              <th className="w-10 px-3 py-2 font-semibold">
                <Checkbox
                  checked={allSelectableChecked}
                  disabled={selectionDisabled || selectableImageAssetIDs.length === 0}
                  onCheckedChange={(checked) => handleToggleSelectAll(checked === true)}
                  aria-label="全选图片资源"
                />
              </th>
              <th className="w-[360px] px-3 py-2 font-semibold">图片资源</th>
              <th className="w-[280px] px-3 py-2 font-semibold">所属文档</th>
              <th className="w-[240px] px-3 py-2 font-semibold">所属空间</th>
              <th className="w-[150px] px-3 py-2 font-semibold">存储</th>
              <th className="w-[130px] px-3 py-2 font-semibold">状态</th>
              <th className="w-[190px] px-3 py-2 font-semibold">最近引用</th>
              <th className="w-[190px] px-3 py-2 font-semibold">更新时间</th>
              <th className="w-[220px] px-3 py-2 font-semibold">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100 text-slate-700">
            {imagesState.items.length === 0 ? (
              <tr>
                <td className="px-3 py-9 text-center text-sm text-slate-500" colSpan={9}>
                  {loading ? "正在加载图片资源..." : "暂无匹配的图片资源记录"}
                </td>
              </tr>
            ) : (
              imagesState.items.map((imageAsset) => {
                const actioning = actioningImageAssetID === imageAsset.imageAssetId || batchActioning;
                const isDeleted = imageAsset.status === "deleted";
                const previewURL = resolveAbsoluteUrl(imageAsset.objectUrl);
                return (
                  <tr key={imageAsset.imageAssetId} className="align-top hover:bg-slate-50/60">
                    <td className="px-3 py-2.5">
                      <Checkbox
                        checked={selectedImageAssetSet.has(imageAsset.imageAssetId)}
                        disabled={selectionDisabled}
                        onCheckedChange={(checked) => handleToggleSelectOne(imageAsset.imageAssetId, checked === true)}
                        aria-label={`选择图片资源 ${imageAsset.imageAssetId}`}
                      />
                    </td>
                    <td className="px-3 py-2.5">
                      <div className="flex items-start gap-3">
                        <ImageAssetThumbnail
                          previewURL={previewURL}
                          imageAssetID={imageAsset.imageAssetId}
                          onOpen={() => openPathInNewTab(previewURL)}
                        />
                        <div className="min-w-0 space-y-1">
                          <p className="line-clamp-1 break-all font-medium text-slate-900">{imageAsset.imageAssetId}</p>
                          <p className="line-clamp-2 break-all text-xs text-slate-500">{imageAsset.objectKey || "-"}</p>
                          <p className="line-clamp-2 break-all text-xs text-slate-500">{imageAsset.objectUrl || "-"}</p>
                        </div>
                      </div>
                    </td>
                    <td className="px-3 py-2.5">
                      <div className="space-y-1">
                        <p className="line-clamp-2 break-all text-sm font-medium text-slate-900">
                          {imageAsset.documentTitle || imageAsset.documentId}
                        </p>
                        <p className="break-all text-xs text-slate-500">{imageAsset.documentId}</p>
                        <p className="text-xs text-slate-500">{renderDocumentStatusLabel(imageAsset.documentStatus)}</p>
                      </div>
                    </td>
                    <td className="px-3 py-2.5">
                      <div className="space-y-1">
                        <p className="line-clamp-2 break-all text-sm font-medium text-slate-900">
                          {imageAsset.spaceName || imageAsset.spaceId}
                        </p>
                        <p className="break-all text-xs text-slate-500">{imageAsset.spaceId}</p>
                        <p className="line-clamp-1 text-xs text-slate-500">
                          所有者：{imageAsset.spaceOwnerName || imageAsset.spaceOwnerEmail || imageAsset.spaceOwnerUserId}
                        </p>
                      </div>
                    </td>
                    <td className="px-3 py-2.5">
                      <Badge variant="outline" className="border-slate-200 bg-slate-50 text-slate-700">
                        {renderStorageProviderLabel(imageAsset.storageProvider)}
                      </Badge>
                    </td>
                    <td className="px-3 py-2.5">
                      <Badge variant="outline" className={renderImageAssetStatusBadgeClass(imageAsset.status)}>
                        {renderImageAssetStatusLabel(imageAsset.status)}
                      </Badge>
                    </td>
                    <td className="px-3 py-2.5 text-xs text-slate-600">{formatDateTime(imageAsset.lastReferencedAt)}</td>
                    <td className="px-3 py-2.5 text-xs text-slate-600">{formatDateTime(imageAsset.updatedAt)}</td>
                    <td className="px-3 py-2.5">
                      <div className="inline-flex rounded-md">
                        <Button
                          type="button"
                          size="sm"
                          variant="outline"
                          className="h-7 rounded-r-none border-r-0 px-2 text-xs"
                          disabled={actioning || !previewURL}
                          onClick={() => openPathInNewTab(previewURL)}
                        >
                          <ExternalLink className="mr-1 h-3.5 w-3.5" />
                          查看
                        </Button>
                        <DropdownMenu modal={false}>
                          <DropdownMenuTrigger asChild>
                            <Button
                              type="button"
                              size="sm"
                              variant="outline"
                              className="h-7 w-8 rounded-l-none px-0"
                              disabled={actioning}
                              aria-label="展开更多操作"
                            >
                              <ChevronDown size={14} />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end" className="w-40">
                            <DropdownMenuItem
                              disabled={isDeleted}
                              onSelect={() =>
                                openPathInNewTab(
                                  buildDocumentReaderPath(
                                    imageAsset.spaceId,
                                    (imageAsset.documentRouteKey || imageAsset.documentId || "").trim()
                                  )
                                )
                              }
                            >
                              <ExternalLink size={14} className="mr-2" />
                              <span>查看文档</span>
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              disabled={actioning}
                              onSelect={() => void handleDelete(imageAsset)}
                              className="text-rose-600 focus:text-rose-600"
                            >
                              {actioning ? (
                                <LoaderCircle size={14} className="mr-2 animate-spin" />
                              ) : (
                                <Trash2 size={14} className="mr-2" />
                              )}
                              <span>删除</span>
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

      <AdminPaginationFooter
        summary={
          loading
            ? "正在加载图片资源列表..."
            : `共 ${imagesState.pagination.total} 条，当前第 ${imagesState.pagination.page} / ${totalPages} 页`
        }
        previousDisabled={loading || page <= 1}
        nextDisabled={loading || page >= totalPages}
        onPrevious={() => setPage((previousPage) => Math.max(1, previousPage - 1))}
        onNext={() => setPage((previousPage) => Math.min(totalPages, previousPage + 1))}
      />

      {dialogs}
    </AdminPageCard>
  );
}
