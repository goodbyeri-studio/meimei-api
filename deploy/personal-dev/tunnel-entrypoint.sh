#!/bin/sh
set -eu

for name in SSH_HOST SSH_USER SSH_REMOTE_PORT TUNNEL_LISTEN_PORT; do
  eval "value=\${$name:-}"
  if [ -z "$value" ]; then
    echo "[personal-dev-tunnel] $name is required" >&2
    exit 1
  fi
done

case "$SSH_REMOTE_PORT:$TUNNEL_LISTEN_PORT" in
  *[!0-9:]*|:*|*:) echo "[personal-dev-tunnel] invalid tunnel port" >&2; exit 1 ;;
esac

install -d -m 0700 /run/personal-dev
install -m 0600 /run/personal-dev-host/tunnel_ed25519 /run/personal-dev/tunnel_ed25519
install -m 0600 /run/personal-dev-host/known_hosts /run/personal-dev/known_hosts

export AUTOSSH_GATETIME=0
export AUTOSSH_POLL=10

exec autossh -M 0 -NT -g \
  -F /dev/null \
  -i /run/personal-dev/tunnel_ed25519 \
  -L "0.0.0.0:${TUNNEL_LISTEN_PORT}:127.0.0.1:${SSH_REMOTE_PORT}" \
  -o BatchMode=yes \
  -o ConnectTimeout=10 \
  -o ExitOnForwardFailure=yes \
  -o GlobalKnownHostsFile=/dev/null \
  -o IdentitiesOnly=yes \
  -o ServerAliveCountMax=3 \
  -o ServerAliveInterval=10 \
  -o StrictHostKeyChecking=yes \
  -o UserKnownHostsFile=/run/personal-dev/known_hosts \
  "${SSH_USER}@${SSH_HOST}"
