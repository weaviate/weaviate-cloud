package cli_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/cli"
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/guide"
	"github.com/weaviate/weaviate-cloud/internal/iostreams"
)

func newGuideTestFactory(ios *iostreams.IOStreams) *factory.Factory {
	return &factory.Factory{
		IOStreams:    ios,
		Now:          func() time.Time { return time.Unix(0, 0).UTC() },
		NewRequestID: func() string { return "test-request-id" },
	}
}

func TestGuideCommand_WritesMarkdownToStdout(t *testing.T) {
	t.Parallel()
	ios, _, out, errBuf := iostreams.Test()
	f := newGuideTestFactory(ios)

	root := cli.NewRootCmd(f)
	root.SetArgs([]string{"guide"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Errorf("expected stderr empty, got %q", errBuf.String())
	}
	if out.String() != guide.Markdown() {
		t.Errorf("stdout does not equal guide.Markdown()\ngot:  %q\nwant: %q", out.String(), guide.Markdown())
	}
}

func TestGuideCommand_IgnoresOutputFormat(t *testing.T) {
	t.Parallel()

	runGuide := func(args ...string) string {
		ios, _, out, _ := iostreams.Test()
		f := newGuideTestFactory(ios)
		root := cli.NewRootCmd(f)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("execute %v: %v", args, err)
		}
		return out.String()
	}

	jsonOut := runGuide("guide", "--output", "json")
	textOut := runGuide("guide", "--output", "text")
	autoOut := runGuide("guide")

	if jsonOut != textOut {
		t.Error("guide output differs between --output json and --output text")
	}
	if jsonOut != autoOut {
		t.Error("guide output differs between --output json and --output auto")
	}
	var envelope struct {
		Data     any `json:"data"`
		Metadata any `json:"metadata"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &envelope); err == nil && envelope.Metadata != nil {
		t.Error("guide output must not be a JSON envelope — it is documentation, not an API result")
	}
	if jsonOut != guide.Markdown() {
		t.Errorf("guide output does not equal guide.Markdown()\ngot:  %q", jsonOut)
	}
}

func TestRootHelpReferencesGuide(t *testing.T) {
	t.Parallel()
	ios, _, _, _ := iostreams.Test()
	f := newGuideTestFactory(ios)
	root := cli.NewRootCmd(f)
	if !strings.Contains(root.Long, "wcloud guide") {
		t.Errorf("root Long help does not mention 'wcloud guide', got:\n%s", root.Long)
	}
}
