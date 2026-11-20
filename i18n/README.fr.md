# NeoSwitch

[English](../README.md) | [简体中文](README.zh-CN.md) | [繁體中文](README.zh-TW.md) | [日本語](README.ja.md) | [한국어](README.ko.md) | [Español](README.es.md) | **Français** | [Deutsch](README.de.md) | [Português (Brasil)](README.pt-BR.md) | [Русский](README.ru.md)

[![CI](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml/badge.svg)](https://github.com/yldm-ai/NeoSwitch/actions/workflows/test.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/yldm-ai/NeoSwitch.svg)](https://pkg.go.dev/github.com/yldm-ai/NeoSwitch) [![License](https://img.shields.io/github/license/yldm-ai/NeoSwitch)](https://github.com/yldm-ai/NeoSwitch/blob/main/LICENSE)

NeoSwitch est un plugin Claude Code qui achemine les projets vers Gemini 4 Neon via un proxy local compatible Anthropic. Vous pouvez choisir Antigravity OAuth, la Gemini Developer API ou Vertex AI tout en conservant le flux de travail, le streaming et l'utilisation des outils de Claude Code.

Deux chemins d'identifiants vers le service amont sont pris en charge :

- Antigravity OAuth via le processus local officiel `agy acp`.
- Des identifiants Gemini Developer API ou Vertex AI via le SDK officiel `google.golang.org/genai`.

Pour Claude Code, le flux du plugin avec Antigravity OAuth est recommandé. Les identifiants OAuth restent dans `agy` ; NeoSwitch ne lit ni n'exporte le jeton.

## Utiliser Claude Code avec Neon

### Prérequis

- Claude Code.
- La CLI officielle Antigravity, disponible sous le nom `agy` dans votre PATH.
- Go. Le plugin compile le binaire NeoSwitch fourni lors de la première utilisation.

### Installer le plugin

Exécutez ces commandes dans Claude Code :

```text
/plugin marketplace add yldm-ai/NeoSwitch
/plugin install neoswitch@neoswitch
```

Si le plugin est déjà installé, mettez-le à jour depuis un terminal après une nouvelle version :

```bash
claude plugin marketplace update neoswitch
claude plugin update neoswitch@neoswitch
```

Redémarrez Claude Code après la mise à jour. Le flux de connexion ACP nécessite la version actuelle du plugin.

### Se connecter et activer Neon

Dans le projet où vous voulez utiliser Neon, exécutez :

```text
/neoswitch:login
```

La commande effectue les étapes suivantes :

1. Démarre le flux d'authentification ACP géré par `agy`.
2. Ouvre le navigateur lorsque Google OAuth est requis.
3. Active le routage NeoSwitch pour le projet courant.
4. Sélectionne `gemini-4-neon` pour la session ACP.

Une fois la commande terminée, fermez puis rouvrez Claude Code dans ce projet. Vérifiez la route avec :

```text
/neoswitch:status
```

Le statut doit afficher `NeoSwitch ... -> gemini-4-neon`. Claude Code peut continuer d'afficher le libellé demandé, par exemple `claude-opus-5-5`, dans l'en-tête de la session. La route amont réelle est indiquée par `/neoswitch:status`.

`/neoswitch:antigravity-login` est un alias de `/neoswitch:login`. `/neoswitch:on` et `/neoswitch:off` restent disponibles pour modifier le routage sans vous connecter.

## Autres chemins d'identifiants

### Clé API Gemini

```bash
export GEMINI_API_KEY='your-key'
go run ./cmd/neoswitch claude
```

Le plugin peut aussi recevoir la clé via `/plugin` → `neoswitch` → **Gemini API key**. Claude Code stocke la clé dans sa configuration sécurisée des plugins.

### Vertex AI

```bash
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT=my-project
export GOOGLE_CLOUD_LOCATION=global
go run ./cmd/neoswitch claude
```

Les Application Default Credentials du Google Cloud SDK doivent déjà être accessibles au processus.

## Commandes du plugin

| Commande | Effet |
|---|---|
| `/neoswitch:login` | S'authentifie avec Antigravity OAuth, active la route Neon de ce projet et utilise ACP |
| `/neoswitch:on` | Active le routage pour les nouvelles sessions Claude Code de ce projet |
| `/neoswitch:status` | Affiche la route du projet et l'état du daemon |
| `/neoswitch:off` | Désactive le routage pour les nouvelles sessions de ce projet |

Après avoir modifié le routage, démarrez une nouvelle session Claude Code. La session en cours conserve sa connexion existante.

## Fonctionnement du plugin

`/neoswitch:on` écrit uniquement `ANTHROPIC_BASE_URL` et `ANTHROPIC_AUTH_TOKEN` dans le fichier `.claude/settings.local.json` du projet. Le jeton autorise le proxy de bouclage et n'est pas un identifiant Antigravity.

Le hook SessionStart démarre un daemon local et enregistre le processus Claude Code avant sa première requête au modèle. Le daemon s'arrête lorsque tous les processus Claude Code enregistrés se sont terminés. Par défaut, le proxy n'écoute que sur l'interface de bouclage.

Avec la route OAuth, le daemon lance le sous-processus officiel `agy acp`. NeoSwitch envoie la requête traduite via une session ACP et reconvertit la réponse, ou le JSON de tool-call compatible Claude, vers le endpoint local compatible Anthropic. Les identifiants OAuth et les rappels du navigateur restent gérés par `agy`.

Un seul processus `agy` sert toutes les sessions, et les requêtes s'exécutent en parallèle dans des sessions ACP distinctes. Le texte est diffusé au fil de sa génération, la consommation de jetons est remontée à Claude Code, et le réglage `/effort` de Claude Code détermine le temps de réflexion de Neon.

Le réglage facultatif du plugin **Use Antigravity OAuth via ACP** force la route ACP lorsqu'un autre identifiant Gemini est également présent. Un `/neoswitch:login` normal utilise automatiquement ACP si aucun identifiant Gemini n'est configuré.

## CLI autonome

Compilez le binaire ou exécutez-le depuis le dépôt :

```bash
go build -o neoswitch ./cmd/neoswitch
go install ./cmd/neoswitch
```

Commandes courantes :

```bash
neoswitch antigravity login   # connexion ACP OAuth via le client officiel agy
neoswitch antigravity probe   # vérifie la session agy enregistrée sans afficher les identifiants
neoswitch doctor              # vérifie le backend sélectionné et la CLI Claude
neoswitch models              # affiche le modèle configuré
neoswitch on [project]        # active le routage d'un projet
neoswitch off [project]       # désactive le routage d'un projet
neoswitch status [project]    # affiche le routage et l'état du daemon
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
| `/neoswitch:login` est inconnu | Mettez à jour le plugin, redémarrez Claude Code et exécutez `/reload-plugins`. |
| `agy: command not found` | Installez la CLI officielle Antigravity ou définissez `ANTIGRAVITY_CLI_PATH`. |
| `no authorization code was entered` | Mettez à jour vers la dernière version du plugin. Le flux actuel utilise ACP `authenticate` et ne lit pas la stdin du shell. |
| `ECONNREFUSED` dans une nouvelle session | Exécutez `/neoswitch:status`. Si la route est active, consultez le message du daemon et redémarrez la session. Exécutez `/neoswitch:off` depuis une session qui utilise Anthropic, puis réactivez-le après avoir corrigé l'identifiant. |
| La session démarre sans identifiants Gemini | Exécutez `/neoswitch:login`, ou configurez une clé API Gemini ou Vertex AI ADC. |
| La compilation du plugin échoue | Installez Go et redémarrez Claude Code. |

## Limites de compatibilité

NeoSwitch utilise le SDK officiel Gemini ou le processus local `agy acp` non modifié. Il n'extrait pas de jetons OAuth, n'appelle pas d'API HTTP privées d'Antigravity, ne relaie pas d'identifiants vers un proxy distant, ne modifie pas le fichier hosts et ne bascule jamais silencieusement vers un autre modèle.

Claude Code reste responsable de la session, des autorisations, de MCP et de la boucle d'outils. Anthropic ne documente pas le routage de Claude Code vers des modèles autres que Claude ; les mises à jour de Claude Code peuvent donc modifier des comportements dont dépend NeoSwitch.

Consultez [docs/protocol-contract.md](../docs/protocol-contract.md) pour le contrat du endpoint et la matrice de compatibilité, et [docs/antigravity.md](../docs/antigravity.md) pour les détails de l'authentification ACP.

## Développement

```bash
go test ./...
go test -race ./...
go vet ./...
```

MIT License
