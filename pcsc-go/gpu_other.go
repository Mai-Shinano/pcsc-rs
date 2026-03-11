//go:build !windows

package main

import "os/exec"

func gpuCmdSetup(cmd *exec.Cmd) {}
