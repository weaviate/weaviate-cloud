package cluster

import (
	"fmt"
	"io"
	"time"

	"github.com/weaviate/weaviate-cloud/internal/api"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

func renderClusterList(w io.Writer, cs []api.Cluster) error {
	rows := make([][]string, len(cs))
	for i, c := range cs {
		rows[i] = []string{c.ID, c.Name, string(c.Status), c.Tier, c.Region}
	}
	return output.Table(w, []string{"ID", "NAME", "STATUS", "TIER", "REGION"}, rows)
}

func renderCluster(w io.Writer, c *api.Cluster) error {
	pairs := [][2]string{
		{"id", c.ID},
		{"name", c.Name},
		{"status", string(c.Status)},
		{"tier", c.Tier},
		{"region", c.Region},
		{"endpoint", c.Endpoint},
		{"grpc_endpoint", c.GRPCEndpoint},
		{"created_at", c.CreatedAt.Format(time.RFC3339)},
	}
	if c.StatusReason != "" {
		pairs = append(pairs, [2]string{"status_reason", c.StatusReason})
	}
	if c.APIKey != nil && c.APIKey.Value != "" {
		pairs = append(pairs, [2]string{"api_key", c.APIKey.Value})
	}
	return output.KeyValue(w, pairs)
}

func renderClusterStatus(w io.Writer, s api.ClusterStatus) error {
	_, err := fmt.Fprintln(w, output.SanitizeText(string(s)))
	return err
}
