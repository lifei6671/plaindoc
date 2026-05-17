import { describe, expect, it } from "vitest";
import type { DocumentRevisionFormat, RestoreDocumentRevisionResult } from "../data-access";
import {
  hasRevisionHistoryBlockingUnsavedChanges,
  shouldReloadOnlyOfficeEditConfigAfterRevisionRestore
} from "./onlyoffice-edit-config";

function createRestoreResult(format: DocumentRevisionFormat): RestoreDocumentRevisionResult {
  return {
    document: {
      id: "doc-office",
      nodeId: "node-office",
      themeId: "default",
      title: "Office 文档",
      contentMd: "",
      format,
      version: 3,
      contentVersion: 3,
      updatedAt: "2026-05-17T00:00:00Z"
    },
    restoredFromRevision: {
      id: "revision-1",
      documentId: "doc-office",
      version: 1,
      baseVersion: 0,
      createdAt: "2026-05-17T00:00:00Z",
      source: "remote",
      format
    }
  };
}

describe("shouldReloadOnlyOfficeEditConfigAfterRevisionRestore", () => {
  it("requires config reload for restored Office documents only", () => {
    expect(shouldReloadOnlyOfficeEditConfigAfterRevisionRestore(createRestoreResult("docx"))).toBe(true);
    expect(shouldReloadOnlyOfficeEditConfigAfterRevisionRestore(createRestoreResult("xlsx"))).toBe(true);
    expect(shouldReloadOnlyOfficeEditConfigAfterRevisionRestore(createRestoreResult("markdown"))).toBe(false);
  });
});

describe("hasRevisionHistoryBlockingUnsavedChanges", () => {
  it("blocks Office revision restore when ONLYOFFICE reports dirty state through ready save status", () => {
    expect(hasRevisionHistoryBlockingUnsavedChanges({
      content: "",
      isOfficeDocument: true,
      lastSavedContent: "",
      saveStatus: "ready"
    })).toBe(true);
  });

  it("keeps markdown restore protection based on editor content diff", () => {
    expect(hasRevisionHistoryBlockingUnsavedChanges({
      content: "# 本地修改",
      isOfficeDocument: false,
      lastSavedContent: "# 已保存",
      saveStatus: "ready"
    })).toBe(true);
    expect(hasRevisionHistoryBlockingUnsavedChanges({
      content: "# 已保存",
      isOfficeDocument: false,
      lastSavedContent: "# 已保存",
      saveStatus: "saved"
    })).toBe(false);
  });
});
