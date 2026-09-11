package output

import (
	"io"

	"github.com/weaviate/weaviate-cloud/internal/iostreams"
)

func Write[T any](
	s *iostreams.IOStreams,
	format Format,
	data T,
	requestID string,
	renderText func(io.Writer, T) error,
) error {
	if format == FormatText {
		return renderText(s.Out, data)
	}
	return WriteEnvelope(s.Out, data, requestID)
}

func WriteEnv[T any](
	s *iostreams.IOStreams,
	format Format,
	env Envelope[T],
	renderText func(io.Writer, T) error,
) error {
	if format == FormatText {
		return renderText(s.Out, env.Data)
	}
	return WriteJSON(s.Out, env)
}
