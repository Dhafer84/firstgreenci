package run

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Status is the outcome of a step or of the whole job, as act reports it.
type Status string

const (
	StatusSuccess Status = "success"
	StatusFailure Status = "failure"
	StatusSkipped Status = "skipped"
	StatusUnknown Status = ""
)

// Step is one step of the pipeline, as it actually ran.
type Step struct {
	Name     string
	Status   Status
	Duration time.Duration
	// Output holds the lines the step produced, without act's own decorated
	// summary lines. It is what the error rules are matched against.
	Output []string
	// Internal marks the steps act adds around the workflow's own, such as
	// preparing the container. They are not something the user wrote, so they
	// are reported differently when they fail.
	Internal bool

	firstSeen time.Time
	lastSeen  time.Time
	// sequence is the rank at which the step finished. act reports the lines
	// of a step out of order — an action can log while it is being fetched,
	// before earlier steps have run — so the report is ordered by when each
	// step reached its outcome, not by when it was first mentioned.
	sequence int
}

// Result is everything that could be learnt from one run.
type Result struct {
	Steps []Step
	// Job is the outcome act announced for the job as a whole. It stays
	// unknown when act died before reaching that point, which is how a tool
	// failure is told apart from a red pipeline.
	Job Status
	// Errors are the messages act logged at error level, in order.
	Errors []string
	// Raw is every line act printed, kept for --verbose and for the report a
	// user may send when nothing could be translated.
	Raw []string
}

// Succeeded reports whether the pipeline went green.
func (r Result) Succeeded() bool { return r.Job == StatusSuccess }

// Ran reports whether act got far enough to announce a verdict. When it did
// not, the problem is with the tooling, not with the user's code.
func (r Result) Ran() bool { return r.Job != StatusUnknown }

// FailedStep returns the first step that failed, if any.
func (r Result) FailedStep() (Step, bool) {
	for _, step := range r.Steps {
		if step.Status == StatusFailure {
			return step, true
		}
	}
	return Step{}, false
}

// actLine is the part of act's JSON output we read. act also prints lines
// that are not JSON at all — a warning about Apple M-series chips, a final
// "Error: ..." — so every line is tried as JSON and kept as raw text when it
// is not.
type actLine struct {
	Level      string `json:"level"`
	Message    string `json:"msg"`
	Step       string `json:"step"`
	StepResult string `json:"stepResult"`
	JobResult  string `json:"jobResult"`
	Time       string `json:"time"`
}

// Parser turns act's output into steps and statuses, one line at a time.
//
// It is fed while act runs so that each step can be shown the moment it
// finishes, rather than after a run that may last minutes.
type Parser struct {
	// OnStep is called as soon as a step reaches its outcome. It is how the
	// command line shows progress live.
	OnStep func(Step)

	result Result
	// index points at the step currently running under a given name. A name
	// leaves the index once it finishes, so that a later occurrence starts a
	// new entry instead of overwriting the finished one.
	index map[string]int
	// finished remembers the names that already reached an outcome. An
	// action such as setup-python runs again at the end of the job under the
	// very same name; that second pass is act's business, not the user's.
	finished map[string]bool
	counter  int
}

// NewParser returns a parser ready to be fed.
func NewParser() *Parser {
	return &Parser{
		index:    make(map[string]int),
		finished: make(map[string]bool),
	}
}

// Line feeds one line of act's output.
func (p *Parser) Line(line string) {
	p.result.Raw = append(p.result.Raw, line)

	var decoded actLine
	if err := json.Unmarshal([]byte(line), &decoded); err != nil || decoded.Message == "" && decoded.Level == "" {
		// Not a JSON log line: act's own warnings and its final error land
		// here. They are kept in Raw, and error-looking ones are collected
		// so that nothing important is silently dropped.
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "Error:") {
			p.result.Errors = append(p.result.Errors, strings.TrimSpace(strings.TrimPrefix(trimmed, "Error:")))
		}
		return
	}

	if decoded.Level == "error" && decoded.Message != "" {
		p.result.Errors = append(p.result.Errors, decoded.Message)
	}

	if decoded.JobResult != "" {
		p.result.Job = Status(decoded.JobResult)
	}

	if decoded.Step == "" {
		return
	}

	position := p.stepAt(decoded.Step)
	step := &p.result.Steps[position]

	if moment, err := time.Parse(time.RFC3339, decoded.Time); err == nil {
		if step.firstSeen.IsZero() {
			step.firstSeen = moment
		}
		step.lastSeen = moment
	}

	if !isDecoration(decoded.Message) {
		step.Output = append(step.Output, decoded.Message)
	}

	if decoded.StepResult != "" {
		p.counter++
		step.Status = Status(decoded.StepResult)
		step.Duration = stepDuration(decoded.Message, step.lastSeen.Sub(step.firstSeen))
		step.sequence = p.counter

		finished := *step
		p.finished[decoded.Step] = true
		delete(p.index, decoded.Step)

		if p.OnStep != nil {
			p.OnStep(finished)
		}
	}
}

// HumanLine turns one line of act's output back into something a person can
// read. The JSON form is asked for because it is reliable to parse, not
// because anyone wants to look at it: --verbose shows the message inside,
// which is what act would have printed on its own.
func HumanLine(line string) string {
	var decoded actLine
	if err := json.Unmarshal([]byte(line), &decoded); err != nil || decoded.Message == "" {
		return line
	}
	if decoded.Step != "" {
		return "[" + decoded.Step + "] " + decoded.Message
	}
	return decoded.Message
}

// Result returns everything gathered so far, with the steps in the order they
// finished. Steps that never finished — act died in the middle of one — keep
// their original order and come last.
func (p *Parser) Result() Result {
	ordered := p.result
	ordered.Steps = make([]Step, len(p.result.Steps))
	copy(ordered.Steps, p.result.Steps)

	sort.SliceStable(ordered.Steps, func(i, j int) bool {
		left, right := ordered.Steps[i].sequence, ordered.Steps[j].sequence
		switch {
		case left == 0:
			return false
		case right == 0:
			return true
		default:
			return left < right
		}
	})

	return ordered
}

// stepAt returns the position of a step, adding it on first sight so that the
// order of the report is the order the steps ran in.
func (p *Parser) stepAt(name string) int {
	if position, known := p.index[name]; known {
		return position
	}

	// A name that already finished coming back means act is running the
	// closing phase of an action, under the same label.
	internal := isInternalStep(name) || p.finished[name]

	p.result.Steps = append(p.result.Steps, Step{Name: name, Internal: internal})
	position := len(p.result.Steps) - 1
	p.index[name] = position
	return position
}

// isInternalStep reports whether a step is one act wraps around the workflow
// rather than one the user's file declares. Preparing the container is not
// something a beginner wrote, so its failure is explained differently.
func isInternalStep(name string) bool {
	switch {
	case name == "Set up job", name == "Complete job":
		return true
	case strings.HasPrefix(name, "Post "):
		return true
	default:
		return false
	}
}

// durationPattern captures the time act appends to the line announcing an
// outcome, as in "❌  Failure - Main Run the tests [303.5615ms]".
var durationPattern = regexp.MustCompile(`\[([0-9.]+(?:ns|µs|us|ms|s|m|h))\]\s*$`)

// stepDuration reads the exact time act measured, falling back to the gap
// between the first and last line of the step. The timestamps act writes have
// no fractional part, so that fallback reads 0s for anything under a second —
// accurate enough to say a step was instant, not to time it.
func stepDuration(message string, fallback time.Duration) time.Duration {
	match := durationPattern.FindStringSubmatch(message)
	if match == nil {
		return fallback
	}
	measured, err := time.ParseDuration(strings.Replace(match[1], "µs", "us", 1))
	if err != nil {
		return fallback
	}
	return measured
}

// isDecoration reports whether a message is one of act's own summary lines
// rather than output from the step itself. Keeping them would put emoji and
// English in front of a user who asked for French.
func isDecoration(message string) bool {
	trimmed := strings.TrimSpace(message)
	for _, marker := range []string{"⭐", "🚀", "🐳", "✅", "❌", "🏁", "☁", "💬"} {
		if strings.HasPrefix(trimmed, marker) {
			return true
		}
	}
	return false
}
