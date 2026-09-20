# FirstGreen CI

**Donne-nous ton projet, on te montre un pipeline CI vert en 10 minutes, sur ton ordinateur, sans écrire une ligne de YAML.**

*Give us your project, we will show you a green CI pipeline in 10 minutes, on your own computer, without writing a single line of YAML.*

---

## Français

### Ce que fait l'outil

```
$ firstgreenci init
Projet détecté : Python (pytest, requirements.txt)
Fichier créé   : .github/workflows/ci.yml

$ firstgreenci run
✓ Récupérer le code                moins d'une seconde
✓ Installer Python                 moins d'une seconde
✓ Installer les dépendances        1 s
✗ Lancer les tests                 moins d'une seconde

Votre pipeline est rouge.

  Cause  : Un test a échoué : tests/test_addition.py::test_addition
  Action : Corrigez ce test, ou le code qu'il vérifie, puis relancez : firstgreenci run
```

Une cause et une action. Jamais un mur de logs.

### Prérequis

- **Docker**, démarré. `firstgreenci doctor` vérifie et explique quoi faire.
- **act**, qui exécute les workflows GitHub Actions en local : `brew install act`.
- **Go 1.23** ou plus récent, tant qu'il n'y a pas d'exécutable téléchargeable.

### Installation

```bash
go install github.com/Dhafer84/firstgreenci/cmd/firstgreenci@latest
```

Ou, depuis une copie du dépôt :

```bash
go build -o firstgreenci ./cmd/firstgreenci
```

### Commandes

| Commande | Ce qu'elle fait |
| --- | --- |
| `firstgreenci init` | Analyse le projet et crée `.github/workflows/ci.yml`, commenté dans votre langue. |
| `firstgreenci run` | Exécute ce pipeline sur votre ordinateur et affiche un résultat par étape. |
| `firstgreenci doctor` | Vérifie Docker et act, et explique pas à pas ce qui manque. |

Options de `init` : `--lang fr|en`, `--project-type python|javascript`, `--force`, `--dry-run`.
Options de `run` : `--lang fr|en`, `--verbose`, `--image <référence>`.

Un dossier peut être donné en argument : `firstgreenci run chemin/vers/le/projet`.

### Au premier lancement

`run` demande une fois dans quelle image de conteneur exécuter votre pipeline :

- **Fidèle à GitHub** (~1,2 Go) : Python et Node déjà installés, votre pipeline se comporte comme sur GitHub.
- **Légère** (~200 Mo) : plus rapide à obtenir, mais sans Python.

La réponse est retenue dans un petit fichier JSON, dans le dossier de configuration de votre système. `--image` la contourne pour un lancement. Aucun secret n'y est jamais écrit.

### Ce qui est reconnu

| Langage | Gestionnaire | Reconnu grâce à | Tests |
| --- | --- | --- | --- |
| Python | pip | `requirements.txt`, `pyproject.toml`, `setup.py` | `pytest` s'il est déclaré, sinon `unittest` |
| Python | Poetry | `[tool.poetry]` dans `pyproject.toml` | `poetry run pytest` |
| Python | Pipenv | `Pipfile` | `pipenv run pytest` |
| JavaScript | npm | `package-lock.json` | la commande `test` de `package.json` |
| JavaScript | Yarn | `yarn.lock` | idem |
| JavaScript | pnpm | `pnpm-lock.yaml` | idem |

La version du langage est lue dans `.python-version`, `requires-python`, `.nvmrc` ou `engines.node`. À défaut, Python 3.12 et Node.js 20.

### Vos fichiers vous appartiennent

- Un fichier existant n'est **jamais** remplacé sans votre accord explicite.
- `run` n'écrit aucun fichier. Sans pipeline, il vous dit de lancer `init`.
- Votre `~/.actrc` n'est jamais modifié : l'image est toujours passée explicitement à act.
- Votre code ne quitte pas votre machine.
- Le fichier produit est ordinaire et commenté : il fonctionne si vous désinstallez FirstGreen CI.

### Écarts connus avec GitHub Actions

L'exécution locale ne peut pas être identique à celle de GitHub. Les écarts connus :

- **Architecture du processeur.** Sur un Mac Apple Silicon, le conteneur tourne en arm64, alors que GitHub exécute en amd64. C'est beaucoup plus rapide, mais un paquet qui n'existe qu'en amd64 se comportera différemment.
- **Image du conteneur.** `catthehacker/ubuntu` n'est pas l'image de GitHub. Des outils présents chez GitHub peuvent y manquer.
- **Cache, services et matrices** ne sont pas reproduits à l'identique par act.
- **Une question au premier lancement.** L'outil évite les questions, mais le choix de l'image engage plus d'un gigaoctet de téléchargement : il ne se devine pas.
- **Langue du système sous Windows** : `LANG` et `LC_ALL` y sont généralement absentes, l'anglais est donc choisi par défaut. Utilisez `--lang fr` ou `FIRSTGREENCI_LANG`.
- **Lecture de `pyproject.toml`** : balayage ligne par ligne, sans analyseur TOML, pour éviter toute dépendance externe.

### Une erreur que l'outil ne sait pas traduire ?

Il le dit franchement, affiche les dernières lignes du journal et vous invite à la signaler. C'est ainsi que la liste des traductions s'allonge : [ouvrez un ticket](https://github.com/Dhafer84/firstgreenci/issues).

### Contribuer

Les modèles de pipeline (`templates/`) et les traductions (`locales/`) sont séparés du code : une contribution n'exige pas de toucher au moteur Go.

```bash
go build ./... && go vet ./... && gofmt -l . && go test ./...
```

Les tests rejouent de vraies exécutions d'act enregistrées dans `testdata/act/`. Ils n'ont besoin ni de Docker, ni d'act, ni du réseau.

Après avoir modifié un modèle ou une traduction :

```bash
go test ./internal/generate -update
```

Licence MIT.

---

## English

### What it does

```
$ firstgreenci init
Project detected: Python (pytest, requirements.txt)
File created   : .github/workflows/ci.yml

$ firstgreenci run
✓ Get the code                     under a second
✓ Install Python                   under a second
✓ Install the dependencies         1 s
✗ Run the tests                    under a second

Your pipeline is red.

  Cause : A test failed: tests/test_addition.py::test_addition
  Action: Fix that test, or the code it checks, then run again: firstgreenci run
```

A cause and an action. Never a wall of logs.

### Requirements

- **Docker**, started. `firstgreenci doctor` checks it and explains what to do.
- **act**, which runs GitHub Actions workflows locally: `brew install act`.
- **Go 1.23** or newer, until downloadable binaries exist.

### Install

```bash
go install github.com/Dhafer84/firstgreenci/cmd/firstgreenci@latest
```

### Commands

| Command | What it does |
| --- | --- |
| `firstgreenci init` | Look at the project and create `.github/workflows/ci.yml`, commented in your language. |
| `firstgreenci run` | Run that pipeline on your own computer and show a result per step. |
| `firstgreenci doctor` | Check Docker and act, and explain step by step what is missing. |

Options for `init`: `--lang fr|en`, `--project-type python|javascript`, `--force`, `--dry-run`.
Options for `run`: `--lang fr|en`, `--verbose`, `--image <reference>`.

### On the first run

`run` asks once which container image to run your pipeline in: faithful to GitHub (~1.2 GB, Python and Node preinstalled) or light (~200 MB, no Python). The answer is remembered in a small JSON file in your system's configuration folder. `--image` overrides it for one run. No secret is ever written there.

### Your files stay yours

- An existing file is **never** replaced without your explicit consent.
- `run` writes no file. Without a pipeline, it tells you to run `init`.
- Your `~/.actrc` is never touched: the image is always passed to act explicitly.
- Your code stays on your machine.
- The generated file is an ordinary, commented workflow: it keeps working if you uninstall FirstGreen CI.

### Known differences from GitHub Actions

- **Processor architecture.** On an Apple Silicon Mac the container runs on arm64, while GitHub runs amd64. Much faster, but a package that only exists for amd64 will behave differently.
- **Container image.** `catthehacker/ubuntu` is not GitHub's image; tools present on GitHub may be missing.
- **Cache, services and matrices** are not reproduced exactly by act.
- **One question on the first run.** The tool avoids questions, but choosing the image commits more than a gigabyte of download, and cannot be guessed for you.
- **System language on Windows**: `LANG` and `LC_ALL` are usually absent there, so English is chosen by default. Use `--lang fr` or `FIRSTGREENCI_LANG`.
- **Reading `pyproject.toml`**: scanned line by line, without a TOML parser, to avoid an external dependency.

### An error it cannot translate?

It says so plainly, shows the last lines of the log, and invites you to report it. That is how the list of translations grows: [open an issue](https://github.com/Dhafer84/firstgreenci/issues).

### Contributing

Pipeline templates (`templates/`) and translations (`locales/`) sit outside the engine on purpose. The tests replay real act runs recorded in `testdata/act/`, so they need neither Docker, nor act, nor the network.

```bash
go build ./... && go vet ./... && gofmt -l . && go test ./...
```

MIT licence.
