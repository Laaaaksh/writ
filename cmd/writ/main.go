// Command writ is the CLI for creating and inspecting writs: the agreed
// scope of a piece of work, approved before any code exists.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is the writ CLI version. Later slices may override this at build
// time via -ldflags.
const version = "dev"

func main() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		var ec exitCodeErr
		if errors.As(err, &ec) {
			os.Exit(ec.code)
		}
		if !errors.Is(err, errSilent) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "writ",
		Short: "writ manages the agreed scope of a piece of work",
		Long: "writ replaces the large, context-stripped diff with a writ: an intent, checkable\n" +
			"acceptance criteria, a declared file scope, and a verification command, agreed\n" +
			"before any code exists.",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	root.AddCommand(newVersionCmd())
	root.AddCommand(newOpenCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newMergeCmd())

	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the writ version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "writ version %s\n", version)
			return nil
		},
	}
}
