# NeoSwitch

[English](../README.md) | [简体中文](README.zh-CN.md) | [繁體中文](README.zh-TW.md) | **日本語** | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md) | [Português (Brasil)](README.pt-BR.md) | [Русский](README.ru.md)

[![CI](https://github.com/yldm-ai/neon-switch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/neon-switch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/neon-switch.svg)](https://pkg.go.dev/github.com/yldm-ai/neon-switch) [![Go version](https://img.shields.io/github/go-mod/go-version/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/blob/main/go.mod) [![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/yldm-ai/neon-switch/blob/main/LICENSE) [![Last commit](https://img.shields.io/github/last-commit/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/commits/main) [![GitHub stars](https://img.shields.io/github/stars/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/stargazers)

[![Claude Code plugin](https://img.shields.io/badge/Claude%20Code-plugin-D97757)](https://github.com/yldm-ai/neon-switch/blob/main/.claude-plugin/plugin.json) [![Model: Gemini 4 Neon](https://img.shields.io/badge/model-Gemini%204%20Neon-4285F4)](https://github.com/yldm-ai/neon-switch/blob/main/docs/protocol-contract.md) [![Platform: macOS and Linux](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey)](https://github.com/yldm-ai/neon-switch/releases) [![Proxy: local-only](https://img.shields.io/badge/proxy-local--only-2ea44f)](https://github.com/yldm-ai/neon-switch/blob/main/docs/protocol-contract.md)

**Keep Claude Code. Switch your models.**

NeoSwitch を使うと、使い慣れたワークフローを変えずに Claude Code で Gemini を利用できます。Claude Code の CLI、コマンド、権限、ツール、スキル、MCP サーバー、フックはそのままで、NeoSwitch はローカルで小さなプロキシを動かし、Claude Code のリクエストを背後のモデル向けに変換します。

現在、NeoSwitch は Claude Code を Gemini 4 Neon にルーティングします。他のプロバイダーは計画中で、まだ対応していません。

## NeoSwitch を使う理由

- **ワークフローはそのまま**：使うのは引き続き Claude Code で、変わるのはモデルだけです。
- **プロジェクト単位**：あるプロジェクトだけルーティングを有効にし、他のプロジェクトは Anthropic のまま使えます。
- **ローカルファースト**：プロキシはループバックのみをリッスンし、ランダムなアクセストークンを使い、プロンプト、レスポンス、認証情報をログに残しません。リモートの中継は一切介しません。
- **互換性を正直に記載**：動くもの、一部だけ動くもの、未対応のものは[後述](#互換性)の一覧にまとめています。

## 認証方法を選ぶ

| 方法 | サインイン方法 | 備考 |
|---|---|---|
| Antigravity OAuth | 公式 `agy` CLI 経由の Google アカウント | プラグインのデフォルトのフロー。先に下の警告を読んでください。 |
| Gemini Developer API | `GEMINI_API_KEY` | Gemini 4 Neon を使えるかはキーによります。`neoswitch doctor` で確認してください。 |
| Vertex AI | Google Cloud Application Default Credentials | 使えるかはプロジェクトとリージョンによります。`neoswitch doctor` で確認してください。 |

> [!WARNING]
> [Google Antigravity 利用規約](https://antigravity.google/terms)（第 6 条）は、サードパーティのソフトウェア、ツール、サービスを通じた Antigravity へのアクセスを違反とみなし、Google が関係する Antigravity および Gemini CLI のアカウントを停止または終了できるとしています。Antigravity OAuth 経由で Claude Code をルーティングすることはこれに該当します。サインインするアカウントについてそのリスクを受け入れる場合にのみ使ってください。そうでなければ Gemini API キーか Vertex AI を使ってください。`neoswitch doctor` と `neoswitch antigravity login` も同じ警告を表示します。

OAuth の認証情報は常に `agy` の中にとどまります。NeoSwitch がトークンを読み取ったり、コピーしたり、書き出したりすることはありません。

## Claude Code で Neon を使う

### 前提条件

- Claude Code。
- Antigravity OAuth を使う場合：公式 Antigravity CLI（`agy` として PATH 上にあること）。
- Go（任意）。Go がある場合、プラグインは初回使用時に同梱の NeoSwitch バイナリをソースからビルドします。Go がない場合は、リポジトリの `VERSION` ファイルが示すリリースのバイナリを macOS または Linux（amd64 または arm64）向けにダウンロードし、実行前に SHA-256 チェックサムを検証します。リリースの合間には、そのバイナリがプラグインのソースより古いことがあります。

### プラグインのインストール

Claude Code 内で次のコマンドを実行します。

```text
/plugin marketplace add yldm-ai/neon-switch
/plugin install neoswitch@neoswitch
```

インストール済みの場合は、新しいリリースの後にシェルから更新します。

```bash
claude plugin marketplace update neoswitch
claude plugin update neoswitch@neoswitch
```

更新後は Claude Code を再起動してください。

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

代わりに Gemini API キーを使う場合は、Claude Code を起動する前に `GEMINI_API_KEY` を export し、`/neoswitch:on` を実行します。プラグインに入力する設定はありません。以前 `/neoswitch:login` でサインインした場合は `NEOSWITCH_BACKEND=gemini` も export してください。そうしないとログイン時に保存された選択が優先されます。

そのプロジェクトの Claude Code を閉じて開き直してください。ルートは次のコマンドで確認できます。

```text
/neoswitch:status
```

`NeoSwitch ... -> gemini-4-neon` と表示されるはずです。Claude Code のセッションヘッダーには、`claude-opus-5-5` のようにリクエストしたラベルが表示されることがありますが、上流のルートは `/neoswitch:status` に表示されるものです。

`/neoswitch:antigravity-login` は `/neoswitch:login` のエイリアスです。`/neoswitch:on` と `/neoswitch:off` はサインインせずにルーティングだけを切り替えます。

## その他の認証方法

### Gemini API キー

```bash
export GEMINI_API_KEY='your-key'
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

プロセスから Google Cloud SDK の Application Default Credentials をすでに利用できる状態にしておく必要があります。

`neoswitch doctor` はモデルを照会して 1 行の生成を実行し、認証情報が使えるかどうか、使えない場合はその理由を報告します。理由は、認証情報の拒否、権限不足、リージョン制限、不明なモデル、クォータ、サービス利用不可のいずれかです。

## プラグインコマンド

| コマンド | 効果 |
|---|---|
| `/neoswitch:login` | Antigravity OAuth で認証し、このプロジェクトの Neon ルートを有効にして ACP を使う |
| `/neoswitch:on` | このプロジェクトの新しい Claude Code セッションでルーティングを有効にする |
| `/neoswitch:status` | プロジェクトのルートとデーモンの状態を表示する |
| `/neoswitch:off` | このプロジェクトの新しいセッションでルーティングを無効にする |

ルーティングを変更したら、新しい Claude Code セッションを開始してください。現在のセッションは既存の接続を維持します。

## 互換性

セッション、権限、ツール実行、MCP サーバー、スキル、フック、`CLAUDE.md`、`/resume` は引き続き Claude Code が担い、NeoSwitch はその背後のモデルだけを置き換えます。これらの機能がどの程度うまく動くかは、モデルが Claude Code のツールプロトコルにどれだけ従えるかによります。日常のコーディングで実際に使って確かめていますが、自動化されたエンドツーエンドのテストではカバーしていません。

| 機能 | Gemini API / Vertex AI | Antigravity OAuth (ACP) |
|---|---|---|
| テキストのストリーミング | 対応 | 対応 |
| ツール呼び出し（並列呼び出しを含む） | Gemini ネイティブの function calling | 一部対応：モデルが Claude のツール呼び出し JSON を書き、ストリーミングではなく一括で届く |
| 複数ターンのツール使用と `/resume` | 対応。thought signature はツール ID に保持される | 対応 |
| 画像 | 対応 | `agy` が画像入力を提供する場合に対応 |
| PDF | 対応 | 非対応。ファイルが省略されたことがモデルに伝えられる |
| `/effort` | Gemini の思考レベルとして送信。実際の API では未検証で、モデルが拒否した場合は外す | `agy` の思考レベルに対応付け |
| トークン使用量と `count_tokens` | 報告される。Developer API では `count_tokens` は推定値 | 報告される。すべてのプロンプトに追加される約 14k トークンの `agy` の指示を含む |
| Gemini 4 Neon へのアクセス | キーまたはプロジェクトによる。`neoswitch doctor` で確認 | サインイン済みの `agy` で検証済み |

どちらの経路でも非対応：Web 検索やコード実行などのサーバーツール、URL 指定の画像やドキュメントのソース、Claude の thinking ブロック。`cache_control` と `context_management` は無視されます。NeoSwitch が黙って別のモデルにフォールバックすることはありません。

エンドポイントの完全な契約は [docs/protocol-contract.md](../docs/protocol-contract.md)、ACP の詳細は [docs/antigravity.md](../docs/antigravity.md) にあります。

## プラグインの仕組み

`/neoswitch:on` がプロジェクトの `.claude/settings.local.json` に書き込むのは `ANTHROPIC_BASE_URL` と `ANTHROPIC_AUTH_TOKEN` だけです。このトークンはループバックプロキシの認可用であり、Antigravity の認証情報ではありません。

SessionStart フックはローカルデーモンを 1 つ起動し、最初のモデルリクエストより前に Claude Code プロセスを登録します。登録されたすべての Claude Code プロセスが終了すると、デーモンも終了します。

OAuth ルートでは、デーモンが公式の `agy acp` サブプロセスを起動します。NeoSwitch は変換したリクエストを ACP セッション経由で送信し、返ってきた回答やツール呼び出し JSON をローカルの Anthropic 互換エンドポイントへ変換して返します。1 つの `agy` プロセスがすべてのセッションを処理し、リクエストは個別の ACP セッションで並行して実行されます。`agy` 自身のファイル、シェル、Web ツールはすべてのリクエストで拒否されるため、ツールは Claude Code の中でのみ、Claude Code の権限ルールに従って実行されます。

`/neoswitch:login` は Antigravity OAuth をバックエンドとして保存するため、他のツール用に export した Gemini キーに切り替わることはありません。保存された選択がない場合、環境に Gemini 認証情報があれば Gemini API を、なければ Antigravity OAuth を使います。`NEOSWITCH_BACKEND=antigravity|gemini|auto` で両方を上書きでき、`/neoswitch:status` は使用中のバックエンドとその理由を表示します。

## スタンドアロン CLI

タグ付けされた各バージョンでは、macOS（Apple Silicon と Intel）、Linux（amd64 と arm64）、Windows（amd64）向けのバイナリが SHA-256 チェックサムとともに [Releases](https://github.com/yldm-ai/neon-switch/releases) ページで公開されます。Windows 版はビルドされていますが、まだテストされていません。ソースからビルドすることもできます。

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

主なコマンド：

```bash
neoswitch doctor              # Claude CLI と、認証情報でモデルを使えるかを確認
neoswitch antigravity login   # 公式 agy クライアントで ACP OAuth ログイン
neoswitch antigravity probe   # 保存済みの agy セッションを確認（認証情報は表示しない）
neoswitch models              # 設定されたモデルを表示
neoswitch on [project]        # プロジェクトのルーティングを有効化
neoswitch off [project]       # プロジェクトのルーティングを無効化
neoswitch status [project]    # ルーティングとデーモンの状態を表示
neoswitch version             # NeoSwitch のバージョンを表示
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
| 認証情報が使えるかわからない | `neoswitch doctor` を実行します。失敗したステップと原因を示します。 |
| `/neoswitch:login` が見つからない | プラグインを更新し、Claude Code を再起動して `/reload-plugins` を実行します。 |
| `agy: command not found` | 公式 Antigravity CLI をインストールするか、`ANTIGRAVITY_CLI_PATH` を設定します。 |
| `no authorization code was entered` | 最新のプラグインに更新してください。現在のフローは ACP `authenticate` を使い、シェルの stdin を読みません。 |
| 新しいセッションで `ECONNREFUSED` | `/neoswitch:status` を実行します。ルートが有効ならデーモンのメッセージを確認してセッションを再起動します。Anthropic を使うセッションから `/neoswitch:off` を実行し、認証情報を修正してから再度有効にしてください。 |
| バックエンドが設定されていないと表示される | `/neoswitch:login` を実行するか、Claude Code の起動前に `GEMINI_API_KEY` または Vertex AI の設定を export します。 |
| プラグインのビルドまたはダウンロードに失敗する | Go をインストールして Claude Code を再起動します。Go があればプラグインはソースからビルドし、何もダウンロードしません。Go がない場合は GitHub Releases にアクセスできるか確認します。チェックサムの不一致は、ダウンロードが拒否され何も実行されなかったことを意味します。 |

## 互換性の範囲

NeoSwitch は公式 Gemini SDK、または改変していないローカルの `agy acp` プロセスを使用します。OAuth トークンの抽出、Antigravity の非公開 HTTP API の呼び出し、リモートプロキシへの認証情報の転送、hosts ファイルの変更は行いません。

Anthropic は Claude Code を Claude 以外のモデルへルーティングする方法を文書化していないため、Claude Code の更新によって NeoSwitch が依存する挙動が変わる可能性があります。

## ロードマップ

- OpenAI を皮切りに、より多くのモデルプロバイダーに対応します。すでに Anthropic 互換エンドポイントを提供しているプロバイダーは、`ANTHROPIC_BASE_URL` を直接設定すれば使え、NeoSwitch は必要ありません。

## 開発

```bash
go test ./...
go test -race ./...
go vet ./...
```

リリースするには `VERSION` を更新し、対応するタグ（`v` に続けてその内容、例: `v0.1.0`）を push します。一致しないタグはリリースワークフローが拒否します。

MIT License
