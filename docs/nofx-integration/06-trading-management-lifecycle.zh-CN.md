# XTrading 交易管理与 Trader 生命周期闭环

## 1. 本批目标

本批把原有交易只读目录扩展为可通过管理端操作的配置闭环，覆盖：

- 交易所账户、AI 模型、策略和 Trader 的新增、详情、编辑、删除与分页查询。
- Binance、Bybit、OKX 交易所只读连接测试。
- Trader 启动、停止、立即同步及运行状态展示。
- 账户快照、仓位和决策的只读观察页面。
- 凭证加密、用户数据范围、引用保护、运行中保护和 Casbin 按钮权限。

本批不包含 AI 决策生成、风控裁决、下单、撤单、平仓和成交对账，因此不能视为实盘交易闭环。

## 2. 管理页面

| 菜单 | 页面 | 主要操作 |
| --- | --- | --- |
| 交易配置 | `/trading/configs` | 交易所、AI 模型、策略 CRUD；交易所连接测试 |
| Trader 管理 | `/trading/traders` | Trader CRUD、启动、停止、立即同步 |
| 账户快照 | `/trading/snapshots` | 按 Trader 和时间查询余额、权益、保证金和未实现盈亏 |
| 仓位记录 | `/trading/positions` | 查询持仓方向、数量、价格、杠杆、保证金和盈亏 |
| 决策记录 | `/trading/decisions` | 查询安全视图，不返回 Prompt、思维链和原始模型响应 |

菜单和按钮均由 `/api/v1/current/menus` 返回的 Casbin 资源控制。交易配置使用 `add/edit/delete/test`，Trader 使用 `add/edit/delete/start/stop/sync`。

## 3. HTTP API

四类配置资源均提供：

```text
GET    /api/v1/trading/{resource}
POST   /api/v1/trading/{resource}
GET    /api/v1/trading/{resource}/{id}
PUT    /api/v1/trading/{resource}/{id}
DELETE /api/v1/trading/{resource}/{id}
```

其中 `{resource}` 为 `exchanges`、`models`、`strategies` 或 `traders`。生命周期命令：

```text
POST /api/v1/trading/exchanges/{id}/test
POST /api/v1/trading/traders/{id}/start
POST /api/v1/trading/traders/{id}/stop
POST /api/v1/trading/traders/{id}/sync
```

## 4. 启动流程

Trader 启动不是直接写状态，实际顺序为：

1. 校验 Trader 属于当前用户，并读取关联交易所、模型和策略。
2. 要求交易所和模型已启用、策略处于激活状态。
3. 解密交易所凭证并构建只读 Provider。
4. 发起一次带签名的账户/仓位只读预检。
5. 预检成功后获取数据库租约并注册 Runtime。
6. 写入账户快照和仓位，更新心跳及运行状态。
7. 按扫描间隔继续同步，停止时释放租约。

预检失败时返回 `trading_trader_preflight_failed`，不会把目标状态改为 `running`。管理端启动与停止均要求二次确认。

## 5. 数据与安全约束

- 普通用户只能操作 `owner_user_id` 为自己的记录；Root 可管理全部记录。
- API 不返回密钥明文，只返回 `has_api_key` 等配置状态。
- 交易所和模型密钥使用 AES-256-GCM 保存，密文带版本前缀并绑定 AAD。
- 编辑时凭证留空表示保留旧值，不会因页面回显而泄露或清空。
- 被 Trader 引用的交易所、模型和策略禁止删除，返回 `409 Conflict`。
- `starting/running` Trader 禁止编辑和删除。
- 当前 Provider 接口只有账户和仓位查询，不定义下单方法。
- 前端确认框对业务失败进行收口，统一请求层展示错误后仍会关闭遮罩，避免页面假死。

## 6. 本地验收结果

Docker 环境下已完成：

- app、MySQL、Redis 健康检查通过。
- 浏览器完成交易所、模型、策略、Trader 的创建、编辑、查询和删除。
- MySQL 布尔校验确认测试凭证为 `ENC:v1:` 密文，且不等于输入明文。
- 无效 Binance 测试密钥启动返回 `400`，Trader 保持 `stopped`。
- 删除被 Trader 引用的交易所返回 `409`；删除 Trader 后可按顺序清理配置。
- 验收用临时记录已全部删除。
- `go test ./...`、前端 `tsc --noEmit`、Docker 镜像构建和 `git diff --check` 通过。

## 7. 下一阶段

进入实盘前至少还需要：

1. 使用三家交易所真实测试网只读凭证完成联网验收。
2. 增加 Provider 限流、有限重试、熔断、指标与告警。
3. 增加交易命令专用不可抵赖审计，而不只依赖通用 HTTP 日志。
4. 完成 Runtime 进程重启恢复、状态收敛和人工接管流程。
5. 迁入 AI 决策、风控、订单状态机、幂等下单、撤单、平仓和对账。
6. 完成以上能力与模拟盘回归后，再单独设计实盘启用开关和多重确认。
