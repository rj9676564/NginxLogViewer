# Nginx Log Viewer 🚀

A lightweight, real-time Nginx log visualization tool built with Go and Vue.js.

![Build Status](https://github.com/rj9676564/NginxLogViewer/actions/workflows/docker-publish.yml/badge.svg)
![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Go](https://img.shields.io/badge/go-1.23+-00ADD8.svg)
![Vue](https://img.shields.io/badge/vue-3.x-4FC08D.svg)

## ✨ Features

- **Real-time Streaming**: Watch your logs flow in milliseconds using WebSockets.
- **Low Resource Usage**: Backend written in Go for minimal memory footprint and high performance.
- **Rich Visualization**: 
  - Color-coded HTTP status codes (2xx, 3xx, 4xx, 5xx).
  - Explicit parsing and display of `Query Strings` and `POST Body`.
- **Powerful Filtering**: Filter logs instantly using Regex or simple text matching.
- **Customizable**: Configure your Nginx `log_format` directly in the UI to match your server configuration.
- **Single-File Frontend**: No `npm install` or complex build steps required. Just one HTML file using Vue 3 via CDN.
- **Docker Ready**: Easy deployment with Docker and Docker Compose.

## 🛠️ Tech Stack

- **Backend**: Go (Golang), `gorilla/websocket`, `hpcloud/tail`
- **Frontend**: HTML5, CSS3, Vue.js 3 (Composition API)

## 🚀 Getting Started

### Prerequisites

- Go 1.20+ (for local run)
- Docker (optional, for containerized deployment)

### Running Locally

1. Clone the repository:
   ```bash
   git clone git@github.com:rj9676564/NginxLogViewer.git
   cd NginxLogViewer
   ```

2. Run the server:
   ```bash
   # Listen on port 58080 and watch a specific log file
   go run main.go -addr :58080 -file /var/log/nginx/access.log
   ```

## Run Locally

### 1. Start Backend (Go)
The backend serves the API and WebSocket at port `58080`.
```bash
go mod tidy
go run main.go -file /path/to/access.log
# Ensure to pass a valid log file path
```

### 2. Start Frontend (Vue + Vite)
In a new terminal, start the frontend dev server. It will proxy API requests to the backend.
```bash
cd frontend
npm install
npm run dev
```
Access the UI at `http://localhost:5173`.

## Docker Build

The Dockerfile is a multi-stage build that compiles both the frontend (Node.js) and backend (Go).

```bash
docker build -t nginx-log-viewer .
docker run -p 58080:58080 -v /var/log/nginx/access.log:/var/log/nginx/access.log nginx-log-viewer
```
(When running via Docker, the Go server serves the static frontend files on port 58080). 
   ```

   **Or using Docker Compose:**

   Edit `docker-compose.yml` to point to your log file, then run:
   ```bash
   docker-compose up -d
   ```

## ⚙️ Configuration

The application can be configured via Command-line flags, Environment variables, or a JSON config file.

### Priority Order
1. **Command-line Flags** (Highest)
2. **Environment Variables**
3. **JSON Config File** (`-config` flag)
4. **Default Values** (Lowest)

### Available Settings
| Flag | Env Var | Config Key | Default | Description |
|------|---------|------------|---------|-------------|
| `-addr` | `LISTEN_ADDR` | `addr` | `:58080` | Server listening address |
| `-file` | `LOG_FILE` | `log_file` | `/var/log/nginx/access.log` | Path to Nginx log file |
| `-format`| `LOG_FORMAT` | `log_format` | (Custom with POST_BODY) | Nginx `log_format` string |
| `-db` | `DB_PATH` | `db_path` | `./logs.db` | Path to SQLite database |
| `-config`| - | - | - | Path to JSON config file |

### Example Config File (`config.json`)
```json
{
    "addr": ":58080",
    "log_file": "/var/log/nginx/access.log",
    "log_format": "$remote_addr - $remote_user [$time_local] \"$request\" $status ..."
}
```

### CLI Arguments

| Flag | Description | Default |
|------|-------------|---------|
| `-addr` | HTTP service address | `:58080` |
| `-file` | Path to the log file to watch | `/var/log/nginx/access.log` |

### Log Format

The viewer supports custom Nginx log formats. click "Configure Format" in the sidebar to paste your format string. 

**Default Format:**
```nginx
$remote_addr - $remote_user [$time_local] "$request" $status GET_ARGS: "$query_string" POST_BODY: "$request_body"
```

## 📄 License

This project is licensed under the MIT License.
