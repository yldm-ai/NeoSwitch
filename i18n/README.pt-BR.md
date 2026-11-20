# NeoSwitch

[English](../README.md) | [简体中文](README.zh-CN.md) | [繁體中文](README.zh-TW.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md) | **Português (Brasil)** | [Русский](README.ru.md)

[![CI](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/NeoSwitch.svg)](https://pkg.go.dev/github.com/yldm-ai/NeoSwitch) [![License](https://img.shields.io/github/license/yldm-ai/NeoSwitch)](https://github.com/yldm-ai/NeoSwitch/blob/main/LICENSE)

NeoSwitch é um plugin do Claude Code que roteia projetos para o Gemini 4 Neon por meio de um proxy local compatível com a Anthropic. Você pode escolher Antigravity OAuth, a Gemini Developer API ou o Vertex AI e manter o fluxo de trabalho, o streaming e o uso de ferramentas do Claude Code.

Há suporte a dois caminhos de credenciais para o serviço de origem:

- Antigravity OAuth pelo processo local oficial `agy acp`.
- Credenciais da Gemini Developer API ou do Vertex AI pelo SDK oficial `google.golang.org/genai`.

Para o Claude Code, recomenda-se o fluxo do plugin com Antigravity OAuth. As credenciais OAuth ficam dentro do `agy`; o NeoSwitch não lê nem exporta o token.

## Usar o Claude Code com o Neon

### Pré-requisitos

- Claude Code.
- A CLI oficial do Antigravity, disponível como `agy` no seu PATH.
- Go. O plugin compila o binário do NeoSwitch incluído no primeiro uso.

### Instalar o plugin

Execute estes comandos dentro do Claude Code:

```text
/plugin marketplace add yldm-ai/NeoSwitch
/plugin install neoswitch@neoswitch
```

Se o plugin já estiver instalado, atualize-o pelo terminal depois de uma nova versão:

```bash
claude plugin marketplace update neoswitch
claude plugin update neoswitch@neoswitch
```

Reinicie o Claude Code depois de atualizar. O fluxo de login ACP exige a versão atual do plugin.

### Entrar e ativar o Neon

No projeto em que você quer usar o Neon, execute:

```text
/neoswitch:login
```

O comando executa as seguintes etapas:

1. Inicia o fluxo de autenticação ACP gerenciado pelo `agy`.
2. Abre o navegador quando o Google OAuth é necessário.
3. Ativa o roteamento do NeoSwitch para o projeto atual.
4. Seleciona `gemini-4-neon` para a sessão ACP.

Quando o comando terminar, feche e reabra o Claude Code nesse projeto. Verifique a rota com:

```text
/neoswitch:status
```

O status deve mostrar `NeoSwitch ... -> gemini-4-neon`. O Claude Code ainda pode exibir o rótulo solicitado, como `claude-opus-5-5`, no cabeçalho da sessão. A rota real de origem é mostrada por `/neoswitch:status`.

`/neoswitch:antigravity-login` é um alias de `/neoswitch:login`. `/neoswitch:on` e `/neoswitch:off` continuam disponíveis para alterar o roteamento sem fazer login.

## Outros caminhos de credenciais

### Chave de API do Gemini

```bash
export GEMINI_API_KEY='your-key'
go run ./cmd/neoswitch claude
```

O plugin também pode receber a chave em `/plugin` → `neoswitch` → **Gemini API key**. O Claude Code armazena a chave na configuração segura de plugins.

### Vertex AI

```bash
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT=my-project
export GOOGLE_CLOUD_LOCATION=global
go run ./cmd/neoswitch claude
```

As Application Default Credentials do Google Cloud SDK já devem estar disponíveis para o processo.

## Comandos do plugin

| Comando | Efeito |
|---|---|
| `/neoswitch:login` | Autentica com Antigravity OAuth, ativa a rota do Neon neste projeto e usa ACP |
| `/neoswitch:on` | Ativa o roteamento para novas sessões do Claude Code neste projeto |
| `/neoswitch:status` | Mostra a rota do projeto e o estado do daemon |
| `/neoswitch:off` | Desativa o roteamento para novas sessões neste projeto |

Depois de alterar o roteamento, inicie uma nova sessão do Claude Code. A sessão atual mantém a conexão existente.

## Como o plugin funciona

`/neoswitch:on` grava apenas `ANTHROPIC_BASE_URL` e `ANTHROPIC_AUTH_TOKEN` no arquivo `.claude/settings.local.json` do projeto. O token autoriza o proxy de loopback e não é uma credencial do Antigravity.

O hook SessionStart inicia um daemon local e registra o processo do Claude Code antes da primeira requisição ao modelo. O daemon é encerrado depois que todos os processos do Claude Code registrados terminam. Por padrão, o proxy escuta apenas em loopback.

Na rota OAuth, o daemon inicia o subprocesso oficial `agy acp`. O NeoSwitch envia a requisição traduzida por uma sessão ACP e converte a resposta, ou o JSON de tool-call compatível com o Claude, de volta para o endpoint local compatível com a Anthropic. As credenciais OAuth e os callbacks do navegador continuam sob responsabilidade do `agy`.

Um único processo `agy` atende todas as sessões, e as requisições são executadas em paralelo em sessões ACP separadas. O texto é transmitido conforme é gerado, o uso de tokens é informado ao Claude Code e a configuração `/effort` do Claude Code define quanto tempo o Neon pensa.

A configuração opcional do plugin **Use Antigravity OAuth via ACP** força a rota ACP quando outra credencial do Gemini também está presente. Um `/neoswitch:login` normal usa ACP automaticamente quando nenhuma credencial do Gemini está configurada.

## CLI independente

Compile o binário ou execute-o a partir do repositório:

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

Comandos comuns:

```bash
neoswitch antigravity login   # login ACP OAuth pelo cliente oficial agy
neoswitch antigravity probe   # verifica a sessão salva do agy sem exibir credenciais
neoswitch doctor              # verifica o backend selecionado e a CLI do Claude
neoswitch models              # exibe o modelo configurado
neoswitch on [project]        # ativa o roteamento de um projeto
neoswitch off [project]       # desativa o roteamento de um projeto
neoswitch status [project]    # mostra o roteamento e o estado do daemon
```

Para executar o proxy local para outro cliente compatível com a Anthropic:

```bash
export NEOSWITCH_AUTH_TOKEN="$(openssl rand -hex 24)"
GEMINI_API_KEY='your-key' neoswitch serve
```

Defina `ANTHROPIC_BASE_URL` e `ANTHROPIC_AUTH_TOKEN` no cliente. `neoswitch claude` configura essas variáveis automaticamente com uma porta e um token temporários.

## Solução de problemas

| Sintoma | Solução |
|---|---|
| `/neoswitch:login` não é reconhecido | Atualize o plugin, reinicie o Claude Code e execute `/reload-plugins`. |
| `agy: command not found` | Instale a CLI oficial do Antigravity ou defina `ANTIGRAVITY_CLI_PATH`. |
| `no authorization code was entered` | Atualize para a versão mais recente do plugin. O fluxo atual usa ACP `authenticate` e não lê a stdin do shell. |
| `ECONNREFUSED` em uma nova sessão | Execute `/neoswitch:status`. Se a rota estiver ativa, verifique a mensagem do daemon e reinicie a sessão. Execute `/neoswitch:off` a partir de uma sessão que use a Anthropic e ative novamente depois de corrigir a credencial. |
| A sessão inicia sem credenciais do Gemini | Execute `/neoswitch:login` ou configure uma chave de API do Gemini ou o Vertex AI ADC. |
| A compilação do plugin falha | Instale o Go e reinicie o Claude Code. |

## Limites de compatibilidade

O NeoSwitch usa o SDK oficial do Gemini ou o processo local `agy acp` sem modificações. Ele não extrai tokens OAuth, não chama APIs HTTP privadas do Antigravity, não repassa credenciais para um proxy remoto, não modifica o arquivo hosts e nunca troca silenciosamente para outro modelo.

O Claude Code continua responsável pela sessão, pelas permissões, pelo MCP e pelo loop de ferramentas. A Anthropic não documenta o roteamento do Claude Code para modelos que não sejam Claude, então atualizações do Claude Code podem alterar comportamentos dos quais o NeoSwitch depende.

Consulte [docs/protocol-contract.md](../docs/protocol-contract.md) para o contrato do endpoint e a matriz de compatibilidade, e [docs/antigravity.md](../docs/antigravity.md) para detalhes da autenticação ACP.

## Desenvolvimento

```bash
go test ./...
go test -race ./...
go vet ./...
```

MIT License
