// psu-monitor-go: CRPS 电源 PMBus 监控服务 (主板 GPIO bitbang I2C) —— Go 版
//
// MIT License
//
// Copyright (c) 2026 psu-monitor contributors
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// 注意：风扇控制为实验性功能。错误的风扇占空比可能影响电源散热。请谨慎调节，
// 并确保电源温度在安全范围内。
//
// 功能与 Python 版 (psu-monitor-fpk/app-src/bin/psu-monitor, v1.0.43) 完全对齐：
//   - 通过主板 F_PANEL GPIO 用软件 bitbang I2C 读取光宝 PS-2551-5L 电源 PMBus
//       SCL = Pin19 (0xA01/bit2),  SDA = Pin20 (0xA05/bit0)
//       方向控制: SCL(Pin19) SuperIO LDN7 0xC9/bit2, SDA(Pin20) 0xCD/bit0 (0=输入/1=输出)
//   - 读取参数: VIN / VOUT / IOUT / PIN / POUT / 温度 / 风扇 / 状态字 / 厂商信息
//   - 控制风扇: FAN_COMMAND_1 (0x3B) 占空比写入 (带 CRC-8 PEC)
//   - 耗电量统计: 后台每 15s 采样 READ_PIN 矩形积分, 按总量/今日/本月/按日累计,
//       持久化 energy.json (默认与 --config 同目录, 可用 --energy 指定), 重启不丢
//   - HTTP API:
//       GET  /api/status           -> 电源参数 JSON
//       GET  /api/bus /api/log /api/config /api/energy
//       POST /api/fan /api/config /api/detect /api/energy (耗电统计清零)
//
// 用法: psu-monitor-go serve --addr 127.0.0.1 --port 3878 --web-root ui/static
//       psu-monitor-go serve --socket /path/psu-monitor.sock --web-root ui/static --config cfg.json --pid pid

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ---------------- 常量 ----------------

const (
	DefaultAddr   = 0x5b // 光宝 PS-2551-5L PMBus 从机地址
	LogPath       = "/tmp/psu-monitor.log"
	LogMaxSize    = 1 << 20                // 日志单文件上限 1MB, 超出轮转
	Delay         = 100 * time.Microsecond // bitbang 半周期
	AppVersion    = "1.0.48"
	MfrCacheTTL   = 60 // 秒
	DefaultSCL    = "Pin19"
	DefaultSDA    = "Pin20"
	GatewayPrefix = "/app/psu-monitor"
	ReadableTTL   = 2 // 秒: /api/config 的 readable 缓存, 避免轮询持续压总线
)

// ---- GPIO 引脚表 (Pin19~22, 全部实测可双向) ----
// 每针: Port=数据口, Bit=数据位, DirReg=方向寄存器(LDN7配置区), DirBit=方向位; 方向位 1=输出, 0=输入
type GPPin struct {
	Port   int
	Bit    int
	DirReg int
	DirBit int
}

var GPPins = map[string]GPPin{
	"Pin19": {Port: 0xA01, Bit: 2, DirReg: 0xC9, DirBit: 2}, // GPIO105
	"Pin20": {Port: 0xA05, Bit: 0, DirReg: 0xCD, DirBit: 0}, // GPIO106
	"Pin21": {Port: 0xA01, Bit: 1, DirReg: 0xC9, DirBit: 1}, // GPIO107
	"Pin22": {Port: 0xA05, Bit: 1, DirReg: 0xCD, DirBit: 1}, // GPIO108
}

var UIPins = []string{"Pin19", "Pin20", "Pin21", "Pin22"} // 界面可选 GPIO 针脚

var Cmd = map[string]byte{
	"CAPABILITY":    0x19,
	"VOUT_MODE":     0x20,
	"STATUS_WORD":   0x79,
	"READ_VIN":      0x88,
	"READ_IIN":      0x89,
	"READ_VOUT":     0x8b,
	"READ_IOUT":     0x8c,
	"READ_T1":       0x8d,
	"READ_T2":       0x8e,
	"READ_FAN1":     0x90,
	"READ_FAN2":     0x91,
	"READ_POUT":     0x96,
	"READ_PIN":      0x97,
	"FAN_COMMAND_1": 0x3b,
	"MFR_ID":        0x99,
	"MFR_MODEL":     0x9a,
	"MFR_REVISION":  0x9b,
}

// ---------------- 日志 (常开句柄 + O_NOFOLLOW + 按大小轮转) ----------------

var (
	logMu   sync.Mutex
	logFile *os.File
)

func ensureLogFile() *os.File {
	if logFile != nil {
		return logFile
	}
	// O_NOFOLLOW: 拒绝把日志写入符号链接目标(防多用户环境下被劫持)
	f, err := os.OpenFile(LogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY|syscall.O_NOFOLLOW, 0644)
	if err != nil {
		return nil
	}
	logFile = f
	return f
}

func logf(format string, a ...any) {
	line := fmt.Sprintf(format, a...)
	fmt.Println(line)
	logMu.Lock()
	defer logMu.Unlock()
	f := ensureLogFile()
	if f == nil {
		return
	}
	if st, err := f.Stat(); err == nil && st.Size() > LogMaxSize {
		f.Close()
		os.Rename(LogPath, LogPath+".1") // 轮转: 当前日志重命名为 .1
		logFile = nil
		f = ensureLogFile()
		if f == nil {
			return
		}
	}
	f.WriteString(line + "\n")
}

// ---------------- 端口 I/O (/dev/port 单字节, 进程级句柄常开) ----------------

var (
	portMu   sync.Mutex
	portFile *os.File
)

// portHandle 懒加载 /dev/port 句柄; 打开成功则进程级复用, 失败每次重试
func portHandle() (*os.File, error) {
	portMu.Lock()
	defer portMu.Unlock()
	if portFile != nil {
		return portFile, nil
	}
	f, err := os.OpenFile("/dev/port", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	portFile = f
	return f, nil
}

func inb(port int) byte {
	f, err := portHandle()
	if err != nil {
		f, err = os.OpenFile("/dev/port", os.O_RDWR, 0)
		if err != nil {
			return 0xFF
		}
		defer f.Close()
	}
	buf := []byte{0}
	if _, err := f.ReadAt(buf, int64(port)); err != nil {
		return 0xFF
	}
	return buf[0]
}

func outb(port int, v byte) {
	f, err := portHandle()
	if err != nil {
		f, err = os.OpenFile("/dev/port", os.O_RDWR, 0)
		if err != nil {
			return
		}
		defer f.Close()
	}
	f.WriteAt([]byte{v & 0xFF}, int64(port))
}

// ---------------- SuperIO LDN7 配置访问 ----------------

func sioEnter() { outb(0x2E, 0x87); outb(0x2E, 0x01); outb(0x2E, 0x55); outb(0x2E, 0x55) }
func sioExit()  { outb(0x2E, 0xAA) }
func sioRead(reg byte) byte {
	outb(0x2E, reg)
	return inb(0x2F)
}
func sioWrite(reg, v byte) {
	outb(0x2E, reg)
	outb(0x2F, v&0xFF)
}

// ---------------- PMBus bitbang 总线 ----------------
//
// 并发模型:
//   - mu        : 保护总线 I/O 时序(readWord/write*/busReset/bit 操作) 及引脚配置字段
//                 (sclName/sdaName/sclCfg/sdaCfg/ok)。任何访问底层 GPIO 的操作必须先持有 mu,
//                 保证切换引脚瞬间不会有正在进行的传输半途换针脚。
//   - stateMu   : 保护非 I/O 状态字段 (configured / fanMode / readable 缓存)。
//   - 方向缓存   : sclOut/sdaOut 记录 SIO 方向寄存器缓存, 只在方向真正变化时写 SIO,
//                 避免每次位操作都完整走一遍 enter/exit (项目惯例要求的性能优化)。

type Bus struct {
	addr       byte
	sclName    string
	sdaName    string
	sclCfg     GPPin
	sdaCfg     GPPin
	sclOut     bool // 方向缓存
	sdaOut     bool
	mu         sync.Mutex // 总线 I/O 与引脚配置
	stateMu    sync.Mutex // configured / fanMode / readable 缓存
	ok         bool
	configured bool // 是否已在界面设置好 PSCL/PSDA
	fanMode    string
	lastOK     bool // readable() TTL 缓存
	lastTs     int64
}

func NewBus(scl, sda string, configured bool) *Bus {
	// 校验引脚名: 非法名会令 GPPins 返回零值 GPPin{0,0,0,0}, 导致向 port 0x000(DMA 区) 写入
	if _, ok := GPPins[scl]; !ok {
		logf("警告: 非法 SCL 引脚 '%s', 回退默认 %s", scl, DefaultSCL)
		scl = DefaultSCL
	}
	if _, ok := GPPins[sda]; !ok {
		logf("警告: 非法 SDA 引脚 '%s', 回退默认 %s", sda, DefaultSDA)
		sda = DefaultSDA
	}
	if scl == sda {
		logf("警告: SCL/SDA 不能同一根针, SDA 回退默认 %s", DefaultSDA)
		sda = DefaultSDA
	}
	b := &Bus{
		addr:       DefaultAddr,
		sclName:    scl,
		sdaName:    sda,
		sclCfg:     GPPins[scl],
		sdaCfg:     GPPins[sda],
		configured: configured,
		fanMode:    "auto",
		ok:         true,
	}
	// 初始化: SCL=输出, SDA=输入(释放)
	b.initDirs()
	return b
}

// setPinDir 无条件写 SIO 方向寄存器 (被带缓存的 ensure* 调用)
func (b *Bus) setPinDir(cfg GPPin, out bool) {
	sioEnter()
	sioWrite(0x07, 0x07) // 选 LDN7 (GPIO)
	cur := sioRead(byte(cfg.DirReg))
	if out {
		cur |= 1 << uint(cfg.DirBit)
	} else {
		cur &^= 1 << uint(cfg.DirBit)
	}
	sioWrite(byte(cfg.DirReg), cur)
	sioExit()
}

// ensure* 只在方向缓存与实际不符时才写 SIO (方向寄存器缓存)
func (b *Bus) ensureSclOut() {
	if b.sclOut {
		return
	}
	b.setPinDir(b.sclCfg, true)
	b.sclOut = true
}
func (b *Bus) ensureSdaOut() {
	if b.sdaOut {
		return
	}
	b.setPinDir(b.sdaCfg, true)
	b.sdaOut = true
}
func (b *Bus) ensureSdaIn() {
	if !b.sdaOut {
		return
	}
	b.setPinDir(b.sdaCfg, false)
	b.sdaOut = false
}

func (b *Bus) initDirs() {
	b.setPinDir(b.sclCfg, true)
	b.sclOut = true
	b.setPinDir(b.sdaCfg, false)
	b.sdaOut = false
}

// ---- 状态字段访问 (stateMu) ----

func (b *Bus) isConfigured() bool { b.stateMu.Lock(); defer b.stateMu.Unlock(); return b.configured }
func (b *Bus) setConfigured(v bool) {
	b.stateMu.Lock()
	b.configured = v
	b.stateMu.Unlock()
}
func (b *Bus) fanModeNow() string { b.stateMu.Lock(); defer b.stateMu.Unlock(); return b.fanMode }
func (b *Bus) setFanMode(v string) {
	b.stateMu.Lock()
	b.fanMode = v
	b.stateMu.Unlock()
}

func (b *Bus) isOK() bool { b.mu.Lock(); defer b.mu.Unlock(); return b.ok }

// pinNames 加锁读当前引脚名 (配合 switchPin 在锁内改写)
func (b *Bus) pinNames() (string, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sclName, b.sdaName
}

// switchPin 切换 SCL/SDA 引脚并初始化方向, 不改变 configured 状态。
// 引脚配置字段在持锁状态下改写, 避免与进行中的 readWord 位操作产生数据竞争。
func (b *Bus) switchPin(scl, sda string) (bool, string) {
	sc, okS := GPPins[scl]
	sd, okD := GPPins[sda]
	if !okS {
		return false, "未知引脚: " + scl
	}
	if !okD {
		return false, "未知引脚: " + sda
	}
	if scl == sda {
		return false, "SCL 与 SDA 不能是同一根针"
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sclName = scl
	b.sclCfg = sc
	b.sdaName = sda
	b.sdaCfg = sd
	b.initDirs() // 重置方向缓存为新引脚
	b.ok = true
	return true, "ok"
}

// reconfigure 运行时切换 SCL/SDA 并标记为已配置(界面保存)
func (b *Bus) reconfigure(scl, sda *string) (bool, string) {
	ns, nd := b.pinNames()
	if scl != nil {
		ns = *scl
	}
	if sda != nil {
		nd = *sda
	}
	ok, msg := b.switchPin(ns, nd)
	if ok {
		b.setConfigured(true) // 界面设置成功 => GPIO 就绪
		b.busReset()          // 复位电源从机, 清掉之前探测可能留下的错误状态
	}
	return ok, msg
}

// autoDetect 自动扫描候选针脚组合, 找到电源实际接的 PSCL/PSDA (默认扫 Pin19~22)。
// 支持 ctx 取消; 中途取消会恢复探测前引脚。
func (b *Bus) autoDetect(ctx context.Context) (string, string) {
	origScl, origSda := b.pinNames()
	restore := func() {
		b.switchPin(origScl, origSda)
		b.busReset() // 探测结束复位, 让电源回到正常态
	}
	for _, scl := range UIPins {
		for _, sda := range UIPins {
			if scl == sda {
				continue
			}
			if !sleepCtx(ctx, 300*time.Millisecond) { // 等电源稳定, 不做总线复位(避免干扰读数)
				restore()
				return "", ""
			}
			if ok, _ := b.switchPin(scl, sda); !ok {
				continue
			}
			raw := b.readVinWithRetry(ctx, 3, 200*time.Millisecond)
			logf("  [detect] try %s/SCL + %s/SDA  VIN=%sV%s", scl, sda, ptrStr(linear11Ptr(raw)), rawStr(raw))
			if raw != nil { // 读到非 None 即命中(错误组合全是 None)
				b.busReset()
				return scl, sda
			}
		}
	}
	restore() // 未找到: 恢复探测前引脚
	return "", ""
}

// readVinWithRetry 读 VIN 并带 n 次重试; 供 autoDetect / readable 复用(去重重试逻辑)
func (b *Bus) readVinWithRetry(ctx context.Context, n int, d time.Duration) *uint16 {
	var raw *uint16
	for i := 0; i < n; i++ {
		raw = b.readWord(Cmd["READ_VIN"])
		if raw != nil {
			return raw
		}
		if i < n-1 && !sleepCtx(ctx, d) {
			return nil
		}
	}
	return raw
}

// readable 实际读一次电源, 判断当前接线是否真能读到数据(而非仅 GPIO 方向 OK)。
// 带 ReadableTTL 缓存: 缓存期内不重新读, 避免 /api/config 被前端轮询时持续压总线。
func (b *Bus) readable(ctx context.Context) bool {
	if !b.isConfigured() || !b.isOK() {
		return false
	}
	b.mu.Lock()
	now := time.Now().Unix()
	if now-b.lastTs < ReadableTTL {
		v := b.lastOK
		b.mu.Unlock()
		return v
	}
	b.mu.Unlock()
	ok := b.readVinWithRetry(ctx, 3, 100*time.Millisecond) != nil
	b.mu.Lock()
	b.lastOK = ok
	b.lastTs = time.Now().Unix()
	b.mu.Unlock()
	return ok
}

// sleepCtx 可取消的睡眠; 返回 false 表示 ctx 已取消
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		select {
		case <-ctx.Done():
			return false
		default:
			return true
		}
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// ---- bit 操作 (调用方须持 mu) ----

func (b *Bus) scl(v int) {
	cur := inb(b.sclCfg.Port)
	if v != 0 {
		cur |= 1 << uint(b.sclCfg.Bit)
	} else {
		cur &^= 1 << uint(b.sclCfg.Bit)
	}
	outb(b.sclCfg.Port, cur)
}

func (b *Bus) sdaWrite(v int) {
	b.ensureSdaOut()
	cur := inb(b.sdaCfg.Port)
	if v != 0 {
		cur |= 1 << uint(b.sdaCfg.Bit)
	} else {
		cur &^= 1 << uint(b.sdaCfg.Bit)
	}
	outb(b.sdaCfg.Port, cur)
}

func (b *Bus) sdaRead() int {
	b.ensureSdaIn()
	return int((inb(b.sdaCfg.Port) >> uint(b.sdaCfg.Bit)) & 1)
}

func (b *Bus) sdaRelease() { b.ensureSdaIn() }

// ---- I2C 时序 (调用方须持 mu) ----

func (b *Bus) i2cStart() {
	b.ensureSclOut()
	b.sdaWrite(1)
	b.scl(1)
	time.Sleep(Delay)
	b.sdaWrite(0)
	time.Sleep(Delay)
	b.scl(0)
	time.Sleep(Delay)
}

func (b *Bus) i2cStop() {
	b.sdaWrite(0)
	b.scl(1)
	time.Sleep(Delay)
	b.sdaRelease()
	time.Sleep(Delay)
}

// busReset SMBus 总线复位: 在当前 SCL 上发 >=9 个时钟脉冲, SDA 释放为高, 结束发 STOP。
// 持 mu 执行, 避免与并发事务的时钟交错产生总线垃圾时序。
func (b *Bus) busReset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ensureSclOut()
	b.ensureSdaIn()
	for i := 0; i < 9; i++ {
		b.scl(0)
		time.Sleep(Delay)
		b.scl(1)
		time.Sleep(Delay)
	}
	b.sdaWrite(0)
	b.scl(1)
	time.Sleep(Delay)
	b.sdaRelease()
	time.Sleep(Delay)
	time.Sleep(50 * time.Millisecond)
}

func (b *Bus) i2cWriteByte(v byte) bool {
	for i := 7; i >= 0; i-- {
		b.sdaWrite(int((v >> uint(i)) & 1))
		time.Sleep(Delay)
		b.scl(1)
		time.Sleep(Delay)
		b.scl(0)
		time.Sleep(Delay)
	}
	b.sdaRelease()
	time.Sleep(Delay)
	b.scl(1)
	time.Sleep(Delay)
	ack := 1 - b.sdaRead()
	b.scl(0)
	time.Sleep(Delay)
	return ack == 1
}

func (b *Bus) i2cReadByte(ack bool) int {
	val := 0
	for i := 7; i >= 0; i-- {
		b.scl(1)
		time.Sleep(Delay)
		val = (val << 1) | b.sdaRead()
		b.scl(0)
		time.Sleep(Delay)
	}
	if ack {
		b.sdaWrite(0)
	} else {
		b.sdaRelease()
	}
	time.Sleep(Delay)
	b.scl(1)
	time.Sleep(Delay)
	b.scl(0)
	time.Sleep(Delay)
	b.sdaRelease()
	return val
}

// ---- 事务 (自带加锁) ----

// readWord 读一个 word (从设备先发低字节), 返回 nil 表示失败
func (b *Bus) readWord(cmd byte) *uint16 {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.i2cStart()
	if !b.i2cWriteByte(b.addr << 1) {
		b.i2cStop()
		return nil
	}
	if !b.i2cWriteByte(cmd) {
		b.i2cStop()
		return nil
	}
	b.i2cStart()
	if !b.i2cWriteByte((b.addr << 1) | 1) {
		b.i2cStop()
		return nil
	}
	lsb := b.i2cReadByte(true)
	msb := b.i2cReadByte(false)
	b.i2cStop()
	v := uint16(msb<<8 | lsb)
	return &v
}

// readBlock block read (厂商信息), 返回 nil 表示失败
func (b *Bus) readBlock(cmd byte) []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.i2cStart()
	if !b.i2cWriteByte(b.addr << 1) {
		b.i2cStop()
		return nil
	}
	if !b.i2cWriteByte(cmd) {
		b.i2cStop()
		return nil
	}
	b.i2cStart()
	if !b.i2cWriteByte((b.addr << 1) | 1) {
		b.i2cStop()
		return nil
	}
	n := b.i2cReadByte(true) // 长度字节 (SMBus block read: 长度须 ACK)
	if n > 32 {
		b.i2cStop()
		return nil
	}
	data := make([]byte, 0, n)
	for i := 0; i < n; i++ {
		data = append(data, byte(b.i2cReadByte(i < n-1)))
	}
	b.i2cStop()
	return data
}

// readMfr 读 MFR 字符串。GPIO bitbang 慢时序下 block read 尾部偶有噪声字节。
// 待真机确认: 尾部噪声很可能是固件在 block read 末尾仍期待 PEC 字节所致(写路径启用了 PEC),
// 读路径标准 SMBus block read 无 PEC。当前用延时复位 + 重试取"最长可打印串"防御。
func (b *Bus) readMfr(cmd byte) string {
	best := ""
	for i := 0; i < 3; i++ {
		time.Sleep(200 * time.Millisecond)
		d := b.readBlock(cmd)
		if d == nil || len(d) == 0 {
			continue
		}
		var sb strings.Builder
		for _, c := range d {
			if c >= 32 && c < 127 {
				sb.WriteByte(c)
			}
		}
		s := strings.Trim(strings.Trim(sb.String(), "#"), " ")
		if len(s) > len(best) {
			best = s
		}
	}
	return best
}

// writeFanRaw 写 FAN_COMMAND_1 原始值 (带 PEC)。固件启用 PEC, 须附 CRC-8。失败重试 3 次
func (b *Bus) writeFanRaw(lsb, msb byte) bool {
	return b.writeWordCmd(Cmd["FAN_COMMAND_1"], lsb, msb)
}

// writeWordCmd 通用 SMBus Write Word (带 PEC)。失败重试 3 次
func (b *Bus) writeWordCmd(cmd, lsb, msb byte) bool {
	pec := crc8(b.addr<<1, []byte{cmd, lsb, msb})
	for t := 0; t < 3; t++ {
		b.mu.Lock()
		ok := true
		b.i2cStart()
		if !b.i2cWriteByte(b.addr << 1) {
			b.i2cStop()
			b.mu.Unlock()
			time.Sleep(50 * time.Millisecond)
			continue
		}
		for _, by := range []byte{cmd, lsb, msb, pec} {
			if !b.i2cWriteByte(by) {
				ok = false
				break
			}
		}
		b.i2cStop()
		b.mu.Unlock()
		if ok {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// ---- 风扇 ----

func (b *Bus) writeFanPct(pct int) bool {
	if pct < 10 { // 手动指定占比下限 10% (光宝固件 10% 以下实测无效)
		pct = 10
	}
	if pct > 100 {
		pct = 100
	}
	return b.writeFanRaw(byte(pct&0xFF), byte((pct>>8)&0xFF))
}

func (b *Bus) fanRpm() *float64 {
	raw := b.readWord(Cmd["READ_FAN1"])
	return linear11Ptr(raw)
}

func (b *Bus) readTemp1() *float64 {
	raw := b.readWord(Cmd["READ_T1"])
	return linear11Ptr(raw)
}

// setFan 设置风扇控制模式
//
//	manual -> FAN_COMMAND_1=pct, 手动指定占空比
//	auto   -> 恢复电源固件自动温控。实测光宝固件: FAN_COMMAND_1=10% 会触发释放控制、
//	          回到自动温控(约2700转低速档); 0 / 0xFFFF / 0x00FF 均实测无效。10% 即软切回触发值
func (b *Bus) setFan(ctx context.Context, mode string, pct int) bool {
	if mode == "manual" {
		ok := b.writeFanPct(pct)
		if ok {
			b.setFanMode("manual")
		}
		return ok
	}
	baseline := b.fanRpm()
	ok := b.writeFanPct(10)
	if !sleepCtx(ctx, 3*time.Second) {
		return ok
	}
	rpm := b.fanRpm()
	logf("切回自动: 写 FAN_COMMAND_1=10%% -> FAN=%s (手动基线=%s)", ptrStr(rpm), ptrStr(baseline))
	b.setFanMode("auto")
	return ok
}

// ---------------- 纯函数 ----------------

// crc8 SMBus CRC-8 (poly 0x07)
func crc8(addrByte byte, data []byte) byte {
	crc := byte(0)
	for _, b := range append([]byte{addrByte}, data...) {
		crc ^= b
		for i := 0; i < 8; i++ {
			if crc&0x80 != 0 {
				crc = (crc << 1) ^ 0x07
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// linear11 解码 PMBus linear11 word (5bit exp + 11bit 有符号尾数), 保留 2 位小数
func linear11(w uint16) float64 {
	exp := int((w >> 11) & 0x1F)
	if exp >= 16 {
		exp -= 32
	}
	mant := int(w & 0x7FF)
	if mant >= 1024 {
		mant -= 2048
	}
	return round2(float64(mant) * math.Pow(2, float64(exp)))
}

func linear11Ptr(w *uint16) *float64 {
	if w == nil {
		return nil
	}
	v := linear11(*w)
	return &v
}

// readVoutReal 用 VOUT_MODE(0x20) 定义的指数解码真实 VOUT (VOUT 为无符号)
func readVoutReal(b *Bus) *float64 {
	mode := b.readWord(Cmd["VOUT_MODE"])
	if mode != nil && (*mode&0x80) == 0 { // bit7=0 表示 Linear 模式
		exp := int(*mode & 0x1F)
		if exp >= 16 {
			exp -= 32
		}
		raw := b.readWord(Cmd["READ_VOUT"])
		if raw != nil {
			v := round2(float64(*raw) * math.Pow(2, float64(exp)))
			return &v
		}
	}
	return nil
}

// ---------------- 四舍五入 helpers (对齐 Python round 显示精度) ----------------

func round1(x float64) float64 { return math.Round(x*10) / 10 }
func round2(x float64) float64 { return math.Round(x*100) / 100 }
func round3(x float64) float64 { return math.Round(x*1000) / 1000 }

func fval(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}
func ptrStr(p *float64) string {
	if p == nil {
		return "None"
	}
	return fmt.Sprintf("%g", *p)
}
func rawStr(raw *uint16) string {
	if raw == nil {
		return ""
	}
	return fmt.Sprintf(" (raw=0x%04X)", *raw)
}

// ---------------- 全局状态 ----------------

var (
	mfrMu    sync.Mutex
	mfrID    string
	mfrModel string
	mfrTs    int64
)

func getMfr(b *Bus) (string, string) {
	now := time.Now().Unix()
	mfrMu.Lock()
	defer mfrMu.Unlock()
	if mfrTs > 0 && now-mfrTs < MfrCacheTTL {
		return mfrID, mfrModel
	}
	id := b.readMfr(Cmd["MFR_ID"])
	model := b.readMfr(Cmd["MFR_MODEL"])
	if id != "" || model != "" {
		if id != "" {
			mfrID = id
		}
		if model != "" {
			mfrModel = model
		}
		mfrTs = now
	}
	return mfrID, mfrModel
}

// ---------------- 耗电量统计 ----------------
//
// 模型: 后台每 15s 采样一次 READ_PIN, 用矩形积分 (功率×时长) 累计耗电量,
// 按 "总量 / 今日 / 本月 / 按日明细" 四个维度累计, 周期落盘 energy.json, 重启不丢。
// 单次采样间隔超过 EnergyMaxGap (关机/挂起/长时间读不到) 不计入, 避免把中断时间算成耗电;
// 服务重启后若距上次落盘 <= EnergyMaxGap, 按断电前功率补计这段时间 (机器确实在耗电)。

const (
	EnergySampleInterval = 15 * time.Second // PIN 采样周期
	EnergySaveInterval   = 5 * time.Minute  // 落盘周期
	EnergyMaxGap         = 10 * time.Minute // 采样间隔上限: 超出视为中断, 不计入
	EnergyDaysKeep       = 400              // 按日明细保留天数 (~13 个月)
)

type EnergyStats struct {
	mu        sync.Mutex
	path      string
	totalWh   float64
	sessionWh float64            // 本次运行累计
	firstTs   int64              // 统计起始时间
	updatedTs int64              // 最后一次累计时间
	lastPin   float64            // 最近一次有效采样功率 (W)
	hasPin    bool               // 是否已有有效功率采样
	days      map[string]float64 // "2006-01-02" -> Wh
	months    map[string]float64 // "2006-01"    -> Wh
	dirty     bool
}

var energyStats *EnergyStats // 全局单例, main 里初始化; readAll 里刷新 lastPin

type energyFile struct {
	TotalWh   float64            `json:"total_wh"`
	FirstTs   int64              `json:"first_ts"`
	UpdatedTs int64              `json:"updated_ts"`
	LastPinW  float64            `json:"last_pin_w"`
	HasPin    bool               `json:"has_pin"`
	Days      map[string]float64 `json:"days"`
	Months    map[string]float64 `json:"months"`
}

func NewEnergy(path string) *EnergyStats {
	e := &EnergyStats{
		path:   path,
		days:   map[string]float64{},
		months: map[string]float64{},
	}
	e.load()
	return e
}

func (e *EnergyStats) load() {
	data, err := os.ReadFile(e.path)
	if err != nil {
		return
	}
	var f energyFile
	if json.Unmarshal(data, &f) != nil || f.Days == nil || f.Months == nil {
		logf("警告: 耗电统计文件损坏, 忽略并重新累计: %s", e.path)
		return
	}
	e.totalWh = f.TotalWh
	e.firstTs = f.FirstTs
	e.updatedTs = f.UpdatedTs
	e.lastPin = f.LastPinW
	e.hasPin = f.HasPin
	e.days = f.Days
	e.months = f.Months
	if e.days == nil {
		e.days = map[string]float64{}
	}
	if e.months == nil {
		e.months = map[string]float64{}
	}
	// 中断补计: 距上次落盘 <= EnergyMaxGap 视为运行中断(如升级重启), 按断电前功率补上;
	// 超过上限视为整机关机过, 不补。
	if e.hasPin && e.updatedTs > 0 {
		gap := time.Since(time.Unix(e.updatedTs, 0))
		if gap > 0 && gap <= EnergyMaxGap {
			e.addWh(e.lastPin*gap.Hours(), time.Now())
			logf("耗电统计: 补计中断期间 %.1f 分钟 ≈ %.4f kWh", gap.Minutes(), e.lastPin*gap.Hours()/1000)
		}
	}
	logf("耗电统计已载入: 累计 %.3f kWh, 按日 %d 天 (%s)", e.totalWh/1000, len(e.days), e.path)
}

// addWh 把 wh 累计进总量/当日/当月 (调用方须持 mu)
func (e *EnergyStats) addWh(wh float64, now time.Time) {
	if wh <= 0 {
		return
	}
	e.totalWh += wh
	e.sessionWh += wh
	if e.firstTs == 0 {
		e.firstTs = now.Unix()
	}
	e.updatedTs = now.Unix()
	e.days[now.Format("2006-01-02")] += wh
	e.months[now.Format("2006-01")] += wh
	for len(e.days) > EnergyDaysKeep { // 按日明细超限: 删最旧
		oldest := ""
		for k := range e.days {
			if oldest == "" || k < oldest {
				oldest = k
			}
		}
		delete(e.days, oldest)
	}
	e.dirty = true
}

// Sample 记录一次功率采样: 用距上次采样的时长做矩形积分
func (e *EnergyStats) Sample(pin float64, dt time.Duration) {
	now := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lastPin = pin
	e.hasPin = true
	if dt > 0 && dt <= EnergyMaxGap {
		e.addWh(pin*dt.Hours(), now)
	} else {
		e.updatedTs = now.Unix() // 间隔异常: 只刷新时间基准, 不计电量
	}
}

// UpdatePin 只刷新最近功率显示 (readAll 每次成功读 PIN 时调用, 不积分)
func (e *EnergyStats) UpdatePin(pin float64) {
	e.mu.Lock()
	e.lastPin = pin
	e.hasPin = true
	e.mu.Unlock()
}

// Snapshot 生成 /api/energy JSON
func (e *EnergyStats) Snapshot() map[string]any {
	now := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()
	dayKeys := make([]string, 0, len(e.days))
	for k := range e.days {
		dayKeys = append(dayKeys, k)
	}
	sort.Strings(dayKeys)
	if len(dayKeys) > 30 {
		dayKeys = dayKeys[len(dayKeys)-30:]
	}
	days := make([]map[string]any, 0, len(dayKeys))
	for _, k := range dayKeys {
		days = append(days, map[string]any{"date": k, "wh": round3(e.days[k])})
	}
	var pin any
	if e.hasPin {
		pin = round1(e.lastPin)
	}
	return map[string]any{
		"ok":         true,
		"total_wh":   round3(e.totalWh),
		"today_wh":   round3(e.days[now.Format("2006-01-02")]),
		"month_wh":   round3(e.months[now.Format("2006-01")]),
		"session_wh": round3(e.sessionWh),
		"pin_w":      pin,
		"days":       days,
		"first_ts":   e.firstTs,
		"updated_ts": e.updatedTs,
		"ts":         now.Unix(),
		"app_version": AppVersion,
	}
}

// Reset 统计清零 (总量/本次运行/按日/按月全部归零)
func (e *EnergyStats) Reset() {
	e.mu.Lock()
	e.totalWh, e.sessionWh = 0, 0
	e.firstTs, e.updatedTs = 0, 0
	e.lastPin, e.hasPin = 0, false
	e.days = map[string]float64{}
	e.months = map[string]float64{}
	e.dirty = true
	e.mu.Unlock()
	e.Save()
	logf("耗电统计已清零")
}

// Save 落盘: 临时文件 + rename 原子替换, 防写一半损坏
func (e *EnergyStats) Save() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.dirty {
		return
	}
	f := energyFile{
		TotalWh: e.totalWh, FirstTs: e.firstTs, UpdatedTs: e.updatedTs,
		LastPinW: e.lastPin, HasPin: e.hasPin,
		Days: e.days, Months: e.months,
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return
	}
	tmp := e.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		logf("耗电统计保存失败: %v", err)
		return
	}
	if err := os.Rename(tmp, e.path); err != nil {
		logf("耗电统计保存失败: %v", err)
		return
	}
	e.dirty = false
}

// energySampler 后台采样循环: 每 EnergySampleInterval 读一次 PIN 并积分, 周期落盘。
// 与前端轮询共用总线 (mutex 串行化), 每周期只多 1 次 readWord, 开销可忽略。
func energySampler(bus *Bus) {
	last := time.Now()
	save := time.NewTicker(EnergySaveInterval)
	defer save.Stop()
	logf("耗电统计采样已启动: 每 %s 采样 READ_PIN, 每 %s 落盘", EnergySampleInterval, EnergySaveInterval)
	for {
		select {
		case <-save.C:
			energyStats.Save()
		case <-time.After(EnergySampleInterval):
		}
		now := time.Now()
		dt := now.Sub(last)
		last = now
		if !bus.isConfigured() || !bus.isOK() {
			continue
		}
		if pin := linear11Ptr(bus.readWord(Cmd["READ_PIN"])); pin != nil && *pin >= 0 && *pin < 10000 {
			energyStats.Sample(*pin, dt)
		}
	}
}


// ---------------- 参数读取 ----------------

func emptyStatus(err string) map[string]any {
	return map[string]any{
		"ok": false, "error": err,
		"vin": nil, "vout": nil, "iout": nil, "pin": nil, "pout": nil,
		"iin": nil, "iin_calc": nil, "pf": nil,
		"efficiency": nil, "temp1": nil, "temp2": nil, "fan": nil,
		"status_raw": nil, "bus": "gpio", "addr": "0x5b",
		"ts":          time.Now().Unix(),
		"app_version": AppVersion,
		"mfr_id":      "", "mfr_model": "", "mfr_rev": "",
	}
}

func readAll(b *Bus) map[string]any {
	if b == nil {
		return emptyStatus("no-gpio")
	}
	if !b.isConfigured() {
		// 尚未在界面设置 PSCL/PSDA: 不读取电源参数
		return emptyStatus("gpio-not-configured")
	}
	if !b.isOK() {
		return emptyStatus("no-gpio")
	}
	sw := b.readWord(Cmd["STATUS_WORD"])
	var statusRaw any
	var okFlag any
	if sw != nil {
		statusRaw = fmt.Sprintf("%04X", *sw)
		okFlag = *sw == 0x0000
	}
	vin := linear11Ptr(b.readWord(Cmd["READ_VIN"]))
	iin := linear11Ptr(b.readWord(Cmd["READ_IIN"]))
	vout := readVoutReal(b) // 用 VOUT_MODE 指数解码
	iout := linear11Ptr(b.readWord(Cmd["READ_IOUT"]))
	pin := linear11Ptr(b.readWord(Cmd["READ_PIN"]))
	pout := linear11Ptr(b.readWord(Cmd["READ_POUT"]))
	t1 := linear11Ptr(b.readWord(Cmd["READ_T1"]))
	t2 := linear11Ptr(b.readWord(Cmd["READ_T2"]))
	fan := linear11Ptr(b.readWord(Cmd["READ_FAN1"]))

	var eff any
	if pin != nil && pout != nil && *pin > 0 {
		eff = round1(*pout / *pin * 100)
	}
	var calcVout any
	if pout != nil && iout != nil && *iout > 0 {
		calcVout = round2(*pout / *iout)
	}
	var iinCalc any
	if pin != nil && vin != nil && *vin > 0 {
		iinCalc = round2(*pin / *vin)
	}
	// 功率因数: PF = 有功功率 PIN / (VIN × IIN)。交流输入下 PF<1(含无功/谐波分量)
	var pf any
	if pin != nil && vin != nil && iin != nil && *vin > 0 && *iin > 0 {
		pf = round3(*pin / (*vin * *iin))
	}

	if energyStats != nil && pin != nil && *pin >= 0 {
		energyStats.UpdatePin(*pin) // 刷新实时功率显示 (积分由后台采样器负责)
	}

	mfrID, mfrModel := getMfr(b)
	var voutFinal any = fval(vout)
	if voutFinal == nil {
		voutFinal = calcVout
	}
	return map[string]any{
		"ok": okFlag, "status_raw": statusRaw,
		"vin": fval(vin), "vout": voutFinal, "vout_calc": calcVout,
		"iout": fval(iout), "iin": fval(iin), "iin_calc": iinCalc,
		"pin": fval(pin), "pout": fval(pout),
		"efficiency": eff, "pf": pf,
		"temp1": fval(t1), "temp2": fval(t2), "fan": fval(fan),
		"mfr_id": mfrID, "mfr_model": mfrModel,
		"fan_mode":    b.fanModeNow(),
		"bus":         "gpio",
		"addr":        fmt.Sprintf("0x%02X", b.addr),
		"ts":          time.Now().Unix(),
		"app_version": AppVersion,
	}
}

// ---------------- HTTP 服务 ----------------

type appServer struct {
	bus        *Bus
	webRoot    string
	configPath string
}

var mimeTypes = map[string]string{
	".html": "text/html; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".js":   "application/javascript; charset=utf-8",
	".png":  "image/png",
	".svg":  "image/svg+xml",
}

func (s *appServer) stripGateway(path string) string {
	if strings.HasPrefix(path, GatewayPrefix) {
		path = path[len(GatewayPrefix):]
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}
	if path == "" {
		return "/"
	}
	return path
}

func (s *appServer) send(w http.ResponseWriter, code int, body any, ctype string) {
	var data []byte
	switch v := body.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		data, _ = json.Marshal(body)
		if ctype == "" {
			ctype = "application/json"
		}
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(code)
	w.Write(data)
}

func (s *appServer) serveStatic(w http.ResponseWriter, path string) {
	if path == "/" || path == "" {
		path = "/index.html"
	}
	root, _ := filepath.Abs(s.webRoot)
	full, err := filepath.Abs(filepath.Join(s.webRoot, strings.TrimPrefix(path, "/")))
	if err != nil || !strings.HasPrefix(full, root+string(filepath.Separator)) {
		s.send(w, 404, "Not Found", "text/plain; charset=utf-8")
		return
	}
	if info, err := os.Stat(full); err != nil || info.IsDir() {
		s.send(w, 404, "Not Found", "text/plain; charset=utf-8")
		return
	}
	data, err := os.ReadFile(full)
	if err != nil {
		s.send(w, 404, "Not Found", "text/plain; charset=utf-8")
		return
	}
	ctype := mimeTypes[strings.ToLower(filepath.Ext(full))]
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	s.send(w, 200, data, ctype)
}

func (s *appServer) doGet(w http.ResponseWriter, r *http.Request, p string) {
	ctx := r.Context()
	switch p {
	case "/api/status":
		s.send(w, 200, readAll(s.bus), "application/json")
	case "/api/bus":
		addr := "0x5b"
		if s.bus != nil {
			addr = fmt.Sprintf("0x%02X", s.bus.addr)
		}
		s.send(w, 200, map[string]any{"bus": "gpio", "addr": addr}, "application/json")
	case "/api/energy":
		if energyStats != nil {
			s.send(w, 200, energyStats.Snapshot(), "application/json")
		} else {
			s.send(w, 200, map[string]any{"ok": false, "error": "no-energy"}, "application/json")
		}
	case "/api/log":
		// 下载运行日志
		if _, err := os.Stat(LogPath); err == nil {
			body, err := os.ReadFile(LogPath)
			if err != nil {
				s.send(w, 500, map[string]any{"error": err.Error()}, "application/json")
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="psu-monitor.log"`)
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.WriteHeader(200)
			w.Write(body)
		} else {
			s.send(w, 200, "日志文件不存在(尚无输出)", "text/plain; charset=utf-8")
		}
	case "/api/config":
		if s.bus != nil {
			scl, sda := s.bus.pinNames()
			// readable 带 TTL 缓存, 前端轮询 config 不会持续压总线
			s.send(w, 200, map[string]any{
				"ok":          true,
				"scl_pin":     scl,
				"sda_pin":     sda,
				"pins":        UIPins,
				"connected":   s.bus.isOK(),        // 仅 GPIO 方向是否 OK
				"readable":    s.bus.readable(ctx), // 真实能否读到电源数据 (TTL 缓存)
				"configured":  s.bus.isConfigured(),
				"app_version": AppVersion,
			}, "application/json")
		} else {
			s.send(w, 200, map[string]any{"ok": false, "error": "no-gpio"}, "application/json")
		}
	default:
		if strings.HasPrefix(p, "/api/") {
			s.send(w, 404, map[string]any{"error": "not found"}, "application/json")
		} else {
			s.serveStatic(w, p)
		}
	}
}

func readJSONBody(r *http.Request) map[string]any {
	data, _ := io.ReadAll(r.Body)
	m := map[string]any{}
	_ = json.Unmarshal(data, &m)
	return m
}

func (s *appServer) doPost(w http.ResponseWriter, r *http.Request, p string) {
	ctx := r.Context()
	switch p {
	case "/api/fan":
		if s.bus == nil || !s.bus.isConfigured() {
			s.send(w, 200, map[string]any{"ok": false, "error": "gpio-not-configured"}, "application/json")
			return
		}
		if !s.bus.isOK() {
			s.send(w, 200, map[string]any{"ok": false, "error": "no-gpio"}, "application/json")
			return
		}
		body := readJSONBody(r)
		mode := "manual"
		if m, ok := body["mode"].(string); ok {
			mode = m
		}
		pct := 10
		if v, ok := body["pct"].(float64); ok {
			pct = int(v)
		} else if v, ok := body["percent"].(float64); ok {
			pct = int(v)
		}
		if pct < 10 { // 手动指定占比范围 10-100%
			pct = 10
		}
		ok := s.bus.setFan(ctx, mode, pct)
		s.send(w, 200, map[string]any{"ok": ok, "mode": s.bus.fanModeNow(), "pct": pct}, "application/json")
	case "/api/config":
		if s.bus == nil {
			s.send(w, 200, map[string]any{"ok": false, "error": "no-gpio"}, "application/json")
			return
		}
		body := readJSONBody(r)
		var sclPtr, sdaPtr *string
		if v, ok := body["scl_pin"].(string); ok {
			sclPtr = &v
		}
		if v, ok := body["sda_pin"].(string); ok {
			sdaPtr = &v
		}
		ok, msg := s.bus.reconfigure(sclPtr, sdaPtr)
		if ok && s.configPath != "" {
			scl, sda := s.bus.pinNames()
			cfg := map[string]string{"scl_pin": scl, "sda_pin": sda}
			if data, err := json.MarshalIndent(cfg, "", "  "); err == nil {
				if err := os.WriteFile(s.configPath, data, 0644); err != nil {
					msg = "已应用, 但配置保存失败: " + err.Error()
				}
			}
		}
		scl, sda := s.bus.pinNames()
		s.send(w, 200, map[string]any{
			"ok": ok, "msg": msg,
			"scl_pin": scl, "sda_pin": sda,
		}, "application/json")
	case "/api/detect":
		if s.bus == nil {
			s.send(w, 200, map[string]any{"ok": false, "error": "no-gpio"}, "application/json")
			return
		}
		scl, sda := s.bus.autoDetect(ctx)
		if scl != "" {
			ok, _ := s.bus.reconfigure(&scl, &sda)
			if ok && s.configPath != "" {
				cs, cd := s.bus.pinNames()
				cfg := map[string]string{"scl_pin": cs, "sda_pin": cd}
				if data, err := json.MarshalIndent(cfg, "", "  "); err == nil {
					if err := os.WriteFile(s.configPath, data, 0644); err != nil {
						logf("已探测到, 但配置保存失败: %v", err)
					}
				}
			}
			s.send(w, 200, map[string]any{
				"ok": true, "found": true,
				"scl_pin": scl, "sda_pin": sda,
				"msg": "自动探测到接线 PSCL=" + scl + " PSDA=" + sda,
			}, "application/json")
		} else {
			s.send(w, 200, map[string]any{
				"ok": false, "found": false,
				"msg": "未探测到电源接线, 请检查线缆后重试",
			}, "application/json")
		}
	case "/api/energy":
		if energyStats == nil {
			s.send(w, 200, map[string]any{"ok": false, "error": "no-energy"}, "application/json")
			return
		}
		body := readJSONBody(r)
		action, _ := body["action"].(string)
		if action == "reset" {
			energyStats.Reset()
			s.send(w, 200, map[string]any{"ok": true, "msg": "耗电统计已清零"}, "application/json")
		} else {
			s.send(w, 200, map[string]any{"ok": false, "error": "unknown-action"}, "application/json")
		}
	default:
		s.send(w, 404, map[string]any{"error": "not found"}, "application/json")
	}
}

func (s *appServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := s.stripGateway(r.URL.Path)
	switch r.Method {
	case http.MethodGet:
		s.doGet(w, r, p)
	case http.MethodPost:
		s.doPost(w, r, p)
	default:
		s.send(w, 405, map[string]any{"error": "method not allowed"}, "application/json")
	}
}

// ---------------- 启动 ----------------

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "serve" {
		args = args[1:]
	}
	fs := flag.NewFlagSet("psu-monitor-go", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1", "")
	port := fs.Int("port", 3878, "")
	sockPath := fs.String("socket", "", "")
	webRoot := fs.String("web-root", "ui/static", "")
	pidFile := fs.String("pid", "", "")
	_ = fs.String("log", "", "") // 兼容 Python 参数, 日志固定写 LogPath
	cfgPath := fs.String("config", "", "")
	energyPath := fs.String("energy", "", "") // 耗电统计文件, 默认与 --config 同目录 energy.json
	_ = fs.Parse(args)

	if *pidFile != "" {
		_ = os.WriteFile(*pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
	}

	// 读取 GPIO 引脚配置 (scl_pin / sda_pin), 缺省用默认
	sclPin, sdaPin := DefaultSCL, DefaultSDA
	configured := false
	if *cfgPath != "" {
		if data, err := os.ReadFile(*cfgPath); err == nil {
			var cfg struct {
				Scl string `json:"scl_pin"`
				Sda string `json:"sda_pin"`
			}
			if json.Unmarshal(data, &cfg) == nil {
				if cfg.Scl != "" {
					sclPin = cfg.Scl
				}
				if cfg.Sda != "" {
					sdaPin = cfg.Sda
				}
				configured = true // 有配置文件 = 已在界面设置过 GPIO
			}
		} else {
			logf("警告: 读取 GPIO 配置失败 (%s): %v", *cfgPath, err)
		}
	}

	bus := NewBus(sclPin, sdaPin, configured)
	srv := &appServer{bus: bus, webRoot: *webRoot, configPath: *cfgPath}

	// 耗电统计文件路径: --energy > 与 config 同目录 > 当前目录
	ep := *energyPath
	if ep == "" && *cfgPath != "" {
		ep = filepath.Join(filepath.Dir(*cfgPath), "energy.json")
	}
	if ep == "" {
		ep = "energy.json"
	}
	energyStats = NewEnergy(ep)
	go energySampler(bus)

	// 启动自检: 仅在 GPIO 已配置后才读取电源
	if bus.isConfigured() {
		var vin *uint16
		if bus.isOK() {
			vin = bus.readWord(Cmd["READ_VIN"])
		}
		scl, sda := bus.pinNames()
		logf("GPIO bitbang PMBus: SCL=%s(0x%X/b%d) SDA=%s(0x%X/b%d) addr=0x%02X readable=%s",
			scl, GPPins[scl].Port, GPPins[scl].Bit,
			sda, GPPins[sda].Port, GPPins[sda].Bit, bus.addr,
			map[bool]string{true: "YES", false: "NO"}[vin != nil])
	} else {
		logf("GPIO 引脚未配置: 请在界面设置 PSCL/PSDA 后开始读取。默认候选 SCL=%s SDA=%s", sclPin, sdaPin)
	}

	var ln net.Listener
	var err error
	if *sockPath != "" {
		// 飞牛网关模式: 监听 Unix domain socket, 由 fnOS 网关反代。
		// 权限 0750(owner+group 可连): 不开放到 0777, 避免任意本地进程绕过网关直连控制风扇。
		// 独立部署时依赖 fnOS 网关鉴权; 若网关以不同用户/组运行导致连不上, 按需放宽。
		_ = os.Remove(*sockPath)
		if d := filepath.Dir(*sockPath); d != "" {
			_ = os.MkdirAll(d, 0755)
		}
		ln, err = net.Listen("unix", *sockPath)
		if err == nil {
			_ = os.Chmod(*sockPath, 0750)
			logf("listening on unix socket %s (0750)", *sockPath)
		}
	} else {
		ln, err = net.Listen("tcp", fmt.Sprintf("%s:%d", *addr, *port))
	}
	if err != nil {
		logf("监听失败: %v", err)
		os.Exit(1)
	}

	hs := &http.Server{Handler: srv}
	go func() {
		if err := hs.Serve(ln); err != nil && err != http.ErrServerClosed {
			logf("服务异常: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	<-sig
	if energyStats != nil {
		energyStats.Save() // 退出前把耗电统计落盘
	}
	_ = hs.Close()
}
