//nolint:testpackage // white-box: exercises the unexported parseHarnesses
package skill

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/errcode"
)

func TestParseHarnesses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		raw     []string
		want    []harness
		wantErr string
	}{
		{"single", []string{"codex"}, []harness{harnessCodex}, ""},
		{"comma list trims spaces", []string{" codex , cursor"}, []harness{harnessCodex, harnessCursor}, ""},
		{"repeated flag", []string{"codex", "cursor"}, []harness{harnessCodex, harnessCursor}, ""},
		{"empty parts skipped", []string{"codex,,"}, []harness{harnessCodex}, ""},
		{"antigravity", []string{"antigravity"}, []harness{harnessAntigravity}, ""},
		{"devin", []string{"devin"}, []harness{harnessDevin}, ""},
		{"windsurf resolves to devin", []string{"windsurf"}, []harness{harnessDevin}, ""},
		{"windsurf then devin is one harness", []string{"windsurf,devin"}, []harness{harnessDevin}, ""},
		{"devin then windsurf is one harness", []string{"devin", "windsurf"}, []harness{harnessDevin}, ""},
		{"duplicate harness", []string{"codex,codex"}, []harness{harnessCodex}, ""},
		{
			"first-seen order kept",
			[]string{"windsurf,codex,devin"},
			[]harness{harnessDevin, harnessCodex},
			"",
		},
		{"alias is case sensitive", []string{"Windsurf"}, nil, `unknown harness "Windsurf"`},
		{"unknown named alongside alias", []string{"bogus,windsurf"}, nil, `unknown harness "bogus"`},
		{"error lists the alias form", []string{"bogus"}, nil, "devin (alias: windsurf)"},
		{"error lists antigravity", []string{"bogus"}, nil, "antigravity"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseHarnesses(tc.raw)
			if tc.wantErr != "" {
				var e *errcode.Error
				if !errors.As(err, &e) {
					t.Fatalf("err = %v, want *errcode.Error", err)
				}
				if e.Code != errcode.CodeValidationFailed || !strings.Contains(e.Message, tc.wantErr) {
					t.Fatalf("err = %q (%s), want validation_failed containing %q", e.Message, e.Code, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseHarnesses(%v): %v", tc.raw, err)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("parseHarnesses(%v) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}
