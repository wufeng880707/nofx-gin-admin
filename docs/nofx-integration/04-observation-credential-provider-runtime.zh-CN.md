# 账户观察、凭证、旧数据导入与 Runtime 迁移记录

## 1. 本批范围

本批将 NOFX 的账户观察链路迁入 `internal/mods/trading`，完成：

1. 账户快照、仓位和 AI 决策持久化模型及只读 API。
2. AES-256-GCM 凭证加密、字段绑定和密钥轮换兼容。
3. 可 dry-run、可审计、幂等的旧 NOFX 数据导入器及 CLI。
4. Binance、Bybit、OKX 永续合约账户只读 Provider。
5. Trader Runtime 注册、数据库租约、心跳和账户/仓位同步。

本批没有开放下单、撤单、平仓等写交易能力。Provider 接口中没有交易命令，Runtime 当前是账户观察运行时，不是完整实盘策略执行器。

## 2. 代码结构

```text
internal/mods/trading/
├── api/observation.api.go          # 账户、仓位、决策只读 HTTP API
├── biz/observation.biz.go          # 用户数据范围和决策脱敏
├── dal/observation.dal.go          # MySQL/GORM 查询
├── schema/observation.go           # 观察、导入报告和租约模型
├── credential/service.go           # AES-256-GCM 凭证服务
├── migration/importer.go           # 旧 NOFX 幂等导入器
├── provider/                       # Binance、Bybit、OKX 只读适配
└── runtime/registry.go             # Runtime 注册、租约和周期同步

cmd/import_nofx.go                  # import-nofx 命令
```

## 3. 数据模型

| 表 | 用途 | 关键约束 |
| --- | --- | --- |
| `trading_account_snapshots` | 按周期保存账户权益、余额、可用余额和未实现盈亏 | `owner_user_id` 数据范围；按 Trader 和时间索引 |
| `trading_positions` | 保存标准化仓位生命周期 | `exchange_id + exchange_position_id` 唯一；只允许 `long/short`、`open/closed` |
| `trading_decisions` | 保存 AI 决策周期和执行结果 | 保存 Exchange 关系；列表 API 不返回 prompt、CoT 或原始响应 |
| `trading_legacy_import_runs` | 保存每次正式旧数据导入报告 | 包含各领域扫描、导入、跳过、失败数量和最多 100 条错误 |
| `trading_runtime_leases` | 防止多实例重复运行同一 Trader | Trader 主键唯一；租约带实例 ID、过期时间和心跳 |

金额、价格、数量和 PnL 均以字符串承载并映射为 MySQL `DECIMAL`，不使用浮点数持久化资金数据。

## 4. 只读 API

| 方法 | 路径 | 常用筛选 |
| --- | --- | --- |
| GET | `/api/v1/trading/account-snapshots` | `traderID`、`exchangeID`、`startAt`、`endAt` |
| GET | `/api/v1/trading/positions` | 上述参数及 `symbol`、`side`、`status` |
| GET | `/api/v1/trading/decisions` | 上述参数及 `success` |

分页参数沿用 gin-admin：`current`、`pageSize`。普通用户只能查询自己的数据；Root 可以显式传 `ownerUserID` 查看指定用户。三个资源已挂入 Trading/Trader 菜单权限，仍需在角色菜单中授权。

决策列表只输出候选币、执行日志和动作 JSON，不输出：

- `system_prompt`
- `input_prompt`
- `cot_trace`
- `decision_json`
- `raw_response`

## 5. 凭证加密

### 5.1 环境变量

```bash
export DATA_ENCRYPTION_KEY="$(openssl rand -base64 32)"
export DATA_ENCRYPTION_KEY_PREVIOUS=""
```

`DATA_ENCRYPTION_KEY` 解码后必须正好为 32 字节。禁止提交到 Git、写入 TOML 或打印到日志。

### 5.2 密文格式

新密文格式为：

```text
ENC:v1:<key-id>:<nonce>:<ciphertext>
```

- 算法：AES-256-GCM。
- 每次加密使用随机 nonce。
- AAD 绑定 `owner_user_id + entity_id + field`，密文复制到其他用户、配置或字段后无法解密。
- 可通过 `DATA_ENCRYPTION_KEY_PREVIOUS` 在轮换窗口读取旧密文。
- 兼容旧 NOFX 的 `ENC:v1:<nonce>:<ciphertext>` 格式。

新主密钥严格要求 32 字节 Base64。`DATA_ENCRYPTION_KEY_PREVIOUS` 仅用于迁移和轮换兼容：它同时接受旧 NOFX 曾支持的 Base64、Hex 或任意口令派生规则。导入器会将可解密的旧两段式密文重新加密为当前主密钥格式；无法解密时 dry-run 和正式导入都会报告错误。迁移完成后应移除旧密钥。

应用可在未配置密钥时启动并浏览无凭证数据，但导入明文凭证、构建交易所 Provider 时会明确失败。

## 6. 旧数据导入

### 6.1 支持的数据

导入器按顺序处理：`exchanges`、`ai_models`、`strategies`、`traders`、`trader_equity_snapshots`、`trader_positions`、`decision_records`。源库缺少某张表时跳过，不视为失败。

幂等键统一为 `legacy_source + legacy_id`。正式导入重复执行时，已存在记录计入 `skipped`，不会再次插入，也不会覆盖新系统已修改的数据。

### 6.2 用户映射

旧 NOFX 用户必须映射到 gin-admin 用户。单用户迁移可使用：

```text
--default-owner <gin-admin-user-id>
```

多用户迁移使用 JSON：

```json
{
  "legacy-user-1": "gin-admin-user-1",
  "legacy-user-2": "gin-admin-user-2"
}
```

然后传入 `--owner-map-file ./owner-map.json`。可同时设置默认 Owner 处理未列出的旧用户。

### 6.3 dry-run

默认不写数据：

```bash
go run . import-nofx \
  --workdir configs \
  --config dev \
  --source-type mysql \
  --source-dsn "$NOFX_LEGACY_DSN" \
  --source-name nofx-production \
  --default-owner root
```

命令向标准输出返回 JSON 报告。dry-run 中的 `imported` 表示“预计可导入”，目标库不会新增业务数据或导入报告。

### 6.4 正式导入

先备份数据库，确认 dry-run 无错误，再执行：

```bash
export DATA_ENCRYPTION_KEY="<base64-encoded-32-byte-key>"

go run . import-nofx \
  --workdir configs \
  --config dev \
  --source-type mysql \
  --source-dsn "$NOFX_LEGACY_DSN" \
  --source-name nofx-production \
  --owner-map-file ./owner-map.json \
  --apply
```

`--config` 决定目标数据库；`--source-dsn` 决定旧库。因此既支持同一 MySQL 数据库内迁移，也支持跨数据库迁移。正式执行前应先启动一次应用或运行版本化迁移，保证目标 `trading_*` 表已存在。

## 7. 交易所只读 Provider

| Provider | 账户接口 | 仓位接口 | 测试网 |
| --- | --- | --- | --- |
| Binance | USD-M Futures Account | Position Risk | 支持 Binance Futures Testnet |
| Bybit | Unified Wallet Balance | Linear Positions | 支持 Bybit Testnet |
| OKX | Account Balance | SWAP Positions | 同域名并发送模拟交易 Header |

官方接口参考：

- [Binance USD-M Account Information V2](https://developers.binance.com/docs/derivatives/usds-margined-futures/account/rest-api/Account-Information-V2)
- [Binance USD-M Position Information V2](https://developers.binance.com/docs/derivatives/usds-margined-futures/trade/rest-api/Position-Information-V2)
- [Bybit Get Wallet Balance](https://bybit-exchange.github.io/docs/v5/account/wallet-balance)
- [Bybit Get Position Info](https://bybit-exchange.github.io/docs/v5/position)
- [OKX API v5](https://www.okx.com/docs-v5/en/)

接入要求：

1. API Key 只授予读取账户和仓位所需权限。
2. 不授予提现权限；当前阶段也不授予交易权限。
3. 配置 IP 白名单。
4. OKX 还需要 Passphrase。
5. Provider 只发送签名 GET 请求；接口类型中不存在下单方法。

HTTP 客户端默认超时 20 秒，响应体最大 4 MiB，非 2xx 响应直接失败。Bybit 仓位使用 `limit=200` 并完整遍历游标，防止漏仓后被 Runtime 错误关闭。单元测试通过本地 HTTP Server 验证签名 Header、GET 方法、游标分页和标准化结果；尚未使用真实账户凭证做联网验收。

## 8. Trader Runtime

Runtime 启动一个 Trader 时执行：

1. 获取 `trading_runtime_leases` 数据库租约。
2. 解密交易所配置并构建只读 Provider。
3. 将 Trader Runtime 状态设为 `starting`。
4. 立即读取一次账户和仓位，事务写入账户快照并 upsert 当前仓位。
5. 将本次响应中不存在的旧 open 仓位关闭。
6. 按 `scan_interval_minutes` 周期同步并续租。
7. 停止、租约丢失或进程释放时清理内存注册和租约。

数据库租约允许过期接管，防止多副本同时同步同一 Trader。当前已开放受 Casbin 控制的 HTTP 启停和立即同步接口：启动前先执行一次带签名的只读账户预检，前端要求二次确认；预检失败不会写入 `running` 目标状态。当前 Runtime 仍只同步账户与仓位，不包含下单能力。

## 9. 验证与剩余工作

已覆盖自动化测试：

- AES-GCM 往返、AAD 防复制、旧密钥轮换和缺失密钥。
- 导入 dry-run、正式导入、幂等、凭证加密和关系映射。
- 三家 Provider 的签名 GET 和响应标准化。
- Runtime 快照写入、仓位关闭和多实例租约互斥。
- 决策安全视图不泄露 prompt、CoT 和原始响应。

下一步：

1. 用版本化 MySQL migration 替代生产 `AutoMigrate`。
2. 使用真实测试网只读凭证完成 Binance、Bybit、OKX 联网验收。
3. 增加 Provider 限流、有限重试、指标和告警。
4. 建立独立的交易命令审计表和 Runtime 进程重启恢复流程。
5. 接入 AI 决策、风控、订单执行和故障对账后，才允许实盘交易权限。
