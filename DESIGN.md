# Telegram Admin Bot Design

## 1. Goal

为 Dujiao-Next 提供一个管理员专用的 Telegram 辅助管理 Bot。

该 Bot 作为独立程序运行在与 Dujiao-Next API 相同的服务器上，通过本机地址访问现有后台 Admin API，不修改主项目核心代码、不接管主项目数据库。

## 2. Scope

### 2.1 Included

- 管理员私聊登录、登出、会话管理
- 基于管理员后台角色的菜单裁剪
- 查询今日 / 本周 / 本月销售统计
- 补充自动发货商品库存
- 人工处理单个订单发货
- 人工批量发货
- 所有写操作先预览再确认
- 本地操作审计与防重复执行
- Docker 独立部署

### 2.2 Excluded

- 不修改 Dujiao-Next 主项目代码
- 不新增主项目专用 Bot 登录接口
- 不支持群聊内敏感操作
- 不支持普通手动库存字段修改
- 不实现后台全部功能镜像
- 不支持多站点聚合管理

## 3. Assumptions

- 管理 Bot 为管理员专用辅助工具，不面向普通用户
- 管理员使用各自的 Dujiao-Next 后台账号登录 Bot
- 管理员权限以 Dujiao-Next 后台 RBAC 为准
- 后台管理员登录验证码场景已关闭，否则 Bot 无法完成账号密码登录
- Bot 与 API 部署在同一台服务器上，默认通过本机内网访问
- 后续所有代码、文档、配置样例都放在 `telegram-admin-bot/` 目录内

## 4. Architecture

Bot 采用独立 sidecar 形态部署，不嵌入 Dujiao-Next API 进程。

### 4.1 Runtime Layers

1. `telegram adapter`
   - 接收命令、按钮回调、文本、多媒体文件
   - 仅处理 Telegram 协议与消息状态
2. `workflow layer`
   - 编排销售查询、补库存、单发货、批量发货
   - 维护多步向导和预览确认流程
3. `dujiao admin client`
   - 封装现有 Admin API
   - 负责登录、鉴权头、错误映射、响应解析
4. `session/auth layer`
   - 维护 Telegram 用户与后台管理员会话关系
   - 缓存 JWT、过期时间、角色快照
5. `storage/audit layer`
   - 使用本地 SQLite 保存会话、待确认动作、执行日志

### 4.2 Why Sidecar

- 满足“所有代码和文本都在一个新文件夹中生成”的约束
- 主项目升级时耦合最低
- Docker 部署简单，不需要改 API 容器
- 失败面可控，Bot 故障不影响主站下单和发货

## 5. Interaction Model

### 5.1 Chat Scope

- 敏感操作仅允许 Telegram 私聊
- 群聊默认不启用
- 只读操作也默认限制在私聊，避免后续泄露订单信息

### 5.2 Mixed UX

同时支持按钮菜单和斜杠命令。

#### Buttons

- 销售统计
- 补自动库存
- 待发货订单
- 单个发货
- 批量发货
- 我的会话

#### Slash Commands

- `/login`
- `/logout`
- `/sales`
- `/restock`
- `/ship`
- `/batch_ship`
- `/session`
- `/help`

### 5.3 Write Safety

所有写操作统一采用三段式：

1. 收集参数
2. 生成预览
3. 点击确认后执行

不提供“收到参数即直接执行”的模式。

## 6. Authentication And Authorization

### 6.1 Login Flow

管理员在私聊中输入后台用户名和密码，Bot 调用 Dujiao-Next 管理员登录接口：

- `POST /api/v1/admin/login`

登录成功后立即查询：

- `GET /api/v1/admin/authz/me`

用于获取：

- `admin_id`
- `username`
- `roles`
- `policies`
- `is_super`

### 6.2 Session Policy

- 不保存明文密码
- 仅保存 JWT、过期时间、管理员标识、角色快照
- JWT 过期后要求管理员重新登录
- 一个 Telegram 用户默认绑定一个后台管理员会话
- 可在本地记录最近登录时间和最近使用时间

### 6.3 Permission Enforcement

Bot 不自定义业务权限，仅根据后台角色快照裁剪功能入口。

示例：

- 只有具备订单相关权限的管理员可见发货菜单
- 只有具备商品 / 卡密相关权限的管理员可见补库存菜单
- 销售统计依赖仪表盘权限

Bot 只做“前置隐藏 + 前置校验”，最终仍以 Admin API 返回结果为准。

## 7. Functional Design

### 7.1 Sales Query

#### Purpose

让管理员快速查看日 / 周 / 月销售核心指标。

#### Data Source

- `GET /api/v1/admin/dashboard/overview?range=today`
- `GET /api/v1/admin/dashboard/overview?range=7d`
- `GET /api/v1/admin/dashboard/overview?range=30d`

#### Display

- GMV
- 已支付订单数
- 已完成订单数
- 利润
- 支付成功率
- 用户余额总额
- 库存预警摘要

#### Notes

- 一期不做复杂自定义日期范围输入
- 一期不做图表，仅返回格式化文本

### 7.2 Auto Fulfillment Restock

#### Business Boundary

此能力仅指“自动发货商品的卡密 / 账号密钥库存补充”。

不修改普通 `manual_stock_total`。

#### Input Modes

- 粘贴多行文本
- 上传 `csv`
- 上传 `txt`

#### Flow

1. 选择商品
2. 如有 SKU，继续选择 SKU
3. 输入文本或上传文件
4. Bot 本地解析、去空行、基础去重
5. 生成预览
6. 管理员确认
7. Bot 调用卡密批量录入接口

#### API Strategy

一期优先统一使用 JSON 批量录入：

- `POST /api/v1/admin/card-secrets/batch`

仅当后续确认确有必要时，才补充透传 CSV 上传接口：

- `POST /api/v1/admin/card-secrets/import`

#### Preview Content

- 商品名称
- SKU 编码 / 规格
- 本次新增条数
- 自动生成或手动输入的批次号
- 前 3 条样例
- 风险提示

### 7.3 Single Manual Fulfillment

#### Purpose

管理员按订单号查找待处理订单并发货。

#### Flow

1. 输入订单号
2. 查询订单详情
3. 校验订单是否允许人工发货
4. 录入发货内容
5. 生成预览
6. 管理员确认
7. 执行发货

#### APIs

- `GET /api/v1/admin/orders`
- `GET /api/v1/admin/orders/:id`
- `POST /api/v1/admin/fulfillments`

#### Payload Form

Bot 支持两种录入模式：

- 纯文本 `payload`
- 结构化 `delivery_data`

结构化模式适用于：

- account
- password
- redeem_url
- note

最终提交时转换为主项目现有发货结构。

### 7.4 Batch Manual Fulfillment

#### Purpose

对一批待人工处理订单进行连续发货。

#### Filtering

一期支持以下筛选条件：

- 订单状态
- 创建时间范围
- 商品关键词
- 订单号前缀 / 精确匹配

#### Flow

1. 输入筛选条件
2. Bot 拉取候选订单
3. 返回待处理列表摘要
4. 收集批量发货内容模板
5. 生成批量预览
6. 管理员确认
7. Bot 串行逐单执行
8. 汇总成功 / 失败结果

#### Execution Rule

- 不并发执行
- 每单独立提交
- 单单记录结果
- 中途失败不回滚已成功订单

这是最符合现有 Admin API 能力和可审计性的做法。

## 8. Preview And Idempotency

### 8.1 Pending Action

每个写操作在确认前都写入本地 `pending_actions` 表，包含：

- action_id
- telegram_user_id
- admin_id
- action_type
- request_payload_snapshot
- preview_summary
- confirm_token
- expire_at
- status

### 8.2 Confirm Rule

确认执行前必须校验：

- 动作未过期
- 动作未执行
- 确认人是原始发起人
- 业务对象状态仍可执行

### 8.3 Anti Double Submit

- 同一个 `confirm_token` 只能消费一次
- 执行成功后立即标记为已消费
- 重复点击返回“已执行或已失效”

## 9. Local Storage Design

Bot 使用独立 SQLite，不接管 Dujiao-Next 主库。

### 9.1 Tables

#### `admin_sessions`

- telegram_user_id
- admin_id
- username
- jwt_token
- jwt_expires_at
- roles_json
- policies_json
- last_login_at
- last_used_at

#### `pending_actions`

- action_id
- action_type
- telegram_user_id
- admin_id
- payload_json
- preview_text
- confirm_token
- status
- expire_at
- created_at

#### `action_logs`

- log_id
- action_type
- telegram_user_id
- admin_id
- target_type
- target_id
- status
- summary
- error_message
- duration_ms
- created_at

### 9.2 Why Local SQLite

- 不污染主项目数据库
- 容器内持久化简单
- 审计边界清晰
- 足够支撑管理员辅助工具规模

## 10. Dujiao API Mapping

### 10.1 Auth

- `POST /api/v1/admin/login`
- `GET /api/v1/admin/authz/me`

### 10.2 Dashboard

- `GET /api/v1/admin/dashboard/overview`

### 10.3 Order

- `GET /api/v1/admin/orders`
- `GET /api/v1/admin/orders/:id`

### 10.4 Fulfillment

- `POST /api/v1/admin/fulfillments`

### 10.5 Stock

- `GET /api/v1/admin/products`
- `GET /api/v1/admin/products/:id`
- `POST /api/v1/admin/card-secrets/batch`
- `GET /api/v1/admin/card-secrets/stats`

## 11. Proposed Folder Layout

```text
telegram-admin-bot/
  DESIGN.md
  README.md
  .env.example
  Dockerfile
  docker-compose.example.yml
  go.mod
  go.sum
  cmd/bot/main.go
  internal/app/
  internal/config/
  internal/telegram/
  internal/dujiao/
  internal/workflow/
  internal/session/
  internal/storage/
  internal/audit/
  internal/render/
  migrations/
  scripts/
```

## 12. Configuration

推荐使用 `.env` 管理：

- `BOT_TOKEN`
- `BOT_MODE=polling`
- `BOT_PRIVATE_ONLY=true`
- `DUJIAO_BASE_URL=http://127.0.0.1:8080/api/v1`
- `SQLITE_PATH=/data/bot.db`
- `ACTION_CONFIRM_TTL_SECONDS=600`
- `SESSION_EXPIRE_SKEW_SECONDS=300`
- `LOG_LEVEL=info`

## 13. Deployment

### 13.1 Mode

独立 Docker 容器，和 Dujiao-Next API 运行在同一服务器。

### 13.2 Telegram Mode

一期采用 `long polling`：

- 不需要对外暴露端口
- 不需要 webhook 域名和证书
- 同机部署更简单

### 13.3 API Access

支持两种配置：

- 宿主机部署：`http://127.0.0.1:8080/api/v1`
- Docker 组网：`http://api:8080/api/v1`

默认文档按本机地址描述，因为当前需求明确要求使用 `localhost` 访问 API。

## 14. Delivery Plan For Phase 1

### 14.1 Must Have

- 管理员登录 / 登出
- 会话检查
- 销售统计
- 自动发货库存补充
- 单个订单人工发货
- 批量发货
- 预览确认机制
- 本地审计
- Docker 部署文件

### 14.2 Nice To Have

- 常用筛选条件缓存
- 最近商品 / 最近 SKU 快捷入口
- 批量发货模板复用

### 14.3 Not In Phase 1

- Bot 内完整后台镜像
- 订单退款
- 钱包操作
- 支付渠道管理
- 多站点切换
- webhook 模式

## 15. Risks

### 15.1 Login Captcha

若后台登录验证码未关闭，Bot 登录流程不可用。

### 15.2 JWT Expiration

当前后台登录为 JWT 模式，无 refresh token，管理员需要重新登录。

### 15.3 Batch Fulfillment Errors

批量发货只能逐单执行，可能出现部分成功、部分失败，需要清晰回执和审计。

### 15.4 Duplicate Restock

补库存本质是追加型写入，必须依赖确认 token 和本地审计防止重复导入。

## 16. Recommendation

按当前约束，最佳落地方式是：

- 在 `telegram-admin-bot/` 中实现独立 Go 程序
- 使用管理员个人后台账号登录
- 使用现有 Admin API 完成业务操作
- 所有写操作先预览后确认
- 使用本地 SQLite 承担会话和审计
- 使用 Docker 独立部署并通过本机地址访问 Dujiao-Next API
