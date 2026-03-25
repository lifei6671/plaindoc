const MERMAID_BLOCK_SELECTOR = "[data-reader-hook='mermaid']";
const MERMAID_SOURCE_SELECTOR = "[data-reader-mermaid-source='1']";
const MERMAID_DIAGRAM_SELECTOR = "[data-reader-mermaid-diagram='1']";
const MERMAID_MESSAGE_SELECTOR = "[data-reader-mermaid-message='1']";
const MERMAID_FALLBACK_SELECTOR = "[data-reader-mermaid-fallback='1']";
const MERMAID_TEXT_LINE_HEIGHT = 16;
const MERMAID_TEXT_CHAR_WIDTH = 8;
const MERMAID_TEXT_PADDING = 12;

let mermaidModulePromise = null;
let mermaidRenderSequence = 0;
const READER_MERMAID_RUNTIME_GLOBAL_KEY = "__plaindocReaderMermaidRuntime__";

function readLengthAttribute(element, attributeName) {
  const rawValue = element.getAttribute(attributeName);
  const parsedValue = rawValue ? Number.parseFloat(rawValue) : Number.NaN;
  return Number.isFinite(parsedValue) ? parsedValue : 0;
}

function estimateTextWidth(rawText) {
  const normalizedText = rawText.replace(/\s+/g, " ").trim();
  return Math.max(
    MERMAID_TEXT_PADDING * 2,
    normalizedText.length * MERMAID_TEXT_CHAR_WIDTH + MERMAID_TEXT_PADDING * 2
  );
}

function mergeBoundingBoxes(boxes) {
  if (!boxes.length) {
    return { x: 0, y: 0, width: 0, height: 0 };
  }
  let minX = boxes[0].x;
  let minY = boxes[0].y;
  let maxX = boxes[0].x + boxes[0].width;
  let maxY = boxes[0].y + boxes[0].height;
  for (let index = 1; index < boxes.length; index += 1) {
    minX = Math.min(minX, boxes[index].x);
    minY = Math.min(minY, boxes[index].y);
    maxX = Math.max(maxX, boxes[index].x + boxes[index].width);
    maxY = Math.max(maxY, boxes[index].y + boxes[index].height);
  }
  return {
    x: minX,
    y: minY,
    width: Math.max(0, maxX - minX),
    height: Math.max(0, maxY - minY)
  };
}

function computeBoundingBox(element) {
  const tagName = element.tagName.toLowerCase();
  switch (tagName) {
    case "text":
    case "tspan":
      return {
        x: readLengthAttribute(element, "x"),
        y: readLengthAttribute(element, "y") - MERMAID_TEXT_LINE_HEIGHT * 0.75,
        width: estimateTextWidth(element.textContent ?? ""),
        height: MERMAID_TEXT_LINE_HEIGHT
      };
    case "rect":
    case "foreignobject":
      return {
        x: readLengthAttribute(element, "x"),
        y: readLengthAttribute(element, "y"),
        width: readLengthAttribute(element, "width"),
        height: readLengthAttribute(element, "height")
      };
    case "circle": {
      const radius = readLengthAttribute(element, "r");
      const cx = readLengthAttribute(element, "cx");
      const cy = readLengthAttribute(element, "cy");
      return {
        x: cx - radius,
        y: cy - radius,
        width: radius * 2,
        height: radius * 2
      };
    }
    case "ellipse": {
      const rx = readLengthAttribute(element, "rx");
      const ry = readLengthAttribute(element, "ry");
      const cx = readLengthAttribute(element, "cx");
      const cy = readLengthAttribute(element, "cy");
      return {
        x: cx - rx,
        y: cy - ry,
        width: rx * 2,
        height: ry * 2
      };
    }
    case "line": {
      const x1 = readLengthAttribute(element, "x1");
      const y1 = readLengthAttribute(element, "y1");
      const x2 = readLengthAttribute(element, "x2");
      const y2 = readLengthAttribute(element, "y2");
      return {
        x: Math.min(x1, x2),
        y: Math.min(y1, y2),
        width: Math.abs(x2 - x1),
        height: Math.abs(y2 - y1)
      };
    }
    default: {
      const childElements = Array.from(element.children).filter((child) => child instanceof Element);
      if (!childElements.length) {
        return {
          x: readLengthAttribute(element, "x"),
          y: readLengthAttribute(element, "y"),
          width: readLengthAttribute(element, "width"),
          height: readLengthAttribute(element, "height")
        };
      }
      return mergeBoundingBoxes(childElements.map((child) => computeBoundingBox(child)));
    }
  }
}

function ensureMermaidMeasurementPolyfills() {
  if (typeof SVGElement === "undefined") {
    return;
  }
  const prototype = SVGElement.prototype;
  if (typeof prototype.getBBox !== "function") {
    prototype.getBBox = function getBBox() {
      return computeBoundingBox(this);
    };
  }
  if (typeof prototype.getComputedTextLength !== "function") {
    prototype.getComputedTextLength = function getComputedTextLength() {
      return estimateTextWidth(this.textContent ?? "");
    };
  }
}

async function loadMermaidModule() {
  if (!mermaidModulePromise) {
    ensureMermaidMeasurementPolyfills();
    mermaidModulePromise = import("mermaid")
      .then((module) => {
        module.default.initialize({
          startOnLoad: false,
          securityLevel: "strict",
          suppressErrorRendering: true
        });
        return module.default;
      })
      .catch((error) => {
        mermaidModulePromise = null;
        throw error;
      });
  }
  return mermaidModulePromise;
}

function ensureMessageNode(blockNode) {
  const existingNode = blockNode.querySelector(MERMAID_MESSAGE_SELECTOR);
  if (existingNode instanceof HTMLParagraphElement) {
    return existingNode;
  }
  const messageNode = document.createElement("p");
  messageNode.setAttribute("data-reader-mermaid-message", "1");
  blockNode.appendChild(messageNode);
  return messageNode;
}

function ensureDiagramNode(blockNode) {
  const existingNode = blockNode.querySelector(MERMAID_DIAGRAM_SELECTOR);
  if (existingNode instanceof HTMLDivElement) {
    return existingNode;
  }
  const diagramNode = document.createElement("div");
  diagramNode.className = "mermaid-block__diagram";
  diagramNode.setAttribute("data-reader-mermaid-diagram", "1");
  blockNode.appendChild(diagramNode);
  return diagramNode;
}

function ensureFallbackNode(blockNode, source) {
  const existingNode = blockNode.querySelector(MERMAID_FALLBACK_SELECTOR);
  if (existingNode instanceof HTMLPreElement) {
    return existingNode;
  }
  const fallbackNode = document.createElement("pre");
  fallbackNode.className = "mermaid-block__fallback";
  fallbackNode.setAttribute("data-reader-mermaid-fallback", "1");
  const codeNode = document.createElement("code");
  codeNode.textContent = source;
  fallbackNode.appendChild(codeNode);
  blockNode.appendChild(fallbackNode);
  return fallbackNode;
}

function setMermaidLoadingState(blockNode) {
  blockNode.className = "mermaid-block mermaid-block--loading";
  blockNode.setAttribute("data-reader-mermaid-status", "loading");
  const messageNode = ensureMessageNode(blockNode);
  messageNode.className = "";
  messageNode.textContent = "Mermaid 图渲染中...";
  const diagramNode = blockNode.querySelector(MERMAID_DIAGRAM_SELECTOR);
  if (diagramNode instanceof HTMLElement) {
    diagramNode.remove();
  }
  const fallbackNode = blockNode.querySelector(MERMAID_FALLBACK_SELECTOR);
  if (fallbackNode instanceof HTMLElement) {
    fallbackNode.remove();
  }
}

function setMermaidReadyState(blockNode, svg) {
  blockNode.className = "mermaid-block";
  blockNode.setAttribute("data-reader-mermaid-status", "ready");
  const diagramNode = ensureDiagramNode(blockNode);
  diagramNode.innerHTML = svg;
  const messageNode = blockNode.querySelector(MERMAID_MESSAGE_SELECTOR);
  if (messageNode instanceof HTMLElement) {
    messageNode.remove();
  }
  const fallbackNode = blockNode.querySelector(MERMAID_FALLBACK_SELECTOR);
  if (fallbackNode instanceof HTMLElement) {
    fallbackNode.remove();
  }
}

function setMermaidErrorState(blockNode, source, message) {
  blockNode.className = "mermaid-block mermaid-block--error";
  blockNode.setAttribute("data-reader-mermaid-status", "error");
  const messageNode = ensureMessageNode(blockNode);
  messageNode.className = "mermaid-block__error-message";
  messageNode.textContent = message;
  const diagramNode = blockNode.querySelector(MERMAID_DIAGRAM_SELECTOR);
  if (diagramNode instanceof HTMLElement) {
    diagramNode.remove();
  }
  ensureFallbackNode(blockNode, source);
}

async function renderReaderMermaidBlock(blockNode) {
  const sourceNode = blockNode.querySelector(MERMAID_SOURCE_SELECTOR);
  const source = sourceNode instanceof HTMLElement ? (sourceNode.textContent ?? "").trim() : "";
  if (!source) {
    setMermaidErrorState(blockNode, "", "Mermaid 代码块为空，无法渲染图表。");
    return;
  }

  setMermaidLoadingState(blockNode);

  try {
    const mermaid = await loadMermaidModule();
    mermaidRenderSequence += 1;
    const renderResult = await mermaid.render(`plaindoc-reader-mermaid-${mermaidRenderSequence}`, source);
    setMermaidReadyState(blockNode, renderResult.svg);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    setMermaidErrorState(blockNode, source, `Mermaid 渲染失败：${message}`);
  }
}

export async function renderReaderMermaidBlocks(root = document) {
  const blockNodes = Array.from(root.querySelectorAll(MERMAID_BLOCK_SELECTOR)).filter(
    (node) => node instanceof HTMLElement
  );
  for (const blockNode of blockNodes) {
    if ((blockNode.getAttribute("data-reader-mermaid-status") || "").trim() === "ready") {
      continue;
    }
    await renderReaderMermaidBlock(blockNode);
  }
}

if (typeof window !== "undefined") {
  window[READER_MERMAID_RUNTIME_GLOBAL_KEY] = {
    renderReaderMermaidBlocks
  };
}
