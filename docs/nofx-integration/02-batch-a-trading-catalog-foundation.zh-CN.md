# 第一批 NOFX 交易配置域迁移记录

## 1. 本批目标

本批把 NOFX 的核心配置数据正式迁入 gin-admin 模块化单体，建立可运行、可查询、受权限和数据范围保护的后端基础。

已迁入：

1. 交易所账户配置。
2. AI 模型配置。
3. 策略摘要。
4. Trader 配置与运行状态快照。
5. 四类分页只读 API。
6. Casbin 菜单资源和用户数据范围。
7. 凭证密文入库约束与 API 脱敏。

本批没有迁入：配置写接口、真实 Provider、账户余额、仓位、决策、Trader 启停、交易循环和下单。

## 2. 代码结构

```text
internal/mods/trading/
├── api/catalog.api.go       # HTTP 参数解析和统一响应
├── biz/catalog.biz.go       # 用户数据范围、脱敏和 DTO 转换
├── dal/catalog.dal.go       # GORM 分页、过滤和 owner 约束
├── schema/model.go          # 四个权威 MySQL 模型
├── schema/query.go          # 查询参数与安全响应模型
├── main.go                  # 生命周期、AutoMigrate 和路由
└── wire.go                  # Wire 依赖集合
```

模块已加入 `internal/mods/mods.go`，随 gin-admin 统一启动、注册路由和释放，不存在独立 NOFX HTTP 服务。

## 3. 数据表

| 表 | 作用 | 所有者字段 |
| --- | --- | --- |
| `trading_exchanges` | 交易所账户和加密凭证 | `owner_user_id` |
| `trading_ai_models` | 模型 Provider 和加密 API Key | `owner_user_id` |
| `trading_strategies` | 策略元数据和内部 JSON 配置 | `owner_user_id` |
| `trading_traders` | Trader 关联配置与运行状态 | `owner_user_id` |

`owner_user_id` 直接对应 gin-admin 用户 ID，不再保留第二套 NOFX 用户体系。`legacy_source + legacy_id` 用于后续幂等导入旧 NOFX 数据。

## 4. API 清单

| 方法 | 路径 | 作用 |
| --- | --- | --- |
| `GET` | `/api/v1/trading/exchanges` | 查询脱敏交易所账户 |
| `GET` | `/api/v1/trading/models` | 查询脱敏模型配置 |
| `GET` | `/api/v1/trading/strategies` | 查询策略摘要 |
| `GET` | `/api/v1/trading/traders` | 查询 Trader 配置和状态快照 |

通用分页参数使用 gin-admin 约定的 `current` 和 `pageSize`，上限为 100。支持 `keyword` 和 `ownerUserID`；各接口另外支持交易所类型、Provider、策略类型、启用状态或 Runtime 状态筛选。

## 5. 权限和数据范围

路由统一经过 gin-admin 的认证、限流和 Casbin 中间件。

- 普通用户查询时，业务层强制使用当前登录用户 ID。
- 普通用户显式请求其他 `ownerUserID` 返回 `403 trading_data_scope_denied`。
- Root 可以查询全部用户，也可以指定 `ownerUserID`。
- `configs/menu.json` 和 `configs/menu_cn.json` 已登记四个 GET 资源，可分配给角色。
- 前端菜单页面尚未实现；目前可通过 API 验证，页面将在前端升级批次重建。

## 6. 凭证安全

- 交易所和模型凭证字段的 JSON 标签为 `-`，不会直接进入响应。
- GORM `BeforeSave` 拒绝非空明文，只接受 `ENC:v1:` 密文格式。
- API 只返回 `has_api_key`、`has_secret_key` 等布尔状态。
- 模型自定义 URL 返回前移除 URL 用户信息、查询参数和片段。
- 策略 `config_json` 不进入管理 API，避免 Prompt 和内部参数直接暴露。

当前未开放配置写接口，因为统一 AES-GCM/KMS 加密服务尚未迁入。不能绕过该前置条件直接保存交易所或模型密钥。

## 7. 工具链调整

仓库原 Wire `v0.5.0` 在当前 Go 版本下生成代码时崩溃。本批固定升级到 Wire `v0.7.0` 并生成 `internal/wirex/wire_gen.go`。

Wire 官方仓库已经归档，当前只把 `v0.7.0` 作为兼容性过渡方案。后续基础设施升级阶段应评估改为显式构造函数，避免长期依赖停止维护的代码生成器。

## 8. 验证结果

已通过：

```text
go test ./internal/mods/trading/... -count=1
go test ./test -timeout 60s -count=1
```

全仓 `go test ./...` 中，新增模块、RBAC、API 测试和其他包均通过；仅 `pkg/cachex/TestRedisCache` 因本机未运行 `127.0.0.1:6379` 而失败。这是外部 Redis 测试环境依赖，不是本批迁移回归。

## 9. 下一批

1. 迁入统一凭证加密服务，之后再开放交易所和模型配置写接口。
2. 迁入账户快照、仓位、决策三个只读数据域。
3. 建立旧 NOFX 用户映射、dry-run 和幂等导入命令。
4. 迁入 Binance、Bybit、OKX 只读 Provider 和连接测试。
5. 迁入 Trader Runtime 注册、心跳和健康检查，不立即开放实盘启动。
