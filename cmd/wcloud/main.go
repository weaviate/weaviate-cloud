// Package main is the entry point for the wcloud CLI binary.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/weaviate/weaviate-cloud/internal/api"
	"github.com/weaviate/weaviate-cloud/internal/cli"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

func main() {
	f, cfgErr := factory.New(cli.Version)
	root := cli.NewRootCmd(f)
	if cfgErr != nil && needsConfig(root, os.Args[1:]) {
		// WHY: argv is unparsed here, so --output would be ignored while rendering this error.
		_ = root.ParseFlags(os.Args[1:])
		fail(f, root, cfgErr)
	}

	ctx, stop := notifyContext()
	defer stop()

	if err := root.ExecuteContext(ctx); err != nil {
		fail(f, root, err)
	}
}

func notifyContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func fail(f *factory.Factory, root *cobra.Command, err error) {
	resolveErrorFormat(f, root)
	writeErrorEnvelope(f, err)
	os.Exit(errcode.ExitCodeFor(err))
}

// resolveErrorFormat guarantees f.OutputFormat is resolved before an error is
// rendered, even when the error occurred before PersistentPreRunE ran (cobra
// arg-validation, unknown command/flag, or --output's own validation failure)
// and so never reached the `f.OutputFormat = format` assignment in root.go.
func resolveErrorFormat(f *factory.Factory, root *cobra.Command) {
	if f.OutputFormat != output.FormatUnresolved {
		return
	}

	flagValue := root.PersistentFlags().Lookup("output").Value.String()

	format, err := output.ResolveFormat(flagValue, f.IOStreams.IsStdoutTTY())
	if err != nil {
		// The flag value itself is invalid; don't compound one validation
		// failure into an unreadable second one — fall back to TTY-only default.
		format, _ = output.ResolveFormat("", f.IOStreams.IsStdoutTTY())
	}
	f.OutputFormat = format
}

func writeErrorEnvelope(f *factory.Factory, err error) {
	code := errcode.CodeInternalError
	if c, ok := errcode.CodeFor(err); ok {
		code = c
	}

	var retryAfterSecs float64
	var apiErr *api.Error
	if errors.As(err, &apiErr) && apiErr.RetryAfter > 0 {
		retryAfterSecs = apiErr.RetryAfter.Seconds()
	}

	if f.Verbose && apiErr != nil && apiErr.RawExcerpt != "" {
		fmt.Fprintf(f.IOStreams.Err, "verbose: raw response body (diagnostic only, not machine-readable): %s\n",
			output.SanitizeText(apiErr.RawExcerpt))
	}

	if f.OutputFormat == output.FormatText {
		safeMsg := output.SanitizeText(err.Error())
		safeCode := output.SanitizeText(code)
		if retryAfterSecs > 0 {
			fmt.Fprintf(f.IOStreams.Err, "Error: %s (%s) [retry_after: %.0fs]\n",
				safeMsg, safeCode, retryAfterSecs)
		} else {
			fmt.Fprintf(f.IOStreams.Err, "Error: %s (%s)\n", safeMsg, safeCode)
		}
		return
	}

	var details map[string]any
	var ec *errcode.Error
	if errors.As(err, &ec) {
		details = ec.Details
	}
	if retryAfterSecs > 0 {
		if details == nil {
			details = make(map[string]any)
		}
		details["retry_after_seconds"] = retryAfterSecs
	}

	env := output.Envelope[struct{}]{
		Error: &output.APIError{
			Code:    code,
			Message: err.Error(),
			Details: details,
		},
		Metadata: output.Metadata{
			APIVersion: cli.APIVersion,
			RequestID:  f.NewRequestID(),
		},
	}
	_ = output.WriteJSON(f.IOStreams.Out, env)
}

// WHY: a broken profiles.json must not hide the commands that explain how to fix
// it — `guide` is where an agent reads what to do, and neither it nor `version`
// resolves an endpoint or a credential.
const (
	cmdGuide   = "guide"
	cmdVersion = "version"
)

func needsConfig(root *cobra.Command, args []string) bool {
	cmd, _, err := root.Find(args)
	if err != nil || cmd == nil {
		return true
	}
	switch cmd.Name() {
	case cmdGuide, cmdVersion:
		return false
	}
	return true
}
