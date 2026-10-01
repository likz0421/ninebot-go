---
title: ninebot-go
---

Ninebot（九号）Passport + 业务 API 的非官方命令行工具 —— **纯 Go 实现**，  
与 Ninebot / Segway 官方无关联，仅供自有设备研究使用。

## 功能

```
ninecli login          # 密码登录（Passport），保存 tokens.json
ninecli login-code     # 短信登录（先 -m 发送，再 -c 消费）
ninecli whoami         # 校验 token（POST /v5/user）
ninecli vehicles       # 列出自有 + 共享车辆
ninecli status SN      # 车辆状态（位置、电量、锁、acc、权限）
ninecli battery SN     # 电池信息（电压、温度、循环、充电功率）
ninecli travel SN      # 行程记录（默认当月；--detail <id> 单条详情）
ninecli bell SN        # 寻车铃
ninecli engine-start SN / engine-stop SN / buck SN   # ⚠️ 车控（y/N 确认，-y 跳过）
ninecli serve          # 明文 REST 代理（默认 127.0.0.1:18009）
ninecli mcp            # MCP 服务器（stdio；--http 启用 Streamable HTTP）
```

全局 flag：`--json`（输出原始解密 JSON）、`--config`（配置目录，  
默认 `$NINEBOT_CONFIG_DIR` 或 `~/.config/ninebot`）、  
`-y/--yes`，以及 `--passport-base` / `--biz-host` / `--ebike-host` /  
`--motor-host` / `--travel-host`（测试联调用 host 覆盖）。

### 图形界面（推荐日常使用）

```
ninecli web            # 启动后自动打开浏览器 http://127.0.0.1:8080
ninecli web --bind 127.0.0.1:9000
```

浏览器内完成全部操作：密码/短信登录、车辆列表、状态/电池/骑行记录、  
寻车铃、电源开关、坐桶（危险操作有二次确认弹窗）。进程内直调 API 层，  
凭证与命令行版共用同一 tokens.json；默认只绑 127.0.0.1，不对外监听。

### 抓包 / 协议校准（不改变 CLI 表面）

- `NINEBOT_CAPTURE_DIR=<dir>`：开启报文捕获，每次 API 交换追加一行  
  JSON 到 `<dir>/capture.jsonl`。加密通道记录含完整 keyData  
  （KReq + 四个 keyData 串）+ 内层明文 + 响应密文/解密结果——  
  响应解密失败时可离线重放，是 M2 DeriveKey 联调的关键产物。
- `ninecli har-verify <capture.har>`（隐藏命令，不出现在 help）：  
  对真机 HAR 跑三类校准探针——响应 `s` 字段 RSA 验签枚举、  
  sign 头 canonical 组合探测（c1 直接调用本仓库 crypto 实现）、  
  请求 envelope 字段序/类型统计。

## 构建

需要 Go 1.25+：

```
# 当前平台
go build -o ninecli.exe .

# 全平台交叉编译（Windows amd64/arm64、Linux amd64/arm64、macOS amd64/arm64）
./build.ps1        # PowerShell
./build.sh         # sh
```

产物在 `dist/`。模块缓存可用环境变量 `GOCACHE`/`GOPATH` 自定义。

## serve 端点

```
GET    /healthz                        # 无需认证
POST   /auth/login                     # {account,password}
POST   /auth/login-code                # {account}
POST   /auth/login-code/consume        # {account,code}
POST   /auth/refresh
GET    /whoami
GET    /vehicles
GET    /vehicles/{sn}/status
GET    /vehicles/{sn}/battery
GET    /vehicles/{sn}/travel?month=YYYYMM
GET    /vehicles/{sn}/travel/{detail_id}
POST   /vehicles/{sn}/engine/start | engine/stop | buck | bell
```

统一响应包：`{"ok":true,"data":...}` / `{"ok":false,"error":{code,message}}`。  
`--token secret` 可要求 Bearer 认证；环境变量 `NINEBOT_SERVE_BIND`、  
`NINEBOT_SERVE_TOKEN` 与原版一致。

## mcp 工具（14 个）

`auth_login`、`auth_send_code`、`auth_consume_code`、`auth_refresh`、  
`whoami`、`vehicles`、`vehicle_status`、`vehicle_battery`、`travel`、  
`travel_detail`、`engine_start`、`engine_stop`、`buck`、`bell`  
（车控工具的描述含 "requires user confirmation"，客户端应先征询用户）。

## 协议校准（已完成 M2/M3 只读链路）

加密层为本复刻的核心难点，现已用真实流量端到端验证：  
`vehicles / status / battery / travel / travel --detail` 的**人读输出与  
`--json` 输出均与 PyPI 原版二进制逐字节相同**（diff 为空）。

**已验证的规则（原版行为）：**

1. 内层请求体是**扁平** JSON —— 没有 `cmd` 字段，也没有 `params` 子对象；  
   路由完全靠 URL 路径。字段顺序固定：  
   `sys_language, client_ver, device_id, regionx, language, ostype, lang,
   platform_ver, platform, login_country, access_token, uid, <端点专用字段>,
   serviceTime, nonce, checkcode`。
2. `serviceTime` 是**数字**（unix 毫秒），`nonce` 为 32 位十六进制；  
   `checkcode` = 大写的 `md5(前面所有字节序列化结果)`（不含结尾 `}`）。
3. `ostype="and"`，`platform_ver = os_version + " " + os_model`（"13 Xiaomi"），  
   来自 config.json 的 `os_version` / `os_model`。
4. 身份：业务通道**不发** `Access-Token` / `access_token` / `MID` 头，  
   凭证只走内层 `access_token`；`Uid` 头与内层 `uid` 都用 **business_uid**  
   （`/user/user/login` 返回的短 uid），填 passport uuid 会稳定得到 4103。
5. 端点与 `Business-Type`：ebike=2，steeldust / cn-cbu / api-jhcx=1；  
   `vehicles` 会同时问 ebike(65) 与 steeldust(64) 两个网关再合并。  
   序列号字段名按端点不同：status 用 `sn_str`，battery/travel 用 `wnumber`。  
   travel 网关另需 `Accept: application/json`、`rn-ver: 743`、  
   `rn-module: Track`、`rn-version: 0` 以及 `access_token` 头。
6. 外层信封 `{"d","h","k","p":"101","t":"0"}`（p/t 为字符串），  
   keyData JSON 里 `platform=2`、`timeStamp` 为 unix **秒**。
7. 网关**双重封装**响应：`{"v":101,"s":...,"r":b64(AES)}`，解出来是  
   `{"data": b64(业务JSON), "timeStamp":N}`，再解一层才是  
   `{"code":1,"data":...,"desc":"成功"}`；业务成功的 code 是 **1**（不是 0）。

**校准方法（可复现）：** 用 `--biz-host/--motor-host/--ebike-host/ --travel-host http://127.0.0.1:<port>` 把**原版二进制**和本仓库的 CLI 同时  
指向一个本地明文转发代理，逐字段对比 method/path/headers/body；原版内层  
密文用「把参考二进制的一份拷贝里的 RSA 公钥换成自建公钥」的方式离线解开。  
排查期间可用 `NINEBOT_CAPTURE_DIR` 记录内层明文与响应密文。

**仍未完成：**

1. 车控（`bell / engine-start / engine-stop / buck`）—— 原版用**独立的  
   control RSA 公钥**走 `BuildControlCmd`（json.Marshal → RSA → base64），  
   与业务信封不同；本仓库实现尚未联调，`ninecli web` 的车控按钮暂时仍  
   委托原版二进制。真机验证需本人在场（会驱动车辆）。
2. `login / login-code / business_login` 的新登录链路未在本轮重跑  
   （沿用已保存的 token），请求形状已按原版 `/user/user/login` 修正。
3. 响应 `s` 字段验签：passport 内嵌公钥 + PKCS1v15 的 6 种组合已在  
   184 条真实响应上全部落空，服务器公钥未定位，当前忽略该字段。

## 许可

MIT
