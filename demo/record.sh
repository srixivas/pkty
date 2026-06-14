#!/usr/bin/env bash
# Generate demo/demo.gif using VHS.
# Tested on Ubuntu 22.04+. Run from the repo root or the demo/ directory.
#
#   bash demo/record.sh
#
set -euo pipefail

cd "$(dirname "$0")/.."

# ── dependencies ────────────────────────────────────────────────────────────

need_apt=()
command -v ffmpeg  &>/dev/null || need_apt+=(ffmpeg)
dpkg -s libpcap-dev &>/dev/null 2>&1  || need_apt+=(libpcap-dev)

if [[ ${#need_apt[@]} -gt 0 ]]; then
  echo "Installing: ${need_apt[*]}"
  sudo apt-get update -qq
  sudo apt-get install -y -qq "${need_apt[@]}"
fi

if ! command -v vhs &>/dev/null; then
  echo "Installing VHS..."
  sudo mkdir -p /etc/apt/keyrings
  curl -fsSL https://repo.charm.sh/apt/gpg.key \
    | sudo gpg --dearmor -o /etc/apt/keyrings/charm.gpg
  echo "deb [signed-by=/etc/apt/keyrings/charm.gpg] https://repo.charm.sh/apt/ * *" \
    | sudo tee /etc/apt/sources.list.d/charm.list >/dev/null
  sudo apt-get update -qq
  sudo apt-get install -y -qq vhs
fi

# ── build ────────────────────────────────────────────────────────────────────

echo "Building pkty..."
go build -o pkty ./cmd/pkty

# ── generate demo pcap ───────────────────────────────────────────────────────

echo "Generating demo/demo.pcap..."
go run demo/gen_pcap.go

# ── record ───────────────────────────────────────────────────────────────────

echo "Recording demo/demo.gif..."
vhs demo/demo.tape

echo ""
echo "Done → demo/demo.gif"
