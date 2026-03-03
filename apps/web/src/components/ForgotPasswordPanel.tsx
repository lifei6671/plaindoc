import { LoaderCircle, Mail } from "lucide-react";
import { useCallback, useMemo, useState, type FormEvent } from "react";
import { Badge } from "./ui/badge";
import { Button } from "./ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "./ui/card";
import { Input } from "./ui/input";

interface ForgotPasswordPanelProps {
  submitting: boolean;
  errorMessage: string | null;
  loginPath: string;
  onSubmit: (email: string) => Promise<void>;
}

export function ForgotPasswordPanel({
  submitting,
  errorMessage,
  loginPath,
  onSubmit
}: ForgotPasswordPanelProps) {
  const [email, setEmail] = useState("");
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  const canSubmit = useMemo(() => {
    return !submitting && email.trim().length > 0;
  }, [email, submitting]);

  const handleSubmit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (!canSubmit) {
        return;
      }
      await onSubmit(email.trim());
      setSuccessMessage("如果邮箱存在，系统将发送重置链接，请注意查收。");
    },
    [canSubmit, email, onSubmit]
  );

  return (
    <div className="admin-auth-page">
      <Card className="mx-auto w-full max-w-md border-slate-200 shadow-xl">
        <CardHeader className="space-y-4 pb-4">
          <Badge variant="outline" className="w-fit gap-1 border-cyan-200 bg-cyan-50 text-cyan-700">
            <Mail size={13} />
            <span>找回密码</span>
          </Badge>
          <div className="space-y-2">
            <CardTitle className="text-2xl tracking-tight">邮箱找回密码</CardTitle>
            <CardDescription>输入注册邮箱，我们将发送重置密码链接。</CardDescription>
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
                placeholder="name@example.com"
                autoComplete="email"
                disabled={submitting}
              />
            </label>

            {successMessage ? <p className="text-xs text-emerald-700">{successMessage}</p> : null}
            {errorMessage ? <p className="admin-auth-form__error">{errorMessage}</p> : null}

            <Button type="submit" className="w-full" disabled={!canSubmit}>
              {submitting ? <LoaderCircle className="admin-auth-form__submit-loader" size={14} /> : null}
              <span>{submitting ? "提交中..." : "发送重置邮件"}</span>
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
