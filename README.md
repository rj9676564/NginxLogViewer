# Sonic Stellar: Nginx Log Viewer 🚀

**Sonic Stellar** is a high-performance, real-time Nginx log visualization and analysis tool. Built with a Go 1.24 backend and a Vue 3 + Vite frontend, it provides an instantaneous, beautiful, and searchable view of your server traffic.

![Build Status](https://github.com/rj9676564/NginxLogViewer/actions/workflows/docker-publish.yml/badge.svg)
![Go Version](https://img.shields.io/badge/go-1.24-00ADD8.svg)
![Vue Version](https://img.shields.io/badge/vue-3.x-4FC08D.svg)
![Docker Multi-Arch](https://img.shields.io/badge/docker-multi--arch-blue.svg)

## ✨ Features

-   **⚡ Real-time Streaming**: Watch logs flow in with millisecond latency via WebSockets.
-   **🔍 Intelligent Parsing**: Robust regex-based parsing that supports standard `combined` formats and complex custom formats (including `$request_body`, `$query_string`, and JSON fields).
-   **🚀 Performance First**:
    -   **Backend**: Compiled Go 1.24 binary with minimal memory footprint.
    -   **Frontend**: Virtualized list rendering for handling thousands of log lines without lag.
    -   **CI/CD**: Optimized multi-arch Docker builds (amd64/arm64) that complete in < 1.5 minutes.
-   **🎨 Premium UI/UX**:
    -   **Modern Aesthetics**: Built with Ant Design Vue for a clean, professional look.
    -   **Dark/Light Mode**: Seamless theme switching with persistence.
    -   **Smart Context**: Auto-decodes URL-encoded query strings and formats JSON request bodies for humans.
-   **📈 Built-in Analytics**: Real-time PV/UV tracking and status code distribution.
-   **🐳 Cloud Native**: Production-ready Docker images with extremely small footprints using Alpine Linux.

## 🛠️ Tech Stack

-   **Backend**: Go 1.24, Gorilla WebSockets, SQLite (for history).
-   **Frontend**: Vue 3 (Composition API), Vite, Ant Design Vue, pnpm.
-   **DevOps**: Docker (Multi-stage + Multi-arch), GitHub Actions (BuildKit Cache).

## 🚀 Quick Start

### 1. Run with Docker (Recommended)

```bash
docker run -d \
  --name nginx-viewer \
  -p 58080:58080 \
  -v /var/log/nginx/access.log:/logs/access.log:ro \
  -e LOG_FILE=/logs/access.log \
  docker.io/rj9676564/nginx-log-viewer:latest
```

### 2. Run with Docker Compose

Edit your `docker-compose.yml`:

```yaml
services:
  log-viewer:
    image: rj9676564/nginx-log-viewer:latest
    ports:
      - "58080:58080"
    volumes:
      - /var/log/nginx/access.log:/var/log/nginx/access.log:ro
    environment:
      - LOG_FILE=/var/log/nginx/access.log
    restart: always
```

## 🛠️ Development Setup

### Prerequisites
-   Go 1.24+
-   Node.js 20+
-   pnpm 9+

### Local Environment

1.  **Start Backend**:
    ```bash
    go mod download
    go run main.go -file /path/to/your/access.log
    ```

2.  **Start Frontend**:
    ```bash
    cd frontend
    pnpm install
    pnpm dev
    ```
    Access the dev portal at `http://localhost:5173`.

## ⚙️ Configuration

| Flag / Env | Config Key | Default | Description |
| :--- | :--- | :--- | :--- |
| `-addr` / `LISTEN_ADDR` | `addr` | `:58080` | Server address |
| `-file` / `LOG_FILE` | `log_file` | `/var/log/nginx/access.log` | Path to log file |
| `-format` / `LOG_FORMAT` | `log_format` | (Custom) | Nginx `log_format` string |
| `-db` / `DB_PATH` | `db_path` | `./logs.db` | SQLite storage path |

### Custom Log Format
Sonic Stellar is designed to excel with custom formats. Example:
```nginx
log_format main '$remote_addr - $remote_user [$time_local] "$request" '
                '$status $body_bytes_sent "$http_referer" '
                '"$http_user_agent" "$request_body"';
```

## 🏗️ Docker Build Optimization
We take CI/CD performance seriously. Our current pipeline features:
-   **Native Cross-Compilation**: Go builds for `arm64` run at native `amd64` speeds using `$BUILDPLATFORM`.
-   **Frontend Tree Shaking**: Ant Design components are imported on-demand, reducing bundle size by 70%.
-   **Cache Mounts**: Persistently caches `pnpm` store and `go mod` across builds.
-   **Single-Phase Frontend**: Shared architectural assets are built only once for multi-platform images.

Detailed optimization records can be found in [DOCKER_OPTIMIZATION.md](./DOCKER_OPTIMIZATION.md).

## 📄 License
MIT License - Developed by [rj9676564](https://github.com/rj9676564).
