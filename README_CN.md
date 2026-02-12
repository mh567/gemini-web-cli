# Gemini Web CLI

Google Gemini 终端客户端，基于 Google One 网页订阅。

Gemini Web CLI 让你直接在终端中与 Google Gemini 交互，无需 API Key。通过浏览器 Cookie 认证，访问 gemini.google.com 上的所有模型。

## 功能特性

- **交互式聊天** — 基于 Bubbletea 的 TUI 界面，支持 Markdown 渲染、流式响应、对话管理
- **单次提问** — `ask` 命令适用于脚本和快速提问
- **多模型切换** — 支持 Gemini 3.0 Pro、Flash、Flash Thinking
- **Gems 系统提示词** — 创建、列出、使用 Gems 自定义模型行为
- **思维链展示** — 通过 `--show-thoughts` 查看模型推理过程
- **图片提取** — 展示响应中的网络图片和 ImageFX 生成图片
- **文件上传** — 在对话中附加文件
- **对话历史** — 浏览和管理历史对话
- **多账户管理** — 支持多个 Google 账户，一键切换
- **Cookie 自动刷新** — 后台每 9 分钟轮换 PSIDTS，保持会话有效
- **自动重试** — 遇到瞬态错误和限流时自动指数退避重试
- **代理支持** — 支持 HTTP/SOCKS5 代理（CLI 参数、配置文件、环境变量）
- **自定义模型** — 在配置文件中定义自定义模型哈希值

## 安装

### 从源码构建

```bash
git clone https://github.com/harris/gemini-web-cli.git
cd gemini-web-cli
make build
```

编译产物位于 `bin/gemini-web-cli`。

### 交叉编译

```bash
make build-all  # darwin-arm64, darwin-amd64, linux-arm64
```

## 快速开始

```bash
# 1. 登录（自动打开浏览器进行 Google 认证）
gemini-web-cli login

# 2. 单次提问
gemini-web-cli ask "法国的首都是哪里？"

# 3. 进入交互式聊天
gemini-web-cli chat
```

## 使用说明

### ask 单次提问

```bash
gemini-web-cli ask "解释快速排序算法"
gemini-web-cli ask --model gemini-3.0-flash "总结这篇文章"
gemini-web-cli ask --gem "代码审查" "审查这个函数"
gemini-web-cli ask --show-thoughts "计算：23 * 47"
```

### chat 交互式聊天

```bash
gemini-web-cli chat
gemini-web-cli chat --model gemini-3.0-pro
gemini-web-cli chat --gem "写作助手"
```

聊天内置命令：
- `/new` — 开始新对话
- `/model <名称>` — 切换模型（或交互式选择器）
- `/upload <路径> [问题]` — 上传文件（可选：同时提问）
- `/history` — 浏览对话历史（方向键导航，回车打开，支持继续历史对话）
- `/thoughts [on|off]` — 切换思考过程显示（默认折叠）
- `Enter` — 发送消息
- `双击 ESC` — 取消正在生成的回复
- `Ctrl+C` — 退出

### gems 系统提示词管理

```bash
gemini-web-cli gems list
gemini-web-cli gems create --name "代码审查" --prompt "你是一位资深代码审查专家..." --desc "审查代码"
gemini-web-cli gems delete <gem-id>
```

### history 对话历史

```bash
gemini-web-cli history                      # 列出所有对话（CLI 模式）
```

在聊天模式中，`/history` 打开交互式历史浏览器：
- **↑↓** — 移动光标选择对话
- **← →** — 翻页（每页 10 条）
- **Enter** — 打开并查看对话内容
- **c** — 继续选中的对话（恢复会话上下文）
- **b** — 返回对话列表
- **↑↓**（详情视图中）— 滚动浏览消息
- **ESC** — 退出历史模式

### accounts 账户管理

```bash
gemini-web-cli login                        # 登录（默认账户）
gemini-web-cli login --account work         # 以指定名称登录
gemini-web-cli accounts list                # 列出所有账户
gemini-web-cli accounts switch work         # 切换默认账户
gemini-web-cli accounts remove work         # 删除账户
```

### 全局参数

```bash
--proxy socks5://127.0.0.1:1080       # 为所有请求设置代理
```

## 可用模型

| 名称 | 说明 |
|------|------|
| `default` | 服务器默认模型（不发送模型头） |
| `gemini-3.0-pro` | Gemini 3.0 Pro |
| `gemini-3.0-flash` | Gemini 3.0 Flash |
| `gemini-3.0-flash-thinking` | Gemini 3.0 Flash Thinking（支持 `--show-thoughts`） |

## 配置文件

配置路径：`~/.config/gemini-web-cli/config.json`（支持 `XDG_CONFIG_HOME`）

```json
{
  "default_account": "default",
  "default_model": "gemini-3.0-pro",
  "request_timeout": 120,
  "request_delay_ms": 500,
  "proxy": "socks5://127.0.0.1:1080",
  "custom_models": {
    "my-model": {
      "name": "自定义模型",
      "header_val": "[1,null,null,null,\"abcdef1234567890\",null,null,0,[4],null,null,1]"
    }
  }
}
```

配置项说明：

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `default_account` | 默认使用的账户名 | `"default"` |
| `default_model` | 默认模型 | `"gemini-2.5-pro"` |
| `request_timeout` | 请求超时（秒） | `120` |
| `request_delay_ms` | 请求间隔（毫秒） | `500` |
| `proxy` | 代理地址 | 空 |
| `custom_models` | 自定义模型映射表 | 空 |

## 项目结构

```
gemini-web-cli/
├── main.go                          # 程序入口
├── Makefile                         # 构建脚本
├── cmd/                             # CLI 命令层（Cobra）
│   ├── root.go                      # 根命令、全局参数
│   ├── ask.go                       # 单次提问命令
│   ├── chat.go                      # 交互式聊天命令
│   ├── gems.go                      # Gems 管理命令
│   ├── history.go                   # 对话历史命令
│   ├── login.go                     # 浏览器登录命令
│   ├── accounts.go                  # 多账户管理命令
│   └── helpers.go                   # 公共初始化逻辑
├── internal/
│   ├── api/                         # Gemini Web API 客户端
│   │   ├── client.go                # HTTP 客户端、认证、Cookie 刷新、重试
│   │   ├── generate.go              # StreamGenerate 端点、响应解析
│   │   ├── conversations.go         # batchexecute RPC、对话增删查
│   │   ├── gems.go                  # Gems 增删查（batchexecute）
│   │   ├── models.go                # 模型定义与查找
│   │   ├── errors.go                # 结构化错误码
│   │   ├── upload.go                # 文件上传
│   │   └── parsing.go               # 帧解析工具
│   ├── auth/                        # 认证模块
│   │   ├── cookies.go               # Cookie 处理、验证、轮换
│   │   ├── login.go                 # 浏览器登录流程
│   │   └── store.go                 # 凭证持久化存储
│   ├── config/                      # 配置模块
│   │   └── config.go                # JSON 配置加载/保存
│   └── tui/                         # 终端界面（Bubbletea）
│       ├── app.go                   # 顶层应用模型
│       ├── chat.go                  # 聊天视图（流式渲染）
│       ├── history.go               # 历史浏览视图
│       └── styles.go                # Lipgloss 样式定义
├── pkg/version/                     # 版本信息（ldflags 注入）
└── go.mod
```

## 技术原理

Gemini Web CLI 通过逆向 Gemini 网页接口实现与模型的交互：

### 1. 认证流程

通过无头浏览器（go-rod）打开 Google 登录页面，用户完成登录后提取以下 Cookie：
- `__Secure-1PSID` — 主会话标识
- `__Secure-1PSIDTS` — 会话时间戳（需定期轮换）
- `__Secure-1PSIDCC` — 会话校验码

Cookie 通过系统密钥链（macOS Keychain / Linux Secret Service）安全存储。

### 2. 会话初始化

请求 `gemini.google.com/app` 页面，从 HTML 中正则提取三个关键令牌：
- `SNlM0e` — CSRF 防伪令牌（每次请求必须携带）
- `cfb2h` — 请求上下文标识（`bl` 参数）
- `FdrFJe` — 会话 ID（`f.sid` 参数）

### 3. 消息生成（StreamGenerate）

向 `/_/BardChatUi/data/assistant.lamda.BardFrontendService/StreamGenerate` 端点发送 POST 请求：

- **请求体**：`f.req` 字段包含双重 JSON 编码的 69 元素数组
  - `inner[0]` — 消息内容 `[prompt, 0, null, fileData, null, null, 0]`
  - `inner[2]` — 对话元数据（会话 ID、响应 ID、选择 ID）
  - `inner[7]` — 流式标志（设为 1）
  - `inner[19]` — Gem ID（可选）
- **模型选择**：通过 `x-goog-ext-525001261-jspb` 请求头指定模型哈希值
- **响应格式**：长度前缀帧协议，每帧包含截至当前的完整文本（非增量）

### 4. batchexecute RPC

对话管理和 Gems 操作通过 `/_/BardChatUi/data/batchexecute` 端点实现，每个操作对应一个 RPC ID：

| 操作 | RPC ID | 说明 |
|------|--------|------|
| 列出对话 | `MaZiqc` | 获取对话列表 |
| 获取对话 | `hNvQHb` | 获取对话详情 |
| 删除对话 | `GzXR5e` | 删除指定对话 |
| 列出 Gems | `CNgdBe` | 获取所有 Gems |
| 创建 Gem | `oMH3Zd` | 创建新 Gem |
| 更新 Gem | `kHv0Vd` | 更新已有 Gem |
| 删除 Gem | `UXcSJb` | 删除指定 Gem |

### 5. 可靠性机制

**结构化错误码：**

| 错误码 | 含义 | 是否可重试 |
|--------|------|-----------|
| 1013 | 瞬态错误 | 是 |
| 1037 | 请求限流 | 是 |
| 1050 | 模型不匹配 | 否 |
| 1052 | 模型头无效 | 否 |
| 1060 | IP 被封禁 | 否 |

**自动重试：** 遇到可重试错误时，按 `(attempt+1) * 5秒` 的间隔进行指数退避，最多重试 3 次。

**Cookie 自动刷新：** 后台 goroutine 每 9 分钟调用 Google 的 `RotateCookies` 端点轮换 `PSIDTS`，并通过读写锁（`sync.RWMutex`）保证线程安全。

## 依赖项

### 运行环境
- Google One 订阅（用于访问 Gemini）
- Chrome/Chromium 浏览器（首次登录时需要）

### 构建环境
- Go 1.21+

### 主要依赖库

| 库 | 用途 |
|----|------|
| `spf13/cobra` | CLI 命令框架 |
| `charmbracelet/bubbletea` | TUI 框架 |
| `charmbracelet/glamour` | 终端 Markdown 渲染 |
| `charmbracelet/lipgloss` | 终端样式 |
| `charmbracelet/bubbles` | TUI 组件（输入框、视口、加载动画） |
| `go-rod/rod` | 无头浏览器（登录流程） |
| `zalando/go-keyring` | 系统密钥链存储 |

## 许可证

MIT
