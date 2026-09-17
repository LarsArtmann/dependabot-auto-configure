package cli_test

import (
	"errors"
	"strings"
	"testing"

	ef "github.com/larsartmann/go-error-family"

	"github.com/larsartmann/dependabot-auto-configure/internal/cli"
)

// TestTypedErrorsCarryDomainContract pins the DDD error contract for the
// reporting boundary: flag rejections are the user's fault, output failures
// are the environment's.
func TestTypedErrorsCarryDomainContract(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("boom")

	tests := []struct {
		name       string
		err        interface {
			error
			ErrorFamily() ef.Family
			ErrorCode() string
			ErrorContext() map[string]string
		}
		wantFamily ef.Family
		wantCode   string
		wantInMsg  string
		contextHas map[string]string
		unwrapInto error
	}{
		{
			name:       "flag value error is a rejection",
			err:        &cli.FlagValueError{Flag: "--fail-on", Value: "sometimes", Cause: sentinel},
			wantFamily: ef.Rejection,
			wantCode:   "cli.flag",
			wantInMsg:  "--fail-on",
			contextHas: map[string]string{"flag": "--fail-on", "value": "sometimes"},
			unwrapInto: sentinel,
		},
		{
			name:       "output error is infrastructure",
			err:        &cli.OutputError{Stream: "stdout", Cause: sentinel},
			wantFamily: ef.Infrastructure,
			wantCode:   "cli.output",
			wantInMsg:  "stdout",
			contextHas: map[string]string{"stream": "stdout"},
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
