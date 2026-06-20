#!/usr/bin/env bash
# E2E test harness: builds thrive for linux, runs it inside ONE privileged
# Docker container, pulls representative OCI images once, and verifies the
# full container lifecycle. A single container is reused to avoid Docker Hub
# pull-rate-limit issues and speed up the test run.
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BINARY_PATH="/tmp/thrive-e2e-linux"
PASS=0
FAIL=0
FAILURES=()
CID=""  # ID of the persistent test container

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_ok()   { echo -e "${GREEN}[PASS]${NC} $*"; PASS=$((PASS+1)); }
log_fail() { echo -e "${RED}[FAIL]${NC} $*"; FAIL=$((FAIL+1)); FAILURES+=("$*"); }
log_info() { echo -e "${YELLOW}[INFO]${NC} $*"; }

# Attempt to retrieve Docker Hub credentials from the local credential store.
# This avoids anonymous pull rate limits (100/6h). Falls back to anonymous if
# no credential helper is available.
DOCKER_USER=""
DOCKER_PASS=""
if command -v docker-credential-desktop &>/dev/null; then
  _creds=$(echo "https://index.docker.io/v1/" | docker-credential-desktop get 2>/dev/null || true)
  if [ -n "$_creds" ]; then
    DOCKER_USER=$(echo "$_creds" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('Username',''))" 2>/dev/null || true)
    DOCKER_PASS=$(echo "$_creds" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('Secret',''))" 2>/dev/null || true)
  fi
fi
if [ -n "$DOCKER_USER" ] && [ -n "$DOCKER_PASS" ]; then
  log_info "Docker Hub credentials found (user: $DOCKER_USER) — authenticated pulls"
else
  log_info "No Docker Hub credentials found — using anonymous pulls (may hit rate limit)"
fi

cleanup() {
  if [ -n "$CID" ]; then
    log_info "Stopping test container $CID..."
    docker stop "$CID" >/dev/null 2>&1 || true
    docker rm   "$CID" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

# ── Build thrive binary ────────────────────────────────────────────────────
# Match the native architecture of the Docker host so images and the binary
# run natively inside the container (avoids QEMU emulation hangs).
HOST_ARCH=$(uname -m)
case "$HOST_ARCH" in
  arm64|aarch64) GOARCH=arm64 ;;
  *) GOARCH=amd64 ;;
esac
log_info "Building thrive for linux/$GOARCH..."
GOOS=linux GOARCH=$GOARCH CGO_ENABLED=0 go build \
  -ldflags "-X main.Version=e2e -X main.Commit=local" \
  -o "$BINARY_PATH" \
  "$REPO_ROOT/cmd/thrive"
log_info "Binary: $(ls -lh "$BINARY_PATH")"

# ── Build test harness image (has ca-certificates; built once) ─────────────
HARNESS_IMAGE="thrive-e2e-harness:latest"
log_info "Building test harness Docker image ($HARNESS_IMAGE)..."
docker build -q -t "$HARNESS_IMAGE" - <<'DOCKERFILE'
FROM ubuntu:22.04
RUN apt-get update -qq && \
    apt-get install -y -qq --no-install-recommends ca-certificates fuse3 && \
    rm -rf /var/lib/apt/lists/*
DOCKERFILE
log_info "Harness image ready."

# ── Start ONE persistent privileged container ──────────────────────────────
log_info "Starting persistent test container..."
CID=$(docker run -d --privileged \
  --cap-add ALL \
  --security-opt apparmor=unconfined \
  -v "$BINARY_PATH:/usr/local/bin/thrive:ro" \
  -e HOME=/root \
  -e _DOCKER_USER="$DOCKER_USER" \
  -e _DOCKER_PASS="$DOCKER_PASS" \
  "$HARNESS_IMAGE" \
  sleep 7200)
log_info "Container: $CID"

# ── Exec helper — runs commands in the shared container ────────────────────
exec_in() {
  docker exec "$CID" bash -c "$*" 2>&1 || true
}

# Write a pull wrapper script inside the container so we can use credentials
# without leaking them in every exec call.
exec_in 'cat > /usr/local/bin/thrive-pull <<'"'"'SH'"'"'
#!/bin/bash
AUTH_FLAGS=""
if [ -n "$_DOCKER_USER" ] && [ -n "$_DOCKER_PASS" ]; then
  AUTH_FLAGS="--username $_DOCKER_USER --password $_DOCKER_PASS"
fi
exec thrive pull $AUTH_FLAGS "$@"
SH
chmod +x /usr/local/bin/thrive-pull'

# Initialize thrive state directories once
exec_in "mkdir -p /var/lib/thrive/images /run/thrive/containers"

# ── Tests ──────────────────────────────────────────────────────────────────

test_version() {
  log_info "--- test: --version ---"
  out=$(exec_in "thrive --version")
  if echo "$out" | grep -q "e2e"; then
    log_ok "--version shows 'e2e'"
  else
    log_fail "--version: unexpected output: $out"
  fi
}

test_pull() {
  local image="$1"
  log_info "--- test: pull $image ---"
  out=$(exec_in "thrive-pull $image 2>&1; echo EXIT:\$?")
  if echo "$out" | grep -q "EXIT:0"; then
    log_ok "pull $image"
  else
    log_fail "pull $image: $out"
  fi
}

test_run() {
  local image="$1"
  local cmd="$2"
  local expected="$3"
  local label="${4:-$image: $cmd}"
  log_info "--- test: run $label ---"
  # Image is already cached — no re-download needed.
  out=$(exec_in "thrive run --rm $image $cmd 2>&1; echo EXIT:\$?")
  if echo "$out" | grep -q "EXIT:0" && echo "$out" | grep -q "$expected"; then
    log_ok "run $label → '$expected'"
  else
    log_fail "run $label: expected '$expected' in output; got: $out"
  fi
}

test_lifecycle() {
  log_info "--- test: full lifecycle (alpine:3.19) ---"
  out=$(exec_in '
    # Pull (should use cache from earlier, thrive-pull passes credentials if set)
    thrive-pull alpine:3.19 2>&1 >/dev/null
    echo "PULL_OK"

    # Images list
    thrive images 2>&1 | grep -q alpine && echo "IMAGES_OK"

    # Run and capture output
    result=$(thrive run --rm alpine:3.19 echo "lifecycle-test" 2>&1)
    echo "$result" | grep -q "lifecycle-test" && echo "RUN_OK"

    # Run detached
    thrive run --detach --name lc-test alpine:3.19 sleep 30 2>&1
    echo "DETACH_OK"

    # ps
    thrive ps 2>&1 | grep -q lc-test && echo "PS_OK"

    # kill
    thrive kill lc-test 2>&1
    echo "KILL_OK"

    # rm
    thrive rm lc-test 2>&1
    echo "RM_OK"

    # rmi
    thrive rmi alpine:3.19 2>&1
    echo "RMI_OK"

    echo ALL_DONE
  ')
  for marker in PULL_OK IMAGES_OK RUN_OK DETACH_OK PS_OK KILL_OK RM_OK RMI_OK ALL_DONE; do
    if echo "$out" | grep -q "$marker"; then
      log_ok "lifecycle: $marker"
    else
      log_fail "lifecycle: $marker missing; full output: $out"
    fi
  done
}

test_system_info() {
  log_info "--- test: system info ---"
  out=$(exec_in "thrive system info 2>&1; echo EXIT:\$?")
  if echo "$out" | grep -q "EXIT:0" && echo "$out" | grep -q "Version:"; then
    log_ok "system info"
  else
    log_fail "system info: $out"
  fi
}

test_system_clean() {
  log_info "--- test: system clean ---"
  out=$(exec_in "
    thrive run --rm busybox:1.36 echo done 2>&1
    thrive system clean 2>&1
    echo EXIT:\$?
  ")
  if echo "$out" | grep -q "EXIT:0"; then
    log_ok "system clean"
  else
    log_fail "system clean: $out"
  fi
}

test_volume() {
  log_info "--- test: volume bind mount (-v) ---"
  out=$(exec_in '
    mkdir -p /tmp/thrivetest-vol
    echo "volume-content-ok" > /tmp/thrivetest-vol/test.txt
    thrive run --rm -v /tmp/thrivetest-vol:/data ubuntu:22.04 cat /data/test.txt 2>&1
    echo EXIT:$?
  ')
  if echo "$out" | grep -q "volume-content-ok" && echo "$out" | grep -q "EXIT:0"; then
    log_ok "volume: bind mount (-v) works"
  else
    log_fail "volume: bind mount failed; output: $out"
  fi
}

test_exec_flags() {
  log_info "--- test: exec flag passthrough ---"
  exec_in "thrive run --detach --name exec-flag-test ubuntu:22.04 sleep 30 2>&1" >/dev/null
  out=$(exec_in "thrive exec exec-flag-test uname -a 2>&1")
  exec_in "thrive kill exec-flag-test 2>&1; thrive rm exec-flag-test 2>&1" >/dev/null
  if echo "$out" | grep -q "Linux"; then
    log_ok "exec: flag passthrough (uname -a)"
  else
    log_fail "exec: flag passthrough failed; output: $out"
  fi
}

test_interactive_stdin() {
  log_info "--- test: interactive stdin (-i) ---"
  out=$(exec_in "printf 'interactive-stdin-ok\n' | thrive run --rm -i ubuntu:22.04 cat 2>&1")
  if echo "$out" | grep -q "interactive-stdin-ok"; then
    log_ok "interactive: -i stdin pipe works"
  else
    log_fail "interactive: -i stdin pipe failed; output: $out"
  fi
}

# ── Image matrix ───────────────────────────────────────────────────────────
IMAGES=(
  "alpine:3.19"
  "ubuntu:22.04"
  "debian:12-slim"
  "busybox:1.36"
  "python:3.12-alpine"
  "node:20-alpine"
  "nginx:alpine"
  "redis:alpine"
  "golang:1.23-alpine"
)

# ── Run all tests ──────────────────────────────────────────────────────────
test_version

# Pull all images first (each image pulled exactly once)
for img in "${IMAGES[@]}"; do
  test_pull "$img"
done

# Run tests — images are cached, no re-downloading
test_run "alpine:3.19"        "echo hello-alpine"      "hello-alpine"
test_run "ubuntu:22.04"       "cat /etc/os-release"    "Ubuntu"
test_run "debian:12-slim"     "cat /etc/os-release"    "Debian"
test_run "busybox:1.36"       "echo busybox-ok"        "busybox-ok"
test_run "python:3.12-alpine" "python3 --version"      "Python 3.12"
test_run "node:20-alpine"     "node --version"         "v20"
test_run "nginx:alpine"       "nginx -v"               "nginx"
test_run "redis:alpine"       "redis-server --version" "Redis"
test_run "golang:1.23-alpine" "go version"             "go1.23"

# Lifecycle + system tests
test_lifecycle
test_system_info
test_system_clean

# Bug-fix regression tests
test_volume
test_exec_flags
test_interactive_stdin

# ── Summary ───────────────────────────────────────────────────────────────
echo ""
echo "═══════════════════════════════════════"
echo -e "  PASS: ${GREEN}$PASS${NC}   FAIL: ${RED}$FAIL${NC}"
echo "═══════════════════════════════════════"
if [ ${#FAILURES[@]} -gt 0 ]; then
  echo "Failed tests:"
  for f in "${FAILURES[@]}"; do
    echo -e "  ${RED}✗${NC} $f"
  done
  echo ""
fi

[ "$FAIL" -eq 0 ]
