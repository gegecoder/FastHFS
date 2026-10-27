// Syslog 外发：把运行与操作日志以 RFC5424 格式通过 UDP 转发到一个或多个目标。
package applog

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	syslogQueueSize  = 1024
	syslogDialTimout = 2 * time.Second
)

var (
	syslogMu      sync.RWMutex
	syslogEnabled bool
	syslogTargets []string
	syslogQueue   chan syslogMessage
	syslogOnce    sync.Once

	syslogHostname = func() string {
		name, err := os.Hostname()
		if err != nil || strings.TrimSpace(name) == "" {
			return "FastHFS"
		}
		return name
	}()
)

type syslogMessage struct {
	level string
	text  string
	at    time.Time
}

// NormalizeSyslogTargets 校验并规范化 "IP:端口" 列表；未写端口时默认 514，并去除重复项。
func NormalizeSyslogTargets(targets []string) ([]string, error) {
	out := make([]string, 0, len(targets))
	seen := make(map[string]bool, len(targets))
	for _, raw := range targets {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		host, port, err := splitSyslogTarget(raw)
		if err != nil {
			return nil, err
		}
		addr := net.JoinHostPort(host, port)
		if seen[addr] {
			continue
		}
		seen[addr] = true
		out = append(out, addr)
	}
	return out, nil
}

func splitSyslogTarget(raw string) (string, string, error) {
	host, port, err := net.SplitHostPort(raw)
	if err != nil {
		if strings.Contains(raw, ":") {
			return "", "", fmt.Errorf("日志外发目标 %q 格式不正确，应为 IP:端口", raw)
		}
		host, port = raw, "514"
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if net.ParseIP(host) == nil {
		return "", "", fmt.Errorf("日志外发目标 %q 不是有效的 IP 地址", raw)
	}
	p, err := strconv.Atoi(strings.TrimSpace(port))
	if err != nil || p < 1 || p > 65535 {
		return "", "", fmt.Errorf("日志外发目标 %q 的端口不合法", raw)
	}
	return host, strconv.Itoa(p), nil
}

// ConfigureSyslog 应用日志外发配置；targets 需为 NormalizeSyslogTargets 的结果。
func ConfigureSyslog(enabled bool, targets []string) {
	if enabled {
		startSyslogWorker()
	}
	syslogMu.Lock()
	syslogEnabled = enabled
	syslogTargets = append([]string(nil), targets...)
	syslogMu.Unlock()
}

// SyslogEnabled 当前是否启用日志外发
func SyslogEnabled() bool {
	syslogMu.RLock()
	defer syslogMu.RUnlock()
	return syslogEnabled
}

func startSyslogWorker() {
	syslogOnce.Do(func() {
		syslogQueue = make(chan syslogMessage, syslogQueueSize)
		go func() {
			for m := range syslogQueue {
				sendSyslog(m.format())
			}
		}()
	})
}

func (m syslogMessage) format() string {
	severity := 6 // info
	switch m.level {
	case "warn":
		severity = 4
	case "error":
		severity = 3
	}
	// facility=1（user）
	pri := 1*8 + severity
	return fmt.Sprintf("<%d>1 %s %s FastHFS - - - %s", pri, m.at.Format(time.RFC3339), syslogHostname, m.text)
}

func sendSyslog(line string) {
	syslogMu.RLock()
	enabled := syslogEnabled
	targets := append([]string(nil), syslogTargets...)
	syslogMu.RUnlock()
	if !enabled {
		return
	}
	for _, addr := range targets {
		conn, err := net.DialTimeout("udp", addr, syslogDialTimout)
		if err != nil {
			continue
		}
		_, _ = conn.Write([]byte(line))
		_ = conn.Close()
	}
}

// enqueueSyslog 异步投递日志，队列满时丢弃，避免阻塞业务日志写入。
func enqueueSyslog(level, msg string, at time.Time) {
	syslogMu.RLock()
	enabled := syslogEnabled
	syslogMu.RUnlock()
	if !enabled || syslogQueue == nil {
		return
	}
	select {
	case syslogQueue <- syslogMessage{level: level, text: msg, at: at}:
	default:
	}
}
