package configure_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/larsartmann/dependabot-auto-configure/pkg/configure"
	ef "github.com/larsartmann/go-error-family"
)

var errBoom = errors.New("boom")

// domainError is the typed-error contract every errors.go type implements.
type domainError interface {
	error
	ErrorFamily() ef.Family
	ErrorCode() string
	ErrorContext() map[string]string
}

// TestTypedErrorsCarryDomainContract pins the DDD error contract: every
// typed failure in this package carries a human message naming the
// user-facing value, a stable family, a machine-readable code, and
// structured context. New error types must join the table.
func TestTypedErrorsCarryDomainContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        domainError
		wantFamily ef.Family
		wantCode   string
		wantInMsg  string
		contextHas map[string]string
		unwrapInto error
	}{
		{
			name:       "shape detection is infrastructure",
			err:        &configure.ShapeDetectionError{Root: "/repo", Cause: errBoom},
			wantFamily: ef.Infrastructure,
			wantCode:   "shape.detect",
			wantInMsg:  "/repo",
			contextHas: map[string]string{"root": "/repo"},
			unwrapInto: errBoom,
		},
		{
			name:       "findings conversion is infrastructure",
			err:        &configure.FindingsConversionError{Tool: configure.ToolName, Cause: errBoom},
			wantFamily: ef.Infrastructure,
			wantCode:   "findings.convert",
			wantInMsg:  configure.ToolName,
			contextHas: map[string]string{"tool": configure.ToolName},
			unwrapInto: errBoom,
		},
		{
			name:       "config read is a rejection",
			err:        &configure.ConfigReadError{Path: "/repo/.github/dependabot.yml", Cause: errBoom},
			wantFamily: ef.Rejection,
			wantCode:   "config.read",
			wantInMsg:  "/repo/.github/dependabot.yml",
			contextHas: map[string]string{"path": "/repo/.github/dependabot.yml"},
			unwrapInto: errBoom,
		},
		{
			name: "config write is infrastructure",
			err: &configure.ConfigWriteError{
				Path:  "/repo/.github/dependabot.yml",
				Step:  configure.WriteStepWrite,
				Cause: errBoom,
			},
			wantFamily: ef.Infrastructure,
			wantCode:   "config.write",
			wantInMsg:  "/repo/.github/dependabot.yml",
			contextHas: map[string]string{
				"path": "/repo/.github/dependabot.yml",
				"step": string(configure.WriteStepWrite),
			},
			unwrapInto: errBoom,
		},
		{
			name: "api transport is transient",
			err: &configure.APITransportError{
				Op:    configure.TransportOpCall,
				URL:   "https://api.github.com/x",
				Cause: errBoom,
			},
			wantFamily: ef.Transient,
			wantCode:   "github.transport",
			wantInMsg:  "https://api.github.com/x",
			contextHas: map[string]string{"op": string(configure.TransportOpCall), "url": "https://api.github.com/x"},
			unwrapInto: errBoom,
		},
		{
			name: "unexpected 5xx status is transient",
			err: &configure.UnexpectedStatusError{
				URL:        "https://api.github.com/x",
				StatusCode: http.StatusBadGateway,
				Body:       "oops",
			},
			wantFamily: ef.Transient,
			wantCode:   "github.status",
			wantInMsg:  "https://api.github.com/x",
			contextHas: map[string]string{"url": "https://api.github.com/x"},
		},
		{
			name: "unexpected 4xx status is a rejection",
			err: &configure.UnexpectedStatusError{
				URL:        "https://api.github.com/x",
				StatusCode: http.StatusUnauthorized,
				Body:       "nope",
			},
			wantFamily: ef.Rejection,
			wantCode:   "github.status",
			wantInMsg:  "https://api.github.com/x",
			contextHas: map[string]string{"url": "https://api.github.com/x"},
		},
		{
			name:       "github request build is infrastructure",
			err:        &configure.GitHubRequestError{URL: "https://api.github.com/x", Cause: errBoom},
			wantFamily: ef.Infrastructure,
			wantCode:   "github.request",
			wantInMsg:  "https://api.github.com/x",
			contextHas: map[string]string{"url": "https://api.github.com/x"},
			unwrapInto: errBoom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.err.Error(); !strings.Contains(got, tt.wantInMsg) {
				t.Errorf("Error() = %q, want it to contain %q", got, tt.wantInMsg)
			}

			if got := tt.err.ErrorFamily(); got != tt.wantFamily {
				t.Errorf("ErrorFamily() = %v, want %v", got, tt.wantFamily)
			}

			if got := tt.err.ErrorCode(); got != tt.wantCode {
				t.Errorf("ErrorCode() = %q, want %q", got, tt.wantCode)
			}

			context := tt.err.ErrorContext()

			for key, want := range tt.contextHas {
				if got, ok := context[key]; !ok || got != want {
					t.Errorf("ErrorContext()[%q] = %q, %v, want %q", key, got, ok, want)
				}
			}

			if tt.unwrapInto != nil && !errors.Is(tt.err, tt.unwrapInto) {
				t.Errorf("errors.Is(err, cause) = false, want the cause chain to survive")
			}
		})
	}
}

// TestErrorContextIsSerializable guards the machine-readable half of the
// contract: contexts feed log pipelines, so nil maps and empty values are
// bugs, not style.
func TestErrorContextIsSerializable(t *testing.T) {
	t.Parallel()

	errs := []domainError{
		&configure.ShapeDetectionError{Root: "/repo", Cause: errBoom},
		&configure.FindingsConversionError{Tool: "t", Cause: errBoom},
		&configure.ConfigReadError{Path: "/p", Cause: errBoom},
		&configure.ConfigWriteError{Path: "/p", Step: configure.WriteStepCreateDirectory, Cause: errBoom},
		&configure.APITransportError{Op: configure.TransportOpCloseResponse, URL: "u", Cause: errBoom},
		&configure.UnexpectedStatusError{URL: "u", StatusCode: 500, BodyErr: io.ErrUnexpectedEOF},
		&configure.GitHubRequestError{URL: "u", Cause: errBoom},
	}

	for _, err := range errs {
		for key, value := range err.ErrorContext() {
			if key == "" || value == "" {
				t.Errorf("%T context has empty key or value: %q=%q", err, key, value)
			}
		}
	}
}
