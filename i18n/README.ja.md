# NeoSwitch

[English](../README.md) | [简体中文](README.zh-CN.md) | [繁體中文](README.zh-TW.md) | **日本語** | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md) | [Português (Brasil)](README.pt-BR.md) | [Русский](README.ru.md)

[![CI](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/NeoSwitch.svg)](https://pkg.go.dev/github.com/yldm-ai/NeoSwitch) [![License](https://img.shields.io/github/license/yldm-ai/NeoSwitch)](https://github.com/yldm-ai/NeoSwitch/blob/main/LICENSE)

NeoSwitch は、ローカルの Anthropic 互換プロキシを介してプロジェクトを Gemini 4 Neon にルーティングする Claude Code プラグインです。Claude Code のワークフロー、ストリーミング、ツール呼び出しはそのままに、Antigravity OAuth、Gemini Developer API、Vertex AI のいずれかを選べます。

上流の認証情報は次の 2 つの経路に対応しています。

- 公式のローカル `agy acp` プロセスを使った Antigravity OAuth。
- 公式 `google.golang.org/genai` SDK を使った Gemini Developer API または Vertex AI の認証情報。

Claude Code では Antigravity OAuth のプラグインフローを推奨します。OAuth の認証情報は `agy` の中にとどまり、NeoSwitch がトークンを読み取ったり書き出したりすることはありません。

## Claude Code で Neon を使う

### 前提条件

- Claude Code。
- 公式 Antigravity CLI（`agy` として PATH 上にあること）。
- Go。プラグインは初回使用時に同梱の NeoSwitch バイナリをビルドします。

### プラグインのインストール

Claude Code 内で次のコマンドを実行します。

```text
/plugin marketplace add yldm-ai/NeoSwitch
/plugin install neoswitch@neoswitch
```

インストール済みの場合は、新しいリリースの後にシェルから更新します。

```bash
claude plugin marketplace update neoswitch
claude plugin update neoswitch@neoswitch
```

更新後は Claude Code を再起動してください。ACP ログインフローには最新バージョンのプラグインが必要です。

### サインインして Neon を有効にする

Neon を使いたいプロジェクトで次を実行します。

```text
/neoswitch:login
```

このコマンドは次の処理を行います。

1. `agy` が管理する ACP 認証フローを開始します。
2. Google OAuth が必要な場合はブラウザを開きます。
3. 現在のプロジェクトで NeoSwitch のルーティングを有効にします。
4. ACP セッションで `gemini-4-neon` を選択します。

コマンドが完了したら、そのプロジェクトの Claude Code を閉じて開き直してください。ルートは次のコマンドで確認できます。

```text
/neoswitch:status
```

`NeoSwitch ... -> gemini-4-neon` と表示されれば正常です。Claude Code のセッションヘッダーには `claude-opus-5-5` などのリクエストラベルが表示されることがありますが、実際の上流ルートは `/neoswitch:status` で確認してください。

`/neoswitch:antigravity-login` は `/neoswitch:login` のエイリアスです。ログインせずにルーティングだけを切り替えたい場合は `/neoswitch:on` と `/neoswitch:off` を使えます。

## その他の認証方法

### Gemini API キー

```bash
export GEMINI_API_KEY='your-key'
go run ./cmd/neoswitch claude
```

`/plugin` → `neoswitch` → **Gemini API key** からキーを設定することもできます。キーは Claude Code のセキュアなプラグイン設定に保存されます。

### Vertex AI

```bash
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT=my-project
export GOOGLE_CLOUD_LOCATION=global
go run ./cmd/neoswitch claude
```

プロセスから Google Cloud SDK の Application Default Credentials を利用できる状態にしておく必要があります。

## プラグインコマンド

| コマンド | 効果 |
|---|---|
| `/neoswitch:login` | Antigravity OAuth で認証し、このプロジェクトの Neon ルートを有効にして ACP を使う |
| `/neoswitch:on` | このプロジェクトの新しい Claude Code セッションでルーティングを有効にする |
| `/neoswitch:status` | プロジェクトのルートとデーモンの状態を表示する |
| `/neoswitch:off` | このプロジェクトの新しいセッションでルーティングを無効にする |

ルーティングを変更したら、新しい Claude Code セッションを開始してください。現在のセッションは既存の接続を維持します。

## プラグインの仕組み

`/neoswitch:on` がプロジェクトの `.claude/settings.local.json` に書き込むのは `ANTHROPIC_BASE_URL` と `ANTHROPIC_AUTH_TOKEN` だけです。このトークンはループバックプロキシの認可用であり、Antigravity の認証情報ではありません。

SessionStart フックは、最初のモデルリクエストより前にローカルデーモンを 1 つ起動し、Claude Code プロセスを登録します。登録されたすべての Claude Code プロセスが終了すると、デーモンも終了します。プロキシはデフォルトでループバックアドレスのみをリッスンします。

OAuth ルートでは、デーモンが公式の `agy acp` サブプロセスを起動します。NeoSwitch は変換したリクエストを ACP セッション経由で送信し、返ってきた回答や Claude 互換の tool-call JSON をローカルの Anthropic 互換エンドポイントへ変換して返します。OAuth の認証情報とブラウザのコールバックは引き続き `agy` が管理します。

1 つの `agy` プロセスがすべてのセッションを処理し、各リクエストは個別の ACP セッションで並行して実行されます。テキストは生成されると同時にストリーミングされ、トークン使用量は Claude Code に報告されます。Claude Code の `/effort` 設定で Neon の思考時間を調整できます。

別の Gemini 認証情報も存在する場合、プラグイン設定の **Use Antigravity OAuth via ACP** を有効にすると ACP ルートを強制できます。Gemini 認証情報が設定されていなければ、通常の `/neoswitch:login` は自動的に ACP を使います。

## スタンドアロン CLI

バイナリをビルドするか、リポジトリから実行します。

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

主なコマンド：

```bash
neoswitch antigravity login   # 公式 agy クライアントで ACP OAuth ログイン
neoswitch antigravity probe   # 保存済みの agy セッションを確認（認証情報は表示しない）
neoswitch doctor              # 選択中のバックエンドと Claude CLI を確認
neoswitch models              # 設定されたモデルを表示
neoswitch on [project]        # プロジェクトのルーティングを有効化
neoswitch off [project]       # プロジェクトのルーティングを無効化
neoswitch status [project]    # ルーティングとデーモンの状態を表示
```

他の Anthropic 互換クライアント向けにローカルプロキシを起動するには：

```bash
export NEOSWITCH_AUTH_TOKEN="$(openssl rand -hex 24)"
GEMINI_API_KEY='your-key' neoswitch serve
```

クライアント側で `ANTHROPIC_BASE_URL` と `ANTHROPIC_AUTH_TOKEN` の両方を設定してください。`neoswitch claude` は一時的なポートとトークンでこれらの変数を自動設定します。

## トラブルシューティング

| 症状 | 対処 |
|---|---|
| `/neoswitch:login` が見つからない | プラグインを更新し、Claude Code を再起動して `/reload-plugins` を実行します。 |
| `agy: command not found` | 公式 Antigravity CLI をインストールするか、`ANTIGRAVITY_CLI_PATH` を設定します。 |
| `no authorization code was entered` | 最新のプラグインに更新してください。現在のフローは ACP `authenticate` を使い、シェルの stdin を読みません。 |
| 新しいセッションで `ECONNREFUSED` | `/neoswitch:status` を実行します。ルートが有効ならデーモンのメッセージを確認してセッションを再起動します。Anthropic を使うセッションから `/neoswitch:off` を実行し、認証情報を修正してから再度有効にしてください。 |
| Gemini の認証情報がない状態でセッションが始まる | `/neoswitch:login` を実行するか、Gemini API キーまたは Vertex AI ADC を設定します。 |
| プラグインのビルドに失敗する | Go をインストールして Claude Code を再起動します。 |

## 互換性の範囲

NeoSwitch は公式 Gemini SDK、または改変していないローカルの `agy acp` プロセスを使用します。OAuth トークンの抽出、Antigravity の非公開 HTTP API の呼び出し、リモートプロキシへの認証情報の転送、hosts ファイルの変更、別モデルへの暗黙のフォールバックは行いません。

セッション、権限、MCP、ツールループは引き続き Claude Code が担います。Anthropic は Claude Code を Claude 以外のモデルへルーティングする方法を文書化していないため、Claude Code の更新によって NeoSwitch が依存する挙動が変わる可能性があります。

エンドポイントの契約と互換性マトリクスは [docs/protocol-contract.md](../docs/protocol-contract.md)、ACP 認証の詳細は [docs/antigravity.md](../docs/antigravity.md) を参照してください。

## 開発

```bash
go test ./...
go test -race ./...
go vet ./...
```

MIT License
