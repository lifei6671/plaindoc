// 编辑器初始化时使用的兜底内容。
export const FALLBACK_CONTENT = `# PlainDoc

加载中...
`;

// 参与锚点映射的 block 级节点类型。
export const BLOCK_NODE_TYPES = new Set([
  "paragraph",
  "heading",
  "blockquote",
  "code",
  // 块级公式同样需要参与锚点映射，避免长公式场景同步漂移。
  "math",
  "table",
  "thematicBreak",
  "html"
]);

// 预览区锚点节点选择器：仅采集 remark 注入的 block 级锚点，避免子节点噪声干扰。
export const BLOCK_ANCHOR_SELECTOR = "[data-anchor-index]";

// TOC 最大显示层级，默认覆盖 h1~h6 标题。
export const TOC_MAX_DEPTH = 6;

// TOC 语法标记匹配规则，仅匹配完整段落的 [TOC]。
export const TOC_MARKER_PATTERN = /^\[toc\]$/i;

// 同步滚动单帧最大步进，避免 TOC 高度失配时出现“瞬移”。
export const MAX_SYNC_STEP_PER_FRAME = 120;

// 同步收敛阈值，小于该值直接视为对齐完成。
export const SYNC_SETTLE_THRESHOLD = 0.5;

// 预览区固定根节点 ID，供外部样式覆盖时稳定选择。
export const PREVIEW_PANE_ID = "plaindoc-preview-pane";

// 预览区固定根节点类名，便于 class 级样式覆盖。
export const PREVIEW_PANE_CLASS = "plaindoc-preview-pane";

// 预览区主题类名前缀，实际类名为 `plaindoc-preview-pane--{themeId}`。
export const PREVIEW_PANE_THEME_CLASS_PREFIX = "plaindoc-preview-pane--";

// 默认主题 ID。
export const DEFAULT_PREVIEW_THEME_ID = "default";

// 预览内容容器类名，外部样式可直接作用于该容器。
export const PREVIEW_BODY_CLASS = "plaindoc-preview-body";

// 预览内容容器查询选择器（DOM 观测使用）。
export const PREVIEW_BODY_SELECTOR = `.${PREVIEW_BODY_CLASS}`;

// 本地持久化自定义样式的键名。
export const PREVIEW_CUSTOM_STYLE_STORAGE_KEY = "plaindoc.preview.custom-style";

// 本地持久化主题模板的键名。
export const PREVIEW_THEME_STORAGE_KEY = "plaindoc.preview.theme-template";

// 外部通知预览样式变更的自定义事件名。
export const PREVIEW_CUSTOM_STYLE_EVENT = "plaindoc:preview-style-change";
