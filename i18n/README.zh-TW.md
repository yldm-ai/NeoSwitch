# NeoSwitch

[English](../README.md) | [简体中文](README.zh-CN.md) | **繁體中文** | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md) | [Português (Brasil)](README.pt-BR.md) | [Русский](README.ru.md)

[![CI](https://github.com/yldm-ai/neon-switch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/neon-switch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/neon-switch.svg)](https://pkg.go.dev/github.com/yldm-ai/neon-switch) [![Go version](https://img.shields.io/github/go-mod/go-version/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/blob/main/go.mod) [![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/yldm-ai/neon-switch/blob/main/LICENSE) [![Last commit](https://img.shields.io/github/last-commit/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/commits/main) [![GitHub stars](https://img.shields.io/github/stars/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/stargazers)

[![Claude Code plugin](https://img.shields.io/badge/Claude%20Code-plugin-D97757)](https://github.com/yldm-ai/neon-switch/blob/main/.claude-plugin/plugin.json) [![Model: Gemini 4 Neon](https://img.shields.io/badge/model-Gemini%204%20Neon-4285F4)](https://github.com/yldm-ai/neon-switch/blob/main/docs/protocol-contract.md) [![Platform: macOS and Linux](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey)](https://github.com/yldm-ai/neon-switch/releases) [![Proxy: local-only](https://img.shields.io/badge/proxy-local--only-2ea44f)](https://github.com/yldm-ai/neon-switch/blob/main/docs/protocol-contract.md)

**Keep Claude Code. Switch your models.**

NeoSwitch 讓你在 Claude Code 中使用 Gemini，不必改變已經熟悉的工作流程。Claude Code 的 CLI、指令、權限、工具、skills、MCP 伺服器與 hooks 都維持不變；NeoSwitch 在本機執行一個小型 Proxy，把 Claude Code 的請求轉換給背後的模型。

目前 NeoSwitch 將 Claude Code 路由到 Gemini 4 Neon。其他模型供應商已在規劃中，尚未支援。

## 為什麼用 NeoSwitch

- **保留你的工作流程**：你仍然在用 Claude Code，改變的只有模型。
- **以專案為單位**：在一個專案中開啟路由，其他專案繼續使用 Anthropic。
- **本機優先**：Proxy 只監聽本機回送位址，使用隨機存取 token，不記錄提示詞、回應或憑證，也不經過任何遠端中繼。
- **如實說明相容性**：哪些功能可用、哪些部分可用、哪些缺少，都列在[下方](#相容性)。

## 選擇憑證方式

| 方式 | 登入方式 | 說明 |
|---|---|---|
| Antigravity OAuth | 透過官方 `agy` CLI 登入 Google 帳號 | 外掛的預設流程。請先閱讀下方警告。 |
| Gemini Developer API | `GEMINI_API_KEY` | 能否存取 Gemini 4 Neon 取決於你的 key。用 `neoswitch doctor` 檢查。 |
| Vertex AI | Google Cloud Application Default Credentials | 能否存取取決於你的專案與區域。用 `neoswitch doctor` 檢查。 |

> [!WARNING]
> [Google Antigravity 服務條款](https://antigravity.google/terms)（第 6 條）將透過第三方軟體、工具或服務存取 Antigravity 視為違約，並表示 Google 可以停權或終止相關的 Antigravity 與 Gemini CLI 帳號。透過 Antigravity OAuth 路由 Claude Code 屬於這種情況。只有在你願意讓登入的帳號承擔這項風險時才使用；否則請使用 Gemini API key 或 Vertex AI。`neoswitch doctor` 與 `neoswitch antigravity login` 也會印出相同的警告。

OAuth 憑證始終留在 `agy` 內部。NeoSwitch 不會讀取、複製或匯出 token。

## 讓 Claude Code 使用 Neon

### 事前準備

- Claude Code。
- 使用 Antigravity OAuth 時：官方 Antigravity CLI，並確認 `agy` 指令位於 PATH 中。
- Go。外掛第一次使用時會編譯內建的 NeoSwitch 執行檔。

### 安裝外掛

在 Claude Code 中執行：

```text
/plugin marketplace add yldm-ai/neon-switch
/plugin install neoswitch@neoswitch
```

如果已經安裝過外掛，新版本發布後在終端機中更新：

```bash
claude plugin marketplace update neoswitch
claude plugin update neoswitch@neoswitch
```

更新後重新啟動 Claude Code。

### 登入並啟用 Neon

在要使用 Neon 的專案中執行：

```text
/neoswitch:login
```

這個指令會完成以下步驟：

1. 啟動由 `agy` 管理的 ACP 驗證流程。
2. 需要 Google OAuth 時開啟瀏覽器。
3. 為目前專案啟用 NeoSwitch 路由。
4. 為 ACP 工作階段選擇 `gemini-4-neon`。

如果要改用 Gemini API key，在啟動 Claude Code 之前 export `GEMINI_API_KEY`，然後執行 `/neoswitch:on`，外掛沒有需要填寫的設定。如果之前用 `/neoswitch:login` 登入過，還要 export `NEOSWITCH_BACKEND=gemini`，否則會繼續使用登入時儲存的選擇。

關閉並重新開啟該專案中的 Claude Code。檢查路由：

```text
/neoswitch:status
```

狀態應顯示 `NeoSwitch ... -> gemini-4-neon`。Claude Code 的工作階段頂端仍可能顯示它請求的標籤，例如 `claude-opus-5-5`；實際的上游路由以 `/neoswitch:status` 顯示的為準。

`/neoswitch:antigravity-login` 是 `/neoswitch:login` 的別名。`/neoswitch:on` 與 `/neoswitch:off` 只切換路由，不會登入。

## 其他憑證方式

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

處理程序必須已經能讀取 Google Cloud SDK 的 Application Default Credentials。

`neoswitch doctor` 會查詢模型並執行一次單行生成，然後回報憑證是否可用，或不可用的原因：憑證遭拒、缺少權限、區域限制、未知模型、配額不足，或服務無法使用。

## 外掛指令

| 指令 | 作用 |
|---|---|
| `/neoswitch:login` | 透過 Antigravity OAuth 驗證，啟用目前專案的 Neon 路由並使用 ACP |
| `/neoswitch:on` | 為目前專案的新 Claude Code 工作階段啟用路由 |
| `/neoswitch:status` | 查看專案路由與 daemon 狀態 |
| `/neoswitch:off` | 為目前專案的新工作階段關閉路由 |

變更路由後要開新的 Claude Code 工作階段。目前的工作階段會維持原本的連線。

## 相容性

工作階段、權限、工具執行、MCP 伺服器、skills、hooks、`CLAUDE.md` 與 `/resume` 仍由 Claude Code 負責；NeoSwitch 只替換它們背後的模型。這些功能的效果取決於模型遵循 Claude Code 工具協定的程度。這一點在日常寫程式中經過實際使用，但沒有自動化的端對端測試涵蓋。

| 功能 | Gemini API / Vertex AI | Antigravity OAuth (ACP) |
|---|---|---|
| 串流文字 | 支援 | 支援 |
| 工具呼叫（含平行呼叫） | Gemini 原生 function calling | 部分支援：模型輸出 Claude 工具呼叫 JSON，整段傳回而非串流傳回 |
| 多輪工具呼叫與 `/resume` | 支援；thought signature 保存在工具 ID 中 | 支援 |
| 圖片 | 支援 | `agy` 提供圖片輸入時支援 |
| PDF | 支援 | 不支援；會告知模型有檔案被省略 |
| `/effort` | 尚未套用 | 對應到 `agy` 的思考層級 |
| token 用量與 `count_tokens` | 會回報；在 Developer API 上 `count_tokens` 為估計值 | 會回報，包含每個提示詞附加的約 14k token 的 `agy` 指示 |
| Gemini 4 Neon 存取 | 取決於你的 key 或專案；用 `neoswitch doctor` 檢查 | 已在登入的 `agy` 上驗證 |

兩種方式都不支援：網路搜尋、程式碼執行等伺服器端工具，URL 形式的圖片與文件來源，以及 Claude thinking 區塊。`cache_control` 與 `context_management` 會被忽略。NeoSwitch 不會在背景默默改用其他模型。

完整的端點約定見 [docs/protocol-contract.md](../docs/protocol-contract.md)，ACP 細節見 [docs/antigravity.md](../docs/antigravity.md)。

## 外掛運作方式

`/neoswitch:on` 只會在專案的 `.claude/settings.local.json` 中寫入 `ANTHROPIC_BASE_URL` 和 `ANTHROPIC_AUTH_TOKEN`。這個 token 用來授權本機回送 Proxy，不是 Antigravity 憑證。

SessionStart hook 會啟動一個本機 daemon，並在 Claude Code 發出第一個模型請求前登記該 Claude Code 處理程序。所有登記的 Claude Code 處理程序結束後，daemon 也會結束。

使用 OAuth 路由時，daemon 會啟動官方 `agy acp` 子處理程序。NeoSwitch 透過 ACP 工作階段送出轉換後的請求，再把傳回的回答或工具呼叫 JSON 轉回本機 Anthropic-compatible endpoint。一個 `agy` 處理程序服務所有工作階段，每個請求在各自的 ACP 工作階段中並行執行。每個請求都會拒絕 `agy` 自身的檔案、shell 與網路工具，因此工具只在 Claude Code 中執行，並遵循 Claude Code 的權限規則。

`/neoswitch:login` 會把 Antigravity OAuth 儲存為後端，環境中為其他工具設定的 Gemini key 不會搶占它。沒有儲存的選擇時，環境中有 Gemini 憑證就用 Gemini API，否則用 Antigravity OAuth。`NEOSWITCH_BACKEND=antigravity|gemini|auto` 可以覆蓋以上兩者，`/neoswitch:status` 會顯示目前使用的後端及原因。

## 獨立 CLI

每個加上標籤的版本都會在 [Releases](https://github.com/yldm-ai/neon-switch/releases) 頁面發布 macOS（Apple Silicon 與 Intel）、Linux（amd64 與 arm64）與 Windows（amd64）的執行檔，並附上 SHA-256 檢查碼。Windows 版本已編譯但尚未測試。也可以從原始碼建置：

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

常用指令：

```bash
neoswitch doctor              # 檢查 Claude CLI，以及憑證能否使用該模型
neoswitch antigravity login   # 透過官方 agy 用戶端完成 ACP OAuth 登入
neoswitch antigravity probe   # 檢查已儲存的 agy 工作階段，不印出憑證
neoswitch models              # 印出目前設定的模型
neoswitch on [project]        # 為專案啟用路由
neoswitch off [project]       # 為專案關閉路由
neoswitch status [project]    # 查看路由與 daemon 狀態
neoswitch version             # 印出 NeoSwitch 版本
```

為其他 Anthropic-compatible 用戶端執行本機 Proxy：

```bash
export NEOSWITCH_AUTH_TOKEN="$(openssl rand -hex 24)"
GEMINI_API_KEY='你的 API Key' neoswitch serve
```

在用戶端中同時設定 `ANTHROPIC_BASE_URL` 和 `ANTHROPIC_AUTH_TOKEN`。`neoswitch claude` 會以臨時連接埠和 token 自動設定這兩個變數。

## 疑難排解

| 現象 | 處理方式 |
|---|---|
| 不確定憑證是否可用 | 執行 `neoswitch doctor`，它會指出失敗的步驟與原因。 |
| `/neoswitch:login` 不存在 | 更新外掛、重新啟動 Claude Code，並執行 `/reload-plugins`。 |
| `agy: command not found` | 安裝官方 Antigravity CLI，或設定 `ANTIGRAVITY_CLI_PATH`。 |
| `no authorization code was entered` | 更新到最新外掛。目前流程使用 ACP `authenticate`，不讀取 shell stdin。 |
| 新工作階段出現 `ECONNREFUSED` | 執行 `/neoswitch:status`。如果路由已啟用，檢查 daemon 訊息並重新啟動工作階段。在使用 Anthropic 的工作階段中執行 `/neoswitch:off`，修正憑證後再重新啟用。 |
| 工作階段提示沒有設定後端 | 執行 `/neoswitch:login`，或在啟動 Claude Code 前 export `GEMINI_API_KEY` 或 Vertex AI 設定。 |
| 外掛編譯失敗 | 安裝 Go，然後重新啟動 Claude Code。 |

## 相容性邊界

NeoSwitch 使用官方 Gemini SDK，或本機未經修改的 `agy acp` 處理程序。它不會擷取 OAuth token、呼叫 Antigravity 私有 HTTP API、將憑證轉送到遠端 Proxy，也不會修改 hosts 檔案。

Anthropic 並未在文件中說明如何將 Claude Code 路由到非 Claude 模型，因此 Claude Code 更新可能改變 NeoSwitch 所依賴的行為。

## 發展藍圖

- 支援更多模型供應商，首先是 OpenAI。已經提供 Anthropic-compatible endpoint 的供應商可以直接設定 `ANTHROPIC_BASE_URL` 使用，不需要 NeoSwitch。
- 為外掛提供預先建置的執行檔，不再需要 Go。

## 開發

```bash
go test ./...
go test -race ./...
go vet ./...
```

MIT License
