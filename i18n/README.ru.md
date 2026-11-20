# NeoSwitch

[English](../README.md) | [简体中文](README.zh-CN.md) | [繁體中文](README.zh-TW.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md) | [Português (Brasil)](README.pt-BR.md) | **Русский**

[![CI](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/NeoSwitch.svg)](https://pkg.go.dev/github.com/yldm-ai/NeoSwitch) [![License](https://img.shields.io/github/license/yldm-ai/NeoSwitch)](https://github.com/yldm-ai/NeoSwitch/blob/main/LICENSE)

**Keep Claude Code. Switch your models.**

NeoSwitch позволяет использовать Gemini в Claude Code, не меняя привычный рабочий процесс. Claude Code сохраняет свой CLI, команды, разрешения, инструменты, skills, MCP-серверы и хуки; NeoSwitch запускает небольшой локальный прокси, который преобразует запросы для стоящей за ним модели.

Сейчас NeoSwitch направляет Claude Code в Gemini 4 Neon. Другие провайдеры запланированы, но пока не поддерживаются.

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

Чтобы вместо этого использовать ключ API Gemini, задайте его в `/plugin` → `neoswitch` → **Gemini API key** (Claude Code хранит его в защищённой конфигурации плагинов) и выполните `/neoswitch:on`.

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

## Команды плагина

| Команда | Действие |
|---|---|
| `/neoswitch:login` | Аутентифицирует через Antigravity OAuth, включает маршрут Neon для этого проекта и использует ACP |
| `/neoswitch:on` | Включает маршрутизацию для новых сессий Claude Code в этом проекте |
| `/neoswitch:status` | Показывает маршрут проекта и состояние демона |
| `/neoswitch:off` | Отключает маршрутизацию для новых сессий в этом проекте |

После изменения маршрутизации запустите новую сессию Claude Code. Текущая сессия сохраняет существующее подключение.

## Совместимость

Claude Code по-прежнему управляет сессией, разрешениями, выполнением инструментов, MCP-серверами, skills, хуками, `CLAUDE.md` и `/resume`; NeoSwitch заменяет только модель за ними. Насколько хорошо работают эти функции, зависит от того, насколько точно модель следует протоколу инструментов Claude Code. Это проверено в повседневной работе с кодом, но не покрыто автоматизированным сквозным набором тестов.

| Возможность | Gemini API / Vertex AI | Antigravity OAuth (ACP) |
|---|---|---|
| Потоковая передача текста | Поддерживается | Поддерживается |
| Вызовы инструментов, включая параллельные | Нативный function calling Gemini | Частично: модель пишет JSON вызова инструмента Claude, который приходит целиком, а не потоком |
| Многоходовое использование инструментов и `/resume` | Поддерживается; thought signatures передаются в ID инструментов | Поддерживается |
| Изображения | Поддерживается | Поддерживается, если `agy` предлагает ввод изображений |
| PDF | Поддерживается | Не поддерживается; модели сообщается, что файл опущен |
| `/effort` | Пока не применяется | Сопоставляется с уровнем размышлений `agy` |
| Расход токенов и `count_tokens` | Сообщается; `count_tokens` в Developer API оценивается приблизительно | Сообщается, включая около 14k токенов инструкций `agy`, добавляемых к каждому промпту |
| Доступ к Gemini 4 Neon | Зависит от вашего ключа или проекта; проверьте с помощью `neoswitch doctor` | Проверено с `agy`, в котором выполнен вход |

Не поддерживается ни в одном из способов: серверные инструменты, такие как веб-поиск и выполнение кода, источники изображений и документов по URL, а также блоки thinking Claude. `cache_control` и `context_management` игнорируются. NeoSwitch никогда незаметно не переключается на другую модель.

Полный контракт конечной точки описан в [docs/protocol-contract.md](../docs/protocol-contract.md), подробности ACP — в [docs/antigravity.md](../docs/antigravity.md).

## Как работает плагин

`/neoswitch:on` записывает в файл проекта `.claude/settings.local.json` только `ANTHROPIC_BASE_URL` и `ANTHROPIC_AUTH_TOKEN`. Этот токен авторизует loopback-прокси и не является учётными данными Antigravity.

Хук SessionStart запускает один локальный демон и регистрирует процесс Claude Code до его первого запроса к модели. Демон завершается, когда завершены все зарегистрированные процессы Claude Code.

При маршруте OAuth демон запускает официальный подпроцесс `agy acp`. NeoSwitch отправляет преобразованный запрос через сессию ACP и преобразует полученный ответ или JSON вызова инструмента обратно в формат локальной Anthropic-совместимой конечной точки. Один процесс `agy` обслуживает все сессии, а запросы выполняются параллельно в отдельных сессиях ACP. Собственные инструменты `agy` для работы с файлами, оболочкой и вебом отклоняются в каждом запросе, поэтому инструменты выполняются только в Claude Code и по его правилам разрешений.

Необязательная настройка плагина **Use Antigravity OAuth via ACP** принудительно включает маршрут ACP, если одновременно есть другие учётные данные Gemini. Обычный `/neoswitch:login` автоматически использует ACP, если учётные данные Gemini не настроены.

## Автономный CLI

Каждая версия с тегом публикует на странице [Releases](https://github.com/yldm-ai/NeoSwitch/releases) бинарные файлы для macOS (Apple Silicon и Intel), Linux (amd64 и arm64) и Windows (amd64) с контрольными суммами SHA-256. Сборка для Windows компилируется, но пока не протестирована. Также можно собрать из исходного кода:

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

Основные команды:

```bash
neoswitch doctor              # проверить Claude CLI и могут ли учётные данные использовать модель
neoswitch antigravity login   # вход ACP OAuth через официальный клиент agy
neoswitch antigravity probe   # проверить сохранённую сессию agy, не выводя учётные данные
neoswitch models              # вывести настроенную модель
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

## Устранение неполадок

| Симптом | Решение |
|---|---|
| Непонятно, работают ли учётные данные | Выполните `neoswitch doctor`; он назовёт шаг, на котором произошёл сбой, и причину. |
| Команда `/neoswitch:login` неизвестна | Обновите плагин, перезапустите Claude Code и выполните `/reload-plugins`. |
| `agy: command not found` | Установите официальный Antigravity CLI или задайте `ANTIGRAVITY_CLI_PATH`. |
| `no authorization code was entered` | Обновите плагин до последней версии. Текущий процесс использует ACP `authenticate` и не читает stdin оболочки. |
| `ECONNREFUSED` в новой сессии | Выполните `/neoswitch:status`. Если маршрут включён, проверьте сообщение демона и перезапустите сессию. Выполните `/neoswitch:off` из сессии, использующей Anthropic, и снова включите маршрут после исправления учётных данных. |
| Сессия запускается без учётных данных Gemini | Выполните `/neoswitch:login` или настройте ключ API Gemini либо Vertex AI ADC. |
| Сборка плагина завершается ошибкой | Установите Go и перезапустите Claude Code. |

## Границы совместимости

NeoSwitch использует официальный Gemini SDK или немодифицированный локальный процесс `agy acp`. Он не извлекает токены OAuth, не обращается к закрытым HTTP API Antigravity, не передаёт учётные данные на удалённый прокси и не изменяет файл hosts.

Anthropic не документирует маршрутизацию Claude Code на модели, отличные от Claude, поэтому обновления Claude Code могут изменить поведение, на которое опирается NeoSwitch.

## Планы

- Больше провайдеров моделей, начиная с OpenAI. Провайдеров, которые уже предоставляют Anthropic-совместимую конечную точку, можно использовать, напрямую задав `ANTHROPIC_BASE_URL`; NeoSwitch для них не нужен.
- Готовые бинарные файлы для плагина, чтобы ему больше не требовался Go.

## Разработка

```bash
go test ./...
go test -race ./...
go vet ./...
```

MIT License
