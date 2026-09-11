package configure

import (
	"strconv"

	ef "github.com/larsartmann/go-error-family"
)

// ShapeDetectionError reports that the repository shape could not be read.
// Without a shape there is nothing to configure; the environment, not the
// user's configuration, is at fault.
type ShapeDetectionError struct {
	// Root is the repository directory detection ran against.
	Root string
	// Cause is the underlying detector failure.
	Cause error
}

// Error names the repository whose shape could not be read.
func (e *ShapeDetectionError) Error() string {
	return "detect repository shape in " + e.Root + ": " + e.Cause.Error()
}

// Unwrap exposes the detector failure.
func (e *ShapeDetectionError) Unwrap() error { return e.Cause }

// ErrorFamily classifies detection failures as infrastructure faults.
func (e *ShapeDetectionError) ErrorFamily() ef.Family { return ef.Infrastructure }

// ErrorCode returns the machine-readable identity of this failure.
func (e *ShapeDetectionError) ErrorCode() string { return "shape.detect" }

// ErrorContext exposes the failure as structured, machine-readable data.
func (e *ShapeDetectionError) ErrorContext() map[string]string {
	return map[string]string{"root": e.Root}
}

// FindingsConversionError reports that detected issues could not be
// converted into fixable findings through the linter-autoconfigure SDK.
// Detection already succeeded; only the report pipeline broke.
type FindingsConversionError struct {
	// Tool is the tool identity the conversion ran under.
	Tool string
	// Cause is the underlying SDK failure.
	Cause error
}

// Error names the tool whose findings could not be converted.
func (e *FindingsConversionError) Error() string {
	return "convert findings for " + e.Tool + ": " + e.Cause.Error()
}

// Unwrap exposes the SDK failure.
func (e *FindingsConversionError) Unwrap() error { return e.Cause }

// ErrorFamily classifies conversion failures as infrastructure faults.
func (e *FindingsConversionError) ErrorFamily() ef.Family { return ef.Infrastructure }

// ErrorCode returns the machine-readable identity of this failure.
func (e *FindingsConversionError) ErrorCode() string { return "findings.convert" }

// ErrorContext exposes the failure as structured, machine-readable data.
func (e *FindingsConversionError) ErrorContext() map[string]string {
	return map[string]string{"tool": e.Tool}
}

// ConfigReadError reports that the configuration file exists but could not
// be read (permissions, I/O). A missing file is the normal first-run state
// and never reaches this type.
type ConfigReadError struct {
	// Path is the absolute configuration file that could not be read.
	Path string
	// Cause is the underlying filesystem failure.
	Cause error
}

// Error names the file that could not be read.
func (e *ConfigReadError) Error() string {
	return "read " + e.Path + ": " + e.Cause.Error()
}

// Unwrap exposes the filesystem failure.
func (e *ConfigReadError) Unwrap() error { return e.Cause }

// ErrorFamily classifies unreadable configuration as a rejection: the
// operator controls the path and its permissions, so a retry cannot help.
func (e *ConfigReadError) ErrorFamily() ef.Family { return ef.Rejection }

// ErrorCode returns the machine-readable identity of this failure.
func (e *ConfigReadError) ErrorCode() string { return "config.read" }

// ErrorContext exposes the failure as structured, machine-readable data.
func (e *ConfigReadError) ErrorContext() map[string]string {
	return map[string]string{"path": e.Path}
}

// WriteStep names the filesystem step of materializing the configuration.
type WriteStep string

// The two materialization steps, in order.
const (
	// WriteStepCreateDirectory creates the configuration's parent directory.
	WriteStepCreateDirectory WriteStep = "create config directory"
	// WriteStepWrite performs the atomic write of the configuration file.
	WriteStepWrite WriteStep = "write"
)

// ConfigWriteError reports that the repaired configuration could not be
// materialized on disk. The desired document exists; only the filesystem
// refused it.
type ConfigWriteError struct {
	// Path is the target the failing step operated on.
	Path string
	// Step names the materialization step that failed.
	Step WriteStep
	// Cause is the underlying filesystem failure.
	Cause error
}

// Error names the step, the path, and the cause.
func (e *ConfigWriteError) Error() string {
	return string(e.Step) + " " + e.Path + ": " + e.Cause.Error()
}

// Unwrap exposes the filesystem failure.
func (e *ConfigWriteError) Unwrap() error { return e.Cause }

// ErrorFamily classifies write failures as infrastructure faults.
func (e *ConfigWriteError) ErrorFamily() ef.Family { return ef.Infrastructure }

// ErrorCode returns the machine-readable identity of this failure.
func (e *ConfigWriteError) ErrorCode() string { return "config.write" }

// ErrorContext exposes the failure as structured, machine-readable data.
func (e *ConfigWriteError) ErrorContext() map[string]string {
	return map[string]string{"path": e.Path, "step": string(e.Step)}
}

// SecurityFixesOutcome is the result of the optional security-fixes
// enablement call. The value is part of the CLI and --json contract
// (Result.SecurityFixes).
type SecurityFixesOutcome string

// Outcomes of EnableSecurityFixes.
const (
	SecurityFixesEnabled   SecurityFixesOutcome = "enabled"
	SecurityFixesNoToken   SecurityFixesOutcome = "skipped-no-token"
	SecurityFixesNoRemote  SecurityFixesOutcome = "skipped-no-remote"
	SecurityFixesNotFound  SecurityFixesOutcome = "skipped-repo-not-found"
	SecurityFixesForbidden SecurityFixesOutcome = "skipped-forbidden"
)

// TransportOp names the transport operation that failed while talking to
// the GitHub API.
type TransportOp string

// The transport operations.
const (
	// TransportOpCall performs the HTTP request.
	TransportOpCall TransportOp = "call"
	// TransportOpCloseResponse releases the response body.
	TransportOpCloseResponse TransportOp = "close response body of"
)

// APITransportError reports that talking to the GitHub API failed at the
// transport layer: the request could not be delivered or its response
// could not be released. This is the one failure class a retry may fix.
type APITransportError struct {
	// Op names the transport operation that failed.
	Op TransportOp
	// URL is the API endpoint involved.
	URL string
	// Cause is the underlying transport failure.
	Cause error
}

// Error names the operation, the endpoint, and the cause.
func (e *APITransportError) Error() string {
	return string(e.Op) + " " + e.URL + ": " + e.Cause.Error()
}

// Unwrap exposes the transport failure.
func (e *APITransportError) Unwrap() error { return e.Cause }

// ErrorFamily classifies transport failures as transient.
func (e *APITransportError) ErrorFamily() ef.Family { return ef.Transient }

// ErrorCode returns the machine-readable identity of this failure.
func (e *APITransportError) ErrorCode() string { return "github.transport" }

// ErrorContext exposes the failure as structured, machine-readable data.
func (e *APITransportError) ErrorContext() map[string]string {
	return map[string]string{"op": string(e.Op), "url": e.URL}
}

// UnexpectedStatusError reports that GitHub answered the security-fixes
// call with a status outside the modeled outcomes. Server-side statuses
// are transient (GitHub's fault); everything else rejects the credentials,
// permissions, or repository this tool was pointed at.
type UnexpectedStatusError struct {
	// URL is the API endpoint that answered.
	URL string
	// StatusCode is the unmodeled HTTP status.
	StatusCode int
	// Body is the (truncated) response body, read best-effort.
	Body string
	// BodyErr carries the read failure when the body was unreadable.
	BodyErr error
}

// Error reports the status, the endpoint, and whatever body detail exists.
func (e *UnexpectedStatusError) Error() string {
	detail := e.Body
	if e.BodyErr != nil {
		detail = "body unreadable: " + e.BodyErr.Error()
	}

	return "unexpected status " + strconv.Itoa(e.StatusCode) +
		" from " + e.URL + ": " + detail
}

// Unwrap exposes the body-read failure, when present.
func (e *UnexpectedStatusError) Unwrap() error {
	if e.BodyErr != nil {
		return e.BodyErr
	}

	return nil
}

// ErrorFamily derives the family from the status: 5xx is GitHub's fault
// and may succeed on retry, anything else is this invocation's fault.
func (e *UnexpectedStatusError) ErrorFamily() ef.Family {
	if e.StatusCode >= 500 {
		return ef.Transient
	}

	return ef.Rejection
}

// ErrorCode returns the machine-readable identity of this failure.
func (e *UnexpectedStatusError) ErrorCode() string { return "github.status" }

// ErrorContext exposes the failure as structured, machine-readable data.
func (e *UnexpectedStatusError) ErrorContext() map[string]string {
	return map[string]string{
		"url":    e.URL,
		"status": strconv.Itoa(e.StatusCode),
	}
}

// GitHubRequestError reports that the security-fixes request could not be
// built. This indicates a programming or environment defect, not user
// input: the endpoint shape is fixed in code.
type GitHubRequestError struct {
	// URL is the endpoint the request was being built for.
	URL string
	// Cause is the underlying construction failure.
	Cause error
}

// Error names the endpoint and the cause.
func (e *GitHubRequestError) Error() string {
	return "build request for " + e.URL + ": " + e.Cause.Error()
}

// Unwrap exposes the construction failure.
func (e *GitHubRequestError) Unwrap() error { return e.Cause }

// ErrorFamily classifies request construction failures as infrastructure.
func (e *GitHubRequestError) ErrorFamily() ef.Family { return ef.Infrastructure }

// ErrorCode returns the machine-readable identity of this failure.
func (e *GitHubRequestError) ErrorCode() string { return "github.request" }

// ErrorContext exposes the failure as structured, machine-readable data.
func (e *GitHubRequestError) ErrorContext() map[string]string {
	return map[string]string{"url": e.URL}
}
