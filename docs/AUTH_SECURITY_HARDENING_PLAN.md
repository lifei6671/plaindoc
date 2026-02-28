# 认证安全加固实施方案（注册/登录）

**Last Updated**: 2026-02-28  
**状态**: 开发完成（本地验证通过，待预发验证）

## 1. 目标与范围

本次加固覆盖两个入口：

1. `POST /api/auth/login`
2. `POST /api/auth/register`

满足以下安全目标：

1. 多次登录失败时，触发图片验证码。
2. 随失败/重试次数增加，验证码复杂度分级提升。
3. 达到阈值后，直接封禁 24 小时（同一风险主体）。
4. 前端与管理后台登录界面都能处理验证码会话与封禁反馈。

## 2. 风险控制策略

### 2.1 风险维度

登录场景按以下维度并行计数，取最高风险级别：

1. `IP`
2. `identifier`（邮箱/用户名）
3. `IP + identifier` 组合

注册场景按以下维度计数：

1. `email`
2. `IP + email` 组合

说明：数据库仅保存哈希后的风险主体标识，不直接保存明文主体键。

### 2.2 阈值（默认）

登录失败阈值：

1. L1 验证码：3
2. L2 验证码：6
3. L3 验证码：9
4. 24h 封禁：12

注册同主体尝试阈值：

1. L1 验证码：2
2. L2 验证码：5
3. L3 验证码：8
4. 24h 封禁：10

窗口参数：

1. 统计窗口：15 分钟
2. 封禁时长：24 小时
3. 验证码有效期：120 秒

### 2.3 验证码复杂度

1. L1：4 位数字 + 轻噪声
2. L2：5 位字母数字 + 中噪声
3. L3：6 位字母数字 + 强噪声

实现约定：

1. 生成器使用 `github.com/mojocn/base64Captcha` 的 `DriverDigit`。
2. 响应字段 `level` 登记为验证码字符数量（如 `4/5/6`），而不是风险等级序号（`1/2/3`）。

## 3. 后端设计

### 3.1 新增持久化对象

1. `auth_risk_states`
   - 存风险状态与计数：尝试数、失败数、验证码失败数、封禁到期时间、窗口起点。
2. `auth_captcha_challenges`
   - 存验证码会话：`captcha_id`、题目哈希、级别、过期时间、是否已消费。

### 3.2 业务服务

新增认证风控服务：

1. 预检（`Check`）：判断是否封禁、是否需要验证码、验证码是否正确。
2. 结果回写（`RecordResult`）：根据登录/注册成功或失败更新计数与封禁状态。
3. 会话生成：生成图片验证码与会话记录。

### 3.3 API 行为约定

新增认证错误码：

1. `CAPTCHA_REQUIRED`
2. `CAPTCHA_INVALID`
3. `AUTH_TEMPORARILY_LOCKED`

错误响应通过统一 `JsonResult` 返回 `data` 扩展字段：

1. 验证码会话数据：`captchaId`、`captchaImageDataUrl`、`level`、`expiresInSeconds`
2. 封禁信息：`lockedUntil`、`retryAfterSeconds`

登录/注册请求体增加可选字段：

1. `captchaId`
2. `captchaAnswer`

## 4. 前端交互设计

### 4.1 登录/注册页

1. 收到 `CAPTCHA_REQUIRED` / `CAPTCHA_INVALID` 时展示验证码图片与输入框。
2. 下次提交时自动附带 `captchaId` + `captchaAnswer`。
3. 收到 `AUTH_TEMPORARILY_LOCKED` 时显示封禁结束时间。

### 4.2 管理后台登录页

`/admin/login` 同样对接验证码会话，避免后台入口在新策略下不可登录。

## 5. 实施清单

### 5.1 文档与契约

- [x] 编写本实施方案文档。
- [x] 更新前后端开发指南中的认证安全章节。

### 5.2 后端

- [x] 新增三套数据库迁移（sqlite/mysql/postgres）：`0015_auth_risk_controls`。
- [x] 新增兼容迁移：`0016_auth_captcha_level_char_count`（修复旧库 `level` 约束从 `1..3` 升级到字符数量语义）。
- [x] 新增模型：`AuthRiskState`、`AuthCaptchaChallenge`。
- [x] 新增仓储接口与 GORM 实现。
- [x] 新增风控策略/服务（阈值、窗口、封禁、验证码）。
- [x] 接入 `auth handler` 的 login/register 链路。
- [x] 增加认证错误码与响应模板。
- [x] 增加基础测试（登录失败升级、封禁、注册验证码触发）。

### 5.3 前端

- [x] 扩展 `AuthLoginInput`/注册入参，支持验证码字段。
- [x] 扩展 HTTP 适配器错误对象，透传错误 `data`。
- [x] 登录/注册页增加验证码 UI 与状态管理。
- [x] Admin 登录页增加验证码 UI 与状态管理。
- [x] 端到端联调验证（login/register/admin-login，本地 API+前端构建链路通过）。

### 5.4 验证与上线

- [x] 本地测试通过（后端单测 + 前端构建）。
- [ ] 在预发环境验证阈值与封禁策略。
- [ ] 发布后观察告警与指标（captcha 命中率、lock 命中率、误拦截反馈）。
