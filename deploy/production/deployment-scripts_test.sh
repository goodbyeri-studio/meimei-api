#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
TEST_ROOT=$(mktemp -d)

cleanup() {
  rm -rf -- "$TEST_ROOT"
}
trap cleanup EXIT

fail() {
  printf '[deployment-scripts-test] ERROR: %s\n' "$*" >&2
  exit 1
}

BASE_ENV="$TEST_ROOT/production.env"
SECRET_ROOT="$TEST_ROOT/secrets/wechatpay"
APP_ROOT="$TEST_ROOT/app"
FAKE_BIN="$TEST_ROOT/bin"
PRIVATE_KEY="$TEST_ROOT/private.pem"
PUBLIC_KEY="$TEST_ROOT/public.pem"

install -m 700 -d "$FAKE_BIN" "$APP_ROOT"
printf 'DATABASE_URL=test\nWECHAT_PAY_APP_ID=legacy\n' > "$BASE_ENV"
printf 'services: {}\n' > "$APP_ROOT/docker-compose.yml"
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$PRIVATE_KEY" >/dev/null 2>&1
openssl pkey -in "$PRIVATE_KEY" -pubout -out "$PUBLIC_KEY" >/dev/null 2>&1

make_bundle() {
  local release_id="$1"
  local merchant_id="$2"
  local bundle="$TEST_ROOT/bundle-$release_id"
  install -m 700 -d "$bundle"
  install -m 600 "$PRIVATE_KEY" "$bundle/merchant-private-key.pem"
  install -m 600 "$PUBLIC_KEY" "$bundle/wechatpay-public-key.pem"
  {
    printf 'WECHAT_PAY_APP_ID=wx-test\n'
    printf 'WECHAT_PAY_MCH_ID=%s\n' "$merchant_id"
    printf 'WECHAT_PAY_MERCHANT_SERIAL_NO=ABC123\n'
    printf 'WECHAT_PAY_MERCHANT_PRIVATE_KEY_PATH=/run/secrets/wechatpay/merchant-private-key.pem\n'
    printf 'WECHAT_PAY_PUBLIC_KEY_ID=PUB_KEY_ID_TEST\n'
    printf 'WECHAT_PAY_PUBLIC_KEY_PATH=/run/secrets/wechatpay/wechatpay-public-key.pem\n'
    printf 'WECHAT_PAY_API_V3_KEY=12345678901234567890123456789012\n'
    printf 'WECHAT_PAY_NOTIFY_URL=https://example.com/api/wechat/notify\n'
  } > "$bundle/wechat-pay.env"
  printf '%s\n' "$bundle"
}

install_release() {
  local release_id="$1"
  local merchant_id="$2"
  local bundle
  bundle=$(make_bundle "$release_id" "$merchant_id")
  WECHAT_PAY_ENV_FILE="$BASE_ENV" WECHAT_PAY_SECRET_DIR="$SECRET_ROOT" \
    "$SCRIPT_DIR/install-wechat-pay.sh" "$bundle" "$release_id"
  [[ ! -e "$bundle" ]] || fail "bundle was not removed: $release_id"
}

install_release release-one 10001
grep -qx 'DATABASE_URL=test' "$SECRET_ROOT/releases/release-one/production.env"
grep -qx 'WECHAT_PAY_MCH_ID=10001' "$SECRET_ROOT/releases/release-one/production.env"
[[ -s "$SECRET_ROOT/releases/release-one/merchant-private-key.pem" ]]
first_hash=$(sha256sum "$SECRET_ROOT/releases/release-one/production.env" | cut -d' ' -f1)

invalid_bundle=$(make_bundle broken-release 19999)
printf 'invalid public key\n' > "$invalid_bundle/wechatpay-public-key.pem"
if WECHAT_PAY_ENV_FILE="$BASE_ENV" WECHAT_PAY_SECRET_DIR="$SECRET_ROOT" \
  "$SCRIPT_DIR/install-wechat-pay.sh" "$invalid_bundle" broken-release; then
  fail 'invalid release unexpectedly installed'
fi
[[ ! -e "$SECRET_ROOT/releases/broken-release" ]]
[[ "$first_hash" == "$(sha256sum "$SECRET_ROOT/releases/release-one/production.env" | cut -d' ' -f1)" ]]

install_release release-two 10002
install_release release-three 10003

cat > "$FAKE_BIN/docker" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat > "$FAKE_BIN/curl" <<'EOF'
#!/usr/bin/env bash
if [[ "${FAKE_CURL_MODE:-success}" == 'fail-once' ]]; then
  count=0
  [[ ! -f "$FAKE_CURL_STATE" ]] || count=$(cat "$FAKE_CURL_STATE")
  count=$((count + 1))
  printf '%s\n' "$count" > "$FAKE_CURL_STATE"
  (( count > 1 )) || exit 1
fi
exit 0
EOF
chmod 700 "$FAKE_BIN/docker" "$FAKE_BIN/curl"

deploy_script() {
  PATH="$FAKE_BIN:$PATH" \
    APP_ROOT="$APP_ROOT" \
    WECHAT_PAY_SECRET_DIR="$SECRET_ROOT" \
    MEIMEI_API_BASE_ENV_FILE="$BASE_ENV" \
    MEIMEI_API_DATA_DIR="$TEST_ROOT/data" \
    MEIMEI_API_LOG_DIR="$TEST_ROOT/log" \
    READY_ATTEMPTS=1 READY_DELAY_SECONDS=0 \
    "$SCRIPT_DIR/deploy.sh" "$@"
}

image_one="registry.digitalocean.com/meimei-api/meimei-api@sha256:$(printf 'a%.0s' {1..64})"
image_two="registry.digitalocean.com/meimei-api/meimei-api@sha256:$(printf 'b%.0s' {1..64})"
image_three="registry.digitalocean.com/meimei-api/meimei-api@sha256:$(printf 'c%.0s' {1..64})"
sha_one=$(printf 'a%.0s' {1..40})
sha_two=$(printf 'b%.0s' {1..40})
sha_three=$(printf 'c%.0s' {1..40})

deploy_script deploy "$image_one" "$sha_one" release-one
deploy_script deploy "$image_two" "$sha_two" release-two
grep -qx "WECHAT_PAY_SECRET_RELEASE_DIR=$SECRET_ROOT/releases/release-two" "$APP_ROOT/.deploy/current.env"
grep -qx "WECHAT_PAY_SECRET_RELEASE_DIR=$SECRET_ROOT/releases/release-one" "$APP_ROOT/.deploy/previous.env"

deploy_script rollback
grep -qx "WECHAT_PAY_SECRET_RELEASE_DIR=$SECRET_ROOT/releases/release-one" "$APP_ROOT/.deploy/current.env"
grep -qx "WECHAT_PAY_SECRET_RELEASE_DIR=$SECRET_ROOT/releases/release-two" "$APP_ROOT/.deploy/previous.env"

export FAKE_CURL_MODE=fail-once
export FAKE_CURL_STATE="$TEST_ROOT/curl-count"
if deploy_script deploy "$image_three" "$sha_three" release-three; then
  fail 'failed readiness unexpectedly succeeded'
fi
grep -qx "WECHAT_PAY_SECRET_RELEASE_DIR=$SECRET_ROOT/releases/release-one" "$APP_ROOT/.deploy/current.env"
grep -qx "MEIMEI_API_ENV_FILE=$SECRET_ROOT/releases/release-one/production.env" "$APP_ROOT/.deploy/current.env"

printf '[deployment-scripts-test] all checks passed\n'
