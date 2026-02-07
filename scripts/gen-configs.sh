#!/bin/bash
# Generates config/private.yaml and nginx/nginx.conf from templates + .env
# Called automatically by make deploy / make deploy-monitoring
set -e

cd "$(dirname "${BASH_SOURCE[0]}")/.."

if [ ! -f ".env" ]; then
    echo "Error: .env not found. Run ./scripts/setup.sh or copy .env.example to .env"
    exit 1
fi

# Export all variables from .env for envsubst
set -a
source .env
set +a

# Generate config/private.yaml
PRIVATE_VARS='${JWT_KEY} ${ENCRYPTION_KEY} ${POSTGRES_USER} ${POSTGRES_PASSWORD} ${POSTGRES_DB} ${SMTP_SERVER} ${SMTP_PORT} ${SMTP_USERNAME} ${SMTP_PASSWORD} ${SMTP_SENDER_NAME}'
envsubst "$PRIVATE_VARS" < config/private.yaml.example > config/private.yaml

if grep -q '${' config/private.yaml 2>/dev/null; then
    echo "⚠ Warning: some variables were not substituted in config/private.yaml"
    grep '${' config/private.yaml
fi
echo "✓ config/private.yaml"

# Generate nginx/nginx.conf
if [ -n "$DOMAIN" ] && [ "$DOMAIN" != "yourdomain.com" ]; then
    sed "s/DOMAIN_PLACEHOLDER/${DOMAIN}/g" nginx/nginx.conf.example > nginx/nginx.conf
    echo "✓ nginx/nginx.conf (${DOMAIN})"
else
    echo "• nginx/nginx.conf skipped (DOMAIN not set in .env)"
fi
