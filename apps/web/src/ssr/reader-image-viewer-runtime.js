const PREVIEW_BODY_SELECTOR = "#plaindoc-preview-body";
const VIEWER_ROOT_SELECTOR = "[data-reader-hook='image-viewer']";
const VIEWER_BACKDROP_SELECTOR = "[data-reader-hook='image-viewer-backdrop']";
const VIEWER_STAGE_SELECTOR = "[data-reader-hook='image-viewer-stage']";
const VIEWER_CONTENT_SELECTOR = "[data-reader-hook='image-viewer-content']";
const VIEWER_CLOSE_SELECTOR = "[data-reader-hook='image-viewer-close']";
const VIEWER_PREV_SELECTOR = "[data-reader-hook='image-viewer-prev']";
const VIEWER_NEXT_SELECTOR = "[data-reader-hook='image-viewer-next']";
const VIEWER_ZOOM_OUT_SELECTOR = "[data-reader-hook='image-viewer-zoom-out']";
const VIEWER_ZOOM_IN_SELECTOR = "[data-reader-hook='image-viewer-zoom-in']";
const VIEWER_ORIGINAL_SELECTOR = "[data-reader-hook='image-viewer-original']";
const VIEWER_ROTATE_SELECTOR = "[data-reader-hook='image-viewer-rotate']";
const VIEWER_INDEX_SELECTOR = "[data-reader-hook='image-viewer-index']";
const VIEWER_SCALE_SELECTOR = "[data-reader-hook='image-viewer-scale']";
const VIEWER_TOOLBAR_SELECTOR = ".reader-image-viewer__toolbar";
const VIEWER_MEDIA_FRAME_SELECTOR = ".reader-image-viewer__media-frame";
const VIEWER_TARGET_ATTRIBUTE = "data-reader-image-viewer-target";
const VIEWER_ITEM_INDEX_ATTRIBUTE = "data-reader-image-viewer-index";
const VIEWER_OPEN_BODY_CLASS = "reader-image-viewer-open";
const GALLERY_ITEM_SELECTOR = `[${VIEWER_TARGET_ATTRIBUTE}='1']`;
const MERMAID_DIAGRAM_SELECTOR = "[data-reader-hook='mermaid'] [data-reader-mermaid-diagram='1'] svg";
const SVG_MIN_VIEWER_EDGE = 32;
const ZOOM_STEP_MULTIPLIER = 1.2;
const MIN_ZOOM_SCALE = 0.2;
const MAX_ZOOM_SCALE = 8;
const ROTATION_STEP_DEGREES = 90;
const READER_IMAGE_VIEWER_RUNTIME_GLOBAL_KEY = "__plaindocReaderImageViewerRuntime__";
const DRAG_OVERFLOW_PADDING = 48;
const DOUBLE_CLICK_TOGGLE_EPSILON = 0.01;

const state = {
  initialized: false,
  root: null,
  stage: null,
  content: null,
  indexNode: null,
  scaleNode: null,
  prevButton: null,
  nextButton: null,
  zoomOutButton: null,
  zoomInButton: null,
  originalButton: null,
  rotateButton: null,
  items: [],
  open: false,
  activeIndex: -1,
  zoom: 1,
  fitScale: 1,
  rotation: 0,
  panX: 0,
  panY: 0,
  dragging: false,
  dragMoved: false,
  dragStartX: 0,
  dragStartY: 0,
  dragOriginPanX: 0,
  dragOriginPanY: 0,
  suppressNextStageClick: false
};

function clamp(value, minValue, maxValue) {
  return Math.min(maxValue, Math.max(minValue, value));
}

function normalizeRotation(value) {
  const normalized = value % 360;
  return normalized < 0 ? normalized + 360 : normalized;
}

function readSVGLength(element, attributeName) {
  const rawValue = element.getAttribute(attributeName);
  if (!rawValue) {
    return Number.NaN;
  }
  const parsedValue = Number.parseFloat(rawValue);
  return Number.isFinite(parsedValue) ? parsedValue : Number.NaN;
}

function resolveSVGDimensions(svgElement) {
  const viewBox = svgElement.viewBox?.baseVal;
  if (viewBox && Number.isFinite(viewBox.width) && Number.isFinite(viewBox.height) && viewBox.width > 0 && viewBox.height > 0) {
    return {
      width: viewBox.width,
      height: viewBox.height
    };
  }

  const width = readSVGLength(svgElement, "width");
  const height = readSVGLength(svgElement, "height");
  if (Number.isFinite(width) && width > 0 && Number.isFinite(height) && height > 0) {
    return {
      width,
      height
    };
  }

  const rect = svgElement.getBoundingClientRect();
  if (rect.width > 0 && rect.height > 0) {
    return {
      width: rect.width,
      height: rect.height
    };
  }

  return {
    width: 320,
    height: 240
  };
}

function resolveImageDimensions(imageElement) {
  const width = imageElement.naturalWidth || imageElement.width || imageElement.getBoundingClientRect().width;
  const height = imageElement.naturalHeight || imageElement.height || imageElement.getBoundingClientRect().height;
  if (width > 0 && height > 0) {
    return { width, height };
  }
  return {
    width: 320,
    height: 240
  };
}

function updateItemDimensions(item) {
  if (!item) {
    return item;
  }
  if (item.kind === "img" && item.element instanceof HTMLImageElement) {
    const dimensions = resolveImageDimensions(item.element);
    item.width = dimensions.width;
    item.height = dimensions.height;
    return item;
  }
  if ((item.kind === "svg" || item.kind === "mermaid") && item.element instanceof SVGSVGElement) {
    const dimensions = resolveSVGDimensions(item.element);
    item.width = dimensions.width;
    item.height = dimensions.height;
  }
  return item;
}

function findPreviewBody(root = document) {
  return root.querySelector(PREVIEW_BODY_SELECTOR);
}

function matchesIgnoredSVG(svgElement, previewBody) {
  if (!(svgElement instanceof SVGSVGElement) || !(previewBody instanceof HTMLElement)) {
    return true;
  }
  if (!previewBody.contains(svgElement)) {
    return true;
  }
  if (svgElement.matches(MERMAID_DIAGRAM_SELECTOR)) {
    return false;
  }
  if (svgElement.closest("button, [role='button'], .reader-image-viewer, .reader-article-actions, .reader-mobile-bar")) {
    return true;
  }
  const dimensions = resolveSVGDimensions(svgElement);
  return dimensions.width < SVG_MIN_VIEWER_EDGE || dimensions.height < SVG_MIN_VIEWER_EDGE;
}

function collectGalleryItems(root = document) {
  const previewBody = findPreviewBody(root);
  if (!(previewBody instanceof HTMLElement)) {
    return [];
  }

  const items = [];
  let nextIndex = 0;
  const pushItem = (element, kind, dimensions, altText) => {
    if (!(element instanceof Element)) {
      return;
    }
    element.setAttribute(VIEWER_TARGET_ATTRIBUTE, "1");
    element.setAttribute(VIEWER_ITEM_INDEX_ATTRIBUTE, String(nextIndex));
    items.push({
      id: `${kind}-${nextIndex}`,
      kind,
      element,
      altText,
      width: dimensions.width,
      height: dimensions.height
    });
    nextIndex += 1;
  };

  const imageNodes = Array.from(previewBody.querySelectorAll("img")).filter(
    (node) => node instanceof HTMLImageElement
  );
  for (const imageNode of imageNodes) {
    if (!(imageNode instanceof HTMLImageElement)) {
      continue;
    }
    pushItem(
      imageNode,
      "img",
      resolveImageDimensions(imageNode),
      (imageNode.getAttribute("alt") || "").trim()
    );
  }

  const mermaidSVGNodes = Array.from(previewBody.querySelectorAll(MERMAID_DIAGRAM_SELECTOR)).filter(
    (node) => node instanceof SVGSVGElement
  );
  for (const svgNode of mermaidSVGNodes) {
    if (!(svgNode instanceof SVGSVGElement)) {
      continue;
    }
    pushItem(svgNode, "mermaid", resolveSVGDimensions(svgNode), "Mermaid 图表");
  }

  const svgNodes = Array.from(previewBody.querySelectorAll("svg")).filter(
    (node) => node instanceof SVGSVGElement
  );
  for (const svgNode of svgNodes) {
    if (!(svgNode instanceof SVGSVGElement)) {
      continue;
    }
    if (svgNode.matches(MERMAID_DIAGRAM_SELECTOR)) {
      continue;
    }
    if (matchesIgnoredSVG(svgNode, previewBody)) {
      svgNode.removeAttribute(VIEWER_TARGET_ATTRIBUTE);
      svgNode.removeAttribute(VIEWER_ITEM_INDEX_ATTRIBUTE);
      continue;
    }
    pushItem(svgNode, "svg", resolveSVGDimensions(svgNode), "SVG 图片");
  }

  return items;
}

function clearStaleTargets(root = document) {
  const staleNodes = Array.from(root.querySelectorAll(GALLERY_ITEM_SELECTOR));
  for (const node of staleNodes) {
    if (!(node instanceof Element)) {
      continue;
    }
    node.removeAttribute(VIEWER_TARGET_ATTRIBUTE);
    node.removeAttribute(VIEWER_ITEM_INDEX_ATTRIBUTE);
  }
}

function ensureViewerNodes(root = document) {
  if (
    state.root instanceof HTMLElement &&
    state.root.isConnected &&
    state.stage instanceof HTMLElement &&
    state.stage.isConnected &&
    state.content instanceof HTMLElement &&
    state.content.isConnected
  ) {
    return true;
  }
  state.root = null;
  state.stage = null;
  state.content = null;
  state.indexNode = null;
  state.scaleNode = null;
  state.prevButton = null;
  state.nextButton = null;
  state.zoomOutButton = null;
  state.zoomInButton = null;
  state.originalButton = null;
  state.rotateButton = null;
  const rootNode = root.querySelector(VIEWER_ROOT_SELECTOR);
  const stageNode = root.querySelector(VIEWER_STAGE_SELECTOR);
  const contentNode = root.querySelector(VIEWER_CONTENT_SELECTOR);
  const indexNode = root.querySelector(VIEWER_INDEX_SELECTOR);
  const scaleNode = root.querySelector(VIEWER_SCALE_SELECTOR);
  const prevButton = root.querySelector(VIEWER_PREV_SELECTOR);
  const nextButton = root.querySelector(VIEWER_NEXT_SELECTOR);
  const zoomOutButton = root.querySelector(VIEWER_ZOOM_OUT_SELECTOR);
  const zoomInButton = root.querySelector(VIEWER_ZOOM_IN_SELECTOR);
  const originalButton = root.querySelector(VIEWER_ORIGINAL_SELECTOR);
  const rotateButton = root.querySelector(VIEWER_ROTATE_SELECTOR);
  if (
    !(rootNode instanceof HTMLElement) ||
    !(stageNode instanceof HTMLElement) ||
    !(contentNode instanceof HTMLElement) ||
    !(indexNode instanceof HTMLElement) ||
    !(scaleNode instanceof HTMLElement) ||
    !(prevButton instanceof HTMLButtonElement) ||
    !(nextButton instanceof HTMLButtonElement) ||
    !(zoomOutButton instanceof HTMLButtonElement) ||
    !(zoomInButton instanceof HTMLButtonElement) ||
    !(originalButton instanceof HTMLButtonElement) ||
    !(rotateButton instanceof HTMLButtonElement)
  ) {
    return false;
  }
  state.root = rootNode;
  state.stage = stageNode;
  state.content = contentNode;
  state.indexNode = indexNode;
  state.scaleNode = scaleNode;
  state.prevButton = prevButton;
  state.nextButton = nextButton;
  state.zoomOutButton = zoomOutButton;
  state.zoomInButton = zoomInButton;
  state.originalButton = originalButton;
  state.rotateButton = rotateButton;
  return true;
}

function renderEmptyState() {
  if (!(state.content instanceof HTMLElement)) {
    return;
  }
  state.content.innerHTML = "";
}

function formatScaleLabel(scaleValue) {
  return `${Math.round(scaleValue * 100)}%`;
}

function updateToolbarState() {
  if (
    !(state.indexNode instanceof HTMLElement) ||
    !(state.scaleNode instanceof HTMLElement) ||
    !(state.prevButton instanceof HTMLButtonElement) ||
    !(state.nextButton instanceof HTMLButtonElement) ||
    !(state.zoomOutButton instanceof HTMLButtonElement) ||
    !(state.zoomInButton instanceof HTMLButtonElement) ||
    !(state.originalButton instanceof HTMLButtonElement) ||
    !(state.rotateButton instanceof HTMLButtonElement)
  ) {
    return;
  }

  const total = state.items.length;
  const current = state.activeIndex >= 0 ? state.activeIndex + 1 : 0;
  state.indexNode.textContent = `${current}/${total}`;
  state.scaleNode.textContent = formatScaleLabel(state.zoom);

  const hasItems = total > 0 && state.activeIndex >= 0;
  const canNavigate = total > 1;
  state.prevButton.disabled = !hasItems || !canNavigate;
  state.nextButton.disabled = !hasItems || !canNavigate;
  state.zoomOutButton.disabled = !hasItems || state.zoom <= MIN_ZOOM_SCALE + 0.001;
  state.zoomInButton.disabled = !hasItems || state.zoom >= MAX_ZOOM_SCALE - 0.001;
  state.originalButton.disabled = !hasItems;
  state.rotateButton.disabled = !hasItems;
}

function resolveActiveItem() {
  if (state.activeIndex < 0 || state.activeIndex >= state.items.length) {
    return null;
  }
  return state.items[state.activeIndex];
}

function computeFitScale(item) {
  if (!(state.stage instanceof HTMLElement) || !item) {
    return 1;
  }
  const stageRect = state.stage.getBoundingClientRect();
  const maxWidth = Math.max(1, stageRect.width - DRAG_OVERFLOW_PADDING);
  const maxHeight = Math.max(1, stageRect.height - DRAG_OVERFLOW_PADDING);
  const widthScale = maxWidth / Math.max(1, item.width);
  const heightScale = maxHeight / Math.max(1, item.height);
  return clamp(Math.min(1, widthScale, heightScale), MIN_ZOOM_SCALE, MAX_ZOOM_SCALE);
}

function resolveDisplayedSize(item, zoom = state.zoom, rotation = state.rotation) {
  if (!item) {
    return { width: 0, height: 0 };
  }
  const normalizedRotation = normalizeRotation(rotation);
  const swapAxes = normalizedRotation === 90 || normalizedRotation === 270;
  const baseWidth = swapAxes ? item.height : item.width;
  const baseHeight = swapAxes ? item.width : item.height;
  return {
    width: baseWidth * zoom,
    height: baseHeight * zoom
  };
}

function applyViewportTransform() {
  const frameNode = state.content?.querySelector(".reader-image-viewer__media-frame");
  if (!(frameNode instanceof HTMLElement)) {
    syncDraggingState();
    updateToolbarState();
    return;
  }
  frameNode.style.transform =
    `translate(${state.panX}px, ${state.panY}px) scale(${state.zoom}) rotate(${state.rotation}deg)`;
  syncDraggingState();
  updateToolbarState();
}

function resolveFrameCenter() {
  const frameNode = state.content?.querySelector(".reader-image-viewer__media-frame");
  if (!(frameNode instanceof HTMLElement)) {
    return null;
  }
  const frameRect = frameNode.getBoundingClientRect();
  return {
    x: frameRect.left + frameRect.width / 2,
    y: frameRect.top + frameRect.height / 2
  };
}

function syncDraggingState() {
  if (!(state.root instanceof HTMLElement)) {
    return;
  }
  state.root.classList.toggle("reader-image-viewer--pannable", state.open && state.activeIndex >= 0);
  state.root.classList.toggle("reader-image-viewer--dragging", state.dragging);
}

function resetDraggingState() {
  state.dragging = false;
  state.dragMoved = false;
  state.dragStartX = 0;
  state.dragStartY = 0;
  state.dragOriginPanX = 0;
  state.dragOriginPanY = 0;
  syncDraggingState();
}

function resetPanState() {
  state.panX = 0;
  state.panY = 0;
}

function buildMediaNode(item) {
  if (!item) {
    return null;
  }
  if (item.kind === "img" && item.element instanceof HTMLImageElement) {
    const nextImage = document.createElement("img");
    nextImage.className = "reader-image-viewer__media reader-image-viewer__media--img";
    nextImage.src = item.element.currentSrc || item.element.src;
    nextImage.alt = item.altText;
    nextImage.decoding = "async";
    nextImage.draggable = false;
    const resolvedDimensions = resolveImageDimensions(item.element);
    if (resolvedDimensions.width > 0 && resolvedDimensions.height > 0) {
      item.width = resolvedDimensions.width;
      item.height = resolvedDimensions.height;
      nextImage.style.width = `${resolvedDimensions.width}px`;
      nextImage.style.height = `${resolvedDimensions.height}px`;
    } else {
      nextImage.style.width = "auto";
      nextImage.style.height = "auto";
    }
    nextImage.addEventListener(
      "load",
      () => {
        const liveWidth = nextImage.naturalWidth;
        const liveHeight = nextImage.naturalHeight;
        if (liveWidth > 0 && liveHeight > 0) {
          const previousFitScale = state.fitScale;
          item.width = liveWidth;
          item.height = liveHeight;
          nextImage.style.width = `${liveWidth}px`;
          nextImage.style.height = `${liveHeight}px`;
          if (resolveActiveItem() === item) {
            state.fitScale = computeFitScale(item);
            if (Math.abs(state.zoom - previousFitScale) <= DOUBLE_CLICK_TOGGLE_EPSILON) {
              state.zoom = state.fitScale;
            }
            applyViewportTransform();
          }
        }
      },
      { once: true }
    );
    return nextImage;
  }

  if (item.element instanceof SVGSVGElement) {
    const nextSVG = item.element.cloneNode(true);
    if (nextSVG instanceof SVGSVGElement) {
      nextSVG.classList.add("reader-image-viewer__media", "reader-image-viewer__media--svg");
      nextSVG.setAttribute("aria-label", item.altText || "图表");
      nextSVG.style.width = `${item.width}px`;
      nextSVG.style.height = `${item.height}px`;
      nextSVG.removeAttribute(VIEWER_TARGET_ATTRIBUTE);
      nextSVG.removeAttribute(VIEWER_ITEM_INDEX_ATTRIBUTE);
      return nextSVG;
    }
  }

  return null;
}

function renderActiveItem() {
  if (!(state.content instanceof HTMLElement)) {
    return;
  }
  const item = updateItemDimensions(resolveActiveItem());
  if (!item) {
    renderEmptyState();
    updateToolbarState();
    return;
  }

  state.content.innerHTML = "";
  const viewportNode = document.createElement("div");
  viewportNode.className = "reader-image-viewer__viewport";
  const frameNode = document.createElement("div");
  frameNode.className = "reader-image-viewer__media-frame";
  const mediaNode = buildMediaNode(item);
  if (mediaNode) {
    frameNode.appendChild(mediaNode);
  }
  viewportNode.appendChild(frameNode);
  state.content.appendChild(viewportNode);
  applyViewportTransform();
  if (state.stage instanceof HTMLElement) {
    state.stage.scrollTop = 0;
    state.stage.scrollLeft = 0;
  }
}

function syncRootVisibility() {
  if (!(state.root instanceof HTMLElement)) {
    return;
  }
  state.root.hidden = !state.open;
  state.root.setAttribute("aria-hidden", state.open ? "false" : "true");
  if (document.body instanceof HTMLElement) {
    document.body.classList.toggle(VIEWER_OPEN_BODY_CLASS, state.open);
  }
}

function closeReaderImageViewer() {
  resetDraggingState();
  state.open = false;
  state.activeIndex = -1;
  state.zoom = 1;
  state.fitScale = 1;
  state.rotation = 0;
  resetPanState();
  state.suppressNextStageClick = false;
  renderEmptyState();
  updateToolbarState();
  syncRootVisibility();
}

function openReaderImageViewer(index) {
  const nextIndex = clamp(index, 0, Math.max(0, state.items.length - 1));
  const item = updateItemDimensions(state.items[nextIndex]);
  if (!item) {
    closeReaderImageViewer();
    return;
  }
  state.open = true;
  syncRootVisibility();
  state.activeIndex = nextIndex;
  state.fitScale = computeFitScale(item);
  state.zoom = state.fitScale;
  state.rotation = 0;
  resetPanState();
  renderActiveItem();
}

function stepToSibling(offset) {
  if (!state.items.length) {
    return;
  }
  const total = state.items.length;
  const nextIndex = (state.activeIndex + offset + total) % total;
  openReaderImageViewer(nextIndex);
}

function setViewerZoom(nextScale, anchorPoint = null) {
  const previousScale = state.zoom;
  const clampedScale = clamp(nextScale, MIN_ZOOM_SCALE, MAX_ZOOM_SCALE);
  if (anchorPoint && previousScale > 0) {
    const frameCenter = resolveFrameCenter();
    const scaleRatio = clampedScale / previousScale;
    if (frameCenter) {
      state.panX += (1 - scaleRatio) * (anchorPoint.x - frameCenter.x);
      state.panY += (1 - scaleRatio) * (anchorPoint.y - frameCenter.y);
    }
  }
  state.zoom = clampedScale;
  applyViewportTransform();
}

function zoomInReaderImageViewer() {
  setViewerZoom(state.zoom * ZOOM_STEP_MULTIPLIER);
}

function zoomOutReaderImageViewer() {
  setViewerZoom(state.zoom / ZOOM_STEP_MULTIPLIER);
}

function resetReaderImageViewerToOriginalSize() {
  setViewerZoom(1);
}

function rotateReaderImageViewer() {
  state.rotation += ROTATION_STEP_DEGREES;
  applyViewportTransform();
}

function toggleReaderImageViewerScale(clientX, clientY) {
  const anchorPoint =
    typeof clientX === "number" && typeof clientY === "number"
      ? { x: clientX, y: clientY }
      : null;
  if (Math.abs(state.zoom - state.fitScale) <= DOUBLE_CLICK_TOGGLE_EPSILON) {
    setViewerZoom(1, anchorPoint);
    return;
  }
  setViewerZoom(state.fitScale, anchorPoint);
}

function handleViewerResize() {
  if (!state.open) {
    return;
  }
  const item = resolveActiveItem();
  if (!item) {
    return;
  }
  const zoomRatio = state.fitScale > 0 ? state.zoom / state.fitScale : 1;
  state.fitScale = computeFitScale(item);
  state.zoom = clamp(state.fitScale * zoomRatio, MIN_ZOOM_SCALE, MAX_ZOOM_SCALE);
  applyViewportTransform();
}

function refreshReaderImageViewer(root = document) {
  if (!ensureViewerNodes(root)) {
    return [];
  }
  syncRootVisibility();
  const activeItem = state.open ? resolveActiveItem() : null;
  clearStaleTargets(root);
  const nextItems = collectGalleryItems(root);
  state.items = nextItems;
  if (state.open) {
    const nextActiveIndex = activeItem
      ? nextItems.findIndex((item) => item.kind === activeItem.kind && item.element === activeItem.element)
      : -1;
    if (nextActiveIndex < 0) {
      closeReaderImageViewer();
    } else {
      state.activeIndex = nextActiveIndex;
      renderActiveItem();
    }
  } else {
    updateToolbarState();
  }
  return state.items;
}

function handleViewerClick(event) {
  if (!(event.target instanceof Element)) {
    return;
  }
  const mediaNode = event.target.closest(GALLERY_ITEM_SELECTOR);
  if (mediaNode instanceof Element) {
    const linkedMediaAnchor = mediaNode.closest("a[href]");
    if (linkedMediaAnchor instanceof HTMLAnchorElement && linkedMediaAnchor.contains(mediaNode)) {
      return;
    }
    const rawIndex = mediaNode.getAttribute(VIEWER_ITEM_INDEX_ATTRIBUTE) || "";
    const nextIndex = Number.parseInt(rawIndex, 10);
    if (Number.isFinite(nextIndex)) {
      event.preventDefault();
      event.stopPropagation();
      openReaderImageViewer(nextIndex);
      return;
    }
  }

  if (event.target.closest(VIEWER_BACKDROP_SELECTOR) || event.target.closest(VIEWER_CLOSE_SELECTOR)) {
    event.preventDefault();
    closeReaderImageViewer();
    return;
  }

  if (event.target.closest(VIEWER_PREV_SELECTOR)) {
    event.preventDefault();
    stepToSibling(-1);
    return;
  }
  if (event.target.closest(VIEWER_NEXT_SELECTOR)) {
    event.preventDefault();
    stepToSibling(1);
    return;
  }
  if (event.target.closest(VIEWER_ZOOM_OUT_SELECTOR)) {
    event.preventDefault();
    zoomOutReaderImageViewer();
    return;
  }
  if (event.target.closest(VIEWER_ZOOM_IN_SELECTOR)) {
    event.preventDefault();
    zoomInReaderImageViewer();
    return;
  }
  if (event.target.closest(VIEWER_ORIGINAL_SELECTOR)) {
    event.preventDefault();
    resetReaderImageViewerToOriginalSize();
    return;
  }
  if (event.target.closest(VIEWER_ROTATE_SELECTOR)) {
    event.preventDefault();
    rotateReaderImageViewer();
    return;
  }

  if (!state.open) {
    return;
  }

  const clickedInsideStage = event.target.closest(VIEWER_STAGE_SELECTOR);
  if (!clickedInsideStage) {
    return;
  }
  if (state.suppressNextStageClick) {
    state.suppressNextStageClick = false;
    event.preventDefault();
    return;
  }
  if (
    event.target.closest(VIEWER_TOOLBAR_SELECTOR) ||
    event.target.closest(VIEWER_CLOSE_SELECTOR) ||
    event.target.closest(VIEWER_MEDIA_FRAME_SELECTOR)
  ) {
    return;
  }
  event.preventDefault();
  closeReaderImageViewer();
}

function handleViewerMouseDown(event) {
  if (
    !(event instanceof MouseEvent) ||
    !state.open ||
    event.button !== 0 ||
    !(event.target instanceof Element) ||
    !(state.stage instanceof HTMLElement)
  ) {
    return;
  }
  if (event.target.closest(VIEWER_TOOLBAR_SELECTOR) || event.target.closest(VIEWER_CLOSE_SELECTOR)) {
    return;
  }
  const frameNode = event.target.closest(VIEWER_MEDIA_FRAME_SELECTOR);
  if (!(frameNode instanceof HTMLElement)) {
    return;
  }
  event.preventDefault();
  state.dragging = true;
  state.dragMoved = false;
  state.dragStartX = event.clientX;
  state.dragStartY = event.clientY;
  state.dragOriginPanX = state.panX;
  state.dragOriginPanY = state.panY;
  syncDraggingState();
}

function handleViewerMouseMove(event) {
  if (!(event instanceof MouseEvent) || !state.dragging) {
    return;
  }
  const deltaX = event.clientX - state.dragStartX;
  const deltaY = event.clientY - state.dragStartY;
  if (!state.dragMoved && (Math.abs(deltaX) > 3 || Math.abs(deltaY) > 3)) {
    state.dragMoved = true;
  }
  state.panX = state.dragOriginPanX + deltaX;
  state.panY = state.dragOriginPanY + deltaY;
  applyViewportTransform();
  event.preventDefault();
}

function handleViewerMouseUp() {
  if (!state.dragging) {
    return;
  }
  if (state.dragMoved) {
    state.suppressNextStageClick = true;
  }
  resetDraggingState();
}

function handleViewerDoubleClick(event) {
  if (
    !(event instanceof MouseEvent) ||
    !state.open ||
    !(event.target instanceof Element) ||
    !event.target.closest(VIEWER_MEDIA_FRAME_SELECTOR)
  ) {
    return;
  }
  event.preventDefault();
  toggleReaderImageViewerScale(event.clientX, event.clientY);
}

function handleViewerWheel(event) {
  if (
    !(event instanceof WheelEvent) ||
    !state.open ||
    !(event.target instanceof Element) ||
    !event.target.closest(VIEWER_STAGE_SELECTOR) ||
    event.target.closest(VIEWER_TOOLBAR_SELECTOR)
  ) {
    return;
  }
  event.preventDefault();
  const anchorPoint = { x: event.clientX, y: event.clientY };
  const nextScale =
    event.deltaY < 0 ? state.zoom * ZOOM_STEP_MULTIPLIER : state.zoom / ZOOM_STEP_MULTIPLIER;
  setViewerZoom(nextScale, anchorPoint);
}

function handleViewerKeydown(event) {
  if (!(event instanceof KeyboardEvent) || !state.open) {
    return;
  }
  if (event.key === "Escape") {
    event.preventDefault();
    closeReaderImageViewer();
    return;
  }
  if (event.key === "ArrowLeft") {
    event.preventDefault();
    stepToSibling(-1);
    return;
  }
  if (event.key === "ArrowRight") {
    event.preventDefault();
    stepToSibling(1);
    return;
  }
  if (event.key === "+" || event.key === "=") {
    event.preventDefault();
    zoomInReaderImageViewer();
    return;
  }
  if (event.key === "-") {
    event.preventDefault();
    zoomOutReaderImageViewer();
    return;
  }
  if (event.key === "0") {
    event.preventDefault();
    resetReaderImageViewerToOriginalSize();
    return;
  }
  if (event.key.toLowerCase() === "r") {
    event.preventDefault();
    rotateReaderImageViewer();
  }
}

export function ensureReaderImageViewer(root = document) {
  if (!ensureViewerNodes(root)) {
    return [];
  }
  if (!state.initialized) {
    document.addEventListener("click", handleViewerClick);
    document.addEventListener("dblclick", handleViewerDoubleClick);
    document.addEventListener("mousedown", handleViewerMouseDown);
    document.addEventListener("wheel", handleViewerWheel, { passive: false });
    window.addEventListener("mousemove", handleViewerMouseMove);
    window.addEventListener("mouseup", handleViewerMouseUp);
    window.addEventListener("resize", handleViewerResize);
    window.addEventListener("keydown", handleViewerKeydown);
    state.initialized = true;
  }
  return refreshReaderImageViewer(root);
}

export {
  closeReaderImageViewer,
  openReaderImageViewer,
  refreshReaderImageViewer,
  resetReaderImageViewerToOriginalSize,
  rotateReaderImageViewer,
  zoomInReaderImageViewer,
  zoomOutReaderImageViewer
};

if (typeof window !== "undefined") {
  window[READER_IMAGE_VIEWER_RUNTIME_GLOBAL_KEY] = {
    closeReaderImageViewer,
    ensureReaderImageViewer,
    openReaderImageViewer,
    refreshReaderImageViewer,
    resetReaderImageViewerToOriginalSize,
    rotateReaderImageViewer,
    zoomInReaderImageViewer,
    zoomOutReaderImageViewer
  };
}
