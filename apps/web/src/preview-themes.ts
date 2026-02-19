import type { CSSProperties } from "react";
import { oneDark, oneLight } from "react-syntax-highlighter/dist/esm/styles/prism";
import type { Theme } from "./data-access/types";
import { BUILTIN_THEME_PRESETS } from "./theme-presets";

// 语法高亮主题标识：用于在主题模板中选择代码高亮方案。
export type PreviewSyntaxThemeId = "one-light" | "one-dark";

// 预览主题模板定义：统一描述预览区可切换样式。
export interface PreviewThemeTemplate {
  id: string;
  name: string;
  description: string;
  variables: Record<string, string>;
  syntaxTheme: PreviewSyntaxThemeId;
  codeBlockStyle: CSSProperties;
  codeBlockCodeStyle: CSSProperties;
  inlineCodeStyle: CSSProperties;
  customCss?: string;
  builtin?: boolean;
}

// 代码高亮主题映射表：根据主题标识提供对应的 Prism 配色对象。
export const PREVIEW_SYNTAX_THEMES: Record<PreviewSyntaxThemeId, Record<string, CSSProperties>> = {
  "one-light": oneLight as Record<string, CSSProperties>,
  "one-dark": oneDark as Record<string, CSSProperties>
};

function normalizeSyntaxTheme(value: string): PreviewSyntaxThemeId {
  return value === "one-dark" ? "one-dark" : "one-light";
}

export function toPreviewThemeTemplate(theme: Theme): PreviewThemeTemplate {
  return {
    id: theme.id,
    name: theme.name,
    description: theme.description,
    variables: theme.variables,
    syntaxTheme: normalizeSyntaxTheme(theme.syntaxTheme),
    codeBlockStyle: theme.codeBlockStyle as CSSProperties,
    codeBlockCodeStyle: theme.codeBlockCodeStyle as CSSProperties,
    inlineCodeStyle: theme.inlineCodeStyle as CSSProperties,
    customCss: theme.customCss,
    builtin: theme.builtin
  };
}

export const BUILTIN_PREVIEW_THEME_TEMPLATES: PreviewThemeTemplate[] =
  BUILTIN_THEME_PRESETS.map(toPreviewThemeTemplate);

const FALLBACK_PREVIEW_THEME_TEMPLATE: PreviewThemeTemplate = {
  id: "default",
  name: "内置默认",
  description: "通用文档风格",
  variables: {},
  syntaxTheme: "one-light",
  codeBlockStyle: {},
  codeBlockCodeStyle: {},
  inlineCodeStyle: {},
  customCss: "",
  builtin: true
};

export const DEFAULT_PREVIEW_THEME_TEMPLATE: PreviewThemeTemplate =
  BUILTIN_PREVIEW_THEME_TEMPLATES[0] ?? FALLBACK_PREVIEW_THEME_TEMPLATE;

// 按主题 ID 返回模板；找不到时回退到传入数组首项或默认主题。
export function resolvePreviewTheme(
  themeId: string,
  themes: PreviewThemeTemplate[]
): PreviewThemeTemplate {
  const source = themes.length ? themes : BUILTIN_PREVIEW_THEME_TEMPLATES;
  if (!source.length) {
    return DEFAULT_PREVIEW_THEME_TEMPLATE;
  }
  const foundTheme = source.find((theme) => theme.id === themeId);
  return foundTheme ?? source[0];
}
