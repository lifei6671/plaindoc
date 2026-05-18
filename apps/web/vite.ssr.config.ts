import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import { createCodeMirrorResolve } from "./vite.codemirror-dedupe";

export default defineConfig({
  plugins: [react()],
  resolve: createCodeMirrorResolve(),
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
    // SSR 构建默认不产出静态资源；这里显式开启，确保字体/KaTeX 资源真实落盘。
    ssrEmitAssets: true,
    // 每次构建前清理输出目录，避免历史 hash 文件造成“引用存在但文件缺失/混淆”。
    emptyOutDir: true,
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
