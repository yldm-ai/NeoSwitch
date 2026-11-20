# NeoSwitch

[English](../README.md) | [简体中文](README.zh-CN.md) | [繁體中文](README.zh-TW.md) | [日本語](README.ja.md) | **한국어** | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md) | [Português (Brasil)](README.pt-BR.md) | [Русский](README.ru.md)

[![CI](https://github.com/yldm-ai/neon-switch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/neon-switch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/neon-switch.svg)](https://pkg.go.dev/github.com/yldm-ai/neon-switch) [![Go version](https://img.shields.io/github/go-mod/go-version/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/blob/main/go.mod) [![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/yldm-ai/neon-switch/blob/main/LICENSE) [![Last commit](https://img.shields.io/github/last-commit/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/commits/main) [![GitHub stars](https://img.shields.io/github/stars/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/stargazers)

[![Claude Code plugin](https://img.shields.io/badge/Claude%20Code-plugin-D97757)](https://github.com/yldm-ai/neon-switch/blob/main/.claude-plugin/plugin.json) [![Model: Gemini 4 Neon](https://img.shields.io/badge/model-Gemini%204%20Neon-4285F4)](https://github.com/yldm-ai/neon-switch/blob/main/docs/protocol-contract.md) [![Platform: macOS and Linux](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey)](https://github.com/yldm-ai/neon-switch/releases) [![Proxy: local-only](https://img.shields.io/badge/proxy-local--only-2ea44f)](https://github.com/yldm-ai/neon-switch/blob/main/docs/protocol-contract.md)

**Keep Claude Code. Switch your models.**

NeoSwitch를 사용하면 익숙한 워크플로를 바꾸지 않고 Claude Code에서 Gemini를 쓸 수 있습니다. Claude Code의 CLI, 명령, 권한, 도구, 스킬, MCP 서버, 훅은 그대로 유지되며, NeoSwitch는 작은 로컬 프록시를 실행해 Claude Code의 요청을 뒤에 있는 모델에 맞게 변환합니다.

현재 NeoSwitch는 Claude Code를 Gemini 4 Neon으로 라우팅합니다. 다른 제공자는 계획되어 있지만 아직 지원하지 않습니다.

## NeoSwitch를 쓰는 이유

- **워크플로 유지**: 여전히 Claude Code를 실행합니다. 바뀌는 것은 모델뿐입니다.
- **프로젝트 단위**: 한 프로젝트에서만 라우팅을 켜고 나머지 프로젝트는 Anthropic을 그대로 사용할 수 있습니다.
- **로컬 우선**: 프록시는 루프백에서만 수신하고, 무작위 접근 토큰을 사용하며, 프롬프트·응답·자격 증명을 로그에 남기지 않습니다. 원격 중계 서버는 거치지 않습니다.
- **솔직한 호환성 정보**: 동작하는 기능, 부분적으로 동작하는 기능, 지원하지 않는 기능을 [아래](#호환성)에 정리했습니다.

## 자격 증명 방식 선택

| 방식 | 로그인 방법 | 비고 |
|---|---|---|
| Antigravity OAuth | 공식 `agy` CLI를 통한 Google 계정 | 플러그인의 기본 흐름입니다. 먼저 아래 경고를 읽으세요. |
| Gemini Developer API | `GEMINI_API_KEY` | Gemini 4 Neon 접근 가능 여부는 키에 따라 다릅니다. `neoswitch doctor`로 확인하세요. |
| Vertex AI | Google Cloud Application Default Credentials | 접근 가능 여부는 프로젝트와 리전에 따라 다릅니다. `neoswitch doctor`로 확인하세요. |

> [!WARNING]
> [Google Antigravity 서비스 약관](https://antigravity.google/terms)(제6조)은 서드파티 소프트웨어, 도구 또는 서비스를 통해 Antigravity에 접근하는 것을 위반으로 보며, Google이 관련된 Antigravity 및 Gemini CLI 계정을 정지하거나 해지할 수 있다고 명시합니다. Antigravity OAuth를 통해 Claude Code를 라우팅하는 것은 이 규정에 해당합니다. 로그인하는 계정에 대해 이 위험을 감수할 수 있을 때만 사용하고, 그렇지 않다면 Gemini API 키나 Vertex AI를 사용하세요. `neoswitch doctor`와 `neoswitch antigravity login`도 같은 경고를 출력합니다.

OAuth 자격 증명은 항상 `agy` 안에만 머무릅니다. NeoSwitch는 토큰을 읽거나 복사하거나 내보내지 않습니다.

## Claude Code에서 Neon 사용하기

### 사전 요구 사항

- Claude Code.
- Antigravity OAuth를 쓰는 경우: 공식 Antigravity CLI (`agy` 명령이 PATH에 있어야 함).
- Go. 플러그인은 처음 사용할 때 내장된 NeoSwitch 바이너리를 빌드합니다.

### 플러그인 설치

Claude Code 안에서 다음 명령을 실행합니다.

```text
/plugin marketplace add yldm-ai/neon-switch
/plugin install neoswitch@neoswitch
```

이미 설치되어 있다면 새 릴리스가 나온 뒤 셸에서 업데이트합니다.

```bash
claude plugin marketplace update neoswitch
claude plugin update neoswitch@neoswitch
```

업데이트 후 Claude Code를 다시 시작하세요.

### 로그인하고 Neon 활성화하기

Neon을 사용하려는 프로젝트에서 다음을 실행합니다.

```text
/neoswitch:login
```

이 명령은 다음 단계를 수행합니다.

1. `agy`가 관리하는 ACP 인증 흐름을 시작합니다.
2. Google OAuth가 필요하면 브라우저를 엽니다.
3. 현재 프로젝트에 NeoSwitch 라우팅을 활성화합니다.
4. ACP 세션에 `gemini-4-neon`을 선택합니다.

대신 Gemini API 키를 사용하려면 Claude Code를 시작하기 전에 `GEMINI_API_KEY`를 export하고 `/neoswitch:on`을 실행하세요. 플러그인에는 입력할 설정이 없습니다. 이전에 `/neoswitch:login`으로 로그인했다면 `NEOSWITCH_BACKEND=gemini`도 export하세요. 그렇지 않으면 로그인 때 저장된 선택이 우선합니다.

해당 프로젝트에서 Claude Code를 닫았다가 다시 엽니다. 다음 명령으로 라우트를 확인합니다.

```text
/neoswitch:status
```

상태에는 `NeoSwitch ... -> gemini-4-neon`이 표시되어야 합니다. Claude Code는 세션 헤더에 `claude-opus-5-5`처럼 요청한 모델 이름을 계속 표시할 수 있습니다. 실제 업스트림 라우트는 `/neoswitch:status`에 표시되는 것입니다.

`/neoswitch:antigravity-login`은 `/neoswitch:login`의 별칭입니다. `/neoswitch:on`과 `/neoswitch:off`는 로그인 없이 라우팅만 변경합니다.

## 기타 자격 증명 방식

### Gemini API 키

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

프로세스가 Google Cloud SDK의 Application Default Credentials를 미리 사용할 수 있어야 합니다.

`neoswitch doctor`는 모델을 조회하고 한 줄짜리 생성을 실행한 뒤, 자격 증명이 동작하는지 또는 동작하지 않는 이유를 보고합니다. 이유로는 거부된 자격 증명, 권한 부족, 리전 제한, 알 수 없는 모델, 할당량, 서비스 사용 불가가 있습니다.

## 플러그인 명령

| 명령 | 효과 |
|---|---|
| `/neoswitch:login` | Antigravity OAuth로 인증하고, 이 프로젝트의 Neon 라우트를 활성화하며, ACP를 사용 |
| `/neoswitch:on` | 이 프로젝트의 새 Claude Code 세션에 라우팅 활성화 |
| `/neoswitch:status` | 프로젝트 라우트와 데몬 상태 표시 |
| `/neoswitch:off` | 이 프로젝트의 새 세션에 라우팅 비활성화 |

라우팅을 변경한 뒤에는 새 Claude Code 세션을 시작하세요. 현재 세션은 기존 연결을 유지합니다.

## 호환성

세션, 권한, 도구 실행, MCP 서버, 스킬, 훅, `CLAUDE.md`, `/resume`은 여전히 Claude Code가 실행합니다. NeoSwitch는 그 뒤의 모델만 교체합니다. 이 기능들이 얼마나 잘 동작하는지는 모델이 Claude Code의 도구 프로토콜을 얼마나 잘 따르는지에 달려 있으며, 이는 일상적인 코딩 작업에서 사용해 봤습니다. 자동화된 엔드투엔드 테스트가 변경될 때마다 실제 Claude Code CLI를 NeoSwitch를 거쳐 스크립트로 작성한 Gemini 서버에 대해 실행하고, 도구 호출과 결과, 파일 편집, Bash, 병렬 호출, 권한 거부, 훅, 스킬, `/resume`, 서브에이전트, MCP 도구가 Gemini API 경로를 그대로 통과하는지 확인합니다. 실제 모델이 이를 얼마나 잘 사용하는지는 측정하지 않습니다.

| 기능 | Gemini API / Vertex AI | Antigravity OAuth (ACP) |
|---|---|---|
| 텍스트 스트리밍 | 지원 | 지원 |
| 병렬 호출을 포함한 도구 호출 | Gemini 네이티브 함수 호출 | 부분 지원: 모델이 Claude tool-call JSON을 작성하며, 스트리밍되지 않고 한 번에 전달됨 |
| 멀티턴 도구 사용과 `/resume` | 지원. thought signature는 도구 ID에 담아 전달 | 지원 |
| 이미지 | 지원 | `agy`가 이미지 입력을 제공할 때 지원 |
| PDF | 지원 | 미지원. 파일이 생략되었다는 사실을 모델에 알림 |
| `/effort` | 아직 적용되지 않음 | `agy`의 사고 수준에 매핑 |
| 토큰 사용량과 `count_tokens` | 보고됨. Developer API에서는 `count_tokens`가 추정값 | 보고됨. 모든 프롬프트에 추가되는 약 14k 토큰의 `agy` 지시문 포함 |
| Gemini 4 Neon 접근 | 키나 프로젝트에 따라 다름. `neoswitch doctor`로 확인 | 로그인된 `agy`로 검증됨 |

두 방식 모두 지원하지 않는 기능: 웹 검색이나 코드 실행 같은 서버 도구, URL 이미지 및 문서 소스, Claude thinking 블록. `cache_control`과 `context_management`는 무시됩니다. NeoSwitch는 다른 모델로 몰래 대체하지 않습니다.

전체 엔드포인트 계약은 [docs/protocol-contract.md](../docs/protocol-contract.md)에, ACP 세부 사항은 [docs/antigravity.md](../docs/antigravity.md)에 있습니다.

## 플러그인 동작 방식

`/neoswitch:on`은 프로젝트의 `.claude/settings.local.json`에 `ANTHROPIC_BASE_URL`과 `ANTHROPIC_AUTH_TOKEN`만 기록합니다. 이 토큰은 루프백 프록시를 인가하는 용도이며 Antigravity 자격 증명이 아닙니다.

SessionStart 훅은 첫 모델 요청 전에 로컬 데몬 하나를 시작하고 Claude Code 프로세스를 등록합니다. 등록된 모든 Claude Code 프로세스가 종료되면 데몬도 종료됩니다.

OAuth 라우트에서는 데몬이 공식 `agy acp` 하위 프로세스를 시작합니다. NeoSwitch는 변환된 요청을 ACP 세션으로 보내고, 반환된 응답이나 tool-call JSON을 로컬 Anthropic 호환 엔드포인트 형식으로 다시 변환합니다. 하나의 `agy` 프로세스가 모든 세션을 처리하며, 요청은 각자의 ACP 세션에서 동시에 실행됩니다. `agy` 자체의 파일, 셸, 웹 도구는 모든 요청에서 거부되므로, 도구는 Claude Code 안에서 Claude Code의 권한 규칙에 따라서만 실행됩니다.

`/neoswitch:login`은 Antigravity OAuth를 백엔드로 저장하므로, 다른 도구용으로 export한 Gemini 키가 이를 대체하지 않습니다. 저장된 선택이 없으면 환경에 Gemini 자격 증명이 있을 때 Gemini API를, 없으면 Antigravity OAuth를 사용합니다. `NEOSWITCH_BACKEND=antigravity|gemini|auto`로 둘 다 재정의할 수 있으며, `/neoswitch:status`에서 사용 중인 백엔드와 그 이유를 확인할 수 있습니다.

## 독립 실행형 CLI

태그가 붙은 각 버전은 macOS(Apple Silicon 및 Intel), Linux(amd64 및 arm64), Windows(amd64)용 바이너리를 SHA-256 체크섬과 함께 [Releases](https://github.com/yldm-ai/neon-switch/releases) 페이지에 게시합니다. Windows 빌드는 컴파일은 되지만 아직 테스트되지 않았습니다. 소스에서 직접 빌드할 수도 있습니다.

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

자주 쓰는 명령:

```bash
neoswitch doctor              # Claude CLI와 자격 증명으로 모델을 사용할 수 있는지 확인
neoswitch antigravity login   # 공식 agy 클라이언트를 통한 ACP OAuth 로그인
neoswitch antigravity probe   # 자격 증명을 출력하지 않고 저장된 agy 세션 확인
neoswitch models              # 설정된 모델 출력
neoswitch on [project]        # 프로젝트 라우팅 활성화
neoswitch off [project]       # 프로젝트 라우팅 비활성화
neoswitch status [project]    # 라우팅과 데몬 상태 표시
neoswitch version             # NeoSwitch 버전 출력
```

다른 Anthropic 호환 클라이언트용으로 로컬 프록시를 실행하려면:

```bash
export NEOSWITCH_AUTH_TOKEN="$(openssl rand -hex 24)"
GEMINI_API_KEY='your-key' neoswitch serve
```

클라이언트에 `ANTHROPIC_BASE_URL`과 `ANTHROPIC_AUTH_TOKEN`을 모두 설정합니다. `neoswitch claude`는 임시 포트와 토큰으로 이 변수들을 자동으로 설정합니다.

## 문제 해결

| 증상 | 해결 방법 |
|---|---|
| 자격 증명이 동작하는지 확실하지 않음 | `neoswitch doctor`를 실행합니다. 실패한 단계와 원인을 알려 줍니다. |
| `/neoswitch:login`을 찾을 수 없음 | 플러그인을 업데이트하고 Claude Code를 다시 시작한 뒤 `/reload-plugins`를 실행합니다. |
| `agy: command not found` | 공식 Antigravity CLI를 설치하거나 `ANTIGRAVITY_CLI_PATH`를 설정합니다. |
| `no authorization code was entered` | 최신 플러그인으로 업데이트하세요. 현재 흐름은 ACP `authenticate`를 사용하며 셸 stdin을 읽지 않습니다. |
| 새 세션에서 `ECONNREFUSED` | `/neoswitch:status`를 실행합니다. 라우트가 활성화되어 있다면 데몬 메시지를 확인하고 세션을 다시 시작합니다. Anthropic을 사용하는 세션에서 `/neoswitch:off`를 실행한 뒤, 자격 증명을 고치고 다시 활성화하세요. |
| 백엔드가 설정되지 않았다는 메시지가 표시됨 | `/neoswitch:login`을 실행하거나, Claude Code를 시작하기 전에 `GEMINI_API_KEY` 또는 Vertex AI 설정을 export하세요. |
| 플러그인 빌드 실패 | Go를 설치하고 Claude Code를 다시 시작합니다. |

## 호환성 범위

NeoSwitch는 공식 Gemini SDK 또는 수정되지 않은 로컬 `agy acp` 프로세스를 사용합니다. OAuth 토큰을 추출하거나, Antigravity 비공개 HTTP API를 호출하거나, 자격 증명을 원격 프록시로 전달하거나, hosts 파일을 수정하지 않습니다.

Anthropic은 Claude Code를 Claude가 아닌 모델로 라우팅하는 방법을 문서화하지 않았으므로, Claude Code 업데이트로 NeoSwitch가 의존하는 동작이 바뀔 수 있습니다.

## 로드맵

- 더 많은 모델 제공자 지원(OpenAI부터 시작). 이미 Anthropic 호환 엔드포인트를 제공하는 제공자는 `ANTHROPIC_BASE_URL`을 직접 설정해 사용할 수 있으며 NeoSwitch가 필요하지 않습니다.
- 플러그인용 사전 빌드 바이너리를 제공해 Go가 더 이상 필요하지 않도록 할 예정입니다.

## 개발

```bash
go test ./...
go test -race ./...
go vet ./...
```

MIT License
