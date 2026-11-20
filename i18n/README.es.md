# NeoSwitch

[English](../README.md) | [简体中文](README.zh-CN.md) | [繁體中文](README.zh-TW.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | **Español** | [Français](README.fr.md) | [Deutsch](README.de.md) | [Português (Brasil)](README.pt-BR.md) | [Русский](README.ru.md)

[![Latest release](https://img.shields.io/github/v/release/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/releases) [![CI](https://github.com/yldm-ai/neon-switch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/neon-switch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/neon-switch.svg)](https://pkg.go.dev/github.com/yldm-ai/neon-switch) [![Go version](https://img.shields.io/github/go-mod/go-version/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/blob/main/go.mod) [![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/yldm-ai/neon-switch/blob/main/LICENSE) [![Last commit](https://img.shields.io/github/last-commit/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/commits/main) [![GitHub stars](https://img.shields.io/github/stars/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/stargazers)

[![Claude Code plugin](https://img.shields.io/badge/Claude%20Code-plugin-D97757)](https://github.com/yldm-ai/neon-switch/blob/main/.claude-plugin/plugin.json) [![Model: Gemini 4 Neon](https://img.shields.io/badge/model-Gemini%204%20Neon-4285F4)](https://github.com/yldm-ai/neon-switch/blob/main/docs/protocol-contract.md) [![Platform: macOS and Linux](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey)](https://github.com/yldm-ai/neon-switch/releases) [![Proxy: local-only](https://img.shields.io/badge/proxy-local--only-2ea44f)](https://github.com/yldm-ai/neon-switch/blob/main/docs/protocol-contract.md)

**Keep Claude Code. Switch your models.**

NeoSwitch te permite usar Gemini con Claude Code sin cambiar el flujo de trabajo que ya conoces. Claude Code conserva su CLI, comandos, permisos, herramientas, skills, servidores MCP y hooks; NeoSwitch ejecuta un pequeño proxy local que traduce sus solicitudes para el modelo que hay detrás.

Hoy NeoSwitch enruta Claude Code a Gemini 4 Neon. OpenAI está disponible como backend experimental: hay que activarlo explícitamente y solo se ha probado contra un upstream simulado con guion.

## Por qué NeoSwitch

- **Conserva tu flujo de trabajo**: sigues ejecutando Claude Code. Solo cambia el modelo.
- **Por proyecto**: activa el enrutamiento en un proyecto y deja los demás en Anthropic.
- **Local primero**: el proxy escucha solo en loopback, usa un token de acceso aleatorio y no registra prompts, respuestas ni credenciales. No interviene ningún relay remoto.
- **Compatibilidad sin rodeos**: lo que funciona, lo que funciona en parte y lo que falta se detalla [más abajo](#compatibilidad).

## Elegir una vía de credenciales

| Vía | Cómo inicia sesión | Notas |
|---|---|---|
| Antigravity OAuth | Cuenta de Google mediante la CLI oficial `agy` | Flujo predeterminado del plugin. Lee primero la advertencia de abajo. |
| Gemini Developer API | `GEMINI_API_KEY` | El acceso a Gemini 4 Neon depende de tu clave. Compruébalo con `neoswitch doctor`. |
| Vertex AI | Application Default Credentials de Google Cloud | El acceso depende de tu proyecto y región. Compruébalo con `neoswitch doctor`. |

> [!WARNING]
> Los [Términos de Servicio de Google Antigravity](https://antigravity.google/terms) (sección 6) consideran una infracción acceder a Antigravity mediante software, herramientas o servicios de terceros, y establecen que Google puede suspender o cancelar las cuentas de Antigravity y Gemini CLI afectadas. Enrutar Claude Code a través de Antigravity OAuth entra en esa regla. Úsalo solo si aceptas ese riesgo para la cuenta con la que inicias sesión; si no, usa una clave de API de Gemini o Vertex AI. `neoswitch doctor` y `neoswitch antigravity login` muestran la misma advertencia.

Las credenciales OAuth siempre permanecen dentro de `agy`. NeoSwitch no lee, copia ni exporta el token.

## Usar Claude Code con Neon

### Requisitos previos

- Claude Code.
- Para Antigravity OAuth: la CLI oficial de Antigravity, disponible como `agy` en tu PATH.
- Go (opcional). Con Go, el plugin compila el binario de NeoSwitch incluido a partir de su código fuente la primera vez que se usa. Sin Go, descarga el binario de la release indicada por el archivo `VERSION` del repositorio para macOS o Linux (amd64 o arm64) y verifica su checksum SHA-256 antes de ejecutarlo. Entre releases, ese binario puede ser más antiguo que el código fuente del plugin.

### Instalar el plugin

Ejecuta estos comandos dentro de Claude Code:

```text
/plugin marketplace add yldm-ai/neon-switch
/plugin install neoswitch@neoswitch
```

Si el plugin ya estaba instalado, actualízalo desde una terminal cuando salga una nueva versión:

```bash
claude plugin marketplace update neoswitch
claude plugin update neoswitch@neoswitch
```

Reinicia Claude Code después de actualizar.

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

Para usar una clave de API de Gemini en su lugar, exporta `GEMINI_API_KEY` antes de iniciar Claude Code y ejecuta `/neoswitch:on`; el plugin no tiene ajustes que rellenar. Si antes iniciaste sesión con `/neoswitch:login`, exporta también `NEOSWITCH_BACKEND=gemini`, porque de lo contrario prevalece la elección guardada al iniciar sesión.

Cierra y vuelve a abrir Claude Code en ese proyecto. Comprueba la ruta con:

```text
/neoswitch:status
```

El estado debería mostrar `NeoSwitch ... -> gemini-4-neon`. Claude Code puede seguir mostrando la etiqueta solicitada, como `claude-opus-5-5`, en el encabezado de la sesión; la ruta real hacia el servicio de origen es la que muestra `/neoswitch:status`.

`/neoswitch:antigravity-login` es un alias de `/neoswitch:login`. `/neoswitch:on` y `/neoswitch:off` cambian el enrutamiento sin iniciar sesión.

## Otras vías de credenciales

### Clave de API de Gemini

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

Las Application Default Credentials del Google Cloud SDK ya deben estar disponibles para el proceso.

`neoswitch doctor` busca el modelo y ejecuta una generación de una línea; después informa si las credenciales funcionan o por qué no: credenciales rechazadas, falta de permisos, restricción de región, modelo desconocido, cuota o servicio no disponible.

### OpenAI (experimental)

El backend de OpenAI envía las solicitudes a la API Responses de OpenAI mediante el SDK oficial `openai-go`. Hay que activarlo explícitamente: la selección automática nunca lo elige, así que define `NEOSWITCH_BACKEND=openai` o ejecuta `neoswitch backend openai`. `OPENAI_MODEL` es obligatorio; NeoSwitch no elige un modelo por ti. `OPENAI_BASE_URL` es opcional y debe incluir `/v1`. Solo se ha probado contra un upstream simulado con guion, no con una cuenta real de OpenAI. Lo que se traduce y lo que no se detalla en [docs/protocol-contract.md](../docs/protocol-contract.md#openai-backend-experimental).

```bash
export NEOSWITCH_BACKEND=openai
export OPENAI_API_KEY='your-key'
export OPENAI_MODEL='model-name'
neoswitch doctor
go run ./cmd/neoswitch claude
```

## Comandos del plugin

| Comando | Efecto |
|---|---|
| `/neoswitch:login` | Se autentica con Antigravity OAuth, activa la ruta Neon de este proyecto y usa ACP |
| `/neoswitch:on` | Activa el enrutamiento para las nuevas sesiones de Claude Code en este proyecto |
| `/neoswitch:status` | Muestra la ruta del proyecto y el estado del daemon |
| `/neoswitch:off` | Desactiva el enrutamiento para las nuevas sesiones en este proyecto |

Después de cambiar el enrutamiento, inicia una nueva sesión de Claude Code. La sesión actual mantiene su conexión existente.

## Compatibilidad

Claude Code sigue ejecutando la sesión, los permisos, la ejecución de herramientas, los servidores MCP, las skills, los hooks, `CLAUDE.md` y `/resume`; NeoSwitch solo sustituye el modelo que hay detrás. Lo bien que funcionen esas funciones depende de lo bien que el modelo siga el protocolo de herramientas de Claude Code, que se ha probado en el uso diario de programación. Una suite automatizada de extremo a extremo ejecuta en cada cambio la CLI real de Claude Code a través de NeoSwitch contra un servidor Gemini guionizado y comprueba que las llamadas a herramientas y sus resultados, las ediciones de archivos, Bash, las llamadas en paralelo, los permisos denegados, los hooks, las skills, `/resume`, los subagentes y las herramientas MCP pasan intactos por la ruta de la API de Gemini; no mide lo bien que un modelo real los usa.

| Capacidad | Gemini API / Vertex AI | Antigravity OAuth (ACP) |
|---|---|---|
| Streaming de texto | Soportado | Soportado |
| Llamadas a herramientas, incluidas las paralelas | Function calling nativo de Gemini | Parcial: el modelo escribe JSON de tool-call de Claude, que llega completo en lugar de en streaming |
| Uso de herramientas en varios turnos y `/resume` | Soportado; las thought signatures viajan en los ID de las herramientas | Soportado |
| Imágenes | Soportado | Soportado cuando `agy` ofrece entrada de imágenes |
| PDF | Soportado | No soportado; se indica al modelo que se omitió un archivo |
| `/effort` | Se envía como nivel de razonamiento de Gemini; no verificado con la API real, y se descarta si el modelo lo rechaza | Se asigna al nivel de razonamiento de `agy` |
| Uso de tokens y `count_tokens` | Se informa; en la Developer API `count_tokens` es una estimación | Se informa, incluidos unos 14k tokens de instrucciones de `agy` añadidos a cada prompt |
| Acceso a Gemini 4 Neon | Depende de tu clave o proyecto; compruébalo con `neoswitch doctor` | Verificado con un `agy` con sesión iniciada |

No soportado en ninguna de las dos vías: herramientas de servidor como la búsqueda web y la ejecución de código, fuentes de imagen y documento por URL, y los bloques de thinking de Claude. `cache_control` y `context_management` se ignoran. NeoSwitch nunca recurre en silencio a otro modelo.

El contrato completo del endpoint está en [docs/protocol-contract.md](../docs/protocol-contract.md), y los detalles de ACP en [docs/antigravity.md](../docs/antigravity.md).

## Cómo funciona el plugin

`/neoswitch:on` escribe solo tres claves en el archivo `.claude/settings.local.json` del proyecto: `ANTHROPIC_BASE_URL` y `ANTHROPIC_AUTH_TOKEN` para el proxy, y `CLAUDE_CODE_USE_GATEWAY=0`, porque en el modo gateway, que activan los lanzadores de gateways, Claude Code tomaría la URL base de su entorno e ignoraría el proxy sin avisar. El token autoriza el proxy de loopback y no es una credencial de Antigravity.

El hook SessionStart inicia un daemon local y registra el proceso de Claude Code antes de su primera solicitud al modelo. El daemon termina cuando todos los procesos de Claude Code registrados han terminado.

Con la ruta OAuth, el daemon inicia el subproceso oficial `agy acp`. NeoSwitch envía la solicitud traducida a través de una sesión ACP y vuelve a traducir la respuesta o el JSON de tool-call devuelto al endpoint local compatible con Anthropic. Un solo proceso `agy` atiende todas las sesiones, y las solicitudes se ejecutan en paralelo en sesiones ACP separadas. Las herramientas propias de `agy` para archivos, shell y web se rechazan en cada solicitud, así que las herramientas solo se ejecutan en Claude Code, bajo sus reglas de permisos.

`/neoswitch:login` guarda Antigravity OAuth como backend, así que una clave de Gemini exportada para otras herramientas no lo sustituye. Sin una elección guardada, NeoSwitch usa las credenciales de Gemini del entorno si existen y, si no, Antigravity OAuth. `NEOSWITCH_BACKEND=antigravity|gemini|openai|auto` anula ambas cosas, y `/neoswitch:status` muestra el backend en uso y el motivo.

## CLI independiente

Cada versión etiquetada publica binarios para macOS (Apple Silicon e Intel), Linux (amd64 y arm64) y Windows (amd64), con sumas de comprobación SHA-256, en la página de [Releases](https://github.com/yldm-ai/neon-switch/releases). La compilación para Windows se genera, pero todavía no se ha probado. También puedes compilar desde el código fuente:

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

Comandos habituales:

```bash
neoswitch doctor              # comprueba la CLI de Claude y si las credenciales pueden usar el modelo
neoswitch antigravity login   # inicio de sesión ACP OAuth mediante el cliente oficial agy
neoswitch antigravity probe   # verifica la sesión de agy guardada sin mostrar credenciales
neoswitch models              # lista los modelos que pueden usar las credenciales y marca el configurado
neoswitch on [project]        # activa el enrutamiento de un proyecto
neoswitch off [project]       # desactiva el enrutamiento de un proyecto
neoswitch status [project]    # muestra el enrutamiento y el estado del daemon
neoswitch version             # muestra la versión de NeoSwitch
```

Para ejecutar el proxy local para otro cliente compatible con Anthropic:

```bash
export NEOSWITCH_AUTH_TOKEN="$(openssl rand -hex 24)"
GEMINI_API_KEY='your-key' neoswitch serve
```

Define tanto `ANTHROPIC_BASE_URL` como `ANTHROPIC_AUTH_TOKEN` en el cliente. `neoswitch claude` configura esas variables automáticamente con un puerto y un token efímeros.

## Actualizar y desinstalar

### Actualizar

Actualiza el plugin con los comandos de [Instalar el plugin](#instalar-el-plugin). Para la CLI independiente, descarga un binario más reciente desde la página de [Releases](https://github.com/yldm-ai/neon-switch/releases) o instala la última versión con Go:

```bash
go install github.com/yldm-ai/neon-switch/cmd/neoswitch@latest
```

El repositorio se trasladó de `yldm-ai/NeoSwitch` a `yldm-ai/neon-switch`. Si agregaste el marketplace con el nombre anterior, vuelve a agregarlo desde la nueva ubicación:

```bash
claude plugin marketplace add yldm-ai/neon-switch
```

### Desinstalar

1. Antes de desinstalar, ejecuta `/neoswitch:off` (o `neoswitch off <project>`) en cada proyecto con el enrutamiento activado. Desinstalar el plugin no modifica los archivos del proyecto: si omites este paso, el objeto `env` del `.claude/settings.local.json` de ese proyecto conserva `ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN` y `CLAUDE_CODE_USE_GATEWAY`, y las sesiones nuevas intentan conectarse a un proxy que ya no se inicia. Si el plugin ya no está, elimina esas tres entradas a mano.
2. Cierra las sesiones de Claude Code que usan NeoSwitch y luego elimina el plugin y su marketplace desde una terminal:

```bash
claude plugin uninstall neoswitch@neoswitch
claude plugin marketplace remove neoswitch
```

3. Borra los archivos que NeoSwitch guarda fuera de tus proyectos:

| Qué | Ubicación |
|---|---|
| Binario del plugin compilado en el primer uso | `$XDG_CACHE_HOME/neoswitch/`, o `~/.cache/neoswitch/` si `XDG_CACHE_HOME` no está definida |
| Directorio de estado: `proxy.json` (dirección del proxy, token, backend guardado) y `daemon.log` | `NEOSWITCH_STATE_DIR` si está definida; si no, `~/Library/Application Support/neoswitch/` en macOS y `$XDG_CONFIG_HOME/neoswitch/` o `~/.config/neoswitch/` en Linux |

El inicio de sesión de Antigravity lo guarda `agy`, no NeoSwitch, y se conserva. Para quitar una CLI independiente instalada con `go install`, borra `neoswitch` de `$(go env GOBIN)`, o de `$(go env GOPATH)/bin` si `GOBIN` no está definida.

## Solución de problemas

| Síntoma | Solución |
|---|---|
| No sabes si las credenciales funcionan | Ejecuta `neoswitch doctor`; indica el paso que falla y la causa. |
| `/neoswitch:login` no se reconoce | Actualiza el plugin, reinicia Claude Code y ejecuta `/reload-plugins`. |
| `agy: command not found` | Instala la CLI oficial de Antigravity o define `ANTIGRAVITY_CLI_PATH`. |
| `no authorization code was entered` | Actualiza a la última versión del plugin. El flujo actual usa ACP `authenticate` y no lee la stdin de la shell. |
| `ECONNREFUSED` en una nueva sesión | Ejecuta `/neoswitch:status`. Si la ruta está activada, revisa el mensaje del daemon y reinicia la sesión. Ejecuta `/neoswitch:off` desde una sesión que use Anthropic y vuelve a activarlo después de corregir la credencial. |
| La sesión indica que no hay ningún backend configurado | Ejecuta `/neoswitch:login`, o exporta `GEMINI_API_KEY` o la configuración de Vertex AI antes de iniciar Claude Code. |
| Falla la compilación o la descarga del plugin | Instala Go y reinicia Claude Code; con Go el plugin compila desde el código fuente y no descarga nada. Sin Go, comprueba que GitHub Releases sea accesible. Un checksum que no coincide significa que la descarga se rechazó y no se ejecutó nada. |

## Límites de compatibilidad

NeoSwitch usa el SDK oficial de Gemini o el proceso local `agy acp` sin modificar. No extrae tokens OAuth, no llama a API HTTP privadas de Antigravity, no reenvía credenciales a un proxy remoto ni modifica el archivo hosts.

Anthropic no documenta el enrutamiento de Claude Code hacia modelos que no son Claude, por lo que las actualizaciones de Claude Code pueden cambiar comportamientos de los que depende NeoSwitch.

## Hoja de ruta

- Más proveedores de modelos. OpenAI está disponible como backend experimental y aún falta verificarlo contra la API real. Los proveedores que ya ofrecen un endpoint compatible con Anthropic se pueden usar definiendo `ANTHROPIC_BASE_URL` directamente y no necesitan NeoSwitch.

## Desarrollo

```bash
go test ./...
go test -race ./...
go vet ./...
```

Para publicar una release, actualiza `VERSION` y sube el tag correspondiente (`v` seguido de su contenido, como `v0.1.0`); el workflow de release rechaza un tag que no coincida.

Consulta [CONTRIBUTING.md](../CONTRIBUTING.md) para el flujo de contribución, [SECURITY.md](../SECURITY.md) para informar de una vulnerabilidad en privado y [CODE_OF_CONDUCT.md](../CODE_OF_CONDUCT.md) para el código de conducta.

MIT License
