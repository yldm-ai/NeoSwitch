# NeoSwitch

[English](../README.md) | [简体中文](README.zh-CN.md) | [繁體中文](README.zh-TW.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | [Français](README.fr.md) | [Deutsch](README.de.md) | **Português (Brasil)** | [Русский](README.ru.md)

[![Latest release](https://img.shields.io/github/v/release/yldm-ai/neo-switch)](https://github.com/yldm-ai/neo-switch/releases) [![CI](https://github.com/yldm-ai/neo-switch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/neo-switch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/neo-switch.svg)](https://pkg.go.dev/github.com/yldm-ai/neo-switch) [![Go version](https://img.shields.io/github/go-mod/go-version/yldm-ai/neo-switch)](https://github.com/yldm-ai/neo-switch/blob/main/go.mod) [![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/yldm-ai/neo-switch/blob/main/LICENSE) [![Last commit](https://img.shields.io/github/last-commit/yldm-ai/neo-switch)](https://github.com/yldm-ai/neo-switch/commits/main) [![GitHub stars](https://img.shields.io/github/stars/yldm-ai/neo-switch)](https://github.com/yldm-ai/neo-switch/stargazers)

[![Claude Code plugin](https://img.shields.io/badge/Claude%20Code-plugin-D97757)](https://github.com/yldm-ai/neo-switch/blob/main/.claude-plugin/plugin.json) [![Model: Gemini 4 Neon](https://img.shields.io/badge/model-Gemini%204%20Neon-4285F4)](https://github.com/yldm-ai/neo-switch/blob/main/docs/protocol-contract.md) [![Platform: macOS and Linux](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey)](https://github.com/yldm-ai/neo-switch/releases) [![Proxy: local-only](https://img.shields.io/badge/proxy-local--only-2ea44f)](https://github.com/yldm-ai/neo-switch/blob/main/docs/protocol-contract.md)

**Keep Claude Code. Switch your models.**

O NeoSwitch permite usar o Gemini com o Claude Code sem mudar o fluxo de trabalho que você já conhece. O Claude Code mantém sua CLI, comandos, permissões, ferramentas, skills, servidores MCP e hooks; o NeoSwitch executa um pequeno proxy local que traduz as requisições para o modelo por trás dele.

Hoje o NeoSwitch roteia o Claude Code para o Gemini 4 Neon. A OpenAI está disponível como backend experimental: precisa ser ativada explicitamente e só foi testada contra um upstream simulado por script.

## Por que o NeoSwitch

- **Mantenha seu fluxo de trabalho**: você continua usando o Claude Code. Só o modelo muda.
- **Por projeto**: ative o roteamento em um projeto e deixe os outros na Anthropic.
- **Local primeiro**: o proxy escuta apenas em loopback, usa um token de acesso aleatório e não registra prompts, respostas nem credenciais. Nenhum relay remoto é envolvido.
- **Compatibilidade honesta**: o que funciona, o que funciona parcialmente e o que falta está listado [abaixo](#compatibilidade).

## Escolha um caminho de credenciais

| Caminho | Como faz login | Observações |
|---|---|---|
| Antigravity OAuth | Conta Google pela CLI oficial `agy` | Fluxo padrão do plugin. Leia o aviso abaixo primeiro. |
| Gemini Developer API | `/neoswitch:key`, ou `GEMINI_API_KEY` para a CLI | O acesso ao Gemini 4 Neon depende da sua chave. Verifique com `neoswitch doctor`. |
| Vertex AI | Application Default Credentials do Google Cloud | O acesso depende do seu projeto e da sua região. Verifique com `neoswitch doctor`. |

> [!WARNING]
> Os [Termos de Serviço do Google Antigravity](https://antigravity.google/terms) (seção 6) tratam o acesso ao Antigravity por software, ferramentas ou serviços de terceiros como violação e dizem que o Google pode suspender ou encerrar as contas do Antigravity e da Gemini CLI envolvidas. Rotear o Claude Code pelo Antigravity OAuth se enquadra nessa regra. Use esse caminho apenas se você aceitar esse risco para a conta com que faz login; caso contrário, use uma chave de API do Gemini ou o Vertex AI. `neoswitch doctor` e `neoswitch antigravity login` exibem o mesmo aviso.

As credenciais OAuth sempre ficam dentro do `agy`. O NeoSwitch não lê, não copia e não exporta o token.

## Usar o Claude Code com o Neon

### Pré-requisitos

- Claude Code.
- Para Antigravity OAuth: a CLI oficial do Antigravity, disponível como `agy` no seu PATH.
- Go (opcional). Com Go, o plugin compila o binário do NeoSwitch incluído a partir do código-fonte no primeiro uso. Sem Go, ele baixa o binário da release indicada pelo arquivo `VERSION` do repositório para macOS ou Linux (amd64 ou arm64) e verifica o checksum SHA-256 antes de executá-lo. Entre releases, esse binário pode ser mais antigo que o código-fonte do plugin.

### Instalar o plugin

Execute estes comandos dentro do Claude Code:

```text
/plugin marketplace add yldm-ai/neo-switch
```

```text
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

Para usar uma chave de API do Gemini em vez disso, o que evita o risco à conta descrito acima, execute `/neoswitch:key`. Uma página abre no navegador para você colar a chave; a chave fica neste computador e não passa pelo chat. Ao salvá-la, o backend passa a ser a API do Gemini, mesmo depois de um `/neoswitch:login` anterior, e este projeto é ativado. Ainda não tem chave? Crie uma no [Google AI Studio](https://aistudio.google.com/apikey).

Feche e reabra o Claude Code nesse projeto. Se o Claude Code também estiver aberto em outras janelas, feche todas primeiro: enquanto alguma estiver aberta, o NeoSwitch mantém a configuração anterior. Verifique a rota com:

```text
/neoswitch:status
```

Quando funciona, a nova sessão começa com um aviso como `NeoSwitch: this session uses Gemini 4 Neon through a Gemini API key.`, e `/neoswitch:status` responde `This session uses Gemini 4 Neon through ..., and NeoSwitch is running.` O Claude Code ainda pode exibir o rótulo solicitado, como `claude-opus-5-5`, no cabeçalho da sessão; o modelo que responde de fato é o que o aviso do NeoSwitch e `/neoswitch:status` mostram.

`/neoswitch:on` e `/neoswitch:off` alternam o projeto sem fazer login de novo. `/neoswitch:on` para com uma mensagem, sem alterar nada, enquanto não houver uma chave da API do Gemini (`/neoswitch:key`) nem o login do Antigravity (`/neoswitch:login`).

## Outros caminhos de credenciais

### Chave de API do Gemini

No Claude Code, execute `/neoswitch:key`. Na CLI independente, exporte a chave em vez disso; se você já entrou com `/neoswitch:login`, execute também `neoswitch backend gemini`, pois caso contrário prevalece a escolha salva.

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

### OpenAI (experimental)

O backend da OpenAI envia as requisições para a API Responses da OpenAI pelo SDK oficial `openai-go`. Ele precisa ser ativado explicitamente: a seleção automática nunca o escolhe, então defina `NEOSWITCH_BACKEND=openai` ou execute `neoswitch backend openai`. `OPENAI_MODEL` é obrigatório; o NeoSwitch não escolhe um modelo por você. `OPENAI_BASE_URL` é opcional e deve incluir `/v1`. Ele só foi testado contra um upstream simulado por script, não com uma conta real da OpenAI. O que é e o que não é mapeado está em [docs/protocol-contract.md](../docs/protocol-contract.md#openai-backend-experimental).

```bash
export NEOSWITCH_BACKEND=openai
export OPENAI_API_KEY='your-key'
export OPENAI_MODEL='model-name'
neoswitch doctor
go run ./cmd/neoswitch claude
```

## Comandos do plugin

| Comando | Efeito |
|---|---|
| `/neoswitch:login` | Autentica com Antigravity OAuth, ativa a rota do Neon neste projeto e usa ACP |
| `/neoswitch:key` | Cola uma chave da API do Gemini em uma página neste computador, muda para a API do Gemini e ativa a rota do Neon neste projeto |
| `/neoswitch:on` | Ativa o roteamento para novas sessões do Claude Code neste projeto |
| `/neoswitch:status` | Diz se este projeto e esta sessão usam o Gemini 4 Neon e o que fazer em seguida |
| `/neoswitch:off` | Desativa o roteamento para novas sessões neste projeto |
| `/neoswitch:off all` | Desativa o roteamento em todos os projetos que o NeoSwitch ativou; execute antes de desinstalar |

Depois de alterar o roteamento, inicie uma nova sessão do Claude Code. A sessão atual mantém a conexão existente.

## Compatibilidade

O Claude Code continua executando a sessão, as permissões, a execução de ferramentas, os servidores MCP, as skills, os hooks, o `CLAUDE.md` e o `/resume`; o NeoSwitch só substitui o modelo por trás deles. O quanto esses recursos funcionam bem depende de quão bem o modelo segue o protocolo de ferramentas do Claude Code, que foi exercitado no uso diário de programação. Uma suíte automatizada de ponta a ponta executa a cada mudança a CLI real do Claude Code através do NeoSwitch contra um servidor Gemini roteirizado e verifica que chamadas de ferramentas e seus resultados, edições de arquivos, Bash, chamadas paralelas, permissões negadas, hooks, skills, `/resume`, subagentes e ferramentas MCP passam intactos pela rota da API do Gemini; ela não mede quão bem um modelo real os usa.

| Recurso | Gemini API / Vertex AI | Antigravity OAuth (ACP) |
|---|---|---|
| Streaming de texto | Suportado | Suportado |
| Chamadas de ferramentas, inclusive paralelas | Function calling nativo do Gemini | Parcial: o modelo escreve o JSON de tool-call do Claude, que chega inteiro em vez de transmitido |
| Uso de ferramentas em vários turnos e `/resume` | Suportado; as thought signatures são levadas nos IDs das ferramentas | Suportado |
| Imagens | Suportado | Suportado quando o `agy` oferece entrada de imagem |
| PDFs | Suportado | Não suportado; o modelo é informado de que um arquivo foi omitido |
| `/effort` | Enviado como nível de pensamento do Gemini; não verificado na API real, e descartado se o modelo o rejeitar | Mapeado para o nível de pensamento do `agy` |
| Uso de tokens e `count_tokens` | Informado; `count_tokens` é estimado na Developer API | Informado, incluindo cerca de 14k tokens de instruções do `agy` adicionados a cada prompt |
| Acesso ao Gemini 4 Neon | Depende da sua chave ou projeto; verifique com `neoswitch doctor` | Verificado com um `agy` conectado |

Não suportado em nenhum dos caminhos: ferramentas de servidor como busca na web e execução de código, fontes de imagem e documento por URL e blocos de thinking do Claude. `cache_control` e `context_management` são ignorados. O NeoSwitch nunca troca silenciosamente para outro modelo.

O contrato completo do endpoint está em [docs/protocol-contract.md](../docs/protocol-contract.md), e os detalhes do ACP estão em [docs/antigravity.md](../docs/antigravity.md).

## Como o plugin funciona

`/neoswitch:on` grava apenas três chaves no arquivo `.claude/settings.local.json` do projeto: `ANTHROPIC_BASE_URL` e `ANTHROPIC_AUTH_TOKEN` para o proxy, e `CLAUDE_CODE_USE_GATEWAY=0`, porque no modo gateway, ativado por lançadores de gateway, o Claude Code usaria a URL base do ambiente e ignoraria o proxy sem avisar. O token autoriza o proxy de loopback e não é uma credencial do Antigravity.

O hook SessionStart inicia um daemon local e registra o processo do Claude Code antes da primeira requisição ao modelo. O daemon é encerrado depois que todos os processos do Claude Code registrados terminam.

Na rota OAuth, o daemon inicia o subprocesso oficial `agy acp`. O NeoSwitch envia a requisição traduzida por uma sessão ACP e converte a resposta, ou o JSON de tool-call, de volta para o endpoint local compatível com a Anthropic. Um único processo `agy` atende todas as sessões, e as requisições são executadas em paralelo em sessões ACP separadas. As ferramentas próprias de arquivo, shell e web do `agy` são recusadas em toda requisição, então as ferramentas rodam apenas no Claude Code, sob as regras de permissão do Claude Code.

`/neoswitch:login` salva o Antigravity OAuth como backend, então uma chave do Gemini exportada para outras ferramentas não o substitui. Sem uma escolha salva, o NeoSwitch usa as credenciais do Gemini presentes no ambiente e, se não houver, o Antigravity OAuth. `NEOSWITCH_BACKEND=antigravity|gemini|openai|auto` substitui ambos, e `/neoswitch:status` mostra o backend em uso e o motivo. `/neoswitch:key` salva a API do Gemini como backend, então substitui um `/neoswitch:login` anterior.

## CLI independente

Cada versão com tag publica binários para macOS (Apple Silicon e Intel), Linux (amd64 e arm64) e Windows (amd64), com checksums SHA-256, na página [Releases](https://github.com/yldm-ai/neo-switch/releases). O build para Windows é compilado, mas ainda não foi testado. Você também pode compilar a partir do código-fonte:

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

Comandos comuns:

```bash
neoswitch doctor              # verifica a CLI do Claude e se as credenciais podem usar o modelo
neoswitch antigravity login   # login ACP OAuth pelo cliente oficial agy
neoswitch antigravity probe   # verifica a sessão salva do agy sem exibir credenciais
neoswitch models              # lista os modelos que as credenciais podem usar e marca o configurado
neoswitch on [project]        # ativa o roteamento de um projeto
neoswitch key [project]       # cola uma chave da API do Gemini em uma página local do navegador e ativa o projeto
neoswitch off [project]       # desativa o roteamento de um projeto
neoswitch off all             # desativa o roteamento em todos os projetos que o NeoSwitch ativou
neoswitch status [-v] [project] # diz se um projeto usa o NeoSwitch e o que fazer (-v adiciona detalhes)
neoswitch version             # exibe a versão do NeoSwitch
```

Para executar o proxy local para outro cliente compatível com a Anthropic:

```bash
export NEOSWITCH_AUTH_TOKEN="$(openssl rand -hex 24)"
GEMINI_API_KEY='your-key' neoswitch serve
```

Defina `ANTHROPIC_BASE_URL` e `ANTHROPIC_AUTH_TOKEN` no cliente. `neoswitch claude` configura essas variáveis automaticamente com uma porta e um token temporários.

## Atualizar e desinstalar

### Atualizar

Atualize o plugin com os comandos de [Instalar o plugin](#instalar-o-plugin). Para a CLI independente, baixe um binário mais novo na página [Releases](https://github.com/yldm-ai/neo-switch/releases) ou instale a versão mais recente com Go:

```bash
go install github.com/yldm-ai/neo-switch/cmd/neoswitch@latest
```

O repositório mudou de `yldm-ai/NeoSwitch` / `yldm-ai/neon-switch` para `yldm-ai/neo-switch`. Se você adicionou o marketplace com o nome antigo, adicione-o de novo a partir do novo local:

```bash
claude plugin marketplace add yldm-ai/neo-switch
```

### Desinstalar

1. Antes de desinstalar, execute `/neoswitch:off all` (ou `neoswitch off all`). Ele desativa o roteamento em todos os projetos que o NeoSwitch ativou e os lista. Projetos ativados com uma versão do NeoSwitch anterior ao `off all` não estão nessa lista; execute `/neoswitch:off` em cada um deles. Desinstalar o plugin não altera os arquivos do projeto: um projeto que continuar ativado mantém `ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN` e `CLAUDE_CODE_USE_GATEWAY` no objeto `env` do seu `.claude/settings.local.json`, e novas sessões ali tentam se conectar a um proxy que não inicia mais. Se o plugin já foi removido, apague essas três entradas manualmente.
2. Feche as sessões do Claude Code que usam o NeoSwitch e então remova o plugin e o marketplace pelo terminal:

```bash
claude plugin uninstall neoswitch@neoswitch
claude plugin marketplace remove neoswitch
```

3. Apague os arquivos que o NeoSwitch mantém fora dos seus projetos:

| O quê | Local |
|---|---|
| Binário do plugin compilado no primeiro uso | `$XDG_CACHE_HOME/neoswitch/`, ou `~/.cache/neoswitch/` quando `XDG_CACHE_HOME` não está definida |
| Diretório de estado: `proxy.json` (endereço do proxy, token, backend salvo), `gemini-api-key` (a chave salva por `/neoswitch:key`), `projects.json` (os projetos que `/neoswitch:off all` restaura) e `daemon.log` | `NEOSWITCH_STATE_DIR` quando definida; caso contrário, `~/Library/Application Support/neoswitch/` no macOS e `$XDG_CONFIG_HOME/neoswitch/` ou `~/.config/neoswitch/` no Linux |

O login do Antigravity é guardado pelo `agy`, não pelo NeoSwitch, e continua lá. Para remover uma CLI independente instalada com `go install`, apague `neoswitch` de `$(go env GOBIN)`, ou de `$(go env GOPATH)/bin` quando `GOBIN` não está definida.

## Solução de problemas

| Sintoma | Solução |
|---|---|
| Não sabe se as credenciais funcionam | Execute `neoswitch doctor`; ele indica a etapa que falhou e a causa. |
| `/neoswitch:login` não é reconhecido | Atualize o plugin, reinicie o Claude Code e execute `/reload-plugins`. |
| `agy: command not found` | Instale a CLI oficial do Antigravity ou defina `ANTIGRAVITY_CLI_PATH`. |
| `no authorization code was entered` | Atualize para a versão mais recente do plugin. O fluxo atual usa ACP `authenticate` e não lê a stdin do shell. |
| `ECONNREFUSED` em uma nova sessão | Execute `/neoswitch:status`. Se a rota estiver ativa, verifique a mensagem do daemon e reinicie a sessão. Execute `/neoswitch:off` a partir de uma sessão que use a Anthropic e ative novamente depois de corrigir a credencial. |
| A sessão informa que nenhum backend está configurado | Execute `/neoswitch:key` para usar uma chave da API do Gemini, ou `/neoswitch:login` para entrar com o Google. Na CLI independente, exporte `GEMINI_API_KEY` ou as configurações do Vertex AI antes de iniciar o Claude Code. |
| A compilação ou o download do plugin falha | Instale o Go e reinicie o Claude Code; com Go o plugin compila a partir do código-fonte e não baixa nada. Sem Go, verifique se o GitHub Releases está acessível. Um checksum divergente significa que o download foi recusado e nada foi executado. |

## Limites de compatibilidade

O NeoSwitch usa o SDK oficial do Gemini ou o processo local `agy acp` sem modificações. Ele não extrai tokens OAuth, não chama APIs HTTP privadas do Antigravity, não repassa credenciais para um proxy remoto e não modifica o arquivo hosts.

A Anthropic não documenta o roteamento do Claude Code para modelos que não sejam Claude, então atualizações do Claude Code podem alterar comportamentos dos quais o NeoSwitch depende.

## Roadmap

- Mais provedores de modelos. A OpenAI está disponível como backend experimental e ainda precisa ser verificada contra a API real. Provedores que já oferecem um endpoint compatível com a Anthropic podem ser usados definindo `ANTHROPIC_BASE_URL` diretamente e não precisam do NeoSwitch.

## Desenvolvimento

```bash
go test ./...
go test -race ./...
go vet ./...
```

Para publicar uma release, atualize `VERSION` e envie a tag correspondente (`v` seguido do conteúdo, como `v0.1.0`); o workflow de release recusa uma tag que não corresponda.

Veja [CONTRIBUTING.md](../CONTRIBUTING.md) para o fluxo de contribuição, [SECURITY.md](../SECURITY.md) para relatar uma vulnerabilidade de forma privada e [CODE_OF_CONDUCT.md](../CODE_OF_CONDUCT.md) para o código de conduta.

MIT License
