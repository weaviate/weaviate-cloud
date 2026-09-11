package output_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/weaviate/weaviate-cloud/internal/output"
)

type payload struct {
	Name string `json:"name"`
}

func TestEnvelope_SuccessSerialization(t *testing.T) {
	t.Parallel()
	env := output.Envelope[payload]{
		Data:     payload{Name: "alpha"},
		Metadata: output.Metadata{APIVersion: "v1", RequestID: "req-1"},
	}

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"data"`) {
		t.Errorf("expected data field, got %s", s)
	}
	if strings.Contains(s, `"error"`) {
		t.Errorf("expected no error field, got %s", s)
	}
	if !strings.Contains(s, `"request_id":"req-1"`) {
		t.Errorf("expected request_id, got %s", s)
	}
}

func TestEnvelope_ErrorSerialization(t *testing.T) {
	t.Parallel()
	env := output.Envelope[payload]{
		Error: &output.APIError{
			Code:    "cluster_not_found",
			Message: "cluster abc does not exist",
		},
		Metadata: output.Metadata{APIVersion: "v1", RequestID: "req-2"},
	}

	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if strings.Contains(s, `"data"`) {
		t.Errorf("expected no data field for error envelope, got %s", s)
	}
	if !strings.Contains(s, `"code":"cluster_not_found"`) {
		t.Errorf("expected error code, got %s", s)
	}
}

func TestEnvelope_RoundTripSuccess(t *testing.T) {
	t.Parallel()
	original := output.Envelope[payload]{
		Data:     payload{Name: "alpha"},
		Metadata: output.Metadata{APIVersion: "v1", RequestID: "req-1"},
	}

	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded struct {
		Data     payload         `json:"data"`
		Error    json.RawMessage `json:"error"`
		Metadata output.Metadata `json:"metadata"`
	}
	if uerr := json.Unmarshal(b, &decoded); uerr != nil {
		t.Fatalf("unmarshal: %v", uerr)
	}
	if decoded.Data.Name != "alpha" {
		t.Errorf("data name = %q, want %q", decoded.Data.Name, "alpha")
	}
	if decoded.Error != nil {
		t.Errorf("expected nil error, got %s", decoded.Error)
	}
	if decoded.Metadata.RequestID != "req-1" {
		t.Errorf("request id = %q, want req-1", decoded.Metadata.RequestID)
	}
}
