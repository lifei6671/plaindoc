import { LoaderCircle } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { ADMIN_BRAND_LOGO_SRC } from "../admin/brand";
import type { AuthCaptchaChallenge } from "../data-access";
import { Button } from "./ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "./ui/card";
import { Input } from "./ui/input";
import { Badge } from "./ui/badge";

interface AdminAuthPanelProps {
  checking: boolean;
  submitting: boolean;
  errorMessage: string | null;
  authChallenge: AuthCaptchaChallenge | null;
  onLogin: (input: { email: string; password: string; captchaId?: string; captchaAnswer?: string }) => Promise<void>;
}

// 管理后台登录面板：复用现有账号体系，仅允许登录入口（不提供注册）。
export function AdminAuthPanel({ checking, submitting, errorMessage, authChallenge, onLogin }: AdminAuthPanelProps) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [captchaAnswer, setCaptchaAnswer] = useState("");

  useEffect(() => {
    setCaptchaAnswer("");
  }, [authChallenge?.captchaId]);

  const canSubmit = useMemo(() => {
    if (checking || submitting) {
      return false;
    }
    if (!email.trim() || !password.trim()) {
      return false;
    }
    if (authChallenge && !captchaAnswer.trim()) {
      return false;
    }
    return true;
  }, [authChallenge, captchaAnswer, checking, email, password, submitting]);

  const submitText = useMemo(() => {
    if (checking) {
      return "检查会话中...";
    }
    if (submitting) {
      return "登录中...";
    }
    return "登录管理后台";
  }, [checking, submitting]);

  const handleSubmit = useCallback(async (event?: FormEvent<HTMLFormElement>) => {
    event?.preventDefault();
    if (!canSubmit) {
      return;
    }
    await onLogin({
      email: email.trim(),
      password,
      captchaId: authChallenge?.captchaId,
      captchaAnswer: captchaAnswer.trim()
    });
    setPassword("");
  }, [authChallenge?.captchaId, canSubmit, captchaAnswer, email, onLogin, password]);

  return (
    <div className="admin-auth-page">
      <Card className="mx-auto w-full max-w-md border-slate-200 shadow-xl">
        <CardHeader className="space-y-4 pb-4">
          <div className="flex items-center gap-2">
            <span className="flex h-8 w-8 shrink-0 items-center justify-center overflow-hidden rounded-md border border-slate-200/80 bg-white shadow-sm">
              <img src={ADMIN_BRAND_LOGO_SRC} alt="PlainDoc logo" className="h-full w-full object-cover" />
            </span>
            <Badge variant="outline" className="w-fit border-cyan-200 bg-cyan-50 text-cyan-700">
              <span>Admin Console</span>
            </Badge>
          </div>
          <div className="space-y-2">
            <CardTitle className="text-2xl tracking-tight">PlainDoc 管理后台</CardTitle>
            <CardDescription>使用现有账号登录，系统会自动校验管理员角色。</CardDescription>
          </div>
        </CardHeader>
        <CardContent>
          <form className="space-y-4" onSubmit={(event) => void handleSubmit(event)}>
            <label className="admin-auth-form__field">
              <span>邮箱</span>
              <Input
                type="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="admin@example.com"
                autoComplete="email"
                disabled={checking || submitting}
              />
            </label>
            <label className="admin-auth-form__field">
              <span>密码</span>
              <Input
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="输入登录密码"
                autoComplete="current-password"
                disabled={checking || submitting}
              />
            </label>
            {authChallenge ? (
              <div className="space-y-2 rounded-md border border-slate-200 bg-slate-50 p-3">
                <p className="text-xs text-slate-600">已触发验证码校验（{authChallenge.level} 位数字）</p>
                <img
                  src={authChallenge.captchaImageDataUrl}
                  alt="验证码"
                  className="h-14 w-full rounded border border-slate-200 bg-white object-contain"
                />
                <label className="admin-auth-form__field">
                  <span>验证码</span>
                  <Input
                    type="text"
                    value={captchaAnswer}
                    onChange={(event) => setCaptchaAnswer(event.target.value)}
                    placeholder="输入图片中的验证码"
                    autoComplete="off"
                    disabled={checking || submitting}
                  />
                </label>
              </div>
            ) : null}
            {errorMessage ? <p className="admin-auth-form__error">{errorMessage}</p> : null}
            <Button type="submit" className="w-full" disabled={!canSubmit}>
              {checking || submitting ? <LoaderCircle className="admin-auth-form__submit-loader" size={14} /> : null}
              <span>{submitText}</span>
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
