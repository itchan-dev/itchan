#!/bin/bash
# Generates config/private.yaml and nginx/nginx.conf from .env
# Called automatically by make deploy / make deploy-monitoring
set -e

cd "$(dirname "${BASH_SOURCE[0]}")/.."

if [ ! -f ".env" ]; then
    echo "Error: .env not found. Run ./scripts/setup.sh or copy .env.example to .env"
    exit 1
fi

docker build -q -t itchan-renderer tools/render-template/ >/dev/null

docker run --rm \
    --env-file .env \
    -v "$(pwd)/templates:/templates:ro" \
    -v "$(pwd)/config:/config" \
    itchan-renderer \
    /templates/private.yaml.j2 /config/private.yaml

docker run --rm \
    --env-file .env \
    -v "$(pwd)/templates:/templates:ro" \
    -v "$(pwd)/nginx:/nginx" \
    itchan-renderer \
    /templates/nginx.conf.j2 /nginx/nginx.conf

echo "✓ configs generated"
