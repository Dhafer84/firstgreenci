package run

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ProbeTimeout bounds the checks made on Docker. "docker info" can hang for a
// long time when the daemon is starting or wedged, and a tool that freezes is
// worse than a tool that explains.
const ProbeTimeout = 15 * time.Second

// DockerNotInstalledError reports that the docker command was not found.
type DockerNotInstalledError struct{}

func (e *DockerNotInstalledError) Error() string { return "docker is not installed" }

// DockerNotRunningError reports that Docker is installed but its daemon
// cannot be reached, which on a desktop means the application is not started.
type DockerNotRunningError struct {
	// Detail is the message Docker itself printed, kept for --verbose and for
	// the report a user may send us.
	Detail string
}

func (e *DockerNotRunningError) Error() string {
	return fmt.Sprintf("the docker daemon is not reachable: %s", e.Detail)
}

// Docker runs the docker command for us.
type Docker struct {
	// Path is the docker executable. Tests set it to a stand-in.
	Path string
}

// FindDocker locates the docker command on the system.
func FindDocker() (*Docker, error) {
	path, err := exec.LookPath("docker")
	if err != nil {
		return nil, &DockerNotInstalledError{}
	}
	return &Docker{Path: path}, nil
}

// CheckDaemon reports whether the Docker daemon answers.
//
// "docker info" is the right probe: it fails with a non-zero status when the
// daemon is unreachable, while "docker version" still prints the client
// version and would need its output picked apart.
func (d *Docker) CheckDaemon(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, ProbeTimeout)
	defer cancel()

	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, d.Path, "info")
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		return &DockerNotRunningError{Detail: lastMeaningfulLine(stderr.String())}
	}
	return nil
}

// HasImage reports whether the image is already on the machine. It is asked
// before a run so that a download of more than a gigabyte can be announced
// rather than looking like the tool has frozen.
func (d *Docker) HasImage(ctx context.Context, reference string) bool {
	ctx, cancel := context.WithTimeout(ctx, ProbeTimeout)
	defer cancel()

	command := exec.CommandContext(ctx, d.Path, "image", "inspect", reference)
	return command.Run() == nil
}

// actContainerPrefix is how act names the containers it creates. Cleanup is
// scoped to that prefix, and further scoped to the containers that appeared
// during our own run, so that a run the user started elsewhere is never
// touched.
const actContainerPrefix = "act-"

// ActContainers lists the containers act is responsible for, running or not.
func (d *Docker) ActContainers(ctx context.Context) []string {
	ctx, cancel := context.WithTimeout(ctx, ProbeTimeout)
	defer cancel()

	command := exec.CommandContext(ctx, d.Path, "ps", "-aq", "--filter", "name="+actContainerPrefix)
	output, err := command.Output()
	if err != nil {
		return nil
	}

	var identifiers []string
	for _, line := range strings.Split(string(output), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			identifiers = append(identifiers, trimmed)
		}
	}
	return identifiers
}

// RemoveContainers deletes the given containers, stopping them if needed. It
// reports the ones it could not remove, so that the user can be told rather
// than reassured wrongly.
func (d *Docker) RemoveContainers(ctx context.Context, identifiers []string) []string {
	if len(identifiers) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, ProbeTimeout)
	defer cancel()

	var remaining []string
	for _, identifier := range identifiers {
		command := exec.CommandContext(ctx, d.Path, "rm", "--force", identifier)
		if err := command.Run(); err != nil {
			remaining = append(remaining, identifier)
		}
	}
	return remaining
}

// lastMeaningfulLine returns the last non-empty line of an output. Docker
// prints its reason on the last line, after any warning.
func lastMeaningfulLine(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return ""
}
