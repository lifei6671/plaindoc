import { waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { READER_ASYNC_ENHANCEMENT_SCRIPT } from "./render-space-reader.async-script";

describe("READER_ASYNC_ENHANCEMENT_SCRIPT", () => {
  afterEach(() => {
    document.head.innerHTML = "";
    document.body.innerHTML = "";
    vi.restoreAllMocks();
  });

  it("keeps code copy working for article content inserted after initialization", async () => {
    document.body.innerHTML = `
      <main data-reader-hook="main">
        <article class="reader-article-shell" data-reader-hook="article-shell"></article>
      </main>
      <script id="plaindoc-reader-state" type="application/json">{}</script>
    `;

    const clipboardWriteText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(window.navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: clipboardWriteText
      }
    });

    window.eval(READER_ASYNC_ENHANCEMENT_SCRIPT);

    const currentArticleShell = document.querySelector("[data-reader-hook='article-shell']");
    const nextArticleShell = document.createElement("article");
    nextArticleShell.className = "reader-article-shell";
    nextArticleShell.setAttribute("data-reader-hook", "article-shell");
    nextArticleShell.innerHTML = `
      <div class="code-block-copy-shell">
        <button
          type="button"
          class="code-block-copy-button"
          data-code-copy-button="1"
          data-copy-state="idle"
          aria-label="复制代码"
        >
          <span class="code-block-copy-button__label code-block-copy-button__label--idle">复制</span>
          <span class="code-block-copy-button__label code-block-copy-button__label--success">复制成功</span>
        </button>
        <pre class="code-block-copy-shell__surface"><code data-code-copy-source="1">const answer = 42;</code></pre>
      </div>
    `;
    currentArticleShell?.replaceWith(nextArticleShell);

    const copyButton = document.querySelector("button[data-code-copy-button='1']");
    expect(copyButton).toBeInstanceOf(HTMLButtonElement);

    (copyButton as HTMLButtonElement).click();

    await waitFor(() => {
      expect(clipboardWriteText).toHaveBeenCalledWith("const answer = 42;");
    });
    expect(copyButton).toHaveAttribute("data-copy-state", "success");
    expect(copyButton).toHaveAttribute("aria-label", "复制成功");
  });

  it("initializes reader image viewer runtime for existing reader images", async () => {
    document.body.innerHTML = `
      <main data-reader-hook="main">
        <article class="reader-article-shell" data-reader-hook="article-shell">
          <article id="plaindoc-preview-body">
            <p><img src="https://example.com/demo.png" alt="示例图片" width="400" height="240" /></p>
          </article>
        </article>
      </main>
      <div data-reader-hook="image-viewer" aria-hidden="true" hidden>
        <button data-reader-hook="image-viewer-backdrop" aria-label="关闭图片浏览器"></button>
        <div data-reader-hook="image-viewer-stage">
          <div data-reader-hook="image-viewer-content"></div>
        </div>
        <button data-reader-hook="image-viewer-close" aria-label="关闭图片浏览器"></button>
        <button data-reader-hook="image-viewer-prev" aria-label="上一张"></button>
        <button data-reader-hook="image-viewer-next" aria-label="下一张"></button>
        <button data-reader-hook="image-viewer-zoom-out" aria-label="缩小"></button>
        <button data-reader-hook="image-viewer-zoom-in" aria-label="放大"></button>
        <button data-reader-hook="image-viewer-original" aria-label="原始尺寸"></button>
        <button data-reader-hook="image-viewer-rotate" aria-label="旋转"></button>
        <span data-reader-hook="image-viewer-index">0/0</span>
        <span data-reader-hook="image-viewer-scale">100%</span>
      </div>
      <script id="plaindoc-reader-state" type="application/json">{}</script>
    `;

    const ensureReaderImageViewer = vi.fn();
    (window as Window & { __plaindocReaderImageViewerRuntime__?: unknown }).__plaindocReaderImageViewerRuntime__ =
      {
        ensureReaderImageViewer,
        refreshReaderImageViewer: vi.fn()
      };

    window.eval(READER_ASYNC_ENHANCEMENT_SCRIPT);

    await waitFor(() => {
      expect(ensureReaderImageViewer).toHaveBeenCalledWith(document);
    });
  });
});
