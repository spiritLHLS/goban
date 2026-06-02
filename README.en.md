# Goban - Bilibili Comment Monitoring and Auto-Report System

[中文](./README.md)

Goban is a Go + Vue full-stack application for monitoring Bilibili video comments across multiple creators, matching comments with keyword or regular-expression rules, and reporting matched comments with a logged-in Bilibili account. It includes a Web UI, SQLite persistence, Docker deployment, CSV export, whitelist support, Webhook notifications, and monitor status metrics.

> This project is for learning and research only. Follow Bilibili rules, applicable laws, and platform rate limits. You are responsible for the consequences of using it.

## Features

- Bilibili account management: QR login, Cookie login, and Cookie validity checks.
- Multi-creator monitoring: one task can monitor multiple UP user IDs.
- Keyword rules: plain text and regular expressions, case sensitivity, and live preview.
- Whitelist: skip comments from selected UIDs or usernames.
- Report throttling: global serialized limiter, defaulting to one report every 6 seconds.
- Cron scheduler: duplicate-run protection and configurable task concurrency.
- API retries: exponential backoff with jitter for Bilibili API failures.
- Comment pagination: fetches multiple comment pages instead of only the first page.
- Monitor status: checked comments, matched comments, report counts, recent status, and errors.
- Report history: filter by task, creator, keyword, status, and time; export CSV.
- Webhook notifications: Telegram and Feishu notifications after successful reports.
- SQLite persistence: accounts, tasks, targets, rules, whitelist, settings, logs, and report records.
- Encrypted Cookie storage: AES-GCM with `GOBAN_SECRET_KEY`, falling back to `PASSWORD` if unset.

## Architecture

```text
goban/
├── server/                 Go backend
│   ├── main.go             Entry point
│   └── internal/
│       ├── bili/           Bilibili API client, login, comments, reports
│       ├── config/         Environment configuration
│       ├── controllers/    HTTP API controllers
│       ├── database/       SQLite initialization and default settings
│       ├── middleware/     Basic Auth
│       ├── models/         GORM models
│       ├── monitor/        Cron scheduler, executor, limiter, Cookie checks
│       ├── notify/         Telegram and Feishu Webhooks
│       ├── rules/          Plain and regex matching
│       ├── secure/         Cookie encryption
│       ├── settings/       Runtime settings
│       └── whitelist/      Whitelist matcher
├── web/                    Vue 3 + Element Plus frontend
├── Dockerfile              Multi-stage frontend/backend image build
├── docker-compose.yml      Docker Compose example
└── .github/workflows/      Release and Docker image workflows
```

The backend exposes Gin APIs under `/api`. Production frontend assets are served by the backend. In development, Vite proxies `/api` to the backend.

## Requirements

- Go 1.24+
- Node.js 24+
- npm 10+
- Docker 24+ optional

## Docker

```bash
docker compose up -d
```

Open:

```text
http://localhost:38080
```

Default credentials:

```text
admin / admin123
```

Change `USERNAME`, `PASSWORD`, and `GOBAN_SECRET_KEY` in production.

## Manual Development

Backend:

```bash
cd server
go mod download
DB_PATH=../data/goban.db \
USERNAME=admin \
PASSWORD=admin123 \
GOBAN_SECRET_KEY=change-me \
go run .
```

Frontend:

```bash
cd web
npm ci
npm run dev
```

The frontend runs on `http://localhost:3000` and proxies APIs to `http://localhost:8080`.

## Environment Variables

| Variable | Description | Default |
| --- | --- | --- |
| `PORT` | Backend port | `8080` |
| `USERNAME` | Web Basic Auth username | `admin` |
| `PASSWORD` | Web Basic Auth password and fallback encryption key | `admin123` |
| `GOBAN_SECRET_KEY` | Cookie encryption key, required for production | empty, falls back to `PASSWORD` |
| `DB_PATH` | SQLite database path | `data/goban.db` |
| `DEBUG` | Gin debug mode | `false` |
| `MAX_CONCURRENT_TASKS` | Maximum concurrent monitor tasks | `2` |
| `TZ` | Container timezone | `Asia/Shanghai` |

## Usage

1. Sign in to the Web UI.
2. Add a Bilibili account by QR login or Cookie login.
3. Create keyword rules and preview them against sample comments.
4. Add whitelist entries when some users should never trigger reports.
5. Create a monitor task, select an account, enter one or more UP user IDs, choose rules, and configure intervals, retries, and proxy settings.
6. Watch counters and recent errors in Monitor Status.
7. Filter and export report history in Report Records.
8. Tune defaults and Webhook notifications in Settings.

## API Overview

All `/api` endpoints require Basic Auth:

```http
Authorization: Basic base64(username:password)
```

Common endpoints:

- `GET /api/users/list`
- `GET /api/users/login`
- `GET /api/users/loginCheck`
- `POST /api/users/loginByCookie`
- `POST /api/users/:id/check`
- `GET /api/tasks/list`
- `POST /api/tasks/create`
- `PUT /api/tasks/:id`
- `GET /api/tasks/:id/test`
- `GET /api/keywords/list`
- `POST /api/keywords/preview`
- `GET /api/whitelist/list`
- `GET /api/status`
- `GET /api/settings` / `PUT /api/settings`
- `GET /api/logs/monitor`
- `GET /api/logs/report`
- `GET /api/logs/report/export`
- `GET /health` without authentication

## Database

Goban uses SQLite by default. `DB_PATH` controls the database location. Main tables include:

- `bili_users`
- `monitor_tasks`
- `monitor_targets`
- `keyword_rules`
- `whitelist_users`
- `app_settings`
- `monitor_logs`
- `report_records`

This version does not guarantee compatibility with older database schemas. To reinitialize, stop the service, delete the database file pointed to by `DB_PATH`, and start the service again.

## CI/CD

GitHub Actions use Node.js 24 runtime action versions and set:

```yaml
FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true
```

The release workflow builds frontend assets and multi-platform backend binaries. The Docker workflow builds `linux/amd64` and `linux/arm64` images.

## License

See [LICENSE](./LICENSE).
