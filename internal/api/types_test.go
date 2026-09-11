package api_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/api"
)

func TestKeyInfo_JSONContract(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		k           api.KeyInfo
		wantKey     string
		wantAbsent  []string
		wantPresent []string
	}{
		{
			name:        "empty warning omitted",
			k:           api.KeyInfo{Value: "mykey"},
			wantKey:     "mykey",
			wantAbsent:  []string{"expires_at", "warning"},
			wantPresent: []string{"value"},
		},
		{
			name:        "non-empty warning serialised",
			k:           api.KeyInfo{Value: "mykey", Warning: "free cluster expires soon"},
			wantKey:     "mykey",
			wantAbsent:  []string{"expires_at"},
			wantPresent: []string{"value", "warning"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertKeyInfoJSON(t, tc.k, tc.wantKey, tc.wantAbsent, tc.wantPresent)
		})
	}
}

func assertKeyInfoJSON(t *testing.T, k api.KeyInfo, wantKey string, wantAbsent, wantPresent []string) {
	t.Helper()

	b, err := json.Marshal(k)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw := string(b)

	for _, key := range wantAbsent {
		if strings.Contains(raw, `"`+key+`"`) {
			t.Errorf("JSON contains %q but should not: %s", key, raw)
		}
	}
	for _, key := range wantPresent {
		if !strings.Contains(raw, `"`+key+`"`) {
			t.Errorf("JSON missing %q: %s", key, raw)
		}
	}

	var got api.KeyInfo
	unmarshalErr := json.Unmarshal(b, &got)
	if unmarshalErr != nil {
		t.Fatalf("unmarshal: %v", unmarshalErr)
	}
	if got.Value != wantKey {
		t.Errorf("Value = %q, want %q", got.Value, wantKey)
	}
}
