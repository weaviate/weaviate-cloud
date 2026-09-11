package output_test

import (
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/output"
)

func TestResolveFormat(t *testing.T) {
	tests := []struct {
		name    string
		flag    string
		isTTY   bool
		ci      string
		term    string
		want    output.Format
		wantErr bool
	}{
		{"auto on tty -> text", "auto", true, "", "xterm-256color", output.FormatText, false},
		{"auto off tty -> json", "auto", false, "", "xterm-256color", output.FormatJSON, false},
		{"empty on tty -> text", "", true, "", "xterm-256color", output.FormatText, false},
		{"empty off tty -> json", "", false, "", "xterm-256color", output.FormatJSON, false},
		{"auto on a ci pty -> json", "auto", true, "true", "xterm-256color", output.FormatJSON, false},
		{"empty on a ci pty -> json", "", true, "1", "xterm-256color", output.FormatJSON, false},
		{"auto on a dumb terminal -> json", "auto", true, "", "dumb", output.FormatJSON, false},
		{"explicit json on tty", "json", true, "", "xterm-256color", output.FormatJSON, false},
		{"explicit text off tty", "text", false, "", "xterm-256color", output.FormatText, false},
		{"explicit text beats ci", "text", true, "true", "xterm-256color", output.FormatText, false},
		{"explicit text beats a dumb terminal", "text", true, "", "dumb", output.FormatText, false},
		{"invalid", "yaml", true, "", "xterm-256color", output.FormatUnresolved, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CI", tt.ci)
			t.Setenv("TERM", tt.term)
			got, err := output.ResolveFormat(tt.flag, tt.isTTY)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatZeroValueIsUnresolved(t *testing.T) {
	t.Parallel()
	var zero output.Format
	if zero != output.FormatUnresolved {
		t.Fatalf("zero value of Format = %v, want FormatUnresolved", zero)
	}
	if zero == output.FormatJSON {
		t.Fatalf("zero value of Format must not equal FormatJSON (that's the ambiguity this type pins against)")
	}
}

func TestResolveFormatNeverReturnsUnresolved(t *testing.T) {
	t.Parallel()
	cases := []struct {
		flag  string
		isTTY bool
	}{
		{"auto", true},
		{"auto", false},
		{"", true},
		{"", false},
		{"json", true},
		{"json", false},
		{"text", true},
		{"text", false},
	}
	for _, tc := range cases {
		got, err := output.ResolveFormat(tc.flag, tc.isTTY)
		if err != nil {
			t.Fatalf("ResolveFormat(%q, %v) unexpected error: %v", tc.flag, tc.isTTY, err)
		}
		if got == output.FormatUnresolved {
			t.Fatalf("ResolveFormat(%q, %v) returned FormatUnresolved on success", tc.flag, tc.isTTY)
		}
	}
}
