// Command writ is the CLI for creating and inspecting writs: the agreed
// scope of a piece of work, approved before any code exists.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// defaultVersion, defaultCommit, and defaultDate are the placeholders shown
// when a build does not inject values via -ldflags (e.g. plain `go build` or
// `go install`).
const (
	defaultVersion = "dev"
	defaultCommit  = "none"
	defaultDate    = "unknown"
)

// version, commit, and date are set at build time via
// -ldflags "-X main.version=... -X main.commit=... -X main.date=...".
// They must remain package-level vars: the linker's -X flag can only
// overwrite a var of type string, not a const.
var (
	version = defaultVersion
	commit  = defaultCommit
	date    = defaultDate
)

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

	// --version / -v on the root must answer with the same line as
	// `writ version` (including goreleaser-stamped commit/date), so scripts
	// probing either form agree on the binary's identity.
	root.Version = versionString(version, commit, date)
	root.SetVersionTemplate("{{.Version}}\n")

	root.AddCommand(newVersionCmd())
	root.AddCommand(newProposeCmd())
	root.AddCommand(newApproveCmd())
	root.AddCommand(newAttestCmd())
	root.AddCommand(newUnattestCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newMergeCmd())
	root.AddCommand(newDiscardCmd())

	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the writ version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), versionString(version, commit, date))
			return nil
		},
	}
}

// versionString renders the version line. When commit and date are still at
// their build-time defaults (a plain `go build` or `go install`), it omits
// the parenthetical rather than printing empty parentheses.
func versionString(version, commit, date string) string {
	if commit == defaultCommit && date == defaultDate {
		return fmt.Sprintf("writ version %s", version)
	}
	return fmt.Sprintf("writ version %s (commit %s, built %s)", version, commit, date)
}
