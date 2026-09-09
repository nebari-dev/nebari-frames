package frames

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
// requirement. requirementFromProto below is the reverse direction, used by the
// write handlers: they are the only place a caller-supplied level is decoded.
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
	//
	// This order is the contract `frames template list` presents and the one
	// this API guarantees; it is not what a screen has to follow. The web
	// picker (TemplatePicker) renders the org's group above built-ins, because
	// grouping by origin reads better there - that choice does not make this
	// order stale, it just means the CLI is this ordering's real consumer.
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

// requirementFromProto reads the wire level. UNSPECIFIED maps to optional, so an
// older client that leaves the field unset gets the permissive reading rather
// than an accidental requirement.
func requirementFromProto(r framesv1.Requirement) Requirement {
	switch r {
	case framesv1.Requirement_REQUIREMENT_RECOMMENDED:
		return RequirementRecommended
	case framesv1.Requirement_REQUIREMENT_REQUIRED:
		return RequirementRequired
	default:
		return RequirementOptional
	}
}

// fieldRulesFromProto converts and validates client-supplied rules. An unknown
// slot key is refused: silently keeping a rule that can never fire would let an
// admin believe they had imposed a standard they had not.
func fieldRulesFromProto(in map[string]*framesv1.FieldRule) (map[string]FieldRule, error) {
	out := make(map[string]FieldRule, len(in))
	for key, rule := range in {
		if _, ok := slotByKey(key); !ok {
			return nil, fmt.Errorf("field_rules names unknown slot %q", key)
		}
		if rule == nil {
			continue
		}
		out[key] = FieldRule{Level: requirementFromProto(rule.Level), Note: rule.Note}
	}
	return out, nil
}

// templateInput is the validated, decoded form of a create or update request.
// Both RPCs take the same fields, so they share one validator: a rule that
// applied on create but not on update would let an admin edit their way around
// it.
type templateInput struct {
	title       string
	description string
	prefill     []byte
	rules       []byte
}

func validateTemplateInput(title, description string, prefill []byte, rules map[string]*framesv1.FieldRule) (templateInput, error) {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if title == "" {
		return templateInput{}, connect.NewError(connect.CodeInvalidArgument, errors.New("title is required"))
	}
	if description == "" {
		return templateInput{}, connect.NewError(connect.CodeInvalidArgument, errors.New("description is required"))
	}
	// An empty prefill is legitimate and means "seed no content", which is what
	// every built-in does. Normalize it to a parseable document so the stored
	// blob is always something ParsePrefill accepts.
	if len(prefill) == 0 {
		empty, err := MarshalPrefill(Prefill{})
		if err != nil {
			return templateInput{}, connect.NewError(connect.CodeInternal, err)
		}
		prefill = empty
	} else {
		decodedPrefill, err := ParsePrefill(prefill)
		if err != nil {
			return templateInput{}, connect.NewError(connect.CodeInvalidArgument, err)
		}
		// A starter that cannot be published is not a starter: these bytes are
		// spliced verbatim into `frames template init`'s scaffold and into the
		// authoring form, so a blank list row saved here becomes a publish
		// failure - "slots.rules[0]: must not be empty" - in a file some other
		// author was handed as ready to fill in. Held to the same content rules
		// a Frame is, at the boundary where the bytes arrive.
		if errs := contentErrors(&decodedPrefill.Slots, decodedPrefill.Extends); len(errs) > 0 {
			return templateInput{}, connect.NewError(connect.CodeInvalidArgument, &ValidationError{Errors: errs})
		}
	}
	decoded, err := fieldRulesFromProto(rules)
	if err != nil {
		return templateInput{}, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rulesJSON, err := MarshalFieldRules(decoded)
	if err != nil {
		return templateInput{}, connect.NewError(connect.CodeInternal, err)
	}
	return templateInput{title: title, description: description, prefill: prefill, rules: rulesJSON}, nil
}

// respondWithRow reads back what was written and returns it, so a caller always
// sees the stored form rather than an optimistic echo of its own request.
func (s *Service) respondWithRow(ctx context.Context, orgID, id string) (*framesv1.FrameTemplate, error) {
	row, err := s.repo.GetFrameTemplate(ctx, orgID, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	tmpl, err := rowToTemplate(row)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return templateToProto(tmpl)
}

func (s *Service) CreateFrameTemplate(ctx context.Context, req *connect.Request[framesv1.CreateFrameTemplateRequest]) (*connect.Response[framesv1.CreateFrameTemplateResponse], error) {
	caller, err := s.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	in, err := validateTemplateInput(req.Msg.Title, req.Msg.Description, req.Msg.Prefill, req.Msg.FieldRules)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	row := &store.FrameTemplate{
		ID: newID(), OrgID: caller.OrgID, Title: in.title, Description: in.description,
		Prefill: in.prefill, FieldRules: in.rules, CreatedBy: caller.Subject,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.CreateFrameTemplate(ctx, row); err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			return nil, connect.NewError(connect.CodeAlreadyExists,
				fmt.Errorf("a template titled %q already exists in this organization", in.title))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pb, err := s.respondWithRow(ctx, caller.OrgID, row.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&framesv1.CreateFrameTemplateResponse{Template: pb}), nil
}

func (s *Service) UpdateFrameTemplate(ctx context.Context, req *connect.Request[framesv1.UpdateFrameTemplateRequest]) (*connect.Response[framesv1.UpdateFrameTemplateResponse], error) {
	caller, err := s.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	// Built-ins are compiled in, so there is no row to change. Refused for an
	// admin too: this is a property of the artifact, not a permission.
	if IsBuiltin(req.Msg.Id) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("built-in templates cannot be edited; create an organization template instead"))
	}
	in, err := validateTemplateInput(req.Msg.Title, req.Msg.Description, req.Msg.Prefill, req.Msg.FieldRules)
	if err != nil {
		return nil, err
	}
	// The store matches on id AND org, so a row in another org is simply not
	// found. There is no read-then-write window to lose a race in.
	err = s.repo.UpdateFrameTemplate(ctx, &store.FrameTemplate{
		ID: req.Msg.Id, OrgID: caller.OrgID, Title: in.title, Description: in.description,
		Prefill: in.prefill, FieldRules: in.rules, UpdatedAt: time.Now().UTC(),
	})
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no template %q", req.Msg.Id))
	case errors.Is(err, store.ErrAlreadyExists):
		return nil, connect.NewError(connect.CodeAlreadyExists,
			fmt.Errorf("a template titled %q already exists in this organization", in.title))
	case err != nil:
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pb, err := s.respondWithRow(ctx, caller.OrgID, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&framesv1.UpdateFrameTemplateResponse{Template: pb}), nil
}

func (s *Service) DeleteFrameTemplate(ctx context.Context, req *connect.Request[framesv1.DeleteFrameTemplateRequest]) (*connect.Response[framesv1.DeleteFrameTemplateResponse], error) {
	caller, err := s.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if IsBuiltin(req.Msg.Id) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("built-in templates cannot be deleted"))
	}
	// No dependency check, unlike DeleteFrame: a template is copied once at
	// creation and nothing references it afterwards, so deleting one cannot
	// affect any Frame made from it.
	err = s.repo.DeleteFrameTemplate(ctx, caller.OrgID, req.Msg.Id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no template %q", req.Msg.Id))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&framesv1.DeleteFrameTemplateResponse{}), nil
}
