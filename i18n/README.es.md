# NeoSwitch

[English](../README.md) | [简体中文](README.zh-CN.md) | [繁體中文](README.zh-TW.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | **Español** | [Français](README.fr.md) | [Deutsch](README.de.md) | [Português (Brasil)](README.pt-BR.md) | [Русский](README.ru.md)

[![CI](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/NeoSwitch.svg)](https://pkg.go.dev/github.com/yldm-ai/NeoSwitch) [![License](https://img.shields.io/github/license/yldm-ai/NeoSwitch)](https://github.com/yldm-ai/NeoSwitch/blob/main/LICENSE)

NeoSwitch es un plugin de Claude Code que enruta proyectos a Gemini 4 Neon a través de un proxy local compatible con Anthropic. Puedes elegir Antigravity OAuth, la Gemini Developer API o Vertex AI y conservar el flujo de trabajo, el streaming y el uso de herramientas de Claude Code.

Admite dos vías de credenciales hacia el servicio de origen:

- Antigravity OAuth mediante el proceso local oficial `agy acp`.
- Credenciales de Gemini Developer API o Vertex AI mediante el SDK oficial `google.golang.org/genai`.

Para Claude Code se recomienda el flujo del plugin con Antigravity OAuth. Las credenciales OAuth permanecen dentro de `agy`; NeoSwitch no lee ni exporta el token.

## Usar Claude Code con Neon

### Requisitos previos

- Claude Code.
- La CLI oficial de Antigravity, disponible como `agy` en tu PATH.
- Go. El plugin compila el binario de NeoSwitch incluido la primera vez que se usa.

### Instalar el plugin

Ejecuta estos comandos dentro de Claude Code:

```text
/plugin marketplace add yldm-ai/NeoSwitch
/plugin install neoswitch@neoswitch
```

Si el plugin ya estaba instalado, actualízalo desde una terminal cuando salga una nueva versión:

```bash
claude plugin marketplace update neoswitch
claude plugin update neoswitch@neoswitch
```

Reinicia Claude Code después de actualizar. El flujo de inicio de sesión ACP requiere la versión actual del plugin.

### Iniciar sesión y activar Neon

En el proyecto donde quieras usar Neon, ejecuta:

```text
/neoswitch:login
```

El comando realiza los siguientes pasos:

1. Inicia el flujo de autenticación ACP gestionado por `agy`.
2. Abre el navegador cuando se requiere Google OAuth.
3. Activa el enrutamiento de NeoSwitch para el proyecto actual.
4. Selecciona `gemini-4-neon` para la sesión ACP.

Cuando termine el comando, cierra y vuelve a abrir Claude Code en ese proyecto. Comprueba la ruta con:

```text
/neoswitch:status
```

El estado debería mostrar `NeoSwitch ... -> gemini-4-neon`. Claude Code puede seguir mostrando la etiqueta solicitada, como `claude-opus-5-5`, en la cabecera de la sesión. La ruta real hacia el origen se muestra con `/neoswitch:status`.

`/neoswitch:antigravity-login` es un alias de `/neoswitch:login`. `/neoswitch:on` y `/neoswitch:off` siguen disponibles si quieres cambiar el enrutamiento sin iniciar sesión.

## Otras vías de credenciales

### Clave de API de Gemini

```bash
export GEMINI_API_KEY='your-key'
go run ./cmd/neoswitch claude
```

El plugin también puede recibir la clave desde `/plugin` → `neoswitch` → **Gemini API key**. Claude Code guarda la clave en su configuración segura de plugins.

### Vertex AI

```bash
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT=my-project
export GOOGLE_CLOUD_LOCATION=global
go run ./cmd/neoswitch claude
```

Las Application Default Credentials del Google Cloud SDK deben estar disponibles para el proceso.

## Comandos del plugin

| Comando | Efecto |
|---|---|
| `/neoswitch:login` | Autentica con Antigravity OAuth, activa la ruta de Neon para este proyecto y usa ACP |
| `/neoswitch:on` | Activa el enrutamiento para las nuevas sesiones de Claude Code en este proyecto |
| `/neoswitch:status` | Muestra la ruta del proyecto y el estado del daemon |
| `/neoswitch:off` | Desactiva el enrutamiento para las nuevas sesiones en este proyecto |

Después de cambiar el enrutamiento, inicia una nueva sesión de Claude Code. La sesión actual conserva su conexión.

## Cómo funciona el plugin

`/neoswitch:on` solo escribe `ANTHROPIC_BASE_URL` y `ANTHROPIC_AUTH_TOKEN` en el archivo `.claude/settings.local.json` del proyecto. El token autoriza el proxy de loopback y no es una credencial de Antigravity.

El hook SessionStart inicia un daemon local y registra el proceso de Claude Code antes de su primera petición al modelo. El daemon se cierra cuando todos los procesos de Claude Code registrados han terminado. Por defecto, el proxy escucha solo en loopback.

Con la ruta OAuth, el daemon inicia el subproceso oficial `agy acp`. NeoSwitch envía la petición traducida a través de una sesión ACP y convierte la respuesta, o el JSON de tool-call compatible con Claude, de vuelta al endpoint local compatible con Anthropic. Las credenciales OAuth y las devoluciones de llamada del navegador siguen gestionadas por `agy`.

Un único proceso `agy` atiende todas las sesiones y las peticiones se ejecutan en paralelo en sesiones ACP independientes. El texto se transmite a medida que se genera, el uso de tokens se informa a Claude Code y el ajuste `/effort` de Claude Code determina cuánto piensa Neon.

El ajuste opcional del plugin **Use Antigravity OAuth via ACP** fuerza la ruta ACP cuando también hay otra credencial de Gemini. Un `/neoswitch:login` normal usa ACP automáticamente si no hay ninguna credencial de Gemini configurada.

## CLI independiente

Compila el binario o ejecútalo desde el repositorio:

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

Comandos habituales:

```bash
neoswitch antigravity login   # inicio de sesión ACP OAuth con el cliente oficial agy
neoswitch antigravity probe   # verifica la sesión guardada de agy sin mostrar credenciales
neoswitch doctor              # comprueba el backend seleccionado y la CLI de Claude
neoswitch models              # muestra el modelo configurado
neoswitch on [project]        # activa el enrutamiento de un proyecto
neoswitch off [project]       # desactiva el enrutamiento de un proyecto
neoswitch status [project]    # muestra el enrutamiento y el estado del daemon
```

Para ejecutar el proxy local para otro cliente compatible con Anthropic:

```bash
export NEOSWITCH_AUTH_TOKEN="$(openssl rand -hex 24)"
GEMINI_API_KEY='your-key' neoswitch serve
```

Configura tanto `ANTHROPIC_BASE_URL` como `ANTHROPIC_AUTH_TOKEN` en el cliente. `neoswitch claude` configura esas variables automáticamente con un puerto y un token efímeros.

## Solución de problemas

| Síntoma | Solución |
|---|---|
| `/neoswitch:login` no se reconoce | Actualiza el plugin, reinicia Claude Code y ejecuta `/reload-plugins`. |
| `agy: command not found` | Instala la CLI oficial de Antigravity o define `ANTIGRAVITY_CLI_PATH`. |
| `no authorization code was entered` | Actualiza a la última versión del plugin. El flujo actual usa ACP `authenticate` y no lee la stdin de la shell. |
| `ECONNREFUSED` en una sesión nueva | Ejecuta `/neoswitch:status`. Si la ruta está activa, revisa el mensaje del daemon y reinicia la sesión. Ejecuta `/neoswitch:off` desde una sesión que use Anthropic y vuelve a activarlo después de corregir la credencial. |
| La sesión se inicia sin credenciales de Gemini | Ejecuta `/neoswitch:login` o configura una clave de API de Gemini o Vertex AI ADC. |
| Falla la compilación del plugin | Instala Go y reinicia Claude Code. |

## Límites de compatibilidad

NeoSwitch usa el SDK oficial de Gemini o el proceso local `agy acp` sin modificar. No extrae tokens OAuth, no llama a APIs HTTP privadas de Antigravity, no reenvía credenciales a un proxy remoto, no modifica el archivo hosts y no cambia silenciosamente a otro modelo.

Claude Code sigue siendo responsable de la sesión, los permisos, MCP y el bucle de herramientas. Anthropic no documenta el enrutamiento de Claude Code hacia modelos que no son Claude, por lo que las actualizaciones de Claude Code pueden cambiar comportamientos de los que depende NeoSwitch.

Consulta [docs/protocol-contract.md](../docs/protocol-contract.md) para el contrato del endpoint y la matriz de compatibilidad, y [docs/antigravity.md](../docs/antigravity.md) para los detalles de la autenticación ACP.

## Desarrollo

```bash
go test ./...
go test -race ./...
go vet ./...
```

MIT License
