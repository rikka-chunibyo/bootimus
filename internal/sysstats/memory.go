package sysstats

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/mem"
)

func getMemoryStats() MemoryStats {
	// A nested container may not be able to see the cgroup enforcing its
	// parent guest's limit. Use a memory snapshot taken in that guest when supplied.
	if path := os.Getenv("BOOTIMUS_MEMORY_INFO"); path != "" {
		if snapshot, ok := readMemoryInfo(path); ok {
			return snapshot
		}
	}
	vm, err := mem.VirtualMemory()
	if err != nil {
		return MemoryStats{}
	}
	stats := memoryFromAvailable(vm.Total, vm.Available)
	if runtime.GOOS == "linux" {
		stats = cgroupMemory(stats, "/sys/fs/cgroup", "/proc/self/cgroup")
	}
	return stats
}

func memoryFromAvailable(total, available uint64) MemoryStats {
	available = min(available, total)
	used := total - available
	stats := MemoryStats{Total: total, Free: available, Used: used}
	if total > 0 {
		stats.UsedPercent = float64(used) * 100 / float64(total)
	}
	return stats
}

func readMemoryInfo(path string) (MemoryStats, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MemoryStats{}, false
	}
	values := make(map[string]uint64, 2)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[2] != "kB" {
			continue
		}
		if fields[0] == "MemTotal:" || fields[0] == "MemAvailable:" {
			n, err := strconv.ParseUint(fields[1], 10, 54)
			if err == nil {
				values[fields[0]] = n * 1024
			}
		}
	}
	total := values["MemTotal:"]
	available, present := values["MemAvailable:"]
	return memoryFromAvailable(total, available), total > 0 && present
}

func cgroupMemory(host MemoryStats, root, membership string) MemoryStats {
	data, err := os.ReadFile(membership)
	if err != nil {
		return host
	}
	for _, line := range strings.Split(string(data), "\n") {
		group, ok := strings.CutPrefix(line, "0::/")
		if !ok {
			continue
		}
		total, available := host.Total, host.Free
		dir := filepath.Join(root, strings.TrimPrefix(filepath.Clean("/"+group), "/"))
		for {
			limit, limited := readMemoryNumber(filepath.Join(dir, "memory.max"))
			if limited {
				total = min(total, limit)
				if current, ok := readMemoryNumber(filepath.Join(dir, "memory.current")); ok {
					cache := inactiveFileMemory(filepath.Join(dir, "memory.stat"))
					used := current - min(current, cache)
					available = min(available, limit-min(limit, used))
				}
			}
			if dir == root {
				break
			}
			dir = filepath.Dir(dir)
		}
		return memoryFromAvailable(total, available)
	}
	return host
}

func readMemoryNumber(path string) (uint64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	value, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	return value, err == nil
}

func inactiveFileMemory(path string) uint64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if value, ok := strings.CutPrefix(line, "inactive_file "); ok {
			n, _ := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
			return n
		}
	}
	return 0
}
