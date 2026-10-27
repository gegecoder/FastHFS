// Package appdir 解析应用数据根目录。
//
// 依次尝试用户配置目录、用户主目录、临时目录，取第一个可写目录，
// 避免启动时创建数据目录失败直接退出。
package appdir

import (
	"os"
	"sync"
)

var (
	once   sync.Once
	cached string
)

// Base 返回可写的应用数据根目录，结果在进程内缓存。
func Base() string {
	once.Do(func() { cached = resolve() })
	return cached
}

func resolve() string {
	for _, dir := range candidates() {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			continue
		}
		if !writable(dir) {
			continue
		}
		return dir
	}
	return "."
}

// writable 通过创建并删除探针文件确认目录可写
func writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".fasthfs-probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

// candidates 候选目录：用户配置目录 → 用户主目录 → 临时目录
func candidates() []string {
	var dirs []string
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		dirs = append(dirs, dir)
	}
	if dir, err := os.UserHomeDir(); err == nil && dir != "" {
		dirs = append(dirs, dir)
	}
	return append(dirs, os.TempDir())
}
