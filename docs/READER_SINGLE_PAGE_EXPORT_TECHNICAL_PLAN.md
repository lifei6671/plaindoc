# 阅读页单页导出技术方案

**文档状态**: Draft（待评审）  
**创建日期**: 2026-03-03  
**适用范围**: `apps/web`（阅读页 SSR 与前端交互）  
**目标**: 在文档阅读页支持单页导出 Markdown 与 PDF（浏览器打印）。

---

## 1. 需求与范围

### 1.1 目标能力

1. 阅读页支持导出当前文档 Markdown 原文。
2. 导出的 Markdown 中：
   - 图片链接必须是完整 URL；
   - 附件链接必须是完整 URL；
   - 不允许保留相对路径。
3. 阅读页支持导出 PDF：
   - 使用浏览器原生打印（`window.print()`）；
   - 保持阅读区预览样式；
   - 不包含左侧文档树。

### 1.2 非目标（本期不做）

1. 批量导出空间下多文档。
2. 后端异步任务式 PDF 生成。
3. Markdown 导出打包附件二进制文件（zip）。

---

## 2. 现状分析

### 2.1 阅读页渲染链路

1. 路由：`GET /r/:spaceId/:docId`。
2. 后端聚合：`ReaderPageService.BuildPage(...)` 返回 `ReaderPagePayload`。
3. 前端 SSR：`apps/web/src/ssr/render-space-reader.tsx` 输出完整 HTML。
4. 前端增强：`apps/web/src/ssr/render-space-reader.async-script.ts` 负责文档切换、附件按钮、大纲同步等。

### 2.2 可复用数据

阅读页已将以下数据注入 `#plaindoc-reader-state`：

1. `document.contentMd`（当前 Markdown 原文）。
2. `document.id`、`document.title`。
3. `attachments[]`（附件 ID、文档 ID、名称、类型等）。
4. `requestOrigin`（用于 URL 解析的请求来源）。

这意味着 Markdown 导出可在前端直接完成，无需新增后端读取接口。

### 2.3 当前缺口

1. 阅读页无导出入口（UI 按钮）。
2. 无 Markdown 资源链接绝对化逻辑。
3. `render-space-reader.base.css` 尚无 `@media print` 规则，打印会包含侧栏并受滚动容器约束。

---

## 3. 总体方案

采用“前端主导、后端零改动（MVP）”方案：

1. 可在标题下方元数据最后，增加打印和markdown图标用来实现导出markdown和打印PDF：`导出 Markdown`、`导出 PDF`。
2. Markdown 导出流程：
   - 从页面状态提取原始 `contentMd`；
   - 对图片/附件/相对资源链接做绝对化；
   - 生成 `.md` 文件并触发浏览器下载。
3. PDF 导出流程：
   - 点击按钮调用 `window.print()`；
   - 用打印样式隐藏左侧文档树和交互控件，仅保留阅读内容区。

---

## 4. Markdown 导出设计

### 4.1 交互与入口

在 `reader-article-header` 增加导出操作区：

1. `导出 Markdown`：下载当前文档的 `.md` 文件。
2. `导出 PDF`：触发打印。

建议新增样式类：

1. `.reader-article-actions`
2. `.reader-article-action`
3. `.reader-article-action--primary`

### 4.2 导出数据来源

通过 `#plaindoc-reader-state` 解析 payload：

1. `contentMd = payload.document.contentMd`。
2. 文件名基于 `payload.document.title`（空值回退“未命名文档”）。
3. 基准 origin 优先级：
   - `payload.requestOrigin`
   - `window.location.origin`（兜底）

### 4.3 URL 绝对化规则

核心函数：`toAbsoluteResourceURL(raw, baseOrigin)`。

不改写（直接返回）的 URL：

1. 以 `http://`、`https://` 开头。
2. 以 `data:`、`blob:`、`mailto:`、`tel:`、`#` 开头。

需要改写为完整 URL 的 URL：

1. 以 `/` 开头的站内路径（如 `/uploads/...`、`/preview/docs/...`、`/api/attachment-downloads/...`）。
2. 相对路径（如 `./images/a.png`、`../files/a.pdf`）。

改写范围：

1. Markdown 图片语法：`![alt](url)`。
2. Markdown 链接语法：`[text](url)`（用于覆盖附件链接）。
3. Markdown 引用式链接定义：`[id]: url`。
4. 内嵌 HTML 的 `src` / `href` 属性（如 `<img>`、`<a>`）。

### 4.4 可靠性策略

为降低误改写风险：

1. 跳过 fenced code block 与行内代码片段中的文本。
2. 保持“原文优先”：仅替换 URL 字面量，不做 Markdown 结构重排。
3. 失败兜底：若绝对化过程异常，提示错误，不下载损坏内容。

### 4.5 文件名与下载

文件名建议：`<文档标题>-YYYYMMDD-HHmm.md`。

下载实现：

1. `Blob([rewrittenMarkdown], { type: "text/markdown;charset=utf-8" })`
2. `URL.createObjectURL` + 隐式 `<a download>` 触发。

---

## 5. PDF 导出设计（浏览器打印）

### 5.1 触发方式

点击 `导出 PDF` 按钮后执行 `window.print()`。

### 5.2 打印样式改造

在 `apps/web/src/ssr/render-space-reader.base.css` 增加 `@media print`：

1. 隐藏非正文区域：
   - `.reader-sidebar`（左侧文档树）
   - `.reader-outline`
   - `.reader-progress`
   - 导出按钮区与附件操作按钮
2. 释放滚动容器：
   - `body { overflow: visible; background: #fff; }`
   - `.reader-layout { display: block; height: auto; }`
   - `.reader-main { height: auto; overflow: visible; padding: 0; }`
3. 保持阅读区样式：
   - 保留 `.reader-article-shell`、`markdown-body`、主题样式、KaTeX 样式。
4. 打印分页优化：
   - 避免代码块、表格、图片被中间截断（`break-inside: avoid`）。

### 5.3 样式一致性说明

阅读区主题样式本就由 SSR `<style>` 注入（`app/style/base/katex/theme`），打印模式只调整布局与可见性，不改 markdown 视觉规则，因此可保持预览效果一致。

---

## 6. 影响文件与改造点

### 6.1 前端 SSR

1. `apps/web/src/ssr/render-space-reader.tsx`
   - 在文章头部增加导出按钮 DOM。
   - 添加 `data-reader-hook` 供异步脚本绑定。

2. `apps/web/src/ssr/render-space-reader.async-script.ts`
   - 新增导出按钮点击处理。
   - 新增 Markdown 导出逻辑（读取 state、URL 绝对化、下载）。
   - 新增 `window.print()` 触发逻辑。

3. `apps/web/src/ssr/render-space-reader.base.css`
   - 新增导出按钮样式。
   - 新增 `@media print` 打印规则。

### 6.2 后端

MVP 不要求后端接口改造。

---

## 7. 安全与权限

1. 导出仅基于当前已授权阅读页面，不新增越权面。
2. 不主动申请临时下载 token 写入导出文件，避免把短时签名链接固化到 Markdown。
3. 对私有附件，导出的完整 URL 仍受服务端鉴权控制（如预览页路径需要登录权限）。

---

## 8. 测试计划

### 8.1 功能验证

1. 阅读页按钮可见、可点击。
2. 导出 Markdown 后：
   - 图片 URL 全为绝对 URL；
   - 附件 URL 全为绝对 URL；
   - 原文内容结构不变。
3. 导出 PDF 时：
   - 左侧文档树不出现；
   - 阅读区样式与页面一致；
   - 长文档分页可读。

### 8.2 边界用例

1. 文档含相对图片：`./a.png`、`../a.png`、`/uploads/...`。
2. 文档含附件链接：`/preview/docs/...`、`/api/attachment-downloads/...`。
3. 文档含外链、锚点、`mailto:`，确认不被错误改写。
4. 文档含代码块中的伪链接，确认不被改写。
5. `requestOrigin` 缺失时，回退 `window.location.origin`。

### 8.3 兼容性

1. Chrome / Edge（主目标浏览器）。
2. 打印为 PDF 时纸张 A4、默认边距。

---

## 9. 分步实施

1. 第一步：补充阅读页导出按钮 UI（SSR + 样式）。
2. 第二步：实现 Markdown 导出与 URL 绝对化。
3. 第三步：实现 `window.print()` 与打印样式。
4. 第四步：完成手工测试与回归（阅读页切换、附件操作不回归）。

---

## 10. 风险与后续演进

### 10.1 已知风险

1. 纯前端字符串改写覆盖面需要严格测试，防止误处理复杂 Markdown 语法。
2. 浏览器打印表现存在细微差异（尤其分页）。

### 10.2 后续演进（可选）

1. 若需要更强稳定性，可追加后端导出接口：服务端解析并返回已绝对化 Markdown。
2. 若需“可离线附件”，可增加“导出 zip（md + assets）”能力。

