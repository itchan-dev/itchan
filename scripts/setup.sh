#!/bin/bash
# First-time setup: generates .env with secrets, obtains SSL certificate, sets up auto-renewal
# Usage: ./scripts/setup.sh <domain> <email>
# Example: ./scripts/setup.sh itchan.example.com admin@example.com

set -e

DOMAIN=$1
EMAIL=$2

if [ -z "$DOMAIN" ] || [ -z "$EMAIL" ]; then
    echo "Usage: ./scripts/setup.sh <domain> <email>"
    echo "Example: ./scripts/setup.sh itchan.example.com admin@example.com"
    exit 1
fi

if [ ! -f "docker-compose.yml" ]; then
    echo "Error: run from project root"
    exit 1
fi

generate_secret() { openssl rand -base64 48; }
generate_password() { openssl rand -base64 32 | tr -d "=+/" | cut -c1-25; }

# 1. Generate .env with random secrets
if [ ! -f ".env" ]; then
    cp .env.example .env
    sed -i "s|DOMAIN=.*|DOMAIN=${DOMAIN}|" .env
    sed -i "s|POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=$(generate_password)|" .env
    sed -i "s|JWT_KEY=.*|JWT_KEY=$(generate_secret)|" .env
    sed -i "s|ENCRYPTION_KEY=.*|ENCRYPTION_KEY=$(generate_secret)|" .env
    echo "✓ .env created"
    echo ""
    echo "⚠ Configure email settings in .env before deploying!"
    echo "  SMTP_SERVER, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD"
    echo ""
else
    echo "• .env already exists, skipping"
fi

# 2. Generate configs from templates + .env
./scripts/gen-configs.sh

# 3. Obtain SSL certificate
mkdir -p nginx/certs nginx/certbot
echo "Obtaining SSL certificate for ${DOMAIN}..."
docker compose down 2>/dev/null || true

docker run --rm \
    -p 80:80 \
    -v "$(pwd)/nginx/certs:/etc/letsencrypt" \
    certbot/certbot certonly --standalone \
    -d "$DOMAIN" \
    --email "$EMAIL" \
    --agree-tos \
    --no-eff-email \
    --non-interactive

echo "✓ SSL certificate obtained"

# 4. Setup auto-renewal cron (daily at 3 AM)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CRON_CMD="0 3 * * * cd $(pwd) && ${SCRIPT_DIR}/renew-ssl.sh >> /var/log/itchan-ssl-renew.log 2>&1"

crontab -l 2>/dev/null | grep -v "renew-ssl.sh" | { cat; echo "$CRON_CMD"; } | crontab -
echo "✓ SSL auto-renewal cron installed (daily 3 AM)"

echo ""
echo "=== Setup complete ==="
echo "1. Configure email in .env"
echo "2. Run: make deploy"
echo "3. Visit: https://${DOMAIN}"
