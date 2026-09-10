// Package store defines the persistence interface for Nebari Frames and an
// in-memory implementation for tests. The SQLite implementation lives in
// store/sqlite.
package store

import (
	"context"
	"errors"
	"strings"
	"time"

	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
)

// CanonicalEmail normalizes an address for storage and comparison. Identity
// providers do not guarantee the case or surrounding whitespace of an email
// claim, and invites are typed by hand, so an address has to be reduced to one
// form before it can be compared or constrained.
//
// Applied to rows this package writes. Rows written before it existed are not
// retrofitted, and the SQLite unique index on (org_id, email) is still
// case-sensitive; both are tracked in #65.
func CanonicalEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

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

// FrameTemplate is one org-authored template row.
//
// A store struct rather than a protobuf message, unlike Frame and Membership:
// the proto is the external representation, and the template model is
// deliberately owned internally with translation at the boundary. Grant and
// ParentEdge above are store structs for the same reason.
//
// Built-in templates are not rows at all. They are compiled into the binary
// (frames.BuiltinTemplates), so nothing in this package knows about them.
//
// Prefill and FieldRules are opaque blobs here. Keeping them as bytes means
// adding a slot to frames.SlotTable never touches this schema, matching how
// frame_versions.content is stored.
type FrameTemplate struct {
	ID          string
	OrgID       string
	Title       string
	Description string
	Prefill     []byte // canonical YAML: slots + extends
	FieldRules  []byte // JSON: slot key -> {level, note}
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
	// CreateMembership inserts a membership and never updates one. It returns
	// ErrAlreadyExists when the subject or the (org, email) pair is taken.
	// Provisioning must not use UpsertMembership: that is UPDATE-first, so a
	// caller acting on a stale "no membership" read would rewrite a role another
	// request had just established.
	CreateMembership(ctx context.Context, m *framesv1.Membership) error
	ListMembershipsByOrg(ctx context.Context, orgID string) ([]*framesv1.Membership, error)
	GetPendingMembershipByEmail(ctx context.Context, email string) (*framesv1.Membership, error)
	CountAdmins(ctx context.Context, orgID string) (int, error)

	AddPendingMembership(ctx context.Context, m *framesv1.Membership) error
	ActivatePendingMembership(ctx context.Context, email, sub string) error
	UpdateMembershipRole(ctx context.Context, orgID, userSub, email, role string) error
	DeleteMembership(ctx context.Context, orgID, userSub, email string) error

	CreateFrameVersion(ctx context.Context, in CreateFrameVersionInput) error
	GetFrameBySlugName(ctx context.Context, orgSlug, name string) (*framesv1.Frame, error)
	GetFrameByID(ctx context.Context, id string) (*framesv1.Frame, error)
	GetFrameVersion(ctx context.Context, frameID, version string) (*framesv1.FrameVersion, []ParentEdge, []string, error)
	ListFrameVersions(ctx context.Context, frameID string) ([]*framesv1.FrameVersionSummary, error)
	ListFramesByOrg(ctx context.Context, orgID string) ([]*framesv1.Frame, error)
	FrameGrants(ctx context.Context, frameID string) ([]Grant, error)

	FrameChildren(ctx context.Context, parentFrameID string) ([]*framesv1.Frame, error)
	DeleteFrame(ctx context.Context, frameID string) error

	// Frame templates. Every accessor takes orgID because org scoping is the
	// only access control a template has: making it impossible to ask for a row
	// without saying who is asking beats relying on each handler to remember. A
	// row belonging to another org reports ErrNotFound rather than a denial, so
	// existence does not leak.
	CreateFrameTemplate(ctx context.Context, t *FrameTemplate) error
	UpdateFrameTemplate(ctx context.Context, t *FrameTemplate) error
	DeleteFrameTemplate(ctx context.Context, orgID, id string) error
	GetFrameTemplate(ctx context.Context, orgID, id string) (*FrameTemplate, error)
	ListFrameTemplatesByOrg(ctx context.Context, orgID string) ([]*FrameTemplate, error)
}
