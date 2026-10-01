# ninebot-go

> Ninebot（九号）Passport + 业务 API 的非官方命令行工具 —— 纯 Go 实现

[![Go Version](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-blue)](#构建)

非官方逆向工程项目，与 Ninebot / Segway 官方无任何关联，仅供自有设备研究使用。

---

## 这是什么

`ninebot-go` 是 Ninebot（九号电动车）官方 CLI 工具的 Go 语言复刻版。它实现了 Ninebot App 与云端之间的完整加密通信协议，让你可以在命令行、Web 界面或 MCP 工具中查看车辆状态、电池信息、骑行记录，甚至控制车辆。

核心亮点：
- 纯 Go 实现，无需 Python 运行时，单文件编译
- 完整逆向了 Ninebot 的 NET 加密协议（AES-128-CBC + RSA-1024）
- 与原版 App 流量逐字节对齐，只读链路已验证
- 三种使用方式：CLI 命令 / Web 界面 / MCP 工具

## 功能一览

| 命令 | 说明 |
|------|------|
| `ninecli login` | 密码登录 |
| `ninecli login-code` | 短信验证码登录 |
| `ninecli whoami` | 查看当前登录账号 |
| `ninecli vehicles` | 列出所有车辆（自有 + 共享） |
| `ninecli status SN` | 车辆状态（位置、电量、锁、权限） |
| `ninecli battery SN` | 电池详情（电压、温度、循环次数） |
| `ninecli travel SN` | 骑行记录（默认当月） |
| `ninecli bell SN` | 寻车铃 |
| `ninecli engine-start SN` | 开机 / 解锁（需确认） |
| `ninecli engine-stop SN` | 关机 / 落锁（需确认） |
| `ninecli buck SN` | 打开坐桶（需确认） |
| `ninecli serve` | 启动 REST API 代理 |
| `ninecli mcp` | 启动 MCP 工具服务器 |
| `ninecli web` | 启动 Web 图形界面 |

> `SN` = 车辆序列号，可通过 `ninecli vehicles` 查看。

## 快速开始

### 1. 构建

需要 Go 1.25+：

```bash
# 编译当前平台
go build -o ninecli .

# 全平台交叉编译
./build.ps1   # Windows PowerShell
./build.sh    # Linux / macOS
```

### 2. 登录

```bash
# 方式一：密码登录
ninecli login -u 手机号 -p 密码

# 方式二：短信验证码登录
ninecli login-code -m 手机号     # 发送验证码
ninecli login-code -c 验证码    # 消费验证码完成登录
```

### 3. 使用

```bash
ninecli vehicles                    # 查看车辆列表
ninecli status N1DEC2315J0435       # 查看车辆状态
ninecli battery N1DEC2315J0435      # 查看电池信息
ninecli travel N1DEC2315J0435        # 查看骑行记录
```

### 4. Web 界面（推荐日常使用）

```bash
ninecli web    # 自动打开浏览器 http://127.0.0.1:8080
```

浏览器内可完成全部操作，危险操作（开机、开坐桶等）有二次确认弹窗。

## 全局参数

| 参数 | 说明 |
|------|------|
| `--json` | 输出原始解密 JSON |
| `--config <dir>` | 配置目录（默认 `~/.config/ninebot`） |
| `-y, --yes` | 跳过危险操作确认 |
| `--ebike-host <url>` | 覆盖 ebike 网关地址（调试用） |
| `--motor-host <url>` | 覆盖 motor 网关地址 |
| `--travel-host <url>` | 覆盖行程网关地址 |

## REST API 代理

`ninecli serve` 启动一个本地 HTTP 服务器，将加密的 Ninebot API 转为明文 REST：

```
GET    /healthz                         # 健康检查
POST   /auth/login                      # {account, password}
POST   /auth/login-code                 # {account}
POST   /auth/login-code/consume         # {account, code}
POST   /auth/refresh                    # 刷新 Token
GET    /whoami                          # 当前用户
GET    /vehicles                        # 车辆列表
GET    /vehicles/{sn}/status            # 车辆状态
GET    /vehicles/{sn}/battery           # 电池信息
GET    /vehicles/{sn}/travel?month=YYYYMM  # 骑行记录
GET    /vehicles/{sn}/travel/{id}       # 单次骑行详情
POST   /vehicles/{sn}/engine/start      # 开机
POST   /vehicles/{sn}/engine/stop       # 关机
POST   /vehicles/{sn}/buck              # 开坐桶
POST   /vehicles/{sn}/bell              # 寻车铃
```

统一响应格式：`{"ok":true,"data":...}` 或 `{"ok":false,"error":{"code":"...","message":"..."}}`

```bash
ninecli serve --bind 127.0.0.1:18009 --token mysecret
```

## MCP 工具

`ninecli mcp` 将 14 个工具暴露为 MCP（Model Context Protocol）接口，可被 AI 助手调用：

| 工具 | 说明 |
|------|------|
| `auth_login` | 密码登录 |
| `auth_send_code` | 发送验证码 |
| `auth_consume_code` | 验证码登录 |
| `auth_refresh` | 刷新 Token |
| `whoami` | 查看当前用户 |
| `vehicles` | 列出车辆 |
| `vehicle_status` | 车辆状态 |
| `vehicle_battery` | 电池信息 |
| `travel` | 骑行记录 |
| `travel_detail` | 单次骑行详情 |
| `engine_start` | 开机（需用户确认） |
| `engine_stop` | 关机（需用户确认） |
| `buck` | 开坐桶（需用户确认） |
| `bell` | 寻车铃 |

```bash
ninecli mcp         # stdio 模式
ninecli mcp --http  # Streamable HTTP 模式
```

## 项目结构

```
ninebot-go/
├── cmd/                # CLI 命令（cobra）
│   ├── root.go         # 根命令 + 全局参数
│   ├── login.go        # 密码 / 短信登录
│   ├── vehicles.go     # 车辆列表
│   ├── battery.go      # 电池信息
│   ├── travel.go       # 骑行记录
│   ├── control.go      # 车控（铃 / 开关机 / 坐桶）
│   ├── serve.go        # REST 代理
│   ├── mcp.go          # MCP 服务器
│   ├── web.go          # Web 界面
│   └── ...
├── internal/
│   ├── api/            # HTTP 客户端 + 各端点封装
│   ├── crypto/         # NET 加密协议（AES / RSA / 密钥推导）
│   ├── config/         # 配置 & Token 持久化
│   ├── proxy/          # REST 代理实现
│   └── mcp/            # MCP 工具实现
├── build.ps1           # Windows 构建脚本
├── build.sh            # Linux / macOS 构建脚本
├── main.go             # 入口
└── go.mod              # 模块定义
```

## 协议逆向说明

Ninebot 的业务 API 使用自定义的 NET 加密协议，本项目的核心价值在于完整逆向了这套协议：

**请求加密流程：**
1. 业务参数序列化为有序 JSON，追加 `serviceTime`、`nonce`、`checkcode`
2. 生成随机 AES-128 密钥（KReq），加密 keyData JSON 得到 `d` 字段
3. RSA-1024 加密 KReq 得到 `k` 字段
4. 组装信封 `{"d":"...","h":"...","k":"...","p":"101","t":"0"}`

**响应解密流程：**
1. 服务器返回 `{"v":101,"s":"签名","r":"密文"}`
2. 用四个 keyData 字符串推导出 AES 密钥（DeriveKey）
3. 解密 `r` 字段得到 `{"data":"base64(业务JSON)"}`
4. 解码内层 base64 得到最终业务数据

**已验证（与原版逐字节对齐）：**
- vehicles / status / battery / travel 的输出与 PyPI 原版二进制完全一致
- 加密信封字段顺序、类型、分隔符均经过真实流量校准

**待完成：**
- 车控命令（bell / engine-start / engine-stop / buck）使用独立的 control RSA 公钥，尚未联调
- 登录链路请求形状已修正但未在本轮重跑
- 响应 `s` 字段验签：服务器公钥未定位，当前忽略

## 许可

[MIT](LICENSE)
