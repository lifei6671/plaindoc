import googleSansCodeLatinStyleText from "@fontsource/google-sans-code/latin.css?inline";
import katexStyleText from "katex/dist/katex.min.css?inline";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import appStyleText from "./styles.css?inline";

// 注入内联样式：避免构建产物通过 <link> 加载 CSS，统一改为 <style> 挂载。
function ensureInlineStyleTag(styleId: string, cssText: string): void {
  if (!cssText.trim()) {
    return;
  }
  const existingStyleTag = document.getElementById(styleId);
  if (existingStyleTag instanceof HTMLStyleElement) {
    if (existingStyleTag.textContent !== cssText) {
      existingStyleTag.textContent = cssText;
    }
    return;
  }

  const styleTag = document.createElement("style");
  styleTag.id = styleId;
  styleTag.textContent = cssText;
  document.head.appendChild(styleTag);
}

// 样式注入顺序：先 KaTeX 基础样式，再注入应用样式以便必要时覆盖。
ensureInlineStyleTag("plaindoc-katex-style", katexStyleText);
ensureInlineStyleTag("plaindoc-app-style", appStyleText);
ensureInlineStyleTag("plaindoc-google-sans-code-style", googleSansCodeLatinStyleText);

// 开发模式关闭 StrictMode 双挂载，避免编辑器滚动容器在调试时反复重建。
const appRoot = import.meta.env.DEV ? (
  <App />
) : (
  <StrictMode>
    <App />
  </StrictMode>
);

createRoot(document.getElementById("root")!).render(appRoot);
