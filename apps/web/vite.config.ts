import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig, loadEnv } from "vite";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, ".", "");
  const backendOrigin = (env.VITE_DEV_PROXY_TARGET || "http://localhost:8080").trim();
  // 仅 Web 路由由 Vite + React Router 处理，其余路径统一转发给后端 SSR。
  const fallbackToBackendPattern = "^/(?!api(?:[/?]|$)|login(?:[/?]|$)|register(?:[/?]|$)|editor(?:[/?]|$)|admin(?:[/?]|$)|@vite(?:/|$)|@react-refresh(?:$|\\?)|@fs(?:/|$)|@id(?:/|$)|__vite_ping(?:$|\\?)|src(?:/|$)|node_modules(?:/|$)).*";

  return {
    plugins: [tailwindcss(), react()],
    // 供 mathjax-full 在浏览器构建中读取版本号，避免触发 Node require 分支。
    define: {
      PACKAGE_VERSION: JSON.stringify("3.2.1")
    },
    server: {
      port: 3001,
      host: "0.0.0.0",
      proxy: {
        "/api": {
          target: backendOrigin,
          changeOrigin: true,
          xfwd: true,
          secure: false
        },
        "/r": {
          target: backendOrigin,
          changeOrigin: true,
          xfwd: true,
          secure: false
        },
        "/uploads": {
          target: backendOrigin,
          changeOrigin: true,
          xfwd: true,
          secure: false
        },
        [fallbackToBackendPattern]: {
          target: backendOrigin,
          changeOrigin: true,
          xfwd: true,
          secure: false
        }
      }
    }
  };
});
