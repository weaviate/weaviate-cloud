package output

import (
	"fmt"
	"os"
)

type Format int

const (
	// FormatUnresolved is the zero value: no format has been resolved yet.
	// Keeping it distinct from FormatJSON lets callers tell "not yet resolved"
	// apart from "explicitly resolved to JSON".
	FormatUnresolved Format = iota
	FormatJSON
	FormatText
)

func ResolveFormat(flag string, isTTY bool) (Format, error) {
	switch flag {
	case "", "auto":
		if isTTY && HumanReaderLikely() {
			return FormatText, nil
		}
		return FormatJSON, nil
	case "json":
		return FormatJSON, nil
	case "text":
		return FormatText, nil
	default:
		return FormatUnresolved,
			fmt.Errorf("invalid value for --output: %q; supported values: [auto, json, text]", flag)
	}
}

// HumanReaderLikely qualifies a TTY check: Buildkite, Jenkins and
// `docker run -t` allocate a PTY, so a terminal alone does not mean a human
// is reading — a pipeline step piping into jq needs the JSON contract, and a
// CI log does not want an animated spinner.
func HumanReaderLikely() bool {
	if os.Getenv("CI") != "" {
		return false
	}
	return os.Getenv("TERM") != "dumb"
}
