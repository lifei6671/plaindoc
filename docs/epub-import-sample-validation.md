# EPUB 导入样本验证记录

> 状态：In Progress
>
> 创建时间：2026-05-17

## 1. 当前结论

当前已使用用户提供样本 `apps/server/data/延迟满足：如何在等待中获得更多.epub` 完成一轮 inspect 验证，说明当前 EPUB 预检链路可以处理至少 1 本真实 EPUB3 书籍。

为避免引入版权不明确的电子书文件，本阶段仍不将 EPUB 样本提交到仓库。后续验证建议继续使用用户明确提供的样本，或使用确认可下载、可临时使用的公版/测试 EPUB，并放在仓库外临时目录。

## 2. 待补样本矩阵

- [ ] EPUB2 样本
- [x] EPUB3 样本
- [ ] Calibre 转换样本
- [ ] 非标准 `media-type` 但扩展名明确的样本

## 3. 每个样本需记录

- inspect 摘要：空间名、章节数、目录层级、图片数、warnings。
- 导入结果：导入耗时、创建的新空间 ID、导入后目录树。
- 内容质量：Markdown 转换质量、内部链接降级情况、图片本地化结果。
- 异常记录：解析失败、转换失败、安全降级或性能异常。

## 4. 本次验证记录

- 样本路径：`apps/server/data/延迟满足：如何在等待中获得更多.epub`
- 验证命令：`go test -timeout 120s ./internal/service -run TestInspectAdminSpaceImportEPUB_UserProvidedSample -count=1 -v`
- inspect 摘要：
  - 空间名：`延迟满足：如何在等待中获得更多【日本销售之神稻盛和夫力荐！张一鸣等大咖倡导的理念！坚持长期主义，学会延迟满足，是一种更高级的利益计算方式。】`
  - 文档数：`36`
  - 目录最大深度：`2`
  - warnings：`0`

## 5. 历史查找记录

```bash
rg --files | rg '\.epub$'
find /tmp -maxdepth 3 -type f -name '*.epub'
find /home/lifei6671/src -maxdepth 4 -type f -name '*.epub'
```

结果：未找到可用 EPUB 样本；`/tmp` 下部分 systemd 私有目录因权限限制不可读取，不影响仓库样本结论。
