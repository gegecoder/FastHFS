// FastHFS 主入口：Wails v3 桌面应用（内嵌管理面板）+ 独立 HTTP 文件服务 + 托盘常驻。
package main

import (
	"embed"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"

	"FastHFS/backend/appconfig"
	"FastHFS/backend/applog"
)

//go:embed all:frontend/dist
var assets embed.FS

// app.config.json 是应用名称/版本等信息的唯一配置源
//
//go:embed app.config.json
var appConfigRaw []byte

// 应用图标（PNG），用于托盘与运行时窗口图标
//
//go:embed assets/icon.png
var appIconPNG []byte

func main() {
	applog.Init()

	cfg, err := appconfig.Load(appConfigRaw)
	if err != nil {
		applog.Errorf("app", "", "", "加载应用配置失败: %v", err)
		os.Exit(1)
	}
	applog.Infof("app", "", "", "%s v%s 启动中…", cfg.Name, cfg.Version)

	svc, err := NewApp(cfg, appIconPNG)
	if err != nil {
		applog.Errorf("app", "", "", "初始化失败: %v", err)
		os.Exit(1)
	}

	opts := application.Options{
		Name:        cfg.Name,
		Description: cfg.Description,
		Linux: application.LinuxOptions{
			ApplicationID: cfg.ProductIdentifier,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		ShouldQuit: func() bool {
			if runtime.GOOS == "darwin" {
				svc.RequestQuit()
				return false
			}
			return true
		},
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		OnShutdown: func() {
			_ = svc.ServiceShutdown()
		},
	}
	app := application.New(opts)

	// 按屏幕大小建窗（屏幕 80%，最小 1180×760），并设置最小可缩放尺寸
	winOpts := application.WebviewWindowOptions{
		Name:             "main",
		Title:            "FastHFS",
		BackgroundColour: application.NewRGB(15, 23, 42),
		URL:              "/",
	}
	width, height := 1180, 760
	if screen := app.Screen.GetPrimary(); screen != nil && screen.Size.Width > 0 && screen.Size.Height > 0 {
		w := screen.Size.Width * 8 / 10
		h := screen.Size.Height * 8 / 10
		if w > width {
			width = w
		}
		if h > height {
			height = h
		}
	}
	winOpts.Width = width
	winOpts.Height = height
	winOpts.MinWidth = 960
	winOpts.MinHeight = 620
	mainWindow := app.Window.NewWithOptions(winOpts)
	// 关闭窗口隐藏到托盘
	svc.InstallCloseHandler(mainWindow)

	if err := app.Run(); err != nil {
		applog.Errorf("app", "", "", "FastHFS 启动失败: %v", err)
		applog.Close()
		os.Exit(1)
	}
	applog.Close()
}
