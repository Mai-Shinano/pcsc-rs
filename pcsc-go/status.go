package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

type CoreData struct {
	Usage float64 `json:"cpu"`
}

type CpuData struct {
	Model string     `json:"model"`
	Cpus  []CoreData `json:"cpus"`
}

type RamData struct {
	Free  uint64 `json:"free"`
	Total uint64 `json:"total"`
}

type SwapData struct {
	Free  uint64 `json:"free"`
	Total uint64 `json:"total"`
}

type StorageData struct {
	Name  string `json:"name"`
	Free  uint64 `json:"free"`
	Total uint64 `json:"total"`
}

type GpuMemory struct {
	Free  uint64 `json:"free"`
	Total uint64 `json:"total"`
}

type GpuData struct {
	Name   string    `json:"name"`
	Usage  *uint64   `json:"usage"`
	Memory GpuMemory `json:"memory"`
}

type SystemStatus struct {
	OS          string        `json:"_os"`
	Hostname    string        `json:"hostname"`
	Version     string        `json:"version"`
	CPU         CpuData       `json:"cpu"`
	RAM         RamData       `json:"ram"`
	Swap        SwapData      `json:"swap"`
	Storages    []StorageData `json:"storages"`
	LoadAverage *[3]float64   `json:"loadavg"`
	Uptime      string        `json:"uptime"`
	GPU         *GpuData      `json:"gpu"`
}

type StatusDataWithPass struct {
	SystemStatus
	Pass string `json:"pass"`
}

func getStatus() SystemStatus {
	// CPU
	cpuInfo, _ := cpu.Info()
	cpuModel := ""
	if len(cpuInfo) > 0 {
		cpuModel = cpuInfo[0].ModelName
	}

	perCPU, _ := cpu.Percent(0, true)
	cores := make([]CoreData, len(perCPU))
	for i, u := range perCPU {
		cores[i] = CoreData{Usage: u}
	}

	// Memory
	memInfo, _ := mem.VirtualMemory()
	swapInfo, _ := mem.SwapMemory()

	ram := RamData{Free: memInfo.Available, Total: memInfo.Total}
	swap := SwapData{Free: swapInfo.Free, Total: swapInfo.Total}

	// Disks
	partitions, _ := disk.Partitions(false)
	seen := make(map[string]bool)
	var storages []StorageData
	for _, p := range partitions {
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil || usage.Total == 0 {
			continue
		}
		key := fmt.Sprintf("%s-%d", p.Device, usage.Total)
		if seen[key] {
			continue
		}
		seen[key] = true
		storages = append(storages, StorageData{
			Name:  p.Device,
			Free:  usage.Free,
			Total: usage.Total,
		})
	}

	// Host info
	hostInfo, _ := host.Info()
	osName := fmt.Sprintf("%s %s", hostInfo.Platform, hostInfo.PlatformVersion)

	hostname := os.Getenv("HOSTNAME")
	if hostname == "" {
		hostname = hostInfo.Hostname
	}

	uptimeStr := formatUptime(hostInfo.Uptime)

	// Load average (not available on Windows)
	var loadAvg *[3]float64
	if runtime.GOOS != "windows" {
		if avg, err := load.Avg(); err == nil {
			loadAvg = &[3]float64{avg.Load1, avg.Load5, avg.Load15}
		}
	}

	gpu := getGPUInfo()

	return SystemStatus{
		OS:          osName,
		Hostname:    hostname,
		Version:     fmt.Sprintf("Go client %s", gitDescribe),
		CPU:         CpuData{Model: cpuModel, Cpus: cores},
		RAM:         ram,
		Swap:        swap,
		Storages:    storages,
		LoadAverage: loadAvg,
		Uptime:      uptimeStr,
		GPU:         gpu,
	}
}
