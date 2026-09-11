package factory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/weaviate/weaviate-cloud/internal/api"
	"github.com/weaviate/weaviate-cloud/internal/auth"
	"github.com/weaviate/weaviate-cloud/internal/config"
	"github.com/weaviate/weaviate-cloud/internal/errcode"
	"github.com/weaviate/weaviate-cloud/internal/iostreams"
	"github.com/weaviate/weaviate-cloud/internal/output"
)

// defaultPollInterval favours a brisk cadence over a coarse "check
// occasionally" one: 10s sits comfortably inside the server's rate limit and
// surfaces real cluster-status transitions promptly instead of every 30s.
// The animation/elapsed-time liveness the user sees is decoupled from this
// value (see waitForNextPoll), so a slower poll cadence doesn't cost any
// on-screen reactivity.
const defaultPollInterval = 10 * time.Second

type APIClient interface {
	ListClusters(ctx context.Context) ([]api.Cluster, error)
	CreateCluster(
		ctx context.Context,
		req api.CreateClusterRequest,
		opts api.CreateClusterOptions,
	) (*api.Cluster, error)
	GetCluster(ctx context.Context, id string) (*api.Cluster, error)
	GetClusterStatus(ctx context.Context, id string) (api.ClusterStatus, error)
	ListRegions(ctx context.Context) ([]api.Region, error)
	Whoami(ctx context.Context) (*api.WhoAmI, error)
}

type Factory struct {
	IOStreams    *iostreams.IOStreams
	Endpoint     string
	Auth         *auth.Provider
	NewAPIClient func(token string) APIClient
	Now          func() time.Time
	NewRequestID func() string
	OutputFormat output.Format
	PollInterval time.Duration
	Verbose      bool
}

// New wires the shared CLI dependencies. When the on-disk config is unreadable
// it returns an error together with a Factory that can render it: Endpoint and
// Auth are left unset so a discarded error cannot silently retarget production.
func New(version string) (*Factory, error) {
	streams := iostreams.System()

	f := &Factory{
		IOStreams:    streams,
		Now:          time.Now,
		NewRequestID: uuid.NewString,
		PollInterval: defaultPollInterval,
	}
	f.NewAPIClient = func(token string) APIClient {
		return api.NewClient(f.Endpoint, token,
			api.WithRequestIDFunc(f.NewRequestID),
			api.WithVersion(version),
		)
	}

	profiles, err := config.LoadProfiles()
	if err != nil {
		return f, errcode.New(errcode.CodeValidationFailed, fmt.Sprintf(
			"%s is unreadable (%v). Fix or delete that file, then run the command again.",
			profilesFilePath(), err))
	}

	cfg := config.ResolveWith(profiles)
	f.Endpoint = cfg.Endpoint
	f.Auth = auth.NewProvider(cfg.Auth, config.ActiveProfile(profiles), streams, nil)
	return f, nil
}

func profilesFilePath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return config.ProfilesFileName
	}
	return filepath.Join(dir, "wcloud", config.ProfilesFileName)
}
