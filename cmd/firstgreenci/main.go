// Command firstgreenci takes a project and shows a green CI pipeline.
//
// It keeps nothing but the wiring: the real work lives in internal/cli, which
// can be exercised without starting a process.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Dhafer84/firstgreenci/internal/cli"
)

func main() {
	// Running a pipeline takes minutes and holds a container. Ctrl-C has to
	// reach act so that it stops the container, rather than killing us and
	// leaving it running.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(cli.Run(cli.Environment{
		Args:        os.Args[1:],
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		LookupEnv:   os.LookupEnv,
		Interactive: isInteractive(),
		Context:     ctx,
	}))
}

// isInteractive reports whether the standard input is a terminal, that is,
// whether a human is there to answer a question. When the tool runs from a
// script or a pipe, it explains what to pass on the command line instead of
// waiting for an answer that will never come.
func isInteractive() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
