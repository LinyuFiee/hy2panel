package server

import (
	"bufio"
	"context"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Status struct {
	SingBox           string  `json:"sing_box"`
	LoadAverage       string  `json:"load_average"`
	MemoryUsedMB      uint64  `json:"memory_used_mb"`
	MemoryTotalMB     uint64  `json:"memory_total_mb"`
	CPUPercent        float64 `json:"cpu_percent"`
	DiskUsedBytes     uint64  `json:"disk_used_bytes"`
	DiskTotalBytes    uint64  `json:"disk_total_bytes"`
	UptimeSeconds     float64 `json:"uptime_seconds"`
	RXBytesPerSecond  float64 `json:"rx_bytes_per_second"`
	TXBytesPerSecond  float64 `json:"tx_bytes_per_second"`
	RXTotalBytes      uint64  `json:"rx_total_bytes"`
	TXTotalBytes      uint64  `json:"tx_total_bytes"`
	CongestionControl string  `json:"congestion_control"`
	DefaultQdisc      string  `json:"default_qdisc"`
	BBREnabled        bool    `json:"bbr_enabled"`
}

type metricSamples struct {
	cpuTotal    uint64
	cpuIdle     uint64
	rxBytes     uint64
	txBytes     uint64
	networkTime time.Time
}

var (
	metricsMu sync.Mutex
	previous  metricSamples
)

func readStatus(ctx context.Context) Status {
	result := Status{SingBox: "unknown", LoadAverage: "unavailable", CongestionControl: "unknown", DefaultQdisc: "unknown"}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "systemctl", "is-active", "sing-box").CombinedOutput()
	status := strings.TrimSpace(string(output))
	if status != "" {
		result.SingBox = status
	} else if err != nil {
		result.SingBox = "unavailable"
	}
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 3 {
			result.LoadAverage = strings.Join(fields[:3], " ")
		}
	}
	readMemory(&result)
	result.DiskUsedBytes, result.DiskTotalBytes = readDiskUsage()
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 0 {
			result.UptimeSeconds, _ = strconv.ParseFloat(fields[0], 64)
		}
	}
	if data, err := os.ReadFile("/proc/sys/net/ipv4/tcp_congestion_control"); err == nil {
		result.CongestionControl = strings.TrimSpace(string(data))
	}
	if data, err := os.ReadFile("/proc/sys/net/core/default_qdisc"); err == nil {
		result.DefaultQdisc = strings.TrimSpace(string(data))
	}
	result.BBREnabled = result.CongestionControl == "bbr" && result.DefaultQdisc == "fq"
	metricsMu.Lock()
	defer metricsMu.Unlock()
	sampleCPU(&result)
	sampleNetwork(&result)
	return result
}

func readMemory(result *Status) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return
	}
	defer file.Close()
	values := make(map[string]uint64)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		if key != "MemTotal" && key != "MemAvailable" {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err == nil {
			values[key] = value
		}
	}
	result.MemoryTotalMB = values["MemTotal"] / 1024
	if result.MemoryTotalMB > 0 && values["MemAvailable"]/1024 <= result.MemoryTotalMB {
		result.MemoryUsedMB = result.MemoryTotalMB - values["MemAvailable"]/1024
	}
}

func sampleCPU(result *Status) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return
	}
	fields := strings.Fields(strings.SplitN(string(data), "\n", 2)[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return
	}
	var total uint64
	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return
		}
		values = append(values, value)
		total += value
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	if previous.cpuTotal > 0 && total > previous.cpuTotal && idle >= previous.cpuIdle && total-previous.cpuTotal > idle-previous.cpuIdle {
		busy := (total - previous.cpuTotal) - (idle - previous.cpuIdle)
		result.CPUPercent = math.Round(float64(busy)/float64(total-previous.cpuTotal)*1000) / 10
	}
	previous.cpuTotal = total
	previous.cpuIdle = idle
}

func sampleNetwork(result *Status) {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return
	}
	var rxTotal, txTotal uint64
	for _, line := range strings.Split(string(data), "\n") {
		_, fieldsText, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		fields := strings.Fields(fieldsText)
		if len(fields) < 9 || strings.HasPrefix(strings.TrimSpace(strings.SplitN(line, ":", 2)[0]), "lo") {
			continue
		}
		rx, rxErr := strconv.ParseUint(fields[0], 10, 64)
		tx, txErr := strconv.ParseUint(fields[8], 10, 64)
		if rxErr == nil && txErr == nil {
			rxTotal += rx
			txTotal += tx
		}
	}
	now := time.Now()
	if !previous.networkTime.IsZero() {
		elapsed := now.Sub(previous.networkTime).Seconds()
		if elapsed > 0 && rxTotal >= previous.rxBytes && txTotal >= previous.txBytes {
			result.RXBytesPerSecond = float64(rxTotal-previous.rxBytes) / elapsed
			result.TXBytesPerSecond = float64(txTotal-previous.txBytes) / elapsed
		}
	}
	result.RXTotalBytes = rxTotal
	result.TXTotalBytes = txTotal
	previous.rxBytes = rxTotal
	previous.txBytes = txTotal
	previous.networkTime = now
}
