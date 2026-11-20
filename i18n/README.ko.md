# NeoSwitch

[English](../README.md) | [简体中文](README.zh-CN.md) | [繁體中文](README.zh-TW.md) | [日本語](README.ja.md) | **한국어** | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md) | [Português (Brasil)](README.pt-BR.md) | [Русский](README.ru.md)

[![CI](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/NeoSwitch.svg)](https://pkg.go.dev/github.com/yldm-ai/NeoSwitch) [![License](https://img.shields.io/github/license/yldm-ai/NeoSwitch)](https://github.com/yldm-ai/NeoSwitch/blob/main/LICENSE)

NeoSwitch는 로컬 Anthropic 호환 프록시를 통해 프로젝트를 Gemini 4 Neon으로 라우팅하는 Claude Code 플러그인입니다. Claude Code의 워크플로, 스트리밍, 도구 호출은 그대로 유지하면서 Antigravity OAuth, Gemini Developer API, Vertex AI 중 하나를 선택할 수 있습니다.

업스트림 자격 증명은 두 가지 경로를 지원합니다.

- 공식 로컬 `agy acp` 프로세스를 통한 Antigravity OAuth.
- 공식 `google.golang.org/genai` SDK를 통한 Gemini Developer API 또는 Vertex AI 자격 증명.

Claude Code에서는 Antigravity OAuth 플러그인 흐름을 권장합니다. OAuth 자격 증명은 `agy` 안에만 머무르며, NeoSwitch는 토큰을 읽거나 내보내지 않습니다.

## Claude Code에서 Neon 사용하기

### 사전 요구 사항

- Claude Code.
- 공식 Antigravity CLI (`agy` 명령이 PATH에 있어야 함).
- Go. 플러그인은 처음 사용할 때 내장된 NeoSwitch 바이너리를 빌드합니다.

### 플러그인 설치

Claude Code 안에서 다음 명령을 실행합니다.

```text
/plugin marketplace add yldm-ai/NeoSwitch
/plugin install neoswitch@neoswitch
```

이미 설치되어 있다면 새 릴리스가 나온 뒤 셸에서 업데이트합니다.

```bash
claude plugin marketplace update neoswitch
claude plugin update neoswitch@neoswitch
```

업데이트 후 Claude Code를 다시 시작하세요. ACP 로그인 흐름에는 최신 버전의 플러그인이 필요합니다.

### 로그인하고 Neon 활성화하기

Neon을 사용할 프로젝트에서 다음을 실행합니다.

```text
/neoswitch:login
```

이 명령은 다음 단계를 수행합니다.

1. `agy`가 관리하는 ACP 인증 흐름을 시작합니다.
2. Google OAuth가 필요하면 브라우저를 엽니다.
3. 현재 프로젝트에 NeoSwitch 라우팅을 활성화합니다.
4. ACP 세션에서 `gemini-4-neon`을 선택합니다.

명령이 끝나면 해당 프로젝트의 Claude Code를 닫았다가 다시 여세요. 다음 명령으로 라우트를 확인합니다.

```text
/neoswitch:status
```

`NeoSwitch ... -> gemini-4-neon`이 표시되어야 합니다. Claude Code 세션 헤더에는 `claude-opus-5-5` 같은 요청 레이블이 계속 표시될 수 있으며, 실제 업스트림 라우트는 `/neoswitch:status`에서 확인할 수 있습니다.

`/neoswitch:antigravity-login`은 `/neoswitch:login`의 별칭입니다. 로그인 없이 라우팅만 바꾸고 싶다면 `/neoswitch:on`과 `/neoswitch:off`를 사용할 수 있습니다.

## 기타 자격 증명 방식

### Gemini API 키

```bash
export GEMINI_API_KEY='your-key'
go run ./cmd/neoswitch claude
```

`/plugin` → `neoswitch` → **Gemini API key**에서 키를 입력할 수도 있습니다. 키는 Claude Code의 보안 플러그인 설정에 저장됩니다.

### Vertex AI

```bash
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT=my-project
export GOOGLE_CLOUD_LOCATION=global
go run ./cmd/neoswitch claude
```

프로세스에서 Google Cloud SDK의 Application Default Credentials를 사용할 수 있어야 합니다.

## 플러그인 명령

| 명령 | 효과 |
|---|---|
| `/neoswitch:login` | Antigravity OAuth로 인증하고, 이 프로젝트의 Neon 라우트를 활성화하며, ACP를 사용 |
| `/neoswitch:on` | 이 프로젝트의 새 Claude Code 세션에 라우팅 활성화 |
| `/neoswitch:status` | 프로젝트 라우트와 데몬 상태 표시 |
| `/neoswitch:off` | 이 프로젝트의 새 세션에 라우팅 비활성화 |

라우팅을 변경한 뒤에는 새 Claude Code 세션을 시작하세요. 현재 세션은 기존 연결을 유지합니다.

## 플러그인 동작 방식

`/neoswitch:on`은 프로젝트의 `.claude/settings.local.json`에 `ANTHROPIC_BASE_URL`과 `ANTHROPIC_AUTH_TOKEN`만 기록합니다. 이 토큰은 루프백 프록시를 인가하는 용도이며 Antigravity 자격 증명이 아닙니다.

SessionStart 훅은 첫 모델 요청 전에 로컬 데몬 하나를 시작하고 Claude Code 프로세스를 등록합니다. 등록된 모든 Claude Code 프로세스가 종료되면 데몬도 종료됩니다. 프록시는 기본적으로 루프백 주소에서만 수신합니다.

OAuth 라우트에서는 데몬이 공식 `agy acp` 하위 프로세스를 시작합니다. NeoSwitch는 변환된 요청을 ACP 세션으로 보내고, 반환된 응답이나 Claude 호환 tool-call JSON을 로컬 Anthropic 호환 엔드포인트 형식으로 다시 변환합니다. OAuth 자격 증명과 브라우저 콜백은 계속 `agy`가 관리합니다.

하나의 `agy` 프로세스가 모든 세션을 처리하며, 요청은 각자의 ACP 세션에서 동시에 실행됩니다. 텍스트는 생성되는 대로 스트리밍되고, 토큰 사용량은 Claude Code에 보고되며, Claude Code의 `/effort` 설정으로 Neon의 사고 시간을 정할 수 있습니다.

다른 Gemini 자격 증명도 함께 있을 때는 플러그인 설정의 **Use Antigravity OAuth via ACP**를 켜서 ACP 라우트를 강제할 수 있습니다. Gemini 자격 증명이 설정되어 있지 않으면 일반 `/neoswitch:login`이 자동으로 ACP를 사용합니다.

## 독립 실행형 CLI

바이너리를 빌드하거나 저장소에서 바로 실행합니다.

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

자주 쓰는 명령:

```bash
neoswitch antigravity login   # 공식 agy 클라이언트로 ACP OAuth 로그인
neoswitch antigravity probe   # 자격 증명을 출력하지 않고 저장된 agy 세션 확인
neoswitch doctor              # 선택된 백엔드와 Claude CLI 점검
neoswitch models              # 설정된 모델 출력
neoswitch on [project]        # 프로젝트 라우팅 활성화
neoswitch off [project]       # 프로젝트 라우팅 비활성화
neoswitch status [project]    # 라우팅과 데몬 상태 표시
```

다른 Anthropic 호환 클라이언트용으로 로컬 프록시를 실행하려면:

```bash
export NEOSWITCH_AUTH_TOKEN="$(openssl rand -hex 24)"
GEMINI_API_KEY='your-key' neoswitch serve
```

클라이언트에 `ANTHROPIC_BASE_URL`과 `ANTHROPIC_AUTH_TOKEN`을 모두 설정하세요. `neoswitch claude`는 임시 포트와 토큰으로 이 변수들을 자동 설정합니다.

## 문제 해결

| 증상 | 해결 방법 |
|---|---|
| `/neoswitch:login`을 찾을 수 없음 | 플러그인을 업데이트하고 Claude Code를 다시 시작한 뒤 `/reload-plugins`를 실행합니다. |
| `agy: command not found` | 공식 Antigravity CLI를 설치하거나 `ANTIGRAVITY_CLI_PATH`를 설정합니다. |
| `no authorization code was entered` | 최신 플러그인으로 업데이트하세요. 현재 흐름은 ACP `authenticate`를 사용하며 셸 stdin을 읽지 않습니다. |
| 새 세션에서 `ECONNREFUSED` | `/neoswitch:status`를 실행합니다. 라우트가 활성화되어 있다면 데몬 메시지를 확인하고 세션을 다시 시작합니다. Anthropic을 사용하는 세션에서 `/neoswitch:off`를 실행한 뒤, 자격 증명을 고치고 다시 활성화하세요. |
| Gemini 자격 증명 없이 세션이 시작됨 | `/neoswitch:login`을 실행하거나 Gemini API 키 또는 Vertex AI ADC를 설정합니다. |
| 플러그인 빌드 실패 | Go를 설치하고 Claude Code를 다시 시작합니다. |

## 호환성 범위

NeoSwitch는 공식 Gemini SDK 또는 수정되지 않은 로컬 `agy acp` 프로세스를 사용합니다. OAuth 토큰을 추출하거나, Antigravity 비공개 HTTP API를 호출하거나, 자격 증명을 원격 프록시로 전달하거나, hosts 파일을 수정하거나, 다른 모델로 몰래 대체하지 않습니다.

세션, 권한, MCP, 도구 루프는 여전히 Claude Code가 담당합니다. Anthropic은 Claude Code를 Claude가 아닌 모델로 라우팅하는 방법을 문서화하지 않았으므로, Claude Code 업데이트로 NeoSwitch가 의존하는 동작이 바뀔 수 있습니다.

엔드포인트 계약과 호환성 매트릭스는 [docs/protocol-contract.md](../docs/protocol-contract.md)를, ACP 인증 세부 사항은 [docs/antigravity.md](../docs/antigravity.md)를 참고하세요.

## 개발

```bash
go test ./...
go test -race ./...
go vet ./...
```

MIT License
