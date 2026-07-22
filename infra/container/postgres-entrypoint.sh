#!/bin/sh
set -eu

ready_file=/tmp/veltrix-bootstrap-ready
rm -f "$ready_file"

/usr/local/bin/docker-entrypoint.sh "$@" &
postgres_pid=$!

stop_postgres() {
  kill -TERM "$postgres_pid" 2>/dev/null || true
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

touch "$ready_file"
wait "$postgres_pid"
