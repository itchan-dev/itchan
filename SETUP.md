# Production Setup

## Prerequisites

| Requirement | Check command | Install |
|---|---|---|
| Docker 20.10+ | `docker --version` | `curl -fsSL https://get.docker.com \| sh` |
| Docker Compose 2.0+ | `docker compose version` | Included with Docker |
| Go 1.24+ | `go version` | `curl -fsSL https://go.dev/dl/go1.24.linux-amd64.tar.gz \| tar -C /usr/local -xz` |
| OpenSSL | `openssl version` | `apt-get install -y openssl` |
| Ports 80, 443 free | `ss -tlnp \| grep -E ':80\|:443'` | Stop conflicting services |
| Domain → server IP | `dig +short yourdomain.com` | Set A record in DNS registrar |

## Setup

```bash
git clone https://github.com/itchan-dev/itchan.git
cd itchan
chmod +x scripts/*.sh
./scripts/setup.sh yourdomain.com your-email@example.com
```

This script generates `.env` with random secrets, renders `config/private.yaml` and `nginx/nginx.conf` from templates, obtains an SSL certificate, and sets up auto-renewal.

**Configure email** (required for registration):

```bash
nano .env
```

Update the SMTP settings (configs are regenerated on `make deploy`):

```env
SMTP_SERVER=smtp.yandex.ru     # or smtp.gmail.com
SMTP_PORT=465                   # 465 for Yandex, 587 for Gmail
SMTP_USERNAME=your-email@example.com
SMTP_PASSWORD=your-app-password
SMTP_SENDER_NAME=Itchan
```

**Deploy:**

```bash
make deploy
```

Verify: `curl -I https://yourdomain.com/health` should return `HTTP/2 200`.

## Create Admin

```bash
# Register at https://yourdomain.com/register, then:
docker compose exec postgres psql -U itchan -d itchan \
  -c "UPDATE users SET admin = true WHERE email = 'your-email@example.com';"
```

## Maintenance

```bash
# Update (configs are regenerated automatically from .env)
git pull && make deploy

# Logs
make logs                    # all
docker compose logs nginx    # nginx only

# Backup database
docker compose exec postgres pg_dump -U itchan itchan > backup.sql

# Manual SSL renewal (auto-renewal runs daily via cron)
./scripts/renew-ssl.sh

# Check cron
crontab -l | grep renew-ssl
```

## Troubleshooting

**502 Bad Gateway** — backend not ready yet. Check: `docker compose ps`, all should be `Up`.

**SSL errors** — check certificates exist: `ls nginx/certs/live/yourdomain.com/`.

**Email not sending** — check SMTP settings in `.env`. Test: `docker compose logs api | grep -i smtp`.

## Generated Files (do not commit)

- `.env` — all secrets (DB, JWT, SMTP, domain); edit this to configure the app
- `config/private.yaml` — generated from `config/private.yaml.tmpl` + `.env` by `make gen-configs`
- `nginx/nginx.conf` — generated from `nginx/nginx.conf.tmpl` + `.env` by `make gen-configs`
- `nginx/certs/` — SSL certificates
