#!/usr/bin/env bash
set -Eeuo pipefail

APP_ROOT="${APP_ROOT:-/opt/meimei-api}"
COMPOSE_FILE="$APP_ROOT/docker-compose.yml"
STATE_DIR="$APP_ROOT/.deploy"
CURRENT_RELEASE="$STATE_DIR/current.env"
PREVIOUS_RELEASE="$STATE_DIR/previous.env"
CANDIDATE_RELEASE="$STATE_DIR/candidate.env"
LOCK_FILE="$STATE_DIR/deploy.lock"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:3000/healthz/ready}"
WECHAT_PAY_SECRET_ROOT="${WECHAT_PAY_SECRET_DIR:-/etc/meimei-api/secrets/wechatpay}"
BASE_ENV_FILE="${MEIMEI_API_BASE_ENV_FILE:-/etc/meimei-api/production.env}"
DATA_DIR="${MEIMEI_API_DATA_DIR:-/var/lib/meimei-api/data}"
LOG_DIR="${MEIMEI_API_LOG_DIR:-/var/log/meimei-api}"
READY_ATTEMPTS="${READY_ATTEMPTS:-30}"
READY_DELAY_SECONDS="${READY_DELAY_SECONDS:-3}"

log() {
  printf '[meimei-api-deploy] %s\n' "$*"
}

fail() {
  printf '[meimei-api-deploy] ERROR: %s\n' "$*" >&2
  exit 1
}

require_image_ref() {
  local image_ref="$1"
  [[ "$image_ref" =~ ^registry\.digitalocean\.com/meimei-api/meimei-api@sha256:[a-f0-9]{64}$ ]] || \
    fail "image reference must use the MeiMei API registry and a sha256 digest"
}

require_sha() {
  local commit_sha="$1"
  [[ "$commit_sha" =~ ^[a-f0-9]{40}$ ]] || fail "commit SHA must be a 40-character hexadecimal value"
}

require_layout() {
  [[ -f "$COMPOSE_FILE" ]] || fail "missing production compose file: $COMPOSE_FILE"
  [[ -f "$BASE_ENV_FILE" ]] || fail "missing $BASE_ENV_FILE"
  [[ "$WECHAT_PAY_SECRET_ROOT" =~ ^/[A-Za-z0-9._/-]+$ && "$WECHAT_PAY_SECRET_ROOT" != *'..'* ]] || \
    fail 'WeChat Pay secret root must be an absolute path without parent traversal'
  mkdir -p "$STATE_DIR" "$DATA_DIR" "$LOG_DIR"
}

wait_for_ready() {
  local attempts=0
  until curl --fail --silent --show-error --max-time 5 "$HEALTH_URL" >/dev/null; do
    attempts=$((attempts + 1))
    if (( attempts >= READY_ATTEMPTS )); then
      return 1
    fi
    sleep "$READY_DELAY_SECONDS"
  done
}

compose() {
  local release_file="$1"
  shift
  docker compose --project-name meimei-api --env-file "$release_file" --file "$COMPOSE_FILE" "$@"
}

write_release() {
  local release_file="$1"
  local image_ref="$2"
  local commit_sha="$3"
  local payment_release="${4:-}"
  local temporary_file="$release_file.tmp"
  umask 077
  {
    printf 'MEIMEI_API_IMAGE=%s\nDEPLOYED_SHA=%s\n' "$image_ref" "$commit_sha"
    if [[ -n "$payment_release" ]]; then
      local payment_dir="$WECHAT_PAY_SECRET_ROOT/releases/$payment_release"
      [[ "$payment_release" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$ ]] || fail 'payment release ID is invalid'
      [[ -s "$payment_dir/production.env" ]] || fail "payment environment release is missing: $payment_release"
      [[ -s "$payment_dir/merchant-private-key.pem" ]] || fail "merchant private key release is missing: $payment_release"
      [[ -s "$payment_dir/wechatpay-public-key.pem" ]] || fail "WeChat Pay public key release is missing: $payment_release"
      printf 'MEIMEI_API_ENV_FILE=%s/production.env\n' "$payment_dir"
      printf 'WECHAT_PAY_SECRET_RELEASE_DIR=%s\n' "$payment_dir"
    fi
  } > "$temporary_file"
  mv "$temporary_file" "$release_file"
}

deploy() {
  local image_ref="$1"
  local commit_sha="$2"
  local payment_release="$3"
  require_image_ref "$image_ref"
  require_sha "$commit_sha"
  require_layout

  write_release "$CANDIDATE_RELEASE" "$image_ref" "$commit_sha" "$payment_release"
  log "pulling $image_ref"
  compose "$CANDIDATE_RELEASE" pull

  if [[ -f "$CURRENT_RELEASE" ]]; then
    cp "$CURRENT_RELEASE" "$PREVIOUS_RELEASE"
  fi
  mv "$CANDIDATE_RELEASE" "$CURRENT_RELEASE"
  log "starting commit $commit_sha"
  if compose "$CURRENT_RELEASE" up -d --remove-orphans --force-recreate && wait_for_ready; then
    log "deployment ready: $commit_sha"
    return 0
  fi

  log "readiness failed; attempting rollback"
  if [[ -f "$PREVIOUS_RELEASE" ]]; then
    cp "$PREVIOUS_RELEASE" "$CURRENT_RELEASE"
    compose "$CURRENT_RELEASE" up -d --remove-orphans --force-recreate
    wait_for_ready || fail "deployment and automatic rollback both failed"
    fail "deployment failed; previous release restored"
  fi

  compose "$CURRENT_RELEASE" down
  rm -f "$CURRENT_RELEASE"
  fail "initial deployment failed; no previous release was available"
}

rollback() {
  require_layout
  [[ -f "$CURRENT_RELEASE" ]] || fail "no current release recorded"
  [[ -f "$PREVIOUS_RELEASE" ]] || fail "no previous release recorded"

  local temporary_file="$STATE_DIR/.rollback.env.tmp"
  cp "$CURRENT_RELEASE" "$temporary_file"
  cp "$PREVIOUS_RELEASE" "$CURRENT_RELEASE"
  mv "$temporary_file" "$PREVIOUS_RELEASE"
  if ! compose "$CURRENT_RELEASE" pull || \
    ! compose "$CURRENT_RELEASE" up -d --remove-orphans --force-recreate || \
    ! wait_for_ready; then
    cp "$CURRENT_RELEASE" "$temporary_file"
    cp "$PREVIOUS_RELEASE" "$CURRENT_RELEASE"
    mv "$temporary_file" "$PREVIOUS_RELEASE"
    compose "$CURRENT_RELEASE" up -d --remove-orphans --force-recreate
    wait_for_ready || fail "rollback and rollback recovery both failed"
    fail "rollback failed; previous current release restored"
  fi

  log "rollback ready: $(sed -n 's/^DEPLOYED_SHA=//p' "$CURRENT_RELEASE")"
}

main() {
  local operation="${1:-}"
  mkdir -p "$STATE_DIR"
  rm -f "$CANDIDATE_RELEASE" "$CANDIDATE_RELEASE.tmp"
  exec 9>"$LOCK_FILE"
  flock -n 9 || fail "another deployment is already running"

  case "$operation" in
    deploy)
      [[ $# -eq 4 ]] || fail "usage: $0 deploy <image-digest> <commit-sha> <payment-release>"
      deploy "$2" "$3" "$4"
      ;;
    rollback)
      [[ $# -eq 1 ]] || fail "usage: $0 rollback"
      rollback
      ;;
    *)
      fail "usage: $0 {deploy <image-digest> <commit-sha> <payment-release>|rollback}"
      ;;
  esac
}

main "$@"
