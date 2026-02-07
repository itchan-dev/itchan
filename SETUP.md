# Production Setup

## Prerequisites

| Requirement | Check command | Install |
|---|---|---|
| Docker 20.10+ | `docker --version` | `curl -fsSL https://get.docker.com \| sh` |
| Docker Compose 2.0+ | `docker compose version` | Included with Docker |
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

This script generates all configs, obtains SSL certificate, and sets up auto-renewal.

**Configure email** (required for registration):

```bash
nano config/private.yaml
```

Update the `email:` section with your SMTP credentials:

```yaml
email:
  smtp_server: "smtp.yandex.ru"   # or smtp.gmail.com
  smtp_port: 465                   # 465 for Yandex, 587 for Gmail
  username: "your-email"
  password: "your-app-password"
  sender_name: "Itchan"
  timeout: 10
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
# Update
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

**Email not sending** — check SMTP settings in `config/private.yaml`. Test: `docker compose logs api | grep -i smtp`.

## Generated Files (do not commit)

- `.env` — PostgreSQL credentials
- `config/private.yaml` — JWT keys, DB password, email settings
- `nginx/certs/` — SSL certificates
- `nginx/.domain` — domain name for scripts
