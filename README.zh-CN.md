# NeoSwitch

[English](README.md)

[![CI](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/NeoSwitch.svg)](https://pkg.go.dev/github.com/yldm-ai/NeoSwitch) [![License](https://img.shields.io/github/license/yldm-ai/NeoSwitch)](https://github.com/yldm-ai/NeoSwitch/blob/main/LICENSE)

NeoSwitch 通过本机 Anthropic-compatible Proxy，让 Claude Code 使用 Gemini 4 Neon。上游凭证支持两种方式：

- 通过官方本机 `agy acp` 使用 Antigravity OAuth。
- 通过官方 `google.golang.org/genai` SDK 使用 Gemini Developer API 或 Vertex AI。

Claude Code 推荐使用 Antigravity OAuth 插件流程。OAuth 凭证始终由 `agy` 管理，NeoSwitch 不读取或导出 token。

## 让 Claude Code 使用 Neon

### 准备环境

- Claude Code。
- 官方 Antigravity CLI，并确保命令 `agy` 在 PATH 中。
- Go。插件首次使用时会自动编译内置的 NeoSwitch 二进制。

### 安装插件

在 Claude Code 中执行：

```text
/plugin marketplace add yldm-ai/NeoSwitch
/plugin install neoswitch@neoswitch
```

插件发布新版本后，在普通终端更新：

```bash
claude plugin marketplace update neoswitch
claude plugin update neoswitch@neoswitch
```

更新后重启 Claude Code。ACP 登录流程需要当前版本的插件。

### 登录并启用 Neon

在要使用 Neon 的项目中运行：

```text
/neoswitch:login
```

这个命令会自动完成：

1. 启动由 `agy` 管理的 ACP 认证流程。
2. 需要时打开浏览器完成 Google OAuth。
3. 打开当前项目的 NeoSwitch 路由。
4. 通过 ACP session 配置选择 `gemini-4-neon`。

命令完成后，关闭并重新打开该项目中的 Claude Code。检查路由：

```text
/neoswitch:status
```

状态应显示 `NeoSwitch ... -> gemini-4-neon`。Claude Code 会话顶部仍可能显示 `claude-opus-5-5` 这样的请求标签，这是客户端标签。实际上游路由以 `/neoswitch:status` 为准。

`/neoswitch:antigravity-login` 是 `/neoswitch:login` 的别名。需要在不重新登录的情况下切换路由时，仍可使用 `/neoswitch:on` 和 `/neoswitch:off`。

## 其他凭证方式

### Gemini API Key

```bash
export GEMINI_API_KEY='你的 API Key'
go run ./cmd/neoswitch claude
```

也可以在 `/plugin` → `neoswitch` → **Gemini API key** 中填写。Claude Code 会把插件配置中的 key 保存到安全存储。

### Vertex AI

```bash
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT=my-project
export GOOGLE_CLOUD_LOCATION=global
go run ./cmd/neoswitch claude
```

进程需要能够读取 Google Cloud SDK 的 Application Default Credentials。

## 插件命令

| 命令 | 作用 |
|---|---|
| `/neoswitch:login` | 完成 Antigravity OAuth，打开当前项目的 Neon 路由并使用 ACP |
| `/neoswitch:on` | 为当前项目之后的新 Claude Code 会话启用路由 |
| `/neoswitch:status` | 查看项目路由和代理状态 |
| `/neoswitch:off` | 为当前项目之后的新会话关闭路由 |

修改路由后要新开 Claude Code 会话。正在运行的会话会保持原来的连接。

## 插件工作方式

`/neoswitch:on` 只会在项目的 `.claude/settings.local.json` 中写入 `ANTHROPIC_BASE_URL` 和 `ANTHROPIC_AUTH_TOKEN`。这个 token 只用于授权本机代理，不是 Antigravity OAuth 凭证。

SessionStart hook 会在 Claude Code 发出第一个模型请求前启动一个本机 daemon，并登记当前 Claude Code 进程。所有登记的 Claude Code 进程退出后，daemon 也会退出。Proxy 默认只监听本机回环地址。

启用 OAuth 路由时，daemon 会启动官方 `agy acp` 子进程。NeoSwitch 把转换后的请求发给 ACP session，再把返回的回答或 Claude-compatible tool-call JSON 转回本机 Anthropic-compatible endpoint。OAuth 凭证和浏览器回调始终由 `agy` 管理。

所有会话共用一个 `agy` 进程，各个请求在各自的 ACP session 里并发执行。文本边生成边流式返回，token 用量会回传给 Claude Code。

如果环境里同时存在其他 Gemini 凭证，可以在插件配置里打开 **Use Antigravity OAuth via ACP** 强制使用 ACP。没有 Gemini 凭证时，正常运行 `/neoswitch:login` 会自动选择 ACP。

## 独立 CLI

从仓库构建或安装：

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

常用命令：

```bash
neoswitch antigravity login   # 通过官方 agy 客户端完成 ACP OAuth 登录
neoswitch antigravity probe   # 检查 agy 会话，不打印凭证
neoswitch doctor              # 检查当前后端和 Claude CLI
neoswitch models              # 打印当前模型
neoswitch on [项目目录]       # 为项目开启路由
neoswitch off [项目目录]      # 为项目关闭路由
neoswitch status [项目目录]   # 查看路由和 daemon 状态
```

要把 NeoSwitch 接到其他 Anthropic-compatible 客户端：

```bash
export NEOSWITCH_AUTH_TOKEN="$(openssl rand -hex 24)"
GEMINI_API_KEY='你的 API Key' neoswitch serve
```

客户端必须同时设置 `ANTHROPIC_BASE_URL` 和 `ANTHROPIC_AUTH_TOKEN`。`neoswitch claude` 会自动使用临时端口和 token 配置这两个变量。

## 故障排查

| 现象 | 处理方式 |
|---|---|
| `/neoswitch:login` 不存在 | 更新插件、重启 Claude Code，并运行 `/reload-plugins`。 |
| `agy: command not found` | 安装官方 Antigravity CLI，或设置 `ANTIGRAVITY_CLI_PATH`。 |
| `no authorization code was entered` | 更新到最新插件。当前流程使用 ACP `authenticate`，不读取 shell stdin。 |
| 新会话出现 `ECONNREFUSED` | 运行 `/neoswitch:status`。如果路由已启用，检查 daemon 提示并重启会话。需要先恢复 Anthropic 时，在能正常连接的会话中运行 `/neoswitch:off`。 |
| 启动时提示没有 Gemini credentials | 运行 `/neoswitch:login`，或配置 Gemini API Key / Vertex AI ADC。 |
| 插件编译失败 | 安装 Go，然后重启 Claude Code。 |

## 兼容边界

NeoSwitch 默认使用官方 Gemini SDK，也可以使用本机未修改的 `agy acp` 进程。它不会提取 OAuth token、调用 Antigravity 私有 HTTP 接口、把凭证转发到远程 Proxy、修改 hosts，或静默切换到其他模型。

Claude Code 仍负责会话、权限、MCP 和工具循环。Anthropic 没有文档化把 Claude Code 路由到非 Claude 模型的用法，因此 Claude Code 更新可能改变 NeoSwitch 依赖的行为。

详见[协议合同和兼容矩阵](docs/protocol-contract.md)与 [Antigravity ACP 说明](docs/antigravity.md)。

## 开发与验证

```bash
go test ./...
go test -race ./...
go vet ./...
```

MIT License
