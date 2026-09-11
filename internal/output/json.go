package output

import (
	"encoding/json"
	"io"
)

func WriteJSON(w io.Writer, env any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}

func WriteEnvelope[T any](w io.Writer, data T, requestID string) error {
	return WriteJSON(w, Envelope[T]{
		Data:     data,
		Metadata: Metadata{APIVersion: APIVersion, RequestID: requestID},
	})
}
