#!/usr/bin/env sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
ENV_FILE="${ROOT_DIR}/.env"

if [ ! -f "${ENV_FILE}" ]; then
  cp "${ROOT_DIR}/.env.example" "${ENV_FILE}"
  echo "Created ${ENV_FILE} from .env.example"
fi

echo "Syncing Go workspace"
(cd "${ROOT_DIR}" && go work sync)

echo "Starting Docker Compose stack"
(cd "${ROOT_DIR}" && docker compose --env-file .env -f deploy/docker-compose.yml up -d --build)

echo "Running available migrations"
"${ROOT_DIR}/scripts/migrate.sh" up

echo "Seeding auth demo data"
(
  set -a
  . "${ENV_FILE}"
  set +a
  cd "${ROOT_DIR}/services/auth-service"
  go run ./cmd/seed-demo
)

cat <<EOF
Bootstrap complete.

Gateway:      http://localhost:8080
Prometheus:   http://localhost:9090
MinIO API:    http://localhost:9000
MinIO UI:     http://localhost:9001
MWS Mock:     http://localhost:8090
EOF
