#!/bin/sh
set -eu

ready_file=/tmp/veltrix-bootstrap-ready
rm -f "$ready_file"

run_postgres() {
  # The function is backgrounded below. exec keeps postgres_pid tied to the
  # official entrypoint/final postgres process instead of an intermediate
  # shell whose termination would leave PostgreSQL running.
  exec env \
    -u DATABASE_URL \
    -u DATABASE_ADMIN_URL \
    -u DATABASE_DISPATCHER_URL \
    -u APP_DB_PASSWORD \
    -u DEMO_SEED \
    -u DEMO_EMAIL \
    -u DEMO_PASSWORD \
    -u SESSION_COOKIE_SECURE \
    -u IDENTITY_ENCRYPTION_KEY_ID \
    -u IDENTITY_ENCRYPTION_KEY_BASE64 \
    /usr/local/bin/docker-entrypoint.sh "$@"
}

run_postgres "$@" &
postgres_pid=$!

stop_postgres() {
  # PostgreSQL SIGINT is its documented fast shutdown: new connections are
  # rejected, active transactions are rolled back, and all server processes
  # exit cleanly. SIGTERM (smart shutdown) can wait indefinitely while the
  # already-running app keeps pooled sessions open during an image upgrade.
  kill -INT "$postgres_pid" 2>/dev/null || true
  wait "$postgres_pid" 2>/dev/null || true
}
trap stop_postgres INT TERM

# The official image briefly starts a Unix-socket-only temporary server while
# creating the requested database. Wait for the final TCP listener so the CRM
# bootstrap cannot race that initialization phase.
until pg_isready -h 127.0.0.1 -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-veltrix_crm}" >/dev/null 2>&1; do
  if ! kill -0 "$postgres_pid" 2>/dev/null; then
    wait "$postgres_pid"
    exit $?
  fi
  sleep 1
done

if ! gosu postgres /app/veltrix-crm bootstrap; then
  stop_postgres
  exit 1
fi

# Replace the privileged bootstrap wrapper with the final PostgreSQL process.
# The restart is graceful and happens before readiness; the steady PID 1 does
# not retain finite bootstrap credentials in its environment.
stop_postgres
touch "$ready_file"
trap - INT TERM
exec env \
  -u DATABASE_URL \
  -u DATABASE_ADMIN_URL \
  -u DATABASE_DISPATCHER_URL \
  -u APP_DB_PASSWORD \
  -u DEMO_SEED \
  -u DEMO_EMAIL \
  -u DEMO_PASSWORD \
  -u SESSION_COOKIE_SECURE \
  -u IDENTITY_ENCRYPTION_KEY_ID \
  -u IDENTITY_ENCRYPTION_KEY_BASE64 \
  /usr/local/bin/docker-entrypoint.sh "$@"
