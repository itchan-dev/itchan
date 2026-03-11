# Technical Documentation

## Table of Contents

- [Architecture](#architecture)
- [Project Structure](#project-structure)
- [Backend Architecture](#backend-architecture)
- [Database Schema](#database-schema)
- [Configuration](#configuration)
- [API Endpoints](#api-endpoints)
- [Frontend](#frontend)
- [Testing](#testing)
- [Security Features](#security-features)
- [Deployment](#deployment)
- [Monitoring](#monitoring)
- [CI/CD](#cicd)

## Architecture

```
    Web Browsers   Mobile Apps    API Clients
    (Cookies)      (Bearer)       (Bearer)
        │              │              │
        └──────────────┼──────────────┘
                       ▼
┌────────────────────────────────────────────────────────────┐
│                      Docker Compose                        │
├────────────────────────────────────────────────────────────┤
│   Nginx (Reverse Proxy)                                    │
│   TLS termination, rate limiting, HTTP/2                   │
│   Ports 80/443                                             │
├──────────────┬──────────────────────┬──────────────────────┤
│   Frontend   │   Backend API        │   PostgreSQL 17.6    │
│   (Go HTML)  │   (REST Service)     │   (Partitioned DB)   │
│   Port 8081  │   Port 8080          │      Port 5432       │
│   (internal) │   (internal)         │      (internal)      │
└──────────────┴──────────────────────┴──────────────────────┘

Backend Layers:
  Handler    ← HTTP routing, JSON I/O, validation
     ↓
  Service    ← Business logic, authentication, file processing
     ↓
  Storage    ← PostgreSQL operations, partitioning, queries

Shared Modules:
  • domain/       - Domain types and models
  • middleware/   - Auth, rate limiting, board access control
  • config/       - Configuration management
  • jwt/          - JWT token handling
  • validation/   - Input validation and file handling
```

## Project Structure

```
itchan/
├── backend/                    # Backend API service
│   ├── cmd/itchan-api/        # Main entry point
│   ├── internal/
│   │   ├── handler/           # HTTP handlers (REST endpoints)
│   │   │   ├── auth.go        # Register, login, logout
│   │   │   ├── blacklist.go   # User blacklist management
│   │   │   ├── board.go       # Board CRUD operations
│   │   │   ├── handler.go     # Handler dependencies
│   │   │   ├── health.go      # Health and readiness probes
│   │   │   ├── helpers.go     # Shared handler helpers
│   │   │   ├── invite.go      # Invite code management
│   │   │   ├── message.go     # Message posting and retrieval
│   │   │   ├── thread.go      # Thread operations
│   │   │   └── user_activity.go
│   │   ├── service/           # Business logic layer
│   │   │   ├── auth.go        # Authentication logic (incl. invites)
│   │   │   ├── board.go
│   │   │   ├── gc.go          # Orphaned media cleanup
│   │   │   ├── media_storage.go
│   │   │   ├── message.go
│   │   │   ├── thread.go
│   │   │   ├── user_activity.go
│   │   │   └── utils/sanitize.go  # EXIF/metadata stripping
│   │   ├── storage/pg/        # PostgreSQL data access layer
│   │   │   ├── auth.go
│   │   │   ├── blacklist.go
│   │   │   ├── board.go
│   │   │   ├── board_enrichment.go
│   │   │   ├── board_view.go  # Materialized view management
│   │   │   ├── message.go
│   │   │   ├── message_enrichment.go
│   │   │   ├── pg.go          # DB connection & partitioning
│   │   │   ├── thread.go
│   │   │   ├── user_activity.go
│   │   │   ├── migrations/init.sql
│   │   │   └── templates/     # SQL templates for partitioning & views
│   │   ├── storage/fs/fs.go   # File upload/download
│   │   ├── utils/             # Backend utilities & email
│   │   ├── router/router.go   # All API routes and middleware
│   │   └── setup/setup.go     # Dependency injection
│
├── frontend/                   # Frontend UI service
│   ├── cmd/frontend/main.go
│   ├── internal/
│   │   ├── handler/           # HTTP handlers for pages
│   │   ├── apiclient/         # Backend API client
│   │   ├── domain/            # Frontend domain models
│   │   ├── markdown/          # Custom markdown parser
│   │   ├── middleware/        # Auth forwarding, CSRF
│   │   ├── router/router.go
│   │   └── setup/setup.go
│   ├── templates/             # HTML templates
│   └── static/                # CSS, JS, favicon
│
├── shared/                     # Shared packages
│   ├── api/                   # API request/response types
│   ├── blacklist/             # Blacklist caching
│   ├── config/
│   ├── crypto/                # AES-256-GCM email encryption
│   ├── csrf/
│   ├── domain/                # Domain models
│   ├── errors/
│   ├── jwt/
│   ├── logger/
│   ├── middleware/            # Auth, security headers, metrics, rate limiting
│   ├── storage/               # Storage interfaces
│   ├── utils/
│   └── validation/            # Input validation & file handling
│
├── nginx/                      # Reverse proxy config
├── monitoring/                 # Prometheus + Grafana
├── scripts/                    # Setup and maintenance scripts
├── config/                     # public.yaml + private.yaml
└── tools/                      # CLI utilities
```

## Backend Architecture

Itchan follows a strict **three-layer architecture**:

### Handler Layer (`internal/handler/`)
Parses HTTP requests, validates input, calls services, returns JSON responses.

### Service Layer (`internal/service/`)
Enforces business rules (bump limits, thread counts), coordinates storage operations, handles file uploads, manages transactions and auth logic.

### Storage Layer (`internal/storage/pg/`)
Executes SQL queries, manages partitioning and materialized views, maps rows to domain models.

## Database Schema

PostgreSQL table partitioning — each board gets its own partition.

### Key Tables

- **users** — accounts with encrypted email and bcrypt password
- **user_blacklist** — banned users with reason (cached for JWT validation)
- **confirmation_data** — email confirmation codes
- **invite_codes** — user-generated invite codes
- **boards** — board metadata
- **board_permissions** — email domain allowlist per board
- **threads** — partitioned by board; title, message count, bump time, pinned flag
- **messages** — partitioned by board; text, author, timestamps, ordinal
- **attachments** — partitioned by board; links messages to files
- **files** — file metadata, both original and sanitized filenames, dimensions, thumbnail path
- **message_replies** — partitioned by board; cross-thread reply relationships

### Materialized Views

- **board_previews** — pre-computed board views with last N messages per thread, refreshed on a configurable interval (`board_preview_refresh_interval`)

## Configuration

### `config/public.yaml` (shared between backend and frontend)

```yaml
jwt_ttl: 24h
threads_per_page: 15
messages_per_thread_page: 1000         # 0 = all
max_thread_count: 100                  # null = unlimited
n_last_msg: 3
bump_limit: 500
board_preview_refresh_internval: 30s
board_activity_window: 3m
blacklist_cache_interval: 300          # seconds

confirmation_code_ttl: 10m

log_level: info                        # debug, info, warn, error
log_format: text                       # text or json

secure_cookies: false                  # set true for HTTPS
csrf_enabled: true

# Text length limits
board_name_max_len: 10
board_short_name_max_len: 3
thread_title_max_len: 50
message_text_max_len: 10000
message_text_min_len: 1
password_min_len: 8
max_replies_per_message: 50

# File upload limits
max_attachments_per_message: 4
max_attachment_size_bytes: 10485760    # 10 MB
max_total_attachment_size: 20971520    # 20 MB
allowed_image_mime_types:
  - image/jpeg
  - image/png
  - image/gif
allowed_video_mime_types:
  - video/mp4
  - video/webm
  - video/ogg

# Invite system
invite_enabled: false
invite_code_length: 12
invite_code_ttl: 720h
max_invites_per_user: 5                # 0 = unlimited
min_account_age_for_invites: 720h

user_messages_page_limit: 50

# Caching
static_cache_max_age: 240h
media_cache_max_age: 168h

# Media processing
media:
  thumbnail_max_size: 225
  thumbnail_display_op: 225
  thumbnail_display_reply: 150
  jpeg_quality_main: 85
  jpeg_quality_thumbnail: 75

allowed_registration_domains: []       # empty = allow all
```

### `config/private.yaml` (generated — never commit)

```yaml
jwt_key: "<secret>"
encryption_key: "<aes-256-key>"        # generate with: go run ./tools/generate-encryption-key/

pg:
  host: localhost
  port: 5432
  user: itchan
  password: itchan
  dbname: itchan

email:
  smtp_server: smtp.gmail.com
  smtp_port: 587
  username: your-email@gmail.com
  password: your-app-password
  sender_name: "Itchan Imageboard"
  timeout: 10

# Only ?ref= values in this list are tracked (empty = allow all)
allowed_refs:
  - twitter
  - reddit
  - telegram
```

## API Endpoints

### Authentication

The API supports two methods:
- **Cookie-based**: login sets an HTTP-only JWT cookie (browsers)
- **Bearer Token**: login returns `access_token` in body (`Authorization: Bearer <token>`)

```
POST /v1/auth/register                 # rate limited: 1/s per email & IP
POST /v1/auth/check_confirmation_code  # 5 attempts per 10 min per email
POST /v1/auth/login                    # returns access_token
POST /v1/auth/register_with_invite     # rate limited: 1/s per IP
POST /v1/auth/logout
```

### Boards
```
GET  /v1/boards
GET  /v1/{board}
GET  /v1/{board}/last_modified
```

### Threads
```
POST /v1/{board}                       # create thread; rate limited: 1/min per user
GET  /v1/{board}/{thread}
GET  /v1/{board}/{thread}/last_modified
```

### Messages
```
POST /v1/{board}/{thread}              # post message; rate limited: 1/s per user
GET  /v1/{board}/{thread}/{message}
```

### Invites (authenticated)
```
GET    /v1/invites/
POST   /v1/invites/                    # rate limited: 1/min per user
DELETE /v1/invites/{codeHash}
```

### User (authenticated)
```
GET /v1/users/me/activity
GET /v1/public_config
```

### Admin
```
POST   /v1/admin/boards
DELETE /v1/admin/{board}
DELETE /v1/admin/{board}/{thread}
POST   /v1/admin/{board}/{thread}/pin
DELETE /v1/admin/{board}/{thread}/{message}
POST   /v1/admin/users/{userId}/blacklist
DELETE /v1/admin/users/{userId}/blacklist
GET    /v1/admin/blacklist
POST   /v1/admin/blacklist/refresh
```

### Health & Monitoring
```
GET /health    # liveness probe
GET /ready     # readiness probe (checks DB)
GET /metrics   # Prometheus metrics
```

### Rate Limits

| Endpoint | Limit |
|---|---|
| Registration | 1/s per email, 1/s per IP, 100 global RPS |
| Confirmation code | 5/10min per email, 1/s per IP |
| Login | 1/s per IP, 1000 global RPS |
| Invite registration | 1/s per IP, 100 global RPS |
| Public board reads (unauthenticated) | 10 RPS per IP |
| Create thread | 1/min per user |
| Post message | 1/s per user |
| Generate invite | 1/min per user |
| General authenticated | 100 RPS per user |
| Admin | No limits |

## Frontend

Server-rendered Go application using `html/template`.

### Templates

`base.html`, `index.html`, `board.html`, `thread.html`, `login.html`, `register.html`, `register_invite.html`, `check_confirmation_code.html`, `account.html`, `admin.html`, `invites.html`, `faq.html`, `about.html`, `contacts.html`, `privacy.html`, `terms.html`, `partials.html`

### Markdown

Custom lightweight parser: fenced code blocks, inline code, bold, italic, strikethrough, greentext (`>`), message links (`>>threadId#msgId`) with hover previews.

### Interactive Features

- Popup reply forms with intelligent positioning
- Hover message previews with 500-item cache and chain navigation
- File upload manager with real-time thumbnails and validation
- Hash-based reply links (`#reply-{id}`)
- Disable Media Mode: cookie-based toggle replacing images/videos with text placeholders

## Testing

```bash
# Backend
cd backend
go test ./internal/service/...        # unit tests
go test ./internal/storage/pg/...     # integration tests (requires PostgreSQL)
go test ./internal/handler/...        # handler tests

# Frontend
cd frontend
go test ./internal/handler/...

# All with coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

Integration tests use a separate test database and clean up after execution.

## Security Features

- **JWT auth** with configurable TTL; cookie + Bearer token support
- **Bcrypt** password hashing; **AES-256-GCM** email encryption
- **CSRF protection** (token-based)
- **Email confirmation** required for registration; optional domain allowlist
- **Blacklist cache**: automatic JWT rejection for banned users
- **Board access**: public (no auth) vs private (email domain check); posting always requires auth
- **Media sanitization**: EXIF stripping via decode/encode (images) and ffmpeg (video)
- **File validation**: MIME type and size limits
- **Multi-tier rate limiting**: Nginx + per-IP + per-user (token bucket, admin-exempt)
- **Security headers**: HSTS, CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy
- **Parameterized queries** throughout; template auto-escaping for XSS prevention
- **Real-IP forwarding**: frontend passes `X-Real-IP` so backend rate limits apply to end users

## Deployment

```bash
# Development
make dev

# Production
make deploy          # builds and starts nginx, frontend, api, postgres
make logs            # all logs
make logs-api
make logs-frontend
docker compose ps
```

See [SETUP.md](SETUP.md) for the full production guide including HTTPS/Let's Encrypt setup.

## Monitoring

Optional Prometheus + Grafana stack.

```bash
make deploy-monitoring   # enable
make deploy              # disable (redeploy without monitoring)
```

Grafana at `http://localhost:3000`, Prometheus at `http://localhost:9090`.

### Metrics

- `http_requests_total{method, path, status}`
- `http_request_duration_seconds{method, path}`
- `http_requests_in_flight`
- Go runtime metrics (goroutines, memory, GC)

## CI/CD

GitHub Actions: tests on PRs, tests + deploy on push to `main`.

Required secrets: `VPS_HOST`, `VPS_USER`, `VPS_SSH_KEY`, `VPS_PROJECT_PATH`.
