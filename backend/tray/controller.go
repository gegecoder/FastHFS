// Package tray 系统托盘：菜单、状态图标与窗口显隐。
// Windows 使用 fyne.io/systray（原生 Win32 消息循环），
// Linux/macOS 使用 Wails v3 原生 SystemTray（避免 Objective-C 符号冲突）。
package tray

import (
	"os"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"FastHFS/backend/applog"
)

// Actions 托盘菜单回调，由 App Service 注入
type Actions struct {
	StartService func()
	StopService  func()
	OpenPanel    func()
	OpenURL      func()
	CopyURL      func()
	Quit         func()
}

// Controller 托盘控制器
type Controller struct {
	iconRunning []byte
	iconStopped []byte
	appName     string
	tooltip     string
	app         *application.App
	quitting    *atomic.Bool
	actions     Actions
	running     atomic.Bool
}

// NewController 创建托盘控制器
func NewController(iconRunning, iconStopped []byte, appName, tooltip string, quitting *atomic.Bool, actions Actions) *Controller {
	return &Controller{
		iconRunning: iconRunning,
		iconStopped: iconStopped,
		appName:     appName,
		tooltip:     tooltip,
		quitting:    quitting,
		actions:     actions,
	}
}

// Bind 注入 Wails Application
func (c *Controller) Bind(app *application.App) { c.app = app }

// Start 启动托盘（内部独立 goroutine）
func (c *Controller) Start() { go startTray(c) }

// Stop 移除托盘图标
func (c *Controller) Stop() { stopTray() }

// SetRunning 更新服务运行状态（图标 + 菜单）
func (c *Controller) SetRunning(running bool) {
	c.running.Store(running)
	updateTrayState(c, running)
}

func (c *Controller) iconFor(running bool) []byte {
	if running {
		return c.iconRunning
	}
	return c.iconStopped
}

// toggleService 启动/停止服务
func (c *Controller) toggleService() {
	if c.running.Load() {
		if c.actions.StopService != nil {
			c.actions.StopService()
		}
		return
	}
	if c.actions.StartService != nil {
		c.actions.StartService()
	}
}

func (c *Controller) openPanel() {
	if c.actions.OpenPanel != nil {
		c.actions.OpenPanel()
		return
	}
	c.showWindow()
}

// showWindow 显示并激活主窗口
func (c *Controller) showWindow() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				applog.Infof("tray", "", "", "showWindow panic: %v", r)
			}
		}()
		if c.app == nil {
			return
		}
		if w, ok := c.app.Window.Get("main"); ok {
			w.Show()
			w.UnMinimise()
			w.Focus()
		}
	}()
}

// QuitApp 退出应用
func (c *Controller) QuitApp() {
	c.quitting.Store(true)
	if c.actions.Quit != nil {
		c.actions.Quit()
		return
	}
	go func() {
		defer func() { _ = recover() }()
		if c.app != nil {
			c.app.Quit()
		}
	}()
	go func() {
		t := time.NewTimer(3 * time.Second)
		defer t.Stop()
		<-t.C
		applog.Infof("tray", "", "", "正常退出超时，强制结束进程")
		os.Exit(0)
	}()
}
