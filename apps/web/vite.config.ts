import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react()],
  // 供 mathjax-full 在浏览器构建中读取版本号，避免触发 Node require 分支。
  define: {
    PACKAGE_VERSION: JSON.stringify("3.2.1")
  },
  server: {
    port: 3000,
    host: "0.0.0.0"
  }
});
