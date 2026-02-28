import { LoaderCircle } from "lucide-react";
import type { FormEvent } from "react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ADMIN_BRAND_LOGO_SRC } from "../admin/brand";
import type { AuthCaptchaChallenge, AuthCaptchaRefreshInput } from "../data-access";
import { Badge } from "./ui/badge";
import { Button } from "./ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "./ui/card";
import { Input } from "./ui/input";

const CAPTCHA_REFRESH_DEBOUNCE_MS = 800;

interface AdminAuthPanelProps {
  checking: boolean;
  submitting: boolean;
  errorMessage: string | null;
  authChallenge: AuthCaptchaChallenge | null;
  onLogin: (input: { email: string; password: string; captchaId?: string; captchaAnswer?: string }) => Promise<void>;
  onRefreshCaptcha: (input: AuthCaptchaRefreshInput) => Promise<void>;
}

// 管理后台登录面板：复用现有账号体系，仅允许登录入口（不提供注册）。
export function AdminAuthPanel({
  checking,
  submitting,
  errorMessage,
  authChallenge,
  onLogin,
  onRefreshCaptcha
}: AdminAuthPanelProps) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [captchaAnswer, setCaptchaAnswer] = useState("");
  const [refreshingCaptcha, setRefreshingCaptcha] = useState(false);
  const refreshDelayTimerRef = useRef<number | null>(null);
  const refreshCancelledRef = useRef(false);

  useEffect(() => {
    return () => {
      refreshCancelledRef.current = true;
      if (refreshDelayTimerRef.current !== null) {
        window.clearTimeout(refreshDelayTimerRef.current);
        refreshDelayTimerRef.current = null;
      }
    };
  }, []);

  useEffect(() => {
    setCaptchaAnswer("");
  }, [authChallenge?.captchaId]);

  const canSubmit = useMemo(() => {
    if (checking || submitting || refreshingCaptcha) {
      return false;
    }
    if (!email.trim() || !password.trim()) {
      return false;
    }
    if (authChallenge && !captchaAnswer.trim()) {
      return false;
    }
    return true;
  }, [authChallenge, captchaAnswer, checking, email, password, refreshingCaptcha, submitting]);

  const canRefreshCaptcha = useMemo(() => {
    if (!authChallenge) {
      return false;
    }
    if (checking || submitting || refreshingCaptcha) {
      return false;
    }
    return email.trim().length > 0;
  }, [authChallenge, checking, email, refreshingCaptcha, submitting]);

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

  const handleRefreshCaptcha = useCallback(async () => {
    if (!authChallenge || !canRefreshCaptcha) {
      return;
    }
    setRefreshingCaptcha(true);
    try {
      await new Promise<void>((resolve) => {
        refreshDelayTimerRef.current = window.setTimeout(() => {
          refreshDelayTimerRef.current = null;
          resolve();
        }, CAPTCHA_REFRESH_DEBOUNCE_MS);
      });
      if (refreshCancelledRef.current) {
        return;
      }
      await onRefreshCaptcha({
        scene: "login",
        identifier: email.trim(),
        captchaId: authChallenge.captchaId
      });
      setCaptchaAnswer("");
    } finally {
      if (!refreshCancelledRef.current) {
        setRefreshingCaptcha(false);
      }
    }
  }, [authChallenge, canRefreshCaptcha, email, onRefreshCaptcha]);

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
                <button
                  type="button"
                  onClick={() => void handleRefreshCaptcha()}
                  disabled={!canRefreshCaptcha}
                  className="inline-block max-w-full disabled:cursor-not-allowed disabled:opacity-70"
                  title="看不清？点击刷新验证码"
                >
                  <div className="flex min-h-14 items-center justify-center">
                    <img
                      src={authChallenge.captchaImageDataUrl}
                      alt="验证码（点击刷新）"
                      className="block h-auto w-auto max-w-full rounded border border-slate-200"
                    />
                  </div>
                </button>
                <p className="text-[11px] text-slate-500">
                  看不清图片可点击刷新{refreshingCaptcha ? "（刷新中...）" : ""}
                </p>
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
