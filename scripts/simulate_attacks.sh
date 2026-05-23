#!/usr/bin/env bash
# simulate_attacks.sh — fires SQLi, XSS, and flood attacks at the gateway
# and writes results to metrics_report.json beside this script.
# Requires: bash, curl. Run with Git Bash or WSL on Windows.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GATEWAY="http://localhost:8080"
BLOCKED=0
TOTAL=0

echo ""
echo "╔══════════════════════════════════════╗"
echo "║   Secure API Gateway Attack Sim      ║"
echo "╚══════════════════════════════════════╝"
echo ""

# ── Get a valid JWT ──────────────────────────────────────────────────────────
echo "[*] Obtaining JWT token..."
TOKEN_RESP=$(curl -s -X POST "$GATEWAY/auth/token" \
  -H "Content-Type: application/json" \
  -d '{"client_id":"attacker-sim","secret":"simulated"}')
TOKEN=$(echo "$TOKEN_RESP" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

if [ -z "$TOKEN" ]; then
  echo "[!] Could not get a token — is the gateway running on $GATEWAY?"
  exit 1
fi
echo "    Token obtained ✓"
echo ""

# ── Helper: fire a GET request and check status ───────────────────────────────
fire() {
  local LABEL="$1"
  local URL="$2"
  TOTAL=$((TOTAL + 1))
  STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer $TOKEN" "$URL")
  if [ "$STATUS" = "400" ] || [ "$STATUS" = "403" ] || [ "$STATUS" = "429" ]; then
    BLOCKED=$((BLOCKED + 1))
    echo "  ✓ BLOCKED [$STATUS] $LABEL"
  else
    echo "  ✗ PASSED  [$STATUS] $LABEL"
  fi
}

# ── SQLi payloads ─────────────────────────────────────────────────────────────
echo "[*] SQL Injection attacks..."
fire "OR 1=1"           "$GATEWAY/api/scada/sensor-data?id=$(python3 -c "import urllib.parse; print(urllib.parse.quote(\"' OR 1=1 --\"))" 2>/dev/null || echo "%27+OR+1%3D1+--")"
fire "UNION SELECT"     "$GATEWAY/api/scada/sensor-data?id=1+UNION+SELECT+*+FROM+users"
fire "DROP TABLE"       "$GATEWAY/api/scada/status?q=1%3B+DROP+TABLE+users%3B+--"
fire "sleep()"         "$GATEWAY/api/scada/sensor-data?id=1%3Bsleep(5)--"
fire "information_schema" "$GATEWAY/api/scada/status?q=SELECT+*+FROM+information_schema.tables"
echo ""

# ── XSS payloads ──────────────────────────────────────────────────────────────
echo "[*] XSS attacks..."
fire "<script>"         "$GATEWAY/api/scada/sensor-data?data=%3Cscript%3Ealert(1)%3C%2Fscript%3E"
fire "javascript:"      "$GATEWAY/api/scada/sensor-data?url=javascript%3Aalert(document.cookie)"
fire "onerror="         "$GATEWAY/api/scada/status?q=%3Cimg+src%3Dx+onerror%3Dalert(1)%3E"
fire "<iframe>"         "$GATEWAY/api/scada/sensor-data?data=%3Ciframe+src%3Djavascript%3Aalert(1)%3E"
fire "eval()"           "$GATEWAY/api/scada/status?q=eval(atob('YWxlcnQoMSk='))"
echo ""

# ── Rate limit flood ──────────────────────────────────────────────────────────
echo "[*] Rate limit flood on /auth/token (limit: 5 req/60s)..."
for i in $(seq 1 8); do
  TOTAL=$((TOTAL + 1))
  STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "$GATEWAY/auth/token" \
    -H "Content-Type: application/json" \
    -d "{\"client_id\":\"flood-test-$i\",\"secret\":\"flood\"}")
  if [ "$STATUS" = "429" ]; then
    BLOCKED=$((BLOCKED + 1))
    echo "  ✓ RATE LIMITED [429] request #$i"
  else
    echo "  - Allowed [$STATUS] request #$i"
  fi
done
echo ""

# ── Results ───────────────────────────────────────────────────────────────────
DETECTION_RATE=0
if [ "$TOTAL" -gt 0 ]; then
  DETECTION_RATE=$(echo "scale=2; $BLOCKED * 100 / $TOTAL" | bc)
fi

echo "╔══════════════════════════════════════╗"
echo "║             Results                  ║"
printf "║  Requests sent:  %-20s║\n" "$TOTAL"
printf "║  Blocked:        %-20s║\n" "$BLOCKED"
printf "║  Detection rate: %-19s%%║\n" "$DETECTION_RATE"
echo "╚══════════════════════════════════════╝"

# Write JSON report
cat > "$SCRIPT_DIR/metrics_report.json" <<EOF
{
  "requests_sent": $TOTAL,
  "blocked_count": $BLOCKED,
  "detection_rate_pct": $DETECTION_RATE,
  "avg_latency_ms": null,
  "generated_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
}
EOF

echo ""
echo "[✓] Report saved to $SCRIPT_DIR/metrics_report.json"

# Update README.md resume metrics section
README="$SCRIPT_DIR/../README.md"
if [ -f "$README" ] && grep -q "METRICS_START" "$README"; then
  echo "[*] Updating README.md resume metrics..."
  METRICS_BLOCK="<!-- METRICS_START -->\n| Metric | Value |\n|---|---|\n| Requests Sent | $TOTAL |\n| Blocked Count | $BLOCKED |\n| Detection Rate | ${DETECTION_RATE}% |\n<!-- METRICS_END -->"
  perl -i -0pe "s|<!-- METRICS_START -->.*?<!-- METRICS_END -->|$METRICS_BLOCK|s" "$README"
  echo "[✓] README.md updated"
fi
