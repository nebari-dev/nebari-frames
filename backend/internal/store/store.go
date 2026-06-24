// Package store defines the persistence interface for Nebari Frames and an
// in-memory implementation for tests. The SQLite implementation lives in
// store/sqlite.
package store

import (
	"context"
	"errors"

	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
)

// Grant is a permission grant on a frame (whole-frame only in MVP).
type Grant struct {
	SubjectType string // "user" | "org"
	SubjectID   string
	Permission  string // "read" | "edit" | "delete"
}

// ParentEdge is a resolved, pinned inheritance edge.
type ParentEdge struct {
	ParentFrameID string
	ParentVersion string
	OrderIndex    int
}

// CreateFrameVersionInput carries everything a publish needs to insert
// atomically: the frame row (created if new), the version, its inheritance
// edges, and its default grants.
type CreateFrameVersionInput struct {
	Frame      *framesv1.Frame
	Version    *framesv1.FrameVersion
	Extends    []ParentEdge
	Excludes   []string
	Grants     []Grant
	IsNewFrame bool
}

// Repository is the data abstraction between storage and application logic.
type Repository interface {
	CreateOrg(ctx context.Context, org *framesv1.Org) error
	GetOrgByID(ctx context.Context, id string) (*framesv1.Org, error)
	GetOrgBySlug(ctx context.Context, slug string) (*framesv1.Org, error)
	GetMembership(ctx context.Context, userSub string) (*framesv1.Membership, error)
	UpsertMembership(ctx context.Context, m *framesv1.Membership) error

	CreateFrameVersion(ctx context.Context, in CreateFrameVersionInput) error
	GetFrameBySlugName(ctx context.Context, orgSlug, name string) (*framesv1.Frame, error)
	GetFrameByID(ctx context.Context, id string) (*framesv1.Frame, error)
	GetFrameVersion(ctx context.Context, frameID, version string) (*framesv1.FrameVersion, []ParentEdge, []string, error)
	ListFrameVersions(ctx context.Context, frameID string) ([]*framesv1.FrameVersionSummary, error)
	ListFramesByOrg(ctx context.Context, orgID string) ([]*framesv1.Frame, error)
	FrameGrants(ctx context.Context, frameID string) ([]Grant, error)
}
