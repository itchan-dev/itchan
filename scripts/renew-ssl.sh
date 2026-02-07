#!/bin/bash
# SSL certificate renewal. Runs via cron daily, renews only if expiring within 30 days.
set -e

cd "$(dirname "${BASH_SOURCE[0]}")/.."

docker run --rm \
    -v "$(pwd)/nginx/certs:/etc/letsencrypt" \
    -v "$(pwd)/nginx/certbot:/var/www/certbot" \
    certbot/certbot renew --webroot -w /var/www/certbot 2>&1

docker compose exec nginx nginx -s reload 2>/dev/null || true
