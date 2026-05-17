package server

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
)

const startupUnavailableCode = 50300

// NewStartupHandler 创建启动期 HTTP handler。它不能依赖数据库、业务服务或 SPA 产物。
func NewStartupHandler(state *StartupState) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSpace(r.URL.Path)
		switch {
		case path == "/api/startup/status":
			writeStartupJSON(w, http.StatusOK, state.Snapshot())
			return
		case path == "/api/healthz":
			writeStartupHealth(w, state.Snapshot())
			return
		case strings.HasPrefix(path, "/api/"):
			writeStartupAPIUnavailable(w, state.Snapshot())
			return
		default:
			writeStartupHTML(w, state.Snapshot())
			return
		}
	})
}

func writeStartupHealth(w http.ResponseWriter, snapshot StartupSnapshot) {
	statusCode := http.StatusServiceUnavailable
	status := "starting"
	if snapshot.Ready {
		statusCode = http.StatusOK
		status = "ok"
	}
	if snapshot.Failed {
		statusCode = http.StatusInternalServerError
		status = "failed"
	}
	writeStartupJSON(w, statusCode, map[string]any{
		"status":  status,
		"startup": snapshot,
	})
}

func writeStartupAPIUnavailable(w http.ResponseWriter, snapshot StartupSnapshot) {
	writeStartupJSON(w, http.StatusServiceUnavailable, map[string]any{
		"code":      startupUnavailableCode,
		"message":   "服务正在初始化，请稍后重试",
		"requestId": "",
		"data": map[string]any{
			"phase":  snapshot.Phase,
			"ready":  snapshot.Ready,
			"failed": snapshot.Failed,
		},
	})
}

func writeStartupJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(value)
}

func writeStartupHTML(w http.ResponseWriter, snapshot StartupSnapshot) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, renderStartupHTML(snapshot))
}

func renderStartupHTML(snapshot StartupSnapshot) string {
	title := "PlainDoc 正在初始化"
	message := html.EscapeString(snapshot.Message)
	current := "准备中"
	if snapshot.CurrentVersion > 0 || strings.TrimSpace(snapshot.CurrentName) != "" {
		current = html.EscapeString(fmt.Sprintf("%04d %s", snapshot.CurrentVersion, snapshot.CurrentName))
	}
	progressText := "正在准备启动任务"
	progressPercent := 8
	if snapshot.TotalCount > 0 {
		progressText = html.EscapeString(fmt.Sprintf("已完成 %d / 共 %d", snapshot.AppliedCount, snapshot.TotalCount))
		progressPercent = int(float64(snapshot.AppliedCount) / float64(snapshot.TotalCount) * 100)
		if progressPercent < 8 {
			progressPercent = 8
		}
		if progressPercent > 100 {
			progressPercent = 100
		}
	}
	failedClass := ""
	if snapshot.Failed {
		failedClass = " is-failed"
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
  <style>
    :root { color-scheme: light; font-family: "Google Sans Code", "PingFang SC", "Microsoft YaHei", Arial, sans-serif; }
    * { box-sizing: border-box; }
    body { margin: 0; min-height: 100vh; display: grid; place-items: center; background: #f8fafc; color: #0f172a; }
    main { width: min(680px, calc(100vw - 32px)); border: 1px solid #dbe2ea; border-radius: 12px; background: #fff; padding: 28px; box-shadow: 0 18px 50px rgba(15, 23, 42, 0.10); }
    h1 { margin: 0; font-size: 24px; line-height: 1.25; }
    p { margin: 10px 0 0; color: #475569; line-height: 1.7; }
    dl { margin: 22px 0 0; display: grid; gap: 10px; grid-template-columns: 120px minmax(0, 1fr); font-size: 14px; }
    dt { color: #64748b; }
    dd { margin: 0; min-width: 0; overflow-wrap: anywhere; }
    .bar { margin-top: 18px; height: 10px; overflow: hidden; border-radius: 999px; background: #e2e8f0; }
    .fill { width: %d%%; height: 100%%; border-radius: inherit; background: #0f172a; transition: width .25s ease; }
    .hint { margin-top: 18px; font-size: 13px; }
    .is-failed .fill { background: #dc2626; }
    .is-failed h1 { color: #991b1b; }
  </style>
</head>
<body>
  <main class="%s">
    <h1>%s</h1>
    <p id="startup-message">%s</p>
    <div class="bar" aria-hidden="true"><div class="fill"></div></div>
    <dl>
      <dt>当前阶段</dt><dd id="startup-phase">%s</dd>
      <dt>迁移进度</dt><dd id="startup-progress">%s</dd>
      <dt>当前迁移</dt><dd id="startup-current">%s</dd>
    </dl>
    <p class="hint">首次启动或版本升级需要完成数据库初始化，请勿关闭服务。页面会在服务就绪后自动刷新。</p>
  </main>
  <script>
    async function pollStartupStatus() {
      try {
        const response = await fetch("/api/startup/status", { cache: "no-store" });
        if (response.status === 404 || response.status === 405) {
          window.location.reload();
          return;
        }
        if (!response.ok) {
          throw new Error("startup status request failed");
        }
        const status = await response.json();
        if (status.ready) {
          window.location.reload();
          return;
        }
        if (status.failed) {
          document.querySelector("main").classList.add("is-failed");
          document.getElementById("startup-message").textContent = status.message || "初始化失败，请查看服务日志。";
          return;
        }
      } catch (error) {
        // 保持当前静态状态即可，下一轮轮询继续尝试。
      }
      window.setTimeout(pollStartupStatus, 1000);
    }
    window.setTimeout(pollStartupStatus, 1000);
  </script>
</body>
</html>`, title, progressPercent, failedClass, title, message, html.EscapeString(string(snapshot.Phase)), progressText, current)
}
