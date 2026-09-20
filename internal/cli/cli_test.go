package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dhafer84/firstgreenci/internal/cli"
	"github.com/Dhafer84/firstgreenci/internal/generate"
)

// result holds everything a command produced, so that a test can assert on
// the exit code and on what the user actually read.
type result struct {
	code   int
	stdout string
	stderr string
}

// run executes the tool the way the process would, with no environment
// variables, so that tests never depend on the locale of the machine.
func run(t *testing.T, input string, interactive bool, args ...string) result {
	t.Helper()

	var stdout, stderr bytes.Buffer
	code := cli.Run(cli.Environment{
		Args:           args,
		Stdin:          strings.NewReader(input),
		Stdout:         &stdout,
		Stderr:         &stderr,
		LookupEnv:      func(string) (string, bool) { return "", false },
		SystemLanguage: func() string { return "" },
		Interactive:    interactive,
	})
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// sampleProject copies one of the sample projects into a temporary folder, so
// that a test can write into it without touching testdata.
func sampleProject(t *testing.T, name string) string {
	t.Helper()

	source := filepath.Join("..", "..", "testdata", "projects", name)
	destination := filepath.Join(t.TempDir(), name)

	if err := os.CopyFS(destination, os.DirFS(source)); err != nil {
		t.Fatalf("copy sample project %s: %v", name, err)
	}
	return destination
}

func workflowPath(root string) string {
	return filepath.Join(root, generate.WorkflowPath())
}

func TestInitCreatesTheWorkflow(t *testing.T) {
	root := sampleProject(t, "python-pip-pytest")

	got := run(t, "", false, "init", "--lang", "fr", root)

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", got.code, got.stderr)
	}
	for _, expected := range []string{
		"Projet détecté : Python (pytest, requirements.txt)",
		"Ce que j'ai vu :",
		"Ce pipeline fait 4 choses :",
		"Récupérer le code",
	} {
		if !strings.Contains(got.stdout, expected) {
			t.Errorf("output does not contain %q:\n%s", expected, got.stdout)
		}
	}

	content, err := os.ReadFile(workflowPath(root))
	if err != nil {
		t.Fatalf("workflow not written: %v", err)
	}
	if !strings.Contains(string(content), "actions/setup-python@v5") {
		t.Errorf("the workflow does not set up Python:\n%s", content)
	}
}

func TestInitSpeaksEnglish(t *testing.T) {
	root := sampleProject(t, "js-npm")

	got := run(t, "", false, "init", "--lang", "en", root)

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "Project detected: JavaScript") {
		t.Errorf("output is not in English:\n%s", got.stdout)
	}

	content, err := os.ReadFile(workflowPath(root))
	if err != nil {
		t.Fatalf("workflow not written: %v", err)
	}
	if !strings.Contains(string(content), "Get the code") {
		t.Errorf("the workflow is not commented in English:\n%s", content)
	}
}

// TestInitRefusesToOverwriteWithoutAnswer covers the case of a script or a
// pipeline: nothing to answer with, so nothing is destroyed.
func TestInitRefusesToOverwriteWithoutAnswer(t *testing.T) {
	root := sampleProject(t, "python-pip-pytest")

	if got := run(t, "", false, "init", "--lang", "fr", root); got.code != 0 {
		t.Fatalf("first run: exit code = %d\nstderr: %s", got.code, got.stderr)
	}
	original, err := os.ReadFile(workflowPath(root))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	got := run(t, "", false, "init", "--lang", "fr", root)
	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	if !strings.Contains(got.stdout, "existe déjà") {
		t.Errorf("the output does not say the file exists:\n%s", got.stdout)
	}
	if !strings.Contains(got.stdout, "--force") {
		t.Errorf("the output does not say how to proceed:\n%s", got.stdout)
	}

	after, err := os.ReadFile(workflowPath(root))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(after) != string(original) {
		t.Error("the existing workflow was modified without an answer")
	}
}

func TestInitOverwriteAnswers(t *testing.T) {
	tests := []struct {
		name     string
		answer   string
		replaced bool
		code     int
	}{
		{name: "yes in French", answer: "o\n", replaced: true, code: 0},
		{name: "yes spelled out", answer: "oui\n", replaced: true, code: 0},
		{name: "no", answer: "n\n", replaced: false, code: 0},
		{name: "empty answer means no", answer: "\n", replaced: false, code: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := sampleProject(t, "python-pip-pytest")

			if got := run(t, "", false, "init", "--lang", "fr", root); got.code != 0 {
				t.Fatalf("first run: exit code = %d", got.code)
			}
			if err := os.WriteFile(workflowPath(root), []byte("# edited by hand\n"), 0o644); err != nil {
				t.Fatalf("simulate a hand-edited file: %v", err)
			}

			got := run(t, test.answer, true, "init", "--lang", "fr", root)
			if got.code != test.code {
				t.Errorf("exit code = %d, want %d\nstderr: %s", got.code, test.code, got.stderr)
			}

			content, err := os.ReadFile(workflowPath(root))
			if err != nil {
				t.Fatalf("read back: %v", err)
			}
			kept := string(content) == "# edited by hand\n"
			if test.replaced && kept {
				t.Error("the file was kept although the answer was yes")
			}
			if !test.replaced && !kept {
				t.Error("the hand-edited file was replaced although the answer was not yes")
			}
		})
	}
}

func TestInitForceReplacesWithoutAsking(t *testing.T) {
	root := sampleProject(t, "python-pip-pytest")

	if got := run(t, "", false, "init", "--lang", "fr", root); got.code != 0 {
		t.Fatalf("first run: exit code = %d", got.code)
	}
	if err := os.WriteFile(workflowPath(root), []byte("# edited by hand\n"), 0o644); err != nil {
		t.Fatalf("simulate a hand-edited file: %v", err)
	}

	got := run(t, "", false, "init", "--lang", "fr", "--force", root)
	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", got.code, got.stderr)
	}

	content, err := os.ReadFile(workflowPath(root))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if strings.Contains(string(content), "edited by hand") {
		t.Error("--force did not replace the file")
	}
}

func TestInitDryRunWritesNothing(t *testing.T) {
	root := sampleProject(t, "python-pip-pytest")

	got := run(t, "", false, "init", "--lang", "fr", "--dry-run", root)

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", got.code, got.stderr)
	}
	if !strings.Contains(got.stdout, "actions/checkout@v4") {
		t.Errorf("the preview does not contain the workflow:\n%s", got.stdout)
	}
	if !strings.Contains(got.stdout, "aucun fichier n'a été écrit") {
		t.Errorf("the preview does not say that nothing was written:\n%s", got.stdout)
	}
	if _, err := os.Stat(workflowPath(root)); !os.IsNotExist(err) {
		t.Error("--dry-run wrote a file")
	}
}

func TestInitAmbiguousProject(t *testing.T) {
	tests := []struct {
		name        string
		answer      string
		interactive bool
		code        int
		wants       string
	}{
		{name: "answer by number", answer: "2\n", interactive: true, code: 0, wants: "actions/setup-node"},
		{name: "answer by name", answer: "python\n", interactive: true, code: 0, wants: "actions/setup-python"},
		{name: "no one to answer", answer: "", interactive: false, code: 1},
		{name: "answer not understood", answer: "peut-être\n", interactive: true, code: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := sampleProject(t, "ambiguous")

			got := run(t, test.answer, test.interactive, "init", "--lang", "fr", root)
			if got.code != test.code {
				t.Fatalf("exit code = %d, want %d\nstdout: %s\nstderr: %s", got.code, test.code, got.stdout, got.stderr)
			}
			if !strings.Contains(got.stdout, "à la fois du Python et du JavaScript") {
				t.Errorf("the output does not explain the ambiguity:\n%s", got.stdout)
			}

			if test.code != 0 {
				if !strings.Contains(got.stdout, "--project-type") {
					t.Errorf("the output does not say how to settle it:\n%s", got.stdout)
				}
				return
			}

			content, err := os.ReadFile(workflowPath(root))
			if err != nil {
				t.Fatalf("workflow not written: %v", err)
			}
			if !strings.Contains(string(content), test.wants) {
				t.Errorf("the workflow does not contain %q:\n%s", test.wants, content)
			}
		})
	}
}

func TestInitProjectTypeSettlesTheAmbiguity(t *testing.T) {
	root := sampleProject(t, "ambiguous")

	got := run(t, "", false, "init", "--lang", "fr", "--project-type", "javascript", root)

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", got.code, got.stderr)
	}
	content, err := os.ReadFile(workflowPath(root))
	if err != nil {
		t.Fatalf("workflow not written: %v", err)
	}
	if !strings.Contains(string(content), "actions/setup-node") {
		t.Errorf("--project-type was not honoured:\n%s", content)
	}
}

func TestInitFailures(t *testing.T) {
	tests := []struct {
		name    string
		project string
		args    []string
		code    int
		says    string
	}{
		{
			name:    "unrecognised project",
			project: "unrecognised",
			code:    1,
			says:    "requirements.txt, pyproject.toml",
		},
		{
			name:    "no test command",
			project: "js-no-test-script",
			code:    1,
			says:    "\"test\": \"node --test\"",
		},
		{
			name:    "manifest that is not valid JSON",
			project: "js-broken-manifest",
			code:    1,
			says:    "Détail technique",
		},
		{
			name:    "unknown project type",
			project: "python-pip-pytest",
			args:    []string{"--project-type", "rust"},
			code:    2,
			says:    "python, javascript",
		},
		{
			name:    "unknown flag",
			project: "python-pip-pytest",
			args:    []string{"--turbo"},
			code:    2,
			says:    "Utilisation",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := sampleProject(t, test.project)

			args := append([]string{"init", "--lang", "fr"}, test.args...)
			got := run(t, "", false, append(args, root)...)

			if got.code != test.code {
				t.Fatalf("exit code = %d, want %d\nstdout: %s\nstderr: %s", got.code, test.code, got.stdout, got.stderr)
			}
			if !strings.Contains(got.stderr, test.says) {
				t.Errorf("stderr does not contain %q:\n%s", test.says, got.stderr)
			}
			if _, err := os.Stat(workflowPath(root)); !os.IsNotExist(err) {
				t.Error("a workflow was written although the command failed")
			}
		})
	}
}

func TestInitRejectsSeveralFolders(t *testing.T) {
	root := sampleProject(t, "python-pip-pytest")

	got := run(t, "", false, "init", "--lang", "fr", root, root)

	if got.code != 2 {
		t.Fatalf("exit code = %d, want 2", got.code)
	}
	if !strings.Contains(got.stderr, "un seul dossier") {
		t.Errorf("stderr does not explain the problem:\n%s", got.stderr)
	}
}

func TestTopLevelCommands(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		code     int
		inStdout string
		inStderr string
	}{
		{name: "no command", args: nil, code: 2, inStderr: "Il manque une commande"},
		{name: "unknown command", args: []string{"deploy"}, code: 2, inStderr: "deploy"},
		{name: "help", args: []string{"help"}, code: 0, inStdout: "Utilisation"},
		{name: "long help flag", args: []string{"--help"}, code: 0, inStdout: "firstgreenci init"},
		{name: "version", args: []string{"version"}, code: 0, inStdout: cli.Version},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := cli.Run(cli.Environment{
				Args:   test.args,
				Stdin:  strings.NewReader(""),
				Stdout: &stdout,
				Stderr: &stderr,
				// The French catalog is selected through the environment, to
				// cover the path used when no --lang flag is given.
				LookupEnv: func(name string) (string, bool) {
					if name == "FIRSTGREENCI_LANG" {
						return "fr", true
					}
					return "", false
				},
				SystemLanguage: func() string { return "" },
			})

			if code != test.code {
				t.Fatalf("exit code = %d, want %d", code, test.code)
			}
			if test.inStdout != "" && !strings.Contains(stdout.String(), test.inStdout) {
				t.Errorf("stdout does not contain %q:\n%s", test.inStdout, stdout.String())
			}
			if test.inStderr != "" && !strings.Contains(stderr.String(), test.inStderr) {
				t.Errorf("stderr does not contain %q:\n%s", test.inStderr, stderr.String())
			}
		})
	}
}

// TestInitRefusesOutsideTheRepositoryRoot covers the worst defect found so
// far: a workflow written below the root runs green locally and is ignored
// by GitHub, silently. The tool must refuse, as it refuses to overwrite.
func TestInitRefusesOutsideTheRepositoryRoot(t *testing.T) {
	repository := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repository, ".git"), 0o755); err != nil {
		t.Fatalf("build the repository: %v", err)
	}

	backend := filepath.Join(repository, "backend")
	if err := os.MkdirAll(backend, 0o755); err != nil {
		t.Fatalf("build the subfolder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backend, "package.json"),
		[]byte(`{"name":"backend","scripts":{"test":"node --test"}}`), 0o644); err != nil {
		t.Fatalf("write the manifest: %v", err)
	}

	got := run(t, "", false, "init", "--lang", "fr", backend)

	if got.code != 1 {
		t.Errorf("exit code = %d, want 1", got.code)
	}
	if !strings.Contains(got.stdout, "racine de votre dépôt") {
		t.Errorf("the reason was not given:\n%s", got.stdout)
	}
	if !strings.Contains(got.stdout, "--force") {
		t.Errorf("the way through was not given:\n%s", got.stdout)
	}
	if _, err := os.Stat(workflowPath(backend)); !os.IsNotExist(err) {
		t.Error("a workflow was written where GitHub would never read it")
	}

	// --force is the deliberate way through, and it must work.
	forced := run(t, "", false, "init", "--lang", "fr", "--force", backend)
	if forced.code != 0 {
		t.Fatalf("with --force: exit code = %d, want 0\n%s", forced.code, forced.stderr)
	}
	if _, err := os.Stat(workflowPath(backend)); err != nil {
		t.Errorf("--force did not write the workflow: %v", err)
	}
}

// TestInitAtTheRepositoryRootIsSilent is the symmetric check: the ordinary
// case must not gain a warning.
func TestInitAtTheRepositoryRootIsSilent(t *testing.T) {
	root := sampleProject(t, "python-pip-pytest")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("build the repository: %v", err)
	}

	got := run(t, "", false, "init", "--lang", "fr", root)

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", got.code, got.stderr)
	}
	for _, unwanted := range []string{"racine de votre dépôt", "n'est pas un dépôt git"} {
		if strings.Contains(got.stdout, unwanted) {
			t.Errorf("an ordinary project was warned about %q:\n%s", unwanted, got.stdout)
		}
	}
}
