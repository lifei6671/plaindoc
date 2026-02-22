import { LoaderCircle, ShieldCheck } from "lucide-react";
import { useCallback, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { Button } from "./ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "./ui/card";
import { Input } from "./ui/input";
import { Badge } from "./ui/badge";

interface AdminAuthPanelProps {
  checking: boolean;
  submitting: boolean;
  errorMessage: string | null;
  onLogin: (input: { email: string; password: string }) => Promise<void>;
}

// 管理后台登录面板：复用现有账号体系，仅允许登录入口（不提供注册）。
export function AdminAuthPanel({ checking, submitting, errorMessage, onLogin }: AdminAuthPanelProps) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  const canSubmit = useMemo(() => {
    if (checking || submitting) {
      return false;
    }
    return Boolean(email.trim() && password.trim());
  }, [checking, email, password, submitting]);

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
      password
    });
    setPassword("");
  }, [canSubmit, email, onLogin, password]);

  return (
    <div className="admin-auth-page">
      <Card className="mx-auto w-full max-w-md border-slate-200 shadow-xl">
        <CardHeader className="space-y-4 pb-4">
          <Badge variant="outline" className="w-fit gap-1 border-cyan-200 bg-cyan-50 text-cyan-700">
            <ShieldCheck size={13} />
            <span>Admin Console</span>
          </Badge>
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
