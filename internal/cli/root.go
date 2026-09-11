// Package cli implements the dependabot-auto-configure command line.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"charm.land/fang/v2"
	"github.com/larsartmann/dependabot-auto-configure/pkg/configure"
	"github.com/spf13/cobra"
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
	rootCmd, code := newRootCmd()
	if err := fang.Execute(ctx, rootCmd, fang.WithVersion(Version)); err != nil {
		if _, printErr := fmt.Fprintln(os.Stderr, err); printErr != nil {
			return exitError
		}

		if *code == exitChanges {
			return exitChanges
		}

		return exitError
	}

	return *code
}

// writeOut renders one line of the text report, surfacing write failures
// instead of dropping them: a report the user never saw did not happen.
func writeOut(w io.Writer, format string, args ...any) error {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		return &OutputError{Stream: "stdout", Cause: err}
	}

	return nil
}

// newRootCmd builds the command. The pointed-to int receives the process
// exit code (default 0, 1 when --check found changes; Execute maps errors
// to 2), so tests can invoke the command without os.Exit.
func newRootCmd() (*cobra.Command, *int) {
	code := exitOK

	var (
		root       string
		configPath string
		check      bool
		dryRun     bool
		jsonOut    bool
		secFixes   bool
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
			result, err := configure.Run(cmd.Context(), configure.Options{
				Root:                root,
				ConfigPath:          configPath,
				Check:               check,
				DryRun:              dryRun,
				EnableSecurityFixes: secFixes,
			})
			if err != nil {
				return err
			}

			if secFixes && !check && !dryRun {
				outcome, secErr := configure.EnableSecurityFixes(cmd.Context(), root)
				if secErr != nil {
					return secErr
				}

				result.SecurityFixes = string(outcome)
			}

			stdout := cmd.OutOrStdout()

			if jsonOut {
				out, marshalErr := configure.MarshalJSONResult(result)
				if marshalErr != nil {
					return marshalErr
				}

				if _, writeErr := stdout.Write(append(out, '\n')); writeErr != nil {
					return &OutputError{Stream: "stdout", Cause: writeErr}
				}

				if check && result.ChangesNeeded() {
					code = exitChanges
				}

				return nil
			}

			for _, f := range result.Findings {
				if err := writeOut(stdout, "%s: %s\n", f.Rule, f.Message); err != nil {
					return err
				}

				if f.Suggestion != "" {
					if err := writeOut(stdout, "  fix: %s\n", f.Suggestion); err != nil {
						return err
					}
				}
			}

			var status string
			switch {
			case result.Wrote:
				status = "wrote .github/dependabot.yml"
			case result.PlannedWrite:
				status = "changes planned (held back by --check/--dry-run)"
			case result.UnsafeRepair:
				status = "config uses unknown constructs; repair is suggest-only"
			case result.Unchanged && len(result.Findings) == 0:
				status = "configuration already canonical"
			}

			if status != "" {
				if err := writeOut(stdout, "%s\n", status); err != nil {
					return err
				}
			}

			if result.SecurityFixes != "" {
				if err := writeOut(stdout, "security fixes: %s\n", result.SecurityFixes); err != nil {
					return err
				}
			}

			if check && result.ChangesNeeded() {
				code = exitChanges
			}

			return nil
		},
	}

	rootCmd.Flags().StringVar(&root, "root", ".", "repository root directory")
	rootCmd.Flags().StringVar(&configPath, "config-path", configure.DefaultConfigPath, "configuration file path relative to --root")
	rootCmd.Flags().BoolVar(&check, "check", false, "report pending changes without writing; exit 1 when changes are needed")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the planned write without performing it")
	rootCmd.Flags().BoolVar(&jsonOut, "json", false, "print the result as JSON instead of text")
	rootCmd.Flags().BoolVar(&secFixes, "enable-security-fixes", false, "also enable Dependabot security updates via the GitHub API (needs GITHUB_TOKEN/GH_TOKEN)")

	return rootCmd, &code
}
