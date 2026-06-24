package frames

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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
	repo   store.Repository
	lookup rbac.GrantLookup
}

var _ framesv1connect.FrameServiceHandler = (*Service)(nil)

func NewService(repo store.Repository) *Service {
	return &Service{repo: repo, lookup: grantLookup{repo}}
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

func (s *Service) PublishFrame(ctx context.Context, req *connect.Request[framesv1.PublishFrameRequest]) (*connect.Response[framesv1.PublishFrameResponse], error) {
	caller, err := s.resolveCaller(ctx)
	if err != nil {
		return nil, err
	}
	if !rbac.CanPublish(caller) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("publisher or admin role required"))
	}

	doc, err := Parse(req.Msg.Content)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if verr := Validate(doc); verr != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, verr)
	}

	org, err := s.repo.GetOrgByID(ctx, caller.OrgID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	existing, err := s.repo.GetFrameBySlugName(ctx, org.Slug, doc.Name)
	isNew := errors.Is(err, store.ErrNotFound)
	if err != nil && !isNew {
		return nil, connect.NewError(connect.CodeInternal, err)
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
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if !allowed {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("edit permission required"))
		}
		frame = existing
		frame.Description = doc.Description
		frame.LatestVersion = doc.Version
		frame.UpdatedAt = now
	}

	edges, err := s.resolveEdges(ctx, org.Slug, doc.Extends)
	if err != nil {
		return nil, err
	}
	excludeIDs, err := s.resolveExcludes(ctx, org.Slug, doc.Excludes)
	if err != nil {
		return nil, err
	}

	digest := sha256.Sum256(req.Msg.Content)
	version := &framesv1.FrameVersion{
		Version: doc.Version, Changelog: req.Msg.Changelog, Digest: hex.EncodeToString(digest[:]),
		SizeBytes: int64(len(req.Msg.Content)), PublishedBy: caller.Subject, PublishedAt: now,
		Content: req.Msg.Content,
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
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("frame version already exists"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&framesv1.PublishFrameResponse{Frame: frame, Version: version}), nil
}

func (s *Service) ListFrames(ctx context.Context, _ *connect.Request[framesv1.ListFramesRequest]) (*connect.Response[framesv1.ListFramesResponse], error) {
	caller, err := s.resolveCaller(ctx)
	if err != nil {
		return nil, err
	}
	org, err := s.repo.GetOrgByID(ctx, caller.OrgID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	all, err := s.repo.ListFramesByOrg(ctx, caller.OrgID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	resp := &framesv1.ListFramesResponse{CanCreate: rbac.CanPublish(caller)}
	for _, f := range all {
		canRead, err := rbac.Can(ctx, s.lookup, caller, f.OrgId, f.Id, rbac.PermRead)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if !canRead {
			continue
		}
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

func (s *Service) ResolveFrame(ctx context.Context, req *connect.Request[framesv1.ResolveFrameRequest]) (*connect.Response[framesv1.ResolveFrameResponse], error) {
	caller, err := s.resolveCaller(ctx)
	if err != nil {
		return nil, err
	}
	frame, version, _, _, err := s.loadForRead(ctx, caller, req.Msg.OrgSlug, req.Msg.Name, req.Msg.Version)
	if err != nil {
		return nil, err
	}
	doc, err := Parse(version.Content)
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
	_ = frame
	out, err := Marshal(resolved)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&framesv1.ResolveFrameResponse{ResolvedContent: out}), nil
}

// --- helpers ---

func (s *Service) resolveCaller(ctx context.Context) (rbac.Caller, error) {
	caller, err := orgs.ResolveCaller(ctx, s.repo)
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
func (s *Service) resolveEdges(ctx context.Context, callerOrgSlug string, refs []ExtendRef) ([]store.ParentEdge, error) {
	var out []store.ParentEdge
	for i, r := range refs {
		orgSlug, name := splitRef(r.Ref, callerOrgSlug)
		pf, err := s.repo.GetFrameBySlugName(ctx, orgSlug, name)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("extends[%d]: parent %s not found", i, r.Ref))
		}
		out = append(out, store.ParentEdge{ParentFrameID: pf.Id, ParentVersion: r.Version, OrderIndex: i})
	}
	return out, nil
}

func (s *Service) resolveExcludes(ctx context.Context, callerOrgSlug string, refs []string) ([]string, error) {
	var out []string
	for _, ref := range refs {
		orgSlug, name := splitRef(ref, callerOrgSlug)
		pf, err := s.repo.GetFrameBySlugName(ctx, orgSlug, name)
		if err != nil {
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
