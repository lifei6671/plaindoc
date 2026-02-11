// 文档保存状态机。
export type SaveStatus = "loading" | "ready" | "saving" | "saved" | "conflict" | "error";

// 状态栏保存图标类型。
export type SaveIndicatorVariant = "unsaved" | "saving" | "saved";

// 当前滚动事件的来源。
export type ScrollSource = "editor" | "preview";

// 编辑区与预览区的单个锚点映射。
export interface ScrollAnchor {
  editorY: number;
  previewY: number;
}

// 单向映射锚点（sourceY -> targetY）。
export interface DirectionAnchor {
  sourceY: number;
  targetY: number;
}

// TOC 单条目录项信息。
export interface TocItem {
  level: number;
  text: string;
  sourceLine: number;
}

// TOC 解析结果：同时返回标题目录与是否存在 [TOC] 语法标记。
export interface TocParseResult {
  items: TocItem[];
  hasMarker: boolean;
}

// 仅包含本模块需要访问的 Markdown AST 字段。
export interface MarkdownNode {
  type: string;
  position?: {
    start?: {
      line?: number;
      offset?: number;
    };
    end?: {
      line?: number;
      offset?: number;
    };
  };
  data?: {
    hProperties?: Record<string, unknown>;
  };
  children?: MarkdownNode[];
}

// 仅包含 TOC 解析所需的 markdown-it token 字段。
export interface MarkdownToken {
  type: string;
  tag?: string;
  map?: number[] | null;
  content?: string;
  children?: MarkdownToken[] | null;
}
