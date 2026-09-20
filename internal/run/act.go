package run

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// The images a pipeline can run in.
//
// FaithfulImage carries Python, Node and the usual tooling, so that
// actions/setup-python and actions/setup-node behave as they do on GitHub. It
// is large, which is why the choice is asked once and then remembered.
// SmallImage is a fifth of the size but has no Python, so a Python project
// re-downloads a runtime on every run.
const (
	FaithfulImage = "catthehacker/ubuntu:act-latest"
	SmallImage    = "node:16-buster-slim"
	DefaultImage  = FaithfulImage
)

// ActNotInstalledError reports that the act command was not found.
type ActNotInstalledError struct{}

func (e *ActNotInstalledError) Error() string { return "act is not installed" }

// Act runs the act command for us.
type Act struct {
	// Path is the act executable. Tests set it to a stand-in.
	Path string
}

// FindAct locates the act command on the system.
func FindAct() (*Act, error) {
	path, err := exec.LookPath("act")
	if err != nil {
		return nil, &ActNotInstalledError{}
	}
	return &Act{Path: path}, nil
}

// Options describes one local run.
type Options struct {
	// Root is the project folder act runs in.
	Root string
	// Workflow is the path of the workflow file, relative to Root.
	Workflow string
	// Image is the container image the pipeline runs in.
	Image string
	// Pull asks act to refresh the image from the registry. It is left off
	// once the image is on the machine: act pulls on every run by default,
	// which costs a network round trip and breaks offline use.
	Pull bool
}

// Arguments returns the command line given to act.
//
// It is a function of its own so that a test can check the arguments without
// running anything, and so that --verbose can show the exact command.
func (o Options) Arguments() []string {
	return []string{
		// The generated workflow reacts to push and pull_request; push is
		// the event a beginner is really waiting on.
		"push",
		// Run this one file, not every workflow the project may hold.
		"-W", filepath.ToSlash(o.Workflow),
		// Always name the image. Without -P, act asks its own question on
		// the first run and writes the answer into the user's ~/.actrc. We
		// never touch someone's personal configuration.
		"-P", "ubuntu-latest=" + o.Image,
		fmt.Sprintf("--pull=%t", o.Pull),
		// Remove containers and volumes left behind by a failed run.
		"--rm",
		// Structured logs, so that steps and statuses are read rather than
		// guessed from decorated text.
		"--json",
	}
}

// cancelGrace is how long act is given to stop its container after Ctrl-C,
// before it is killed outright.
const cancelGrace = 15 * time.Second

// Command builds the act invocation, ready to run.
func (a *Act) Command(ctx context.Context, options Options) *exec.Cmd {
	command := exec.CommandContext(ctx, a.Path, options.Arguments()...)
	command.Dir = options.Root

	// On cancellation, exec kills the process outright by default. act would
	// then never remove the container it started, and it would keep running
	// after the user pressed Ctrl-C. Interrupting it instead lets it clean up.
	command.Cancel = func() error {
		if err := command.Process.Signal(os.Interrupt); err != nil {
			// Windows does not deliver an interrupt this way; there is
			// nothing gentler available there.
			return command.Process.Kill()
		}
		return nil
	}
	// If act ignores the interrupt, it is killed after the grace period
	// rather than hanging the command line for ever.
	command.WaitDelay = cancelGrace

	return command
}

// Execute runs act and hands every output line to onLine as it arrives.
//
// Lines are delivered during the run, not at the end: a single step can take
// a minute, and the user has to see that something is happening.
//
// The returned error is the exit status of act. A red pipeline is a non-zero
// status too, so the caller decides what it means by looking at what was
// parsed, not at the status alone.
func (a *Act) Execute(ctx context.Context, options Options, onLine func(string)) error {
	command := a.Command(ctx, options)

	// act writes its logs to stderr and the steps' own output to stdout.
	// Both are read through the same splitter so that the order of the lines
	// is the order they were produced in.
	splitter := &lineSplitter{onLine: onLine}
	command.Stdout = splitter
	command.Stderr = splitter

	err := command.Run()
	splitter.flush()
	return err
}

// lineSplitter turns a stream of bytes into whole lines. Writes arrive in
// arbitrary chunks, so a partial line is held until its newline shows up.
type lineSplitter struct {
	mutex   sync.Mutex
	pending bytes.Buffer
	onLine  func(string)
}

func (s *lineSplitter) Write(chunk []byte) (int, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.pending.Write(chunk)

	for {
		line, err := s.pending.ReadString('\n')
		if err != nil {
			// No newline yet: put the fragment back and wait for more.
			s.pending.Reset()
			s.pending.WriteString(line)
			break
		}
		s.emit(line)
	}

	return len(chunk), nil
}

// flush delivers a last line that ended without a newline.
func (s *lineSplitter) flush() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.pending.Len() > 0 {
		s.emit(s.pending.String())
		s.pending.Reset()
	}
}

func (s *lineSplitter) emit(line string) {
	if s.onLine == nil {
		return
	}
	s.onLine(strings.TrimRight(line, "\r\n"))
}
