import { useEffect, useMemo, useRef, useState } from "react";
import CodeMirror from "@uiw/react-codemirror";
import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import { languages } from "@codemirror/language-data";
import { EditorView } from "@codemirror/view";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { ConflictError, getDataGateway, type TreeNode } from "./data-access";

const fallbackContent = `# PlainDoc

加载中...
`;

type SaveStatus = "loading" | "ready" | "saving" | "saved" | "conflict" | "error";

function findFirstDocId(nodes: TreeNode[]): string | null {
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

function formatError(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return "未知错误";
}

export default function App() {
  const dataGateway = useMemo(() => getDataGateway(), []);
  const [content, setContent] = useState(fallbackContent);
  const [activeDocId, setActiveDocId] = useState<string | null>(null);
  const [baseVersion, setBaseVersion] = useState(0);
  const [lastSavedContent, setLastSavedContent] = useState(fallbackContent);
  const [saveStatus, setSaveStatus] = useState<SaveStatus>("loading");
  const [statusMessage, setStatusMessage] = useState("初始化中...");
  const editorScrollerRef = useRef<HTMLElement | null>(null);
  const previewScrollerRef = useRef<HTMLElement | null>(null);
  const syncingRef = useRef(false);
  const lastScrollSourceRef = useRef<"editor" | "preview">("editor");

  const getRatio = (element: HTMLElement): number => {
    const maxScrollable = element.scrollHeight - element.clientHeight;
    if (maxScrollable <= 0) {
      return 0;
    }
    return element.scrollTop / maxScrollable;
  };

  const applyRatio = (element: HTMLElement, ratio: number): void => {
    const maxScrollable = element.scrollHeight - element.clientHeight;
    if (maxScrollable <= 0) {
      element.scrollTop = 0;
      return;
    }
    element.scrollTop = Math.max(0, Math.min(maxScrollable, ratio * maxScrollable));
  };

  const extensions = useMemo(
    () => [
      EditorView.lineWrapping,
      markdown({
        base: markdownLanguage,
        codeLanguages: languages
      })
    ],
    []
  );

  useEffect(() => {
    let cancelled = false;

    const bootstrap = async () => {
      try {
        const spaces = await dataGateway.workspace.listSpaces();
        const space =
          spaces[0] ??
          (await dataGateway.workspace.createSpace({
            name: "默认空间"
          }));
        const tree = await dataGateway.workspace.getTree(space.id);
        const existingDocId = findFirstDocId(tree);
        const docId =
          existingDocId ??
          (
            await dataGateway.workspace.createNode({
              spaceId: space.id,
              parentId: null,
              type: "doc",
              title: "未命名文档"
            })
          ).docId;

        if (!docId) {
          throw new Error("无法创建初始化文档");
        }

        const document = await dataGateway.document.getDocument(docId);
        if (cancelled) {
          return;
        }

        setActiveDocId(document.id);
        setBaseVersion(document.version);
        setContent(document.contentMd);
        setLastSavedContent(document.contentMd);
        setSaveStatus("ready");
        setStatusMessage(`已加载文档 v${document.version}`);
      } catch (error) {
        if (cancelled) {
          return;
        }
        setSaveStatus("error");
        setStatusMessage(`加载失败：${formatError(error)}`);
      }
    };

    void bootstrap();

    return () => {
      cancelled = true;
    };
  }, [dataGateway]);

  useEffect(() => {
    if (
      !activeDocId ||
      content === lastSavedContent ||
      saveStatus === "loading" ||
      saveStatus === "saving"
    ) {
      return;
    }

    const timer = window.setTimeout(async () => {
      setSaveStatus("saving");
      setStatusMessage("保存中...");
      try {
        const result = await dataGateway.document.saveDocument({
          docId: activeDocId,
          contentMd: content,
          baseVersion
        });
        setBaseVersion(result.document.version);
        setLastSavedContent(result.document.contentMd);
        setSaveStatus("saved");
        setStatusMessage(`已保存 v${result.document.version}`);
      } catch (error) {
        if (error instanceof ConflictError) {
          setSaveStatus("conflict");
          setStatusMessage(
            `检测到冲突：当前基线 v${baseVersion}，最新版本 v${error.latestDocument.version}`
          );
          return;
        }
        setSaveStatus("error");
        setStatusMessage(`保存失败：${formatError(error)}`);
      }
    }, 800);

    return () => {
      window.clearTimeout(timer);
    };
  }, [activeDocId, baseVersion, content, dataGateway, lastSavedContent, saveStatus]);

  useEffect(() => {
    const editorElement = editorScrollerRef.current;
    const previewElement = previewScrollerRef.current;
    if (!editorElement || !previewElement) {
      return;
    }

    const syncByRatio = (source: HTMLElement, target: HTMLElement, sourceName: "editor" | "preview") => {
      if (syncingRef.current) {
        return;
      }
      syncingRef.current = true;
      lastScrollSourceRef.current = sourceName;
      applyRatio(target, getRatio(source));
      requestAnimationFrame(() => {
        syncingRef.current = false;
      });
    };

    const onEditorScroll = () => {
      syncByRatio(editorElement, previewElement, "editor");
    };

    const onPreviewScroll = () => {
      syncByRatio(previewElement, editorElement, "preview");
    };

    const onPreviewImageLoad = (event: Event) => {
      const target = event.target as HTMLElement | null;
      if (!target || target.tagName !== "IMG") {
        return;
      }
      if (syncingRef.current) {
        return;
      }
      syncingRef.current = true;
      if (lastScrollSourceRef.current === "editor") {
        applyRatio(previewElement, getRatio(editorElement));
      } else {
        applyRatio(editorElement, getRatio(previewElement));
      }
      requestAnimationFrame(() => {
        syncingRef.current = false;
      });
    };

    editorElement.addEventListener("scroll", onEditorScroll, { passive: true });
    previewElement.addEventListener("scroll", onPreviewScroll, { passive: true });
    previewElement.addEventListener("load", onPreviewImageLoad, true);

    return () => {
      editorElement.removeEventListener("scroll", onEditorScroll);
      previewElement.removeEventListener("scroll", onPreviewScroll);
      previewElement.removeEventListener("load", onPreviewImageLoad, true);
    };
  }, [activeDocId]);

  const syncLatestVersion = async () => {
    if (!activeDocId) {
      return;
    }
    try {
      const latestDocument = await dataGateway.document.getDocument(activeDocId);
      setContent(latestDocument.contentMd);
      setBaseVersion(latestDocument.version);
      setLastSavedContent(latestDocument.contentMd);
      setSaveStatus("ready");
      setStatusMessage(`已同步到最新版本 v${latestDocument.version}`);
    } catch (error) {
      setSaveStatus("error");
      setStatusMessage(`同步失败：${formatError(error)}`);
    }
  };

  return (
    <div className="page">
      <header className="header">
        <h1>PlainDoc</h1>
        <span>{statusMessage}</span>
      </header>
      <main className="workspace">
        <section className="pane editor-pane">
          <CodeMirror
            value={content}
            extensions={extensions}
            height="100%"
            onCreateEditor={(view) => {
              editorScrollerRef.current = view.scrollDOM;
            }}
            onChange={(value) => {
              setContent(value);
              if (saveStatus !== "loading") {
                setSaveStatus("ready");
              }
            }}
            basicSetup={{
              lineNumbers: false,
              foldGutter: false
            }}
          />
        </section>
        <section
          className="pane preview-pane"
          ref={(node) => {
            previewScrollerRef.current = node;
          }}
        >
          <article className="markdown-body">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
          </article>
        </section>
      </main>
      {saveStatus === "conflict" ? (
        <footer className="conflict-footer">
          <span>当前文档存在版本冲突，请先同步最新版本后再手动合并。</span>
          <button type="button" onClick={() => void syncLatestVersion()}>
            同步最新版本
          </button>
        </footer>
      ) : null}
    </div>
  );
}
