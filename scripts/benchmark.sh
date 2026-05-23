#!/usr/bin/env bash
# benchmark.sh — measures gateway throughput and avg latency.
# Writes results to metrics_report.json beside this script.
# Requires: bash, curl. Run with Git Bash or WSL on Windows.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GATEWAY="http://localhost:8080"
REQUESTS=200
ERRORS=0
TOTAL_LATENCY_MS=0

echo ""
echo "╔══════════════════════════════════════╗"
echo "║   Secure API Gateway Benchmark       ║"
echo "╚══════════════════════════════════════╝"
echo ""

# ── Get a valid JWT ──────────────────────────────────────────────────────────
echo "[*] Obtaining JWT token..."
TOKEN_RESP=$(curl -s -X POST "$GATEWAY/auth/token" \
  -H "Content-Type: application/json" \
  -d '{"client_id":"bench-client","secret":"bench"}')
TOKEN=$(echo "$TOKEN_RESP" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

if [ -z "$TOKEN" ]; then
  echo "[!] Could not get a token — is the gateway running on $GATEWAY?"
  exit 1
fi
echo "    Token obtained ✓"
echo "[*] Firing $REQUESTS requests against /api/scada/sensor-data..."
echo ""

# ── Benchmark loop ────────────────────────────────────────────────────────────
START_EPOCH=$(date +%s%3N)   # milliseconds since epoch

for i in $(seq 1 $REQUESTS); do
  REQ_START=$(date +%s%3N)
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer $TOKEN" \
    "$GATEWAY/api/scada/sensor-data")
  REQ_END=$(date +%s%3N)

  LATENCY=$((REQ_END - REQ_START))
  TOTAL_LATENCY_MS=$((TOTAL_LATENCY_MS + LATENCY))

  if [ "$HTTP_CODE" != "200" ] && [ "$HTTP_CODE" != "502" ]; then
    ERRORS=$((ERRORS + 1))
  fi

  # Progress bar every 50 requests
  if [ $((i % 50)) -eq 0 ]; then
    echo "    ... $i / $REQUESTS done"
  fi
done

END_EPOCH=$(date +%s%3N)
TOTAL_TIME_MS=$((END_EPOCH - START_EPOCH))
TOTAL_TIME_S=$(echo "scale=2; $TOTAL_TIME_MS / 1000" | bc)
AVG_LATENCY=$(echo "scale=2; $TOTAL_LATENCY_MS / $REQUESTS" | bc)
RPS=$(echo "scale=2; $REQUESTS * 1000 / $TOTAL_TIME_MS" | bc)

echo ""
echo "╔══════════════════════════════════════╗"
echo "║             Results                  ║"
printf "║  Requests:       %-20s║\n" "$REQUESTS"
printf "║  Total time:     %-18ss║\n" "$TOTAL_TIME_S"
printf "║  Avg latency:    %-18sms║\n" "$AVG_LATENCY"
printf "║  Throughput:     %-16sreq/s║\n" "$RPS"
printf "║  Errors:         %-20s║\n" "$ERRORS"
echo "╚══════════════════════════════════════╝"

# Write/merge JSON report (preserves attack sim fields if present)
REPORT_PATH="$SCRIPT_DIR/metrics_report.json"
GENERATED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

if [ -f "$REPORT_PATH" ]; then
  # Merge — preserve existing requests_sent / blocked_count from attack sim
  EXISTING_SENT=$(grep -o '"requests_sent":[^,}]*' "$REPORT_PATH" | grep -o '[0-9]*' || echo 0)
  EXISTING_BLOCKED=$(grep -o '"blocked_count":[^,}]*' "$REPORT_PATH" | grep -o '[0-9]*' || echo 0)
  EXISTING_RATE=$(grep -o '"detection_rate_pct":[^,}]*' "$REPORT_PATH" | grep -o '[0-9.]*' || echo 0)
else
  EXISTING_SENT=0; EXISTING_BLOCKED=0; EXISTING_RATE=0
fi

cat > "$REPORT_PATH" <<EOF
{
  "requests_sent": $EXISTING_SENT,
  "blocked_count": $EXISTING_BLOCKED,
  "detection_rate_pct": $EXISTING_RATE,
  "avg_latency_ms": $AVG_LATENCY,
  "requests_per_second": $RPS,
  "total_time_seconds": $TOTAL_TIME_S,
  "errors": $ERRORS,
  "generated_at": "$GENERATED_AT"
}
EOF

echo ""
echo "[✓] Report saved to $REPORT_PATH"

# Update README.md if markers are present
README="$SCRIPT_DIR/../README.md"
if [ -f "$README" ] && grep -q "METRICS_START" "$README"; then
  echo "[*] Updating README.md resume metrics..."
  METRICS_BLOCK="<!-- METRICS_START -->\n| Metric | Value |\n|---|---|\n| Requests Sent | $EXISTING_SENT |\n| Blocked Count | $EXISTING_BLOCKED |\n| Detection Rate | ${EXISTING_RATE}% |\n| Avg Latency | ${AVG_LATENCY}ms |\n| Throughput | ${RPS} req/s |\n<!-- METRICS_END -->"
  perl -i -0pe "s|<!-- METRICS_START -->.*?<!-- METRICS_END -->|$METRICS_BLOCK|s" "$README"
  echo "[✓] README.md updated"
fi
