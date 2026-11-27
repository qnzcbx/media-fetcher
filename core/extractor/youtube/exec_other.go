//go:build !windows

package youtube

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}
