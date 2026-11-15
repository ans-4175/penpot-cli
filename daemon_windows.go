//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

const (
	createNewProcessGroup = 0x00000200
	detachedProcess       = 0x00000008
)

// detachDaemonProcess: Windows tak punya setsid; grup proses baru + tanpa console.
func detachDaemonProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup | detachedProcess}
}

// Windows tak punya SIGTERM antarproses: daemon dimatikan paksa, sock/pid
// basi dibersihkan oleh pemanggil.
func terminateProcess(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}
