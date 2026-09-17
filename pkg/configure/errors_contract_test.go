package configure_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	ef "github.com/larsartmann/go-error-family"

	"github.com/larsartmann/dependabot-auto-configure/pkg/configure"
)

// TestTypedErrorsCarryDomainContract pins the DDD error contract: every
// typed failure in this package carries a human message naming the
// user-facing value, a stable family, a machine-readable code, and
// structured context. New error types must join the table.
func TestTypedErrorsCarryDomainContract(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("boom")

	type domainError interface {
		error
		ErrorFamily() ef.Family
		ErrorCode() string
		ErrorContext() map[string]string
	}

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
			err:        &configure.ShapeDetectionError{Root: "/repo", Cause: sentinel},
			wantFamily: ef.Infrastructure,
			wantCode:   "shape.detect",
			wantInMsg:  "/repo",
			contextHas: map[string]string{"root": "/repo"},
			unwrapInto: sentinel,
		},
		{
			name:       "findings conversion is infrastructure",
			err:        &configure.FindingsConversionError{Tool: configure.ToolName, Cause: sentinel},
			wantFamily: ef.Infrastructure,
			wantCode:   "findings.convert",
			wantInMsg:  configure.ToolName,
			contextHas: map[string]string{"tool": configure.ToolName},
			unwrapInto: sentinel,
		},
		{
			name:       "config read is a rejection",
			err:        &configure.ConfigReadError{Path: "/repo/.github/dependabot.yml", Cause: sentinel},
			wantFamily: ef.Rejection,
			wantCode:   "config.read",
			wantInMsg:  "/repo/.github/dependabot.yml",
			contextHas: map[string]string{"path": "/repo/.github/dependabot.yml"},
			unwrapInto: sentinel,
		},
		{
			name:       "config write is infrastructure",
			err:        &configure.ConfigWriteError{Path: "/repo/.github/dependabot.yml", Step: configure.WriteStepWrite, Cause: sentinel},
			wantFamily: ef.Infrastructure,
			wantCode:   "config.write",
			wantInMsg:  "/repo/.github/dependabot.yml",
			contextHas: map[string]string{"path": "/repo/.github/dependabot.yml", "step": string(configure.WriteStepWrite)},
			unwrapInto: sentinel,
		},
		{
			name:       "api transport is transient",
			err:        &configure.APITransportError{Op: configure.TransportOpCall, URL: "https://api.github.com/x", Cause: sentinel},
			wantFamily: ef.Transient,
			wantCode:   "github.transport",
			wantInMsg:  "https://api.github.com/x",
			contextHas: map[string]string{"op": string(configure.TransportOpCall), "url": "https://api.github.com/x"},
			unwrapInto: sentinel,
		},
		{
			name:       "unexpected 5xx status is transient",
			err:        &configure.UnexpectedStatusError{URL: "https://api.github.com/x", StatusCode: http.StatusBadGateway, Body: "oops"},
			wantFamily: ef.Transient,
			wantCode:   "github.status",
			wantInMsg:  "https://api.github.com/x",
			contextHas: map[string]string{"url": "https://api.github.com/x"},
		},
		{
			name:       "unexpected 4xx status is a rejection",
			err:        &configure.UnexpectedStatusError{URL: "https://api.github.com/x", StatusCode: http.StatusUnauthorized, Body: "nope"},
			wantFamily: ef.Rejection,
			wantCode:   "github.status",
			wantInMsg:  "https://api.github.com/x",
			contextHas: map[string]string{"url": "https://api.github.com/x"},
		},
		{
			name:       "github request build is infrastructure",
			err:        &configure.GitHubRequestError{URL: "https://api.github.com/x", Cause: sentinel},
			wantFamily: ef.Infrastructure,
			wantCode:   "github.request",
			wantInMsg:  "https://api.github.com/x",
			contextHas: map[string]string{"url": "https://api.github.com/x"},
			unwrapInto: sentinel,
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

	sentinel := errors.New("boom")

	errs := []domainErrorAlias{
		&configure.ShapeDetectionError{Root: "/repo", Cause: sentinel},
		&configure.FindingsConversionError{Tool: "t", Cause: sentinel},
		&configure.ConfigReadError{Path: "/p", Cause: sentinel},
		&configure.ConfigWriteError{Path: "/p", Step: configure.WriteStepCreateDirectory, Cause: sentinel},
		&configure.APITransportError{Op: configure.TransportOpCloseResponse, URL: "u", Cause: sentinel},
		&configure.UnexpectedStatusError{URL: "u", StatusCode: 500, BodyErr: io.ErrUnexpectedEOF},
		&configure.GitHubRequestError{URL: "u", Cause: sentinel},
	}

	for _, err := range errs {
		for key, value := range err.ErrorContext() {
			if key == "" || value == "" {
				t.Errorf("%T context has empty key or value: %q=%q", err, key, value)
			}
		}
	}
}

// domainErrorAlias lets the serializability test iterate heterogeneous types
// without re-declaring the interface at every use site.
type domainErrorAlias = interface {
	error
	ErrorFamily() ef.Family
	ErrorCode() string
	ErrorContext() map[string]string
}
