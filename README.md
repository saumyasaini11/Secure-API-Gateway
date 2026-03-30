# 🛡️ Secure API Gateway

A high-performance, enterprise-grade secure API gateway built with Go. Designed for mission-critical endpoints (SCADA, Actuator, IoT), this gateway acts as a robust middlebox providing Authentication, Rate Limiting, Request Proxying, and Real-time Analytics. 

It comes with a built-in high-fidelity cybersecurity dashboard for real-time traffic monitoring and threat detection.

## ✨ Key Features

- **🚀 High-Performance Proxy**: Efficient routing to core backend microservices (e.g., SCADA, Control Systems).
- **🔐 JWT Authentication**: Asymmetric key-based JWT issuance and strict endpoint verification. 
- **⚖️ Dynamic Rate Limiting**: Built-in limits managed via Redis or Memory to protect against DDoS and abuse. Configuration driven at the per-route level.
- **📊 Real-time Dashboard**: A premium frontend providing comprehensive visibility into active connections, blocked requests, and geographical threat visualizations.
- **⚙️ Configuration Driven**: Easy to configure through `config.yaml` to specify backend routes, environments, and rate limit rules.

## 🏗️ Architecture

- **Backend**: Golang (`cmd/gateway/main.go`)
- **Frontend**: Vite (`frontend/` and `dashboard/`)
- **Storage/Caching**: Redis (`docker-compose.yml`)

## 🛠️ Prerequisites

- **Go** (1.21+)
- **Node.js** & **npm** (for the frontend dashboard)
- **Docker & Docker Compose** (for running the Redis container)

## 🚀 Getting Started

### 1. Set Up Redis
Ensure you have Redis running in the background for distributed rate limiting and state management.
```bash
docker-compose up -d
```

### 2. Generate SSL/JWT Keys
Make sure you have your Private and Public `.pem` keys available inside the `keys/` directory for secure JWT authentication:
```bash
mkdir keys
# Generate your RSA keys here
```

### 3. Run the Go Backend (API Gateway)
Serve the API gateway locally:
```bash
go run cmd/gateway/main.go
```
The gateway will start on the port configured in `config.yaml` (default: `8080`) and the admin analytics server on `8081`.

### 4. Start the Cybersecurity Dashboard
Open a new terminal to start the high-fidelity UI dashboard:
```bash
cd frontend
npm install
npm run dev
```

## ⚙️ Configuration Reference (`config.yaml`)

Your API Gateway depends heavily on the `config.yaml` properties. Below is an example structure:

```yaml
server:
  port: 8080
  admin_port: 8081
  env: development

redis:
  addr: localhost:6379
  password: securepass123
  db: 0

jwt:
  private_key_path: ./keys/private.pem
  public_key_path: ./keys/public.pem

routes:
  - path: /api/scada/sensor-data
    backend: http://localhost:9001
    rate_limit:
      requests: 100
      window_seconds: 60
```

## 🤝 Contributing
Contributions, issues, and feature requests are welcome!

## 📄 License
This project is licensed under the MIT License.
