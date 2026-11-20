# NeoSwitch

[English](../README.md) | [简体中文](README.zh-CN.md) | [繁體中文](README.zh-TW.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md) | [Português (Brasil)](README.pt-BR.md) | **Русский**

[![Latest release](https://img.shields.io/github/v/release/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/releases) [![CI](https://github.com/yldm-ai/neon-switch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/neon-switch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/neon-switch.svg)](https://pkg.go.dev/github.com/yldm-ai/neon-switch) [![Go version](https://img.shields.io/github/go-mod/go-version/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/blob/main/go.mod) [![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/yldm-ai/neon-switch/blob/main/LICENSE) [![Last commit](https://img.shields.io/github/last-commit/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/commits/main) [![GitHub stars](https://img.shields.io/github/stars/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/stargazers)

[![Claude Code plugin](https://img.shields.io/badge/Claude%20Code-plugin-D97757)](https://github.com/yldm-ai/neon-switch/blob/main/.claude-plugin/plugin.json) [![Model: Gemini 4 Neon](https://img.shields.io/badge/model-Gemini%204%20Neon-4285F4)](https://github.com/yldm-ai/neon-switch/blob/main/docs/protocol-contract.md) [![Platform: macOS and Linux](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey)](https://github.com/yldm-ai/neon-switch/releases) [![Proxy: local-only](https://img.shields.io/badge/proxy-local--only-2ea44f)](https://github.com/yldm-ai/neon-switch/blob/main/docs/protocol-contract.md)

**Keep Claude Code. Switch your models.**

NeoSwitch позволяет использовать Gemini в Claude Code, не меняя привычный рабочий процесс. Claude Code сохраняет свой CLI, команды, разрешения, инструменты, skills, MCP-серверы и хуки; NeoSwitch запускает небольшой локальный прокси, который преобразует запросы для стоящей за ним модели.

Сейчас NeoSwitch направляет Claude Code в Gemini 4 Neon. OpenAI доступен как экспериментальный бэкенд: его нужно выбрать явно, и он проверен только на сценарном тестовом сервере.

## Зачем NeoSwitch

- **Привычный рабочий процесс**: вы по-прежнему работаете в Claude Code. Меняется только модель.
- **Для каждого проекта отдельно**: включите маршрутизацию в одном проекте, а остальные оставьте на Anthropic.
- **Всё локально**: прокси слушает только loopback-интерфейс, использует случайный токен доступа и не записывает в журнал промпты, ответы или учётные данные. Удалённый ретранслятор не используется.
- **Честная совместимость**: что работает, что работает частично и чего нет, перечислено [ниже](#совместимость).

## Выбор способа получения учётных данных

| Способ | Как выполняется вход | Примечания |
|---|---|---|
| Antigravity OAuth | Аккаунт Google через официальный CLI `agy` | Сценарий плагина по умолчанию. Сначала прочитайте предупреждение ниже. |
| Gemini Developer API | `GEMINI_API_KEY` | Доступ к Gemini 4 Neon зависит от вашего ключа. Проверьте его с помощью `neoswitch doctor`. |
| Vertex AI | Application Default Credentials Google Cloud | Доступ зависит от вашего проекта и региона. Проверьте его с помощью `neoswitch doctor`. |

> [!WARNING]
> [Условия использования Google Antigravity](https://antigravity.google/terms) (раздел 6) считают доступ к Antigravity через стороннее программное обеспечение, инструменты или сервисы нарушением и допускают, что Google может приостановить или закрыть соответствующие аккаунты Antigravity и Gemini CLI. Маршрутизация Claude Code через Antigravity OAuth подпадает под это правило. Используйте этот способ, только если принимаете этот риск для аккаунта, под которым входите; в противном случае используйте ключ API Gemini или Vertex AI. `neoswitch doctor` и `neoswitch antigravity login` выводят то же предупреждение.

Учётные данные OAuth всегда остаются внутри `agy`. NeoSwitch не читает, не копирует и не экспортирует токен.

## Использование Claude Code с Neon

### Требования

- Claude Code.
- Для Antigravity OAuth: официальный Antigravity CLI, доступный в PATH как `agy`.
- Go (необязательно). Если Go установлен, при первом использовании плагин собирает встроенный бинарный файл NeoSwitch из исходного кода. Без Go он скачивает бинарный файл релиза, указанного в файле `VERSION` репозитория, для macOS или Linux (amd64 или arm64) и перед запуском проверяет его контрольную сумму SHA-256. Между релизами этот файл может быть старее исходного кода плагина.

### Установка плагина

Выполните эти команды в Claude Code:

```text
/plugin marketplace add yldm-ai/neon-switch
/plugin install neoswitch@neoswitch
```

Если плагин уже установлен, после выхода новой версии обновите его из терминала:

```bash
claude plugin marketplace update neoswitch
claude plugin update neoswitch@neoswitch
```

После обновления перезапустите Claude Code.

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

Чтобы вместо этого использовать ключ API Gemini, экспортируйте `GEMINI_API_KEY` перед запуском Claude Code и выполните `/neoswitch:on`; в плагине нет настроек для заполнения. Если вы ранее входили через `/neoswitch:login`, экспортируйте также `NEOSWITCH_BACKEND=gemini`, иначе будет использоваться выбор, сохранённый при входе.

Закройте и снова откройте Claude Code в этом проекте. Проверьте маршрут командой:

```text
/neoswitch:status
```

Статус должен показывать `NeoSwitch ... -> gemini-4-neon`. В заголовке сессии Claude Code может по-прежнему отображать запрошенную метку, например `claude-opus-5-5`; вышестоящий маршрут — тот, который показывает `/neoswitch:status`.

`/neoswitch:antigravity-login` — псевдоним `/neoswitch:login`. `/neoswitch:on` и `/neoswitch:off` переключают маршрутизацию без входа.

## Другие способы получения учётных данных

### Ключ API Gemini

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

Application Default Credentials из Google Cloud SDK должны быть доступны процессу.

`neoswitch doctor` находит модель и выполняет генерацию одной строки, а затем сообщает, работают ли учётные данные или почему нет: учётные данные отклонены, нет разрешения, региональное ограничение, неизвестная модель, квота или недоступный сервис.

### OpenAI (экспериментально)

Бэкенд OpenAI отправляет запросы в OpenAI Responses API через официальный SDK `openai-go`. Его нужно выбрать явно: автоматический выбор его никогда не использует, поэтому задайте `NEOSWITCH_BACKEND=openai` или выполните `neoswitch backend openai`. `OPENAI_MODEL` обязателен; NeoSwitch не выбирает модель за вас. `OPENAI_BASE_URL` необязателен и должен включать `/v1`. Бэкенд проверен только на сценарном тестовом сервере, а не на реальной учётной записи OpenAI. Что транслируется, а что нет, описано в [docs/protocol-contract.md](../docs/protocol-contract.md#openai-backend-experimental).

```bash
export NEOSWITCH_BACKEND=openai
export OPENAI_API_KEY='your-key'
export OPENAI_MODEL='model-name'
neoswitch doctor
go run ./cmd/neoswitch claude
```

## Команды плагина

| Команда | Действие |
|---|---|
| `/neoswitch:login` | Аутентифицирует через Antigravity OAuth, включает маршрут Neon для этого проекта и использует ACP |
| `/neoswitch:on` | Включает маршрутизацию для новых сессий Claude Code в этом проекте |
| `/neoswitch:status` | Показывает маршрут проекта и состояние демона |
| `/neoswitch:off` | Отключает маршрутизацию для новых сессий в этом проекте |

После изменения маршрутизации запустите новую сессию Claude Code. Текущая сессия сохраняет существующее подключение.

## Совместимость

Claude Code по-прежнему управляет сессией, разрешениями, выполнением инструментов, MCP-серверами, skills, хуками, `CLAUDE.md` и `/resume`; NeoSwitch заменяет только модель за ними. Насколько хорошо работают эти функции, зависит от того, насколько точно модель следует протоколу инструментов Claude Code. Это проверено в повседневной работе с кодом. Автоматизированный сквозной набор тестов при каждом изменении запускает настоящий CLI Claude Code через NeoSwitch против сервера Gemini, работающего по сценарию, и проверяет, что вызовы инструментов и их результаты, правка файлов, Bash, параллельные вызовы, отказы в разрешениях, хуки, skills, `/resume`, субагенты и MCP-инструменты проходят через маршрут Gemini API без искажений; насколько хорошо ими пользуется настоящая модель, он не измеряет.

| Возможность | Gemini API / Vertex AI | Antigravity OAuth (ACP) |
|---|---|---|
| Потоковая передача текста | Поддерживается | Поддерживается |
| Вызовы инструментов, включая параллельные | Нативный function calling Gemini | Частично: модель пишет JSON вызова инструмента Claude, который приходит целиком, а не потоком |
| Многоходовое использование инструментов и `/resume` | Поддерживается; thought signatures передаются в ID инструментов | Поддерживается |
| Изображения | Поддерживается | Поддерживается, если `agy` предлагает ввод изображений |
| PDF | Поддерживается | Не поддерживается; модели сообщается, что файл опущен |
| `/effort` | Передаётся как уровень размышлений Gemini; не проверено на реальном API, и отбрасывается, если модель его отклоняет | Сопоставляется с уровнем размышлений `agy` |
| Расход токенов и `count_tokens` | Сообщается; `count_tokens` в Developer API оценивается приблизительно | Сообщается, включая около 14k токенов инструкций `agy`, добавляемых к каждому промпту |
| Доступ к Gemini 4 Neon | Зависит от вашего ключа или проекта; проверьте с помощью `neoswitch doctor` | Проверено с `agy`, в котором выполнен вход |

Не поддерживается ни в одном из способов: серверные инструменты, такие как веб-поиск и выполнение кода, источники изображений и документов по URL, а также блоки thinking Claude. `cache_control` и `context_management` игнорируются. NeoSwitch никогда незаметно не переключается на другую модель.

Полный контракт конечной точки описан в [docs/protocol-contract.md](../docs/protocol-contract.md), подробности ACP — в [docs/antigravity.md](../docs/antigravity.md).

## Как работает плагин

`/neoswitch:on` записывает в файл проекта `.claude/settings.local.json` только три ключа: `ANTHROPIC_BASE_URL` и `ANTHROPIC_AUTH_TOKEN` для прокси, а также `CLAUDE_CODE_USE_GATEWAY=0`, потому что в режиме шлюза, который включают лаунчеры шлюзов, Claude Code взял бы базовый URL из окружения и молча проигнорировал прокси. Этот токен авторизует loopback-прокси и не является учётными данными Antigravity.

Хук SessionStart запускает один локальный демон и регистрирует процесс Claude Code до его первого запроса к модели. Демон завершается, когда завершены все зарегистрированные процессы Claude Code.

При маршруте OAuth демон запускает официальный подпроцесс `agy acp`. NeoSwitch отправляет преобразованный запрос через сессию ACP и преобразует полученный ответ или JSON вызова инструмента обратно в формат локальной Anthropic-совместимой конечной точки. Один процесс `agy` обслуживает все сессии, а запросы выполняются параллельно в отдельных сессиях ACP. Собственные инструменты `agy` для работы с файлами, оболочкой и вебом отклоняются в каждом запросе, поэтому инструменты выполняются только в Claude Code и по его правилам разрешений.

`/neoswitch:login` сохраняет Antigravity OAuth как бэкенд, поэтому ключ Gemini, экспортированный для других инструментов, его не заменяет. Если выбор не сохранён, NeoSwitch использует учётные данные Gemini из окружения, а при их отсутствии — Antigravity OAuth. `NEOSWITCH_BACKEND=antigravity|gemini|openai|auto` переопределяет оба варианта, а `/neoswitch:status` показывает используемый бэкенд и причину.

## Автономный CLI

Каждая версия с тегом публикует на странице [Releases](https://github.com/yldm-ai/neon-switch/releases) бинарные файлы для macOS (Apple Silicon и Intel), Linux (amd64 и arm64) и Windows (amd64) с контрольными суммами SHA-256. Сборка для Windows компилируется, но пока не протестирована. Также можно собрать из исходного кода:

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

Основные команды:

```bash
neoswitch doctor              # проверить Claude CLI и могут ли учётные данные использовать модель
neoswitch antigravity login   # вход ACP OAuth через официальный клиент agy
neoswitch antigravity probe   # проверить сохранённую сессию agy, не выводя учётные данные
neoswitch models              # перечислить модели, доступные с этими учётными данными, отметив настроенную
neoswitch on [project]        # включить маршрутизацию для проекта
neoswitch off [project]       # отключить маршрутизацию для проекта
neoswitch status [project]    # показать маршрутизацию и состояние демона
neoswitch version             # вывести версию NeoSwitch
```

Чтобы запустить локальный прокси для другого Anthropic-совместимого клиента:

```bash
export NEOSWITCH_AUTH_TOKEN="$(openssl rand -hex 24)"
GEMINI_API_KEY='your-key' neoswitch serve
```

Задайте в клиенте обе переменные: `ANTHROPIC_BASE_URL` и `ANTHROPIC_AUTH_TOKEN`. `neoswitch claude` настраивает их автоматически, используя временный порт и токен.

## Обновление и удаление

### Обновление

Плагин обновляется командами из раздела [Установка плагина](#установка-плагина). Для автономного CLI скачайте более новый бинарный файл со страницы [Releases](https://github.com/yldm-ai/neon-switch/releases) или установите последнюю версию через Go:

```bash
go install github.com/yldm-ai/neon-switch/cmd/neoswitch@latest
```

Репозиторий переехал из `yldm-ai/NeoSwitch` в `yldm-ai/neon-switch`. Если вы добавляли маркетплейс под старым именем, добавьте его заново из нового расположения:

```bash
claude plugin marketplace add yldm-ai/neon-switch
```

### Удаление

1. Перед удалением выполните `/neoswitch:off` (или `neoswitch off <project>`) в каждом проекте, где включена маршрутизация. Удаление плагина не меняет файлы проектов: без этого шага объект `env` в файле проекта `.claude/settings.local.json` сохранит `ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN` и `CLAUDE_CODE_USE_GATEWAY`, и новые сессии будут пытаться подключиться к прокси, который больше не запускается. Если плагин уже удалён, удалите эти три записи вручную.
2. Закройте сессии Claude Code, которые используют NeoSwitch, затем удалите плагин и его маркетплейс из терминала:

```bash
claude plugin uninstall neoswitch@neoswitch
claude plugin marketplace remove neoswitch
```

3. Удалите файлы, которые NeoSwitch хранит вне ваших проектов:

| Что | Где |
|---|---|
| Бинарный файл плагина, собранный при первом запуске | `$XDG_CACHE_HOME/neoswitch/` или `~/.cache/neoswitch/`, если `XDG_CACHE_HOME` не задана |
| Каталог состояния: `proxy.json` (адрес прокси, токен, сохранённый бэкенд) и `daemon.log` | `NEOSWITCH_STATE_DIR`, если задана; иначе `~/Library/Application Support/neoswitch/` в macOS и `$XDG_CONFIG_HOME/neoswitch/` или `~/.config/neoswitch/` в Linux |

Вход в Antigravity хранит `agy`, а не NeoSwitch, и он сохраняется. Чтобы удалить автономный CLI, установленный через `go install`, удалите `neoswitch` из `$(go env GOBIN)` или, если `GOBIN` не задана, из `$(go env GOPATH)/bin`.

## Устранение неполадок

| Симптом | Решение |
|---|---|
| Непонятно, работают ли учётные данные | Выполните `neoswitch doctor`; он назовёт шаг, на котором произошёл сбой, и причину. |
| Команда `/neoswitch:login` неизвестна | Обновите плагин, перезапустите Claude Code и выполните `/reload-plugins`. |
| `agy: command not found` | Установите официальный Antigravity CLI или задайте `ANTIGRAVITY_CLI_PATH`. |
| `no authorization code was entered` | Обновите плагин до последней версии. Текущий процесс использует ACP `authenticate` и не читает stdin оболочки. |
| `ECONNREFUSED` в новой сессии | Выполните `/neoswitch:status`. Если маршрут включён, проверьте сообщение демона и перезапустите сессию. Выполните `/neoswitch:off` из сессии, использующей Anthropic, и снова включите маршрут после исправления учётных данных. |
| Сессия сообщает, что бэкенд не настроен | Выполните `/neoswitch:login` или экспортируйте `GEMINI_API_KEY` либо настройки Vertex AI перед запуском Claude Code. |
| Сборка или загрузка плагина завершается ошибкой | Установите Go и перезапустите Claude Code; с Go плагин собирается из исходного кода и ничего не скачивает. Без Go проверьте, что GitHub Releases доступен. Несовпадение контрольной суммы означает, что загрузка отклонена и ничего не было запущено. |

## Границы совместимости

NeoSwitch использует официальный Gemini SDK или немодифицированный локальный процесс `agy acp`. Он не извлекает токены OAuth, не обращается к закрытым HTTP API Antigravity, не передаёт учётные данные на удалённый прокси и не изменяет файл hosts.

Anthropic не документирует маршрутизацию Claude Code на модели, отличные от Claude, поэтому обновления Claude Code могут изменить поведение, на которое опирается NeoSwitch.

## Планы

- Больше провайдеров моделей. OpenAI доступен как экспериментальный бэкенд, и его ещё нужно проверить на реальном API. Провайдеров, которые уже предоставляют Anthropic-совместимую конечную точку, можно использовать, напрямую задав `ANTHROPIC_BASE_URL`; NeoSwitch для них не нужен.

## Разработка

```bash
go test ./...
go test -race ./...
go vet ./...
```

Чтобы выпустить релиз, обновите `VERSION` и отправьте соответствующий тег (`v`, за которым следует его содержимое, например `v0.1.0`); workflow релиза отклоняет несовпадающий тег.

Порядок участия описан в [CONTRIBUTING.md](../CONTRIBUTING.md), о том, как сообщить об уязвимости конфиденциально, — в [SECURITY.md](../SECURITY.md), правила поведения — в [CODE_OF_CONDUCT.md](../CODE_OF_CONDUCT.md).

MIT License
