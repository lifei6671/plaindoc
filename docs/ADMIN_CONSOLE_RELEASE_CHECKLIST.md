# Admin Console 发布清单（首期）

**Last Updated**: 2026-02-20  
**Scope**: 用户管理、空间管理、文档管理、主题管理、系统配置、审计日志

---

## 0. 当前决策

- [x] 首期不执行单独性能压测，性能基线不作为上线阻塞项。  
- [ ] 如后续出现性能瓶颈，再补专项压测计划与优化迭代。

---

## 1. 配置检查（上线前）

- [ ] `APP_ENV=production`
- [ ] `WEB_ORIGIN` 已配置为生产域名
- [ ] `JWT_SECRET` 已替换为强随机密钥
- [ ] `JWT_ACCESS_TOKEN_TTL` 与 `JWT_REFRESH_TOKEN_TTL` 符合安全策略
- [ ] 数据库连接配置（SQLite/PostgreSQL/MySQL）指向生产实例
- [ ] 存储目录/权限（若启用 SQLite）可写且已备份策略
- [ ] 管理员初始化流程已执行（至少 1 个 `platform_admin`）

---

## 2. 迁移与回滚

### 上线前演练

- [ ] 在预发布环境执行全量迁移并验证通过  
  命令示例：`go test ./internal/storage -count=1`
- [ ] 校验关键表存在：`user_admin_roles`、`space_admin_scopes`、`audit_logs`、`system_configs`
- [ ] 校验索引存在：`audit_logs` 相关索引

### 回滚预案

- [ ] 已确认上一个可用版本 tag/commit
- [ ] 已确认数据库回滚策略（结构回滚 + 数据保留策略）
- [ ] 已确认紧急回滚负责人和值班渠道

回滚步骤（建议）：

1. 停止流量切换并保留只读窗口。  
2. 回滚应用版本到上一个稳定版本。  
3. 若涉及结构回滚，执行受控迁移回退并复验关键查询。  
4. 恢复流量并监控 30 分钟核心指标。  

---

## 3. 监控与告警

### 必备监控指标

- [ ] `/api/admin/*` 请求量、错误率（4xx/5xx）
- [ ] 关键接口 P95 延迟：
  - `GET /api/admin/users`
  - `GET /api/admin/spaces`
  - `GET /api/admin/documents`
  - `GET /api/admin/audits`
- [ ] `OPERATION_TOKEN_*` 错误码趋势（防重放命中率）
- [ ] 审计日志写入失败率

### 告警建议阈值

- [ ] 5xx 错误率 > 1% 持续 5 分钟触发告警
- [ ] 关键接口 P95 > 800ms 持续 10 分钟触发告警
- [ ] 审计写入失败连续 3 次触发告警
- [ ] `OPERATION_TOKEN_REPLAYED` 异常突增触发安全告警

---

## 4. 发布验收（功能与权限）

- [ ] `platform_admin` 菜单与操作权限完整可用
- [ ] `space_admin` 仅可管理授权空间与文档
- [ ] 普通用户访问 `/api/admin/*` 被拒绝（401/403）
- [ ] 高风险操作要求 `X-Admin-Operation-Token`
- [ ] 一次性 token 重放被拒绝（409）
- [ ] 封禁/删除/配置修改可在审计日志检索

---

## 5. 核心验证命令

- 后端管理接口测试：`go test ./internal/server -run TestRouter_Admin -count=1`
- 后端服务与路由回归：`go test ./internal/service ./internal/server -count=1`
- 前端构建检查：`npm run build -w @plaindoc/web`

---

## 6. 发布记录

| 项目 | 值 |
|---|---|
| 发布窗口 | 待填写 |
| 发布人 | 待填写 |
| 回滚负责人 | 待填写 |
| 结果 | 待填写 |
| 备注 | 待填写 |
