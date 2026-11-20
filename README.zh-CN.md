# NeoSwitch

[English](README.md) | [简体中文](README.zh-CN.md)

NeoSwitch 是一个本地运行、兼容性优先的桥接器，让 Claude Code 通过官方 Gemini API 使用 Gemini 4 Neon。

NeoSwitch 的定位很明确：Claude Code 继续负责 Agent 运行时、工具循环、权限和会话；NeoSwitch 只负责 Anthropic Messages API 与 Gemini API 之间的协议转换。代理默认只监听本机回环地址，凭证只存在于进程环境中，不会把提示词或响应写入磁盘。

## 当前状态

目前支持：

- `neoswitch serve`：启动本地 Anthropic-compatible Proxy。
- `neoswitch claude`：通过临时本地 Proxy 启动 Claude Code。
- `neoswitch antigravity login`：调用官方 `agy` CLI 完成浏览器 OAuth 登录。
- `neoswitch antigravity probe`：验证现有 `agy` 登录是否可用，不读取 Antigravity token；它不是 Claude Code 后端。
- 文本、图片、PDF、Claude Code 的工具 schema、多轮工具调用（包括 Gemini 的 thought signature）、流式与非流式响应，以及 `count_tokens`。
- 通过官方 `google.golang.org/genai` SDK 接入 Gemini Developer API 或 Vertex AI，默认模型 `gemini-4-neon`。

## 快速开始

### 1. 准备凭证

使用 Gemini API Key：

```bash
export GEMINI_API_KEY='你的 Gemini API Key'
```

或者使用 Vertex AI 和 Application Default Credentials：

```bash
export GOOGLE_GENAI_USE_VERTEXAI=true GOOGLE_CLOUD_PROJECT=my-project GOOGLE_CLOUD_LOCATION=global
```

### 2. 启动 Claude Code

在项目目录中运行：

```bash
go run ./cmd/neoswitch claude
```

也可以显式指定模型和 Claude CLI 路径：

```bash
GEMINI_MODEL=gemini-4-neon \
CLAUDE_CLI_PATH=claude \
go run ./cmd/neoswitch claude
```

### 3. 直接启动 Proxy

如果你要把 NeoSwitch 接到其他 Anthropic-compatible 客户端：

```bash
export NEOSWITCH_AUTH_TOKEN="$(openssl rand -hex 24)"
GEMINI_API_KEY='你的 Gemini API Key' go run ./cmd/neoswitch serve

# 在客户端的环境里
ANTHROPIC_BASE_URL=http://127.0.0.1:8080 ANTHROPIC_AUTH_TOKEN="$NEOSWITCH_AUTH_TOKEN" claude
```

两个变量都要设置：如果 Claude Code 只拿到 base URL，会把已登录的 claude.ai 凭证发给 Proxy。`neoswitch claude` 会自动用随机端口和一次性 token 完成这一步。

Proxy 默认监听 `127.0.0.1:8080`。只有在确实需要时，才设置 `NEOSWITCH_LISTEN` 到其他回环地址。`GEMINI_BASE_URL` 用于覆盖 Gemini API 的主机地址。

## Claude Code 插件

装上插件后，可以只把选定的项目切到 Gemini，其他项目里照常直接用 `claude`，不需要记额外的启动命令。

```text
/plugin marketplace add yldm-ai/NeoSwitch
/plugin install neoswitch@neoswitch
```

在插件配置里填入 Gemini API Key（`/plugin` → neoswitch），Claude Code 会把它存进 macOS 钥匙串。不填的话，NeoSwitch 会使用启动 Claude Code 时环境里的 `GEMINI_API_KEY` 或 Vertex AI 凭证。

| 命令 | 作用 |
|---|---|
| `/neoswitch:on` | 让这个项目之后的新会话走 Gemini |
| `/neoswitch:status` | 查看路由状态和代理是否在运行 |
| `/neoswitch:off` | 让这个项目之后的新会话改回 Anthropic |

执行 `/neoswitch:on` 或 `/neoswitch:off` 之后，要在项目里开一个新会话才生效，正在运行的会话保持原来的连接。

工作方式：

- `/neoswitch:on` 在项目的 `.claude/settings.local.json` 里加入 `ANTHROPIC_BASE_URL` 和 `ANTHROPIC_AUTH_TOKEN`，指向一个所有已启用项目共用的本机代理。文件里的其他设置原样保留，`/neoswitch:off` 只删除这两项。
- SessionStart hook 会在会话发出第一个请求之前启动代理，并把这个 Claude Code 进程登记给代理。所有登记过的会话都退出后，代理自己退出，关掉 Claude Code 后不会留下后台进程。
- 插件第一次使用时会用 Go 编译自带的源码，所以需要安装 Go。支持 macOS 和 Linux。

注意事项：

- 启用后 `.claude/settings.local.json` 里会存有代理 token，这个文件不要提交到版本库；如果 git 没有忽略它，`/neoswitch:on` 会给出提示。
- 已启用的项目里无法使用 Remote Control：只要 `ANTHROPIC_BASE_URL` 不指向 Anthropic，Claude Code 就会禁用它。
- Anthropic 的文档写明不支持把 Claude Code 路由到非 Claude 模型，所以 Claude Code 升级后，NeoSwitch 依赖的行为可能发生变化。

## 常用命令

```bash
neoswitch on [项目目录]           # 让项目的新会话走 Gemini（插件的 /neoswitch:on 调用的就是它）
neoswitch off [项目目录]          # 改回 Anthropic
neoswitch status [项目目录]       # 查看路由和代理状态
neoswitch doctor                 # 检查 Gemini 凭证和 Claude CLI
neoswitch models                 # 查看当前配置的模型
neoswitch antigravity login      # 通过官方 agy CLI 完成 OAuth 登录
neoswitch antigravity probe      # 检查 agy 登录，不输出 token
```

如果尚未安装二进制，也可以把 `neoswitch` 替换为：

```bash
go run ./cmd/neoswitch
```

## 稳定性边界

NeoSwitch 的稳定核心只使用官方 Gemini API Key 或 Vertex AI（ADC）凭证。以下方式不属于稳定核心：

- 读取或导出 Antigravity OAuth/token profile。
- 抓取 Antigravity 内部接口。
- 修改 hosts 或拦截 TLS 流量。
- 把用户凭证转发到远程第三方 Proxy。
- 在模型不可用时静默切换到其他模型。

如果配置的 Google 账号无权访问 Gemini 4 Neon，NeoSwitch 会返回明确错误，而不是偷偷切换模型。

## Antigravity 登录说明

Antigravity 的登录状态不是 `GEMINI_API_KEY`。官方 `agy` CLI 自己管理账户凭证和 token profile，不能把登录后的 token 直接填入 `GEMINI_API_KEY`。

可以用下面的命令检查登录状态：

```bash
neoswitch antigravity probe
```

首次登录或切换账号时，运行下面的命令。它会把终端交给官方 `agy` CLI，由 `agy` 自动打开浏览器完成 Google OAuth，并把会话保存到系统钥匙串。远程 SSH 环境下，`agy` 会显示授权 URL 和验证码。

```bash
neoswitch antigravity login
```

这个命令只通过官方 `agy` CLI 发起一次固定的无敏感测试请求，并且只输出成功或失败状态。它不会读取、打印或保存 token，也不会让 Claude Code 使用 Antigravity 的 Agent 循环。

如果 probe 失败，请先在终端运行 `agy` 完成登录，再重新执行 probe。不要把 token 粘贴到 issue、日志或环境变量中。

## 架构

```text
Claude Code
    │ Anthropic Messages API
    ▼
NeoSwitch 本地 Proxy
    │ Gemini generateContent
    ▼
Gemini 4 Neon
```

Claude Code 继续控制：

- 文件编辑和权限确认。
- MCP 和工具循环。
- 会话恢复。
- 项目配置和 Agent 工作流。

NeoSwitch 负责：

- Anthropic 请求解析。
- Gemini 请求转换。
- Gemini 响应转换。
- 工具调用 ID 和工具结果关联。
- 本地认证、健康检查和错误诊断。

## 文档

- [协议合同和兼容矩阵](docs/protocol-contract.md)
- [Antigravity 账户会话说明](docs/antigravity.md)

## 开发与验证

```bash
go test ./...
go test -race ./...
go vet ./...
```

项目当前优先保证协议边界清晰和失败可诊断，不追求第一版支持大量 Provider。稳定性优先于模型数量和功能数量。

## 许可证

MIT License
