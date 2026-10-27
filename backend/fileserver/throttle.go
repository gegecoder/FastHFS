package fileserver

import (
	"io"
	"net/http"
	"sync"
	"time"
)

// ipLimiter 单 IP 令牌桶限流（基础防爆破/爬虫）。
type ipLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64 // 每秒补充令牌
	capacity float64
	lastGC   time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newIPLimiter(perMinute int) *ipLimiter {
	if perMinute <= 0 {
		perMinute = 300
	}
	return &ipLimiter{
		buckets:  make(map[string]*bucket),
		rate:     float64(perMinute) / 60,
		capacity: float64(perMinute),
		lastGC:   time.Now(),
	}
}

// Allow 消耗一个令牌
func (l *ipLimiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.capacity, last: now}
		l.buckets[key] = b
	}
	b.tokens += now.Sub(b.last).Seconds() * l.rate
	if b.tokens > l.capacity {
		b.tokens = l.capacity
	}
	b.last = now
	// 定期清理空闲桶，避免内存增长
	if now.Sub(l.lastGC) > 10*time.Minute {
		for k, v := range l.buckets {
			if now.Sub(v.last) > 10*time.Minute {
				delete(l.buckets, k)
			}
		}
		l.lastGC = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// bandwidth 按字节速率限速（0 表示不限速）。
type bandwidth struct {
	limit int64 // 字节/秒
	start time.Time
	sent  int64
}

func newBandwidth(kbPerSec int) *bandwidth {
	if kbPerSec <= 0 {
		return &bandwidth{limit: 0}
	}
	return &bandwidth{limit: int64(kbPerSec) * 1024, start: time.Now()}
}

func (b *bandwidth) throttle(n int) {
	if b == nil || b.limit <= 0 || n <= 0 {
		return
	}
	if b.start.IsZero() {
		b.start = time.Now()
	}
	b.sent += int64(n)
	expected := time.Duration(float64(b.sent) / float64(b.limit) * float64(time.Second))
	if elapsed := time.Since(b.start); expected > elapsed {
		time.Sleep(expected - elapsed)
	}
}

// throttleWriter 限速响应写出
type throttleWriter struct {
	http.ResponseWriter
	bw *bandwidth
}

func (w *throttleWriter) Write(p []byte) (int, error) {
	if w.bw != nil {
		w.bw.throttle(len(p))
	}
	return w.ResponseWriter.Write(p)
}

// throttleReader 限速请求读取
type throttleReader struct {
	r  io.ReadCloser
	bw *bandwidth
}

func (r *throttleReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if n > 0 {
		r.bw.throttle(n)
	}
	return n, err
}

func (r *throttleReader) Close() error { return r.r.Close() }
