import { Check, Pencil, Plus, Trash2, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEventHandler } from "react";
import { createPortal } from "react-dom";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import type { AdminSpaceCategory } from "../../data-access";

interface AdminSpaceCategoriesDialogProps {
  open: boolean;
  loading: boolean;
  categories: AdminSpaceCategory[];
  onOpenChange: (open: boolean) => void;
  onCreate: (name: string) => Promise<void>;
  onRename: (category: AdminSpaceCategory, name: string) => Promise<void>;
  onDelete: (category: AdminSpaceCategory) => Promise<void>;
}

export function AdminSpaceCategoriesDialog({
  open,
  loading,
  categories,
  onOpenChange,
  onCreate,
  onRename,
  onDelete
}: AdminSpaceCategoriesDialogProps) {
  const [newName, setNewName] = useState("");
  const [editingCategoryID, setEditingCategoryID] = useState("");
  const [editingName, setEditingName] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!open) {
      return;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        onOpenChange(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      document.body.style.overflow = previousOverflow;
    };
  }, [onOpenChange, open]);

  const sortedCategories = useMemo(
    () =>
      [...categories].sort((left, right) => {
        if (left.isDefault !== right.isDefault) {
          return left.isDefault ? -1 : 1;
        }
        return left.name.localeCompare(right.name, "zh-CN");
      }),
    [categories]
  );

  const closeDialog = useCallback(() => {
    if (loading || submitting) {
      return;
    }
    setNewName("");
    setEditingCategoryID("");
    setEditingName("");
    onOpenChange(false);
  }, [loading, onOpenChange, submitting]);

  const handleCreate = useCallback(async () => {
    const name = newName.trim();
    if (!name) {
      return;
    }
    setSubmitting(true);
    try {
      await onCreate(name);
      setNewName("");
    } finally {
      setSubmitting(false);
    }
  }, [newName, onCreate]);

  const handleCreateSubmit = useCallback<FormEventHandler<HTMLFormElement>>(
    (event) => {
      event.preventDefault();
      void handleCreate();
    },
    [handleCreate]
  );

  const startRename = useCallback((category: AdminSpaceCategory) => {
    setEditingCategoryID(category.categoryId);
    setEditingName(category.name);
  }, []);

  const handleRename = useCallback(
    async (category: AdminSpaceCategory) => {
      const nextName = editingName.trim();
      if (!nextName) {
        return;
      }
      setSubmitting(true);
      try {
        await onRename(category, nextName);
        setEditingCategoryID("");
        setEditingName("");
      } finally {
        setSubmitting(false);
      }
    },
    [editingName, onRename]
  );

  const handleDelete = useCallback(
    async (category: AdminSpaceCategory) => {
      setSubmitting(true);
      try {
        await onDelete(category);
        if (editingCategoryID === category.categoryId) {
          setEditingCategoryID("");
          setEditingName("");
        }
      } finally {
        setSubmitting(false);
      }
    },
    [editingCategoryID, onDelete]
  );

  if (!open) {
    return null;
  }

  return createPortal(
    <div
      className="fixed inset-0 z-[2800] flex items-center justify-center bg-slate-900/45 p-4 backdrop-blur-sm"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) {
          closeDialog();
        }
      }}
    >
      <section className="grid max-h-[88vh] w-full max-w-[760px] grid-rows-[auto_auto_minmax(0,1fr)] overflow-hidden rounded-xl border border-slate-200 bg-white shadow-[0_24px_64px_rgba(15,23,42,0.28)]">
        <header className="flex items-start justify-between border-b border-slate-200 bg-gradient-to-r from-slate-50 to-sky-50 px-5 py-4">
          <div className="space-y-1">
            <h3 className="text-lg font-semibold text-slate-900">空间分类管理</h3>
            <p className="text-xs text-slate-600">支持新增、重命名、删除；删除分类会把空间自动迁移到“未分类”。</p>
          </div>
          <Button type="button" size="icon" variant="ghost" className="h-8 w-8" disabled={loading || submitting} onClick={closeDialog}>
            <X size={16} />
          </Button>
        </header>

        <form className="flex items-end gap-2 border-b border-slate-200 bg-slate-50/70 px-5 py-3" onSubmit={handleCreateSubmit}>
          <label className="grid flex-1 gap-1">
            <span className="text-xs font-semibold text-slate-700">新增分类</span>
            <Input
              value={newName}
              maxLength={40}
              placeholder="输入分类名称（最多 40 字）"
              disabled={loading || submitting}
              onChange={(event) => setNewName(event.target.value)}
            />
          </label>
          <Button type="submit" disabled={loading || submitting || newName.trim().length === 0}>
            <Plus size={14} />
            新增
          </Button>
        </form>

        <div className="overflow-y-auto px-5 py-4">
          {sortedCategories.length === 0 ? (
            <p className="rounded-md border border-dashed border-slate-300 bg-slate-50 px-4 py-8 text-center text-sm text-slate-500">暂无分类</p>
          ) : (
            <div className="space-y-2">
              {sortedCategories.map((category) => {
                const editing = editingCategoryID === category.categoryId;
                return (
                  <div key={category.categoryId} className="flex items-center gap-2 rounded-md border border-slate-200 bg-white px-3 py-2">
                    <div className="min-w-0 flex-1">
                      {editing ? (
                        <Input
                          value={editingName}
                          maxLength={40}
                          disabled={loading || submitting}
                          onChange={(event) => setEditingName(event.target.value)}
                        />
                      ) : (
                        <div className="flex min-w-0 items-center gap-2">
                          <span className="truncate text-sm font-medium text-slate-900">{category.name}</span>
                          {category.isDefault ? (
                            <Badge variant="secondary" className="whitespace-nowrap">
                              默认
                            </Badge>
                          ) : null}
                        </div>
                      )}
                      <p className="mt-1 truncate font-mono text-[11px] text-slate-500">{category.categoryId}</p>
                    </div>

                    {editing ? (
                      <>
                        <Button
                          type="button"
                          size="sm"
                          disabled={loading || submitting || editingName.trim().length === 0}
                          onClick={() => void handleRename(category)}
                        >
                          <Check size={14} />
                          保存
                        </Button>
                        <Button
                          type="button"
                          size="sm"
                          variant="outline"
                          disabled={loading || submitting}
                          onClick={() => {
                            setEditingCategoryID("");
                            setEditingName("");
                          }}
                        >
                          取消
                        </Button>
                      </>
                    ) : (
                      <>
                        <Button type="button" size="sm" variant="outline" disabled={loading || submitting || category.isDefault} onClick={() => startRename(category)}>
                          <Pencil size={14} />
                          重命名
                        </Button>
                        <Button
                          type="button"
                          size="sm"
                          variant="destructive"
                          disabled={loading || submitting || category.isDefault}
                          onClick={() => void handleDelete(category)}
                        >
                          <Trash2 size={14} />
                          删除
                        </Button>
                      </>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </section>
    </div>,
    document.body
  );
}
