package server

import (
	"bufio"
	"context"

	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Status struct {
	SingBox       string `json:"sing_box"`
	LoadAverage   string `json:"load_average"`
	MemoryUsedMB  uint64 `json:"memory_used_mb"`
	MemoryTotalMB uint64 `json:"memory_total_mb"`
}

func readStatus(ctx context.Context) Status {
	result := Status{SingBox: "unknown", LoadAverage: "unavailable"}
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
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return result
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
	return result
}
