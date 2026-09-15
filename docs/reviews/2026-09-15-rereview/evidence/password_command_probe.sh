#!/usr/bin/env bash
set -euo pipefail

: "${LAZYSLICE_REVIEW_BIN:?set LAZYSLICE_REVIEW_BIN to the pinned lazyslice binary}"

name="lazyslice-password-probe-$$"
scratch=$(mktemp -d /tmp/lazyslice-password-probe.XXXXXX)
docker run --rm -d --name "$name" -e POSTGRES_PASSWORD=synthetic_pw \
  -p 127.0.0.1::5432 postgres:18 >/dev/null
cleanup() { docker rm -f "$name" >/dev/null 2>&1 || true; }
trap cleanup EXIT

for _ in $(seq 1 100); do
  docker exec "$name" pg_isready -h 127.0.0.1 -U postgres >/dev/null 2>&1 && break
  sleep 0.2
done

port=$(docker port "$name" 5432/tcp | sed 's/.*://')
PGPASSWORD=synthetic_pw psql -h 127.0.0.1 -p "$port" -U postgres -d postgres \
  -c 'CREATE DATABASE source_probe' >/dev/null
PGPASSWORD=synthetic_pw psql -h 127.0.0.1 -p "$port" -U postgres -d postgres \
  -c 'CREATE DATABASE target_probe' >/dev/null
PGPASSWORD=synthetic_pw psql -h 127.0.0.1 -p "$port" -U postgres -d source_probe \
  -c 'CREATE TABLE items(id int primary key); INSERT INTO items VALUES(1)' >/dev/null

set +e
(
  cd "$scratch"
  LAZYSLICE_SECRET="$(printf '42%.0s' {1..32})" "$LAZYSLICE_REVIEW_BIN" \
    --source "postgres://postgres@127.0.0.1:${port}/source_probe?sslmode=disable" \
    --target "postgres://postgres:synthetic_pw@127.0.0.1:${port}/target_probe?sslmode=disable" \
    --root public.items --take 1 --yes --no-config \
    --password-command 'printf synthetic_pw'
) >"$scratch/run.log" 2>&1
code=$?
set -e

printf 'exit=%s\n' "$code"
rg -n 'password authentication|source|target' "$scratch/run.log" || true
printf 'evidence=%s\n' "$scratch"
