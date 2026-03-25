# 阅读页图片浏览器技术方案

**文档状态**: Completed（已实现）  
**创建日期**: 2026-03-25  
**适用范围**: `apps/web`（阅读页 SSR、异步增强脚本、阅读页样式）  
**目标**: 在文档阅读页提供统一的图片浏览器能力，支持普通图片、正文 inline SVG，以及 Mermaid 渲染后的 SVG 图表。

---

## 1. 目标与范围

### 1.1 目标能力

1. 阅读页文档加载完成后，点击正文任意可浏览图片，打开全屏浮层浏览器。
2. 浮层使用黑色遮罩层，顶部右上角提供关闭按钮。
3. 点击图片之外的浮层空白区域，直接关闭图片浏览器。
3. 浮层底部提供紧凑型工具条，支持：
   - 上一张
   - 下一张
   - 缩小
   - 放大
   - 原始尺寸
   - 旋转
4. 打开的图片支持鼠标拖动平移。
5. 浏览器支持以下媒体类型：
   - Markdown / HTML 正文中的 `img`
   - 正文中的 inline `svg`
   - Mermaid 渲染完成后的 SVG
6. 阅读页内部切换文档后，图片浏览器索引自动刷新，不要求整页刷新。

### 1.2 非目标（本期不做）

1. 鼠标滚轮缩放。
2. 双击缩放。
3. 缩略图列表。
4. 下载图片到本地。
5. 编辑器预览页同步支持图片浏览器。

---

## 2. 现状分析

### 2.1 当前阅读页架构

1. SSR HTML 由 `apps/web/src/ssr/render-space-reader.tsx` 生成。
2. 阅读页交互增强由 `apps/web/src/ssr/render-space-reader.async-script.ts` 注入执行。
3. 文档正文容器统一为 `#plaindoc-preview-body`。
4. 阅读页内部文档切换使用异步抓取 HTML 后替换 `article-shell`，不是整页跳转。
5. Mermaid 渲染采用异步增强方式：
   - SSR 先输出占位容器
   - 浏览器端由 `reader-mermaid-runtime.js` 渲染真实 SVG

### 2.2 当前缺口

1. 阅读页正文中的图片、SVG、Mermaid 图表没有统一浏览能力。
2. Mermaid 是异步生成 SVG，阅读页当前没有“图表渲染完成后重新建索引”的机制。
3. 阅读页没有通用媒体浮层容器，也没有对应的 `data-reader-*` hook 契约。

### 2.3 约束

1. 阅读页异步脚本必须依赖 `data-reader-*` hook，不能依赖视觉 class 作为行为契约。
2. 方案不能破坏当前：
   - 文档树切换
   - 大纲同步
   - Markdown/PDF 导出
   - 附件操作
   - Mermaid 异步渲染
3. 第一版不新增第三方依赖，优先复用现有 SSR + 异步脚本架构。

---

## 3. 总体方案

采用“SSR 输出浏览器壳层 + 前端异步脚本维护状态”的方案。

### 3.1 核心思路

1. 在阅读页 SSR 中预置一个默认隐藏的图片浏览器浮层 DOM。
2. 异步脚本在页面初始化、文档切换完成、Mermaid 渲染完成后扫描正文媒体，建立 gallery 索引。
3. 点击正文媒体后，异步脚本打开浮层，并将当前媒体克隆到浏览区域。
4. 缩放、旋转、切换图片均由异步脚本控制，不新增服务端接口。

### 3.2 设计原则

1. **行为集中**：所有状态收敛到阅读页异步脚本，避免散落到 Markdown 组件层。
2. **媒体统一**：`img`、`svg`、Mermaid SVG 统一抽象成 gallery item。
3. **视图稳定**：切换图片时重置为默认视图，避免把前一张的缩放/旋转污染到下一张。
4. **无侵入**：正文结构尽量少改，只补可识别的 hook。

---

## 4. 交互设计

### 4.1 打开与关闭

1. 点击正文中的目标媒体，打开全屏浮层。
2. 点击右上角关闭按钮可关闭。
3. 点击遮罩空白区可关闭。
4. 按 `Escape` 可关闭。

### 4.2 切换规则

1. 点击上一张 / 下一张切换当前媒体。
2. 切换后重置：
   - 缩放比例恢复到适应视口
   - 旋转角度恢复到 `0deg`
3. 当只有 1 张图片时：
   - 左右切换按钮保留但禁用
   - 工具条计数显示 `1/1`

### 4.3 缩放与旋转

1. 默认打开时按“适应视口”显示。
2. 放大 / 缩小基于当前比例按固定步长调整，建议倍率为 `1.2x`。
3. 原始尺寸按钮把缩放恢复到 `1`。
4. 旋转按钮每次顺时针旋转 `90deg`。
5. 缩放显示文案使用百分比，例如 `100%`、`120%`、`80%`。

---

## 5. UI 视觉规范

### 5.1 浮层

1. 全屏固定定位，遮罩色建议：`rgba(0, 0, 0, 0.88)`。
2. 图片显示区域居中，默认最大尺寸不超过视口：
   - 宽：`calc(100vw - 96px)`
   - 高：`calc(100vh - 180px)`
3. 关闭按钮固定在右上角，独立于底部工具条。

### 5.2 底部工具条

底部工具条视觉要求以用户提供的参考图为准，采用“深色胶囊条 + 分段控件”风格。

工具条规范：

1. 背景为深灰接近黑色，带轻微圆角，整体为横向胶囊条。
2. 图标与文字使用白色或接近白色，保证高对比度。
3. 各功能区之间用细分隔线切开，不做大块按钮边框。
4. 计数与缩放比例是稳定文本位，不随 hover 抖动。
5. 工具条顺序固定为：
   - 上一张
   - 当前序号，例如 `1/6`
   - 下一张
   - 分隔
   - 缩小
   - 当前缩放比例，例如 `100%`
   - 放大
   - 分隔
   - 原始尺寸
   - 分隔
   - 旋转
6. hover 态只做轻量亮度变化，不做花哨动画。
7. 移动端允许工具条换行或压缩间距，但仍保持“深色胶囊条 + 分隔段”的同一视觉语言。

### 5.3 可用性要求

1. 可点击图片默认显示放大光标或可点击态。
2. 工具条按钮需要有 `aria-label`。
3. 禁用态按钮要有明显降噪效果，但不能完全不可见。

---

## 6. DOM 与 Hook 设计

### 6.1 新增 Hook

建议在阅读页 DOM 中新增以下契约：

1. `data-reader-hook="image-viewer"`
2. `data-reader-hook="image-viewer-backdrop"`
3. `data-reader-hook="image-viewer-stage"`
4. `data-reader-hook="image-viewer-content"`
5. `data-reader-hook="image-viewer-close"`
6. `data-reader-hook="image-viewer-prev"`
7. `data-reader-hook="image-viewer-next"`
8. `data-reader-hook="image-viewer-zoom-out"`
9. `data-reader-hook="image-viewer-zoom-in"`
10. `data-reader-hook="image-viewer-original"`
11. `data-reader-hook="image-viewer-rotate"`
12. `data-reader-hook="image-viewer-index"`
13. `data-reader-hook="image-viewer-scale"`

### 6.2 正文媒体识别范围

仅扫描 `#plaindoc-preview-body` 内的媒体节点，不扫描：

1. 侧栏目录
2. 操作按钮
3. 工具图标
4. 附件区按钮

目标媒体：

1. `#plaindoc-preview-body img`
2. `#plaindoc-preview-body svg`
3. `[data-reader-hook='mermaid'] [data-reader-mermaid-diagram='1'] svg`

### 6.3 SVG 过滤规则

为避免把小图标错误识别为浏览内容，建议过滤：

1. 位于 `button`、操作链接、工具栏中的 SVG
2. 宽高都很小的装饰性 SVG
3. 非正文阅读内容区域的 SVG

Mermaid SVG 单独优先识别，不与普通正文小图标混淆。

---

## 7. 状态模型

异步脚本维护以下核心状态：

```ts
type ReaderGalleryItem = {
  id: string;
  kind: "img" | "svg" | "mermaid";
  sourceNode: Element;
  altText: string;
  intrinsicWidth: number;
  intrinsicHeight: number;
};

type ReaderImageViewerState = {
  open: boolean;
  activeIndex: number;
  zoom: number;
  fitScale: number;
  rotation: 0 | 90 | 180 | 270;
};
```

状态规则：

1. `galleryItems` 由正文扫描结果生成。
2. `fitScale` 由当前媒体尺寸与视口尺寸计算。
3. 打开媒体时：
   - `open = true`
   - `activeIndex = 当前媒体索引`
   - `zoom = fitScale`
   - `rotation = 0`
4. 切换媒体时重置 `zoom` 与 `rotation`。

---

## 8. 渲染与刷新时机

### 8.1 初次加载

1. 页面初始化
2. Mermaid 异步渲染完成
3. 构建 galleryItems

### 8.2 阅读页内部切换

阅读页内部切换文档后，必须执行：

1. `replaceArticleShell(...)`
2. `renderReaderMermaidBlocks()`
3. `refreshReaderGalleryRegistry()`

### 8.3 Viewer 打开中的处理

1. 若正在浏览旧文档图片，此时文档被异步切换，建议直接关闭 viewer。
2. 关闭后重建索引，避免索引错位和悬空 DOM 引用。

---

## 9. 影响文件

### 9.1 前端 SSR

1. `apps/web/src/ssr/render-space-reader.tsx`
   - 新增 viewer 壳层 DOM
   - 注入 `data-reader-*` hook

### 9.2 异步增强脚本

1. `apps/web/src/ssr/render-space-reader.async-script.ts`
   - 新增媒体扫描
   - 新增 viewer 状态管理
   - 新增点击、关闭、切换、缩放、旋转、键盘事件
   - 在 Mermaid 渲染完成和文档切换后刷新 gallery registry

### 9.3 样式

1. `apps/web/src/ssr/render-space-reader.base.css`
   - 新增遮罩层、舞台区、关闭按钮、底部工具条样式
   - 新增移动端适配样式
   - 新增 print 隐藏规则（打印时不显示浏览器浮层）

### 9.4 测试

1. `apps/web/src/ssr/render-space-reader.test.tsx`
2. `apps/web/src/ssr/render-space-reader.async-script.test.ts`
3. `apps/web/src/ssr/reader-mermaid-runtime.test.ts`

### 9.5 文档

1. `docs/FRONTEND_DEVELOPER_GUIDE.md`
   - 补充新增 hook 契约
2. `docs/README.md`
   - 补充专题文档导航

---

## 10. 测试计划

### 10.1 功能验证

1. 点击正文普通图片，浮层正常打开。
2. 点击正文 inline SVG，浮层正常打开。
3. 点击 Mermaid 渲染后的 SVG，浮层正常打开。
4. 左右切换、放大、缩小、原始尺寸、旋转按钮均生效。
5. 点击关闭按钮、遮罩、`Escape` 能关闭。
6. 点击图片外部空白区域能关闭。
7. 鼠标拖动图片时，浮层保持打开且舞台滚动位置发生变化。

### 10.2 路由切换回归

1. 阅读页内部切换到下一篇文档后，图片索引正确刷新。
2. 切换后 Mermaid SVG 仍可打开。
3. 浮层打开时触发文档切换，不出现报错或空白内容悬挂。

### 10.3 响应式验证

1. 桌面端工具条单行显示，符合参考图风格。
2. 移动端工具条不溢出屏幕。
3. 竖屏和横屏都能正常操作关闭与切换。

### 10.4 回归项

1. 文档树切换不回归。
2. 导出 Markdown / PDF 不回归。
3. 附件下载 / 预览不回归。
4. Mermaid 异步渲染不回归。

---

## 11. 风险与控制

### 11.1 风险

1. 正文中的小 SVG 图标可能被误识别为可浏览对象。
2. Mermaid SVG 是异步生成，若刷新顺序错误会导致无法点开。
3. 文档切换后旧 galleryItems 可能持有失效节点引用。

### 11.2 控制策略

1. 通过正文范围、尺寸阈值、Mermaid 专属选择器做过滤。
2. 明确在 Mermaid 渲染完成后统一刷新 gallery registry。
3. 在文档切换成功后关闭 viewer 并重建索引。

---

## 12. 分步实施建议

1. 第一步：在 SSR 中加入 viewer 壳层与 hook。
2. 第二步：实现 galleryItems 扫描与普通 `img` 浏览。
3. 第三步：补齐 inline SVG 与 Mermaid SVG 支持。
4. 第四步：实现底部工具条、缩放、原始尺寸、旋转。
5. 第五步：补移动端适配、测试与文档回写。

---

## 13. 当前结论

这是一个前端侧可独立落地的阅读增强功能，不要求新增后端接口。  
第一版建议严格控制范围，只做“图片浏览器基础能力 + Mermaid/SVG 兼容 + 文档切换刷新”，确保实现复杂度与回归风险可控。

---

## 14. 实现落地记录

### 14.1 已完成内容

1. 阅读页 SSR 已加入独立壳层组件 `ReaderImageViewerShell.tsx`。
2. 图片浏览器交互已拆到独立运行时 `reader-image-viewer-runtime.js`，通过异步脚本按需加载。
3. 已支持正文 `img`、正文 inline `svg`、Mermaid 渲染后的 SVG。
4. 已支持关闭、上一张、下一张、缩小、放大、原始尺寸、旋转。
5. 已支持点击图片外部空白区域关闭浮层。
6. 已支持任意尺寸图片的鼠标四向自由拖动平移。
7. 已支持阅读页内部切换文档后自动重建索引。
8. 已支持窗口尺寸变化时按当前缩放比例重新计算 fit scale。
9. 已支持旋转按累计角度连续前进，避免 `270° -> 0°` 时出现视觉回转。
10. 已支持鼠标滚轮按指针位置缩放。
11. 已支持双击在 fit scale 与原始尺寸之间切换。

### 14.2 已执行验证

1. `npm run test:run -w @plaindoc/web -- src/ssr/render-space-reader.test.tsx src/ssr/render-space-reader.async-script.test.ts src/ssr/reader-mermaid-runtime.test.ts src/ssr/reader-image-viewer-runtime.test.tsx`
2. `npm run build -w @plaindoc/web`

### 14.3 当前未覆盖项

1. 尚未引入真实浏览器端 E2E 覆盖移动端手势与视觉细节。
2. 本期仍不支持拖拽平移、滚轮缩放、双击缩放与下载。
