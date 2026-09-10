// Package cli implements the dependabot-auto-configure command line.
package cli

import (
	"context"
	"fmt"
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
		_, _ = fmt.Fprintln(os.Stderr, err)

		if *code == exitChanges {
			return exitChanges
		}

		return exitError
	}

	return *code
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
				result.SecurityFixes, err = configure.EnableSecurityFixes(cmd.Context(), root)
				if err != nil {
					return err
				}
			}

			if jsonOut {
				out, marshalErr := configure.MarshalJSONResult(result)
				if marshalErr != nil {
					return marshalErr
				}

				_, _ = cmd.OutOrStdout().Write(append(out, []byte("
")))

				if check && result.ChangesNeeded() {
					code = exitChanges
				}

				return nil
			}

			for _, f := range result.Findings {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", f.Rule, f.Message)
				if f.Suggestion != "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  fix: %s\n", f.Suggestion)
				}
			}

			switch {
			case result.Wrote:
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "wrote .github/dependabot.yml")
			case result.PlannedWrite:
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "changes planned (held back by --check/--dry-run)")
			case result.UnsafeRepair:
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "config uses unknown constructs; repair is suggest-only")
			case result.Unchanged && len(result.Findings) == 0:
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "configuration already canonical")
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
