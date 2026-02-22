import { LoaderCircle, Plus, Trash2, UserPlus, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEventHandler } from "react";
import { createPortal } from "react-dom";
import { Badge } from "../../components/ui/badge";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { showToast } from "../../components/ui/toast";
import type { AdminSpace, AdminSpaceMember, DataGateway } from "../../data-access";
import { formatError } from "../../editor/status-utils";

interface AdminSpaceMembersDialogProps {
  open: boolean;
  space: AdminSpace | null;
  dataGateway: DataGateway;
  onOpenChange: (open: boolean) => void;
}

function normalizeMemberRole(value: AdminSpaceMember["role"]): "owner" | "collaborator" | "reader" {
  if (value === "owner" || value === "collaborator" || value === "reader") {
    return value;
  }
  return "reader";
}

function memberRoleLabel(role: AdminSpaceMember["role"]): string {
  switch (role) {
    case "owner":
      return "拥有者";
    case "collaborator":
      return "协作者";
    case "reader":
      return "只读成员";
    default:
      return role;
  }
}

function formatDateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime()) || date.getFullYear() <= 1) {
    return "-";
  }
  return date.toLocaleString("zh-CN", { hour12: false });
}

function sortMembers(items: AdminSpaceMember[]): AdminSpaceMember[] {
  const roleWeight = (role: AdminSpaceMember["role"]) => {
    switch (normalizeMemberRole(role)) {
      case "owner":
        return 0;
      case "collaborator":
        return 1;
      case "reader":
        return 2;
      default:
        return 3;
    }
  };

  return [...items].sort((left, right) => {
    if (left.isOwner !== right.isOwner) {
      return left.isOwner ? -1 : 1;
    }
    const roleDiff = roleWeight(left.role) - roleWeight(right.role);
    if (roleDiff !== 0) {
      return roleDiff;
    }
    const leftUpdated = new Date(left.updatedAt).getTime();
    const rightUpdated = new Date(right.updatedAt).getTime();
    if (Number.isFinite(leftUpdated) && Number.isFinite(rightUpdated) && leftUpdated !== rightUpdated) {
      return rightUpdated - leftUpdated;
    }
    return left.userId.localeCompare(right.userId);
  });
}

function upsertMember(list: AdminSpaceMember[], member: AdminSpaceMember): AdminSpaceMember[] {
  const next = [...list];
  const index = next.findIndex((item) => item.userId === member.userId);
  if (index >= 0) {
    next[index] = member;
  } else {
    next.push(member);
  }
  return sortMembers(next);
}

export function AdminSpaceMembersDialog({ open, space, dataGateway, onOpenChange }: AdminSpaceMembersDialogProps) {
  const [members, setMembers] = useState<AdminSpaceMember[]>([]);
  const [loadingMembers, setLoadingMembers] = useState(false);
  const [submittingAdd, setSubmittingAdd] = useState(false);
  const [actioningUserID, setActioningUserID] = useState<string | null>(null);

  const [targetInput, setTargetInput] = useState("");
  const [newRole, setNewRole] = useState<"collaborator" | "reader">("collaborator");
  const spaceID = space?.spaceId ?? "";

  const openToast = useCallback((message: string, variant: "success" | "info" | "error" = "error") => {
    showToast(message, variant);
  }, []);

  const loadMembers = useCallback(async () => {
    if (!spaceID) {
      setMembers([]);
      return;
    }
    setLoadingMembers(true);
    try {
      const payload = await dataGateway.admin.listSpaceMembers(spaceID);
      setMembers(sortMembers(payload));
    } catch (error) {
      openToast(`加载成员失败：${formatError(error)}`);
      setMembers([]);
    } finally {
      setLoadingMembers(false);
    }
  }, [dataGateway.admin, openToast, spaceID]);

  useEffect(() => {
    if (!open) {
      return;
    }
    const previousOverflow = document.body.style.overflow;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") {
        return;
      }
      event.preventDefault();
      if (submittingAdd || loadingMembers || actioningUserID) {
        return;
      }
      onOpenChange(false);
    };

    document.body.style.overflow = "hidden";
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [actioningUserID, loadingMembers, onOpenChange, open, submittingAdd]);

  useEffect(() => {
    if (!open) {
      return;
    }
    void loadMembers();
  }, [loadMembers, open]);

  useEffect(() => {
    if (open) {
      return;
    }
    setMembers([]);
    setTargetInput("");
    setNewRole("collaborator");
    setLoadingMembers(false);
    setSubmittingAdd(false);
    setActioningUserID(null);
  }, [open]);

  const closeDialog = useCallback(() => {
    if (submittingAdd || loadingMembers || actioningUserID) {
      return;
    }
    onOpenChange(false);
  }, [actioningUserID, loadingMembers, onOpenChange, submittingAdd]);

  const handleAddMember = useCallback<FormEventHandler<HTMLFormElement>>(
    async (event) => {
      event.preventDefault();
      if (!space) {
        return;
      }
      const target = targetInput.trim();
      if (!target) {
        openToast("请输入用户 ID 或邮箱");
        return;
      }

      setSubmittingAdd(true);
      try {
        const payload =
          target.includes("@")
            ? await dataGateway.admin.addSpaceMember({
                spaceId: space.spaceId,
                targetEmail: target,
                role: newRole
              })
            : await dataGateway.admin.addSpaceMember({
                spaceId: space.spaceId,
                targetUserId: target,
                role: newRole
              });
        setMembers((previous) => upsertMember(previous, payload));
        setTargetInput("");
        openToast("成员已保存", "success");
      } catch (error) {
        openToast(`保存成员失败：${formatError(error)}`);
      } finally {
        setSubmittingAdd(false);
      }
    },
    [dataGateway.admin, newRole, openToast, space, targetInput]
  );

  const handleRoleChange = useCallback(
    async (member: AdminSpaceMember, role: "collaborator" | "reader") => {
      if (!space || member.isOwner || member.role === role) {
        return;
      }
      setActioningUserID(member.userId);
      try {
        const payload = await dataGateway.admin.updateSpaceMemberRole({
          spaceId: space.spaceId,
          userId: member.userId,
          role
        });
        setMembers((previous) => upsertMember(previous, payload));
        openToast("成员角色已更新", "success");
      } catch (error) {
        openToast(`更新成员角色失败：${formatError(error)}`);
      } finally {
        setActioningUserID(null);
      }
    },
    [dataGateway.admin, openToast, space]
  );

  const handleRemoveMember = useCallback(
    async (member: AdminSpaceMember) => {
      if (!space || member.isOwner) {
        return;
      }
      const confirmed = window.confirm(`确认将成员「${member.name || member.email || member.userId}」移出当前空间吗？`);
      if (!confirmed) {
        return;
      }
      setActioningUserID(member.userId);
      try {
        await dataGateway.admin.removeSpaceMember({
          spaceId: space.spaceId,
          userId: member.userId
        });
        setMembers((previous) => sortMembers(previous.filter((item) => item.userId !== member.userId)));
        openToast("成员已移除", "success");
      } catch (error) {
        openToast(`移除成员失败：${formatError(error)}`);
      } finally {
        setActioningUserID(null);
      }
    },
    [dataGateway.admin, openToast, space]
  );

  const actioningSet = useMemo(() => new Set(actioningUserID ? [actioningUserID] : []), [actioningUserID]);

  if (!open || !space) {
    return null;
  }

  return createPortal(
    <div className="fixed inset-0 z-[2400] flex items-center justify-center bg-slate-900/45 px-4 py-6 backdrop-blur-[1px]">
      <div className="flex h-[min(90vh,760px)] w-full max-w-5xl flex-col overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-2xl">
        <header className="flex items-start justify-between border-b border-slate-200 px-6 py-4">
          <div className="space-y-1">
            <h2 className="flex items-center gap-2 text-lg font-semibold text-slate-900">
              <UserPlus size={18} />
              <span>空间成员管理</span>
            </h2>
            <p className="text-sm text-slate-500">
              当前空间：{space.name}（{space.spaceId}）
            </p>
          </div>
          <Button
            type="button"
            size="icon"
            variant="ghost"
            className="h-8 w-8"
            disabled={submittingAdd || loadingMembers || actioningUserID !== null}
            onClick={closeDialog}
            aria-label="关闭成员管理"
          >
            <X size={16} />
          </Button>
        </header>

        <div className="grid gap-4 border-b border-slate-200 bg-slate-50/60 px-6 py-4">
          <form className="grid gap-3 md:grid-cols-[minmax(0,1fr)_180px_auto]" onSubmit={handleAddMember}>
            <Input
              value={targetInput}
              disabled={submittingAdd || loadingMembers}
              placeholder="输入用户 ID 或邮箱，添加为当前空间成员"
              onChange={(event) => setTargetInput(event.target.value)}
            />
            <Select
              value={newRole}
              onValueChange={(value) => setNewRole((value === "reader" ? "reader" : "collaborator"))}
              disabled={submittingAdd || loadingMembers}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="collaborator">协作者</SelectItem>
                <SelectItem value="reader">只读成员</SelectItem>
              </SelectContent>
            </Select>
            <Button type="submit" disabled={submittingAdd || loadingMembers}>
              {submittingAdd ? <LoaderCircle size={14} className="animate-spin" /> : <Plus size={14} />}
              <span>添加成员</span>
            </Button>
          </form>
          <p className="text-xs text-slate-500">支持输入已注册用户的用户 ID 或邮箱；已存在成员会直接更新为所选角色。</p>
        </div>

        <div className="min-h-0 flex-1 overflow-auto px-6 py-4">
          {loadingMembers ? (
            <div className="flex h-full items-center justify-center gap-2 text-sm text-slate-500">
              <LoaderCircle size={15} className="animate-spin" />
              <span>正在加载空间成员...</span>
            </div>
          ) : members.length === 0 ? (
            <div className="flex h-full items-center justify-center text-sm text-slate-500">暂无成员数据</div>
          ) : (
            <table className="w-full min-w-[760px] table-fixed border-collapse text-left text-sm">
              <thead className="sticky top-0 z-10 bg-white">
                <tr className="text-xs uppercase tracking-wide text-slate-500">
                  <th className="w-[310px] border-b border-slate-200 px-3 py-2 font-semibold">成员</th>
                  <th className="w-[180px] border-b border-slate-200 px-3 py-2 font-semibold">角色</th>
                  <th className="w-[180px] border-b border-slate-200 px-3 py-2 font-semibold">加入时间</th>
                  <th className="w-[180px] border-b border-slate-200 px-3 py-2 font-semibold">最近更新</th>
                  <th className="w-[120px] border-b border-slate-200 px-3 py-2 font-semibold text-right">操作</th>
                </tr>
              </thead>
              <tbody>
                {members.map((member) => {
                  const rowActioning = actioningSet.has(member.userId);
                  const role = normalizeMemberRole(member.role);
                  return (
                    <tr key={member.userId} className="border-b border-slate-100 align-top text-slate-700">
                      <td className="px-3 py-3">
                        <div className="grid gap-0.5">
                          <strong className="truncate text-sm font-semibold text-slate-900">{member.name || "-"}</strong>
                          <span className="truncate text-xs text-slate-600">{member.email || "-"}</span>
                          <code className="truncate text-[11px] text-slate-400">{member.userId}</code>
                        </div>
                      </td>
                      <td className="px-3 py-3">
                        {member.isOwner || role === "owner" ? (
                          <Badge variant="outline" className="border-indigo-200 bg-indigo-50 text-indigo-700">
                            拥有者
                          </Badge>
                        ) : (
                          <Select
                            value={role === "reader" ? "reader" : "collaborator"}
                            disabled={submittingAdd || loadingMembers || rowActioning}
                            onValueChange={(value) =>
                              void handleRoleChange(member, value === "reader" ? "reader" : "collaborator")
                            }
                          >
                            <SelectTrigger className="w-[160px]">
                              <SelectValue>{memberRoleLabel(role)}</SelectValue>
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="collaborator">协作者</SelectItem>
                              <SelectItem value="reader">只读成员</SelectItem>
                            </SelectContent>
                          </Select>
                        )}
                      </td>
                      <td className="px-3 py-3 text-xs text-slate-600">{formatDateTime(member.createdAt)}</td>
                      <td className="px-3 py-3 text-xs text-slate-600">{formatDateTime(member.updatedAt)}</td>
                      <td className="px-3 py-3 text-right">
                        {member.isOwner || role === "owner" ? (
                          <span className="text-xs text-slate-400">不可移除</span>
                        ) : (
                          <Button
                            type="button"
                            size="sm"
                            variant="outline"
                            className="border-rose-200 text-rose-600 hover:bg-rose-50 hover:text-rose-700"
                            disabled={submittingAdd || loadingMembers || rowActioning}
                            onClick={() => void handleRemoveMember(member)}
                          >
                            {rowActioning ? <LoaderCircle size={14} className="animate-spin" /> : <Trash2 size={14} />}
                            <span>移除</span>
                          </Button>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
        </div>

        <footer className="flex items-center justify-end border-t border-slate-200 px-6 py-3">
          <Button type="button" variant="outline" onClick={closeDialog} disabled={submittingAdd || loadingMembers || actioningUserID !== null}>
            关闭
          </Button>
        </footer>
      </div>
    </div>,
    document.body
  );
}
