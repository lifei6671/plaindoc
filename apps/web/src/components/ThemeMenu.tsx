import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { resolvePreviewTheme, type PreviewThemeTemplate } from "../preview-themes";
import { StyleDetailsDrawer } from "./StyleDetailsDrawer";

// 主题菜单组件入参：由父组件提供当前主题和切换回调。
interface ThemeMenuProps {
  themes: PreviewThemeTemplate[];
  activeThemeId: string;
  onSelectTheme: (themeId: string) => void;
  customPreviewStyleText: string;
}

// 独立主题菜单：开关状态内聚在子组件中，避免影响整页渲染。
export const ThemeMenu = memo(function ThemeMenu({
  themes,
  activeThemeId,
  onSelectTheme,
  customPreviewStyleText
}: ThemeMenuProps) {
  // 菜单展开状态仅影响当前子树，不触发父组件重渲染。
  const [isThemeMenuOpen, setIsThemeMenuOpen] = useState(false);
  // 样式详情抽屉对应的主题 ID；为空表示抽屉关闭。
  const [detailsThemeId, setDetailsThemeId] = useState<string | null>(null);
  // 菜单根节点引用：用于判断点击是否发生在菜单外部。
  const themeMenuRef = useRef<HTMLDivElement | null>(null);

  // 当前主题文案显示。
  const activeTheme = useMemo(() => {
    const foundTheme = themes.find((theme) => theme.id === activeThemeId);
    return foundTheme ?? themes[0];
  }, [themes, activeThemeId]);

  // 切换主题菜单显示状态。
  const toggleThemeMenu = useCallback(() => {
    setIsThemeMenuOpen((previous) => !previous);
  }, []);

  // 应用选中的主题并关闭菜单。
  const applyTheme = useCallback(
    (themeId: string) => {
      onSelectTheme(themeId);
      setIsThemeMenuOpen(false);
    },
    [onSelectTheme]
  );

  // 当前抽屉展示的主题对象。
  const detailsTheme = useMemo(
    () => (detailsThemeId ? resolvePreviewTheme(detailsThemeId) : null),
    [detailsThemeId]
  );

  // 打开指定主题的样式详情抽屉。
  const openThemeDetails = useCallback((themeId: string) => {
    const targetTheme = resolvePreviewTheme(themeId);
    setDetailsThemeId(targetTheme.id);
    setIsThemeMenuOpen(false);
  }, []);

  // 关闭样式详情抽屉。
  const closeStyleDetailsDrawer = useCallback(() => {
    setDetailsThemeId(null);
  }, []);

  // 主题菜单弹出时监听外部点击与 ESC，提升交互可控性。
  useEffect(() => {
    if (!isThemeMenuOpen) {
      return;
    }

    const onWindowMouseDown = (event: MouseEvent) => {
      const menuRootElement = themeMenuRef.current;
      if (!menuRootElement) {
        return;
      }
      if (event.target instanceof Node && menuRootElement.contains(event.target)) {
        return;
      }
      setIsThemeMenuOpen(false);
    };

    const onWindowKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setIsThemeMenuOpen(false);
      }
    };

    window.addEventListener("mousedown", onWindowMouseDown);
    window.addEventListener("keydown", onWindowKeyDown);
    return () => {
      window.removeEventListener("mousedown", onWindowMouseDown);
      window.removeEventListener("keydown", onWindowKeyDown);
    };
  }, [isThemeMenuOpen]);

  return (
    <>
      <div className="theme-menu" ref={themeMenuRef}>
        <button
          type="button"
          className="theme-menu__trigger"
          aria-label="选择预览主题"
          aria-haspopup="listbox"
          aria-expanded={isThemeMenuOpen}
          onClick={toggleThemeMenu}
        >
          <span className="theme-menu__trigger-label">主题</span>
          <span className="theme-menu__trigger-value">{activeTheme.name}</span>
        </button>
        {isThemeMenuOpen ? (
          <ul className="theme-menu__dropdown" role="listbox" aria-label="预览主题列表">
            {themes.map((themeTemplate) => {
              const isActiveTheme = themeTemplate.id === activeTheme.id;
              return (
                <li key={themeTemplate.id} className="theme-menu__item-row">
                  <button
                    type="button"
                    role="option"
                    aria-selected={isActiveTheme}
                    className={`theme-menu__item ${isActiveTheme ? "theme-menu__item--active" : ""}`}
                    onClick={() => applyTheme(themeTemplate.id)}
                  >
                    <span className="theme-menu__item-name">{themeTemplate.name}</span>
                    <span className="theme-menu__item-description">{themeTemplate.description}</span>
                  </button>
                  <button
                    type="button"
                    className="theme-menu__details-button"
                    aria-label={`查看 ${themeTemplate.name} 样式详情`}
                    onClick={() => openThemeDetails(themeTemplate.id)}
                  >
                    查看
                  </button>
                </li>
              );
            })}
          </ul>
        ) : null}
      </div>
      <StyleDetailsDrawer
        theme={detailsTheme}
        customPreviewStyleText={customPreviewStyleText}
        onClose={closeStyleDetailsDrawer}
      />
    </>
  );
});
