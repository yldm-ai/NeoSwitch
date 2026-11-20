# NeoSwitch

[English](../README.md) | [简体中文](README.zh-CN.md) | **繁體中文** | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md) | [Português (Brasil)](README.pt-BR.md) | [Русский](README.ru.md)

[![CI](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/NeoSwitch.svg)](https://pkg.go.dev/github.com/yldm-ai/NeoSwitch) [![License](https://img.shields.io/github/license/yldm-ai/NeoSwitch)](https://github.com/yldm-ai/NeoSwitch/blob/main/LICENSE)

NeoSwitch 是一個 Claude Code 外掛，透過本機 Anthropic-compatible Proxy 將專案路由到 Gemini 4 Neon。你可以選擇 Antigravity OAuth、Gemini Developer API 或 Vertex AI，同時保留 Claude Code 的工作階段、串流輸出與工具呼叫流程。

上游憑證支援兩種方式：

- 透過官方本機 `agy acp` 使用 Antigravity OAuth。
- 透過官方 `google.golang.org/genai` SDK 使用 Gemini Developer API 或 Vertex AI。

Claude Code 建議使用 Antigravity OAuth 外掛流程。OAuth 憑證始終由 `agy` 管理，NeoSwitch 不會讀取或匯出 token。

## 讓 Claude Code 使用 Neon

### 事前準備

- Claude Code。
- 官方 Antigravity CLI，並確認 `agy` 指令位於 PATH 中。
- Go。外掛第一次使用時會自動編譯內建的 NeoSwitch 執行檔。

### 安裝外掛

在 Claude Code 中執行：

```text
/plugin marketplace add yldm-ai/NeoSwitch
/plugin install neoswitch@neoswitch
```

外掛發布新版本後，在一般終端機中更新：

```bash
claude plugin marketplace update neoswitch
claude plugin update neoswitch@neoswitch
```

更新後重新啟動 Claude Code。ACP 登入流程需要目前版本的外掛。

### 登入並啟用 Neon

在要使用 Neon 的專案中執行：

```text
/neoswitch:login
```

這個指令會自動完成：

1. 啟動由 `agy` 管理的 ACP 驗證流程。
2. 需要時開啟瀏覽器完成 Google OAuth。
3. 開啟目前專案的 NeoSwitch 路由。
4. 為 ACP session 選擇 `gemini-4-neon`。

指令完成後，關閉並重新開啟該專案中的 Claude Code。檢查路由：

```text
/neoswitch:status
```

狀態應顯示 `NeoSwitch ... -> gemini-4-neon`。Claude Code 工作階段頂端仍可能顯示 `claude-opus-5-5` 這類請求標籤，這是用戶端標籤。實際上游路由以 `/neoswitch:status` 為準。

`/neoswitch:antigravity-login` 是 `/neoswitch:login` 的別名。需要在不重新登入的情況下切換路由時，仍可使用 `/neoswitch:on` 和 `/neoswitch:off`。

## 其他憑證方式

### Gemini API Key

```bash
export GEMINI_API_KEY='你的 API Key'
go run ./cmd/neoswitch claude
```

也可以在 `/plugin` → `neoswitch` → **Gemini API key** 中填寫。Claude Code 會把外掛設定中的 key 儲存在安全儲存區。

### Vertex AI

```bash
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT=my-project
export GOOGLE_CLOUD_LOCATION=global
go run ./cmd/neoswitch claude
```

處理程序必須能讀取 Google Cloud SDK 的 Application Default Credentials。

## 外掛指令

| 指令 | 作用 |
|---|---|
| `/neoswitch:login` | 完成 Antigravity OAuth，開啟目前專案的 Neon 路由並使用 ACP |
| `/neoswitch:on` | 為目前專案之後的新 Claude Code 工作階段啟用路由 |
| `/neoswitch:status` | 查看專案路由與 daemon 狀態 |
| `/neoswitch:off` | 為目前專案之後的新工作階段關閉路由 |

變更路由後要開新的 Claude Code 工作階段。執行中的工作階段會維持原本的連線。

## 外掛運作方式

`/neoswitch:on` 只會在專案的 `.claude/settings.local.json` 中寫入 `ANTHROPIC_BASE_URL` 和 `ANTHROPIC_AUTH_TOKEN`。這個 token 只用來授權本機 Proxy，不是 Antigravity 憑證。

SessionStart hook 會在 Claude Code 發出第一個模型請求前啟動一個本機 daemon，並登記目前的 Claude Code 處理程序。所有登記的 Claude Code 處理程序結束後，daemon 也會結束。Proxy 預設只監聽本機回送位址。

使用 OAuth 路由時，daemon 會啟動官方 `agy acp` 子處理程序。NeoSwitch 將轉換後的請求送往 ACP session，再把傳回的回答或 Claude-compatible tool-call JSON 轉回本機 Anthropic-compatible endpoint。OAuth 憑證與瀏覽器回呼始終由 `agy` 管理。

所有工作階段共用一個 `agy` 處理程序，每個請求在各自的 ACP session 中並行執行。文字邊產生邊串流傳回，token 用量會回報給 Claude Code，Claude Code 的 `/effort` 設定決定 Neon 的思考深度。

如果環境中同時存在其他 Gemini 憑證，可以在外掛設定中開啟 **Use Antigravity OAuth via ACP** 強制使用 ACP。沒有設定 Gemini 憑證時，正常執行 `/neoswitch:login` 會自動選擇 ACP。

## 獨立 CLI

從儲存庫建置或安裝：

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

常用指令：

```bash
neoswitch antigravity login   # 透過官方 agy 用戶端完成 ACP OAuth 登入
neoswitch antigravity probe   # 檢查已儲存的 agy 工作階段，不印出憑證
neoswitch doctor              # 檢查目前後端與 Claude CLI
neoswitch models              # 印出目前設定的模型
neoswitch on [專案目錄]       # 為專案開啟路由
neoswitch off [專案目錄]      # 為專案關閉路由
neoswitch status [專案目錄]   # 查看路由與 daemon 狀態
```

要將 NeoSwitch 接到其他 Anthropic-compatible 用戶端：

```bash
export NEOSWITCH_AUTH_TOKEN="$(openssl rand -hex 24)"
GEMINI_API_KEY='你的 API Key' neoswitch serve
```

用戶端必須同時設定 `ANTHROPIC_BASE_URL` 和 `ANTHROPIC_AUTH_TOKEN`。`neoswitch claude` 會自動以臨時連接埠和 token 設定這兩個變數。

## 疑難排解

| 現象 | 處理方式 |
|---|---|
| `/neoswitch:login` 不存在 | 更新外掛、重新啟動 Claude Code，並執行 `/reload-plugins`。 |
| `agy: command not found` | 安裝官方 Antigravity CLI，或設定 `ANTIGRAVITY_CLI_PATH`。 |
| `no authorization code was entered` | 更新到最新外掛。目前流程使用 ACP `authenticate`，不讀取 shell stdin。 |
| 新工作階段出現 `ECONNREFUSED` | 執行 `/neoswitch:status`。如果路由已啟用，檢查 daemon 訊息並重新啟動工作階段。需要先恢復 Anthropic 時，在能正常連線的工作階段中執行 `/neoswitch:off`，修正憑證後再重新啟用。 |
| 啟動時提示沒有 Gemini credentials | 執行 `/neoswitch:login`，或設定 Gemini API Key / Vertex AI ADC。 |
| 外掛編譯失敗 | 安裝 Go，然後重新啟動 Claude Code。 |

## 相容性邊界

NeoSwitch 使用官方 Gemini SDK，或本機未經修改的 `agy acp` 處理程序。它不會擷取 OAuth token、呼叫 Antigravity 私有 HTTP API、將憑證轉送到遠端 Proxy、修改 hosts 檔案，也不會在背景改用其他模型。

Claude Code 仍負責工作階段、權限、MCP 與工具迴圈。Anthropic 並未在文件中說明將 Claude Code 路由到非 Claude 模型的用法，因此 Claude Code 更新可能改變 NeoSwitch 所依賴的行為。

詳見[協定約定與相容性矩陣](../docs/protocol-contract.md)與 [Antigravity ACP 說明](../docs/antigravity.md)。

## 開發與驗證

```bash
go test ./...
go test -race ./...
go vet ./...
```

MIT License
