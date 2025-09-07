package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Champion 英雄信息结构体
type Champion struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ChampionData 英雄数据结构体
type ChampionData struct {
	Version string             `json:"version"`
	Data    map[string]Champion `json:"data"`
}

// DDragonChampion Data Dragon API返回的英雄结构
type DDragonChampion struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// DDragonResponse Data Dragon API响应结构
type DDragonResponse struct {
	Data map[string]DDragonChampion `json:"data"`
}

// ChampionManager 英雄数据管理器
type ChampionManager struct {
	data   *ChampionData
	client *http.Client
}

// NewChampionManager 创建新的英雄数据管理器
func NewChampionManager() *ChampionManager {
	return &ChampionManager{
		data: &ChampionData{Data: make(map[string]Champion)},
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// LoadChampions 从本地文件加载英雄数据
func (cm *ChampionManager) LoadChampions() error {
	filename, err := GetChampionsPath()
	if err != nil {
		return fmt.Errorf("failed to get champions path: %w", err)
	}
	
	if _, statErr := os.Stat(filename); os.IsNotExist(statErr) {
		// 文件不存在，使用空数据
		cm.data = &ChampionData{
			Version: "",
			Data:    make(map[string]Champion),
		}
		return nil
	}
	
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read champions file: %w", err)
	}
	
	if err := json.Unmarshal(data, cm.data); err != nil {
		return fmt.Errorf("failed to parse champions file: %w", err)
	}
	
	if cm.data.Data == nil {
		cm.data.Data = make(map[string]Champion)
	}
	
	return nil
}

// SaveChampions 保存英雄数据到本地文件
func (cm *ChampionManager) SaveChampions() error {
	filename, err := GetChampionsPath()
	if err != nil {
		return fmt.Errorf("failed to get champions path: %w", err)
	}
	
	data, err := json.MarshalIndent(cm.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal champions data: %w", err)
	}
	
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write champions file: %w", err)
	}
	
	return nil
}

// GetLatestVersion 获取最新的游戏版本号
func (cm *ChampionManager) GetLatestVersion() (string, error) {
	resp, err := cm.client.Get("https://ddragon.leagueoflegends.com/api/versions.json")
	if err != nil {
		return "", fmt.Errorf("failed to get versions: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get versions: status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read versions response: %w", err)
	}
	
	var versions []string
	if err := json.Unmarshal(body, &versions); err != nil {
		return "", fmt.Errorf("failed to parse versions response: %w", err)
	}
	
	if len(versions) == 0 {
		return "", fmt.Errorf("no versions found")
	}
	
	return versions[0], nil
}

// FetchChampionsData 从Data Dragon API获取英雄数据
func (cm *ChampionManager) FetchChampionsData(version string) error {
	url := fmt.Sprintf("https://ddragon.leagueoflegends.com/cdn/%s/data/zh_CN/champion.json", version)
	
	resp, err := cm.client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch champions data: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch champions data: status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read champions response: %w", err)
	}
	
	var response DDragonResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse champions response: %w", err)
	}
	
	// 转换数据格式
	champions := make(map[string]Champion)
	for _, champ := range response.Data {
		// 将key转换为整数ID
		var id int
		if _, err := fmt.Sscanf(champ.Key, "%d", &id); err != nil {
			continue // 跳过无法解析的英雄
		}
		
		champions[champ.Key] = Champion{
			ID:   id,
			Name: champ.Name,
		}
	}
	
	cm.data = &ChampionData{
		Version: version,
		Data:    champions,
	}
	
	return nil
}

// UpdateChampionsIfNeeded 检查并更新英雄数据
func (cm *ChampionManager) UpdateChampionsIfNeeded() error {
	latestVersion, err := cm.GetLatestVersion()
	if err != nil {
		fmt.Printf("[WARNING] Failed to get latest version: %v\n", err)
		return nil // 不返回错误，使用现有数据
	}
	
	if cm.data.Version != latestVersion {
		fmt.Printf("[INFO] Updating champions data from %s to %s\n", cm.data.Version, latestVersion)
		
		if err := cm.FetchChampionsData(latestVersion); err != nil {
			fmt.Printf("[WARNING] Failed to fetch champions data: %v\n", err)
			return nil // 不返回错误，使用现有数据
		}
		
		if err := cm.SaveChampions(); err != nil {
			fmt.Printf("[WARNING] Failed to save champions data: %v\n", err)
			return nil // 不返回错误，数据已更新到内存
		}
		
		fmt.Printf("[INFO] Champions data updated successfully\n")
	}
	
	return nil
}

// GetChampions 获取英雄列表
func (cm *ChampionManager) GetChampions() []Champion {
	champions := make([]Champion, 0, len(cm.data.Data))
	for _, champ := range cm.data.Data {
		champions = append(champions, champ)
	}
	return champions
}

// GetVersion 获取当前版本
func (cm *ChampionManager) GetVersion() string {
	return cm.data.Version
}

// GetChampionByID 根据ID获取英雄信息
func (cm *ChampionManager) GetChampionByID(id int) *Champion {
	for _, champ := range cm.data.Data {
		if champ.ID == id {
			return &champ
		}
	}
	return nil
}