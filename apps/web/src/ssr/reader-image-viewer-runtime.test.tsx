import { render, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ReaderImageViewerShell } from "./ReaderImageViewerShell";
import {
  ensureReaderImageViewer,
  refreshReaderImageViewer
} from "./reader-image-viewer-runtime.js";

function mockStageRect(initialWidth = 1200, initialHeight = 800) {
  const stageNode = document.querySelector("[data-reader-hook='image-viewer-stage']");
  if (!(stageNode instanceof HTMLElement)) {
    throw new Error("image viewer stage not found");
  }
  const rect = { width: initialWidth, height: initialHeight };
  stageNode.getBoundingClientRect = () =>
    ({
      width: rect.width,
      height: rect.height,
      top: 0,
      left: 0,
      right: rect.width,
      bottom: rect.height,
      x: 0,
      y: 0,
      toJSON: () => ({})
    }) as DOMRect;
  return {
    setSize(width: number, height: number) {
      rect.width = width;
      rect.height = height;
    }
  };
}

describe("reader-image-viewer-runtime", () => {
  afterEach(() => {
    document.head.innerHTML = "";
    document.body.innerHTML = "";
    document.body.className = "";
    vi.restoreAllMocks();
  });

  it("opens image viewer for img and supports toolbar actions", async () => {
    render(
      <>
        <ReaderImageViewerShell />
        <article id="plaindoc-preview-body">
          <p>
            <img src="https://example.com/a.png" alt="示例图片 A" width="640" height="480" />
          </p>
          <p>
            <img src="https://example.com/b.png" alt="示例图片 B" width="320" height="200" />
          </p>
        </article>
      </>
    );
    const stageRect = mockStageRect();

    ensureReaderImageViewer(document);

    const firstImage = document.querySelector("img[alt='示例图片 A']");
    expect(firstImage).toBeInstanceOf(HTMLImageElement);
    expect(document.querySelector("[data-reader-hook='image-viewer']")?.hasAttribute("hidden")).toBe(true);
    (firstImage as HTMLImageElement).click();

    const viewerRoot = document.querySelector("[data-reader-hook='image-viewer']");
    expect(viewerRoot).toBeInstanceOf(HTMLElement);
    expect(viewerRoot).not.toHaveAttribute("hidden");
    expect(document.body.classList.contains("reader-image-viewer-open")).toBe(true);
    expect(document.querySelector("[data-reader-hook='image-viewer-index']")?.textContent).toBe("1/2");
    expect(document.querySelector("[data-reader-hook='image-viewer-scale']")?.textContent).toBe("100%");

    const zoomInButton = document.querySelector("[data-reader-hook='image-viewer-zoom-in']");
    expect(zoomInButton).toBeInstanceOf(HTMLButtonElement);
    (zoomInButton as HTMLButtonElement).click();
    expect(document.querySelector("[data-reader-hook='image-viewer-scale']")?.textContent).toBe("120%");

    stageRect.setSize(500, 400);
    window.dispatchEvent(new Event("resize"));
    expect(document.querySelector("[data-reader-hook='image-viewer-scale']")?.textContent).toBe("85%");

    const rotateButton = document.querySelector("[data-reader-hook='image-viewer-rotate']");
    expect(rotateButton).toBeInstanceOf(HTMLButtonElement);
    (rotateButton as HTMLButtonElement).click();
    const frameNode = document.querySelector(".reader-image-viewer__media-frame");
    expect(frameNode).toBeInstanceOf(HTMLElement);
    expect((frameNode as HTMLElement).style.transform).toContain("rotate(90deg)");
    (rotateButton as HTMLButtonElement).click();
    (rotateButton as HTMLButtonElement).click();
    (rotateButton as HTMLButtonElement).click();
    expect((frameNode as HTMLElement).style.transform).toContain("rotate(360deg)");

    const nextButton = document.querySelector("[data-reader-hook='image-viewer-next']");
    expect(nextButton).toBeInstanceOf(HTMLButtonElement);
    (nextButton as HTMLButtonElement).click();
    expect(document.querySelector("[data-reader-hook='image-viewer-index']")?.textContent).toBe("2/2");
    expect(document.querySelector("[data-reader-hook='image-viewer-scale']")?.textContent).toBe("100%");

    const closeButton = document.querySelector("[data-reader-hook='image-viewer-close']");
    expect(closeButton).toBeInstanceOf(HTMLButtonElement);
    (closeButton as HTMLButtonElement).click();
    expect((viewerRoot as HTMLElement).hidden).toBe(true);
    expect(document.body.classList.contains("reader-image-viewer-open")).toBe(false);
  });

  it("closes viewer when clicking outside the media frame", async () => {
    render(
      <>
        <ReaderImageViewerShell />
        <article id="plaindoc-preview-body">
          <p>
            <img src="https://example.com/c.png" alt="示例图片 C" width="640" height="480" />
          </p>
        </article>
      </>
    );
    mockStageRect();

    ensureReaderImageViewer(document);

    const imageNode = document.querySelector("img[alt='示例图片 C']");
    expect(imageNode).toBeInstanceOf(HTMLImageElement);
    imageNode?.dispatchEvent(new MouseEvent("click", { bubbles: true }));

    const stageNode = document.querySelector("[data-reader-hook='image-viewer-stage']");
    expect(stageNode).toBeInstanceOf(HTMLElement);
    stageNode?.dispatchEvent(new MouseEvent("click", { bubbles: true }));

    expect(document.querySelector("[data-reader-hook='image-viewer']")?.getAttribute("aria-hidden")).toBe("true");
    expect(document.body.classList.contains("reader-image-viewer-open")).toBe(false);
  });

  it("re-resolves image dimensions on open instead of keeping fallback size", async () => {
    render(
      <>
        <ReaderImageViewerShell />
        <article id="plaindoc-preview-body">
          <p>
            <img src="https://example.com/tall.png" alt="长图" />
          </p>
        </article>
      </>
    );
    mockStageRect();

    const imageNode = document.querySelector("img[alt='长图']");
    expect(imageNode).toBeInstanceOf(HTMLImageElement);
    if (!(imageNode instanceof HTMLImageElement)) {
      throw new Error("image node not found");
    }

    Object.defineProperty(imageNode, "naturalWidth", { configurable: true, get: () => 0 });
    Object.defineProperty(imageNode, "naturalHeight", { configurable: true, get: () => 0 });
    imageNode.getBoundingClientRect = () =>
      ({
        width: 0,
        height: 0,
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        x: 0,
        y: 0,
        toJSON: () => ({})
      }) as DOMRect;

    ensureReaderImageViewer(document);

    Object.defineProperty(imageNode, "naturalWidth", { configurable: true, get: () => 125 });
    Object.defineProperty(imageNode, "naturalHeight", { configurable: true, get: () => 459 });
    imageNode.click();

    const frameNode = document.querySelector(".reader-image-viewer__media-frame");
    const viewerImage = document.querySelector(".reader-image-viewer__media--img");
    expect(frameNode).toBeInstanceOf(HTMLElement);
    expect(viewerImage).toBeInstanceOf(HTMLImageElement);
    expect((viewerImage as HTMLImageElement).style.width).toBe("125px");
    expect((viewerImage as HTMLImageElement).style.height).toBe("459px");
    expect((viewerImage as HTMLImageElement).style.width).not.toBe("320px");
    expect((viewerImage as HTMLImageElement).style.height).not.toBe("240px");
  });

  it("supports dragging media with mouse without closing viewer", async () => {
    render(
      <>
        <ReaderImageViewerShell />
        <article id="plaindoc-preview-body">
          <p>
            <img src="https://example.com/d.png" alt="示例图片 D" width="1280" height="960" />
          </p>
        </article>
      </>
    );
    mockStageRect();

    ensureReaderImageViewer(document);

    const imageNode = document.querySelector("img[alt='示例图片 D']");
    expect(imageNode).toBeInstanceOf(HTMLImageElement);
    imageNode?.dispatchEvent(new MouseEvent("click", { bubbles: true }));

    const stageNode = document.querySelector("[data-reader-hook='image-viewer-stage']");
    const frameNode = document.querySelector(".reader-image-viewer__media-frame");
    const zoomInButton = document.querySelector("[data-reader-hook='image-viewer-zoom-in']");
    expect(stageNode).toBeInstanceOf(HTMLElement);
    expect(frameNode).toBeInstanceOf(HTMLElement);
    expect(zoomInButton).toBeInstanceOf(HTMLButtonElement);

    if (
      !(stageNode instanceof HTMLElement) ||
      !(frameNode instanceof HTMLElement) ||
      !(zoomInButton instanceof HTMLButtonElement)
    ) {
      throw new Error("viewer stage or frame or zoom in button not found");
    }

    zoomInButton.click();

    frameNode.dispatchEvent(new MouseEvent("mousedown", { bubbles: true, button: 0, clientX: 140, clientY: 120 }));
    window.dispatchEvent(new MouseEvent("mousemove", { bubbles: true, clientX: 210, clientY: 170 }));
    window.dispatchEvent(new MouseEvent("mouseup", { bubbles: true, clientX: 210, clientY: 170 }));

    expect(frameNode.style.transform).toContain("translate(70px, 50px)");
    expect(document.querySelector("[data-reader-hook='image-viewer']")?.getAttribute("aria-hidden")).toBe("false");
    expect(document.querySelector("[data-reader-hook='image-viewer']")?.classList.contains("reader-image-viewer--pannable")).toBe(true);
  });

  it("zooms around pointer with mouse wheel", async () => {
    render(
      <>
        <ReaderImageViewerShell />
        <article id="plaindoc-preview-body">
          <p>
            <img src="https://example.com/e.png" alt="示例图片 E" width="1280" height="960" />
          </p>
        </article>
      </>
    );
    mockStageRect(500, 400);

    ensureReaderImageViewer(document);

    const imageNode = document.querySelector("img[alt='示例图片 E']");
    expect(imageNode).toBeInstanceOf(HTMLImageElement);
    imageNode?.dispatchEvent(new MouseEvent("click", { bubbles: true }));

    const stageNode = document.querySelector("[data-reader-hook='image-viewer-stage']");
    const frameNode = document.querySelector(".reader-image-viewer__media-frame");
    expect(stageNode).toBeInstanceOf(HTMLElement);
    expect(frameNode).toBeInstanceOf(HTMLElement);
    expect(document.querySelector("[data-reader-hook='image-viewer-scale']")?.textContent).toBe("35%");

    stageNode?.dispatchEvent(
      new WheelEvent("wheel", { bubbles: true, cancelable: true, deltaY: -100, clientX: 250, clientY: 200 })
    );

    expect(document.querySelector("[data-reader-hook='image-viewer-scale']")?.textContent).toBe("42%");
    expect((frameNode as HTMLElement).style.transform).toContain("scale(0.42375)");
  });

  it("toggles between fit scale and original size on double click", async () => {
    render(
      <>
        <ReaderImageViewerShell />
        <article id="plaindoc-preview-body">
          <p>
            <img src="https://example.com/f.png" alt="示例图片 F" width="1280" height="960" />
          </p>
        </article>
      </>
    );
    mockStageRect(500, 400);

    ensureReaderImageViewer(document);

    const imageNode = document.querySelector("img[alt='示例图片 F']");
    expect(imageNode).toBeInstanceOf(HTMLImageElement);
    imageNode?.dispatchEvent(new MouseEvent("click", { bubbles: true }));

    const frameNode = document.querySelector(".reader-image-viewer__media-frame");
    expect(frameNode).toBeInstanceOf(HTMLElement);
    expect(document.querySelector("[data-reader-hook='image-viewer-scale']")?.textContent).toBe("35%");

    frameNode?.dispatchEvent(new MouseEvent("dblclick", { bubbles: true, clientX: 250, clientY: 200 }));
    expect(document.querySelector("[data-reader-hook='image-viewer-scale']")?.textContent).toBe("100%");

    frameNode?.dispatchEvent(new MouseEvent("dblclick", { bubbles: true, clientX: 250, clientY: 200 }));
    expect(document.querySelector("[data-reader-hook='image-viewer-scale']")?.textContent).toBe("35%");
  });

  it("does not intercept linked media clicks", async () => {
    render(
      <>
        <ReaderImageViewerShell />
        <article id="plaindoc-preview-body">
          <p>
            <a href="/linked-target">
              <img src="https://example.com/linked.png" alt="带链接图片" width="640" height="480" />
            </a>
          </p>
        </article>
      </>
    );
    mockStageRect();

    ensureReaderImageViewer(document);

    const anchorNode = document.querySelector("a[href='/linked-target']");
    const imageNode = document.querySelector("img[alt='带链接图片']");
    expect(anchorNode).toBeInstanceOf(HTMLAnchorElement);
    expect(imageNode).toBeInstanceOf(HTMLImageElement);

    const anchorClickSpy = vi.fn();
    anchorNode?.addEventListener("click", anchorClickSpy);

    const clickEvent = new MouseEvent("click", { bubbles: true, cancelable: true });
    imageNode?.dispatchEvent(clickEvent);

    expect(anchorClickSpy).toHaveBeenCalledTimes(1);
    expect(clickEvent.defaultPrevented).toBe(false);
    expect(document.querySelector("[data-reader-hook='image-viewer']")?.hasAttribute("hidden")).toBe(true);
  });

  it("collects mermaid svg and refreshes registry after content replacement", async () => {
    render(
      <>
        <ReaderImageViewerShell />
        <article id="plaindoc-preview-body">
          <div className="mermaid-block" data-reader-hook="mermaid" data-reader-mermaid-status="ready">
            <div data-reader-mermaid-diagram="1">
              <svg viewBox="0 0 200 100">
                <rect x="10" y="10" width="180" height="80" />
              </svg>
            </div>
          </div>
        </article>
      </>
    );
    mockStageRect();

    const initialItems = ensureReaderImageViewer(document);
    expect(initialItems).toHaveLength(1);

    const mermaidSVG = document.querySelector("[data-reader-mermaid-diagram='1'] svg");
    expect(mermaidSVG).toBeInstanceOf(SVGSVGElement);
    mermaidSVG?.dispatchEvent(new MouseEvent("click", { bubbles: true }));

    await waitFor(() => {
      expect(document.querySelector("[data-reader-hook='image-viewer-index']")?.textContent).toBe("1/1");
    });

    const previewBody = document.getElementById("plaindoc-preview-body");
    if (!(previewBody instanceof HTMLElement)) {
      throw new Error("preview body not found");
    }
    previewBody.innerHTML = `
      <p><img src="https://example.com/refresh.png" alt="刷新后图片" width="400" height="240" /></p>
    `;

    const refreshedItems = refreshReaderImageViewer(document);
    expect(refreshedItems).toHaveLength(1);
    expect(document.querySelector("[data-reader-hook='image-viewer']")?.getAttribute("aria-hidden")).toBe("true");

    const refreshedImage = document.querySelector("img[alt='刷新后图片']");
    expect(refreshedImage).toBeInstanceOf(HTMLImageElement);
    refreshedImage?.dispatchEvent(new MouseEvent("click", { bubbles: true }));

    await waitFor(() => {
      expect(document.querySelector("[data-reader-hook='image-viewer-index']")?.textContent).toBe("1/1");
    });
  });

  it("keeps viewer open when gallery refresh preserves the active media", async () => {
    render(
      <>
        <ReaderImageViewerShell />
        <article id="plaindoc-preview-body">
          <p>
            <img src="https://example.com/persist.png" alt="常驻图片" width="640" height="480" />
          </p>
          <div className="mermaid-block" data-reader-hook="mermaid" data-reader-mermaid-status="loading">
            <pre hidden data-reader-mermaid-source="1">
              graph TD;A--&gt;B;
            </pre>
          </div>
        </article>
      </>
    );
    mockStageRect();

    ensureReaderImageViewer(document);

    const imageNode = document.querySelector("img[alt='常驻图片']");
    expect(imageNode).toBeInstanceOf(HTMLImageElement);
    imageNode?.dispatchEvent(new MouseEvent("click", { bubbles: true }));

    await waitFor(() => {
      expect(document.querySelector("[data-reader-hook='image-viewer']")?.getAttribute("aria-hidden")).toBe("false");
      expect(document.querySelector("[data-reader-hook='image-viewer-index']")?.textContent).toBe("1/1");
    });

    const mermaidNode = document.querySelector("[data-reader-hook='mermaid']");
    if (!(mermaidNode instanceof HTMLElement)) {
      throw new Error("mermaid node not found");
    }
    mermaidNode.setAttribute("data-reader-mermaid-status", "ready");
    mermaidNode.innerHTML = `
      <div data-reader-mermaid-diagram="1">
        <svg viewBox="0 0 200 100">
          <rect x="10" y="10" width="180" height="80" />
        </svg>
      </div>
    `;

    const refreshedItems = refreshReaderImageViewer(document);
    expect(refreshedItems).toHaveLength(2);
    expect(document.querySelector("[data-reader-hook='image-viewer']")?.getAttribute("aria-hidden")).toBe("false");
    expect(document.querySelector("[data-reader-hook='image-viewer-index']")?.textContent).toBe("1/2");
  });
});
