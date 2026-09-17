#!/usr/bin/env bash
#
# tools/egress/run.sh — THREAT_MODEL.md T4's egress test (tracker T-0270).
#
# T4's stated control is that the binary "opens exactly three kinds of
# connection — the source, the target, and the Docker socket" and has "no
# telemetry, update check, model call or registry". This script proves it for
# the network half: it cross-builds lazyslice for linux, starts a source and
# a target postgres:16 on a Docker network, and runs the binary inside a
# third container that holds NET_ADMIN and an iptables OUTPUT policy that
# accepts loopback traffic to 127.0.0.1, established traffic and the two
# database addresses on 5432 and counts-then-rejects everything else —
# including a DNS query to 127.0.0.11, the embedded resolver a container on
# a user-defined Docker network reaches over its own loopback interface, so
# a beaconing DNS lookup is not exempted by the loopback rule. A real run
# must leave that final rule's packet counter at zero — no DNS lookup, no
# telemetry, nothing else dialled. A negative control after the real run
# deliberately connects to a third address from the same container, so the
# test also proves its own counter counts: a rule set that never sees a
# rejected packet would pass this test for the wrong reason.
#
# What this does not prove: a connection to the Docker socket is a unix
# socket, not an IP connection an OUTPUT rule ever sees, so this test does
# not (and cannot) cover it; and it proves nothing about a subprocess, which
# is what internal/repo's and internal/discover's own allowlist tests cover.
# This run also never opens the Docker socket in the first place — --source
# and --target are both named, which short-circuits internal/discover's
# ladder before it makes any Docker call (internal/core/CLAUDE.md,
# "resolveEndpoints").
#
# Every resource this script creates carries a unique suffix and is removed
# on every exit path (the `cleanup` trap below); it never touches a
# container, network or image it did not create itself.
#
# Usage: tools/egress/run.sh (no arguments; run via `make egress`). Needs a
# Docker endpoint that can grant NET_ADMIN to a container — Docker Desktop on
# macOS and GitHub's ubuntu-latest runners both do.

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

SUFFIX="$(date +%s)-$$"
NET="lzs-egress-net-$SUFFIX"
SRC="lzs-egress-src-$SUFFIX"
TGT="lzs-egress-tgt-$SUFFIX"
RUNNER="lzs-egress-runner-$SUFFIX"
IMG="lzs-egress-img-$SUFFIX"

DB_USER="lazyslice"
DB_PASSWORD="lazyslice"
DB_NAME="lazyslice_egress"
PG_IMAGE="${LAZYSLICE_EGRESS_POSTGRES_IMAGE:-postgres:16}"

BUILD_DIR=""

log() { echo "==> egress: $*"; }
die() {
	echo "egress: FAIL: $*" >&2
	exit 1
}

# cleanup runs on every exit path (normal, `set -e`, or a signal) and removes
# only the resources this run created, named with $SUFFIX above. It must
# never fail the script itself — a container or network that was already
# gone (an earlier step never got that far) is not an error here.
cleanup() {
	local status=$?
	set +e
	docker rm -f "$RUNNER" "$SRC" "$TGT" >/dev/null 2>&1
	docker network rm "$NET" >/dev/null 2>&1
	docker image rm "$IMG" >/dev/null 2>&1
	[ -n "$BUILD_DIR" ] && rm -rf "$BUILD_DIR"
	if [ "$status" -eq 0 ]; then
		log "cleaned up (network $NET, containers $SRC/$TGT/$RUNNER, image $IMG)"
	else
		echo "egress: cleaned up after failure (network $NET, containers $SRC/$TGT/$RUNNER, image $IMG)" >&2
	fi
	exit "$status"
}
trap cleanup EXIT INT TERM

command -v docker >/dev/null 2>&1 || die "docker is not on PATH"
command -v go >/dev/null 2>&1 || die "go is not on PATH"
command -v python3 >/dev/null 2>&1 || die "python3 is not on PATH (used to generate the masking key)"
docker version >/dev/null 2>&1 || die "no reachable Docker daemon (docker version failed)"

BUILD_DIR="$(mktemp -d)"

# ---------- 1. cross-build the binary for linux on the host architecture ----------
HOST_ARCH="$(go env GOARCH)"
log "cross-building lazyslice for linux/$HOST_ARCH (CGO_ENABLED=0)"
(
	cd "$REPO_ROOT"
	CGO_ENABLED=0 GOOS=linux GOARCH="$HOST_ARCH" go build -o "$BUILD_DIR/lazyslice" ./cmd/lazyslice
) || die "cross-build failed"

# ---------- 2. a Docker network, a source and a target postgres:16 ----------
log "creating network $NET"
docker network create "$NET" >/dev/null || die "docker network create failed"

log "starting source ($SRC) and target ($TGT) on $PG_IMAGE"
docker run -d --name "$SRC" --network "$NET" \
	-e POSTGRES_USER="$DB_USER" -e POSTGRES_PASSWORD="$DB_PASSWORD" -e POSTGRES_DB="$DB_NAME" \
	"$PG_IMAGE" >/dev/null || die "starting the source container failed"
docker run -d --name "$TGT" --network "$NET" \
	-e POSTGRES_USER="$DB_USER" -e POSTGRES_PASSWORD="$DB_PASSWORD" -e POSTGRES_DB="$DB_NAME" \
	"$PG_IMAGE" >/dev/null || die "starting the target container failed"

wait_ready() {
	local container="$1" tries=90
	while [ "$tries" -gt 0 ]; do
		if docker exec "$container" pg_isready -U "$DB_USER" -d "$DB_NAME" -h 127.0.0.1 >/dev/null 2>&1; then
			return 0
		fi
		tries=$((tries - 1))
		sleep 2
	done
	return 1
}
log "waiting for the source to accept connections"
wait_ready "$SRC" || die "$SRC never became ready (pg_isready timed out)"
log "waiting for the target to accept connections"
wait_ready "$TGT" || die "$TGT never became ready (pg_isready timed out)"

# ---------- 3. load a small schema with invented personal data into the source ----------
#
# Nothing here is real: the names, emails and phone numbers are made up, and
# the phone numbers use the NANP 555 range reserved for fiction. The target
# starts empty — lazyslice creates its schema.
log "loading the fixture schema into the source"
docker exec -i "$SRC" psql -v ON_ERROR_STOP=1 -q -U "$DB_USER" -d "$DB_NAME" <"$SCRIPT_DIR/fixture.sql" >/dev/null ||
	die "loading the fixture schema into the source failed"

SRC_ROWS_CUSTOMERS="$(docker exec "$SRC" psql -tA -U "$DB_USER" -d "$DB_NAME" -c 'SELECT count(*) FROM public.customers')"
SRC_ROWS_ORDERS="$(docker exec "$SRC" psql -tA -U "$DB_USER" -d "$DB_NAME" -c 'SELECT count(*) FROM public.orders')"
SRC_EMAILS="$(docker exec "$SRC" psql -tA -U "$DB_USER" -d "$DB_NAME" -c 'SELECT email FROM public.customers ORDER BY id')"
[ "$SRC_ROWS_CUSTOMERS" -gt 0 ] || die "the fixture loaded no customers rows"
[ "$SRC_ROWS_ORDERS" -gt 0 ] || die "the fixture loaded no orders rows"

SRC_IP="$(docker inspect -f "{{ (index .NetworkSettings.Networks \"$NET\").IPAddress }}" "$SRC")"
TGT_IP="$(docker inspect -f "{{ (index .NetworkSettings.Networks \"$NET\").IPAddress }}" "$TGT")"
[ -n "$SRC_IP" ] || die "could not read the source container's address"
[ -n "$TGT_IP" ] || die "could not read the target container's address"
log "source $SRC_IP, target $TGT_IP"

# ---------- 4. build the runner image: alpine plus iptables, nothing else ----------
#
# Built at test time, not checked in, so the image the run inspects is
# exactly the three lines below: an alpine base, iptables installed, and a
# command that keeps the container alive between `docker exec` calls.
cat >"$BUILD_DIR/Dockerfile" <<'DOCKERFILE'
FROM alpine:3.20
RUN apk add --no-cache iptables
CMD ["sleep", "infinity"]
DOCKERFILE

log "building the runner image"
docker build -q -t "$IMG" -f "$BUILD_DIR/Dockerfile" "$BUILD_DIR" >/dev/null || die "building the runner image failed"

log "starting the runner ($RUNNER) with NET_ADMIN"
docker run -d --name "$RUNNER" --network "$NET" --cap-add=NET_ADMIN \
	-v "$BUILD_DIR/lazyslice:/usr/local/bin/lazyslice:ro" \
	"$IMG" >/dev/null || die "starting the runner container failed"

# ---------- 5. the OUTPUT policy: loopback-to-127.0.0.1, established, the two databases, then count-and-reject ----------
#
# Order matters: the first matching rule wins, so the catch-all REJECT must
# be last. Its own pkts/bytes counter (`iptables -L OUTPUT -v -n -x`) is the
# whole test — every packet this container tries to send that is not to
# 127.0.0.1 over loopback, not a reply on an already-open connection and not
# to the source or the target on 5432 is counted there before it is
# rejected. The loopback accept is scoped to 127.0.0.1 rather than the whole
# `lo` interface on purpose: on a user-defined Docker network, the
# container's resolver is 127.0.0.11 (Docker's embedded DNS) and is reached
# over `lo` too, so an unscoped `-o lo` accept would silently exempt a DNS
# beacon from the counter. Scoped this way, a query to 127.0.0.11 falls
# through every ACCEPT rule and is counted and rejected like any other
# address this run never named.
log "installing the OUTPUT policy inside the runner"
docker exec "$RUNNER" sh -c "
	iptables -A OUTPUT -o lo -d 127.0.0.1 -j ACCEPT &&
	iptables -A OUTPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT &&
	iptables -A OUTPUT -d $SRC_IP -p tcp --dport 5432 -j ACCEPT &&
	iptables -A OUTPUT -d $TGT_IP -p tcp --dport 5432 -j ACCEPT &&
	iptables -A OUTPUT -j REJECT
" || die "installing the iptables rules failed"

reject_count() {
	# The catch-all REJECT is always the last OUTPUT rule this script
	# installs, and it is the only rule with REJECT as its target, so match
	# on that column rather than assuming it is the chain's last printed
	# line (a warning on stdout or a future extra rule would otherwise be
	# read as the counter). Die rather than hand back a value that is not
	# actually a packet count.
	local line count
	line="$(docker exec "$RUNNER" iptables -L OUTPUT -v -n -x | awk '$3 == "REJECT" { print; exit }')"
	[ -n "$line" ] || die "could not find the catch-all REJECT rule in the runner's OUTPUT chain"
	count="$(awk '{print $1}' <<<"$line")"
	[[ "$count" =~ ^[0-9]+$ ]] || die "the REJECT rule's packet counter is not a number: '$count'"
	echo "$count"
}

BEFORE="$(reject_count)"
[ "$BEFORE" = "0" ] || die "the reject counter was already non-zero ($BEFORE) before lazyslice ran"

# ---------- 6. the real run: --yes --no-config --root customers, LAZYSLICE_SECRET set ----------
#
# Both endpoints are named as raw IP addresses, so no DNS lookup is a
# legitimate part of this run — none is in the allowed OUTPUT policy either.
# --allow-remote-target is needed only because the target is not loopback
# from the runner's point of view (a container's own address, not
# 127.0.0.1); it names that exact address and nothing wider.
SECRET="$(python3 -c 'import secrets; print(secrets.token_hex(32))')"
SRC_DSN="postgres://$DB_USER:$DB_PASSWORD@$SRC_IP:5432/$DB_NAME?sslmode=disable"
TGT_DSN="postgres://$DB_USER:$DB_PASSWORD@$TGT_IP:5432/$DB_NAME?sslmode=disable"

log "running lazyslice inside the runner"
set +e
docker exec -e LAZYSLICE_SECRET="$SECRET" "$RUNNER" /usr/local/bin/lazyslice \
	--source "$SRC_DSN" \
	--target "$TGT_DSN" \
	--root customers \
	--allow-remote-target "$TGT_IP" \
	--yes --no-config
RUN_EXIT=$?
set -e
[ "$RUN_EXIT" -eq 0 ] || die "lazyslice exited $RUN_EXIT, want 0"
log "lazyslice exited 0"

# ---------- 7. the target holds the rows, and the email column holds no source value ----------
TGT_ROWS_CUSTOMERS="$(docker exec "$TGT" psql -tA -U "$DB_USER" -d "$DB_NAME" -c 'SELECT count(*) FROM public.customers')"
TGT_ROWS_ORDERS="$(docker exec "$TGT" psql -tA -U "$DB_USER" -d "$DB_NAME" -c 'SELECT count(*) FROM public.orders')"
[ "$TGT_ROWS_CUSTOMERS" = "$SRC_ROWS_CUSTOMERS" ] ||
	die "target customers row count is $TGT_ROWS_CUSTOMERS, want $SRC_ROWS_CUSTOMERS"
[ "$TGT_ROWS_ORDERS" = "$SRC_ROWS_ORDERS" ] ||
	die "target orders row count is $TGT_ROWS_ORDERS, want $SRC_ROWS_ORDERS"
log "target holds $TGT_ROWS_CUSTOMERS customers and $TGT_ROWS_ORDERS orders rows"

TGT_NONEMPTY_EMAILS="$(docker exec "$TGT" psql -tA -U "$DB_USER" -d "$DB_NAME" -c "SELECT count(*) FROM public.customers WHERE email IS NOT NULL AND email <> ''")"
[ "$TGT_NONEMPTY_EMAILS" = "$SRC_ROWS_CUSTOMERS" ] ||
	die "target has $TGT_NONEMPTY_EMAILS non-empty email(s), want $SRC_ROWS_CUSTOMERS — masking must replace the value, not blank it"

TGT_EMAILS="$(docker exec "$TGT" psql -tA -U "$DB_USER" -d "$DB_NAME" -c 'SELECT email FROM public.customers ORDER BY id')"
COMPARED=0
while IFS= read -r email; do
	[ -n "$email" ] || continue
	COMPARED=$((COMPARED + 1))
	if grep -qxF "$email" <<<"$SRC_EMAILS"; then
		die "target email column holds a source value ($email)"
	fi
done <<<"$TGT_EMAILS"
[ "$COMPARED" -gt 0 ] || die "compared zero target emails against the source; the masking assertion did not actually run"
log "target email column holds no source value ($COMPARED compared)"

# ---------- 8. the counter is zero after the real run ----------
AFTER_RUN="$(reject_count)"
[ "$AFTER_RUN" = "0" ] ||
	die "the reject rule's packet counter reads $AFTER_RUN after the real run, want 0 — lazyslice dialled something outside the source and the target"
log "reject counter is 0 after the real run"

# ---------- 9. negative control: the counter itself counts ----------
#
# One deliberate connection attempt from the runner to a third address —
# the Docker network's own gateway, which is on-link but named in none of
# the ACCEPT rules above — must move the counter off zero. If it does not,
# the policy above would pass this test whether or not it actually rejects
# anything, which is the failure this step exists to catch.
GATEWAY_IP="$(docker network inspect "$NET" -f '{{ (index .IPAM.Config 0).Gateway }}')"
[ -n "$GATEWAY_IP" ] || die "could not read the network's gateway address for the negative control"
[ "$GATEWAY_IP" != "$SRC_IP" ] && [ "$GATEWAY_IP" != "$TGT_IP" ] ||
	die "the network gateway ($GATEWAY_IP) collided with a database address; cannot run the negative control"

log "negative control: connecting from the runner to $GATEWAY_IP:5432 (expected to be rejected)"
docker exec "$RUNNER" wget -T 2 -O /dev/null "http://$GATEWAY_IP:5432/" >/dev/null 2>&1 || true

AFTER_CONTROL="$(reject_count)"
[ "$AFTER_CONTROL" -gt 0 ] ||
	die "negative control: the reject counter still reads 0 after a deliberate connection attempt to $GATEWAY_IP — a rule set that counts nothing cannot pass this test"
log "negative control: reject counter is $AFTER_CONTROL after the deliberate attempt — the counter counts"

log "PASS: lazyslice reached only the source and the target; the reject counter proves it, and the counter itself is proven to count"
