import { useEffect, type ReactNode } from "react";

export type TopToastVariant = "success" | "info" | "error";

// 顶部提示组件入参：支持图标、文案、自动关闭与样式变体。
interface TopToastProps {
  open: boolean;
  message: string;
  icon?: ReactNode;
  variant?: TopToastVariant;
  durationMs?: number;
  triggerKey?: number;
  onClose?: () => void;
}

// 顶部 Toast：用于“复制成功”等短提示，默认若干秒后自动消失。
export function TopToast({
  open,
  message,
  icon,
  variant = "success",
  durationMs = 2600,
  triggerKey = 0,
  onClose
}: TopToastProps) {
  // 每次打开或 triggerKey 变化时重新计时，确保连续触发也能完整展示。
  useEffect(() => {
    if (!open || !onClose) {
      return;
    }
    const timer = window.setTimeout(() => {
      onClose();
    }, durationMs);
    return () => {
      window.clearTimeout(timer);
    };
  }, [durationMs, onClose, open, triggerKey]);

  if (!open || !message) {
    return null;
  }

  return (
    <div className={`top-toast top-toast--${variant}`} role="status" aria-live="polite">
      {icon ? <span className="top-toast__icon">{icon}</span> : null}
      <span className="top-toast__message">{message}</span>
    </div>
  );
}
