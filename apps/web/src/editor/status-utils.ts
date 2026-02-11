import type { TreeNode } from "../data-access";
import type { SaveIndicatorVariant, SaveStatus } from "./types";

// 在文档树中找到首个文档节点。
export function findFirstDocId(nodes: TreeNode[]): string | null {
  for (const node of nodes) {
    if (node.type === "doc") {
      return node.id;
    }
    const childDocId = findFirstDocId(node.children);
    if (childDocId) {
      return childDocId;
    }
  }
  return null;
}

// 将 unknown 错误转换为可展示文案。
export function formatError(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return "未知错误";
}

// 将 ISO 时间格式化为“时:分:秒”。
export function formatSavedTime(isoTime: string | null): string {
  if (!isoTime) {
    return "未保存";
  }
  const parsed = new Date(isoTime);
  if (Number.isNaN(parsed.getTime())) {
    return "未保存";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    hour12: false,
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit"
  }).format(parsed);
}

// 将保存状态映射为状态栏图标的展示类型。
export function resolveSaveIndicatorVariant(saveStatus: SaveStatus): SaveIndicatorVariant {
  if (saveStatus === "saved") {
    return "saved";
  }
  if (saveStatus === "saving" || saveStatus === "loading") {
    return "saving";
  }
  return "unsaved";
}
