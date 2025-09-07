# AutoBP - 英雄联盟自动BP助手

一个使用Go语言和Wails框架开发的英雄联盟自动Ban/Pick助手，提供现代化的桌面应用体验，支持自动接受对局、自动预选英雄、自动禁用英雄和按位置自动选择英雄等功能。

**[📦 点击下载最新版本](https://github.com/dayun-cloud/AutoBP/releases/latest/download/AutoBP.exe)**

## ✨ 主要功能

### 🎯 核心功能
- **自动接受对局** - 自动接受匹配到的游戏
- **自动预选英雄** - 进入选人阶段后自动显示意向英雄
- **自动禁用英雄** - 在Ban阶段自动禁用指定英雄
- **自动选择英雄** - 在Pick阶段自动锁定指定英雄

### 🎮 按位置选择功能
- **智能位置识别** - 自动识别玩家在排位中的分配位置
- **按位置配置英雄** - 为每个位置单独配置英雄

### 🖥️ 用户界面
- **现代化桌面应用** - 基于Wails框架的原生桌面应用
- **无边框窗口** - 自定义标题栏和窗口控制
- **实时状态显示** - 显示LCU连接状态和客户端状态
- **英雄搜索功能** - 支持英雄名称搜索和快速选择

## 🚀 快速开始

### 环境要求
- Go 1.18+
- Node.js 16+ (用于前端构建)
- Wails CLI v2.x
- 英雄联盟客户端

### 安装Wails CLI
```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### 开发模式运行
```bash
# 克隆项目
git clone https://github.com/dayun-cloud/AutoBP.git

# 安装依赖
go mod tidy

# 运行开发服务器
wails dev
```

### 构建生产版本
```bash
# 构建可执行文件
wails build

# 构建后的文件位于 build/bin/ 目录
```

## 🛠️ 技术架构

### 后端技术
- **Go语言** - 高性能的系统级编程语言
- **Wails v2** - Go语言的桌面应用框架
- **LCU API** - 英雄联盟客户端API集成
- **JSON配置** - 轻量级配置管理

### 前端技术
- **原生JavaScript** - 无框架依赖的前端实现
- **现代CSS** - 响应式设计和暗色主题
- **Wails绑定** - Go后端与前端的无缝通信

### 核心组件
- **应用上下文** - Wails应用生命周期管理
- **LCU连接器** - 监听游戏客户端事件
- **配置管理** - 动态加载和保存用户配置
- **英雄数据管理** - 英雄信息的获取和管理
- **自动化引擎** - 执行自动BP操作

## 📦 项目结构

```
AutoBP/
├── app.go              # 主应用逻辑
├── main.go             # 程序入口点
├── champion.go         # 英雄数据管理
├── config.go           # 配置管理
├── lcu.go              # LCU API连接
├── lcu_handlers.go     # LCU事件处理器
├── utils.go            # 工具函数和路径管理
├── wails.json          # Wails项目配置
├── go.mod              # Go模块依赖
├── go.sum              # 依赖校验文件
├── frontend/           # 前端资源
│   ├── index.html      # 主界面
│   └── wailsjs/        # Wails生成的JS绑定
└── README.md        # 项目说明文档
```

## 🔧 开发说明

### 主要功能模块

#### 应用管理 (app.go)
- 应用启动和关闭
- 上下文管理
- 前后端通信桥梁

#### LCU集成 (lcu.go, lcu_handlers.go)
- 自动发现LCU端口和认证令牌
- 监听游戏状态变化事件
- 执行自动化操作（接受对局、Ban/Pick英雄）

#### 配置管理 (config.go)
- 用户配置的加载和保存
- 英雄选择偏好设置
- 自动化功能开关控制

#### 英雄数据 (champion.go)
- 英雄信息的获取和缓存
- 英雄搜索和过滤功能
- 数据更新和同步

### 开发工作流

1. **实时开发**
   ```bash
   wails dev
   ```
   - 提供热重载功能
   - 前端修改即时生效
   - Go代码修改自动重编译

2. **调试模式**
   - 开发模式下可在浏览器中访问 `http://localhost:34115`
   - 支持浏览器开发者工具调试
   - 可直接调用Go方法进行测试

3. **构建发布**
   ```bash
   wails build -clean
   ```
   - 生成优化的生产版本
   - 自动打包所有依赖
   - 输出单一可执行文件

### API接口

应用提供以下主要方法供前端调用：

- `GetConfig()` - 获取当前配置
- `SaveConfig(config)` - 保存配置
- `GetChampions()` - 获取英雄列表
- `GetLCUStatus()` - 获取LCU连接状态
- `StartAutoAccept()` - 开始自动接受对局
- `StopAutoAccept()` - 停止自动接受对局

## ⚠️ 注意事项

1. **系统要求**
   - Windows 10/11 (主要支持平台)
   - macOS 10.13+ (实验性支持)
   - Linux (实验性支持)

2. **客户端要求**
   - 英雄联盟客户端必须运行并登录

3. **其他**
   - 首次运行会自动下载最新的英雄数据
   - 配置文件和英雄数据文件会自动创建在用户数据目录下


## 🤝 贡献

欢迎提交Issue和Pull Request来改进这个项目！


## 📄 许可证

本项目仅供学习和个人使用，请勿用于商业用途。

## 🔗 相关链接

- [Wails官方文档](https://wails.io/docs/introduction)
- [Go语言官网](https://golang.org/)
- [英雄联盟开发者文档](https://developer.riotgames.com/)
- [LCU API 速查手册](http://www.mingweisamuel.com/lcu-schema/tool/#/)

---

**免责声明**：本工具仅为游戏辅助工具，使用时请遵守游戏官方规则。作者不承担因使用本工具而产生的任何后果。
