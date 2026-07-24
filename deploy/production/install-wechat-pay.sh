#!/usr/bin/env bash
set -Eeuo pipefail

BUNDLE_DIR="${1:-}"
RELEASE_ID="${2:-}"
APP_ROOT="${APP_ROOT:-/opt/meimei-api}"
BASE_ENV_FILE="${WECHAT_PAY_ENV_FILE:-/etc/meimei-api/production.env}"
SECRET_ROOT="${WECHAT_PAY_SECRET_DIR:-$APP_ROOT/.secrets/wechatpay}"
RELEASES_DIR="$SECRET_ROOT/releases"
RELEASE_DIR="$RELEASES_DIR/$RELEASE_ID"
STAGING_DIR=''

fail() {
  printf '[wechat-pay-config] ERROR: %s\n' "$*" >&2
  exit 1
}

if [[ "${EUID:-$(id -u)}" -ne 0 && "$SECRET_ROOT" == /etc/* ]]; then
  exec sudo -n -- "$0" "$@"
fi

[[ -n "$BUNDLE_DIR" && -d "$BUNDLE_DIR" ]] || fail 'configuration bundle is missing'
[[ "$RELEASE_ID" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$ ]] || fail 'release ID is invalid'
[[ -f "$BASE_ENV_FILE" ]] || fail "production environment file is missing: $BASE_ENV_FILE"

PAYMENT_ENV="$BUNDLE_DIR/wechat-pay.env"
MERCHANT_KEY="$BUNDLE_DIR/merchant-private-key.pem"
PUBLIC_KEY="$BUNDLE_DIR/wechatpay-public-key.pem"

cleanup() {
  [[ -z "$STAGING_DIR" ]] || rm -rf -- "$STAGING_DIR"
  rm -f -- "$PAYMENT_ENV" "$MERCHANT_KEY" "$PUBLIC_KEY"
  rmdir -- "$BUNDLE_DIR" 2>/dev/null || true
}
trap cleanup EXIT

[[ -s "$PAYMENT_ENV" ]] || fail 'WeChat Pay environment bundle is empty'
[[ -s "$MERCHANT_KEY" ]] || fail 'merchant private key is empty'
[[ -s "$PUBLIC_KEY" ]] || fail 'WeChat Pay public key is empty'
grep -q '^WECHAT_PAY_API_V3_KEY=.' "$PAYMENT_ENV" || fail 'API v3 key is missing'
grep -qx 'WECHAT_PAY_MERCHANT_PRIVATE_KEY_PATH=/run/secrets/wechatpay/merchant-private-key.pem' "$PAYMENT_ENV" || fail 'merchant private key path is invalid'
grep -qx 'WECHAT_PAY_PUBLIC_KEY_PATH=/run/secrets/wechatpay/wechatpay-public-key.pem' "$PAYMENT_ENV" || fail 'WeChat Pay public key path is invalid'
openssl pkey -in "$MERCHANT_KEY" -check -noout >/dev/null || fail 'merchant private key is invalid'
openssl pkey -pubin -in "$PUBLIC_KEY" -noout >/dev/null || fail 'WeChat Pay public key is invalid'

install -m 700 -d "$SECRET_ROOT" "$RELEASES_DIR"
[[ ! -e "$RELEASE_DIR" ]] || fail "configuration release already exists: $RELEASE_ID"
STAGING_DIR=$(mktemp -d "$RELEASES_DIR/.staging.$RELEASE_ID.XXXXXX")
chmod 700 "$STAGING_DIR"
install -m 600 "$MERCHANT_KEY" "$STAGING_DIR/merchant-private-key.pem"
install -m 600 "$PUBLIC_KEY" "$STAGING_DIR/wechatpay-public-key.pem"

awk '!/^WECHAT_PAY_/' "$BASE_ENV_FILE" > "$STAGING_DIR/production.env"
cat "$PAYMENT_ENV" >> "$STAGING_DIR/production.env"
if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
  chmod --reference="$BASE_ENV_FILE" "$STAGING_DIR/production.env"
  chown --reference="$BASE_ENV_FILE" "$STAGING_DIR/production.env"
else
  chmod 600 "$STAGING_DIR/production.env"
fi

# The rename is the only commit point. Until it succeeds, no deployment can
# reference this release and the currently running configuration is untouched.
mv -- "$STAGING_DIR" "$RELEASE_DIR"
STAGING_DIR=''

printf '[wechat-pay-config] configuration release installed: %s\n' "$RELEASE_ID"
