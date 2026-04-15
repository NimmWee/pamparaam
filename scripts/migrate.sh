#!/usr/bin/env sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
ENV_FILE="${ROOT_DIR}/.env"

if [ ! -f "${ENV_FILE}" ]; then
  ENV_FILE="${ROOT_DIR}/.env.example"
fi

set -a
. "${ENV_FILE}"
set +a

DIRECTION="${1:-up}"
NETWORK="${COMPOSE_PROJECT_NAME:-wiki-editor}_default"

run_migration() {
  service_name="$1"
  migration_dir="$2"
  database_url="$3"

  if [ ! -d "${ROOT_DIR}/${migration_dir}" ]; then
    echo "Skipping ${service_name}: missing ${migration_dir}"
    return
  fi

  if [ -z "$(find "${ROOT_DIR}/${migration_dir}" -maxdepth 1 -type f -name '*.sql' -print -quit)" ]; then
    echo "Skipping ${service_name}: no SQL migrations in ${migration_dir}"
    return
  fi

  if [ "${DIRECTION}" = "down" ]; then
    docker run --rm \
      --network "${NETWORK}" \
      -v "${ROOT_DIR}:/workspace" \
      migrate/migrate:v4.18.2 \
      -path="/workspace/${migration_dir}" \
      -database "${database_url}" \
      down 1
    return
  fi

  docker run --rm \
    --network "${NETWORK}" \
    -v "${ROOT_DIR}:/workspace" \
    migrate/migrate:v4.18.2 \
    -path="/workspace/${migration_dir}" \
    -database "${database_url}" \
    up
}

run_migration "auth-service" "services/auth-service/migrations" "${AUTH_DATABASE_URL}"
run_migration "page-service" "services/page-service/migrations" "${PAGE_DATABASE_URL}"
run_migration "knowledge-graph-search-service" "services/knowledge-graph-search-service/migrations" "${SEARCH_DATABASE_URL}"
run_migration "file-service" "services/file-service/migrations" "${FILE_DATABASE_URL}"
