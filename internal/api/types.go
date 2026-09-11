package api

import (
	"errors"
	"time"
)

type Metadata struct {
	APIVersion string `json:"api_version"`
	RequestID  string `json:"request_id"`
}

type ListMetadata struct {
	APIVersion string `json:"api_version"`
	RequestID  string `json:"request_id"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type Envelope[T any] struct {
	Data     T        `json:"data"`
	Metadata Metadata `json:"metadata"`
}

// payloadValidator lets a decoded single-resource envelope reject a payload
// that carries none of the fields the caller will act on, so a 200 with a null
// or empty body is not reported as success.
type payloadValidator interface {
	validate() error
}

func (e *Envelope[T]) validate() error {
	if v, ok := any(&e.Data).(payloadValidator); ok {
		return v.validate()
	}
	return nil
}

type ListEnvelope[T any] struct {
	Data     []T          `json:"data"`
	Metadata ListMetadata `json:"metadata"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorEnvelope struct {
	Error    ErrorDetail `json:"error"`
	Metadata Metadata    `json:"metadata"`
}

type ClusterStatus string

const (
	StatusPending   ClusterStatus = "PENDING"
	StatusCreating  ClusterStatus = "CREATING"
	StatusReady     ClusterStatus = "READY"
	StatusUpdating  ClusterStatus = "UPDATING"
	StatusFailed    ClusterStatus = "FAILED"
	StatusWaiting   ClusterStatus = "WAITING"
	StatusDeleting  ClusterStatus = "DELETING"
	StatusDeleted   ClusterStatus = "DELETED"
	StatusExpired   ClusterStatus = "EXPIRED"
	StatusSuspended ClusterStatus = "SUSPENDED"
	StatusUnknown   ClusterStatus = "UNKNOWN"
)

const TierFree = "free"

type Cluster struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Status       ClusterStatus `json:"status"`
	Tier         string        `json:"tier"`
	Region       string        `json:"region"`
	Endpoint     string        `json:"endpoint"`
	GRPCEndpoint string        `json:"grpc_endpoint"`
	ExpiresAt    *time.Time    `json:"expires_at,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	StatusReason string        `json:"status_reason,omitempty"`
	APIKey       *KeyInfo      `json:"api_key,omitempty"`
}

func (c *Cluster) validate() error {
	if c.ID == "" {
		return errors.New("carried a cluster with no id")
	}
	return nil
}

type KeyInfo struct {
	Value   string `json:"value"`
	Warning string `json:"warning,omitempty"`
}

type ClusterStatusOnly struct {
	Status ClusterStatus `json:"status"`
}

func (s *ClusterStatusOnly) validate() error {
	if s.Status == "" {
		return errors.New("carried a cluster with no status")
	}
	return nil
}

type CreateClusterRequest struct {
	Name   string `json:"name,omitempty"`
	Region string `json:"region,omitempty"`
	Tier   string `json:"tier,omitempty"`
}

type RegionStatus string

type Region struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	CloudProvider string       `json:"cloud_provider"`
	Status        RegionStatus `json:"status"`
	IsDefault     bool         `json:"is_default"`
}

type WhoAmI struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	OrgID  string `json:"org_id"`
}

func (w *WhoAmI) validate() error {
	if w.UserID == "" {
		return errors.New("carried an identity with no user id")
	}
	return nil
}
