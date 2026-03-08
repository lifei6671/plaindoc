/* 阅读页异步增强脚本：通过字符串注入到 SSR HTML，避免内联大段模板影响主文件可读性。 */
export const READER_ASYNC_ENHANCEMENT_SCRIPT = `(() => {
  const DOC_LINK_SELECTOR = "a[data-reader-doc-link='1']";
  const TREE_DETAILS_SELECTOR = "[data-reader-hook='tree-details']";
  const TREE_SUMMARY_SELECTOR = "[data-reader-hook='tree-summary']";
  const TREE_ARROW_SELECTOR = "[data-reader-hook='tree-arrow']";
  const TREE_ROW_SELECTOR = "[data-reader-hook='tree-row']";
  const TREE_LABEL_SELECTOR = "[data-reader-hook='tree-label']";
  const TREE_ROW_ACTIVE_SELECTOR = "[data-reader-hook='tree-row'][data-reader-active='1']";
  const TREE_LABEL_ACTIVE_SELECTOR = "[data-reader-hook='tree-label'][data-reader-label-active='1']";
  const ARTICLE_SHELL_SELECTOR = "[data-reader-hook='article-shell']";
  const MAIN_SELECTOR = "[data-reader-hook='main']";
  const MOBILE_SIDEBAR_OPEN_TRIGGER_SELECTOR = "[data-reader-hook='mobile-sidebar-open']";
  const MOBILE_SIDEBAR_CLOSE_TRIGGER_SELECTOR = "[data-reader-hook='mobile-sidebar-close']";
  const MOBILE_SIDEBAR_OVERLAY_SELECTOR = "[data-reader-hook='mobile-overlay']";
  const MOBILE_BAR_TITLE_SELECTOR = "[data-reader-hook='mobile-bar-title']";
  const MOBILE_SIDEBAR_OPEN_CLASS = "reader-mobile-sidebar-open";
  const TREE_TITLE_TOOLTIP_SELECTOR = "[data-reader-hook='tree-title-tooltip'][data-tooltip]";
  const TREE_ROW_ACTIVE_CLASS = "reader-tree__row--active";
  const TREE_LABEL_ACTIVE_CLASS = "reader-tree__label--active";
  const STATE_SCRIPT_SELECTOR = "#plaindoc-reader-state";
  const PROGRESS_SELECTOR = "[data-reader-hook='progress']";
  const PROGRESS_VISIBLE_CLASS = "reader-progress--visible";
  const GOOGLE_SANS_CODE_STYLE_ID = "plaindoc-reader-google-sans-code-style";
  const APP_STYLE_ID = "plaindoc-reader-app-style";
  const BASE_STYLE_ID = "plaindoc-reader-base-style";
  const KATEX_STYLE_ID = "plaindoc-reader-katex-style";
  const THEME_STYLE_ID = "plaindoc-reader-theme-style";
  const THEME_CUSTOM_STYLE_ID = "plaindoc-reader-theme-custom-style";
  const OUTLINE_SELECTOR = "[data-reader-hook='outline']";
  const OUTLINE_LINK_SELECTOR = "[data-reader-hook='outline-link'][data-outline-index]";
  const OUTLINE_LINK_ACTIVE_CLASS = "reader-outline__link--active";
  const ATTACHMENT_ACTION_SELECTOR = "button[data-reader-attachment-action='1']";
  const ATTACHMENT_STATUS_SELECTOR = "[data-reader-hook='attachment-status']";
  const ATTACHMENT_ACTION_BUSY_CLASS = "reader-attachment__action--busy";
  const ATTACHMENT_LINK_SELECTOR = "a[data-reader-attachment-link='1']";
  const EXPORT_ACTION_SELECTOR = "button[data-reader-export-action]";
  const EXPORT_ACTION_BUSY_CLASS = "reader-article-action--busy";
  const OFFICE_EDITOR_SELECTOR = "[data-reader-office-editor='1']";
  const OFFICE_PLACEHOLDER_SELECTOR = "[data-reader-office-placeholder='1']";
  const OFFICE_TITLE_SELECTOR = "[data-reader-office-title='1']";
  const OFFICE_MESSAGE_SELECTOR = "[data-reader-office-message='1']";
  const OFFICE_DOWNLOAD_SELECTOR = "a[data-reader-office-download='1']";
  const OFFICE_PANE_SELECTOR = ".reader-office-pane";
  const OFFICE_EDITOR_HIDDEN_CLASS = "office-pane__editor--hidden";
  const OFFICE_PLACEHOLDER_ERROR_CLASS = "office-pane__placeholder--error";
  const OFFICE_XLSX_READER_SELECTOR = "[data-office-xlsx-reader='1']";
  const OFFICE_XLSX_TAB_SELECTOR = "[data-office-sheet-tab]";
  const OFFICE_XLSX_PANEL_SELECTOR = "[data-office-sheet-panel]";
  const OFFICE_XLSX_TABLE_WRAP_SELECTOR = ".office-xlsx-sheet__table-wrap";
  const OFFICE_XLSX_TABLE_HEAD_SELECTOR = ".office-xlsx-sheet__table thead";
  const OFFICE_XLSX_TAB_ACTIVE_CLASS = "office-xlsx-tab--active";
  const OFFICE_XLSX_PANEL_ACTIVE_CLASS = "office-xlsx-sheet--active";
  const ONLYOFFICE_SCRIPT_LOADER_GLOBAL_KEY = "__plaindocOnlyOfficeScriptLoaders__";
  const OFFICE_PANE_MIN_HEIGHT = 360;
  const OFFICE_PANE_BOTTOM_GAP = 24;
  const PREVIEW_HEADING_SELECTOR =
    "#plaindoc-preview-body h1, #plaindoc-preview-body h2, #plaindoc-preview-body h3, #plaindoc-preview-body h4, #plaindoc-preview-body h5, #plaindoc-preview-body h6";
  const OUTLINE_SCROLL_OFFSET = 16;
  const OUTLINE_ACTIVE_OFFSET = 108;
  const MOBILE_SIDEBAR_BREAKPOINT = 1024;
  const queryActiveRow = () => document.querySelector(TREE_ROW_ACTIVE_SELECTOR);

  const normalizePathname = (pathname) => pathname.replace(/\\/+$/, "") || "/";
  const isModifiedClick = (event) =>
    !(event instanceof MouseEvent) ||
    event.button !== 0 ||
    event.metaKey ||
    event.ctrlKey ||
    event.shiftKey ||
    event.altKey;

  const isMobileSidebarViewport = () => window.innerWidth <= MOBILE_SIDEBAR_BREAKPOINT;

  const collectMobileSidebarOpenButtons = () =>
    Array.from(document.querySelectorAll(MOBILE_SIDEBAR_OPEN_TRIGGER_SELECTOR)).filter(
      (node) => node instanceof HTMLButtonElement
    );

  let mobileSidebarOpen = false;

  const syncMobileSidebarDOMState = () => {
    const shouldOpen = isMobileSidebarViewport() && mobileSidebarOpen;
    if (document.body instanceof HTMLElement) {
      document.body.classList.toggle(MOBILE_SIDEBAR_OPEN_CLASS, shouldOpen);
    }

    const mobileSidebarOpenButtons = collectMobileSidebarOpenButtons();
    for (const button of mobileSidebarOpenButtons) {
      if (!(button instanceof HTMLButtonElement)) {
        continue;
      }
      button.setAttribute("aria-expanded", shouldOpen ? "true" : "false");
    }

    const overlayNode = document.querySelector(MOBILE_SIDEBAR_OVERLAY_SELECTOR);
    if (overlayNode instanceof HTMLElement) {
      overlayNode.hidden = !shouldOpen;
    }
  };

  const setMobileSidebarOpen = (nextOpen) => {
    mobileSidebarOpen = nextOpen === true;
    syncMobileSidebarDOMState();
  };

  const syncNestedTreeStateAfterExpand = (detailsElement) => {
    if (!(detailsElement instanceof HTMLDetailsElement)) {
      return;
    }
    window.requestAnimationFrame(() => {
      if (!detailsElement.open) {
        return;
      }
      const nestedDetails = detailsElement.querySelectorAll(TREE_DETAILS_SELECTOR);
      const activeRow = queryActiveRow();
      for (const nestedDetail of nestedDetails) {
        if (!(nestedDetail instanceof HTMLDetailsElement) || nestedDetail === detailsElement) {
          continue;
        }
        if (activeRow instanceof HTMLElement && nestedDetail.contains(activeRow)) {
          nestedDetail.open = true;
          continue;
        }
        nestedDetail.open = false;
      }
    });
  };

  const toSameOriginURL = (rawHref) => {
    try {
      const parsedURL = new URL(rawHref, window.location.href);
      if (parsedURL.origin !== window.location.origin) {
        return null;
      }
      return parsedURL;
    } catch {
      return null;
    }
  };

  const resolveJSONPayload = (rawText) => {
    const normalizedText = typeof rawText === "string" ? rawText.trim() : "";
    if (!normalizedText) {
      return null;
    }
    try {
      return JSON.parse(normalizedText);
    } catch {
      return null;
    }
  };

  const resolveJsonResultData = (payload) => {
    if (!payload || typeof payload !== "object") {
      return null;
    }
    if ("data" in payload && payload.data && typeof payload.data === "object") {
      return payload.data;
    }
    return payload;
  };

  const setAttachmentStatus = (message, isError) => {
    const statusNode = document.querySelector(ATTACHMENT_STATUS_SELECTOR);
    if (!(statusNode instanceof HTMLElement)) {
      return;
    }
    const text = typeof message === "string" ? message.trim() : "";
    if (!text) {
      return;
    }
    statusNode.textContent = text;
    statusNode.classList.toggle("reader-attachments__hint--error", isError === true);
  };

  const resolveReaderStatePayload = () => {
    const stateNode = document.querySelector(STATE_SCRIPT_SELECTOR);
    if (!(stateNode instanceof HTMLScriptElement)) {
      return null;
    }
    return resolveJSONPayload(stateNode.textContent || "");
  };

  const isRecord = (value) => value !== null && typeof value === "object";
  const isOfficeDocumentFormat = (value) => value === "docx" || value === "xlsx";
  const resolveOfficeDocumentLabel = (value) => (value === "xlsx" ? "Excel 文档" : "Word 文档");

  const cloneOnlyOfficeConfigPayload = (input) => {
    if (typeof structuredClone === "function") {
      return structuredClone(input);
    }
    return JSON.parse(JSON.stringify(input));
  };

  const toOnlyOfficeEventHandler = (value) => (typeof value === "function" ? value : null);

  const resolveOnlyOfficeErrorMessage = (event) => {
    if (isRecord(event)) {
      const directMessage = typeof event.message === "string" ? event.message.trim() : "";
      if (directMessage) {
        return directMessage;
      }
      if (isRecord(event.data)) {
        const detail = event.data;
        const description =
          typeof detail.errorDescription === "string"
            ? detail.errorDescription.trim()
            : typeof detail.errorMessage === "string"
              ? detail.errorMessage.trim()
              : typeof detail.message === "string"
                ? detail.message.trim()
                : "";
        if (description) {
          return description;
        }
      }
    }
    return "ONLYOFFICE 阅读器发生错误";
  };

  const attachOnlyOfficeRuntimeEvents = (config, runtimeHandlers) => {
    const existingEvents = isRecord(config.events) ? config.events : {};
    const onDocumentReady = toOnlyOfficeEventHandler(existingEvents.onDocumentReady);
    const onError = toOnlyOfficeEventHandler(existingEvents.onError);

    return {
      ...config,
      events: {
        ...existingEvents,
        onDocumentReady: (...args) => {
          if (onDocumentReady) {
            onDocumentReady(...args);
          }
          runtimeHandlers.onDocumentReady();
        },
        onError: (event, ...args) => {
          if (onError) {
            onError(event, ...args);
          }
          runtimeHandlers.onError(resolveOnlyOfficeErrorMessage(event));
        }
      }
    };
  };

  const resolveOnlyOfficeLoaderMap = () => {
    const target = window;
    const existing = target[ONLYOFFICE_SCRIPT_LOADER_GLOBAL_KEY];
    if (existing instanceof Map) {
      return existing;
    }
    const next = new Map();
    target[ONLYOFFICE_SCRIPT_LOADER_GLOBAL_KEY] = next;
    return next;
  };

  const buildOnlyOfficeApiScriptURL = (documentServerUrl) => {
    const normalizedBaseURL = typeof documentServerUrl === "string" ? documentServerUrl.trim().replace(/\\/+$/, "") : "";
    if (!normalizedBaseURL) {
      return "";
    }
    return normalizedBaseURL + "/web-apps/apps/api/documents/api.js";
  };

  const findOnlyOfficeScriptElement = (scriptURL) =>
    Array.from(document.querySelectorAll("script[data-onlyoffice-api]")).find(
      (node) => node instanceof HTMLScriptElement && node.dataset.onlyofficeApi === scriptURL
    );

  const loadOnlyOfficeApiScript = (documentServerUrl) => {
    if (typeof window.DocsAPI?.DocEditor === "function") {
      return Promise.resolve();
    }

    const scriptURL = buildOnlyOfficeApiScriptURL(documentServerUrl);
    if (!scriptURL) {
      return Promise.reject(new Error("ONLYOFFICE 服务地址为空"));
    }

    const loaders = resolveOnlyOfficeLoaderMap();
    const cachedLoader = loaders.get(scriptURL);
    if (cachedLoader) {
      return cachedLoader;
    }

    const loader = new Promise((resolve, reject) => {
      const handleLoad = () => {
        if (typeof window.DocsAPI?.DocEditor !== "function") {
          reject(new Error("ONLYOFFICE Docs API 未就绪"));
          return;
        }
        resolve();
      };
      const handleError = () => {
        loaders.delete(scriptURL);
        reject(new Error("加载 ONLYOFFICE 脚本失败"));
      };

      const existingScript = findOnlyOfficeScriptElement(scriptURL);
      const scriptElement = existingScript || document.createElement("script");
      scriptElement.addEventListener("load", handleLoad, { once: true });
      scriptElement.addEventListener("error", handleError, { once: true });

      if (!existingScript) {
        scriptElement.src = scriptURL;
        scriptElement.async = true;
        scriptElement.defer = true;
        scriptElement.dataset.onlyofficeApi = scriptURL;
        document.head.appendChild(scriptElement);
      }
    });

    loaders.set(scriptURL, loader);
    return loader;
  };

  let activeOnlyOfficeReader = null;
  let onlyOfficeReaderSeq = 0;
  let officeDownloadSeq = 0;
  let officePaneHeightRafID = 0;

  const resolveOfficeEditorNode = () => document.querySelector(OFFICE_EDITOR_SELECTOR);
  const resolveOfficePlaceholderNode = () => document.querySelector(OFFICE_PLACEHOLDER_SELECTOR);
  const resolveOfficePaneNode = () => document.querySelector(OFFICE_PANE_SELECTOR);

  const syncOfficePaneViewportHeight = () => {
    const officePaneNode = resolveOfficePaneNode();
    if (!(officePaneNode instanceof HTMLElement)) {
      return;
    }
    const viewportHeight = window.innerHeight || document.documentElement.clientHeight || 0;
    if (viewportHeight <= 0) {
      return;
    }
    const paneRect = officePaneNode.getBoundingClientRect();
    let nextHeight = Math.floor(viewportHeight - paneRect.top - OFFICE_PANE_BOTTOM_GAP);
    if (!Number.isFinite(nextHeight) || nextHeight <= 0) {
      nextHeight = OFFICE_PANE_MIN_HEIGHT;
    }
    if (nextHeight < OFFICE_PANE_MIN_HEIGHT) {
      nextHeight = OFFICE_PANE_MIN_HEIGHT;
    }
    officePaneNode.style.setProperty("--reader-office-pane-height", String(nextHeight) + "px");
  };

  const scheduleOfficePaneViewportHeightSync = () => {
    if (officePaneHeightRafID) {
      window.cancelAnimationFrame(officePaneHeightRafID);
    }
    officePaneHeightRafID = window.requestAnimationFrame(() => {
      officePaneHeightRafID = 0;
      syncOfficePaneViewportHeight();
    });
  };

  const setOfficeDownloadLink = (downloadURL, fileName) => {
    const downloadNode = document.querySelector(OFFICE_DOWNLOAD_SELECTOR);
    if (!(downloadNode instanceof HTMLAnchorElement)) {
      return;
    }
    const normalizedURL = typeof downloadURL === "string" ? downloadURL.trim() : "";
    if (!normalizedURL) {
      downloadNode.removeAttribute("href");
      downloadNode.removeAttribute("download");
      downloadNode.removeAttribute("data-reader-office-download-ready");
      downloadNode.setAttribute("aria-disabled", "true");
      return;
    }
    downloadNode.href = normalizedURL;
    downloadNode.rel = "noopener noreferrer";
    downloadNode.setAttribute("data-reader-office-download-ready", "1");
    downloadNode.setAttribute("aria-disabled", "false");
    const normalizedFileName = typeof fileName === "string" ? fileName.trim() : "";
    if (normalizedFileName) {
      downloadNode.setAttribute("download", normalizedFileName);
    } else {
      downloadNode.removeAttribute("download");
    }
  };

  const setOfficeDownloadBusy = (busy) => {
    const downloadNode = document.querySelector(OFFICE_DOWNLOAD_SELECTOR);
    if (!(downloadNode instanceof HTMLAnchorElement)) {
      return;
    }
    const isBusy = busy === true;
    downloadNode.classList.toggle(EXPORT_ACTION_BUSY_CLASS, isBusy);
    downloadNode.setAttribute("aria-disabled", isBusy ? "true" : downloadNode.getAttribute("href") ? "false" : "true");
  };

  const setOfficePlaceholderState = (status, title, message) => {
    const editorNode = resolveOfficeEditorNode();
    if (editorNode instanceof HTMLElement) {
      editorNode.classList.toggle(OFFICE_EDITOR_HIDDEN_CLASS, status !== "ready");
    }

    const placeholderNode = resolveOfficePlaceholderNode();
    if (!(placeholderNode instanceof HTMLElement)) {
      return;
    }
    // 不使用 hidden 属性，因为样式里的 display:flex 会覆盖 UA 默认的 [hidden]{display:none}。
    // 显式切换 display，避免“已就绪文案仍覆盖编辑器”的情况。
    placeholderNode.style.display = status === "ready" ? "none" : "flex";
    placeholderNode.setAttribute("aria-hidden", status === "ready" ? "true" : "false");
    placeholderNode.setAttribute("data-reader-office-status", status);
    placeholderNode.classList.toggle(OFFICE_PLACEHOLDER_ERROR_CLASS, status === "error");

    const titleNode = document.querySelector(OFFICE_TITLE_SELECTOR);
    if (titleNode instanceof HTMLElement && typeof title === "string" && title.trim()) {
      titleNode.textContent = title.trim();
    }
    const messageNode = document.querySelector(OFFICE_MESSAGE_SELECTOR);
    if (messageNode instanceof HTMLElement && typeof message === "string" && message.trim()) {
      messageNode.textContent = message.trim();
    }
  };

  const destroyOnlyOfficeReader = () => {
    if (activeOnlyOfficeReader && typeof activeOnlyOfficeReader.destroyEditor === "function") {
      activeOnlyOfficeReader.destroyEditor();
    }
    activeOnlyOfficeReader = null;
    const editorNode = resolveOfficeEditorNode();
    if (editorNode instanceof HTMLElement) {
      editorNode.classList.add(OFFICE_EDITOR_HIDDEN_CLASS);
    }
  };

  const activateSpreadsheetTab = (readerNode, sheetKey, shouldFocus) => {
    if (!(readerNode instanceof HTMLElement)) {
      return;
    }
    const normalizedSheetKey = typeof sheetKey === "string" ? sheetKey.trim() : "";
    if (!normalizedSheetKey) {
      return;
    }
    const tabNodes = Array.from(readerNode.querySelectorAll(OFFICE_XLSX_TAB_SELECTOR)).filter(
      (node) => node instanceof HTMLButtonElement
    );
    const panelNodes = Array.from(readerNode.querySelectorAll(OFFICE_XLSX_PANEL_SELECTOR)).filter(
      (node) => node instanceof HTMLElement
    );
    if (!tabNodes.length || !panelNodes.length) {
      return;
    }

    for (const tabNode of tabNodes) {
      if (!(tabNode instanceof HTMLButtonElement)) {
        continue;
      }
      const isActive = (tabNode.getAttribute("data-office-sheet-tab") || "").trim() === normalizedSheetKey;
      tabNode.classList.toggle(OFFICE_XLSX_TAB_ACTIVE_CLASS, isActive);
      tabNode.setAttribute("aria-selected", isActive ? "true" : "false");
      tabNode.tabIndex = isActive ? 0 : -1;
      if (isActive && shouldFocus) {
        tabNode.focus({ preventScroll: true });
      }
    }

    for (const panelNode of panelNodes) {
      if (!(panelNode instanceof HTMLElement)) {
        continue;
      }
      const isActive = (panelNode.getAttribute("data-office-sheet-panel") || "").trim() === normalizedSheetKey;
      panelNode.classList.toggle(OFFICE_XLSX_PANEL_ACTIVE_CLASS, isActive);
      panelNode.hidden = !isActive;
    }
    syncSpreadsheetStickyHeaders(readerNode);
  };

  const syncSpreadsheetStickyHeader = (tableWrapNode) => {
    if (!(tableWrapNode instanceof HTMLElement)) {
      return;
    }
    const headNode = tableWrapNode.querySelector(OFFICE_XLSX_TABLE_HEAD_SELECTOR);
    if (!(headNode instanceof HTMLElement)) {
      return;
    }
    headNode.style.setProperty("--office-xlsx-head-offset", String(Math.max(0, tableWrapNode.scrollTop)) + "px");
  };

  const syncSpreadsheetStickyHeaders = (rootNode) => {
    const scopeNode =
      rootNode && typeof rootNode.querySelectorAll === "function"
        ? rootNode
        : document;
    const tableWrapNodes = Array.from(scopeNode.querySelectorAll(OFFICE_XLSX_TABLE_WRAP_SELECTOR)).filter(
      (node) => node instanceof HTMLElement
    );
    for (const tableWrapNode of tableWrapNodes) {
      syncSpreadsheetStickyHeader(tableWrapNode);
    }
  };

  const syncSpreadsheetTabs = () => {
    const spreadsheetReaders = Array.from(document.querySelectorAll(OFFICE_XLSX_READER_SELECTOR)).filter(
      (node) => node instanceof HTMLElement
    );
    for (const readerNode of spreadsheetReaders) {
      if (!(readerNode instanceof HTMLElement)) {
        continue;
      }
      const tabNodes = Array.from(readerNode.querySelectorAll(OFFICE_XLSX_TAB_SELECTOR)).filter(
        (node) => node instanceof HTMLButtonElement
      );
      if (!tabNodes.length) {
        continue;
      }
      const activeTab =
        tabNodes.find(
          (tabNode) =>
            tabNode instanceof HTMLButtonElement &&
            (tabNode.classList.contains(OFFICE_XLSX_TAB_ACTIVE_CLASS) ||
              (tabNode.getAttribute("aria-selected") || "").trim() === "true")
        ) || tabNodes[0];
      const activeSheetKey =
        activeTab instanceof HTMLButtonElement ? (activeTab.getAttribute("data-office-sheet-tab") || "").trim() : "";
      if (!activeSheetKey) {
        continue;
      }
      activateSpreadsheetTab(readerNode, activeSheetKey, false);
    }
    syncSpreadsheetStickyHeaders(document);
  };

  const resolveOnlyOfficeViewConfigPath = (payload) => {
    if (!payload || typeof payload !== "object") {
      return "";
    }
    const documentData = payload.document && typeof payload.document === "object" ? payload.document : null;
    if (!documentData || !isOfficeDocumentFormat(documentData.format)) {
      return "";
    }
    const spaceData = payload.space && typeof payload.space === "object" ? payload.space : null;
    const spaceID = typeof spaceData?.id === "string" ? spaceData.id.trim() : "";
    const shareData = payload.share && typeof payload.share === "object" ? payload.share : null;
    const docKey = shareData && typeof shareData.documentRouteKey === "string" && shareData.documentRouteKey.trim()
      ? shareData.documentRouteKey.trim()
      : typeof documentData.routeKey === "string" && documentData.routeKey.trim()
        ? documentData.routeKey.trim()
        : typeof documentData.id === "string"
          ? documentData.id.trim()
          : "";
    if (!spaceID || !docKey) {
      return "";
    }
    if (shareData && shareData.enabled === true) {
      return "/api/shares/" + encodeURIComponent(spaceID) + "/" + encodeURIComponent(docKey) + "/onlyoffice/view-config";
    }
    return (
      "/api/reader/spaces/" +
      encodeURIComponent(spaceID) +
      "/docs/" +
      encodeURIComponent(docKey) +
      "/onlyoffice/view-config"
    );
  };

  const resolveOfficeSourceAccessLinkPath = (payload) => {
    if (!payload || typeof payload !== "object") {
      return "";
    }
    const documentData = payload.document && typeof payload.document === "object" ? payload.document : null;
    const shareData = payload.share && typeof payload.share === "object" ? payload.share : null;
    const format = documentData && typeof documentData.format === "string" ? documentData.format.trim() : "";
    if (!isOfficeDocumentFormat(format)) {
      return "";
    }

    if (shareData && shareData.enabled === true) {
      const shareSpaceID =
        typeof shareData.spaceId === "string"
          ? shareData.spaceId.trim()
          : payload.space && typeof payload.space === "object" && typeof payload.space.id === "string"
            ? payload.space.id.trim()
            : "";
      const shareDocKey =
        typeof shareData.documentRouteKey === "string"
          ? shareData.documentRouteKey.trim()
          : documentData && typeof documentData.routeKey === "string"
            ? documentData.routeKey.trim()
            : documentData && typeof documentData.id === "string"
              ? documentData.id.trim()
              : "";
      if (!shareSpaceID || !shareDocKey) {
        return "";
      }
      return (
        "/api/shares/" +
        encodeURIComponent(shareSpaceID) +
        "/" +
        encodeURIComponent(shareDocKey) +
        "/office-source/access-link"
      );
    }

    const spaceID =
      payload.space && typeof payload.space === "object" && typeof payload.space.id === "string"
        ? payload.space.id.trim()
        : "";
    const documentID =
      documentData && typeof documentData.id === "string"
        ? documentData.id.trim()
        : documentData && typeof documentData.routeKey === "string"
          ? documentData.routeKey.trim()
          : "";
    if (!spaceID || !documentID) {
      return "";
    }
    return (
      "/api/reader/spaces/" +
      encodeURIComponent(spaceID) +
      "/docs/" +
      encodeURIComponent(documentID) +
      "/office-source/access-link"
    );
  };

  const fetchOnlyOfficeViewConfig = async () => {
    const payload = resolveReaderStatePayload();
    const requestPath = resolveOnlyOfficeViewConfigPath(payload);
    if (!requestPath) {
      throw new Error("当前页面不是 Office 文档");
    }
    const response = await fetch(requestPath, {
      method: "GET",
      credentials: "include",
      headers: {
        Accept: "application/json",
        "X-Requested-With": "plaindoc-reader-async"
      }
    });
    const rawResponseText = await response.text();
    const responsePayload = resolveJSONPayload(rawResponseText);
    const responseData = resolveJsonResultData(responsePayload);
    const responseCode =
      responsePayload && typeof responsePayload === "object" && typeof responsePayload.code === "number"
        ? responsePayload.code
        : 0;
    const responseMessage =
      responsePayload && typeof responsePayload === "object" && typeof responsePayload.message === "string"
        ? responsePayload.message.trim()
        : "";
    const documentServerUrl =
      responseData && typeof responseData === "object" && typeof responseData.documentServerUrl === "string"
        ? responseData.documentServerUrl.trim()
        : "";
    const config = responseData && typeof responseData === "object" && isRecord(responseData.config) ? responseData.config : null;
    if (!response.ok || responseCode !== 0 || !documentServerUrl || !config) {
      throw new Error(responseMessage || "获取 ONLYOFFICE 阅读配置失败");
    }
    return {
      documentServerUrl,
      config
    };
  };

  const fetchOfficeSourceAccessLink = async () => {
    const payload = resolveReaderStatePayload();
    const requestPath = resolveOfficeSourceAccessLinkPath(payload);
    if (!requestPath) {
      throw new Error("当前页面不是 Office 文档");
    }
    const response = await fetch(requestPath, {
      method: "POST",
      credentials: "include",
      headers: {
        Accept: "application/json",
        "X-Requested-With": "plaindoc-reader-async"
      }
    });
    const rawResponseText = await response.text();
    const responsePayload = resolveJSONPayload(rawResponseText);
    const responseData = resolveJsonResultData(responsePayload);
    const responseCode =
      responsePayload && typeof responsePayload === "object" && typeof responsePayload.code === "number"
        ? responsePayload.code
        : 0;
    const responseMessage =
      responsePayload && typeof responsePayload === "object" && typeof responsePayload.message === "string"
        ? responsePayload.message.trim()
        : "";
    const downloadURL =
      responseData && typeof responseData === "object" && typeof responseData.url === "string"
        ? responseData.url.trim()
        : "";
    const fileName =
      responseData && typeof responseData === "object" && typeof responseData.fileName === "string"
        ? responseData.fileName.trim()
        : "";
    if (!response.ok || responseCode !== 0 || !downloadURL) {
      throw new Error(responseMessage || "获取原文件下载链接失败");
    }
    return {
      downloadURL,
      fileName
    };
  };

  const syncOfficeDownloadAction = async () => {
    const downloadNode = document.querySelector(OFFICE_DOWNLOAD_SELECTOR);
    if (!(downloadNode instanceof HTMLAnchorElement)) {
      return;
    }
    const payload = resolveReaderStatePayload();
    const documentData = payload && typeof payload === "object" && payload.document && typeof payload.document === "object"
      ? payload.document
      : null;
    const format = documentData ? documentData.format : "";

    officeDownloadSeq += 1;
    const currentSeq = officeDownloadSeq;
    setOfficeDownloadBusy(false);
    setOfficeDownloadLink("", "");
    if (!isOfficeDocumentFormat(format)) {
      return;
    }

    try {
      const accessLink = await fetchOfficeSourceAccessLink();
      if (currentSeq !== officeDownloadSeq) {
        return;
      }
      setOfficeDownloadLink(accessLink.downloadURL, accessLink.fileName);
    } catch (error) {
      if (currentSeq !== officeDownloadSeq) {
        return;
      }
      setOfficeDownloadLink("", "");
      console.error("[reader][office-download] fetch access link failed", error);
    } finally {
      if (currentSeq === officeDownloadSeq) {
        setOfficeDownloadBusy(false);
      }
    }
  };

  const requestOfficeSourceDownload = async () => {
    const downloadNode = document.querySelector(OFFICE_DOWNLOAD_SELECTOR);
    if (!(downloadNode instanceof HTMLAnchorElement)) {
      return;
    }

    setOfficeDownloadBusy(true);
    try {
      const accessLink = await fetchOfficeSourceAccessLink();
      setOfficeDownloadLink(accessLink.downloadURL, accessLink.fileName);
      if (!triggerAttachmentNavigation(accessLink.downloadURL, "download")) {
        throw new Error("原文件下载链接无效");
      }
    } catch (error) {
      console.error("[reader][office-download] request download failed", error);
    } finally {
      setOfficeDownloadBusy(false);
    }
  };

  const syncOnlyOfficeReader = async () => {
    const payload = resolveReaderStatePayload();
    const documentData = payload && typeof payload === "object" && payload.document && typeof payload.document === "object"
      ? payload.document
      : null;
    const format = documentData ? documentData.format : "";

    onlyOfficeReaderSeq += 1;
    const currentSeq = onlyOfficeReaderSeq;
    destroyOnlyOfficeReader();
    setOfficeDownloadLink("", "");

    if (!isOfficeDocumentFormat(format)) {
      const officePaneNode = resolveOfficePaneNode();
      if (officePaneNode instanceof HTMLElement) {
        officePaneNode.style.removeProperty("--reader-office-pane-height");
      }
      return;
    }

    const editorNode = resolveOfficeEditorNode();
    const placeholderNode = resolveOfficePlaceholderNode();
    if (!(editorNode instanceof HTMLElement) || !(placeholderNode instanceof HTMLElement)) {
      return;
    }

    const officeTitle = resolveOfficeDocumentLabel(format);
    scheduleOfficePaneViewportHeightSync();
    setOfficePlaceholderState("loading", officeTitle, "正在获取 ONLYOFFICE 阅读配置...");

    try {
      const viewConfig = await fetchOnlyOfficeViewConfig();
      if (currentSeq !== onlyOfficeReaderSeq) {
        return;
      }

      const documentConfig = isRecord(viewConfig.config.document) ? viewConfig.config.document : null;
      const downloadURL = documentConfig && typeof documentConfig.url === "string" ? documentConfig.url.trim() : "";
      const downloadFileName = documentConfig && typeof documentConfig.title === "string" ? documentConfig.title.trim() : "";
      setOfficeDownloadLink(downloadURL, downloadFileName);

      setOfficePlaceholderState("loading", officeTitle, "正在加载 ONLYOFFICE 阅读器...");
      await loadOnlyOfficeApiScript(viewConfig.documentServerUrl);
      if (currentSeq !== onlyOfficeReaderSeq) {
        return;
      }

      const DocEditor = window.DocsAPI?.DocEditor;
      if (typeof DocEditor !== "function") {
        throw new Error("ONLYOFFICE Docs API 未就绪");
      }

      const runtimeConfig = attachOnlyOfficeRuntimeEvents(
        cloneOnlyOfficeConfigPayload(viewConfig.config),
        {
          onDocumentReady: () => {
            if (currentSeq !== onlyOfficeReaderSeq) {
              return;
            }
            setOfficePlaceholderState("ready", officeTitle, "ONLYOFFICE 阅读器已就绪");
          },
          onError: (message) => {
            if (currentSeq !== onlyOfficeReaderSeq) {
              return;
            }
            destroyOnlyOfficeReader();
            setOfficePlaceholderState("error", officeTitle, message || "ONLYOFFICE 阅读器初始化失败");
          }
        }
      );

      editorNode.classList.add(OFFICE_EDITOR_HIDDEN_CLASS);
      activeOnlyOfficeReader = new DocEditor(editorNode.id, runtimeConfig);
      scheduleOfficePaneViewportHeightSync();
      setOfficePlaceholderState("loading", officeTitle, "正在初始化文档...");
    } catch (error) {
      if (currentSeq !== onlyOfficeReaderSeq) {
        return;
      }
      const message =
        error && typeof error === "object" && "message" in error && typeof error.message === "string"
          ? error.message
          : "加载 ONLYOFFICE 阅读器失败";
      destroyOnlyOfficeReader();
      setOfficePlaceholderState("error", officeTitle, message);
    }
  };

  const resolveExportBaseURL = (payload) => {
    const fallbackOrigin =
      typeof window.location.origin === "string" && window.location.origin.trim()
        ? window.location.origin
        : window.location.href;
    if (!payload || typeof payload !== "object") {
      return fallbackOrigin;
    }
    const requestOrigin = typeof payload.requestOrigin === "string" ? payload.requestOrigin.trim() : "";
    const rawBase = requestOrigin || fallbackOrigin;
    try {
      return new URL(rawBase, window.location.href).toString();
    } catch {
      return fallbackOrigin;
    }
  };

  const sanitizeExportFileNameSegment = (value, fallback) => {
    const normalized =
      typeof value === "string"
        ? value.replace(/[\\\\/:*?"<>|]+/g, " ").replace(/\\s+/g, " ").trim()
        : "";
    return normalized || fallback;
  };

  const formatExportTimestampSegment = () => {
    const now = new Date();
    const padTwoDigits = (value) => String(Math.max(0, value)).padStart(2, "0");
    return (
      String(now.getFullYear()) +
      padTwoDigits(now.getMonth() + 1) +
      padTwoDigits(now.getDate()) +
      "-" +
      padTwoDigits(now.getHours()) +
      padTwoDigits(now.getMinutes())
    );
  };

  const buildMarkdownExportFileName = (title) => {
    const safeTitle = sanitizeExportFileNameSegment(title, "未命名文档");
    return safeTitle + "-" + formatExportTimestampSegment() + ".md";
  };

  const triggerTextFileDownload = (content, fileName, contentType) => {
    const blob = new Blob([content], { type: contentType });
    const objectURL = URL.createObjectURL(blob);
    try {
      const anchor = document.createElement("a");
      anchor.href = objectURL;
      anchor.download = fileName;
      anchor.rel = "noopener noreferrer";
      document.body.appendChild(anchor);
      anchor.click();
      document.body.removeChild(anchor);
    } finally {
      window.setTimeout(() => {
        URL.revokeObjectURL(objectURL);
      }, 15000);
    }
  };

  const toAbsoluteResourceURL = (rawURL, baseURL) => {
    const normalizedURL = typeof rawURL === "string" ? rawURL.trim() : "";
    if (!normalizedURL) {
      return normalizedURL;
    }
    if (/^(?:[a-z][a-z0-9+.-]*:|#)/i.test(normalizedURL)) {
      return normalizedURL;
    }
    if (normalizedURL.startsWith("//")) {
      try {
        return new URL(window.location.protocol + normalizedURL).toString();
      } catch {
        return normalizedURL;
      }
    }
    try {
      return new URL(normalizedURL, baseURL).toString();
    } catch {
      return normalizedURL;
    }
  };

  const rewriteMarkdownLinkDestination = (rawDestination, baseURL) => {
    const input = typeof rawDestination === "string" ? rawDestination : "";
    const leadingWhitespaceMatch = /^\\s*/.exec(input);
    const trailingWhitespaceMatch = /\\s*$/.exec(input);
    const leadingWhitespace = leadingWhitespaceMatch ? leadingWhitespaceMatch[0] : "";
    const trailingWhitespace = trailingWhitespaceMatch ? trailingWhitespaceMatch[0] : "";
    const core = input.slice(leadingWhitespace.length, input.length - trailingWhitespace.length);
    if (!core) {
      return input;
    }

    if (core.startsWith("<")) {
      const rightBracketIndex = core.indexOf(">");
      if (rightBracketIndex > 0) {
        const innerURL = core.slice(1, rightBracketIndex).trim();
        const suffix = core.slice(rightBracketIndex + 1);
        if (!innerURL) {
          return input;
        }
        return (
          leadingWhitespace +
          "<" +
          toAbsoluteResourceURL(innerURL, baseURL) +
          ">" +
          suffix +
          trailingWhitespace
        );
      }
    }

    const whitespaceIndex = core.search(/\\s/);
    const resourceURL = whitespaceIndex >= 0 ? core.slice(0, whitespaceIndex) : core;
    const suffix = whitespaceIndex >= 0 ? core.slice(whitespaceIndex) : "";
    if (!resourceURL) {
      return input;
    }
    return leadingWhitespace + toAbsoluteResourceURL(resourceURL, baseURL) + suffix + trailingWhitespace;
  };

  const rewriteMarkdownInlineLinks = (input, baseURL) =>
    input.replace(/(!?\\[[^\\]\\n]*\\]\\()([^)\\n]+)(\\))/g, (fullMatch, prefix, destination, suffix) => {
      const rewrittenDestination = rewriteMarkdownLinkDestination(destination, baseURL);
      return prefix + rewrittenDestination + suffix;
    });

  const rewriteMarkdownReferenceDefinition = (input, baseURL) =>
    input.replace(/^(\\s*\\[[^\\]\\r\\n]+\\]:\\s*)(\\S+)(.*)$/, (fullMatch, prefix, urlToken, suffix) => {
      const normalizedToken = typeof urlToken === "string" ? urlToken.trim() : "";
      if (!normalizedToken) {
        return fullMatch;
      }
      if (normalizedToken.startsWith("<") && normalizedToken.endsWith(">") && normalizedToken.length > 2) {
        const innerURL = normalizedToken.slice(1, -1).trim();
        if (!innerURL) {
          return fullMatch;
        }
        return prefix + "<" + toAbsoluteResourceURL(innerURL, baseURL) + ">" + suffix;
      }
      return prefix + toAbsoluteResourceURL(normalizedToken, baseURL) + suffix;
    });

  const rewriteHtmlResourceAttributes = (input, baseURL) =>
    input.replace(
      /(<(?:img|a)\\b[^>]*?\\b(?:src|href)\\s*=\\s*)(["'])([^"']+)(\\2)/gi,
      (fullMatch, prefix, quote, urlValue, suffix) => {
        const rewrittenURL = toAbsoluteResourceURL(urlValue, baseURL);
        return prefix + quote + rewrittenURL + suffix;
      }
    );

  const MARKDOWN_BACKTICK = String.fromCharCode(96);

  const rewriteOutsideInlineCode = (line, rewriteSegment) => {
    if (typeof line !== "string" || line.indexOf(MARKDOWN_BACKTICK) < 0) {
      return rewriteSegment(line);
    }
    let output = "";
    let cursor = 0;
    let activeTickSize = 0;

    while (cursor < line.length) {
      if (activeTickSize === 0) {
        const nextTickIndex = line.indexOf(MARKDOWN_BACKTICK, cursor);
        if (nextTickIndex < 0) {
          output += rewriteSegment(line.slice(cursor));
          break;
        }
        output += rewriteSegment(line.slice(cursor, nextTickIndex));
        let tickEndIndex = nextTickIndex;
        while (tickEndIndex < line.length && line.charAt(tickEndIndex) === MARKDOWN_BACKTICK) {
          tickEndIndex += 1;
        }
        activeTickSize = tickEndIndex - nextTickIndex;
        output += line.slice(nextTickIndex, tickEndIndex);
        cursor = tickEndIndex;
        continue;
      }

      const closingToken = MARKDOWN_BACKTICK.repeat(activeTickSize);
      const closingIndex = line.indexOf(closingToken, cursor);
      if (closingIndex < 0) {
        output += line.slice(cursor);
        break;
      }
      output += line.slice(cursor, closingIndex + activeTickSize);
      cursor = closingIndex + activeTickSize;
      activeTickSize = 0;
    }

    return output;
  };

  const resolveFenceMarker = (line) => {
    const normalizedLine = typeof line === "string" ? line : "";
    const trimmedLeftLine = normalizedLine.replace(/^\\s{0,3}/, "");
    if (!trimmedLeftLine) {
      return null;
    }
    const markerChar = trimmedLeftLine.charAt(0);
    if (markerChar !== MARKDOWN_BACKTICK && markerChar !== "~") {
      return null;
    }
    let markerLength = 0;
    while (markerLength < trimmedLeftLine.length && trimmedLeftLine.charAt(markerLength) === markerChar) {
      markerLength += 1;
    }
    if (markerLength < 3) {
      return null;
    }
    return { markerChar, markerLength };
  };

  const rewriteMarkdownForExport = (markdownText, baseURL) => {
    const normalizedText = typeof markdownText === "string" ? markdownText : "";
    if (!normalizedText) {
      return "";
    }

    const hasTrailingLineBreak = /\\r?\\n$/.test(normalizedText);
    const sourceLines = normalizedText.replace(/\\r\\n/g, "\\n").split("\\n");
    const rewrittenLines = [];
    let activeFenceMarker = null;

    for (const sourceLine of sourceLines) {
      const fenceMarker = resolveFenceMarker(sourceLine);
      if (fenceMarker) {
        if (!activeFenceMarker) {
          activeFenceMarker = fenceMarker;
          rewrittenLines.push(sourceLine);
          continue;
        }
        if (
          fenceMarker.markerChar === activeFenceMarker.markerChar &&
          fenceMarker.markerLength >= activeFenceMarker.markerLength
        ) {
          activeFenceMarker = null;
          rewrittenLines.push(sourceLine);
          continue;
        }
      }

      if (activeFenceMarker) {
        rewrittenLines.push(sourceLine);
        continue;
      }

      let rewrittenLine = rewriteOutsideInlineCode(sourceLine, (segment) => {
        let output = rewriteMarkdownInlineLinks(segment, baseURL);
        output = rewriteHtmlResourceAttributes(output, baseURL);
        return output;
      });
      rewrittenLine = rewriteMarkdownReferenceDefinition(rewrittenLine, baseURL);
      rewrittenLines.push(rewrittenLine);
    }

    let output = rewrittenLines.join("\\n");
    if (hasTrailingLineBreak && !output.endsWith("\\n")) {
      output += "\\n";
    }
    return output;
  };

  const handleMarkdownExport = (actionButton) => {
    if (!(actionButton instanceof HTMLButtonElement)) {
      return;
    }
    actionButton.disabled = true;
    actionButton.classList.add(EXPORT_ACTION_BUSY_CLASS);
    try {
      const payload = resolveReaderStatePayload();
      if (!payload || typeof payload !== "object") {
        throw new Error("页面状态读取失败，请刷新页面后重试。");
      }
      const documentData = payload.document && typeof payload.document === "object" ? payload.document : null;
      if (!documentData) {
        throw new Error("文档数据缺失，请刷新页面后重试。");
      }
      const markdownContent = typeof documentData.contentMd === "string" ? documentData.contentMd : "";
      const baseURL = resolveExportBaseURL(payload);
      const rewrittenMarkdown = rewriteMarkdownForExport(markdownContent, baseURL);
      const documentTitle = typeof documentData.title === "string" ? documentData.title : "";
      const exportFileName = buildMarkdownExportFileName(documentTitle);
      triggerTextFileDownload(rewrittenMarkdown, exportFileName, "text/markdown;charset=utf-8");
    } catch (error) {
      console.error("[reader][export] markdown export failed", error);
    } finally {
      actionButton.disabled = false;
      actionButton.classList.remove(EXPORT_ACTION_BUSY_CLASS);
    }
  };

  const handlePdfExport = (actionButton) => {
    if (!(actionButton instanceof HTMLButtonElement)) {
      return;
    }
    actionButton.disabled = true;
    actionButton.classList.add(EXPORT_ACTION_BUSY_CLASS);
    try {
      const attachmentLinks = document.querySelectorAll(ATTACHMENT_LINK_SELECTOR);
      for (const node of attachmentLinks) {
        if (!(node instanceof HTMLAnchorElement)) {
          continue;
        }
        const rawHref = (node.getAttribute("href") || "").trim();
        if (!rawHref) {
          continue;
        }
        try {
          const absoluteURL = new URL(rawHref, window.location.href).toString();
          node.href = absoluteURL;
          node.setAttribute("data-reader-print-url", absoluteURL);
        } catch {
          node.removeAttribute("data-reader-print-url");
        }
      }
      window.print();
    } catch (error) {
      console.error("[reader][export] pdf print failed", error);
    } finally {
      actionButton.disabled = false;
      actionButton.classList.remove(EXPORT_ACTION_BUSY_CLASS);
    }
  };

  const handleExportAction = (actionButton) => {
    if (!(actionButton instanceof HTMLButtonElement)) {
      return;
    }
    const exportAction = (actionButton.getAttribute("data-reader-export-action") || "").trim().toLowerCase();
    if (exportAction === "markdown") {
      handleMarkdownExport(actionButton);
      return;
    }
    if (exportAction === "pdf") {
      handlePdfExport(actionButton);
    }
  };

  const triggerAttachmentNavigation = (targetURL, purpose) => {
    const normalizedURL = typeof targetURL === "string" ? targetURL.trim() : "";
    if (!normalizedURL) {
      return false;
    }
    const anchor = document.createElement("a");
    anchor.href = normalizedURL;
    if (purpose === "preview") {
      anchor.target = "_blank";
      anchor.rel = "noopener noreferrer";
    }
    document.body.appendChild(anchor);
    anchor.click();
    document.body.removeChild(anchor);
    return true;
  };

  const buildAttachmentPreviewPageURL = (documentID, attachmentID, customPreviewPagePath) => {
    const normalizedCustomPath = typeof customPreviewPagePath === "string" ? customPreviewPagePath.trim() : "";
    if (normalizedCustomPath) {
      return normalizedCustomPath;
    }
    const normalizedDocumentID = typeof documentID === "string" ? documentID.trim() : "";
    const normalizedAttachmentID = typeof attachmentID === "string" ? attachmentID.trim() : "";
    if (!normalizedDocumentID || !normalizedAttachmentID) {
      return "";
    }
    return (
      "/preview/docs/" +
      encodeURIComponent(normalizedDocumentID) +
      "/attachments/" +
      encodeURIComponent(normalizedAttachmentID)
    );
  };

  const requestAttachmentAccessLink = async (actionButton) => {
    if (!(actionButton instanceof HTMLButtonElement)) {
      return;
    }
    const documentID = (actionButton.getAttribute("data-reader-doc-id") || "").trim();
    const attachmentID = (actionButton.getAttribute("data-reader-attachment-id") || "").trim();
    const purposeValue = (actionButton.getAttribute("data-reader-attachment-purpose") || "").trim();
    const customAccessLinkPath = (actionButton.getAttribute("data-reader-attachment-access-link-path") || "").trim();
    const customPreviewPagePath = (actionButton.getAttribute("data-reader-attachment-preview-page-path") || "").trim();
    const previewDirectMode = (actionButton.getAttribute("data-reader-attachment-preview-direct") || "").trim() === "1";
    const purpose = purposeValue === "preview" ? "preview" : "download";
    if (!documentID || !attachmentID) {
      setAttachmentStatus("附件参数无效，请刷新页面后重试。", true);
      return;
    }
    if (purpose === "preview" && !previewDirectMode) {
      const previewPageURL = buildAttachmentPreviewPageURL(documentID, attachmentID, customPreviewPagePath);
      if (!previewPageURL || !triggerAttachmentNavigation(previewPageURL, "preview")) {
        setAttachmentStatus("打开预览页失败，请稍后重试。", true);
        return;
      }
      setAttachmentStatus("已打开预览。", false);
      return;
    }

    actionButton.disabled = true;
    actionButton.classList.add(ATTACHMENT_ACTION_BUSY_CLASS);
    setAttachmentStatus("正在生成下载链接...", false);

    try {
      const requestPath = customAccessLinkPath
        ? customAccessLinkPath
        : "/api/docs/" +
          encodeURIComponent(documentID) +
          "/attachments/" +
          encodeURIComponent(attachmentID) +
          "/access-link?purpose=" +
          encodeURIComponent(purpose);
      const response = await fetch(requestPath, {
        method: "POST",
        credentials: "include",
        headers: {
          Accept: "application/json",
          "X-Requested-With": "plaindoc-reader-async"
        }
      });
      const rawResponseText = await response.text();
      const payload = resolveJSONPayload(rawResponseText);
      const responseData = resolveJsonResultData(payload);
      const responseCode = payload && typeof payload === "object" && typeof payload.code === "number" ? payload.code : 0;
      const responseMessage =
        payload && typeof payload === "object" && typeof payload.message === "string" ? payload.message.trim() : "";
      const accessURL = responseData && typeof responseData === "object" && typeof responseData.url === "string"
        ? responseData.url.trim()
        : "";
      if (!response.ok || responseCode !== 0 || !accessURL) {
        throw new Error(responseMessage || "附件访问链接生成失败");
      }
      if (!triggerAttachmentNavigation(accessURL, purpose)) {
        throw new Error("附件访问链接无效");
      }
      setAttachmentStatus(purpose === "preview" ? "已打开预览。" : "已开始下载。", false);
    } catch (error) {
      const message =
        error && typeof error === "object" && "message" in error && typeof error.message === "string"
          ? error.message
          : "附件操作失败，请稍后重试。";
      setAttachmentStatus(message, true);
    } finally {
      actionButton.disabled = false;
      actionButton.classList.remove(ATTACHMENT_ACTION_BUSY_CLASS);
    }
  };

  const expandAncestorsForActiveRow = () => {
    const allDetails = document.querySelectorAll(TREE_DETAILS_SELECTOR);
    for (const detailNode of allDetails) {
      if (detailNode instanceof HTMLDetailsElement) {
        detailNode.open = false;
      }
    }
    const activeRow = queryActiveRow();
    if (!(activeRow instanceof HTMLElement)) {
      return;
    }

    const activeDetails = activeRow.closest(TREE_DETAILS_SELECTOR);
    const activeSummaryElement = activeRow.closest(TREE_SUMMARY_SELECTOR);
    const isActiveInsideOwnSummary =
      activeDetails instanceof HTMLDetailsElement &&
      activeSummaryElement instanceof HTMLElement &&
      activeDetails.firstElementChild === activeSummaryElement;
    let parentElement =
      activeDetails instanceof HTMLDetailsElement
        ? isActiveInsideOwnSummary
          ? activeDetails.parentElement
          : activeDetails
        : activeRow.parentElement;
    while (parentElement) {
      if (parentElement instanceof HTMLDetailsElement) {
        parentElement.open = true;
      }
      parentElement = parentElement.parentElement;
    }
    activeRow.scrollIntoView({ block: "nearest", inline: "nearest" });
  };

  const progressRoot = document.querySelector(PROGRESS_SELECTOR);
  let progressTimer = 0;
  let progressValue = 0;
  const setProgress = (value) => {
    if (!(progressRoot instanceof HTMLElement)) {
      return;
    }
    progressValue = Math.max(0, Math.min(100, value));
    progressRoot.style.setProperty("--reader-progress", progressValue.toFixed(2) + "%");
  };
  const startProgress = () => {
    if (!(progressRoot instanceof HTMLElement)) {
      return;
    }
    if (progressTimer) {
      window.clearInterval(progressTimer);
      progressTimer = 0;
    }
    progressRoot.classList.add(PROGRESS_VISIBLE_CLASS);
    setProgress(8);
    progressTimer = window.setInterval(() => {
      if (progressValue >= 90) {
        return;
      }
      const increment = Math.max(1.2, (90 - progressValue) * 0.16);
      setProgress(progressValue + increment);
    }, 120);
  };
  const finishProgress = () => {
    if (!(progressRoot instanceof HTMLElement)) {
      return;
    }
    if (progressTimer) {
      window.clearInterval(progressTimer);
      progressTimer = 0;
    }
    setProgress(100);
    window.setTimeout(() => {
      progressRoot.classList.remove(PROGRESS_VISIBLE_CLASS);
      setProgress(0);
    }, 220);
  };

  const syncHeadStyleByID = (styleID, nextDocument) => {
    const nextStyleNode = nextDocument.getElementById(styleID);
    const nextStyleText = nextStyleNode ? nextStyleNode.textContent || "" : "";
    const currentStyleNode = document.getElementById(styleID);
    if (!nextStyleText.trim()) {
      if (currentStyleNode && currentStyleNode.parentNode) {
        currentStyleNode.parentNode.removeChild(currentStyleNode);
      }
      return;
    }
    if (currentStyleNode instanceof HTMLStyleElement) {
      currentStyleNode.textContent = nextStyleText;
      return;
    }
    const createdStyleNode = document.createElement("style");
    createdStyleNode.id = styleID;
    createdStyleNode.textContent = nextStyleText;
    document.head.appendChild(createdStyleNode);
  };

  const syncReaderStateScript = (nextDocument) => {
    const nextStateNode = nextDocument.querySelector(STATE_SCRIPT_SELECTOR);
    const currentStateNode = document.querySelector(STATE_SCRIPT_SELECTOR);
    if (!(nextStateNode instanceof HTMLScriptElement) || !(currentStateNode instanceof HTMLScriptElement)) {
      return;
    }
    currentStateNode.textContent = nextStateNode.textContent || "{}";
  };

  const syncDocumentHead = (nextDocument, targetURL) => {
    if (typeof nextDocument.title === "string" && nextDocument.title.trim()) {
      document.title = nextDocument.title.trim();
    }
    const nextCanonicalNode = nextDocument.querySelector("link[rel='canonical']");
    let currentCanonicalNode = document.querySelector("link[rel='canonical']");
    if (!(currentCanonicalNode instanceof HTMLLinkElement)) {
      currentCanonicalNode = document.createElement("link");
      currentCanonicalNode.setAttribute("rel", "canonical");
      document.head.appendChild(currentCanonicalNode);
    }
    if (nextCanonicalNode instanceof HTMLLinkElement && nextCanonicalNode.href) {
      currentCanonicalNode.href = nextCanonicalNode.href;
    } else {
      currentCanonicalNode.href = targetURL.toString();
    }

    const nextRobotsNode = nextDocument.querySelector("meta[name='robots']");
    let currentRobotsNode = document.querySelector("meta[name='robots']");
    const nextRobotsContent =
      nextRobotsNode instanceof HTMLMetaElement ? (nextRobotsNode.getAttribute("content") || "").trim() : "";
    if (!nextRobotsContent) {
      if (currentRobotsNode instanceof HTMLMetaElement) {
        currentRobotsNode.remove();
      }
      return;
    }
    if (!(currentRobotsNode instanceof HTMLMetaElement)) {
      currentRobotsNode = document.createElement("meta");
      currentRobotsNode.setAttribute("name", "robots");
      document.head.appendChild(currentRobotsNode);
    }
    currentRobotsNode.setAttribute("content", nextRobotsContent);
  };

  const syncMobileBarTitle = (nextDocument) => {
    const currentTitleNode = document.querySelector(MOBILE_BAR_TITLE_SELECTOR);
    if (!(currentTitleNode instanceof HTMLElement)) {
      return;
    }
    const nextTitleNode = nextDocument.querySelector(MOBILE_BAR_TITLE_SELECTOR);
    let nextTitle = "";
    if (nextTitleNode instanceof HTMLElement) {
      nextTitle = (nextTitleNode.textContent || "").trim();
    }
    if (!nextTitle) {
      const fallbackTitleNode = nextDocument.querySelector(".reader-article-title");
      if (fallbackTitleNode instanceof HTMLElement) {
        nextTitle = (fallbackTitleNode.textContent || "").trim();
      }
    }
    if (!nextTitle) {
      return;
    }
    currentTitleNode.textContent = nextTitle;
    currentTitleNode.setAttribute("title", nextTitle);
  };

  const replaceArticleShell = (nextDocument) => {
    const currentArticleShell = document.querySelector(ARTICLE_SHELL_SELECTOR);
    const nextArticleShell =
      nextDocument.querySelector(ARTICLE_SHELL_SELECTOR) || nextDocument.querySelector(".reader-article-shell");
    if (!(currentArticleShell instanceof HTMLElement) || !(nextArticleShell instanceof HTMLElement)) {
      return false;
    }
    const importedArticleShell = document.importNode(nextArticleShell, true);
    if (importedArticleShell instanceof HTMLElement) {
      importedArticleShell.id = "plaindoc-reader-article-shell";
    }
    currentArticleShell.replaceWith(importedArticleShell);
    return true;
  };

  const markActiveTreeItemByPathname = (pathname) => {
    const normalizedPathname = normalizePathname(pathname);
    for (const rowNode of document.querySelectorAll(TREE_ROW_SELECTOR)) {
      if (!(rowNode instanceof HTMLElement)) {
        continue;
      }
      rowNode.removeAttribute("data-reader-active");
      rowNode.classList.remove(TREE_ROW_ACTIVE_CLASS);
    }
    for (const labelNode of document.querySelectorAll(TREE_LABEL_SELECTOR)) {
      if (!(labelNode instanceof HTMLElement)) {
        continue;
      }
      labelNode.removeAttribute("data-reader-label-active");
      labelNode.classList.remove(TREE_LABEL_ACTIVE_CLASS);
    }

    let matchedLink = null;
    for (const linkNode of document.querySelectorAll(DOC_LINK_SELECTOR)) {
      if (!(linkNode instanceof HTMLAnchorElement)) {
        continue;
      }
      const linkURL = toSameOriginURL(linkNode.href);
      if (!linkURL) {
        continue;
      }
      if (normalizePathname(linkURL.pathname) === normalizedPathname) {
        matchedLink = linkNode;
        break;
      }
    }
    if (!(matchedLink instanceof HTMLAnchorElement)) {
      return;
    }

    const rowElement = matchedLink.closest(TREE_ROW_SELECTOR);
    if (rowElement instanceof HTMLElement) {
      rowElement.setAttribute("data-reader-active", "1");
      rowElement.classList.add(TREE_ROW_ACTIVE_CLASS);
      rowElement.scrollIntoView({ block: "nearest", inline: "nearest" });
    }
    const labelElement =
      matchedLink.matches(TREE_LABEL_SELECTOR)
        ? matchedLink
        : matchedLink.querySelector(TREE_LABEL_SELECTOR);
    if (labelElement instanceof HTMLElement) {
      labelElement.setAttribute("data-reader-label-active", "1");
      labelElement.classList.add(TREE_LABEL_ACTIVE_CLASS);
    }

    const activeDetails = rowElement instanceof HTMLElement ? rowElement.closest(TREE_DETAILS_SELECTOR) : null;
    const activeSummaryElement = rowElement instanceof HTMLElement ? rowElement.closest(TREE_SUMMARY_SELECTOR) : null;
    const isActiveInsideOwnSummary =
      activeDetails instanceof HTMLDetailsElement &&
      activeSummaryElement instanceof HTMLElement &&
      activeDetails.firstElementChild === activeSummaryElement;
    let parentNode =
      activeDetails instanceof HTMLDetailsElement
        ? isActiveInsideOwnSummary
          ? activeDetails.parentElement
          : activeDetails
        : matchedLink.parentElement;
    while (parentNode) {
      if (parentNode instanceof HTMLDetailsElement) {
        parentNode.open = true;
      }
      parentNode = parentNode.parentElement;
    }
  };

  let outlineLinks = [];
  let outlineHeadings = [];
  let outlineRAF = 0;

  const collectOutlineLinks = () =>
    Array.from(document.querySelectorAll(OUTLINE_LINK_SELECTOR)).filter((node) => node instanceof HTMLElement);

  const collectOutlineHeadings = () =>
    Array.from(document.querySelectorAll(PREVIEW_HEADING_SELECTOR)).filter(
      (node) => node instanceof HTMLElement && String(node.textContent || "").trim().length > 0
    );

  const clearOutlineActiveState = () => {
    for (const linkNode of outlineLinks) {
      if (!(linkNode instanceof HTMLElement)) {
        continue;
      }
      linkNode.classList.remove(OUTLINE_LINK_ACTIVE_CLASS);
    }
  };

  const setOutlineActiveIndex = (activeIndex) => {
    for (const linkNode of outlineLinks) {
      if (!(linkNode instanceof HTMLElement)) {
        continue;
      }
      const linkIndex = Number(linkNode.getAttribute("data-outline-index"));
      const isActive = Number.isFinite(linkIndex) && linkIndex === activeIndex;
      linkNode.classList.toggle(OUTLINE_LINK_ACTIVE_CLASS, isActive);
    }
  };

  const resolveOutlineActiveIndex = () => {
    const readerMain = document.querySelector(MAIN_SELECTOR);
    if (!(readerMain instanceof HTMLElement) || !outlineHeadings.length) {
      return null;
    }
    const readerMainRect = readerMain.getBoundingClientRect();
    let activeIndex = 0;
    for (let headingIndex = 0; headingIndex < outlineHeadings.length; headingIndex += 1) {
      const headingNode = outlineHeadings[headingIndex];
      if (!(headingNode instanceof HTMLElement)) {
        continue;
      }
      const relativeTop = headingNode.getBoundingClientRect().top - readerMainRect.top;
      if (relativeTop <= OUTLINE_ACTIVE_OFFSET) {
        activeIndex = headingIndex;
        continue;
      }
      break;
    }
    return activeIndex;
  };

  const syncOutlineActiveState = () => {
    if (!outlineLinks.length || !outlineHeadings.length) {
      clearOutlineActiveState();
      return;
    }
    const activeIndex = resolveOutlineActiveIndex();
    if (!Number.isFinite(activeIndex)) {
      clearOutlineActiveState();
      return;
    }
    setOutlineActiveIndex(activeIndex);
  };

  const scheduleOutlineActiveSync = () => {
    if (outlineRAF) {
      window.cancelAnimationFrame(outlineRAF);
    }
    outlineRAF = window.requestAnimationFrame(() => {
      outlineRAF = 0;
      syncOutlineActiveState();
    });
  };

  const refreshOutlineRegistry = () => {
    outlineLinks = collectOutlineLinks();
    outlineHeadings = collectOutlineHeadings();
    scheduleOutlineActiveSync();
  };

  const scrollToOutlineIndex = (outlineIndex) => {
    const readerMain = document.querySelector(MAIN_SELECTOR);
    if (!(readerMain instanceof HTMLElement)) {
      return;
    }
    if (!Number.isFinite(outlineIndex) || outlineIndex < 0 || outlineIndex >= outlineHeadings.length) {
      return;
    }
    const targetHeading = outlineHeadings[outlineIndex];
    if (!(targetHeading instanceof HTMLElement)) {
      return;
    }
    const readerMainRect = readerMain.getBoundingClientRect();
    const targetTop =
      targetHeading.getBoundingClientRect().top - readerMainRect.top + readerMain.scrollTop - OUTLINE_SCROLL_OFFSET;
    readerMain.scrollTo({
      top: Math.max(0, targetTop),
      behavior: "smooth"
    });
    setOutlineActiveIndex(outlineIndex);
  };

  const replaceReaderOutline = (nextDocument) => {
    const readerMain = document.querySelector(MAIN_SELECTOR);
    if (!(readerMain instanceof HTMLElement)) {
      return false;
    }
    const currentOutlineNode = document.querySelector(OUTLINE_SELECTOR);
    const nextOutlineNode = nextDocument.querySelector(OUTLINE_SELECTOR);
    if (!(nextOutlineNode instanceof HTMLElement)) {
      if (currentOutlineNode instanceof HTMLElement) {
        currentOutlineNode.remove();
      }
      readerMain.removeAttribute("data-reader-outline");
      return true;
    }

    const importedOutlineNode = document.importNode(nextOutlineNode, true);
    if (currentOutlineNode instanceof HTMLElement) {
      currentOutlineNode.replaceWith(importedOutlineNode);
    } else {
      const articleShellNode = document.querySelector(ARTICLE_SHELL_SELECTOR);
      if (articleShellNode instanceof HTMLElement && articleShellNode.parentElement === readerMain) {
        readerMain.insertBefore(importedOutlineNode, articleShellNode);
      } else if (readerMain.firstChild) {
        readerMain.insertBefore(importedOutlineNode, readerMain.firstChild);
      } else {
        readerMain.appendChild(importedOutlineNode);
      }
    }
    readerMain.setAttribute("data-reader-outline", "1");
    return true;
  };

  let inflightController = null;
  let inflightSeq = 0;
  const loadReaderPage = async (targetURL, pushHistory) => {
    if (!(targetURL instanceof URL)) {
      return;
    }
    const targetPathname = normalizePathname(targetURL.pathname);
    if (pushHistory && targetPathname === normalizePathname(window.location.pathname)) {
      return;
    }

    inflightSeq += 1;
    const currentSeq = inflightSeq;
    if (inflightController instanceof AbortController) {
      inflightController.abort();
    }
    inflightController = new AbortController();
    startProgress();

    try {
      const response = await fetch(targetURL.toString(), {
        method: "GET",
        credentials: "include",
        signal: inflightController.signal,
        headers: {
          Accept: "text/html",
          "X-Requested-With": "plaindoc-reader-async"
        }
      });
      if (currentSeq !== inflightSeq) {
        return;
      }
      if (!response.ok) {
        window.location.assign(targetURL.toString());
        return;
      }
      const htmlText = await response.text();
      if (currentSeq !== inflightSeq) {
        return;
      }
      const parsedDocument = new DOMParser().parseFromString(htmlText, "text/html");
      destroyOnlyOfficeReader();
      if (!replaceArticleShell(parsedDocument)) {
        window.location.assign(targetURL.toString());
        return;
      }
      if (!replaceReaderOutline(parsedDocument)) {
        window.location.assign(targetURL.toString());
        return;
      }

      syncDocumentHead(parsedDocument, targetURL);
      syncHeadStyleByID(GOOGLE_SANS_CODE_STYLE_ID, parsedDocument);
      syncHeadStyleByID(APP_STYLE_ID, parsedDocument);
      syncHeadStyleByID(BASE_STYLE_ID, parsedDocument);
      syncHeadStyleByID(KATEX_STYLE_ID, parsedDocument);
      syncHeadStyleByID(THEME_STYLE_ID, parsedDocument);
      syncHeadStyleByID(THEME_CUSTOM_STYLE_ID, parsedDocument);
      syncReaderStateScript(parsedDocument);
      syncMobileBarTitle(parsedDocument);
      markActiveTreeItemByPathname(targetPathname);
      setMobileSidebarOpen(false);

      const readerMain = document.querySelector(MAIN_SELECTOR);
      if (readerMain instanceof HTMLElement) {
        readerMain.scrollTop = 0;
      }
      refreshOutlineRegistry();
      syncSpreadsheetTabs();
      scheduleOfficePaneViewportHeightSync();
      void syncOfficeDownloadAction();
      await syncOnlyOfficeReader();
      if (pushHistory) {
        window.history.pushState({ reader: true }, "", targetURL.toString());
      }
    } catch (error) {
      if (
        error &&
        typeof error === "object" &&
        "name" in error &&
        (error).name === "AbortError"
      ) {
        return;
      }
      window.location.assign(targetURL.toString());
    } finally {
      if (currentSeq === inflightSeq) {
        finishProgress();
      }
    }
  };

  try {
    expandAncestorsForActiveRow();
    markActiveTreeItemByPathname(window.location.pathname);
    refreshOutlineRegistry();
    setMobileSidebarOpen(false);
    syncSpreadsheetTabs();
    void syncOfficeDownloadAction();
    void syncOnlyOfficeReader();
  } catch {
    // no-op: initialization enhancement should never block rendering.
  }

  try {
    document.addEventListener(
      "click",
      (event) => {
        if (!(event.target instanceof Element)) {
          return;
        }

        const exportActionButton = event.target.closest(EXPORT_ACTION_SELECTOR);
        if (exportActionButton instanceof HTMLButtonElement) {
          if (event.defaultPrevented || isModifiedClick(event)) {
            return;
          }
          event.preventDefault();
          event.stopPropagation();
          handleExportAction(exportActionButton);
          return;
        }

        const attachmentActionButton = event.target.closest(ATTACHMENT_ACTION_SELECTOR);
        if (attachmentActionButton instanceof HTMLButtonElement) {
          if (event.defaultPrevented || isModifiedClick(event)) {
            return;
          }
          event.preventDefault();
          event.stopPropagation();
          void requestAttachmentAccessLink(attachmentActionButton);
          return;
        }

        const officeDownloadLink = event.target.closest(OFFICE_DOWNLOAD_SELECTOR);
        if (officeDownloadLink instanceof HTMLAnchorElement) {
          if (event.defaultPrevented) {
            return;
          }
          event.preventDefault();
          event.stopPropagation();
          void requestOfficeSourceDownload();
          return;
        }

        const mobileSidebarOpenButton = event.target.closest(MOBILE_SIDEBAR_OPEN_TRIGGER_SELECTOR);
        if (mobileSidebarOpenButton instanceof HTMLButtonElement) {
          if (event.defaultPrevented || isModifiedClick(event)) {
            return;
          }
          event.preventDefault();
          event.stopPropagation();
          setMobileSidebarOpen(true);
          return;
        }

        const mobileSidebarCloseButton = event.target.closest(MOBILE_SIDEBAR_CLOSE_TRIGGER_SELECTOR);
        if (mobileSidebarCloseButton instanceof HTMLButtonElement) {
          if (event.defaultPrevented || isModifiedClick(event)) {
            return;
          }
          event.preventDefault();
          event.stopPropagation();
          setMobileSidebarOpen(false);
          return;
        }

        const mobileSidebarOverlay = event.target.closest(MOBILE_SIDEBAR_OVERLAY_SELECTOR);
        if (mobileSidebarOverlay instanceof HTMLElement) {
          if (event.defaultPrevented || isModifiedClick(event)) {
            return;
          }
          event.preventDefault();
          event.stopPropagation();
          setMobileSidebarOpen(false);
          return;
        }

        const outlineLink = event.target.closest(OUTLINE_LINK_SELECTOR);
        if (outlineLink instanceof HTMLElement) {
          if (event.defaultPrevented || isModifiedClick(event)) {
            return;
          }
          const outlineIndex = Number(outlineLink.getAttribute("data-outline-index"));
          if (!Number.isFinite(outlineIndex)) {
            return;
          }
          event.preventDefault();
          event.stopPropagation();
          scrollToOutlineIndex(Math.max(0, Math.floor(outlineIndex)));
          return;
        }

        const spreadsheetTab = event.target.closest(OFFICE_XLSX_TAB_SELECTOR);
        if (spreadsheetTab instanceof HTMLButtonElement) {
          if (event.defaultPrevented || isModifiedClick(event)) {
            return;
          }
          const spreadsheetReader = spreadsheetTab.closest(OFFICE_XLSX_READER_SELECTOR);
          const sheetKey = (spreadsheetTab.getAttribute("data-office-sheet-tab") || "").trim();
          if (!(spreadsheetReader instanceof HTMLElement) || !sheetKey) {
            return;
          }
          event.preventDefault();
          event.stopPropagation();
          activateSpreadsheetTab(spreadsheetReader, sheetKey, true);
          return;
        }

        const docLink = event.target.closest(DOC_LINK_SELECTOR);
        if (docLink instanceof HTMLAnchorElement) {
          if (event.defaultPrevented || isModifiedClick(event)) {
            return;
          }
          const targetURL = toSameOriginURL(docLink.getAttribute("href") || docLink.href);
          if (!targetURL) {
            return;
          }
          event.preventDefault();
          event.stopPropagation();
          setMobileSidebarOpen(false);
          void loadReaderPage(targetURL, true);
          return;
        }

        const summaryElement = event.target.closest(TREE_SUMMARY_SELECTOR);
        if (!(summaryElement instanceof HTMLElement)) {
          return;
        }
        const detailsElement = summaryElement.parentElement;
        if (!(detailsElement instanceof HTMLDetailsElement)) {
          return;
        }
        const clickedArrow = event.target.closest(TREE_ARROW_SELECTOR) instanceof HTMLElement;
        const hasDocumentLink = summaryElement.querySelector(DOC_LINK_SELECTOR) instanceof HTMLAnchorElement;
        const allowSummaryToggle = clickedArrow || !hasDocumentLink;
        if (allowSummaryToggle) {
          syncNestedTreeStateAfterExpand(detailsElement);
          return;
        }

        event.preventDefault();
        event.stopPropagation();
      },
      true
    );
  } catch {
    // no-op: click behavior enhancement should never block rendering.
  }

  try {
    window.addEventListener("popstate", () => {
      const targetURL = toSameOriginURL(window.location.href);
      if (!targetURL) {
        return;
      }
      setMobileSidebarOpen(false);
      void loadReaderPage(targetURL, false);
    });
  } catch {
    // no-op: history enhancement should never block rendering.
  }

  try {
    window.addEventListener("keydown", (event) => {
      if (!(event instanceof KeyboardEvent)) {
        return;
      }
      if (event.key !== "Escape") {
        return;
      }
      setMobileSidebarOpen(false);
    });
  } catch {
    // no-op: mobile sidebar enhancement should never block rendering.
  }

  try {
    const readerMain = document.querySelector(MAIN_SELECTOR);
    if (readerMain instanceof HTMLElement) {
      readerMain.addEventListener("scroll", scheduleOutlineActiveSync, { passive: true });
    }
    document.addEventListener(
      "scroll",
      (event) => {
        const scrollTarget = event.target;
        if (!(scrollTarget instanceof HTMLElement) || !scrollTarget.matches(OFFICE_XLSX_TABLE_WRAP_SELECTOR)) {
          return;
        }
        syncSpreadsheetStickyHeader(scrollTarget);
      },
      { passive: true, capture: true }
    );
    window.addEventListener("resize", () => {
      if (!isMobileSidebarViewport()) {
        setMobileSidebarOpen(false);
      } else {
        syncMobileSidebarDOMState();
      }
      refreshOutlineRegistry();
      scheduleOfficePaneViewportHeightSync();
      syncSpreadsheetStickyHeaders(document);
    }, { passive: true });
  } catch {
    // no-op: outline enhancement should never block rendering.
  }

  try {
    const tooltipTargetSelector = TREE_TITLE_TOOLTIP_SELECTOR;
    const tooltipClassName = "reader-floating-tooltip";
    const tooltipVisibleClassName = "reader-floating-tooltip--visible";
    const viewportPadding = 10;
    const tooltipOffset = 8;
    let activeTarget = null;
    let tooltipNode = null;
    let rafId = 0;

    const ensureTooltipNode = () => {
      if (tooltipNode instanceof HTMLElement) {
        return tooltipNode;
      }
      const createdNode = document.createElement("div");
      createdNode.className = tooltipClassName;
      createdNode.setAttribute("role", "tooltip");
      createdNode.setAttribute("aria-hidden", "true");
      document.body.appendChild(createdNode);
      tooltipNode = createdNode;
      return createdNode;
    };

    const updateTooltipPosition = () => {
      rafId = 0;
      if (!(activeTarget instanceof HTMLElement) || !(tooltipNode instanceof HTMLElement)) {
        return;
      }
      const anchorRect = activeTarget.getBoundingClientRect();
      tooltipNode.style.left = viewportPadding + "px";
      tooltipNode.style.top = "-9999px";

      const tooltipRect = tooltipNode.getBoundingClientRect();
      let left = anchorRect.left;
      const maxLeft = window.innerWidth - viewportPadding - tooltipRect.width;
      if (left > maxLeft) {
        left = maxLeft;
      }
      if (left < viewportPadding) {
        left = viewportPadding;
      }

      let top = anchorRect.top - tooltipRect.height - tooltipOffset;
      if (top < viewportPadding) {
        top = anchorRect.bottom + tooltipOffset;
      }
      const maxTop = window.innerHeight - viewportPadding - tooltipRect.height;
      if (top > maxTop) {
        top = maxTop;
      }
      if (top < viewportPadding) {
        top = viewportPadding;
      }

      tooltipNode.style.left = Math.round(left) + "px";
      tooltipNode.style.top = Math.round(top) + "px";
    };

    const scheduleTooltipPositionUpdate = () => {
      if (rafId) {
        window.cancelAnimationFrame(rafId);
      }
      rafId = window.requestAnimationFrame(updateTooltipPosition);
    };

    const showTooltip = (target) => {
      const tooltipText = (target.getAttribute("data-tooltip") || "").trim();
      if (!tooltipText) {
        return;
      }
      activeTarget = target;
      const ensuredTooltipNode = ensureTooltipNode();
      ensuredTooltipNode.textContent = tooltipText;
      ensuredTooltipNode.setAttribute("aria-hidden", "false");
      ensuredTooltipNode.classList.add(tooltipVisibleClassName);
      scheduleTooltipPositionUpdate();
    };

    const hideTooltip = () => {
      activeTarget = null;
      if (rafId) {
        window.cancelAnimationFrame(rafId);
        rafId = 0;
      }
      if (!(tooltipNode instanceof HTMLElement)) {
        return;
      }
      tooltipNode.classList.remove(tooltipVisibleClassName);
      tooltipNode.setAttribute("aria-hidden", "true");
      tooltipNode.style.left = "-9999px";
      tooltipNode.style.top = "-9999px";
    };

    const tooltipTargets = document.querySelectorAll(tooltipTargetSelector);
    for (const targetNode of tooltipTargets) {
      if (!(targetNode instanceof HTMLElement)) {
        continue;
      }
      targetNode.addEventListener("mouseenter", () => showTooltip(targetNode));
      targetNode.addEventListener("mouseleave", hideTooltip);
      targetNode.addEventListener("focusin", () => showTooltip(targetNode));
      targetNode.addEventListener("focusout", hideTooltip);
    }

    window.addEventListener(
      "scroll",
      () => {
        if (activeTarget) {
          scheduleTooltipPositionUpdate();
        }
      },
      true
    );
    window.addEventListener("resize", () => {
      if (activeTarget) {
        scheduleTooltipPositionUpdate();
      }
    });
  } catch {
    // no-op: tooltip enhancement should never block rendering.
  }
})();`;
