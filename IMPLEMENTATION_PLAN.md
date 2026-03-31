# Telegram Admin Bot Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a standalone Telegram administrator bot under `telegram-admin-bot/` that logs in with Dujiao-Next admin accounts, queries sales, restocks auto-fulfillment inventory, and performs single/batch manual fulfillment with preview-confirmation and notification hints.

**Architecture:** Create `telegram-admin-bot/` as its own Go submodule so all code and text stay isolated from the root API module. The bot runs long-polling, stores sessions/pending actions/audit logs in a local SQLite database, and talks to the existing Dujiao-Next Admin API over localhost or the local Docker network using a thin HTTP client.

**Tech Stack:** Go 1.25.3 child module, `net/http`, `github.com/go-telegram/bot`, GORM + `github.com/glebarez/sqlite`, Docker

---

## File Structure

All implementation stays under `telegram-admin-bot/`.

- `telegram-admin-bot/go.mod`: child Go module and bot-only dependencies
- `telegram-admin-bot/cmd/bot/main.go`: process entrypoint
- `telegram-admin-bot/internal/app/`: dependency wiring and lifecycle
- `telegram-admin-bot/internal/config/`: environment parsing and validation
- `telegram-admin-bot/internal/storage/`: SQLite schema, repositories, migrations
- `telegram-admin-bot/internal/dujiao/`: Dujiao-Next Admin API client
- `telegram-admin-bot/internal/session/`: admin login/session orchestration
- `telegram-admin-bot/internal/audit/`: action log writes and summaries
- `telegram-admin-bot/internal/workflow/`: sales/restock/fulfillment use cases
- `telegram-admin-bot/internal/telegram/`: Telegram update router and handlers
- `telegram-admin-bot/internal/render/`: Telegram-friendly message builders
- `telegram-admin-bot/README.md`: runbook and deployment guide
- `telegram-admin-bot/.env.example`: runtime configuration template
- `telegram-admin-bot/Dockerfile`: standalone image build
- `telegram-admin-bot/docker-compose.example.yml`: sample sidecar deployment

Implementation rule: do not modify files outside `telegram-admin-bot/` during this plan.

### Task 1: Bootstrap The Child Module

**Files:**
- Create: `telegram-admin-bot/go.mod`
- Create: `telegram-admin-bot/cmd/bot/main.go`
- Create: `telegram-admin-bot/internal/app/app.go`
- Create: `telegram-admin-bot/internal/config/config.go`
- Create: `telegram-admin-bot/internal/config/config_test.go`
- Create: `telegram-admin-bot/.env.example`
- Create: `telegram-admin-bot/README.md`
- Create: `telegram-admin-bot/Dockerfile`
- Create: `telegram-admin-bot/docker-compose.example.yml`

- [ ] **Step 1: Write the failing config test**

```go
func TestLoadConfigRequiresBotToken(t *testing.T) {
    _, err := LoadFromEnv(map[string]string{
        "DUJIAO_BASE_URL": "http://127.0.0.1:8080/api/v1",
        "SQLITE_PATH":     "./data/bot.db",
    })
    if err == nil {
        t.Fatal("expected bot token validation error")
    }
}
```

- [ ] **Step 2: Run the test to verify the module has not been scaffolded yet**

Run: `cd telegram-admin-bot && go test ./internal/config -run TestLoadConfigRequiresBotToken -v`
Expected: FAIL because the package and loader do not exist yet

- [ ] **Step 3: Create the child module, config loader, and process entrypoint**

```go
type Config struct {
    BotToken                 string
    DujiaoBaseURL            string
    SQLitePath               string
    ActionConfirmTTLSeconds  int
    SessionExpireSkewSeconds int
    LogLevel                 string
}

func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatal(err)
    }
    app, err := app.New(cfg)
    if err != nil {
        log.Fatal(err)
    }
    if err := app.Run(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

- [ ] **Step 4: Add the runtime docs and container stubs**

Include:
- `.env.example` with all required env vars
- `README.md` with local run and Docker run commands
- `Dockerfile` that builds only `telegram-admin-bot/cmd/bot`
- `docker-compose.example.yml` showing `DUJIAO_BASE_URL=http://api:8080/api/v1`

- [ ] **Step 5: Run tests to verify the scaffold is valid**

Run: `cd telegram-admin-bot && go test ./...`
Expected: PASS for config tests, no compile failures

- [ ] **Step 6: Commit**

```bash
git add telegram-admin-bot/go.mod telegram-admin-bot/cmd/bot/main.go telegram-admin-bot/internal/app/app.go telegram-admin-bot/internal/config/config.go telegram-admin-bot/internal/config/config_test.go telegram-admin-bot/.env.example telegram-admin-bot/README.md telegram-admin-bot/Dockerfile telegram-admin-bot/docker-compose.example.yml
git commit -m "feat: scaffold telegram admin bot module"
```

### Task 2: Add SQLite Storage For Sessions, Pending Actions, And Audit Logs

**Files:**
- Create: `telegram-admin-bot/internal/storage/db.go`
- Create: `telegram-admin-bot/internal/storage/models.go`
- Create: `telegram-admin-bot/internal/storage/migrate.go`
- Create: `telegram-admin-bot/internal/storage/session_store.go`
- Create: `telegram-admin-bot/internal/storage/pending_action_store.go`
- Create: `telegram-admin-bot/internal/storage/action_log_store.go`
- Create: `telegram-admin-bot/internal/storage/storage_test.go`

- [ ] **Step 1: Write the failing migration test**

```go
func TestAutoMigrateCreatesCoreTables(t *testing.T) {
    db := openTestDB(t)
    if err := Migrate(db); err != nil {
        t.Fatal(err)
    }
    assertTableExists(t, db, "admin_sessions")
    assertTableExists(t, db, "pending_actions")
    assertTableExists(t, db, "action_logs")
}
```

- [ ] **Step 2: Run the test to confirm the storage layer does not exist yet**

Run: `cd telegram-admin-bot && go test ./internal/storage -run TestAutoMigrateCreatesCoreTables -v`
Expected: FAIL because `Migrate` and the stores do not exist

- [ ] **Step 3: Implement the storage models and migration entrypoint**

```go
type AdminSession struct {
    ID           uint `gorm:"primaryKey"`
    TelegramUser int64 `gorm:"uniqueIndex;not null"`
    AdminID      uint  `gorm:"index;not null"`
    Username     string
    JWTToken     string
    JWTExpiresAt time.Time
    RolesJSON    string
    PoliciesJSON string
    LastLoginAt  time.Time
    LastUsedAt   time.Time
}
```

- [ ] **Step 4: Implement repositories with deterministic APIs**

Required repository methods:
- `UpsertSession`
- `GetSessionByTelegramUser`
- `DeleteSessionByTelegramUser`
- `CreatePendingAction`
- `ConsumePendingAction`
- `MarkPendingActionExpired`
- `CreateActionLog`
- `ListRecentActionLogs`

- [ ] **Step 5: Run tests to validate migrations and round-trip storage behavior**

Run: `cd telegram-admin-bot && go test ./internal/storage -v`
Expected: PASS with table creation and CRUD round-trip coverage

- [ ] **Step 6: Commit**

```bash
git add telegram-admin-bot/internal/storage/db.go telegram-admin-bot/internal/storage/models.go telegram-admin-bot/internal/storage/migrate.go telegram-admin-bot/internal/storage/session_store.go telegram-admin-bot/internal/storage/pending_action_store.go telegram-admin-bot/internal/storage/action_log_store.go telegram-admin-bot/internal/storage/storage_test.go
git commit -m "feat: add bot sqlite storage"
```

### Task 3: Build The Dujiao Admin API Client

**Files:**
- Create: `telegram-admin-bot/internal/dujiao/client.go`
- Create: `telegram-admin-bot/internal/dujiao/types.go`
- Create: `telegram-admin-bot/internal/dujiao/auth.go`
- Create: `telegram-admin-bot/internal/dujiao/dashboard.go`
- Create: `telegram-admin-bot/internal/dujiao/orders.go`
- Create: `telegram-admin-bot/internal/dujiao/products.go`
- Create: `telegram-admin-bot/internal/dujiao/settings.go`
- Create: `telegram-admin-bot/internal/dujiao/channel_clients.go`
- Create: `telegram-admin-bot/internal/dujiao/client_test.go`

- [ ] **Step 1: Write the failing login client test**

```go
func TestLoginStoresReturnedTokenEnvelope(t *testing.T) {
    api := newTestServerReturningJSON(t, 200, map[string]any{
        "code": 0,
        "data": map[string]any{
            "token": "jwt-demo",
            "expires_at": "2026-03-31T18:00:00Z",
            "user": map[string]any{"id": 3, "username": "ops"},
        },
    })
    client := New(api.URL)
    resp, err := client.Login(context.Background(), "ops", "secret")
    if err != nil {
        t.Fatal(err)
    }
    if resp.Token != "jwt-demo" {
        t.Fatalf("unexpected token: %s", resp.Token)
    }
}
```

- [ ] **Step 2: Run the targeted test**

Run: `cd telegram-admin-bot && go test ./internal/dujiao -run TestLoginStoresReturnedTokenEnvelope -v`
Expected: FAIL because the client does not exist yet

- [ ] **Step 3: Implement the HTTP client and shared response decoder**

```go
type Client struct {
    baseURL    string
    httpClient *http.Client
}

func (c *Client) doJSON(ctx context.Context, method, path string, token string, in any, out any) error
```

- [ ] **Step 4: Implement typed methods for all required APIs**

Required methods:
- `Login`
- `GetAuthzMe`
- `GetDashboardOverview`
- `ListOrders`
- `GetOrder`
- `CreateFulfillment`
- `ListProducts`
- `GetProduct`
- `CreateCardSecretBatch`
- `GetSMTPSettings`
- `GetTelegramBotRuntimeStatus`
- `ListChannelClients`
- `GetUser`

- [ ] **Step 5: Cover success and error mapping with `httptest.Server`**

Run: `cd telegram-admin-bot && go test ./internal/dujiao -v`
Expected: PASS with login, authz, order, and settings client coverage

- [ ] **Step 6: Commit**

```bash
git add telegram-admin-bot/internal/dujiao/client.go telegram-admin-bot/internal/dujiao/types.go telegram-admin-bot/internal/dujiao/auth.go telegram-admin-bot/internal/dujiao/dashboard.go telegram-admin-bot/internal/dujiao/orders.go telegram-admin-bot/internal/dujiao/products.go telegram-admin-bot/internal/dujiao/settings.go telegram-admin-bot/internal/dujiao/channel_clients.go telegram-admin-bot/internal/dujiao/client_test.go
git commit -m "feat: add dujiao admin api client"
```

### Task 4: Implement Admin Session And Audit Services

**Files:**
- Create: `telegram-admin-bot/internal/session/service.go`
- Create: `telegram-admin-bot/internal/session/service_test.go`
- Create: `telegram-admin-bot/internal/audit/service.go`
- Create: `telegram-admin-bot/internal/audit/service_test.go`

- [ ] **Step 1: Write the failing session service test**

```go
func TestLoginPersistsTelegramAdminSession(t *testing.T) {
    deps := newSessionTestDeps(t)
    _, err := deps.Service.Login(context.Background(), 123456789, "ops", "secret")
    if err != nil {
        t.Fatal(err)
    }
    session, err := deps.Store.GetSessionByTelegramUser(context.Background(), 123456789)
    if err != nil {
        t.Fatal(err)
    }
    if session.AdminID == 0 {
        t.Fatal("expected admin id to be stored")
    }
}
```

- [ ] **Step 2: Run the targeted test**

Run: `cd telegram-admin-bot && go test ./internal/session -run TestLoginPersistsTelegramAdminSession -v`
Expected: FAIL because the service does not exist yet

- [ ] **Step 3: Implement login, logout, and session validation**

```go
type Service struct {
    api      *dujiao.Client
    sessions storage.SessionStore
}

func (s *Service) Login(ctx context.Context, telegramUser int64, username, password string) (*SessionView, error)
func (s *Service) RequireSession(ctx context.Context, telegramUser int64) (*SessionView, error)
func (s *Service) Logout(ctx context.Context, telegramUser int64) error
```

- [ ] **Step 4: Implement audit helpers used by every write workflow**

Required methods:
- `LogActionStarted`
- `LogActionSucceeded`
- `LogActionFailed`
- `RenderRecentActionSummary`

- [ ] **Step 5: Run session and audit tests**

Run: `cd telegram-admin-bot && go test ./internal/session ./internal/audit -v`
Expected: PASS with login persistence and action log coverage

- [ ] **Step 6: Commit**

```bash
git add telegram-admin-bot/internal/session/service.go telegram-admin-bot/internal/session/service_test.go telegram-admin-bot/internal/audit/service.go telegram-admin-bot/internal/audit/service_test.go
git commit -m "feat: add session and audit services"
```

### Task 5: Add Telegram Runtime, Router, And Private-Chat Guards

**Files:**
- Create: `telegram-admin-bot/internal/telegram/bot.go`
- Create: `telegram-admin-bot/internal/telegram/router.go`
- Create: `telegram-admin-bot/internal/telegram/context.go`
- Create: `telegram-admin-bot/internal/telegram/handlers_auth.go`
- Create: `telegram-admin-bot/internal/telegram/handlers_menu.go`
- Create: `telegram-admin-bot/internal/telegram/handlers_session.go`
- Create: `telegram-admin-bot/internal/telegram/telegram_test.go`

- [ ] **Step 1: Write the failing private-chat guard test**

```go
func TestRejectsSensitiveCommandOutsidePrivateChat(t *testing.T) {
    result := handleUpdate(newGroupMessage("/login"))
    if !strings.Contains(result.Text, "仅支持私聊") {
        t.Fatalf("unexpected result: %#v", result)
    }
}
```

- [ ] **Step 2: Run the targeted test**

Run: `cd telegram-admin-bot && go test ./internal/telegram -run TestRejectsSensitiveCommandOutsidePrivateChat -v`
Expected: FAIL because the router does not exist yet

- [ ] **Step 3: Implement the long-polling bot and command router**

Required behaviors:
- register `/login`, `/logout`, `/sales`, `/restock`, `/ship`, `/batch_ship`, `/session`, `/help`
- reject sensitive commands outside private chat
- render a home keyboard after successful login

- [ ] **Step 4: Implement auth/session handlers**

Required handlers:
- `/login`
- `/logout`
- `/session`
- `/help`
- home menu callback entrypoints

- [ ] **Step 5: Run Telegram router tests**

Run: `cd telegram-admin-bot && go test ./internal/telegram -v`
Expected: PASS with private-chat guard and command dispatch coverage

- [ ] **Step 6: Commit**

```bash
git add telegram-admin-bot/internal/telegram/bot.go telegram-admin-bot/internal/telegram/router.go telegram-admin-bot/internal/telegram/context.go telegram-admin-bot/internal/telegram/handlers_auth.go telegram-admin-bot/internal/telegram/handlers_menu.go telegram-admin-bot/internal/telegram/handlers_session.go telegram-admin-bot/internal/telegram/telegram_test.go
git commit -m "feat: add telegram runtime and command router"
```

### Task 6: Implement The Sales Query Workflow

**Files:**
- Create: `telegram-admin-bot/internal/workflow/sales.go`
- Create: `telegram-admin-bot/internal/render/sales.go`
- Create: `telegram-admin-bot/internal/telegram/handlers_sales.go`
- Create: `telegram-admin-bot/internal/workflow/sales_test.go`

- [ ] **Step 1: Write the failing sales workflow test**

```go
func TestSalesTodayUsesDashboardOverviewRangeToday(t *testing.T) {
    deps := newSalesTestDeps(t)
    _, err := deps.Workflow.BuildOverview(context.Background(), deps.Session, "today")
    if err != nil {
        t.Fatal(err)
    }
    if deps.API.LastRange != "today" {
        t.Fatalf("expected range=today, got %s", deps.API.LastRange)
    }
}
```

- [ ] **Step 2: Run the targeted test**

Run: `cd telegram-admin-bot && go test ./internal/workflow -run TestSalesTodayUsesDashboardOverviewRangeToday -v`
Expected: FAIL because the sales workflow does not exist yet

- [ ] **Step 3: Implement the sales use case and message renderer**

```go
func (w *SalesWorkflow) BuildOverview(ctx context.Context, session *session.SessionView, rangeKey string) (*SalesOverviewView, error)

func RenderSalesOverview(v *SalesOverviewView) string
```

- [ ] **Step 4: Wire the workflow into buttons and `/sales`**

Supported inputs:
- `/sales today`
- `/sales week`
- `/sales month`
- home buttons for today / week / month

- [ ] **Step 5: Run workflow and handler tests**

Run: `cd telegram-admin-bot && go test ./internal/workflow ./internal/render ./internal/telegram -v`
Expected: PASS with today/week/month routing coverage

- [ ] **Step 6: Commit**

```bash
git add telegram-admin-bot/internal/workflow/sales.go telegram-admin-bot/internal/render/sales.go telegram-admin-bot/internal/telegram/handlers_sales.go telegram-admin-bot/internal/workflow/sales_test.go
git commit -m "feat: add sales query workflow"
```

### Task 7: Implement Auto-Fulfillment Restock With Preview-Confirm

**Files:**
- Create: `telegram-admin-bot/internal/workflow/restock.go`
- Create: `telegram-admin-bot/internal/workflow/confirm.go`
- Create: `telegram-admin-bot/internal/render/restock.go`
- Create: `telegram-admin-bot/internal/telegram/handlers_restock.go`
- Create: `telegram-admin-bot/internal/workflow/restock_test.go`

- [ ] **Step 1: Write the failing restock parser test**

```go
func TestParseRestockTextDropsBlankLinesAndDuplicates(t *testing.T) {
    items := ParseRestockText("A\n\nB\nA\n")
    if len(items) != 2 {
        t.Fatalf("expected 2 unique items, got %d", len(items))
    }
}
```

- [ ] **Step 2: Run the targeted test**

Run: `cd telegram-admin-bot && go test ./internal/workflow -run TestParseRestockTextDropsBlankLinesAndDuplicates -v`
Expected: FAIL because the restock workflow does not exist yet

- [ ] **Step 3: Implement parsing, product selection, and preview creation**

Required behaviors:
- support pasted text
- support uploaded `csv` and `txt`
- resolve product and optional SKU before preview
- generate preview summary and persist a pending action

- [ ] **Step 4: Implement confirmation execution**

Required confirmation behavior:
- validate pending action ownership
- reject expired or consumed confirms
- call `CreateCardSecretBatch`
- write audit log

- [ ] **Step 5: Run workflow and handler tests**

Run: `cd telegram-admin-bot && go test ./internal/workflow ./internal/render ./internal/telegram -v`
Expected: PASS with parsing, preview, and one-time confirmation coverage

- [ ] **Step 6: Commit**

```bash
git add telegram-admin-bot/internal/workflow/restock.go telegram-admin-bot/internal/workflow/confirm.go telegram-admin-bot/internal/render/restock.go telegram-admin-bot/internal/telegram/handlers_restock.go telegram-admin-bot/internal/workflow/restock_test.go
git commit -m "feat: add restock preview and confirmation flow"
```

### Task 8: Implement Single And Batch Fulfillment With Notification Hints

**Files:**
- Create: `telegram-admin-bot/internal/workflow/fulfillment_single.go`
- Create: `telegram-admin-bot/internal/workflow/fulfillment_batch.go`
- Create: `telegram-admin-bot/internal/workflow/notification_hint.go`
- Create: `telegram-admin-bot/internal/render/fulfillment.go`
- Create: `telegram-admin-bot/internal/telegram/handlers_fulfillment.go`
- Create: `telegram-admin-bot/internal/workflow/fulfillment_test.go`

- [ ] **Step 1: Write the failing notification-hint test**

```go
func TestNotificationHintSkipsTelegramWhenUserHasNoTelegramIdentity(t *testing.T) {
    deps := newFulfillmentTestDeps(t)
    hint, err := deps.Workflow.BuildNotificationHint(context.Background(), deps.Session, deps.OrderWithoutTelegram)
    if err != nil {
        t.Fatal(err)
    }
    if hint.Telegram.Status != "expected_skip" {
        t.Fatalf("unexpected telegram hint: %#v", hint.Telegram)
    }
}
```

- [ ] **Step 2: Run the targeted test**

Run: `cd telegram-admin-bot && go test ./internal/workflow -run TestNotificationHintSkipsTelegramWhenUserHasNoTelegramIdentity -v`
Expected: FAIL because the fulfillment workflow does not exist yet

- [ ] **Step 3: Implement single fulfillment preview and execution**

Required behaviors:
- search by order number
- fetch order detail
- reject orders that are not eligible for manual fulfillment
- collect payload or structured delivery data
- persist a confirmation action
- execute `CreateFulfillment` on confirm

- [ ] **Step 4: Implement notification-hint computation**

Required hint checks:
- email receiver presence
- SMTP enabled
- placeholder-email skip
- user telegram identity presence
- active `telegram_bot` channel client
- channel client callback URL
- Telegram runtime status

- [ ] **Step 5: Implement batch fulfillment**

Required behaviors:
- filter candidate orders
- build a batch preview
- execute serial per-order fulfillment
- collect per-order result and notification-hint summary

- [ ] **Step 6: Run fulfillment tests**

Run: `cd telegram-admin-bot && go test ./internal/workflow ./internal/render ./internal/telegram -v`
Expected: PASS with single fulfillment, batch fulfillment, and notification-hint coverage

- [ ] **Step 7: Commit**

```bash
git add telegram-admin-bot/internal/workflow/fulfillment_single.go telegram-admin-bot/internal/workflow/fulfillment_batch.go telegram-admin-bot/internal/workflow/notification_hint.go telegram-admin-bot/internal/render/fulfillment.go telegram-admin-bot/internal/telegram/handlers_fulfillment.go telegram-admin-bot/internal/workflow/fulfillment_test.go
git commit -m "feat: add fulfillment workflows and notification hints"
```

### Task 9: Finish Deployment Docs And End-To-End Verification

**Files:**
- Modify: `telegram-admin-bot/README.md`
- Modify: `telegram-admin-bot/.env.example`
- Modify: `telegram-admin-bot/Dockerfile`
- Modify: `telegram-admin-bot/docker-compose.example.yml`
- Create: `telegram-admin-bot/scripts/smoke_local.sh`

- [ ] **Step 1: Write the failing smoke test wrapper expectation**

```bash
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
go test ./...
```
Expected behavior: exits non-zero until all previous tasks are implemented

- [ ] **Step 2: Document the exact deployment contract**

README must cover:
- disabling admin login captcha for bot-based login
- required Dujiao-Next services: API, queue worker, SMTP if email is expected
- Telegram bot sidecar env vars
- Docker Compose networking examples
- “notification hint is not delivery proof” boundary

- [ ] **Step 3: Add the final verification commands**

Run:
- `cd telegram-admin-bot && go test ./...`
- `cd telegram-admin-bot && go test -race ./...`
- `cd telegram-admin-bot && go build ./cmd/bot`
- `cd telegram-admin-bot && docker build -t telegram-admin-bot:local .`

Expected:
- all tests PASS
- race build PASS
- binary build PASS
- Docker image build PASS

- [ ] **Step 4: Commit**

```bash
git add telegram-admin-bot/README.md telegram-admin-bot/.env.example telegram-admin-bot/Dockerfile telegram-admin-bot/docker-compose.example.yml telegram-admin-bot/scripts/smoke_local.sh
git commit -m "docs: finalize telegram admin bot deployment plan"
```

## Verification Notes

- Do not claim email delivery success from the bot. Only claim preview generation, API submission, and notification hint computation.
- Do not claim Telegram customer delivery success from the bot. Only claim that Dujiao-Next should attempt callback notification when the visible prerequisites are satisfied.
- Verification levels must stay distinct:
  - unit tests
  - API client `httptest` coverage
  - workflow tests
  - binary build
  - Docker build
  - optional manual live smoke test against a local Dujiao-Next instance

## Execution Guardrails

- Keep all files inside `telegram-admin-bot/`
- Do not modify the root `go.mod`
- Do not add features outside the approved scope
- Preserve the “preview then confirm” rule for every write operation
- Prefer serial execution for batch fulfillment; do not add concurrency until correctness is proven
