package cli

import "github.com/spf13/cobra"

// NewRootCmdForTest exposes the root command builder to the external test
// binary so exit-code behavior can be verified without os.Exit.
func NewRootCmdForTest() (*cobra.Command, *int) {
	return newRootCmd()
}
