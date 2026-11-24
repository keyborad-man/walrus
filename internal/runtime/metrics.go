package runtime

import (
	"math"
	goruntime "runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

func CollectHostMetrics(dataDir, runtimeMode string) HostMetrics {
	m := HostMetrics{
		OS:          goruntime.GOOS,
		Arch:        goruntime.GOARCH,
		RuntimeMode: runtimeMode,
	}

	if pcts, err := cpu.Percent(120*time.Millisecond, false); err == nil && len(pcts) > 0 {
		m.CPUPercent = round1(pcts[0])
	}
	if vm, err := mem.VirtualMemory(); err == nil && vm != nil {
		m.MemoryTotalMB = round1(float64(vm.Total) / (1024 * 1024))
		m.MemoryUsedMB = round1(float64(vm.Used) / (1024 * 1024))
		m.MemoryPercent = round1(vm.UsedPercent)
	}
	if du, err := disk.Usage(dataDir); err == nil && du != nil {
		m.DiskTotalGB = round1(float64(du.Total) / (1024 * 1024 * 1024))
		m.DiskUsedGB = round1(float64(du.Used) / (1024 * 1024 * 1024))
		m.DiskPercent = round1(du.UsedPercent)
	}
	return m
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}
