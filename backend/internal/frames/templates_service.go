package frames

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	"github.com/nebari-dev/nebari-frames/backend/internal/rbac"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

// The five template RPCs. Kept out of service.go, which is already large,
// following the precedent of admin.go.
//
// Templates are an authoring affordance, not Frames, so there are no per-item
// grants: org membership is the only access control, and it is enforced by
// passing the caller's org into every store accessor. Managing templates is
// admin-only because a template is an org standard, which puts it beside member
// management rather than beside Frame authoring.

// requirementToProto converts the level for the wire. The proto enum has an
// explicit UNSPECIFIED that maps to optional, so an older client that leaves
// the field unset gets the permissive reading rather than an accidental
// requirement. The reverse direction belongs to the write handlers in Task 8,
// which is the only place anything decodes a caller-supplied level.
func requirementToProto(r Requirement) framesv1.Requirement {
	switch r {
	case RequirementRecommended:
		return framesv1.Requirement_REQUIREMENT_RECOMMENDED
	case RequirementRequired:
		return framesv1.Requirement_REQUIREMENT_REQUIRED
	default:
		return framesv1.Requirement_REQUIREMENT_OPTIONAL
	}
}

func fieldRulesToProto(rules map[string]FieldRule) map[string]*framesv1.FieldRule {
	if len(rules) == 0 {
		return nil
	}
	out := make(map[string]*framesv1.FieldRule, len(rules))
	for key, rule := range rules {
		out[key] = &framesv1.FieldRule{Level: requirementToProto(rule.Level), Note: rule.Note}
	}
	return out
}

func templateSummaryToProto(t *Template) *framesv1.FrameTemplateSummary {
	return &framesv1.FrameTemplateSummary{
		Id: t.ID, Title: t.Title, Description: t.Description, Builtin: IsBuiltin(t.ID),
	}
}

func templateToProto(t *Template) (*framesv1.FrameTemplate, error) {
	prefill, err := MarshalPrefill(t.Prefill)
	if err != nil {
		return nil, err
	}
	return &framesv1.FrameTemplate{
		Id: t.ID, Title: t.Title, Description: t.Description, Builtin: IsBuiltin(t.ID),
		Prefill: prefill, FieldRules: fieldRulesToProto(t.FieldRules),
	}, nil
}

// rowToTemplate decodes a stored row into the internal model.
func rowToTemplate(row *store.FrameTemplate) (*Template, error) {
	prefill, err := ParsePrefill(row.Prefill)
	if err != nil {
		return nil, fmt.Errorf("template %s: %w", row.ID, err)
	}
	rules, err := ParseFieldRules(row.FieldRules)
	if err != nil {
		return nil, fmt.Errorf("template %s: %w", row.ID, err)
	}
	return &Template{
		ID: row.ID, Title: row.Title, Description: row.Description,
		Prefill: prefill, FieldRules: rules,
	}, nil
}

// lookupTemplate resolves an id to a template the caller may use: a built-in, or
// a row in the caller's own org. Everything else, including another org's row,
// reports NotFound so existence does not leak - the same rule frames follow.
func (s *Service) lookupTemplate(ctx context.Context, caller rbac.Caller, id string) (*Template, error) {
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	if IsBuiltin(id) {
		tmpl, ok := BuiltinTemplate(id)
		if !ok {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no template %q", id))
		}
		return &tmpl, nil
	}
	row, err := s.repo.GetFrameTemplate(ctx, caller.OrgID, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no template %q", id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	tmpl, err := rowToTemplate(row)
	if err != nil {
		// A row this server wrote failed to decode, so the data is corrupt
		// rather than the request wrong.
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return tmpl, nil
}

func (s *Service) ListFrameTemplates(ctx context.Context, _ *connect.Request[framesv1.ListFrameTemplatesRequest]) (*connect.Response[framesv1.ListFrameTemplatesResponse], error) {
	caller, err := s.resolveCaller(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListFrameTemplatesByOrg(ctx, caller.OrgID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	// Built-ins first (Blank leads), then the org's own. No shadowing: an org
	// template sharing a built-in's title sits beside it rather than replacing
	// it, because a precedence rule here would be a permanent source of "why am
	// I seeing the wrong one". The UI groups them instead.
	out := make([]*framesv1.FrameTemplateSummary, 0, len(rows)+6)
	for _, tmpl := range BuiltinTemplates() {
		out = append(out, templateSummaryToProto(&tmpl))
	}
	for _, row := range rows {
		out = append(out, &framesv1.FrameTemplateSummary{
			Id: row.ID, Title: row.Title, Description: row.Description, Builtin: false,
		})
	}
	return connect.NewResponse(&framesv1.ListFrameTemplatesResponse{
		Templates: out,
		CanManage: caller.Role == rbac.RoleAdmin,
	}), nil
}

func (s *Service) GetFrameTemplate(ctx context.Context, req *connect.Request[framesv1.GetFrameTemplateRequest]) (*connect.Response[framesv1.GetFrameTemplateResponse], error) {
	caller, err := s.resolveCaller(ctx)
	if err != nil {
		return nil, err
	}
	tmpl, err := s.lookupTemplate(ctx, caller, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	pb, err := templateToProto(tmpl)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&framesv1.GetFrameTemplateResponse{Template: pb}), nil
}

// TEMPORARY (Task 7): placeholder so the FrameServiceHandler assertion in service.go
// compiles. Task 8 replaces this with the real handler. If you are reading this in
// merged code, it escaped review.
func (s *Service) CreateFrameTemplate(context.Context, *connect.Request[framesv1.CreateFrameTemplateRequest]) (*connect.Response[framesv1.CreateFrameTemplateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented until Task 8"))
}

// TEMPORARY (Task 7): placeholder so the FrameServiceHandler assertion in service.go
// compiles. Task 8 replaces this with the real handler. If you are reading this in
// merged code, it escaped review.
func (s *Service) UpdateFrameTemplate(context.Context, *connect.Request[framesv1.UpdateFrameTemplateRequest]) (*connect.Response[framesv1.UpdateFrameTemplateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented until Task 8"))
}

// TEMPORARY (Task 7): placeholder so the FrameServiceHandler assertion in service.go
// compiles. Task 8 replaces this with the real handler. If you are reading this in
// merged code, it escaped review.
func (s *Service) DeleteFrameTemplate(context.Context, *connect.Request[framesv1.DeleteFrameTemplateRequest]) (*connect.Response[framesv1.DeleteFrameTemplateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented until Task 8"))
}
