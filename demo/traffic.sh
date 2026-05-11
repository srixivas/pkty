#!/usr/bin/env bash
# Generate a spread of live traffic for manual pkty testing / screenshots.
# Run this in a second terminal while pkty is capturing on your interface.
#
# Usage: bash demo/traffic.sh

set -e

ROUNDS=${1:-5}

echo "Generating traffic — ${ROUNDS} rounds. Ctrl+C to stop."

for i in $(seq 1 "$ROUNDS"); do
  echo "Round $i / $ROUNDS"

  # HTTPS to a spread of real hosts (DNS + TLS + bandwidth)
  curl -sf https://github.com        -o /dev/null &
  curl -sf https://api.github.com    -o /dev/null &
  curl -sf https://google.com        -o /dev/null &
  curl -sf https://cloudflare.com    -o /dev/null &
  curl -sf https://slack.com         -o /dev/null &
  curl -sf https://fastly.com        -o /dev/null &

  # HTTP (plaintext) — shows as HTTP in Proto Dist widget
  curl -sf http://neverssl.com       -o /dev/null &
  curl -sf http://example.com        -o /dev/null &

  wait

  # DNS-only burst — fills the DNS Queries widget fast
  for host in github.com google.com cloudflare.com api.github.com \
              fastly.net slack.com 1.1.1.1 8.8.8.8; do
    dig "$host" A +short > /dev/null 2>&1 &
  done
  wait

  # Larger download — makes the bandwidth sparkline spike
  curl -sf https://github.com/srixivas/pkty/archive/refs/heads/main.zip \
       -o /dev/null &

  wait
  sleep 2
done

echo "Done."
