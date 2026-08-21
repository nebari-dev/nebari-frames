package frames

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/nebari-dev/nebari-frames/backend/internal/orgs"
	"github.com/nebari-dev/nebari-frames/backend/internal/rbac"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
	"github.com/nebari-dev/nebari-frames/gen/go/frames/v1/framesv1connect"
)

type Service struct {
	repo              store.Repository
	lookup            rbac.GrantLookup
	defaultMembership orgs.DefaultMembership
}

// Option configures a Service. Options exist so that adding configuration does
// not churn every NewService call site; the zero configuration is fail-closed.
type Option func(*Service)

// WithDefaultMembership grants authenticated callers with no stored membership
// a baseline role in the named org. Omit it to deny such callers (the default).
func WithDefaultMembership(def orgs.DefaultMembership) Option {
	return func(s *Service) { s.defaultMembership = def }
}

var _ framesv1connect.FrameServiceHandler = (*Service)(nil)

func NewService(repo store.Repository, opts ...Option) *Service {
	s := &Service{repo: repo, lookup: grantLookup{repo}}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// grantLookup adapts store grants to rbac.Grant.
type grantLookup struct{ repo store.Repository }

func (g grantLookup) FrameGrants(ctx context.Context, frameID string) ([]rbac.Grant, error) {
	sg, err := g.repo.FrameGrants(ctx, frameID)
	if err != nil {
		return nil, err
	}
	out := make([]rbac.Grant, len(sg))
	for i, x := range sg {
		out[i] = rbac.Grant{SubjectType: x.SubjectType, SubjectID: x.SubjectID, Permission: rbac.Permission(x.Permission)}
	}
	return out, nil
}

func newID() string {
	return ulid.MustNew(ulid.Now(), rand.Reader).String()
}

func (s *Service) GetMe(ctx context.Context, _ *connect.Request[framesv1.GetMeRequest]) (*connect.Response[framesv1.GetMeResponse], error) {
	caller, err := s.resolveCaller(ctx)
	if err != nil {
		return nil, err
	}
	org, err := s.repo.GetOrgByID(ctx, caller.OrgID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&framesv1.GetMeResponse{
		Subject:   caller.Subject,
		Email:     caller.Email,
		Org:       org,
		Role:      string(caller.Role),
		CanCreate: rbac.CanPublish(caller),
	}), nil
}

// MaxContentBytes caps a single Frame version's stored document. The limit is
// per-version and applies to the canonical stored bytes, not to the resolved
// form: a Frame that inherits heavily can still compose to more than this.
const MaxContentBytes = 512 * 1024

// PublishIntent tells PublishDoc whether the caller means to create a new
// frame, update an existing one, or either. It exists so the create/update
// distinction is enforced next to the RBAC checks rather than by each caller:
// the MCP tools rely on it, and the Connect RPC keeps its upsert behavior.
type PublishIntent int

const (
	// PublishUpsert creates the frame when absent and updates it when present.
	PublishUpsert PublishIntent = iota
	// PublishCreate requires the frame not to exist yet.
	PublishCreate
	// PublishUpdate requires the frame to already exist.
	PublishUpdate
)

func (s *Service) PublishFrame(ctx context.Context, req *connect.Request[framesv1.PublishFrameRequest]) (*connect.Response[framesv1.PublishFrameResponse], error) {
	// Authorization stays ahead of parsing, as it was before this path was
	// split: parsing is the expensive, caller-controlled step, and someone who
	// may not publish should never reach it.
	caller, err := s.authorizePublish(ctx)
	if err != nil {
		return nil, err
	}
	doc, err := Parse(req.Msg.Content)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// The submitted bytes are stored verbatim rather than re-marshalled from
	// doc: an author's comments and formatting survive, and the digest stays
	// stable for a document that did not change.
	frame, version, err := s.publish(ctx, caller, doc, req.Msg.Content, req.Msg.Changelog, PublishUpsert, "")
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&framesv1.PublishFrameResponse{Frame: frame, Version: version}), nil
}

// authorizePublish resolves the caller and checks the role required to publish
// at all. Both front doors call it before touching caller-supplied content.
func (s *Service) authorizePublish(ctx context.Context) (rbac.Caller, error) {
	caller, err := s.resolveCaller(ctx)
	if err != nil {
		return rbac.Caller{}, err
	}
	if !rbac.CanPublish(caller) {
		return rbac.Caller{}, connect.NewError(connect.CodePermissionDenied, errors.New("publisher or admin role required"))
	}
	return caller, nil
}

// PublishDoc validates and publishes doc as a new version, enforcing RBAC:
// creating a frame needs the publisher or admin role, and writing to an
// existing frame needs edit permission on it. It is the single write path
// shared by the Connect RPC and the MCP tools, so neither can drift from the
// other or skip a check.
//
// Errors are connect errors so both front doors can map them without
// translation: PermissionDenied, InvalidArgument (with field violations),
// AlreadyExists, NotFound, Internal.
func (s *Service) PublishDoc(ctx context.Context, doc *Doc, changelog string, intent PublishIntent) (*framesv1.Frame, *framesv1.FrameVersion, error) {
	return s.PublishDocFrom(ctx, doc, changelog, intent, "")
}

// PublishDocFrom is PublishDoc with a concurrency check. baseVersion is the
// version the caller read before composing doc; the publish is rejected with
// CodeFailedPrecondition when the frame has moved on since. An empty
// baseVersion means the caller did not check, which is the unguarded behaviour
// the Connect RPC has always had.
//
// A read-modify-write without this check silently loses one of two concurrent
// updates: both merge onto the same base, both pick different version strings
// so nothing collides, and both report success.
func (s *Service) PublishDocFrom(ctx context.Context, doc *Doc, changelog string, intent PublishIntent, baseVersion string) (*framesv1.Frame, *framesv1.FrameVersion, error) {
	caller, err := s.authorizePublish(ctx)
	if err != nil {
		return nil, nil, err
	}
	content, err := Marshal(doc)
	if err != nil {
		return nil, nil, connect.NewError(connect.CodeInternal, err)
	}
	return s.publish(ctx, caller, doc, content, changelog, intent, baseVersion)
}

// publish is the shared implementation, called only with a caller that
// authorizePublish has already cleared. content is the canonical stored form of
// doc; callers holding the author's original bytes pass those so they are not
// normalized away.
func (s *Service) publish(ctx context.Context, caller rbac.Caller, doc *Doc, content []byte, changelog string, intent PublishIntent, baseVersion string) (*framesv1.Frame, *framesv1.FrameVersion, error) {
	if verr := Validate(doc); verr != nil {
		return nil, nil, violationErr(verr)
	}
	// Enforced here rather than at either entry point so the Connect API and the
	// MCP tools share one limit. It matters more now that an LLM can author a
	// Frame: the content lands verbatim in a single-writer SQLite database and is
	// re-read on every read and every child's inheritance walk.
	if len(content) > MaxContentBytes {
		return nil, nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(
			"frame content is %d bytes, over the %d byte limit", len(content), MaxContentBytes))
	}

	org, err := s.repo.GetOrgByID(ctx, caller.OrgID)
	if err != nil {
		return nil, nil, connect.NewError(connect.CodeInternal, err)
	}

	existing, err := s.repo.GetFrameBySlugName(ctx, org.Slug, doc.Name)
	isNew := errors.Is(err, store.ErrNotFound)
	if err != nil && !isNew {
		return nil, nil, connect.NewError(connect.CodeInternal, err)
	}
	switch {
	case isNew && intent == PublishUpdate:
		return nil, nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no frame named %q to update", doc.Name))
	case !isNew && intent == PublishCreate:
		return nil, nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("a frame named %q already exists; update it instead", doc.Name))
	}

	now := timestamppb.Now()
	var frame *framesv1.Frame
	if isNew {
		frame = &framesv1.Frame{
			Id: newID(), OrgId: caller.OrgID, Name: doc.Name, Description: doc.Description,
			OwnerSub: caller.Subject, LatestVersion: doc.Version, CreatedAt: now, UpdatedAt: now,
		}
	} else {
		// editing an existing frame requires edit permission
		allowed, err := rbac.Can(ctx, s.lookup, caller, existing.OrgId, existing.Id, rbac.PermEdit)
		if err != nil {
			return nil, nil, connect.NewError(connect.CodeInternal, err)
		}
		if !allowed {
			return nil, nil, connect.NewError(connect.CodePermissionDenied, errors.New("edit permission required"))
		}
		if baseVersion != "" && existing.LatestVersion != baseVersion {
			return nil, nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf(
				"frame %q has moved on: you based this change on %s but the latest is %s; re-read it and apply your change again",
				doc.Name, baseVersion, existing.LatestVersion))
		}
		// latest_version must only move forward. Otherwise publishing an older
		// version silently unpublishes newer content: every default read, and
		// the merge base of the next update, resolves to whatever is "latest".
		// Strictly below only: republishing the current version is a duplicate,
		// which CreateFrameVersion reports as AlreadyExists - a more precise
		// answer than "does not advance".
		if cmp, ok := compareVersions(doc.Version, existing.LatestVersion); ok && cmp < 0 {
			// Reported as a field violation on `version`, so the web form marks
			// the offending field rather than showing a form-level error, and
			// the CLI names the field too.
			return nil, nil, violationErr(&ValidationError{Errors: []FieldError{{
				Path: "version",
				Message: fmt.Sprintf("must be higher than the current version %s",
					existing.LatestVersion),
			}}})
		}
		frame = existing
		frame.Description = doc.Description
		frame.LatestVersion = doc.Version
		frame.UpdatedAt = now
	}

	edges, err := s.resolveEdges(ctx, caller, org.Slug, doc.Extends)
	if err != nil {
		return nil, nil, err
	}
	excludeIDs, err := s.resolveExcludes(ctx, caller, org.Slug, doc.Excludes)
	if err != nil {
		return nil, nil, err
	}

	digest := sha256.Sum256(content)
	version := &framesv1.FrameVersion{
		Version: doc.Version, Changelog: changelog, Digest: hex.EncodeToString(digest[:]),
		SizeBytes: int64(len(content)), PublishedBy: caller.Subject, PublishedAt: now,
		Content: content,
	}

	in := store.CreateFrameVersionInput{
		Frame: frame, Version: version, Extends: edges, Excludes: excludeIDs, IsNewFrame: isNew,
	}
	if isNew {
		dg := rbac.DefaultGrants(caller.Subject, caller.OrgID)
		for _, g := range dg {
			in.Grants = append(in.Grants, store.Grant{SubjectType: g.SubjectType, SubjectID: g.SubjectID, Permission: string(g.Permission)})
		}
	}
	if err := s.repo.CreateFrameVersion(ctx, in); err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			return nil, nil, connect.NewError(connect.CodeAlreadyExists, errors.New("frame version already exists"))
		}
		return nil, nil, connect.NewError(connect.CodeInternal, err)
	}
	return frame, version, nil
}

// readableFramesInOrg resolves the caller and returns the frames in their org
// the caller may read (PermRead). It is the single RBAC read-filter path shared
// by ListFrames and ListReadable so the rule cannot diverge between them.
func (s *Service) readableFramesInOrg(ctx context.Context) (rbac.Caller, *framesv1.Org, []*framesv1.Frame, error) {
	caller, err := s.resolveCaller(ctx)
	if err != nil {
		return rbac.Caller{}, nil, nil, err
	}
	org, err := s.repo.GetOrgByID(ctx, caller.OrgID)
	if err != nil {
		return rbac.Caller{}, nil, nil, connect.NewError(connect.CodeInternal, err)
	}
	all, err := s.repo.ListFramesByOrg(ctx, caller.OrgID)
	if err != nil {
		return rbac.Caller{}, nil, nil, connect.NewError(connect.CodeInternal, err)
	}
	readable := make([]*framesv1.Frame, 0, len(all))
	for _, f := range all {
		canRead, err := rbac.Can(ctx, s.lookup, caller, f.OrgId, f.Id, rbac.PermRead)
		if err != nil {
			return rbac.Caller{}, nil, nil, connect.NewError(connect.CodeInternal, err)
		}
		if canRead {
			readable = append(readable, f)
		}
	}
	return caller, org, readable, nil
}

func (s *Service) ListFrames(ctx context.Context, _ *connect.Request[framesv1.ListFramesRequest]) (*connect.Response[framesv1.ListFramesResponse], error) {
	caller, org, readable, err := s.readableFramesInOrg(ctx)
	if err != nil {
		return nil, err
	}
	resp := &framesv1.ListFramesResponse{
		CanCreate: rbac.CanPublish(caller),
		Frames:    []*framesv1.FrameSummary{},
	}
	for _, f := range readable {
		canEdit, err := rbac.Can(ctx, s.lookup, caller, f.OrgId, f.Id, rbac.PermEdit)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		canDelete, err := rbac.Can(ctx, s.lookup, caller, f.OrgId, f.Id, rbac.PermDelete)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		resp.Frames = append(resp.Frames, &framesv1.FrameSummary{
			OrgSlug: org.Slug, Name: f.Name, Description: f.Description, OwnerSub: f.OwnerSub,
			LatestVersion: f.LatestVersion, UpdatedAt: f.UpdatedAt,
			Permissions: &framesv1.Permissions{CanEdit: canEdit, CanDelete: canDelete},
		})
	}
	return connect.NewResponse(resp), nil
}

func (s *Service) GetFrame(ctx context.Context, req *connect.Request[framesv1.GetFrameRequest]) (*connect.Response[framesv1.GetFrameResponse], error) {
	caller, err := s.resolveCaller(ctx)
	if err != nil {
		return nil, err
	}
	frame, version, edges, excludes, err := s.loadForRead(ctx, caller, req.Msg.OrgSlug, req.Msg.Name, req.Msg.Version)
	if err != nil {
		return nil, err
	}
	canEdit, _ := rbac.Can(ctx, s.lookup, caller, frame.OrgId, frame.Id, rbac.PermEdit)
	canDelete, _ := rbac.Can(ctx, s.lookup, caller, frame.OrgId, frame.Id, rbac.PermDelete)
	extends := make([]*framesv1.ParentRef, 0, len(edges))
	for _, e := range edges {
		pf, err := s.repo.GetFrameByID(ctx, e.ParentFrameID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		org, _ := s.repo.GetOrgByID(ctx, pf.OrgId)
		extends = append(extends, &framesv1.ParentRef{Ref: org.Slug + "/" + pf.Name, Version: e.ParentVersion})
	}
	exRefs := make([]string, 0, len(excludes))
	for _, id := range excludes {
		pf, err := s.repo.GetFrameByID(ctx, id)
		if err != nil {
			continue
		}
		org, _ := s.repo.GetOrgByID(ctx, pf.OrgId)
		exRefs = append(exRefs, org.Slug+"/"+pf.Name)
	}
	return connect.NewResponse(&framesv1.GetFrameResponse{
		Frame: frame, Version: version, Extends: extends, Excludes: exRefs,
		Permissions: &framesv1.Permissions{CanEdit: canEdit, CanDelete: canDelete},
	}), nil
}

// ListFrameVersions lists a frame's published versions, read-enforced.
// Missing read is reported as NotFound to avoid leaking existence.
func (s *Service) ListFrameVersions(ctx context.Context, req *connect.Request[framesv1.ListFrameVersionsRequest]) (*connect.Response[framesv1.ListFrameVersionsResponse], error) {
	caller, err := s.resolveCaller(ctx)
	if err != nil {
		return nil, err
	}
	notFound := connect.NewError(connect.CodeNotFound, errors.New("frame not found"))
	frame, err := s.repo.GetFrameBySlugName(ctx, req.Msg.OrgSlug, req.Msg.Name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, notFound
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	canRead, err := rbac.Can(ctx, s.lookup, caller, frame.OrgId, frame.Id, rbac.PermRead)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !canRead {
		return nil, notFound
	}
	versions, err := s.repo.ListFrameVersions(ctx, frame.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&framesv1.ListFrameVersionsResponse{Versions: versions}), nil
}

// ReadableFrame is a flattened, RBAC-readable frame summary for non-RPC consumers
// (e.g. the MCP adapter). Returned by ListReadable.
type ReadableFrame struct {
	OrgSlug     string
	OrgDisplay  string
	Name        string
	Version     string // latest version
	Description string
}

// ListReadable returns the caller's RBAC-readable frames in their org. It applies
// the same Read check ListFrames uses; unreadable frames are omitted.
func (s *Service) ListReadable(ctx context.Context) ([]ReadableFrame, error) {
	_, org, readable, err := s.readableFramesInOrg(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ReadableFrame, 0, len(readable))
	for _, f := range readable {
		out = append(out, ReadableFrame{
			OrgSlug:     org.Slug,
			OrgDisplay:  org.DisplayName,
			Name:        f.Name,
			Version:     f.LatestVersion,
			Description: f.Description,
		})
	}
	return out, nil
}

// SourceDoc returns a frame's own stored document, scoped to the caller's org
// and read-enforced (a denied or missing read is CodeNotFound, no existence
// leak). Unlike ResolveDoc it does NOT merge ancestors, which is what makes it
// the safe input to a write: feeding a resolved document back into a publish
// would copy every parent's slots into the child and drop its extends edges.
func (s *Service) SourceDoc(ctx context.Context, name, version string) (*Doc, error) {
	caller, err := s.resolveCaller(ctx)
	if err != nil {
		return nil, err
	}
	org, err := s.repo.GetOrgByID(ctx, caller.OrgID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	_, v, _, _, err := s.loadForRead(ctx, caller, org.Slug, name, version)
	if err != nil {
		return nil, err
	}
	doc, err := Parse(v.Content)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return doc, nil
}

// ResolveDoc returns the inheritance-merged Doc for a frame, read-enforced.
// A denied or missing read returns connect.CodeNotFound (no existence leak).
func (s *Service) ResolveDoc(ctx context.Context, orgSlug, name, version string) (*Doc, error) {
	caller, err := s.resolveCaller(ctx)
	if err != nil {
		return nil, err
	}
	_, v, _, _, err := s.loadForRead(ctx, caller, orgSlug, name, version)
	if err != nil {
		return nil, err
	}
	doc, err := Parse(v.Content)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	callerOrg, err := s.repo.GetOrgByID(ctx, caller.OrgID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	fetcher := &readFetcher{s: s, caller: caller, callerOrgSlug: callerOrg.Slug}
	resolved, err := Resolve(ctx, fetcher, doc, doc.Extends, doc.Excludes)
	if err != nil {
		var ce *CycleError
		if errors.As(err, &ce) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, ce)
		}
		if errors.Is(err, ErrParentUnreadable) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("frame not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return resolved, nil
}

func (s *Service) ResolveFrame(ctx context.Context, req *connect.Request[framesv1.ResolveFrameRequest]) (*connect.Response[framesv1.ResolveFrameResponse], error) {
	resolved, err := s.ResolveDoc(ctx, req.Msg.OrgSlug, req.Msg.Name, req.Msg.Version)
	if err != nil {
		return nil, err
	}
	out, err := Marshal(resolved)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&framesv1.ResolveFrameResponse{ResolvedContent: out}), nil
}

// --- helpers ---

func (s *Service) resolveCaller(ctx context.Context) (rbac.Caller, error) {
	caller, err := orgs.ResolveCaller(ctx, s.repo, s.defaultMembership)
	if err != nil {
		switch {
		case errors.Is(err, orgs.ErrNoClaims):
			return rbac.Caller{}, connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
		case errors.Is(err, orgs.ErrNoMembership):
			return rbac.Caller{}, connect.NewError(connect.CodePermissionDenied, errors.New("no org membership"))
		default:
			return rbac.Caller{}, connect.NewError(connect.CodeInternal, err)
		}
	}
	return caller, nil
}

// loadForRead fetches a frame+version and enforces read; missing read -> 404.
func (s *Service) loadForRead(ctx context.Context, caller rbac.Caller, orgSlug, name, version string) (*framesv1.Frame, *framesv1.FrameVersion, []store.ParentEdge, []string, error) {
	notFound := connect.NewError(connect.CodeNotFound, errors.New("frame not found"))
	frame, err := s.repo.GetFrameBySlugName(ctx, orgSlug, name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil, nil, nil, notFound
		}
		return nil, nil, nil, nil, connect.NewError(connect.CodeInternal, err)
	}
	canRead, err := rbac.Can(ctx, s.lookup, caller, frame.OrgId, frame.Id, rbac.PermRead)
	if err != nil {
		return nil, nil, nil, nil, connect.NewError(connect.CodeInternal, err)
	}
	if !canRead {
		return nil, nil, nil, nil, notFound // do not leak existence
	}
	if version == "" {
		version = frame.LatestVersion
	}
	v, edges, excludes, err := s.repo.GetFrameVersion(ctx, frame.Id, version)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil, nil, nil, notFound
		}
		return nil, nil, nil, nil, connect.NewError(connect.CodeInternal, err)
	}
	return frame, v, edges, excludes, nil
}

// resolveEdges maps YAML extends refs -> pinned store.ParentEdge rows.
// caller is used to enforce read permission on each parent frame; a denied
// read returns the same error as a missing parent to avoid oracle leakage.
func (s *Service) resolveEdges(ctx context.Context, caller rbac.Caller, callerOrgSlug string, refs []ExtendRef) ([]store.ParentEdge, error) {
	out := []store.ParentEdge{}
	for i, r := range refs {
		orgSlug, name := splitRef(r.Ref, callerOrgSlug)
		pf, err := s.repo.GetFrameBySlugName(ctx, orgSlug, name)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("extends[%d]: parent %s not found", i, r.Ref))
		}
		canRead, err := rbac.Can(ctx, s.lookup, caller, pf.OrgId, pf.Id, rbac.PermRead)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if !canRead {
			// Return the same error as absent to prevent oracle leakage.
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("extends[%d]: parent %s not found", i, r.Ref))
		}
		out = append(out, store.ParentEdge{ParentFrameID: pf.Id, ParentVersion: r.Version, OrderIndex: i})
	}
	return out, nil
}

// resolveExcludes maps YAML excludes refs -> frame IDs.
// caller is used to enforce read permission on each excluded frame; a denied
// read returns the same error as a missing frame to avoid oracle leakage.
func (s *Service) resolveExcludes(ctx context.Context, caller rbac.Caller, callerOrgSlug string, refs []string) ([]string, error) {
	out := []string{}
	for _, ref := range refs {
		orgSlug, name := splitRef(ref, callerOrgSlug)
		pf, err := s.repo.GetFrameBySlugName(ctx, orgSlug, name)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("excludes: %s not found", ref))
		}
		canRead, err := rbac.Can(ctx, s.lookup, caller, pf.OrgId, pf.Id, rbac.PermRead)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if !canRead {
			// Return the same error as absent to prevent oracle leakage.
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("excludes: %s not found", ref))
		}
		out = append(out, pf.Id)
	}
	return out, nil
}

func splitRef(ref, callerOrgSlug string) (orgSlug, name string) {
	if orgSlug, name, ok := strings.Cut(ref, "/"); ok {
		return orgSlug, name
	}
	return callerOrgSlug, ref // same-org may omit slug
}

// readFetcher adapts the store to frames.ParentFetcher, enforcing read on each parent.
type readFetcher struct {
	s             *Service
	caller        rbac.Caller
	callerOrgSlug string // fallback org slug for bare (same-org) refs
}

func (f *readFetcher) FetchParent(ctx context.Context, ref, version string) (*Doc, []ExtendRef, []string, error) {
	orgSlug, name := splitRef(ref, f.callerOrgSlug)
	frame, err := f.s.repo.GetFrameBySlugName(ctx, orgSlug, name)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w: %s", ErrParentUnreadable, ref)
	}
	canRead, err := rbac.Can(ctx, f.s.lookup, f.caller, frame.OrgId, frame.Id, rbac.PermRead)
	if err != nil {
		return nil, nil, nil, err
	}
	if !canRead {
		return nil, nil, nil, fmt.Errorf("%w: %s", ErrParentUnreadable, ref)
	}
	v, _, _, err := f.s.repo.GetFrameVersion(ctx, frame.Id, version)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w: %s@%s", ErrParentUnreadable, ref, version)
	}
	doc, err := Parse(v.Content)
	if err != nil {
		return nil, nil, nil, err
	}
	return doc, doc.Extends, doc.Excludes, nil
}

// violationErr maps a *ValidationError onto an InvalidArgument Connect error
// carrying FieldViolations, so a client can attach each failure to the input
// that caused it. Shared by PublishFrame (value errors, paths like
// "slots.terminology[2].definition") and ConvertFrame (markdown structure
// errors, path "markdown").
func violationErr(err error) *connect.Error {
	cerr := connect.NewError(connect.CodeInvalidArgument, err)
	var ve *ValidationError
	if errors.As(err, &ve) {
		fv := &framesv1.FieldViolations{}
		for _, fe := range ve.Errors {
			fv.Violations = append(fv.Violations, &framesv1.FieldViolation{
				Field: fe.Path, Message: fe.Message,
			})
		}
		if detail, derr := connect.NewErrorDetail(fv); derr == nil {
			cerr.AddDetail(detail)
		}
	}
	return cerr
}

// ConvertFrame translates between the canonical slot YAML and the .frame.md
// interchange format defined by Frame Spec v0.2. It is a pure function of its
// input - it touches no storage - so it requires only that the caller is a
// member of an org. It backs the web app's markdown editor, import, and export.
//
// Converting markdown in reports only structural failures. Value-level problems
// (an unpinned inherits ref, an empty description) convert successfully and are
// left for Validate on publish, so they surface on the relevant form field
// instead of blocking an import outright.
func (s *Service) ConvertFrame(ctx context.Context, req *connect.Request[framesv1.ConvertFrameRequest]) (*connect.Response[framesv1.ConvertFrameResponse], error) {
	if _, err := s.resolveCaller(ctx); err != nil {
		return nil, err
	}
	switch src := req.Msg.GetSource().(type) {
	case *framesv1.ConvertFrameRequest_Yaml:
		doc, err := Parse(src.Yaml)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		md, err := MarshalMarkdown(doc)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		return connect.NewResponse(&framesv1.ConvertFrameResponse{Yaml: src.Yaml, Markdown: md}), nil

	case *framesv1.ConvertFrameRequest_Markdown:
		doc, err := UnmarshalMarkdown(src.Markdown)
		if err != nil {
			return nil, violationErr(err)
		}
		y, err := Marshal(doc)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		return connect.NewResponse(&framesv1.ConvertFrameResponse{Yaml: y, Markdown: src.Markdown}), nil

	default:
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("exactly one of yaml or markdown must be set"))
	}
}

// compareVersions orders two validated semantic versions, reporting false when
// either cannot be parsed. Validate already enforces the shape, so a false here
// means an unexpected form rather than user error - the caller lets it through
// rather than rejecting a document it cannot reason about.
func compareVersions(a, b string) (int, bool) {
	pa, ok := parseVersion(a)
	if !ok {
		return 0, false
	}
	pb, ok := parseVersion(b)
	if !ok {
		return 0, false
	}
	for i := range pa {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1, true
			}
			return 1, true
		}
	}
	return 0, true
}

func parseVersion(v string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
