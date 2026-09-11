package region

import (
	"io"
	"strconv"

	"github.com/weaviate/weaviate-cloud/internal/api"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

func renderRegionList(w io.Writer, rs []api.Region) error {
	rows := make([][]string, len(rs))
	for i, r := range rs {
		rows[i] = []string{r.ID, r.Name, r.CloudProvider, string(r.Status), strconv.FormatBool(r.IsDefault)}
	}
	return output.Table(w, []string{"ID", "NAME", "CLOUD_PROVIDER", "STATUS", "DEFAULT"}, rows)
}
