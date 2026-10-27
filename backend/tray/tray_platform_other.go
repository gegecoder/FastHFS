//go:build !windows

// Linux/macOS 托盘实现：Wails v3 原生 SystemTray。
package tray

import (
	"github.com/wailsapp/wails/v3/pkg/application"

	"FastHFS/backend/applog"
)

var (
	platformTray *application.SystemTray
	mToggle      *application.MenuItem
	mOpenURL     *application.MenuItem
	mCopyURL     *application.MenuItem
	trayReady    bool
)

func toggleLabel(running bool) string {
	if running {
		return "停止服务"
	}
	return "启动服务"
}

func startTray(c *Controller) {
	if c.app == nil {
		return
	}
	applog.Infof("tray", "", "", "初始化系统托盘")
	platformTray = c.app.SystemTray.New()
	platformTray.SetLabel(c.appName)
	platformTray.SetIcon(c.iconFor(c.running.Load()))
	platformTray.SetTooltip(c.tooltip)

	menu := c.app.Menu.New()
	mToggle = menu.Add(toggleLabel(c.running.Load())).OnClick(func(*application.Context) {
		c.toggleService()
	})
	menu.AddSeparator()
	menu.Add("打开管理面板").OnClick(func(*application.Context) {
		c.openPanel()
	})
	mOpenURL = menu.Add("打开访问地址").OnClick(func(*application.Context) {
		if c.actions.OpenURL != nil {
			c.actions.OpenURL()
		}
	})
	mCopyURL = menu.Add("复制访问地址").OnClick(func(*application.Context) {
		if c.actions.CopyURL != nil {
			c.actions.CopyURL()
		}
	})
	menu.AddSeparator()
	menu.Add("退出 FastHFS").OnClick(func(*application.Context) {
		applog.Infof("tray", "", "", "用户点击退出")
		c.QuitApp()
	})
	platformTray.SetMenu(menu)
	platformTray.OnClick(func() {
		c.showWindow()
	})
	trayReady = true
	updateTrayState(c, c.running.Load())
}

func stopTray() {
	if platformTray != nil {
		platformTray.Destroy()
		platformTray = nil
	}
	trayReady = false
}

// updateTrayState 根据运行状态更新图标与菜单
func updateTrayState(c *Controller, running bool) {
	if !trayReady || platformTray == nil {
		return
	}
	platformTray.SetIcon(c.iconFor(running))
	if mToggle != nil {
		mToggle.SetLabel(toggleLabel(running))
	}
	if mOpenURL != nil {
		mOpenURL.SetEnabled(running)
	}
	if mCopyURL != nil {
		mCopyURL.SetEnabled(running)
	}
}
