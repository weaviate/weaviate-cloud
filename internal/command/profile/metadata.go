package profile

import (
	"github.com/weaviate/weaviate-cloud/internal/factory"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

func makeMetadata(f *factory.Factory) output.Metadata {
	return output.Metadata{APIVersion: output.APIVersion, RequestID: f.NewRequestID()}
}
