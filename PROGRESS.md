# 项目进度档案（CRPS 电源监控）

> 本档案是整个项目的进度快照，供重装系统/换机后恢复上下文使用。
> Trae 记忆原始文件备份在 [backup/trae-memory/](backup/trae-memory/)，恢复方法见文末。

## 项目是什么

光宝 LiteOn PS-2551-5L CRPS 550W 电源的 PMBus 监控（电压/电流/功率/温度/风扇转速/状态字），
通过主板 F_PANEL GPIO 软件模拟 I2C（SCL=Pin19, SDA=Pin20, 地址 0x5B）读写，
含耗电统计（15s 采样矩形积分，按日明细保留 400 天）与风扇手动/自动控制（手动占比 10-100%）。

- **仓库**: https://github.com/yufeise/TAD6S4N10G-fnos（上游 powerguard 的 fork，CRPS 已合并为内置板块）
- **最终形态**: CRPS 融合进上游 tad-module 应用（同进程），fnOS 应用中心安装 fpk，主界面新增"🔌 CRPS 电源"标签页

## 当前线上状态（2026-10-16）

| 项 | 值 |
|---|---|
| NAS | 飞牛 fnOS 192.168.0.2（账号 yufeise，SSH 22 免密 key `~/.ssh/fnos_key`） |
| 运行应用 | tad-module（融合版，内含 CRPS 板块 v1.0.49 逻辑） |
| 应用目录 | `/usr/local/apps/@appcenter/tad-module/`（bin/tad-module + ui/static/） |
| CRPS 配置 | `/usr/local/apps/@appconf/tad-module/crps.json`（scl_pin/sda_pin） |
| 耗电数据 | `/usr/local/apps/@appconf/tad-module/energy.json`（5min 落盘） |
| API 入口 | unix socket `tad-module.sock`，CRPS 路由 `/api/crps/*`（内部改写为板块 /api/*，⚠️ 不能用 http.StripPrefix） |
| 旧独立版 | 已停用（bin/psu-monitor 改名 .disabled-1.0.48 防抢总线），数据已迁移 |

## 里程碑时间线

### 独立版阶段（v1.0.45 → v1.0.48，部署于 @appcenter/psu-monitor）
- **1.0.46** 耗电统计功能 + 前端面板（累计/今日/本月/14 日柱状图/清零）
- **1.0.47** 功能精简：删温控曲线、删两个诊断按钮及仅诊断用的 helpers（不影响其他功能）
- **1.0.48** 风扇手动占比范围收紧 10-100%（前端滑条 + 后端双重钳制）

### 融合阶段（git 仓库 + GitHub）
- git 初始化 → fork 仓库 `yufeise/TAD6S4N10G-fnos` → 恢复上游全树 → CRPS 合并为独立板块
  （`internal/crpsump/{log,portio,bus,energy,http}.go` + `backend/crpsump/` 独立入口 + UI）
- **v0.10.21→0.10.22** fpk 打包（manifest 升版 → git tag → GitHub Actions build-release.yml → Release 附件 → fnOS 应用中心手动安装）；修复 StripPrefix 路由 bug
- **v0.10.23** 日志弹窗标题修复；数据迁移（gpio/energy.json → @appconf/tad-module/）
- **v0.10.24** CRPS 页面风格统一（对齐主应用设计令牌/暗色模式联动/铺满布局）+ 状态快照缓存（后台 3s 预读，打开页面秒出数据）
- 后续修复（main 分支，未打 tag）：CSP 行内 style 失效弹窗、图表首帧变形、柱状图固定 14 天槽位排列、
  风扇切换响应 3s→0.2s（3s 等待挪后台）、连点合并不排队、按钮切换保护期防跳

## 本机工具链（重装后需重建）

| 工具 | 位置/安装方式 |
|---|---|
| Go 1.23.4 | `%TEMP%\gotc\go\bin\go.exe`（zip 解压自阿里云镜像，需重下）；上游 go.mod 要求 1.25，GOTOOLCHAIN 自动下载，配 `GOPROXY=https://goproxy.cn,direct` |
| git 2.55 | winget install Git.Git → `C:\Program Files\Git\cmd\git.exe` |
| GitHub 认证 | GCM OAuth 已存 wincred（重装后首次 push 会再弹浏览器授权一次） |
| SSH key | `~/.ssh/fnos_key`（ed25519，需重建并放回 NAS authorized_keys） |
| fnpack | fnOS fpk 打包工具（CI 内使用，本地不必装） |

## 常用流程

**改代码 → 部署**（详见 [backend/crpsump/README.md](backend/crpsump/README.md) 部署一节）：
1. 改 `internal/crpsump/`（业务）或 `app/ui/static/crps/`（融合版 UI）；UI 改动需同步 `backend/crpsump/ui/static/`
2. 交叉编译: `$env:GOOS="linux"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"; go build -trimpath -ldflags "-s -w" -o out ./backend/powerguard`
3. scp 到 NAS `/tmp/deploy/` → 参考 `deploy/install_burst.sh`（备份→install→pkill→restart trim_app_center）
4. 等 45s+ → `deploy/verify_burst.sh` 风格脚本验证（socket 直调 API）
5. 提交推送；发版才需要：升 manifest version → tag v* → CI 构建 fpk → Release → 应用中心手动上传安装

**经验教训**（踩过的坑）:
- PowerShell 向 ssh 传参会本地展开 `$()`、引号嵌套易碎 → 一律写 .sh 脚本 scp 上去执行；sudo bash -c 内的变量用单引号包（`\$SOCK` 转义会失效）
- `http.StripPrefix` 剥掉 /api/crps 后剩 /status，板块期望 /api/status → 404，必须 handler 内手动改写路由
- fnOS 应用中心拉起有 ~1 分钟延迟，重启后验证先等 45s+
- scp 偶发 "dest open Failure"，mkdir -p 后重传即可
- CSP `style-src 'self'` 会忽略 HTML 行内 style（初始隐藏要用 class）
- 验证脚本不要写 `rm -rf /tmp/deploy` 自毁目录

## 重装系统后恢复指南

> **最省事路径**: 直接解压本地备份 zip（如 `d:\crps电源控制-backup-2026-10-17.zip`，
> 每次重要改动后重新打包一次）——它 = 本仓库完整内容 + git 全历史 + Trae 记忆 + **对话历史数据库**。
> 恢复只做下面 2 个"放回"动作，不用花积分让 AI 重新读代码理解项目。

1. **代码**: 解压 zip 即得全部（或 `git clone https://github.com/yufeise/TAD6S4N10G-fnos.git`）
2. **Trae 记忆导入**: 把 `backup/trae-memory/` 下的内容放回
   `c:\Users\web\.trae-cn\memory\projects\-d-crps------p2-0e1e6b334b9462401340\`
   （project_memory.md 放根目录，20261014/20261015 日期目录原样放回）→ 新会话自动带上项目上下文
3. **对话历史导入**: 关闭 Trae → 把 `backup/trae-chatdb/` 里的 database.db / .db-shm / .db-wal
   三个文件放回 `C:\Users\web\AppData\Roaming\Trae CN\ModularData\ai-agent\`（覆盖）→ 启动 Trae，
   历史会话全部回来。⚠️ 放回的库是备份时点的快照，之后的对话不在里面；此目录 git 已忽略（25MB+ 二进制）
4. **工具链**: 按上表重装 Go/git，重建 SSH key（公钥追加到 NAS `~/.ssh/authorized_keys`）
5. **NAS 侧不受影响**（除非同时重装 fnOS）: 应用与数据都在 NAS 上；
   若 fnOS 重装：应用中心装最新 Release fpk → 把 `backup/` 里导出的 crps.json/energy.json
   恢复到 `/usr/local/apps/@appconf/tad-module/` → 重启 trim_app_center

**备份维护**: 每次重要改动后说一句"更新备份"即可——刷新记忆文件拷贝 + 对话库拷贝 + 重新打 zip + 推 GitHub。
