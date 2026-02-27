import { LoaderCircle, LogIn, UserPlus } from "lucide-react";
import { useCallback, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { Badge } from "./ui/badge";
import { Button } from "./ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "./ui/card";
import { Input } from "./ui/input";

type AuthMode = "login" | "register";

interface AuthPanelProps {
  mode: AuthMode;
  switchPath: string;
  redirectTarget: string | null;
  checking: boolean;
  submitting: boolean;
  errorMessage: string | null;
  onLogin: (input: { email: string; password: string }) => Promise<void>;
  onRegister: (input: { name: string; email: string; password: string }) => Promise<void>;
}

// 登录注册入口：独立于编辑器主视图，避免未登录状态触发业务加载链路。
export function AuthPanel({
  mode,
  switchPath,
  redirectTarget,
  checking,
  submitting,
  errorMessage,
  onLogin,
  onRegister
}: AuthPanelProps) {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  const validationErrorMessage = useMemo(() => {
    if (mode !== "register") {
      return null;
    }
    if (!confirmPassword) {
      return null;
    }
    if (password !== confirmPassword) {
      return "两次输入的密码不一致";
    }
    return null;
  }, [confirmPassword, mode, password]);

  const canSubmit = useMemo(() => {
    if (checking || submitting) {
      return false;
    }
    if (!email.trim() || !password.trim()) {
      return false;
    }
    if (mode === "register" && !name.trim()) {
      return false;
    }
    if (mode === "register") {
      if (!confirmPassword) {
        return false;
      }
      if (password !== confirmPassword) {
        return false;
      }
    }
    return true;
  }, [checking, confirmPassword, email, mode, name, password, submitting]);

  const submitText = useMemo(() => {
    if (checking) {
      return "检查会话中...";
    }
    if (submitting) {
      return mode === "login" ? "登录中..." : "注册中...";
    }
    return mode === "login" ? "登录" : "注册";
  }, [checking, mode, submitting]);

  const switchActionText = mode === "login" ? "注册新账号" : "去登录";
  const switchPromptText = mode === "login" ? "还没有账号？" : "已有账号？";
  const headingText = mode === "login" ? "PlainDoc 登录" : "PlainDoc 注册";
  const introText = mode === "login" ? "登录后开始编辑文档" : "注册后创建你的文档空间";
  const badgeText = mode === "login" ? "账号登录" : "账号注册";
  const switchHref = useMemo(() => {
    if (!redirectTarget) {
      return switchPath;
    }
    return `${switchPath}?redirect=${encodeURIComponent(redirectTarget)}`;
  }, [redirectTarget, switchPath]);

  const handleSubmit = useCallback(async (event?: FormEvent<HTMLFormElement>) => {
    event?.preventDefault();
    if (!canSubmit) {
      return;
    }
    if (mode === "login") {
      await onLogin({
        email: email.trim(),
        password
      });
      setPassword("");
      return;
    }
    await onRegister({
      name: name.trim(),
      email: email.trim(),
      password
    });
    setPassword("");
    setConfirmPassword("");
  }, [canSubmit, email, mode, name, onLogin, onRegister, password]);

  return (
    <div className="admin-auth-page">
      <Card className="mx-auto w-full max-w-md border-slate-200 shadow-xl">
        <CardHeader className="space-y-4 pb-4">
          <Badge variant="outline" className="w-fit gap-1 border-cyan-200 bg-cyan-50 text-cyan-700">
            {mode === "login" ? <LogIn size={13} /> : <UserPlus size={13} />}
            <span>{badgeText}</span>
          </Badge>
          <div className="space-y-2">
            <CardTitle className="text-2xl tracking-tight">{headingText}</CardTitle>
            <CardDescription>{introText}</CardDescription>
          </div>
        </CardHeader>
        <CardContent>
          <form className="space-y-4" onSubmit={(event) => void handleSubmit(event)}>
            {mode === "register" ? (
              <label className="admin-auth-form__field">
                <span>昵称</span>
                <Input
                  type="text"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  placeholder="输入昵称"
                  autoComplete="name"
                  disabled={checking || submitting}
                />
              </label>
            ) : null}

            <label className="admin-auth-form__field">
              <span>邮箱</span>
              <Input
                type="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="name@example.com"
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
                placeholder="至少 6 位"
                autoComplete={mode === "login" ? "current-password" : "new-password"}
                disabled={checking || submitting}
              />
            </label>

            {mode === "register" ? (
              <label className="admin-auth-form__field">
                <span>确认密码</span>
                <Input
                  type="password"
                  value={confirmPassword}
                  onChange={(event) => setConfirmPassword(event.target.value)}
                  placeholder="再次输入密码"
                  autoComplete="new-password"
                  disabled={checking || submitting}
                />
              </label>
            ) : null}

            {validationErrorMessage ? <p className="admin-auth-form__error">{validationErrorMessage}</p> : null}
            {!validationErrorMessage && errorMessage ? <p className="admin-auth-form__error">{errorMessage}</p> : null}

            <Button type="submit" className="w-full" disabled={!canSubmit}>
              {checking || submitting ? <LoaderCircle className="admin-auth-form__submit-loader" size={14} /> : null}
              <span>{submitText}</span>
            </Button>

            <p className="text-center text-sm text-slate-600">
              <span>{switchPromptText}</span>
              <a href={switchHref} className="ml-2 font-medium text-cyan-700 hover:text-cyan-800 hover:underline">
                {switchActionText}
              </a>
            </p>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
