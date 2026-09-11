package api

import "context"

// DoForTesting is the test-visible entry point for do.
func (c *Client) DoForTesting(ctx context.Context, method string) error {
	return c.do(ctx, method, "/whoami", nil, nil, nil)
}
