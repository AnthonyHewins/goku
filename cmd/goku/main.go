package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const appName = "goku"

var version string

var l = logger{os.Stderr}

func main() {
	root := cobra.Command{
		Use:     appName,
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("no base command")
		},
	}

	err := root.Execute()
	if err == nil {
		return
	}

	l.err(err.Error())
	switch {
	case errors.Is(err, context.Canceled):
		os.Exit(130)
	case errors.Is(err, context.DeadlineExceeded):
		os.Exit(124)
	default:
		os.Exit(1)
	}
}
