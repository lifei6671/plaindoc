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
