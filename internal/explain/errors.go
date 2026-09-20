package explain

import (
	"regexp"
	"strings"

	"github.com/Dhafer84/firstgreenci/internal/i18n"
	"github.com/Dhafer84/firstgreenci/internal/run"
)

// excerptLines is how much of the log is shown when the cause has to be read
// by a human. Enough to hold a stack trace, short enough not to be the wall
// of text people gave up on.
const excerptLines = 12

// Diagnosis explains a failed run in two sentences: what happened, and what
// to do about it.
type Diagnosis struct {
	Cause  string
	Action string
	// Known is false when no rule matched. The caller then shows the log
	// excerpt, because an honest "I cannot translate this yet" with the real
	// lines beats a confident wrong guess.
	Known bool
	// Excerpt holds the last useful lines of the failed step.
	Excerpt []string
}

// rule ties a pattern seen in a real run to a pair of messages.
//
// Every pattern below was taken from an act run recorded in testdata/act, not
// from memory. causeArgs and actionArgs say how many captured groups each
// message uses, so that no message is ever handed arguments it has no place
// for.
type rule struct {
	pattern    *regexp.Regexp
	key        string
	causeArgs  int
	actionArgs int
}

// rules are tried in order, most specific first. Infrastructure comes before
// the user's code: if Docker went away, nothing else matters.
var rules = []rule{
	{
		pattern: regexp.MustCompile(`failed to connect to the docker API|Cannot connect to the Docker daemon`),
		key:     "docker",
	},
	{
		pattern: regexp.MustCompile(`no space left on device`),
		key:     "disk_full",
	},
	{
		pattern: regexp.MustCompile(`pull access denied|manifest unknown|failed to pull image|Error response from daemon: .*pull`),
		key:     "image_pull",
	},
	{
		pattern:    regexp.MustCompile(`ModuleNotFoundError: No module named '([^']+)'`),
		key:        "python_module",
		causeArgs:  1,
		actionArgs: 1,
	},
	{
		pattern:   regexp.MustCompile(`Could not find a version that satisfies the requirement (\S+)`),
		key:       "pip_not_found",
		causeArgs: 1,
	},
	{
		pattern: regexp.MustCompile(`Missing script: "test"|npm ERR! Missing script`),
		key:     "npm_missing_script",
	},
	{
		pattern: regexp.MustCompile(`can only install packages when your package\.json and package-lock\.json`),
		key:     "npm_lockfile",
	},
	{
		pattern:    regexp.MustCompile(`Cannot find module '([^']+)'`),
		key:        "node_module",
		causeArgs:  1,
		actionArgs: 1,
	},
	{
		// pytest exits with 5 and says so when it collected nothing. The
		// pipeline is red, but the code is not at fault.
		pattern: regexp.MustCompile(`no tests ran in|collected 0 items`),
		key:     "no_tests",
	},
	{
		pattern:   regexp.MustCompile(`FAILED (\S+)`),
		key:       "test_failed",
		causeArgs: 1,
	},
	{
		pattern: regexp.MustCompile(`\d+ failed[ ,]|Tests:.*failed|AssertionError`),
		key:     "test_failed_generic",
	},
}

// DiagnoseRun explains why a run did not go green.
func DiagnoseRun(catalog *i18n.Catalog, result run.Result) Diagnosis {
	failed, hasFailed := result.FailedStep()

	// Everything the rules are matched against: what the failing step
	// printed, plus what act logged as an error.
	searched := strings.Join(append(append([]string{}, failed.Output...), result.Errors...), "\n")

	diagnosis := Diagnosis{Excerpt: lastLines(failed.Output, excerptLines)}

	for _, candidate := range rules {
		match := candidate.pattern.FindStringSubmatch(searched)
		if match == nil {
			continue
		}

		captures := match[1:]
		diagnosis.Cause = catalog.T("pipeline.error."+candidate.key+".cause", arguments(captures, candidate.causeArgs)...)
		diagnosis.Action = catalog.T("pipeline.error."+candidate.key+".action", arguments(captures, candidate.actionArgs)...)
		diagnosis.Known = true
		return diagnosis
	}

	// A step act added around the workflow failed, so the user's own steps
	// never ran. That is a tooling problem, and it has its own answer.
	if hasFailed && failed.Internal {
		diagnosis.Cause = catalog.T("pipeline.error.setup.cause")
		diagnosis.Action = catalog.T("pipeline.error.setup.action")
		diagnosis.Known = true
		return diagnosis
	}

	diagnosis.Cause = catalog.T("pipeline.error.unknown.cause")
	diagnosis.Action = catalog.T("pipeline.error.unknown.action")
	return diagnosis
}

// arguments returns the first count captures, as arguments for a message.
// Handing a message more arguments than it has places for would print Go's
// own complaint in the middle of a French sentence.
func arguments(captures []string, count int) []any {
	if count > len(captures) {
		count = len(captures)
	}

	args := make([]any, count)
	for i := 0; i < count; i++ {
		args[i] = captures[i]
	}
	return args
}

// lastLines returns the last count non-empty lines, in order.
func lastLines(lines []string, count int) []string {
	var kept []string
	for i := len(lines) - 1; i >= 0 && len(kept) < count; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			kept = append(kept, lines[i])
		}
	}

	// Reverse back into reading order.
	for left, right := 0, len(kept)-1; left < right; left, right = left+1, right-1 {
		kept[left], kept[right] = kept[right], kept[left]
	}
	return kept
}
