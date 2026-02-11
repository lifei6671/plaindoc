import { memo, useCallback, useEffect, useRef, useState } from "react";
import type { TocItem } from "../editor/types";

// 目录菜单组件入参：由父组件提供 TOC 数据和跳转行为。
interface TocMenuProps {
  items: TocItem[];
  onSelectItem: (item: TocItem) => void;
}

// 目录菜单：仅控制自身开关状态，目录点击后导航到对应标题。
export const TocMenu = memo(function TocMenu({ items, onSelectItem }: TocMenuProps) {
  // 目录菜单展开状态独立维护，避免影响主视图。
  const [isTocMenuOpen, setIsTocMenuOpen] = useState(false);
  // 菜单根节点引用：用于判断是否点击了菜单外部。
  const tocMenuRef = useRef<HTMLDivElement | null>(null);
  // 目录是否为空。
  const hasItems = items.length > 0;

  // 切换目录菜单显示状态。
  const toggleTocMenu = useCallback(() => {
    setIsTocMenuOpen((previous) => !previous);
  }, []);

  // 选择目录条目并关闭菜单。
  const handleSelectItem = useCallback(
    (item: TocItem) => {
      onSelectItem(item);
      setIsTocMenuOpen(false);
    },
    [onSelectItem]
  );

  // 目录菜单弹出时监听外部点击与 ESC，保证交互一致性。
  useEffect(() => {
    if (!isTocMenuOpen) {
      return;
    }

    const onWindowMouseDown = (event: MouseEvent) => {
      const menuRootElement = tocMenuRef.current;
      if (!menuRootElement) {
        return;
      }
      if (event.target instanceof Node && menuRootElement.contains(event.target)) {
        return;
      }
      setIsTocMenuOpen(false);
    };

    const onWindowKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setIsTocMenuOpen(false);
      }
    };

    window.addEventListener("mousedown", onWindowMouseDown);
    window.addEventListener("keydown", onWindowKeyDown);
    return () => {
      window.removeEventListener("mousedown", onWindowMouseDown);
      window.removeEventListener("keydown", onWindowKeyDown);
    };
  }, [isTocMenuOpen]);

  return (
    <div className="toc-menu" ref={tocMenuRef}>
      <button
        type="button"
        className="toc-menu__trigger"
        aria-label="打开目录"
        aria-haspopup="listbox"
        aria-expanded={isTocMenuOpen}
        onClick={toggleTocMenu}
      >
        <span className="toc-menu__trigger-label">目录</span>
        <span className="toc-menu__trigger-value">{hasItems ? `${items.length} 项` : "暂无"}</span>
      </button>
      {isTocMenuOpen ? (
        <div className="toc-menu__dropdown">
          {hasItems ? (
            <ul className="toc-menu__list" role="listbox" aria-label="目录列表">
              {items.map((item) => (
                <li key={`${item.sourceLine}-${item.level}`} className="toc-menu__item-row">
                  <button
                    type="button"
                    role="option"
                    className="toc-menu__item"
                    title={item.text}
                    // 根据标题层级做视觉缩进，强化目录结构层次。
                    style={{ paddingLeft: `${10 + (item.level - 1) * 14}px` }}
                    onClick={() => handleSelectItem(item)}
                  >
                    {item.text}
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <p className="toc-menu__empty">当前文档暂无标题。</p>
          )}
        </div>
      ) : null}
    </div>
  );
});
