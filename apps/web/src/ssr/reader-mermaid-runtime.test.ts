import { describe, expect, it } from "vitest";
import { renderReaderMermaidBlocks } from "./reader-mermaid-runtime.js";

describe("renderReaderMermaidBlocks", () => {
  it("replaces loading mermaid placeholders with rendered svg", async () => {
    document.body.innerHTML = `
      <article id="plaindoc-preview-body">
        <div
          class="mermaid-block mermaid-block--loading"
          data-reader-hook="mermaid"
          data-reader-mermaid-status="loading"
        >
          <pre hidden data-reader-mermaid-source="1"><code>graph TD
A-->B</code></pre>
          <p data-reader-mermaid-message="1">Mermaid 图渲染中...</p>
        </div>
      </article>
    `;

    await renderReaderMermaidBlocks(document);

    const blockNode = document.querySelector("[data-reader-hook='mermaid']");
    expect(blockNode).toBeInstanceOf(HTMLElement);
    expect(blockNode?.getAttribute("data-reader-mermaid-status")).toBe("ready");
    expect(blockNode?.className).toBe("mermaid-block");
    expect(blockNode?.querySelector("[data-reader-mermaid-diagram='1'] svg")).not.toBeNull();
    expect(blockNode?.textContent).not.toContain("Mermaid 图渲染中...");
  });
});
