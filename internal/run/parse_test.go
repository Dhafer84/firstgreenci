package run_test

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dhafer84/firstgreenci/internal/run"
)

// feedRecording replays a recorded act run through the parser, the same way
// the real output is fed while act is running.
func feedRecording(t *testing.T, name string) (run.Result, []run.Step) {
	t.Helper()

	file, err := os.Open(filepath.Join("..", "..", "testdata", "act", name))
	if err != nil {
		t.Fatalf("open the recording: %v", err)
	}
	defer file.Close()

	var announced []run.Step
	parser := run.NewParser()
	parser.OnStep = func(step run.Step) { announced = append(announced, step) }

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		parser.Line(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read the recording: %v", err)
	}

	return parser.Result(), announced
}

// TestParseDockerUnreachable works on a real act run, recorded against a
// socket that does not exist. It is the shape of the failure a user hits when
// Docker is not started.
func TestParseDockerUnreachable(t *testing.T) {
	result, announced := feedRecording(t, "docker-unreachable.json.log")

	if !result.Ran() {
		t.Error("act announced a job result, so Ran should be true")
	}
	if result.Succeeded() {
		t.Error("the run failed, but Succeeded reports true")
	}
	if result.Job != run.StatusFailure {
		t.Errorf("Job = %q, want %q", result.Job, run.StatusFailure)
	}

	failed, ok := result.FailedStep()
	if !ok {
		t.Fatal("no failed step found")
	}
	if failed.Name != "Set up job" {
		t.Errorf("the failed step is %q, want %q", failed.Name, "Set up job")
	}
	if !failed.Internal {
		t.Error("\"Set up job\" is one of act's own steps and should be marked internal")
	}

	if len(announced) != 1 {
		t.Errorf("%d steps were announced live, want 1", len(announced))
	}

	// The reason has to survive all the way to the error rules.
	joined := strings.Join(result.Errors, "\n")
	if !strings.Contains(joined, "failed to connect to the docker API") {
		t.Errorf("the reason was lost; collected errors:\n%s", joined)
	}

	// act's decorated summary lines are noise in front of a French speaker.
	for _, step := range result.Steps {
		for _, line := range step.Output {
			if strings.Contains(line, "❌") || strings.Contains(line, "⭐") || strings.Contains(line, "🐳") {
				t.Errorf("a decorated line was kept as step output: %q", line)
			}
		}
	}
}

// TestParseKeepsEveryLine checks that nothing is dropped, including the lines
// act prints outside JSON: --verbose has to be able to show the whole run.
func TestParseKeepsEveryLine(t *testing.T) {
	result, _ := feedRecording(t, "docker-unreachable.json.log")

	raw := strings.Join(result.Raw, "\n")
	if !strings.Contains(raw, "Apple M-series chip") {
		t.Error("the non-JSON warning act prints was dropped")
	}
	if !strings.Contains(raw, "Error: unable to determine if image already exists") {
		t.Error("the final line, which is not JSON either, was dropped")
	}
}

// TestParseIgnoresNoise checks the lines that are not act log lines at all.
func TestParseIgnoresNoise(t *testing.T) {
	parser := run.NewParser()
	for _, line := range []string{"", "   ", "not json at all", "{broken", "{}"} {
		parser.Line(line)
	}

	result := parser.Result()
	if len(result.Steps) != 0 {
		t.Errorf("%d steps were invented out of noise", len(result.Steps))
	}
	if result.Ran() {
		t.Error("Ran reports true although act never announced a verdict")
	}
	if len(result.Raw) != 5 {
		t.Errorf("Raw holds %d lines, want 5", len(result.Raw))
	}
}

// TestParseGreenRun works on a real, fully green act run. It pins down the
// order of the report, which is not the order act mentions the steps in.
func TestParseGreenRun(t *testing.T) {
	result, announced := feedRecording(t, "green.json.log")

	if !result.Succeeded() {
		t.Fatalf("Job = %q, want %q", result.Job, run.StatusSuccess)
	}
	if _, failed := result.FailedStep(); failed {
		t.Error("a failed step was found in a green run")
	}

	// act logs an action while it is still being fetched, so "Installer
	// Python" is mentioned before "Récupérer le code" has run. The report
	// must follow the order the steps finished in.
	var visible []string
	for _, step := range result.Steps {
		if !step.Internal {
			visible = append(visible, step.Name)
		}
	}

	want := []string{"Récupérer le code", "Installer Python", "Installer les dépendances", "Lancer les tests"}
	if len(visible) != len(want) {
		t.Fatalf("visible steps = %v, want %v", visible, want)
	}
	for i := range want {
		if visible[i] != want[i] {
			t.Errorf("step %d is %q, want %q", i, visible[i], want[i])
		}
	}

	// setup-python runs a second time at the end of the job, under the same
	// name. That second pass belongs to act, not to the user's pipeline.
	if count := countNamed(result.Steps, "Installer Python"); count != 2 {
		t.Errorf("%q appears %d times, want 2 (the step and act's closing pass)", "Installer Python", count)
	}

	// Every visible step was announced while the run was going on.
	if len(announced) < len(want) {
		t.Errorf("%d steps were announced live, want at least %d", len(announced), len(want))
	}

	// act measures each step precisely; the report should use that.
	for _, step := range result.Steps {
		if step.Name == "Lancer les tests" && step.Duration <= 0 {
			t.Error("the duration of the test step was not read")
		}
	}
}

// TestParseKeepsTestOutput checks that what pytest printed survives, since the
// error rules are matched against it.
func TestParseKeepsTestOutput(t *testing.T) {
	result, _ := feedRecording(t, "red.json.log")

	failed, ok := result.FailedStep()
	if !ok {
		t.Fatal("no failed step found in a red run")
	}
	if failed.Name != "Lancer les tests" {
		t.Errorf("the failed step is %q, want %q", failed.Name, "Lancer les tests")
	}

	output := strings.Join(failed.Output, "\n")
	if !strings.Contains(output, "assert (1 + 1) == 3") {
		t.Errorf("what pytest printed was lost:\n%s", output)
	}
}

func countNamed(steps []run.Step, name string) int {
	count := 0
	for _, step := range steps {
		if step.Name == name {
			count++
		}
	}
	return count
}
