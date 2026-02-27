# Itchan Imageboard

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![CI](https://github.com/itchan-dev/itchan/actions/workflows/ci.yml/badge.svg)](https://github.com/itchan-dev/itchan/actions/workflows/ci.yml)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-17.6-336791?logo=postgresql&logoColor=white)](https://www.postgresql.org/)

A modern, high-performance imageboard built with Go, featuring a clean three-layer architecture, advanced authentication, and comprehensive rate limiting. Designed for privacy-conscious communities with domain-based board access control.

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Tech Stack](#tech-stack)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Quick Start with Docker](#quick-start-with-docker)
  - [Local Development](#local-development)
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
- [Contributing](#contributing)
- [License](#license)

## Features

### Core Functionality
- **Boards & Threads**: Create boards with custom names, organize discussions in threads
- **Messages & Replies**: Post messages with cross-thread reply support
- **File Attachments**: Upload images (JPEG, PNG, GIF) and videos (MP4, WebM, OGG) with automatic thumbnail generation
- **Attachments-Only Messages**: Post messages with only attachments (text is optional)
- **Markdown Support**: Rich text formatting using a custom markdown parser
- **Thread Bumping**: Automatic bump-to-top with configurable bump limits
- **Pinned Threads**: Pin important threads to the top of boards
- **Pagination**: Efficient board browsing with configurable threads per page
- **Disable Media Mode**: Cookie-based toggle to replace all images and videos with text placeholders

### Authentication & Authorization
- **User Registration**: Email-based registration with confirmation codes
- **Invite System**: Registered users can generate invite codes to bring in friends without corporate email
- **JWT Authentication**: Secure, stateless authentication using JWT tokens
- **Dual Auth Methods**: Cookie-based (browsers) and Bearer token (API/mobile clients) support
- **Admin Roles**: Special privileges for board and content moderation
- **User Blacklist**: Admin-managed user blacklist with cached validation for banned users
- **Board Permissions**: Domain-based access control (restrict boards to specific email domains)
- **CSRF Protection**: Token-based CSRF protection for state-changing requests

### Performance & Security
- **Nginx Reverse Proxy**: TLS termination, HTTP/2, connection limits, slowloris protection
- **Two-Layer Rate Limiting**: Nginx (per-IP, connection limits) + Go (per-user, token bucket)
- **Media Sanitization**: Automatic EXIF/metadata stripping from images and videos using ffmpeg
- **Email Encryption**: AES-256-GCM encryption of email addresses at the application level
- **Database Partitioning**: Automatic table partitioning by board for optimal performance
- **Materialized Views**: Fast board previews with configurable refresh intervals
- **Security Headers**: HSTS, CSP, X-Frame-Options, X-Content-Type-Options
- **Gzip Compression**: Automatic response compression
- **CORS Support**: Configured for secure cross-origin requests
- **Graceful Shutdown**: Proper cleanup and connection handling

### Developer Experience
- **Clean Architecture**: Three-layer backend (Storage → Service → Handler)
- **Comprehensive Testing**: Unit and integration tests for all layers
- **Docker Deployment**: Full containerization with Docker Compose
- **Type Safety**: Strong domain types and validation
- **Middleware Chain**: Composable authentication, authorization, and rate limiting

## Architecture

```
    Web Browsers   Mobile Apps    API Clients
    (Cookies)      (Bearer)       (Bearer)
        │              │              │
        └──────────────┼──────────────┘
                       ▼
┌─────────────────────────────────────────────────────────────┐
│                      Docker Compose                          │
├──────────────────────────────────────────────────────────────┤
│   Nginx (Reverse Proxy)                                      │
│   TLS termination, rate limiting, HTTP/2                     │
│   Ports 80/443                                               │
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

## Tech Stack

- **Backend**: Go 1.24+ (frontend-agnostic JSON REST API)
- **Frontend**: Go templates (html/template) - replaceable with any client
- **Database**: PostgreSQL 17.6 with partitioning
- **Reverse Proxy**: Nginx (TLS termination, rate limiting, HTTP/2)
- **Router**: Chi (go-chi/chi)
- **Auth**: JWT (golang-jwt/jwt) with dual-mode support (Cookie + Bearer)
- **TLS**: Let's Encrypt (automatic renewal via certbot)
- **Markdown**: Custom parser
- **Media Processing**: ffmpeg (for metadata sanitization)
- **Monitoring**: Prometheus + Grafana (optional)
- **Container**: Docker & Docker Compose
- **CI/CD**: GitHub Actions
- **Email**: SMTP support for confirmations

## Getting Started

### Prerequisites

- **Docker** 20.10+ and **Docker Compose** 2.0+
- **Go** 1.24+ (for local development)
- **PostgreSQL** 17.6+ (if running without Docker)
- **ffmpeg** (for media sanitization - automatically included in Docker image)

### Production Deployment

See **[SETUP.md](SETUP.md)** for full production setup guide with HTTPS and nginx.

```bash
git clone https://github.com/itchan-dev/itchan.git
cd itchan
chmod +x scripts/*.sh
./scripts/setup.sh yourdomain.com your-email@example.com
# Configure email in config/private.yaml
make deploy
```

### Quick Start (Development)

For local development without nginx/HTTPS:

```bash
git clone https://github.com/itchan-dev/itchan.git
cd itchan
docker-compose up --build

# Frontend UI: http://localhost:8081
# Backend API: http://localhost:8080/v1
```

The database will be automatically initialized with the schema from `backend/internal/storage/pg/migrations/init.sql`.

### Local Development

For development without Docker:

```bash
# 1. Start PostgreSQL (adjust connection settings in config/private.yaml)
# Make sure PostgreSQL is running on localhost:5432

# 2. Run database migrations
psql -U itchan -d itchan -f backend/internal/storage/pg/migrations/init.sql

# 3. Configure your environment
cp .env.example .env
# Edit .env with your database and email settings
make gen-configs

# 4. Start the backend API
cd backend
go run cmd/itchan-api/main.go -config_folder ../config

# 5. In a new terminal, start the frontend
cd frontend
go run cmd/frontend/main.go -config_folder ../config
```

## Project Structure

```
itchan/
├── backend/                    # Backend API service
│   ├── cmd/
│   │   ├── itchan-api/        # Main entry point
│   │   │   └── main.go
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
│   │   │   └── user_activity.go # User activity tracking
│   │   ├── service/           # Business logic layer
│   │   │   ├── auth.go        # Authentication logic (incl. invites)
│   │   │   ├── board.go       # Board management
│   │   │   ├── gc.go          # Orphaned media cleanup
│   │   │   ├── media_storage.go # File storage service
│   │   │   ├── message.go     # Message processing
│   │   │   ├── thread.go      # Thread management
│   │   │   ├── user_activity.go # User activity tracking
│   │   │   └── utils/         # Service utilities
│   │   │       └── sanitize.go # Media sanitization (EXIF/metadata stripping)
│   │   ├── storage/           # Data access layer
│   │   │   ├── pg/            # PostgreSQL implementation
│   │   │   │   ├── auth.go    # User storage operations
│   │   │   │   ├── blacklist.go # User blacklist operations
│   │   │   │   ├── board.go   # Board storage
│   │   │   │   ├── board_enrichment.go # Board data enrichment
│   │   │   │   ├── board_view.go  # Materialized view management
│   │   │   │   ├── message.go # Message storage
│   │   │   │   ├── message_enrichment.go # Message data enrichment
│   │   │   │   ├── pg.go      # DB connection & partitioning
│   │   │   │   ├── thread.go  # Thread storage
│   │   │   │   ├── user_activity.go # User activity storage
│   │   │   │   ├── migrations/init.sql  # Database schema
│   │   │   │   └── templates/ # SQL templates for partitioning & views
│   │   │   └── fs/            # File system storage
│   │   │       └── fs.go      # File upload/download
│   │   ├── utils/             # Backend utilities
│   │   │   ├── utils.go       # Helper functions
│   │   │   └── email/         # Email sending service
│   │   ├── router/            # Route configuration
│   │   │   └── router.go      # All API routes and middleware
│   │   └── setup/             # Dependency injection
│   │       └── setup.go
│   └── Dockerfile
│
├── frontend/                   # Frontend UI service
│   ├── cmd/frontend/          # Main entry point
│   │   └── main.go
│   ├── internal/
│   │   ├── handler/           # HTTP handlers for pages
│   │   │   ├── account.go     # Account management page
│   │   │   ├── auth.go        # Login/register pages
│   │   │   ├── blacklist.go   # Blacklist management page
│   │   │   ├── board.go       # Board view
│   │   │   ├── faq.go         # FAQ page
│   │   │   ├── handler.go     # Handler dependencies
│   │   │   ├── helpers.go     # Shared handler helpers
│   │   │   ├── index.go       # Homepage
│   │   │   ├── invites.go     # Invite management page
│   │   │   ├── legal.go       # Legal pages (terms, privacy, contacts)
│   │   │   ├── message.go     # Message actions
│   │   │   ├── render.go      # Template rendering
│   │   │   ├── settings.go    # User settings
│   │   │   └── thread.go      # Thread view
│   │   ├── apiclient/         # Backend API client
│   │   │   ├── apiclient.go
│   │   │   ├── auth.go
│   │   │   ├── blacklist.go
│   │   │   ├── board.go
│   │   │   ├── invites.go
│   │   │   ├── message.go
│   │   │   ├── thread.go
│   │   │   └── user_activity.go
│   │   ├── domain/            # Frontend domain models
│   │   │   ├── board.go
│   │   │   ├── common.go      # Common template data
│   │   │   ├── message.go
│   │   │   ├── pages.go       # Page-specific data structs
│   │   │   ├── partials.go    # Partial-specific data structs
│   │   │   └── thread.go
│   │   ├── markdown/          # Markdown processing
│   │   │   ├── parser.go      # Custom parser with block/inline rules
│   │   │   └── trie.go        # Trie for fast marker matching
│   │   ├── middleware/        # Frontend middleware
│   │   │   ├── auth.go        # Authentication forwarding
│   │   │   └── csrf.go        # CSRF token handling
│   │   ├── router/            # Route configuration
│   │   │   └── router.go
│   │   └── setup/             # Dependency setup
│   │       └── setup.go
│   ├── templates/             # HTML templates
│   │   ├── base.html          # Base layout
│   │   ├── index.html         # Board list
│   │   ├── board.html         # Board view
│   │   ├── thread.html        # Thread view
│   │   ├── login.html         # Login page
│   │   ├── register.html      # Registration page
│   │   ├── register_invite.html # Invite-based registration
│   │   ├── check_confirmation_code.html # Email confirmation
│   │   ├── account.html       # Account management
│   │   ├── admin.html         # Admin panel
│   │   ├── invites.html       # Invite management
│   │   ├── faq.html           # FAQ and rules
│   │   ├── about.html         # About page
│   │   ├── contacts.html      # Contact information
│   │   ├── privacy.html       # Privacy policy
│   │   ├── terms.html         # Terms of service
│   │   └── partials.html      # Reusable components
│   ├── static/                # Static assets
│   │   ├── css/style.css
│   │   ├── js/main.js
│   │   └── favicon.ico
│   └── Dockerfile
│
├── shared/                     # Shared packages
│   ├── api/                   # API request/response types
│   ├── blacklist/             # Blacklist caching
│   ├── config/                # Configuration management
│   ├── crypto/                # Email encryption (AES-256-GCM)
│   ├── csrf/                  # CSRF token protection
│   ├── domain/                # Domain models
│   │   ├── attachments.go     # File attachment models
│   │   ├── auth.go            # User, credentials
│   │   ├── board.go           # Board models
│   │   ├── helpers.go         # Domain helper functions
│   │   ├── message.go         # Message models
│   │   ├── thread.go          # Thread models
│   │   └── types.go           # Custom types and validation
│   ├── errors/                # Error types
│   ├── jwt/                   # JWT utilities
│   ├── logger/                # Structured logging
│   ├── middleware/            # HTTP middleware
│   │   ├── auth.go            # JWT authentication
│   │   ├── security_headers.go # Security headers (HSTS, CSP, etc.)
│   │   ├── board_access/      # Board permission check
│   │   ├── metrics/           # Prometheus metrics
│   │   └── ratelimiter/       # Rate limiting
│   ├── storage/               # Storage interfaces
│   ├── utils/                 # Utility functions
│   │   ├── crypto.go          # Hashing utilities
│   │   └── web.go             # HTTP helpers
│   └── validation/            # Input validation
│       ├── attachment.go      # File validation
│       ├── multipart.go       # Multipart form handling
│       └── errors.go
│
├── nginx/                      # Reverse proxy
│   ├── nginx.conf             # Nginx configuration
│   ├── nginx.conf.explained   # Line-by-line explanation
│   ├── certs/                 # SSL certificates (generated)
│   └── certbot/               # ACME challenge files
│
├── monitoring/                 # Prometheus + Grafana
│   ├── prometheus.yml         # Prometheus configuration
│   └── grafana/               # Grafana dashboards
│
├── scripts/                    # Setup and maintenance
│   ├── setup.sh               # Full initial setup (configs + SSL + cron)
│   ├── gen-configs.sh         # Generate configuration files
│   └── renew-ssl.sh           # SSL certificate renewal (called by cron)
│
├── config/                     # Configuration files
│   ├── public.yaml            # Public configuration (shared with frontend)
│   └── private.yaml           # Private configuration (backend only, generated)
│
├── tools/                      # CLI utilities (run from project root)
│   ├── generate-encryption-key/ # Generate AES-256 encryption key
│   └── render-template/       # Render Go templates from env vars
│
├── docker-compose.yml         # Docker orchestration (production)
├── docker-compose.dev.yml     # Docker orchestration (development)
├── docker-compose.monitoring.yml # Monitoring stack
├── Makefile                   # Build and deployment commands
├── SETUP.md                   # Production setup guide
├── CONTRIBUTING.md            # Contribution guidelines
├── SECURITY.md                # Security policy
├── CODE_OF_CONDUCT.md         # Code of conduct
├── go.mod                     # Go module definition
├── go.sum                     # Go module checksums
└── README.md
```

## Backend Architecture

Itchan follows a strict **three-layer architecture** for maintainability and testability:

### 1. Handler Layer (`internal/handler/`)
**Responsibility**: HTTP request/response handling

- Parse HTTP requests and extract parameters
- Validate input format and types
- Call appropriate service methods
- Convert service responses to HTTP responses
- Handle HTTP-specific errors (404, 401, etc.)

**Example**: `handler/board.go`
```go
func (h *Handler) CreateBoard(w http.ResponseWriter, r *http.Request) {
    // Parse request body
    // Validate input
    // Call service.Board.Create()
    // Return JSON response
}
```

### 2. Service Layer (`internal/service/`)
**Responsibility**: Business logic and orchestration

- Enforce business rules (bump limits, thread counts, etc.)
- Coordinate between multiple storage operations
- Handle file uploads and processing
- Manage transactions
- Authentication and authorization logic

**Example**: `service/thread.go`
```go
func (s *Thread) Create(ctx context.Context, data domain.ThreadCreationData) error {
    // Check thread count limits
    // Validate board exists
    // Create thread in storage
    // Trigger GC if needed
}
```

### 3. Storage Layer (`internal/storage/pg/`)
**Responsibility**: Data persistence

- Execute SQL queries
- Manage database connections
- Handle table partitioning
- Manage materialized views
- Map between database rows and domain models

**Example**: `storage/pg/board.go`
```go
func (s *Storage) CreateBoard(ctx context.Context, board *domain.Board) error {
    // Execute INSERT statement
    // Create partition for new board
    // Return database errors
}
```

## Database Schema

The database uses PostgreSQL's **table partitioning** for scalability. Each board gets its own partition.

### Key Tables

- **users**: User accounts with encrypted email and password hash
- **user_blacklist**: Blacklisted users with reason and admin reference (cached for JWT validation)
- **confirmation_data**: Email confirmation codes for registration
- **invite_codes**: User-generated invite codes for registration
- **boards**: Board metadata (name, short_name, activity timestamps)
- **board_permissions**: Domain-based access control (email domain allowlist per board)
- **threads**: Thread metadata (title, message count, bump time, pinned flag) - **partitioned by board**
- **messages**: Individual posts (text, author, timestamps, ordinal) - **partitioned by board**
- **attachments**: Links messages to uploaded files - **partitioned by board**
- **files**: File metadata with both original and sanitized filenames/MIME types (path, size, dimensions, thumbnail path)
- **message_replies**: Cross-reference table for reply relationships - **partitioned by board**

### Materialized Views

- **board_previews**: Pre-computed board views with last N messages per thread
  - Refreshed on configurable intervals (`board_preview_refresh_interval`)
  - Dramatically improves board page load times

### Indexes

Optimized indexes for common queries:
- Thread lookup by board and bump time
- Message lookup by thread
- Reply lookup by receiver message
- Author lookup for moderation

## Configuration

Configuration is split into two files:

### `config/public.yaml` (Shared between backend and frontend)
```yaml
jwt_ttl: 24h                           # JWT token lifetime
threads_per_page: 15                   # Pagination
messages_per_thread_page: 1000         # Messages per thread page (0 = all)
max_thread_count: 100                  # Max threads per board (null = unlimited)
n_last_msg: 3                          # Messages shown in board preview
bump_limit: 500                        # Messages before thread stops bumping
board_preview_refresh_internval: 30s   # Materialized view refresh rate
board_activity_window: 3m              # How far back to check for board activity
blacklist_cache_interval: 300          # Blacklist cache refresh interval (seconds)

# Auth
confirmation_code_ttl: 10m             # Email confirmation code expiration

# Logging
log_level: info                        # debug, info, warn, error
log_format: text                       # text or json

# Security settings
secure_cookies: false                  # Set to true for HTTPS
csrf_enabled: true                     # Enable CSRF protection

# Text length limits (defaults shown, all optional)
board_name_max_len: 10
board_short_name_max_len: 3
thread_title_max_len: 50
message_text_max_len: 10000
message_text_min_len: 1
password_min_len: 8
max_replies_per_message: 50            # Max >>reply links per message

# File upload limits
max_attachments_per_message: 4
max_attachment_size_bytes: 10485760    # 10 MB per file
max_total_attachment_size: 20971520    # 20 MB total
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
invite_code_ttl: 720h                  # 30 days
max_invites_per_user: 5                # 0 = unlimited
min_account_age_for_invites: 720h      # 30 days

# User activity page
user_messages_page_limit: 50           # Messages shown on account page

# Caching
static_cache_max_age: 240h             # Static file cache (CSS, JS)
media_cache_max_age: 168h              # User-uploaded media cache

# Media processing
media:
  thumbnail_max_size: 225              # Max dimension (px) for thumbnails
  thumbnail_display_op: 225            # Display size for OP thumbnails
  thumbnail_display_reply: 150         # Display size for reply thumbnails
  jpeg_quality_main: 85               # JPEG quality for main images (0-100)
  jpeg_quality_thumbnail: 75          # JPEG quality for thumbnails (0-100)

# Registration restrictions
allowed_registration_domains: []       # Empty = allow all domains. Example: ["gmail.com", "company.com"]
```

### `config/private.yaml` (Backend only - never commit to git!)
```yaml
jwt_key: "<your-secret-key>"           # JWT signing key (generate random 512+ bit string)
encryption_key: "<your-encryption-key>" # AES-256-GCM key for email encryption (generate with: go run ./tools/generate-encryption-key/)

pg:
  host: localhost
  port: 5432
  user: itchan
  password: itchan
  dbname: itchan

email:
  smtp_server: smtp.gmail.com
  smtp_port: 587                          # 587 = STARTTLS, 465 = implicit TLS (Yandex)
  username: your-email@gmail.com
  password: your-app-password
  sender_name: "Itchan Imageboard"
  timeout: 10

# Referral source allowlist (optional)
# Only ?ref= values in this list are tracked and saved to the database.
# Empty or omitted = allow all sources (no filtering).
allowed_refs:
  - twitter
  - reddit
  - telegram
```

## API Endpoints

### Authentication Methods

The API supports **two authentication methods**:

1. **Cookie-based** (automatic for browsers):
   - Login sets an HTTP-only cookie with the JWT
   - Subsequent requests include the cookie automatically
   - Best for browser-based clients

2. **Bearer Token** (for API/mobile clients):
   - Login returns `access_token` in response body
   - Include token in requests: `Authorization: Bearer <token>`
   - Best for mobile apps, CLI tools, third-party integrations

### Authentication Endpoints
```
POST   /v1/auth/register                 - Register new user (rate limited: 1/s per email & IP)
POST   /v1/auth/check_confirmation_code  - Verify email confirmation code (5 attempts per 10 min per email)
POST   /v1/auth/login                    - Login with credentials (returns access_token)
POST   /v1/auth/register_with_invite     - Register with an invite code (rate limited: 1/s per IP)
POST   /v1/auth/logout                   - Logout (clear cookies)
```

**Login Response:**
```json
{
  "message": "You logged in",
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### Boards (Authenticated)
```
GET    /v1/boards                        - List all accessible boards
GET    /v1/{board}                       - Get board with threads (paginated)
GET    /v1/{board}/last_modified         - Get board's last modification timestamp
```

**Example with Bearer Token:**
```bash
curl -H "Authorization: Bearer <your_token>" \
  http://localhost:8080/v1/boards
```

### Threads (Authenticated)
```
POST   /v1/{board}                       - Create new thread (rate limited: 1/minute per user)
GET    /v1/{board}/{thread}              - Get thread with all messages
GET    /v1/{board}/{thread}/last_modified - Get thread's last modification timestamp
```

### Messages (Authenticated)
```
POST   /v1/{board}/{thread}              - Post message to thread (rate limited: 1/second per user)
GET    /v1/{board}/{thread}/{message}    - Get single message
```

### Invites (Authenticated)
```
GET    /v1/invites/                      - Get user's invite codes
POST   /v1/invites/                      - Generate new invite code (rate limited: 1/minute per user)
DELETE /v1/invites/{codeHash}            - Revoke an invite code
```

### User (Authenticated)
```
GET    /v1/users/me/activity             - Get current user's activity data
GET    /v1/public_config                 - Get public configuration
```

### Admin (Admin only)
```
POST   /v1/admin/boards                  - Create new board
DELETE /v1/admin/{board}                 - Delete board
DELETE /v1/admin/{board}/{thread}        - Delete thread
POST   /v1/admin/{board}/{thread}/pin    - Toggle thread pinned status
DELETE /v1/admin/{board}/{thread}/{message} - Delete message
POST   /v1/admin/users/{userId}/blacklist - Blacklist user (with optional reason)
DELETE /v1/admin/users/{userId}/blacklist - Remove user from blacklist
GET    /v1/admin/blacklist               - Get all blacklisted users with details
POST   /v1/admin/blacklist/refresh       - Manually refresh blacklist cache
```

### Health & Monitoring (No auth)
```
GET    /health                           - Liveness probe (200 if running)
GET    /ready                            - Readiness probe (200 if DB connected)
GET    /metrics                          - Prometheus metrics
```

### Rate Limits

- **Registration**: 1/s per email, 1/s per IP, 100 global RPS
- **Confirmation Code**: 5 attempts per 10 minutes per email, 1/s per IP, 100 global RPS
- **Login**: 1/s per IP, 1000 global RPS
- **Invite Registration**: 1/s per IP, 100 global RPS
- **Create Thread**: 1 per minute per user
- **Post Message**: 1 per second per user
- **Generate Invite**: 1 per minute per user
- **Get Board**: 10 RPS per user
- **General authenticated**: 100 RPS per user
- **Admin**: No rate limits

### Example Usage

**Browser/Frontend (Cookie-based):**
```javascript
// Login - cookie set automatically
await fetch('http://localhost:8080/v1/auth/login', {
  method: 'POST',
  credentials: 'include',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email: 'user@example.com', password: 'pass' })
});

// Subsequent requests - cookie sent automatically
await fetch('http://localhost:8080/v1/boards', { credentials: 'include' });
```

**API/Mobile Clients (Bearer Token):**
```bash
# Login and extract token
TOKEN=$(curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"pass"}' \
  | jq -r '.access_token')

# Use token in requests
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/boards
```

**Python:**
```python
import requests

# Login
response = requests.post('http://localhost:8080/v1/auth/login',
    json={'email': 'user@example.com', 'password': 'pass'})
token = response.json()['access_token']

# Use token
headers = {'Authorization': f'Bearer {token}'}
boards = requests.get('http://localhost:8080/v1/boards', headers=headers)
```

See [BEARER_AUTH_EXAMPLE.md](BEARER_AUTH_EXAMPLE.md) for more examples (React Native, Flutter, Node.js, etc.)

## Frontend

The frontend is a server-rendered Go application using `html/template`.

### Templates

- **base.html**: Common layout (header, footer, navigation)
- **index.html**: Board listing and creation form
- **board.html**: Thread list with pagination
- **thread.html**: Full thread view with all messages and reply form
- **login.html** / **register.html** / **register_invite.html**: Authentication pages
- **check_confirmation_code.html**: Email confirmation page
- **account.html**: Account management
- **admin.html**: Admin panel
- **invites.html**: Invite code management
- **faq.html** / **about.html** / **contacts.html**: Info pages
- **privacy.html** / **terms.html**: Legal pages
- **partials.html**: Reusable components (message rendering, forms)

### Markdown Support

The frontend includes a lightweight custom markdown parser optimized for imageboard use:
- Code blocks (fenced with ```)
- Inline code, bold, italic, strikethrough
- Greentext (lines starting with >)
- Message links (>>threadId#msgId) with hover previews

### Interactive Features

- **Popup Reply System**: Dynamic popup forms for quick replies with intelligent positioning
- **Message Preview System**: Hover over message links to see inline previews with caching (500 message cache)
- **Preview Chains**: Navigate through reply chains with automatic cleanup and smart timeout handling
- **File Upload Manager**: Real-time file preview with thumbnails, size validation, and individual file removal
- **Hash-based Reply Links**: Support for #reply-{id} URLs to auto-populate reply forms
- **Disable Media Mode**: Toggle in header to hide all images/videos, showing only filenames and metadata as text links (useful for slow connections, persists via cookie, works without JS)

### Static Assets

- **CSS**: Minimal, responsive stylesheet
- **JavaScript**: Vanilla JS for dynamic reply forms, message previews, and file upload management
- **Favicon**: Custom imageboard icon

## Testing

Comprehensive test coverage across all layers:

```bash
# Backend tests
cd backend

# Unit tests (service layer)
go test ./internal/service/...

# Integration tests (storage layer - requires running PostgreSQL)
go test ./internal/storage/pg/...

# Handler tests
go test ./internal/handler/...

# Frontend tests
cd ../frontend
go test ./internal/handler/...

# Run all tests with coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Test Structure

- **Unit tests**: Business logic validation (service layer)
- **Integration tests**: Database operations (storage layer with test DB)
- **Handler tests**: HTTP endpoint behavior (mocked services)

Integration tests use a separate test database and clean up after execution.

## Security Features

### Authentication & Authorization
- **JWT-based authentication** with configurable TTL
- **Dual authentication support**: Cookie-based (browsers) and Bearer token (API clients)
- **Invite system**: Users can generate invite codes for registration without corporate email
- **Blacklist cache validation**: Automatic JWT rejection for blacklisted users with cached validation
- **Secure cookie storage** with HttpOnly and Secure flags (when enabled)
- **Password hashing** using bcrypt
- **Email encryption**: AES-256-GCM encryption of email addresses at the application level
- **CSRF protection**: Token-based CSRF protection for state-changing requests
- **Email confirmation** required for registration
- **Registration domain restrictions**: Optional email domain allowlist for registration
- **Admin role** for moderation capabilities

### Rate Limiting
- **Multi-tier rate limiting**: Per-user, per-IP, and global limits
- **Token bucket algorithm** with configurable refill rates
- **Admin exemption**: Admin users bypass rate limits
- **Fingerprinting fallback**: Uses request fingerprint if IP unavailable

### Board Access Control
- **Domain-based permissions**: Restrict boards to specific email domains
- **Middleware enforcement**: Board access checked on every request
- **Automatic filtering**: Users only see boards they can access

### Input Validation & Media Processing
- **File type validation**: Strict MIME type checking for uploads
- **File size limits**: Per-file and total attachment size limits
- **Media sanitization**: Automatic EXIF/metadata stripping from images (decode/encode) and videos (ffmpeg)
- **Format normalization**: GIF/WebP images converted to JPEG, PNG preserved
- **Image dimension extraction**: Metadata validation
- **SQL injection prevention**: Parameterized queries throughout
- **XSS prevention**: Template auto-escaping, markdown sanitization

### Infrastructure
- **Nginx reverse proxy**: TLS termination, multi-tier rate limiting, connection limits, slowloris protection
- **Let's Encrypt**: Automatic SSL certificate provisioning and renewal
- **Security headers**: HSTS, CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy
- **CORS configuration**: Controlled cross-origin access
- **Gzip compression**: Automatic response compression (nginx)
- **Graceful shutdown**: Proper connection cleanup
- **Database partitioning**: Isolation between boards

## Deployment

### Development

```bash
make dev  # or: docker compose up --build
```

### Production

Full setup guide: **[SETUP.md](SETUP.md)**

```bash
make deploy          # Build and start all services (nginx, frontend, api, postgres)
```

```bash
docker compose ps    # View running services
make logs            # View all logs
make logs-api        # View API logs
make logs-frontend   # View frontend logs
```

## Monitoring

Optional Prometheus + Grafana stack for metrics and dashboards.

### Enable Monitoring

```bash
make deploy-monitoring
```

Access:
- **Grafana**: http://localhost:3000 (default: admin/admin)
- **Prometheus**: http://localhost:9090

### Disable Monitoring

```bash
make deploy  # Redeploy without monitoring compose file
docker compose stop prometheus grafana  # Stop monitoring services
```

### Available Metrics

- `http_requests_total{method, path, status}` - Request count
- `http_request_duration_seconds{method, path}` - Latency histogram
- `http_requests_in_flight` - Concurrent requests gauge
- Go runtime metrics (goroutines, memory, GC)

### Health Endpoints

- `GET /health` - Liveness probe (200 if running)
- `GET /ready` - Readiness probe (200 if DB connected)
- `GET /metrics` - Prometheus metrics (backend only)

## CI/CD

Automated testing and deployment via GitHub Actions.

### Workflow

1. **On Pull Request**: Run tests
2. **On Push to main**: Run tests → Deploy to VPS

### Setup

Add these secrets in GitHub (Settings → Secrets → Actions):

- `VPS_HOST` - Your server IP/hostname
- `VPS_USER` - SSH username
- `VPS_SSH_KEY` - Private SSH key content
- `VPS_PROJECT_PATH` - Path to project on VPS (e.g., `/home/user/itchan`)

## Contributing

Contributions are welcome! Please follow these guidelines:

1. **Fork** the repository
2. **Create a feature branch** (`git checkout -b feature/amazing-feature`)
3. **Write tests** for your changes
4. **Ensure all tests pass** (`go test ./...`)
5. **Format your code** (`go fmt ./...`)
6. **Commit your changes** (`git commit -m 'Add amazing feature'`)
7. **Push to the branch** (`git push origin feature/amazing-feature`)
8. **Open a Pull Request**

### Code Style

- Follow standard Go conventions (`gofmt`, `golint`)
- Write descriptive commit messages
- Add comments for complex logic
- Keep functions focused and testable
- Maintain the three-layer architecture separation

## License

MIT License - see [LICENSE](LICENSE) for details.

---

**Built with ❤️ using Go**
