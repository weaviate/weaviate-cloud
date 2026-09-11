package output_test

import (
	"io"
	"strings"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/iostreams"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

type sample struct {
	Name string `json:"name"`
}

func renderSample(w io.Writer, s sample) error {
	return output.KeyValue(w, [][2]string{{"name", s.Name}})
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()
	streams, _, out, _ := iostreams.Test()
	if err := output.Write(streams, output.FormatJSON, sample{Name: "foo"}, "req-1", renderSample); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"name": "foo"`) || !strings.Contains(out.String(), `"request_id": "req-1"`) {
		t.Fatalf("expected json envelope, got %q", out.String())
	}
}

func TestWriteText(t *testing.T) {
	t.Parallel()
	streams, _, out, _ := iostreams.Test()
	if err := output.Write(streams, output.FormatText, sample{Name: "foo"}, "req-1", renderSample); err != nil {
		t.Fatal(err)
	}
	if out.String() != "name:  foo\n" {
		t.Fatalf("got %q", out.String())
	}
	if strings.Contains(out.String(), "request_id") {
		t.Fatal("text output must not contain envelope metadata")
	}
}

func TestWriteEnvText(t *testing.T) {
	t.Parallel()
	streams, _, out, _ := iostreams.Test()
	env := output.Envelope[sample]{
		Data:     sample{Name: "bar"},
		Metadata: output.Metadata{APIVersion: output.APIVersion, RequestID: "req-2"},
	}
	if err := output.WriteEnv(streams, output.FormatText, env, renderSample); err != nil {
		t.Fatal(err)
	}
	if out.String() != "name:  bar\n" {
		t.Fatalf("got %q", out.String())
	}
}
