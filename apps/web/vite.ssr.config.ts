import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react()],
  define: {
    PACKAGE_VERSION: JSON.stringify("3.2.1")
  },
  ssr: {
    noExternal: true
  },
  build: {
    ssr: "src/ssr/worker-entry.ts",
    outDir: "dist-ssr",
    // 与客户端构建保持一致，避免 SSR 内嵌样式产物引用 /assets/* 导致后端路由冲突。
    assetsDir: "web-assets",
    emptyOutDir: false,
    target: "node20",
    minify: false,
    sourcemap: true,
    rollupOptions: {
      output: {
        format: "es",
        entryFileNames: "worker-entry.js"
      }
    }
  }
});
