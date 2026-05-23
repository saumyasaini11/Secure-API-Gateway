# simulate_attacks.ps1 — fires SQLi, XSS, and flood attacks at the gateway.
# Writes results to metrics_report.json in the same directory as this script.
# Runs natively on Windows PowerShell — no extra tools required.

$ErrorActionPreference = 'SilentlyContinue'
$SCRIPT_DIR  = $PSScriptRoot
$GATEWAY_URL = "http://localhost:8080"
$BLOCKED     = 0
$TOTAL       = 0

Write-Host ""
Write-Host "======================================"
Write-Host "   Secure API Gateway Attack Sim      "
Write-Host "======================================"
Write-Host ""

# -- Obtain a JWT token --------------------------------------------------------
Write-Host "[*] Obtaining JWT token..." -ForegroundColor Gray
try {
    $tokenResp = Invoke-RestMethod -Uri "$GATEWAY_URL/auth/token" `
        -Method POST `
        -ContentType "application/json" `
        -Body '{"client_id":"attacker-sim","secret":"simulated"}'
    $TOKEN = $tokenResp.token
} catch {
    Write-Host "[!] Could not reach gateway at $GATEWAY_URL" -ForegroundColor Red
    exit 1
}
if (-not $TOKEN) {
    Write-Host "[!] Gateway returned no token. Is it running?" -ForegroundColor Red
    exit 1
}
Write-Host "    Token obtained [OK]" -ForegroundColor Green
Write-Host ""

# -- Helper: fire one GET and check the response code -------------------------
function Fire-Request($Label, $Url) {
    $script:TOTAL++
    try {
        $resp = Invoke-WebRequest -Uri $Url `
            -Headers @{ Authorization = "Bearer $TOKEN" } `
            -UseBasicParsing `
            -ErrorAction Stop
        $code = $resp.StatusCode
    } catch [System.Net.WebException] {
        $code = [int]$_.Exception.Response.StatusCode
    } catch {
        $code = 0
    }
    if ($code -in @(400, 403, 429)) {
        $script:BLOCKED++
        Write-Host "  [BLOCKED $code] $Label" -ForegroundColor Green
    } else {
        Write-Host "  [PASSED  $code] $Label" -ForegroundColor Yellow
    }
}

# -- SQL Injection -------------------------------------------------------------
Write-Host "[*] SQL Injection attacks..." -ForegroundColor Gray
Fire-Request "OR 1=1"              "$GATEWAY_URL/api/scada/sensor-data?id=%27+OR+1%3D1+--"
Fire-Request "UNION SELECT"        "$GATEWAY_URL/api/scada/sensor-data?id=1+UNION+SELECT+*+FROM+users"
Fire-Request "DROP TABLE"          "$GATEWAY_URL/api/scada/status?q=1%3B+DROP+TABLE+users%3B+--"
Fire-Request "sleep()"             "$GATEWAY_URL/api/scada/sensor-data?id=1%3Bsleep(5)--"
Fire-Request "information_schema"  "$GATEWAY_URL/api/scada/status?q=SELECT+*+FROM+information_schema.tables"
Write-Host ""

# -- XSS ----------------------------------------------------------------------
Write-Host "[*] XSS attacks..." -ForegroundColor Gray
Fire-Request "script tag"  "$GATEWAY_URL/api/scada/sensor-data?data=%3Cscript%3Ealert(1)%3C%2Fscript%3E"
Fire-Request "javascript:" "$GATEWAY_URL/api/scada/sensor-data?url=javascript%3Aalert(document.cookie)"
Fire-Request "onerror="    "$GATEWAY_URL/api/scada/status?q=%3Cimg+src%3Dx+onerror%3Dalert(1)%3E"
Fire-Request "iframe src"  "$GATEWAY_URL/api/scada/sensor-data?data=%3Ciframe+src%3Djavascript%3Aalert(1)%3E"
Fire-Request "eval()"      "$GATEWAY_URL/api/scada/status?q=eval(atob('YWxlcnQoMSk='))"
Write-Host ""

# -- Rate limit flood on /auth/token (limit: 5 req/60s) -----------------------
Write-Host "[*] Rate limit flood on /auth/token..." -ForegroundColor Gray
for ($i = 1; $i -le 8; $i++) {
    $script:TOTAL++
    try {
        $r = Invoke-WebRequest -Uri "$GATEWAY_URL/auth/token" `
            -Method POST -ContentType "application/json" `
            -Body "{`"client_id`":`"flood-test-$i`",`"secret`":`"flood`"}" `
            -UseBasicParsing `
            -ErrorAction Stop
        $code = $r.StatusCode
    } catch [System.Net.WebException] {
        $code = [int]$_.Exception.Response.StatusCode
    } catch { $code = 0 }

    if ($code -eq 429) {
        $script:BLOCKED++
        Write-Host "  [RATE LIMITED 429] request #$i" -ForegroundColor Green
    } else {
        Write-Host "  [Allowed $code] request #$i" -ForegroundColor Gray
    }
}
Write-Host ""

# -- Results -------------------------------------------------------------------
$DetectionRate = if ($TOTAL -gt 0) { [math]::Round($BLOCKED / $TOTAL * 100, 2) } else { 0 }

Write-Host "Results:" -ForegroundColor Cyan
Write-Host "  Requests sent:  $TOTAL"
Write-Host "  Blocked:        $BLOCKED"
Write-Host "  Detection rate: $DetectionRate%"

# Write JSON report anchored to $PSScriptRoot
$reportPath = Join-Path $SCRIPT_DIR "metrics_report.json"
$report = [ordered]@{
    requests_sent      = $TOTAL
    blocked_count      = $BLOCKED
    detection_rate_pct = $DetectionRate
    avg_latency_ms     = $null
    generated_at       = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ")
} | ConvertTo-Json
$report | Set-Content -Path $reportPath -Encoding UTF8

Write-Host ""
Write-Host "[OK] Report saved to $reportPath" -ForegroundColor Green

# Update README.md resume metrics section if marker exists
$readmePath = Join-Path $SCRIPT_DIR "..\README.md"
if (Test-Path $readmePath) {
    $readme = Get-Content $readmePath -Raw
    if ($readme -match "METRICS_START") {
        Write-Host "[*] Updating README.md resume metrics..." -ForegroundColor Gray
        $metricsBlock = @"
<!-- METRICS_START -->
| Metric | Value |
|---|---|
| Requests Sent | $TOTAL |
| Blocked Count | $BLOCKED |
| Detection Rate | ${DetectionRate}% |
<!-- METRICS_END -->
"@
        $readme = $readme -replace '(?s)<!-- METRICS_START -->.*?<!-- METRICS_END -->', $metricsBlock
        $readme | Set-Content -Path $readmePath -Encoding UTF8 -NoNewline
        Write-Host "[OK] README.md updated" -ForegroundColor Green
    }
}
