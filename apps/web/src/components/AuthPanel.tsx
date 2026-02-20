import { LoaderCircle } from "lucide-react";
import { useCallback, useMemo, useState } from "react";

type AuthMode = "login" | "register";

interface AuthPanelProps {
  checking: boolean;
  submitting: boolean;
  errorMessage: string | null;
  onLogin: (input: { email: string; password: string }) => Promise<void>;
  onRegister: (input: { name: string; email: string; password: string }) => Promise<void>;
}

// 登录注册入口：独立于编辑器主视图，避免未登录状态触发业务加载链路。
export function AuthPanel({
  checking,
  submitting,
  errorMessage,
  onLogin,
  onRegister
}: AuthPanelProps) {
  const [mode, setMode] = useState<AuthMode>("login");
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

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
    return true;
  }, [checking, email, mode, name, password, submitting]);

  const submitText = useMemo(() => {
    if (checking) {
      return "检查会话中...";
    }
    if (submitting) {
      return mode === "login" ? "登录中..." : "注册中...";
    }
    return mode === "login" ? "登录" : "注册";
  }, [checking, mode, submitting]);

  const switchMode = useCallback((nextMode: AuthMode) => {
    setMode(nextMode);
  }, []);

  const handleSubmit = useCallback(async () => {
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
  }, [canSubmit, email, mode, name, onLogin, onRegister, password]);

  return (
    <div className="auth-page">
      <div className="auth-card">
        <div className="auth-header">
          <h1>PlainDoc</h1>
          <p>{mode === "login" ? "登录后开始编辑文档" : "注册后创建你的文档空间"}</p>
        </div>

        <div className="auth-mode-switch" role="tablist" aria-label="登录或注册">
          <button
            type="button"
            role="tab"
            aria-selected={mode === "login"}
            className={`auth-mode-switch__button ${mode === "login" ? "auth-mode-switch__button--active" : ""}`}
            onClick={() => switchMode("login")}
            disabled={checking || submitting}
          >
            登录
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={mode === "register"}
            className={`auth-mode-switch__button ${mode === "register" ? "auth-mode-switch__button--active" : ""}`}
            onClick={() => switchMode("register")}
            disabled={checking || submitting}
          >
            注册
          </button>
        </div>

        <div className="auth-form">
          {mode === "register" ? (
            <label className="auth-form__field">
              <span>昵称</span>
              <input
                type="text"
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="输入昵称"
                autoComplete="name"
                disabled={checking || submitting}
              />
            </label>
          ) : null}

          <label className="auth-form__field">
            <span>邮箱</span>
            <input
              type="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              placeholder="name@example.com"
              autoComplete="email"
              disabled={checking || submitting}
            />
          </label>

          <label className="auth-form__field">
            <span>密码</span>
            <input
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              placeholder="至少 6 位"
              autoComplete={mode === "login" ? "current-password" : "new-password"}
              disabled={checking || submitting}
            />
          </label>

          {errorMessage ? <p className="auth-form__error">{errorMessage}</p> : null}

          <button
            type="button"
            className="auth-form__submit"
            onClick={() => void handleSubmit()}
            disabled={!canSubmit}
          >
            {checking || submitting ? <LoaderCircle className="auth-form__submit-loader" size={14} /> : null}
            <span>{submitText}</span>
          </button>
        </div>
      </div>
    </div>
  );
}
