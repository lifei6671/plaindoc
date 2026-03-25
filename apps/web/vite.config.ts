import { resolve } from "node:path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig, loadEnv } from "vite";

export default defineConfig(({ mode }) => {
  const appEntryPath = resolve(__dirname, "index.html");
  const readerMermaidRuntimeEntryPath = resolve(
    __dirname,
    "src/ssr/reader-mermaid-runtime.js"
  );
  const readerImageViewerRuntimeEntryPath = resolve(
    __dirname,
    "src/ssr/reader-image-viewer-runtime.js"
  );
  const env = loadEnv(mode, ".", "");
  const backendOrigin = (env.VITE_DEV_PROXY_TARGET || "http://localhost:8080").trim();
  // 仅 Web 路由由 Vite + React Router 处理，其余路径统一转发给后端 SSR。
  const fallbackToBackendPattern = "^/(?!api(?:[/?]|$)|login(?:[/?]|$)|register(?:[/?]|$)|forgot-password(?:[/?]|$)|reset-password(?:[/?]|$)|editor(?:[/?]|$)|admin(?:[/?]|$)|@vite(?:/|$)|@react-refresh(?:$|\\?)|@fs(?:/|$)|@id(?:/|$)|__vite_ping(?:$|\\?)|src(?:/|$)|node_modules(?:/|$)).*";

  return {
    plugins: [tailwindcss(), react()],
    // 供 mathjax-full 在浏览器构建中读取版本号，避免触发 Node require 分支。
    define: {
      PACKAGE_VERSION: JSON.stringify("3.2.1")
    },
    build: {
      // 与首页 SSR 模板静态资源路由 /assets 隔离，避免构建产物冲突。
      assetsDir: "web-assets",
      rollupOptions: {
        input: {
          app: appEntryPath,
          readerMermaidRuntime: readerMermaidRuntimeEntryPath,
          readerImageViewerRuntime: readerImageViewerRuntimeEntryPath
        },
        output: {
          entryFileNames: (chunkInfo) => {
            const facadeModuleID = chunkInfo.facadeModuleId ?? "";
            if (facadeModuleID === readerMermaidRuntimeEntryPath) {
              return "web-assets/reader-mermaid-runtime.js";
            }
            if (facadeModuleID === readerImageViewerRuntimeEntryPath) {
              return "web-assets/reader-image-viewer-runtime.js";
            }
            return "web-assets/[name]-[hash].js";
          }
        }
      }
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
        "^/r(?:[/?]|$)": {
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
