# llmapi

> 一个基于 Go + Gin 的 LLM API 代理转发与用户管理系统，统一 OpenAI / Anthropic 协议，按用户计费、配额与并发管理。

产品名：**能爬 API**（`nengpa.com`）。系统把多个上游 LLM 服务商（MiniMax、智谱 BigModel/GLM 等）聚合为 OpenAI / Anthropic 兼容端点，提供 Web 管理后台、用户自服务面板、API Key 管理、用量统计、激活码（卡密）等能力。

---

## 目录

- [项目简介](#项目简介)
- [功能特性](#功能特性)
- [技术栈](#技术栈)
- [目录结构](#目录结构)
- [架构与请求生命周期](#架构与请求生命周期)
- [快速开始](#快速开始)
- [配置项详解](#配置项详解)
- [数据库表结构](#数据库表结构)
- [HTTP 路由清单](#http-路由清单)
- [关键代码索引](#关键代码索引)
- [常见问题 FAQ](#常见问题-faq)

---

## 项目简介

`llmapi` 是一个**单二进制**部署的 LLM 网关：

- **多上游聚合**：内置 MiniMax（`https://api.minimaxi.com`）和智谱 GLM（`https://open.bigmodel.cn`）双上游，命中 GLM 模型白名单时自动路由到智谱。
- **双协议入口**：`/v1/*` 走 OpenAI 协议（`Authorization: Bearer ...`），`/anthropic/*` 走 Anthropic 协议（`x-api-key: ...`），通过请求头自动识别。
- **多 Key 池动态权重**：每 50 秒调上游 `coding_plan/remains` 接口拉取实时剩余额度，动态调整各 Key 的权重，权重为 0 的 Key 自动从池中剔除。
- **用户体系**：邮箱注册 + 验证码、API Key 自助管理、激活码/卡密、用量查询、邮箱绑定。
- **管理后台**：用户管理、待激活用户、API Key、用量统计、上游额度查看。

适用场景：自建 / 内部分发的大模型 API 转发服务，OpenAI / Anthropic 兼容客户端可零修改接入。

---

## 功能特性

### 代理与协议层

| 能力 | 说明 |
|---|---|
| OpenAI / Anthropic 双协议 | `internal/handlers/proxy.go:130-145` 通过 `Authorization` / `x-api-key` 头自动识别 |
| 多上游 Key 池 | 两组 key 池（普通 / 高速），每组独立权重调度 |
| 动态权重轮询 | `tools/DynamicWeightedSelector.go` + `cmd/server/main.go:24-49` 初始化；`main.go:280-441` 定时刷新 |
| GLM 模型白名单分流 | 28 个 GLM / CogView / CogVideoX / Vidu / AutoGLM 等模型自动转发到 `open.bigmodel.cn` |
| GLM 模型级信号量 | `tools/KeyLock.go` 按 (key + model) 并发限流，避免触达上游并发限制 |
| 失败自动重试 | 默认 6 次，识别 `rate_limit_error` / `overloaded_error` / `2064 集群负载高` / `1305 模型访问量大` / 5xx，2 秒退避 |
| SSE 流式透传 | `internal/handlers/proxy.go:364-516` 逐行 `Flusher.Flush()`，强制 `no-cache` 头 |
| 响应自动解压 | gzip / brotli 自动解码后写入用量日志（`internal/handlers/response.go:88-107`） |
| 路径黑名单 | 拦截 `v1/files/upload`、`v1/video`、`v1/music_generation`、`v1/lyrics_generation` |

### 用户与额度

- **注册**：邮箱 + 密码（≥ 6 位）+ 6 位邮箱验证码（5 分钟有效，60s 限频），默认赠送 300 次额度，当天 0 点过期。
- **登录**：支持用户名、邮箱、内置管理员账号；bcrypt 校验。
- **双密码字段**：`PasswordHash`（创建/激活用）+ `PasswordHash2`（绑定邮箱后用 email 登录时校验），支持两种登录入口。
- **用户分层**：`level=1` 普通 / `level=2` 高速，分别使用不同的 Key 池。
- **时段并发限制**：15:00–17:30 = 5 并发/用户，其余时段 = 10 并发/用户，60 秒无活动自动释放。
- **双轨额度**：总额度（`RequestLimit`）+ 周额度（`HasWeeklyLimit=1` 时启用 `WeekRequestLimit`）。

### API Key 与用量

- 每用户最多 **5 个** Key，格式 `sk-cp-` + 64 hex 字符。
- 支持创建 / 重置 / 启用停用（toggle）/ 删除 / 过期时间。
- 用量日志字段：`api_key_id`、`user_id`、`model`、`prompt_tokens`、`completion_tokens`、`total_tokens`、`cost(decimal(10,6))`、`latency_ms`、`request_id`。
- 估算价：每 token `0.00001`。
- **阈值扣费**：单次请求先扣 1 次；当 `total_tokens > 50000` 时，每多 50k 再额外扣 1 次。
- 统计维度：用户当日 / 累计、API Key、上游 `coding_plan/remains` 剩余额度。

### 激活码 / 邀请码

- 管理员在 `activation_users` 表中预置账号（含 `valid_days`、`request_limit`、`level`、GLM/Kimi 开关、备注）。
- 用户首次登录时若未在 `users` 表中，会去 `activation_users` 查找并自动激活升级为正式用户，原激活记录被删除。

### 管理后台

4 个 Tab：

1. **用户管理**：用户名 / 备注模糊搜索，按 level / user_id / `use_glm` / `use_kimi` 过滤。
2. **待激活用户**：激活码（卡密）批量生成、删除、查询。
3. **API Keys**：跨用户查询、重置、启用/停用。
4. **用量统计**：总请求 / 总 token / 总费用 / 总用户；单用户用量明细；上游各 key 剩余额度。

---

## 技术栈

| 类别 | 选型 | 版本 | 用途 |
|---|---|---|---|
| 语言 | Go | 1.22.6 | 主程序 |
| Web 框架 | gin-gonic/gin | v1.9.1 | HTTP 路由、中间件、模板渲染 |
| ORM | gorm.io/gorm | v1.25.5 | 数据访问、AutoMigrate |
| MySQL 驱动 | gorm.io/driver/mysql | v1.5.2 | MySQL 连接 |
| 配置 | spf13/viper | v1.18.2 | YAML 配置加载 |
| 加密 | golang.org/x/crypto | - | bcrypt 密码哈希 |
| UUID | google/uuid | v1.6.0 | 请求 ID |
| Token 计数 | pkoukk/tiktoken-go | v0.1.8 | `cl100k_base` 编码 |
| Brotli | andybalholm/brotli | v1.2.0 | 上游响应解压 |
| 数据库 | MySQL | 5.7+ / 8.0 | 唯一存储 |
| 前端 | 原生 HTML + CSS + JS | - | 服务端模板 + 静态资源（非前后端分离） |

---

## 目录结构

```
llmapi/
├── cmd/
│   └── server/
│       └── main.go              # 程序入口、路由装配、定时任务
├── configs/
│   └── config.yaml              # 全部运行时配置
├── internal/
│   ├── config/
│   │   └── config.go            # Viper 加载 + LLMConfig 运行时状态
│   ├── database/
│   │   └── mysql.go             # GORM 初始化、AutoMigrate
│   ├── handlers/
│   │   ├── auth.go              # 登录/注册/验证码/Session、API Key 鉴权
│   │   ├── admin.go             # 管理后台 HTTP handler
│   │   ├── proxy.go             # LLM 代理转发（核心）
│   │   ├── models.go            # /v1/models 列表
│   │   ├── response.go          # 响应日志、用量统计
│   │   └── response_test.go
│   ├── middleware/
│   │   └── ratelimit.go         # CORS、限流（占位）
│   ├── models/
│   │   ├── user.go              # User + ActivationUser
│   │   ├── apikey.go            # APIKey
│   │   └── usage.go             # UsageLog
│   └── services/
│       ├── user.go              # 用户业务
│       ├── apikey.go            # API Key 业务
│       ├── usage.go             # 用量统计业务
│       └── proxy.go             # 代理业务（备用，主路径在 handler）
├── migrations/                  # 空目录，迁移由 GORM AutoMigrate 完成
├── pkg/
│   └── utils/                   # 空目录（占位）
├── tools/
│   ├── DynamicWeightedSelector.go  # 动态权重 key 选择器
│   ├── KeyLock.go                  # GLM 模型级信号量锁
│   ├── Email.go                    # SMTP 邮件发送
│   ├── TokenExtractor.go           # 从响应中提取 token 用量
│   └── *_test.go                   # 工具层单元测试
├── web/
│   ├── views/                   # Gin 模板
│   │   ├── index.html           # 落地页
│   │   ├── login.html
│   │   ├── register.html
│   │   ├── dashboard.html       # 用户面板
│   │   ├── admin.html           # 管理后台
│   │   └── help.html
│   └── static/
│       ├── css/                 # 样式
│       ├── js/                  # 前端逻辑
│       └── image/               # 图片资源
├── go.mod / go.sum
└── bash.sh                      # 跨平台编译脚本
```

---

## 架构与请求生命周期

```
客户端 (OpenAI / Anthropic SDK)
        │
        ▼
  /v1/*  或  /anthropic/*  或  /api/*
        │  ┌─ CORS 中间件
        │  ├─ APIKeyAuth（auth.go:469-610）
        │  │     ├─ 路径黑名单拦截
        │  │     ├─ 解析 Bearer / x-api-key
        │  │     ├─ 时段并发信号量（5/10）
        │  │     └─ 扣减用户额度
        │  └─ ResponseLogger（response.go:63-206）
        │        └─ 包装 ResponseWriter，异步收集 body
        ▼
  ProxyHandler / ProxyHandlerNew
        │  ├─ 协议识别（OpenAI vs Anthropic）
        │  ├─ GLM 模型白名单 → 路由到 open.bigmodel.cn
        │  ├─ 动态权重选 key
        │  ├─ GLM 模型级 KeyLock
        │  ├─ 转发请求
        │  ├─ 失败重试（最多 6 次）
        │  └─ SSE 逐行 Flusher
        ▼
  上游（api.minimaxi.com / open.bigmodel.cn）
        │
        ▼
  响应回写 + SaveResponseUsage（异步）
        ├─ 解析 usage（正则兜底）
        ├─ 写入 usage_logs
        └─ 阈值扣减 RequestCount
```

关键代码位置：

- 入口与路由：[cmd/server/main.go:98-273](cmd/server/main.go#L98-L273)
- 启动初始化：[cmd/server/main.go:100-127](cmd/server/main.go#L100-L127)
- 动态权重初始化：[cmd/server/main.go:24-49](cmd/server/main.go#L24-L49)
- 定时任务（每 50 秒）：[cmd/server/main.go:280-441](cmd/server/main.go#L280-L441)
- 代理（非流式）：[internal/handlers/proxy.go:102-362](internal/handlers/proxy.go#L102-L362)
- 代理（流式）：[internal/handlers/proxy.go:364-516](internal/handlers/proxy.go#L364-L516)
- GLM 模型白名单：[internal/handlers/proxy.go:61-92](internal/handlers/proxy.go#L61-L92)
- 响应日志 / 用量入库：[internal/handlers/response.go:63-248](internal/handlers/response.go#L63-L248)

---

## 快速开始

### 环境要求

- **Go** ≥ 1.22.6
- **MySQL** ≥ 5.7（推荐 8.0）
- 可选：上游账号（MiniMax / 智谱 BigModel）
- 可选：SMTP 邮箱（用于发送注册验证码；默认配置 163 邮箱 SSL 465）

### 启动步骤

```bash
# 1. 克隆
git clone <repo-url> llmapi && cd llmapi

# 2. 初始化 MySQL
mysql -uroot -p
> CREATE DATABASE llmapi DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
> exit;

# 3. 编辑配置
vim configs/config.yaml    # 必改字段见下表

# 4. 编译
go build -o llmapi cmd/server/main.go

# 5. 运行
./llmapi
# 浏览器访问 http://localhost:3000
# 内置管理员账号：admin / huangyongda（请在 config.yaml 中修改！）
```

### 跨平台编译

| 平台 | 命令 |
|---|---|
| Windows amd64 | `GOOS=windows GOARCH=amd64 go build -o llmapi-windows.exe cmd/server/main.go` |
| macOS Intel | `GOOS=darwin GOARCH=amd64 go build -o llmapi-macos-amd64 cmd/server/main.go` |
| macOS Apple Silicon | `GOOS=darwin GOARCH=arm64 go build -o llmapi-macos-arm64 cmd/server/main.go` |
| Linux amd64 | `GOOS=linux GOARCH=amd64 go build -o llmapi-linux-amd64 cmd/server/main.go` |

仓库内已附 `bash.sh` 跨平台编译脚本。

### 必改配置项

启动前**必须**修改的字段：

| 字段 | 位置 | 说明 |
|---|---|---|
| `database.password` | `configs/config.yaml` | MySQL 密码 |
| `admin.username` / `admin.password` | `configs/config.yaml` | 内置管理员账密（首次启动会自动建号） |
| `llm.api_keys` | `configs/config.yaml` | 至少填入一个 MiniMax `sk-cp-...` key |
| `llm.api_url` | `configs/config.yaml` | 默认 `https://api.minimaxi.com`，可改 `https://open.bigmodel.cn` |
| `email.username` / `email.password` | `configs/config.yaml` | SMTP 邮箱账号密码（注册验证码需要） |

---

## 配置项详解

`configs/config.yaml` 全部字段（默认值来自仓库当前配置）：

### server

| 字段 | 默认值 | 说明 |
|---|---|---|
| `server.host` | `0.0.0.0` | 监听地址 |
| `server.port` | `3000` | 监听端口 |
| `server.api_port` | `3002` | 配置存在但代码未使用 |

### database

| 字段 | 默认值 | 说明 |
|---|---|---|
| `database.host` | `localhost` | MySQL 主机 |
| `database.port` | `3306` | MySQL 端口 |
| `database.username` | `root` | MySQL 用户 |
| `database.password` | `12345678` | MySQL 密码（**必改**） |
| `database.name` | `llmapi` | 库名 |
| `database.max_idle_conns` | `10` | 连接池空闲连接数 |
| `database.max_open_conns` | `100` | 连接池最大连接数 |

### redis（未使用）

| 字段 | 默认值 | 说明 |
|---|---|---|
| `redis.host` / `port` / `password` / `db` | - | 配置保留，代码未引用，可忽略 |

### llm

| 字段 | 默认值 | 说明 |
|---|---|---|
| `llm.provider` | `custom` | 协议适配标识 |
| `llm.max_retries` | `6` | 单请求最大重试次数 |
| `llm.api_url` | `https://api.minimaxi.com` | 上游 baseURL |
| `llm.api_keys` | `[]` | 主 key 池（普通用户使用） |
| `llm.api_weights` | `[]` | 主 key 初始权重，索引与 keys 一一对应 |
| `llm.api_keys2` | `[]` | 备用 key 池（高速用户使用） |
| `llm.api_weights2` | `[]` | 备用 key 初始权重 |
| `llm.glm_api_keys` | `[]` | GLM 模型专用 key 池 |
| `llm.timeout` | `60` | 上游超时（秒） |
| `llm.proxy_url` | 空 | 可选 HTTP 代理（如 `http://127.0.0.1:7890`） |
| `llm.model_mapping` | `{}` | 模型名映射（保留字段） |
| `llm.api_keys_bak` | 注释中 | 配置存在但未在结构体中定义，加载被忽略 |
| `llm.use_bak_key` | `5` | 配置存在但代码未使用 |

### admin

| 字段 | 默认值 | 说明 |
|---|---|---|
| `admin.username` | `admin` | 内置管理员用户名（**必改**） |
| `admin.password` | `huangyongda` | 内置管理员密码（**必改**） |

### cache（未使用）

| 字段 | 默认值 | 说明 |
|---|---|---|
| `cache.api_key_ttl` | `86400` | 配置保留，代码未引用 |
| `cache.user_limit_ttl` | `300` | 配置保留，代码未引用 |

### email

| 字段 | 默认值 | 说明 |
|---|---|---|
| `email.enabled` | `true` | 是否启用邮件发送 |
| `email.smtp_host` | `smtp.163.com` | SMTP 服务器 |
| `email.smtp_port` | `465` | SMTP 端口 |
| `email.username` | `a3421675@163.com` | SMTP 账号（**必改**） |
| `email.password` | `...` | SMTP 密码 / 授权码（**必改**） |
| `email.from` | `a3421675@163.com` | 发件人 |
| `email.from_name` | `能爬 API` | 发件人显示名 |
| `email.use_tls` | `true` | 启用 STARTTLS |
| `email.use_ssl` | `true` | 启用 SSL（163 推荐 465 + SSL） |
| `email.timeout` | `30` | 发送超时（秒） |

---

## 数据库表结构

由 GORM `AutoMigrate`（[internal/database/mysql.go:48-61](internal/database/mysql.go#L48-L61)）自动创建，字符集 `utf8mb4`。

### users — 用户表

模型定义：[internal/models/user.go:9-30](internal/models/user.go#L9-L30)

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uint PK | 主键 |
| `username` | varchar(191) unique | 用户名 |
| `email` | varchar(191) | 邮箱（绑定后写入） |
| `password_hash` | varchar(255) | 创建/激活用密码哈希 |
| `password_hash2` | varchar(255) | 邮箱登录用密码哈希 |
| `request_limit` | int | 总额度 |
| `request_count` | int | 已用次数 |
| `week_request_limit` | int | 周额度（`has_weekly_limit=1` 时生效） |
| `week_request_count` | int | 本周已用 |
| `is_admin` | tinyint | 是否管理员 |
| `level` | int | 1=普通 / 2=高速 |
| `use_glm` | tinyint | 1=启用 GLM 模型 |
| `use_kimi` | tinyint | 1=启用 Kimi（字段保留） |
| `has_weekly_limit` | tinyint | 1=启用周额度 |
| `expires_at` | datetime index | 账号过期时间 |
| `remark` | varchar(255) | 备注 |
| `created_at` / `updated_at` | datetime | GORM 标准时间戳 |
| `deleted_at` | datetime index | 软删除 |

### api_keys — API Key 表

模型定义：[internal/models/apikey.go:9-19](internal/models/apikey.go#L9-L19)

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uint PK | 主键 |
| `user_id` | uint index | 所属用户 |
| `key_value` | varchar(191) unique | `sk-cp-` + 64 hex |
| `key_name` | varchar(255) | 显示名 |
| `is_active` | tinyint | 1=启用 |
| `expires_at` | datetime | 过期时间 |
| `created_at` / `deleted_at` | datetime | GORM 标准 |

### usage_logs — 用量日志表

模型定义：[internal/models/usage.go:9-22](internal/models/usage.go#L9-L22)

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uint PK | 主键 |
| `api_key_id` | uint index | 使用的 API Key |
| `user_id` | uint index | 所属用户 |
| `model` | varchar(255) | 模型名 |
| `prompt_tokens` | int | 输入 token |
| `completion_tokens` | int | 输出 token |
| `total_tokens` | int | 总 token |
| `cost` | decimal(10,6) | 估算费用 |
| `latency_ms` | int | 耗时（毫秒） |
| `request_id` | varchar(255) | 请求 ID（UUID） |
| `created_at` / `deleted_at` | datetime | GORM 标准 |

### activation_users — 激活码/卡密表

模型定义：[internal/models/user.go:79-129](internal/models/user.go#L79-L129)

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | uint PK | 主键 |
| `username` | varchar(191) | 卡密账号 |
| `password` | varchar(255) | 卡密密码 |
| `valid_days` | int | 有效天数 |
| `request_limit` | int | 激活后额度 |
| `level` | int | 激活后等级 |
| `use_glm` | tinyint | 激活后是否启用 GLM |
| `use_kimi` | tinyint | 激活后是否启用 Kimi |
| `has_weekly_limit` | tinyint | 激活后是否启用周额度 |
| `remark` | varchar(255) | 备注 |
| `created_at` / `updated_at` | datetime | GORM 标准 |

---

## HTTP 路由清单

### 公开页面（GET，无鉴权）

| 路径 | 说明 |
|---|---|
| `/` | 落地页 |
| `/index.html` | 落地页（同 `/`） |
| `/login.html` | 登录页 |
| `/register.html` | 注册页 |
| `/dashboard.html` | 用户面板 |
| `/help.html` | 帮助文档 |
| `/admin.html` | 管理后台 |
| `/static/*` | 静态资源 |
| `/health` | 健康检查 |
| `/custom-timeout` | 调试用超时接口 |

### 公开鉴权接口

| 方法 | 路径 | Handler | 文件:行 |
|---|---|---|---|
| POST | `/web/auth/send-code` | 发送注册验证码 | [auth.go:125](internal/handlers/auth.go#L125) |
| POST | `/web/auth/register` | 邮箱注册 | [auth.go:185](internal/handlers/auth.go#L185) |
| POST | `/web/auth/login` | 登录（同时用于 `/admin/login`） | [auth.go:62](internal/handlers/auth.go#L62) |

### LLM 代理（需 API Key 鉴权）

所有路径挂 `APIKeyAuth` + `ResponseLogger` 中间件（[main.go:178-198](cmd/server/main.go#L178-L198)），通配到 `ProxyHandler`（[proxy.go:102](internal/handlers/proxy.go#L102)）。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET / POST | `/v1/*path` | OpenAI 协议入口（`Authorization: Bearer`） |
| GET / POST | `/anthropic/*path` | Anthropic 协议入口（`x-api-key`） |
| GET / POST | `/api/*path` | 通用代理入口 |
| GET | `/v1/models` | 模型列表（按用户 level/use_glm 过滤） |

#### curl 示例

```bash
# OpenAI Chat Completions
curl http://localhost:3000/v1/chat/completions \
  -H "Authorization: Bearer sk-cp-xxxxxxxx" \
  -H "Content-Type: application/json" \
  -d '{"model":"MiniMax-M2","messages":[{"role":"user","content":"hello"}]}'

# OpenAI 模型列表
curl http://localhost:3000/v1/models \
  -H "Authorization: Bearer sk-cp-xxxxxxxx"

# Anthropic Messages
curl http://localhost:3000/anthropic/v1/messages \
  -H "x-api-key: sk-cp-xxxxxxxx" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-3-5-sonnet-20241022","max_tokens":1024,"messages":[{"role":"user","content":"hello"}]}'
```

### 用户自服务（Session 鉴权）

[main.go:212-222](cmd/server/main.go#L212-L222) 注册。

| 方法 | 路径 | Handler | 文件:行 |
|---|---|---|---|
| GET | `/web/user/me` | 当前用户信息 | [auth.go:269](internal/handlers/auth.go#L269) |
| GET | `/web/user/apikeys` | 我的 API Key 列表 | [admin.go:333](internal/handlers/admin.go#L333) |
| POST | `/web/user/apikeys` | 创建 API Key | [admin.go:354](internal/handlers/admin.go#L354) |
| DELETE | `/web/user/apikeys/:id` | 删除 API Key | [admin.go:382](internal/handlers/admin.go#L382) |
| GET | `/web/user/usage` | 我的用量统计 | [admin.go:415](internal/handlers/admin.go#L415) |
| PUT | `/web/user/bind-email` | 绑定邮箱 | [auth.go:285](internal/handlers/auth.go#L285) |

### 管理员后台（Session + IsAdmin 双重校验）

[main.go:225-266](cmd/server/main.go#L225-L266) 注册。

| 方法 | 路径 | Handler | 文件:行 |
|---|---|---|---|
| GET | `/admin/users` | 用户列表（搜索/筛选/排序） | [admin.go:32](internal/handlers/admin.go#L32) |
| POST | `/admin/users` | 创建用户 | [admin.go:61](internal/handlers/admin.go#L61) |
| PUT | `/admin/users/:id` | 更新用户 | [admin.go:108](internal/handlers/admin.go#L108) |
| DELETE | `/admin/users/:id` | 删除用户 | [admin.go:139](internal/handlers/admin.go#L139) |
| GET | `/admin/apikeys` | 跨用户 API Key 列表 | [admin.go:154](internal/handlers/admin.go#L154) |
| POST | `/admin/apikeys` | 管理员创建 API Key | [admin.go:188](internal/handlers/admin.go#L188) |
| POST | `/admin/apikeys/:id/reset` | 重置 API Key | [admin.go:213](internal/handlers/admin.go#L213) |
| DELETE | `/admin/apikeys/:id` | 删除 API Key | [admin.go:229](internal/handlers/admin.go#L229) |
| POST | `/admin/apikeys/:id/toggle` | 启用/停用 API Key | [admin.go:244](internal/handlers/admin.go#L244) |
| GET | `/admin/usage` | 全量用量 | [admin.go:259](internal/handlers/admin.go#L259) |
| GET | `/admin/users/:user_id/usage` | 单用户用量 | [admin.go:286](internal/handlers/admin.go#L286) |
| GET | `/admin/stats` | 总览统计 | [admin.go:321](internal/handlers/admin.go#L321) |
| GET | `/admin/upstream-usage` | 上游各 key 剩余额度 | [admin.go:455](internal/handlers/admin.go#L455) |
| GET | `/admin/activation-users` | 激活码列表 | [admin.go:508](internal/handlers/admin.go#L508) |
| POST | `/admin/activation-users` | 单条创建激活码 | [admin.go:532](internal/handlers/admin.go#L532) |
| POST | `/admin/activation-users/batch` | 批量创建激活码 | [admin.go:598](internal/handlers/admin.go#L598) |
| DELETE | `/admin/activation-users/:id` | 删除激活码 | [admin.go:582](internal/handlers/admin.go#L582) |

---

## 关键代码索引

| 主题 | 路径 |
|---|---|
| 程序入口 | [cmd/server/main.go:98](cmd/server/main.go#L98) |
| 路由装配 | [cmd/server/main.go:178-266](cmd/server/main.go#L178-L266) |
| 启动初始化 | [cmd/server/main.go:100-127](cmd/server/main.go#L100-L127) |
| 动态权重选择器 | [cmd/server/main.go:24-49](cmd/server/main.go#L24-L49), [tools/DynamicWeightedSelector.go](tools/DynamicWeightedSelector.go) |
| 定时任务 | [cmd/server/main.go:280-441](cmd/server/main.go#L280-L441) |
| 配置文件 | [configs/config.yaml](configs/config.yaml) |
| 配置结构 | [internal/config/config.go:13-19](internal/config/config.go#L13-L19) |
| GORM 初始化 | [internal/database/mysql.go:18-61](internal/database/mysql.go#L18-L61) |
| User / ActivationUser 模型 | [internal/models/user.go:9-30](internal/models/user.go#L9-L30), [L79-129](internal/models/user.go#L79-L129) |
| APIKey 模型 | [internal/models/apikey.go:9-19](internal/models/apikey.go#L9-L19) |
| UsageLog 模型 | [internal/models/usage.go:9-22](internal/models/usage.go#L9-L22) |
| 鉴权 / API Key 鉴权 | [internal/handlers/auth.go:62-610](internal/handlers/auth.go#L62-L610) |
| 代理转发（非流式） | [internal/handlers/proxy.go:102-362](internal/handlers/proxy.go#L102-L362) |
| 代理转发（流式） | [internal/handlers/proxy.go:364-516](internal/handlers/proxy.go#L364-L516) |
| GLM 模型白名单 | [internal/handlers/proxy.go:61-92](internal/handlers/proxy.go#L61-L92) |
| 响应日志 / 用量入库 | [internal/handlers/response.go:63-248](internal/handlers/response.go#L63-L248) |
| 用户业务 | [internal/services/user.go](internal/services/user.go) |
| API Key 业务 | [internal/services/apikey.go](internal/services/apikey.go) |
| 用量业务 | [internal/services/usage.go](internal/services/usage.go) |
| 管理员 handler | [internal/handlers/admin.go](internal/handlers/admin.go) |
| 上游额度查询 | [internal/handlers/admin.go:455-505](internal/handlers/admin.go#L455-L505) |
| 模型列表 | [internal/handlers/models.go](internal/handlers/models.go) |
| 邮件发送 | [tools/Email.go](tools/Email.go) |
| GLM 模型锁 | [tools/KeyLock.go](tools/KeyLock.go) |
| Token 提取 | [tools/TokenExtractor.go](tools/TokenExtractor.go) |
| 前端模板 | [web/views/](web/views/) |
| 静态资源 | [web/static/](web/static/) |
| 跨平台编译脚本 | [bash.sh](bash.sh) |

---

## 常见问题 FAQ

**Q: 是否有 Docker 支持？**
A: 当前仓库**没有** `Dockerfile` / `docker-compose.yml`。需要自行编写容器化部署脚本，或直接使用 `bash.sh` 编译产物 + 外部 nginx/caddy 反代。

**Q: 是否有 LICENSE？**
A: 仓库**无** LICENSE 文件，默认为"All rights reserved"。

**Q: `config.yaml` 中的 `redis` 段和 `cache` 段有什么用？**
A: 这两个段是**配置保留但代码未引用**的字段，不会生效。Session 鉴权使用进程内存 map，限流使用进程内信号量（`sync.Mutex`），并未真正接入 Redis。

**Q: 数据库迁移如何管理？**
A: 完全由 GORM `AutoMigrate` 在启动时自动建表（[internal/database/mysql.go:48-61](internal/database/mysql.go#L48-L61)）。`migrations/` 目录为空。如需手动迁移或版本管理，需自行扩展。

**Q: 公告 / 续费支付是怎么实现的？**
A: **未做后端化**。`web/views/dashboard.html:132-141` 写死公告区块；续费月卡 / 季卡跳外部 `http://120.24.86.32:8080/pay.php?...` PHP 支付页；管理员通过 `activation_users` 表手工发放账号。

**Q: `pkg/utils/` 目录是空的？**
A: 是的，目前为占位目录，未使用。

**Q: `services/proxy.go` 中的 `ProxyRequest` / `ForwardChatCompletion` / `HandleSSE` 是做什么的？**
A: 备用实现（[internal/services/proxy.go](internal/services/proxy.go)），主路径走 `handlers/proxy.go` 的 `ProxyHandler`，没有在 `main.go` 中注册。

**Q: `Logout` 函数为什么没生效？**
A: `Logout` 在 [internal/handlers/auth.go:109-122](internal/handlers/auth.go#L109-L122) 定义，但**未注册到任何路由**（前端登出走的是清 cookie + 重定向）。

**Q: CORS 配置能否跨域携带凭据？**
A: 当前 [main.go:172](cmd/server/main.go#L172) 设置 `Access-Control-Allow-Origin: *` 同时 `Access-Control-Allow-Credentials: true`——按浏览器规范这两者**互斥**，跨域凭据不会真正生效。如需携带凭据请修改 `internal/middleware/ratelimit.go:36-50` 显式回写来源域。

**Q: 用户并发限制的逻辑是什么？**
A: 15:00–17:30 = 5 并发/用户，其他时段 = 10 并发/用户；每次请求取信号量，60 秒无活动自动释放（[internal/handlers/auth.go:417-447](internal/handlers/auth.go#L417-L447)）。`UseGlm=1` 的用户**不**扣减请求次数（视为内部渠道）。

**Q: 阈值扣费的规则？**
A: 每次成功请求先扣 1 次额度；当 `total_tokens > 50000` 时，每多 50k 向上取整额外扣 1 次（[internal/handlers/response.go:195-202](internal/handlers/response.go#L195-L202)）。例：80k tokens 总共扣 2 次（1 + ceil((80000-50000)/50000)=2）。

**Q: 仓库根的 `test4-13*.txt` / `testlog*.txt` 是什么？**
A: 早期开发 / 调试时残留的日志文件，建议清理。这些文件不影响运行，但会污染仓库。

**Q: 我可以同时跑多个实例吗？**
A: Session 鉴权使用进程内内存 map，**多实例不共享 Session**——同一用户在多个实例上会得到不同的会话。Session 改造为 Redis 存储是潜在改进点（`config.yaml` 已预留 `redis` 段）。
