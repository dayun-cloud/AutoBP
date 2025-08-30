import asyncio
import json
import time
import os
import sys
import webbrowser
import threading
from typing import Any, Dict, List, Optional
from contextlib import asynccontextmanager
from pathlib import Path

import httpx
from fastapi import FastAPI, Request
from fastapi.responses import FileResponse, JSONResponse
from fastapi.middleware.cors import CORSMiddleware
from lcu_driver import Connector
import uvicorn

# ------------------ Configuration ------------------
def get_resource_path(relative_path: str) -> str:
    """获取资源文件的绝对路径，支持开发环境和打包后的exe"""
    try:
        # PyInstaller创建临时文件夹，并将路径存储在_MEIPASS中
        base_path = sys._MEIPASS
    except Exception:
        # 开发环境或非单exe文件打包环境
        if getattr(sys, 'frozen', False):
            # 非单exe文件打包环境，资源在_internal目录中
            base_path = os.path.join(os.path.dirname(sys.executable), '_internal')
        else:
            # 开发环境
            base_path = os.path.abspath(".")
    return os.path.join(base_path, relative_path)

CONFIG_FILE = "config.json"
CHAMPION_FILE = "champion.json"
PORT = 1206

def load_config() -> Dict[str, Any]:
    """Load configuration from file or return defaults"""
    default_config = {
        "auto_accept_enabled": False,
        "preselect_enabled": False,
        "auto_ban_enabled": False,
        "auto_pick_enabled": False,
        "preselect_champion_id": None,
        "auto_ban_champion_id": None,
        "auto_pick_champion_id": None,
    }
    
    if os.path.exists(CONFIG_FILE):
        try:
            with open(CONFIG_FILE, 'r', encoding='utf-8') as f:
                saved_config = json.load(f)
                default_config.update(saved_config)
        except Exception as e:
            print(f"[WARNING] Failed to load config file: {e}, using defaults")
    
    return default_config

def save_config(config: Dict[str, Any]) -> None:
    """Save configuration to file"""
    try:
        with open(CONFIG_FILE, 'w', encoding='utf-8') as f:
            json.dump(config, f, indent=2, ensure_ascii=False)
    except Exception as e:
        print(f"[ERROR] Failed to save config file: {e}")

def load_champions() -> Dict[str, Any]:
    """Load champions data from local file"""
    if os.path.exists(CHAMPION_FILE):
        try:
            with open(CHAMPION_FILE, 'r', encoding='utf-8') as f:
                return json.load(f)
        except Exception as e:
            print(f"[WARNING] Failed to load champions file: {e}")
    return {"version": "", "data": {}}

def save_champions(champions_data: Dict[str, Any]) -> None:
    """Save champions data to local file"""
    try:
        with open(CHAMPION_FILE, 'w', encoding='utf-8') as f:
            json.dump(champions_data, f, indent=2, ensure_ascii=False)
    except Exception as e:
        print(f"[ERROR] Failed to save champions file: {e}")

# Global variables
CONFIG: Dict[str, Any] = load_config()
CHAMPIONS_DATA: Dict[str, Any] = load_champions()

# ------------------ LCU Connection Status ------------------
LCU_STATUS = {
    "connected": False,
    "client_status": "unknown",
    "champ_select": None
}

# 跟踪已处理的操作，避免重复执行
# 全局变量
_processed_actions = set()
_last_preselect_champion = None
_ready_check_accepted = False
_logged_warnings = set()  # 用于跟踪已经打印过的警告信息，避免重复打印

# ------------------ Champion Data Management ------------------
async def get_latest_version() -> str:
    """获取最新的游戏版本号"""
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get("https://ddragon.leagueoflegends.com/api/versions.json", timeout=10)
            response.raise_for_status()
            versions = response.json()
            return versions[0] if versions else ""
    except Exception as e:
        print(f"[ERROR] Failed to get latest version: {e}")
        return ""

async def fetch_champions_data(version: str) -> Dict[str, Any]:
    """从Data Dragon API获取英雄数据"""
    try:
        url = f"https://ddragon.leagueoflegends.com/cdn/{version}/data/zh_CN/champion.json"
        async with httpx.AsyncClient() as client:
            response = await client.get(url, timeout=15)
            response.raise_for_status()
            data = response.json()
            
            # 提取关键信息
            champions = {}
            for champ_key, champ_data in data["data"].items():
                champions[champ_data["key"]] = {
                    "id": int(champ_data["key"]),
                    "name": champ_data["name"]
                }
            
            return {
                "version": version,
                "data": champions
            }
    except Exception as e:
        print(f"[ERROR] Failed to fetch champions data: {e}")
        return {"version": "", "data": {}}

async def update_champions_if_needed():
    """检查并更新英雄数据"""
    global CHAMPIONS_DATA
    
    latest_version = await get_latest_version()
    if not latest_version:
        return
    
    current_version = CHAMPIONS_DATA.get("version", "")
    if current_version != latest_version:
        print(f"[INFO] Updating champions data from {current_version} to {latest_version}")
        new_data = await fetch_champions_data(latest_version)
        if new_data["data"]:
            CHAMPIONS_DATA = new_data
            save_champions(CHAMPIONS_DATA)
            print(f"[INFO] Champions data updated successfully")

# ------------------ LCU Automation Logic ------------------
connector = Connector()

async def _accept_ready_check(connection) -> None:
    """自动接受对局"""
    try:
        await connection.request("post", "/lol-matchmaking/v1/ready-check/accept")
        print("[INFO] Auto accepted ready check")
    except Exception:
        pass

async def _get_current_action(connection, local_cell: int, action_type: str) -> Optional[Dict[str, Any]]:
    """获取当前需要执行的操作（Ban或Pick）"""
    try:
        response = await connection.request("GET", "/lol-champ-select/v1/session")
        if response.status == 200:
            session_info = await response.json()
            actions = session_info.get('actions', [])
            
            # actions是一个二维数组，需要展平
            for action_group in actions:
                for action in action_group:
                    if (action.get('actorCellId') == local_cell and
                        not action.get('completed', False) and
                        action.get('type') == action_type and
                        action.get('isInProgress', False)):
                        return action
    except Exception as e:
        print(f"[ERROR] Error getting current action: {e}")
    return None

async def _patch_action(connection, action_id: int, champion_id: int, completed: bool = True) -> bool:
    """执行Ban/Pick/预选操作
    
    Args:
        connection: LCU连接
        action_id: 操作ID
        champion_id: 英雄ID
        completed: True=确认选择/禁用, False=预选
    """
    url = f"/lol-champ-select/v1/session/actions/{action_id}"
    payload = {
        "championId": champion_id,
        "completed": completed
    }
    
    action_type = "确认" if completed else "预选"
    
    try:
        response = await connection.request("PATCH", url, json=payload)
        if response.status in (200, 204):
            return True
        else:
            response_text = await response.text()
            print(f"[ERROR] Failed to {action_type} action {action_id}: {response.status} - {response_text}")
    except Exception as e:
        print(f"[ERROR] Exception during {action_type} action: {e}")
    return False

async def _get_pick_action_for_preselect(connection, local_cell: int) -> Optional[Dict[str, Any]]:
    """获取用于预选的pick action（不检查isInProgress状态）"""
    try:
        response = await connection.request("GET", "/lol-champ-select/v1/session")
        if response.status == 200:
            session_info = await response.json()
            actions = session_info.get('actions', [])
            
            # actions是一个二维数组，需要展平
            for action_group in actions:
                for action in action_group:
                    if (action.get('actorCellId') == local_cell and
                        not action.get('completed', False) and
                        action.get('type') == 'pick'):
                        return action
    except Exception as e:
        print(f"[ERROR] Error getting pick action for preselect: {e}")
    return None

async def _get_player_assigned_position(connection, local_cell: int) -> Optional[str]:
    """获取当前玩家的分配位置"""
    try:
        response = await connection.request("GET", "/lol-champ-select/v1/session")
        if response.status == 200:
            session_info = await response.json()
            my_team = session_info.get("myTeam", [])
            
            for player in my_team:
                if player.get("cellId") == local_cell:
                    return player.get("assignedPosition")
    except Exception as e:
        print(f"[ERROR] Failed to get player assigned position: {e}")
    return None

def _get_champion_id_for_position(position: str) -> Optional[int]:
    """根据位置获取对应的英雄ID"""
    position_champions = CONFIG.get("position_champions", {})
    if position:
        # 将位置转换为大写以匹配配置文件中的键
        position_upper = position.upper()
        if position_upper in position_champions:
            return position_champions[position_upper]
    return None

async def _get_client_status(connection) -> str:
    """获取客户端当前状态"""
    try:
        response = await connection.request("get", "/lol-gameflow/v1/gameflow-phase")
        if response.status == 200:
            phase = await response.json()
            return phase if phase else "None"
    except Exception as e:
        print(f"[ERROR] Error getting client status: {e}")
    return "unknown"

async def _get_champ_select_details(connection) -> Optional[Dict[str, Any]]:
    """获取英雄选择阶段的详细信息"""
    try:
        response = await connection.request("get", "/lol-champ-select/v1/session")
        if response.status == 200:
            session_data = await response.json()
            return {
                "timer_phase": session_data.get("timer", {}).get("phase"),
                "local_player_cell_id": session_data.get("localPlayerCellId"),
                "actions": session_data.get("actions", []),
                "my_team": session_data.get("myTeam", []),
                "their_team": session_data.get("theirTeam", [])
            }
    except Exception as e:
        print(f"[ERROR] Error getting champ select details: {e}")
    return None

# ------------------ LCU Event Handlers ------------------
@connector.ready
async def connect(connection):
    print("[INFO] LCU API is ready to be used.")
    print("[INFO] 🚀 后端服务启动成功！可按Ctrl+C退出。")
    LCU_STATUS["connected"] = True
    LCU_STATUS["client_status"] = await _get_client_status(connection)
    
    if LCU_STATUS["client_status"] == "ChampSelect":
        LCU_STATUS["champ_select"] = await _get_champ_select_details(connection)

@connector.close
async def disconnect(_):
    print("[INFO] LCU disconnected.")
    LCU_STATUS["connected"] = False
    LCU_STATUS["client_status"] = "unknown"
    LCU_STATUS["champ_select"] = None

@connector.ws.register("/lol-matchmaking/v1/ready-check", event_types=("UPDATE", "CREATE"))
async def on_ready_check(connection, event):
    global _ready_check_accepted
    if CONFIG.get("auto_accept_enabled") and not _ready_check_accepted:
        _ready_check_accepted = True
        await _accept_ready_check(connection)

@connector.ws.register("/lol-gameflow/v1/gameflow-phase", event_types=("UPDATE", "CREATE"))
async def on_gameflow_phase(connection, event):
    global _processed_actions, _last_preselect_champion, _ready_check_accepted, _logged_warnings
    if event.data:
        LCU_STATUS["client_status"] = event.data
        print(f"[INFO] Game phase changed to: {event.data}")
        
        # 清理已处理的操作记录
        if event.data in ["Lobby", "Matchmaking", "ReadyCheck"]:
            _processed_actions.clear()
            _last_preselect_champion = None
            _ready_check_accepted = False
            _logged_warnings.clear()  # 清理已记录的警告，允许在新游戏中重新显示
        
        if event.data == "ChampSelect":
            _processed_actions.clear()
            _logged_warnings.clear()  # 进入英雄选择时清理警告记录
            LCU_STATUS["champ_select"] = await _get_champ_select_details(connection)
        else:
            LCU_STATUS["champ_select"] = None
            _processed_actions.clear()

@connector.ws.register("/lol-champ-select/v1/session", event_types=("UPDATE", "CREATE"))
async def on_champ_select(connection, event):
    global _processed_actions, _last_preselect_champion
    data = event.data or {}
    
    if data:
        LCU_STATUS["champ_select"] = {
            "timer_phase": data.get("timer", {}).get("phase"),
            "local_player_cell_id": data.get("localPlayerCellId"),
            "actions": data.get("actions", []),
            "my_team": data.get("myTeam", []),
            "their_team": data.get("theirTeam", [])
        }
    
    local_cell = data.get("localPlayerCellId")
    timer = data.get("timer", {})
    phase = timer.get("phase")

    # 预选英雄
    if CONFIG.get("preselect_enabled") and phase in ("PLANNING", "BAN_PICK", "FINALIZATION"):
        # 获取玩家分配的位置
        current_champion = None
        position = await _get_player_assigned_position(connection, local_cell)
        
        if position:
            # 有分配位置，严格按位置预选英雄
            current_champion = _get_champion_id_for_position(position)
            if not current_champion:
                # 避免重复打印相同的警告信息
                warning_key = f"no_champion_for_position_{position}"
                if warning_key not in _logged_warnings:
                    print(f"[INFO] No champion configured for position {position}, skipping preselect")
                    _logged_warnings.add(warning_key)
        else:
            # 没有分配位置，使用默认预选英雄
            if CONFIG.get("preselect_champion_id"):
                current_champion = CONFIG["preselect_champion_id"]
            else:
                # 避免重复打印相同的警告信息
                warning_key = "no_default_preselect_champion"
                if warning_key not in _logged_warnings:
                    print(f"[INFO] No position assigned and no default preselect champion configured")
                    _logged_warnings.add(warning_key)
        
        if current_champion:
            # 检查当前选择的英雄是否已经是目标英雄
            my_selection = data.get("myTeam", [])
            current_pick_intent = None
            for player in my_selection:
                if player.get("cellId") == local_cell:
                    current_pick_intent = player.get("championPickIntent")
                    break
            
            # 只有当前预选英雄与目标不同时才设置
            if current_pick_intent != current_champion and _last_preselect_champion != current_champion:
                if position:
                    print(f"[INFO] Attempting to preselect position-based champion {current_champion} for {position}")
                else:
                    print(f"[INFO] Attempting to preselect default champion {current_champion}")
                
                # 尝试获取当前的pick action
                current_action = await _get_pick_action_for_preselect(connection, local_cell)
                if current_action:
                    action_id = current_action.get("id")
                    if action_id:
                        action_key = f"{action_id}_pick_preselect"
                        # 避免重复预选同一个action
                        if action_key not in _processed_actions:
                            # 使用pick action进行预选，但不锁定 (completed=False)
                            success = await _patch_action(connection, action_id, current_champion, completed=False)
                            if success:
                                _last_preselect_champion = current_champion
                                _processed_actions.add(action_key)
                                print(f"[INFO] Successfully preselected champion {current_champion} using pick action")
                            else:
                                print(f"[ERROR] Failed to preselect champion {current_champion} using pick action")
                        else:
                            print(f"[INFO] Champion {current_champion} already preselected for action {action_id}")
                    else:
                        print(f"[ERROR] No valid action ID found for preselect")
                else:
                    print(f"[ERROR] No current pick action found for preselect")

    # 自动Ban
    if CONFIG.get("auto_ban_enabled") and CONFIG.get("auto_ban_champion_id") and phase == "BAN_PICK":
        current_ban_action = await _get_current_action(connection, local_cell, "ban")
        if current_ban_action:
            action_id = current_ban_action.get("id")
            action_key = f"{action_id}_ban"
            if action_key not in _processed_actions:
                champion_id = CONFIG["auto_ban_champion_id"]
                print(f"[INFO] Auto banning champion {champion_id} (action {action_id})")
                _processed_actions.add(action_key)
                await asyncio.sleep(0.5)
                success = await _patch_action(connection, action_id, champion_id)
                if success:
                    print(f"[INFO] Successfully banned champion {champion_id}")
                else:
                    print(f"[ERROR] Failed to ban champion {champion_id}")
    
    # 自动Pick
    if CONFIG.get("auto_pick_enabled") and phase in ("BAN_PICK", "FINALIZATION"):
        current_pick_action = await _get_current_action(connection, local_cell, "pick")
        if current_pick_action:
            action_id = current_pick_action.get("id")
            action_key = f"{action_id}_pick_completed"
            # 检查是否已经完成锁定（而不是仅仅预选）
            if action_key not in _processed_actions:
                # 获取玩家分配的位置
                champion_id = None
                position = await _get_player_assigned_position(connection, local_cell)
                
                if position:
                    # 有分配位置，严格按位置选择英雄
                    champion_id = _get_champion_id_for_position(position)
                    if not champion_id:
                        # 避免重复打印相同的警告信息
                        warning_key = f"no_champion_for_position_{position}_auto_pick"
                        if warning_key not in _logged_warnings:
                            print(f"[INFO] No champion configured for position {position}, skipping auto pick")
                            _logged_warnings.add(warning_key)
                else:
                    # 没有分配位置，使用默认秒选英雄
                    if CONFIG.get("auto_pick_champion_id"):
                        champion_id = CONFIG["auto_pick_champion_id"]
                    else:
                        # 避免重复打印相同的警告信息
                        warning_key = "no_default_auto_pick_champion"
                        if warning_key not in _logged_warnings:
                            print(f"[INFO] No position assigned and no default champion configured")
                            _logged_warnings.add(warning_key)
                
                if champion_id:
                    if position:
                        print(f"[INFO] Auto picking position-based champion {champion_id} for {position} (action {action_id})")
                    else:
                        print(f"[INFO] Auto picking default champion {champion_id} (action {action_id})")
                    _processed_actions.add(action_key)
                    await asyncio.sleep(0.5)
                    success = await _patch_action(connection, action_id, champion_id, completed=True)
                    if success:
                        print(f"[INFO] Successfully picked and locked champion {champion_id}")
                    else:
                        print(f"[ERROR] Failed to pick champion {champion_id}")
                else:
                    # 避免重复打印相同的警告信息
                    warning_key = "no_champion_configured_for_auto_pick"
                    if warning_key not in _logged_warnings:
                        print(f"[INFO] No champion configured for auto pick")
                        _logged_warnings.add(warning_key)

# ------------------ FastAPI Application ------------------
@asynccontextmanager
async def lifespan(app: FastAPI):
    # 启动时更新英雄数据
    await update_champions_if_needed()
    
    # 启动LCU连接器 - 在单独的线程中运行
    def _runner():
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        connector.loop = loop
        try:
            connector.start()
        except Exception as e:
            print(f"[WARNING] Failed to start LCU connector: {e}")
    
    t = threading.Thread(target=_runner, daemon=True)
    t.start()
    print("[INFO] LCU connector started in background thread")
    
    try:
        yield
    finally:
        # 关闭时断开LCU连接
        try:
            if hasattr(connector, 'loop') and connector.loop and connector.loop.is_running():
                fut = asyncio.run_coroutine_threadsafe(connector.stop(), connector.loop)
                fut.result(timeout=5)
            else:
                await connector.stop()
            print("[INFO] LCU connector closed")
        except Exception as e:
            print(f"[WARNING] Error closing LCU connector: {e}")

app = FastAPI(title="AutoBP", lifespan=lifespan)

# 添加CORS支持
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# 静态文件服务
@app.get("/")
async def index() -> FileResponse:
    html_path = get_resource_path("index.html")
    if os.path.exists(html_path):
        return FileResponse(html_path)
    # 如果打包后的文件不存在，返回当前目录的HTML文件
    return FileResponse("index.html")

@app.get("/api/champions")
async def api_champions() -> JSONResponse:
    """获取英雄列表"""
    champions_list = []
    for champ_id, champ_data in CHAMPIONS_DATA.get("data", {}).items():
        champions_list.append({
            "id": champ_data["id"],
            "name": champ_data["name"]
        })
    
    # 按名称排序
    champions_list.sort(key=lambda x: x["name"])
    
    return JSONResponse({
        "version": CHAMPIONS_DATA.get("version", ""),
        "champions": champions_list
    })

@app.get("/api/config")
async def get_config() -> JSONResponse:
    """获取当前配置"""
    return JSONResponse(CONFIG)

@app.post("/api/config")
async def set_config(req: Request) -> JSONResponse:
    """更新配置"""
    global CONFIG
    try:
        new_config = await req.json()
        CONFIG.update(new_config)
        save_config(CONFIG)
        return JSONResponse({"success": True})
    except Exception as e:
        return JSONResponse({"success": False, "error": str(e)}, status_code=400)

@app.get("/api/status")
async def get_status() -> JSONResponse:
    """获取当前状态"""
    return JSONResponse(LCU_STATUS)

# ------------------ Main Entry Point ------------------
def show_menu():
    """显示启动菜单"""
    print("\n" + "="*50)
    print("           AutoBP - 英雄联盟自动BP助手")
    print("="*50)
    print("请选择启动模式:")
    print("1. 打开操作面板并启动后端")
    print("2. 仅启动后端")
    print("3. 退出")
    print("="*50)
    
    while True:
        try:
            choice = input("请输入选择 (1-3): ").strip()
            if choice in ["1", "2", "3"]:
                return int(choice)
            else:
                print("无效选择，请输入 1、2 或 3")
        except KeyboardInterrupt:
            print("\n程序已退出")
            return 3

def start_server(open_browser: bool = False):
    """启动服务器"""
    print(f"\n[INFO] Starting AutoBP server on port {PORT}...")
    if open_browser:
        # 使用get_resource_path获取正确的HTML文件路径（支持打包环境）
        html_path = get_resource_path("index.html")
        if os.path.exists(html_path):
            print(f"[INFO] 正在打开前端控制面板")
            def open_html_delayed():
                webbrowser.open(f"file:///{html_path.replace(os.sep, '/')}")
            
            threading.Thread(target=open_html_delayed, daemon=True).start()
        else:
            print(f"[WARNING] HTML file not found: {html_path}")
    
    # 配置uvicorn日志级别，减少无关日志
    uvicorn.run(
        app, 
        host="127.0.0.1", 
        port=PORT,
        log_level="warning"  # 只显示警告和错误日志
    )

if __name__ == "__main__":
    choice = show_menu()
    
    if choice == 1:
        start_server(open_browser=True)
    elif choice == 2:
        start_server(open_browser=False)
    elif choice == 3:
        print("程序已退出")
        sys.exit(0)