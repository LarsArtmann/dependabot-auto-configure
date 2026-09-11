package cli

import (
	ef "github.com/larsartmann/go-error-family"
)

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
