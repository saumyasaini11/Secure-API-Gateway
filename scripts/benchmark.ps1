# benchmark.ps1 - measures gateway throughput and avg latency.
# Writes results to metrics_report.json in the same directory as this script.
# Runs natively on Windows PowerShell - no extra tools required.

$ErrorActionPreference = 'SilentlyContinue'
$SCRIPT_DIR  = $PSScriptRoot
$GATEWAY_URL = "http://localhost:8080"
$REQUESTS    = 200
$Errors      = 0
$TotalLatencyMs = 0

Write-Host ""
Write-Host "+--------------------------------------+" -ForegroundColor Cyan
Write-Host "|   Secure API Gateway Benchmark       |" -ForegroundColor Cyan
Write-Host "+--------------------------------------+" -ForegroundColor Cyan
Write-Host ""

# -- Obtain a JWT token --------------------------------------------------------
Write-Host "[*] Obtaining JWT token..." -ForegroundColor Gray
try {
    $tokenResp = Invoke-RestMethod -Uri "$GATEWAY_URL/auth/token" `
        -Method POST -ContentType "application/json" `
        -Body '{"client_id":"bench-client","secret":"bench"}'
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
Write-Host "[*] Firing $REQUESTS requests against /api/scada/sensor-data..." -ForegroundColor Gray
Write-Host ""

# -- Benchmark loop ------------------------------------------------------------
$OverallStart = Get-Date

for ($i = 1; $i -le $REQUESTS; $i++) {
    $reqStart = Get-Date
    try {
        $r = Invoke-WebRequest -Uri "$GATEWAY_URL/api/scada/sensor-data" `
            -Headers @{ Authorization = "Bearer $TOKEN" } `
            -UseBasicParsing `
            -ErrorAction Stop
        $code = $r.StatusCode
    } catch [System.Net.WebException] {
        $code = [int]$_.Exception.Response.StatusCode
    } catch { $code = 0 }

    $latency = ((Get-Date) - $reqStart).TotalMilliseconds
    $TotalLatencyMs += $latency

    # Any code other than 200 or 502 is unexpected
    if ($code -notin @(200, 502)) {
        $Errors++
    }

    if ($i % 50 -eq 0) {
        Write-Host ("    ... {0} / {1} done" -f $i, $REQUESTS) -ForegroundColor Gray
    }
}

$TotalTimeS  = [math]::Round(((Get-Date) - $OverallStart).TotalSeconds, 2)
$AvgLatency  = [math]::Round($TotalLatencyMs / $REQUESTS, 2)
$RPS         = [math]::Round($REQUESTS / $TotalTimeS, 2)

Write-Host ""
Write-Host "+--------------------------------------+" -ForegroundColor Cyan
Write-Host "|             Results                  |" -ForegroundColor Cyan
Write-Host ("|  Requests:       {0,-20}|" -f $REQUESTS) -ForegroundColor Cyan
Write-Host ("|  Total time:     {0,-18}s|" -f $TotalTimeS) -ForegroundColor Cyan
Write-Host ("|  Avg latency:    {0,-18}ms|" -f $AvgLatency) -ForegroundColor Cyan
Write-Host ("|  Throughput:     {0,-16}req/s|" -f $RPS) -ForegroundColor Cyan
Write-Host ("|  Errors:         {0,-20}|" -f $Errors) -ForegroundColor Cyan
Write-Host "+--------------------------------------+" -ForegroundColor Cyan

# -- Write / merge metrics_report.json (anchored to $PSScriptRoot) -------------
$reportPath = Join-Path $SCRIPT_DIR "metrics_report.json"

# Preserve existing attack sim fields if the file already exists
$ExistingSent    = 0
$ExistingBlocked = 0
$ExistingRate    = 0
if (Test-Path $reportPath) {
    $existing = Get-Content $reportPath -Raw | ConvertFrom-Json
    $ExistingSent    = $existing.requests_sent
    $ExistingBlocked = $existing.blocked_count
    $ExistingRate    = $existing.detection_rate_pct
}
if (-not $ExistingSent) { $ExistingSent = 0 }
if (-not $ExistingBlocked) { $ExistingBlocked = 0 }
if (-not $ExistingRate) { $ExistingRate = 0 }
$ExistingSent    = [int]$ExistingSent
$ExistingBlocked = [int]$ExistingBlocked
$ExistingRate    = [double]$ExistingRate

$report = [ordered]@{
    requests_sent        = $ExistingSent
    blocked_count        = $ExistingBlocked
    detection_rate_pct   = $ExistingRate
    avg_latency_ms       = $AvgLatency
    requests_per_second  = $RPS
    total_time_seconds   = $TotalTimeS
    errors               = $Errors
    generated_at         = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ")
} | ConvertTo-Json

$report | Set-Content -Path $reportPath -Encoding UTF8

Write-Host ""
Write-Host "[OK] Report saved to $reportPath" -ForegroundColor Green

# -- Update README.md resume metrics section -----------------------------------
$readmePath = Join-Path $SCRIPT_DIR "..\README.md"
if (Test-Path $readmePath) {
    $readme = Get-Content $readmePath -Raw
    if ($readme -match "METRICS_START") {
        Write-Host "[*] Updating README.md resume metrics..." -ForegroundColor Gray
        $metricsBlock = @"
<!-- METRICS_START -->
| Metric | Value |
|---|---|
| Requests Sent | $ExistingSent |
| Blocked Count | $ExistingBlocked |
| Detection Rate | ${ExistingRate}% |
| Avg Latency | ${AvgLatency}ms |
| Throughput | ${RPS} req/s |
<!-- METRICS_END -->
"@
        $readme = $readme -replace '(?s)<!-- METRICS_START -->.*?<!-- METRICS_END -->', $metricsBlock
        $readme | Set-Content -Path $readmePath -Encoding UTF8 -NoNewline
        Write-Host "[OK] README.md updated" -ForegroundColor Green
    }
}
