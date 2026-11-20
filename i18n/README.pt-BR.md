# NeoSwitch

[English](../README.md) | [简体中文](README.zh-CN.md) | [繁體中文](README.zh-TW.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md) | **Português (Brasil)** | [Русский](README.ru.md)

[![CI](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/NeoSwitch.svg)](https://pkg.go.dev/github.com/yldm-ai/NeoSwitch) [![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/yldm-ai/NeoSwitch/blob/main/LICENSE)

**Keep Claude Code. Switch your models.**

O NeoSwitch permite usar o Gemini com o Claude Code sem mudar o fluxo de trabalho que você já conhece. O Claude Code mantém sua CLI, comandos, permissões, ferramentas, skills, servidores MCP e hooks; o NeoSwitch executa um pequeno proxy local que traduz as requisições para o modelo por trás dele.

Hoje o NeoSwitch roteia o Claude Code para o Gemini 4 Neon. Outros provedores estão planejados, mas ainda não são suportados.

## Por que o NeoSwitch

- **Mantenha seu fluxo de trabalho**: você continua usando o Claude Code. Só o modelo muda.
- **Por projeto**: ative o roteamento em um projeto e deixe os outros na Anthropic.
- **Local primeiro**: o proxy escuta apenas em loopback, usa um token de acesso aleatório e não registra prompts, respostas nem credenciais. Nenhum relay remoto é envolvido.
- **Compatibilidade honesta**: o que funciona, o que funciona parcialmente e o que falta está listado [abaixo](#compatibilidade).

## Escolha um caminho de credenciais

| Caminho | Como faz login | Observações |
|---|---|---|
| Antigravity OAuth | Conta Google pela CLI oficial `agy` | Fluxo padrão do plugin. Leia o aviso abaixo primeiro. |
| Gemini Developer API | `GEMINI_API_KEY` | O acesso ao Gemini 4 Neon depende da sua chave. Verifique com `neoswitch doctor`. |
| Vertex AI | Application Default Credentials do Google Cloud | O acesso depende do seu projeto e da sua região. Verifique com `neoswitch doctor`. |

> [!WARNING]
> Os [Termos de Serviço do Google Antigravity](https://antigravity.google/terms) (seção 6) tratam o acesso ao Antigravity por software, ferramentas ou serviços de terceiros como violação e dizem que o Google pode suspender ou encerrar as contas do Antigravity e da Gemini CLI envolvidas. Rotear o Claude Code pelo Antigravity OAuth se enquadra nessa regra. Use esse caminho apenas se você aceitar esse risco para a conta com que faz login; caso contrário, use uma chave de API do Gemini ou o Vertex AI. `neoswitch doctor` e `neoswitch antigravity login` exibem o mesmo aviso.

As credenciais OAuth sempre ficam dentro do `agy`. O NeoSwitch não lê, não copia e não exporta o token.

## Usar o Claude Code com o Neon

### Pré-requisitos

- Claude Code.
- Para Antigravity OAuth: a CLI oficial do Antigravity, disponível como `agy` no seu PATH.
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

Reinicie o Claude Code depois de atualizar.

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

Para usar uma chave de API do Gemini em vez disso, defina-a em `/plugin` → `neoswitch` → **Gemini API key** (o Claude Code a armazena na configuração segura de plugins) e execute `/neoswitch:on`.

Feche e reabra o Claude Code nesse projeto. Verifique a rota com:

```text
/neoswitch:status
```

O status deve mostrar `NeoSwitch ... -> gemini-4-neon`. O Claude Code ainda pode exibir o rótulo solicitado, como `claude-opus-5-5`, no cabeçalho da sessão; a rota de origem é a que `/neoswitch:status` mostra.

`/neoswitch:antigravity-login` é um alias de `/neoswitch:login`. `/neoswitch:on` e `/neoswitch:off` alteram o roteamento sem fazer login.

## Outros caminhos de credenciais

### Chave de API do Gemini

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

As Application Default Credentials do Google Cloud SDK já devem estar disponíveis para o processo.

`neoswitch doctor` procura o modelo e executa uma geração de uma linha, depois informa se as credenciais funcionam ou por que não funcionam: credenciais rejeitadas, permissão ausente, restrição de região, modelo desconhecido, cota ou serviço indisponível.

## Comandos do plugin

| Comando | Efeito |
|---|---|
| `/neoswitch:login` | Autentica com Antigravity OAuth, ativa a rota do Neon neste projeto e usa ACP |
| `/neoswitch:on` | Ativa o roteamento para novas sessões do Claude Code neste projeto |
| `/neoswitch:status` | Mostra a rota do projeto e o estado do daemon |
| `/neoswitch:off` | Desativa o roteamento para novas sessões neste projeto |

Depois de alterar o roteamento, inicie uma nova sessão do Claude Code. A sessão atual mantém a conexão existente.

## Compatibilidade

O Claude Code continua executando a sessão, as permissões, a execução de ferramentas, os servidores MCP, as skills, os hooks, o `CLAUDE.md` e o `/resume`; o NeoSwitch só substitui o modelo por trás deles. O quanto esses recursos funcionam bem depende de quão bem o modelo segue o protocolo de ferramentas do Claude Code, que foi exercitado no uso diário de programação, mas não é coberto por uma suíte automatizada de ponta a ponta.

| Recurso | Gemini API / Vertex AI | Antigravity OAuth (ACP) |
|---|---|---|
| Streaming de texto | Suportado | Suportado |
| Chamadas de ferramentas, inclusive paralelas | Function calling nativo do Gemini | Parcial: o modelo escreve o JSON de tool-call do Claude, que chega inteiro em vez de transmitido |
| Uso de ferramentas em vários turnos e `/resume` | Suportado; as thought signatures são levadas nos IDs das ferramentas | Suportado |
| Imagens | Suportado | Suportado quando o `agy` oferece entrada de imagem |
| PDFs | Suportado | Não suportado; o modelo é informado de que um arquivo foi omitido |
| `/effort` | Ainda não aplicado | Mapeado para o nível de pensamento do `agy` |
| Uso de tokens e `count_tokens` | Informado; `count_tokens` é estimado na Developer API | Informado, incluindo cerca de 14k tokens de instruções do `agy` adicionados a cada prompt |
| Acesso ao Gemini 4 Neon | Depende da sua chave ou projeto; verifique com `neoswitch doctor` | Verificado com um `agy` conectado |

Não suportado em nenhum dos caminhos: ferramentas de servidor como busca na web e execução de código, fontes de imagem e documento por URL e blocos de thinking do Claude. `cache_control` e `context_management` são ignorados. O NeoSwitch nunca troca silenciosamente para outro modelo.

O contrato completo do endpoint está em [docs/protocol-contract.md](../docs/protocol-contract.md), e os detalhes do ACP estão em [docs/antigravity.md](../docs/antigravity.md).

## Como o plugin funciona

`/neoswitch:on` grava apenas `ANTHROPIC_BASE_URL` e `ANTHROPIC_AUTH_TOKEN` no arquivo `.claude/settings.local.json` do projeto. O token autoriza o proxy de loopback e não é uma credencial do Antigravity.

O hook SessionStart inicia um daemon local e registra o processo do Claude Code antes da primeira requisição ao modelo. O daemon é encerrado depois que todos os processos do Claude Code registrados terminam.

Na rota OAuth, o daemon inicia o subprocesso oficial `agy acp`. O NeoSwitch envia a requisição traduzida por uma sessão ACP e converte a resposta, ou o JSON de tool-call, de volta para o endpoint local compatível com a Anthropic. Um único processo `agy` atende todas as sessões, e as requisições são executadas em paralelo em sessões ACP separadas. As ferramentas próprias de arquivo, shell e web do `agy` são recusadas em toda requisição, então as ferramentas rodam apenas no Claude Code, sob as regras de permissão do Claude Code.

A configuração opcional do plugin **Use Antigravity OAuth via ACP** força a rota ACP quando outra credencial do Gemini também está presente. Um `/neoswitch:login` normal usa ACP automaticamente quando nenhuma credencial do Gemini está configurada.

## CLI independente

Cada versão com tag publica binários para macOS (Apple Silicon e Intel), Linux (amd64 e arm64) e Windows (amd64), com checksums SHA-256, na página [Releases](https://github.com/yldm-ai/NeoSwitch/releases). O build para Windows é compilado, mas ainda não foi testado. Você também pode compilar a partir do código-fonte:

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

Comandos comuns:

```bash
neoswitch doctor              # verifica a CLI do Claude e se as credenciais podem usar o modelo
neoswitch antigravity login   # login ACP OAuth pelo cliente oficial agy
neoswitch antigravity probe   # verifica a sessão salva do agy sem exibir credenciais
neoswitch models              # exibe o modelo configurado
neoswitch on [project]        # ativa o roteamento de um projeto
neoswitch off [project]       # desativa o roteamento de um projeto
neoswitch status [project]    # mostra o roteamento e o estado do daemon
neoswitch version             # exibe a versão do NeoSwitch
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
| Não sabe se as credenciais funcionam | Execute `neoswitch doctor`; ele indica a etapa que falhou e a causa. |
| `/neoswitch:login` não é reconhecido | Atualize o plugin, reinicie o Claude Code e execute `/reload-plugins`. |
| `agy: command not found` | Instale a CLI oficial do Antigravity ou defina `ANTIGRAVITY_CLI_PATH`. |
| `no authorization code was entered` | Atualize para a versão mais recente do plugin. O fluxo atual usa ACP `authenticate` e não lê a stdin do shell. |
| `ECONNREFUSED` em uma nova sessão | Execute `/neoswitch:status`. Se a rota estiver ativa, verifique a mensagem do daemon e reinicie a sessão. Execute `/neoswitch:off` a partir de uma sessão que use a Anthropic e ative novamente depois de corrigir a credencial. |
| A sessão inicia sem credenciais do Gemini | Execute `/neoswitch:login` ou configure uma chave de API do Gemini ou o Vertex AI ADC. |
| A compilação do plugin falha | Instale o Go e reinicie o Claude Code. |

## Limites de compatibilidade

O NeoSwitch usa o SDK oficial do Gemini ou o processo local `agy acp` sem modificações. Ele não extrai tokens OAuth, não chama APIs HTTP privadas do Antigravity, não repassa credenciais para um proxy remoto e não modifica o arquivo hosts.

A Anthropic não documenta o roteamento do Claude Code para modelos que não sejam Claude, então atualizações do Claude Code podem alterar comportamentos dos quais o NeoSwitch depende.

## Roadmap

- Mais provedores de modelos, começando pela OpenAI. Provedores que já oferecem um endpoint compatível com a Anthropic podem ser usados definindo `ANTHROPIC_BASE_URL` diretamente e não precisam do NeoSwitch.
- Binários pré-compilados para o plugin, para que ele não precise mais do Go.

## Desenvolvimento

```bash
go test ./...
go test -race ./...
go vet ./...
```

MIT License
