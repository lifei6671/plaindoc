# plaindoc

一个前后端同仓的 Markdown 编辑器项目（Monorepo）。

## 目录结构

```text
.
├── apps
│   ├── server   # Go + Gin API
│   └── web      # React + Vite 编辑器前端
└── docs
```

## 前端初始化（React + Vite + CodeMirror）

```bash
npm install
cp apps/web/.env.example apps/web/.env
npm run web:dev
```

默认地址：`http://localhost:5173`

- `VITE_DATA_DRIVER=local`：纯前端本地模式（默认）
- `VITE_DATA_DRIVER=http`：切换到后端 API 模式

## 预览样式覆盖（Markdown 渲染主题）

前端预览区提供了稳定选择器，方便你定义自定义主题并覆盖默认样式：

- 预览容器 ID：`#plaindoc-preview-pane`
- 预览容器类：`.plaindoc-preview-pane`、`.plaindoc-preview-pane--default`
- 预览正文类：`.plaindoc-preview-body`

默认使用内置样式。若你需要外部覆盖，可使用以下任一方式注入 CSS：

1. 启动前在全局注入：

```js
window.__PLAINDOC_PREVIEW_STYLE__ = `
#plaindoc-preview-pane .plaindoc-preview-body {
  --pd-preview-text-color: #334155;
  --pd-preview-link-color: #0f766e;
}
`;
```

2. 运行时派发事件：

```js
window.dispatchEvent(
  new CustomEvent("plaindoc:preview-style-change", {
    detail: `
      #plaindoc-preview-pane .plaindoc-preview-body h2 {
        border-bottom-color: #0f766e;
      }
    `
  })
);
```

说明：

- 自定义样式会覆盖内置样式，并自动持久化到 `localStorage`（键名：`plaindoc.preview.custom-style`）。
- 若 `detail` 传空字符串，会清空自定义覆盖并恢复默认主题。

## 后端初始化（Go + Gin）

```bash
cd apps/server
cp .env.example .env
go mod tidy
go run ./cmd/server
```

默认地址：`http://localhost:8080`

健康检查接口：`GET /api/healthz`

## 下一步

- 接入登录注册接口（JWT）
- 接入空间/目录/文档数据模型
- 接入文档版本冲突检测与本地历史
