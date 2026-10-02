package detect

import (
	ef "github.com/larsartmann/go-error-family"
)

// UnreadableManifestError reports a package.json detection could not read
// or parse. Detection never guesses: without the file's contents, workspace
// membership is unknown, so the walk stops and the failure surfaces instead
// of silently producing a partial shape.
type UnreadableManifestError struct {
	// Path is the slash-separated manifest path relative to the root.
	Path string
	// Cause is the underlying read or JSON failure.
	Cause error
}

// Error renders the failing manifest path with its cause.
func (e *UnreadableManifestError) Error() string {
	return "read package.json " + e.Path + ": " + e.Cause.Error()
}

// Unwrap exposes the read or parse failure for errors.Is/AsType inspection.
func (e *UnreadableManifestError) Unwrap() error { return e.Cause }

// ErrorFamily classifies an unreadable manifest as corruption: the file
// exists but its content cannot be trusted.
func (e *UnreadableManifestError) ErrorFamily() ef.Family { return ef.Corruption }

// ErrorCode returns the machine-readable identity of this failure.
func (e *UnreadableManifestError) ErrorCode() string { return "shape.manifest_unreadable" }

// ErrorContext exposes the failure as structured, machine-readable data.
func (e *UnreadableManifestError) ErrorContext() map[string]string {
	return map[string]string{"manifest_path": e.Path}
}
