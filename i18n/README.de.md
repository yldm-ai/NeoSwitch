# NeoSwitch

[English](../README.md) | [简体中文](README.zh-CN.md) | [繁體中文](README.zh-TW.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | **Deutsch** | [Português (Brasil)](README.pt-BR.md) | [Русский](README.ru.md)

[![Latest release](https://img.shields.io/github/v/release/yldm-ai/neo-switch)](https://github.com/yldm-ai/neo-switch/releases) [![CI](https://github.com/yldm-ai/neo-switch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/neo-switch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/neo-switch.svg)](https://pkg.go.dev/github.com/yldm-ai/neo-switch) [![Go version](https://img.shields.io/github/go-mod/go-version/yldm-ai/neo-switch)](https://github.com/yldm-ai/neo-switch/blob/main/go.mod) [![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/yldm-ai/neo-switch/blob/main/LICENSE) [![Last commit](https://img.shields.io/github/last-commit/yldm-ai/neo-switch)](https://github.com/yldm-ai/neo-switch/commits/main) [![GitHub stars](https://img.shields.io/github/stars/yldm-ai/neo-switch)](https://github.com/yldm-ai/neo-switch/stargazers)

[![Claude Code plugin](https://img.shields.io/badge/Claude%20Code-plugin-D97757)](https://github.com/yldm-ai/neo-switch/blob/main/.claude-plugin/plugin.json) [![Model: Gemini 4 Neon](https://img.shields.io/badge/model-Gemini%204%20Neon-4285F4)](https://github.com/yldm-ai/neo-switch/blob/main/docs/protocol-contract.md) [![Platform: macOS and Linux](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey)](https://github.com/yldm-ai/neo-switch/releases) [![Proxy: local-only](https://img.shields.io/badge/proxy-local--only-2ea44f)](https://github.com/yldm-ai/neo-switch/blob/main/docs/protocol-contract.md)

**Keep Claude Code. Switch your models.**

Mit NeoSwitch verwenden Sie Gemini in Claude Code, ohne Ihren gewohnten Workflow zu ändern. Claude Code behält seine CLI, Befehle, Berechtigungen, Tools, Skills, MCP-Server und Hooks; NeoSwitch betreibt einen kleinen lokalen Proxy, der die Anfragen für das dahinterliegende Modell übersetzt.

Derzeit leitet NeoSwitch Claude Code an Gemini 4 Neon weiter. OpenAI ist als experimentelles Backend verfügbar: Es muss ausdrücklich gewählt werden und wurde nur gegen einen skriptgesteuerten Upstream getestet.

## Warum NeoSwitch

- **Workflow beibehalten**: Sie arbeiten weiterhin mit Claude Code. Nur das Modell ändert sich.
- **Pro Projekt**: Aktivieren Sie das Routing in einem Projekt und lassen Sie die anderen bei Anthropic.
- **Lokal zuerst**: Der Proxy lauscht nur auf Loopback, verwendet ein zufälliges Zugriffstoken und protokolliert weder Prompts noch Antworten oder Zugangsdaten. Es ist kein entferntes Relay beteiligt.
- **Ehrliche Kompatibilität**: Was funktioniert, was nur teilweise funktioniert und was fehlt, ist [unten](#kompatibilität) aufgeführt.

## Zugangsdaten-Weg wählen

| Weg | Anmeldung | Hinweise |
|---|---|---|
| Antigravity OAuth | Google-Konto über die offizielle `agy`-CLI | Standardablauf des Plugins. Lesen Sie zuerst die Warnung unten. |
| Gemini Developer API | `GEMINI_API_KEY` | Der Zugriff auf Gemini 4 Neon hängt von Ihrem Schlüssel ab. Prüfen Sie ihn mit `neoswitch doctor`. |
| Vertex AI | Google Cloud Application Default Credentials | Der Zugriff hängt von Ihrem Projekt und Ihrer Region ab. Prüfen Sie ihn mit `neoswitch doctor`. |

> [!WARNING]
> Die [Nutzungsbedingungen von Google Antigravity](https://antigravity.google/terms) (Abschnitt 6) werten den Zugriff auf Antigravity über Software, Tools oder Dienste Dritter als Verstoß und erlauben Google, die betroffenen Antigravity- und Gemini-CLI-Konten zu sperren oder zu kündigen. Das Weiterleiten von Claude Code über Antigravity OAuth fällt unter diese Regel. Verwenden Sie diesen Weg nur, wenn Sie dieses Risiko für das angemeldete Konto akzeptieren; andernfalls verwenden Sie einen Gemini-API-Schlüssel oder Vertex AI. `neoswitch doctor` und `neoswitch antigravity login` geben dieselbe Warnung aus.

OAuth-Zugangsdaten bleiben immer in `agy`. NeoSwitch liest, kopiert oder exportiert das Token nicht.

## Claude Code mit Neon verwenden

### Voraussetzungen

- Claude Code.
- Für Antigravity OAuth: die offizielle Antigravity-CLI, als `agy` im PATH verfügbar.
- Go (optional). Mit Go baut das Plugin das mitgelieferte NeoSwitch-Binary bei der ersten Verwendung aus dem Quellcode. Ohne Go lädt es das Release-Binary herunter, das die Datei `VERSION` des Repositorys benennt, für macOS oder Linux (amd64 oder arm64), und prüft vor dem Ausführen seine SHA-256-Prüfsumme. Zwischen zwei Releases kann dieses Binary älter sein als der Quellcode des Plugins.

### Plugin installieren

Führen Sie diese Befehle in Claude Code aus:

```text
/plugin marketplace add yldm-ai/neo-switch
/plugin install neoswitch@neoswitch
```

Ist das Plugin bereits installiert, aktualisieren Sie es nach einem neuen Release in einer Shell:

```bash
claude plugin marketplace update neoswitch
claude plugin update neoswitch@neoswitch
```

Starten Sie Claude Code nach dem Update neu.

### Anmelden und Neon aktivieren

Führen Sie in dem Projekt, in dem Sie Neon verwenden möchten, Folgendes aus:

```text
/neoswitch:login
```

Der Befehl führt folgende Schritte aus:

1. Startet den von `agy` verwalteten ACP-Authentifizierungsablauf.
2. Öffnet den Browser, wenn Google OAuth erforderlich ist.
3. Aktiviert das NeoSwitch-Routing für das aktuelle Projekt.
4. Wählt `gemini-4-neon` für die ACP-Sitzung aus.

Um stattdessen einen Gemini-API-Schlüssel zu verwenden, exportieren Sie `GEMINI_API_KEY` vor dem Start von Claude Code und führen `/neoswitch:on` aus; das Plugin hat keine Einstellungen zum Ausfüllen. Wenn Sie sich zuvor mit `/neoswitch:login` angemeldet haben, exportieren Sie zusätzlich `NEOSWITCH_BACKEND=gemini`, da sonst die beim Login gespeicherte Wahl gilt.

Schließen Sie Claude Code in diesem Projekt und öffnen Sie es erneut. Prüfen Sie die Route mit:

```text
/neoswitch:status
```

Der Status sollte `NeoSwitch ... -> gemini-4-neon` anzeigen. Claude Code kann im Sitzungskopf weiterhin die angeforderte Bezeichnung anzeigen, etwa `claude-opus-5-5`; maßgeblich ist die Upstream-Route, die `/neoswitch:status` anzeigt.

`/neoswitch:antigravity-login` ist ein Alias für `/neoswitch:login`. `/neoswitch:on` und `/neoswitch:off` schalten das Routing ohne Anmeldung um.

## Weitere Zugangsdaten-Wege

### Gemini-API-Schlüssel

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

Die Application Default Credentials des Google Cloud SDK müssen für den Prozess bereits verfügbar sein.

`neoswitch doctor` sucht das Modell, führt eine einzeilige Generierung aus und meldet, ob die Zugangsdaten funktionieren oder warum nicht: abgelehnte Zugangsdaten, fehlende Berechtigung, regionale Einschränkung, unbekanntes Modell, Kontingent oder ein nicht verfügbarer Dienst.

### OpenAI (experimentell)

Das OpenAI-Backend sendet Anfragen über das offizielle `openai-go`-SDK an die OpenAI Responses API. Es muss ausdrücklich gewählt werden, die automatische Auswahl wählt es nie: Setzen Sie `NEOSWITCH_BACKEND=openai` oder führen Sie `neoswitch backend openai` aus. `OPENAI_MODEL` ist Pflicht; NeoSwitch wählt kein Modell für Sie. `OPENAI_BASE_URL` ist optional und muss `/v1` enthalten. Es wurde nur gegen einen skriptgesteuerten Upstream getestet, nicht mit einem echten OpenAI-Konto. Was abgebildet wird und was nicht, steht in [docs/protocol-contract.md](../docs/protocol-contract.md#openai-backend-experimental).

```bash
export NEOSWITCH_BACKEND=openai
export OPENAI_API_KEY='your-key'
export OPENAI_MODEL='model-name'
neoswitch doctor
go run ./cmd/neoswitch claude
```

## Plugin-Befehle

| Befehl | Wirkung |
|---|---|
| `/neoswitch:login` | Authentifiziert mit Antigravity OAuth, aktiviert die Neon-Route für dieses Projekt und verwendet ACP |
| `/neoswitch:on` | Aktiviert das Routing für neue Claude-Code-Sitzungen in diesem Projekt |
| `/neoswitch:status` | Zeigt die Projektroute und den Daemon-Status |
| `/neoswitch:off` | Deaktiviert das Routing für neue Sitzungen in diesem Projekt |

Starten Sie nach einer Routing-Änderung eine neue Claude-Code-Sitzung. Die laufende Sitzung behält ihre bestehende Verbindung.

## Kompatibilität

Claude Code führt weiterhin Sitzung, Berechtigungen, Tool-Ausführung, MCP-Server, Skills, Hooks, `CLAUDE.md` und `/resume` aus; NeoSwitch ersetzt nur das Modell dahinter. Wie gut diese Funktionen arbeiten, hängt davon ab, wie zuverlässig das Modell dem Tool-Protokoll von Claude Code folgt. Das wurde im täglichen Programmiereinsatz erprobt. Eine automatisierte End-to-End-Testsuite lässt bei jeder Änderung die echte Claude-Code-CLI über NeoSwitch gegen einen geskripteten Gemini-Server laufen und prüft, dass Tool-Aufrufe und -Ergebnisse, Dateiänderungen, Bash, parallele Aufrufe, verweigerte Berechtigungen, Hooks, Skills, `/resume`, Subagents und MCP-Tools unverändert über die Gemini-API-Route laufen; wie gut ein echtes Modell sie nutzt, misst sie nicht.

| Funktion | Gemini API / Vertex AI | Antigravity OAuth (ACP) |
|---|---|---|
| Text-Streaming | Unterstützt | Unterstützt |
| Tool-Aufrufe, auch parallele | Natives Gemini Function Calling | Teilweise: Das Modell schreibt Claude-Tool-Call-JSON, das vollständig statt gestreamt ankommt |
| Mehrstufige Tool-Nutzung und `/resume` | Unterstützt; Thought Signatures werden in den Tool-IDs mitgeführt | Unterstützt |
| Bilder | Unterstützt | Unterstützt, wenn `agy` Bildeingabe anbietet |
| PDFs | Unterstützt | Nicht unterstützt; dem Modell wird mitgeteilt, dass eine Datei ausgelassen wurde |
| `/effort` | Als Thinking-Stufe von Gemini gesendet; nicht gegen die Live-API geprüft, und verworfen, wenn das Modell sie ablehnt | Auf die Thought-Stufe von `agy` abgebildet |
| Token-Verbrauch und `count_tokens` | Gemeldet; `count_tokens` wird bei der Developer API geschätzt | Gemeldet, einschließlich etwa 14k Tokens an `agy`-Anweisungen, die jedem Prompt hinzugefügt werden |
| Zugriff auf Gemini 4 Neon | Hängt von Ihrem Schlüssel oder Projekt ab; prüfen mit `neoswitch doctor` | Mit angemeldetem `agy` verifiziert |

Auf keinem der beiden Wege unterstützt: Server-Tools wie Websuche und Codeausführung, Bild- und Dokumentquellen per URL sowie Claude-Thinking-Blöcke. `cache_control` und `context_management` werden ignoriert. NeoSwitch weicht nie stillschweigend auf ein anderes Modell aus.

Den vollständigen Endpunkt-Vertrag finden Sie in [docs/protocol-contract.md](../docs/protocol-contract.md), ACP-Details in [docs/antigravity.md](../docs/antigravity.md).

## Funktionsweise des Plugins

`/neoswitch:on` schreibt nur drei Schlüssel in die Datei `.claude/settings.local.json` des Projekts: `ANTHROPIC_BASE_URL` und `ANTHROPIC_AUTH_TOKEN` für den Proxy sowie `CLAUDE_CODE_USE_GATEWAY=0`, weil Claude Code im Gateway-Modus, den Gateway-Launcher einschalten, die Basis-URL aus seiner Umgebung übernehmen und den Proxy stillschweigend ignorieren würde. Das Token autorisiert den Loopback-Proxy und ist keine Antigravity-Zugangsberechtigung.

Der SessionStart-Hook startet einen lokalen Daemon und registriert den Claude-Code-Prozess vor dessen erster Modellanfrage. Der Daemon beendet sich, sobald alle registrierten Claude-Code-Prozesse beendet sind.

Bei der OAuth-Route startet der Daemon den offiziellen Unterprozess `agy acp`. NeoSwitch sendet die übersetzte Anfrage über eine ACP-Sitzung und übersetzt die zurückgegebene Antwort bzw. das Tool-Call-JSON zurück an den lokalen Anthropic-kompatiblen Endpunkt. Ein einziger `agy`-Prozess bedient alle Sitzungen, und Anfragen laufen parallel in separaten ACP-Sitzungen. Die eigenen Datei-, Shell- und Web-Tools von `agy` werden bei jeder Anfrage abgelehnt, sodass Tools nur in Claude Code und unter dessen Berechtigungsregeln ausgeführt werden.

`/neoswitch:login` speichert Antigravity OAuth als Backend, sodass ein für andere Tools exportierter Gemini-Schlüssel es nicht ersetzt. Ohne gespeicherte Wahl verwendet NeoSwitch vorhandene Gemini-Zugangsdaten aus der Umgebung und sonst Antigravity OAuth. `NEOSWITCH_BACKEND=antigravity|gemini|openai|auto` überschreibt beides, und `/neoswitch:status` zeigt das verwendete Backend und den Grund.

## Eigenständige CLI

Jede getaggte Version veröffentlicht auf der Seite [Releases](https://github.com/yldm-ai/neo-switch/releases) Binaries für macOS (Apple Silicon und Intel), Linux (amd64 und arm64) und Windows (amd64) mit SHA-256-Prüfsummen. Der Windows-Build wird kompiliert, ist aber noch nicht getestet. Sie können auch aus dem Quellcode bauen:

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

Häufige Befehle:

```bash
neoswitch doctor              # Claude-CLI prüfen und ob die Zugangsdaten das Modell nutzen können
neoswitch antigravity login   # ACP-OAuth-Anmeldung über den offiziellen agy-Client
neoswitch antigravity probe   # gespeicherte agy-Sitzung prüfen, ohne Zugangsdaten auszugeben
neoswitch models              # mit den Zugangsdaten nutzbare Modelle auflisten, das konfigurierte markiert
neoswitch on [project]        # Routing für ein Projekt aktivieren
neoswitch off [project]       # Routing für ein Projekt deaktivieren
neoswitch status [project]    # Routing und Daemon-Status anzeigen
neoswitch version             # NeoSwitch-Version ausgeben
```

So starten Sie den lokalen Proxy für einen anderen Anthropic-kompatiblen Client:

```bash
export NEOSWITCH_AUTH_TOKEN="$(openssl rand -hex 24)"
GEMINI_API_KEY='your-key' neoswitch serve
```

Setzen Sie im Client sowohl `ANTHROPIC_BASE_URL` als auch `ANTHROPIC_AUTH_TOKEN`. `neoswitch claude` setzt diese Variablen automatisch mit einem temporären Port und Token.

## Aktualisieren und deinstallieren

### Aktualisieren

Aktualisieren Sie das Plugin mit den Befehlen aus [Plugin installieren](#plugin-installieren). Für die eigenständige CLI laden Sie ein neueres Binary von der Seite [Releases](https://github.com/yldm-ai/neo-switch/releases) herunter oder installieren die neueste Version mit Go:

```bash
go install github.com/yldm-ai/neo-switch/cmd/neoswitch@latest
```

Das Repository ist von `yldm-ai/NeoSwitch` / `yldm-ai/neon-switch` nach `yldm-ai/neo-switch` umgezogen. Wenn Sie den Marketplace unter dem alten Namen hinzugefügt haben, fügen Sie ihn vom neuen Ort erneut hinzu:

```bash
claude plugin marketplace add yldm-ai/neo-switch
```

### Deinstallieren

1. Führen Sie vor der Deinstallation in jedem Projekt mit aktiviertem Routing `/neoswitch:off` (oder `neoswitch off <project>`) aus. Das Deinstallieren des Plugins ändert keine Projektdateien: Ohne diesen Schritt behält das `env`-Objekt in der `.claude/settings.local.json` des Projekts `ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN` und `CLAUDE_CODE_USE_GATEWAY`, und neue Sitzungen versuchen, einen Proxy zu erreichen, der nicht mehr startet. Ist das Plugin bereits entfernt, löschen Sie diese drei Einträge von Hand.
2. Schließen Sie die Claude-Code-Sitzungen, die NeoSwitch verwenden, und entfernen Sie dann in einer Shell das Plugin und seinen Marketplace:

```bash
claude plugin uninstall neoswitch@neoswitch
claude plugin marketplace remove neoswitch
```

3. Löschen Sie die Dateien, die NeoSwitch außerhalb Ihrer Projekte ablegt:

| Was | Ort |
|---|---|
| Beim ersten Einsatz gebautes Plugin-Binary | `$XDG_CACHE_HOME/neoswitch/`, oder `~/.cache/neoswitch/`, wenn `XDG_CACHE_HOME` nicht gesetzt ist |
| Zustandsverzeichnis: `proxy.json` (Proxy-Adresse, Token, gespeichertes Backend) und `daemon.log` | `NEOSWITCH_STATE_DIR`, wenn gesetzt; sonst `~/Library/Application Support/neoswitch/` unter macOS und `$XDG_CONFIG_HOME/neoswitch/` oder `~/.config/neoswitch/` unter Linux |

Die Antigravity-Anmeldung speichert `agy`, nicht NeoSwitch; sie bleibt erhalten. Um eine mit `go install` installierte eigenständige CLI zu entfernen, löschen Sie `neoswitch` aus `$(go env GOBIN)` oder, wenn `GOBIN` nicht gesetzt ist, aus `$(go env GOPATH)/bin`.

## Fehlerbehebung

| Symptom | Lösung |
|---|---|
| Unklar, ob die Zugangsdaten funktionieren | `neoswitch doctor` ausführen; es nennt den fehlgeschlagenen Schritt und die Ursache. |
| `/neoswitch:login` ist unbekannt | Plugin aktualisieren, Claude Code neu starten und `/reload-plugins` ausführen. |
| `agy: command not found` | Die offizielle Antigravity-CLI installieren oder `ANTIGRAVITY_CLI_PATH` setzen. |
| `no authorization code was entered` | Auf die neueste Plugin-Version aktualisieren. Der aktuelle Ablauf nutzt ACP `authenticate` und liest nicht von der Shell-stdin. |
| `ECONNREFUSED` in einer neuen Sitzung | `/neoswitch:status` ausführen. Ist die Route aktiv, die Daemon-Meldung prüfen und die Sitzung neu starten. `/neoswitch:off` aus einer Sitzung ausführen, die Anthropic verwendet, und nach Korrektur der Zugangsdaten wieder aktivieren. |
| Die Sitzung meldet, dass kein Backend konfiguriert ist | Führen Sie `/neoswitch:login` aus oder exportieren Sie `GEMINI_API_KEY` bzw. die Vertex-AI-Einstellungen vor dem Start von Claude Code. |
| Der Plugin-Build oder -Download schlägt fehl | Go installieren und Claude Code neu starten; mit Go baut das Plugin aus dem Quellcode und lädt nichts herunter. Ohne Go sicherstellen, dass GitHub Releases erreichbar ist. Eine abweichende Prüfsumme bedeutet, dass der Download abgelehnt und nichts ausgeführt wurde. |

## Kompatibilitätsgrenzen

NeoSwitch verwendet das offizielle Gemini-SDK oder den unveränderten lokalen Prozess `agy acp`. Es extrahiert keine OAuth-Tokens, ruft keine privaten HTTP-APIs von Antigravity auf, leitet keine Zugangsdaten an einen entfernten Proxy weiter und ändert keine hosts-Datei.

Anthropic dokumentiert das Weiterleiten von Claude Code an Nicht-Claude-Modelle nicht, daher können Claude-Code-Updates Verhalten ändern, auf das NeoSwitch angewiesen ist.

## Roadmap

- Weitere Modellanbieter. OpenAI ist als experimentelles Backend verfügbar und muss noch gegen die echte API geprüft werden. Anbieter, die bereits einen Anthropic-kompatiblen Endpunkt bereitstellen, lassen sich direkt über `ANTHROPIC_BASE_URL` nutzen und benötigen NeoSwitch nicht.

## Entwicklung

```bash
go test ./...
go test -race ./...
go vet ./...
```

Für ein Release `VERSION` erhöhen und den passenden Tag pushen (`v` gefolgt vom Inhalt, etwa `v0.1.0`); der Release-Workflow lehnt einen Tag ab, der nicht passt.

Siehe [CONTRIBUTING.md](../CONTRIBUTING.md) für den Beitragsablauf, [SECURITY.md](../SECURITY.md) für vertrauliche Meldungen von Sicherheitslücken und [CODE_OF_CONDUCT.md](../CODE_OF_CONDUCT.md) für den Verhaltenskodex.

MIT License
