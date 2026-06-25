package frames

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/nebari-dev/nebari-frames/backend/internal/rbac"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

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
			org, _ := s.repo.GetOrgByID(ctx, c.OrgId)
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
