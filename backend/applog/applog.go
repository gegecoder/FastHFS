// Package applog 运行日志：内存环形缓冲 + 日志文件，供管理面板查看与问题排查。
package applog

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"FastHFS/backend/appdir"
)

// Entry 单条日志
type Entry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

const (
	maxEntries  = 2000
	appDirName  = "FastHFS"
	logFileName = "fasthfs.log"
)

var (
	mu      sync.Mutex
	entries []Entry
	console io.Writer

	fileMu  sync.Mutex
	logFile *os.File
)

// DefaultDir 默认日志目录
func DefaultDir() string {
	return filepath.Join(appdir.Base(), appDirName, "logs")
}

// Dir 当前日志目录
func Dir() string {
	fileMu.Lock()
	defer fileMu.Unlock()
	return DefaultDir()
}

// Init 初始化日志：打开默认目录并接管标准库 log。
func Init() {
	console = safeWriter{os.Stdout}
	log.SetOutput(io.MultiWriter(&fileSink{}, safeWriter{os.Stdout}, &lineWriter{}))
	if err := openFile(DefaultDir()); err != nil {
		fmt.Fprintf(os.Stderr, "applog: 初始化日志目录失败: %v\n", err)
	}
}

func openFile(dir string) error {
	fileMu.Lock()
	defer fileMu.Unlock()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, logFileName), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if logFile != nil {
		_ = logFile.Close()
	}
	logFile = f
	return nil
}

// Close 关闭日志文件
func Close() {
	fileMu.Lock()
	defer fileMu.Unlock()
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
}

type fileSink struct{}

func (w *fileSink) Write(p []byte) (int, error) {
	fileMu.Lock()
	defer fileMu.Unlock()
	if logFile != nil {
		_, _ = logFile.Write(p)
	}
	return len(p), nil
}

type safeWriter struct{ w io.Writer }

func (s safeWriter) Write(p []byte) (int, error) {
	n, _ := s.w.Write(p)
	return n, nil
}

type lineWriter struct{}

func (w *lineWriter) Write(p []byte) (int, error) {
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		level := "info"
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "失败") || strings.Contains(lower, "panic") {
			level = "error"
		} else if strings.Contains(lower, "warn") || strings.Contains(lower, "警告") {
			level = "warn"
		}
		push(Entry{Time: time.Now().Format("15:04:05"), Level: level, Message: line})
	}
	return len(p), nil
}

func push(e Entry) {
	mu.Lock()
	defer mu.Unlock()
	entries = append(entries, e)
	if len(entries) > maxEntries {
		entries = entries[len(entries)-maxEntries:]
	}
}

// Infof 记录结构化 info 日志
func Infof(typ, ip, user, format string, args ...any) {
	write("info", formatLine(typ, ip, user, fmt.Sprintf(format, args...)))
}

// Warnf 记录结构化 warn 日志
func Warnf(typ, ip, user, format string, args ...any) {
	write("warn", formatLine(typ, ip, user, fmt.Sprintf(format, args...)))
}

// Errorf 记录结构化 error 日志
func Errorf(typ, ip, user, format string, args ...any) {
	write("error", formatLine(typ, ip, user, fmt.Sprintf(format, args...)))
}

// formatLine 统一日志字段顺序：type=类型 ip=IP user=用户 content=内容；
// 缺失的类型、IP、用户统一记为 -，保证每行结构一致。
func formatLine(typ, ip, user, content string) string {
	if strings.TrimSpace(typ) == "" {
		typ = "app"
	}
	if strings.TrimSpace(ip) == "" {
		ip = "-"
	}
	if strings.TrimSpace(user) == "" {
		user = "-"
	}
	return fmt.Sprintf("type=%s ip=%s user=%s content=%s", typ, ip, user, content)
}

func write(level, msg string) {
	msg = strings.TrimRight(msg, "\n")
	if msg == "" {
		return
	}
	now := time.Now()
	push(Entry{Time: now.Format("15:04:05"), Level: level, Message: msg})
	line := fmt.Sprintf("%s [%s] %s\n", now.Format("2006/01/02 15:04:05"), level, msg)
	fileMu.Lock()
	if logFile != nil {
		_, _ = logFile.WriteString(line)
	}
	fileMu.Unlock()
	if console != nil {
		_, _ = fmt.Fprint(console, line)
	}
	enqueueSyslog(level, msg, now)
}

// Recent 最近日志
func Recent() []Entry {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Entry, len(entries))
	copy(out, entries)
	return out
}

// Clear 清空日志
func Clear() {
	mu.Lock()
	entries = nil
	mu.Unlock()
	fileMu.Lock()
	if logFile != nil {
		_ = logFile.Truncate(0)
	}
	fileMu.Unlock()
}
