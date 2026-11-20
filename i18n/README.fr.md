# NeoSwitch

[English](../README.md) | [简体中文](README.zh-CN.md) | [繁體中文](README.zh-TW.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | **Français** | [Deutsch](README.de.md) | [Português (Brasil)](README.pt-BR.md) | [Русский](README.ru.md)

[![CI](https://github.com/yldm-ai/neon-switch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/neon-switch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/neon-switch.svg)](https://pkg.go.dev/github.com/yldm-ai/neon-switch) [![Go version](https://img.shields.io/github/go-mod/go-version/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/blob/main/go.mod) [![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://github.com/yldm-ai/neon-switch/blob/main/LICENSE) [![Last commit](https://img.shields.io/github/last-commit/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/commits/main) [![GitHub stars](https://img.shields.io/github/stars/yldm-ai/neon-switch)](https://github.com/yldm-ai/neon-switch/stargazers)

[![Claude Code plugin](https://img.shields.io/badge/Claude%20Code-plugin-D97757)](https://github.com/yldm-ai/neon-switch/blob/main/.claude-plugin/plugin.json) [![Model: Gemini 4 Neon](https://img.shields.io/badge/model-Gemini%204%20Neon-4285F4)](https://github.com/yldm-ai/neon-switch/blob/main/docs/protocol-contract.md) [![Platform: macOS and Linux](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-lightgrey)](https://github.com/yldm-ai/neon-switch/releases) [![Proxy: local-only](https://img.shields.io/badge/proxy-local--only-2ea44f)](https://github.com/yldm-ai/neon-switch/blob/main/docs/protocol-contract.md)

**Keep Claude Code. Switch your models.**

NeoSwitch vous permet d'utiliser Gemini avec Claude Code sans changer le flux de travail que vous connaissez déjà. Claude Code conserve sa CLI, ses commandes, ses autorisations, ses outils, ses skills, ses serveurs MCP et ses hooks ; NeoSwitch exécute un petit proxy local qui traduit ses requêtes pour le modèle qui se trouve derrière.

Aujourd'hui, NeoSwitch achemine Claude Code vers Gemini 4 Neon. OpenAI est disponible comme backend expérimental : il doit être activé explicitement et n'a été testé que contre un upstream scripté.

## Pourquoi NeoSwitch

- **Conservez votre flux de travail** : vous utilisez toujours Claude Code. Seul le modèle change.
- **Par projet** : activez le routage dans un projet et laissez les autres sur Anthropic.
- **Local d'abord** : le proxy n'écoute que sur l'interface de bouclage, utilise un jeton d'accès aléatoire et ne journalise ni prompts, ni réponses, ni identifiants. Aucun relais distant n'intervient.
- **Compatibilité sans détour** : ce qui fonctionne, ce qui fonctionne partiellement et ce qui manque est détaillé [plus bas](#compatibilité).

## Choisir un chemin d'identifiants

| Chemin | Mode de connexion | Remarques |
|---|---|---|
| Antigravity OAuth | Compte Google via la CLI officielle `agy` | Flux par défaut du plugin. Lisez d'abord l'avertissement ci-dessous. |
| Gemini Developer API | `GEMINI_API_KEY` | L'accès à Gemini 4 Neon dépend de votre clé. Vérifiez-le avec `neoswitch doctor`. |
| Vertex AI | Application Default Credentials de Google Cloud | L'accès dépend de votre projet et de votre région. Vérifiez-le avec `neoswitch doctor`. |

> [!WARNING]
> Les [Conditions d'utilisation de Google Antigravity](https://antigravity.google/terms) (section 6) considèrent l'accès à Antigravity via des logiciels, outils ou services tiers comme une infraction et indiquent que Google peut suspendre ou résilier les comptes Antigravity et Gemini CLI concernés. Acheminer Claude Code via Antigravity OAuth relève de cette règle. Ne l'utilisez que si vous acceptez ce risque pour le compte avec lequel vous vous connectez ; sinon, utilisez une clé API Gemini ou Vertex AI. `neoswitch doctor` et `neoswitch antigravity login` affichent le même avertissement.

Les identifiants OAuth restent toujours dans `agy`. NeoSwitch ne lit, ne copie ni n'exporte le jeton.

## Utiliser Claude Code avec Neon

### Prérequis

- Claude Code.
- Pour Antigravity OAuth : la CLI officielle Antigravity, disponible sous le nom `agy` dans votre PATH.
- Go. Le plugin compile le binaire NeoSwitch fourni lors de la première utilisation.

### Installer le plugin

Exécutez ces commandes dans Claude Code :

```text
/plugin marketplace add yldm-ai/neon-switch
/plugin install neoswitch@neoswitch
```

Si le plugin est déjà installé, mettez-le à jour depuis un terminal après une nouvelle version :

```bash
claude plugin marketplace update neoswitch
claude plugin update neoswitch@neoswitch
```

Redémarrez Claude Code après la mise à jour.

### Se connecter et activer Neon

Dans le projet où vous voulez utiliser Neon, exécutez :

```text
/neoswitch:login
```

La commande effectue les étapes suivantes :

1. Lance le flux d'authentification ACP géré par `agy`.
2. Ouvre le navigateur lorsque Google OAuth est requis.
3. Active le routage NeoSwitch pour le projet courant.
4. Sélectionne `gemini-4-neon` pour la session ACP.

Pour utiliser plutôt une clé API Gemini, exportez `GEMINI_API_KEY` avant de lancer Claude Code et exécutez `/neoswitch:on` ; le plugin n'a aucun réglage à remplir. Si vous vous êtes connecté auparavant avec `/neoswitch:login`, exportez aussi `NEOSWITCH_BACKEND=gemini`, sinon le choix enregistré à la connexion l'emporte.

Fermez puis rouvrez Claude Code dans ce projet. Vérifiez la route avec :

```text
/neoswitch:status
```

L'état doit afficher `NeoSwitch ... -> gemini-4-neon`. Claude Code peut continuer d'afficher le nom de modèle demandé, par exemple `claude-opus-5-5`, dans l'en-tête de session ; la route réellement utilisée vers le service amont est celle qu'affiche `/neoswitch:status`.

`/neoswitch:antigravity-login` est un alias de `/neoswitch:login`. `/neoswitch:on` et `/neoswitch:off` modifient le routage sans connexion.

## Autres chemins d'identifiants

### Clé API Gemini

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

Les Application Default Credentials du Google Cloud SDK doivent déjà être disponibles pour le processus.

`neoswitch doctor` recherche le modèle et lance une génération d'une ligne, puis indique si les identifiants fonctionnent ou pourquoi ils ne fonctionnent pas : identifiants refusés, autorisation manquante, restriction de région, modèle inconnu, quota ou service indisponible.

### OpenAI (expérimental)

Le backend OpenAI envoie les requêtes à l'API Responses d'OpenAI via le SDK officiel `openai-go`. Il doit être activé explicitement : la sélection automatique ne le choisit jamais, donc définissez `NEOSWITCH_BACKEND=openai` ou exécutez `neoswitch backend openai`. `OPENAI_MODEL` est obligatoire ; NeoSwitch ne choisit pas de modèle à votre place. `OPENAI_BASE_URL` est facultatif et doit inclure `/v1`. Il n'a été testé que contre un upstream scripté, pas avec un vrai compte OpenAI. Ce qui est traduit et ce qui ne l'est pas est détaillé dans [docs/protocol-contract.md](../docs/protocol-contract.md#openai-backend-experimental).

```bash
export NEOSWITCH_BACKEND=openai
export OPENAI_API_KEY='your-key'
export OPENAI_MODEL='model-name'
neoswitch doctor
go run ./cmd/neoswitch claude
```

## Commandes du plugin

| Commande | Effet |
|---|---|
| `/neoswitch:login` | S'authentifie avec Antigravity OAuth, active la route Neon de ce projet et utilise ACP |
| `/neoswitch:on` | Active le routage pour les nouvelles sessions Claude Code de ce projet |
| `/neoswitch:status` | Affiche la route du projet et l'état du daemon |
| `/neoswitch:off` | Désactive le routage pour les nouvelles sessions de ce projet |

Après avoir modifié le routage, démarrez une nouvelle session Claude Code. La session en cours conserve sa connexion existante.

## Compatibilité

Claude Code continue d'exécuter la session, les autorisations, l'exécution des outils, les serveurs MCP, les skills, les hooks, `CLAUDE.md` et `/resume` ; NeoSwitch remplace uniquement le modèle qui se trouve derrière. Le bon fonctionnement de ces fonctionnalités dépend de la manière dont le modèle suit le protocole d'outils de Claude Code, qui a été éprouvé au quotidien pour du développement mais n'est pas couvert par une suite automatisée de bout en bout.

| Fonctionnalité | Gemini API / Vertex AI | Antigravity OAuth (ACP) |
|---|---|---|
| Streaming de texte | Pris en charge | Pris en charge |
| Appels d'outils, y compris en parallèle | Function calling natif de Gemini | Partiel : le modèle écrit du JSON de tool-call Claude, qui arrive en entier au lieu d'être diffusé en streaming |
| Utilisation d'outils sur plusieurs tours et `/resume` | Pris en charge ; les thought signatures sont transportées dans les ID d'outils | Pris en charge |
| Images | Pris en charge | Pris en charge lorsque `agy` accepte les images en entrée |
| PDF | Pris en charge | Non pris en charge ; le modèle est informé qu'un fichier a été omis |
| `/effort` | Pas encore appliqué | Associé au niveau de réflexion de `agy` |
| Consommation de jetons et `count_tokens` | Remontée ; `count_tokens` est estimé sur la Developer API | Remontée, y compris environ 14k jetons d'instructions `agy` ajoutés à chaque prompt |
| Accès à Gemini 4 Neon | Dépend de votre clé ou de votre projet ; vérifiez avec `neoswitch doctor` | Vérifié avec un `agy` connecté |

Non pris en charge sur aucun des deux chemins : les outils serveur comme la recherche web et l'exécution de code, les sources d'images et de documents par URL, et les blocs de thinking de Claude. `cache_control` et `context_management` sont ignorés. NeoSwitch ne bascule jamais silencieusement vers un autre modèle.

Le contrat complet du endpoint se trouve dans [docs/protocol-contract.md](../docs/protocol-contract.md), et les détails ACP dans [docs/antigravity.md](../docs/antigravity.md).

## Fonctionnement du plugin

`/neoswitch:on` écrit uniquement `ANTHROPIC_BASE_URL` et `ANTHROPIC_AUTH_TOKEN` dans le fichier `.claude/settings.local.json` du projet. Le jeton autorise le proxy de bouclage et n'est pas un identifiant Antigravity.

Le hook SessionStart démarre un daemon local et enregistre le processus Claude Code avant sa première requête au modèle. Le daemon s'arrête lorsque tous les processus Claude Code enregistrés se sont terminés.

Avec la route OAuth, le daemon lance le sous-processus officiel `agy acp`. NeoSwitch envoie la requête traduite via une session ACP et reconvertit la réponse ou le JSON de tool-call renvoyé vers le endpoint local compatible Anthropic. Un seul processus `agy` sert toutes les sessions, et les requêtes s'exécutent en parallèle dans des sessions ACP distinctes. Les outils de fichiers, de shell et web propres à `agy` sont refusés à chaque requête : les outils ne s'exécutent donc que dans Claude Code, selon les règles d'autorisation de Claude Code.

`/neoswitch:login` enregistre Antigravity OAuth comme backend, si bien qu'une clé Gemini exportée pour d'autres outils ne le remplace pas. Sans choix enregistré, NeoSwitch utilise les identifiants Gemini présents dans l'environnement, sinon Antigravity OAuth. `NEOSWITCH_BACKEND=antigravity|gemini|openai|auto` prime sur les deux, et `/neoswitch:status` indique le backend utilisé et pourquoi.

## CLI autonome

Chaque version étiquetée publie des binaires pour macOS (Apple Silicon et Intel), Linux (amd64 et arm64) et Windows (amd64), avec des sommes de contrôle SHA-256, sur la page [Releases](https://github.com/yldm-ai/neon-switch/releases). La version Windows est compilée mais pas encore testée. Vous pouvez aussi compiler depuis les sources :

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

Commandes courantes :

```bash
neoswitch doctor              # vérifie la CLI Claude et si les identifiants peuvent utiliser le modèle
neoswitch antigravity login   # connexion ACP OAuth via le client officiel agy
neoswitch antigravity probe   # vérifie la session agy enregistrée sans afficher les identifiants
neoswitch models              # affiche le modèle configuré
neoswitch on [project]        # active le routage d'un projet
neoswitch off [project]       # désactive le routage d'un projet
neoswitch status [project]    # affiche le routage et l'état du daemon
neoswitch version             # affiche la version de NeoSwitch
```

Pour lancer le proxy local pour un autre client compatible Anthropic :

```bash
export NEOSWITCH_AUTH_TOKEN="$(openssl rand -hex 24)"
GEMINI_API_KEY='your-key' neoswitch serve
```

Définissez à la fois `ANTHROPIC_BASE_URL` et `ANTHROPIC_AUTH_TOKEN` dans le client. `neoswitch claude` configure automatiquement ces variables avec un port et un jeton éphémères.

## Dépannage

| Symptôme | Solution |
|---|---|
| Vous ne savez pas si les identifiants fonctionnent | Exécutez `neoswitch doctor` ; il indique l'étape en échec et la cause. |
| `/neoswitch:login` est inconnu | Mettez à jour le plugin, redémarrez Claude Code et exécutez `/reload-plugins`. |
| `agy: command not found` | Installez la CLI officielle Antigravity ou définissez `ANTIGRAVITY_CLI_PATH`. |
| `no authorization code was entered` | Mettez à jour vers la dernière version du plugin. Le flux actuel utilise ACP `authenticate` et ne lit pas la stdin du shell. |
| `ECONNREFUSED` dans une nouvelle session | Exécutez `/neoswitch:status`. Si la route est active, consultez le message du daemon et redémarrez la session. Exécutez `/neoswitch:off` depuis une session qui utilise Anthropic, puis réactivez-le après avoir corrigé l'identifiant. |
| La session indique qu'aucun backend n'est configuré | Exécutez `/neoswitch:login`, ou exportez `GEMINI_API_KEY` ou les réglages Vertex AI avant de lancer Claude Code. |
| La compilation du plugin échoue | Installez Go et redémarrez Claude Code. |

## Limites de compatibilité

NeoSwitch utilise le SDK officiel Gemini ou le processus local `agy acp` non modifié. Il n'extrait pas de jetons OAuth, n'appelle pas d'API HTTP privées d'Antigravity, ne relaie pas d'identifiants vers un proxy distant et ne modifie pas le fichier hosts.

Anthropic ne documente pas le routage de Claude Code vers des modèles autres que Claude ; les mises à jour de Claude Code peuvent donc modifier des comportements dont dépend NeoSwitch.

## Feuille de route

- Davantage de fournisseurs de modèles. OpenAI est disponible comme backend expérimental et doit encore être vérifié contre l'API réelle. Les fournisseurs qui proposent déjà un endpoint compatible Anthropic peuvent être utilisés en définissant directement `ANTHROPIC_BASE_URL` et n'ont pas besoin de NeoSwitch.
- Des binaires précompilés pour le plugin, afin qu'il n'ait plus besoin de Go.

## Développement

```bash
go test ./...
go test -race ./...
go vet ./...
```

MIT License
