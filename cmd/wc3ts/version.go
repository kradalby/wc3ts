//nolint:forbidigo // CLI output uses fmt.Print
package main

import (
	"context"
	"fmt"

	"github.com/kradalby/wc3ts/version"
	"github.com/peterbourgon/ff/v3/ffcli"
)

func newVersionCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "version",
		ShortUsage: "wc3ts version",
		ShortHelp:  "Print version information",
		Exec: func(_ context.Context, _ []string) error {
			v := version.Get()
			fmt.Printf("wc3ts %s\n", v.String())

			if v.GoVer != "" {
				fmt.Printf("  go: %s\n", v.GoVer)
			}

			return nil
		},
	}
}
