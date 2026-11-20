# NeoSwitch

[English](../README.md) | [简体中文](README.zh-CN.md) | [繁體中文](README.zh-TW.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | **Deutsch** | [Português (Brasil)](README.pt-BR.md) | [Русский](README.ru.md)

[![CI](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/NeoSwitch.svg)](https://pkg.go.dev/github.com/yldm-ai/NeoSwitch) [![License](https://img.shields.io/github/license/yldm-ai/NeoSwitch)](https://github.com/yldm-ai/NeoSwitch/blob/main/LICENSE)

NeoSwitch ist ein Claude-Code-Plugin, das Projekte über einen lokalen Anthropic-kompatiblen Proxy an Gemini 4 Neon weiterleitet. Sie können zwischen Antigravity OAuth, der Gemini Developer API und Vertex AI wählen und behalten dabei den Workflow, das Streaming und die Tool-Nutzung von Claude Code.

Es werden zwei Wege für Upstream-Zugangsdaten unterstützt:

- Antigravity OAuth über den offiziellen lokalen Prozess `agy acp`.
- Zugangsdaten für die Gemini Developer API oder Vertex AI über das offizielle SDK `google.golang.org/genai`.

Für Claude Code wird der Plugin-Ablauf mit Antigravity OAuth empfohlen. Die OAuth-Zugangsdaten bleiben in `agy`; NeoSwitch liest oder exportiert das Token nicht.

## Claude Code mit Neon verwenden

### Voraussetzungen

- Claude Code.
- Die offizielle Antigravity-CLI, als `agy` im PATH verfügbar.
- Go. Das Plugin baut das mitgelieferte NeoSwitch-Binary bei der ersten Verwendung.

### Plugin installieren

Führen Sie diese Befehle in Claude Code aus:

```text
/plugin marketplace add yldm-ai/NeoSwitch
/plugin install neoswitch@neoswitch
```

Ist das Plugin bereits installiert, aktualisieren Sie es nach einem neuen Release in einer Shell:

```bash
claude plugin marketplace update neoswitch
claude plugin update neoswitch@neoswitch
```

Starten Sie Claude Code nach dem Update neu. Der ACP-Anmeldeablauf erfordert die aktuelle Plugin-Version.

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

Schließen Sie Claude Code in diesem Projekt nach Abschluss des Befehls und öffnen Sie es erneut. Prüfen Sie die Route mit:

```text
/neoswitch:status
```

Der Status sollte `NeoSwitch ... -> gemini-4-neon` anzeigen. Claude Code kann im Sitzungskopf weiterhin die angeforderte Bezeichnung anzeigen, etwa `claude-opus-5-5`. Die tatsächliche Upstream-Route zeigt `/neoswitch:status`.

`/neoswitch:antigravity-login` ist ein Alias für `/neoswitch:login`. Mit `/neoswitch:on` und `/neoswitch:off` können Sie das Routing weiterhin ohne Anmeldung umschalten.

## Weitere Zugangsdaten-Wege

### Gemini-API-Schlüssel

```bash
export GEMINI_API_KEY='your-key'
go run ./cmd/neoswitch claude
```

Das Plugin kann den Schlüssel auch über `/plugin` → `neoswitch` → **Gemini API key** erhalten. Claude Code speichert ihn in seiner sicheren Plugin-Konfiguration.

### Vertex AI

```bash
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT=my-project
export GOOGLE_CLOUD_LOCATION=global
go run ./cmd/neoswitch claude
```

Die Application Default Credentials des Google Cloud SDK müssen für den Prozess bereits verfügbar sein.

## Plugin-Befehle

| Befehl | Wirkung |
|---|---|
| `/neoswitch:login` | Authentifiziert mit Antigravity OAuth, aktiviert die Neon-Route für dieses Projekt und verwendet ACP |
| `/neoswitch:on` | Aktiviert das Routing für neue Claude-Code-Sitzungen in diesem Projekt |
| `/neoswitch:status` | Zeigt die Projektroute und den Daemon-Status |
| `/neoswitch:off` | Deaktiviert das Routing für neue Sitzungen in diesem Projekt |

Starten Sie nach einer Routing-Änderung eine neue Claude-Code-Sitzung. Die laufende Sitzung behält ihre bestehende Verbindung.

## Funktionsweise des Plugins

`/neoswitch:on` schreibt nur `ANTHROPIC_BASE_URL` und `ANTHROPIC_AUTH_TOKEN` in die Datei `.claude/settings.local.json` des Projekts. Das Token autorisiert den Loopback-Proxy und ist keine Antigravity-Zugangsberechtigung.

Der SessionStart-Hook startet einen lokalen Daemon und registriert den Claude-Code-Prozess vor dessen erster Modellanfrage. Der Daemon beendet sich, sobald alle registrierten Claude-Code-Prozesse beendet sind. Standardmäßig lauscht der Proxy nur auf Loopback.

Bei der OAuth-Route startet der Daemon den offiziellen Unterprozess `agy acp`. NeoSwitch sendet die übersetzte Anfrage über eine ACP-Sitzung und übersetzt die zurückgegebene Antwort bzw. das Claude-kompatible Tool-Call-JSON zurück an den lokalen Anthropic-kompatiblen Endpunkt. OAuth-Zugangsdaten und Browser-Callbacks bleiben in der Verantwortung von `agy`.

Ein einziger `agy`-Prozess bedient alle Sitzungen, und Anfragen laufen parallel in separaten ACP-Sitzungen. Text wird während der Generierung gestreamt, der Token-Verbrauch wird an Claude Code gemeldet, und die Claude-Code-Einstellung `/effort` legt fest, wie lange Neon nachdenkt.

Die optionale Plugin-Einstellung **Use Antigravity OAuth via ACP** erzwingt die ACP-Route, wenn zusätzlich andere Gemini-Zugangsdaten vorhanden sind. Ein normales `/neoswitch:login` verwendet automatisch ACP, wenn keine Gemini-Zugangsdaten konfiguriert sind.

## Eigenständige CLI

Bauen Sie das Binary oder führen Sie es direkt aus dem Repository aus:

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

Häufige Befehle:

```bash
neoswitch antigravity login   # ACP-OAuth-Anmeldung über den offiziellen agy-Client
neoswitch antigravity probe   # gespeicherte agy-Sitzung prüfen, ohne Zugangsdaten auszugeben
neoswitch doctor              # gewähltes Backend und Claude-CLI prüfen
neoswitch models              # konfiguriertes Modell ausgeben
neoswitch on [project]        # Routing für ein Projekt aktivieren
neoswitch off [project]       # Routing für ein Projekt deaktivieren
neoswitch status [project]    # Routing und Daemon-Status anzeigen
```

So starten Sie den lokalen Proxy für einen anderen Anthropic-kompatiblen Client:

```bash
export NEOSWITCH_AUTH_TOKEN="$(openssl rand -hex 24)"
GEMINI_API_KEY='your-key' neoswitch serve
```

Setzen Sie im Client sowohl `ANTHROPIC_BASE_URL` als auch `ANTHROPIC_AUTH_TOKEN`. `neoswitch claude` setzt diese Variablen automatisch mit einem temporären Port und Token.

## Fehlerbehebung

| Symptom | Lösung |
|---|---|
| `/neoswitch:login` ist unbekannt | Plugin aktualisieren, Claude Code neu starten und `/reload-plugins` ausführen. |
| `agy: command not found` | Die offizielle Antigravity-CLI installieren oder `ANTIGRAVITY_CLI_PATH` setzen. |
| `no authorization code was entered` | Auf die neueste Plugin-Version aktualisieren. Der aktuelle Ablauf nutzt ACP `authenticate` und liest nicht von der Shell-stdin. |
| `ECONNREFUSED` in einer neuen Sitzung | `/neoswitch:status` ausführen. Ist die Route aktiv, die Daemon-Meldung prüfen und die Sitzung neu starten. `/neoswitch:off` aus einer Sitzung ausführen, die Anthropic verwendet, und nach Korrektur der Zugangsdaten wieder aktivieren. |
| Die Sitzung startet ohne Gemini-Zugangsdaten | `/neoswitch:login` ausführen oder einen Gemini-API-Schlüssel bzw. Vertex AI ADC konfigurieren. |
| Der Plugin-Build schlägt fehl | Go installieren und Claude Code neu starten. |

## Kompatibilitätsgrenzen

NeoSwitch verwendet das offizielle Gemini-SDK oder den unveränderten lokalen Prozess `agy acp`. Es extrahiert keine OAuth-Tokens, ruft keine privaten HTTP-APIs von Antigravity auf, leitet keine Zugangsdaten an einen entfernten Proxy weiter, ändert keine hosts-Datei und weicht nie stillschweigend auf ein anderes Modell aus.

Claude Code bleibt für Sitzung, Berechtigungen, MCP und die Tool-Schleife verantwortlich. Anthropic dokumentiert das Weiterleiten von Claude Code an Nicht-Claude-Modelle nicht, daher können Claude-Code-Updates Verhalten ändern, auf das NeoSwitch angewiesen ist.

Den Endpunkt-Vertrag und die Kompatibilitätsmatrix finden Sie in [docs/protocol-contract.md](../docs/protocol-contract.md), Details zur ACP-Authentifizierung in [docs/antigravity.md](../docs/antigravity.md).

## Entwicklung

```bash
go test ./...
go test -race ./...
go vet ./...
```

MIT License
