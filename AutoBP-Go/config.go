package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Config 配置结构体
type Config struct {
	AutoAcceptEnabled    bool                   `json:"auto_accept_enabled"`
	PreselectEnabled     bool                   `json:"preselect_enabled"`
	AutoBanEnabled       bool                   `json:"auto_ban_enabled"`
	AutoPickEnabled      bool                   `json:"auto_pick_enabled"`
	PreselectChampionID  *int                   `json:"preselect_champion_id"`
	AutoBanChampionID    *int                   `json:"auto_ban_champion_id"`
	AutoPickChampionID   *int                   `json:"auto_pick_champion_id"`
	PositionChampions    map[string]*int        `json:"position_champions"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		AutoAcceptEnabled:   false,
		PreselectEnabled:    false,
		AutoBanEnabled:      false,
		AutoPickEnabled:     false,
		PreselectChampionID: nil,
		AutoBanChampionID:   nil,
		AutoPickChampionID:  nil,
		PositionChampions: map[string]*int{
			"TOP":     nil,
			"JUNGLE":  nil,
			"MIDDLE":  nil,
			"BOTTOM":  nil,
			"UTILITY": nil,
		},
	}
}

// LoadConfig 从文件加载配置
func LoadConfig() (*Config, error) {
	config := DefaultConfig()
	
	filename, err := GetConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}
	
	if _, statErr := os.Stat(filename); os.IsNotExist(statErr) {
		// 配置文件不存在，返回默认配置
		return config, nil
	}
	
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	
	// 确保position_champions不为nil
	if config.PositionChampions == nil {
		config.PositionChampions = map[string]*int{
			"TOP":     nil,
			"JUNGLE":  nil,
			"MIDDLE":  nil,
			"BOTTOM":  nil,
			"UTILITY": nil,
		}
	}
	
	return config, nil
}

// SaveConfig 保存配置到文件
func (c *Config) SaveConfig() error {
	filename, err := GetConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}
	
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	
	return nil
}

// UpdateConfig 更新配置
func (c *Config) UpdateConfig(newConfig map[string]interface{}) error {
	// 将map转换为JSON再转换为Config结构体
	data, err := json.Marshal(newConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal new config: %w", err)
	}
	
	var tempConfig Config
	if err := json.Unmarshal(data, &tempConfig); err != nil {
		return fmt.Errorf("failed to unmarshal new config: %w", err)
	}
	
	// 更新当前配置
	c.AutoAcceptEnabled = tempConfig.AutoAcceptEnabled
	c.PreselectEnabled = tempConfig.PreselectEnabled
	c.AutoBanEnabled = tempConfig.AutoBanEnabled
	c.AutoPickEnabled = tempConfig.AutoPickEnabled
	c.PreselectChampionID = tempConfig.PreselectChampionID
	c.AutoBanChampionID = tempConfig.AutoBanChampionID
	c.AutoPickChampionID = tempConfig.AutoPickChampionID
	
	// 更新位置英雄配置
	if tempConfig.PositionChampions != nil {
		if c.PositionChampions == nil {
			c.PositionChampions = make(map[string]*int)
		}
		for pos, champID := range tempConfig.PositionChampions {
			c.PositionChampions[pos] = champID
		}
	}
	
	return nil
}

// GetChampionIDForPosition 根据位置获取英雄ID
func (c *Config) GetChampionIDForPosition(position string) *int {
	if c.PositionChampions == nil {
		return nil
	}
	// 将位置转换为大写以匹配配置中的键
	position = strings.ToUpper(position)
	return c.PositionChampions[position]
}