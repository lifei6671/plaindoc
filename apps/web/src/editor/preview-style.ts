import type { CSSProperties } from "react";
import {
  PREVIEW_BODY_ID,
  PREVIEW_THEME_CLASS_PREFIX
} from "./constants";
import type { PreviewThemeTemplate } from "../preview-themes";

// 抽屉样式键值对：用于统一展示当前生效样式。
interface StyleDetailEntry {
  property: string;
  value: string;
}

// 规范化外部传入的样式文本，统一裁剪空白并兜底为空字符串。
export function normalizePreviewStyleText(styleText: unknown): string {
  if (typeof styleText !== "string") {
    return "";
  }
  return styleText.trim();
}

// 根据主题 ID 生成预览正文类名。
export function getPreviewThemeClassName(themeId: string): string {
  return `${PREVIEW_THEME_CLASS_PREFIX}${themeId}`;
}

// 将主题变量序列化为 style 标签文本，便于动态注入。
export function buildPreviewThemeStyleText(theme: PreviewThemeTemplate): string {
  const declarations = Object.entries(theme.variables)
    .map(([variableName, variableValue]) => `  ${variableName}: ${variableValue};`)
    .join("\n");
  if (!declarations) {
    return "";
  }
  const themeSelector = `#${PREVIEW_BODY_ID}.${getPreviewThemeClassName(theme.id)}`;
  return `${themeSelector} {\n${declarations}\n}`;
}

// 将驼峰样式键转换为 kebab-case，便于用户直观查看 CSS 属性名。
function toKebabCaseStyleProperty(styleProperty: string): string {
  if (!styleProperty) {
    return styleProperty;
  }
  if (styleProperty.startsWith("--")) {
    return styleProperty;
  }
  return styleProperty.replace(/[A-Z]/g, (matched) => `-${matched.toLowerCase()}`);
}

// 统一格式化 CSSProperties 的值，便于在抽屉中输出。
function formatStyleDetailValue(styleValue: unknown): string {
  if (typeof styleValue === "string" || typeof styleValue === "number") {
    return String(styleValue);
  }
  if (styleValue === null || styleValue === undefined) {
    return "";
  }
  return String(styleValue);
}

// 将 CSSProperties 转成可展示的键值数组，并过滤空值。
function buildStyleDetailEntries(styleObject: CSSProperties): StyleDetailEntry[] {
  return Object.entries(styleObject as Record<string, unknown>)
    .map(([property, value]) => ({
      property: toKebabCaseStyleProperty(property),
      value: formatStyleDetailValue(value)
    }))
    .filter((entry) => entry.property && entry.value)
    .sort((left, right) => left.property.localeCompare(right.property));
}

// 将样式条目序列化为 CSS declaration 文本。
function buildCssDeclarationsSource(entries: StyleDetailEntry[]): string {
  if (!entries.length) {
    return "  /* 无样式声明 */";
  }
  return entries.map((entry) => `  ${entry.property}: ${entry.value};`).join("\n");
}

// 生成当前主题可复制的 CSS 模板（包含注释说明）。
export function buildThemeCssTemplate(theme: PreviewThemeTemplate): string {
  const previewBodySelector = `#${PREVIEW_BODY_ID}`;
  const previewThemeSelector = `${previewBodySelector}.${getPreviewThemeClassName(theme.id)}`;
  const sortedVariables = Object.entries(theme.variables).sort(([left], [right]) =>
    left.localeCompare(right)
  );
  const variableSource =
    sortedVariables.length > 0
      ? sortedVariables.map(([name, value]) => `  ${name}: ${value};`).join("\n")
      : "  /* 无变量声明 */";
  const codeBlockSource = buildCssDeclarationsSource(buildStyleDetailEntries(theme.codeBlockStyle));
  const codeBlockCodeSource = buildCssDeclarationsSource(
    buildStyleDetailEntries(theme.codeBlockCodeStyle)
  );
  const inlineCodeSource = buildCssDeclarationsSource(buildStyleDetailEntries(theme.inlineCodeStyle));

  return `/* PlainDoc 主题样式模板（可复制后直接修改） 
 * 主题名称：${theme.name}
 * 主题 ID：${theme.id}
 * 语法高亮：${theme.syntaxTheme}
 */

/* 预览区基础变量 */
${previewThemeSelector} {
${variableSource}
}

/* 代码块容器（fenced code -> pre） */
${previewBodySelector} pre {
${codeBlockSource}
}

/* 代码块文本（pre > code） */
${previewBodySelector} pre code {
${codeBlockCodeSource}
}

/* 行内代码（p/li/table 内 code） */
${previewBodySelector} p code,
${previewBodySelector} li code,
${previewBodySelector} table code {
${inlineCodeSource}
}`;
}
