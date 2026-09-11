package dependabot

import (
	"strconv"
	"strings"

	ef "github.com/larsartmann/go-error-family"
)

// DecodeStage names the lenient pass that failed while reading an existing
// configuration document. Decode always runs two passes: parsing the
// document into a Config, then auditing its raw keys for constructs this
// tool does not model.
type DecodeStage string

// The two decode passes.
const (
	// DecodeStageParse fails when the document is not readable YAML.
	DecodeStageParse DecodeStage = "parse"
	// DecodeStageInspect fails when the raw-key audit cannot read the
	// document shape.
	DecodeStageInspect DecodeStage = "inspect"
)

// UnparseableConfigError reports a configuration document this tool cannot
// read at all. Unreadable configuration is corruption: repair must stay
// suggest-only because nothing about the document can be trusted. Callers
// currently downgrade this to a warning finding; the typed error exists so
// that decision is explicit and classifiable, not stringly guessed.
type UnparseableConfigError struct {
	// Stage names the decode pass that failed.
	Stage DecodeStage
	// Cause is the underlying YAML failure.
	Cause error
}

// Error renders the failure with the pass that failed and its cause.
func (e *UnparseableConfigError) Error() string {
	if e.Stage == DecodeStageInspect {
		return "inspect dependabot config keys: " + e.Cause.Error()
	}

	return "parse dependabot config: " + e.Cause.Error()
}

// Unwrap exposes the YAML failure for errors.Is/AsType inspection.
func (e *UnparseableConfigError) Unwrap() error { return e.Cause }

// ErrorFamily classifies unparseable configuration as corruption.
func (e *UnparseableConfigError) ErrorFamily() ef.Family { return ef.Corruption }

// ErrorCode returns the machine-readable identity of this failure.
func (e *UnparseableConfigError) ErrorCode() string { return "config.unparseable" }

// ErrorContext exposes the failure as structured, machine-readable data.
func (e *UnparseableConfigError) ErrorContext() map[string]string {
	return map[string]string{"stage": string(e.Stage)}
}

// EncodeError reports a Config that cannot be serialized back to YAML.
// Encoding a schema-valid Config should never fail; when it does, the
// document cannot be represented and callers must not write partial output.
type EncodeError struct {
	// Cause is the underlying encoder failure.
	Cause error
}

// Error renders the failure with its cause.
func (e *EncodeError) Error() string {
	return "encode dependabot config: " + e.Cause.Error()
}

// Unwrap exposes the encoder failure.
func (e *EncodeError) Unwrap() error { return e.Cause }

// ErrorFamily classifies unrepresentable documents as corruption.
func (e *EncodeError) ErrorFamily() ef.Family { return ef.Corruption }

// ErrorCode returns the machine-readable identity of this failure.
func (e *EncodeError) ErrorCode() string { return "config.encode" }

// RequiredField is a field the Dependabot schema mandates on every
// updates entry.
type RequiredField string

// Required entry fields.
const (
	FieldPackageEcosystem RequiredField = "package-ecosystem"
	FieldDirectory        RequiredField = "directory"
)

// InvalidEntryError reports one updates entry missing required fields.
// This is a rejection of malformed user input, never a transient fault:
// repair refuses to write a config containing such entries, so retrying
// cannot help — only fixing the entry can.
type InvalidEntryError struct {
	// Index is the position of the offending entry under "updates:".
	Index int
	// Missing names the required fields the entry does not carry.
	Missing []RequiredField
}

// Error names the entry position and exactly which fields are missing.
func (e *InvalidEntryError) Error() string {
	names := make([]string, 0, len(e.Missing))
	for _, field := range e.Missing {
		names = append(names, string(field))
	}

	return "invalid dependabot update entry: entry " +
		strconv.Itoa(e.Index) + " needs " + strings.Join(names, " and ")
}

// ErrorFamily classifies malformed entries as a rejection of user input.
func (e *InvalidEntryError) ErrorFamily() ef.Family { return ef.Rejection }

// ErrorCode returns the machine-readable identity of this failure.
func (e *InvalidEntryError) ErrorCode() string { return "config.entry.invalid" }

// ErrorContext exposes the failure as structured, machine-readable data.
func (e *InvalidEntryError) ErrorContext() map[string]string {
	return map[string]string{"entry_index": strconv.Itoa(e.Index)}
}
