import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import { languages } from "@codemirror/language-data";
import { MergeView } from "@codemirror/merge";
import { EditorState } from "@codemirror/state";
import { EditorView, lineNumbers } from "@codemirror/view";
import { History, X } from "lucide-react";
import { memo, useCallback, useEffect, useMemo, useRef, useState, type UIEvent } from "react";
import {
  ConflictError,
  type DataGateway,
  type DocumentRevisionDetail,
  type DocumentRevisionSummary,
  type RestoreDocumentRevisionResult
} from "../data-access";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger
} from "./ui/tooltip";

const REVISION_PAGE_SIZE = 30;

interface DocumentRevisionHistoryTriggerProps {
  disabled: boolean;
  onOpen: () => void;
}

interface DocumentRevisionHistoryDialogProps {
  open: boolean;
  documentId: string | null | undefined;
  documentTitle: string;
  currentContent: string;
  currentDocumentVersion: number;
  hasUnsavedChanges: boolean;
  dataGateway: DataGateway;
  onOpenChange: (open: boolean) => void;
  onRestoreSuccess: (result: RestoreDocumentRevisionResult) => void;
}

interface MarkdownRevisionDiffViewProps {
  revisionId: string;
  historicalContent: string;
  currentContent: string;
}

interface OfficeRevisionMetadataPanelProps {
  revision: DocumentRevisionDetail;
}

function formatRevisionCreatedAt(createdAt: string): string {
  const parsedDate = new Date(createdAt);
  if (Number.isNaN(parsedDate.getTime())) {
    return createdAt || "未知时间";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(parsedDate);
}

function getRevisionEditorName(revision: DocumentRevisionSummary): string {
  return revision.editorUser?.displayName?.trim() || "未知创建人";
}

function getRevisionFormatLabel(format: DocumentRevisionSummary["format"]): string {
  if (format === "markdown") {
    return "Markdown";
  }
  if (format === "docx") {
    return "Word 文档";
  }
  return "Excel 表格";
}

function isOfficeRevision(revision: DocumentRevisionSummary | DocumentRevisionDetail): boolean {
  return revision.format === "docx" || revision.format === "xlsx";
}

function getOfficeRevisionFileName(revision: DocumentRevisionDetail): string {
  return revision.file?.fileName?.trim() || revision.fileName?.trim() || "未记录文件名";
}

function getOfficeRevisionMimeType(revision: DocumentRevisionDetail): string {
  return revision.file?.mimeType?.trim() || revision.mimeType?.trim() || "未记录 MIME";
}

function formatRestoreError(error: unknown): string {
  if (error instanceof ConflictError) {
    return `版本冲突：当前文档已更新到 v${error.latestDocument.version}，请刷新或重新选择历史版本后再试。`;
  }
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  return "恢复历史版本失败";
}

function mergeRevisionSummaries(
  currentRevisions: DocumentRevisionSummary[],
  nextRevisions: DocumentRevisionSummary[]
): DocumentRevisionSummary[] {
  const seenRevisionIDs = new Set(currentRevisions.map((revision) => revision.id));
  const mergedRevisions = [...currentRevisions];
  for (const revision of nextRevisions) {
    if (seenRevisionIDs.has(revision.id)) {
      continue;
    }
    seenRevisionIDs.add(revision.id);
    mergedRevisions.push(revision);
  }
  return mergedRevisions;
}

function createReadonlyMarkdownExtensions() {
  return [
    lineNumbers(),
    markdown({ base: markdownLanguage, codeLanguages: languages }),
    EditorView.lineWrapping,
    EditorView.editable.of(false),
    EditorState.readOnly.of(true)
  ];
}

function MarkdownRevisionDiffView({
  revisionId,
  historicalContent,
  currentContent
}: MarkdownRevisionDiffViewProps) {
  const mergeViewParentRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const parent = mergeViewParentRef.current;
    if (!parent) {
      return undefined;
    }
    parent.replaceChildren();

    // Markdown diff 视图使用 CodeMirror MergeView，只读展示历史正文与当前编辑器正文。
    const mergeView = new MergeView({
      a: {
        doc: historicalContent,
        extensions: createReadonlyMarkdownExtensions()
      },
      b: {
        doc: currentContent,
        extensions: createReadonlyMarkdownExtensions()
      },
      collapseUnchanged: { margin: 3, minSize: 8 },
      diffConfig: { scanLimit: 1200, timeout: 500 },
      parent
    });
    console.info("[历史版本] Markdown 差异视图已渲染", {
      revisionID: revisionId,
      historicalLength: historicalContent.length,
      currentLength: currentContent.length
    });

    return () => {
      mergeView.destroy();
      parent.replaceChildren();
    };
  }, [currentContent, historicalContent, revisionId]);

  return (
    <section className="revision-history-dialog__diff" role="region" aria-label="Markdown 差异视图">
      <div className="revision-history-dialog__diff-header">
        <span>历史版本</span>
        <span>当前内容</span>
      </div>
      <div ref={mergeViewParentRef} className="revision-history-dialog__merge-view" />
    </section>
  );
}

function OfficeRevisionMetadataPanel({ revision }: OfficeRevisionMetadataPanelProps) {
  const editorName = getRevisionEditorName(revision);
  const createdAtText = formatRevisionCreatedAt(revision.createdAt);

  return (
    <section
      className="revision-history-dialog__office"
      role="region"
      aria-label="Office 历史版本元数据"
    >
      <p className="revision-history-dialog__office-note">Office 文档暂不支持二进制差异预览。</p>
      {/* Office 版本首期不做二进制 diff，只展示服务端详情接口返回的源文件元数据。 */}
      <dl className="revision-history-dialog__office-grid">
        <div>
          <dt>文件名</dt>
          <dd>{getOfficeRevisionFileName(revision)}</dd>
        </div>
        <div>
          <dt>MIME</dt>
          <dd>{getOfficeRevisionMimeType(revision)}</dd>
        </div>
        <div>
          <dt>版本号</dt>
          <dd>v{revision.version}</dd>
        </div>
        <div>
          <dt>创建时间</dt>
          <dd>{createdAtText}</dd>
        </div>
        <div>
          <dt>创建人</dt>
          <dd>{editorName}</dd>
        </div>
      </dl>
    </section>
  );
}

// 顶部历史版本入口：只负责打开弹窗，不在按钮点击阶段触发任何网络请求。
export const DocumentRevisionHistoryTrigger = memo(function DocumentRevisionHistoryTrigger({
  disabled,
  onOpen
}: DocumentRevisionHistoryTriggerProps) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          className="revision-history-trigger"
          aria-label="历史版本"
          disabled={disabled}
          onClick={onOpen}
        >
          <History size={14} />
        </button>
      </TooltipTrigger>
      <TooltipContent side="bottom">历史版本</TooltipContent>
    </Tooltip>
  );
});

// 历史版本弹窗：统一管理列表分页、详情加载、Markdown 只读 diff 和 Office 元数据状态。
export function DocumentRevisionHistoryDialog({
  open,
  documentId,
  documentTitle,
  currentContent,
  currentDocumentVersion,
  hasUnsavedChanges,
  dataGateway,
  onOpenChange,
  onRestoreSuccess
}: DocumentRevisionHistoryDialogProps) {
  const normalizedDocumentTitle = documentTitle.trim() || "未命名文档";
  const normalizedDocumentID = documentId?.trim() || "";
  const [revisions, setRevisions] = useState<DocumentRevisionSummary[]>([]);
  const [selectedRevisionID, setSelectedRevisionID] = useState<string | null>(null);
  const [currentPage, setCurrentPage] = useState(1);
  const [hasMoreRevisions, setHasMoreRevisions] = useState(false);
  const [isInitialLoading, setIsInitialLoading] = useState(false);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [loadErrorMessage, setLoadErrorMessage] = useState<string | null>(null);
  const [revisionDetail, setRevisionDetail] = useState<DocumentRevisionDetail | null>(null);
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [detailErrorMessage, setDetailErrorMessage] = useState<string | null>(null);
  const [restoreConfirmationRevisionID, setRestoreConfirmationRevisionID] = useState<string | null>(null);
  const [isRestoring, setIsRestoring] = useState(false);
  const [restoreErrorMessage, setRestoreErrorMessage] = useState<string | null>(null);
  const [restoreSuccessMessage, setRestoreSuccessMessage] = useState<string | null>(null);
  const detailRequestIDRef = useRef(0);

  const selectedRevision = useMemo(() => {
    if (selectedRevisionID) {
      return revisions.find((revision) => revision.id === selectedRevisionID) ?? revisions[0] ?? null;
    }
    return revisions[0] ?? null;
  }, [revisions, selectedRevisionID]);

  const loadRevisionPage = useCallback(async (targetPage: number, mode: "reset" | "append") => {
    if (!normalizedDocumentID) {
      return;
    }
    if (mode === "reset") {
      setIsInitialLoading(true);
      setSelectedRevisionID(null);
    } else {
      setIsLoadingMore(true);
    }
    setLoadErrorMessage(null);

    try {
      const nextRevisions = await dataGateway.document.listRevisions(normalizedDocumentID, {
        page: targetPage,
        pageSize: REVISION_PAGE_SIZE
      });
      setRevisions((currentRevisions) => {
        if (mode === "reset") {
          return mergeRevisionSummaries([], nextRevisions);
        }
        return mergeRevisionSummaries(currentRevisions, nextRevisions);
      });
      setCurrentPage(targetPage);
      setHasMoreRevisions(nextRevisions.length >= REVISION_PAGE_SIZE);
      console.info("[历史版本] 历史版本列表加载成功", {
        documentID: normalizedDocumentID,
        page: targetPage,
        pageSize: REVISION_PAGE_SIZE,
        count: nextRevisions.length
      });
    } catch (error) {
      const message = error instanceof Error ? error.message : "历史版本加载失败";
      setLoadErrorMessage(message);
      console.error("[历史版本] 历史版本列表加载失败", {
        documentID: normalizedDocumentID,
        page: targetPage,
        error
      });
    } finally {
      if (mode === "reset") {
        setIsInitialLoading(false);
      } else {
        setIsLoadingMore(false);
      }
    }
  }, [dataGateway, normalizedDocumentID]);

  const handleRetry = useCallback(() => {
    void loadRevisionPage(revisions.length > 0 ? currentPage + 1 : 1, revisions.length > 0 ? "append" : "reset");
  }, [currentPage, loadRevisionPage, revisions.length]);

  const handleLoadMore = useCallback(() => {
    if (!hasMoreRevisions || isInitialLoading || isLoadingMore) {
      return;
    }
    void loadRevisionPage(currentPage + 1, "append");
  }, [currentPage, hasMoreRevisions, isInitialLoading, isLoadingMore, loadRevisionPage]);

  const handleRevisionListScroll = useCallback((event: UIEvent<HTMLDivElement>) => {
    const target = event.currentTarget;
    const distanceToBottom = target.scrollHeight - target.scrollTop - target.clientHeight;
    if (distanceToBottom <= 24) {
      handleLoadMore();
    }
  }, [handleLoadMore]);

  useEffect(() => {
    if (!open || !normalizedDocumentID) {
      setRevisions([]);
      setSelectedRevisionID(null);
      setLoadErrorMessage(null);
      setRevisionDetail(null);
      setDetailErrorMessage(null);
      setRestoreConfirmationRevisionID(null);
      setRestoreErrorMessage(null);
      setRestoreSuccessMessage(null);
      setIsRestoring(false);
      return;
    }
    // 弹窗每次打开都从第一页重新加载，避免不同文档之间复用旧分页状态。
    void loadRevisionPage(1, "reset");
  }, [loadRevisionPage, normalizedDocumentID, open]);

  useEffect(() => {
    setRestoreConfirmationRevisionID(null);
    setRestoreErrorMessage(null);
    setIsRestoring(false);
  }, [selectedRevision?.id]);

  useEffect(() => {
    if (!open || !normalizedDocumentID || !selectedRevision) {
      setRevisionDetail(null);
      setDetailErrorMessage(null);
      setIsDetailLoading(false);
      return undefined;
    }
    const requestID = detailRequestIDRef.current + 1;
    detailRequestIDRef.current = requestID;
    setRevisionDetail(null);
    setDetailErrorMessage(null);
    setIsDetailLoading(true);
    console.info("[历史版本] 开始加载历史版本详情", {
      documentID: normalizedDocumentID,
      revisionID: selectedRevision.id,
      format: selectedRevision.format
    });

    let isCurrentRequest = true;
    void dataGateway.document.getRevisionDetail(normalizedDocumentID, selectedRevision.id)
      .then((detail) => {
        if (!isCurrentRequest || detailRequestIDRef.current !== requestID) {
          console.info("[历史版本] 忽略过期的历史版本详情响应", {
            documentID: normalizedDocumentID,
            revisionID: selectedRevision.id
          });
          return;
        }
        setRevisionDetail(detail);
        console.info("[历史版本] 历史版本详情加载成功", {
          documentID: normalizedDocumentID,
          revisionID: selectedRevision.id,
          format: detail.format,
          contentLength: detail.contentMd?.length ?? 0
        });
      })
      .catch((error) => {
        if (!isCurrentRequest || detailRequestIDRef.current !== requestID) {
          return;
        }
        const message = error instanceof Error ? error.message : "历史版本详情加载失败";
        setDetailErrorMessage(message);
        console.error("[历史版本] 历史版本详情加载失败", {
          documentID: normalizedDocumentID,
          revisionID: selectedRevision.id,
          format: selectedRevision.format,
          error
        });
      })
      .finally(() => {
        if (isCurrentRequest && detailRequestIDRef.current === requestID) {
          setIsDetailLoading(false);
        }
      });

    return () => {
      isCurrentRequest = false;
    };
  }, [dataGateway, normalizedDocumentID, open, selectedRevision]);

  const handleOpenRestoreConfirmation = useCallback((revision: DocumentRevisionDetail) => {
    setRestoreConfirmationRevisionID(revision.id);
    setRestoreErrorMessage(null);
    setRestoreSuccessMessage(null);
  }, []);

  const handleConfirmRestore = useCallback(async () => {
    if (!normalizedDocumentID || !revisionDetail || hasUnsavedChanges || isRestoring) {
      return;
    }

    setIsRestoring(true);
    setRestoreErrorMessage(null);
    setRestoreSuccessMessage(null);
    console.info("[历史版本] 开始恢复历史版本", {
      documentID: normalizedDocumentID,
      revisionID: revisionDetail.id,
      baseVersion: currentDocumentVersion,
      format: revisionDetail.format
    });

    try {
      const result = await dataGateway.document.restoreRevision({
        docId: normalizedDocumentID,
        revisionId: revisionDetail.id,
        baseVersion: currentDocumentVersion
      });
      onRestoreSuccess(result);
      setRestoreConfirmationRevisionID(null);
      setRestoreSuccessMessage(`已恢复到 v${revisionDetail.version}，当前文档版本 v${result.document.version}。`);
      console.info("[历史版本] 历史版本恢复成功", {
        documentID: normalizedDocumentID,
        revisionID: revisionDetail.id,
        restoredVersion: revisionDetail.version,
        currentVersion: result.document.version
      });
      // 恢复成功会新增一条 revision；立即刷新第一页，避免列表仍停留在旧版本集合。
      await loadRevisionPage(1, "reset");
    } catch (error) {
      const message = formatRestoreError(error);
      setRestoreErrorMessage(message);
      console.error("[历史版本] 历史版本恢复失败", {
        documentID: normalizedDocumentID,
        revisionID: revisionDetail.id,
        baseVersion: currentDocumentVersion,
        error
      });
    } finally {
      setIsRestoring(false);
    }
  }, [
    currentDocumentVersion,
    dataGateway,
    hasUnsavedChanges,
    isRestoring,
    loadRevisionPage,
    normalizedDocumentID,
    onRestoreSuccess,
    revisionDetail
  ]);

  const renderRestoreControls = (detail: DocumentRevisionDetail) => {
    const isConfirmationOpen = restoreConfirmationRevisionID === detail.id;
    const editorName = getRevisionEditorName(detail);
    const createdAtText = formatRevisionCreatedAt(detail.createdAt);
    const isOffice = isOfficeRevision(detail);
    const disabled = hasUnsavedChanges || isRestoring;
    const hint = hasUnsavedChanges
      ? "存在未保存修改，恢复前请先保存或放弃当前编辑。"
      : isOffice
        ? "恢复前会要求二次确认，确认后会切换当前 Office 源文件版本。"
        : "恢复前会要求二次确认，确认后当前正文会被历史版本覆盖。";

    return (
      <>
        {restoreSuccessMessage ? (
          <p className="revision-history-dialog__restore-success" role="status">{restoreSuccessMessage}</p>
        ) : null}
        <div className="revision-history-dialog__restore-row">
          <span>{hint}</span>
          <button type="button" disabled={disabled} onClick={() => handleOpenRestoreConfirmation(detail)}>
            恢复到此版本
          </button>
        </div>
        {isConfirmationOpen ? (
          <section
            className="revision-history-dialog__restore-confirm"
            role="alertdialog"
            aria-label="确认恢复历史版本"
          >
            <p>
              即将恢复到 v{detail.version}（{createdAtText}，{editorName}）。
              {isOffice
                ? "确认后会切换当前 Office 源文件版本。"
                : "确认后当前 Markdown 正文会被该历史版本覆盖。"}
            </p>
            {restoreErrorMessage ? (
              <p className="revision-history-dialog__restore-error" role="alert">{restoreErrorMessage}</p>
            ) : null}
            <div className="revision-history-dialog__restore-actions">
              <button
                type="button"
                className="revision-history-dialog__restore-secondary"
                disabled={isRestoring}
                onClick={() => setRestoreConfirmationRevisionID(null)}
              >
                取消
              </button>
              <button type="button" disabled={isRestoring} onClick={() => void handleConfirmRestore()}>
                {isRestoring ? "正在恢复..." : "确认恢复"}
              </button>
            </div>
          </section>
        ) : null}
      </>
    );
  };

  if (!open) {
    return null;
  }

  return (
    <div className="revision-history-dialog-layer">
      <button
        type="button"
        className="revision-history-dialog-layer__backdrop"
        aria-label="关闭历史版本弹窗背景"
        onClick={() => onOpenChange(false)}
      />
      <section className="revision-history-dialog" role="dialog" aria-modal="true" aria-label="历史版本">
        <header className="revision-history-dialog__header">
          <div className="revision-history-dialog__title-group">
            <p className="revision-history-dialog__eyebrow">历史版本</p>
            <h2 className="revision-history-dialog__title" title={normalizedDocumentTitle}>
              {normalizedDocumentTitle}
            </h2>
            {normalizedDocumentID ? (
              <p className="revision-history-dialog__meta" title={normalizedDocumentID}>
                {normalizedDocumentID}
              </p>
            ) : null}
          </div>
          <button
            type="button"
            className="revision-history-dialog__close"
            aria-label="关闭历史版本"
            onClick={() => onOpenChange(false)}
          >
            <X size={16} />
          </button>
        </header>
        <div className="revision-history-dialog__body">
          <aside className="revision-history-dialog__list-panel" aria-label="历史版本列表">
            <div className="revision-history-dialog__list-header">
              <span>版本列表</span>
              <strong>{revisions.length}</strong>
            </div>
            <div className="revision-history-dialog__list" onScroll={handleRevisionListScroll}>
              {isInitialLoading ? (
                <div className="revision-history-dialog__state">正在加载历史版本...</div>
              ) : null}
              {!isInitialLoading && loadErrorMessage && revisions.length === 0 ? (
                <div className="revision-history-dialog__state revision-history-dialog__state--error">
                  <span>{loadErrorMessage}</span>
                  <button type="button" onClick={handleRetry}>重试</button>
                </div>
              ) : null}
              {!isInitialLoading && !loadErrorMessage && revisions.length === 0 ? (
                <div className="revision-history-dialog__state">暂无历史版本</div>
              ) : null}
              {revisions.map((revision) => {
                const editorName = getRevisionEditorName(revision);
                const createdAtText = formatRevisionCreatedAt(revision.createdAt);
                const isSelected = selectedRevision?.id === revision.id;
                return (
                  <button
                    type="button"
                    key={revision.id}
                    className={
                      isSelected
                        ? "revision-history-dialog__item revision-history-dialog__item--active"
                        : "revision-history-dialog__item"
                    }
                    aria-pressed={isSelected}
                    aria-label={`版本 ${revision.version}，${editorName}`}
                    onClick={() => {
                      setRestoreSuccessMessage(null);
                      setSelectedRevisionID(revision.id);
                    }}
                  >
                    <span className="revision-history-dialog__item-version">v{revision.version}</span>
                    <span className="revision-history-dialog__item-time">{createdAtText}</span>
                    <span className="revision-history-dialog__item-user">{editorName}</span>
                  </button>
                );
              })}
            </div>
            {loadErrorMessage && revisions.length > 0 ? (
              <div className="revision-history-dialog__inline-error">
                <span>{loadErrorMessage}</span>
                <button type="button" onClick={handleRetry}>重试</button>
              </div>
            ) : null}
            {hasMoreRevisions ? (
              <button
                type="button"
                className="revision-history-dialog__load-more"
                disabled={isLoadingMore}
                onClick={handleLoadMore}
              >
                {isLoadingMore ? "正在加载..." : "加载更多"}
              </button>
            ) : null}
          </aside>
          <div className="revision-history-dialog__detail-panel" role="region" aria-label="历史版本详情">
            {selectedRevision ? (
              <div className="revision-history-dialog__detail-card">
                <p className="revision-history-dialog__detail-eyebrow">选中版本</p>
                <h3>v{selectedRevision.version}</h3>
                <dl>
                  <div>
                    <dt>创建时间</dt>
                    <dd>{formatRevisionCreatedAt(selectedRevision.createdAt)}</dd>
                  </div>
                  <div>
                    <dt>创建人</dt>
                    <dd>{getRevisionEditorName(selectedRevision)}</dd>
                  </div>
                  <div>
                    <dt>格式</dt>
                    <dd>{getRevisionFormatLabel(selectedRevision.format)}</dd>
                  </div>
                </dl>
                {isDetailLoading ? (
                  <p className="revision-history-dialog__detail-placeholder">正在加载版本详情...</p>
                ) : null}
                {!isDetailLoading && detailErrorMessage ? (
                  <div className="revision-history-dialog__detail-error">
                    <span>{detailErrorMessage}</span>
                  </div>
                ) : null}
                {!isDetailLoading && !detailErrorMessage && revisionDetail?.format === "markdown" ? (
                  <>
                    <MarkdownRevisionDiffView
                      revisionId={revisionDetail.id}
                      historicalContent={revisionDetail.contentMd ?? ""}
                      currentContent={currentContent}
                    />
                    {renderRestoreControls(revisionDetail)}
                  </>
                ) : null}
                {!isDetailLoading && !detailErrorMessage && revisionDetail && isOfficeRevision(revisionDetail) ? (
                  <>
                    <OfficeRevisionMetadataPanel revision={revisionDetail} />
                    {renderRestoreControls(revisionDetail)}
                  </>
                ) : null}
                {!isDetailLoading && !detailErrorMessage && !revisionDetail ? (
                  <p className="revision-history-dialog__detail-placeholder">
                    版本详情和差异视图将在后续步骤接入。
                  </p>
                ) : null}
              </div>
            ) : (
              <div className="revision-history-dialog__placeholder">
                <span>请选择一个历史版本。</span>
              </div>
            )}
          </div>
        </div>
      </section>
    </div>
  );
}
