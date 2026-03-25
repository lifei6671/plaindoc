import { EditorView } from "@codemirror/view";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  BLOCK_ANCHOR_SELECTOR,
  MAX_SYNC_STEP_PER_FRAME,
  PREVIEW_BODY_SELECTOR,
  SYNC_SETTLE_THRESHOLD
} from "./constants";
import { scrollPreviewToTocItem } from "./markdown-utils";
import { buildDirectionAnchors, clamp, getMaxScrollable } from "./scroll-sync";
import type {
  DirectionAnchor,
  PreviewViewportMode,
  ScrollAnchor,
  ScrollSource,
  TocItem
} from "./types";

interface UseScrollSyncOptions {
  content: string;
  renderedContent: string;
  previewThemeClassName: string;
  customPreviewStyleText: string;
  previewViewportMode: PreviewViewportMode;
}

interface UseScrollSyncResult {
  handleEditorPaneRef: (node: HTMLElement | null) => void;
  handlePreviewScrollerRef: (node: HTMLElement | null) => void;
  handleEditorCreate: (view: EditorView) => void;
  handleTocNavigate: (item: TocItem) => void;
}

const EDITOR_DRIVEN_SYNC_LOCK_MS = 1200;
const EDITOR_SCROLL_JUMP_MIN_PIXELS = 320;
const EDITOR_SCROLL_JUMP_VIEWPORT_RATIO = 0.8;
const EDITOR_SCROLL_JUMP_DOCUMENT_RATIO = 0.025;

type SyncMode = "smooth" | "snap";

// 将滚动同步相关状态与副作用封装成 Hook，降低 App 复杂度。
export function useScrollSync({
  content,
  renderedContent,
  previewThemeClassName,
  customPreviewStyleText,
  previewViewportMode
}: UseScrollSyncOptions): UseScrollSyncResult {
  // 编辑器面板根节点，用于在 StrictMode 下追踪滚动容器的重建。
  const [editorPaneElement, setEditorPaneElement] = useState<HTMLElement | null>(null);
  // 编辑区滚动容器（state 版本），用于保证监听器绑定时机稳定。
  const [editorScrollerElement, setEditorScrollerElement] = useState<HTMLElement | null>(null);
  // 预览区滚动容器（state 版本），用于保证监听器绑定时机稳定。
  const [previewScrollerElement, setPreviewScrollerElement] = useState<HTMLElement | null>(null);
  // CodeMirror 滚动容器。
  const editorScrollerRef = useRef<HTMLElement | null>(null);
  // 编辑器面板节点（ref 版本），用于在回调里兜底查询最新滚动容器。
  const editorPaneRef = useRef<HTMLElement | null>(null);
  // CodeMirror 实例，用于文档偏移 -> 像素位置换算。
  const editorViewRef = useRef<EditorView | null>(null);
  // 预览区滚动容器。
  const previewScrollerRef = useRef<HTMLElement | null>(null);
  // 编辑区 -> 预览区锚点映射表。
  const editorToPreviewAnchorsRef = useRef<DirectionAnchor[]>([]);
  // 预览区 -> 编辑区锚点映射表。
  const previewToEditorAnchorsRef = useRef<DirectionAnchor[]>([]);
  // 映射重建调度句柄（rAF）。
  const rebuildMapRafRef = useRef<number | null>(null);
  // 本轮重建完成后是否需要主动回对齐滚动位置。
  const pendingResyncAfterRebuildRef = useRef(false);
  // 同步滚动补帧句柄：用于分帧追平大跨度映射。
  const syncFollowRafRef = useRef<number | null>(null);
  // 编辑过程中进入“编辑区优先”模式：禁止预览区事件反向推回编辑区。
  const editorDrivenOnlyRef = useRef(false);
  // 编辑区优先模式的释放定时器。
  const editorDrivenOnlyTimerRef = useRef<number | null>(null);
  // 延迟重建定时器集合：用于处理粘贴/批量改动后的异步布局收敛。
  const delayedRebuildTimersRef = useRef<number[]>([]);
  // 记录上一次编辑区滚动位置，用于识别滚动条拖拽等大跨度跳转。
  const lastObservedEditorScrollTopRef = useRef<number | null>(null);
  // 上一次内容快照：用于判断本次改动是否属于“大幅变更”（如整段粘贴）。
  const previousRenderedContentSnapshotRef = useRef(renderedContent);
  // 防止双向同步引发循环滚动。
  const syncingRef = useRef(false);
  // 记录最近一次主动滚动来源，用于重算后回对齐。
  const lastScrollSourceRef = useRef<ScrollSource>("editor");

  // 统一更新编辑区滚动容器引用，避免重复 setState 触发不必要重渲染。
  const setEditorScrollerNode = useCallback((node: HTMLElement | null) => {
    editorScrollerRef.current = node;
    setEditorScrollerElement((previous) => (previous === node ? previous : node));
  }, []);

  // 编辑器面板 ref：用于感知 CodeMirror 在 StrictMode/HMR 下的重挂载。
  const handleEditorPaneRef = useCallback(
    (node: HTMLElement | null) => {
      // 维护 ref 版本，便于在 useCallback 闭包中读取最新面板节点。
      editorPaneRef.current = node;
      setEditorPaneElement((previous) => (previous === node ? previous : node));
      // 面板卸载时清空滚动容器引用，防止监听器挂在旧节点上。
      if (!node) {
        setEditorScrollerNode(null);
      }
    },
    [setEditorScrollerNode]
  );

  // 预览区 ref 采用稳定回调，避免每次渲染都触发 null -> node 抖动。
  const handlePreviewScrollerRef = useCallback((node: HTMLElement | null) => {
    previewScrollerRef.current = node;
    setPreviewScrollerElement((previous) => (previous === node ? previous : node));
  }, []);

  // 获取“当前仍可用”的编辑区滚动容器：优先用 ref，其次回退到编辑器面板查询。
  const resolveLiveEditorScroller = useCallback((): HTMLElement | null => {
    const currentScroller = editorScrollerRef.current;
    if (currentScroller && currentScroller.isConnected) {
      return currentScroller;
    }
    const fallbackScroller = editorPaneRef.current?.querySelector<HTMLElement>(".cm-scroller") ?? null;
    if (fallbackScroller && fallbackScroller.isConnected) {
      // 回写最新节点，避免后续逻辑继续使用过期引用。
      setEditorScrollerNode(fallbackScroller);
      return fallbackScroller;
    }
    return null;
  }, [setEditorScrollerNode]);

  // 获取“当前仍可用”的 EditorView：优先使用已有实例，失效时从 DOM 反查恢复。
  const resolveLiveEditorView = useCallback((editorScroller: HTMLElement): EditorView | null => {
    const currentEditorView = editorViewRef.current;
    if (currentEditorView && currentEditorView.dom.isConnected) {
      return currentEditorView;
    }
    const recoveredEditorView = EditorView.findFromDOM(editorScroller);
    if (recoveredEditorView && recoveredEditorView.dom.isConnected) {
      // 回写当前实例，避免映射重建持续失败。
      editorViewRef.current = recoveredEditorView;
      return recoveredEditorView;
    }
    return null;
  }, []);

  // 将当前滚动位置转换为滚动比例，用于锚点不足时兜底。
  const getRatio = (element: HTMLElement): number => {
    const maxScrollable = getMaxScrollable(element);
    if (maxScrollable <= 0) {
      return 0;
    }
    return element.scrollTop / maxScrollable;
  };

  // 在单向锚点表上做二分查找 + 分段线性插值。
  const mapScrollWithDirectionAnchors = (
    anchors: DirectionAnchor[],
    sourceY: number
  ): number => {
    if (anchors.length === 0) {
      return 0;
    }
    if (anchors.length === 1) {
      return anchors[0].targetY;
    }

    // 超出边界时直接钳到首尾锚点。
    const first = anchors[0];
    const last = anchors[anchors.length - 1];

    if (sourceY <= first.sourceY) {
      return first.targetY;
    }
    if (sourceY >= last.sourceY) {
      return last.targetY;
    }

    // 二分定位 sourceY 所在区间。
    let left = 0;
    let right = anchors.length - 1;
    while (left + 1 < right) {
      const middle = (left + right) >> 1;
      if (anchors[middle].sourceY <= sourceY) {
        left = middle;
      } else {
        right = middle;
      }
    }

    const start = anchors[left];
    const end = anchors[right];
    const sourceDistance = end.sourceY - start.sourceY;
    // source 轴重合时退到区间终点，保证尾部重合锚点可被命中。
    if (sourceDistance <= 0) {
      return end.targetY;
    }
    // 线性插值计算目标滚动位置。
    const progress = (sourceY - start.sourceY) / sourceDistance;
    return start.targetY + progress * (end.targetY - start.targetY);
  };

  // 根据当前来源区域，计算目标区域应设置的 scrollTop。
  const getMappedTargetScrollTop = useCallback(
    (sourceName: ScrollSource): number => {
      const editorElement = resolveLiveEditorScroller();
      const previewElement = previewScrollerRef.current;
      if (!editorElement || !previewElement) {
        return 0;
      }

      if (sourceName === "editor") {
        const previewMaxScrollable = getMaxScrollable(previewElement);
        const anchors = editorToPreviewAnchorsRef.current;
        // 锚点充足时优先走插值。
        if (anchors.length >= 2) {
          return clamp(
            mapScrollWithDirectionAnchors(anchors, editorElement.scrollTop),
            0,
            previewMaxScrollable
          );
        }
        // 锚点不足时退回比例映射。
        return clamp(getRatio(editorElement) * previewMaxScrollable, 0, previewMaxScrollable);
      }

      const editorMaxScrollable = getMaxScrollable(editorElement);
      const anchors = previewToEditorAnchorsRef.current;
      // 预览 -> 编辑同理。
      if (anchors.length >= 2) {
        return clamp(
          mapScrollWithDirectionAnchors(anchors, previewElement.scrollTop),
          0,
          editorMaxScrollable
        );
      }
      return clamp(getRatio(previewElement) * editorMaxScrollable, 0, editorMaxScrollable);
    },
    [resolveLiveEditorScroller]
  );

  // 清理同步补帧任务，避免旧来源任务持续写入滚动位置。
  const clearSyncFollowRaf = useCallback(() => {
    if (syncFollowRafRef.current !== null) {
      window.cancelAnimationFrame(syncFollowRafRef.current);
      syncFollowRafRef.current = null;
    }
  }, []);

  // 清理“编辑区优先”释放定时器。
  const clearEditorDrivenOnlyTimer = useCallback(() => {
    if (editorDrivenOnlyTimerRef.current !== null) {
      window.clearTimeout(editorDrivenOnlyTimerRef.current);
      editorDrivenOnlyTimerRef.current = null;
    }
  }, []);

  // 输入期间强制锁定为“只允许编辑区驱动预览区”，保护编辑区滚动稳定。
  const activateEditorDrivenOnlyMode = useCallback(() => {
    editorDrivenOnlyRef.current = true;
    lastScrollSourceRef.current = "editor";
    clearSyncFollowRaf();
    clearEditorDrivenOnlyTimer();
    editorDrivenOnlyTimerRef.current = window.setTimeout(() => {
      editorDrivenOnlyRef.current = false;
      editorDrivenOnlyTimerRef.current = null;
    }, EDITOR_DRIVEN_SYNC_LOCK_MS);
  }, [clearEditorDrivenOnlyTimer, clearSyncFollowRaf]);

  // 编辑区出现大跨度 scrollTop 变化时，视为滚动条拖拽/快速跳转，目标区域直接吸附。
  const resolveEditorSyncMode = useCallback((editorElement: HTMLElement): SyncMode => {
    const previousTop = lastObservedEditorScrollTopRef.current;
    const currentTop = editorElement.scrollTop;
    lastObservedEditorScrollTopRef.current = currentTop;
    if (previousTop === null) {
      return "smooth";
    }

    const deltaAbs = Math.abs(currentTop - previousTop);
    const viewportThreshold = editorElement.clientHeight * EDITOR_SCROLL_JUMP_VIEWPORT_RATIO;
    const maxScrollable = getMaxScrollable(editorElement);
    const documentThreshold = maxScrollable * EDITOR_SCROLL_JUMP_DOCUMENT_RATIO;
    const jumpThreshold = Math.max(
      EDITOR_SCROLL_JUMP_MIN_PIXELS,
      viewportThreshold,
      documentThreshold
    );

    return deltaAbs >= jumpThreshold ? "snap" : "smooth";
  }, []);

  // 预览区追随编辑区时允许更激进的动态步进，避免拖动滚动条时目标区域落后过多。
  const resolveMaxSyncStep = useCallback((sourceName: ScrollSource, delta: number): number => {
    const deltaAbs = Math.abs(delta);
    if (sourceName !== "editor") {
      return MAX_SYNC_STEP_PER_FRAME;
    }
    if (deltaAbs <= MAX_SYNC_STEP_PER_FRAME) {
      return MAX_SYNC_STEP_PER_FRAME;
    }
    if (deltaAbs >= 2400) {
      return deltaAbs;
    }
    return clamp(deltaAbs * 0.45, MAX_SYNC_STEP_PER_FRAME, 1200);
  }, []);

  // 单帧同步：限制最大步进，避免高斜率区间（如展开 TOC）瞬间跨越。
  const applySyncStep = useCallback(
    (sourceName: ScrollSource, syncMode: SyncMode = "smooth"): boolean => {
      const editorElement = resolveLiveEditorScroller();
      const previewElement = previewScrollerRef.current;
      if (!editorElement || !previewElement) {
        return false;
      }

      const targetElement = sourceName === "editor" ? previewElement : editorElement;
      const mappedTarget = getMappedTargetScrollTop(sourceName);
      const delta = mappedTarget - targetElement.scrollTop;
      if (Math.abs(delta) <= SYNC_SETTLE_THRESHOLD) {
        // 误差足够小时直接吸附到目标，避免小数误差抖动。
        targetElement.scrollTop = mappedTarget;
        return false;
      }
      if (syncMode === "snap") {
        targetElement.scrollTop = clamp(mappedTarget, 0, getMaxScrollable(targetElement));
        return false;
      }

      const maxSyncStep = resolveMaxSyncStep(sourceName, delta);
      const limitedStep = clamp(delta, -maxSyncStep, maxSyncStep);
      const nextTop = clamp(targetElement.scrollTop + limitedStep, 0, getMaxScrollable(targetElement));
      targetElement.scrollTop = nextTop;
      return Math.abs(delta) > maxSyncStep;
    },
    [getMappedTargetScrollTop, resolveLiveEditorScroller, resolveMaxSyncStep]
  );

  // 若目标位移过大，继续按帧追平，保证视觉连续而不是一次跳跃。
  const scheduleFollowSync = useCallback(
    (sourceName: ScrollSource) => {
      if (syncFollowRafRef.current !== null) {
        return;
      }

      const follow = () => {
        syncFollowRafRef.current = null;
        if (syncingRef.current) {
          syncFollowRafRef.current = window.requestAnimationFrame(follow);
          return;
        }
        // 来源已变化时停止旧任务，避免跟当前用户输入“打架”。
        if (lastScrollSourceRef.current !== sourceName) {
          return;
        }

        syncingRef.current = true;
        let shouldContinue = false;
        try {
          shouldContinue = applySyncStep(sourceName);
        } catch {
          // 同步过程中发生异常时立即释放锁，避免后续滚动全部失效。
          shouldContinue = false;
        }
        window.requestAnimationFrame(() => {
          syncingRef.current = false;
          if (shouldContinue && lastScrollSourceRef.current === sourceName) {
            syncFollowRafRef.current = window.requestAnimationFrame(follow);
          }
        });
      };

      syncFollowRafRef.current = window.requestAnimationFrame(follow);
    },
    [applySyncStep]
  );

  // 执行一次单向同步，并用锁避免对端 scroll 反向触发。
  const syncFromSource = useCallback(
    (sourceName: ScrollSource, syncMode: SyncMode = "smooth") => {
      // 编辑阶段禁止预览区滚动事件反向推回编辑区，优先保证输入稳定。
      if (sourceName === "preview" && editorDrivenOnlyRef.current) {
        return;
      }
      if (syncingRef.current) {
        return;
      }
      const editorElement = resolveLiveEditorScroller();
      const previewElement = previewScrollerRef.current;
      if (!editorElement || !previewElement) {
        return;
      }

      // 新输入到来时取消旧补帧任务，优先响应当前滚动来源。
      clearSyncFollowRaf();
      // 同步阶段写入锁并记录来源。
      syncingRef.current = true;
      lastScrollSourceRef.current = sourceName;
      let shouldFollow = false;
      try {
        shouldFollow = applySyncStep(sourceName, syncMode);
      } catch {
        // 防止异常导致锁永远不释放。
        shouldFollow = false;
      }
      window.requestAnimationFrame(() => {
        syncingRef.current = false;
        if (shouldFollow && syncMode !== "snap") {
          scheduleFollowSync(sourceName);
        }
      });
    },
    [applySyncStep, clearSyncFollowRaf, resolveLiveEditorScroller, scheduleFollowSync]
  );

  // 映射表重建后，按最近来源做一次回对齐。
  const resyncFromLastSource = useCallback(() => {
    if (syncingRef.current) {
      return;
    }
    const editorElement = resolveLiveEditorScroller();
    const previewElement = previewScrollerRef.current;
    if (!editorElement || !previewElement) {
      return;
    }
    syncingRef.current = true;
    const sourceName = editorDrivenOnlyRef.current ? "editor" : lastScrollSourceRef.current;
    let shouldFollow = false;
    try {
      shouldFollow = applySyncStep(sourceName);
    } catch {
      // 防止异常导致锁永远不释放。
      shouldFollow = false;
    }
    window.requestAnimationFrame(() => {
      syncingRef.current = false;
      if (shouldFollow) {
        scheduleFollowSync(sourceName);
      }
    });
  }, [applySyncStep, resolveLiveEditorScroller, scheduleFollowSync]);

  // 重建 block 级锚点映射表：source offset -> editorY 与 previewY。
  const rebuildScrollAnchors = useCallback(() => {
    const editorElement = resolveLiveEditorScroller();
    const previewElement = previewScrollerRef.current;
    const editorView = editorElement ? resolveLiveEditorView(editorElement) : null;
    // editorView 可能在 StrictMode 旧实例卸载后短暂失效，需等待新实例就绪。
    if (!editorElement || !previewElement || !editorView || !editorView.dom.isConnected) {
      editorToPreviewAnchorsRef.current = [];
      previewToEditorAnchorsRef.current = [];
      return;
    }

    const editorMaxScrollable = getMaxScrollable(editorElement);
    const previewMaxScrollable = getMaxScrollable(previewElement);
    const docLength = editorView.state.doc.length;
    const previewRect = previewElement.getBoundingClientRect();
    // 读取所有被 remark 注入的锚点节点。
    const anchorNodes = previewElement.querySelectorAll<HTMLElement>(BLOCK_ANCHOR_SELECTOR);
    // 先收集原始锚点，再分别构建双向映射表。
    const rawAnchors: ScrollAnchor[] = [];
    // 已处理锚点序号集合：用于去重被渲染器复制到子节点的重复锚点。
    const seenAnchorIndices = new Set<string>();

    // 统一解析 block 的起止源码位置，映射为编辑区像素坐标。
    const resolveEditorAnchorY = (
      rawLine: string | undefined,
      rawOffset: string | undefined,
      anchorEdge: "start" | "end"
    ): number | null => {
      if (rawLine) {
        const parsedLine = Number(rawLine);
        if (Number.isFinite(parsedLine)) {
          const lineNumber = clamp(Math.floor(parsedLine), 1, editorView.state.doc.lines);
          const lineFrom = editorView.state.doc.line(lineNumber).from;
          const lineBlock = editorView.lineBlockAt(lineFrom);
          return clamp(
            anchorEdge === "end" ? lineBlock.bottom : lineBlock.top,
            0,
            editorMaxScrollable
          );
        }
      }
      if (!rawOffset) {
        return null;
      }
      const parsedOffset = Number(rawOffset);
      if (!Number.isFinite(parsedOffset)) {
        return null;
      }
      const normalizedOffset = clamp(Math.floor(parsedOffset), 0, docLength);
      // end offset 在 AST 语义上通常指向“块后一个字符”，这里回退 1 以命中块末尾行。
      const anchorOffset =
        anchorEdge === "end" ? clamp(normalizedOffset - 1, 0, docLength) : normalizedOffset;
      const lineBlock = editorView.lineBlockAt(anchorOffset);
      return clamp(
        anchorEdge === "end" ? lineBlock.bottom : lineBlock.top,
        0,
        editorMaxScrollable
      );
    };

    for (const node of anchorNodes) {
      const anchorIndex = node.dataset.anchorIndex;
      // 仅使用插件生成的锚点，避免误采集到渲染库内部节点。
      if (!anchorIndex) {
        continue;
      }
      if (seenAnchorIndices.has(anchorIndex)) {
        continue;
      }
      seenAnchorIndices.add(anchorIndex);

      const rawStartLine = node.dataset.sourceLine;
      const rawStartOffset = node.dataset.sourceOffset;
      const rawEndLine = node.dataset.sourceEndLine;
      const rawEndOffset = node.dataset.sourceEndOffset;

      const editorStartY = resolveEditorAnchorY(rawStartLine, rawStartOffset, "start");
      if (editorStartY === null) {
        continue;
      }
      const editorEndYCandidate = resolveEditorAnchorY(rawEndLine, rawEndOffset, "end");
      const editorEndY =
        editorEndYCandidate === null ? editorStartY : Math.max(editorStartY, editorEndYCandidate);

      // 将节点视口坐标转换为容器内容坐标，并为块底部补一组锚点。
      const nodeRect = node.getBoundingClientRect();
      const previewStartY = clamp(
        nodeRect.top - previewRect.top + previewElement.scrollTop,
        0,
        previewMaxScrollable
      );
      const previewEndY = clamp(
        nodeRect.bottom - previewRect.top + previewElement.scrollTop,
        0,
        previewMaxScrollable
      );

      rawAnchors.push({
        editorY: editorStartY,
        previewY: previewStartY
      });
      // 对可见高度大于一行的 block（如块状公式）补充底部锚点，避免滚动跨越。
      if (
        editorEndYCandidate !== null &&
        editorEndY > editorStartY &&
        previewEndY > previewStartY
      ) {
        rawAnchors.push({
          editorY: editorEndY,
          previewY: Math.max(previewStartY, previewEndY)
        });
      }
    }

    // 构建编辑区 -> 预览区映射：同 editorY 聚合到最远 previewY。
    editorToPreviewAnchorsRef.current = buildDirectionAnchors(
      rawAnchors.map((anchor) => ({
        sourceY: anchor.editorY,
        targetY: anchor.previewY
      })),
      editorMaxScrollable,
      previewMaxScrollable
    );

    // 构建预览区 -> 编辑区映射：同 previewY 聚合到最远 editorY。
    previewToEditorAnchorsRef.current = buildDirectionAnchors(
      rawAnchors.map((anchor) => ({
        sourceY: anchor.previewY,
        targetY: anchor.editorY
      })),
      previewMaxScrollable,
      editorMaxScrollable
    );
  }, [resolveLiveEditorScroller, resolveLiveEditorView]);

  // 用 requestAnimationFrame 合并多次重建请求，降低重排频率。
  const scheduleRebuildScrollAnchors = useCallback((shouldResync = true) => {
    pendingResyncAfterRebuildRef.current =
      pendingResyncAfterRebuildRef.current || shouldResync;
    if (rebuildMapRafRef.current !== null) {
      return;
    }

    rebuildMapRafRef.current = window.requestAnimationFrame(() => {
      rebuildMapRafRef.current = null;
      const shouldApplyResync = pendingResyncAfterRebuildRef.current;
      pendingResyncAfterRebuildRef.current = false;
      rebuildScrollAnchors();
      if (shouldApplyResync) {
        resyncFromLastSource();
      }
    });
  }, [rebuildScrollAnchors, resyncFromLastSource]);

  // 清理所有延迟重建任务，避免重复排队造成无效重建。
  const clearDelayedRebuildTimers = useCallback(() => {
    for (const timerId of delayedRebuildTimersRef.current) {
      window.clearTimeout(timerId);
    }
    delayedRebuildTimersRef.current = [];
  }, []);

  // 追加多次延迟重建：覆盖粘贴后编辑器/预览异步布局更新窗口。
  const scheduleDelayedRebuilds = useCallback(
    (delays: number[]) => {
      clearDelayedRebuildTimers();
      delayedRebuildTimersRef.current = delays.map((delay) => {
        const scheduledTimerId = window.setTimeout(() => {
          scheduleRebuildScrollAnchors();
          // 执行后移出记录，避免数组持续增长。
          delayedRebuildTimersRef.current = delayedRebuildTimersRef.current.filter(
            (timerId) => timerId !== scheduledTimerId
          );
        }, delay);
        return scheduledTimerId;
      });
    },
    [clearDelayedRebuildTimers, scheduleRebuildScrollAnchors]
  );

  // 当滚动容器就绪后重建一次映射，避免首屏阶段因时序问题拿到空锚点。
  useEffect(() => {
    if (!editorScrollerElement || !previewScrollerElement) {
      return;
    }
    scheduleRebuildScrollAnchors();
  }, [editorScrollerElement, previewScrollerElement, scheduleRebuildScrollAnchors]);

  // 监听编辑器面板中的 DOM 变化，确保滚动容器引用始终指向“当前活跃实例”。
  useEffect(() => {
    if (!editorPaneElement) {
      return;
    }

    const refreshEditorScroller = () => {
      const currentScroller = editorPaneElement.querySelector<HTMLElement>(".cm-scroller");
      // 仅接受仍在文档中的节点，避免绑定到已销毁实例。
      if (currentScroller && currentScroller.isConnected) {
        setEditorScrollerNode(currentScroller);
        return;
      }
      setEditorScrollerNode(null);
    };

    refreshEditorScroller();

    let mutationObserver: MutationObserver | null = null;
    if (typeof MutationObserver !== "undefined") {
      mutationObserver = new MutationObserver(() => {
        refreshEditorScroller();
      });
      mutationObserver.observe(editorPaneElement, {
        childList: true,
        subtree: true
      });
    }

    return () => {
      mutationObserver?.disconnect();
    };
  }, [editorPaneElement, setEditorScrollerNode]);

  // 绑定编辑区与预览区滚动事件，触发单向同步。
  useEffect(() => {
    if (!editorScrollerElement || !previewScrollerElement) {
      return;
    }
    lastObservedEditorScrollTopRef.current = editorScrollerElement.scrollTop;

    const onEditorScroll = () => {
      const liveEditorScroller = resolveLiveEditorScroller();
      const syncMode = liveEditorScroller ? resolveEditorSyncMode(liveEditorScroller) : "smooth";
      syncFromSource("editor", syncMode);
    };

    const onPreviewScroll = () => {
      syncFromSource("preview");
    };

    editorScrollerElement.addEventListener("scroll", onEditorScroll, { passive: true });
    previewScrollerElement.addEventListener("scroll", onPreviewScroll, { passive: true });

    return () => {
      editorScrollerElement.removeEventListener("scroll", onEditorScroll);
      previewScrollerElement.removeEventListener("scroll", onPreviewScroll);
    };
  }, [
    editorScrollerElement,
    previewScrollerElement,
    resolveEditorSyncMode,
    resolveLiveEditorScroller,
    syncFromSource
  ]);

  // 编辑发生后立即进入“编辑区优先”窗口，避免预览区事件在输入期间反向抢焦点。
  useEffect(() => {
    activateEditorDrivenOnlyMode();
  }, [activateEditorDrivenOnlyMode, content]);

  // 当前已渲染的预览内容变更后，再重建锚点映射，避免对着旧 DOM 提前计算位置。
  useEffect(() => {
    // 判断是否为批量变更（例如一次性粘贴长文）；批量变更时增加延迟重建兜底。
    const previousContent = previousRenderedContentSnapshotRef.current;
    const currentContent = renderedContent;
    const characterDelta = Math.abs(currentContent.length - previousContent.length);
    const isBulkContentChange = characterDelta >= 120;
    previousRenderedContentSnapshotRef.current = currentContent;

    scheduleRebuildScrollAnchors();
    if (isBulkContentChange) {
      // 多轮重建用于覆盖 CodeMirror 重排、图片加载与高亮渲染的延迟窗口。
      scheduleDelayedRebuilds([80, 240, 520]);
    }
  }, [renderedContent, scheduleDelayedRebuilds, scheduleRebuildScrollAnchors]);

  // 处理“大段粘贴”场景：粘贴后追加延迟重建，覆盖图片与布局异步更新窗口。
  useEffect(() => {
    const editorElement = editorScrollerElement;
    if (!editorElement) {
      return;
    }

    // 记录本次粘贴触发的延迟任务，组件卸载或重复粘贴时统一清理。
    const pendingTimers = new Set<number>();

    const onPaste = () => {
      // 粘贴本质也是输入，进入“编辑区优先”窗口。
      activateEditorDrivenOnlyMode();
      // 第一次重建：尽快刷新映射，保证初次滚动就可同步。
      scheduleRebuildScrollAnchors();
      // 第二次重建：等待 CodeMirror 完成一轮布局更新。
      const timerAfterLayout = window.setTimeout(() => {
        pendingTimers.delete(timerAfterLayout);
        scheduleRebuildScrollAnchors();
      }, 60);
      pendingTimers.add(timerAfterLayout);
      // 第三次重建：兜底等待图片尺寸与高亮等异步渲染完成。
      const timerAfterAsyncRender = window.setTimeout(() => {
        pendingTimers.delete(timerAfterAsyncRender);
        scheduleRebuildScrollAnchors();
      }, 220);
      pendingTimers.add(timerAfterAsyncRender);
    };

    // 使用捕获阶段监听 paste，规避 CodeMirror 在冒泡阶段拦截事件导致监听不到。
    editorElement.addEventListener("paste", onPaste, true);
    return () => {
      editorElement.removeEventListener("paste", onPaste, true);
      for (const timerId of pendingTimers) {
        window.clearTimeout(timerId);
      }
      pendingTimers.clear();
    };
  }, [activateEditorDrivenOnlyMode, editorScrollerElement, scheduleRebuildScrollAnchors]);

  // 主题样式或外部覆盖样式变化后，主动重建锚点映射，避免滚动同步漂移。
  useEffect(() => {
    scheduleRebuildScrollAnchors();
  }, [
    previewThemeClassName,
    customPreviewStyleText,
    previewViewportMode,
    scheduleRebuildScrollAnchors
  ]);

  // 监听图片异步加载与容器尺寸变化，保障长图场景下映射实时更新。
  useEffect(() => {
    const previewElement = previewScrollerElement;
    const editorElement = editorScrollerElement;
    if (!previewElement || !editorElement) {
      return;
    }

    // 单图事件处理器：任意图片完成/失败都触发锚点重建。
    const onImageEvent = () => {
      scheduleRebuildScrollAnchors(false);
    };

    // 维护已绑定的图片集合，避免重复绑定监听器。
    const boundImages = new Set<HTMLImageElement>();

    // 为当前预览区图片绑定监听；若图片已完成加载则立即触发一次重建。
    const refreshImageBindings = () => {
      const currentImages = Array.from(previewElement.querySelectorAll<HTMLImageElement>("img"));
      const currentImageSet = new Set(currentImages);

      for (const image of currentImages) {
        if (boundImages.has(image)) {
          continue;
        }
        image.addEventListener("load", onImageEvent);
        image.addEventListener("error", onImageEvent);
        boundImages.add(image);
        // 处理缓存命中场景：已完成图片不会再次触发 load，需要主动重建映射。
        if (image.complete) {
          scheduleRebuildScrollAnchors(false);
        }
      }

      for (const image of boundImages) {
        if (currentImageSet.has(image)) {
          continue;
        }
        image.removeEventListener("load", onImageEvent);
        image.removeEventListener("error", onImageEvent);
        boundImages.delete(image);
      }
    };

    refreshImageBindings();

    let resizeObserver: ResizeObserver | null = null;
    if (typeof ResizeObserver !== "undefined") {
      resizeObserver = new ResizeObserver(() => {
        // 任意尺寸变化都触发一次合并后的重建。
        scheduleRebuildScrollAnchors(false);
      });
      resizeObserver.observe(editorElement);
      // 直接观察预览滚动容器尺寸变化。
      resizeObserver.observe(previewElement);
      const markdownBody = previewElement.querySelector<HTMLElement>(PREVIEW_BODY_SELECTOR);
      if (markdownBody) {
        resizeObserver.observe(markdownBody);
      }
    }

    let mutationObserver: MutationObserver | null = null;
    if (typeof MutationObserver !== "undefined") {
      mutationObserver = new MutationObserver(() => {
        // 预览 DOM 变更（例如图片节点替换）后刷新监听并重建映射。
        refreshImageBindings();
        scheduleRebuildScrollAnchors(false);
      });
      mutationObserver.observe(previewElement, {
        childList: true,
        subtree: true
      });
    }

    return () => {
      for (const image of boundImages) {
        image.removeEventListener("load", onImageEvent);
        image.removeEventListener("error", onImageEvent);
      }
      resizeObserver?.disconnect();
      mutationObserver?.disconnect();
    };
  }, [editorScrollerElement, previewScrollerElement, scheduleRebuildScrollAnchors]);

  // 卸载时取消未执行的重建任务，避免悬挂回调。
  useEffect(() => {
    return () => {
      if (rebuildMapRafRef.current !== null) {
        window.cancelAnimationFrame(rebuildMapRafRef.current);
      }
      clearSyncFollowRaf();
      clearEditorDrivenOnlyTimer();
      clearDelayedRebuildTimers();
    };
  }, [clearDelayedRebuildTimers, clearEditorDrivenOnlyTimer, clearSyncFollowRaf]);

  // 接收 CodeMirror 初始化回调，记录滚动容器并触发首次重建。
  const handleEditorCreate = useCallback(
    (view: EditorView) => {
      editorViewRef.current = view;
      setEditorScrollerNode(view.scrollDOM);
      scheduleRebuildScrollAnchors();
    },
    [scheduleRebuildScrollAnchors, setEditorScrollerNode]
  );

  // 根据目录条目滚动预览区，让同步滚动机制继续驱动编辑区对齐。
  const handleTocNavigate = useCallback((item: TocItem) => {
    const previewElement = previewScrollerRef.current;
    if (!previewElement) {
      return;
    }
    scrollPreviewToTocItem(previewElement, item);
  }, []);

  return {
    handleEditorPaneRef,
    handlePreviewScrollerRef,
    handleEditorCreate,
    handleTocNavigate
  };
}
