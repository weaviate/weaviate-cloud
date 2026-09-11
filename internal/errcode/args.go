package errcode

import "github.com/spf13/cobra"

// Args wraps a cobra positional-argument validator so a rejection surfaces
// as a *Error with CodeValidationFailed instead of cobra's untyped error,
// keeping arg-count/usage failures out of the internal_error default in
// cmd/wcloud/main.go's writeErrorEnvelope.
func Args(v cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := v(cmd, args); err != nil {
			return New(CodeValidationFailed, err.Error())
		}
		return nil
	}
}
