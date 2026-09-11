package api

import (
	"context"
	"net/http"
)

func (c *Client) Whoami(ctx context.Context) (*WhoAmI, error) {
	var env Envelope[WhoAmI]
	if err := c.do(ctx, http.MethodGet, "/whoami", nil, &env, nil); err != nil {
		return nil, err
	}
	return &env.Data, nil
}
