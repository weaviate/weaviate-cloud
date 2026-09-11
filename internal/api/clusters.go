package api

import (
	"context"
	"net/http"
	"net/url"
)

// ListClusters returns the caller's READY clusters. The provisioning API does
// not yet support pagination; the wire envelope still carries a next_cursor
// field but it is always empty in v0.1.
func (c *Client) ListClusters(ctx context.Context) ([]Cluster, error) {
	var env ListEnvelope[Cluster]
	if err := c.do(ctx, http.MethodGet, "/clusters", nil, &env, nil); err != nil {
		return nil, err
	}
	return env.Data, nil
}

func (c *Client) GetCluster(ctx context.Context, id string) (*Cluster, error) {
	var env Envelope[Cluster]
	if err := c.do(ctx, http.MethodGet, "/clusters/"+url.PathEscape(id), nil, &env, nil); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

func (c *Client) GetClusterStatus(ctx context.Context, id string) (ClusterStatus, error) {
	var env Envelope[ClusterStatusOnly]
	path := "/clusters/" + url.PathEscape(id) + "/status"
	if err := c.do(ctx, http.MethodGet, path, nil, &env, nil); err != nil {
		return "", err
	}
	return env.Data.Status, nil
}

type CreateClusterOptions struct {
	IdempotencyKey string
	ProvisionedBy  string
}

func (c *Client) CreateCluster(
	ctx context.Context,
	req CreateClusterRequest,
	opts CreateClusterOptions,
) (*Cluster, error) {
	headers := make(map[string]string)
	if opts.IdempotencyKey != "" {
		headers["Idempotency-Key"] = opts.IdempotencyKey
	}
	if opts.ProvisionedBy != "" {
		headers["X-Provisioned-By"] = opts.ProvisionedBy
	}
	var env Envelope[Cluster]
	if err := c.do(ctx, http.MethodPost, "/clusters", req, &env, headers); err != nil {
		return nil, err
	}
	return &env.Data, nil
}
