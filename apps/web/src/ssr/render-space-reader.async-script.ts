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
  const EXPORT_ACTION_SELECTOR = "button[data-reader-export-action]";
  const EXPORT_ACTION_BUSY_CLASS = "reader-article-action--busy";
  const PREVIEW_HEADING_SELECTOR =
    "#plaindoc-preview-body h1, #plaindoc-preview-body h2, #plaindoc-preview-body h3, #plaindoc-preview-body h4, #plaindoc-preview-body h5, #plaindoc-preview-body h6";
  const OUTLINE_SCROLL_OFFSET = 16;
  const OUTLINE_ACTIVE_OFFSET = 108;
  const queryActiveRow = () => document.querySelector(TREE_ROW_ACTIVE_SELECTOR);

  const normalizePathname = (pathname) => pathname.replace(/\\/+$/, "") || "/";
  const isModifiedClick = (event) =>
    !(event instanceof MouseEvent) ||
    event.button !== 0 ||
    event.metaKey ||
    event.ctrlKey ||
    event.shiftKey ||
    event.altKey;

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

  const buildAttachmentPreviewPageURL = (documentID, attachmentID) => {
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
    const purpose = purposeValue === "preview" ? "preview" : "download";
    if (!documentID || !attachmentID) {
      setAttachmentStatus("附件参数无效，请刷新页面后重试。", true);
      return;
    }
    if (purpose === "preview") {
      const previewPageURL = buildAttachmentPreviewPageURL(documentID, attachmentID);
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
      const requestPath =
        "/api/docs/" +
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
      markActiveTreeItemByPathname(targetPathname);

      const readerMain = document.querySelector(MAIN_SELECTOR);
      if (readerMain instanceof HTMLElement) {
        readerMain.scrollTop = 0;
      }
      refreshOutlineRegistry();
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
          void loadReaderPage(targetURL, true);
          return;
        }

        const summaryElement = event.target.closest(TREE_SUMMARY_SELECTOR);
        if (!(summaryElement instanceof HTMLElement)) {
          return;
        }
        if (event.target.closest(TREE_ARROW_SELECTOR)) {
          const detailsElement = summaryElement.parentElement;
          if (detailsElement instanceof HTMLDetailsElement) {
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
          }
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
      void loadReaderPage(targetURL, false);
    });
  } catch {
    // no-op: history enhancement should never block rendering.
  }

  try {
    const readerMain = document.querySelector(MAIN_SELECTOR);
    if (readerMain instanceof HTMLElement) {
      readerMain.addEventListener("scroll", scheduleOutlineActiveSync, { passive: true });
    }
    window.addEventListener("resize", refreshOutlineRegistry, { passive: true });
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
