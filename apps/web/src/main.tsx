import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import "./styles.css";

// 开发模式关闭 StrictMode 双挂载，避免编辑器滚动容器在调试时反复重建。
const appRoot = import.meta.env.DEV ? (
  <App />
) : (
  <StrictMode>
    <App />
  </StrictMode>
);

createRoot(document.getElementById("root")!).render(appRoot);
