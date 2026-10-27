package fileserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"FastHFS/backend/applog"
	"FastHFS/backend/fileserver/www"
	"FastHFS/backend/models"
	"FastHFS/backend/store"
)

// Server 管理对外 HTTP 文件服务生命周期与路由。
type Server struct {
	store     *store.Store
	jwtSecret []byte

	cfgPtr atomic.Pointer[models.Config]

	mu         sync.Mutex
	httpServer *http.Server
	listener   net.Listener
	running    bool
	startedAt  time.Time
	onStatus   func(models.ServiceStatus)

	limiter     *ipLimiter
	authLimiter *ipLimiter
}

// New 创建服务管理器
func New(st *store.Store, onStatus func(models.ServiceStatus)) (*Server, error) {
	cfg, err := st.GetConfig()
	if err != nil {
		return nil, err
	}
	secret, err := st.EnsureJWTSecret()
	if err != nil {
		return nil, err
	}
	s := &Server{
		store:       st,
		jwtSecret:   secret,
		onStatus:    onStatus,
		limiter:     newIPLimiter(cfg.RateLimitPerMin),
		authLimiter: newIPLimiter(20),
	}
	s.cfgPtr.Store(&cfg)
	return s, nil
}

func (s *Server) cfg() *models.Config { return s.cfgPtr.Load() }

// UpdateConfig 应用最新配置（服务运行中需重启才能切换监听地址/端口）。
func (s *Server) UpdateConfig(cfg models.Config) {
	s.cfgPtr.Store(&cfg)
}

// ReloadConfig 从数据库重新读取配置
func (s *Server) ReloadConfig() error {
	cfg, err := s.store.GetConfig()
	if err != nil {
		return err
	}
	s.UpdateConfig(cfg)
	return nil
}

// IsRunning 服务是否运行中
func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Start 启动 HTTP 服务
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return errors.New("服务已在运行中")
	}
	cfg := *s.cfg()
	listenAddr, err := NormalizeListenAddr(cfg.ListenAddr)
	if err != nil {
		return err
	}
	cfg.ListenAddr = listenAddr
	s.cfgPtr.Store(&cfg)
	addr := net.JoinHostPort(listenAddr, strconv.Itoa(cfg.Port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if listenAddr != "0.0.0.0" && listenAddr != "127.0.0.1" {
			return fmt.Errorf("无法绑定网卡地址 %s 的端口 %d（网卡可能已断开或端口被占用）: %w", listenAddr, cfg.Port, err)
		}
		return fmt.Errorf("端口 %d 监听失败，可能已被占用: %w", cfg.Port, err)
	}
	s.limiter = newIPLimiter(cfg.RateLimitPerMin)
	s.authLimiter = newIPLimiter(20)

	router := s.buildRouter(cfg)
	handler := http.Handler(router)
	if cfg.BasePath != "" {
		handler = &basePathHandler{base: cfg.BasePath, next: router}
	}
	hs := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	s.httpServer = hs
	s.listener = ln
	s.running = true
	s.startedAt = time.Now()

	go func() {
		if err := hs.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			applog.Errorf("server", "", "", "HTTP 服务异常退出: %v", err)
		}
		s.mu.Lock()
		if s.httpServer == hs {
			s.running = false
		}
		s.mu.Unlock()
		s.emitStatus()
	}()
	go s.cleanupLoop()

	applog.Infof("server", "", "", "已启动 http://%s%s", addr, cfg.BasePath)
	s.emitStatusLocked()
	return nil
}

// Stop 优雅关闭服务
func (s *Server) Stop() error {
	s.mu.Lock()
	hs := s.httpServer
	if !s.running || hs == nil {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	s.httpServer = nil
	s.listener = nil
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := hs.Shutdown(ctx)
	applog.Infof("server", "", "", "服务已停止")
	s.emitStatus()
	return err
}

// Status 当前状态
func (s *Server) Status() models.ServiceStatus {
	s.mu.Lock()
	running := s.running
	startedAt := s.startedAt
	s.mu.Unlock()
	cfg := s.cfg()
	st := models.ServiceStatus{
		Running:  running,
		Listen:   cfg.ListenAddr,
		BasePath: cfg.BasePath,
	}
	if running {
		st.StartedAt = startedAt.Unix()
		st.URLs = s.urls(cfg)
	}
	return st
}

// URLs 当前可访问地址（运行中）
func (s *Server) URLs() []string {
	return s.urls(s.cfg())
}

func (s *Server) urls(cfg *models.Config) []string {
	port := strconv.Itoa(cfg.Port)
	var hosts []string
	if cfg.ListenAddr == "" || cfg.ListenAddr == "0.0.0.0" {
		hosts = LocalIPv4()
		if len(hosts) == 0 {
			hosts = []string{"127.0.0.1"}
		}
	} else {
		// 绑定指定网卡（或本机回环）时，只有该地址可访问
		hosts = []string{cfg.ListenAddr}
	}
	out := make([]string, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, fmt.Sprintf("http://%s:%s%s", h, port, cfg.BasePath))
	}
	sort.Strings(out)
	return out
}

// PrimaryURL 首选访问地址
func (s *Server) PrimaryURL() string {
	urls := s.URLs()
	if len(urls) == 0 {
		cfg := s.cfg()
		return fmt.Sprintf("http://127.0.0.1:%d%s", cfg.Port, cfg.BasePath)
	}
	// 优先非回环地址（局域网分享场景）
	for _, u := range urls {
		if !strings.Contains(u, "127.0.0.1") {
			return u
		}
	}
	return urls[0]
}

func (s *Server) emitStatus() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emitStatusLocked()
}

func (s *Server) emitStatusLocked() {
	if s.onStatus == nil {
		return
	}
	cfg := s.cfg()
	st := models.ServiceStatus{
		Running:  s.running,
		Listen:   cfg.ListenAddr,
		BasePath: cfg.BasePath,
	}
	if s.running {
		st.StartedAt = s.startedAt.Unix()
		st.URLs = s.urls(cfg)
	}
	go s.onStatus(st)
}

// LocalIPv4 返回本机非回环 IPv4 地址
func LocalIPv4() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil || ip4.IsLoopback() || ip4.IsLinkLocalUnicast() {
				continue
			}
			if seen[ip4.String()] {
				continue
			}
			seen[ip4.String()] = true
			out = append(out, ip4.String())
		}
	}
	sort.Strings(out)
	return out
}

// NetworkInterfaces 返回本机可用于绑定监听的网卡（含回环网卡）及其 IPv4 地址
func NetworkInterfaces() []models.NetworkInterface {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make([]models.NetworkInterface, 0, len(ifaces))
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		ips := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil || ip4.IsLinkLocalUnicast() {
				continue
			}
			ips = append(ips, ip4.String())
		}
		if len(ips) == 0 {
			continue
		}
		sort.Strings(ips)
		out = append(out, models.NetworkInterface{
			Name:      iface.Name,
			Index:     iface.Index,
			Loopback:  iface.Flags&net.FlagLoopback != 0,
			Addresses: ips,
		})
	}
	return out
}

// NormalizeListenAddr 校验并规范化监听地址：0.0.0.0、127.0.0.1 或本机网卡 IPv4
func NormalizeListenAddr(addr string) (string, error) {
	v := strings.TrimSpace(addr)
	if v == "" || strings.EqualFold(v, "localhost") {
		return "127.0.0.1", nil
	}
	ip := net.ParseIP(v)
	if ip == nil || ip.To4() == nil {
		return "", errors.New("监听地址必须是 IPv4 地址")
	}
	v4 := ip.To4().String()
	if v4 == "0.0.0.0" || v4 == "127.0.0.1" {
		return v4, nil
	}
	for _, iface := range NetworkInterfaces() {
		for _, local := range iface.Addresses {
			if local == v4 {
				return v4, nil
			}
		}
	}
	return "", fmt.Errorf("地址 %s 不是本机网卡地址，请选择当前可用的网卡", v4)
}

// buildRouter 构建 Gin 路由
func (s *Server) buildRouter(cfg models.Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.SetTrustedProxies(nil)
	r.Use(gin.Recovery(), s.accessLog(), securityHeaders(), s.rateLimit())

	api := r.Group("/api")
	api.Use(s.authMiddleware())
	api.GET("/auth/me", s.handleMe)
	api.POST("/auth/login", s.strictLimit(), s.handleLogin)
	api.POST("/auth/logout", s.handleLogout)

	api.GET("/mounts", s.handleMounts)
	api.GET("/list", s.handleList)
	api.GET("/dl", s.handleDownload)
	api.GET("/zip", s.handleZip)
	api.POST("/mkdir", s.handleMkdir)
	api.POST("/rename", s.handleRename)
	api.POST("/delete", s.handleDelete)
	api.POST("/share", s.requireUser(), s.handleCreateShare)

	// 分片上传（tus）
	api.OPTIONS("/tus/create/:mount", s.handleTusOptions)
	api.POST("/tus/create/:mount", s.handleTusCreate)
	api.OPTIONS("/tus/uploads/:id", s.handleTusOptions)
	api.HEAD("/tus/uploads/:id", s.handleTusHead)
	api.PATCH("/tus/uploads/:id", s.handleTusPatch)
	api.DELETE("/tus/uploads/:id", s.handleTusDelete)

	// 分享公开信息与解锁
	api.GET("/s/:token/info", s.handleShareInfo)
	api.POST("/s/:token/unlock", s.strictLimit(), s.handleShareUnlock)

	// 分享内容（需分享会话）
	sg := api.Group("/s/:token")
	sg.Use(s.shareMiddleware())
	sg.GET("/list", s.handleShareList)
	sg.GET("/dl", s.handleShareDownload)
	sg.GET("/zip", s.handleShareZip)
	sg.OPTIONS("/tus/create", s.handleTusOptions)
	sg.POST("/tus/create", s.handleShareTusCreate)
	sg.OPTIONS("/tus/uploads/:id", s.handleTusOptions)
	sg.HEAD("/tus/uploads/:id", s.handleTusHead)
	sg.PATCH("/tus/uploads/:id", s.handleTusPatch)
	sg.DELETE("/tus/uploads/:id", s.handleTusDelete)

	r.NoRoute(s.servePublic)
	return r
}

// accessLog 仅记录异常状态，避免正常请求刷日志
func (s *Server) accessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if c.Writer.Status() >= 400 && !strings.HasPrefix(c.Request.URL.Path, "/api/dl") {
			applog.Warnf("http", c.ClientIP(), s.access(c).Username, "%s %s → %d", c.Request.Method, c.Request.URL.Path, c.Writer.Status())
		}
	}
}

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		c.Next()
	}
}

func (s *Server) rateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Next()
			return
		}
		if !s.limiter.Allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "请求过于频繁，请稍后再试", "code": "rate_limited"})
			return
		}
		c.Next()
	}
}

func (s *Server) strictLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.authLimiter.Allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "尝试次数过多，请稍后再试", "code": "rate_limited"})
			return
		}
		c.Next()
	}
}

// servePublic 对外站点静态资源与 SPA 回退
func (s *Server) servePublic(c *gin.Context) {
	p := c.Request.URL.Path
	if strings.HasPrefix(p, "/api/") {
		c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
		return
	}
	dist, err := fs.Sub(www.Dist, "dist")
	if err != nil {
		c.String(http.StatusInternalServerError, "站点资源缺失")
		return
	}
	clean := strings.TrimPrefix(path.Clean("/"+p), "/")
	if clean != "" {
		if info, err := fs.Stat(dist, clean); err == nil && !info.IsDir() {
			if strings.HasPrefix(clean, "assets/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				c.Header("Cache-Control", "no-cache")
			}
			if ct := mime.TypeByExtension(path.Ext(clean)); ct != "" {
				c.Header("Content-Type", ct)
			}
			http.ServeFileFS(c.Writer, c.Request, dist, clean)
			return
		}
	}
	data, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		c.String(http.StatusNotFound, "页面不存在")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", injectPublicBase(data, s.cfg().BasePath, s.cfg().Language))
}

// injectPublicBase 向 index.html 注入部署子路径（<base href> + 全局变量）与界面语言，
// 使相对资源路径、前端 API 请求与文案在多语言/子路径部署下正确解析。
func injectPublicBase(page []byte, basePath, language string) []byte {
	base := strings.TrimRight(basePath, "/")
	quoted, err := json.Marshal(base)
	if err != nil {
		quoted = []byte(`""`)
	}
	if language != "en" {
		language = "zh"
	}
	lang, _ := json.Marshal(language)
	href := html.EscapeString(base + "/")
	inject := fmt.Sprintf(
		"\n    <base href=\"%s\" />\n    <script>window.__FASTHFS_BASE__ = %s; window.__FASTHFS_LANG__ = %s;</script>",
		href, quoted, lang,
	)
	return []byte(strings.Replace(string(page), "<head>", "<head>"+inject, 1))
}

// basePathHandler 支持反向代理子路径部署
type basePathHandler struct {
	base string
	next http.Handler
}

func (h *basePathHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	if p == h.base {
		http.Redirect(w, r, h.base+"/", http.StatusMovedPermanently)
		return
	}
	if !strings.HasPrefix(p, h.base+"/") {
		http.NotFound(w, r)
		return
	}
	r2 := r.Clone(r.Context())
	r2.URL.Path = strings.TrimPrefix(p, h.base)
	h.next.ServeHTTP(w, r2)
}

func (s *Server) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	s.cleanupOnce()
	for {
		s.mu.Lock()
		running := s.running
		s.mu.Unlock()
		if !running {
			return
		}
		<-ticker.C
		s.cleanupOnce()
	}
}

func (s *Server) cleanupOnce() {
	if n, err := s.store.CleanupExpiredShares(); err != nil {
		applog.Warnf("cleanup", "", "", "清理过期分享失败: %v", err)
	} else if n > 0 {
		applog.Infof("cleanup", "", "", "已清理 %d 条过期/用尽分享", n)
	}
	if n, err := s.store.CleanupStaleTempTasks(24 * time.Hour); err != nil {
		applog.Warnf("cleanup", "", "", "清理分片临时文件失败: %v", err)
	} else if n > 0 {
		applog.Infof("cleanup", "", "", "已清理 %d 个超时上传任务", n)
	}
}
