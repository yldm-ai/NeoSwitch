# NeoSwitch

[English](../README.md) | **简体中文** | [繁體中文](README.zh-TW.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md) | [Português (Brasil)](README.pt-BR.md) | [Русский](README.ru.md)

[![CI](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/NeoSwitch.svg)](https://pkg.go.dev/github.com/yldm-ai/NeoSwitch) [![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/yldm-ai/NeoSwitch/blob/main/LICENSE)

**Keep Claude Code. Switch your models.**

NeoSwitch 让你在 Claude Code 中使用 Gemini，而不必改变已经熟悉的工作流。Claude Code 的 CLI、命令、权限、工具、skills、MCP 服务器和 hooks 都保持不变；NeoSwitch 在本机运行一个小型代理，把 Claude Code 的请求转换给背后的模型。

目前 NeoSwitch 把 Claude Code 路由到 Gemini 4 Neon。其他模型提供商已在计划中，尚未支持。

## 为什么用 NeoSwitch

- **保留你的工作流**：你仍然在用 Claude Code，变的只是模型。
- **按项目启用**：在一个项目中开启路由，其他项目继续使用 Anthropic。
- **本机优先**：代理只监听本机回环地址，使用随机访问 token，不记录提示词、回复或凭证，也不经过任何远程中转。
- **如实说明兼容性**：哪些功能可用、哪些部分可用、哪些缺失，都列在[下文](#兼容性)。

## 选择凭证方式

| 方式 | 登录方式 | 说明 |
|---|---|---|
| Antigravity OAuth | 通过官方 `agy` CLI 登录 Google 账号 | 插件的默认流程。请先阅读下方警告。 |
| Gemini Developer API | `GEMINI_API_KEY` | 能否访问 Gemini 4 Neon 取决于你的 key。用 `neoswitch doctor` 检查。 |
| Vertex AI | Google Cloud Application Default Credentials | 能否访问取决于你的项目和区域。用 `neoswitch doctor` 检查。 |

> [!WARNING]
> [Google Antigravity 服务条款](https://antigravity.google/terms)（第 6 条）将通过第三方软件、工具或服务访问 Antigravity 视为违约，并说明 Google 可以暂停或终止相关的 Antigravity 和 Gemini CLI 账号。通过 Antigravity OAuth 路由 Claude Code 属于这种情况。只有在你接受登录账号承担这一风险时才使用它；否则请使用 Gemini API key 或 Vertex AI。`neoswitch doctor` 和 `neoswitch antigravity login` 也会打印同样的警告。

OAuth 凭证始终留在 `agy` 内部。NeoSwitch 不读取、复制或导出 token。

## 让 Claude Code 使用 Neon

### 准备环境

- Claude Code。
- 使用 Antigravity OAuth 时：官方 Antigravity CLI，并确保命令 `agy` 在 PATH 中。
- Go。插件首次使用时会编译内置的 NeoSwitch 二进制。

### 安装插件

在 Claude Code 中执行：

```text
/plugin marketplace add yldm-ai/NeoSwitch
/plugin install neoswitch@neoswitch
```

如果已经安装过插件，新版本发布后在终端中更新：

```bash
claude plugin marketplace update neoswitch
claude plugin update neoswitch@neoswitch
```

更新后重启 Claude Code。

### 登录并启用 Neon

在要使用 Neon 的项目中运行：

```text
/neoswitch:login
```

这个命令会完成以下步骤：

1. 启动由 `agy` 管理的 ACP 认证流程。
2. 需要 Google OAuth 时打开浏览器。
3. 为当前项目启用 NeoSwitch 路由。
4. 为 ACP 会话选择 `gemini-4-neon`。

如果要改用 Gemini API key，在 `/plugin` → `neoswitch` → **Gemini API key** 中填写（Claude Code 会把它保存在安全的插件配置中），然后运行 `/neoswitch:on`。

关闭并重新打开该项目中的 Claude Code。检查路由：

```text
/neoswitch:status
```

状态应显示 `NeoSwitch ... -> gemini-4-neon`。Claude Code 的会话顶部仍可能显示它请求的标签，例如 `claude-opus-5-5`；实际的上游路由以 `/neoswitch:status` 显示的为准。

`/neoswitch:antigravity-login` 是 `/neoswitch:login` 的别名。`/neoswitch:on` 和 `/neoswitch:off` 只切换路由，不会登录。

## 其他凭证方式

### Gemini API key

```bash
export GEMINI_API_KEY='你的 API Key'
neoswitch doctor
go run ./cmd/neoswitch claude
```

### Vertex AI

```bash
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT=my-project
export GOOGLE_CLOUD_LOCATION=global
neoswitch doctor
go run ./cmd/neoswitch claude
```

进程需要已经能够读取 Google Cloud SDK 的 Application Default Credentials。

`neoswitch doctor` 会查询模型并执行一次单行生成，然后报告凭证是否可用，或不可用的原因：凭证被拒绝、缺少权限、区域限制、未知模型、配额不足，或服务不可用。

## 插件命令

| 命令 | 作用 |
|---|---|
| `/neoswitch:login` | 通过 Antigravity OAuth 认证，启用当前项目的 Neon 路由并使用 ACP |
| `/neoswitch:on` | 为当前项目的新 Claude Code 会话启用路由 |
| `/neoswitch:status` | 查看项目路由和 daemon 状态 |
| `/neoswitch:off` | 为当前项目的新会话关闭路由 |

修改路由后要新开 Claude Code 会话。当前会话会保持原来的连接。

## 兼容性

会话、权限、工具执行、MCP 服务器、skills、hooks、`CLAUDE.md` 和 `/resume` 仍由 Claude Code 负责；NeoSwitch 只替换它们背后的模型。这些功能的效果取决于模型遵循 Claude Code 工具协议的程度。这一点在日常编码中经过实际使用，但没有自动化的端到端测试覆盖。

| 能力 | Gemini API / Vertex AI | Antigravity OAuth (ACP) |
|---|---|---|
| 流式文本 | 支持 | 支持 |
| 工具调用（含并行调用） | Gemini 原生 function calling | 部分支持：模型输出 Claude 工具调用 JSON，整段返回而非流式返回 |
| 多轮工具调用和 `/resume` | 支持；thought signature 保存在工具 ID 中 | 支持 |
| 图片 | 支持 | `agy` 提供图片输入时支持 |
| PDF | 支持 | 不支持；会告知模型有文件被省略 |
| `/effort` | 尚未生效 | 映射到 `agy` 的思考级别 |
| token 用量和 `count_tokens` | 会上报；在 Developer API 上 `count_tokens` 为估算值 | 会上报，包括每个提示词附加的约 14k token 的 `agy` 指令 |
| Gemini 4 Neon 访问 | 取决于你的 key 或项目；用 `neoswitch doctor` 检查 | 已在登录的 `agy` 上验证 |

两种方式都不支持：web 搜索、代码执行等服务端工具，URL 形式的图片和文档来源，以及 Claude thinking 块。`cache_control` 和 `context_management` 会被忽略。NeoSwitch 不会静默回退到其他模型。

完整的接口约定见 [docs/protocol-contract.md](../docs/protocol-contract.md)，ACP 细节见 [docs/antigravity.md](../docs/antigravity.md)。

## 插件工作方式

`/neoswitch:on` 只会在项目的 `.claude/settings.local.json` 中写入 `ANTHROPIC_BASE_URL` 和 `ANTHROPIC_AUTH_TOKEN`。这个 token 用于授权本机回环代理，不是 Antigravity 凭证。

SessionStart hook 会启动一个本机 daemon，并在 Claude Code 发出第一个模型请求前登记该 Claude Code 进程。所有登记的 Claude Code 进程退出后，daemon 也会退出。

使用 OAuth 路由时，daemon 会启动官方 `agy acp` 子进程。NeoSwitch 通过 ACP 会话发送转换后的请求，再把返回的回答或工具调用 JSON 转回本机 Anthropic-compatible endpoint。一个 `agy` 进程服务所有会话，各个请求在各自的 ACP 会话中并发执行。每个请求都会拒绝 `agy` 自带的文件、shell 和 web 工具，因此工具只在 Claude Code 中执行，并遵循 Claude Code 的权限规则。

如果同时存在其他 Gemini 凭证，可选的插件设置 **Use Antigravity OAuth via ACP** 会强制使用 ACP 路由。没有配置 Gemini 凭证时，正常运行 `/neoswitch:login` 会自动使用 ACP。

## 独立 CLI

每个打标签的版本都会在 [Releases](https://github.com/yldm-ai/NeoSwitch/releases) 页面发布 macOS（Apple Silicon 和 Intel）、Linux（amd64 和 arm64）和 Windows（amd64）的二进制，并附带 SHA-256 校验和。Windows 版本已编译但尚未测试。也可以从源码构建：

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

常用命令：

```bash
neoswitch doctor              # 检查 Claude CLI，以及凭证能否使用该模型
neoswitch antigravity login   # 通过官方 agy 客户端完成 ACP OAuth 登录
neoswitch antigravity probe   # 检查已保存的 agy 会话，不打印凭证
neoswitch models              # 打印当前配置的模型
neoswitch on [project]        # 为项目启用路由
neoswitch off [project]       # 为项目关闭路由
neoswitch status [project]    # 查看路由和 daemon 状态
neoswitch version             # 打印 NeoSwitch 版本
```

为其他 Anthropic-compatible 客户端运行本机代理：

```bash
export NEOSWITCH_AUTH_TOKEN="$(openssl rand -hex 24)"
GEMINI_API_KEY='你的 API Key' neoswitch serve
```

在客户端中同时设置 `ANTHROPIC_BASE_URL` 和 `ANTHROPIC_AUTH_TOKEN`。`neoswitch claude` 会使用临时端口和 token 自动配置这两个变量。

## 故障排查

| 现象 | 处理方式 |
|---|---|
| 不确定凭证是否可用 | 运行 `neoswitch doctor`，它会指出失败的步骤和原因。 |
| `/neoswitch:login` 不存在 | 更新插件、重启 Claude Code，并运行 `/reload-plugins`。 |
| `agy: command not found` | 安装官方 Antigravity CLI，或设置 `ANTIGRAVITY_CLI_PATH`。 |
| `no authorization code was entered` | 更新到最新插件。当前流程使用 ACP `authenticate`，不读取 shell stdin。 |
| 新会话出现 `ECONNREFUSED` | 运行 `/neoswitch:status`。如果路由已启用，检查 daemon 提示并重启会话。在使用 Anthropic 的会话中运行 `/neoswitch:off`，修复凭证后再重新启用。 |
| 会话启动时没有 Gemini 凭证 | 运行 `/neoswitch:login`，或配置 Gemini API key 或 Vertex AI ADC。 |
| 插件编译失败 | 安装 Go，然后重启 Claude Code。 |

## 兼容边界

NeoSwitch 使用官方 Gemini SDK，或本机未修改的 `agy acp` 进程。它不会提取 OAuth token、调用 Antigravity 私有 HTTP 接口、把凭证转发到远程代理，也不会修改 hosts 文件。

Anthropic 没有文档说明如何把 Claude Code 路由到非 Claude 模型，因此 Claude Code 更新可能改变 NeoSwitch 依赖的行为。

## 路线图

- 支持更多模型提供商，首先是 OpenAI。已经提供 Anthropic-compatible endpoint 的提供商可以直接设置 `ANTHROPIC_BASE_URL` 使用，不需要 NeoSwitch。
- 为插件提供预编译二进制，不再需要 Go。

## 开发

```bash
go test ./...
go test -race ./...
go vet ./...
```

MIT License
