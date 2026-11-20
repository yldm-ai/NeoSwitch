# NeoSwitch

[English](../README.md) | [简体中文](README.zh-CN.md) | [繁體中文](README.zh-TW.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md) | [Português (Brasil)](README.pt-BR.md) | **Русский**

[![CI](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/NeoSwitch.svg)](https://pkg.go.dev/github.com/yldm-ai/NeoSwitch) [![License](https://img.shields.io/github/license/yldm-ai/NeoSwitch)](https://github.com/yldm-ai/NeoSwitch/blob/main/LICENSE)

NeoSwitch — плагин для Claude Code, который направляет проекты в Gemini 4 Neon через локальный Anthropic-совместимый прокси. Можно выбрать Antigravity OAuth, Gemini Developer API или Vertex AI, сохранив привычный рабочий процесс Claude Code, потоковый вывод и вызов инструментов.

Поддерживаются два способа получения учётных данных для вышестоящего сервиса:

- Antigravity OAuth через официальный локальный процесс `agy acp`.
- Учётные данные Gemini Developer API или Vertex AI через официальный SDK `google.golang.org/genai`.

Для Claude Code рекомендуется сценарий плагина с Antigravity OAuth. Учётные данные OAuth остаются внутри `agy`; NeoSwitch не читает и не экспортирует токен.

## Использование Claude Code с Neon

### Требования

- Claude Code.
- Официальный Antigravity CLI, доступный в PATH как `agy`.
- Go. При первом использовании плагин собирает встроенный бинарный файл NeoSwitch.

### Установка плагина

Выполните эти команды в Claude Code:

```text
/plugin marketplace add yldm-ai/NeoSwitch
/plugin install neoswitch@neoswitch
```

Если плагин уже установлен, после выхода новой версии обновите его из терминала:

```bash
claude plugin marketplace update neoswitch
claude plugin update neoswitch@neoswitch
```

После обновления перезапустите Claude Code. Для входа через ACP нужна актуальная версия плагина.

### Вход и включение Neon

В проекте, где вы хотите использовать Neon, выполните:

```text
/neoswitch:login
```

Команда выполняет следующие шаги:

1. Запускает процесс аутентификации ACP, которым управляет `agy`.
2. Открывает браузер, если требуется Google OAuth.
3. Включает маршрутизацию NeoSwitch для текущего проекта.
4. Выбирает `gemini-4-neon` для сессии ACP.

После завершения команды закройте и снова откройте Claude Code в этом проекте. Проверьте маршрут командой:

```text
/neoswitch:status
```

Статус должен показывать `NeoSwitch ... -> gemini-4-neon`. В заголовке сессии Claude Code может по-прежнему отображаться запрошенная метка, например `claude-opus-5-5`. Фактический вышестоящий маршрут показывает `/neoswitch:status`.

`/neoswitch:antigravity-login` — псевдоним `/neoswitch:login`. Команды `/neoswitch:on` и `/neoswitch:off` по-прежнему доступны, если нужно переключить маршрутизацию без входа.

## Другие способы получения учётных данных

### Ключ API Gemini

```bash
export GEMINI_API_KEY='your-key'
go run ./cmd/neoswitch claude
```

Плагин также может получить ключ через `/plugin` → `neoswitch` → **Gemini API key**. Claude Code хранит ключ в защищённой конфигурации плагинов.

### Vertex AI

```bash
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT=my-project
export GOOGLE_CLOUD_LOCATION=global
go run ./cmd/neoswitch claude
```

Application Default Credentials из Google Cloud SDK должны быть доступны процессу.

## Команды плагина

| Команда | Действие |
|---|---|
| `/neoswitch:login` | Аутентифицирует через Antigravity OAuth, включает маршрут Neon для этого проекта и использует ACP |
| `/neoswitch:on` | Включает маршрутизацию для новых сессий Claude Code в этом проекте |
| `/neoswitch:status` | Показывает маршрут проекта и состояние демона |
| `/neoswitch:off` | Отключает маршрутизацию для новых сессий в этом проекте |

После изменения маршрутизации запустите новую сессию Claude Code. Текущая сессия сохраняет существующее подключение.

## Как работает плагин

`/neoswitch:on` записывает в файл проекта `.claude/settings.local.json` только `ANTHROPIC_BASE_URL` и `ANTHROPIC_AUTH_TOKEN`. Этот токен авторизует loopback-прокси и не является учётными данными Antigravity.

Хук SessionStart запускает один локальный демон и регистрирует процесс Claude Code до его первого запроса к модели. Демон завершается, когда завершены все зарегистрированные процессы Claude Code. По умолчанию прокси слушает только loopback-интерфейс.

При маршруте OAuth демон запускает официальный подпроцесс `agy acp`. NeoSwitch отправляет преобразованный запрос через сессию ACP и преобразует полученный ответ или Claude-совместимый JSON вызова инструмента обратно в формат локальной Anthropic-совместимой конечной точки. Учётными данными OAuth и обратными вызовами браузера по-прежнему управляет `agy`.

Один процесс `agy` обслуживает все сессии, а запросы выполняются параллельно в отдельных сессиях ACP. Текст передаётся потоком по мере генерации, расход токенов сообщается Claude Code, а настройка `/effort` в Claude Code определяет, сколько времени Neon тратит на размышления.

Необязательная настройка плагина **Use Antigravity OAuth via ACP** принудительно включает маршрут ACP, если одновременно есть другие учётные данные Gemini. Обычный `/neoswitch:login` автоматически использует ACP, если учётные данные Gemini не настроены.

## Автономный CLI

Соберите бинарный файл или запустите его из репозитория:

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

Основные команды:

```bash
neoswitch antigravity login   # вход ACP OAuth через официальный клиент agy
neoswitch antigravity probe   # проверка сохранённой сессии agy без вывода учётных данных
neoswitch doctor              # проверка выбранного бэкенда и Claude CLI
neoswitch models              # вывод настроенной модели
neoswitch on [project]        # включить маршрутизацию для проекта
neoswitch off [project]       # отключить маршрутизацию для проекта
neoswitch status [project]    # показать маршрутизацию и состояние демона
```

Чтобы запустить локальный прокси для другого Anthropic-совместимого клиента:

```bash
export NEOSWITCH_AUTH_TOKEN="$(openssl rand -hex 24)"
GEMINI_API_KEY='your-key' neoswitch serve
```

Задайте в клиенте обе переменные: `ANTHROPIC_BASE_URL` и `ANTHROPIC_AUTH_TOKEN`. `neoswitch claude` настраивает их автоматически, используя временный порт и токен.

## Устранение неполадок

| Симптом | Решение |
|---|---|
| Команда `/neoswitch:login` неизвестна | Обновите плагин, перезапустите Claude Code и выполните `/reload-plugins`. |
| `agy: command not found` | Установите официальный Antigravity CLI или задайте `ANTIGRAVITY_CLI_PATH`. |
| `no authorization code was entered` | Обновите плагин до последней версии. Текущий процесс использует ACP `authenticate` и не читает stdin оболочки. |
| `ECONNREFUSED` в новой сессии | Выполните `/neoswitch:status`. Если маршрут включён, проверьте сообщение демона и перезапустите сессию. Выполните `/neoswitch:off` из сессии, использующей Anthropic, и снова включите маршрут после исправления учётных данных. |
| Сессия запускается без учётных данных Gemini | Выполните `/neoswitch:login` или настройте ключ API Gemini либо Vertex AI ADC. |
| Сборка плагина завершается ошибкой | Установите Go и перезапустите Claude Code. |

## Границы совместимости

NeoSwitch использует официальный Gemini SDK или немодифицированный локальный процесс `agy acp`. Он не извлекает токены OAuth, не обращается к закрытым HTTP API Antigravity, не передаёт учётные данные на удалённый прокси, не изменяет файл hosts и никогда незаметно не переключается на другую модель.

За сессию, разрешения, MCP и цикл инструментов по-прежнему отвечает Claude Code. Anthropic не документирует маршрутизацию Claude Code на модели, отличные от Claude, поэтому обновления Claude Code могут изменить поведение, на которое опирается NeoSwitch.

Контракт конечной точки и матрица совместимости описаны в [docs/protocol-contract.md](../docs/protocol-contract.md), подробности аутентификации ACP — в [docs/antigravity.md](../docs/antigravity.md).

## Разработка

```bash
go test ./...
go test -race ./...
go vet ./...
```

MIT License
