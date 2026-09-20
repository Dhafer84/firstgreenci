package detect_test

import (
	"errors"
	"path/filepath"
	"testing"

	firstgreenci "github.com/Dhafer84/firstgreenci"
	"github.com/Dhafer84/firstgreenci/internal/detect"
	"github.com/Dhafer84/firstgreenci/internal/i18n"
)

// projectPath builds the path of a sample project. filepath.Join keeps the
// tests meaningful on Windows, where the separator is not a slash.
func projectPath(name string) string {
	return filepath.Join("..", "..", "testdata", "projects", name)
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name           string
		project        string
		language       detect.Language
		packageManager string
		runtimeVersion string
		installCommand string
		testCommand    string
		testTool       string
		manifest       string
	}{
		{
			name:           "pip with pytest declared",
			project:        "python-pip-pytest",
			language:       detect.Python,
			packageManager: "pip",
			runtimeVersion: "3.12",
			installCommand: "python -m pip install --upgrade pip\npython -m pip install -r requirements.txt",
			testCommand:    "pytest",
			testTool:       "pytest",
			manifest:       "requirements.txt",
		},
		{
			name:           "poetry pins the version in pyproject",
			project:        "python-poetry",
			language:       detect.Python,
			packageManager: "poetry",
			runtimeVersion: "3.11",
			installCommand: "python -m pip install poetry\npoetry install",
			testCommand:    "poetry run pytest",
			testTool:       "pytest",
			manifest:       "pyproject.toml",
		},
		{
			name:           "no pytest declared falls back to unittest",
			project:        "python-unittest",
			language:       detect.Python,
			packageManager: "pip",
			runtimeVersion: "3.12",
			installCommand: "python -m pip install --upgrade pip\npython -m pip install -r requirements.txt",
			testCommand:    "python -m unittest discover",
			testTool:       "unittest",
			manifest:       "requirements.txt",
		},
		{
			name:           "pyproject alone is installed as a package",
			project:        "python-pyproject",
			language:       detect.Python,
			packageManager: "pip",
			runtimeVersion: "3.10",
			installCommand: "python -m pip install --upgrade pip\npython -m pip install .",
			testCommand:    "python -m unittest discover",
			testTool:       "unittest",
			manifest:       "pyproject.toml",
		},
		{
			name:           "pipenv runs the tests through its environment",
			project:        "python-pipenv",
			language:       detect.Python,
			packageManager: "pipenv",
			runtimeVersion: "3.12",
			installCommand: "python -m pip install pipenv\npipenv install --dev",
			testCommand:    "pipenv run pytest",
			testTool:       "pytest",
			manifest:       "Pipfile",
		},
		{
			name:           "a .python-version file wins over the default",
			project:        "python-root-tests",
			language:       detect.Python,
			packageManager: "pip",
			runtimeVersion: "3.11",
			installCommand: "python -m pip install --upgrade pip\npython -m pip install -r requirements.txt",
			testCommand:    "pytest",
			testTool:       "pytest",
			manifest:       "requirements.txt",
		},
		{
			name:           "npm with a lock file installs with npm ci",
			project:        "js-npm",
			language:       detect.JavaScript,
			packageManager: "npm",
			runtimeVersion: "20",
			installCommand: "npm ci",
			testCommand:    "npm test",
			testTool:       "npm test",
			manifest:       "package.json",
		},
		{
			name:           "yarn reads the version from engines",
			project:        "js-yarn",
			language:       detect.JavaScript,
			packageManager: "yarn",
			runtimeVersion: "22",
			installCommand: "corepack enable\nyarn install --frozen-lockfile",
			testCommand:    "yarn test",
			testTool:       "yarn test",
			manifest:       "package.json",
		},
		{
			name:           "pnpm reads the version from .nvmrc",
			project:        "js-pnpm",
			language:       detect.JavaScript,
			packageManager: "pnpm",
			runtimeVersion: "18",
			installCommand: "corepack enable\npnpm install --frozen-lockfile",
			testCommand:    "pnpm test",
			testTool:       "pnpm test",
			manifest:       "package.json",
		},
		{
			name:           "without a lock file npm ci cannot be used",
			project:        "js-no-lockfile",
			language:       detect.JavaScript,
			packageManager: "npm",
			runtimeVersion: "20",
			installCommand: "npm install",
			testCommand:    "npm test",
			testTool:       "npm test",
			manifest:       "package.json",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project, err := detect.Detect(projectPath(test.project))
			if err != nil {
				t.Fatalf("Detect(%s): %v", test.project, err)
			}

			checks := []struct {
				field string
				got   string
				want  string
			}{
				{"Language", string(project.Language), string(test.language)},
				{"PackageManager", project.PackageManager, test.packageManager},
				{"RuntimeVersion", project.RuntimeVersion, test.runtimeVersion},
				{"InstallCommand", project.InstallCommand, test.installCommand},
				{"TestCommand", project.TestCommand, test.testCommand},
				{"TestTool", project.TestTool, test.testTool},
				{"Manifest", project.Manifest, test.manifest},
			}
			for _, check := range checks {
				if check.got != check.want {
					t.Errorf("%s = %q, want %q", check.field, check.got, check.want)
				}
			}

			if len(project.Evidence) == 0 {
				t.Error("Evidence is empty: the tool would announce a verdict without showing what it saw")
			}
		})
	}
}

func TestDetectFailures(t *testing.T) {
	tests := []struct {
		name    string
		project string
		target  any
	}{
		{"nothing recognised", "unrecognised", new(*detect.NotRecognisedError)},
		{"two languages at once", "ambiguous", new(*detect.AmbiguousError)},
		{"no test command declared", "js-no-test-script", new(*detect.MissingTestCommandError)},
		{"manifest that is not valid JSON", "js-broken-manifest", new(*detect.InvalidManifestError)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := detect.Detect(projectPath(test.project))
			if err == nil {
				t.Fatalf("Detect(%s) succeeded, want an error", test.project)
			}
			if !errors.As(err, test.target) {
				t.Fatalf("Detect(%s) returned %T (%v), want %T", test.project, err, err, test.target)
			}
		})
	}
}

// TestAmbiguousListsBothLanguages checks the question the user will be asked.
func TestAmbiguousListsBothLanguages(t *testing.T) {
	_, err := detect.Detect(projectPath("ambiguous"))

	var ambiguous *detect.AmbiguousError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("got %T, want *detect.AmbiguousError", err)
	}

	want := []detect.Language{detect.Python, detect.JavaScript}
	if len(ambiguous.Candidates) != len(want) {
		t.Fatalf("Candidates = %v, want %v", ambiguous.Candidates, want)
	}
	for i, candidate := range ambiguous.Candidates {
		if candidate != want[i] {
			t.Errorf("Candidates[%d] = %q, want %q", i, candidate, want[i])
		}
	}
}

// TestDetectAsResolvesAmbiguity checks that naming the language sidesteps the
// question, which is what --project-type is for.
func TestDetectAsResolvesAmbiguity(t *testing.T) {
	for _, language := range []detect.Language{detect.Python, detect.JavaScript} {
		project, err := detect.DetectAs(projectPath("ambiguous"), language)
		if err != nil {
			t.Fatalf("DetectAs(%q): %v", language, err)
		}
		if project.Language != language {
			t.Errorf("Language = %q, want %q", project.Language, language)
		}
	}
}

func TestParseLanguage(t *testing.T) {
	tests := []struct {
		value   string
		want    detect.Language
		wantErr bool
	}{
		{value: "python", want: detect.Python},
		{value: "  Python  ", want: detect.Python},
		{value: "py", want: detect.Python},
		{value: "javascript", want: detect.JavaScript},
		{value: "JS", want: detect.JavaScript},
		{value: "node", want: detect.JavaScript},
		{value: "rust", wantErr: true},
		{value: "", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			got, err := detect.ParseLanguage(test.value)
			if test.wantErr {
				if err == nil {
					t.Fatalf("ParseLanguage(%q) = %q, want an error", test.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseLanguage(%q): %v", test.value, err)
			}
			if got != test.want {
				t.Errorf("ParseLanguage(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

// TestEvidenceKeysAreTranslated makes sure every fact the tool can report has
// a message in every catalog. A missing key would show a raw key to the user.
func TestEvidenceKeysAreTranslated(t *testing.T) {
	projects := []string{
		"python-pip-pytest", "python-poetry", "python-unittest", "python-pyproject",
		"python-pipenv", "python-root-tests", "js-npm", "js-yarn", "js-pnpm", "js-no-lockfile",
	}

	for _, lang := range i18n.Supported {
		catalog, err := i18n.Load(firstgreenci.LocalesFS, lang)
		if err != nil {
			t.Fatalf("load catalog %s: %v", lang, err)
		}

		for _, name := range projects {
			project, err := detect.Detect(projectPath(name))
			if err != nil {
				t.Fatalf("Detect(%s): %v", name, err)
			}
			for _, evidence := range project.Evidence {
				if !catalog.Has(evidence.MessageKey) {
					t.Errorf("catalog %s has no message for evidence key %q (project %s)", lang, evidence.MessageKey, name)
				}
			}
		}
	}
}
