package output

import "encoding/json"

const APIVersion = "v1"

type Envelope[T any] struct {
	Data     T
	Error    *APIError
	Metadata Metadata
}

type Metadata struct {
	APIVersion string `json:"api_version"`
	RequestID  string `json:"request_id"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type APIError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func (e Envelope[T]) MarshalJSON() ([]byte, error) {
	if e.Error != nil {
		return json.Marshal(struct {
			Error    *APIError `json:"error"`
			Metadata Metadata  `json:"metadata"`
		}{e.Error, e.Metadata})
	}
	return json.Marshal(struct {
		Data     T        `json:"data"`
		Metadata Metadata `json:"metadata"`
	}{e.Data, e.Metadata})
}
