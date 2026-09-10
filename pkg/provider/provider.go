// Package provider wires dependabot-auto-configure into BuildFlow's DAG via
// the buildflow/tool-sdk Spec contract. BuildFlow discovers this Provider
// automatically through toolsdk.All() when a consumer blank-imports this
// package:
//
//	import _ "github.com/larsartmann/dependabot-auto-configure/pkg/provider"
package provider

import (
	"context"
	"fmt"

	"github.com/larsartmann/dependabot-auto-configure/pkg/configure"
	toolsdk "github.com/larsartmann/buildflow/tool-sdk"
	"github.com/larsartmann/go-finding"
)

// workingDir resolves the project directory from the context, falling back
// to the process working directory, mirroring go-structure-linter's
// provider so BuildFlow's WithWorkingDir fan-out works identically.
func workingDir(ctx context.Context) string {
	if dir := finding.WorkingDirFromContext(ctx); dir != "" {
		return dir
	}

	return "."
}

//nolint:gochecknoglobals // BuildFlow plugin SDK requires package-level Provider registration
var Provider = toolsdk.Register(toolsdk.Spec{
	Name: configure.ToolName,
	Description: "Detects a missing or under-configured .github/dependabot.yml and repairs it: " +
		"weekly grouped updates for every Go module, GitHub Actions, and npm",
	Trigger: toolsdk.AnyLanguage(
		"**/go.mod",
		"package.json",
		".github/workflows/*.yml",
		".github/workflows/*.yaml",
	),
	Inputs: []string{
		"**/go.mod",
		"package.json",
		".github/workflows/*.yml",
		".github/workflows/*.yaml",
		".github/dependabot.yml",
	},
	Detect: finding.NamedDetectorFunc(configure.ToolName, func(ctx context.Context) ([]finding.Finding, error) {
		result, err := configure.Run(ctx, configure.Options{Root: workingDir(ctx), Check: true})
		if err != nil {
			return nil, fmt.Errorf("%s detect: %w", configure.ToolName, err)
		}

		return result.Findings, nil
	}),
	Repair: toolsdk.RepairerFunc(func(ctx context.Context) (toolsdk.RepairResult, error) {
		result, err := configure.Run(ctx, configure.Options{
			Root:   workingDir(ctx),
			DryRun: toolsdk.DryRunFromContext(ctx),
		})
		if err != nil {
			return toolsdk.RepairResult{}, fmt.Errorf("%s repair: %w", configure.ToolName, err)
		}

		switch {
		case result.UnsafeRepair:
			return toolsdk.RepairResult{
				Description: "configuration uses constructs outside the known schema; repair is suggest-only, see findings",
			}, nil
		case result.Wrote:
			return toolsdk.RepairResult{Description: "wrote grouped weekly .github/dependabot.yml"}, nil
		case result.Unchanged && len(result.Findings) == 0:
			return toolsdk.RepairResult{Description: "configuration already canonical, nothing to do"}, nil
		default:
			return toolsdk.RepairResult{
				Description: fmt.Sprintf("held back by dry-run: %d finding(s) pending", len(result.Findings)),
			}, nil
		}
	}),
})
