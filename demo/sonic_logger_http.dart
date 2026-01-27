import 'dart:convert';
import 'dart:io';
import 'package:http/http.dart' as http;

/// SonicLoggerHttp - 基于 http 库的日志上报工具
class SonicLogger {
  static const String serverUrl = "http://localhost:58080"; 
  static const int maxBatchSize = 100;
  static const Duration idleTimeout = Duration(seconds: 20);

  static String _deviceId = "unknown_flutter_device";
  static final List<Map<String, dynamic>> _queue = [];
  static Timer? _idleTimer;

  static void init({required String deviceId}) {
    _deviceId = deviceId;
  }

  static void v(String tag, String text, {Map<String, dynamic>? body}) => _log("v", tag, text, body);
  static void d(String tag, String text, {Map<String, dynamic>? body}) => _log("d", tag, text, body);
  static void e(String tag, String text, {Map<String, dynamic>? body}) => _log("e", tag, text, body);

  static void _log(String level, String tag, String text, Map<String, dynamic>? body) {
    _queue.add({
      "level": level,
      "tag": tag,
      "text": text,
      "body": body != null ? jsonEncode(body) : "-",
      "time": DateTime.now().toIso8601String(),
    });

    if (_queue.length >= maxBatchSize) {
      flush();
      return;
    }
    _resetIdleTimer();
  }

  static void _resetIdleTimer() {
    _idleTimer?.cancel();
    _idleTimer = Timer(idleTimeout, () {
      if (_queue.isNotEmpty) flush();
    });
  }

  static Future<void> flush() async {
    _idleTimer?.cancel();
    if (_queue.isEmpty) return;

    final List<Map<String, dynamic>> logsToSend = List.from(_queue);
    _queue.clear();

    final payload = {
      "device_id": _deviceId,
      "logs": logsToSend,
    };

    try {
      final bodyString = jsonEncode(payload);
      final bodyBytes = utf8.encode(bodyString);
      final gzippedBytes = GZipCodec().encode(bodyBytes);

      await http.post(
        Uri.parse("$serverUrl/api/log/batch/$_deviceId"),
        headers: {
          "Content-Type": "application/json",
          "Content-Encoding": "gzip",
        },
        body: gzippedBytes,
      ).timeout(const Duration(seconds: 10));
    } catch (e) {
      print("[SonicLoggerHttp] Error: $e");
    }
  }
}
 suburbancy
