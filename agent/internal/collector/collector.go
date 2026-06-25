// Package collector 在 agent 上采集系统指标。
// 跨平台（gopsutil）：Linux 走 /proc，macOS 走系统调用 —— 便于本机开发。
// 网络字段储存"自上次采样以来的 bytes/秒"（rate），而不是累计计数。
package collector

import (
	"context"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

type Sample struct {
	TS         time.Time `json:"ts"`
	CPUPct     float64   `json:"cpuPct"`
	MemUsed    uint64    `json:"memUsed"`
	MemTotal   uint64    `json:"memTotal"`
	DiskUsed   uint64    `json:"diskUsed"`
	DiskTotal  uint64    `json:"diskTotal"`
	NetRx      uint64    `json:"netRx"` // bytes/s
	NetTx      uint64    `json:"netTx"` // bytes/s
	Load1      float64   `json:"load1"`
	UptimeSec  uint64    `json:"uptimeSec"`
	Distro     string    `json:"distro,omitempty"`
	Kernel     string    `json:"kernel,omitempty"`
}

type Collector struct {
	prevRx uint64
	prevTx uint64
	prevTs time.Time
}

func New() *Collector { return &Collector{} }

// Sample 抓一次快照。短时间窗内 CPU 采样靠 gopsutil 内置；其他都是即时读取。
func (c *Collector) Sample(ctx context.Context) (*Sample, error) {
	now := time.Now().UTC()

	cpuPcts, err := cpu.PercentWithContext(ctx, 0, false)
	if err != nil {
		return nil, err
	}
	var cpuPct float64
	if len(cpuPcts) > 0 {
		cpuPct = cpuPcts[0]
	}

	v, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}

	du, err := disk.UsageWithContext(ctx, "/")
	if err != nil {
		return nil, err
	}

	netIO, err := net.IOCountersWithContext(ctx, false) // 聚合所有网卡
	if err != nil {
		return nil, err
	}
	var rx, tx uint64
	if len(netIO) > 0 {
		rx = netIO[0].BytesRecv
		tx = netIO[0].BytesSent
	}
	var netRxRate, netTxRate uint64
	if !c.prevTs.IsZero() {
		dt := now.Sub(c.prevTs).Seconds()
		if dt > 0 && rx >= c.prevRx && tx >= c.prevTx {
			netRxRate = uint64(float64(rx-c.prevRx) / dt)
			netTxRate = uint64(float64(tx-c.prevTx) / dt)
		}
	}
	c.prevRx, c.prevTx, c.prevTs = rx, tx, now

	loadAvg, _ := load.AvgWithContext(ctx)
	var l1 float64
	if loadAvg != nil {
		l1 = loadAvg.Load1
	}

	upSec, _ := host.UptimeWithContext(ctx)

	s := &Sample{
		TS:        now,
		CPUPct:    cpuPct,
		MemUsed:   v.Used,
		MemTotal:  v.Total,
		DiskUsed:  du.Used,
		DiskTotal: du.Total,
		NetRx:     netRxRate,
		NetTx:     netTxRate,
		Load1:     l1,
		UptimeSec: upSec,
	}

	// 系统信息：每次都带（成本可忽略，且便于后端在 enroll 后回填）
	if info, err := host.InfoWithContext(ctx); err == nil && info != nil {
		s.Distro = info.Platform + " " + info.PlatformVersion
		s.Kernel = info.KernelVersion
		if s.Distro == " " {
			s.Distro = runtime.GOOS
		}
	}
	return s, nil
}
