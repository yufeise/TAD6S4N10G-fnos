# 计划：CRPS 板块融合进 TAD6S4N 模块（单应用 + fpk 部署）

## Context

用户 fork 的上游 fnOS 应用"TAD6S4N模块"（tad-module v0.10.20）已装在 NAS 并运行；同仓库的 CRPS PMBus 监控板块（internal/crpsump + backend/crpsump）目前作为独立应用 psu-monitor 部署。用户选择方案 C：真正融合——主应用单进程提供 CRPS API 与 UI，上游前端加 CRPS 标签页，打成一个 fpk，应用中心一键安装/升级。

架构决策：**同进程融合**（backend/powerguard import internal/crpsump），不做双进程/反代。理由：单一 fpk、单一进程、无跨进程权限/路径问题；crps 的 bus/energy 代码已是独立包，接入面小。

## 改动内容

### 1. internal/crpsump/http.go — 导出 API 适配器（最小改动）
- 新增导出方法 `func (s *AppServer) APIHandler() http.Handler`：返回一个 http.Handler，内部把 GET 分发给 doGet、POST 分发给 doPost（现有小写方法直接复用，不搬代码）。
- 融合模式下忽略 WebRoot（静态页由主 server 的 FileServer 服务）。

### 2. internal/powerguard/server.go — 挂载 CRPS 路由
- Server 增加可选字段 `Crps http.Handler`（nil 时跳过挂载，保持上游行为不变）。
- 在现有路由注册处（server.go:83-124 附近）追加：
  `mux.Handle("/api/crps/", http.StripPrefix("/api/crps", s.Crps))`
  路径链路：`/app/tad-module/api/crps/status` → BasePath 剥离 → `/api/crps/status` → StripPrefix → `/api/status` → crpsump doGet 原样工作。

### 3. backend/powerguard/main.go — serve 启动编排接入 crps
- 在 Manager 初始化后、`ListenAndServe` 前（main.go:131-220 区域）：
  - 读 crps 独立配置 `@appconf/tad-module/crps.json`（crpsump 现有读写逻辑不动，不改上游 Config 结构）
  - `crpsump.NewBus(scl, sda, configured)` + `crpsump.StartEnergy(bus, <crps.json 同目录>/energy.json)`
  - 失败仅记日志不阻塞主应用（`/api/crps/*` 仍挂载，返回 bus 不可用错误）
- crps 数据文件沿用独立文件：`crps.json` / `energy.json` 放在 `@appconf/tad-module/` 下。
- 日志：crpsump.Logf 维持写 `/tmp/psu-monitor.log`（自带轮转），暂不并入主日志。

### 4. 前端融合
- **静态挂载（零后端代码）**：crps 三件套放入 `app/ui/static/crps/`，主 server 的 `http.FileServer(WebRoot)` 自动服务 `/crps/`。
- **app/ui/static/index.html**：侧栏 `nav.app-tabs`（58-65 行）加 `<button id="tab-crps">CRPS 电源</button>`，body 加 `<section id="panel-crps" class="tab-panel"><iframe src="crps/"></iframe></section>`（iframe 懒加载：首次切换时再设 src，避免常驻轮询）。
- **app/ui/static/app.js**：按现有 tab 切换机制注册 tab-crps（沿用既有模式，找现有 tab 事件绑定处追加）。
- **backend/crpsump/ui/static/app.js（crps 前端）**：所有 `fetch("api/...")` 改为 `fetch("../api/crps/...")`（相对路径在 `/app/tad-module/crps/` 下解析为 `/app/tad-module/api/crps/...`，经 BasePath+StripPrefix 两级剥离后命中 crps API）。

### 5. 版本与打包
- `manifest`：version 0.10.20 → 0.10.21，changelog 加一行 CRPS 融合说明。
- `scripts/build.sh`：已构建 crps-module（独立入口保留不动）；需增加一步把 `backend/crpsump/ui/static` 复制进 `app/ui/static/crps/`（或直接改 app/ui/static/crps/ 为真源，backend/crpsump/ui 保留给独立入口——**采用后者**：两份静态文件，改动同步维护，避免打包脚本复杂化）。注意：crps 前端 fetch 路径改动需同步到两份 app.js。
- CI 已就绪（.github/workflows/build-release.yml：推 `v*` tag 自动下载 fnpack 1.2.3 构建 `tad-module.fpk` + sha256）。

### 6. 部署与数据迁移
1. 推 tag `v0.10.21` → GitHub Actions 产 fpk → 下载 artifact
2. fnOS 应用中心上传 fpk 安装/升级（覆盖 tad-module 0.10.20）
3. 数据迁移：`@appconf/psu-monitor/energy.json` 与 `gpio.json` 复制到 `@appconf/tad-module/`（保留耗电历史 289+ Wh）
4. unix socket 验证 `/api/crps/status`、`/api/crps/energy`、风扇钳制 pct=5→10
5. 验证通过后：旧 psu-monitor 应用先停止（不卸载，保留数据备份），UI 入口统一走 TAD6S4N 模块的 CRPS 标签页

## 不改的东西
- backend/crpsump 独立入口与 build.sh 的 crps-module 目标（单机独立使用场景保留）
- crpsump 的配置读写、能量统计、bus 逻辑（已按包拆好，直接复用）
- 上游 Config 结构（crps 配置走独立 crps.json）

## 验证
1. 本地：`go vet ./...` + 交叉编译 `go build ./backend/powerguard`；本地起 serve（TCP 模式）curl `/api/crps/status`（无 GPIO 硬件应返回 bus 错误而非 404）、`/crps/index.html` 200、主 UI `/` 200
2. GitHub：tag 触发 CI，确认 fpk artifact 产出
3. NAS：fpk 升级安装后 socket 验证（第 6 节第 4 步），浏览器验证 CRPS 标签页图表/风扇控制/耗电面板正常、主功能（温度/风扇/GPIO 面板）回归正常

## 风险
- 上游 app.js 的 tab 机制若非统一注册，改前端入口时需按其实际模式适配（实现时先读该段代码）
- fpk 升级会覆盖 tad-module 应用目录（cmd/main、ui）——因此本次全部改动进仓库随 fpk 发布，不手动改 NAS 上的文件
- 升级失败回退：应用中心重装上游 v0.10.20 官方 fpk 即可
