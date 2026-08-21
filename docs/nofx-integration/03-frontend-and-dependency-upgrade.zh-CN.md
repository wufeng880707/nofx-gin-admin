# 前端接入与前后端依赖升级记录

## 1. 本批目标

本批完成两项基础工作：

1. 将官方 [`gin-admin/gin-admin-frontend`](https://github.com/gin-admin/gin-admin-frontend) 作为本项目唯一管理前端，迁入仓库 `web/`。
2. 将 gin-admin 的前后端依赖升级到当前可验证、可兼容的最新版本，并完成必要的主版本代码迁移。

前端来源提交：`e006373fc5f2ca1998fdd142c997531aaa8c2d7e`。

## 2. 仓库结构

```text
nofx-gin-admin/
├── cmd/                    # Go 命令入口
├── configs/                # 后端配置和菜单资源
├── internal/               # RBAC、Trading 等业务模块
├── pkg/                    # JWT、Redis、OSS 等公共能力
├── web/                    # gin-admin-frontend，独立 Node 工程
│   ├── config/             # Umi 路由和布局配置
│   ├── src/                # React 页面、服务和权限逻辑
│   ├── package.json
│   └── package-lock.json
└── docs/nofx-integration/  # 融合决策和实施记录
```

`web/go.mod` 只用于建立 Go 模块扫描边界，避免根目录执行 `go test ./...` 时进入 `web/node_modules`；它不把前端变成 Go 工程。

## 3. 前端技术栈

| 组件 | 原始前端 | 当前版本 | 说明 |
| --- | --- | --- | --- |
| Node.js | 16 | >= 22 | 当前验证环境为 Node 25.9.0 |
| npm | 未锁定 | >= 10 | 当前 lockfile 由 npm 11.12.1 生成 |
| React | 17 | 18.3.1 | Umi 4 依赖链完整声明支持的最新 React 主版本 |
| Umi | 3.5 | 4.7.2 | 使用 `@umijs/max`，移除 Umi 3 配置项 |
| Ant Design | 4.x | 5.29.3 | Pro Components 2.8.10 尚未声明支持 Ant Design 6 |
| Pro Components | 1.1 | 2.8.10 | 已适配菜单、下拉和布局 API |
| TypeScript | 4.x | 5.9.3 | Umi 工具链已验证的最新稳定主版本 |
| Playwright | 无 | 1.62.1 | 为后续登录、RBAC 和交易页面端到端测试预留 |
| Prettier | 2.x | 3.9.6 | 统一前端格式检查 |

没有直接采用 React 19、Ant Design 6 和 TypeScript 7，原因不是遗漏更新，而是当前核心框架的 peer dependency 或构建兼容性尚未闭合。待 Umi 和 Pro Components 正式支持后，应单独建立升级批次并运行完整页面回归。

## 4. 前端适配内容

- 将 Umi 导入统一切换到 `@umijs/max`。
- 移除 Umi 3 的 `dva`、`dynamicImport`、IE target、旧主题变量和 OpenAPI 开发路由配置。
- 适配 Umi 4 路由、history query、菜单和运行时 layout API。
- 适配 Ant Design 5 的菜单类型、Dropdown 和 notification API。
- 清理 Ant Design 4 内部 Less 路径，改为工程内明确样式值。
- 移除已失效的 Jest、ESLint、Stylelint 和 Umi 3 配置文件。
- 提交 `package-lock.json`，保证依赖安装可复现。
- 移除登录页预填的 `admin / abc-123`，避免把默认凭证展示给所有访问者。
- 默认关闭后端完整配置打印，避免 JWT、数据库和管理员配置进入启动日志。
- 将前端产物统一输出到 `web/dist`，根 Makefile 的静态目录同步更新。

注意：登录接口目前仍按原后端协议在浏览器端执行 MD5，这属于下一项安全基线任务。本批只移除了固定凭证展示，不能把“移除预填密码”误认为密码体系已完成升级。

## 5. 后端升级结果

| 组件 | 原版本 | 当前版本 |
| --- | --- | --- |
| Go directive | 1.19 | 1.26.0 |
| Gin | 1.9.0 | 1.12.0 |
| GORM | 1.24 开发版 | 1.31.2 |
| GORM MySQL | 1.4.7 | 1.6.0 |
| Casbin | 2.68.0 | 2.135.0 |
| Validator | 10.12.0 | 10.30.3 |
| JWT | v3.2.2 | v5.3.1 |
| go-redis | v8/v9 混用 | 业务代码统一 v9.22.0 |
| redis_rate | v9.1.2 | v10.0.1 |
| AWS SDK | v1.44.300 | SDK v2，S3 1.107.0 |
| MySQL driver | 1.7.0 | 1.10.0 |
| Prometheus client | 1.14.0 | 1.24.1 |
| Wire | 0.5.0 | 0.7.0 |

`github.com/go-redis/redis/v8` 仍会作为 `LyricTian/captcha` 的间接依赖存在，但项目自身的 Redis 缓存和限流代码不再直接使用 v8。

## 6. 后端代码迁移

- JWT claims 从 `StandardClaims` 迁移到 v5 `RegisteredClaims` 和 `NumericDate`。
- Redis 限流器改用 `redis_rate/v10` 与 `redis/go-redis/v9`。
- S3 适配器从已废弃的 AWS SDK v1 迁移到模块化 AWS SDK v2。
- 修复 Go 1.26 vet 对动态格式字符串的检查。
- Redis 集成测试增加 200ms 前置探测；本机未启动 Redis 时明确跳过，不再等待几十秒后产生误导性失败。

## 7. 本地命令

前端：

```bash
cd web
npm install
npm run typecheck
npm run dev
```

生产构建：

```bash
make build
```

该命令先生成 `web/dist`，再编译 Go 服务。只构建前端可执行：

```bash
make web-build
```

后端验证：

```bash
go test ./... -timeout 3m -count=1
```

## 8. 验证结果

本批已完成：

- `npm ls --depth=0`：通过，无 missing 或 invalid 顶层依赖。
- `npm run typecheck`：通过。
- `npm run build`：通过，Umi 4 生产构建成功。
- `go test ./... -timeout 3m -count=1`：通过；Redis 不可用时对应集成测试明确跳过。

当前非阻断警告：

- Umi 启动阶段会输出 Node `--localstorage-file` 警告，来自工具链内部。
- Browserslist 提示 `caniuse-lite` 数据可更新，不影响当前构建结果。
- 主入口 gzip 约 416 KB，后续交易页面增多前应增加分包和体积预算。

## 9. 下一步

1. 完成服务端密码哈希升级、初始管理员一次性引导和旧 MD5 兼容迁移。
2. 将开发环境默认数据库切换为 MySQL，并增加版本化迁移与 Docker Compose。
3. 为已完成的交易配置 API 增加前端页面，接通 Casbin 菜单和按钮权限。
4. 迁移持久化 Scheduler，再进入 NOFX Provider 和 Runtime 集成。
