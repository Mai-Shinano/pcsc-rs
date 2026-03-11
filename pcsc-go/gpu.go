package main

import (
	"os/exec"
	"regexp"
	"strconv"
)

func getGPUInfo() *GpuData {
	cmd := exec.Command("nvidia-smi",
		"--format=csv",
		"--query-gpu=name,utilization.gpu,memory.free,memory.total",
	)
	gpuCmdSetup(cmd)

	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := regexp.MustCompile(`\r?\n`).Split(string(output), -1)
	if len(lines) < 2 {
		return nil
	}

	cleaned := regexp.MustCompile(` %| MiB| GiB|\r`).ReplaceAllString(lines[1], "")
	fields := regexp.MustCompile(`, `).Split(cleaned, -1)
	if len(fields) < 4 {
		return nil
	}

	var usage *uint64
	if fields[1] != "[N/A]" {
		if v, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
			usage = &v
		}
	}

	free, _ := strconv.ParseUint(fields[2], 10, 64)
	total, _ := strconv.ParseUint(fields[3], 10, 64)

	return &GpuData{
		Name:  fields[0],
		Usage: usage,
		Memory: GpuMemory{
			Free:  free,
			Total: total,
		},
	}
}
