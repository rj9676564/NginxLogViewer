import 'dart:convert';
import 'dart:io';
import 'package:dio/dio.dart';

/// SonicLoggerDio - 基于 Dio 库的日志上报工具
class SonicLogger {
  static const String serverUrl = "http://localhost:58080"; 
  static const int maxBatchSize = 100;
  static const Duration idleTimeout = Duration(seconds: 20);

  static String _deviceId = "unknown_flutter_device";
  static final List<Map<String, dynamic>> _queue = [];
  static Timer? _idleTimer;
  
  // 建议重用 Dio 实例
  static final Dio _dio = Dio(BaseOptions(
    connectTimeout: const Duration(seconds: 5),
    receiveTimeout: const Duration(seconds: 5),
  ));

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
      // 启用 Gzip 压缩以减少流量
      final bodyBytes = utf8.encode(jsonEncode(payload));
      final gzippedBytes = GZipCodec().encode(bodyBytes);

      await _dio.post(
        "$serverUrl/api/log/batch/$_deviceId",
        data: Stream.fromIterable([gzippedBytes]),
        options: Options(
          headers: {
            "Content-Encoding": "gzip",
            "Content-Type": "application/json",
          },
        ),
      );
    } catch (e) {
      print("[SonicLoggerDio] Error: $e");
      // 优化建议：在此处将 logsToSend 存入本地数据库 (如 sqflite)，待网络恢复后重试
    }
  }
}
