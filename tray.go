package main

import (
	_ "embed"

	"github.com/energye/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

// initTray 初始化系统托盘（在 startup 中以 goroutine 启动）
func (a *App) initTray() {
	systray.Run(a.onTrayReady, nil)
}

func (a *App) onTrayReady() {
	systray.SetIcon(trayIcon)
	systray.SetTooltip("AutoBP")

	// 托盘仅用于唤起主窗口，不提供弹出菜单（TrackPopupMenu 存在卡死风险）
	systray.SetOnClick(func(menu systray.IMenu) {
		a.showMainWindow()
	})
	systray.SetOnRClick(func(menu systray.IMenu) {
		a.showMainWindow()
	})
}

// showMainWindow 从托盘恢复显示主窗口
func (a *App) showMainWindow() {
	wailsruntime.WindowUnminimise(a.ctx)
	// WindowShow 对被隐藏的窗口不可靠，必须用 Show
	wailsruntime.Show(a.ctx)
}

// MinimizeToTray 隐藏窗口到托盘（关闭询问弹窗调用）
func (a *App) MinimizeToTray() {
	wailsruntime.WindowHide(a.ctx)
}

// QuitApp 真正退出应用程序（关闭询问弹窗调用）
func (a *App) QuitApp() {
	wailsruntime.Quit(a.ctx)
}
