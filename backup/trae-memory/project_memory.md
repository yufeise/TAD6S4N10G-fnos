# crps电源控制 项目记忆

## 备份流程 (backup.ps1) 故障与修复 (2026-10-17)
- "更新备份" = 运行 d:\crps电源控制\backup.ps1（五步：memory 同步/chatdb 快照/NAS json 快照/zip/git push）
- NAS 192.168.0.2 于 10-14 前后重装/更新（OpenSSH 9.2p1 Debian12），导致三连故障：
  1. Windows 端 known_hosts 缺失 → ssh 挂死在 host key 交互确认（非交互环境永不超时）；修复：ssh -o StrictHostKeyChecking=accept-new 记录新指纹
  2. NAS authorized_keys 被 trae-deploy 覆盖，备份钥匙 fnos_key(trae-backup) 认证被拒；修复：密码通道追加 fnos_key.pub
  3. PS5.1 在 $ErrorActionPreference='Stop' 下，native 命令 stderr（sudo 的 [sudo] password: 提示）会抛 NativeCommandError 中止脚本；修复：第 3 步用 Continue/Stop 包裹（脚本第 5 步同款解法）
- 诊断特征：backup 卡在第 3 步 + Get-Process ssh 有同命令行进程挂着 = host key 交互挂起
- askpass.bat（echo NAS 密码）位于 Temp，可能被清理，需重建
- fnos_key.pub 注释名 trae-backup；NAS 端另一把 trae-deploy 属其他部署流程，勿删

## 数据存储与架构
- 耗电统计数据使用纯 JSON 文件（energy.json）存储，无额外数据库依赖，400天明细封顶约24KB，全年占用空间极小，每5分钟整体重写一次，写入磨损可忽略
- 界面设计遵循简洁化原则：去除冗余灰色背景框、滚动条延伸至页面底部、参数信息采用单排紧凑布局，窄屏场景自动回退为两排避免溢出
- 电量柱状图默认展示近30天每日耗电数据，日期标签每5天标注一次，历史数据保留上限为400天
- 页面区块固定顺序：参数监控 → 耗电统计 → 风扇控制 → GPIO 引脚配置 → 电源信息（最底部），桌面端与手机端保持一致
- GPIO 引脚配置区桌面端单排紧凑：PSCL/PSDA 下拉框自适应宽度（min-width 96px）与三个操作按钮同行靠左；手机端两列字段+按钮独占一行三等分
- 风扇模式切换用分段控件（圆角灰底容器+主题色胶囊选中态）；转速与控件紧凑靠左同行，禁止两端 space-between 分散布局（用户明确嫌丑）
- 风扇滑条桌面端限宽 420px，松手（change 事件）才发指令；卡片顺序：功率因数在输入功率后，温度卡命名"进风口温度/出风口温度"
- 侧边栏标签统一使用插头造型的 SVG 图标，标签文字最终统一为「电源」，换图标只改 SVG 文件内容，文件名与引用不动，手机端布局零影响
- 风扇设置延迟优化链（2026-10-18）：后端 POST 成功后异步延后 1.2s 刷新快照（atomic.Bool 合并防连点），前端成功后 1.3s 立即 poll，转速显示延迟从最坏 ~6s 压到 ~1.5s
- 飞牛 App 内嵌 iframe 滚动问题修复：桌面端维持整页滚动，手机端（≤640px）固定 iframe 高度为可视区域高度，由 iframe 内部自行滚动，避免手势被 iframe 吃掉导致无法滑动
- CRPS 板块接入主应用采用轻量结构：板块暴露 New(cfg Config) *Server 和 Server.Handler() http.Handler 方法，主应用仅需一行调用 New 并挂载路由，禁止使用包级全局变量，需改为结构体方法实现以支持可测试性
- GPIO 引脚配置区在用户未保存有效配置前，仅显示配置区块本身，隐藏其余所有功能区块，避免无意义的空内容展示
- GPIO 自动探测功能按钮命名为「自动识别保存」，位于「保存并应用」按钮前方，识别成功后自动完成配置保存与持久化，识别失败时在配置区下方显示固定红字提示：检测信号失败，请重新接线保证正确连接信号线，并附带后端返回的 msg 方便排查
- detect 接口仅允许用户点击「自动识别保存」按钮触发（2026-10-21 作者复审合并阻塞项）：禁止任何后台自动扫描（tryAutoDetect/60s 退避已删除），因扫描会全组合改写 Pin19-22 方向并可能改写 crps.json；已配置但断电/拔线的恢复走 3s 状态轮询，不做自动重识别。回归测试在 app/ui/static/crps/app.test.js（node --test 运行，断言未配置页渲染与 63s 轮询 0 次 detect、点击恰好 1 次）
- 接线示意图入口按钮命名为「查看连线方法」，点击触发弹层展示背板接线参考图与说明文字，悬停触发逻辑已移除；按钮位于「保存并应用」右侧，桌面端弹层从按钮左缘展开，手机端弹层横跨整行、不遮挡后续内容
- 日志功能保留但移除界面入口，不再提供点击查看日志的交互
- 当用户已保存有效GPIO配置但临时拔线导致电源信号无法检测时，界面保留GPIO配置区块与原有针脚选择，在配置区下方显示固定红字提示「未检测到电源：请检查 PSCL/PSDA 接线，或点「自动识别保存」重新识别」，隐藏所有参数监控、耗电统计、风扇控制、电源信息区块；重新插线检测到信号后自动恢复全部功能区块，无需重新配置
- 电源状态判断需区分三分支：ok=true 为正常可读、ok=false+status_raw 为接线可读但电源故障、ok=null 为真正断线，避免将电源故障误引导为接线问题
- 自动探测清理失败需明确上报：未配置分支聚合失败针脚名返回给前端，已配置分支透传 switchPin 失败原因，不得静默吞错，失败后保留 driven 追踪供重试

## 工程约束
- 所有 CRPS 写接口（POST /api/crps/fan、/config、/detect、/energy reset）必须与宿主保持一致的管理员鉴权逻辑，使用 X-Trim-Isadmin 头校验，非管理员请求返回 403
- SuperIO /dev/port 访问必须与宿主现有 GPIO 访问共用全局锁，禁止并行执行 sioEnter/sioExit 操作，避免并发时交错进入配置空间导致硬件访问异常
- CRPS 模块仅在 configured==true（用户已完成首次保存或探测成功）时才执行 initDirs 写入 GPIO/SuperIO 方向寄存器，未配置状态下不得修改任何硬件引脚状态
- 所有 CRPS 接口的 JSON 请求体必须使用 LimitReader 限制大小，与宿主 configRequestMaxBytes 阈值保持一致
- 仓库惯例要求 powerguard 相关模块必须编写单元测试，硬件相关逻辑通过接口 mock 实现，无需依赖实机运行
- 手动风扇请求占空比≤10时，需按固件自动模式处理，避免与自动模式冲突
- 切换GPIO引脚前需释放不再使用的旧引脚，将其方向设为输入，避免残留输出
- 电源厂商信息读取失败时需记录10秒失败时间戳，TTL内直接返回缓存值，避免重复读取阻塞
- /api/log接口需使用O_NOFOLLOW拒绝符号链接，仅返回通用错误信息，不暴露原始错误详情
- 风扇占空比校验需同时限制上下限，确保响应回显值与实际写入值一致
- 风扇滑杆需添加aria-label可访问标注
- 标签页切换离开时暂停定时轮询，重新进入时立即刷新并恢复轮询
- 风扇设置请求失败时需清除lastSent缓存，确保用户重选同值时可重新发送请求
- 配置写盘失败提示需在loadGpioConfig刷新后保留，避免被读取结果覆盖
- CRPS模块初始化后需注册延迟保存，覆盖所有退出路径确保耗电统计落盘
- SuperIO 端口写失败需沿调用链向上传播，写入失败时不得更新方向缓存，reconfigure、读取、风扇写入接口需如实返回失败而非静默成功或仅记录日志
- HTTP 请求体为 null 或非法 JSON 时需返回 400 错误，请求体超限需返回 413 错误，不得按默认值静默执行硬件操作
- GPIO 全局锁需仅包裹单次端口读写操作，sleep 等等待期间需释放锁，避免阻塞其他硬件访问逻辑
- 总线初始化时若原始引脚名非法或两针相同，需将configuredPins设为false，避免垃圾配置文件启动时写入GPIO方向
- sioExit需通过defer注册，确保SuperIO配置模式中途失败时也能正常退出
- 耗电统计Reset操作后首个采样仅建立时间基准不积分，避免补记清零前时段数据
- 耗电统计Reset与内部save操作需返回错误，落盘失败时HTTP接口返回500状态码
- 日志接口请求需先检查响应ok状态，非管理员403需走错误提示而非直接显示JSON内容
- 方向寄存器索引写入与数据读取错误必须沿调用链向上传递，读取失败时禁止写回方向寄存器、禁止更新方向缓存
- 切针/初始化过程中需追踪每个实际驱动过的引脚，释放旧引脚失败时不得报告成功或遗忘旧引脚，初始化失败需安全回滚或保留可重试清理状态
- 自动探测接口需区分「找到接线」「已应用」「已保存」三种状态，配置应用或写盘失败需明确返回失败给前端，不得忽略错误仅记录日志
- 所有代码提交前需执行 gofmt -l 检查，确保注释与代码格式对齐，避免非必要的格式差异
- 未配置状态下的引脚清理操作必须持有总线互斥锁执行，避免与后台读总线产生竞态，清理完成后还原引脚字段为 orig，不得残留扫描搭档问题
- 构建发布工作流需包含 crps 前端代码语法检查与单元测试环节，确保提交的前端代码可正常运行
- SSH 密钥暴露后需立即轮换新密钥，移除旧密钥的授权访问并核查所有登录日志确认无异常使用
- PR 合并前需完成实机验证，确认所有评审意见全部处理完毕后再执行合并，可采用 squash 合并方式清理提交历史
- 交叉编译部署必须显式传入 `-X main.version=` 参数，否则二进制默认携带 `dev` 版本号会导致前端版本比较逻辑返回 null，触发更新检测失败

##  Lessons Learned
- 【部署 2026-10-21】NAS 安装目录已变为子目录结构：二进制在 /usr/local/apps/@appcenter/tad-module/bin/tad-module（旧备份同名目录 *.bak-*）；pkill+restart trim_app_center 后出现 APP_CRASH 事件属正常，进程拉起需 ~3 分钟（非 50-60s），socket 时间戳刷新即成功；curl 打 socket 需 sudo（socket 属 root:www-data），模块自身路由为 /api/crps/*（socket 直连无需去前缀）
- 【环境 2026-10-21】本机 go/node 工具链曾整体丢失：winget、go.dev、golang.google.cn 下载均不稳；可靠方案=阿里云镜像 mirrors.aliyun.com/golang 下载便携 zip（curl -C - 断点续传）+ tar 解压（Expand-Archive 对该 zip 会静默失败）；go.mod 要求 go1.25.0，须下 go1.25.0.windows-amd64.zip（约 67MB，Content-Length 以 HEAD 为准勿臆测）；node 走 registry.npmmirror.com/-/binary/node/；工具链装在 %TEMP% 会被清理，重装按此配方
- 电源厂商信息读取存在偶发噪声字节，需采用连续3次读取多数一致的校验逻辑，避免错误值被缓存放大
- 【严重教训 2026-10-18】编译部署 NAS 必须显式设 $env:GOOS="linux"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"，漏设会编出 Windows PE（MZ 头），安装后 tad-module 反复崩溃（日志特征："MZ? not found / Syntax error"），进程消失+socket 不存在。恢复流程：重交叉编译→scp→install→pkill→restart trim_app_center（拉起需 50~60s）
- 滑条 change 事件曾调用不存在的 postFan（重构改名 sendFan/requestFan 时漏改）导致手动调速静默失效——重构改名后必须全局 grep 旧函数名
- SSH 验证 NAS：需 $env:SSH_ASKPASS="$env:TEMP\askpass.bat"; $env:SSH_ASKPASS_REQUIRE="force"; $env:DISPLAY="localhost:0" 三件套； tad-module socket 实际路径 /usr/local/apps/@appcenter/tad-module/tad-module.sock（非 /run）；嵌套引号（ssh 'sh -c "..."'）在 PowerShell 双层转易丢参数，复杂命令拆成多条单引号语句执行
- GitHub 网络不通时可改用 GitHub Git Data API 逐对象重建提交，避免 orphan 分支无共同祖先导致 PR 无法重开的问题
- PowerShell 5.1 发送 GitHub API 请求时，直接传入字符串作为请求体会被按 ISO-8859-1 重编码导致中文乱码，需改用 UTF-8 字节数组作为请求体才能保证提交信息中文正常显示