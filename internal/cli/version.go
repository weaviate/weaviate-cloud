package cli

import (
	"io"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"

	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

const APIVersion = output.APIVersion

//nolint:gochecknoglobals // stamped at release time by `go build -ldflags -X`; a const cannot be.
var Version = "0.1.0-dev"

type versionData struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	GoVersion string `json:"go_version"`
}

func renderVersion(w io.Writer, d versionData) error {
	return output.KeyValue(w, [][2]string{
		{"version", d.Version},
		{"commit", d.Commit},
		{"go_version", d.GoVersion},
	})
}

func NewVersionCmd(f *factory.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print wcloud version information",
		Args:  errcode.Args(cobra.NoArgs),
		RunE: func(_ *cobra.Command, _ []string) error {
			return output.Write(f.IOStreams, f.OutputFormat, versionData{
				Version:   resolvedVersion(),
				Commit:    commitSHA(),
				GoVersion: runtime.Version(),
			}, f.NewRequestID(), renderVersion)
		},
	}
}

// resolvedVersion falls back to the Go module version recorded by `go install
// module@version` when ldflags never stamped Version — the only way a
// `go install` release carries a real version, since goreleaser's -X flag
// only reaches binaries it builds itself.
func resolvedVersion() string {
	if Version != "0.1.0-dev" {
		return Version
	}
	info, ok := debug.ReadBuildInfo()
	if ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	return Version
}

func commitSHA() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			return s.Value
		}
	}
	return "unknown"
}
