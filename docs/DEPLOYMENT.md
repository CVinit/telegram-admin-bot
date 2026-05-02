# Telegram Admin Bot 发货调试部署教程

本文档用于在你已经部署好完整 Dujiao-Next 的前提下，单独启动 `telegram-admin-bot`，通过它调试管理员发货功能。

关键结论：不需要再拉一个 Dujiao-Next API 容器。Bot 只需要能访问你现有站点的 Admin API。

默认使用 GitHub Actions 自动构建并推送到 GHCR 的镜像：

```text
ghcr.io/cvinit/telegram-admin-bot:latest
```

## 架构

推荐部署形态：

```text
Telegram Admin Bot -> 已部署的 Dujiao-Next Admin API -> 已部署的数据库/Redis/worker/通知服务
```

不要在本地再启动一套独立 API，除非你明确想调试一套全新的空数据库。否则 Bot 会连接到另一套订单数据，发货调试结果不会反映你已经部署好的站点。

## 前置条件

1. 完整 Dujiao-Next 已经部署并可访问。
2. Dujiao-Next 的 Admin API 可以从 Bot 容器访问，例如 `https://your-dujiao-domain.com/api/v1`。
3. 已从 BotFather 创建 Telegram Bot，并拿到 `TELEGRAM_BOT_TOKEN`。
4. Dujiao 后台登录验证码必须关闭，Bot 登录走普通管理员登录 API，不能处理验证码。
5. 用于调试发货的订单必须是人工发货订单，状态必须是 `paid` 或 `fulfilling`，且不能已有 fulfillment。

客户通知相关限制：

- Bot 发货成功表示已调用 Dujiao 管理端发货 API。
- Bot 显示的邮件或 Telegram 通知只是预期触发提示，不代表最终送达。
- 邮件通知依赖你已部署 Dujiao-Next 的 SMTP 和 worker。
- 客户 Telegram 通知依赖你已部署 Dujiao-Next 的 `telegram_bot` channel client、callback URL、客户 Telegram runtime，以及订单用户已绑定 Telegram。

## 配置 `.env`

在仓库根目录创建 `.env`：

```bash
cat > .env <<'EOF'
TELEGRAM_BOT_TOKEN=替换为BotFather给你的token
TELEGRAM_ADMIN_BOT_IMAGE=ghcr.io/cvinit/telegram-admin-bot:latest
DUJIAO_BASE_URL=https://你的dujiao域名/api/v1
LOG_LEVEL=info
ACTION_CONFIRM_TTL_SECONDS=300
SESSION_EXPIRE_SKEW_SECONDS=60
EOF
```

`docker compose` 会自动读取当前目录的 `.env` 做变量替换，不需要把 token 写进 YAML 文件。

如果你的完整 Dujiao-Next 跑在同一台机器的宿主机端口，例如 `127.0.0.1:8080`，容器内不能直接用 `127.0.0.1` 访问宿主机。使用：

```env
DUJIAO_BASE_URL=http://host.docker.internal:8080/api/v1
```

如果 Bot 和 Dujiao-Next 在同一个 Docker network 中，并且 Dujiao-Next 服务名是 `api`，可以使用：

```env
DUJIAO_BASE_URL=http://api:8080/api/v1
```

注意：服务名 `api` 只有在 Bot 容器加入 Dujiao-Next 所在 Docker network 时才可用。如果 Bot 使用本文的独立 Compose 文件启动，默认会创建自己的网络，不能自动解析另一套 Compose 里的 `api` 服务名。最简单稳定的方式是使用你已经配置好的 HTTPS 域名。

如果确实要使用同一个 Docker network，可以在 `docker-compose.example.yml` 中把 Bot 接入已有外部网络，例如：

```yaml
services:
  telegram-admin-bot:
    networks:
      - dujiao-net

networks:
  dujiao-net:
    external: true
    name: 你的dujiao compose网络名
```

## 启动 Bot

在仓库根目录执行：

```bash
docker compose -f docker-compose.example.yml pull
docker compose -f docker-compose.example.yml up -d
```

检查状态：

```bash
docker compose -f docker-compose.example.yml ps
```

查看日志：

```bash
docker compose -f docker-compose.example.yml logs -f telegram-admin-bot
```

升级到远程最新镜像：

```bash
docker compose -f docker-compose.example.yml pull
docker compose -f docker-compose.example.yml up -d
```

## GitHub Actions 构建镜像

仓库包含 `.github/workflows/docker-image.yml`。它会在以下场景构建并推送 Docker 镜像到 GHCR：

- 推送到 `main`。
- 推送形如 `v*` 的 tag。
- 在 GitHub Actions 页面手动运行 `docker image`。

推送目标：

```text
ghcr.io/cvinit/telegram-admin-bot
```

常用标签：

- `latest`: `main` 分支最新构建。
- `main`: `main` 分支构建。
- `sha-<commit>`: 精确到提交的镜像，适合生产锁版本。
- `v...`: tag 构建。

如果服务器拉取镜像时报权限错误，说明 GHCR package 不是公开的。可以在 GitHub 仓库页面进入 `Packages`，把 `telegram-admin-bot` package 设置为 public；或者在服务器上登录 GHCR：

```bash
echo '<github_pat_with_read_packages>' | docker login ghcr.io -u '<github_username>' --password-stdin
```

## 验证 API 可达

先在宿主机验证已部署 Dujiao-Next：

```bash
curl https://你的dujiao域名/health
```

如果你在 `.env` 中配置的是 `DUJIAO_BASE_URL=https://你的dujiao域名/api/v1`，Bot 实际会访问：

```text
https://你的dujiao域名/api/v1/admin/login
https://你的dujiao域名/api/v1/admin/orders
https://你的dujiao域名/api/v1/admin/fulfillments
```

如果 `/login` 失败，优先检查 Dujiao 后台登录验证码是否关闭，以及管理员账号密码是否正确。

## Telegram Bot 登录

打开你的 Telegram Bot 私聊窗口，先登录 Dujiao 管理员：

```text
/login <admin_username> <admin_password>
```

检查当前会话：

```text
/session
```

查看帮助：

```text
/help
```

敏感命令只能在私聊使用，群聊里会被拒绝。

## 准备发货订单

`/ship` 只能处理满足以下条件的订单：

- 订单号存在。
- 订单状态是 `paid` 或 `fulfilling`。
- 订单没有现有 fulfillment。
- 订单不是父订单。
- 订单所有商品项都是 `manual` 人工发货类型。

如果你要用 API 辅助排查订单，先登录已部署 Dujiao-Next：

```bash
TOKEN=$(curl -s https://你的dujiao域名/api/v1/admin/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"你的管理员账号","password":"你的管理员密码"}' | jq -r '.data.token')

curl -s "https://你的dujiao域名/api/v1/admin/orders?status=paid&page=1&page_size=20" \
  -H "Authorization: Bearer $TOKEN" | jq
```

上面的命令依赖 `jq`。如果没有 `jq`，直接查看原始 JSON 也可以。

## 查询待发货订单

在 Telegram Bot 私聊中发送：

```text
/pending_ship product_id=9 sku_id=3 limit=20
```

默认查询 `paid` 和 `fulfilling` 两类待发货订单。支持的过滤参数：

- `status`: 可设为 `paid`、`fulfilling` 或 `pending`，默认 `pending`。
- `product_id`: 指定商品 ID。
- `sku_id`: 指定 SKU ID，必须和 `product_id` 一起使用。
- `product`: 商品关键词，对应管理端订单列表的 `product_keyword`。
- `from`: 创建时间起点，对应 `created_from`。
- `to`: 创建时间终点，对应 `created_to`。
- `limit`: 返回数量，默认 `20`。

指定 `product_id` 后，Bot 会逐个读取订单详情，只保留整个订单都属于该商品和 SKU 范围的人工发货订单，避免混合商品订单被误发。

## 调试单个发货

在 Telegram Bot 私聊中发送：

```text
/ship <order_no>
账号=demo-user
密码=demo-pass
备注=发货调试
```

如果每一行都是 `key=value`，Bot 会把内容转成结构化 `delivery_data`。如果不是 `key=value` 格式，Bot 会把后续正文作为普通 `payload` 提交。

普通文本示例：

```text
/ship <order_no>
这是交付内容
第二行内容
```

Bot 会先返回发货预览，并显示确认按钮。只有点击确认按钮后，Bot 才会调用：

```text
POST /api/v1/admin/fulfillments
```

确认成功后，订单会生成 fulfillment，后续再次发货会被拒绝。

## 调试批量发货

批量发货会先按筛选条件拉取订单列表，再逐个读取详情并过滤出符合人工发货条件的订单。

基础示例：

```text
/batch_ship status=paid limit=5
账号=shared-demo
密码=shared-pass
```

带商品关键词和时间范围：

```text
/batch_ship status=paid product=测试商品 from=2026-04-01 to=2026-04-30 limit=10
批量发货统一内容
```

支持的过滤参数：

- `status`: 可设为 `paid`、`fulfilling` 或 `pending`，默认 `pending`。
- `product_id`: 指定商品 ID。设置后进入同商品卡密分配模式。
- `sku_id`: 指定 SKU ID，必须和 `product_id` 一起使用。
- `product`: 商品关键词，对应管理端订单列表的 `product_keyword`。
- `from`: 创建时间起点，对应 `created_from`。
- `to`: 创建时间终点，对应 `created_to`。
- `limit`: 拉取数量，默认 `20`。

同商品批量发货示例：

```text
/batch_ship product_id=9 sku_id=3 limit=20
CARD-001
CARD-002
CARD-003
```

该模式下每一行是一条卡密。Bot 会先统计当前筛选范围内的待发货订单和订单数量，并要求卡密行数必须等于总发货数量。预览确认后，Bot 会按付款时间从早到晚分配卡密，例如数量为 2 的订单会收到连续 2 条卡密。数量不匹配或卡密重复时不会生成确认动作。

批量发货同样会先返回预览，点击确认按钮后才执行。

所有发货最终都调用 Dujiao 管理端：

```text
POST /api/v1/admin/fulfillments
```

因此邮件、客户 Telegram 通知和下游回调仍由 Dujiao-Next 的人工发货流程触发。Bot 不直接改订单状态，也不绕过 Dujiao 的通知队列。

## 调试自动发货补库存

如果你还要调试自动发货商品的库存补充，可以用 `/restock`：

```text
/restock <product_id> [sku_id]
卡密1
卡密2
卡密3
```

也可以上传 `txt` 或 `csv` 文件，并把 `/restock <product_id> [sku_id]` 放在文件 caption 中。该流程只适用于自动发货商品，会先预览，再确认导入。

## 常用排查

Bot 容器启动后反复退出：

```bash
docker compose -f docker-compose.example.yml logs telegram-admin-bot
```

如果日志显示 `TELEGRAM_BOT_TOKEN is required` 或 `DUJIAO_BASE_URL is required`，说明根目录 `.env` 没有配置对应变量。

Bot `/login` 失败：

- 确认 `DUJIAO_BASE_URL` 必须以 `/api/v1` 结尾。
- 确认 Bot 容器能访问该域名或地址。
- 确认管理员账号密码正确。
- 确认 Dujiao 后台登录验证码已关闭。
- 查看已部署 Dujiao-Next 的 API 日志。

`/ship` 返回订单不可发货：

- 确认传入的是订单号 `order_no`，不是数据库 ID。
- 确认订单状态是 `paid` 或 `fulfilling`。
- 确认订单商品是 `manual`。
- 确认订单没有现有 fulfillment。

发货成功但客户没有收到通知：

- 邮件通知需要已部署 Dujiao-Next 配好 SMTP，且 worker 正常运行。
- Telegram 客户通知需要 Dujiao 侧的客户 Telegram runtime，不是这个管理员 Bot 本身。
- Bot 结果中的通知字段只是预期触发提示，不是送达证明。

## 停止和重置

停止 Bot：

```bash
docker compose -f docker-compose.example.yml down
```

删除 Bot 本地会话数据并重建：

```bash
docker compose -f docker-compose.example.yml down -v
docker compose -f docker-compose.example.yml up -d
```

`down -v` 只会删除 Bot 的本地 SQLite 会话、确认动作和审计日志，不会影响你已部署的 Dujiao-Next 数据库。
