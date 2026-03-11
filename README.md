# Itchan Imageboard

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![CI](https://github.com/itchan-dev/itchan/actions/workflows/ci.yml/badge.svg)](https://github.com/itchan-dev/itchan/actions/workflows/ci.yml)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-17.6-336791?logo=postgresql&logoColor=white)](https://www.postgresql.org/)

A modern, high-performance imageboard built with Go. Designed for privacy-conscious communities with domain-based board access control.

## Features

- **Boards, Threads & Messages** with file attachments (images and video), markdown, and cross-thread replies
- **Authentication**: JWT-based with cookie and Bearer token support, email confirmation, invite system
- **Access Control**: Public boards readable without login; private boards restricted by email domain
- **Admin Tools**: Board/thread/message moderation, user blacklist
- **Rate Limiting**: Multi-tier (Nginx + per-IP + per-user)
- **Media Sanitization**: EXIF/metadata stripping via ffmpeg
- **CSRF Protection**, security headers, gzip, graceful shutdown

## Quick Start

```bash
git clone https://github.com/itchan-dev/itchan.git
cd itchan
docker-compose up --build

# Frontend: http://localhost:8081
# API:      http://localhost:8080/v1
```

## Production Deployment

```bash
chmod +x scripts/*.sh
./scripts/setup.sh yourdomain.com your-email@example.com
make deploy
```

See [SETUP.md](SETUP.md) for the full guide.

## Tech Stack

- **Backend**: Go 1.24+ — REST API (Chi router, JWT auth)
- **Frontend**: Go templates (`html/template`)
- **Database**: PostgreSQL 17.6 with table partitioning and materialized views
- **Proxy**: Nginx (TLS, HTTP/2, rate limiting)
- **Media**: ffmpeg for sanitization
- **Monitoring**: Prometheus + Grafana (optional)
- **Container**: Docker & Docker Compose

## Documentation

- [TECHNICAL.md](TECHNICAL.md) — Architecture, API reference, configuration, database schema, security details
- [SETUP.md](SETUP.md) — Production setup guide
- [CONTRIBUTING.md](CONTRIBUTING.md) — Contribution guidelines
- [SECURITY.md](SECURITY.md) — Security policy

## License

MIT — see [LICENSE](LICENSE).
