// app.go Wails v3 应用服务：管理面板后端 API、服务生命周期、托盘与退出流程。
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"golang.org/x/crypto/bcrypt"

	"FastHFS/backend/appconfig"
	"FastHFS/backend/applog"
	"FastHFS/backend/fileserver"
	"FastHFS/backend/models"
	"FastHFS/backend/store"
	"FastHFS/backend/tray"
	"FastHFS/backend/update"
	"FastHFS/backend/windowicon"
)

// App Wails 服务：管理面板与托盘共用的后端入口
type App struct {
	appCfg  appconfig.AppConfig
	store   *store.Store
	server  *fileserver.Server
	iconPNG []byte

	app  *application.App
	tray *tray.Controller

	quitting     atomic.Bool
	quitOnce     sync.Once
	shutdownOnce sync.Once
}

// NewApp 初始化存储与 HTTP 服务管理器
func NewApp(cfg appconfig.AppConfig, iconPNG []byte) (*App, error) {
	st, err := store.New(cfg.Name)
	if err != nil {
		return nil, fmt.Errorf("初始化数据存储失败: %w", err)
	}
	a := &App{
		appCfg:  cfg,
		store:   st,
		iconPNG: iconPNG,
	}
	srv, err := fileserver.New(st, func(status models.ServiceStatus) {
		a.onServiceStatus(status)
	})
	if err != nil {
		_ = st.Close()
		return nil, fmt.Errorf("初始化文件服务失败: %w", err)
	}
	a.server = srv
	// 应用持久化的日志外发配置（配置非法时保持关闭，避免启动失败）
	if cfg, cfgErr := st.GetConfig(); cfgErr == nil {
		if targets, normErr := applog.NormalizeSyslogTargets(cfg.SyslogTargets); normErr == nil {
			applog.ConfigureSyslog(cfg.SyslogEnabled, targets)
		}
	}
	return a, nil
}

// ServiceStartup Wails Service 启动钩子
func (a *App) ServiceStartup(ctx context.Context, opts application.ServiceOptions) error {
	a.app = application.Get()
	a.tray = tray.NewController(a.iconPNG, a.iconPNG,
		a.appCfg.Name, "FastHFS - 你的文件服务助手",
		&a.quitting,
		tray.Actions{
			StartService: func() { a.StartService() },
			StopService:  func() { a.StopService() },
			OpenPanel:    func() { a.ShowPanel() },
			OpenURL:      a.openAccessURL,
			CopyURL:      a.copyAccessURL,
			Quit:         func() { a.RequestQuit() },
		})
	a.tray.Bind(a.app)
	a.tray.Start()
	a.tray.SetRunning(a.server.IsRunning())

	const windowTitle = "FastHFS"
	windowicon.ApplyWindowIcon(windowTitle, a.iconPNG)
	go func() {
		t := time.NewTimer(600 * time.Millisecond)
		defer t.Stop()
		<-t.C
		windowicon.ApplyWindowIcon(windowTitle, a.iconPNG)
	}()
	applog.Infof("app", "", "", "%s v%s 已就绪，数据目录 %s", a.appCfg.Name, a.appCfg.Version, a.store.BaseDir())
	return nil
}

// ServiceShutdown Wails Service 关闭钩子
func (a *App) ServiceShutdown() error {
	a.shutdownOnce.Do(func() {
		applog.Infof("app", "", "", "正在关闭服务与托盘…")
		if a.server != nil {
			_ = a.server.Stop()
		}
		if a.tray != nil {
			a.tray.Stop()
		}
		if a.store != nil {
			_ = a.store.Close()
		}
		applog.Infof("app", "", "", "清理完成")
	})
	return nil
}

// onServiceStatus 服务状态变化：推送前端事件并同步托盘图标
func (a *App) onServiceStatus(status models.ServiceStatus) {
	if a.tray != nil {
		a.tray.SetRunning(status.Running)
	}
	if a.app == nil {
		return
	}
	func() {
		defer func() { _ = recover() }()
		a.app.Event.Emit("service:status", status)
	}()
}

// InstallCloseHandler 关闭窗口时隐藏到托盘（托盘常驻）
func (a *App) InstallCloseHandler(w *application.WebviewWindow) {
	if w == nil {
		return
	}
	w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if a.quitting.Load() {
			return
		}
		e.Cancel()
		go func() {
			defer func() { _ = recover() }()
			w.Hide()
		}()
	})
}

// ShowPanel 显示管理面板窗口
func (a *App) ShowPanel() {
	go func() {
		defer func() { _ = recover() }()
		if a.app == nil {
			return
		}
		if w, ok := a.app.Window.Get("main"); ok {
			w.Show()
			w.UnMinimise()
			w.Focus()
		}
	}()
}

// RequestQuit 退出应用：优雅关闭服务后结束进程
func (a *App) RequestQuit() {
	a.quitOnce.Do(func() {
		a.quitting.Store(true)
		go func() {
			defer func() { _ = recover() }()
			applog.Infof("app", "", "", "用户请求退出")
			_ = a.ServiceShutdown()
			applog.Close()
			if a.app != nil {
				a.app.Quit()
			}
			time.AfterFunc(3*time.Second, func() {
				applog.Infof("app", "", "", "正常退出超时，强制结束进程")
				os.Exit(0)
			})
		}()
	})
}

func (a *App) openAccessURL() {
	if !a.server.IsRunning() {
		return
	}
	_ = a.app.Browser.OpenURL(a.server.PrimaryURL())
}

func (a *App) copyAccessURL() {
	if !a.server.IsRunning() {
		return
	}
	if a.app.Clipboard.SetText(a.server.PrimaryURL()) {
		applog.Infof("app", "", "", "已复制访问地址 %s", a.server.PrimaryURL())
	}
}

// === 应用信息 ===

// Version 应用名称与版本
func (a *App) Version() map[string]string {
	return map[string]string{
		"name":        a.appCfg.Name,
		"version":     a.appCfg.Version,
		"description": a.appCfg.Description,
		"homepage":    a.appCfg.Homepage,
		"copyright":   a.appCfg.Copyright,
		"author":      a.appCfg.Author.Name,
		"blog":        a.appCfg.Author.Blog,
		"dataDir":     a.store.BaseDir(),
	}
}

// CheckForUpdate 检测新版本
func (a *App) CheckForUpdate() update.Info {
	return update.CheckForUpdate(a.appCfg)
}

// GetRecentLogs 最近运行日志
func (a *App) GetRecentLogs() []applog.Entry { return applog.Recent() }

// ClearLogs 清空运行日志
func (a *App) ClearLogs() { applog.Clear() }

// OpenLogDir 打开日志目录
func (a *App) OpenLogDir() models.APIResult {
	if err := a.app.Browser.OpenFile(applog.Dir()); err != nil {
		return models.APIResult{Success: false, Message: "打开日志目录失败：" + err.Error()}
	}
	return models.APIResult{Success: true, Message: applog.Dir()}
}

// === 服务配置与生命周期 ===

// GetConfig 当前配置
func (a *App) GetConfig() models.Config {
	cfg, err := a.store.GetConfig()
	if err != nil {
		return models.DefaultConfig()
	}
	return cfg
}

// SaveConfig 保存配置；监听地址/端口/子路径变更需重启服务生效
func (a *App) SaveConfig(cfg models.Config) models.APIResult {
	old, _ := a.store.GetConfig()
	listenAddr, err := fileserver.NormalizeListenAddr(cfg.ListenAddr)
	if err != nil {
		return models.APIResult{Success: false, Message: err.Error()}
	}
	cfg.ListenAddr = listenAddr
	cfg.BasePath = strings.TrimSpace(cfg.BasePath)
	targets, err := applog.NormalizeSyslogTargets(cfg.SyslogTargets)
	if err != nil {
		return models.APIResult{Success: false, Message: err.Error()}
	}
	if cfg.SyslogEnabled && len(targets) == 0 {
		return models.APIResult{Success: false, Message: "启用日志外发时请至少配置一个目标地址"}
	}
	cfg.SyslogTargets = targets
	if err := a.store.SaveConfig(cfg); err != nil {
		return models.APIResult{Success: false, Message: "保存配置失败：" + err.Error()}
	}
	saved, err := a.store.GetConfig()
	if err != nil {
		return models.APIResult{Success: false, Message: "读取配置失败：" + err.Error()}
	}
	a.server.UpdateConfig(saved)
	applog.ConfigureSyslog(saved.SyslogEnabled, saved.SyslogTargets)
	basePath := saved.BasePath
	if basePath == "" {
		basePath = "/"
	}
	applog.Infof("config", "", "", "保存设置：监听 %s:%d，子路径 %s", saved.ListenAddr, saved.Port, basePath)
	if saved.SyslogEnabled {
		applog.Infof("config", "", "", "日志外发已启用，目标 %s", strings.Join(saved.SyslogTargets, "、"))
	} else {
		applog.Infof("config", "", "", "日志外发已关闭")
	}
	restart := old.Port != saved.Port || old.ListenAddr != saved.ListenAddr || old.BasePath != saved.BasePath
	if restart && a.server.IsRunning() {
		return models.APIResult{Success: true, Message: "配置已保存，监听地址/端口/子路径变更需重启服务生效"}
	}
	return models.APIResult{Success: true, Message: "配置已保存"}
}

// GetStatus 服务状态
func (a *App) GetStatus() models.ServiceStatus { return a.server.Status() }

// StartService 启动 HTTP 服务（端口被占用时自动尝试相邻端口）
func (a *App) StartService() models.APIResult {
	if a.server.IsRunning() {
		return models.APIResult{Success: true, Message: "服务已在运行中"}
	}
	err := a.server.Start()
	if err == nil {
		return models.APIResult{Success: true, Message: "服务已启动"}
	}
	cfg, cfgErr := a.store.GetConfig()
	if cfgErr != nil {
		return models.APIResult{Success: false, Message: err.Error()}
	}
	if !a.portInUse(cfg.Port) {
		return models.APIResult{Success: false, Message: err.Error()}
	}
	free := a.findFreePort(cfg.Port + 1)
	if free == 0 {
		return models.APIResult{Success: false, Message: fmt.Sprintf("端口 %d 已被占用，且未找到可用端口", cfg.Port)}
	}
	oldPort := cfg.Port
	cfg.Port = free
	if saveErr := a.store.SaveConfig(cfg); saveErr != nil {
		return models.APIResult{Success: false, Message: "端口被占用，自动切换端口失败：" + saveErr.Error()}
	}
	a.server.UpdateConfig(cfg)
	if startErr := a.server.Start(); startErr != nil {
		return models.APIResult{Success: false, Message: startErr.Error()}
	}
	return models.APIResult{Success: true, Message: fmt.Sprintf("端口 %d 已被占用，已自动切换到 %d", oldPort, free)}
}

// StopService 停止 HTTP 服务
func (a *App) StopService() models.APIResult {
	if !a.server.IsRunning() {
		return models.APIResult{Success: true, Message: "服务未运行"}
	}
	if err := a.server.Stop(); err != nil {
		return models.APIResult{Success: false, Message: "停止服务失败：" + err.Error()}
	}
	return models.APIResult{Success: true, Message: "服务已停止"}
}

// RestartService 重启 HTTP 服务
func (a *App) RestartService() models.APIResult {
	_ = a.server.Stop()
	return a.StartService()
}

// GetURLs 当前可访问地址
func (a *App) GetURLs() []string { return a.server.URLs() }

// GetPrimaryURL 首选访问地址
func (a *App) GetPrimaryURL() string { return a.server.PrimaryURL() }

// GetLocalIPs 本机局域网 IPv4 地址
func (a *App) GetLocalIPs() []string {
	ips := fileserver.LocalIPv4()
	if len(ips) == 0 {
		return []string{"127.0.0.1"}
	}
	return ips
}

// GetNetworkInterfaces 本机可用网卡（含回环网卡）及其 IPv4 地址，用于绑定指定网卡监听
func (a *App) GetNetworkInterfaces() []models.NetworkInterface {
	ifaces := fileserver.NetworkInterfaces()
	if ifaces == nil {
		return []models.NetworkInterface{}
	}
	return ifaces
}

// CheckPortInUse 检测端口是否已被占用
func (a *App) CheckPortInUse(port int) bool { return a.portInUse(port) }

func (a *App) portInUse(port int) bool {
	if port < 1 || port > 65535 {
		return false
	}
	for _, host := range []string{"0.0.0.0", "127.0.0.1"} {
		ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
		if err != nil {
			return true
		}
		_ = ln.Close()
	}
	return false
}

// FindFreePort 从 start 开始查找可用端口，找不到返回 0
func (a *App) FindFreePort(start int) int { return a.findFreePort(start) }

func (a *App) findFreePort(start int) int {
	if start < 1 {
		start = 1024
	}
	for port := start; port <= 65535 && port < start+200; port++ {
		if !a.portInUse(port) {
			return port
		}
	}
	return 0
}

// CopyText 复制文本到剪贴板
func (a *App) CopyText(text string) models.APIResult {
	if !a.app.Clipboard.SetText(text) {
		return models.APIResult{Success: false, Message: "复制失败"}
	}
	return models.APIResult{Success: true, Message: "已复制"}
}

// OpenURL 使用系统浏览器打开地址
func (a *App) OpenURL(url string) models.APIResult {
	if !strings.HasPrefix(strings.ToLower(url), "http://") && !strings.HasPrefix(strings.ToLower(url), "https://") {
		return models.APIResult{Success: false, Message: "地址不合法"}
	}
	if err := a.app.Browser.OpenURL(url); err != nil {
		return models.APIResult{Success: false, Message: "打开浏览器失败：" + err.Error()}
	}
	return models.APIResult{Success: true, Message: "已打开"}
}

// PickDirectory 选择本地文件夹，取消返回空串
func (a *App) PickDirectory(title string) string {
	if strings.TrimSpace(title) == "" {
		title = "选择要挂载的文件夹"
	}
	path, err := a.app.Dialog.OpenFile().
		SetTitle(title).
		CanChooseFiles(false).
		CanChooseDirectories(true).
		PromptForSingleSelection()
	if err != nil {
		return ""
	}
	return path
}

// === 挂载管理 ===

var aliasRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// ListMounts 全部挂载点
func (a *App) ListMounts() []models.Mount {
	mounts, err := a.store.ListMounts()
	if err != nil {
		applog.Errorf("mount", "", "", "读取挂载点失败: %v", err)
		return []models.Mount{}
	}
	return mounts
}

func normalizeMount(m models.Mount) (models.Mount, error) {
	m.Name = strings.TrimSpace(m.Name)
	m.Alias = strings.TrimSpace(m.Alias)
	m.Root = strings.TrimSpace(m.Root)
	if m.Name == "" {
		return m, errors.New("挂载名称不能为空")
	}
	if !aliasRe.MatchString(m.Alias) {
		return m, errors.New("访问别名只能包含字母、数字、点、下划线和短横线，且需以字母或数字开头")
	}
	if strings.EqualFold(m.Alias, "api") {
		return m, errors.New("访问别名不能使用 api")
	}
	if m.Root == "" {
		return m, errors.New("请选择要挂载的目录")
	}
	abs, err := filepath.Abs(filepath.Clean(m.Root))
	if err != nil {
		return m, errors.New("目录路径不合法")
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return m, errors.New("目录不存在或不是文件夹")
	}
	m.Root = abs
	if m.IgnoreRules == nil {
		m.IgnoreRules = []string{}
	}
	return m, nil
}

// AddMount 新增挂载点
func (a *App) AddMount(m models.Mount) (models.Mount, error) {
	normalized, err := normalizeMount(m)
	if err != nil {
		return models.Mount{}, err
	}
	if existing, err := a.store.GetMountByAlias(normalized.Alias); err == nil && existing.ID != "" {
		return models.Mount{}, errors.New("访问别名已被使用")
	}
	mounts, err := a.store.ListMounts()
	if err != nil {
		return models.Mount{}, err
	}
	for _, item := range mounts {
		if strings.EqualFold(filepath.Clean(item.Root), filepath.Clean(normalized.Root)) {
			return models.Mount{}, errors.New("该目录已被挂载")
		}
	}
	normalized.ID = ""
	normalized.CreatedAt = 0
	created, err := a.store.AddMount(normalized)
	if err != nil {
		return models.Mount{}, err
	}
	applog.Infof("mount", "", "", "新增挂载 %s → %s", created.Alias, created.Root)
	return created, nil
}

// UpdateMount 更新挂载点
func (a *App) UpdateMount(m models.Mount) (models.Mount, error) {
	if strings.TrimSpace(m.ID) == "" {
		return models.Mount{}, errors.New("缺少挂载点 ID")
	}
	if _, err := a.store.GetMount(m.ID); err != nil {
		return models.Mount{}, errors.New("挂载点不存在")
	}
	normalized, err := normalizeMount(m)
	if err != nil {
		return models.Mount{}, err
	}
	if existing, err := a.store.GetMountByAlias(normalized.Alias); err == nil && existing.ID != m.ID {
		return models.Mount{}, errors.New("访问别名已被使用")
	}
	mounts, err := a.store.ListMounts()
	if err != nil {
		return models.Mount{}, err
	}
	for _, item := range mounts {
		if item.ID != m.ID && strings.EqualFold(filepath.Clean(item.Root), filepath.Clean(normalized.Root)) {
			return models.Mount{}, errors.New("该目录已被其他挂载点使用")
		}
	}
	if err := a.store.UpdateMount(normalized); err != nil {
		return models.Mount{}, err
	}
	updated, err := a.store.GetMount(m.ID)
	if err != nil {
		return models.Mount{}, err
	}
	applog.Infof("mount", "", "", "更新挂载 %s → %s", updated.Alias, updated.Root)
	return updated, nil
}

// DeleteMount 删除挂载点
func (a *App) DeleteMount(id string) models.APIResult {
	if err := a.store.DeleteMount(id); err != nil {
		return models.APIResult{Success: false, Message: "删除失败：" + err.Error()}
	}
	applog.Infof("mount", "", "", "删除挂载 %s", id)
	return models.APIResult{Success: true, Message: "挂载点已删除"}
}

// ListMountDir 管理端浏览挂载目录（用于创建分享时选择路径）
func (a *App) ListMountDir(mountID, path string) ([]models.Entry, error) {
	m, err := a.store.GetMount(mountID)
	if err != nil {
		return nil, errors.New("挂载点不存在")
	}
	return fileserver.ListMountEntries(m, path)
}

// === 账号管理 ===

// ListUsers 账号列表
func (a *App) ListUsers() []models.User {
	users, err := a.store.ListUsers()
	if err != nil {
		return []models.User{}
	}
	return users
}

// AddUser 新增账号
func (a *App) AddUser(username, password string) models.APIResult {
	username = strings.TrimSpace(username)
	if username == "" || len([]rune(username)) > 64 {
		return models.APIResult{Success: false, Message: "用户名长度需为 1-64 个字符"}
	}
	if len(password) < 4 || len(password) > 128 {
		return models.APIResult{Success: false, Message: "密码长度需为 4-128 位"}
	}
	if _, err := a.store.GetUserByUsername(username); err == nil {
		return models.APIResult{Success: false, Message: "用户名已存在"}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return models.APIResult{Success: false, Message: "生成密码哈希失败"}
	}
	if _, err := a.store.AddUser(username, string(hash)); err != nil {
		return models.APIResult{Success: false, Message: "创建账号失败：" + err.Error()}
	}
	applog.Infof("user", "", username, "新增账号")
	return models.APIResult{Success: true, Message: "账号已创建"}
}

// DeleteUser 删除账号
func (a *App) DeleteUser(id string) models.APIResult {
	username := id
	if users, err := a.store.ListUsers(); err == nil {
		for _, u := range users {
			if u.ID == id {
				username = u.Username
				break
			}
		}
	}
	if err := a.store.DeleteUser(id); err != nil {
		return models.APIResult{Success: false, Message: "删除账号失败：" + err.Error()}
	}
	applog.Infof("user", "", username, "删除账号")
	return models.APIResult{Success: true, Message: "账号已删除"}
}

// ResetUserPassword 重置账号密码（密码以 bcrypt 哈希重新存储）
func (a *App) ResetUserPassword(id, password string) models.APIResult {
	id = strings.TrimSpace(id)
	if id == "" {
		return models.APIResult{Success: false, Message: "缺少账号 ID"}
	}
	if len(password) < 4 || len(password) > 128 {
		return models.APIResult{Success: false, Message: "密码长度需为 4-128 位"}
	}
	username := ""
	if users, err := a.store.ListUsers(); err == nil {
		for _, u := range users {
			if u.ID == id {
				username = u.Username
				break
			}
		}
	}
	if username == "" {
		return models.APIResult{Success: false, Message: "账号不存在"}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return models.APIResult{Success: false, Message: "生成密码哈希失败"}
	}
	if err := a.store.UpdateUserPassword(id, string(hash)); err != nil {
		return models.APIResult{Success: false, Message: "重置密码失败：" + err.Error()}
	}
	applog.Infof("user", "", username, "重置密码")
	return models.APIResult{Success: true, Message: "密码已重置"}
}

// === 分享管理 ===

// ListShares 分享列表
func (a *App) ListShares() []models.Share {
	shares, err := a.store.ListShares()
	if err != nil {
		return []models.Share{}
	}
	return shares
}

// CreateShare 创建分享链接
func (a *App) CreateShare(req models.ShareCreateRequest) (models.ShareCreated, error) {
	m, err := a.store.GetMount(strings.TrimSpace(req.MountID))
	if err != nil && strings.TrimSpace(req.Mount) != "" {
		m, err = a.store.GetMountByAlias(strings.TrimSpace(req.Mount))
	}
	if err != nil {
		return models.ShareCreated{}, errors.New("挂载点不存在")
	}
	abs, clean, err := fileserver.ResolveMountPath(m, req.Path)
	if err != nil {
		return models.ShareCreated{}, errors.New("路径不合法")
	}
	if _, err := os.Stat(abs); err != nil {
		return models.ShareCreated{}, errors.New("目标不存在")
	}
	mode := strings.TrimSpace(req.Mode)
	switch mode {
	case models.SharePreview, models.ShareDownload, models.ShareCollect:
	case "":
		mode = models.ShareDownload
	default:
		return models.ShareCreated{}, errors.New("分享模式不合法")
	}
	pwdHash := ""
	if p := strings.TrimSpace(req.Password); p != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
		if err != nil {
			return models.ShareCreated{}, errors.New("生成分享密码失败")
		}
		pwdHash = string(hash)
	}
	var expiresAt int64
	if req.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(req.ExpiresIn) * time.Hour).Unix()
	}
	maxVisits := req.MaxVisits
	if maxVisits < 0 {
		maxVisits = 0
	}
	sh, err := a.store.CreateShare(models.Share{
		MountID:      m.ID,
		MountAlias:   m.Alias,
		MountName:    m.Name,
		Path:         clean,
		PasswordHash: pwdHash,
		HasPassword:  pwdHash != "",
		ExpiresAt:    expiresAt,
		MaxVisits:    maxVisits,
		Mode:         mode,
		CreatedBy:    "admin",
	})
	if err != nil {
		return models.ShareCreated{}, fmt.Errorf("创建分享失败: %w", err)
	}
	sh.MountAlias = m.Alias
	sh.MountName = m.Name
	url := strings.TrimRight(a.server.PrimaryURL(), "/") + "/s/" + sh.Token
	applog.Infof("share", "", "", "创建分享 → %s", url)
	return models.ShareCreated{Share: sh, URL: url}, nil
}

// DeleteShare 撤销分享
func (a *App) DeleteShare(id string) models.APIResult {
	if err := a.store.DeleteShare(id); err != nil {
		return models.APIResult{Success: false, Message: "撤销分享失败：" + err.Error()}
	}
	applog.Infof("share", "", "", "撤销分享 %s", id)
	return models.APIResult{Success: true, Message: "分享已撤销"}
}

// GetShareURL 分享访问地址
func (a *App) GetShareURL(token string) string {
	return strings.TrimRight(a.server.PrimaryURL(), "/") + "/s/" + token
}

// === 开机自启 ===

// GetAutoStartStatus 当前开机自启状态（以系统实际状态为准）
func (a *App) GetAutoStartStatus() bool {
	if a.app != nil {
		if enabled, err := a.app.Autostart.IsEnabled(); err == nil {
			return enabled
		}
	}
	cfg, err := a.store.GetConfig()
	if err != nil {
		return false
	}
	return cfg.AutoStart
}

// SetAutoStart 设置开机自启
func (a *App) SetAutoStart(enabled bool) models.APIResult {
	if a.app == nil {
		return models.APIResult{Success: false, Message: "应用尚未就绪"}
	}
	var err error
	if enabled {
		err = a.app.Autostart.Enable()
	} else {
		err = a.app.Autostart.Disable()
	}
	if err != nil {
		if errors.Is(err, application.ErrAutostartNotSupported) {
			return models.APIResult{Success: false, Message: "当前系统不支持开机自启"}
		}
		return models.APIResult{Success: false, Message: "设置开机自启失败：" + err.Error()}
	}
	cfg, err := a.store.GetConfig()
	if err == nil {
		cfg.AutoStart = enabled
		if err := a.store.SaveConfig(cfg); err == nil {
			a.server.UpdateConfig(cfg)
		}
	}
	if enabled {
		applog.Infof("app", "", "", "已开启开机自启")
		return models.APIResult{Success: true, Message: "已开启开机自启"}
	}
	applog.Infof("app", "", "", "已关闭开机自启")
	return models.APIResult{Success: true, Message: "已关闭开机自启"}
}

// === 环境信息 ===

// RuntimeInfo 运行环境
func (a *App) RuntimeInfo() map[string]any {
	return map[string]any{
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
		"dataDir": a.store.BaseDir(),
		"logDir":  applog.Dir(),
	}
}
