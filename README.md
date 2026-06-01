# wa-app

`wa-app` 是 WA CTF 解题链路的应用层服务。它把 WA CTF 解题链路实现为 Go 原生原子 gRPC 接口，并用 PG/Redis 管理服务状态。`wa-re` / `app-release-re` 仅作为已验证参考材料，不作为运行时桥接脚本。

## 边界

- 不修改 `wa-re/` 和 `app-release-re/` 下的脚本与材料，也不通过进程桥接这些脚本。
- Proto 是 RPC、状态、错误码和事件语义的真源。
- PG 存长期事实：WAAccount、client profile、注册记录、消息元数据、解密/候选值投影。
- 注册成功后的登录态以 `LoginState` 投影保存；敏感登录材料只保存在受控 profile/state 工作区或 TTL 运行态。
- Redis 存 TTL 运行态：幂等窗口、消息会话 lease。
- OTP、Flag、token、authkey、key/session 等敏感值不写普通日志；PG 中只保存引用或脱敏值。

## 目录

- `proto/byte/v/forge/waapp/v1/`：服务契约。
- `gen/go/`：由 proto 生成的 Go 类型和 gRPC server/client 接口。
- `internal/app/`：服务编排、PG store、Redis runtime、Go 原生协议 engine、detached tooling。
- `cmd/wa-app-service/`：gRPC 服务入口。
- `migrations/001_init.sql`：PG 表结构。
- `docs/modeling.md`：动作拆分和状态边界说明。
- `workflows/n8n/wa/`：WA 编排工作流定义；只串联原子动作，不桥接参考脚本。
- `webui/`：WA 管理 dashboard 远程模块。

## 运行配置

必需：

- `WA_APP_PG_DSN`
- `PLATFORM_REDIS_URL`

可选：

- `WA_APP_LISTEN_ADDR`，默认 `:50091`
- `WA_APP_DASHBOARD_HTTP_ADDR`，默认 `:8080`
- `WA_APP_DASHBOARD_STATIC_DIR`，默认 `/app/dashboard/wa`
- `WA_N8N_WEBHOOK_BASE_URL`，例如 `http://n8n-webhook:5678/webhook`
- `PROXY_RUNTIME_API_BASE_URL`，用于直连登录态检测与长连接恢复时申请/释放美国随机动态 IP
- `PLATFORM_NATS_URL`，用于把 WA 长连接收到并解出的 OTP 发布到平台 MQ（NATS JetStream）

协议 endpoint、WASafe 公钥、WA app 版本、User-Agent、ChatD host/TLS/timeout、请求 timeout、本地 profile 目录和设备指纹策略都内置在 wa-app 原生 engine 中，不再作为运行时环境变量暴露；号码探测/注册必须通过 proxy-runtime 的短租约动态代理传入。

## 原子 RPC

- Discovery：登记 app artifact、记录协议能力 profile。
- Profile：创建 WAAccount、准备/查询/退役客户端 profile。
- Registration：账号探测、请求验证码、提交验证码、查询注册记录。
- LoginState：查询注册成功后的活动登录态；`CheckLoginState` 会用原生 chatd 被动短连接握手检测远端登录态，并在消息会话打开前校验该登录态属于对应 WAAccount/client profile。
- Messaging：打开消息会话、批量接收消息、ack、关闭会话、查询长连接状态；注册/登录态检测成功后会自动恢复消息长连接。
- Extraction：解密消息、提取 OTP/Flag 候选值；长连接收到 OTP 后发布 `byte.v.forge.wa.otp.received` 平台事件，payload 标明号码、来源和 OTP。
- Tooling：生成 detached phone profile、导入 WAMSYS 捕获、构造注册请求材料、WASafe ENC、APK token/authkey 派生。

## n8n 编排

- `workflows/n8n/wa/proxy/wa-us-dynamic-ip.workflow.json`：从 proxy-runtime 申请美国随机动态 IP lease；不做出口 IP、风控、CF 或目标连通性预检。
- `workflows/n8n/wa/registration/wa-register.workflow.json`：注册编排只申请动态 IP、生成并提交注册用设备指纹、发起 SMS OTP、等待 OTP resume 回调、提交验证码并持久化登录态；号码探测不进入 n8n。

登录态检测是 wa-app 直连接口：`POST /api/wa/login-state-check`，不会进入 n8n。

详细输入/输出和 action API 约定见 `workflows/n8n/wa/README.md`。

n8n 工作流需要 `PROXY_RUNTIME_API_BASE_URL`，用于向 proxy-runtime 申请和释放美国随机动态 IP lease。

## 前端管理页

`webui/` 发布 module federation 远程模块 `/mf/wa/remoteEntry.js`，导航项为 `WA 管理`。页面包含：

- 账号管理：WAAccount 列表、分页和添加入口是默认首屏。
- 工具箱：输入手机号和国家拨号码执行轻量号码探测。
- 号码探测动作：调用 `/api/wa/phone/sms-probe`，由 wa-app dashboard BFF 直连 wa-app 原子能力；每次探测生成随机设备指纹但不持久化，使用 1 分钟 proxy-runtime 美国随机动态 IP 短租约，用完立即释放，不进入 n8n。
- 号码解析使用 libphonenumber 元数据，不维护固定国家码表，也不强解未规范化号码。
- n8n action gateway：`/api/wa/actions/*` 由 wa-app 提供，负责指纹临时态、注册 OTP、登录态持久化和登录态远端检测动作。
- 注册流程仍由 `POST /api/wa/register` 转发到 n8n `wa/register`，不属于号码池或批量准备页面。
- 后端自动补齐：`job_id`、`request_id`；检测号码和检测 SMS 可发送使用 proxy-runtime 美国随机动态 IP 短租约，不暴露也不持久化代理账号、代理国家/地区或具体代理绑定。
- 工作流状态页：展示当前 n8n webhook 接入状态和 WA webhook path。
- 长连接状态：号码详情会展示当前长连接、chatd ping 心跳、最近心跳和最近消息时间。

前端展示会脱敏 token、OTP、cookie、session、auth/key 等字段。

## 本地验证

当前仅做非构建类验证：

```bash
wa-app/scripts/generate-proto.sh
protoc -I wa-app/proto --descriptor_set_out=/tmp/wa-app.pb $(find wa-app/proto -name '*.proto' | sort)
(cd wa-app && go list ./...)
(cd wa-app && go vet ./...)
(cd wa-app/webui && npm run lint)
```

不要在 Mac 本机执行业务构建、镜像构建或部署验证。
