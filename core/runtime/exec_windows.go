//go:build windows

package runtime

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// HideWindow 隐藏 Windows 控制台子进程窗口，避免桌面端调用外部 CLI 时出现黑框闪现。
func HideWindow(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
