# WA n8n 工作流

本目录保存 WA CTF 链路的 n8n 编排定义。编排只串联原子动作，不把 Python 脚本作为运行时桥接；设备指纹、注册请求、登录态等具体能力由 `wa-app` 的原子服务或其 HTTP action gateway 提供。

## 工作流

| 文件 | 触发方式 | 职责 |
| --- | --- | --- |
| `proxy/wa-us-dynamic-ip.workflow.json` | Execute Workflow | 从 proxy-runtime `/leases/acquire` 申请美国随机动态 IP lease，返回工作流内部使用的代理 URL 和对外摘要；不做预检。 |
| `probe/wa-number-sms-probe.workflow.json` | Webhook `POST /wa/number-sms-probe`；Execute Workflow | 根据号码确定号码国家，使用美国随机动态 IP，生成临时随机指纹、号码检测、SMS 可发送性检测；检测用指纹不入库。 |
| `registration/wa-register.workflow.json` | Webhook `POST /wa/register` | 先复用号码/SMS 检测；检测通过后提交同一临时指纹为该号码设备指纹，发起 SMS OTP，等待 OTP 回调，提交验证码并持久化登录态。 |

## 运行环境变量

- `WA_ACTION_API_BASE_URL`：WA 原子动作 HTTP 入口，默认指向 `wa-app-service:8080/api/wa/actions`；该入口薄封装 `wa-app` gRPC/服务动作，并负责 PG/Redis 持久化动作。
- `PROXY_RUNTIME_API_BASE_URL`：proxy-runtime HTTP API，动态代理工作流通过 `/leases/acquire` / `/leases/release` 申请和释放美国随机动态 IP lease。

## 输入

### 号码/SMS 检测

`POST /wa/number-sms-probe` 或由注册工作流调用：

```json
{
  "workspace_id": "default",
  "region": "ID",
  "phone": "81234567890"
}
```

前端只提交号码池、workspace 和一个号码国家/拨号码选择，不提交 `job_id` 或代理参数。dashboard BFF 会补齐 `job_id`、`request_id`。也支持 `country_calling_code`、`cc`、`country_iso2`、`national_number`、`number`、`e164_number`、`region_code` 等同义输入；`country_code` 是数字时按拨号码处理，`+E.164` 号码优先按号码本身推导国家。输出包含 `phone_status.account_status`、`phone_status.sms_status`、`phone_status.can_register`、动态代理摘要和 `transient_fingerprint_ref`。检测工作流会把 `fingerprint_persistence` 标记为 `TRANSIENT_NOT_COMMITTED`，不得在 action gateway 中写成该号码的长期设备指纹。

### 注册

`POST /wa/register`：

```json
{
  "workspace_id": "default",
  "region": "ID",
  "phone": "81234567890"
}
```

注册工作流在检测通过后调用 `/fingerprints/commit`，把同一个 `transient_fingerprint_ref` 固化到该号码的 `wa_account_id` / `client_profile_id`。随后请求 SMS OTP，并把 `$execution.resumeUrl` 注册到 `/registration/await-otp`。

代理策略细节：

- 检测号码、检测 SMS 可发送和注册 OTP 请求统一使用 proxy-runtime 美国随机动态 IP lease；`wa-app` action gateway 只在内部使用 `proxy_url` 发起 WA 请求。
- 动态 IP 只按 `country_code: "US"` 申请；不做出口 IP、风控、CF 或目标连通性预检，不按 workspace、号码、账号、号码国家或地区绑定代理。
- 终态分支会调用 `/leases/release` 释放本次 lease。
- 最终响应只返回 `{ "proxy_mode": "US_RANDOM_DYNAMIC_IP", "country_code": "US" }` 等非敏感摘要，不返回具体代理 URL 或凭据。

OTP 回调向 n8n wait 节点的 resume URL 发送：

```json
{
  "otp": "123456"
}
```

也兼容 `code` 或 `verification_code` 字段。OTP 是敏感值，action gateway 和 n8n 日志都不得输出明文。

## Action API 约定

工作流当前使用以下动作端点，端点内部可以映射到 `wa-app` gRPC 原子接口：

| 端点 | 用途 |
| --- | --- |
| `POST /proxy-settings` | 返回 WA 美国随机动态 IP 模式摘要；具体代理 URL 只在工作流内部流转。 |
| `POST /fingerprints/random` | 生成随机模拟设备指纹；`persist:false` 时只返回临时引用。 |
| `POST /number-probe/account` | 执行号码账号状态检测。 |
| `POST /number-probe/sms` | 执行 SMS 可发送性检测，不发起注册 OTP 持久化。 |
| `POST /fingerprints/commit` | 将检测阶段的临时指纹提交为号码设备指纹，并返回 `wa_account_id`、`client_profile_id`、`protocol_profile_id`。 |
| `POST /registration/request-sms-otp` | 发起 SMS OTP 请求。 |
| `POST /registration/await-otp` | 记录 n8n resume URL 与等待窗口，便于外部 OTP 采集侧回调。 |
| `POST /registration/submit-otp` | 提交 OTP 完成注册。 |
| `POST /registration/persist-login-state` | 注册成功后幂等确认登录态投影，并返回脱敏登录态摘要。 |
| `POST /registration/check-login-state` | 使用原生 chatd 被动短连接握手检测账号远端登录态；可传 `login_state_id`、`registered_identity_id` 或 `wa_account_id`/`client_profile_id`。 |

## 状态边界

- 检测：只允许临时 fingerprint 引用，返回号码状态，不写长期 profile。
- 注册：只有检测通过后才 commit fingerprint，并把登录态作为长期事实保存；登录态未激活时注册工作流返回失败。登录态激活后由 wa-app 自动启动消息长连接，不需要额外 workflow。
- 登录态检测：远端握手成功会刷新 `last_verified_at` 并触发长连接恢复；明确失效会把登录态置为 `INVALID`；代理/网络不可达只返回 `UNREACHABLE` 检测结果，不把账号直接判失效。
- 等待 OTP、幂等窗口属于短期运行态，应设置 TTL。
- 响应中避免返回 token、authkey、cookie、OTP、可复用请求体、代理凭据等敏感材料。
