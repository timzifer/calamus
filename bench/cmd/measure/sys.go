package main

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// cpuName names the processor: ratios can shift between processors.
func cpuName() string {
	switch runtime.GOOS {
	case "windows":
		out, err := exec.Command("reg", "query", `HKLM\HARDWARE\DESCRIPTION\System\CentralProcessor\0`, "/v", "ProcessorNameString").Output()
		if err == nil {
			for _, l := range strings.Split(string(out), "\n") {
				if _, v, ok := strings.Cut(l, "REG_SZ"); ok {
					return strings.TrimSpace(v)
				}
			}
		}
	case "linux":
		if v := cpuinfo()["model name"]; v != "" {
			return v
		}
	case "darwin":
		if out, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return "unknown"
}

// physicalCores counts cores, not hardware threads; 0 if unknown.
func physicalCores() int {
	switch runtime.GOOS {
	case "windows":
		out, err := exec.Command("powershell", "-NoProfile", "-Command",
			"(Get-CimInstance Win32_Processor | Measure-Object NumberOfCores -Sum).Sum").Output()
		if err == nil {
			n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
			return n
		}
	case "linux":
		f, err := os.Open("/proc/cpuinfo")
		if err != nil {
			return 0
		}
		defer f.Close()
		cores := map[string]bool{}
		var phys string
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			k, v, _ := strings.Cut(sc.Text(), ":")
			switch strings.TrimSpace(k) {
			case "physical id":
				phys = strings.TrimSpace(v)
			case "core id":
				cores[phys+"/"+strings.TrimSpace(v)] = true
			}
		}
		return len(cores)
	case "darwin":
		if out, err := exec.Command("sysctl", "-n", "hw.physicalcpu").Output(); err == nil {
			n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
			return n
		}
	}
	return 0
}

func cpuinfo() map[string]string {
	m := map[string]string{}
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return m
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if k, v, ok := strings.Cut(sc.Text(), ":"); ok {
			if k = strings.TrimSpace(k); m[k] == "" {
				m[k] = strings.TrimSpace(v)
			}
		}
	}
	return m
}
