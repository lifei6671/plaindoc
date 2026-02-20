import { useCallback, useEffect, useMemo, useRef, useState, type FormEventHandler, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { Textarea } from "../../components/ui/textarea";

type AdminDialogTone = "default" | "warning" | "danger";

export interface AdminConfirmOptions {
  title: string;
  description?: string;
  confirmText?: string;
  cancelText?: string;
  tone?: AdminDialogTone;
}

export interface AdminPromptFieldOption {
  value: string;
  label: string;
}

export interface AdminPromptField {
  key: string;
  label: string;
  type?: "text" | "textarea" | "select";
  placeholder?: string;
  required?: boolean;
  description?: string;
  rows?: number;
  defaultValue?: string;
  options?: AdminPromptFieldOption[];
}

export interface AdminPromptOptions extends AdminConfirmOptions {
  fields: AdminPromptField[];
}

interface InternalConfirmState extends Required<Pick<AdminConfirmOptions, "confirmText" | "cancelText" | "tone">> {
  title: string;
  description: string;
  resolve: (result: boolean) => void;
}

interface InternalPromptState extends Required<Pick<AdminPromptOptions, "confirmText" | "cancelText" | "tone">> {
  title: string;
  description: string;
  fields: AdminPromptField[];
  requestKey: number;
  resolve: (result: Record<string, string> | null) => void;
}

interface AdminDialogLayerProps {
  open: boolean;
  title: string;
  description?: string;
  onClose: () => void;
  footer: ReactNode;
  children?: ReactNode;
}

function AdminDialogLayer({ open, title, description, onClose, footer, children }: AdminDialogLayerProps) {
  useEffect(() => {
    if (!open) {
      return;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        onClose();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [onClose, open]);

  useEffect(() => {
    if (!open) {
      return;
    }
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = previousOverflow;
    };
  }, [open]);

  if (!open) {
    return null;
  }

  return createPortal(
    <div className="admin-dialog-layer" onMouseDown={(event) => {
      if (event.target === event.currentTarget) {
        onClose();
      }
    }}>
      <section className="admin-dialog" aria-modal="true" role="dialog" aria-label={title}>
        <header className="admin-dialog__header">
          <h3>{title}</h3>
          {description ? <p>{description}</p> : null}
        </header>
        {children ? <div className="admin-dialog__content">{children}</div> : null}
        <footer className="admin-dialog__footer">{footer}</footer>
      </section>
    </div>,
    document.body
  );
}

function createPromptInitialValues(fields: AdminPromptField[]): Record<string, string> {
  const values: Record<string, string> = {};
  for (const field of fields) {
    values[field.key] = field.defaultValue ?? "";
  }
  return values;
}

export function useAdminDialogs() {
  const [confirmState, setConfirmState] = useState<InternalConfirmState | null>(null);
  const [promptState, setPromptState] = useState<InternalPromptState | null>(null);
  const promptRequestCounter = useRef(0);

  const [promptValues, setPromptValues] = useState<Record<string, string>>({});
  const [promptErrors, setPromptErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    if (!promptState) {
      setPromptValues({});
      setPromptErrors({});
      return;
    }
    setPromptValues(createPromptInitialValues(promptState.fields));
    setPromptErrors({});
  }, [promptState]);

  const closeConfirm = useCallback((result: boolean) => {
    setConfirmState((previousState) => {
      if (!previousState) {
        return previousState;
      }
      previousState.resolve(result);
      return null;
    });
  }, []);

  const closePrompt = useCallback((result: Record<string, string> | null) => {
    setPromptState((previousState) => {
      if (!previousState) {
        return previousState;
      }
      previousState.resolve(result);
      return null;
    });
  }, []);

  const confirm = useCallback((options: AdminConfirmOptions) => {
    return new Promise<boolean>((resolve) => {
      setConfirmState((previousState) => {
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

  const prompt = useCallback((options: AdminPromptOptions) => {
    const nextRequestKey = promptRequestCounter.current + 1;
    promptRequestCounter.current = nextRequestKey;
    return new Promise<Record<string, string> | null>((resolve) => {
      setPromptState((previousState) => {
        if (previousState) {
          previousState.resolve(null);
        }
        return {
          title: options.title,
          description: options.description ?? "",
          fields: options.fields,
          requestKey: nextRequestKey,
          confirmText: options.confirmText ?? "确认",
          cancelText: options.cancelText ?? "取消",
          tone: options.tone ?? "default",
          resolve
        };
      });
    });
  }, []);

  const handlePromptSubmit = useCallback<FormEventHandler<HTMLFormElement>>((event) => {
    event.preventDefault();
    if (!promptState) {
      return;
    }

    const nextErrors: Record<string, string> = {};
    for (const field of promptState.fields) {
      if (!field.required) {
        continue;
      }
      const value = promptValues[field.key] ?? "";
      if (!value.trim()) {
        nextErrors[field.key] = `${field.label}不能为空`;
      }
    }
    setPromptErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) {
      return;
    }

    closePrompt(promptValues);
  }, [closePrompt, promptState, promptValues]);

  const dialogs = useMemo(() => {
    return (
      <>
        <AdminDialogLayer
          open={confirmState !== null}
          title={confirmState?.title ?? ""}
          description={confirmState?.description}
          onClose={() => closeConfirm(false)}
          footer={
            <>
              <Button type="button" size="sm" variant="outline" onClick={() => closeConfirm(false)}>
                {confirmState?.cancelText ?? "取消"}
              </Button>
              <Button
                type="button"
                size="sm"
                variant={confirmState?.tone === "danger" ? "destructive" : confirmState?.tone === "warning" ? "secondary" : "default"}
                onClick={() => closeConfirm(true)}
              >
                {confirmState?.confirmText ?? "确认"}
              </Button>
            </>
          }
        />

        <AdminDialogLayer
          open={promptState !== null}
          title={promptState?.title ?? ""}
          description={promptState?.description}
          onClose={() => closePrompt(null)}
          footer={
            <>
              <Button type="button" size="sm" variant="outline" onClick={() => closePrompt(null)}>
                {promptState?.cancelText ?? "取消"}
              </Button>
              <Button
                type="submit"
                form="admin-dialog-prompt-form"
                size="sm"
                variant={promptState?.tone === "danger" ? "destructive" : promptState?.tone === "warning" ? "secondary" : "default"}
              >
                {promptState?.confirmText ?? "确认"}
              </Button>
            </>
          }
        >
          <form id="admin-dialog-prompt-form" className="admin-dialog-form" onSubmit={handlePromptSubmit} key={promptState?.requestKey ?? 0}>
            {(promptState?.fields ?? []).map((field) => {
              const value = promptValues[field.key] ?? "";
              return (
                <label className="admin-dialog-form__field" key={field.key}>
                  <span>{field.label}</span>
                  {field.type === "textarea" ? (
                    <Textarea
                      rows={field.rows ?? 5}
                      value={value}
                      placeholder={field.placeholder}
                      onChange={(event) => {
                        const nextValue = event.target.value;
                        setPromptValues((previousValues) => ({
                          ...previousValues,
                          [field.key]: nextValue
                        }));
                        setPromptErrors((previousErrors) => {
                          if (!previousErrors[field.key]) {
                            return previousErrors;
                          }
                          const nextErrors = { ...previousErrors };
                          delete nextErrors[field.key];
                          return nextErrors;
                        });
                      }}
                    />
                  ) : field.type === "select" ? (
                    <Select
                      value={
                        value === ""
                          ? (field.options ?? []).some((option) => option.value === "")
                            ? "__PD_EMPTY_VALUE__"
                            : undefined
                          : value
                      }
                      onValueChange={(nextRawValue) => {
                        const nextValue = nextRawValue === "__PD_EMPTY_VALUE__" ? "" : nextRawValue;
                        setPromptValues((previousValues) => ({
                          ...previousValues,
                          [field.key]: nextValue
                        }));
                        setPromptErrors((previousErrors) => {
                          if (!previousErrors[field.key]) {
                            return previousErrors;
                          }
                          const nextErrors = { ...previousErrors };
                          delete nextErrors[field.key];
                          return nextErrors;
                        });
                      }}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder={field.placeholder || `请选择${field.label}`} />
                      </SelectTrigger>
                      <SelectContent>
                        {(field.options ?? []).map((option) => (
                          <SelectItem
                            key={option.value || "__PD_EMPTY_VALUE__"}
                            value={option.value === "" ? "__PD_EMPTY_VALUE__" : option.value}
                          >
                            {option.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  ) : (
                    <Input
                      type="text"
                      value={value}
                      placeholder={field.placeholder}
                      onChange={(event) => {
                        const nextValue = event.target.value;
                        setPromptValues((previousValues) => ({
                          ...previousValues,
                          [field.key]: nextValue
                        }));
                        setPromptErrors((previousErrors) => {
                          if (!previousErrors[field.key]) {
                            return previousErrors;
                          }
                          const nextErrors = { ...previousErrors };
                          delete nextErrors[field.key];
                          return nextErrors;
                        });
                      }}
                    />
                  )}
                  {field.description ? <small>{field.description}</small> : null}
                  {promptErrors[field.key] ? <em>{promptErrors[field.key]}</em> : null}
                </label>
              );
            })}
          </form>
        </AdminDialogLayer>
      </>
    );
  }, [closeConfirm, closePrompt, confirmState, handlePromptSubmit, promptErrors, promptState, promptValues]);

  return {
    confirm,
    prompt,
    dialogs
  };
}
