#!/bin/bash
# Generates config/private.yaml and nginx/nginx.conf from .env
# Called automatically by make deploy / make deploy-monitoring
set -e

cd "$(dirname "${BASH_SOURCE[0]}")/.."

if [ ! -f ".env" ]; then
    echo "Error: .env not found. Run ./scripts/setup.sh or copy .env.example to .env"
    exit 1
fi

# Export all variables from .env
set -a
source .env
set +a

go run ./tools/render-template/ -t config/private.yaml.tmpl -o config/private.yaml
go run ./tools/render-template/ -t nginx/nginx.conf.tmpl    -o nginx/nginx.conf
