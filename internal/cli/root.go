// Package cli implements the dependabot-auto-configure command line.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/larsartmann/dependabot-auto-configure/pkg/configure"
	"github.com/spf13/cobra"
	"charm.land/fang/v2"
)

// Exit codes: 0 = nothing to do or repaired, 1 = changes needed (--check),
// 2 = operational error.
const (
	exitOK      = 0
	exitChanges = 1
	exitError   = 2
)

// Version is overridden at build time via -ldflags.
var Version = "dev"

// Execute runs the CLI and returns the process exit code.
func Execute(ctx context.Context) int {
	var (
		root       string
		configPath string
		check      bool
		dryRun     bool
	)

	rootCmd := &cobra.Command{
		Use:   "dependabot-auto-configure",
		Short: "Auto-configure .github/dependabot.yml for the detected repository shape",
		Long: "Detects Go modules, GitHub Actions workflows, and npm, then generates or repairs " +
			".github/dependabot.yml with weekly grouped updates and an explicit PR limit. " +
			"Existing non-canonical choices (schedules, limits, extra entries) are preserved; " +
			"configs with unknown constructs are reported but never rewritten.",
		Version: Version,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := configure.Run(ctx, configure.Options{
				Root:       root,
				ConfigPath: configPath,
				Check:      check,
				DryRun:     dryRun,
			})
			if err != nil {
				return err
			}

			for _, f := range result.Findings {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", f.Rule, f.Message)
				if f.Suggestion != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  fix: %s\n", f.Suggestion)
				}
			}

			switch {
			case result.Wrote:
				fmt.Fprintln(cmd.OutOrStdout(), "wrote .github/dependabot.yml")
			case result.PlannedWrite:
				fmt.Fprintln(cmd.OutOrStdout(), "changes planned (held back by --check/--dry-run)")
			case result.UnsafeRepair:
				fmt.Fprintln(cmd.OutOrStdout(), "config uses unknown constructs; repair is suggest-only")
			case result.Unchanged && len(result.Findings) == 0:
				fmt.Fprintln(cmd.OutOrStdout(), "configuration already canonical")
			}

			if check && result.ChangesNeeded() {
				os.Exit(exitChanges)
			}

			return nil
		},
	}

	rootCmd.Flags().StringVar(&root, "root", ".", "repository root directory")
	rootCmd.Flags().StringVar(&configPath, "config-path", configure.DefaultConfigPath, "configuration file path relative to --root")
	rootCmd.Flags().BoolVar(&check, "check", false, "report pending changes without writing; exit 1 when changes are needed")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the planned write without performing it")

	if err := fang.Execute(ctx, rootCmd, fang.WithVersion(Version)); err != nil {
		fmt.Fprintln(os.Stderr, err)

		return exitError
	}

	return exitOK
}
