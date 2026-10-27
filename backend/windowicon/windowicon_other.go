//go:build !windows && !linux

// Package windowicon 其他平台无需运行时设置窗口图标。
package windowicon

// ApplyWindowIcon no-op
func ApplyWindowIcon(_ string, _ []byte) bool { return false }
