// Package detect works out what a project is made of: its language, the tool
// that manages its dependencies, and the command that runs its tests.
//
// Detection never guesses when it is unsure. When two languages are present,
// or when nothing is recognised, it returns a typed error that the caller
// turns into a question or into an explanation. That is design principle 4:
// ask only when detection fails.
package detect

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Language is a project language supported by the MVP.
type Language string

const (
	Python     Language = "python"
	JavaScript Language = "javascript"
)

// Default runtime versions, used when the project does not pin one. They are
// deliberately recent but not bleeding edge: the pipeline has to go green on
// the first try.
const (
	defaultPythonVersion = "3.12"
	defaultNodeVersion   = "20"
)

// Evidence records one fact that led to a conclusion, so that the tool can
// show its reasoning instead of announcing a verdict.
type Evidence struct {
	// File is the name shown to the user, relative to the project root.
	File string
	// MessageKey is the translation key describing what the file proves. Its
	// first argument is always File.
	MessageKey string
	// Args are the extra arguments of the message, after File.
	Args []any
}

// Project is everything the generator needs to write a workflow.
type Project struct {
	Root           string
	Language       Language
	PackageManager string
	RuntimeVersion string
	InstallCommand string
	TestCommand    string

	// TestTool and Manifest are short labels used in the one-line summary,
	// for example "pytest" and "requirements.txt".
	TestTool string
	Manifest string

	Evidence []Evidence
}

// NotRecognisedError reports that no supported project was found.
type NotRecognisedError struct {
	Root string
}

func (e *NotRecognisedError) Error() string {
	return fmt.Sprintf("no supported project found in %s", e.Root)
}

// AmbiguousError reports that several languages were found, so the user has
// to pick one.
type AmbiguousError struct {
	Root       string
	Candidates []Language
}

func (e *AmbiguousError) Error() string {
	names := make([]string, len(e.Candidates))
	for i, candidate := range e.Candidates {
		names[i] = string(candidate)
	}
	return fmt.Sprintf("several languages found in %s: %s", e.Root, strings.Join(names, ", "))
}

// UnknownLanguageError reports a --project-type value that is not supported.
type UnknownLanguageError struct {
	Value string
}

func (e *UnknownLanguageError) Error() string {
	return fmt.Sprintf("unknown project type %q", e.Value)
}

// MissingTestCommandError reports a project whose test command cannot be
// determined, and names the file where the user should declare one.
type MissingTestCommandError struct {
	Manifest string
}

func (e *MissingTestCommandError) Error() string {
	return fmt.Sprintf("no test command declared in %s", e.Manifest)
}

// Detect inspects root and returns the project it describes.
func Detect(root string) (*Project, error) {
	candidates := candidates(root)

	switch len(candidates) {
	case 0:
		return nil, &NotRecognisedError{Root: root}
	case 1:
		return DetectAs(root, candidates[0])
	default:
		return nil, &AmbiguousError{Root: root, Candidates: candidates}
	}
}

// DetectAs inspects root as a project of the given language, skipping the
// language detection. It backs the --project-type flag and the answer to the
// question asked when detection is ambiguous.
func DetectAs(root string, language Language) (*Project, error) {
	switch language {
	case Python:
		return detectPython(root)
	case JavaScript:
		return detectJavaScript(root)
	default:
		return nil, &UnknownLanguageError{Value: string(language)}
	}
}

// ParseLanguage turns a user-supplied value into a Language.
func ParseLanguage(value string) (Language, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "python", "py":
		return Python, nil
	case "javascript", "js", "node":
		return JavaScript, nil
	default:
		return "", &UnknownLanguageError{Value: value}
	}
}

// candidates lists the languages whose marker files are present in root, in a
// stable order so that the question asked on ambiguity never changes shape.
func candidates(root string) []Language {
	var found []Language

	for _, marker := range []string{"requirements.txt", "pyproject.toml", "Pipfile", "setup.py"} {
		if fileExists(root, marker) {
			found = append(found, Python)
			break
		}
	}

	if fileExists(root, "package.json") {
		found = append(found, JavaScript)
	}

	return found
}

// fileExists reports whether name is a regular file inside root.
func fileExists(root, name string) bool {
	info, err := os.Stat(filepath.Join(root, name))
	return err == nil && !info.IsDir()
}

// dirExists reports whether name is a directory inside root.
func dirExists(root, name string) bool {
	info, err := os.Stat(filepath.Join(root, name))
	return err == nil && info.IsDir()
}

// readFile returns the content of name inside root, or an empty string when
// the file is absent or unreadable. Callers treat both cases as "no signal".
func readFile(root, name string) string {
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		return ""
	}
	return string(data)
}

// versionPattern captures a major.minor version inside a constraint such as
// ">=3.11,<4.0" or "^20.10.0".
var versionPattern = regexp.MustCompile(`(\d+)(?:\.(\d+))?`)

// firstVersion extracts the first major.minor version found in value. It
// returns an empty string when value holds no number.
func firstVersion(value string, withMinor bool) string {
	match := versionPattern.FindStringSubmatch(value)
	if match == nil {
		return ""
	}
	if !withMinor || match[2] == "" {
		return match[1]
	}
	return match[1] + "." + match[2]
}
