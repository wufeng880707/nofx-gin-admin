# nofx-shiyu-admin 已完成功能迁移映射

## 1. 目的

`nofx-shiyu-admin` 已经完成大量融合实现。切换到 gin-admin 不代表从零重新设计，本文件定义哪些成果直接复用、哪些需要按 gin-admin 分层重构、哪些不能迁入。

旧项目保持只读迁移来源，不在原目录上替换底层框架。

## 2. 迁移原则

1. 不整仓复制，不直接合并 ShiyuAdmin 的系统管理代码。
2. 不迁移 ShiyuAdmin 自研 RBAC，改接 gin-admin Casbin。
3. 复用业务状态机、数据约束、DTO 语义、Repository 测试和页面需求。
4. 后端按 gin-admin 的 `api/biz/dal/schema` 重新落位。
5. 前端页面按升级后的新技术栈重建，不复制旧项目框架壳层。
6. 每迁移一批都执行数据范围、脱敏、RBAC、审计和回归验收。

## 3. 已完成能力映射

| 旧项目能力 | 旧项目位置 | 新项目目标 | 处理方式 |
| --- | --- | --- | --- |
| 持久化任务模型 | `internal/model/entity/scheduler.go` | `internal/mods/scheduler/schema` | 重构迁入 |
| 任务 Repository | `internal/repository/db/scheduler_repo.go` | `internal/mods/scheduler/dal` | 复用约束和测试，改造接口 |
| 任务 Service/Runner | `internal/service/scheduler` | `internal/mods/scheduler/biz` | 复用状态机和并发规则 |
| 任务 API | `internal/api/v1/scheduler` | `internal/mods/scheduler/api` | 改接 gin-admin 响应和 Casbin |
| 任务管理页面 | `frontend/.../pages/scheduler` | `web/src/pages/scheduler` | 按新前端重建 |
| 交易只读模型 | `internal/model/entity/trading.go` | `internal/mods/trading/schema` | 重构迁入 |
| 交易只读 Repository | `internal/repository/db/trading_repo.go` | `internal/mods/trading/dal` | 保留数据范围和脱敏约束 |
| 交易只读 Service/API | `internal/service/trading`、`internal/api/v1/trading` | `internal/mods/trading/biz`、`api` | 改接 Casbin 和统一用户 |
| 旧数据导入器 | `internal/migration`、`cmd/nofx-migrate` | `internal/migration`、`cmd/nofx-migrate` | 复用 dry-run、幂等和报告逻辑 |
| 凭证安全 | `internal/security` | `internal/mods/trading/security` 或 `pkg/crypto` | 安全复审后迁入 |
| Provider 只读余额 | `internal/trading` | `internal/mods/trading/provider` | 复用交易所适配和测试 |
| 实时仓位同步 | `internal/trading` | `internal/mods/trading/provider` | 复用关闭语义和快照规则 |
| Runtime 注册/心跳 | `internal/trading` | `internal/mods/trading/runtime` | 复用租约边界，接入 Wire |
| 交易管理页面 | `frontend/.../pages/trading` | `web/src/pages/trading` | 按新前端重建 |
| Scheduler/Trading 前端 API | `frontend/.../services/shiyu-api` | `web/src/services` | 从新 OpenAPI 重新生成 |

## 4. 不迁入内容

- ShiyuAdmin 用户、角色、菜单、部门和 JWT 实现。
- ShiyuAdmin 自研 RBAC 中间件。
- ShiyuAdmin Server、Bootstrap、Repository 基类和响应包装。
- ShiyuAdmin 前端路由壳层、权限入口和旧接口生成代码。
- PostgreSQL 默认配置和 Shiyu 特有环境变量命名。
- 任何允许任意 Shell、SQL、HTTP 或用户代码的任务能力。

## 5. 迁移批次

### Batch A：基础设施与安全

- MySQL、Redis、配置环境变量和版本迁移。
- 统一用户标识、请求上下文和数据范围。
- 密钥加密、日志脱敏和高风险审计。

### Batch B：Scheduler

- 任务定义、运行记录、Handler Registry。
- Cron/时区、超时、重试、misfire、并发策略和租约。
- Casbin 权限、菜单、审计和现代前端页面。

### Batch C：交易只读域和数据导入

- 六个核心交易数据域。
- dry-run、用户映射、幂等来源键和迁移报告。
- 列表、详情、数据范围和脱敏。

### Batch D：Provider 与账户状态

- Binance、Bybit、OKX 只读余额。
- 账户快照、Provider 健康和 Trader 健康检查。

### Batch E：仓位与 Runtime

- 实时仓位同步和缺失仓位关闭语义。
- Runtime 实例注册、心跳和租约。
- 最小模拟 Runtime 和 Trader 启动预检。

## 6. 每批验收门槛

- 单元测试和集成测试通过。
- MySQL 迁移可重复执行，失败可回滚或安全停止。
- Casbin 后端权限和前端按钮权限一致。
- 普通用户无法读取其他用户或租户数据。
- API Key、Secret、Passphrase、Token 和数据库 DSN 不出现在响应和日志中。
- 写操作具备 request ID、幂等和审计。
- 多实例不会重复执行任务或重复持有同一 Trader。
- 文档、OpenAPI 和实现同步更新。
