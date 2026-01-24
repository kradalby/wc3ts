package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func main() {
	runCmd := newRunCommand()

	root := &ffcli.Command{
		ShortUsage: "wc3ts <subcommand> [flags]",
		ShortHelp:  "WC3 LAN game proxy over Tailscale",
		Subcommands: []*ffcli.Command{
			runCmd,
			newProbeCommand(),
			newVersionCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			// Default to run command when no subcommand is specified
			return runCmd.Exec(ctx, args)
		},
	}

	err := root.ParseAndRun(context.Background(), os.Args[1:])
	if err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
