//go:build windows

// Windows 托盘实现：fyne.io/systray（原生 Win32 消息循环 + ICO 图标）。
package tray

import (
	stdruntime "runtime"
	"sync"
	"sync/atomic"

	"fyne.io/systray"

	"FastHFS/backend/applog"
)

var (
	trayStopOnce sync.Once
	trayReady    atomic.Bool

	mToggle  *systray.MenuItem
	mOpenURL *systray.MenuItem
	mCopyURL *systray.MenuItem
)

func startTray(c *Controller) {
	stdruntime.LockOSThread()
	systray.Run(c.onTrayReady, nil)
}

func stopTray() {
	trayStopOnce.Do(func() {
		systray.Quit()
	})
}

func toggleLabel(running bool) string {
	if running {
		return "停止服务"
	}
	return "启动服务"
}

func (c *Controller) onTrayReady() {
	systray.SetIcon(MustICO(c.iconFor(c.running.Load())))
	systray.SetTooltip(c.tooltip)

	mToggle = systray.AddMenuItem(toggleLabel(c.running.Load()), "启动或停止 HTTP 文件服务")
	mOpenPanel := systray.AddMenuItem("打开管理面板", "显示 FastHFS 管理面板")
	mOpenURL = systray.AddMenuItem("打开访问地址", "在浏览器中打开文件服务")
	mCopyURL = systray.AddMenuItem("复制访问地址", "复制当前访问地址到剪贴板")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出 FastHFS", "退出应用")

	systray.SetOnTapped(func() {
		c.showWindow()
	})
	trayReady.Store(true)
	updateTrayState(c, c.running.Load())

	go func() {
		for {
			select {
			case <-mToggle.ClickedCh:
				c.toggleService()
			case <-mOpenPanel.ClickedCh:
				c.openPanel()
			case <-mOpenURL.ClickedCh:
				if c.actions.OpenURL != nil {
					c.actions.OpenURL()
				}
			case <-mCopyURL.ClickedCh:
				if c.actions.CopyURL != nil {
					c.actions.CopyURL()
				}
			case <-mQuit.ClickedCh:
				applog.Infof("tray", "", "", "用户点击退出")
				c.QuitApp()
				return
			}
		}
	}()
}

// updateTrayState 根据运行状态更新图标与菜单可用性
func updateTrayState(c *Controller, running bool) {
	if !trayReady.Load() {
		return
	}
	systray.SetIcon(MustICO(c.iconFor(running)))
	if mToggle != nil {
		mToggle.SetTitle(toggleLabel(running))
	}
	if mOpenURL != nil {
		if running {
			mOpenURL.Enable()
		} else {
			mOpenURL.Disable()
		}
	}
	if mCopyURL != nil {
		if running {
			mCopyURL.Enable()
		} else {
			mCopyURL.Disable()
		}
	}
}
