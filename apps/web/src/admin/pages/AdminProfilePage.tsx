import { Camera, KeyRound, LoaderCircle, Save, X } from "lucide-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ChangeEvent,
  type FormEvent,
  type PointerEvent as ReactPointerEvent,
  type WheelEvent as ReactWheelEvent
} from "react";
import { createPortal } from "react-dom";
import { Button } from "../../components/ui/button";
import { Input } from "../../components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/select";
import { showToast } from "../../components/ui/toast";
import { type AdminProfile, type DataGateway } from "../../data-access";
import { formatError } from "../../editor/status-utils";
import { AdminPageCard } from "../components/AdminPageLayout";

const AVATAR_FRAME_SIZE = 220;
const AVATAR_STAGE_SIZE = 320;
const AVATAR_MAX_EXPORT_SIZE = 512;

interface AdminProfilePageProps {
  dataGateway: DataGateway;
  onProfileUpdated?: (profile: AdminProfile) => void;
}

interface LoadedAvatarImage {
  element: HTMLImageElement;
  width: number;
  height: number;
  objectURL: string;
  fileName: string;
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}

function computeAvatarFrameScale(image: LoadedAvatarImage | null, zoom: number) {
  if (!image) {
    return {
      scale: 1,
      displayWidth: AVATAR_FRAME_SIZE,
      displayHeight: AVATAR_FRAME_SIZE
    };
  }
  const baseScale = Math.max(AVATAR_FRAME_SIZE / image.width, AVATAR_FRAME_SIZE / image.height);
  const scale = baseScale * zoom;
  return {
    scale,
    displayWidth: image.width * scale,
    displayHeight: image.height * scale
  };
}

function computeAvatarOffsetBounds(image: LoadedAvatarImage | null, zoom: number) {
  const { displayWidth, displayHeight } = computeAvatarFrameScale(image, zoom);
  const maxOffsetX = Math.max(0, (displayWidth - AVATAR_FRAME_SIZE) / 2);
  const maxOffsetY = Math.max(0, (displayHeight - AVATAR_FRAME_SIZE) / 2);
  return {
    minX: -maxOffsetX,
    maxX: maxOffsetX,
    minY: -maxOffsetY,
    maxY: maxOffsetY
  };
}

function readAvatarImageFile(file: File): Promise<LoadedAvatarImage> {
  return new Promise((resolve, reject) => {
    const objectURL = URL.createObjectURL(file);
    const image = new Image();
    image.onload = () => {
      resolve({
        element: image,
        width: image.naturalWidth,
        height: image.naturalHeight,
        objectURL,
        fileName: file.name || "avatar"
      });
    };
    image.onerror = () => {
      URL.revokeObjectURL(objectURL);
      reject(new Error("读取图片失败，请更换图片重试"));
    };
    image.src = objectURL;
  });
}

async function exportCroppedAvatarWebP(
  image: LoadedAvatarImage,
  zoom: number,
  offsetX: number,
  offsetY: number,
  quality = 0.9
): Promise<File> {
  const { scale, displayWidth, displayHeight } = computeAvatarFrameScale(image, zoom);
  const imageLeft = AVATAR_FRAME_SIZE / 2 + offsetX - displayWidth / 2;
  const imageTop = AVATAR_FRAME_SIZE / 2 + offsetY - displayHeight / 2;

  const sourceX = clamp((-imageLeft) / scale, 0, image.width);
  const sourceY = clamp((-imageTop) / scale, 0, image.height);
  const sourceWidth = clamp(AVATAR_FRAME_SIZE / scale, 1, image.width - sourceX);
  const sourceHeight = clamp(AVATAR_FRAME_SIZE / scale, 1, image.height - sourceY);
  const sourceSize = Math.min(sourceWidth, sourceHeight);

  const downScale = Math.min(1, AVATAR_MAX_EXPORT_SIZE / sourceSize);
  const outputSize = Math.max(1, Math.round(sourceSize * downScale));

  const canvas = document.createElement("canvas");
  canvas.width = outputSize;
  canvas.height = outputSize;
  const context = canvas.getContext("2d");
  if (!context) {
    throw new Error("浏览器不支持 Canvas 2D，头像导出失败");
  }
  context.imageSmoothingEnabled = true;
  context.imageSmoothingQuality = "high";
  context.drawImage(image.element, sourceX, sourceY, sourceSize, sourceSize, 0, 0, outputSize, outputSize);

  const blob = await new Promise<Blob>((resolve, reject) => {
    canvas.toBlob(
      (value) => {
        if (!value) {
          reject(new Error("头像导出失败，请重试"));
          return;
        }
        resolve(value);
      },
      "image/webp",
      quality
    );
  });

  const safeName = image.fileName.replace(/\.[^.]+$/, "") || "avatar";
  return new File([blob], `${safeName}.webp`, {
    type: "image/webp",
    lastModified: Date.now()
  });
}

function resolveAvatarFallback(profile: AdminProfile | null): string {
  const name = (profile?.name ?? "").trim();
  if (name) {
    return name.slice(0, 1).toUpperCase();
  }
  const email = (profile?.email ?? "").trim();
  if (email) {
    return email.slice(0, 1).toUpperCase();
  }
  return "A";
}

function resolvePrimaryAdminRole(profile: AdminProfile | null): "platform_admin" | "space_admin" | "none" {
  const roles = profile?.roles ?? [];
  if (roles.includes("platform_admin")) {
    return "platform_admin";
  }
  if (roles.includes("space_admin")) {
    return "space_admin";
  }
  return "none";
}

export function AdminProfilePage({ dataGateway, onProfileUpdated }: AdminProfilePageProps) {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const avatarDragStartRef = useRef<{ pointerX: number; pointerY: number; offsetX: number; offsetY: number } | null>(null);

  const [profile, setProfile] = useState<AdminProfile | null>(null);
  const [nameInput, setNameInput] = useState("");

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  const [loadingProfile, setLoadingProfile] = useState(false);
  const [savingProfile, setSavingProfile] = useState(false);
  const [uploadingAvatar, setUploadingAvatar] = useState(false);
  const [savingPassword, setSavingPassword] = useState(false);
  const [cropDialogOpen, setCropDialogOpen] = useState(false);
  const [loadedAvatarImage, setLoadedAvatarImage] = useState<LoadedAvatarImage | null>(null);
  const [avatarZoom, setAvatarZoom] = useState(1);
  const [avatarOffsetX, setAvatarOffsetX] = useState(0);
  const [avatarOffsetY, setAvatarOffsetY] = useState(0);

  const avatarPreviewURL = useMemo(() => (profile?.avatarUrl ?? "").trim(), [profile?.avatarUrl]);
  const primaryAdminRole = useMemo(() => resolvePrimaryAdminRole(profile), [profile]);
  const avatarFrameInfo = useMemo(
    () => computeAvatarFrameScale(loadedAvatarImage, avatarZoom),
    [loadedAvatarImage, avatarZoom]
  );

  useEffect(() => {
    return () => {
      if (loadedAvatarImage?.objectURL) {
        URL.revokeObjectURL(loadedAvatarImage.objectURL);
      }
    };
  }, [loadedAvatarImage?.objectURL]);

  const loadProfile = useCallback(async () => {
    setLoadingProfile(true);
    try {
      const payload = await dataGateway.admin.getProfile();
      setProfile(payload);
      setNameInput(payload.name ?? "");
      onProfileUpdated?.(payload);
    } catch (error) {
      showToast(`加载个人信息失败：${formatError(error)}`);
      setProfile(null);
      setNameInput("");
    } finally {
      setLoadingProfile(false);
    }
  }, [dataGateway.admin, onProfileUpdated]);

  useEffect(() => {
    void loadProfile();
  }, [loadProfile]);

  const handleSaveProfile = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      const name = nameInput.trim();
      if (!name) {
        showToast("昵称不能为空");
        return;
      }
      setSavingProfile(true);
      try {
        const payload = await dataGateway.admin.updateProfile({ name });
        setProfile(payload);
        setNameInput(payload.name ?? "");
        onProfileUpdated?.(payload);
        showToast("个人信息已更新", "success");
      } catch (error) {
        showToast(`更新个人信息失败：${formatError(error)}`);
      } finally {
        setSavingProfile(false);
      }
    },
    [dataGateway.admin, nameInput, onProfileUpdated]
  );

  const resetAvatarCropState = useCallback(() => {
    avatarDragStartRef.current = null;
    setAvatarZoom(1);
    setAvatarOffsetX(0);
    setAvatarOffsetY(0);
    setLoadedAvatarImage((previous) => {
      if (previous?.objectURL) {
        URL.revokeObjectURL(previous.objectURL);
      }
      return null;
    });
  }, []);

  const closeAvatarCropDialog = useCallback(() => {
    if (uploadingAvatar) {
      return;
    }
    setCropDialogOpen(false);
    resetAvatarCropState();
  }, [resetAvatarCropState, uploadingAvatar]);

  useEffect(() => {
    if (!cropDialogOpen) {
      return;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        closeAvatarCropDialog();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      document.body.style.overflow = previousOverflow;
    };
  }, [closeAvatarCropDialog, cropDialogOpen]);

  const applyClampedAvatarOffsets = useCallback(
    (nextX: number, nextY: number, nextZoom: number = avatarZoom) => {
      const bounds = computeAvatarOffsetBounds(loadedAvatarImage, nextZoom);
      setAvatarOffsetX(clamp(nextX, bounds.minX, bounds.maxX));
      setAvatarOffsetY(clamp(nextY, bounds.minY, bounds.maxY));
    },
    [avatarZoom, loadedAvatarImage]
  );

  const handleAvatarFileChange = useCallback(
    async (event: ChangeEvent<HTMLInputElement>) => {
      const file = event.target.files?.[0];
      event.target.value = "";
      if (!file) {
        return;
      }
      try {
        const image = await readAvatarImageFile(file);
        setLoadedAvatarImage((previous) => {
          if (previous?.objectURL) {
            URL.revokeObjectURL(previous.objectURL);
          }
          return image;
        });
        setAvatarZoom(1);
        setAvatarOffsetX(0);
        setAvatarOffsetY(0);
        setCropDialogOpen(true);
      } catch (error) {
        showToast(`读取头像失败：${formatError(error)}`);
      }
    },
    []
  );

  const triggerUploadAvatar = useCallback(() => {
    fileInputRef.current?.click();
  }, []);

  const handleAvatarZoomChange = useCallback(
    (event: ChangeEvent<HTMLInputElement>) => {
      const nextZoom = Number.parseFloat(event.target.value);
      if (!Number.isFinite(nextZoom)) {
        return;
      }
      setAvatarZoom(nextZoom);
      applyClampedAvatarOffsets(avatarOffsetX, avatarOffsetY, nextZoom);
    },
    [applyClampedAvatarOffsets, avatarOffsetX, avatarOffsetY]
  );

  const handleAvatarPointerDown = useCallback(
    (event: ReactPointerEvent<HTMLDivElement>) => {
      if (!loadedAvatarImage) {
        return;
      }
      avatarDragStartRef.current = {
        pointerX: event.clientX,
        pointerY: event.clientY,
        offsetX: avatarOffsetX,
        offsetY: avatarOffsetY
      };
      event.currentTarget.setPointerCapture(event.pointerId);
    },
    [avatarOffsetX, avatarOffsetY, loadedAvatarImage]
  );

  const handleAvatarPointerMove = useCallback(
    (event: ReactPointerEvent<HTMLDivElement>) => {
      const dragState = avatarDragStartRef.current;
      if (!dragState || !loadedAvatarImage) {
        return;
      }
      const deltaX = event.clientX - dragState.pointerX;
      const deltaY = event.clientY - dragState.pointerY;
      applyClampedAvatarOffsets(dragState.offsetX + deltaX, dragState.offsetY + deltaY);
    },
    [applyClampedAvatarOffsets, loadedAvatarImage]
  );

  const handleAvatarPointerUp = useCallback((event: ReactPointerEvent<HTMLDivElement>) => {
    avatarDragStartRef.current = null;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  }, []);

  const handleAvatarWheel = useCallback(
    (event: ReactWheelEvent<HTMLDivElement>) => {
      if (!loadedAvatarImage) {
        return;
      }
      event.preventDefault();
      const delta = event.deltaY < 0 ? 0.08 : -0.08;
      const nextZoom = clamp(Number((avatarZoom + delta).toFixed(2)), 1, 3);
      if (nextZoom === avatarZoom) {
        return;
      }
      setAvatarZoom(nextZoom);
      applyClampedAvatarOffsets(avatarOffsetX, avatarOffsetY, nextZoom);
    },
    [applyClampedAvatarOffsets, avatarOffsetX, avatarOffsetY, avatarZoom, loadedAvatarImage]
  );

  const handleAvatarCropUpload = useCallback(async () => {
    if (!loadedAvatarImage) {
      showToast("请先选择头像图片");
      return;
    }
    setUploadingAvatar(true);
    try {
      const croppedFile = await exportCroppedAvatarWebP(loadedAvatarImage, avatarZoom, avatarOffsetX, avatarOffsetY);
      const payload = await dataGateway.admin.uploadAvatar(croppedFile);
      setProfile(payload);
      setNameInput(payload.name ?? "");
      onProfileUpdated?.(payload);
      setCropDialogOpen(false);
      resetAvatarCropState();
      showToast("头像上传成功", "success");
    } catch (error) {
      showToast(`头像上传失败：${formatError(error)}`);
    } finally {
      setUploadingAvatar(false);
    }
  }, [avatarOffsetX, avatarOffsetY, avatarZoom, dataGateway.admin, loadedAvatarImage, onProfileUpdated, resetAvatarCropState]);

  const handleClearAvatar = useCallback(async () => {
    setUploadingAvatar(true);
    try {
      const payload = await dataGateway.admin.updateProfile({
        avatarUrl: ""
      });
      setProfile(payload);
      setNameInput(payload.name ?? "");
      onProfileUpdated?.(payload);
      showToast("头像已清空", "success");
    } catch (error) {
      showToast(`清空头像失败：${formatError(error)}`);
    } finally {
      setUploadingAvatar(false);
    }
  }, [dataGateway.admin, onProfileUpdated]);

  const handleUpdatePassword = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (!currentPassword.trim()) {
        showToast("请输入当前密码");
        return;
      }
      if (newPassword.length < 6) {
        showToast("新密码长度至少为 6 位");
        return;
      }
      if (newPassword !== confirmPassword) {
        showToast("两次输入的新密码不一致");
        return;
      }
      setSavingPassword(true);
      try {
        await dataGateway.admin.updatePassword({
          currentPassword,
          newPassword,
          confirmPassword
        });
        setCurrentPassword("");
        setNewPassword("");
        setConfirmPassword("");
        showToast("密码修改成功", "success");
      } catch (error) {
        showToast(`修改密码失败：${formatError(error)}`);
      } finally {
        setSavingPassword(false);
      }
    },
    [confirmPassword, currentPassword, dataGateway.admin, newPassword]
  );

  return (
    <div className="space-y-5">
      <AdminPageCard className="rounded-xl border border-slate-200 bg-white shadow-sm" contentClassName="space-y-5 p-5">
        <header className="space-y-1">
          <h3 className="text-lg font-semibold text-slate-900">个人信息</h3>
          <p className="text-sm text-slate-500">维护昵称、头像与账号基础信息。</p>
        </header>

        <form className="space-y-4" onSubmit={handleSaveProfile}>
          <div className="flex flex-wrap items-center gap-4">
            <div className="flex h-16 w-16 items-center justify-center overflow-hidden rounded-full border border-slate-200 bg-slate-100 text-xl font-semibold text-slate-700">
              {avatarPreviewURL ? (
                <img src={avatarPreviewURL} alt="avatar preview" className="h-full w-full object-cover" />
              ) : (
                <span>{resolveAvatarFallback(profile)}</span>
              )}
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Button type="button" variant="outline" size="sm" onClick={triggerUploadAvatar} disabled={uploadingAvatar}>
                {uploadingAvatar ? <LoaderCircle size={14} className="animate-spin" /> : <Camera size={14} />}
                上传头像
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => void handleClearAvatar()}
                disabled={uploadingAvatar || !avatarPreviewURL}
              >
                清空头像
              </Button>
              <input
                ref={fileInputRef}
                type="file"
                accept="image/*"
                className="hidden"
                onChange={handleAvatarFileChange}
              />
            </div>
          </div>

          <div className="grid gap-3 lg:grid-cols-2">
            <label className="space-y-1.5 text-sm text-slate-700">
              <span className="font-medium">用户 ID</span>
              <Input value={profile?.userId ?? ""} disabled placeholder="-" />
            </label>
            <label className="space-y-1.5 text-sm text-slate-700">
              <span className="font-medium">邮箱</span>
              <Input value={profile?.email ?? ""} disabled placeholder="-" />
            </label>
          </div>

          <div className="grid gap-3 lg:grid-cols-[minmax(0,360px)_minmax(0,220px)]">
            <label className="grid gap-1.5 text-sm text-slate-700">
              <span className="font-medium">昵称</span>
              <Input
                value={nameInput}
                onChange={(event) => setNameInput(event.target.value)}
                placeholder="输入昵称"
                className="max-w-[360px]"
              />
            </label>
            <label className="grid gap-1.5 text-sm text-slate-700">
              <span className="font-medium">角色</span>
              <Select value={primaryAdminRole} disabled>
                <SelectTrigger className="max-w-[220px] bg-slate-50">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="platform_admin">全站管理员</SelectItem>
                  <SelectItem value="space_admin">空间管理员</SelectItem>
                  <SelectItem value="none">普通用户</SelectItem>
                </SelectContent>
              </Select>
            </label>
          </div>

          <div className="flex justify-end">
            <Button type="submit" disabled={loadingProfile || savingProfile}>
              {savingProfile ? <LoaderCircle size={14} className="animate-spin" /> : <Save size={14} />}
              保存个人信息
            </Button>
          </div>
        </form>
      </AdminPageCard>

      <AdminPageCard className="rounded-xl border border-slate-200 bg-white shadow-sm" contentClassName="space-y-5 p-5">
        <header className="space-y-1">
          <h3 className="text-lg font-semibold text-slate-900">修改密码</h3>
          <p className="text-sm text-slate-500">密码修改后立即生效，建议使用高强度密码。</p>
        </header>

        <form className="space-y-3.5" onSubmit={handleUpdatePassword}>
          <div className="grid gap-3 lg:grid-cols-3">
            <label className="space-y-1.5 text-sm text-slate-700">
              <span className="font-medium">当前密码</span>
              <Input
                type="password"
                value={currentPassword}
                onChange={(event) => setCurrentPassword(event.target.value)}
                autoComplete="current-password"
                placeholder="输入当前密码"
              />
            </label>
            <label className="space-y-1.5 text-sm text-slate-700">
              <span className="font-medium">新密码</span>
              <Input
                type="password"
                value={newPassword}
                onChange={(event) => setNewPassword(event.target.value)}
                autoComplete="new-password"
                placeholder="至少 6 位"
              />
            </label>
            <label className="space-y-1.5 text-sm text-slate-700">
              <span className="font-medium">确认新密码</span>
              <Input
                type="password"
                value={confirmPassword}
                onChange={(event) => setConfirmPassword(event.target.value)}
                autoComplete="new-password"
                placeholder="再次输入新密码"
              />
            </label>
          </div>
          <div className="flex justify-end">
            <Button type="submit" disabled={savingPassword}>
              {savingPassword ? <LoaderCircle size={14} className="animate-spin" /> : <KeyRound size={14} />}
              更新密码
            </Button>
          </div>
        </form>
      </AdminPageCard>

      {cropDialogOpen && loadedAvatarImage
        ? createPortal(
          <div
            className="fixed inset-0 z-[2800] flex items-center justify-center bg-slate-900/50 p-4 backdrop-blur-sm"
            onMouseDown={(event) => {
              if (event.target === event.currentTarget) {
                closeAvatarCropDialog();
              }
            }}
          >
            <section className="grid max-h-[92vh] w-full max-w-[760px] grid-rows-[auto_minmax(0,1fr)_auto] overflow-hidden rounded-xl border border-slate-200 bg-white shadow-[0_30px_80px_rgba(15,23,42,0.28)]">
              <header className="flex items-start justify-between border-b border-slate-200 bg-gradient-to-r from-slate-50 to-sky-50 px-5 py-4">
                <div className="space-y-1">
                  <h3 className="text-lg font-semibold text-slate-900">裁剪头像</h3>
                  <p className="text-xs text-slate-600">头像将裁剪为正方形并按比例缩放后上传。</p>
                </div>
                <Button
                  type="button"
                  size="icon"
                  variant="ghost"
                  className="h-8 w-8"
                  disabled={uploadingAvatar}
                  onClick={closeAvatarCropDialog}
                >
                  <X size={16} />
                </Button>
              </header>

              <div className="grid min-h-0 gap-4 overflow-y-auto px-5 py-4 md:grid-cols-[minmax(0,1fr)_220px]">
                <div className="space-y-2 rounded-lg border border-dashed border-slate-300 bg-slate-50 p-3">
                  <p className="text-xs font-medium text-slate-700">拖拽平移，滚轮或滑杆缩放</p>
                  <div
                    className="relative mx-auto cursor-grab overflow-hidden rounded-md border border-slate-300 bg-slate-100 shadow-inner active:cursor-grabbing"
                    style={{
                      width: `${AVATAR_STAGE_SIZE}px`,
                      height: `${AVATAR_STAGE_SIZE}px`,
                      touchAction: "none"
                    }}
                    onPointerDown={handleAvatarPointerDown}
                    onPointerMove={handleAvatarPointerMove}
                    onPointerUp={handleAvatarPointerUp}
                    onPointerCancel={handleAvatarPointerUp}
                    onWheel={handleAvatarWheel}
                  >
                    <img
                      src={loadedAvatarImage.objectURL}
                      alt="avatar-crop-preview"
                      draggable={false}
                      className="pointer-events-none absolute left-1/2 top-1/2 select-none"
                      style={{
                        width: `${avatarFrameInfo.displayWidth}px`,
                        height: `${avatarFrameInfo.displayHeight}px`,
                        transform: `translate(-50%, -50%) translate(${avatarOffsetX}px, ${avatarOffsetY}px)`
                      }}
                    />
                    <div
                      className="pointer-events-none absolute left-1/2 top-1/2 rounded-md border border-white/90 shadow-[0_0_0_9999px_rgba(15,23,42,0.5)]"
                      style={{
                        width: `${AVATAR_FRAME_SIZE}px`,
                        height: `${AVATAR_FRAME_SIZE}px`,
                        transform: "translate(-50%, -50%)"
                      }}
                    />
                  </div>
                  <label className="grid gap-1.5">
                    <span className="text-xs font-medium text-slate-700">缩放</span>
                    <input
                      type="range"
                      min={1}
                      max={3}
                      step={0.01}
                      value={avatarZoom}
                      onChange={handleAvatarZoomChange}
                    />
                  </label>
                </div>

                <aside className="rounded-lg border border-slate-200 bg-white p-3 text-xs text-slate-600">
                  <p className="font-medium text-slate-800">说明</p>
                  <ul className="mt-2 list-disc space-y-1 pl-4">
                    <li>输出为正方形头像</li>
                    <li>前端先裁剪，再上传后端</li>
                    <li>默认导出 WebP，加载更快</li>
                  </ul>
                </aside>
              </div>

              <footer className="flex justify-end gap-2 border-t border-slate-200 px-5 py-4">
                <Button type="button" variant="outline" onClick={closeAvatarCropDialog} disabled={uploadingAvatar}>
                  取消
                </Button>
                <Button type="button" onClick={() => void handleAvatarCropUpload()} disabled={uploadingAvatar}>
                  {uploadingAvatar ? <LoaderCircle size={14} className="animate-spin" /> : <Camera size={14} />}
                  裁剪并上传
                </Button>
              </footer>
            </section>
          </div>,
          document.body
        )
        : null}
    </div>
  );
}
