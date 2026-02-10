# 基于 React + Vite + CodeMirror 的 Markdown 编辑器开发排期

> 目标：按里程碑分阶段实现一个类似语雀体验的 Markdown 编辑器，支持流程图、甘特图、公式渲染与代码高亮。

## 1. 项目节奏与周期建议

- 总周期：6 周（可压缩为 4 周，或扩展为 8 周）
- 交付方式：每周一个主要里程碑，周中联调，周末验收
- 节奏建议：
  - 周一：需求确认与任务拆分
  - 周二~周四：开发与联调
  - 周五：测试、修复与里程碑验收

---

## 2. 里程碑开发排期（按周）

## 第 1 周：里程碑 1（项目初始化 + 双栏布局）

### 本周目标

- 初始化 React + Vite + TypeScript 项目
- 接入 CodeMirror 6 作为 Markdown 编辑器
- 实现基础双栏布局（左编辑 / 右预览占位）

### 任务拆解

- Day 1
  - 初始化工程、统一目录结构
  - 配置 ESLint / Prettier（如你计划使用）
- Day 2
  - 封装 `MarkdownEditor` 组件（CodeMirror 基础配置）
  - 绑定文本状态（受控或半受控）
- Day 3
  - 完成双栏布局与响应式适配（桌面优先）
  - 增加预览占位区域
- Day 4
  - 编辑器主题与基础快捷键（Tab、撤销重做）
  - 处理最小可用错误边界
- Day 5
  - 里程碑验收与文档补充

### 验收标准

- 能输入 Markdown
- 页面具备稳定的双栏编辑界面
- 基础状态更新链路可用

---

## 第 2 周：里程碑 2（Markdown 渲染管线）

### 本周目标

- 打通 unified 渲染链路
- 支持 GFM（表格、任务列表、删除线）
- 增加基础安全策略（sanitize）

### 任务拆解

- Day 1
  - 接入 `unified + remark-parse + remark-gfm`
- Day 2
  - 接入 `remark-rehype + rehype-stringify`
- Day 3
  - 接入 `rehype-sanitize`，确定白名单策略
- Day 4
  - 完成预览组件与样式（标题、列表、表格）
- Day 5
  - 测试常见 Markdown 样例并修复问题

### 验收标准

- 常见 Markdown 语法渲染正确
- 非法 HTML 不会直接执行

---

## 第 3 周：里程碑 3（代码块高亮）

### 本周目标

- 支持 fenced code block 语言识别
- 接入代码高亮（建议 MVP 用 `highlight.js`）

### 任务拆解

- Day 1
  - 设计代码块渲染策略（语言识别、兜底）
- Day 2
  - 集成 `highlight.js` 并接入预览链路
- Day 3
  - 增加代码主题、行内代码样式统一
- Day 4
  - 处理未知语言与空代码块边界
- Day 5
  - 回归测试与性能小优化（避免重复高亮）

### 验收标准

- ` ```lang ` 代码块按语言高亮
- 未识别语言时优雅降级

---

## 第 4 周：里程碑 4（公式 + 流程图/甘特图）

### 本周目标

- 支持公式（KaTeX）
- 支持 Mermaid（流程图、甘特图）

### 任务拆解

- Day 1
  - 集成 `remark-math + rehype-katex + katex`
- Day 2
  - 验证 `$...$` 与 `$$...$$` 渲染效果
- Day 3
  - 集成 Mermaid 渲染流程（code block -> SVG）
- Day 4
  - 支持 `flowchart`、`gantt` 示例并补齐样式
- Day 5
  - 增加 Mermaid 渲染失败兜底与错误提示

### 验收标准

- 公式可稳定渲染
- 流程图与甘特图可稳定渲染
- 渲染失败不影响整体页面可用性

---

## 第 5 周：里程碑 5（编辑体验增强）

### 本周目标

- 接近语雀基础写作体验
- 增强可用性：工具栏、滚动同步、自动保存

### 任务拆解

- Day 1
  - 工具栏（标题/加粗/斜体/代码块/引用）
- Day 2
  - 快捷键与命令系统整合
- Day 3
  - 编辑区与预览区滚动同步
- Day 4
  - 本地草稿保存（`localforage`）
- Day 5
  - 大纲导航（按标题提取）与可用性修正

### 验收标准

- 写作链路完整顺滑
- 关闭页面后可恢复草稿
- 大纲可点击跳转

---

## 第 6 周：里程碑 6（工程化 + 质量保障）

### 本周目标

- 完成模块化重构与基础测试
- 补齐性能优化与发布准备

### 任务拆解

- Day 1
  - 目录重构（editor/parser/renderers/features）
- Day 2
  - 增加 parser 单元测试（关键语法覆盖）
- Day 3
  - 增加预览组件行为测试
- Day 4
  - 性能优化（防抖、节流、必要时懒渲染）
- Day 5
  - 发布前检查与最终验收

### 验收标准

- 关键模块结构清晰，便于扩展
- 核心渲染能力具备测试覆盖
- 具备上线/内部试用条件

---

## 3. 依赖包建议（分阶段安装）

### 第 1~2 周（基础能力）

- `react react-dom`
- `vite @vitejs/plugin-react typescript`
- `codemirror @codemirror/state @codemirror/view @codemirror/commands @codemirror/language @codemirror/lang-markdown @codemirror/search @codemirror/history @codemirror/autocomplete`
- `unified remark-parse remark-gfm remark-rehype rehype-stringify rehype-sanitize`

### 第 3 周（代码高亮）

- `highlight.js`

### 第 4 周（公式与图表）

- `remark-math rehype-katex katex`
- `mermaid`

### 第 5~6 周（体验增强）

- `localforage`
- `clsx`（可选）
- `react-use`（可选）

---

## 4. 风险与预案

- Mermaid 与 React 生命周期冲突
  - 预案：在预览 HTML 更新后统一触发渲染，做好销毁与重绘控制
- 预览渲染性能抖动
  - 预案：输入防抖（如 150~300ms）、大文档分段渲染
- XSS 风险
  - 预案：严格 `rehype-sanitize` 白名单；必要时叠加 `DOMPurify`
- 样式冲突（KaTeX / Markdown / 代码主题）
  - 预案：为预览容器加作用域 class，统一样式层级

---

## 5. 每周评审清单（建议）

- 功能完成度：是否达到里程碑验收标准
- 稳定性：是否存在阻断型缺陷
- 性能：输入延迟、渲染耗时是否可接受
- 可维护性：模块边界是否清晰、命名是否一致
- 下周风险：是否有依赖阻塞、是否需提前预研

---

## 6. 最小可交付版本（MVP）定义

满足以下条件即可进入试用：

- Markdown 基础语法 + GFM 可用
- 代码块高亮可用
- 公式可用（行内 + 块级）
- Mermaid 流程图 + 甘特图可用
- 工具栏 + 自动保存 + 滚动同步可用

> 建议在第 5 周结束时发布 MVP 内测版，第 6 周做工程化和质量加固。

