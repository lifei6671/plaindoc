import type { CSSProperties } from "react";
import { oneDark, oneLight } from "react-syntax-highlighter/dist/esm/styles/prism";

// 语法高亮主题标识：用于在主题模板中选择代码高亮方案。
export type PreviewSyntaxThemeId = "one-light" | "one-dark";

// 内置主题模板定义：用于统一描述预览区可切换样式。
export interface PreviewThemeTemplate {
  id: string;
  name: string;
  description: string;
  variables: Record<string, string>;
  syntaxTheme: PreviewSyntaxThemeId;
  codeBlockStyle: CSSProperties;
  codeBlockCodeStyle: CSSProperties;
  inlineCodeStyle: CSSProperties;
}

// 代码高亮主题映射表：根据主题标识提供对应的 Prism 配色对象。
export const PREVIEW_SYNTAX_THEMES: Record<PreviewSyntaxThemeId, Record<string, CSSProperties>> = {
  "one-light": oneLight as Record<string, CSSProperties>,
  "one-dark": oneDark as Record<string, CSSProperties>
};

// 默认代码块容器样式：复制到第三方平台时尽可能保留视觉表现。
const DEFAULT_CODE_BLOCK_STYLE: CSSProperties = {
  margin: "16px 0",
  padding: "14px 16px",
  borderRadius: "10px",
  border: "1px solid #dbe2ea",
  boxShadow: "0 1px 2px rgba(15, 23, 42, 0.06)",
  overflowX: "auto",
  fontSize: "13px",
  lineHeight: 1.65,
  background: "#f8fafc"
};

// 默认代码块 code 标签样式：统一等宽字体提升可读性。
const DEFAULT_CODE_BLOCK_CODE_STYLE: CSSProperties = {
  fontFamily: "\"Operator Mono\",\"SFMono-Regular\", Menlo, Monaco, Consolas, \"Liberation Mono\", \"Courier New\", monospace"
};

// 默认行内代码样式：保证未启用 fenced code 时仍有清晰区分。
const DEFAULT_INLINE_CODE_STYLE: CSSProperties = {
  padding: "1px 6px",
  borderRadius: "5px",
  border: "1px solid #dbe2ea",
  background: "#f1f5f9",
  color: "#0f172a",
  fontSize: "0.92em",
  fontFamily: "\"Operator Mono\",\"SFMono-Regular\", Menlo, Monaco, Consolas, \"Liberation Mono\", \"Courier New\", monospace"
};

// 内置主题模板列表：供主题菜单直接切换。
export const PREVIEW_THEME_TEMPLATES: PreviewThemeTemplate[] = [
  {
    id: "default",
    name: "内置默认",
    description: "通用文档风格",
    variables: {
      "--pd-preview-padding": "30px",
      "--pd-preview-font-family":
        "Optima-Regular, Optima, PingFangSC-light, PingFangTC-light, 'PingFang SC', Cambria, Cochin, Georgia, Times, 'Times New Roman', serif",
      "--pd-preview-text-color": "rgb(89, 89, 89)",
      "--pd-preview-link-color": "rgb(71, 193, 168)",
      "--pd-preview-inline-code-color": "rgb(71, 193, 168)",
      "--pd-preview-font-size": "16px",
      "--pd-preview-line-height": "26px",
      "--pd-preview-word-spacing": "3px",
      "--pd-preview-letter-spacing": "0.02em",
      "--pd-preview-paragraph-margin-top": "5px",
      "--pd-preview-paragraph-margin-bottom": "5px",
      "--pd-preview-paragraph-indent": "2em",
      "--pd-preview-title-color": "rgb(89, 89, 89)",
      // 标题层级字号变量：支持 h1~h6 独立定制。
      "--pd-preview-h1-font-size": "2em",
      "--pd-preview-h2-font-size": "1.5em",
      "--pd-preview-h3-font-size": "1.25em",
      "--pd-preview-h4-font-size": "1.05em",
      "--pd-preview-h5-font-size": "0.95em",
      "--pd-preview-h6-font-size": "0.88em",
      // 标题层级颜色变量：支持 h1~h6 独立定制。
      "--pd-preview-h1-color": "rgb(89, 89, 89)",
      "--pd-preview-h2-color": "rgb(89, 89, 89)",
      "--pd-preview-h3-color": "rgb(89, 89, 89)",
      "--pd-preview-h4-color": "rgb(89, 89, 89)",
      "--pd-preview-h5-color": "rgb(89, 89, 89)",
      "--pd-preview-h6-color": "rgb(89, 89, 89)",
      // 标题层级边框宽度：默认仅 h2 展示下边框，其余层级默认关闭。
      "--pd-preview-h1-border-width": "0",
      "--pd-preview-h2-border-width": "2px",
      "--pd-preview-h3-border-width": "0",
      "--pd-preview-h4-border-width": "0",
      "--pd-preview-h5-border-width": "0",
      "--pd-preview-h6-border-width": "0",
      // 标题层级边框颜色：用户可单独开启任意层级的边框表现。
      "--pd-preview-h1-border-color": "transparent",
      "--pd-preview-h2-border-color": "rgb(89, 89, 89)",
      "--pd-preview-h3-border-color": "transparent",
      "--pd-preview-h4-border-color": "transparent",
      "--pd-preview-h5-border-color": "transparent",
      "--pd-preview-h6-border-color": "transparent",
      "--pd-preview-blockquote-text-color": "#666666",
      "--pd-preview-blockquote-mark-color": "#555555",
      "--pd-preview-blockquote-background": "#f8fafc",
      "--pd-preview-blockquote-border-color": "#cbd5e1",
      "--pd-preview-strong-color": "rgb(71, 193, 168)",
      "--pd-preview-em-color": "rgb(71, 193, 168)",
      "--pd-preview-hr-color": "#cbd5e1",
      "--pd-preview-image-width": "auto",
      "--pd-preview-table-font-size": "14px",
      "--pd-preview-table-border-color": "#dbe2ea",
      "--pd-preview-table-cell-padding": "10px 12px"
    },
    syntaxTheme: "one-light",
    codeBlockStyle: { ...DEFAULT_CODE_BLOCK_STYLE },
    codeBlockCodeStyle: { ...DEFAULT_CODE_BLOCK_CODE_STYLE },
    inlineCodeStyle: { ...DEFAULT_INLINE_CODE_STYLE }
  },
  {
    id: "newspaper",
    name: "报刊主题",
    description: "更适合长文阅读",
    variables: {
      "--pd-preview-padding": "34px",
      "--pd-preview-font-family":
        "Optima-Regular, Optima, PingFangSC-light, PingFangTC-light, 'PingFang SC', Cambria, Cochin, Georgia, Times, 'Times New Roman', serif",
      "--pd-preview-text-color": "#334155",
      "--pd-preview-link-color": "#0f766e",
      "--pd-preview-inline-code-color": "#0f766e",
      "--pd-preview-font-size": "16px",
      "--pd-preview-line-height": "30px",
      "--pd-preview-word-spacing": "1px",
      "--pd-preview-letter-spacing": "0.01em",
      "--pd-preview-paragraph-margin-top": "8px",
      "--pd-preview-paragraph-margin-bottom": "8px",
      "--pd-preview-paragraph-indent": "2em",
      "--pd-preview-title-color": "rgb(89,89,89)",
      // 标题层级字号变量：支持 h1~h6 独立定制。
      "--pd-preview-h1-font-size": "2.15em",
      "--pd-preview-h2-font-size": "1.7em",
      "--pd-preview-h3-font-size": "1.35em",
      "--pd-preview-h4-font-size": "1.15em",
      "--pd-preview-h5-font-size": "1em",
      "--pd-preview-h6-font-size": "0.92em",
      // 标题层级颜色变量：支持 h1~h6 独立定制。
      "--pd-preview-h1-color": "rgb(89,89,89)",
      "--pd-preview-h2-color": "rgb(89,89,89)",
      "--pd-preview-h3-color": "rgb(89,89,89)",
      "--pd-preview-h4-color": "rgb(89,89,89)",
      "--pd-preview-h5-color": "rgb(89,89,89)",
      "--pd-preview-h6-color": "rgb(89,89,89)",
      // 标题层级边框宽度：默认仅 h2 展示下边框，其余层级默认关闭。
      "--pd-preview-h1-border-width": "0",
      "--pd-preview-h2-border-width": "2px",
      "--pd-preview-h3-border-width": "0",
      "--pd-preview-h4-border-width": "0",
      "--pd-preview-h5-border-width": "0",
      "--pd-preview-h6-border-width": "0",
      // 标题层级边框颜色：用户可单独开启任意层级的边框表现。
      "--pd-preview-h1-border-color": "transparent",
      "--pd-preview-h2-border-color": "#334155",
      "--pd-preview-h3-border-color": "transparent",
      "--pd-preview-h4-border-color": "transparent",
      "--pd-preview-h5-border-color": "transparent",
      "--pd-preview-h6-border-color": "transparent",
      "--pd-preview-blockquote-text-color": "#475569",
      "--pd-preview-blockquote-mark-color": "#334155",
      "--pd-preview-blockquote-background": "#f8fafc",
      "--pd-preview-blockquote-border-color": "#94a3b8",
      "--pd-preview-strong-color": "#0f766e",
      "--pd-preview-em-color": "#0f766e",
      "--pd-preview-hr-color": "#94a3b8",
      "--pd-preview-image-width": "auto",
      "--pd-preview-table-font-size": "14px",
      "--pd-preview-table-border-color": "#cbd5e1",
      "--pd-preview-table-cell-padding": "10px 12px"
    },
    syntaxTheme: "one-light",
    codeBlockStyle: {
      ...DEFAULT_CODE_BLOCK_STYLE,
      borderRadius: "8px",
      border: "1px solid #cbd5e1",
      background: "#f8fafc"
    },
    codeBlockCodeStyle: {
      ...DEFAULT_CODE_BLOCK_CODE_STYLE,
      fontFamily: "\"Source Code Pro\", \"SFMono-Regular\", Menlo, Monaco, Consolas, monospace"
    },
    inlineCodeStyle: {
      ...DEFAULT_INLINE_CODE_STYLE,
      background: "#ecfeff",
      border: "1px solid #99f6e4",
      color: "#115e59"
    }
  },
  {
    id: "clean-tech",
    name: "清爽技术",
    description: "偏开发文档排版",
    variables: {
      "--pd-preview-padding": "26px",
      "--pd-preview-font-family":
        "Optima-Regular, Optima, PingFangSC-light, PingFangTC-light, 'PingFang SC', Cambria, Cochin, Georgia, Times, 'Times New Roman', serif",
      "--pd-preview-text-color": "#334155",
      "--pd-preview-link-color": "#0ea5e9",
      "--pd-preview-inline-code-color": "#0284c7",
      "--pd-preview-font-size": "16px",
      "--pd-preview-line-height": "24px",
      "--pd-preview-word-spacing": "0",
      "--pd-preview-letter-spacing": "0",
      "--pd-preview-paragraph-margin-top": "6px",
      "--pd-preview-paragraph-margin-bottom": "6px",
      "--pd-preview-paragraph-indent": "0",
      "--pd-preview-title-color": "rgb(89,89,89)",
      // 标题层级字号变量：支持 h1~h6 独立定制。
      "--pd-preview-h1-font-size": "2.0em",
      "--pd-preview-h2-font-size": "1.65em",
      "--pd-preview-h3-font-size": "1.4em",
      "--pd-preview-h4-font-size": "1.25em",
      "--pd-preview-h5-font-size": "1.15em",
      "--pd-preview-h6-font-size": ".08em",
      // 标题层级颜色变量：支持 h1~h6 独立定制。
      "--pd-preview-h1-color": "rgb(89,89,89)",
      "--pd-preview-h2-color": "rgb(89,89,89)",
      "--pd-preview-h3-color": "rgb(89,89,89)",
      "--pd-preview-h4-color": "rgb(89,89,89)",
      "--pd-preview-h5-color": "rgb(89,89,89)",
      "--pd-preview-h6-color": "rgb(89,89,89)",
      // 标题层级边框宽度：默认仅 h2 展示下边框，其余层级默认关闭。
      "--pd-preview-h1-border-width": "0",
      "--pd-preview-h2-border-width": "2px",
      "--pd-preview-h3-border-width": "0",
      "--pd-preview-h4-border-width": "0",
      "--pd-preview-h5-border-width": "0",
      "--pd-preview-h6-border-width": "0",
      // 标题层级边框颜色：用户可单独开启任意层级的边框表现。
      "--pd-preview-h1-border-color": "transparent",
      "--pd-preview-h2-border-color": "#475569",
      "--pd-preview-h3-border-color": "transparent",
      "--pd-preview-h4-border-color": "transparent",
      "--pd-preview-h5-border-color": "transparent",
      "--pd-preview-h6-border-color": "transparent",
      "--pd-preview-blockquote-text-color": "#475569",
      "--pd-preview-blockquote-mark-color": "#64748b",
      "--pd-preview-blockquote-background": "#f1f5f9",
      "--pd-preview-blockquote-border-color": "#cbd5e1",
      "--pd-preview-strong-color": "#0369a1",
      "--pd-preview-em-color": "#0284c7",
      "--pd-preview-hr-color": "#cbd5e1",
      "--pd-preview-image-width": "auto",
      "--pd-preview-table-font-size": "13px",
      "--pd-preview-table-border-color": "#cbd5e1",
      "--pd-preview-table-cell-padding": "8px 10px"
    },
    syntaxTheme: "one-dark",
    codeBlockStyle: {
      ...DEFAULT_CODE_BLOCK_STYLE,
      borderRadius: "12px",
      border: "1px solid #1e293b",
      boxShadow: "0 8px 18px rgba(2, 6, 23, 0.2)",
      background: "#0f172a"
    },
    codeBlockCodeStyle: {
      ...DEFAULT_CODE_BLOCK_CODE_STYLE,
      color: "#dbeafe"
    },
    inlineCodeStyle: {
      ...DEFAULT_INLINE_CODE_STYLE,
      background: "#e9eef6",
      border: "none",
      borderRadius: "6px",
      padding: "1px 6px",
      color: "#444746",
      fontFamily: "Google Sans Mono,monospace"
    }
  }
];

// 按主题 ID 返回模板；找不到时回退默认模板（列表第一项）。
export function resolvePreviewTheme(themeId: string): PreviewThemeTemplate {
  const foundTheme = PREVIEW_THEME_TEMPLATES.find((theme) => theme.id === themeId);
  return foundTheme ?? PREVIEW_THEME_TEMPLATES[0];
}
