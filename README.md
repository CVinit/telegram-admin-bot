# Telegram Admin Bot

This module contains the standalone Telegram admin bot service for Dujiao Next.

## Environment

Copy and edit the example environment file:

```bash
cp .env.example .env
```

## Run Locally

```bash
mkdir -p data
set -a
source .env
set +a
go run ./cmd/bot
```

## Run With Docker

Build the image:

```bash
docker build -t telegram-admin-bot .
```

Run the container with env file:

```bash
mkdir -p data
docker run --rm --env-file .env -v "$(pwd)/data:/app/data" telegram-admin-bot
```

Use the compose stub:

```bash
docker compose -f docker-compose.example.yml up --build
```

The example SQLite path writes to `./data/bot.db`, so the data directory must exist
and be mounted into `/app/data` when running in Docker.
