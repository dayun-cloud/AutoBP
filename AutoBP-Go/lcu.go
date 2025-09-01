package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ImOlli/go-lcu/lcu"
	"github.com/gorilla/websocket"
)

// LCUCredentials LCU连接凭据
type LCUCredentials struct {
	Port     int
	Token    string
	Protocol string
}

// LCUStatus LCU连接状态
type LCUStatus struct {
	Connected    bool                   `json:"connected"`
	ClientStatus string                 `json:"client_status"`
	ChampSelect  map[string]interface{} `json:"champ_select"`
}

// LCUConnector LCU连接器
type LCUConnector struct {
	credentials *LCUCredentials
	client      *http.Client
	ws          *websocket.Conn
	status      *LCUStatus
	statusLock  sync.RWMutex
	connected   bool
	connLock    sync.RWMutex
	stopChan    chan struct{}
	app         *App // 引用主应用
	
	// 跟踪已处理的操作
	processedActions map[string]bool
	actionLock       sync.RWMutex
	
	// 跟踪状态
	lastPreselectChampion *int
	readyCheckAccepted    bool
	loggedWarnings        map[string]bool
	warningLock           sync.RWMutex
}

// NewLCUConnector 创建新的LCU连接器
func NewLCUConnector(app *App) *LCUConnector {
	return &LCUConnector{
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
		status: &LCUStatus{
			Connected:    false,
			ClientStatus: "unknown",
			ChampSelect:  nil,
		},
		stopChan:         make(chan struct{}),
		app:              app,
		processedActions: make(map[string]bool),
		loggedWarnings:   make(map[string]bool),
	}
}

// findLCUCredentials 查找LCU连接凭据
func (lcuConn *LCUConnector) findLCUCredentials() (*LCUCredentials, error) {
	// 使用go-lcu库自动获取LCU连接信息 <mcreference link="https://pkg.go.dev/github.com/ImOlli/go-lcu/lcu" index="1">1</mcreference>
	info, err := lcu.FindLCUConnectInfo()
	if err != nil {
		if lcu.IsProcessNotFoundError(err) {
			fmt.Println("[ERROR] LeagueClientUx.exe进程未找到")
			return nil, fmt.Errorf("LeagueClientUx.exe process not found - League client may not be running")
		}
		fmt.Printf("[ERROR] 获取LCU连接信息失败: %v\n", err)
		return nil, fmt.Errorf("failed to find LCU credentials: %w", err)
	}
	
	// 转换端口为整数
	port, err := strconv.Atoi(info.Port)
	if err != nil {
		return nil, fmt.Errorf("invalid port number: %s", info.Port)
	}
	
	return &LCUCredentials{
		Port:     port,
		Token:    info.AuthToken,
		Protocol: "https",
	}, nil
}

// Connect 连接到LCU
func (lcu *LCUConnector) Connect() error {
	creds, err := lcu.findLCUCredentials()
	if err != nil {
		return fmt.Errorf("failed to find LCU credentials: %w", err)
	}
	
	lcu.credentials = creds
	
	// 测试HTTP连接
	if err := lcu.testConnection(); err != nil {
		return fmt.Errorf("failed to test LCU connection: %w", err)
	}
	
	// 建立WebSocket连接
	if err := lcu.connectWebSocket(); err != nil {
		return fmt.Errorf("failed to connect WebSocket: %w", err)
	}
	
	lcu.setConnected(true)
	lcu.updateStatus()
	
	fmt.Println("[INFO] LCU API is ready to be used.")
	fmt.Println("[INFO] 🚀 后端服务启动成功！")
	
	return nil
}

// testConnection 测试HTTP连接
func (lcu *LCUConnector) testConnection() error {
	if lcu.credentials == nil {
		return fmt.Errorf("not connected to LCU")
	}
	
	url := fmt.Sprintf("https://127.0.0.1:%d/lol-gameflow/v1/gameflow-phase", lcu.credentials.Port)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("[ERROR] 创建HTTP请求失败: %v\n", err)
		return err
	}
	
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("riot:%s", lcu.credentials.Token)))
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := lcu.client.Do(req)
	if err != nil {
		fmt.Printf("[ERROR] HTTP请求失败: %v\n", err)
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Printf("[ERROR] LCU API返回错误状态码: %d\n", resp.StatusCode)
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	
	return nil
}

// connectWebSocket 连接WebSocket
func (lcu *LCUConnector) connectWebSocket() error {
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("riot:%s", lcu.credentials.Token)))
	header := http.Header{}
	header.Add("Authorization", "Basic "+auth)
	
	u := url.URL{
		Scheme: "wss",
		Host:   fmt.Sprintf("127.0.0.1:%d", lcu.credentials.Port),
		Path:   "/",
	}
	
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		Subprotocols:    []string{"wamp"},
		HandshakeTimeout: 10 * time.Second,
	}
	
	ws, resp, err := dialer.Dial(u.String(), header)
	if err != nil {
		fmt.Printf("[ERROR] WebSocket连接失败: %v\n", err)
		if resp != nil {
			fmt.Printf("[ERROR] WebSocket响应状态: %s\n", resp.Status)
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("[ERROR] WebSocket响应体: %s\n", string(body))
		}
		return err
	}
	
	lcu.ws = ws
	
	// 启动消息处理循环
	go lcu.handleWebSocketMessages()
	
	// 等待一小段时间确保连接稳定
	time.Sleep(100 * time.Millisecond)
	
	// 订阅事件
	lcu.subscribeToEvents()
	
	return nil
}

// subscribeToEvents 订阅LCU事件
func (lcu *LCUConnector) subscribeToEvents() {
	// LCU WebSocket使用WAMP 1.0协议
	// 订阅OnJsonApiEvent来接收所有JSON API事件
	msg := []interface{}{5, "OnJsonApiEvent"}
	if err := lcu.ws.WriteJSON(msg); err != nil {
		fmt.Printf("[ERROR] Failed to subscribe to OnJsonApiEvent: %v\n", err)
	}
}

// handleWebSocketMessages 处理WebSocket消息
func (lcu *LCUConnector) handleWebSocketMessages() {
	defer func() {
		if lcu.ws != nil {
			lcu.ws.Close()
			lcu.ws = nil
		}
		lcu.setConnected(false)
	}()
	
	// 设置读取超时
	lcu.ws.SetReadDeadline(time.Time{}) // 无限期等待
	
	for {
		var msg json.RawMessage
		if err := lcu.ws.ReadJSON(&msg); err != nil {
			// 只在非EOF错误时打印错误信息
			if !strings.Contains(err.Error(), "EOF") && !strings.Contains(err.Error(), "close") {
				fmt.Printf("[ERROR] WebSocket read error: %v\n", err)
			}
			return
		}
		
		// 解析JSON消息
		var parsedMsg []interface{}
		if err := json.Unmarshal(msg, &parsedMsg); err != nil {
			fmt.Printf("[ERROR] JSON解析失败: %v\n", err)
			continue
		}
		
		lcu.handleEvent(parsedMsg)
	}
}

// handleEvent 处理LCU事件
func (lcu *LCUConnector) handleEvent(msg []interface{}) {
	if len(msg) < 3 {
		return
	}
	
	// 检查opcode是否为8（事件消息）
	opcode, ok := msg[0].(float64)
	if !ok || opcode != 8 {
		return
	}
	
	// 检查事件名称
	eventName, ok := msg[1].(string)
	if !ok || eventName != "OnJsonApiEvent" {
		return
	}
	
	// 解析事件数据
	eventPayload, ok := msg[2].(map[string]interface{})
	if !ok {
		return
	}
	
	uri, _ := eventPayload["uri"].(string)
	data := eventPayload["data"]
	
	switch {
	case strings.Contains(uri, "/lol-matchmaking/v1/ready-check"):
		lcu.handleReadyCheck(data)
	case strings.Contains(uri, "/lol-gameflow/v1/gameflow-phase"):
		lcu.handleGameflowPhase(data)
	case strings.Contains(uri, "/lol-champ-select/v1/session"):
		lcu.handleChampSelect(data)
	}
}

// request 发送HTTP请求到LCU API
func (lcu *LCUConnector) request(method, path string, body interface{}) (map[string]interface{}, error) {
	if lcu.credentials == nil {
		return nil, fmt.Errorf("not connected to LCU")
	}
	
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(jsonData)
	}
	
	url := fmt.Sprintf("https://127.0.0.1:%d%s", lcu.credentials.Port, path)
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}
	
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("riot:%s", lcu.credentials.Token)))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := lcu.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	
	var result map[string]interface{}
	if len(respBody) > 0 {
		// 尝试解析为JSON对象
		if err := json.Unmarshal(respBody, &result); err != nil {
			// 如果解析失败，可能是字符串响应，创建一个包含字符串值的map
			var stringResult string
			if err := json.Unmarshal(respBody, &stringResult); err != nil {
				// 如果也不是有效的JSON字符串，直接使用原始字符串
				result = map[string]interface{}{"value": string(respBody)}
			} else {
				result = map[string]interface{}{"value": stringResult}
			}
		}
	}
	
	return result, nil
}

// setConnected 设置连接状态
func (lcu *LCUConnector) setConnected(connected bool) {
	lcu.connLock.Lock()
	defer lcu.connLock.Unlock()
	lcu.connected = connected
	
	lcu.statusLock.Lock()
	defer lcu.statusLock.Unlock()
	lcu.status.Connected = connected
	if !connected {
		lcu.status.ClientStatus = "unknown"
		lcu.status.ChampSelect = nil
	}
}

// IsConnected 检查是否已连接
func (lcu *LCUConnector) IsConnected() bool {
	lcu.connLock.RLock()
	defer lcu.connLock.RUnlock()
	return lcu.connected
}

// GetStatus 获取当前状态
func (lcu *LCUConnector) GetStatus() *LCUStatus {
	lcu.statusLock.RLock()
	defer lcu.statusLock.RUnlock()
	
	// 创建状态副本
	status := &LCUStatus{
		Connected:    lcu.status.Connected,
		ClientStatus: lcu.status.ClientStatus,
	}
	
	if lcu.status.ChampSelect != nil {
		status.ChampSelect = make(map[string]interface{})
		for k, v := range lcu.status.ChampSelect {
			status.ChampSelect[k] = v
		}
	}
	
	return status
}

// updateStatus 更新客户端状态
func (lcu *LCUConnector) updateStatus() {
	if !lcu.IsConnected() {
		return
	}
	
	result, err := lcu.request("GET", "/lol-gameflow/v1/gameflow-phase", nil)
	if err != nil {
		fmt.Printf("[ERROR] Failed to get gameflow phase: %v\n", err)
		return
	}
	
	// LCU API直接返回字符串，不是包装在data字段中
	if phase, ok := result["value"].(string); ok {
		lcu.statusLock.Lock()
		lcu.status.ClientStatus = phase
		lcu.statusLock.Unlock()
	}
}

// Disconnect 断开连接
func (lcu *LCUConnector) Disconnect() {
	lcu.setConnected(false)
	
	if lcu.ws != nil {
		lcu.ws.Close()
	}
	
	close(lcu.stopChan)
	fmt.Println("[INFO] LCU disconnected.")
}

// 清理状态的辅助方法
func (lcu *LCUConnector) clearProcessedActions() {
	lcu.actionLock.Lock()
	defer lcu.actionLock.Unlock()
	lcu.processedActions = make(map[string]bool)
}

func (lcu *LCUConnector) clearLoggedWarnings() {
	lcu.warningLock.Lock()
	defer lcu.warningLock.Unlock()
	lcu.loggedWarnings = make(map[string]bool)
}

func (lcu *LCUConnector) addProcessedAction(actionKey string) {
	lcu.actionLock.Lock()
	defer lcu.actionLock.Unlock()
	lcu.processedActions[actionKey] = true
}

func (lcu *LCUConnector) isActionProcessed(actionKey string) bool {
	lcu.actionLock.RLock()
	defer lcu.actionLock.RUnlock()
	return lcu.processedActions[actionKey]
}

func (lcu *LCUConnector) addLoggedWarning(warningKey string) {
	lcu.warningLock.Lock()
	defer lcu.warningLock.Unlock()
	lcu.loggedWarnings[warningKey] = true
}

func (lcu *LCUConnector) isWarningLogged(warningKey string) bool {
	lcu.warningLock.RLock()
	defer lcu.warningLock.RUnlock()
	return lcu.loggedWarnings[warningKey]
}