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
npm run web:dev
```

默认地址：`http://localhost:5173`

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
