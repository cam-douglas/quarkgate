#!/usr/bin/env bash
# First-boot setup for Ubuntu 24.04 on the quarkgate DO droplet.
# Run as root on the VPS (via SSH or DO web console):
#   curl -fsSL https://raw.githubusercontent.com/.../remote-vps-bootstrap.sh | bash
# Or copy from repo after clone:
#   bash quarkgate/scripts/remote-vps-bootstrap.sh
set -eo pipefail

DEPLOY_USER="${DEPLOY_USER:-deploy}"
REPO_URL="${REPO_URL:-https://github.com/cam-douglas/quarkOS.git}"
REPO_DIR="${REPO_DIR:-/opt/quarkOS}"
GO_VERSION="${GO_VERSION:-1.23.4}"

if [ "$(id -u)" -ne 0 ]; then
  echo "Run as root (sudo bash $0)" >&2
  exit 1
fi

echo "==> Base packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq git curl ca-certificates ufw \
  build-essential python3 python3-pip python3-venv postgresql-client redis-tools
if ! command -v docker >/dev/null 2>&1; then
  curl -fsSL https://get.docker.com | sh
fi
systemctl enable --now docker
usermod -aG docker "$DEPLOY_USER" 2>/dev/null || true

echo "==> UFW (host firewall — DO cloud firewall also applies)"
ufw --force reset
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

echo "==> Go $GO_VERSION"
if ! command -v go >/dev/null 2>&1; then
  curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | tar -C /usr/local -xz
  echo 'export PATH=$PATH:/usr/local/go/bin' >/etc/profile.d/go.sh
  export PATH="$PATH:/usr/local/go/bin"
fi

echo "==> Clone quarkOS (skip if $REPO_DIR exists — rsync from deploy host may replace this)"
if [ ! -d "$REPO_DIR/.git" ]; then
  git clone --depth 1 "$REPO_URL" "$REPO_DIR" || mkdir -p "$REPO_DIR"
fi

echo "Bootstrap base packages complete. Deploy script rsyncs repo + installs systemd units next."
