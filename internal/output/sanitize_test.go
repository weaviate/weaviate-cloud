package output_test

import (
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/output"
)

func TestSanitizeText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "ansi csi colour and cursor sequences stripped",
			in:   "legit-cluster\x1b[2K\r\x1b[32m✔ cluster READY — SPOOFED STATUS LINE\x1b[0m",
			want: "legit-cluster✔ cluster READY — SPOOFED STATUS LINE",
		},
		{
			name: "osc hyperlink sequence stripped",
			in:   "click \x1b]8;;http://example.invalid\x1b\\here\x1b]8;;\x1b\\",
			want: "click here",
		},
		{
			name: "c0 control bytes stripped",
			in:   "abc\x00\x01\x02def",
			want: "abcdef",
		},
		{
			name: "del byte stripped",
			in:   "abc\x7fdef",
			want: "abcdef",
		},
		{
			name: "c1 control rune stripped without corrupting utf8 neighbours",
			in:   "line1" + string(rune(0x85)) + "line2",
			want: "line1line2",
		},
		{
			name: "bidi override rune stripped (the name-spoofing primitive)",
			in:   "prod" + string(rune(0x202E)) + "gpj.eldnab",
			want: "prodgpj.eldnab",
		},
		{
			name: "every bidi embedding control stripped",
			in: "a" + string(rune(0x202A)) + string(rune(0x202B)) + string(rune(0x202C)) +
				string(rune(0x202D)) + string(rune(0x202E)) + "b",
			want: "ab",
		},
		{
			name: "bidi isolates stripped",
			in: "a" + string(rune(0x2066)) + string(rune(0x2067)) + string(rune(0x2068)) +
				string(rune(0x2069)) + "b",
			want: "ab",
		},
		{
			name: "zero-width space stripped (the two-names-look-like-one primitive)",
			in:   "dev" + string(rune(0x200B)) + "prod",
			want: "devprod",
		},
		{
			name: "zero-width joiners and directional marks stripped",
			in: "a" + string(rune(0x200C)) + string(rune(0x200D)) + string(rune(0x200E)) +
				string(rune(0x200F)) + "b",
			want: "ab",
		},
		{
			name: "soft hyphen stripped",
			in:   "clus" + string(rune(0x00AD)) + "ter",
			want: "cluster",
		},
		{
			name: "byte order mark stripped",
			in:   string(rune(0xFEFF)) + "my-cluster",
			want: "my-cluster",
		},
		{
			name: "printable runes bordering the stripped ranges survive",
			in: string(rune(0x200A)) + "a" + string(rune(0x2010)) + "b" + string(rune(0x2029)) +
				string(rune(0x2065)) + string(rune(0x206A)) + "c",
			want: string(rune(0x200A)) + "a" + string(rune(0x2010)) + "b" + string(rune(0x2029)) +
				string(rune(0x2065)) + string(rune(0x206A)) + "c",
		},
		{
			name: "bare carriage return stripped",
			in:   "abc\rdef",
			want: "abcdef",
		},
		{
			name: "embedded bare newline stripped (the hide-a-row primitive)",
			in:   "line-one\nline-two",
			want: "line-oneline-two",
		},
		{
			name: "utf8 multibyte content survives byte-exact",
			in:   "✔ cluster READY — done",
			want: "✔ cluster READY — done",
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},
		{
			name: "clean ascii is a no-op",
			in:   "my-cluster-1",
			want: "my-cluster-1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := output.SanitizeText(tc.in)
			if got != tc.want {
				t.Fatalf("SanitizeText(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
