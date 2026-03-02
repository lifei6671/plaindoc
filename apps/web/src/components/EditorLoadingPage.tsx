import { LoaderCircle } from "lucide-react";

interface EditorLoadingPageProps {
  title?: string;
  description?: string;
  detail?: string | null;
}

export function EditorLoadingPage({
  title = "正在准备编辑器",
  description = "首次加载会稍慢，请稍候...",
  detail
}: EditorLoadingPageProps) {
  return (
    <div className="editor-loading-page" role="status" aria-live="polite" aria-busy="true">
      <div className="editor-loading-page__panel">
        <div className="editor-loading-page__icon-wrap" aria-hidden="true">
          <LoaderCircle size={20} className="editor-loading-page__icon" />
        </div>
        <h1 className="editor-loading-page__title">{title}</h1>
        <p className="editor-loading-page__description">{description}</p>
        {detail ? <p className="editor-loading-page__detail">{detail}</p> : null}
      </div>
    </div>
  );
}
