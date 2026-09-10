// Package configure orchestrates detection, diffing, and repair for a
// repository's Dependabot configuration. It is the single entry point the
// CLI and the BuildFlow provider share, so both always behave identically.
package configure

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"

	"github.com/larsartmann/dependabot-auto-configure/pkg/dependabot"
	"github.com/larsartmann/dependabot-auto-configure/pkg/detect"
	"github.com/larsartmann/go-atomic-write"
	"github.com/larsartmann/go-finding"
	autoconfigure "github.com/larsartmann/linter-autoconfigure-sdk"
)

// ToolName is this tool's identity in findings and BuildFlow.
const ToolName = "dependabot-auto-configure"

// DefaultConfigPath is the only location GitHub reads Dependabot
// configuration from, relative to the repository root.
const DefaultConfigPath = ".github/dependabot.yml"

// Options controls one configure run.
type Options struct {
	// Root is the repository root directory.
	Root string
	// ConfigPath overrides DefaultConfigPath (tests only; GitHub never
	// reads another location).
	ConfigPath string
	// Check reports what would change without writing, for CI gates.
	Check bool
	// DryRun prints the planned write without performing it.
	DryRun bool
}

// Result reports what a run found and did.
type Result struct {
	// Findings is every issue detected, converted through the
	// linter-autoconfigure-sdk so suggestions arrive as fixable findings.
	Findings []finding.Finding
	// Wrote is true when the configuration file was actually written.
	Wrote bool
	// PlannedWrite is true when Check or DryRun held back a real write.
	PlannedWrite bool
	// Unchanged is true when the desired configuration already matches
	// the file byte for byte.
	Unchanged bool
	// UnsafeRepair is true when the existing configuration contains
	// constructs this tool does not model; repair becomes suggest-only so
	// a rewrite can never silently drop user customizations.
	UnsafeRepair bool
}

// ChangesNeeded reports whether any finding or planned write is pending.
func (r Result) ChangesNeeded() bool {
	return len(r.Findings) > 0 || r.PlannedWrite
}

// Run detects the repository shape, compares it against the configuration
// file, and repairs the file unless Check or DryRun is set.
func Run(ctx context.Context, opts Options) (Result, error) {
	result := Result{}

	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = DefaultConfigPath
	}

	shape, err := detect.NewDetector(opts.Root).Shape()
	if err != nil {
		return result, fmt.Errorf("detect repository shape in %s: %w", opts.Root, err)
	}

	desired, cap := dependabot.Generate(shape)

	absConfig := filepath.Join(opts.Root, filepath.FromSlash(configPath))

	data, readErr := os.ReadFile(absConfig)
	switch {
	case errors.Is(readErr, fs.ErrNotExist):
		issues := dependabot.Diff(nil, dependabot.DecodeResult{}, desired, cap)
		result.Findings, err = autoconfigure.FindingsFromIssues(ToolName, issues)
		if err != nil {
			return result, fmt.Errorf("convert findings: %w", err)
		}

		if len(desired.Updates) == 0 {
			result.Unchanged = true

			return result, nil
		}

		out, encErr := desired.Encode()
		if encErr != nil {
			return result, encErr
		}

		return result, planOrWrite(&result, opts, absConfig, out)
	case readErr != nil:
		return result, fmt.Errorf("read %s: %w", absConfig, readErr)
	}

	dec, decErr := dependabot.Decode(data)
	if decErr != nil {
		return result, decErr
	}

	existing := dec.Config

	issues := dependabot.Diff(&existing, dec, desired, cap)
	result.Findings, err = autoconfigure.FindingsFromIssues(ToolName, issues)
	if err != nil {
		return result, fmt.Errorf("convert findings: %w", err)
	}

	if dec.Unsafe {
		result.UnsafeRepair = true

		return result, nil
	}

	reconciled := dependabot.Reconcile(existing, desired)

	if reflect.DeepEqual(existing, reconciled) {
		result.Unchanged = true

		return result, nil
	}

	out, encErr := reconciled.Encode()
	if encErr != nil {
		return result, encErr
	}

	if bytes.Equal(out, data) {
		result.Unchanged = true

		return result, nil
	}

	return result, planOrWrite(&result, opts, absConfig, out)
}

// planOrWrite centralizes the Check/DryRun gate: both hold the write back
// and mark it as planned; only a real run performs the atomic write.
func planOrWrite(result *Result, opts Options, absConfig string, out []byte) error {
	if opts.Check || opts.DryRun {
		result.PlannedWrite = true

		return nil
	}

	if err := os.MkdirAll(filepath.Dir(absConfig), 0o755); err != nil {
		return fmt.Errorf("create config directory %s: %w", filepath.Dir(absConfig), err)
	}

	if err := atomicwrite.Write(absConfig, out); err != nil {
		return fmt.Errorf("write %s: %w", absConfig, err)
	}

	result.Wrote = true

	return nil
}
