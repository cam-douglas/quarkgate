#!/usr/bin/env bash
# Deploy quarkgate to the DO VPS from a machine with SSH access.
# Usage:
#   export DO_DROPLET_IP=168.144.168.198
#   export SSH_KEY=quarkgate/.deploy/quarkgate-vps-deploy   # or your own key
#   bash quarkgate/scripts/deploy-to-vps.sh
set -eo pipefail
cd "$(dirname "$0")/.."
ROOT="$(cd .. && pwd)"

set +u
set -a
# shellcheck disable=SC1091
source ./.env
set +a
set -u

HOST="${DO_DROPLET_IP:?Set DO_DROPLET_IP in quarkgate/.env}"
SSH_USER="${DO_SSH_USER:-root}"
SSH_KEY="${SSH_KEY:-${QUARKGATE_SSH_KEY:-$HOME/.ssh/quarkgate-ssh}}"
# Fallback to passphrase-free deploy key for automation
if [ ! -f "$SSH_KEY" ] || ! ssh-keygen -y -f "$SSH_KEY" >/dev/null 2>&1; then
  SSH_KEY="./.deploy/quarkgate-vps-deploy"
fi
SSH_OPTS=(-o BatchMode=yes -o StrictHostKeyChecking=accept-new -i "$SSH_KEY")

if [ ! -f "$SSH_KEY" ]; then
  echo "SSH key not found: $SSH_KEY" >&2
  echo "Generate with: ssh-keygen -t ed25519 -f $SSH_KEY -N ''" >&2
  exit 1
fi

echo "==> Testing SSH to ${SSH_USER}@${HOST}"
if ! ssh "${SSH_OPTS[@]}" "${SSH_USER}@${HOST}" 'echo ok' 2>/dev/null; then
  cat <<EOF >&2
SSH failed. The droplet has no authorized key for $SSH_KEY yet.

Add the deploy public key via DigitalOcean web console:
  1. https://cloud.digitalocean.com/droplets → quarkgate → Access → Launch Droplet Console
  2. Run:
     mkdir -p ~/.ssh && chmod 700 ~/.ssh
     echo '$(cat "${SSH_KEY}.pub")' >> ~/.ssh/authorized_keys
     chmod 600 ~/.ssh/authorized_keys

Or add key id 57465753 (quarkgate-cursor-deploy) and rebuild droplet with SSH keys selected.

Then re-run: bash quarkgate/scripts/deploy-to-vps.sh
EOF
  exit 1
fi

echo "==> Running remote bootstrap"
scp "${SSH_OPTS[@]}" scripts/remote-vps-bootstrap.sh "${SSH_USER}@${HOST}:/tmp/"
ssh "${SSH_OPTS[@]}" "${SSH_USER}@${HOST}" 'bash /tmp/remote-vps-bootstrap.sh'

echo "==> Syncing repository (tar over SSH)"
tar czf - \
  --exclude='.git' \
  --exclude='node_modules' \
  --exclude='**/.venv' \
  --exclude='**/__pycache__' \
  --exclude='quarkgate/.deploy' \
  --exclude='quarkgate/vault' \
  --exclude='quarkgate/data' \
  --exclude='.cursor' \
  --exclude='**/uploads' \
  --exclude='**/.DS_Store' \
  --exclude='**/._*' \
  --exclude='quarkgate/tools/repos' \
  -C "$ROOT" . | ssh "${SSH_OPTS[@]}" "${SSH_USER}@${HOST}" 'mkdir -p /opt/quarkOS && tar xzf - -C /opt/quarkOS'

echo "==> Syncing production env (local .env → VPS)"
scp "${SSH_OPTS[@]}" "$ROOT/.env" "${SSH_USER}@${HOST}:/opt/quarkOS/.env"
scp "${SSH_OPTS[@]}" ./.env "${SSH_USER}@${HOST}:/opt/quarkOS/quarkgate/.env"

# Production-oriented overrides on VPS
ssh "${SSH_OPTS[@]}" "${SSH_USER}@${HOST}" bash -s <<REMOTE
set -eo pipefail
for f in /opt/quarkOS/.env /opt/quarkOS/quarkgate/.env; do
  grep -q '^LISTEN_ADDR=' "\$f" 2>/dev/null && sed -i 's/^LISTEN_ADDR=.*/LISTEN_ADDR=:8080/' "\$f" || echo 'LISTEN_ADDR=:8080' >> "\$f"
  grep -q '^QUARKGATE_URL=' /opt/quarkOS/quarkgate/.env && sed -i 's|^QUARKGATE_URL=.*|QUARKGATE_URL=http://127.0.0.1:8080|' /opt/quarkOS/quarkgate/.env
  grep -q '^REDIS_URL=' /opt/quarkOS/.env && sed -i 's|^REDIS_URL=.*|REDIS_URL=redis://redis:6379|' /opt/quarkOS/.env || echo 'REDIS_URL=redis://redis:6379' >> /opt/quarkOS/.env
done
REMOTE

echo "==> Install systemd units"
ssh "${SSH_OPTS[@]}" "${SSH_USER}@${HOST}" bash -s <<'REMOTE'
set -eo pipefail
install -d /etc/quarkgate
install -m 644 /opt/quarkOS/quarkgate/infrastructure/systemd/quarkgate-gateway.service /etc/systemd/system/
install -m 644 /opt/quarkOS/quarkgate/infrastructure/systemd/quarkgate-ledger-worker.service /etc/systemd/system/
systemctl daemon-reload
REMOTE

echo "==> Build QuarkGate + start compose + systemd"
ssh "${SSH_OPTS[@]}" "${SSH_USER}@${HOST}" bash -s <<'REMOTE'
set -eo pipefail
cd /opt/quarkOS
export PATH="$PATH:/usr/local/go/bin"
make install-production
make prod-up
cd quarkgate
bash scripts/bootstrap-vault-from-env.sh || true
go build -o bin/gateway ./cmd/gateway
go build -o bin/ledger-worker ./cmd/ledger-worker
systemctl enable --now quarkgate-gateway quarkgate-ledger-worker
cd /opt/quarkOS && make prod-smoke || true
REMOTE

echo "Deploy script finished. Verify: curl -sf http://${HOST}:8080/healthz"
