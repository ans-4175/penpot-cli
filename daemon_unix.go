//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// detachDaemonProcess: daemon hidup sendiri walau terminal pemanggil ditutup.
func detachDaemonProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func terminateProcess(pid int) error { return syscall.Kill(pid, syscall.SIGTERM) }
