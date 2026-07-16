// Command apifox-api is the Go rewrite of the Apifox OpenAPI CLI.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"apifox-api/go-version/internal/cli"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return cli.ExitFailure
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return cli.ExitFailure
	}

	dependencies := cli.Dependencies{
		Streams: cli.Streams{
			In:  os.Stdin,
			Out: os.Stdout,
			Err: os.Stderr,
		},
		CWD:     cwd,
		HomeDir: homeDir,
		Env:     envMapFromOS(),
	}

	return cli.Run(ctx, dependencies, args)
}

func envMapFromOS() map[string]string {
	env := map[string]string{}
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env
}
