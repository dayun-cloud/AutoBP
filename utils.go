package main

import (
	"os"
	"path/filepath"
)

// GetUserDataDir 获取用户数据目录下的AutoBP.exe文件夹路径
func GetUserDataDir() (string, error) {
	// 获取APPDATA目录
	appData := os.Getenv("APPDATA")
	if appData == "" {
		// 如果APPDATA不存在，回退到用户主目录
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		appData = userHome
	}
	
	// 使用Wails的标准数据目录：%APPDATA%\[BinaryName.exe]
	// 通常生成在: C:\Users\用户名\AppData\Roaming\AutoBP.exe\
	autoBPDir := filepath.Join(appData, "AutoBP.exe")
	
	// 确保目录存在
	if err := os.MkdirAll(autoBPDir, 0755); err != nil {
		return "", err
	}
	
	return autoBPDir, nil
}

// GetConfigPath 获取配置文件的完整路径
func GetConfigPath() (string, error) {
	dataDir, err := GetUserDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "config.json"), nil
}

// GetChampionsPath 获取英雄数据文件的完整路径
func GetChampionsPath() (string, error) {
	dataDir, err := GetUserDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "champions.json"), nil
}