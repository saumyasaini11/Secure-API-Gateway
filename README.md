# Secure API Gateway & Threat Intelligence Monitor

A production-grade, high-performance security gateway built in **Go** to wrap and defend critical back-end services (such as SCADA, Actuator, and Industrial IoT endpoints). It functions as a hardened middleware stack providing multi-layer attack detection, asymmetric JWT authentication, sliding-window rate limiting, IP blocklists, and reverse proxying, paired with an interactive 3D cybersecurity dashboard (React + Vite).

---

## Live Demo & Walkthrough

* **Live Dashboard (Vercel):** *[Link your Vercel deployment URL here]*
* **Gateway API (Railway):** *[Link your Railway backend deployment URL here]*

### Interactive Demo Video / GIF
![Demo Mode & Attack Simulation](demo.gif)
*(Recruiters: Click the **"Run Attack Simulation"** button in the threats panel or generate tokens using the **JWT Generator** to watch the dashboard populate and update in real-time, even if the backend is spun down.)*

---

## Features

- **Scenario A (Live Presentation):** Connects to the backend via `GATEWAY_BASE` and `ADMIN_BASE` configurations. Triggers actual HTTP requests that register blocking events in Redis.
- **Scenario B (Unattended Portfolio Visit):** If the backend is spun down or unreachable, the dashboard automatically falls back to **Demo Mode** with a live indicator badge and seeds realistic threat statistics.
- **One-Click Attack Simulation:** An interactive button in the dashboard fires a volley of URL-encoded SQLi and XSS payloads at the gateway, populating metrics live.
- **Interactive JWT Generator:** Directly issues valid JWTs on the admin API (or mock JWTs in Demo Mode) and saves them in the browser's `localStorage` for immediate API testing.

---

## Architecture

```
                    +-------------------------------------------------------+
                    |                      Gateway  :8080                   |
                    |                                                       |
  Client ---------> |  [1] IP Blocklist       (gateway:blocked_ips Redis)   |
                    |  [2] Attack Detection   (SQLi / XSS / Oversized)      |
                    |  [3] JWT Validation     (RS256 asymmetric)            |
                    |  [4] Rate Limiter       (sliding window, per-route)   |
                    |  [5] Analytics Logger   (Redis lists & counters)      |
                    |  [6] Reverse Proxy  --> backends :9001 / :9002        |
                    +-------------------------------------------------------+
                    +-------------------------------------------------------+
                    |                     Admin API  :8081                  |
                    |  GET  /admin/stats        / GET  /admin/threats       |
                    |  GET  /admin/metrics      / GET  /admin/blocked-ips   |
                    |  POST /admin/block-ip     / POST /admin/token         |
                    |  GET  /admin/health                                   |
                    +-------------------------------------------------------+
                    +-------------------------------------------------------+
                    |                React Dashboard  :5173                 |
                    |  Globe / Packet terminal / Request feed               |
                    |  Threat intelligence / Attack breakdown / Live feed    |
                    +-------------------------------------------------------+
```

### Redis Key Schema

| Key Pattern | Type | Purpose |
|---|---|---|
| `ratelimit:<clientID>:<route>` | Sorted Set | Sliding-window rate limit timestamps |
| `analytics:requests` | List | Last 1000 request logs (JSON) |
| `analytics:client:<id>:count` | String | Per-client request counter |
| `analytics:route:<path>:count` | String | Per-route request counter |
| `analytics:bruteforce:<id>` | String | 401 counter (10-min window) |
| `analytics:threats` | List | Last 200 threat event logs |
| `analytics:threats:today:<date>` | String | Today's blocked count |
| `analytics:threats:type:<type>` | String | All-time per-attack-type count |
| `analytics:threats:endpoint:<path>` | String | All-time per-endpoint threat count |
| `gateway:blocked_ips` | Set | Permanently blocked IP addresses |

---

## Security Layers

| Layer | What It Does |
|---|---|
| **IP Blocklist** | Rejects IPs in `gateway:blocked_ips` Redis Set with `403`. Fail-open on Redis error. |
| **SQLi Detection** | Regex patterns: `OR 1=1`, `UNION SELECT`, `DROP TABLE`, `sleep()`, `xp_cmdshell`, etc. |
| **XSS Detection** | Patterns: `<script>`, `javascript:`, `onerror=`, `<iframe>`, `eval()`, `document.cookie`. |
| **Oversized Payload** | Rejects bodies > 1 MB with `413`. |
| **JWT Auth (RS256)** | Asymmetric signing - private key issues, public key validates. TTL: 15 min. |
| **Rate Limiting** | Sliding-window per `clientID + route`. Per-route limits in `config.yaml`. |
| **Brute Force Detection** | 5x HTTP 401 in 10 min flags a client in Redis. |

---

## Prerequisites

- **Go** 1.22+
- **Node.js** 18+ & **npm**
- **Docker Desktop** (for Redis)

---

## Local Development

### Option A - Full Docker Stack (Gateway + Redis)
```bash
docker-compose up --build -d
```
The gateway starts on `:8080` (API) and `:8081` (admin). 

Then start the dashboard separately:
```bash
cd frontend && npm install && npm run dev
```

### Option B - Manual Development
1. **Start Redis:**
```bash
docker-compose up -d redis
```
2. **Run the gateway:**
```bash
go run cmd/gateway/main.go
```
3. **Start the dashboard:**
```bash
cd frontend && npm install && npm run dev
```
Dashboard -> http://localhost:5173

---

## API Reference

### Gateway (:8080)
| Endpoint | Auth | Description |
|---|---|---|
| `POST /auth/token` | None (rate-limited) | Issue a JWT - body: `{"client_id":"x","secret":"y"}` |
| `GET /api/scada/sensor-data` | Bearer JWT | Proxied to backend :9001 |
| `GET /api/scada/status` | Bearer JWT | Proxied to backend :9001 |
| `POST /api/control/actuator` | Bearer JWT | Proxied to backend :9002 |

### Admin (:8081)
| Endpoint | Description |
|---|---|
| `GET /admin/stats` | Analytics: request counts, client breakdown, brute-force flags |
| `GET /admin/threats?page=1&size=20` | Threat log + attack type breakdown + top endpoints |
| `GET /admin/metrics` | Computed stats: uptime, block %, avg latency, detection rate |
| `POST /admin/block-ip` | Add IP to blocklist. Body: `{"ip":"1.2.3.4"}`. Requires `X-Admin-Key` header. |
| `GET /admin/blocked-ips` | List all currently blocked IPs |
| `GET /admin/health` | Health check |

---

## Scripts

Run from the project root or from inside `scripts/`:

```powershell
# Attack simulation (PowerShell - Windows native)
.\scripts\simulate_attacks.ps1

# Benchmarking
.\scripts\benchmark.ps1
```

```bash
# Bash equivalents (Git Bash / WSL)
bash scripts/simulate_attacks.sh
bash scripts/benchmark.sh
```

---

## Resume Metrics

<!-- METRICS_START -->
| Metric | Value |
|---|---|
| Requests Sent | 18 |
| Blocked Count | 14 |
| Detection Rate | 77.78% |
| Avg Latency | 4.54ms |
| Throughput | 210.53 req/s |
<!-- METRICS_END -->
