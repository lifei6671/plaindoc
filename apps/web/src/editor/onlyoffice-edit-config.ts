import type { DocumentFormat, RestoreDocumentRevisionResult } from "../data-access";
import type { SaveStatus } from "./types";

function isOnlyOfficeDocumentFormat(format: DocumentFormat | undefined): boolean {
  return format === "docx" || format === "xlsx";
}

interface RevisionHistoryUnsavedChangesInput {
  content: string;
  isOfficeDocument: boolean;
  lastSavedContent: string;
  saveStatus: SaveStatus;
}

// 历史版本恢复会在后端切换 Office 文档的源文件 blob；前端必须重新拉取编辑配置，
// 否则 ONLYOFFICE 仍会继续使用恢复前的 document key 和下载地址。
export function shouldReloadOnlyOfficeEditConfigAfterRevisionRestore(
  result: RestoreDocumentRevisionResult
): boolean {
  return isOnlyOfficeDocumentFormat(result.document.format);
}

// 历史版本恢复会覆盖当前文档内容。Markdown 通过正文差异判断未保存修改；
// Office 文档的正文不在 React 状态中，dirty 事件会映射为 saveStatus=ready，需要单独拦截。
export function hasRevisionHistoryBlockingUnsavedChanges(
  input: RevisionHistoryUnsavedChangesInput
): boolean {
  if (input.isOfficeDocument) {
    return input.saveStatus === "ready";
  }
  return input.content !== input.lastSavedContent &&
    input.saveStatus !== "loading" &&
    input.saveStatus !== "saving";
}
