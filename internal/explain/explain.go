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

	// The wording depends on where the version came from. Claiming "the same
	// version as on your computer" about a number nobody declared was false,
	// and measurably so: one project ran Node 26 while the pipeline was
	// given 20.
	switch project.Language {
	case detect.JavaScript:
		steps = append(steps, Step{
			Title:       catalog.T("workflow.step.setup_node.name"),
			Description: runtimeDescription(catalog, "explain.step.setup_node", project),
		})
	default:
		steps = append(steps, Step{
			Title:       catalog.T("workflow.step.setup_python.name"),
			Description: runtimeDescription(catalog, "explain.step.setup_python", project),
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

// Warnings renders, in the user's language, what the detection saw coming.
// They are shown before the pipeline is written, so that nobody waits three
// minutes for a red result that was predictable from the start.
func Warnings(catalog *i18n.Catalog, project *detect.Project) []string {
	messages := make([]string, 0, len(project.Warnings))
	for _, warning := range project.Warnings {
		messages = append(messages, catalog.T(warning.MessageKey, warning.Args...))
	}
	return messages
}

// runtimeDescription says where the version came from, or admits that it was
// chosen for the user.
func runtimeDescription(catalog *i18n.Catalog, prefix string, project *detect.Project) string {
	if project.RuntimeVersionSource == "" {
		return catalog.T(prefix+".default", project.RuntimeVersion)
	}
	return catalog.T(prefix+".declared", project.RuntimeVersion, project.RuntimeVersionSource)
}

// Notes renders what is worth knowing but breaks nothing.
func Notes(catalog *i18n.Catalog, project *detect.Project) []string {
	messages := make([]string, 0, len(project.Notes))
	for _, note := range project.Notes {
		messages = append(messages, catalog.T(note.MessageKey, note.Args...))
	}
	return messages
}
