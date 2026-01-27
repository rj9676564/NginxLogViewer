import 'sonic_logger.dart';

void main() async {
  // 1. 初始化
  SonicLogger.init(deviceId: "flutter_demo_user_123");

  print("开始产生日志...");

  // 2. 模拟快速产生一些日志
  for (int i = 1; i <= 5; i++) {
    SonicLogger.d("HomeModule", "用户进入了第 $i 个页面");
  }

  // 3. 模拟一个错误日志，带上复杂的 body
  SonicLogger.e("Network", "接口请求超时", body: {
    "url": "https://api.example.com/v1/user",
    "method": "GET",
    "latency": "5000ms"
  });

  print("日志已加入缓存，正在等待 20 秒空闲...");
  
  // 在真实 Flutter 环境中，你不需要手动等待，
  // 只要用户停止操作 20 秒，后台就会自动触发 flush()。
}
