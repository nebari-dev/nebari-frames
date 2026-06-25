package frames

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/nebari-dev/nebari-frames/backend/internal/rbac"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

var validRoles = map[string]bool{"viewer": true, "publisher": true, "admin": true}

// requireAdmin resolves the caller and enforces the admin role.
func (s *Service) requireAdmin(ctx context.Context) (rbac.Caller, error) {
	caller, err := s.resolveCaller(ctx)
	if err != nil {
		return rbac.Caller{}, err
	}
	if caller.Role != rbac.RoleAdmin {
		return rbac.Caller{}, connect.NewError(connect.CodePermissionDenied, errors.New("admin role required"))
	}
	return caller, nil
}

func (s *Service) ListOrgMembers(ctx context.Context, _ *connect.Request[framesv1.ListOrgMembersRequest]) (*connect.Response[framesv1.ListOrgMembersResponse], error) {
	caller, err := s.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	members, err := s.repo.ListMembershipsByOrg(ctx, caller.OrgID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&framesv1.ListOrgMembersResponse{Members: members}), nil
}

func (s *Service) AddOrgMember(ctx context.Context, req *connect.Request[framesv1.AddOrgMemberRequest]) (*connect.Response[framesv1.AddOrgMemberResponse], error) {
	caller, err := s.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.Email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email is required"))
	}
	if !validRoles[req.Msg.Role] {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("role must be viewer, publisher, or admin"))
	}
	m := &framesv1.Membership{
		OrgId:   caller.OrgID,
		UserSub: "",
		Role:    req.Msg.Role,
		Email:   req.Msg.Email,
		AddedAt: timestamppb.Now(),
	}
	if err := s.repo.AddPendingMembership(ctx, m); err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("a member with that email already exists"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&framesv1.AddOrgMemberResponse{Member: m}), nil
}

func (s *Service) DeleteFrame(ctx context.Context, req *connect.Request[framesv1.DeleteFrameRequest]) (*connect.Response[framesv1.DeleteFrameResponse], error) {
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
	allowed, err := rbac.Can(ctx, s.lookup, caller, frame.OrgId, frame.Id, rbac.PermDelete)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !allowed {
		return nil, notFound // do not leak existence to non-deleters
	}

	children, err := s.repo.FrameChildren(ctx, frame.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if len(children) > 0 && !req.Msg.Force {
		refs := make([]string, 0, len(children))
		for _, c := range children {
			org, err := s.repo.GetOrgByID(ctx, c.OrgId)
			if err != nil && !errors.Is(err, store.ErrNotFound) {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			slug := ""
			if org != nil {
				slug = org.Slug
			}
			refs = append(refs, slug+"/"+c.Name)
		}
		cerr := connect.NewError(connect.CodeFailedPrecondition, errors.New("frame is a parent of other frames"))
		if detail, derr := connect.NewErrorDetail(&framesv1.DeleteBlocked{BlockingFrames: refs}); derr == nil {
			cerr.AddDetail(detail)
		}
		return nil, cerr
	}

	if err := s.repo.DeleteFrame(ctx, frame.Id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, notFound
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&framesv1.DeleteFrameResponse{}), nil
}
