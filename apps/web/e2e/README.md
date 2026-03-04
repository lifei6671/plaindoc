# Playwright E2E（文档模板）

## 目录说明

1. `admin-document-templates.smoke.spec.ts`：后台模板管理 CRUD 冒烟。
2. `workspace-document-template-create.smoke.spec.ts`：编辑器创建文档时选择模板 + 自定义标识冒烟。

## 本地运行

1. 安装依赖（仓库根目录）：

```bash
npm install
```

2. 安装 Chromium（首次）：

```bash
npm run web:e2e:install
```

3. 运行 E2E：

```bash
npm run web:e2e
```

4. 仅查看用例清单（不执行）：

```bash
npm run web:e2e:list
```

## 运行模式

1. 默认（无 `PLAINDOC_E2E_BASE_URL`）：
   - Playwright 会自动启动 `apps/web` 的 Vite 开发服务（`127.0.0.1:4173`）。
2. 外部环境：
   - 设置 `PLAINDOC_E2E_BASE_URL` 后，Playwright 不再拉起本地 dev server，改为直连目标环境。

## 说明

1. 当前用例使用 `route` 拦截 `**/api/**`，以隔离后端依赖，优先保证前端关键交互回归。
2. 若接入真实后端集成回归，可复用同名场景并替换为测试专用 seed 数据。
