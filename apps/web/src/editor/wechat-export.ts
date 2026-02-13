import juice from "juice";
import { PREVIEW_BODY_ID } from "./constants";
import type { PreviewLinkRenderMode } from "./types";

// 微信复制选项：允许调用方覆盖预览容器定位，便于后续复用。
interface CopyPreviewToWechatOptions {
  previewPaneId?: string;
  previewBodyClass?: string;
  linkRenderMode?: PreviewLinkRenderMode;
}

// 复制结果：返回最终 HTML 与纯文本，便于调试或扩展埋点。
interface WechatClipboardPayload {
  html: string;
  plainText: string;
}

interface WechatExternalFootnote {
  index: number;
  href: string;
  title: string | null;
}

interface WechatFootnoteStyleTokens {
  referenceColor: string | null;
  wordColor: string | null;
}

const LEGACY_COPY_INPUT_ID = "plaindoc-wechat-copy-input";

// 从 HTML 生成纯文本版本，剪贴板里同时放 text/plain 便于兜底粘贴。
function toPlainTextFromHtml(html: string): string {
  const container = document.createElement("div");
  container.innerHTML = html;
  return container.textContent?.trim() ?? "";
}

// 判定是否为微信环境不可用的外链：仅处理 http/https/mailto/tel 与协议相对地址。
function isExternalLinkHref(rawHref: string): boolean {
  const href = rawHref.trim();
  if (!href || href.startsWith("#")) {
    return false;
  }
  if (href.startsWith("//")) {
    return true;
  }
  return /^(https?:|mailto:|tel:)/i.test(href);
}

// 去掉预览阶段生成的“参考链接”区块，避免导出时与微信外链脚注重复。
function removeReferenceFootnoteSection(rootElement: HTMLElement): void {
  const referenceTitles = rootElement.querySelectorAll<HTMLElement>(".reference-footnote-title");
  referenceTitles.forEach((titleElement) => {
    const previousElement = titleElement.previousElementSibling;
    if (previousElement?.tagName.toLowerCase() === "hr") {
      previousElement.remove();
    }

    let sibling = titleElement.nextElementSibling;
    while (sibling instanceof HTMLElement && sibling.classList.contains("reference-footnote-line")) {
      const nextSibling = sibling.nextElementSibling;
      sibling.remove();
      sibling = nextSibling;
    }
    titleElement.remove();
  });
}

// 从预览区采样指定类名的计算颜色，作为微信导出时的稳定内联颜色来源。
function resolvePreviewClassColor(sourceRoot: HTMLElement, className: string): string | null {
  const probeElement = document.createElement("span");
  probeElement.className = className;
  probeElement.textContent = "1";
  probeElement.style.position = "absolute";
  probeElement.style.left = "-9999px";
  probeElement.style.top = "0";
  probeElement.style.pointerEvents = "none";
  sourceRoot.appendChild(probeElement);
  const color = window.getComputedStyle(probeElement).color.trim();
  probeElement.remove();
  if (!color || color === "rgba(0, 0, 0, 0)") {
    return null;
  }
  return color;
}

// 汇总脚注相关样式令牌：确保编号与链接文本在微信侧保持原主题颜色。
function resolveWechatFootnoteStyleTokens(sourceRoot: HTMLElement): WechatFootnoteStyleTokens {
  return {
    referenceColor: resolvePreviewClassColor(sourceRoot, "footnote-ref"),
    wordColor: resolvePreviewClassColor(sourceRoot, "footnote-word")
  };
}

// 将正文中的外链替换为纯文本 + 上标编号，并返回按出现顺序收集的脚注定义。
function convertExternalLinksToFootnotes(
  rootElement: HTMLElement,
  styleTokens: WechatFootnoteStyleTokens
): WechatExternalFootnote[] {
  const footnotes: WechatExternalFootnote[] = [];
  const footnoteIndexByHref = new Map<string, number>();
  const anchorElements = Array.from(rootElement.querySelectorAll<HTMLAnchorElement>("a[href]"));

  for (const anchorElement of anchorElements) {
    const rawHref = anchorElement.getAttribute("href");
    if (!rawHref || !isExternalLinkHref(rawHref)) {
      continue;
    }

    const normalizedHref = rawHref.trim();
    const titleText = anchorElement.getAttribute("title")?.trim() || null;
    let footnoteIndex = footnoteIndexByHref.get(normalizedHref);
    if (!footnoteIndex) {
      footnoteIndex = footnotes.length + 1;
      footnoteIndexByHref.set(normalizedHref, footnoteIndex);
      footnotes.push({
        index: footnoteIndex,
        href: normalizedHref,
        title: titleText
      });
    } else if (titleText) {
      const existingFootnote = footnotes[footnoteIndex - 1];
      if (existingFootnote && !existingFootnote.title) {
        existingFootnote.title = titleText;
      }
    }

    const replacementFragment = document.createDocumentFragment();
    if (anchorElement.childNodes.length) {
      while (anchorElement.firstChild) {
        replacementFragment.appendChild(anchorElement.firstChild);
      }
    } else {
      replacementFragment.appendChild(document.createTextNode(normalizedHref));
    }

    const referenceElement = document.createElement("sup");
    referenceElement.className = "footnote-ref wechat-link-footnote-ref";
    referenceElement.textContent = `[${footnoteIndex}]`;
    // 微信对 `<sup>` 的保留不稳定，导出前先写入关键内联样式兜底。
    referenceElement.style.verticalAlign = "super";
    referenceElement.style.fontSize = "0.78em";
    referenceElement.style.lineHeight = "1";
    referenceElement.style.marginLeft = "2px";
    if (styleTokens.referenceColor) {
      referenceElement.style.color = styleTokens.referenceColor;
    }
    replacementFragment.appendChild(referenceElement);
    anchorElement.replaceWith(replacementFragment);
  }

  return footnotes;
}

// 在文末追加“外链脚注”区块，仅保留纯文本 URL，确保公众号粘贴后不出现可点击外链。
function appendWechatExternalFootnotes(
  rootElement: HTMLElement,
  footnotes: WechatExternalFootnote[],
  styleTokens: WechatFootnoteStyleTokens
): void {
  if (!footnotes.length) {
    return;
  }

  const dividerElement = document.createElement("hr");
  dividerElement.className = "wechat-link-footnotes-divider";
  rootElement.appendChild(dividerElement);

  const titleElement = document.createElement("p");
  titleElement.className = "wechat-link-footnotes-title";
  titleElement.textContent = "外链脚注";
  rootElement.appendChild(titleElement);

  footnotes.forEach((footnote) => {
    const lineElement = document.createElement("p");
    lineElement.className = "wechat-link-footnotes-item";

    const indexElement = document.createElement("span");
    indexElement.className = "footnote-ref wechat-link-footnotes-item-ref";
    indexElement.textContent = `[${footnote.index}]`;
    if (styleTokens.referenceColor) {
      indexElement.style.color = styleTokens.referenceColor;
    }

    const hrefElement = document.createElement("span");
    hrefElement.className = "footnote-word wechat-link-footnotes-item-url";
    hrefElement.textContent = footnote.href;
    if (styleTokens.wordColor) {
      hrefElement.style.color = styleTokens.wordColor;
    }

    lineElement.appendChild(indexElement);
    lineElement.appendChild(document.createTextNode(" "));
    lineElement.appendChild(hrefElement);

    if (footnote.title) {
      const titleTextElement = document.createElement("span");
      titleTextElement.className = "wechat-link-footnotes-item-title";
      titleTextElement.textContent = ` "${footnote.title}"`;
      lineElement.appendChild(titleTextElement);
    }

    rootElement.appendChild(lineElement);
  });
}

// 公众号导出专用：把外链统一转为无链接脚注，避免微信环境丢失跳转行为。
function normalizeExternalLinksForWechatExport(
  rootElement: HTMLElement,
  styleTokens: WechatFootnoteStyleTokens
): void {
  removeReferenceFootnoteSection(rootElement);
  const footnotes = convertExternalLinksToFootnotes(rootElement, styleTokens);
  appendWechatExternalFootnotes(rootElement, footnotes, styleTokens);
}

const WECHAT_CRITICAL_STYLE_PROPS = [
  "display",
  "color",
  "background",
  "background-color",
  "font-family",
  "font-size",
  "font-weight",
  "font-style",
  "line-height",
  "letter-spacing",
  "word-spacing",
  "text-align",
  "text-indent",
  "text-decoration",
  "text-transform",
  "white-space",
  "vertical-align",
  "padding",
  "padding-top",
  "padding-right",
  "padding-bottom",
  "padding-left",
  "margin",
  "margin-top",
  "margin-right",
  "margin-bottom",
  "margin-left",
  "border",
  "border-top",
  "border-right",
  "border-bottom",
  "border-left",
  "border-radius"
];

// 微信导出关键选择器清单：按“标签/类”分类维护，便于和预览区样式同步。
// 约束：预览区新增关键样式时，必须同步补充到这里，避免复制到微信后丢失样式。
const WECHAT_CRITICAL_SELECTORS_BY_TYPE = {
  tag: ["h1", "h2", "h3", "h4", "h5", "h6", "mark", "strong"],
  class: [
    ".prefix",
    ".content",
    ".suffix",
    ".footnote-word",
    ".footnote-ref",
    ".reference-footnote-title",
    ".reference-footnote-line",
    ".reference-footnote-note"
  ]
} as const;

// 把 source 对应节点的关键计算样式写到 clone，作为 juice 失败或微信过滤时的兜底。
function inlineCriticalStylesBySelector(
  sourceRoot: HTMLElement,
  clonedRoot: HTMLElement,
  selector: string
): void {
  const sourceNodes = sourceRoot.querySelectorAll<HTMLElement>(selector);
  const clonedNodes = clonedRoot.querySelectorAll<HTMLElement>(selector);
  const nodeCount = Math.min(sourceNodes.length, clonedNodes.length);
  for (let index = 0; index < nodeCount; index += 1) {
    const sourceNode = sourceNodes[index];
    const clonedNode = clonedNodes[index];
    const computedStyle = window.getComputedStyle(sourceNode);
    const styleEntries: string[] = [];
    WECHAT_CRITICAL_STYLE_PROPS.forEach((propertyName) => {
      const propertyValue = computedStyle.getPropertyValue(propertyName).trim();
      if (propertyValue) {
        styleEntries.push(`${propertyName}:${propertyValue}`);
      }
    });
    if (!styleEntries.length) {
      continue;
    }
    const existingStyle = clonedNode.getAttribute("style");
    const mergedStyle = existingStyle ? `${existingStyle};${styleEntries.join(";")}` : styleEntries.join(";");
    clonedNode.setAttribute("style", mergedStyle);
  }
}

// 微信导出重点兜底：标题、标题装饰段、mark 标注、脚注引用都强制写入内联样式。
function inlineWechatCriticalStyles(sourceRoot: HTMLElement, clonedRoot: HTMLElement): void {
  Object.values(WECHAT_CRITICAL_SELECTORS_BY_TYPE).forEach((selectorGroup) => {
    const mergedSelector = selectorGroup.join(", ");
    if (!mergedSelector) {
      return;
    }
    inlineCriticalStylesBySelector(sourceRoot, clonedRoot, mergedSelector);
  });
}

// 替换标签名并保留属性与子节点内容，用于微信不稳定标签的导出阶段兼容。
function replaceElementTag(
  originalElement: HTMLElement,
  targetTagName: "p" | "span"
): HTMLElement {
  const replacementElement = document.createElement(targetTagName);
  Array.from(originalElement.attributes).forEach((attribute) => {
    replacementElement.setAttribute(attribute.name, attribute.value);
  });
  while (originalElement.firstChild) {
    replacementElement.appendChild(originalElement.firstChild);
  }
  originalElement.replaceWith(replacementElement);
  return replacementElement;
}

// 公众号粘贴兼容：把 h1~h6 转 p，把 mark 转 span，降低微信编辑器二次清洗丢样式概率。
function normalizeWechatHtmlCompatibility(html: string): string {
  const wrapperElement = document.createElement("div");
  wrapperElement.innerHTML = html;

  const rootElement = wrapperElement.firstElementChild;
  if (!(rootElement instanceof HTMLElement)) {
    return html;
  }

  const headingElements = Array.from(rootElement.querySelectorAll<HTMLElement>("h1, h2, h3, h4, h5, h6"));
  headingElements.forEach((headingElement) => {
    const headingTagName = headingElement.tagName.toLowerCase();
    const paragraphElement = replaceElementTag(headingElement, "p");
    paragraphElement.classList.add("wechat-heading", `wechat-heading-${headingTagName}`);
  });

  const markElements = Array.from(rootElement.querySelectorAll<HTMLElement>("mark"));
  markElements.forEach((markElement) => {
    const spanElement = replaceElementTag(markElement, "span");
    spanElement.classList.add("wechat-mark");
    // 如果原样式缺少背景色，给一个保守兜底，确保“标注”语义可见。
    if (!spanElement.style.backgroundColor) {
      spanElement.style.backgroundColor = "#fff3a3";
    }
    if (!spanElement.style.padding) {
      spanElement.style.padding = "0 2px";
    }
  });

  // 微信端对 strong 的样式保留不稳定，导出时改为 span 并显式保留加粗与颜色。
  const strongElements = Array.from(rootElement.querySelectorAll<HTMLElement>("strong"));
  strongElements.forEach((strongElement) => {
    const spanElement = replaceElementTag(strongElement, "span");
    spanElement.classList.add("wechat-strong");
    if (!spanElement.style.fontWeight) {
      spanElement.style.fontWeight = "700";
    }
  });

  // 正文脚注编号统一转为 span + super，对齐“右上角数字”且减少微信对 sup 的清洗影响。
  const inlineFootnoteReferenceElements = Array.from(
    rootElement.querySelectorAll<HTMLElement>("sup.wechat-link-footnote-ref, sup.footnote-ref")
  );
  inlineFootnoteReferenceElements.forEach((referenceElement) => {
    const spanElement = replaceElementTag(referenceElement, "span");
    spanElement.classList.add("wechat-inline-footnote-ref");
    if (!spanElement.style.verticalAlign) {
      spanElement.style.verticalAlign = "super";
    }
    if (!spanElement.style.fontSize) {
      spanElement.style.fontSize = "0.78em";
    }
    if (!spanElement.style.lineHeight) {
      spanElement.style.lineHeight = "1";
    }
    if (!spanElement.style.marginLeft) {
      spanElement.style.marginLeft = "2px";
    }
  });

  return rootElement.outerHTML;
}

// 扫描 CSS 文本中的 `--foo: value` 声明，建立自定义变量映射。
function collectCssVariablesFromText(cssText: string): Map<string, string> {
  const variableMap = new Map<string, string>();
  const variableDeclarationPattern = /(--[\w-]+)\s*:\s*([^;{}]+);/g;
  let matched: RegExpExecArray | null = variableDeclarationPattern.exec(cssText);
  while (matched) {
    const variableName = matched[1].trim();
    const variableValue = matched[2].trim();
    if (variableName && variableValue) {
      variableMap.set(variableName, variableValue);
    }
    matched = variableDeclarationPattern.exec(cssText);
  }
  return variableMap;
}

// 从指定元素的计算样式读取全部自定义变量，兜底覆盖文本解析不到的值。
function collectCssVariablesFromComputedStyle(
  element: HTMLElement | null,
  variableMap: Map<string, string>
): void {
  if (!element) {
    return;
  }
  const computedStyle = window.getComputedStyle(element);
  for (let index = 0; index < computedStyle.length; index += 1) {
    const propertyName = computedStyle.item(index);
    if (!propertyName.startsWith("--")) {
      continue;
    }
    const propertyValue = computedStyle.getPropertyValue(propertyName).trim();
    if (!propertyValue) {
      continue;
    }
    variableMap.set(propertyName, propertyValue);
  }
}

// 解析 var(...) 参数：支持 `var(--x)` 与 `var(--x, fallback)` 两种格式。
function splitVarFunctionArguments(argumentText: string): [string, string | null] {
  const trimmedArguments = argumentText.trim();
  let depth = 0;
  for (let index = 0; index < trimmedArguments.length; index += 1) {
    const currentChar = trimmedArguments[index];
    if (currentChar === "(") {
      depth += 1;
      continue;
    }
    if (currentChar === ")") {
      depth = Math.max(0, depth - 1);
      continue;
    }
    if (currentChar === "," && depth === 0) {
      const variableName = trimmedArguments.slice(0, index).trim();
      const fallbackValue = trimmedArguments.slice(index + 1).trim();
      return [variableName, fallbackValue || null];
    }
  }
  return [trimmedArguments, null];
}

// 在 CSS 文本中展开 var(...)，让 juice 在无变量能力时也能吃到最终值。
function resolveCssVariables(
  cssText: string,
  variableMap: Map<string, string>,
  maxDepth = 10
): string {
  const varFunctionPattern = /var\(\s*([^()]+(?:\([^()]*\)[^()]*)*)\s*\)/g;
  const resolveVariableReference = (inputText: string, depth: number): string => {
    if (depth >= maxDepth) {
      return inputText;
    }
    return inputText.replace(varFunctionPattern, (_matched, innerArguments) => {
      const [variableName, fallbackValue] = splitVarFunctionArguments(String(innerArguments));
      if (!variableName.startsWith("--")) {
        return fallbackValue ?? "";
      }
      const resolvedValue = variableMap.get(variableName);
      if (resolvedValue && resolvedValue.trim()) {
        return resolveVariableReference(resolvedValue.trim(), depth + 1);
      }
      if (fallbackValue) {
        return resolveVariableReference(fallbackValue, depth + 1);
      }
      return "";
    });
  };
  return resolveVariableReference(cssText, 0);
}

interface MathJaxSvgRenderer {
  renderFormulaToSvgMarkup: (texSource: string, displayMode: boolean) => string;
}

let mathJaxSvgRendererPromise: Promise<MathJaxSvgRenderer | null> | null = null;

// 惰性初始化 MathJax SVG 渲染器：仅在“复制到公众号”时加载，避免影响首屏体积。
async function getMathJaxSvgRenderer(): Promise<MathJaxSvgRenderer | null> {
  if (mathJaxSvgRendererPromise) {
    return mathJaxSvgRendererPromise;
  }

  mathJaxSvgRendererPromise = (async () => {
    try {
      const [{ mathjax }, { TeX }, { SVG }, { liteAdaptor }, { RegisterHTMLHandler }, { AllPackages }] =
        await Promise.all([
          import("mathjax-full/js/mathjax.js"),
          import("mathjax-full/js/input/tex.js"),
          import("mathjax-full/js/output/svg.js"),
          import("mathjax-full/js/adaptors/liteAdaptor.js"),
          import("mathjax-full/js/handlers/html.js"),
          import("mathjax-full/js/input/tex/AllPackages.js")
        ]);

      const adaptor = liteAdaptor();
      RegisterHTMLHandler(adaptor);
      const texInput = new TeX({
        packages: AllPackages
      });
      const svgOutput = new SVG({
        fontCache: "none"
      });
      const mathDocument = mathjax.document("", {
        InputJax: texInput,
        OutputJax: svgOutput
      });

      return {
        renderFormulaToSvgMarkup: (texSource: string, displayMode: boolean): string => {
          const mathNode = mathDocument.convert(texSource, {
            display: displayMode,
            em: 16,
            ex: 8,
            containerWidth: 80 * 16
          });
          return adaptor.outerHTML(mathNode);
        }
      };
    } catch (error) {
      console.log("MathJax 初始化失败，公式将保留 KaTeX HTML：", error);
      return null;
    }
  })();

  return mathJaxSvgRendererPromise;
}

// 从 KaTeX 渲染节点中提取原始 TeX 源码，优先使用 annotation 编码字段。
function extractKatexTexSource(katexElement: Element): string {
  const annotationElement = katexElement.querySelector(
    "annotation[encoding='application/x-tex']"
  );
  return annotationElement?.textContent?.trim() ?? "";
}

interface KatexFormulaDescriptor {
  texSource: string;
  displayMode: boolean;
  replaceTargetElement: Element;
}

// 收集公式节点描述：区分行内与块级，并产出后续替换目标元素。
function collectKatexFormulaDescriptors(rootElement: HTMLElement): KatexFormulaDescriptor[] {
  const descriptors: KatexFormulaDescriptor[] = [];
  const candidates = rootElement.querySelectorAll(".katex-display, .katex");
  candidates.forEach((candidateElement) => {
    const isDisplayWrapper = candidateElement.classList.contains("katex-display");
    if (!isDisplayWrapper && candidateElement.closest(".katex-display")) {
      // 块级公式内部的 .katex 由外层 .katex-display 统一处理，避免重复转换。
      return;
    }

    const katexElement = isDisplayWrapper
      ? candidateElement.querySelector(".katex")
      : candidateElement;
    if (!katexElement) {
      return;
    }

    const texSource = extractKatexTexSource(katexElement);
    if (!texSource) {
      return;
    }

    descriptors.push({
      texSource,
      displayMode: isDisplayWrapper,
      replaceTargetElement: candidateElement
    });
  });
  return descriptors;
}

// 将 MathJax 渲染结果提炼为纯 SVG，避免公众号环境对 mjx-container 的兼容性差异。
function extractPureSvgMarkupFromMathJax(
  mathJaxMarkup: string,
  texSource: string,
  displayMode: boolean
): string | null {
  const wrapperElement = document.createElement("div");
  wrapperElement.innerHTML = mathJaxMarkup;
  const svgElement = wrapperElement.querySelector("svg");
  if (!svgElement) {
    return null;
  }

  svgElement.removeAttribute("focusable");
  svgElement.setAttribute("role", "img");
  svgElement.setAttribute("xmlns", "http://www.w3.org/2000/svg");

  const width = svgElement.getAttribute("width");
  const height = svgElement.getAttribute("height");
  if (width) {
    svgElement.style.width = width;
    svgElement.removeAttribute("width");
  }
  if (height) {
    svgElement.style.height = height;
    svgElement.removeAttribute("height");
  }
  svgElement.style.maxWidth = "100%";
  svgElement.style.overflow = "visible";
  svgElement.style.display = displayMode ? "block" : "inline-block";
  if (displayMode) {
    svgElement.style.margin = "0 auto";
  }

  if (!svgElement.querySelector("title")) {
    const titleElement = document.createElementNS("http://www.w3.org/2000/svg", "title");
    titleElement.textContent = texSource;
    svgElement.insertBefore(titleElement, svgElement.firstChild);
  }

  return svgElement.outerHTML;
}

// 构造公式替换节点：内联公式使用 span，块级公式使用 div。
function createMathSvgReplacementElement(
  svgMarkup: string,
  texSource: string,
  displayMode: boolean
): HTMLElement {
  const replacementElement = document.createElement(displayMode ? "div" : "span");
  replacementElement.className = displayMode
    ? "plaindoc-export-formula plaindoc-export-formula--display"
    : "plaindoc-export-formula plaindoc-export-formula--inline";
  replacementElement.setAttribute("data-formula", texSource);
  replacementElement.innerHTML = svgMarkup;

  if (displayMode) {
    replacementElement.style.display = "block";
    replacementElement.style.margin = "14px 0";
    replacementElement.style.textAlign = "center";
    replacementElement.style.overflowX = "auto";
  } else {
    replacementElement.style.display = "inline-block";
    replacementElement.style.verticalAlign = "middle";
    replacementElement.style.maxWidth = "100%";
  }

  return replacementElement;
}

// 导出阶段将 KaTeX 公式动态转换为 SVG，避免公众号端对 KaTeX DOM/CSS 支持不完整。
async function convertKatexFormulasToSvg(
  sourceRoot: HTMLElement | null,
  clonedRoot: HTMLElement | null
): Promise<void> {
  if (!sourceRoot || !clonedRoot) {
    return;
  }

  const mathJaxSvgRenderer = await getMathJaxSvgRenderer();
  if (!mathJaxSvgRenderer) {
    return;
  }

  const sourceFormulaDescriptors = collectKatexFormulaDescriptors(sourceRoot);
  const clonedFormulaDescriptors = collectKatexFormulaDescriptors(clonedRoot);
  const formulaCount = Math.min(sourceFormulaDescriptors.length, clonedFormulaDescriptors.length);
  let convertedCount = 0;
  const formulaSvgCache = new Map<string, string>();

  for (let formulaIndex = 0; formulaIndex < formulaCount; formulaIndex += 1) {
    const sourceDescriptor = sourceFormulaDescriptors[formulaIndex];
    const clonedDescriptor = clonedFormulaDescriptors[formulaIndex];
    const texSource = sourceDescriptor.texSource;
    const displayMode = sourceDescriptor.displayMode;
    const cacheKey = `${displayMode ? "display" : "inline"}:${texSource}`;

    let svgMarkup = formulaSvgCache.get(cacheKey);
    if (!svgMarkup) {
      try {
        const rawMathJaxMarkup = mathJaxSvgRenderer.renderFormulaToSvgMarkup(texSource, displayMode);
        const pureSvgMarkup = extractPureSvgMarkupFromMathJax(rawMathJaxMarkup, texSource, displayMode);
        if (!pureSvgMarkup) {
          continue;
        }
        svgMarkup = pureSvgMarkup;
      } catch (error) {
        console.log("MathJax 公式转 SVG 失败，保留原 KaTeX 结果：", error);
        continue;
      }
      formulaSvgCache.set(cacheKey, svgMarkup);
    }

    clonedDescriptor.replaceTargetElement.replaceWith(
      createMathSvgReplacementElement(svgMarkup, texSource, displayMode)
    );
    convertedCount += 1;
  }

  console.log(`公式 SVG 转换完成：${convertedCount}/${formulaCount}`);
}

// 构建公众号可粘贴内容：读取当前预览区并转换为内联样式 HTML。
async function buildWechatClipboardPayload(
  options: CopyPreviewToWechatOptions = {}
): Promise<WechatClipboardPayload> {
  const previewBodyId = options.previewPaneId ?? PREVIEW_BODY_ID;
  const linkRenderMode = options.linkRenderMode ?? "link";

  const previewBody = document.getElementById(previewBodyId);
  if (!previewBody) {
    throw new Error(`导出失败：未找到预览内容节点 #${previewBodyId}`);
  }

  const clonedPreviewBody = previewBody.cloneNode(true);
  if (!(clonedPreviewBody instanceof HTMLElement)) {
    throw new Error("导出失败：预览内容克隆异常");
  }

  const footnoteStyleTokens = resolveWechatFootnoteStyleTokens(previewBody);
  // 脚注模式优先复用预览区现成“角标 + 文末脚注”，避免导出阶段重复编号。
  const hasPreviewFootnoteSection = Boolean(
    clonedPreviewBody.querySelector(".reference-footnote-title, .reference-footnote-line")
  );
  if (linkRenderMode === "footnote" && hasPreviewFootnoteSection) {
    // 保留预览区脚注结构，只做后续样式内联与兼容转换。
  } else {
    normalizeExternalLinksForWechatExport(clonedPreviewBody, footnoteStyleTokens);
  }
  inlineWechatCriticalStyles(previewBody, clonedPreviewBody);
  await convertKatexFormulasToSvg(previewBody, clonedPreviewBody);
  // Mermaid 的 SVG 样式依赖渲染时注入的 class 规则，需要从真实预览节点读取计算样式。
  inlineMermaidSvgStyles(previewBody, clonedPreviewBody);

  // 这里保留正文容器本身，保证 `#plaindoc-preview-body` 作用域选择器仍可匹配。
  const html = clonedPreviewBody.outerHTML;

  let inlinedHtml = "";
  try {
    const themeStyleText = document.getElementById("plaindoc-preview-theme-style")?.innerText ?? "";
    const appStyleText = document.getElementById("plaindoc-app-style")?.innerText ?? "";
    const katexStyleText = document.getElementById("plaindoc-katex-style")?.innerText ?? "";
    const rawStyleText = `${katexStyleText}\n${appStyleText}\n${themeStyleText}\n`;

    const cssVariableMap = collectCssVariablesFromText(rawStyleText);
    collectCssVariablesFromComputedStyle(previewBody, cssVariableMap);
    collectCssVariablesFromComputedStyle(document.documentElement, cssVariableMap);
    const resolvedStyleText = resolveCssVariables(rawStyleText, cssVariableMap);

    inlinedHtml = juice.inlineContent(html, resolvedStyleText, {
      inlinePseudoElements: true,
      preserveImportant: true
    });
    console.log("CSS 内联成功，生成的 HTML 长度：", inlinedHtml, resolvedStyleText);
  } catch (error) {
    console.log("请检查 CSS 文件是否编写正确！", error);
    inlinedHtml = html;
  }

  const normalizedWechatHtml = normalizeWechatHtmlCompatibility(inlinedHtml);

  return {
    html: normalizedWechatHtml,
    plainText: toPlainTextFromHtml(normalizedWechatHtml)
  };
}

// Clipboard API：现代浏览器优先路径，支持同时写入 HTML 与纯文本。
async function copyWithClipboardApi(payload: WechatClipboardPayload): Promise<void> {
  if (!navigator.clipboard || typeof ClipboardItem === "undefined") {
    throw new Error("当前浏览器不支持 Clipboard API");
  }

  await navigator.clipboard.write([
    new ClipboardItem({
      "text/html": new Blob([payload.html], { type: "text/html" }),
      "text/plain": new Blob([payload.plainText], { type: "text/plain" })
    })
  ]);
}

// execCommand 兼容回退：覆盖 Safari 等浏览器对 HTML 剪贴板写入限制。
function copyWithExecCommand(payload: WechatClipboardPayload): void {
  const existingInput = document.getElementById(LEGACY_COPY_INPUT_ID);
  let inputElement: HTMLInputElement;
  if (existingInput instanceof HTMLInputElement) {
    inputElement = existingInput;
  } else {
    inputElement = document.createElement("input");
    inputElement.id = LEGACY_COPY_INPUT_ID;
    inputElement.setAttribute("aria-hidden", "true");
    inputElement.tabIndex = -1;
    inputElement.style.position = "fixed";
    inputElement.style.top = "-10000px";
    inputElement.style.opacity = "0";
    inputElement.style.pointerEvents = "none";
    document.body.appendChild(inputElement);
  }

  inputElement.value = "copy";
  inputElement.focus();
  inputElement.setSelectionRange(0, inputElement.value.length);

  const onCopy = (event: ClipboardEvent) => {
    if (!event.clipboardData) {
      return;
    }
    event.preventDefault();
    event.clipboardData.setData("text/html", payload.html);
    event.clipboardData.setData("text/plain", payload.plainText);
  };

  document.addEventListener("copy", onCopy);
  try {
    const copied = document.execCommand("copy");
    if (!copied) {
      throw new Error("浏览器拒绝执行复制");
    }
  } finally {
    document.removeEventListener("copy", onCopy);
  }
}

// 对外导出：将当前预览区复制为公众号可粘贴内容。
export async function copyPreviewToWechat(
  options: CopyPreviewToWechatOptions = {}
): Promise<WechatClipboardPayload> {
  const payload = await buildWechatClipboardPayload(options);
  try {
    await copyWithClipboardApi(payload);
  } catch {
    copyWithExecCommand(payload);
  }
  return payload;
}

const MERMAID_SVG_STYLE_PROPS = [
  "display",
  "visibility",
  "fill",
  "fill-opacity",
  "stroke",
  "stroke-opacity",
  "stroke-width",
  "stroke-dasharray",
  "stroke-dashoffset",
  "stroke-linecap",
  "stroke-linejoin",
  "stroke-miterlimit",
  "font-family",
  "font-size",
  "font-weight",
  "font-style",
  "font-variant",
  "line-height",
  "white-space",
  "color",
  "opacity",
  "text-anchor",
  "text-align",
  "text-decoration",
  "text-rendering",
  "dominant-baseline",
  "alignment-baseline",
  "baseline-shift",
  "letter-spacing",
  "word-spacing",
  "background-color",
  "vector-effect",
  "filter",
  "clip-path",
  "mask",
  "transform",
  "transform-origin",
  "shape-rendering",
  "marker-start",
  "marker-mid",
  "marker-end",
  "pointer-events",
];

// 从 Mermaid 生成的 <style> 文本提取属性名，降低固定白名单遗漏风险。
function collectMermaidSvgStyleProps(svg: Element): string[] {
  const propertyNames = new Set<string>();
  const propertyPattern = /([a-zA-Z-]+)\s*:/g;
  const styleNodes = svg.querySelectorAll("style");
  styleNodes.forEach((styleNode) => {
    const cssText = styleNode.textContent ?? "";
    let matched: RegExpExecArray | null = propertyPattern.exec(cssText);
    while (matched) {
      const propertyName = matched[1].trim().toLowerCase();
      if (propertyName && !propertyName.startsWith("--")) {
        propertyNames.add(propertyName);
      }
      matched = propertyPattern.exec(cssText);
    }
  });
  return Array.from(propertyNames);
}

function inlineMermaidSvgStyles(
  sourceRoot: HTMLElement | null,
  clonedRoot: HTMLElement | null
): void {
  if (!sourceRoot || !clonedRoot) {
    return;
  }

  const sourceSvgs = sourceRoot.querySelectorAll(".mermaid-block svg");
  const clonedSvgs = clonedRoot.querySelectorAll(".mermaid-block svg");
  const svgCount = Math.min(sourceSvgs.length, clonedSvgs.length);

  for (let svgIndex = 0; svgIndex < svgCount; svgIndex += 1) {
    const sourceSvg = sourceSvgs[svgIndex];
    const clonedSvg = clonedSvgs[svgIndex];
    const stylePropNames = Array.from(
      new Set([...MERMAID_SVG_STYLE_PROPS, ...collectMermaidSvgStyleProps(sourceSvg)])
    );
    const sourceNodes: Element[] = [sourceSvg, ...sourceSvg.querySelectorAll("*")];
    const clonedNodes: Element[] = [clonedSvg, ...clonedSvg.querySelectorAll("*")];
    const nodeCount = Math.min(sourceNodes.length, clonedNodes.length);

    // 按同构顺序把“真实预览”里的计算样式写到“克隆导出树”，避免污染页面本身。
    for (let nodeIndex = 0; nodeIndex < nodeCount; nodeIndex += 1) {
      const sourceNode = sourceNodes[nodeIndex];
      const clonedNode = clonedNodes[nodeIndex];
      const computed = window.getComputedStyle(sourceNode);
      const parts: string[] = [];
      stylePropNames.forEach((prop) => {
        const value = computed.getPropertyValue(prop).trim();
        if (value) {
          parts.push(`${prop}:${value}`);
        }
      });
      if (parts.length) {
        const existing = clonedNode.getAttribute("style");
        const merged = existing ? `${existing};${parts.join(";")}` : parts.join(";");
        clonedNode.setAttribute("style", merged);
      }
    }

    // 清理 Mermaid 内嵌 style，避免公众号端丢失 class 规则后出现样式偏差。
    clonedSvg.querySelectorAll("style").forEach((styleNode) => styleNode.remove());
  }
}
