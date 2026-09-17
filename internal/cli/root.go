// Package cli implements the dependabot-auto-configure command line.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"charm.land/fang/v2"
	"github.com/larsartmann/dependabot-auto-configure/pkg/configure"
	"github.com/larsartmann/go-finding"
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

// failOnPolicy decides whether pending findings trip the exit code under
// --check: any pending change (default), a minimum finding severity, or
// never.
type failOnPolicy struct {
	threshold finding.Severity
	anyChange bool
	never     bool
}

// parseFailOn interprets a --fail-on value: "any" (default), "none", or a
// finding severity name accepted by go-finding (error, warning, info,
// critical, and common aliases).
func parseFailOn(value string) (failOnPolicy, error) {
	switch value {
	case "", "any":
		return failOnPolicy{anyChange: true}, nil
	case "none":
		return failOnPolicy{never: true}, nil
	}

	threshold, err := finding.ParseSeverity(value)
	if err != nil {
		return failOnPolicy{}, &FlagValueError{Flag: "--fail-on", Value: value, Cause: err}
	}

	return failOnPolicy{threshold: threshold}, nil
}

// exceeded reports whether the run's pending work meets the policy.
func (p failOnPolicy) exceeded(result configure.Result) bool {
	switch {
	case p.anyChange:
		return result.ChangesNeeded()
	case p.never:
		return false
	}

	for _, f := range result.Findings {
		if f.Severity.GreaterThanOrEqual(p.threshold) {
			return true
		}
	}

	return false
}

// newRootCmd builds the command. The pointed-to int receives the process
// exit code (default 0, 1 when --check trips the --fail-on policy; Execute
// maps errors to 2), so tests can invoke the command without os.Exit.
func newRootCmd() (*cobra.Command, *int) {
	code := exitOK

	var (
		root       string
		configPath string
		check      bool
		dryRun     bool
		jsonOut    bool
		secFixes   bool
		failOn     string
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
			policy, err := parseFailOn(failOn)
			if err != nil {
				return err
			}

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

			if jsonOut {
				if err := reportJSON(cmd, result); err != nil {
					return err
				}
			} else if err := reportText(cmd, result); err != nil {
				return err
			}

			if check && policy.exceeded(result) {
				code = exitChanges
			}

			return nil
		},
	}

	rootCmd.Flags().StringVar(&root, "root", ".", "repository root directory")
	rootCmd.Flags().StringVar(&configPath, "config-path", configure.DefaultConfigPath, "configuration file path relative to --root")
	rootCmd.Flags().BoolVar(&check, "check", false, "report pending changes without writing; exit 1 when the --fail-on policy is exceeded")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the planned write without performing it")
	rootCmd.Flags().BoolVar(&jsonOut, "json", false, "print the result as JSON instead of text")
	rootCmd.Flags().BoolVar(&secFixes, "enable-security-fixes", false, "also enable Dependabot security updates via the GitHub API (needs GITHUB_TOKEN/GH_TOKEN)")
	rootCmd.Flags().StringVar(&failOn, "fail-on", "any", "under --check, the minimum finding severity that fails the run: any (default), none, or a severity (error, warning, info, critical)")

	return rootCmd, &code
}

// reportJSON prints the result as machine-readable JSON.
func reportJSON(cmd *cobra.Command, result configure.Result) error {
	out, err := configure.MarshalJSONResult(result)
	if err != nil {
		return err
	}

	stdout := cmd.OutOrStdout()
	if _, writeErr := stdout.Write(append(out, '\n')); writeErr != nil {
		return &OutputError{Stream: "stdout", Cause: writeErr}
	}

	return nil
}

// reportText prints findings and the run status as human-readable lines.
func reportText(cmd *cobra.Command, result configure.Result) error {
	stdout := cmd.OutOrStdout()

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

	status := statusLine(result)
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

	return nil
}

// statusLine summarizes the run outcome in one line, or "" when there is
// nothing to say.
func statusLine(result configure.Result) string {
	switch {
	case result.Wrote:
		return "wrote .github/dependabot.yml"
	case result.PlannedWrite:
		return "changes planned (held back by --check/--dry-run)"
	case result.UnsafeRepair:
		return "config uses unknown constructs; repair is suggest-only"
	case result.Unchanged && len(result.Findings) == 0:
		return "configuration already canonical"
	}

	return ""
}
