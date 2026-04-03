# Log Viewer

一个基于 Go + Vue 3 的实时日志查看工具。

当前版本的核心能力是：

- 通过 HTTP API 接收日志
- 通过 WebSocket 实时推送到前端
- 通过 SQLite 保存历史记录
- 支持按设备、级别、标签筛选日志

![Build Status](https://github.com/rj9676564/NginxLogViewer/actions/workflows/docker-publish.yml/badge.svg)
![Go Version](https://img.shields.io/badge/go-1.24-00ADD8.svg)
![Vue Version](https://img.shields.io/badge/vue-3.x-4FC08D.svg)

## ✨ 功能概览

- **实时日志流**：新日志写入后立即广播到前端
- **单条 / 批量写入**：支持 `push` 和 `batch` 两种接入方式
- **历史记录查询**：日志持久化到 SQLite，页面刷新后仍可回看
- **多维筛选**：支持按 `device_id`、`level`、`tag` 查询
- **接入成本低**：脚本、前端、移动端都可以直接通过 HTTP 接入
- **部署简单**：提供 Dockerfile 和 `docker-compose.yml`

## 🧱 项目结构

```text
.
├── backend/            Go 后端
├── frontend/           Vue 3 前端
├── demo/               接入示例
├── scripts/            辅助脚本
├── Dockerfile
├── docker-compose.yml
├── config.json.example
├── README.md
└── API.md
```

## 🚀 快速开始

### 1. 使用 Docker 启动

```bash
docker run -d \
  --name log-viewer \
  -p 58080:58080 \
  -v ./data:/app/data \
  laibin2886/log-viewer:latest \
  -db /app/data/logs.db
```

启动后访问：

```text
http://localhost:58080
```

### 2. 使用 Docker Compose 启动

仓库已提供 [docker-compose.yml](/Users/laibin/Documents/UGit/NginxLogViewer/docker-compose.yml)。

直接启动：

```bash
docker compose up -d
```

### 3. 本地开发启动

#### 后端

```bash
cd backend
go mod download
go run . -db ./logs.db
```

#### 前端

```bash
cd frontend
npm install
npm run dev
```

前端开发地址默认是：

```text
http://localhost:5173
```

## ⚙️ 配置说明

程序支持 4 种配置来源，优先级从高到低如下：

1. 命令行参数
2. 环境变量
3. `-config` 指定的 JSON 配置文件
4. 默认值

### 可用配置项

| 参数 | 环境变量 | 配置文件字段 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `-addr` | `LISTEN_ADDR` | `addr` | `:58080` | 服务监听地址 |
| `-db` | `DB_PATH` | `db_path` | `./logs.db` | SQLite 数据库路径 |
| `-static` | `STATIC_DIR` | `static_dir` | `./frontend/dist` | 前端静态文件目录 |
| `-config` | 无 | 无 | 空 | JSON 配置文件路径 |

### 配置文件示例

可以参考 [config.json.example](/Users/laibin/Documents/UGit/NginxLogViewer/config.json.example)：

```json
{
  "addr": ":58080",
  "db_path": "./logs.db",
  "static_dir": "./frontend/dist"
}
```

使用方式：

```bash
cd backend
go run . -config ../config.json.example
```

## 📡 API 文档

日志写入和读取接口见：

- [API.md](/Users/laibin/Documents/UGit/NginxLogViewer/API.md)

适用场景：

- Flutter / Android / iOS 调试日志上报
- 脚本或定时任务记录运行日志
- 小型内部工具做统一日志面板

## 🛠️ 开发说明

### 技术栈

- 后端：Go 1.24、WebSocket、SQLite
- 前端：Vue 3、Vite、Ant Design Vue

### 常见开发流程

1. 启动后端服务
2. 启动前端开发服务器
3. 打开浏览器查看实时日志
4. 使用 `demo/` 或脚本向接口推送日志
5. 在页面中查看实时数据和历史记录

## 📄 License

MIT
