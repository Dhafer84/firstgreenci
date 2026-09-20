// Command fakeact stands in for act in tests: it prints what it is told to
// print, on the stream it is told to use, then exits with the asked status.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	// Record the arguments so that a test can check what act would receive.
	if path := os.Getenv("FAKE_ACT_ARGS_FILE"); path != "" {
		_ = os.WriteFile(path, []byte(strings.Join(os.Args[1:], "\n")), 0o644)
	}

	// Replaying a recording lets a test drive the whole command with the
	// output of a real act run, without Docker and without the network.
	if path := os.Getenv("FAKE_ACT_RECORDING"); path != "" {
		recorded, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
		os.Stdout.Write(recorded)
	}

	for _, line := range splitLines(os.Getenv("FAKE_ACT_STDOUT")) {
		fmt.Fprintln(os.Stdout, line)
	}
	for _, line := range splitLines(os.Getenv("FAKE_ACT_STDERR")) {
		fmt.Fprintln(os.Stderr, line)
	}
	// A last fragment with no newline, to exercise the flush.
	if tail := os.Getenv("FAKE_ACT_TAIL"); tail != "" {
		fmt.Fprint(os.Stdout, tail)
	}

	// Telling the test that the run has really begun lets it cancel at the
	// right moment, instead of racing a timer.
	if path := os.Getenv("FAKE_ACT_STARTED_FILE"); path != "" {
		_ = os.WriteFile(path, []byte("started"), 0o644)
	}

	// Staying alive for a moment lets a test cancel the run while it is
	// going on, which is what Ctrl-C does.
	if pause, err := strconv.Atoi(os.Getenv("FAKE_ACT_SLEEP_MS")); err == nil && pause > 0 {
		time.Sleep(time.Duration(pause) * time.Millisecond)
	}

	status, err := strconv.Atoi(os.Getenv("FAKE_ACT_EXIT"))
	if err != nil {
		status = 0
	}
	os.Exit(status)
}

func splitLines(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, "|")
}
