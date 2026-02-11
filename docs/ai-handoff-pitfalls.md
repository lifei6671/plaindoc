# PlainDoc AI 接手与避坑指南

> 更新时间：2026-02-11  
> 目的：让后续 AI 会话快速理解项目现状，并避免重复踩坑。

## 1. 项目速览（前端重点）

- 仓库结构：Monorepo，前端在 `apps/web`，后端在 `apps/server`。
- 前端技术栈：React + Vite + TypeScript + CodeMirror + ReactMarkdown + react-syntax-highlighter + rehype-raw + rehype-sanitize。
- 当前核心文件：
  - `apps/web/src/App.tsx`：主业务逻辑（编辑、预览、同步滚动、主题、样式详情等）。
  - `apps/web/src/editor/markdown-sanitize.ts`：内嵌 HTML 白名单清洗配置（XSS 防护 + 锚点保留）。
  - `apps/web/src/styles.css`：整体样式与预览区主题样式。
  - `apps/web/src/main.tsx`：应用挂载与 StrictMode 策略。
  - `apps/web/src/data-access/*`：数据访问抽象（本地/HTTP）。

## 2. 当前关键能力

### 2.1 同步滚动（编辑区 <-> 预览区）

- 采用锚点映射 + 二分插值，不是简单滚动比例。
- 预览区锚点来源：`remarkBlockAnchorPlugin` 注入 `data-source-line / data-source-offset`。
- 双向映射表：
  - 编辑区 -> 预览区：`editorToPreviewAnchorsRef`
  - 预览区 -> 编辑区：`previewToEditorAnchorsRef`
- 重建触发：
  - 内容变更
  - 图片 `load/error`
  - `ResizeObserver`（容器和正文）
  - 预览 DOM `MutationObserver`
  - 主题样式或外部覆盖样式变更

### 2.2 预览主题与代码块样式

- 内置主题配置在 `PREVIEW_THEME_TEMPLATES`。
- 每个主题包含：
  - 正文 CSS 变量 `variables`
  - 代码高亮主题 `syntaxTheme`
  - 代码块容器样式 `codeBlockStyle`
  - 代码块文本样式 `codeBlockCodeStyle`
  - 行内代码样式 `inlineCodeStyle`
- 主题样式通过 `<style id="plaindoc-preview-theme-style">` 动态注入。

### 2.3 外部样式覆盖

- 支持三种入口：
  - `window.__PLAINDOC_PREVIEW_STYLE__`
  - `localStorage`（`plaindoc.preview.custom-style`）
  - `CustomEvent("plaindoc:preview-style-change")`
- 覆盖顺序：内置主题在前，外部覆盖在后。

### 2.4 主题菜单与样式详情抽屉（性能敏感）

- 主题菜单与样式详情抽屉状态在 `ThemeMenu` 子组件内维护。
- 目的：避免菜单/抽屉开关触发 `App` 根组件重渲染，影响编辑器和预览性能。
- 下拉中每个主题项右侧有“查看”按钮，可打开对应主题的样式详情抽屉。

### 2.5 Markdown 内嵌 HTML（含安全清洗）

- 已支持 Markdown 中内嵌 HTML 渲染，渲染链路为：
  - `remark`（GFM / Math / 锚点）
  - `rehype-raw`（解析原始 HTML）
  - `rehype-sanitize`（白名单清洗）
  - `rehype-katex`（公式渲染）
- 关键配置：
  - `PREVIEW_MARKDOWN_REHYPE_OPTIONS.allowDangerousHtml = true`
  - `PREVIEW_HTML_SANITIZE_SCHEMA`
- 安全策略（当前）：
  - `href` 仅允许 `http / https / mailto / tel`
  - `src` 仅允许 `http / https`
  - 保留 `className` 与 `data*`（避免滚动锚点属性被清洗）

## 3. 本轮高频坑（问题 -> 根因 -> 正确做法）

### 坑 1：长图场景同步滚动失效

- 根因：
  - 改动时破坏锚点 DOM 属性传递，或重算时机不足（图片异步加载后未重算）。
- 正确做法：
  - `pre/code` 自定义渲染时保留并透传锚点属性。
  - 必须保留图片 `load/error`、`ResizeObserver`、`MutationObserver` 的重算逻辑。

### 坑 2：仅改样式也导致滚动映射漂移

- 根因：
  - 主题切换/外部样式覆盖会改变预览高度，但未触发映射重建。
- 正确做法：
  - 主题样式文本或外部样式文本变化时，主动 `scheduleRebuildScrollAnchors()`。

### 坑 3：主题下拉点击有明显延迟

- 根因：
  - 菜单开关状态在 `App` 内，导致整棵树重渲染（包含较重的 Markdown 渲染与编辑器区域）。
- 正确做法：
  - 将菜单开关状态下沉到独立子组件，并用 `memo` 隔离重渲染。

### 坑 4：抽屉开关再次引入卡顿

- 根因：
  - 抽屉状态一度放回 `App`，菜单/抽屉开关又触发主组件渲染。
- 正确做法：
  - 抽屉状态也放在 `ThemeMenu` 内部；`App` 只接收“主题真正变更”。

### 坑 5：改 CSS 选择器后覆盖链断裂

- 根因：
  - 预览区 ID/Class 变动导致主题变量或外部样式找不到目标节点。
- 正确做法：
  - 保持以下选择器稳定：
    - `#plaindoc-preview-pane`
    - `.plaindoc-preview-pane`
    - `.plaindoc-preview-body`

### 坑 6：代码块样式“可看不可复制”

- 根因：
  - 只用外部 CSS 或依赖 class，复制到第三方平台后易丢样式。
- 正确做法：
  - 代码块关键样式走内联（主题配置 + `SyntaxHighlighter` inline style）。
  - 样式详情抽屉提供可复制的注释化 CSS 模板。

### 坑 7：`npm run web:dev` 后偶发同步失效

- 根因：
  - 开发态 `StrictMode` 双挂载或 HMR 重建会导致旧滚动容器/旧 editorView 引用失效，映射与监听器绑定到旧节点。
- 正确做法：
  - 开发模式禁用 `StrictMode` 双挂载（见 `apps/web/src/main.tsx`）。
  - 在 `App.tsx` 中持续检测编辑器容器和滚动节点变化，节点变更后重新绑定监听并重建锚点映射。

### 坑 8：支持内嵌 HTML 后实时滚动突然失效

- 根因：
  - `rehype-sanitize` 白名单配置不当，清洗时移除了 `remarkBlockAnchorPlugin` 注入的 `data-*` 锚点属性；
  - `use-scroll-sync` 依赖 `data-anchor-index` 聚合锚点，属性丢失后映射退化，表现为同步滚动失效/明显漂移。
- 正确做法：
  - 在 sanitize schema 的 `attributes["*"]` 中使用 `data*` 放行自定义 `data-*` 属性；
  - 保持插件顺序为 `rehype-raw -> rehype-sanitize -> rehype-katex`；
  - 每次改 sanitize 规则后，必须手工验证长文档双向滚动。

## 4. 高风险改动区（请谨慎）

- `apps/web/src/App.tsx` 中以下区域：
  - `remarkBlockAnchorPlugin`
  - `remarkRehypeOptions` / `rehypePlugins`
  - `rebuildScrollAnchors` / `scheduleRebuildScrollAnchors`
  - `markdownComponents`（`pre/code` 自定义渲染）
  - `ThemeMenu`（菜单和抽屉局部状态）
- `apps/web/src/editor/markdown-sanitize.ts`：
  - `PREVIEW_HTML_SANITIZE_SCHEMA`（白名单 + 协议限制）
  - `PREVIEW_MARKDOWN_REHYPE_OPTIONS`
- `apps/web/src/styles.css` 中以下区域：
  - 预览区选择器（`#plaindoc-preview-pane ...`）
  - 主题菜单下拉布局
  - 抽屉层级与遮罩

## 5. 给后续 AI 的改动原则

- 原则 1：菜单/抽屉等 UI 微交互状态尽量放在局部子组件，不要放 `App` 根组件。
- 原则 2：不要改动预览区稳定选择器命名（ID/Class）除非同步调整所有依赖点。
- 原则 3：所有会改变预览高度的行为都要考虑触发滚动映射重建。
- 原则 4：改 `pre/code` 渲染器时必须确认锚点属性仍可落到预览 DOM。
- 原则 5：每次改动后至少执行一次构建和一次手工长图滚动验证。
- 原则 6（强制规范）：所有 AI 生成或修改的代码必须包含中文注释，至少覆盖模块职责、关键函数和复杂分支；不满足该规范的改动视为不合格。
- 原则 7：每一块功能的引入需要考虑是否可以抽离成独立模块，方便后续迭代。
- 原则 8：涉及 `rehype-sanitize` 的改动必须验证“锚点属性保留 + XSS 拦截 + 公式渲染”三件事同时成立。

## 6. 新会话最小验证清单

```bash
npm run build -w @plaindoc/web
```

手工检查建议：

1. 打开长文档并插入长图，验证编辑区与预览区双向滚动同步。
2. 切换主题后，立即再次滚动，确认未漂移。
3. 下拉菜单展开/收起、样式抽屉打开/关闭时，编辑体验无明显卡顿。
4. 代码块和行内代码主题切换后样式明显变化。
5. 样式详情抽屉里能看到“带注释 CSS 模板”，可直接复制。
6. 编写包含内嵌 HTML 的 Markdown（如 `<div class="note">`）可正确渲染，且无脚本执行。
7. 包含恶意链接（如 `javascript:`）时被 sanitize 阻断，不可执行。

## 7. 关键常量速查（便于快速定位）

- 预览容器：
  - `PREVIEW_PANE_ID`
  - `PREVIEW_PANE_CLASS`
  - `PREVIEW_BODY_CLASS`
- 外部样式：
  - `PREVIEW_CUSTOM_STYLE_STORAGE_KEY`
  - `PREVIEW_CUSTOM_STYLE_EVENT`
- 主题：
  - `PREVIEW_THEME_TEMPLATES`
  - `PREVIEW_THEME_STORAGE_KEY`
- HTML 安全渲染：
  - `PREVIEW_HTML_SANITIZE_SCHEMA`
  - `PREVIEW_MARKDOWN_REHYPE_OPTIONS`
- 滚动同步：
  - `BLOCK_ANCHOR_SELECTOR`
  - `rebuildScrollAnchors`
  - `scheduleRebuildScrollAnchors`

## 8. 后续可选优化（非必须）

- 在开发模式增加“渲染次数”调试开关，快速定位意外重渲染。
- 将样式详情抽屉增加“一键复制 CSS 模板”按钮。
- 为同步滚动建立 E2E 回归用例（长图/主题切换/外部样式覆盖组合场景）。
