package collectors

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"crucible/internal/monitor"
)

// SystemCollector collects system-wide metrics using /proc filesystem
type SystemCollector struct{}

// NewSystemCollector creates a new system metrics collector
func NewSystemCollector() *SystemCollector {
	return &SystemCollector{}
}

// Collect gathers current system metrics
func (s *SystemCollector) Collect() (*monitor.SystemMetrics, error) {
	metrics := &monitor.SystemMetrics{
		Timestamp: time.Now(),
	}

	// Collect CPU metrics
	cpuMetrics, err := s.collectCPUMetrics()
	if err != nil {
		return nil, fmt.Errorf("failed to collect CPU metrics: %w", err)
	}
	metrics.CPU = cpuMetrics

	// Collect memory metrics
	memoryMetrics, err := s.collectMemoryMetrics()
	if err != nil {
		return nil, fmt.Errorf("failed to collect memory metrics: %w", err)
	}
	metrics.Memory = memoryMetrics

	// Collect load metrics
	loadMetrics, err := s.collectLoadMetrics()
	if err != nil {
		return nil, fmt.Errorf("failed to collect load metrics: %w", err)
	}
	metrics.Load = loadMetrics

	// Collect disk metrics
	diskMetrics, err := s.collectDiskMetrics()
	if err != nil {
		return nil, fmt.Errorf("failed to collect disk metrics: %w", err)
	}
	metrics.Disk = diskMetrics

	// Collect network metrics
	networkMetrics, err := s.collectNetworkMetrics()
	if err != nil {
		return nil, fmt.Errorf("failed to collect network metrics: %w", err)
	}
	metrics.Network = networkMetrics

	return metrics, nil
}

// collectCPUMetrics reads CPU usage from /proc/stat
func (s *SystemCollector) collectCPUMetrics() (monitor.CPUMetrics, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return monitor.CPUMetrics{}, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return monitor.CPUMetrics{}, fmt.Errorf("failed to read CPU line from /proc/stat")
	}

	line := scanner.Text()
	fields := strings.Fields(line)
	if len(fields) < 8 || fields[0] != "cpu" {
		return monitor.CPUMetrics{}, fmt.Errorf("invalid CPU line format")
	}

	// Parse CPU times: user, nice, system, idle, iowait, irq, softirq, steal
	var times [8]uint64
	for i := 1; i <= 8 && i < len(fields); i++ {
		times[i-1], err = strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			return monitor.CPUMetrics{}, fmt.Errorf("failed to parse CPU time %d: %w", i, err)
		}
	}

	user := times[0] + times[1] // user + nice
	system := times[2]          // system
	idle := times[3]            // idle
	iowait := times[4]          // iowait
	total := user + system + idle + iowait + times[5] + times[6] + times[7]

	if total == 0 {
		return monitor.CPUMetrics{}, fmt.Errorf("invalid CPU total time")
	}

	return monitor.CPUMetrics{
		UsagePercent:  float64(total-idle) / float64(total) * 100,
		UserPercent:   float64(user) / float64(total) * 100,
		SystemPercent: float64(system) / float64(total) * 100,
		IdlePercent:   float64(idle) / float64(total) * 100,
		IOWaitPercent: float64(iowait) / float64(total) * 100,
	}, nil
}

// collectMemoryMetrics reads memory usage from /proc/meminfo
func (s *SystemCollector) collectMemoryMetrics() (monitor.MemoryMetrics, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return monitor.MemoryMetrics{}, err
	}
	defer file.Close()

	memInfo := make(map[string]uint64)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			key := strings.TrimSuffix(fields[0], ":")
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err == nil {
				// Convert from KB to bytes
				memInfo[key] = value * 1024
			}
		}
	}

	totalBytes := memInfo["MemTotal"]
	freeBytes := memInfo["MemFree"]
	availableBytes := memInfo["MemAvailable"]
	if availableBytes == 0 {
		// Fallback calculation if MemAvailable is not present
		availableBytes = freeBytes + memInfo["Buffers"] + memInfo["Cached"]
	}
	usedBytes := totalBytes - availableBytes

	var usagePercent float64
	if totalBytes > 0 {
		usagePercent = float64(usedBytes) / float64(totalBytes) * 100
	}

	swapTotal := memInfo["SwapTotal"]
	swapFree := memInfo["SwapFree"]
	swapUsed := swapTotal - swapFree
	var swapUsagePercent float64
	if swapTotal > 0 {
		swapUsagePercent = float64(swapUsed) / float64(swapTotal) * 100
	}

	return monitor.MemoryMetrics{
		TotalBytes:       totalBytes,
		UsedBytes:        usedBytes,
		FreeBytes:        freeBytes,
		AvailableBytes:   availableBytes,
		UsagePercent:     usagePercent,
		SwapTotalBytes:   swapTotal,
		SwapUsedBytes:    swapUsed,
		SwapUsagePercent: swapUsagePercent,
	}, nil
}

// collectLoadMetrics reads load average from /proc/loadavg
func (s *SystemCollector) collectLoadMetrics() (monitor.LoadMetrics, error) {
	file, err := os.Open("/proc/loadavg")
	if err != nil {
		return monitor.LoadMetrics{}, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return monitor.LoadMetrics{}, fmt.Errorf("failed to read from /proc/loadavg")
	}

	fields := strings.Fields(scanner.Text())
	if len(fields) < 3 {
		return monitor.LoadMetrics{}, fmt.Errorf("invalid loadavg format")
	}

	load1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return monitor.LoadMetrics{}, fmt.Errorf("failed to parse load1: %w", err)
	}

	load5, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return monitor.LoadMetrics{}, fmt.Errorf("failed to parse load5: %w", err)
	}

	load15, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return monitor.LoadMetrics{}, fmt.Errorf("failed to parse load15: %w", err)
	}

	return monitor.LoadMetrics{
		Load1:  load1,
		Load5:  load5,
		Load15: load15,
	}, nil
}

// collectDiskMetrics reads disk usage using syscall for mounted filesystems
func (s *SystemCollector) collectDiskMetrics() ([]monitor.DiskMetrics, error) {
	// Read mount points from /proc/mounts
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var diskMetrics []monitor.DiskMetrics
	scanner := bufio.NewScanner(file)
	mountPoints := make(map[string]string) // mountpoint -> device

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 3 {
			device := fields[0]
			mountPoint := fields[1]
			fsType := fields[2]

			// Skip special filesystems
			if strings.HasPrefix(device, "/dev/") &&
				fsType != "tmpfs" &&
				fsType != "devtmpfs" &&
				fsType != "sysfs" &&
				fsType != "proc" {
				mountPoints[mountPoint] = device
			}
		}
	}

	// Get disk usage for each mount point
	for mountPoint, device := range mountPoints {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(mountPoint, &stat); err != nil {
			continue // Skip on error
		}

		totalBytes := stat.Blocks * uint64(stat.Bsize)
		freeBytes := stat.Bavail * uint64(stat.Bsize)
		usedBytes := totalBytes - freeBytes

		var usagePercent float64
		if totalBytes > 0 {
			usagePercent = float64(usedBytes) / float64(totalBytes) * 100
		}

		diskMetrics = append(diskMetrics, monitor.DiskMetrics{
			MountPoint:   mountPoint,
			Device:       device,
			TotalBytes:   totalBytes,
			UsedBytes:    usedBytes,
			FreeBytes:    freeBytes,
			UsagePercent: usagePercent,
			InodesTotal:  stat.Files,
			InodesUsed:   stat.Files - stat.Ffree,
			InodesFree:   stat.Ffree,
		})
	}

	return diskMetrics, nil
}

// collectNetworkMetrics reads network interface statistics from /proc/net/dev
func (s *SystemCollector) collectNetworkMetrics() ([]monitor.NetworkMetrics, error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var networkMetrics []monitor.NetworkMetrics
	scanner := bufio.NewScanner(file)

	// Skip header lines
	scanner.Scan() // Inter-|   Receive                  |  Transmit
	scanner.Scan() // face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		colonIndex := strings.Index(line, ":")
		if colonIndex == -1 {
			continue
		}

		interfaceName := strings.TrimSpace(line[:colonIndex])
		stats := strings.Fields(line[colonIndex+1:])

		if len(stats) < 16 {
			continue
		}

		// Skip loopback interface
		if interfaceName == "lo" {
			continue
		}

		bytesRecv, _ := strconv.ParseUint(stats[0], 10, 64)
		packetsRecv, _ := strconv.ParseUint(stats[1], 10, 64)
		errorsRecv, _ := strconv.ParseUint(stats[2], 10, 64)
		droppedRecv, _ := strconv.ParseUint(stats[3], 10, 64)

		bytesSent, _ := strconv.ParseUint(stats[8], 10, 64)
		packetsSent, _ := strconv.ParseUint(stats[9], 10, 64)
		errorsSent, _ := strconv.ParseUint(stats[10], 10, 64)
		droppedSent, _ := strconv.ParseUint(stats[11], 10, 64)

		networkMetrics = append(networkMetrics, monitor.NetworkMetrics{
			Interface:   interfaceName,
			BytesRecv:   bytesRecv,
			BytesSent:   bytesSent,
			PacketsRecv: packetsRecv,
			PacketsSent: packetsSent,
			ErrorsRecv:  errorsRecv,
			ErrorsSent:  errorsSent,
			DroppedRecv: droppedRecv,
			DroppedSent: droppedSent,
		})
	}

	return networkMetrics, nil
}
