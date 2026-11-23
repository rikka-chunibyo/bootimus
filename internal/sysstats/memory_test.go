package sysstats

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMemoryUsesGuestSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "meminfo")
	if err := os.WriteFile(path, []byte("MemTotal: 2097152 kB\nMemFree: 400000 kB\nMemAvailable: 1835008 kB\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BOOTIMUS_MEMORY_INFO", path)
	want := MemoryStats{Total: 2 << 30, Free: 1792 << 20, Used: 256 << 20, UsedPercent: 12.5}
	if got := getMemoryStats(); got != want {
		t.Fatalf("guest memory = %+v, want %+v", got, want)
	}
}

func TestCgroupMemoryUsesParentHeadroom(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"self.cgroup":                 "0::/guest/docker\n",
		"memory.max":                  "max\n",
		"guest/memory.max":            "2147483648\n", // 2 GiB
		"guest/memory.current":        "805306368\n",  // 768 MiB
		"guest/memory.stat":           "anon 671088640\ninactive_file 268435456\n",
		"guest/docker/memory.max":     "max\n",
		"guest/docker/memory.current": "33554432\n",
	}
	for name, data := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	host := memoryFromAvailable(16<<30, 6<<30)
	want := MemoryStats{Total: 2 << 30, Free: 1536 << 20, Used: 512 << 20, UsedPercent: 25}
	if got := cgroupMemory(host, root, filepath.Join(root, "self.cgroup")); got != want {
		t.Fatalf("container memory = %+v, want %+v", got, want)
	}
}

func TestCgroupMemoryKeepsNativeStatsWithoutLimit(t *testing.T) {
	root := t.TempDir()
	membership := filepath.Join(root, "self.cgroup")
	if err := os.WriteFile(membership, []byte("0::/system.slice/bootimus.service\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "memory.max"), []byte("max\n"), 0600); err != nil {
		t.Fatal(err)
	}
	host := memoryFromAvailable(16<<30, 6<<30)
	if got := cgroupMemory(host, root, membership); got != host {
		t.Fatalf("native memory = %+v, want %+v", got, host)
	}
}
