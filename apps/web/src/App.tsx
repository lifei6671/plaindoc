import { useMemo, useState } from "react";
import CodeMirror from "@uiw/react-codemirror";
import { markdown, markdownLanguage } from "@codemirror/lang-markdown";
import { languages } from "@codemirror/language-data";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

const initialContent = `# PlainDoc

这是工程初始化版本。

## 已接入

- React + Vite + TypeScript
- CodeMirror 6
- Markdown 实时预览

\`\`\`mermaid
flowchart TD
  A[后端 Gin] --> B[API]
  C[前端 React] --> B
\`\`\`
`;

export default function App() {
  const [content, setContent] = useState(initialContent);

  const extensions = useMemo(
    () => [
      markdown({
        base: markdownLanguage,
        codeLanguages: languages
      })
    ],
    []
  );

  return (
    <div className="page">
      <header className="header">
        <h1>PlainDoc</h1>
        <span>Monorepo Init</span>
      </header>
      <main className="workspace">
        <section className="pane">
          <div className="pane-title">编辑区</div>
          <div className="pane-body editor-pane">
            <CodeMirror
              value={content}
              extensions={extensions}
              height="100%"
              onChange={setContent}
              basicSetup={{
                lineNumbers: true,
                foldGutter: true
              }}
            />
          </div>
        </section>
        <section className="pane">
          <div className="pane-title">预览区</div>
          <article className="pane-body markdown-body">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
          </article>
        </section>
      </main>
    </div>
  );
}
