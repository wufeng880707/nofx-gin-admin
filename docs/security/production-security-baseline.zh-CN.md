# XTrading 生产安全基线

## 认证与密码

- 浏览器提交原始密码，必须由 HTTPS/TLS 保护传输。
- 服务端只存储 bcrypt 哈希，不再采用浏览器 MD5 协议。
- 历史 `bcrypt(MD5(password))` 用户在首次成功登录后自动升级为 `bcrypt(password)`。
- 新建用户和重置密码必须显式提供至少 12 位、最多 72 字节的密码。
- Docker 首次启动使用 `ADMIN_INITIAL_PASSWORD`，或自动生成强密码并仅在容器日志显示一次。
- Root bcrypt 哈希持久化到 `app-data:/app/data/admin-password.hash`，重启不会重新生成。

## JWT

- 生产配置必须启用 `[General.Security] Strict = true`。
- `JWT_SIGNING_KEY` 至少 32 字节，禁止仓库示例值和历史默认值。
- 轮换时将旧密钥短期配置为 `OldSigningKey`，新令牌只使用当前密钥签发。
- JWT 只接受配置的确切 HMAC 算法；登出后的令牌进入撤销存储并立即失效。

## 启动拒绝条件

严格模式下出现以下任一情况，服务拒绝启动：

- Debug 开启或配置打印未关闭；
- 认证被关闭；
- JWT 当前/旧密钥过短、使用示例值或相同；
- Root 密码不是有效 bcrypt 哈希；
- Prometheus 启用后仍使用 `admin/admin`。

## 持续检查

`.github/workflows/security.yml` 在提交、PR 和每周计划任务中执行：

- Go 全量测试与 `govulncheck`；
- `npm audit --audit-level=critical` 和前端类型检查；
- Gitleaks 凭证泄漏扫描。

Dependabot 每周检查 Go、npm 和 Docker 依赖。当前无法由应用代码消除的 Umi 传递依赖告警，应在上游发布兼容修复后更新锁文件并重新审计。
