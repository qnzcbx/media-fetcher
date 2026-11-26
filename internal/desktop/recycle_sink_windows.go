//go:build windows

package desktop

import (
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Shell 回调只允许标记为回收的单项操作，并核验实际回收项目。
type recycleSink struct {
	table    *[19]uintptr
	pre      bool
	post     bool
	recycled bool
}

var sinkTable = [19]uintptr{
	syscall.NewCallback(func(self, iid, out uintptr) uintptr {
		wanted := *(*windows.GUID)(unsafe.Pointer(iid))
		unknown := windows.GUID{Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
		sinkIID := windows.GUID{Data1: 0x04b0f1a7, Data2: 0x9490, Data3: 0x44bc, Data4: [8]byte{0x96, 0xe1, 0x42, 0x96, 0xa3, 0x12, 0x52, 0xe2}}
		if wanted != unknown && wanted != sinkIID {
			*(*uintptr)(unsafe.Pointer(out)) = 0
			return 0x80004002
		}
		*(*uintptr)(unsafe.Pointer(out)) = self
		return 0
	}),
	syscall.NewCallback(func(uintptr) uintptr { return 1 }),
	syscall.NewCallback(func(uintptr) uintptr { return 1 }),
	syscall.NewCallback(func(uintptr) uintptr { return 0 }),
	syscall.NewCallback(func(uintptr, uintptr) uintptr { return 0 }),
	syscall.NewCallback(func(uintptr, uintptr, uintptr, uintptr) uintptr { return 0x80004004 }),
	syscall.NewCallback(func(uintptr, uintptr, uintptr, uintptr, uintptr, uintptr) uintptr { return 0 }),
	syscall.NewCallback(func(uintptr, uintptr, uintptr, uintptr, uintptr) uintptr { return 0x80004004 }),
	syscall.NewCallback(func(uintptr, uintptr, uintptr, uintptr, uintptr, uintptr, uintptr) uintptr { return 0 }),
	syscall.NewCallback(func(uintptr, uintptr, uintptr, uintptr, uintptr) uintptr { return 0x80004004 }),
	syscall.NewCallback(func(uintptr, uintptr, uintptr, uintptr, uintptr, uintptr, uintptr) uintptr { return 0 }),
	syscall.NewCallback(func(self, flags, item uintptr) uintptr {
		s := (*recycleSink)(unsafe.Pointer(self))
		if flags&0x80 == 0 {
			return 0x80004004
		}
		s.pre = true
		return 0
	}),
	syscall.NewCallback(func(self, flags, item, result, recycled uintptr) uintptr {
		s := (*recycleSink)(unsafe.Pointer(self))
		s.post = true
		s.recycled = int32(result) >= 0 && recycled != 0 && flags&0x80 != 0
		return 0
	}),
	syscall.NewCallback(func(uintptr, uintptr, uintptr, uintptr) uintptr { return 0x80004004 }),
	syscall.NewCallback(func(uintptr, uintptr, uintptr, uintptr, uintptr, uintptr, uintptr) uintptr { return 0 }),
	syscall.NewCallback(func(uintptr, uintptr, uintptr) uintptr { return 0 }),
	syscall.NewCallback(func(uintptr) uintptr { return 0 }),
	syscall.NewCallback(func(uintptr) uintptr { return 0 }),
	syscall.NewCallback(func(uintptr) uintptr { return 0 }),
}

func newRecycleSink() *recycleSink { return &recycleSink{table: &sinkTable} }
func keepSink(s *recycleSink)      { runtime.KeepAlive(s) }
