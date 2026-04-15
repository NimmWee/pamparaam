#!/usr/bin/env sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
ENV_FILE="${ROOT_DIR}/.env"

if [ ! -f "${ENV_FILE}" ]; then
  cp "${ROOT_DIR}/.env.example" "${ENV_FILE}"
  echo "Created ${ENV_FILE} from .env.example"
fi

COMPOSE="docker compose --env-file ${ENV_FILE} -f ${ROOT_DIR}/deploy/docker-compose.yml"
NETWORK="${COMPOSE_PROJECT_NAME:-wiki-editor}_default"

on_error() {
  echo "Compose smoke test failed. Recent service state:"
  ${COMPOSE} ps || true
  ${COMPOSE} logs --tail=100 || true
}
trap on_error INT TERM HUP

echo "Starting demo runtime"
${COMPOSE} up -d --build

echo "Running migrations"
"${ROOT_DIR}/scripts/migrate.sh" up

echo "Seeding auth demo data"
${COMPOSE} exec -T auth-service /app/seed-demo

wait_health() {
  url="$1"
  name="$2"
  attempts=0
  until curl -fsS "$url" >/dev/null 2>&1; do
    attempts=$((attempts + 1))
    if [ "$attempts" -ge 60 ]; then
      echo "Timed out waiting for ${name} at ${url}" >&2
      on_error
      exit 1
    fi
    sleep 2
  done
}

wait_health "http://localhost:8081/health/ready" "auth-service"
wait_health "http://localhost:8082/health/ready" "page-service"
wait_health "http://localhost:8083/health/ready" "collaboration-service"
wait_health "http://localhost:8084/health/ready" "search-service"
wait_health "http://localhost:8085/health/ready" "mws-integration-service"
wait_health "http://localhost:8086/health/ready" "file-service"
wait_health "http://localhost:8080/health/ready" "gateway"

check_env() {
  service="$1"
  variable="$2"
  if ! ${COMPOSE} exec -T "$service" sh -lc "printenv ${variable} >/dev/null"; then
    echo "Missing ${variable} in ${service}" >&2
    on_error
    exit 1
  fi
}

check_env gateway AUTH_SERVICE_GRPC_ADDR
check_env page-service AUTH_SERVICE_GRPC_ADDR
check_env page-service MWS_INTEGRATION_SERVICE_GRPC_ADDR
check_env page-service FILE_SERVICE_GRPC_ADDR
check_env page-service PAGE_REDIS_ADDR
check_env page-service PAGE_NATS_URL
check_env collaboration-service PAGE_SERVICE_GRPC_ADDR
check_env collaboration-service AUTH_SERVICE_GRPC_ADDR
check_env collaboration-service COLLABORATION_REDIS_ADDR
check_env collaboration-service COLLABORATION_NATS_URL
check_env knowledge-graph-search-service AUTH_SERVICE_GRPC_ADDR
check_env knowledge-graph-search-service SEARCH_NATS_URL
check_env file-service FILE_MINIO_ENDPOINT
check_env file-service FILE_MINIO_PUBLIC_BASE_URL
check_env file-service FILE_MINIO_BUCKET

echo "Running end-to-end compose smoke probe"
docker run --rm \
  --network "${NETWORK}" \
  -e GATEWAY_BASE_URL="http://gateway:8080/api/v1" \
  -e SMOKE_MINIO_INTERNAL_BASE_URL="http://minio:9000" \
  -v "${ROOT_DIR}:/workspace" \
  -w /workspace \
  golang:1.23 \
  sh -lc "export PATH=/usr/local/go/bin:\$PATH && go run ./tests/compose/demo_smoke_probe.go"

echo "Compose smoke test passed"
