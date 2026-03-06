import { act, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { OnlyOfficeEditorPane } from "./OnlyOfficeEditorPane";

describe("OnlyOfficeEditorPane", () => {
  const destroyEditor = vi.fn();
  let latestConfig: Record<string, unknown> | null = null;
  const docEditorMock = vi.fn(function MockDocEditor(
    _placeholderId: string,
    config: Record<string, unknown>
  ) {
    latestConfig = config;
    return {
      destroyEditor
    };
  });

  beforeEach(() => {
    destroyEditor.mockReset();
    docEditorMock.mockClear();
    latestConfig = null;
    window.DocsAPI = {
      DocEditor: docEditorMock as never
    };
  });

  afterEach(() => {
    delete window.DocsAPI;
  });

  it("creates and destroys doc editor when edit config is ready", async () => {
    const onStateChange = vi.fn();
    const { unmount } = render(
      <OnlyOfficeEditorPane
        documentId="doc-office-1"
        documentTitle="季度计划"
        documentFormat="docx"
        editConfig={{
          documentServerUrl: "https://onlyoffice.example.com",
          config: {
            document: {
              key: "doc-office-1:1"
            }
          }
        }}
        isConfigLoading={false}
        errorMessage={null}
        onStateChange={onStateChange}
      />
    );

    await waitFor(() => {
      expect(docEditorMock).toHaveBeenCalledTimes(1);
    });

    expect(docEditorMock).toHaveBeenCalledWith(
      expect.stringMatching(/^onlyoffice-editor-/),
      expect.objectContaining({
        document: expect.objectContaining({
          key: "doc-office-1:1"
        })
      })
    );
    expect(onStateChange).toHaveBeenCalledWith(
      expect.objectContaining({
        status: "loading",
        message: "正在加载 ONLYOFFICE 编辑器..."
      })
    );
    expect(onStateChange).toHaveBeenCalledWith(
      expect.objectContaining({
        status: "loading",
        message: "正在初始化 ONLYOFFICE 文档..."
      })
    );

    const events = latestConfig?.events as Record<string, unknown>;
    expect(events).toBeTruthy();

    act(() => {
      (events.onDocumentReady as () => void)();
    });

    expect(onStateChange).toHaveBeenCalledWith(
      expect.objectContaining({
        status: "ready",
        message: "Word编辑器已就绪"
      })
    );

    act(() => {
      (events.onDocumentStateChange as (event: { data: boolean }) => void)({ data: true });
    });

    expect(onStateChange).toHaveBeenCalledWith(
      expect.objectContaining({
        status: "dirty",
        message: "Word文档存在未提交更改"
      })
    );

    act(() => {
      (events.onDocumentStateChange as (event: { data: boolean }) => void)({ data: false });
    });

    expect(onStateChange).toHaveBeenCalledWith(
      expect.objectContaining({
        status: "ready",
        message: "Word文档已同步",
        shouldRefreshMetadata: true
      })
    );

    unmount();

    expect(destroyEditor).toHaveBeenCalledTimes(1);
  });

  it("shows explicit permission/session hint for unauthorized errors", async () => {
    render(
      <OnlyOfficeEditorPane
        documentId="doc-office-2"
        documentTitle="季度计划"
        documentFormat="docx"
        editConfig={null}
        isConfigLoading={false}
        errorMessage="获取 ONLYOFFICE 编辑配置失败：401 Unauthorized"
      />
    );

    await waitFor(() => {
      expect(screen.getByText("登录会话已过期，请刷新页面后重新登录。")).toBeInTheDocument();
    });
  });

  it("shows explicit hint when onlyoffice script loading fails", async () => {
    render(
      <OnlyOfficeEditorPane
        documentId="doc-office-3"
        documentTitle="季度计划"
        documentFormat="xlsx"
        editConfig={null}
        isConfigLoading={false}
        errorMessage="加载 ONLYOFFICE 脚本失败"
      />
    );

    await waitFor(() => {
      expect(screen.getByText("ONLYOFFICE 脚本加载失败，请检查 Document Server 连接后重试。")).toBeInTheDocument();
    });
  });

  it("maps onlyoffice rights error to permission denied hint", async () => {
    render(
      <OnlyOfficeEditorPane
        documentId="doc-office-4"
        documentTitle="季度计划"
        documentFormat="docx"
        editConfig={null}
        isConfigLoading={false}
        errorMessage="You are trying to perform an action you do not have rights for."
      />
    );

    await waitFor(() => {
      expect(screen.getByText("当前账号没有该文档的编辑权限。")).toBeInTheDocument();
    });
  });
});
