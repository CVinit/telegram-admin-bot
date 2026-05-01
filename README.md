# Telegram Admin Bot

`telegram-admin-bot/` is a standalone administrator-side Telegram bot for Dujiao Next.
It runs as a sidecar service, stores its own SQLite state, and talks to the existing
Dujiao Admin API over `localhost` or the local Docker network.

## Scope

Current implementation covers:

- admin login, logout, and session lookup
- daily, weekly, and monthly sales overview
- auto-fulfillment restock preview and confirmation
- single and batch manual fulfillment preview and confirmation
- customer notification hint computation for email and Telegram

Important boundary:

- notification hints are not delivery proof
- the bot can only infer whether Dujiao Next should attempt email or Telegram notification
- the bot cannot prove asynchronous worker success or customer receipt

## Prerequisites

Before enabling bot-based admin login, disable the Dujiao admin login captcha for the
target site. The bot uses the normal admin login API and cannot solve captchas.

Required Dujiao-side services:

- API service
- queue worker, if you expect downstream email tasks or Telegram callback tasks to run
- SMTP, if you expect order-status email attempts
- an active Telegram channel client with callback URL, if you expect customer Telegram attempts

## Environment

Copy the example file and edit it:

```bash
cp .env.example .env
```

Main variables:

- `TELEGRAM_BOT_TOKEN`: bot token from BotFather
- `DUJIAO_BASE_URL`: Dujiao Admin API base, for example `http://127.0.0.1:8080/api/v1`
- `SQLITE_PATH`: bot-local SQLite database path
- `ACTION_CONFIRM_TTL_SECONDS`: preview-confirm expiry window
- `SESSION_EXPIRE_SKEW_SECONDS`: reserved session skew knob
- `LOG_LEVEL`: current runtime log level

## Local Run

```bash
mkdir -p data
set -a
source .env
set +a
go run ./cmd/bot
```

Run the local smoke checks:

```bash
./scripts/smoke_local.sh
```

## Docker Image

GitHub Actions builds and pushes the bot image to GHCR:

```text
ghcr.io/cvinit/telegram-admin-bot:latest
```

Use `latest` for the newest `main` build, or pin `sha-<commit>` for a fixed
deployment.

## Local Docker Build

```bash
docker build -t telegram-admin-bot:local .
```

Run with an env file and a persistent SQLite directory:

```bash
mkdir -p data
docker run --rm \
  --env-file .env \
  -v "$(pwd)/data:/app/data" \
  telegram-admin-bot:local
```

## Docker Compose

The checked-in compose example runs only the bot and points it at an existing Dujiao
Next Admin API. Configure `.env` first:

```env
TELEGRAM_BOT_TOKEN=replace-with-your-telegram-bot-token
TELEGRAM_ADMIN_BOT_IMAGE=ghcr.io/cvinit/telegram-admin-bot:latest
DUJIAO_BASE_URL=https://your-dujiao-domain.com/api/v1
```

Bring the stack up:

```bash
mkdir -p data
docker compose -f docker-compose.example.yml pull
docker compose -f docker-compose.example.yml up -d
```

If the Dujiao API runs on the host machine, use:

```env
DUJIAO_BASE_URL=http://host.docker.internal:8080/api/v1
```

If both containers run in the same Compose network, use the Dujiao API service name:

```env
DUJIAO_BASE_URL=http://api:8080/api/v1
```

## Commands

Current command set:

- `/login <username> <password>`
- `/logout`
- `/session`
- `/help`
- `/sales today|week|month`
- `/restock <product_id> [sku_id]` followed by pasted secrets on the next lines
- `/ship <order_no>` followed by fulfillment content on the next lines
- `/batch_ship status=<status> [product=<keyword>] [from=<date>] [to=<date>] [limit=<n>]` followed by fulfillment content on the next lines

Current upload support:

- `csv` and `txt` documents for `/restock`
- put the `/restock <product_id> [sku_id]` command in the document caption

Confirmation model:

- write operations create a preview first
- execution happens only after clicking the inline confirm button
- consumed or expired confirmations are rejected

## Verification

Recommended verification sequence:

```bash
go test ./...
go test -race ./...
go build ./cmd/bot
docker build -t telegram-admin-bot:local .
```
