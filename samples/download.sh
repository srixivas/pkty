#!/usr/bin/env bash
# Download sample pcap files into this directory.
# Run: bash samples/download.sh
#
# Files are gitignored — run this script to get them locally.

set -e
cd "$(dirname "$0")"

echo "📡 Downloading pkty sample captures..."

# --- Unit42 Wireshark tutorial pcaps (real malware traffic) ---
# 5 pcaps: Loki Bot, IcedID, Ave Maria RAT, infostealer, spambot
# Great for: DNS widget, TLS Inspector, Remote Hosts, NetGraph
if [ ! -f unit42-tutorial.zip ]; then
  echo "→ Unit42 malware traffic (9 MB, password: infected)..."
  wget -q --show-progress -O unit42-tutorial.zip \
    'https://raw.githubusercontent.com/PaloAltoNetworks/Unit42-Wireshark-tutorials/main/Wireshark-tutorial-filter-expressions-5-pcaps.zip'
  unzip -q -P infected unit42-tutorial.zip -d unit42/ 2>/dev/null || \
    unzip -P infected unit42-tutorial.zip -d unit42/
  echo "✓ Extracted to samples/unit42/"
else
  echo "✓ unit42-tutorial.zip already present"
fi

# --- The Ultimate PCAP (90+ protocols, no password) ---
# Synthetic but extremely broad protocol coverage
# Great for: Protocol Dist widget, hex dump deep-dives
if [ ! -f ultimate.7z ]; then
  echo "→ The Ultimate PCAP (5 MB, 90+ protocols)..."
  wget -q --show-progress -O ultimate.7z \
    'https://weberblog.net/wp-content/uploads/2020/02/The-Ultimate-PCAP.7z'
  if command -v 7z >/dev/null 2>&1; then
    7z x ultimate.7z -o ultimate/ -y > /dev/null
    echo "✓ Extracted to samples/ultimate/"
  else
    echo "⚠  7z not installed — extract manually: 7z x ultimate.7z"
    echo "   Install: sudo apt install p7zip-full  (Linux)"
  fi
else
  echo "✓ ultimate.7z already present"
fi

echo ""
echo "Usage:"
echo "  pkty -r samples/unit42/<file>.pcap"
echo "  pkty -r samples/ultimate/<file>.pcap"
