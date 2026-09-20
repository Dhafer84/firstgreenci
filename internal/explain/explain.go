// Package explain turns what the tool did into sentences a beginner can read.
//
// Design principle 1: no unexplained jargon. Every generated step gets one
// sentence, and every conclusion of the detection can be traced back to the
// file it came from.
package explain

import (
	"github.com/Dhafer84/firstgreenci/internal/detect"
	"github.com/Dhafer84/firstgreenci/internal/i18n"
)

// Step is one step of the generated workflow, described for a human.
type Step struct {
	// Title is the step name as it appears in the workflow file, so that the
	// terminal and the generated file use the same words.
	Title string
	// Description is one sentence saying what the step does.
	Description string
}

// Steps describes, in order, what the generated pipeline will do.
//
// The order mirrors the templates in templates/. A test checks that the two
// stay in step, because a pipeline explained differently from what it does
// would be worse than one left unexplained.
func Steps(catalog *i18n.Catalog, project *detect.Project) []Step {
	steps := []Step{
		{
			Title:       catalog.T("workflow.step.checkout.name"),
			Description: catalog.T("explain.step.checkout"),
		},
	}

	switch project.Language {
	case detect.JavaScript:
		steps = append(steps, Step{
			Title:       catalog.T("workflow.step.setup_node.name"),
			Description: catalog.T("explain.step.setup_node", project.RuntimeVersion),
		})
	default:
		steps = append(steps, Step{
			Title:       catalog.T("workflow.step.setup_python.name"),
			Description: catalog.T("explain.step.setup_python", project.RuntimeVersion),
		})
	}

	return append(steps,
		Step{
			Title:       catalog.T("workflow.step.install.name"),
			Description: catalog.T("explain.step.install"),
		},
		Step{
			Title:       catalog.T("workflow.step.test.name"),
			Description: catalog.T("explain.step.test", project.TestCommand),
		},
	)
}

// Evidence lists, in the user's language, the facts that led the detection to
// its conclusion.
func Evidence(catalog *i18n.Catalog, project *detect.Project) []string {
	lines := make([]string, 0, len(project.Evidence))
	for _, evidence := range project.Evidence {
		args := append([]any{evidence.File}, evidence.Args...)
		lines = append(lines, catalog.T(evidence.MessageKey, args...))
	}
	return lines
}

// LanguageName is the display name of a project language.
func LanguageName(catalog *i18n.Catalog, language detect.Language) string {
	switch language {
	case detect.JavaScript:
		return catalog.T("language.javascript")
	default:
		return catalog.T("language.python")
	}
}

// Summary is the one-line verdict of the detection, for example
// "Projet détecté : Python (pytest, requirements.txt)".
func Summary(catalog *i18n.Catalog, project *detect.Project) string {
	return catalog.T("detect.summary", LanguageName(catalog, project.Language), project.TestTool, project.Manifest)
}
