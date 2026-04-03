# API 文档

本文档说明如何把客户端日志推送到 Log Viewer，以及如何读取历史数据和辅助筛选数据。

默认服务地址示例：

```text
http://localhost:58080
```

## ✨ 接口总览

| 接口 | 方法 | 说明 |
| --- | --- | --- |
| `/api/log/batch/:device_id` | `POST` | 批量上报日志 |
| `/api/log/push/:device_id` | `POST` | 单条上报日志 |
| `/api/history` | `GET` | 查询历史日志 |
| `/api/stats` | `GET` | 查询基础统计 |
| `/api/devices` | `GET` | 获取设备列表 |
| `/api/tags` | `GET` | 获取标签列表 |
| `/ws` | `WS` | 实时日志流 |

## 📥 写入类接口

### 1. 批量上报日志

适合高频日志、移动端批量补传、离线重发等场景。

- **路径**：`POST /api/log/batch/:device_id`
- **Content-Type**：`application/json`
- **可选压缩**：支持 `Content-Encoding: gzip`

说明：

- `device_id` 可以放在 URL 路径中
- 也可以放在 JSON 里的 `device_id` 字段中
- 如果两者都传，代码当前优先使用请求体中的 `device_id`

#### 请求体

```json
{
  "device_id": "optional-device-id",
  "logs": [
    {
      "level": "d",
      "tag": "AUTH",
      "text": "User login success",
      "time": "2026-01-27T11:10:07.403",
      "body": "{\"user_id\":123}"
    },
    {
      "level": "e",
      "tag": "NETWORK",
      "text": "Connection timeout"
    }
  ]
}
```

#### 字段说明

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `device_id` | 否 | 设备标识，也可放在 URL 中 |
| `logs` | 是 | 日志数组 |
| `logs[].level` | 否 | 日志级别，常见值：`v` / `d` / `i` / `w` / `e` |
| `logs[].tag` | 否 | 模块或分类标签 |
| `logs[].text` | 否 | 主日志内容，服务端会映射为 `query` 字段 |
| `logs[].time` | 否 | 日志时间。未传时服务端使用当前时间 |
| `logs[].body` | 否 | 详情信息，通常放 JSON 字符串 |

#### 成功响应

```text
Processed 2 logs
```

#### cURL 示例

```bash
curl -X POST "http://localhost:58080/api/log/batch/device-001" \
  -H "Content-Type: application/json" \
  -d '{
    "logs": [
      {
        "level": "i",
        "tag": "SYNC",
        "text": "Sync started"
      },
      {
        "level": "e",
        "tag": "SYNC",
        "text": "Sync failed",
        "body": "{\"reason\":\"timeout\"}"
      }
    ]
  }'
```

### 2. 单条上报日志

适合脚本快速接入、调试打点、低频日志上报。

- **路径**：`POST /api/log/push/:device_id`

支持两种模式：

#### 模式 A：纯文本

请求体直接传字符串，`level` 和 `tag` 可以走 query 参数。

```bash
curl -X POST \
  "http://localhost:58080/api/log/push/script-01?level=d&tag=CRON" \
  -d "Backup task started"
```

#### 模式 B：JSON

```json
{
  "level": "i",
  "tag": "SYNC",
  "text": "Sync completed",
  "body": "{\"items\":42}"
}
```

#### cURL 示例

```bash
curl -X POST "http://localhost:58080/api/log/push/mobile-01" \
  -H "Content-Type: application/json" \
  -d '{
    "level": "w",
    "tag": "AUTH",
    "text": "Token will expire soon",
    "body": "{\"expire_in\":120}"
  }'
```

#### 注意事项

- 请求体为空会返回 `400`
- 如果请求体是 JSON，但既没有 `text` 也没有 `body`，服务端会按纯文本处理
- 该接口当前文档语义是“单条日志”，但服务端不会强制校验 `level` / `tag`

## 📤 读取类接口

### 3. 查询历史日志

- **路径**：`GET /api/history`

#### 查询参数

| 参数 | 说明 |
| --- | --- |
| `device` | 按设备 ID 过滤 |
| `level` | 按日志级别过滤 |
| `tag` | 按标签模糊匹配 |

说明：

- 当前服务端固定返回最近 `200` 条
- 虽然旧文档里提到 `limit`，但当前代码并未实际支持自定义 `limit`

#### 示例

```bash
curl "http://localhost:58080/api/history?device=device-001&level=e&tag=AUTH"
```

#### 返回示例

```json
[
  {
    "id": 101,
    "ip": "127.0.0.1:59422",
    "time": "27/Mar/2026:10:00:00 +0800",
    "method": "PUSH",
    "path": "/api/log/push",
    "status": 200,
    "bytes": 0,
    "referer": "",
    "ua": "",
    "browser": "",
    "os": "",
    "device": "",
    "device_id": "device-001",
    "level": "e",
    "tag": "AUTH",
    "query": "Login failed",
    "body": "{\"reason\":\"expired token\"}",
    "raw": "[e] AUTH: Login failed",
    "created_at": 1774576800
  }
]
```

### 4. 查询统计数据

- **路径**：`GET /api/stats`

当前返回：

- `pv`：日志总数
- `uv`：按 `ip` 去重后的数量

```bash
curl "http://localhost:58080/api/stats"
```

响应示例：

```json
{
  "pv": 1024,
  "uv": 138
}
```

### 5. 获取设备列表

- **路径**：`GET /api/devices`

返回所有非空的 `device_id` 去重结果。

```bash
curl "http://localhost:58080/api/devices"
```

### 6. 获取标签列表

- **路径**：`GET /api/tags`

返回所有非空的 `tag` 去重结果。

```bash
curl "http://localhost:58080/api/tags"
```

## 🔄 实时日志流

### 7. WebSocket 实时订阅

- **路径**：`WS /ws`

连接成功后，服务端会把新收到的日志实时广播给所有客户端。

适合：

- 前端实时日志面板
- 自定义监控页面
- 调试期间旁路监听

浏览器示例：

```javascript
const ws = new WebSocket("ws://localhost:58080/ws");

ws.onmessage = (event) => {
  const log = JSON.parse(event.data);
  console.log(log);
};
```

## 🧪 接入示例

仓库中已经提供了几个参考文件：

- [demo/log_viewer_logger.ts](/Users/laibin/Documents/UGit/NginxLogViewer/demo/log_viewer_logger.ts)
- [demo/log_viewer_logger.js](/Users/laibin/Documents/UGit/NginxLogViewer/demo/log_viewer_logger.js)
- [demo/log_viewer_logger_http.dart](/Users/laibin/Documents/UGit/NginxLogViewer/demo/log_viewer_logger_http.dart)
- [demo/log_viewer_logger_dio.dart](/Users/laibin/Documents/UGit/NginxLogViewer/demo/log_viewer_logger_dio.dart)

TypeScript 示例：

```typescript
const logger = new LogViewerLogger("http://localhost:58080", "web-001");
await logger.push("App Started", { level: "i", tag: "INIT" });
```

## ⚠️ 已知实现细节

为了避免文档和代码脱节，这里补充几个当前实现上的真实行为：

- `/api/history` 当前固定返回最近 200 条，不支持自定义 `limit`
- 批量上报支持 `Content-Encoding: gzip`，不是 `Content-Type: application/x-gzip`
- `time` 字段服务端不强制做时间格式校验，建议调用方自行保持统一格式
- 所有读取接口都带有 `Access-Control-Allow-Origin: *`
