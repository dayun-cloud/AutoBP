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

// GetGameVersion 获取游戏版本号
func (a *App) GetGameVersion() (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.championManager.GetLatestVersion()
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

// StartRankedQueue 开始单双排位赛
func (a *App) StartRankedQueue() error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.lcuConnector == nil || !a.lcuConnector.IsConnected() {
		return fmt.Errorf("LCU not connected")
	}

	// 创建单双排位赛队列 (queueId: 420)
	lobbyData := map[string]interface{}{
		"queueId": 420,
	}

	_, err := a.lcuConnector.request("POST", "/lol-lobby/v2/lobby", lobbyData)
	if err != nil {
		fmt.Printf("[ERROR] Failed to create ranked lobby: %v\n", err)
		return fmt.Errorf("failed to create ranked lobby: %w", err)
	}

	fmt.Println("[INFO] Successfully created ranked lobby")
	return nil
}

// GoToMainMenu 回到主界面
func (a *App) GoToMainMenu() error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// 检查LCU连接器是否已初始化且已连接
	if a.lcuConnector == nil || !a.lcuConnector.IsConnected() {
		return fmt.Errorf("LCU not connected")
	}

	// 尝试退出当前lobby
	_, err := a.lcuConnector.request("DELETE", "/lol-lobby/v2/lobby", nil)
	if err != nil {
		// 如果没有lobby可以退出，创建一个匹配模式lobby然后退出
		lobbyData := map[string]interface{}{
			"queueId": 430, // 匹配模式队列ID
		}
		_, createErr := a.lcuConnector.request("POST", "/lol-lobby/v2/lobby", lobbyData)
		if createErr != nil {
			fmt.Printf("[ERROR] Failed to create lobby: %v\n", createErr)
			return createErr
		}

		// 创建成功后立即退出lobby
		_, deleteErr := a.lcuConnector.request("DELETE", "/lol-lobby/v2/lobby", nil)
		if deleteErr != nil {
			fmt.Printf("[ERROR] Failed to leave created lobby: %v\n", deleteErr)
			return deleteErr
		}
	}

	return nil
}

// PlayerProfile 玩家基本信息结构
type PlayerProfile struct {
	SummonerName  string `json:"summonerName"`
	SummonerLevel int    `json:"summonerLevel"`
	ProfileIconID int    `json:"profileIconId"`
	AccountID     int64  `json:"accountId"`
	SummonerID    int64  `json:"summonerId"`
	PUUID         string `json:"puuid"`
}

// RankedStats 排位统计信息结构
type RankedStats struct {
	QueueType    string `json:"queueType"`
	Tier         string `json:"tier"`
	Rank         string `json:"rank"`
	LeaguePoints int    `json:"leaguePoints"`
	Wins         int    `json:"wins"`
	Losses       int    `json:"losses"`
	HotStreak    bool   `json:"hotStreak"`
	Veteran      bool   `json:"veteran"`
	FreshBlood   bool   `json:"freshBlood"`
	Inactive     bool   `json:"inactive"`
}

// GetPlayerProfile 获取玩家基本信息
func (a *App) GetPlayerProfile() (*PlayerProfile, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.lcuConnector == nil || !a.lcuConnector.IsConnected() {
		return nil, fmt.Errorf("LCU not connected")
	}

	// 获取当前召唤师信息
	response, err := a.lcuConnector.request("GET", "/lol-summoner/v1/current-summoner", nil)
	if err != nil {
		fmt.Printf("[ERROR] Failed to get current summoner: %v\n", err)
		return nil, fmt.Errorf("failed to get current summoner: %w", err)
	}

	profile := &PlayerProfile{}
	// 优先使用gameName，如果没有则使用displayName
	if gameName, ok := response["gameName"].(string); ok {
		profile.SummonerName = gameName
	} else if displayName, ok := response["displayName"].(string); ok {
		profile.SummonerName = displayName
	}
	if summonerLevel, ok := response["summonerLevel"].(float64); ok {
		profile.SummonerLevel = int(summonerLevel)
	}
	if profileIconID, ok := response["profileIconId"].(float64); ok {
		profile.ProfileIconID = int(profileIconID)
	}
	if accountID, ok := response["accountId"].(float64); ok {
		profile.AccountID = int64(accountID)
	}
	if summonerID, ok := response["summonerId"].(float64); ok {
		profile.SummonerID = int64(summonerID)
	}
	if puuid, ok := response["puuid"].(string); ok {
		profile.PUUID = puuid
	}
	return profile, nil
}

// GetRankedStats 获取排位统计数据
func (a *App) GetRankedStats() ([]RankedStats, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.lcuConnector == nil || !a.lcuConnector.IsConnected() {
		return nil, fmt.Errorf("LCU not connected")
	}

	// 获取排位统计信息
	response, err := a.lcuConnector.request("GET", "/lol-ranked/v1/current-ranked-stats", nil)
	if err != nil {
		fmt.Printf("[ERROR] Failed to get ranked stats: %v\n", err)
		return nil, fmt.Errorf("failed to get ranked stats: %w", err)
	}

	var rankedStats []RankedStats

	// 解析排位统计数据
	if queues, ok := response["queues"].([]interface{}); ok {
		for _, queue := range queues {
			if queueData, ok := queue.(map[string]interface{}); ok {
				stats := RankedStats{}

				if queueType, ok := queueData["queueType"].(string); ok {
					stats.QueueType = queueType
					// 过滤掉TFT相关的队列，只保留召唤师峡谷排位赛
					if queueType != "RANKED_SOLO_5x5" && queueType != "RANKED_FLEX_SR" {
						continue
					}
				}
				if tier, ok := queueData["tier"].(string); ok {
					stats.Tier = tier
				}
				if rank, ok := queueData["division"].(string); ok {
					stats.Rank = rank
				}
				if lp, ok := queueData["leaguePoints"].(float64); ok {
					stats.LeaguePoints = int(lp)
				}
				if wins, ok := queueData["wins"].(float64); ok {
					stats.Wins = int(wins)
				}
				if losses, ok := queueData["losses"].(float64); ok {
					stats.Losses = int(losses)
				}
				if hotStreak, ok := queueData["isHotStreak"].(bool); ok {
					stats.HotStreak = hotStreak
				}
				if veteran, ok := queueData["veteran"].(bool); ok {
					stats.Veteran = veteran
				}
				if freshBlood, ok := queueData["freshBlood"].(bool); ok {
					stats.FreshBlood = freshBlood
				}
				if inactive, ok := queueData["inactive"].(bool); ok {
					stats.Inactive = inactive
				}

				rankedStats = append(rankedStats, stats)
			}
		}
	}
	return rankedStats, nil
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}
