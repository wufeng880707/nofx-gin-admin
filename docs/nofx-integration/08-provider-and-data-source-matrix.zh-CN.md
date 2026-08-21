# Provider 与量化数据源迁移评估

## 1. 结论

目标项目的核心只读闭环以 Binance、OKX 为第一优先级，Bybit 为第二优先级。Bitget、Gate、KuCoin 已完成源码级评估，但在没有完成统一分页、错误归一化、限流和大账户验收前，不进入可配置列表。Hyperliquid 涉及 Agent Wallet 私钥、主钱包授权、nonce 所有权和 Builder Fee，不与普通 CEX API Key 接入混为一类。

运行时能力可以通过 `GET /api/v1/market/provider-capabilities` 查询，前端和运维不应根据名称推断支持状态。

## 2. CEX 评估

| Provider | NOFX 源码证据 | 目标状态 | 迁移前置条件 |
| --- | --- | --- | --- |
| Binance | `trader/binance` | 快照、历史、公开行情、用户流、USD-M 公共强平流已实现 | 真实大账户和真实用户流凭证验收 |
| OKX | `trader/okx` | 快照、历史、公开行情、用户流已实现 | 超过 100 条历史分页和真实私有流验收 |
| Bybit | `trader/bybit` | 快照与历史已实现 | 真实大账户验收；用户流另行开发 |
| Bitget | `trader/bitget` | 已评估，未排期 | V2 API 分页、节流、Passphrase 和错误契约测试 |
| Gate | `trader/gate` | 已评估，未排期 | `gateapi-go/v6` 依赖审计、分页和精度规则归一化 |
| KuCoin | `trader/kucoin` | 已评估，未排期 | Futures 时间窗、游标、Passphrase 和限流验收 |

评估完成不等于 Provider 已实现。未实现的交易所不会出现在 `ExchangeForm` 的允许列表中，也不会由 Provider Factory 构造，避免“能保存但运行时报错”的假支持。

## 3. Hyperliquid 与 X402

Hyperliquid 的正确安全模型是：

1. 主钱包地址只用于读取账户和授权关系。
2. 系统只保存专用 Agent Wallet 私钥的密文，绝不收集主钱包私钥。
3. Agent Wallet 授权由用户在 Hyperliquid 官方界面完成，系统只验证授权状态和到期时间。
4. nonce 由单一签名执行器持有，多实例必须通过租约串行化。
5. Builder Fee 必须单独披露和确认，不能从 CEX 手续费配置隐式继承。
6. X402 是支付协议，不是交易所账户认证方案，保持独立立项。

在威胁模型、密钥托管、授权撤销和真实资金审计未通过前，目标项目保留数据模型字段但不注册 DEX 读写 Provider。

## 4. NOFXOS 替代策略

Binance 和 OKX 公开 API 已提供免费、无需凭证的 K 线、Order Book、OI、资金费率、标记价格和近期主动买卖成交代理数据，可覆盖行情页、信号矩阵和风险雷达的基础输入。近期主动成交只标记为 `proxy`，不能伪装成机构 NetFlow。

NOFXOS 的 AI500、NetFlow 和单币种聚合仍是可选增强项，不作为 Trader 的强制依赖。采购前必须确认价格、速率、历史深度、数据授权、SLA 和故障降级。目标项目已使用 Binance/OKX 公共数据实现按流动性和 OI 排名的候选池，但该结果不冒充 NOFXOS 的专有 AI500 排名。

Binance USD-M 使用公开 `!forceOrder@arr` WebSocket 采集强平快照，完成事件哈希去重、MySQL 持久化、7 天默认保留以及时间/价格分桶，并由 `GET /api/v1/market/liquidations/heatmap` 提供热力图数据。采集器默认关闭，未采集到数据时页面展示覆盖说明，不生成随机或合成数据。OKX 已下线公开全市场强平订单频道，因此当前不会伪造 OKX 清算热力图；OKX 页面明确展示“不支持”。

## 5. 能力边界

- 公共候选池、OI、资金费率、Order Book、成交方向代理和清算数据不需要用户交易所凭证。
- 私有账户快照、订单、成交和用户数据流必须使用用户自己的只读凭证，并始终按 owner 隔离。
- 将扩展行情上下文发送给外部 AI Provider 仍需要 Trader 启动时的独立数据共享确认；默认不会自动调用外部 AI。
- 真实下单 Provider 和自动外部通知保持默认关闭，不能由公开行情能力间接启用。
