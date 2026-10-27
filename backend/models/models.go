// Package models 定义跨层共享的数据结构（同时用于 JSON API 与 Wails 绑定）。
package models

const (
	// AnonymousDeny 关闭匿名访问，必须登录
	AnonymousDeny = "deny"
	// AnonymousReadonly 匿名只读（浏览/下载）
	AnonymousReadonly = "readonly"
	// AnonymousUpload 匿名可浏览/下载/上传
	AnonymousUpload = "upload"

	// SharePreview 只读预览
	SharePreview = "preview"
	// ShareDownload 允许下载
	ShareDownload = "download"
	// ShareCollect 收集模式（仅允许上传）
	ShareCollect = "collect"
)

// Config 全局服务配置（存于 config 表）
type Config struct {
	ListenAddr      string   `json:"listenAddr"`      // 127.0.0.1 / 0.0.0.0 / 指定网卡 IPv4
	Port            int      `json:"port"`            // 监听端口
	DownloadLimitKB int      `json:"downloadLimitKB"` // 下载限速 KB/s，0=不限
	UploadLimitKB   int      `json:"uploadLimitKB"`   // 上传限速 KB/s，0=不限
	AnonymousMode   string   `json:"anonymousMode"`   // deny/readonly/upload
	Theme           string   `json:"theme"`           // light/dark
	MaxUploadMB     int64    `json:"maxUploadMB"`     // 单文件上限 MB，0=不限
	Blacklist       []string `json:"blacklist"`       // 上传后缀黑名单，如 ".exe"
	BasePath        string   `json:"basePath"`        // 反向代理子路径，如 /files
	RateLimitPerMin int      `json:"rateLimitPerMin"` // 单 IP 每分钟请求上限，0=默认
	AutoStart       bool     `json:"autoStart"`       // 开机自启
	SyslogEnabled   bool     `json:"syslogEnabled"`   // 是否启用日志外发
	SyslogTargets   []string `json:"syslogTargets"`   // 日志外发目标，格式 IP:端口
	Language        string   `json:"language"`        // 界面语言：zh / en
}

// DefaultConfig 默认配置：默认只监听本机，匿名只读
func DefaultConfig() Config {
	return Config{
		ListenAddr:      "127.0.0.1",
		Port:            8080,
		DownloadLimitKB: 0,
		UploadLimitKB:   0,
		AnonymousMode:   AnonymousReadonly,
		Theme:           "light",
		MaxUploadMB:     4096,
		Blacklist:       []string{".exe", ".bat", ".cmd", ".com", ".scr", ".msi", ".ps1", ".sh"},
		BasePath:        "",
		RateLimitPerMin: 300,
		AutoStart:       false,
		Language:        "zh",
	}
}

// NetworkInterface 本机网卡信息，用于把服务绑定到指定网卡的 IPv4 地址
type NetworkInterface struct {
	Name      string   `json:"name"`      // 系统网卡名，如 WLAN、以太网、eth0
	Index     int      `json:"index"`     // 网卡序号
	Loopback  bool     `json:"loopback"`  // 是否回环网卡
	Addresses []string `json:"addresses"` // 网卡上的 IPv4 地址（已排除链路本地地址）
}

// Mount 挂载目录
type Mount struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Alias         string   `json:"alias"`
	Root          string   `json:"root"`
	AllowBrowse   bool     `json:"allowBrowse"`
	AllowDownload bool     `json:"allowDownload"`
	AllowUpload   bool     `json:"allowUpload"`
	AllowDelete   bool     `json:"allowDelete"`
	IgnoreRules   []string `json:"ignoreRules"`
	CreatedAt     int64    `json:"createdAt"`
}

// User 账号（V1 全局权限）
type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	CreatedAt    int64  `json:"createdAt"`
	LastLoginAt  int64  `json:"lastLoginAt"`
}

// Share 分享链接
type Share struct {
	ID           string `json:"id"`
	Token        string `json:"token"`
	MountID      string `json:"mountId"`
	MountAlias   string `json:"mountAlias"`
	MountName    string `json:"mountName"`
	Path         string `json:"path"`
	PasswordHash string `json:"-"`
	HasPassword  bool   `json:"hasPassword"`
	ExpiresAt    int64  `json:"expiresAt"` // Unix 秒，0=永久
	MaxVisits    int64  `json:"maxVisits"` // 0=不限
	Visits       int64  `json:"visits"`
	Mode         string `json:"mode"`
	CreatedBy    string `json:"createdBy"`
	CreatedAt    int64  `json:"createdAt"`
}

// TempTask tus 分片临时任务
type TempTask struct {
	ID        string `json:"id"`
	MountID   string `json:"mountId"`
	RelDir    string `json:"relDir"`
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	Offset    int64  `json:"offset"`
	TempPath  string `json:"tempPath"`
	CreatedAt int64  `json:"createdAt"`
	ShareID   string `json:"-"`
}

// APIResult 简单结果
type APIResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Entry 目录项
type Entry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
	MIME    string `json:"mime"`
}

// ListResult 目录列表响应
type ListResult struct {
	Mount       string  `json:"mount"`
	MountName   string  `json:"mountName"`
	Path        string  `json:"path"`
	Parent      string  `json:"parent"`
	Entries     []Entry `json:"entries"`
	CanUpload   bool    `json:"canUpload"`
	CanDelete   bool    `json:"canDelete"`
	CanDownload bool    `json:"canDownload"`
	CanShare    bool    `json:"canShare"`
}

// ServiceStatus 服务状态
type ServiceStatus struct {
	Running   bool     `json:"running"`
	StartedAt int64    `json:"startedAt"`
	Listen    string   `json:"listen"`
	URLs      []string `json:"urls"`
	BasePath  string   `json:"basePath"`
}

// MountOption 公开站点可见挂载
type MountOption struct {
	Alias         string `json:"alias"`
	Name          string `json:"name"`
	AllowBrowse   bool   `json:"allowBrowse"`
	AllowDownload bool   `json:"allowDownload"`
	AllowUpload   bool   `json:"allowUpload"`
}

// MountsResult 公开站点挂载列表响应
type MountsResult struct {
	AuthRequired bool          `json:"authRequired"`
	Anonymous    string        `json:"anonymous"`
	Mounts       []MountOption `json:"mounts"`
}

// AuthInfo 当前登录态
type AuthInfo struct {
	Authenticated bool   `json:"authenticated"`
	Username      string `json:"username"`
	AnonymousMode string `json:"anonymousMode"`
	CanLogin      bool   `json:"canLogin"`
}

// ShareCreateRequest 创建分享请求
type ShareCreateRequest struct {
	MountID   string `json:"mountId"`
	Mount     string `json:"mount"` // 挂载别名（对外站点使用，优先于 MountID）
	Path      string `json:"path"`
	Password  string `json:"password"`
	ExpiresIn int64  `json:"expiresIn"` // 小时，0=永久
	MaxVisits int64  `json:"maxVisits"` // 0=不限
	Mode      string `json:"mode"`
}

// ShareInfo 分享公开信息
type ShareInfo struct {
	Token         string `json:"token"`
	MountName     string `json:"mountName"`
	Path          string `json:"path"`
	Mode          string `json:"mode"`
	HasPassword   bool   `json:"hasPassword"`
	ExpiresAt     int64  `json:"expiresAt"`
	MaxVisits     int64  `json:"maxVisits"`
	Visits        int64  `json:"visits"`
	Authenticated bool   `json:"authenticated"`
}

// ShareUnlockRequest 分享解锁请求
type ShareUnlockRequest struct {
	Password string `json:"password"`
}

// ShareCreated 创建分享结果
type ShareCreated struct {
	Share Share  `json:"share"`
	URL   string `json:"url"`
}
