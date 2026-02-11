import { memo, useEffect, useMemo } from "react";
import type { PreviewThemeTemplate } from "../preview-themes";
import { buildThemeCssTemplate } from "../editor/preview-style";

// 样式详情抽屉入参：由父组件传入当前主题与外部覆盖样式。
interface StyleDetailsDrawerProps {
  theme: PreviewThemeTemplate | null;
  customPreviewStyleText: string;
  onClose: () => void;
}

// 右侧样式详情抽屉：用于查看当前主题与覆盖样式细节。
export const StyleDetailsDrawer = memo(function StyleDetailsDrawer({
  theme,
  customPreviewStyleText,
  onClose
}: StyleDetailsDrawerProps) {
  // 仅当存在主题时才展示抽屉。
  const isOpen = Boolean(theme);
  // 当前主题 CSS 模板：用于复制后快速二次修改。
  const themeCssTemplate = useMemo(
    () => (theme ? buildThemeCssTemplate(theme) : ""),
    [theme]
  );

  // 抽屉打开时支持 ESC 快捷关闭。
  useEffect(() => {
    if (!isOpen) {
      return;
    }
    const onWindowKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };
    window.addEventListener("keydown", onWindowKeyDown);
    return () => {
      window.removeEventListener("keydown", onWindowKeyDown);
    };
  }, [isOpen, onClose]);

  if (!theme) {
    return null;
  }

  return (
    <div className="style-drawer-layer" role="dialog" aria-modal="true" aria-label="当前样式详情">
      <button
        type="button"
        className="style-drawer-backdrop"
        aria-label="关闭样式详情抽屉"
        onClick={onClose}
      />
      <aside className="style-drawer">
        <header className="style-drawer__header">
          <div className="style-drawer__header-copy">
            <h2>当前生效样式</h2>
            <p>已生成带注释 CSS 模板，可直接复制后修改。</p>
          </div>
          <button type="button" className="style-drawer__close" onClick={onClose}>
            关闭
          </button>
        </header>
        <div className="style-drawer__body">
          <section className="style-drawer-section">
            <h3>主题信息</h3>
            <dl className="style-drawer-kv">
              <dt>主题 ID</dt>
              <dd>{theme.id}</dd>
              <dt>主题名称</dt>
              <dd>{theme.name}</dd>
              <dt>主题描述</dt>
              <dd>{theme.description}</dd>
              <dt>高亮主题</dt>
              <dd>{theme.syntaxTheme}</dd>
            </dl>
          </section>

          <section className="style-drawer-section">
            <h3>主题 CSS 源码（含注释，可复制）</h3>
            <pre className="style-drawer-code">{themeCssTemplate}</pre>
          </section>

          <section className="style-drawer-section">
            <h3>外部覆盖样式</h3>
            {customPreviewStyleText ? (
              <pre className="style-drawer-code">{customPreviewStyleText}</pre>
            ) : (
              <p className="style-drawer-empty">当前没有外部覆盖样式。</p>
            )}
          </section>
        </div>
      </aside>
    </div>
  );
});
