# psu-monitor-go — CRPS 电源 PMBus 监控（Go 版）

光宝 LiteOn PS-2551-5L CRPS 550W 电源的 PMBus 监控服务，通过主板 F_PANEL GPIO
软件 bitbang I2C 读取电压/电流/功率/温度/转速/状态字，并控制风扇。
本工程是原 Python 版（psu-monitor-fpk）的 **Go 语言移植**，API 与启动参数完全兼容。

## 版本
- **1.0.48**：风扇手动占比范围收紧为 10-100%——前端滑条 min 由 0 改 10、提示文案同步；后端 /api/fan 与 writeFanPct 对 <10 的占比钳制到 10%（光宝固件 10% 以下实测无效）。
- **1.0.47**：功能精简——移除温控曲线（前端面板/导出、recordCurve 采样、GET /api/curve）；移除 GPIO 配置区两个诊断按钮及其后端（"试0x3A恢复自动" GET /api/fan-test、"写假温度测风扇" GET /api/temp-test），连带删除仅被诊断使用的 writeByte/readByteData/tempToLinear11/hexStr/pval；风扇手动/自动控制、状态读取、耗电统计、GPIO 配置/探测均不受影响。
- **1.0.46**：新增耗电量统计——后台每 15s 采样 READ_PIN 矩形积分，按总量/今日/本月/按日明细累计；持久化 energy.json（默认与 --config 同目录，可用 --energy 指定，临时文件+rename 原子写入），服务重启若距上次落盘 ≤10 分钟按断电前功率补计；新增 GET /api/energy 与 POST /api/energy（统计清零）；前端新增"耗电统计"面板（累计/今日/本月/实时功率卡片 + 近 14 日柱状图 + 统计清零）。
- **1.0.45**：并发与安全加固——总线切换/复位全程加锁消除数据竞争；/dev/port 句柄常开+方向寄存器缓存（位速率提升）；NewBus 校验非法引脚名防向 DMA 区写入；Unix socket 权限收紧 0750；日志 O_NOFOLLOW+按大小轮转；文件头补 MIT 署名；/api/config readable 加 TTL 缓存；诊断端点支持请求取消；自动探测与连通检测去重重试。
- **1.0.44**：后端移植为 Go 静态二进制（不依赖 Python 解释器）
- 默认接线：SCL=Pin19、SDA=Pin20，PMBus 地址 0x5B

## 目录
```
main.go          后端主程序（~1200 行，唯一源文件）
go.mod           Go module 定义
ui/static/       前端三件套（index.html / app.js / styles.css，与原版一致）
```

## 编译
```bash
go build -ldflags "-s -w" -o psu-monitor-go .   # 本机架构(-s -w 去掉调试符号, 8MB→5.4MB)
# 飞牛 NAS（x86_64）交叉编译：
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o psu-monitor-go .
```

## 运行
```bash
# TCP 模式（本机调试）
./psu-monitor-go serve --addr 127.0.0.1 --port 3878 --web-root ui/static

# 飞牛网关模式（Unix socket，由 fnOS 网关反代）
./psu-monitor-go serve --socket /path/psu-monitor.sock \
  --web-root ui/static --config gpio.json --pid psu-monitor.pid --log psu-monitor.log
# 耗电统计文件默认与 --config 同目录 (energy.json)，也可用 --energy /path/energy.json 指定
```

## API
| 方法 | 路径 | 说明 |
|---|---|---|
| GET  | /api/status  | 电源参数 JSON（VIN/VOUT/IOUT/PIN/POUT/温度/转速/状态/功率因数） |
| GET  | /api/bus     | 总线与地址 |
| GET  | /api/energy  | 耗电统计（累计/今日/本月/按日明细/实时功率） |
| POST | /api/energy  | {"action":"reset"} 耗电统计清零 |
| GET  | /api/log     | 运行日志下载 |
| GET  | /api/config  | 当前 GPIO 配置/连通性 |
| POST | /api/fan     | {mode: manual/auto, pct} 手动占比范围 10-100% |
| POST | /api/config  | {scl_pin, sda_pin} |
| POST | /api/detect  | 自动探测 PSCL/PSDA |

前端访问时网关前缀为 /app/psu-monitor（由程序自动剥离）。

## 权限
需要 root 权限访问 /dev/port（GPIO bitbang 直接 inb/outb）。
