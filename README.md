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

- **Docker**, démarré. C'est le seul prérequis que vous devez obtenir vous-même.
- **act**, qui exécute les workflows GitHub Actions en local. Les méthodes 1 et 2 ci-dessous l'installent pour vous.

Rien d'autre. L'exécutable ne dépend d'aucune bibliothèque système, et Go n'est pas nécessaire pour l'utiliser.

### Installation

Trois méthodes. Prenez celle qui correspond à votre système.

#### 1. macOS — avec Homebrew

```bash
brew install Dhafer84/tap/firstgreenci
```

`act` est installé en même temps. Il ne vous restera que Docker.

Pour mettre à jour plus tard :

```bash
brew update && brew upgrade firstgreenci
```

Le `brew update` n'est pas facultatif : Homebrew ne rafraîchit pas les dépôts tiers tout seul, et `brew upgrade` seul répondrait « already installed » même si une nouvelle version existe.

#### 2. Windows — avec Scoop

```powershell
scoop bucket add firstgreenci https://github.com/Dhafer84/homebrew-tap
scoop install firstgreenci
```

`act` est installé en même temps, là aussi. Le nom du dépôt parle de Homebrew : c'est normal, il héberge les recettes des deux systèmes.

#### 3. Linux, ou sans gestionnaire de paquets — téléchargement direct

Prenez le fichier qui correspond à votre machine sur la [page des versions](https://github.com/Dhafer84/firstgreenci/releases/latest) :

| Votre machine | Fichier à prendre |
| --- | --- |
| Mac Apple Silicon (M1 et suivants) | `..._macOS_AppleSilicon.tar.gz` |
| Mac Intel | `..._macOS_Intel.tar.gz` |
| Windows | `..._Windows_x86-64.zip` |
| Linux | `..._Linux_x86-64.tar.gz` |

**Sur macOS et Linux**, téléchargez avec `curl` plutôt qu'avec le navigateur, en adaptant la version et le nom du fichier :

```bash
curl -L -o firstgreenci.tar.gz https://github.com/Dhafer84/firstgreenci/releases/download/v0.1.2/firstgreenci_0.1.2_macOS_AppleSilicon.tar.gz
tar -xzf firstgreenci.tar.gz
sudo mv firstgreenci /usr/local/bin/
```

Pourquoi `curl` ? Parce qu'un fichier téléchargé par le navigateur est mis en quarantaine par macOS et refuse de se lancer. Si cela vous arrive :

```bash
xattr -d com.apple.quarantine firstgreenci
```

**Sur Windows**, décompressez le `.zip`, puis placez `firstgreenci.exe` dans un dossier de votre `PATH`. SmartScreen peut avertir à la première exécution : les exécutables ne sont pas signés, faute de certificat.

**Vérifiez ce que vous avez téléchargé.** Chaque version est accompagnée d'un `checksums.txt` :

```bash
shasum -a 256 -c checksums.txt --ignore-missing
```

Avec cette méthode, `act` n'est pas installé pour vous : `brew install act`, `scoop install act`, ou les [exécutables d'act](https://github.com/nektos/act/releases).

#### Puis, quelle que soit la méthode

```bash
firstgreenci doctor
```

Il vérifie Docker, act et l'image de conteneur, et explique pas à pas ce qui manque.

*Si vous avez déjà Go 1.23 ou plus récent, `go install github.com/Dhafer84/firstgreenci/cmd/firstgreenci@latest` fonctionne aussi.*

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
| Python | pip | `requirements-dev.txt`, s'il existe | installé aussi, pour garder la production sans outils de test |
| Python | Poetry | `[tool.poetry]` dans `pyproject.toml` | `poetry run pytest` |
| Python | Pipenv | `Pipfile` | `pipenv run pytest` |
| JavaScript | npm | `package-lock.json` | la commande `test` de `package.json` |
| JavaScript | Yarn | `yarn.lock` | idem |
| JavaScript | pnpm | `pnpm-lock.yaml` | idem |

La version du langage est lue dans `.python-version`, `requires-python`, `.nvmrc` ou `engines.node`. À défaut, Python 3.12 et Node.js 20 — et l'outil **le dit**, au lieu de laisser croire qu'il a repris la vôtre :

```
·  Votre projet ne fixe pas de version de Node.js ; le pipeline utilisera la 20.
   Pour la choisir : un fichier .nvmrc, ou « engines » dans package.json.
```

### Projets à plusieurs dossiers

GitHub ne lit les workflows que dans `.github/workflows/` **à la racine du dépôt**. Un fichier placé ailleurs s'exécute très bien sur votre machine et n'est jamais déclenché par GitHub.

L'outil vérifie donc où il écrit. Si vous le lancez dans un sous-dossier d'un dépôt, il **refuse** et vous donne la commande à lancer depuis la racine ; `--force` passe outre. Et si la racine ne contient aucun projet reconnaissable, il nomme les sous-dossiers qui en contiennent un :

```
Je n'ai pas reconnu de projet Python ou JavaScript dans …

J'ai trouvé ceci juste en dessous :
  firstgreenci init backend
  firstgreenci init frontend
```

### Il vous prévient avant de vous faire attendre

Si vos tests ne pourront pas être collectés par l'outil détecté, `init` le dit **immédiatement**, chiffres à l'appui, au lieu de vous laisser découvrir un pipeline rouge après plusieurs minutes d'exécution :

```
⚠  Vos tests ne seront pas trouvés par unittest.

     Fichiers de test          : 19
     Fonctions « test_ »       : 406
     Classes unittest.TestCase : 0
     Outil de test déclaré     : aucun
```

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
- **Cache partagé entre vos projets.** act garde un volume Docker `act-toolcache` réutilisé par toutes vos exécutions locales : un paquet installé par un projet reste disponible pour les suivants. **Un pipeline peut donc être vert chez vous et rouge sur GitHub**, qui repart d'une machine neuve à chaque fois. Pour vérifier sans filet : `docker volume rm act-toolcache` avant de relancer.
- **Cache, services et matrices** ne sont pas reproduits à l'identique par act.
- **Une question au premier lancement.** L'outil évite les questions, mais le choix de l'image engage plus d'un gigaoctet de téléchargement : il ne se devine pas.
- **Choix de la langue.** `LANG` et `LC_ALL` font foi quand elles existent, et elles sont presque toujours définies dans un terminal — y compris sur un Mac réglé en français, où macOS peut composer une valeur anglaise si votre région n'a pas de locale française. **En l'absence de ces variables seulement**, l'outil lit la langue d'interface de macOS. Sous Windows il ne la lit pas, car cela coûterait un démarrage de PowerShell à chaque commande. Dans tous les cas, `--lang fr` ou `FIRSTGREENCI_LANG=fr` tranche.
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

- **Docker**, started. It is the only prerequisite you have to get yourself.
- **act**, which runs GitHub Actions workflows locally. Methods 1 and 2 below install it for you.

Nothing else. The executable depends on no system library, and Go is not needed to use it.

### Install

Three methods. Take the one matching your system.

#### 1. macOS — with Homebrew

```bash
brew install Dhafer84/tap/firstgreenci
```

`act` is installed along with it. Only Docker is left to get.

To update later:

```bash
brew update && brew upgrade firstgreenci
```

The `brew update` is not optional: Homebrew does not refresh third-party taps on its own, and `brew upgrade` alone would answer "already installed" even when a new version exists.

#### 2. Windows — with Scoop

```powershell
scoop bucket add firstgreenci https://github.com/Dhafer84/homebrew-tap
scoop install firstgreenci
```

`act` is installed along with it here too. The repository name says Homebrew: that is expected, it holds the recipes for both systems.

#### 3. Linux, or without a package manager — direct download

Take the file matching your machine from the [releases page](https://github.com/Dhafer84/firstgreenci/releases/latest):

| Your machine | File to take |
| --- | --- |
| Mac with Apple Silicon (M1 and later) | `..._macOS_AppleSilicon.tar.gz` |
| Mac with Intel | `..._macOS_Intel.tar.gz` |
| Windows | `..._Windows_x86-64.zip` |
| Linux | `..._Linux_x86-64.tar.gz` |

**On macOS and Linux**, download with `curl` rather than the browser, adapting the version and the file name:

```bash
curl -L -o firstgreenci.tar.gz https://github.com/Dhafer84/firstgreenci/releases/download/v0.1.2/firstgreenci_0.1.2_macOS_AppleSilicon.tar.gz
tar -xzf firstgreenci.tar.gz
sudo mv firstgreenci /usr/local/bin/
```

Why `curl`? Because a browser-downloaded file is quarantined by macOS and refuses to run. If that happens:

```bash
xattr -d com.apple.quarantine firstgreenci
```

**On Windows**, unzip, then put `firstgreenci.exe` in a folder on your `PATH`. SmartScreen may warn on first run: the executables are not signed, for want of a certificate.

**Check what you downloaded.** Every release ships a `checksums.txt`:

```bash
shasum -a 256 -c checksums.txt --ignore-missing
```

With this method `act` is not installed for you: `brew install act`, `scoop install act`, or the [act executables](https://github.com/nektos/act/releases).

#### Then, whichever method you used

```bash
firstgreenci doctor
```

It checks Docker, act and the container image, and explains step by step what is missing.

*If you already have Go 1.23 or newer, `go install github.com/Dhafer84/firstgreenci/cmd/firstgreenci@latest` works too.*

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

### What is recognised

Python with pip, Poetry or Pipenv, and JavaScript with npm, Yarn or pnpm.

The runtime version is read from `.python-version`, `requires-python`, `.nvmrc` or `engines.node`. Failing that, Python 3.12 and Node.js 20 — and the tool **says so**, rather than letting you believe it picked up yours:

```
·  Your project does not pin a Node.js version; the pipeline will use 20.
   To choose it: a .nvmrc file, or "engines" in package.json.
```

### Projects made of several folders

GitHub only reads workflows from `.github/workflows/` **at the root of the repository**. A file placed anywhere else runs perfectly well on your machine and is never triggered by GitHub.

So the tool checks where it writes. Run it in a subfolder of a repository and it **refuses**, giving you the command to run from the root; `--force` goes through anyway. And when the root holds no recognisable project, it names the subfolders that do:

```
I did not recognise a Python or JavaScript project in …

I found these just below:
  firstgreenci init backend
  firstgreenci init frontend
```

### It warns you before making you wait

If your tests cannot be collected by the tool that was detected, `init` says so **straight away**, with the numbers, instead of letting you find a red pipeline after minutes of running:

```
⚠  Your tests will not be found by unittest.

     Test files                : 19
     "test_" functions         : 406
     unittest.TestCase classes : 0
     Test tool declared        : none
```

### Your files stay yours

- An existing file is **never** replaced without your explicit consent.
- `run` writes no file. Without a pipeline, it tells you to run `init`.
- Your `~/.actrc` is never touched: the image is always passed to act explicitly.
- Your code stays on your machine.
- The generated file is an ordinary, commented workflow: it keeps working if you uninstall FirstGreen CI.

### Known differences from GitHub Actions

- **Processor architecture.** On an Apple Silicon Mac the container runs on arm64, while GitHub runs amd64. Much faster, but a package that only exists for amd64 will behave differently.
- **Container image.** `catthehacker/ubuntu` is not GitHub's image; tools present on GitHub may be missing.
- **A cache shared between your projects.** act keeps a Docker volume named `act-toolcache` and reuses it for every local run: a package installed by one project stays available to the next. **A pipeline can therefore be green on your machine and red on GitHub**, which starts from a fresh runner every time. To check without a safety net: `docker volume rm act-toolcache` before running again.
- **Cache, services and matrices** are not reproduced exactly by act.
- **One question on the first run.** The tool avoids questions, but choosing the image commits more than a gigabyte of download, and cannot be guessed for you.
- **Choosing the language.** `LANG` and `LC_ALL` decide when they exist, and a terminal almost always sets them — including on a Mac set to French, where macOS may compose an English value if your region has no French locale. **Only when those variables are absent** does the tool read the macOS interface language. On Windows it does not, because that would cost a PowerShell start on every command. Either way, `--lang fr` or `FIRSTGREENCI_LANG=fr` settles it.
- **Reading `pyproject.toml`**: scanned line by line, without a TOML parser, to avoid an external dependency.

### An error it cannot translate?

It says so plainly, shows the last lines of the log, and invites you to report it. That is how the list of translations grows: [open an issue](https://github.com/Dhafer84/firstgreenci/issues).

### Contributing

Pipeline templates (`templates/`) and translations (`locales/`) sit outside the engine on purpose. The tests replay real act runs recorded in `testdata/act/`, so they need neither Docker, nor act, nor the network.

```bash
go build ./... && go vet ./... && gofmt -l . && go test ./...
```

MIT licence.
