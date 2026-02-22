import { AlertCircle, House, RotateCcw } from "lucide-react";

interface EditorAccessErrorPageProps {
  spaceId: string | null;
  description: string;
  technicalMessage?: string | null;
  onRetry: () => void;
  onBackHome: () => void;
}

// 编辑器访问失败页：用于空间不存在或无权限时的统一简约兜底视图。
export function EditorAccessErrorPage({
  spaceId,
  description,
  technicalMessage,
  onRetry,
  onBackHome
}: EditorAccessErrorPageProps) {
  return (
    <section className="editor-access-error-page" role="alert" aria-live="polite">
      <div className="editor-access-error-card">
        <div className="editor-access-error-icon" aria-hidden="true">
          <AlertCircle size={20} />
        </div>
        <h1 className="editor-access-error-title">无法访问该空间</h1>
        <p className="editor-access-error-description">{description}</p>
        {spaceId ? (
          <p className="editor-access-error-meta">
            空间 ID：<code>{spaceId}</code>
          </p>
        ) : null}
        {technicalMessage ? (
          <p className="editor-access-error-meta">
            详情：<code>{technicalMessage}</code>
          </p>
        ) : null}
        <div className="editor-access-error-actions">
          <button
            type="button"
            className="editor-access-error-button editor-access-error-button--primary"
            onClick={onRetry}
          >
            <RotateCcw size={14} />
            <span>重新校验</span>
          </button>
          <button
            type="button"
            className="editor-access-error-button"
            onClick={onBackHome}
          >
            <House size={14} />
            <span>返回首页</span>
          </button>
        </div>
      </div>
    </section>
  );
}
