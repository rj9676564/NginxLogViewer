# Sonic Stellar API Documentation 📚

Sonic Stellar provides several API endpoints for pushing logs from various clients (Android, iOS, Flutter, etc.) and retrieving stats or historical entries.

## 📥 Inbound Logs (Pushing Data)

### 1. Batch Log Upload
Push multiple log entries in a single request. Supports Gzip compression for high-frequency logging.

- **Endpoint**: `POST /api/log/batch/:device_id`
- **Content-Type**: `application/json` (or `application/x-gzip`)
- **Body Structure**:
```json
{
  "device_id": "optional_if_in_url",
  "logs": [
    {
      "level": "d",
      "tag": "AUTH",
      "text": "User login success",
      "time": "2026-01-27T11:10:07.403",
      "body": "{\"user_id\": 123}"
    },
    {
      "level": "e",
      "tag": "NETWORK",
      "text": "Connection timeout"
    }
  ]
}
```
- **Fields**:
  - `level`: Log level (`v`, `d`, `i`, `w`, `e`).
  - `tag`: Category or module name.
  - `text`: Primary log message (mapped to Query in UI).
  - `time`: (Optional) ISO format. Server time used if omitted.
  - `body`: (Optional) Extended JSON data for the detail drawer.

---

### 2. Single Log Push (Quick Integration)
A simplified endpoint for sending single log entries without complex nesting.

- **Endpoint**: `POST /api/log/push/:device_id`
- **Supports**: Raw Text OR JSON body.

#### Mode A: Raw Text (Lowest Barrier)
```bash
curl -X POST http://YOUR_SERVER:58080/api/log/push/my-device-id \
     -d "System reached critical temperature"
```
*Tip: Add `?level=e&tag=HW` to the URL to specify metadata.*

#### Mode B: JSON (Structured)
```json
{
  "level": "i",
  "tag": "SYNC",
  "text": "Syncing completed",
  "body": "{\"items\": 42}"
}
```

---

## 🔍 Outbound Data (Retrieving Logs)

### 3. Fetch History
Used by the frontend to load persistent logs from SQLite.

- **Endpoint**: `GET /api/history`
- **Query Parameters**:
  - `limit`: Max items (default: 50).
  - `device`: Filter by device ID.
  - `level`: Filter by level.
  - `tag`: Filter by tag.

### 4. Real-time Stream
- **Endpoint**: `WS /ws`
- **Protocol**: WebSocket
- **Behavior**: Broadcasts every new log entry (parsed from Nginx file or received via API) to all connected clients.

---

## 🛠️ Utility APIs

### 5. Get Filter Metadata
Used to populate the sidebar dropdowns.

- **Endpoints**: 
  - `GET /api/devices`: Returns a JSON array of unique device IDs.
  - `GET /api/tags`: Returns a JSON array of unique tags.
  
---

## 🧩 Integration Examples

### Flutter (Dart)
Using `Dio` or `http` to send batches:
```dart
void sendLogs(List<Map> logs) async {
  await dio.post('/api/log/batch/flutter-client', data: {
    "logs": logs
  });
}
```

### Shell (cURL)
```bash
# Push a quick debug message
curl -X POST "http://localhost:58080/api/log/push/script-01?level=d&tag=CRON" \
     -d "Backup task started"
```
