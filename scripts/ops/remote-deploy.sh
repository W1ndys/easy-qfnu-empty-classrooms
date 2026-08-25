#!/usr/bin/env bash

set -euo pipefail

export COMPOSE_PROFILES=production

deadline=$((SECONDS + 90))
until docker compose exec -T postgres sh -c 'pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"' >/dev/null 2>&1; do
  if [ $SECONDS -ge $deadline ]; then
    printf '[remote] ERROR: PostgreSQL did not become ready\n' >&2
    docker compose logs --tail=100 postgres >&2 || true
    exit 1
  fi
  sleep 2
done

legacy_db="data/stats.db"
migration_marker="data/.sqlite-to-postgresql-migrated"
if [ -f "$legacy_db" ] && [ ! -f "$migration_marker" ]; then
  printf '[remote] backing up legacy SQLite files\n'
  cp -a "$legacy_db" "${legacy_db}.pre-postgresql"
  if [ -f "${legacy_db}-wal" ]; then
    cp -a "${legacy_db}-wal" "${legacy_db}-wal.pre-postgresql"
  fi
  if [ -f "${legacy_db}-shm" ]; then
    cp -a "${legacy_db}-shm" "${legacy_db}-shm.pre-postgresql"
  fi

  printf '[remote] migrating legacy SQLite data to PostgreSQL\n'
  docker compose run --rm \
    -v "$PWD/data:/legacy:ro" \
    backend \
    /app/migrate-sqlite-to-postgres \
    -sqlite /legacy/stats.db
  date -u '+%Y-%m-%dT%H:%M:%SZ' > "$migration_marker"
  printf '[remote] legacy migration completed\n'
else
  printf '[remote] no pending legacy SQLite migration\n'
fi

docker compose up -d
