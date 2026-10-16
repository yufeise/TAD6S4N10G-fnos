# 项目备忘：CRPS 电源监控 (psu-monitor-go)

## 当前形态：CRPS 融合进 tad-module 主应用 (2026-10-15 上线, 仓库 v0.10.24)
- **融合架构**: 同进程 — backend/powerguard import internal/crpsump; 主 server 挂 `mux.Handle("/api/crps/", srv.APIHandler())`, APIHandler 内部把 /api/crps/<ep> 改写为板块原路由 /api/<ep> (⚠️ 不要用 http.StripPrefix: 剥掉 /api/crps 后剩 /status, 板块期望 /api/status, 会 404); crps 初始化在 initCrps() (backend/powerguard/main.go 尾部), 配置 @appconf/tad-module/crps.json (scl_pin/sda_pin), 耗电 @appconf/tad-module/energy.json, 失败仅降级不阻塞主功能
- **UI**: 主应用新增 "🔌 CRPS 电源" 标签页 (tab-crps/panel-crps, iframe 懒加载 data-src), 内嵌页面 = app/ui/static/crps/ 三件套 (fetch 相对路径 ../api/crps/); 独立版 backend/crpsump/ui/static/ 保留 (fetch "api/"), 两份文件改动需同步
- **fpk 发布流程**: manifest version 升版 → git tag v* → GitHub Actions (build-release.yml, fnpack 1.2.3) → Release 附件 tad-module.fpk → fnOS 应用中心手动上传安装 (fnpack 无 install 子命令, fpk 安装只能走网页)
- **旧 psu-monitor 已停用**: bin/psu-monitor 改名 .disabled-1.0.48 (防与融合版抢 GPIO 总线); 数据已迁移 @appconf/psu-monitor/{gpio,energy}.json → @appconf/tad-module/{crps,energy}.json; 旧目录保留未卸载
- NAS 实测 (tad-module.sock): /api/crps/status 正常 (LITEON PS-2551-5L), energy 历史无缝 (325Wh), 风扇钳制 pct=5→10, 主功能回归正常; 脚本 deploy/{verify,diag,install_fusion_fix,migrate_crps_data,finalize_fusion}.sh
- tad-module 应用目录无 manifest 文件 (fpk 安装后 manifest 收进系统数据库); bin 备份 bin/tad-module.bak-0.10.21
- fpk 升级重启只拉起升级的应用, 其他 app (psu-monitor) pkill 后不一定拉起
- **CRPS 性能优化**: 后端新增后台协程每 3 秒提前缓存电源状态快照, /api/crps/status 直接返回缓存避免实时总线读取延迟, 数据最大滞后 3 秒, 连续读取失败时自动回退实时读取
- **柱状图规则**: 固定按 14 天容量划分槽位, 柱子从左侧依次排列, 柱宽固定上限 44px, 避免随天数变化拉伸变形

## 环境与部署
- 用户飞牛 fnOS NAS: 192.168.0.2, x86_64, 主机名 yunas
- SSH: 端口 22 开放, 用户 yufeise (Administrators 组), sudo 可用; 本机已配免密钥 `~/.ssh/fnos_key` (ed25519, trae-deploy), hostkey SHA256:QsLTayV2sJpLKcEo86ssWRG8vYxhy6PfRuHbah6LqdE
- fnOS 网页: http://192.168.0.2:5666 强制跳 https://192.168.0.2:5667 (自签证书, 浏览器自动化无法过拦截页)
- 应用安装路径: /usr/local/apps/@appcenter/tad-module (融合版, bin/tad-module + ui/static/ 含 crps/ 子目录), 配置 @appconf/tad-module/config.json + crps.json + energy.json, 由 trim_app_center.service 托管 (pkill 后不会自动拉起, 需 `sudo systemctl restart trim_app_center` 重启全部应用, 拉起延迟 ~1 分钟)
- 电源: 光宝 PS-2551-5L CRPS 550W, PMBus addr 0x5B, GPIO bitbang SCL=Pin19 SDA=Pin20

## 已完成功能 (CRPS 板块 app_version 1.0.49)
- v1.0.48 风扇手动占比范围 10-100%: 前端滑条 min=10 + 后端 writeFanPct 与 /api/fan 双重钳制 (<10→10); 实测 POST pct=5 返回 pct=10
- v1.0.47 功能精简: 删除温控曲线(前端面板/导出/recordCurve//api/curve)、删除 GPIO 配置区两个诊断(写假温度测风扇 /api/temp-test、试0x3A恢复自动 /api/fan-test)及仅诊断用的 writeByte/readByteData/tempToLinear11/hexStr/pval; 风扇手动/自动(/api/fan 0x3B FAN_COMMAND_1)、状态、耗电统计、GPIO 配置/探测保留
- 耗电量统计: internal/crpsump/energy.go, 后台每 15s 采样 READ_PIN 矩形积分; 总量/今日/本月/按日明细(保留400天); energy.json 落盘 5min 周期 + 退出落盘; 重启 ≤10min gap 按断电前功率补计
- CRPS API (融合后经 /api/crps/ 前缀): GET /api/crps/{status,bus,energy,log,config}, POST /api/crps/{fan,config,detect,energy}
- 旧独立版历程: v1.0.45(安全加固) → 1.0.46(耗电统计) → 1.0.47(精简) → 1.0.48(风扇钳制), release/ 有留档
- 验证脚本 socket 路径: 融合版 /usr/local/apps/@appcenter/tad-module/tad-module.sock; 独立版(已停) @appcenter/psu-monitor/psu-monitor.sock; 验证脚本不要写 `rm -rf /tmp/deploy` 自毁目录; NAS scp 偶发 "dest open Failure", mkdir -p 后重传即可
- 部署注意: scp 上传多个本地文件到 /tmp/deploy/ 时 UI 三件套要与 install.sh 里路径一致; trim_app_center 重启后拉起应用有 ~1 分钟延迟, 验证前先等 45s+; 本机 PowerShell 不支持 `&&` 链接命令, 用 `;`
- **UI 统一**: CRPS 页面已对齐主应用设计令牌、配色、布局、暗色模式联动, 移除限宽居中限制改为左右铺满内容区, 修复 CSP 导致的日志弹窗自动弹出问题, 修复图表未就绪时拉伸变形问题

## Git 与 GitHub (2026-10-15 配置)
- 本地 git 2.55.0.5 (winget 安装, "$env:ProgramFiles\Git\cmd\git.exe"); 仓库初始化于 d:\crps电源控制, main 分支
- remote origin = https://github.com/yufeise/TAD6S4N10G-fnos.git (用户 fork 的上游 fnos 电源项目 powerguard)
- 仓库历史: 上游历史(62cd898, 版本0.10.20) + bba767b(简化版替换) + 1a3ddbd(恢复上游全树) + ec5e68b(CRPS 板块合并)
- **当前结构**: 用户的简化版已合并为上游的独立板块 — internal/crpsump/{log,portio,bus,energy,http}.go + backend/crpsump/{main.go,README.md,ui/static/} + build.sh 的 crps-module 目标; src/ 已删除; NAS 上部署的独立版 (v1.0.48) 与仓库解耦, 以后改代码要改 internal/crpsump + backend/crpsump 再从 backend/crpsump 构建
- 上游 go.mod 要求 go 1.25.0; 本地验证需 GOPROXY=https://goproxy.cn,direct (proxy.golang.org 不可达), GOTOOLCHAIN 自动下载新工具链
- 提交者: yufeise <yufeise@nas.local> (用 -c 参数, 未写入 config); GCM 已完成 GitHub OAuth 认证 (凭据已存 wincred, 无需再登录)
- GitHub 网络时好时坏, push 失败重试即可; 本机 PowerShell 不支持 `&&`
- git 处理中文文件名: PowerShell 里 git 输出的八进制转义会破坏 pathspec → 用 `git checkout <commit> -- .` 整树恢复或 git-bash -z 循环
- .gitignore 排除 release/ 二进制与 .trae-html-share-packages/; .gitattributes 强制 *.sh LF

## 工具链与经验
- 本机无 Go/Node/Python; Go 1.23.4 工具链在 %TEMP%\gotc\go\bin\go.exe (zip 在 %TEMP%\go1.23.4.windows-amd64.zip, 阿里云镜像 mirrors.aliyun.com/golang)
- 交叉编译: GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w"
- Windows PowerShell 调 ssh 远程命令时 `$()` 和反引号会被本地展开/引号嵌套易碎 → 一律写 .sh 脚本 scp 上去执行
- PowerShell 里 `\"` 转义会让 Get-Item 静默失败返回 0 (假象)
- NAS 上无 node 命令 (nodejs app 的 bin 路径未确认)
