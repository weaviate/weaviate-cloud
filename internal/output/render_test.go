package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/output"
)

func TestTable(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := output.Table(&buf,
		[]string{"ID", "NAME"},
		[][]string{{"abc", "foo"}, {"de", "barbar"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "ID    NAME\nabc   foo\nde    barbar\n"
	if buf.String() != want {
		t.Fatalf("got %q, want %q", buf.String(), want)
	}
}

func TestTableEmpty(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := output.Table(&buf, []string{"ID", "NAME"}, nil); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "ID   NAME\n" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestKeyValue(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := output.KeyValue(&buf, [][2]string{{"id", "abc"}, {"name", "foo"}})
	if err != nil {
		t.Fatal(err)
	}
	want := "id:    abc\nname:  foo\n"
	if buf.String() != want {
		t.Fatalf("got %q, want %q", buf.String(), want)
	}
}

func TestTableSanitizesControlAndEscapeSequences(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	malicious := "legit-cluster\x1b[2K\r\x1b[32m✔ cluster READY — SPOOFED STATUS LINE\x1b[0m"
	if err := output.Table(&buf, []string{"ID", "NAME"}, [][]string{{"abc", malicious}}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if strings.ContainsAny(got, "\x1b\r") {
		t.Fatalf("table output still contains raw ESC/CR bytes: %q", got)
	}
	if !strings.Contains(got, "legit-cluster✔ cluster READY — SPOOFED STATUS LINE") {
		t.Fatalf("expected sanitized text, got %q", got)
	}
}

func TestKeyValueSanitizesControlAndEscapeSequences(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	malicious := "legit-cluster\x1b[2K\r\x1b[32m✔ cluster READY — SPOOFED STATUS LINE\x1b[0m"
	if err := output.KeyValue(&buf, [][2]string{{"name", malicious}}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if strings.ContainsAny(got, "\x1b\r") {
		t.Fatalf("key-value output still contains raw ESC/CR bytes: %q", got)
	}
	if !strings.Contains(got, "legit-cluster✔ cluster READY — SPOOFED STATUS LINE") {
		t.Fatalf("expected sanitized text, got %q", got)
	}
}
