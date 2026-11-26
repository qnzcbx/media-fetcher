//go:build windows

package desktop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func fileIdentity(info os.FileInfo) string {
	// 使用大小与纳秒修改时间识别明显替换，旧记录不补写身份。
	return fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
}
func safeLocalPath(path string) error {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		p, e := windows.UTF16PtrFromString(current)
		if e != nil {
			return e
		}
		attrs, e := windows.GetFileAttributes(p)
		if e != nil {
			return e
		}
		if attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return errors.New("不支持链接文件")
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	volume := filepath.VolumeName(path)
	if len(volume) != 2 {
		return errors.New("仅支持本地磁盘")
	}
	root, _ := windows.UTF16PtrFromString(volume + "\\")
	if windows.GetDriveType(root) != windows.DRIVE_FIXED {
		return errors.New("仅支持本地固定磁盘")
	}
	return nil
}

type shellCOM struct{ table *[23]uintptr }

func (o *shellCOM) call(index int, args ...uintptr) error {
	all := append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)
	hr, _, _ := syscall.SyscallN(o.table[index], all...)
	if int32(hr) < 0 {
		return fmt.Errorf("Shell HRESULT 0x%x", uint32(hr))
	}
	return nil
}
func recycleFile(path string) error {
	if err := safeLocalPath(path); err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole := windows.NewLazySystemDLL("ole32.dll")
	hr, _, _ := ole.NewProc("CoInitializeEx").Call(0, 2)
	if int32(hr) < 0 {
		return errors.New("无法初始化回收站操作")
	}
	defer ole.NewProc("CoUninitialize").Call()
	clsid := windows.GUID{Data1: 0x3ad05575, Data2: 0x8857, Data3: 0x4850, Data4: [8]byte{0x92, 0x77, 0x11, 0xb8, 0x5b, 0xdb, 0x8e, 0x09}}
	iid := windows.GUID{Data1: 0x947aab5f, Data2: 0x0a5c, Data3: 0x4c13, Data4: [8]byte{0xb4, 0xd6, 0x4b, 0xf7, 0x83, 0x6f, 0xc9, 0xf8}}
	var operation *shellCOM
	hr, _, _ = ole.NewProc("CoCreateInstance").Call(uintptr(unsafe.Pointer(&clsid)), 0, 1, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&operation)))
	if int32(hr) < 0 {
		return errors.New("回收站操作不可用")
	}
	defer operation.call(2)
	// RECYCLEONDELETE + EARLYFAILURE，禁止自动永久删除的后备路径。
	if err := operation.call(5, 0x80000|0x100000|0x400|0x2000|0x4000); err != nil {
		return err
	}
	itemIID := windows.GUID{Data1: 0x43826d1e, Data2: 0xe718, Data3: 0x42ee, Data4: [8]byte{0xbc, 0x55, 0xa1, 0xe2, 0x61, 0xc3, 0x7b, 0xfe}}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	var item *shellCOM
	hr, _, _ = windows.NewLazySystemDLL("shell32.dll").NewProc("SHCreateItemFromParsingName").Call(uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&itemIID)), uintptr(unsafe.Pointer(&item)))
	if int32(hr) < 0 {
		return errors.New("文件不可用")
	}
	defer item.call(2)
	if err = operation.call(18, uintptr(unsafe.Pointer(item)), 0); err != nil {
		return err
	}
	if err = operation.call(21); err != nil {
		return err
	}
	var aborted int32
	if err = operation.call(22, uintptr(unsafe.Pointer(&aborted))); err != nil {
		return err
	}
	if aborted != 0 {
		return errRecycleCancelled
	}
	if _, err = os.Lstat(path); !os.IsNotExist(err) {
		return errors.New("系统未确认文件已回收")
	}
	return nil
}
