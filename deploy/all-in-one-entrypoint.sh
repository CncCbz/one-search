#!/bin/sh
set -eu

log() {
  printf '%s\n' "$*"
}

escape_sql_literal() {
  printf "%s" "$1" | sed "s/'/''/g"
}

escape_sql_ident() {
  printf '"%s"' "$(printf "%s" "$1" | sed 's/"/""/g')"
}

normalize_container_proxy_env() {
  normalize_one_proxy HTTP_PROXY
  normalize_one_proxy HTTPS_PROXY
  normalize_one_proxy ALL_PROXY
  normalize_one_proxy http_proxy
  normalize_one_proxy https_proxy
  normalize_one_proxy all_proxy
}

normalize_one_proxy() {
  name="$1"
  value=$(eval "printf '%s' \"\${$name:-}\"")
  if [ -z "$value" ]; then
    return
  fi
  value=$(printf '%s' "$value" | sed 's#//127\.0\.0\.1:#//host.docker.internal:#; s#//localhost:#//host.docker.internal:#')
  eval "export $name=\"$value\""
}

psql_admin() {
  su-exec postgres psql -U "$POSTGRES_USER" -d postgres -v ON_ERROR_STOP=1 "$@"
}

psql_admin_scalar() {
  su-exec postgres psql -U "$POSTGRES_USER" -d postgres -tAc "$1" | tr -d '[:space:]'
}

cleanup() {
  trap - INT TERM EXIT
  for pid in ${nginx_pid:-} ${backend_pid:-} ${postgres_pid:-}; do
    if [ -n "${pid:-}" ] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
    fi
  done
}

wait_for_backend() {
  attempts=0
  while ! curl --noproxy '*' -fsS http://127.0.0.1:8080/healthz >/dev/null 2>&1; do
    if ! kill -0 "$backend_pid" 2>/dev/null; then
      log "backend exited before becoming healthy"
      exit 1
    fi
    attempts=$((attempts + 1))
    if [ "$attempts" -ge 120 ]; then
      log "backend did not become healthy in time"
      exit 1
    fi
    sleep 1
  done
}

start_postgres() {
  log "starting postgres"
  su-exec postgres postgres \
    -D "$PGDATA" \
    -p 5432 \
    -c listen_addresses=127.0.0.1 \
    -c logging_collector=off \
    -c log_destination=stderr \
    -c client_min_messages=warning \
    >/proc/1/fd/1 2>&1 &
  postgres_pid=$!

  until pg_isready -U "$POSTGRES_USER" >/dev/null 2>&1; do
    if ! kill -0 "$postgres_pid" 2>/dev/null; then
      log "postgres exited during startup"
      exit 1
    fi
    sleep 1
  done
  log "postgres is ready"
}

ensure_database() {
  app_db_lit=$(escape_sql_literal "$POSTGRES_DB")
  app_pass_lit=$(escape_sql_literal "$POSTGRES_PASSWORD")
  app_db_ident=$(escape_sql_ident "$POSTGRES_DB")
  app_user_ident=$(escape_sql_ident "$POSTGRES_USER")

  log "ensuring database and role"
  psql_admin -c "ALTER ROLE $app_user_ident WITH LOGIN PASSWORD '$app_pass_lit';"

  if [ "$(psql_admin_scalar "SELECT 1 FROM pg_database WHERE datname = '$app_db_lit'")" != "1" ]; then
    psql_admin -c "CREATE DATABASE $app_db_ident OWNER $app_user_ident;"
  else
    psql_admin -c "ALTER DATABASE $app_db_ident OWNER TO $app_user_ident;"
  fi
}

start_backend() {
  export APP_ENV="${APP_ENV:-production}"
  export HTTP_ADDR="${HTTP_ADDR:-:8080}"
  export MIGRATIONS_DIR="${MIGRATIONS_DIR:-/app/backend/migrations}"
  export RUN_MIGRATIONS="${RUN_MIGRATIONS:-true}"
  export ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
  export ADMIN_PASSWORD="${ADMIN_PASSWORD:-}"
  : "${ENCRYPTION_KEY:?ENCRYPTION_KEY is required and must be at least 32 characters}"
  export ENCRYPTION_KEY
  export API_AUTH_REQUIRED="${API_AUTH_REQUIRED:-true}"
  export MCP_ENABLED="${MCP_ENABLED:-false}"
  export MCP_PATH="${MCP_PATH:-/mcp}"
  export CORS_ALLOWED_ORIGINS="${CORS_ALLOWED_ORIGINS:-http://localhost:5173,http://localhost:8080}"
  export UPSTREAM_USER_AGENT="${UPSTREAM_USER_AGENT:-OneSearchRelay/0.1}"
  export REQUEST_TIMEOUT_MS="${REQUEST_TIMEOUT_MS:-20000}"
  export DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@127.0.0.1:5432/${POSTGRES_DB}?sslmode=disable"

  log "starting backend on ${HTTP_ADDR}"
  /usr/local/bin/one-search &
  backend_pid=$!

  wait_for_backend
  log "backend is healthy"
}

start_nginx() {
  log "starting nginx"
  nginx -g 'daemon off;' &
  nginx_pid=$!
}

main() {
  : "${PGDATA:=/var/lib/postgresql/data}"
  : "${POSTGRES_DB:=one_search}"
  : "${POSTGRES_USER:=one_search}"
  : "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}"

  trap 'cleanup; exit 0' INT TERM
  trap cleanup EXIT

  mkdir -p "$PGDATA" /run/postgresql
  chown -R postgres:postgres "$PGDATA" /run/postgresql

  if [ ! -s "$PGDATA/PG_VERSION" ]; then
    log "initializing postgres data directory"
    pwfile=$(mktemp)
    printf '%s\n' "$POSTGRES_PASSWORD" > "$pwfile"
    chown postgres:postgres "$pwfile"
    su-exec postgres initdb \
      -D "$PGDATA" \
      --username="$POSTGRES_USER" \
      --pwfile="$pwfile" \
      --auth-local=trust \
      --auth-host=scram-sha-256 \
      >/proc/1/fd/1 2>&1
    rm -f "$pwfile"
  fi

  normalize_container_proxy_env

  start_postgres
  ensure_database
  start_backend
  start_nginx

  log "all-in-one stack is ready"

  while true; do
    if ! kill -0 "$postgres_pid" 2>/dev/null; then
      log "postgres stopped unexpectedly"
      exit 1
    fi
    if ! kill -0 "$backend_pid" 2>/dev/null; then
      log "backend stopped unexpectedly"
      exit 1
    fi
    if ! kill -0 "$nginx_pid" 2>/dev/null; then
      log "nginx stopped unexpectedly"
      exit 1
    fi
    sleep 5
  done
}

main "$@"
