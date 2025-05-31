package worker

import (
	linux "github.com/c9s/goprocinfo/linux"
	"log"
)

type Stats struct {
	MemStats  *linux.MemInfo
	DiskStats *linux.Disk
	CpuStats  *linux.CPUStat
	LoadStats *linux.LoadAvg
	TaskCount int
}

func (s *Stats) MemInfoKb() uint64 {
	//gets the total memory in kb
	return s.MemStats.MemTotal
}

func (s *Stats) MemAvailableKb() uint64 {
	return s.MemStats.MemAvailable
}

func (s *Stats) MemoryUsedKb() uint64 {
	return s.MemStats.MemTotal - s.MemStats.MemAvailable
}

func (s *Stats) MemoryPercentUsed() float64 {
	return float64(s.MemoryUsedKb()) / float64(s.MemInfoKb()) * 100.0
}

func (s *Stats) DiskTotalKb() uint64 {
	return s.DiskStats.All
}

func (s *Stats) DiskFreeKb() uint64 {
	return s.DiskStats.Free
}

func (s *Stats) DiskUsedKb() uint64 {
	return s.DiskStats.Used
}

func (s *Stats) CpuUsage() float64 {
	//formula: ((idle + busy) - idle) / (idle + busy)
	idle := s.CpuStats.Idle + s.CpuStats.IOWait
	busy := s.CpuStats.User + s.CpuStats.Nice + s.CpuStats.System + s.CpuStats.IRQ + s.CpuStats.SoftIRQ
	total := idle + busy
	if total == 0 {
		return 0.00
	}
	return (float64(total) - float64(idle)) / float64(total)
}
func GetStats() *Stats {
	return &Stats{
		MemStats:  GetMemoryInfo(),
		DiskStats: GetDiskInfo(),
		CpuStats:  GetCpuStats(),
		LoadStats: GetLoadAvg(),
	}
}

func GetMemoryInfo() *linux.MemInfo {
	memstats, err := linux.ReadMemInfo("/proc/meminfo")
	if err != nil {
		log.Printf("Error reading from /proc/meminfo")
		return &linux.MemInfo{}
	}
	return memstats
}

// GetDiskInfo See https://godoc.org/github.com/c9s/goprocinfo/linux#Disk
func GetDiskInfo() *linux.Disk {
	diskstats, err := linux.ReadDisk("/")
	if err != nil {
		log.Printf("Error reading from /")
		return &linux.Disk{}
	}
	return diskstats
}

// GetCpuInfo See https://godoc.org/github.com/c9s/goprocinfo/linux#CPUStat
func GetCpuStats() *linux.CPUStat {
	stats, err := linux.ReadStat("/proc/stat")
	if err != nil {
		log.Printf("Error reading from /proc/stat")
		return &linux.CPUStat{}
	}
	return &stats.CPUStatAll
}

// GetLoadAvg See https://godoc.org/github.com/c9s/goprocinfo/linux#LoadAvg
func GetLoadAvg() *linux.LoadAvg {
	loadavg, err := linux.ReadLoadAvg("/proc/loadavg")
	if err != nil {
		log.Printf("Error reading from /proc/loadavg")
		return &linux.LoadAvg{}
	}
	return loadavg
}
