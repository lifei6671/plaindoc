import { KeyRound, LoaderCircle } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { Badge } from "./ui/badge";
import { Button } from "./ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "./ui/card";
import { Input } from "./ui/input";

interface ResetPasswordPanelProps {
  submitting: boolean;
  errorMessage: string | null;
  loginPath: string;
  onVerifyToken: (token: string) => Promise<{ valid: boolean; expiresAt: string }>;
  onSubmit: (input: { token: string; newPassword: string; confirmPassword: string }) => Promise<void>;
}

function readTokenFromHash(): string {
  const hashText = window.location.hash.trim();
  if (!hashText.startsWith("#")) {
    return "";
  }
  const query = new URLSearchParams(hashText.slice(1));
  return (query.get("token") ?? "").trim();
}

export function ResetPasswordPanel({
  submitting,
  errorMessage,
  loginPath,
  onVerifyToken,
  onSubmit
}: ResetPasswordPanelProps) {
  const [token, setToken] = useState<string>(() => readTokenFromHash());
  const [verifying, setVerifying] = useState(false);
  const [verifyMessage, setVerifyMessage] = useState<string | null>(null);
  const [isTokenValid, setIsTokenValid] = useState(false);
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  useEffect(() => {
    setToken(readTokenFromHash());
  }, []);

  useEffect(() => {
    let cancelled = false;
    const verify = async () => {
      if (!token) {
        setIsTokenValid(false);
        setVerifyMessage("重置链接缺少令牌参数，请重新申请找回密码。");
        return;
      }
      setVerifying(true);
      setVerifyMessage(null);
      try {
        const result = await onVerifyToken(token);
        if (cancelled) {
          return;
        }
        if (!result.valid) {
          setIsTokenValid(false);
          setVerifyMessage("重置链接无效，请重新申请找回密码。");
          return;
        }
        setIsTokenValid(true);
        if (result.expiresAt) {
          const expiresAtDate = new Date(result.expiresAt);
          const expiresAtText = Number.isNaN(expiresAtDate.getTime())
            ? result.expiresAt
            : expiresAtDate.toLocaleString("zh-CN", { hour12: false });
          setVerifyMessage(`重置链接有效，过期时间：${expiresAtText}`);
        } else {
          setVerifyMessage("重置链接有效，请输入新密码。");
        }
      } catch {
        if (cancelled) {
          return;
        }
        setIsTokenValid(false);
        setVerifyMessage("重置链接校验失败，请重新申请找回密码。");
      } finally {
        if (!cancelled) {
          setVerifying(false);
        }
      }
    };
    void verify();
    return () => {
      cancelled = true;
    };
  }, [onVerifyToken, token]);

  const localValidationError = useMemo(() => {
    if (!confirmPassword) {
      return null;
    }
    if (newPassword !== confirmPassword) {
      return "两次输入的密码不一致";
    }
    return null;
  }, [confirmPassword, newPassword]);

  const canSubmit = useMemo(() => {
    if (submitting || verifying || !isTokenValid) {
      return false;
    }
    if (!newPassword || !confirmPassword) {
      return false;
    }
    if (newPassword !== confirmPassword) {
      return false;
    }
    return true;
  }, [confirmPassword, isTokenValid, newPassword, submitting, verifying]);

  const handleSubmit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (!canSubmit) {
        return;
      }
      await onSubmit({
        token,
        newPassword,
        confirmPassword
      });
    },
    [canSubmit, confirmPassword, newPassword, onSubmit, token]
  );

  return (
    <div className="admin-auth-page">
      <Card className="mx-auto w-full max-w-md border-slate-200 shadow-xl">
        <CardHeader className="space-y-4 pb-4">
          <Badge variant="outline" className="w-fit gap-1 border-cyan-200 bg-cyan-50 text-cyan-700">
            <KeyRound size={13} />
            <span>重置密码</span>
          </Badge>
          <div className="space-y-2">
            <CardTitle className="text-2xl tracking-tight">设置新密码</CardTitle>
            <CardDescription>请输入新的登录密码，提交后需要重新登录。</CardDescription>
          </div>
        </CardHeader>
        <CardContent>
          <form className="space-y-4" onSubmit={(event) => void handleSubmit(event)}>
            <label className="admin-auth-form__field">
              <span>新密码</span>
              <Input
                type="password"
                value={newPassword}
                onChange={(event) => setNewPassword(event.target.value)}
                placeholder="至少 6 位"
                autoComplete="new-password"
                disabled={submitting || verifying || !isTokenValid}
              />
            </label>
            <label className="admin-auth-form__field">
              <span>确认新密码</span>
              <Input
                type="password"
                value={confirmPassword}
                onChange={(event) => setConfirmPassword(event.target.value)}
                placeholder="再次输入新密码"
                autoComplete="new-password"
                disabled={submitting || verifying || !isTokenValid}
              />
            </label>

            {verifyMessage ? (
              <p className={isTokenValid ? "text-xs text-emerald-700" : "admin-auth-form__error"}>{verifyMessage}</p>
            ) : null}
            {localValidationError ? <p className="admin-auth-form__error">{localValidationError}</p> : null}
            {!localValidationError && errorMessage ? <p className="admin-auth-form__error">{errorMessage}</p> : null}

            <Button type="submit" className="w-full" disabled={!canSubmit}>
              {submitting || verifying ? <LoaderCircle className="admin-auth-form__submit-loader" size={14} /> : null}
              <span>{submitting ? "提交中..." : verifying ? "校验中..." : "确认重置"}</span>
            </Button>

            <p className="text-center text-sm text-slate-600">
              <a href={loginPath} className="font-medium text-cyan-700 hover:text-cyan-800 hover:underline">
                返回登录
              </a>
            </p>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
