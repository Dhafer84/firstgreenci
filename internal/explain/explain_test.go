package explain_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	firstgreenci "github.com/Dhafer84/firstgreenci"
	"github.com/Dhafer84/firstgreenci/internal/detect"
	"github.com/Dhafer84/firstgreenci/internal/explain"
	"github.com/Dhafer84/firstgreenci/internal/i18n"
)

func projectPath(name string) string {
	return filepath.Join("..", "..", "testdata", "projects", name)
}

// TestStepsMatchTheGeneratedWorkflow is the guard against the worst kind of
// drift: a pipeline explained differently from what it actually does. Every
// step announced in the terminal must appear in the workflow, and the two
// must have the same number of steps.
func TestStepsMatchTheGeneratedWorkflow(t *testing.T) {
	tests := []struct {
		project   string
		reference string
		lang      string
	}{
		{"python-pip-pytest", "python-pip-pytest.fr.yml", "fr"},
		{"python-pip-pytest", "python-pip-pytest.en.yml", "en"},
		{"js-npm", "js-npm.fr.yml", "fr"},
		{"js-pnpm", "js-pnpm.fr.yml", "fr"},
	}

	for _, test := range tests {
		t.Run(test.reference, func(t *testing.T) {
			project, err := detect.Detect(projectPath(test.project))
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}

			catalog, err := i18n.Load(firstgreenci.LocalesFS, test.lang)
			if err != nil {
				t.Fatalf("load catalog: %v", err)
			}

			workflow, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", test.reference))
			if err != nil {
				t.Fatalf("read reference workflow: %v", err)
			}

			steps := explain.Steps(catalog, project)
			if declared := strings.Count(string(workflow), "      - name: "); declared != len(steps) {
				t.Errorf("the workflow declares %d steps, the explanation describes %d", declared, len(steps))
			}

			for _, step := range steps {
				if !strings.Contains(string(workflow), "- name: \""+step.Title+"\"") {
					t.Errorf("step %q is explained but absent from the workflow", step.Title)
				}
				if step.Description == "" {
					t.Errorf("step %q has no explanation", step.Title)
				}
				// A description left as its own key means a missing message.
				if strings.HasPrefix(step.Description, "explain.step.") {
					t.Errorf("step %q shows a raw key instead of a sentence: %q", step.Title, step.Description)
				}
			}
		})
	}
}

// TestSummaryNamesWhatWasFound checks the one-line verdict shown first.
func TestSummaryNamesWhatWasFound(t *testing.T) {
	project, err := detect.Detect(projectPath("python-pip-pytest"))
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	catalog, err := i18n.Load(firstgreenci.LocalesFS, "fr")
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}

	want := "Projet détecté : Python (pytest, requirements.txt)"
	if got := explain.Summary(catalog, project); got != want {
		t.Errorf("Summary = %q, want %q", got, want)
	}
}

// TestEvidenceIsTranslated checks that every fact is rendered as a sentence,
// never as a bare key.
func TestEvidenceIsTranslated(t *testing.T) {
	for _, name := range []string{"python-pip-pytest", "python-poetry", "js-npm", "js-yarn"} {
		project, err := detect.Detect(projectPath(name))
		if err != nil {
			t.Fatalf("Detect(%s): %v", name, err)
		}

		catalog, err := i18n.Load(firstgreenci.LocalesFS, "fr")
		if err != nil {
			t.Fatalf("load catalog: %v", err)
		}

		lines := explain.Evidence(catalog, project)
		if len(lines) == 0 {
			t.Errorf("%s: no evidence shown", name)
		}
		for _, line := range lines {
			if strings.HasPrefix(line, "detect.evidence.") {
				t.Errorf("%s: raw key shown instead of a sentence: %q", name, line)
			}
			if strings.Contains(line, "%!") {
				t.Errorf("%s: wrong number of arguments in a message: %q", name, line)
			}
		}
	}
}
