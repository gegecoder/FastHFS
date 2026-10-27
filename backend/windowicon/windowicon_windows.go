//go:build windows

// Package windowicon 通过 Win32 WM_SETICON 设置窗口图标（dev 与打包均生效）。
package windowicon

import (
	"os"
	"syscall"
	"unsafe"

	traypkg "FastHFS/backend/tray"
)

const (
	imageIcon      = 1
	lrLoadFromFile = 0x00000010
	wmSetIcon      = 0x0080
	iconSmall      = 0
	iconBig        = 1
	smCXIcon       = 11
	smCYIcon       = 12
	smCXSmIcon     = 49
	smCYSmIcon     = 50
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procFindWindowW      = user32.NewProc("FindWindowW")
	procLoadImageW       = user32.NewProc("LoadImageW")
	procSendMessageW     = user32.NewProc("SendMessageW")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
)

func sysMetric(index int) int {
	v, _, _ := procGetSystemMetrics.Call(uintptr(index))
	return int(v)
}

func loadSizedIcon(path string, cx, cy int) syscall.Handle {
	pathPtr, _ := syscall.UTF16PtrFromString(path)
	h, _, _ := procLoadImageW.Call(0, uintptr(unsafe.Pointer(pathPtr)), imageIcon,
		uintptr(cx), uintptr(cy), lrLoadFromFile)
	return syscall.Handle(h)
}

// ApplyWindowIcon 应用多尺寸图标到已创建窗口
func ApplyWindowIcon(title string, iconPNG []byte) bool {
	if len(iconPNG) == 0 {
		return false
	}
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	if hwnd == 0 {
		return false
	}
	icoBytes := traypkg.MustICOScaled(iconPNG, []int{16, 20, 24, 32, 40, 48, 64})
	f, err := os.CreateTemp("", "fasthfs-icon-*.ico")
	if err != nil {
		return false
	}
	tmpPath := f.Name()
	_, _ = f.Write(icoBytes)
	_ = f.Close()
	defer os.Remove(tmpPath)

	smCX := sysMetric(smCXSmIcon)
	smCY := sysMetric(smCYSmIcon)
	bigCX := sysMetric(smCXIcon)
	bigCY := sysMetric(smCYIcon)

	applied := false
	if h := loadSizedIcon(tmpPath, smCX, smCY); h != 0 {
		procSendMessageW.Call(hwnd, wmSetIcon, iconSmall, uintptr(h))
		applied = true
	}
	if h := loadSizedIcon(tmpPath, bigCX, bigCY); h != 0 {
		procSendMessageW.Call(hwnd, wmSetIcon, iconBig, uintptr(h))
		applied = true
	}
	return applied
}
