package main

import (
	_ "embed"
	"fmt"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

// initTray 初始化系统托盘（在 startup 中以 goroutine 启动）
func (a *App) initTray() {
	systray.Run(a.onTrayReady, nil)
}

func (a *App) onTrayReady() {
	systray.SetIcon(trayIcon)
	systray.SetTitle("AutoBP")
	systray.SetTooltip("AutoBP")

	mShow := systray.AddMenuItem("显示主窗口", "显示 AutoBP 主窗口")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出 AutoBP")

	// 左键直接呼出主窗口，右键弹出菜单
	systray.SetOnClick(func(menu systray.IMenu) {
		a.showMainWindow()
	})
	systray.SetOnRClick(func(menu systray.IMenu) {
		menu.ShowMenu()
	})

	mShow.Click(func() { a.showMainWindow() })
	mQuit.Click(func() { a.QuitApp() })
}

// showMainWindow 从托盘恢复显示主窗口
func (a *App) showMainWindow() {
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
}

// MinimizeToTray 隐藏窗口到托盘
func (a *App) MinimizeToTray() {
	runtime.WindowHide(a.ctx)
}

// QuitApp 真正退出应用程序
func (a *App) QuitApp() {
	a.mu.Lock()
	a.exiting = true
	a.mu.Unlock()
	runtime.Quit(a.ctx)
}

// SetMinimizeToTray 设置点击关闭时的行为并写入配置文件
func (a *App) SetMinimizeToTray(enabled bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.config.MinimizeToTray = &enabled
	if err := a.config.SaveConfig(); err != nil {
		fmt.Printf("[ERROR] Failed to save minimize-to-tray config: %v\n", err)
		return err
	}

	fmt.Println("[INFO] Minimize-to-tray setting saved:", enabled)
	return nil
}
