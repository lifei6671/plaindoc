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
});
