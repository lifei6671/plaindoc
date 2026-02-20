import type { ReactNode } from "react";
import { Button } from "../../components/ui/button";
import { Card, CardContent } from "../../components/ui/card";
import { cn } from "../../lib/utils";

interface AdminPageCardProps {
  children: ReactNode;
  className?: string;
  contentClassName?: string;
}

export function AdminPageCard({ children, className, contentClassName }: AdminPageCardProps) {
  return (
    <Card className={cn("border-0 bg-transparent shadow-none", className)}>
      <CardContent className={cn("space-y-3.5 px-5 pb-5 pt-3", contentClassName)}>{children}</CardContent>
    </Card>
  );
}

interface AdminToolbarActionsProps {
  children: ReactNode;
  className?: string;
  actionsClassName?: string;
}

export function AdminToolbarActions({ children, className, actionsClassName }: AdminToolbarActionsProps) {
  return (
    <div className={cn("space-y-1.5 self-start", className)}>
      <span aria-hidden="true" className="invisible block text-xs font-semibold tracking-wide text-slate-600">
        操作
      </span>
      <div className={cn("flex h-9 flex-wrap items-center gap-2", actionsClassName)}>{children}</div>
    </div>
  );
}

interface AdminBulkActionBarProps {
  selectedCount: number;
  children: ReactNode;
  className?: string;
}

export function AdminBulkActionBar({ selectedCount, children, className }: AdminBulkActionBarProps) {
  return (
    <div className={cn("flex min-h-[52px] flex-wrap items-center justify-between gap-2 rounded-sm bg-slate-50 px-3 py-2", className)}>
      <p className="text-xs font-medium leading-5 text-slate-600">已选 {selectedCount} 项</p>
      <div className="flex flex-wrap items-center gap-2">{children}</div>
    </div>
  );
}

interface AdminTableContainerProps {
  children: ReactNode;
  className?: string;
  scrollClassName?: string;
}

export function AdminTableContainer({ children, className, scrollClassName }: AdminTableContainerProps) {
  return (
    <div className={cn("overflow-hidden rounded-sm border border-slate-200/70", className)}>
      <div className={cn("admin-table-scroll overflow-x-auto overflow-y-visible", scrollClassName)}>{children}</div>
    </div>
  );
}

interface AdminPaginationFooterProps {
  summary: ReactNode;
  previousDisabled: boolean;
  nextDisabled: boolean;
  onPrevious: () => void;
  onNext: () => void;
  className?: string;
}

export function AdminPaginationFooter({
  summary,
  previousDisabled,
  nextDisabled,
  onPrevious,
  onNext,
  className
}: AdminPaginationFooterProps) {
  return (
    <footer className={cn("flex flex-wrap items-center justify-between gap-3", className)}>
      <p className="text-xs text-slate-600">{summary}</p>
      <div className="flex gap-2">
        <Button type="button" size="sm" variant="outline" disabled={previousDisabled} onClick={onPrevious}>
          上一页
        </Button>
        <Button type="button" size="sm" variant="outline" disabled={nextDisabled} onClick={onNext}>
          下一页
        </Button>
      </div>
    </footer>
  );
}
