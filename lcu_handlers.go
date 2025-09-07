package main

import (
	"fmt"
	"time"
)

// handleReadyCheck 处理准备检查事件
func (lcu *LCUConnector) handleReadyCheck(_ interface{}) {
	if !lcu.app.config.AutoAcceptEnabled {
		return
	}
	
	if lcu.readyCheckAccepted {
		return
	}
	
	lcu.readyCheckAccepted = true
	go lcu.acceptReadyCheck()
}

// handleGameflowPhase 处理游戏流程阶段变化
func (lcu *LCUConnector) handleGameflowPhase(eventData interface{}) {
	phase, ok := eventData.(string)
	if !ok {
		return
	}
	
	lcu.statusLock.Lock()
	lcu.status.ClientStatus = phase
	lcu.statusLock.Unlock()
	
	fmt.Printf("[INFO] Game phase changed to: %s\n", phase)
	
	// 清理状态
	switch phase {
	case "Lobby", "Matchmaking", "ReadyCheck":
		lcu.clearProcessedActions()
		lcu.lastPreselectChampion = nil
		lcu.readyCheckAccepted = false
		lcu.clearLoggedWarnings()
	case "ChampSelect":
		lcu.clearProcessedActions()
		lcu.clearLoggedWarnings()
		lcu.updateChampSelectDetails()
	default:
		lcu.statusLock.Lock()
		lcu.status.ChampSelect = nil
		lcu.statusLock.Unlock()
		lcu.clearProcessedActions()
	}
}

// handleChampSelect 处理英雄选择事件
func (lcu *LCUConnector) handleChampSelect(eventData interface{}) {
	data, ok := eventData.(map[string]interface{})
	if !ok || data == nil {
		return
	}
	
	// 更新英雄选择状态
	lcu.statusLock.Lock()
	lcu.status.ChampSelect = data
	lcu.statusLock.Unlock()
	
	localCellID := lcu.getLocalPlayerCellID(data)
	if localCellID == -1 {
		return
	}
	
	timer, _ := data["timer"].(map[string]interface{})
	phase, _ := timer["phase"].(string)
	
	// 处理预选英雄
	if lcu.app.config.PreselectEnabled && (phase == "PLANNING" || phase == "BAN_PICK" || phase == "FINALIZATION") {
		lcu.handlePreselect(data, localCellID)
	}
	
	// 处理自动Ban
	if lcu.app.config.AutoBanEnabled && lcu.app.config.AutoBanChampionID != nil && phase == "BAN_PICK" {
		lcu.handleAutoBan(data, localCellID)
	}
	
	// 处理自动Pick
	if lcu.app.config.AutoPickEnabled && (phase == "BAN_PICK" || phase == "FINALIZATION") {
		lcu.handleAutoPick(data, localCellID)
	}
}

// acceptReadyCheck 自动接受对局
func (lcu *LCUConnector) acceptReadyCheck() {
	// 检查当前游戏状态，只有在ReadyCheck阶段才尝试接受
	lcu.statusLock.Lock()
	currentPhase := lcu.status.ClientStatus
	lcu.statusLock.Unlock()
	
	if currentPhase != "ReadyCheck" {
		return
	}
	
	_, err := lcu.request("POST", "/lol-matchmaking/v1/ready-check/accept", nil)
	if err != nil {
		fmt.Printf("[ERROR] Failed to accept ready check: %v\n", err)
	} else {
		fmt.Println("[INFO] Auto accepted ready check")
	}
}

// handlePreselect 处理预选英雄
func (lcu *LCUConnector) handlePreselect(data map[string]interface{}, localCellID int) {
	var currentChampion *int
	
	// 获取玩家分配的位置
	position := lcu.getPlayerAssignedPosition(data, localCellID)
	
	if position != "" {
		// 有分配位置，按位置预选英雄
		currentChampion = lcu.app.config.GetChampionIDForPosition(position)
		if currentChampion == nil {
			warningKey := fmt.Sprintf("no_champion_for_position_%s", position)
			if !lcu.isWarningLogged(warningKey) {
				fmt.Printf("[INFO] No champion configured for position %s, skipping preselect\n", position)
				lcu.addLoggedWarning(warningKey)
			}
			return
		}
	} else {
		// 没有分配位置，使用默认预选英雄
		if lcu.app.config.PreselectChampionID != nil {
			currentChampion = lcu.app.config.PreselectChampionID
		} else {
			warningKey := "no_default_preselect_champion"
			if !lcu.isWarningLogged(warningKey) {
				fmt.Println("[INFO] No position assigned and no default preselect champion configured")
				lcu.addLoggedWarning(warningKey)
			}
			return
		}
	}
	
	if currentChampion == nil {
		return
	}
	
	// 检查当前选择的英雄是否已经是目标英雄
	currentPickIntent := lcu.getCurrentPickIntent(data, localCellID)
	if currentPickIntent == *currentChampion && lcu.lastPreselectChampion != nil && *lcu.lastPreselectChampion == *currentChampion {
		return
	}
	
	// 尝试预选
	action := lcu.getPickActionForPreselect(data, localCellID)
	if action == nil {
		return
	}
	
	actionID := lcu.getActionID(action)
	if actionID == -1 {
		return
	}
	
	actionKey := fmt.Sprintf("%d_pick_preselect", actionID)
	if lcu.isActionProcessed(actionKey) {
		return
	}
	
	if position != "" {
		fmt.Printf("[INFO] Attempting to preselect position-based champion %d for %s\n", *currentChampion, position)
	} else {
		fmt.Printf("[INFO] Attempting to preselect default champion %d\n", *currentChampion)
	}
	
	success := lcu.patchAction(actionID, *currentChampion, false)
	if success {
		lcu.lastPreselectChampion = currentChampion
		lcu.addProcessedAction(actionKey)
		fmt.Printf("[INFO] Successfully preselected champion %d\n", *currentChampion)
	} else {
		fmt.Printf("[ERROR] Failed to preselect champion %d\n", *currentChampion)
	}
}

// handleAutoBan 处理自动Ban
func (lcu *LCUConnector) handleAutoBan(data map[string]interface{}, localCellID int) {
	action := lcu.getCurrentAction(data, localCellID, "ban")
	if action == nil {
		return
	}
	
	actionID := lcu.getActionID(action)
	if actionID == -1 {
		return
	}
	
	actionKey := fmt.Sprintf("%d_ban", actionID)
	if lcu.isActionProcessed(actionKey) {
		return
	}
	
	championID := *lcu.app.config.AutoBanChampionID
	fmt.Printf("[INFO] Auto banning champion %d (action %d)\n", championID, actionID)
	
	lcu.addProcessedAction(actionKey)
	
	// 延迟0.5秒
	time.Sleep(500 * time.Millisecond)
	
	success := lcu.patchAction(actionID, championID, true)
	if success {
		fmt.Printf("[INFO] Successfully banned champion %d\n", championID)
	} else {
		fmt.Printf("[ERROR] Failed to ban champion %d\n", championID)
	}
}

// handleAutoPick 处理自动Pick
func (lcu *LCUConnector) handleAutoPick(data map[string]interface{}, localCellID int) {
	action := lcu.getCurrentAction(data, localCellID, "pick")
	if action == nil {
		return
	}
	
	actionID := lcu.getActionID(action)
	if actionID == -1 {
		return
	}
	
	actionKey := fmt.Sprintf("%d_pick_completed", actionID)
	if lcu.isActionProcessed(actionKey) {
		return
	}
	
	var championID *int
	
	// 获取玩家分配的位置
	position := lcu.getPlayerAssignedPosition(data, localCellID)
	
	if position != "" {
		// 有分配位置，按位置选择英雄
		championID = lcu.app.config.GetChampionIDForPosition(position)
		if championID == nil {
			warningKey := fmt.Sprintf("no_champion_for_position_%s_auto_pick", position)
			if !lcu.isWarningLogged(warningKey) {
				fmt.Printf("[INFO] No champion configured for position %s, skipping auto pick\n", position)
				lcu.addLoggedWarning(warningKey)
			}
			return
		}
	} else {
		// 没有分配位置，使用默认秒选英雄
		if lcu.app.config.AutoPickChampionID != nil {
			championID = lcu.app.config.AutoPickChampionID
		} else {
			warningKey := "no_default_auto_pick_champion"
			if !lcu.isWarningLogged(warningKey) {
				fmt.Println("[INFO] No position assigned and no default champion configured")
				lcu.addLoggedWarning(warningKey)
			}
			return
		}
	}
	
	if championID == nil {
		return
	}
	
	if position != "" {
		fmt.Printf("[INFO] Auto picking position-based champion %d for %s (action %d)\n", *championID, position, actionID)
	} else {
		fmt.Printf("[INFO] Auto picking default champion %d (action %d)\n", *championID, actionID)
	}
	
	lcu.addProcessedAction(actionKey)
	
	// 延迟0.5秒
	time.Sleep(500 * time.Millisecond)
	
	success := lcu.patchAction(actionID, *championID, true)
	if success {
		fmt.Printf("[INFO] Successfully picked and locked champion %d\n", *championID)
	} else {
		fmt.Printf("[ERROR] Failed to pick champion %d\n", *championID)
	}
}

// 辅助方法

// getLocalPlayerCellID 获取本地玩家的CellID
func (lcu *LCUConnector) getLocalPlayerCellID(data map[string]interface{}) int {
	if cellID, ok := data["localPlayerCellId"].(float64); ok {
		return int(cellID)
	}
	return -1
}

// getPlayerAssignedPosition 获取玩家分配的位置
func (lcu *LCUConnector) getPlayerAssignedPosition(data map[string]interface{}, localCellID int) string {
	myTeam, ok := data["myTeam"].([]interface{})
	if !ok {
		return ""
	}
	
	for _, player := range myTeam {
		if playerMap, ok := player.(map[string]interface{}); ok {
			cellID, cellIDOk := playerMap["cellId"].(float64)
			position, positionOk := playerMap["assignedPosition"].(string)
			
			if cellIDOk && int(cellID) == localCellID {
				if positionOk && position != "" {
					return position
				} else {
					return ""
				}
			}
		}
	}
	
	return ""
}

// getCurrentPickIntent 获取当前预选英雄
func (lcu *LCUConnector) getCurrentPickIntent(data map[string]interface{}, localCellID int) int {
	myTeam, ok := data["myTeam"].([]interface{})
	if !ok {
		return -1
	}
	
	for _, player := range myTeam {
		if playerMap, ok := player.(map[string]interface{}); ok {
			if cellID, ok := playerMap["cellId"].(float64); ok && int(cellID) == localCellID {
				if intent, ok := playerMap["championPickIntent"].(float64); ok {
					return int(intent)
				}
			}
		}
	}
	
	return -1
}

// getCurrentAction 获取当前需要执行的操作
func (lcu *LCUConnector) getCurrentAction(data map[string]interface{}, localCellID int, actionType string) map[string]interface{} {
	actions, ok := data["actions"].([]interface{})
	if !ok {
		return nil
	}
	
	for _, actionGroup := range actions {
		if group, ok := actionGroup.([]interface{}); ok {
			for _, action := range group {
				if actionMap, ok := action.(map[string]interface{}); ok {
					actorCellID, _ := actionMap["actorCellId"].(float64)
					completed, _ := actionMap["completed"].(bool)
					aType, _ := actionMap["type"].(string)
					isInProgress, _ := actionMap["isInProgress"].(bool)
					
					if int(actorCellID) == localCellID && !completed && aType == actionType && isInProgress {
						return actionMap
					}
				}
			}
		}
	}
	
	return nil
}

// getPickActionForPreselect 获取用于预选的pick action
func (lcu *LCUConnector) getPickActionForPreselect(data map[string]interface{}, localCellID int) map[string]interface{} {
	actions, ok := data["actions"].([]interface{})
	if !ok {
		return nil
	}
	
	for _, actionGroup := range actions {
		if group, ok := actionGroup.([]interface{}); ok {
			for _, action := range group {
				if actionMap, ok := action.(map[string]interface{}); ok {
					actorCellID, _ := actionMap["actorCellId"].(float64)
					completed, _ := actionMap["completed"].(bool)
					aType, _ := actionMap["type"].(string)
					
					if int(actorCellID) == localCellID && !completed && aType == "pick" {
						return actionMap
					}
				}
			}
		}
	}
	
	return nil
}

// getActionID 获取操作ID
func (lcu *LCUConnector) getActionID(action map[string]interface{}) int {
	if id, ok := action["id"].(float64); ok {
		return int(id)
	}
	return -1
}

// patchAction 执行Ban/Pick/预选操作
func (lcu *LCUConnector) patchAction(actionID int, championID int, completed bool) bool {
	path := fmt.Sprintf("/lol-champ-select/v1/session/actions/%d", actionID)
	payload := map[string]interface{}{
		"championId": championID,
		"completed":  completed,
	}
	
	_, err := lcu.request("PATCH", path, payload)
	return err == nil
}

// updateChampSelectDetails 更新英雄选择详情
func (lcu *LCUConnector) updateChampSelectDetails() {
	result, err := lcu.request("GET", "/lol-champ-select/v1/session", nil)
	if err != nil {
		fmt.Printf("[ERROR] Failed to get champ select details: %v\n", err)
		return
	}
	
	lcu.statusLock.Lock()
	lcu.status.ChampSelect = result
	lcu.statusLock.Unlock()
}