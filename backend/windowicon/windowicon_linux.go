//go:build linux

// Package windowicon Linux 用户级桌面集成（直接运行二进制时显示应用图标）。
package windowicon

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// appID 与 main.go 中 LinuxOptions.ApplicationID 保持一致
const appID = "com.fasthfs.app"

// ApplyWindowIcon 安装/刷新用户级图标与 .desktop 条目
func ApplyWindowIcon(_ string, iconPNG []byte) bool {
	if os.Getenv("APPIMAGE") != "" || len(iconPNG) == 0 {
		return false
	}
	if installedDesktopExists() {
		return true
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	exe, err := os.Executable()
	if err != nil || exe == "" {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	appsDir := filepath.Join(home, ".local", "share", "applications")
	iconDir := filepath.Join(home, ".local", "share", "icons", "hicolor", "64x64", "apps")
	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		return false
	}
	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		return false
	}
	if err := os.WriteFile(filepath.Join(iconDir, appID+".png"), iconPNG, 0o644); err != nil {
		return false
	}

	desktop := strings.Join([]string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=FastHFS",
		`Exec="` + exe + `"`,
		"Icon=" + appID,
		"Terminal=false",
		"Categories=Network;",
		"StartupNotify=false",
		"StartupWMClass=" + appID,
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(appsDir, appID+".desktop"), []byte(desktop), 0o644); err != nil {
		return false
	}
	if p, err := exec.LookPath("update-desktop-database"); err == nil {
		_ = exec.Command(p, appsDir).Start()
	}
	return true
}

func installedDesktopExists() bool {
	dirs := os.Getenv("XDG_DATA_DIRS")
	if dirs == "" {
		dirs = "/usr/local/share:/usr/share"
	}
	for _, dir := range strings.Split(dirs, ":") {
		if dir == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "applications", appID+".desktop")); err == nil {
			return true
		}
	}
	return false
}
