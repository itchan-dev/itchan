!! project still in development

# Itchan Imageboard

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
- [Contributing](#contributing)
- [License](#license)

## Features

### Core Functionality
- **Boards & Threads**: Create boards with custom names, organize discussions in threads
- **Messages & Replies**: Post messages with cross-thread reply support
- **File Attachments**: Upload images (PNG, JPEG, GIF, WebP) and videos with automatic thumbnail generation
- **Attachments-Only Messages**: Post messages with only attachments (text is optional)
- **Markdown Support**: Rich text formatting using a custom markdown parser
- **Thread Bumping**: Automatic bump-to-top with configurable bump limits
- **Sticky Threads**: Pin important threads to the top of boards
- **Pagination**: Efficient board browsing with configurable threads per page

### Authentication & Authorization
- **User Registration**: Email-based registration with confirmation codes
- **JWT Authentication**: Secure, stateless authentication using JWT tokens
- **Admin Roles**: Special privileges for board and content moderation
- **User Blacklist**: Admin-managed user blacklist with cached validation for banned users
- **Board Permissions**: Domain-based access control (restrict boards to specific email domains)

### Performance & Security
- **Media Sanitization**: Automatic EXIF/metadata stripping from images and videos using ffmpeg
- **Rate Limiting**: Multi-tier rate limiting (per-user, per-IP, global)
- **Database Partitioning**: Automatic table partitioning by board for optimal performance
- **Materialized Views**: Fast board previews with configurable refresh intervals
- **Security Headers**: CSP, X-Frame-Options, and other security headers
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
┌─────────────────────────────────────────────────────────────┐
│                      Docker Compose                          │
├──────────────┬──────────────────────┬──────────────────────┤
│   Frontend   │      Backend API      │   PostgreSQL 17.6    │
│   (Go HTML)  │    (REST Service)     │   (Partitioned DB)   │
│   Port 8081  │      Port 8080        │      Port 5432       │
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

- **Backend**: Go 1.24+
- **Frontend**: Go templates (html/template)
- **Database**: PostgreSQL 17.6 with partitioning
- **Router**: Gorilla Mux
- **Auth**: JWT (golang-jwt/jwt)
- **Markdown**: Goldmark
- **Media Processing**: ffmpeg (for metadata sanitization)
- **Container**: Docker & Docker Compose
- **Email**: SMTP support for confirmations

## Getting Started

### Prerequisites

- **Docker** 20.10+ and **Docker Compose** 2.0+
- **Go** 1.24+ (for local development)
- **PostgreSQL** 17.6+ (if running without Docker)
- **ffmpeg** (for media sanitization - automatically included in Docker image)

### Quick Start with Docker

The easiest way to run Itchan is with Docker Compose:

```bash
# Clone the repository
git clone https://github.com/itchan-dev/itchan.git
cd itchan

# Start all services
docker-compose up --build

# Access the application
# Frontend UI: http://localhost:8081
# Backend API: http://localhost:8080/v1
# Database:    localhost:5432
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
cp config/private.yaml.example config/private.yaml
# Edit config/private.yaml with your database and email settings

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
│   ├── cmd/itchan-api/        # Main entry point
│   │   └── main.go
│   ├── internal/
│   │   ├── handler/           # HTTP handlers (REST endpoints)
│   │   │   ├── auth.go        # Register, login, logout
│   │   │   ├── blacklist.go   # User blacklist management
│   │   │   ├── board.go       # Board CRUD operations
│   │   │   ├── thread.go      # Thread operations
│   │   │   ├── message.go     # Message posting and retrieval
│   │   │   └── handler.go     # Handler dependencies
│   │   ├── service/           # Business logic layer
│   │   │   ├── auth.go        # Authentication logic
│   │   │   ├── board.go       # Board management
│   │   │   ├── thread.go      # Thread management
│   │   │   ├── message.go     # Message processing
│   │   │   ├── fs.go          # File storage service
│   │   │   ├── gc.go          # Board garbage collection
│   │   │   ├── thread_gc.go   # Thread cleanup
│   │   │   └── utils/         # Service utilities
│   │   │       └── sanitize.go # Media sanitization (EXIF/metadata stripping)
│   │   ├── storage/           # Data access layer
│   │   │   ├── pg/            # PostgreSQL implementation
│   │   │   │   ├── auth.go    # User storage operations
│   │   │   │   ├── blacklist.go # User blacklist operations
│   │   │   │   ├── board.go   # Board storage
│   │   │   │   ├── board_view.go  # Materialized view management
│   │   │   │   ├── thread.go  # Thread storage
│   │   │   │   ├── message.go # Message storage
│   │   │   │   ├── pg.go      # DB connection & partitioning
│   │   │   │   └── migrations/init.sql  # Database schema
│   │   │   └── fs/            # File system storage
│   │   │       └── fs.go      # File upload/download
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
│   │   │   ├── index.go       # Homepage
│   │   │   ├── board.go       # Board view
│   │   │   ├── thread.go      # Thread view
│   │   │   ├── message.go     # Message actions
│   │   │   ├── auth.go        # Login/register pages
│   │   │   └── render.go      # Template rendering
│   │   ├── apiclient/         # Backend API client
│   │   │   ├── apiclient.go
│   │   │   ├── auth.go
│   │   │   ├── board.go
│   │   │   ├── thread.go
│   │   │   └── message.go
│   │   ├── domain/            # Frontend domain models
│   │   │   ├── board.go
│   │   │   ├── thread.go
│   │   │   └── message.go
│   │   ├── markdown/          # Markdown processing
│   │   │   ├── parser.go      # Custom parser
│   │   │   └── processing.go  # Text processing
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
│   │   └── partials.html      # Reusable components
│   ├── static/                # Static assets
│   │   ├── css/style.css
│   │   ├── js/main.js
│   │   └── favicon.ico
│   └── Dockerfile
│
├── shared/                     # Shared packages
│   ├── domain/                # Domain models
│   │   ├── auth.go            # User, credentials
│   │   ├── board.go           # Board models
│   │   ├── thread.go          # Thread models
│   │   ├── message.go         # Message models
│   │   ├── attachments.go     # File attachment models
│   │   └── types.go           # Custom types and validation
│   ├── middleware/            # HTTP middleware
│   │   ├── auth.go            # JWT authentication
│   │   ├── board_access/      # Board permission check
│   │   │   └── board_access.go
│   │   └── ratelimiter/       # Rate limiting
│   │       └── ratelimiter.go
│   ├── validation/            # Input validation
│   │   ├── attachment.go      # File validation
│   │   ├── multipart.go       # Multipart form handling
│   │   └── errors.go
│   ├── config/                # Configuration management
│   │   └── config.go
│   ├── jwt/                   # JWT utilities
│   │   └── jwt.go
│   ├── storage/               # Storage interfaces
│   │   └── storage.go
│   └── utils/                 # Utility functions
│       ├── crypto.go          # Hashing utilities
│       └── web.go             # HTTP helpers
│
├── config/                     # Configuration files
│   ├── public.yaml            # Public configuration (shared with frontend)
│   └── private.yaml           # Private configuration (backend only)
│
├── docker-compose.yml         # Docker orchestration
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

- **users**: User accounts with email and password hash
- **user_blacklist**: Blacklisted users with reason and admin reference (cached for JWT validation)
- **confirmation_data**: Email confirmation codes for registration
- **boards**: Board metadata (name, short_name, activity timestamps)
- **board_permissions**: Domain-based access control (email domain allowlist per board)
- **threads**: Thread metadata (title, message count, bump time, sticky flag) - **partitioned by board**
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
max_thread_count: 100                  # Max threads per board (null = unlimited)
n_last_msg: 3                          # Messages shown in board preview
bump_limit: 500                        # Messages before thread stops bumping
board_preview_refresh_internval: 30s   # Materialized view refresh rate
secure_cookies: false                  # Set to true for HTTPS

# Text length limits
board_name_max_len: 100
board_short_name_max_len: 10
thread_title_max_len: 200
message_text_max_len: 10000
message_text_min_len: 1
password_min_len: 8

# File upload limits
max_attachments_per_message: 4
max_attachment_size_bytes: 10485760    # 10 MB per file
max_total_attachment_size: 20971520    # 20 MB total
allowed_image_mime_types:
  - image/jpeg
  - image/png
  - image/gif
  - image/webp
allowed_video_mime_types:
  - video/mp4
  - video/webm
```

### `config/private.yaml` (Backend only - never commit to git!)
```yaml
jwt_key: "<your-secret-key>"           # JWT signing key (generate random 512+ bit string)

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
  use_tls: true
  skip_verify: false
  timeout: 10
```

## API Endpoints

### Authentication
```
POST   /v1/auth/register                 - Register new user (rate limited: 1/10s per IP)
POST   /v1/auth/check_confirmation_code  - Verify email confirmation code
POST   /v1/auth/login                    - Login with credentials
POST   /v1/auth/logout                   - Logout (clear cookies)
```

### Boards (Authenticated)
```
GET    /v1/boards                        - List all accessible boards
GET    /v1/{board}                       - Get board with threads (paginated)
```

### Threads (Authenticated)
```
POST   /v1/{board}                       - Create new thread (rate limited: 1/minute per user)
GET    /v1/{board}/{thread}              - Get thread with all messages
DELETE /v1/admin/{board}/{thread}        - Delete thread (admin only)
```

### Messages (Authenticated)
```
POST   /v1/{board}/{thread}              - Post message to thread (rate limited: 1/second per user)
GET    /v1/{board}/{thread}/{message}    - Get single message
DELETE /v1/admin/{board}/{thread}/{message} - Delete message (admin only)
```

### Admin (Admin only)
```
POST   /v1/admin/boards                  - Create new board
DELETE /v1/admin/{board}                 - Delete board
POST   /v1/admin/users/{userId}/blacklist - Blacklist user (with optional reason)
DELETE /v1/admin/users/{userId}/blacklist - Remove user from blacklist
GET    /v1/admin/blacklist               - Get all blacklisted users with details
POST   /v1/admin/blacklist/refresh       - Manually refresh blacklist cache
```

### Rate Limits

- **Registration**: 1 request per 10 seconds per IP, 100 global RPS
- **Login**: 1 request per second per IP, 1000 global RPS
- **Create Thread**: 1 per minute per user
- **Post Message**: 1 per second per user (fixed, not token bucket)
- **Get Board**: 10 RPS per user
- **General authenticated**: 100 RPS per user
- **Admin**: No rate limits

## Frontend

The frontend is a server-rendered Go application using `html/template`.

### Templates

- **base.html**: Common layout (header, footer, navigation)
- **index.html**: Board listing and creation form
- **board.html**: Thread list with pagination
- **thread.html**: Full thread view with all messages and reply form
- **login.html** / **register.html**: Authentication pages
- **partials.html**: Reusable components (message rendering, forms)

### Markdown Support

The frontend includes a custom markdown parser built on Goldmark:
- Code blocks with syntax highlighting
- Inline code, bold, italic, strikethrough
- Links (auto-detected and manually created)
- Quote blocks
- Lists (ordered and unordered)

### Interactive Features

- **Popup Reply System**: Dynamic popup forms for quick replies with intelligent positioning
- **Message Preview System**: Hover over message links to see inline previews with caching (500 message cache)
- **Preview Chains**: Navigate through reply chains with automatic cleanup and smart timeout handling
- **File Upload Manager**: Real-time file preview with thumbnails, size validation, and individual file removal
- **Hash-based Reply Links**: Support for #reply-{id} URLs to auto-populate reply forms

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
- **Blacklist cache validation**: Automatic JWT rejection for blacklisted users with cached validation
- **Secure cookie storage** with HttpOnly and Secure flags (when enabled)
- **Password hashing** using bcrypt
- **Email confirmation** required for registration
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
- **Security headers**: Content Security Policy (CSP), X-Frame-Options, X-Content-Type-Options
- **CORS configuration**: Controlled cross-origin access
- **Gzip compression**: Automatic response compression
- **Graceful shutdown**: Proper connection cleanup
- **Database partitioning**: Isolation between boards

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

MIT © 2025 Itchan Imageboard

See [LICENSE](LICENSE) for details.

---

**Built with ❤️ using Go**
