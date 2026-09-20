package generate_test

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	firstgreenci "github.com/Dhafer84/firstgreenci"
	"github.com/Dhafer84/firstgreenci/internal/detect"
	"github.com/Dhafer84/firstgreenci/internal/generate"
	"github.com/Dhafer84/firstgreenci/internal/i18n"
)

// update rewrites the reference files instead of comparing against them:
// go test ./internal/generate -update
var update = flag.Bool("update", false, "rewrite the reference workflows")

func projectPath(name string) string {
	return filepath.Join("..", "..", "testdata", "projects", name)
}

// TestRenderMatchesReference compares the generated workflow with a file kept
// under version control, so that any change to a template or to a translation
// shows up as a readable diff during review.
func TestRenderMatchesReference(t *testing.T) {
	tests := []struct {
		project string
		lang    string
	}{
		{"python-pip-pytest", "fr"},
		{"python-pip-pytest", "en"},
		{"python-poetry", "fr"},
		{"python-unittest", "en"},
		{"js-npm", "fr"},
		{"js-npm", "en"},
		{"js-pnpm", "fr"},
		{"js-yarn", "en"},
	}

	for _, test := range tests {
		t.Run(test.project+"."+test.lang, func(t *testing.T) {
			project, err := detect.Detect(projectPath(test.project))
			if err != nil {
				t.Fatalf("Detect(%s): %v", test.project, err)
			}

			catalog, err := i18n.Load(firstgreenci.LocalesFS, test.lang)
			if err != nil {
				t.Fatalf("load catalog %s: %v", test.lang, err)
			}

			got, err := generate.Render(firstgreenci.TemplatesFS, catalog, project)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}

			reference := filepath.Join("..", "..", "testdata", "golden", test.project+"."+test.lang+".yml")
			if *update {
				if err := os.MkdirAll(filepath.Dir(reference), 0o755); err != nil {
					t.Fatalf("create reference directory: %v", err)
				}
				if err := os.WriteFile(reference, got, 0o644); err != nil {
					t.Fatalf("write reference: %v", err)
				}
				return
			}

			want, err := os.ReadFile(reference)
			if err != nil {
				t.Fatalf("read reference (run: go test ./internal/generate -update): %v", err)
			}

			if string(got) != string(want) {
				t.Errorf("generated workflow differs from %s\n--- generated ---\n%s", reference, got)
			}
		})
	}
}

// TestRenderTranslatesComments checks the promise that the generated file
// speaks the user's language, comments included.
func TestRenderTranslatesComments(t *testing.T) {
	project, err := detect.Detect(projectPath("python-pip-pytest"))
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	tests := []struct {
		lang string
		want string
	}{
		{"fr", "Récupérer le code"},
		{"en", "Get the code"},
	}

	for _, test := range tests {
		t.Run(test.lang, func(t *testing.T) {
			catalog, err := i18n.Load(firstgreenci.LocalesFS, test.lang)
			if err != nil {
				t.Fatalf("load catalog: %v", err)
			}

			rendered, err := generate.Render(firstgreenci.TemplatesFS, catalog, project)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}

			if !strings.Contains(string(rendered), test.want) {
				t.Errorf("workflow in %s does not contain %q:\n%s", test.lang, test.want, rendered)
			}
		})
	}
}

// TestWriteNeverOverwritesWithoutForce is design principle 6, under test.
func TestWriteNeverOverwritesWithoutForce(t *testing.T) {
	root := t.TempDir()

	relative, err := generate.Write(root, []byte("first\n"), false)
	if err != nil {
		t.Fatalf("first write: %v", err)
	}
	if relative != generate.WorkflowPath() {
		t.Errorf("Write returned %q, want %q", relative, generate.WorkflowPath())
	}
	if !generate.Exists(root) {
		t.Error("Exists reports no workflow right after writing one")
	}

	_, err = generate.Write(root, []byte("second\n"), false)
	var exists *generate.FileExistsError
	if !errors.As(err, &exists) {
		t.Fatalf("second write returned %v, want *generate.FileExistsError", err)
	}

	onDisk, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(onDisk) != "first\n" {
		t.Errorf("file content is %q, want the original %q", onDisk, "first\n")
	}

	if _, err := generate.Write(root, []byte("second\n"), true); err != nil {
		t.Fatalf("forced write: %v", err)
	}
	onDisk, err = os.ReadFile(filepath.Join(root, relative))
	if err != nil {
		t.Fatalf("read back after force: %v", err)
	}
	if string(onDisk) != "second\n" {
		t.Errorf("forced write left %q, want %q", onDisk, "second\n")
	}
}

// TestWriteCreatesMissingDirectories covers a project with no .github folder,
// which is the usual case for a first pipeline.
func TestWriteCreatesMissingDirectories(t *testing.T) {
	root := t.TempDir()

	if _, err := generate.Write(root, []byte("content\n"), false); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".github", "workflows", "ci.yml")); err != nil {
		t.Fatalf("workflow not found where expected: %v", err)
	}
}

// TestGeneratedWorkflowIsStructurallySound checks the invariants that make the
// output valid YAML. The project carries no YAML parser on purpose, so these
// are verified by hand: a broken indent helper would produce a file that
// GitHub refuses, which is the worst possible first experience.
func TestGeneratedWorkflowIsStructurallySound(t *testing.T) {
	references, err := filepath.Glob(filepath.Join("..", "..", "testdata", "golden", "*.yml"))
	if err != nil {
		t.Fatalf("list reference files: %v", err)
	}
	if len(references) == 0 {
		t.Fatal("no reference file found")
	}

	for _, reference := range references {
		t.Run(filepath.Base(reference), func(t *testing.T) {
			content, err := os.ReadFile(reference)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			text := string(content)

			if strings.Contains(text, "\t") {
				t.Error("the file contains a tab, which YAML forbids for indentation")
			}
			if !strings.HasSuffix(text, "\n") {
				t.Error("the file does not end with a newline")
			}

			lines := strings.Split(text, "\n")
			for i, line := range lines {
				if !strings.HasSuffix(strings.TrimRight(line, " "), "run: |") {
					continue
				}
				keyIndent := len(line) - len(strings.TrimLeft(line, " "))

				if i+1 >= len(lines) {
					t.Fatalf("line %d opens a block scalar but nothing follows", i+1)
				}
				next := lines[i+1]
				nextIndent := len(next) - len(strings.TrimLeft(next, " "))
				if strings.TrimSpace(next) == "" || nextIndent <= keyIndent {
					t.Errorf("line %d opens a block scalar, but line %d is not indented under it: %q", i+1, i+2, next)
				}
			}
		})
	}
}
