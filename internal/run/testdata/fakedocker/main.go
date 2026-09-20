// Command fakedocker stands in for the docker command in tests.
//
// It is a real executable rather than a shell script so that the tests run
// unchanged on Windows. Its behaviour is driven by environment variables, so
// one build covers every case.
package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) < 2 {
		os.Exit(2)
	}

	switch os.Args[1] {
	case "info":
		// The daemon prints its reason on stderr and exits non-zero.
		if message := os.Getenv("FAKE_DOCKER_INFO_STDERR"); message != "" {
			fmt.Fprintln(os.Stderr, message)
		}
		os.Exit(exitCode("FAKE_DOCKER_INFO_EXIT"))

	case "image":
		os.Exit(exitCode("FAKE_DOCKER_IMAGE_EXIT"))

	case "ps":
		// The list has to change between the two calls: the cleanup only
		// touches containers that appeared during the run.
		fmt.Print(listContainers())
		os.Exit(0)

	case "rm":
		os.Exit(exitCode("FAKE_DOCKER_RM_EXIT"))

	default:
		os.Exit(0)
	}
}

// listContainers answers the first call with FAKE_DOCKER_PS_FIRST and every
// later one with FAKE_DOCKER_PS_LATER. The count is kept in a file, because
// each call is a separate process.
func listContainers() string {
	counter := os.Getenv("FAKE_DOCKER_PS_COUNTER")
	if counter == "" {
		return os.Getenv("FAKE_DOCKER_PS_FIRST")
	}

	if _, err := os.Stat(counter); err != nil {
		_ = os.WriteFile(counter, []byte("seen"), 0o644)
		return os.Getenv("FAKE_DOCKER_PS_FIRST")
	}
	return os.Getenv("FAKE_DOCKER_PS_LATER")
}

// exitCode reads a status from the environment, defaulting to success.
func exitCode(name string) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil {
		return 0
	}
	return value
}
