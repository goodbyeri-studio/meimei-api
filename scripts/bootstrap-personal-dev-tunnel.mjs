#!/usr/bin/env node

import {
  chmodSync,
  existsSync,
  mkdirSync,
  readFileSync,
  writeFileSync,
} from "node:fs";
import { homedir, hostname } from "node:os";
import { resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { lookup } from "node:dns/promises";

const repoRoot = resolve(import.meta.dirname, "..");
const configPath = resolve(repoRoot, ".env.personal-dev");
const tunnelDir = resolve(repoRoot, ".personal-dev");
const privateKeyPath = resolve(tunnelDir, "tunnel_ed25519");
const publicKeyPath = `${privateKeyPath}.pub`;
const knownHostsPath = resolve(tunnelDir, "known_hosts");
const tunnelEnvPath = resolve(tunnelDir, "tunnel.env");

function fail(message) {
  console.error(`[personal-dev-tunnel] ${message}`);
  process.exit(1);
}

function readEnvFile(path) {
  if (!existsSync(path)) return {};
  const values = {};
  for (const raw of readFileSync(path, "utf8").split(/\r?\n/)) {
    const match = /^([A-Z0-9_]+)=(.*)$/.exec(raw.trim());
    if (match) values[match[1]] = match[2];
  }
  return values;
}

function expandHome(path) {
  if (path === "~") return homedir();
  if (path.startsWith("~/")) return resolve(homedir(), path.slice(2));
  return path;
}

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: repoRoot,
    encoding: "utf8",
    env: process.env,
    maxBuffer: 4 * 1024 * 1024,
    ...options,
  });
  if (result.error) fail(result.error.message);
  if (result.status !== 0) {
    if (result.stderr) process.stderr.write(result.stderr);
    fail(`${command} failed with exit code ${result.status ?? "unknown"}`);
  }
  return result.stdout.trim();
}

if (!existsSync(configPath)) {
  fail("run `make personal-dev-init` first");
}

const config = readEnvFile(configPath);
const sshHost = config.PERSONAL_DEV_SSH_HOST || "dev-fpsmeimei";
const sshUser = config.PERSONAL_DEV_USER || "deploy";
const adminKey = expandHome(
  config.PERSONAL_DEV_SSH_KEY || "~/.ssh/2049-personal-dev",
);
const remotePort = config.PERSONAL_DEV_API_PORT || "3100";
const localPort = config.PERSONAL_DEV_LOCAL_API_PORT || "3310";

if (!existsSync(adminKey)) fail(`admin SSH key not found: ${adminKey}`);
for (const port of [remotePort, localPort]) {
  if (!/^\d{2,5}$/.test(port) || Number(port) > 65535) {
    fail(`invalid tunnel port: ${port}`);
  }
}

const sshConfig = run("ssh", ["-G", sshHost]);
const effectiveHost = sshConfig
  .split(/\r?\n/)
  .find((line) => line.startsWith("hostname "))
  ?.slice("hostname ".length)
  .trim();
if (!effectiveHost || !/^[A-Za-z0-9.-]+$/.test(effectiveHost)) {
  fail("PERSONAL_DEV_SSH_HOST must resolve to a hostname usable by Docker");
}
const resolvedHosts = await lookup(effectiveHost, { all: true }).catch(() => []);
const tailscaleAddress = resolvedHosts.find(({ address, family }) => {
  if (family !== 4) return false;
  const [first, second] = address.split(".").map(Number);
  return first === 100 && second >= 64 && second <= 127;
})?.address;
const tunnelHost = tailscaleAddress || effectiveHost;

mkdirSync(tunnelDir, { recursive: true, mode: 0o700 });
chmodSync(tunnelDir, 0o700);
if (!existsSync(privateKeyPath)) {
  run("ssh-keygen", [
    "-q",
    "-t",
    "ed25519",
    "-N",
    "",
    "-C",
    `meimei-api-personal-tunnel-${hostname()}`,
    "-f",
    privateKeyPath,
  ]);
}
chmodSync(privateKeyPath, 0o600);
chmodSync(publicKeyPath, 0o644);

const adminSshArgs = [
  "-o",
  "BatchMode=yes",
  "-o",
  "StrictHostKeyChecking=yes",
  "-o",
  "ConnectTimeout=15",
  "-i",
  adminKey,
];
const target = `${sshUser}@${sshHost}`;
const hostKey = run("ssh", [
  ...adminSshArgs,
  target,
  "cat /etc/ssh/ssh_host_ed25519_key.pub",
]);
const hostKeyParts = hostKey.split(/\s+/);
if (hostKeyParts[0] !== "ssh-ed25519" || !hostKeyParts[1]) {
  fail("VPS did not return a valid Ed25519 SSH host key");
}
writeFileSync(
  knownHostsPath,
  `${tunnelHost} ${hostKeyParts[0]} ${hostKeyParts[1]}\n`,
  { mode: 0o600 },
);

const publicKey = readFileSync(publicKeyPath, "utf8").trim();
const publicKeyParts = publicKey.split(/\s+/);
if (publicKeyParts[0] !== "ssh-ed25519" || !publicKeyParts[1]) {
  fail("generated tunnel public key is invalid");
}
const authorizedEntry = `restrict,port-forwarding,permitopen="127.0.0.1:${remotePort}",command="/bin/false" ${publicKey}\n`;
const encodedEntry = Buffer.from(authorizedEntry).toString("base64");
run(
  "ssh",
  [...adminSshArgs, target, "sh", "-s"],
  {
    input: `set -eu
umask 077
mkdir -p "$HOME/.ssh"
touch "$HOME/.ssh/authorized_keys"
if ! grep -Fq '${publicKeyParts[1]}' "$HOME/.ssh/authorized_keys"; then
  printf '%s' '${encodedEntry}' | base64 -d >> "$HOME/.ssh/authorized_keys"
fi
chmod 0700 "$HOME/.ssh"
chmod 0600 "$HOME/.ssh/authorized_keys"
`,
  },
);

writeFileSync(
  tunnelEnvPath,
  [
    `SSH_HOST=${tunnelHost}`,
    `SSH_USER=${sshUser}`,
    `SSH_REMOTE_PORT=${remotePort}`,
    `TUNNEL_LISTEN_PORT=${localPort}`,
    "",
  ].join("\n"),
  { mode: 0o600 },
);
chmodSync(knownHostsPath, 0o600);
chmodSync(tunnelEnvPath, 0o600);

const fingerprint = run("ssh-keygen", ["-lf", knownHostsPath]);
console.log(`[personal-dev-tunnel] tunnel-only key installed for ${tunnelHost}`);
console.log(`[personal-dev-tunnel] pinned VPS host key: ${fingerprint}`);
