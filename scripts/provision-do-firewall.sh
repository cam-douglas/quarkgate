#!/usr/bin/env bash
# Create (or update) DigitalOcean cloud firewall for the quarkgate droplet.
# Requires DIGITALOCEAN_TOKEN, DO_DROPLET_ID in quarkgate/.env
set -eo pipefail
cd "$(dirname "$0")/.."
set +u
set -a
# shellcheck disable=SC1091
source ./.env
set +a
set -u

: "${DIGITALOCEAN_TOKEN:?Set DIGITALOCEAN_TOKEN in .env}"
: "${DO_DROPLET_ID:?Set DO_DROPLET_ID in .env}"

FW_NAME="${DO_FIREWALL_NAME:-quarkgate-prod}"
DROPLET_ID="$DO_DROPLET_ID"

export FW_NAME DROPLET_ID
payload="$(python3 <<PY
import json, os
rules = {
  "name": os.environ["FW_NAME"],
  "droplet_ids": [int(os.environ["DROPLET_ID"])],
  "inbound_rules": [
    {"protocol": "tcp", "ports": "22", "sources": {"addresses": ["0.0.0.0/0", "::/0"]}},
    {"protocol": "tcp", "ports": "80", "sources": {"addresses": ["0.0.0.0/0", "::/0"]}},
    {"protocol": "tcp", "ports": "443", "sources": {"addresses": ["0.0.0.0/0", "::/0"]}},
  ],
  "outbound_rules": [
    {"protocol": "tcp", "ports": "1-65535", "destinations": {"addresses": ["0.0.0.0/0", "::/0"]}},
    {"protocol": "udp", "ports": "1-65535", "destinations": {"addresses": ["0.0.0.0/0", "::/0"]}},
    {"protocol": "icmp", "destinations": {"addresses": ["0.0.0.0/0", "::/0"]}},
  ],
}
print(json.dumps(rules))
PY
)"

existing="$(curl -sf -H "Authorization: Bearer $DIGITALOCEAN_TOKEN" \
  "https://api.digitalocean.com/v2/firewalls" | python3 -c "
import json,sys,os
name=os.environ.get('FW_NAME','quarkgate-prod')
for f in json.load(sys.stdin).get('firewalls',[]):
    if f.get('name')==name:
        print(f['id'])
        break
" FW_NAME="$FW_NAME")"

if [ -n "$existing" ]; then
  echo "Firewall $FW_NAME already exists (id=$existing)"
  curl -sf -X PUT -H "Authorization: Bearer $DIGITALOCEAN_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$payload" \
    "https://api.digitalocean.com/v2/firewalls/$existing" >/dev/null
  echo "Updated firewall id=$existing for droplet $DROPLET_ID"
else
  resp="$(curl -sf -X POST -H "Authorization: Bearer $DIGITALOCEAN_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$payload" \
    "https://api.digitalocean.com/v2/firewalls")"
  id="$(echo "$resp" | python3 -c "import json,sys; print(json.load(sys.stdin)['firewall']['id'])")"
  echo "Created firewall $FW_NAME id=$id for droplet $DROPLET_ID"
fi
