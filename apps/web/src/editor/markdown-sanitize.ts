import type { Schema } from "hast-util-sanitize";
import { defaultSchema } from "rehype-sanitize";

const defaultAttributes = defaultSchema.attributes ?? {};
const defaultGlobalAttributes = defaultAttributes["*"] ?? [];
const defaultCodeAttributes = defaultAttributes.code ?? [];
const defaultLinkAttributes = defaultAttributes.a ?? [];

// Markdown 内嵌 HTML 的清洗白名单：在开启 rehype-raw 后用于兜底 XSS 防护。
export const PREVIEW_HTML_SANITIZE_SCHEMA: Schema = {
  ...defaultSchema,
  attributes: {
    ...defaultSchema.attributes,
    // 保留 className 与 data 属性，确保滚动同步锚点（data-source-line 等）不会被清洗掉。
    "*": [
      ...defaultGlobalAttributes,
      "className",
      // hast-util-sanitize 对 data 属性建议使用 `data*` 放行；否则会把自定义锚点清洗掉。
      "data*"
    ],
    // 数学公式依赖 `math-inline` / `math-display` class，必须在清洗阶段保留。
    code: [
      ...defaultCodeAttributes,
      ["className", /^language-./, "math-inline", "math-display"]
    ],
    // 允许受控 target/rel，满足常见链接需求，同时避免不安全组合。
    a: [
      ...defaultLinkAttributes,
      ["target", "_blank", "_self", "_parent", "_top"],
      ["rel", "noopener", "noreferrer", "nofollow", "ugc"]
    ]
  },
  // 仅放行常用安全协议，阻断 `javascript:` 和 `data:` 等潜在攻击向量。
  protocols: {
    ...defaultSchema.protocols,
    href: ["http", "https", "mailto", "tel"],
    src: ["http", "https"]
  }
};

// 允许 remark-rehype 接收原始 HTML，再由 rehype-sanitize 做白名单清洗。
export const PREVIEW_MARKDOWN_REHYPE_OPTIONS = {
  allowDangerousHtml: true
};
