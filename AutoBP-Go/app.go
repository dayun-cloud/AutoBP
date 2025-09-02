package main

import (
	"context"
	"fmt"
	"sync"
)

// App struct
type App struct {
	ctx             context.Context
	config          *Config
	championManager *ChampionManager
	lcuConnector    *LCUConnector
	mu              sync.RWMutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context here
// can be used to call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 初始化配置
	config, err := LoadConfig()
	if err != nil {
		fmt.Printf("[ERROR] Failed to load config: %v\n", err)
		config = DefaultConfig()
	}
	a.config = config

	// 初始化英雄管理器
	a.championManager = NewChampionManager()

	// 加载本地英雄数据
	if err := a.championManager.LoadChampions(); err != nil {
		fmt.Printf("[WARNING] Failed to load champions: %v\n", err)
	}

	// 更新英雄数据
	go func() {
		err := a.championManager.UpdateChampionsIfNeeded()
		if err != nil {
			fmt.Printf("[ERROR] Failed to update champions: %v\n", err)
		}
	}()

	// 初始化LCU连接器
	a.lcuConnector = NewLCUConnector(a)

	// 启动LCU连接器
	go func() {
		err := a.lcuConnector.Connect()
		if err != nil {
			fmt.Printf("[ERROR] Failed to connect to LCU: %v\n", err)
		}
	}()
}

// domReady is called after front-end resources have been loaded
func (a *App) domReady(_ context.Context) {
	// Add your action here
}

// beforeClose is called when the application is about to quit,
// either by clicking the window close button or calling runtime.Quit.
// Returning true will cause the application to continue, false will continue shutdown as normal.
func (a *App) beforeClose(_ context.Context) (prevent bool) {
	if a.lcuConnector != nil {
		a.lcuConnector.Disconnect()
	}
	return false
}

// shutdown is called during application termination
func (a *App) shutdown(_ context.Context) {
	if a.lcuConnector != nil {
		a.lcuConnector.Disconnect()
	}
}

// API方法供前端调用

// GetChampions 获取英雄列表
func (a *App) GetChampions() []Champion {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.championManager.GetChampions()
}

// GetConfig 获取配置
func (a *App) GetConfig() *Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.config
}

// SaveConfig 保存配置
func (a *App) SaveConfig(configData map[string]interface{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.config.UpdateConfig(configData)
	err := a.config.SaveConfig()
	if err != nil {
		fmt.Printf("[ERROR] Failed to save config: %v\n", err)
		return err
	}

	fmt.Println("[INFO] Configuration saved successfully")
	return nil
}

// GetStatus 获取LCU状态
func (a *App) GetStatus() *LCUStatus {
	if a.lcuConnector == nil {
		return &LCUStatus{
			Connected:    false,
			ClientStatus: "Disconnected",
		}
	}
	return a.lcuConnector.GetStatus()
}

// ReconnectLCU 重新连接LCU
func (a *App) ReconnectLCU() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.lcuConnector != nil {
		// 断开现有连接
		a.lcuConnector.Disconnect()

		// 重新初始化连接器
		a.lcuConnector = NewLCUConnector(a)

		// 尝试重新连接
		go func() {
			err := a.lcuConnector.Connect()
			if err != nil {
				fmt.Printf("[ERROR] Failed to reconnect to LCU: %v\n", err)
			} else {
				fmt.Println("[INFO] Successfully reconnected to LCU")
			}
		}()
	}

	return nil
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}
