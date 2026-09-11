package api

import (
	"context"
	"net/http"
)

func (c *Client) ListRegions(ctx context.Context) ([]Region, error) {
	var env ListEnvelope[Region]
	if err := c.do(ctx, http.MethodGet, "/regions", nil, &env, nil); err != nil {
		return nil, err
	}
	return env.Data, nil
}
