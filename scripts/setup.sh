#!/bin/bash
# Full Itchan setup: generates configs, obtains SSL certificate, sets up auto-renewal
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

# 1. Generate .env
if [ ! -f ".env" ]; then
    PG_PASSWORD=$(generate_password)
    cp .env.example .env
    sed -i "s/<generate-strong-password-here>/${PG_PASSWORD}/g" .env
    echo "✓ .env created"
else
    echo "• .env already exists, skipping"
fi

# 2. Generate config/private.yaml
if [ ! -f "config/private.yaml" ]; then
    cp config/private.yaml.example config/private.yaml
    sed -i "s|<generate-random-base64-string>|$(generate_secret)|" config/private.yaml
    sed -i "s|<generate-random-base64-string>|$(generate_secret)|" config/private.yaml
    source .env
    sed -i "s|<your-postgres-password>|${POSTGRES_PASSWORD}|" config/private.yaml
    echo "✓ config/private.yaml created"
    echo ""
    echo "⚠ Configure email settings in config/private.yaml before deploying!"
    echo "  smtp_server, smtp_port, username, password, sender_name"
    echo ""
else
    echo "• config/private.yaml already exists, skipping"
fi

# 3. Set domain in nginx.conf
sed -i "s/DOMAIN_PLACEHOLDER/${DOMAIN}/g" nginx/nginx.conf 2>/dev/null || true
echo "$DOMAIN" > nginx/.domain
echo "✓ nginx.conf configured for ${DOMAIN}"

# 4. Obtain SSL certificate
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

# 5. Setup auto-renewal cron (daily at 3 AM)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CRON_CMD="0 3 * * * cd $(pwd) && ${SCRIPT_DIR}/renew-ssl.sh >> /var/log/itchan-ssl-renew.log 2>&1"

crontab -l 2>/dev/null | grep -v "renew-ssl.sh" | { cat; echo "$CRON_CMD"; } | crontab -
echo "✓ SSL auto-renewal cron installed (daily 3 AM)"

echo ""
echo "=== Setup complete ==="
echo "1. Configure email in config/private.yaml"
echo "2. Run: make deploy"
echo "3. Visit: https://${DOMAIN}"
