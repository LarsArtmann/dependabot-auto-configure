package cli

import (
	ef "github.com/larsartmann/go-error-family"
)

// FlagValueError reports a flag value the command cannot interpret. The
// process did nothing: the request itself was malformed.
type FlagValueError struct {
	// Flag is the flag whose value was rejected (e.g. "--fail-on").
	Flag string
	// Value is the rejected value.
	Value string
	// Cause explains what the flag accepts.
	Cause error
}

// Error names the flag, the value, and what the flag accepts.
func (e *FlagValueError) Error() string {
	return "invalid value " + e.Value + " for " + e.Flag + ": " + e.Cause.Error()
}

// Unwrap exposes the underlying parse failure.
func (e *FlagValueError) Unwrap() error { return e.Cause }

// ErrorFamily classifies malformed flag values as user error.
func (e *FlagValueError) ErrorFamily() ef.Family { return ef.Rejection }

// ErrorCode returns the machine-readable identity of this failure.
func (e *FlagValueError) ErrorCode() string { return "cli.flag" }

// ErrorContext exposes the failure as structured, machine-readable data.
func (e *FlagValueError) ErrorContext() map[string]string {
	return map[string]string{"flag": e.Flag, "value": e.Value}
}

// OutputError reports that the CLI could not write its report to the
// terminal. The command's work already happened (detection, repair, API
// calls); only the reporting failed, which is an infrastructure fault,
// not a rejected request.
type OutputError struct {
	// Stream names the output stream that failed ("stdout", "stderr").
	Stream string
	// Cause is the underlying write failure.
	Cause error
}

// Error names the stream and the cause.
func (e *OutputError) Error() string {
	return "write " + e.Stream + ": " + e.Cause.Error()
}

// Unwrap exposes the write failure.
func (e *OutputError) Unwrap() error { return e.Cause }

// ErrorFamily classifies report-write failures as infrastructure faults.
func (e *OutputError) ErrorFamily() ef.Family { return ef.Infrastructure }

// ErrorCode returns the machine-readable identity of this failure.
func (e *OutputError) ErrorCode() string { return "cli.output" }

// ErrorContext exposes the failure as structured, machine-readable data.
func (e *OutputError) ErrorContext() map[string]string {
	return map[string]string{"stream": e.Stream}
}
