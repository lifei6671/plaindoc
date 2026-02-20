import { useCallback, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";

export type ConfirmDialogTone = "default" | "warning" | "danger";

export interface ConfirmDialogOptions {
  title: string;
  description?: string;
  confirmText?: string;
  cancelText?: string;
  tone?: ConfirmDialogTone;
}

interface ConfirmDialogState {
  title: string;
  description: string;
  confirmText: string;
  cancelText: string;
  tone: ConfirmDialogTone;
  resolve: (result: boolean) => void;
}

interface ConfirmDialogLayerProps {
  state: ConfirmDialogState | null;
  onResolve: (result: boolean) => void;
}

function ConfirmDialogLayer({ state, onResolve }: ConfirmDialogLayerProps) {
  useEffect(() => {
    if (!state) {
      return;
    }
    const handleWindowKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        onResolve(false);
      }
    };
    window.addEventListener("keydown", handleWindowKeyDown);
    return () => {
      window.removeEventListener("keydown", handleWindowKeyDown);
    };
  }, [onResolve, state]);

  useEffect(() => {
    if (!state) {
      return;
    }
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = previousOverflow;
    };
  }, [state]);

  if (!state) {
    return null;
  }

  return createPortal(
    <div
      className="confirm-dialog-layer"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) {
          onResolve(false);
        }
      }}
    >
      <section className="confirm-dialog" role="dialog" aria-modal="true" aria-label={state.title}>
        <header className="confirm-dialog__header">
          <h3>{state.title}</h3>
          {state.description ? <p>{state.description}</p> : null}
        </header>
        <footer className="confirm-dialog__footer">
          <button type="button" className="confirm-dialog__button" onClick={() => onResolve(false)}>
            {state.cancelText}
          </button>
          <button
            type="button"
            className={`confirm-dialog__button confirm-dialog__button--${state.tone}`}
            onClick={() => onResolve(true)}
          >
            {state.confirmText}
          </button>
        </footer>
      </section>
    </div>,
    document.body
  );
}

export function useConfirmDialog() {
  const [dialogState, setDialogState] = useState<ConfirmDialogState | null>(null);

  const resolveDialog = useCallback((result: boolean) => {
    setDialogState((previousState) => {
      if (!previousState) {
        return previousState;
      }
      previousState.resolve(result);
      return null;
    });
  }, []);

  const confirm = useCallback((options: ConfirmDialogOptions): Promise<boolean> => {
    return new Promise<boolean>((resolve) => {
      setDialogState((previousState) => {
        if (previousState) {
          previousState.resolve(false);
        }
        return {
          title: options.title,
          description: options.description ?? "",
          confirmText: options.confirmText ?? "确认",
          cancelText: options.cancelText ?? "取消",
          tone: options.tone ?? "default",
          resolve
        };
      });
    });
  }, []);

  const dialog = useMemo(() => <ConfirmDialogLayer state={dialogState} onResolve={resolveDialog} />, [dialogState, resolveDialog]);

  return {
    confirm,
    dialog
  };
}
