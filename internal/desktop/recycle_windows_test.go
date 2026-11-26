//go:build windows

package desktop

import "testing"

func TestRecycleFlagsDoNotAutoConfirm(t *testing.T) {
	const fofNoConfirmation = 0x0010
	if recycleOperationFlags&fofNoConfirmation != 0 {
		t.Fatal("回收操作不得自动回答系统确认")
	}
	if recycleOperationFlags&fofWantNukeWarning == 0 || recycleOperationFlags&fofxRecycleOnDelete == 0 {
		t.Fatal("回收和永久删除警告标志不可丢失")
	}
}
