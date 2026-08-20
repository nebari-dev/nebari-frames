package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
)

// termInput is one vocabulary entry in the terminology slot.
type termInput struct {
	Term       string `json:"term" jsonschema:"the term being defined"`
	Definition string `json:"definition" jsonschema:"what the term means in this organization"`
}

// extendInput is a pinned reference to a parent Frame.
type extendInput struct {
	Ref     string `json:"ref" jsonschema:"parent Frame reference as org_slug/frame_name"`
	Version string `json:"version" jsonschema:"the parent version to pin, e.g. 1.0.0"`
}

// writeFrameInput is the typed input shared by create_frame and update_frame. It
// mirrors frames.Doc field for field so an MCP client is guided by the tool
// schema instead of authoring YAML blind.
//
// Every optional field is a pointer or a slice so that "not mentioned" is
// distinguishable from "set to empty". update_frame relies on that distinction:
// an omitted field keeps the Frame's current value, while an explicitly empty
// one clears it. Without it, an AI updating a single slot would silently erase
// the Frame's metadata and - far worse - its inheritance edges.
//
// TestWriteFrameInputCoversDocFields walks frames.SlotTable and the frames.Doc
// field set, so adding a slot or a document field without adding it here fails.
type writeFrameInput struct {
	Name      string `json:"name" jsonschema:"Frame name: lowercase letters, digits and dashes, e.g. brand-voice"`
	Version   string `json:"version" jsonschema:"semantic version for the new revision, e.g. 1.1.0; must not already exist"`
	Changelog string `json:"changelog,omitempty" jsonschema:"optional note describing what changed in this version"`

	Description *string `json:"description,omitempty" jsonschema:"one-line summary of what this Frame carries. Required when creating; when updating, omit to keep the current one"`
	Visibility  *string `json:"visibility,omitempty" jsonschema:"declared intent, one of private, internal, shared, public; omit to keep the current one, pass an empty string to clear it. Access is decided by registry permissions, not by this field"`
	Scope       *string `json:"scope,omitempty" jsonschema:"who this Frame applies to, e.g. company or team-platform; omit to keep the current one, pass an empty string to clear it"`
	Maintainer  *string `json:"maintainer,omitempty" jsonschema:"who owns this Frame; omit to keep the current one, pass an empty string to clear it"`

	Terminology     []termInput `json:"terminology,omitempty" jsonschema:"named concepts and their definitions; omit to keep the current list, pass an empty list to clear it"`
	Rules           []string    `json:"rules,omitempty" jsonschema:"constraints that must be followed; omit to keep the current list, pass an empty list to clear it"`
	Skills          []string    `json:"skills,omitempty" jsonschema:"capabilities this Frame expects; omit to keep, empty list to clear"`
	Prompts         []string    `json:"prompts,omitempty" jsonschema:"reusable prompts; omit to keep, empty list to clear"`
	ToolSpecs       *string     `json:"tool_specs,omitempty" jsonschema:"tool specifications, as markdown; omit to keep the current text, pass an empty string to clear it"`
	Goals           *string     `json:"goals,omitempty" jsonschema:"what the organization is trying to achieve, as markdown; omit to keep the current text, pass an empty string to clear it"`
	Style           *string     `json:"style,omitempty" jsonschema:"voice and formatting conventions, as markdown; omit to keep the current text, pass an empty string to clear it"`
	Norms           *string     `json:"norms,omitempty" jsonschema:"team norms and expectations, as markdown; omit to keep the current text, pass an empty string to clear it"`
	Architecture    *string     `json:"architecture,omitempty" jsonschema:"system architecture context, as markdown; omit to keep the current text, pass an empty string to clear it"`
	BusinessProcess *string     `json:"business_process,omitempty" jsonschema:"business process context, as markdown; omit to keep the current text, pass an empty string to clear it"`

	Extends  []extendInput `json:"extends,omitempty" jsonschema:"parent Frames this one inherits from, each pinned to a version; later parents win. Omit to keep the current inheritance, pass an empty list to remove all parents"`
	Excludes []string      `json:"excludes,omitempty" jsonschema:"parent references to exclude from inheritance; omit to keep, empty list to clear"`
}

// applyTo overlays the supplied fields onto base, which is the Frame's current
// document for an update and an empty document for a create. Fields the caller
// omitted are left as they were. Validation is deliberately not performed here:
// frames.PublishDoc runs the canonical validator, so there is one definition of
// a valid Frame.
func (in writeFrameInput) applyTo(base *frames.Doc) *frames.Doc {
	d := *base
	d.Name = in.Name
	d.Version = in.Version

	setString(&d.Description, in.Description)
	setString(&d.Visibility, in.Visibility)
	setString(&d.Scope, in.Scope)
	setString(&d.Maintainer, in.Maintainer)

	setString(&d.Slots.ToolSpecs, in.ToolSpecs)
	setString(&d.Slots.Goals, in.Goals)
	setString(&d.Slots.Style, in.Style)
	setString(&d.Slots.Norms, in.Norms)
	setString(&d.Slots.Architecture, in.Architecture)
	setString(&d.Slots.BusinessProcess, in.BusinessProcess)

	if in.Rules != nil {
		d.Slots.Rules = in.Rules
	}
	if in.Skills != nil {
		d.Slots.Skills = in.Skills
	}
	if in.Prompts != nil {
		d.Slots.Prompts = in.Prompts
	}
	if in.Terminology != nil {
		terms := make([]frames.Term, len(in.Terminology))
		for i, t := range in.Terminology {
			terms[i] = frames.Term{Term: t.Term, Definition: t.Definition}
		}
		d.Slots.Terminology = terms
	}
	if in.Extends != nil {
		refs := make([]frames.ExtendRef, len(in.Extends))
		for i, e := range in.Extends {
			refs[i] = frames.ExtendRef{Ref: e.Ref, Version: e.Version}
		}
		d.Extends = refs
	}
	if in.Excludes != nil {
		d.Excludes = in.Excludes
	}
	return &d
}

// setString assigns only when the caller supplied the field.
func setString(dst *string, src *string) {
	if src != nil {
		*dst = *src
	}
}

// createFrameTool publishes a new Frame. It performs no permission check of its
// own: PublishDoc enforces the publisher/admin role and rejects a name that
// already exists, so the MCP surface cannot drift from the Connect API.
func (rs *resourceServer) createFrameTool(claims *auth.Claims) gomcp.ToolHandlerFor[writeFrameInput, any] {
	return func(ctx context.Context, _ *gomcp.CallToolRequest, in writeFrameInput) (*gomcp.CallToolResult, any, error) {
		ctx = auth.WithClaims(ctx, claims)
		return rs.publish(ctx, in, in.applyTo(&frames.Doc{}), frames.PublishCreate)
	}
}

// updateFrameTool publishes a new version of an existing Frame, merging the
// caller's changes onto the Frame's current document. PublishDoc enforces edit
// permission on the target and rejects an unknown name.
//
// The merge base is SourceDoc - the Frame's OWN document - and not the composed
// form get_frame returns. Merging onto a resolved document would copy every
// parent's slots into the child and drop its extends edges, quietly destroying
// the inheritance graph.
func (rs *resourceServer) updateFrameTool(claims *auth.Claims) gomcp.ToolHandlerFor[writeFrameInput, any] {
	return func(ctx context.Context, _ *gomcp.CallToolRequest, in writeFrameInput) (*gomcp.CallToolResult, any, error) {
		ctx = auth.WithClaims(ctx, claims)
		current, err := rs.src.SourceDoc(ctx, in.Name, "")
		if err != nil {
			return errorResult(writeErrorText(err)), nil, nil
		}
		return rs.publish(ctx, in, in.applyTo(current), frames.PublishUpdate)
	}
}

func (rs *resourceServer) publish(ctx context.Context, in writeFrameInput, doc *frames.Doc, intent frames.PublishIntent) (*gomcp.CallToolResult, any, error) {
	frame, version, err := rs.src.PublishDoc(ctx, doc, in.Changelog, intent)
	if err != nil {
		return errorResult(writeErrorText(err)), nil, nil
	}
	return textResult(fmt.Sprintf("Published %s@%s.", frame.Name, version.Version)), nil, nil
}

// writeErrorText turns a service error into text an AI client can act on. The
// connect code carries the meaning, so it is mapped rather than pattern-matched
// on message strings.
func writeErrorText(err error) string {
	msg := connectMessage(err)
	switch connect.CodeOf(err) {
	case connect.CodePermissionDenied:
		return "permission denied: " + msg
	case connect.CodeNotFound:
		return "frame not found: " + msg
	case connect.CodeAlreadyExists:
		// Deliberately reveals that the name is taken, which is what makes the
		// error actionable for a client picking a name. The pre-existing Connect
		// path already disclosed as much by returning a permission error for a
		// name the caller cannot edit.
		return "already exists: " + msg
	case connect.CodeInvalidArgument:
		return "invalid frame: " + msg
	case connect.CodeUnauthenticated:
		return "not authenticated: " + msg
	case connect.CodeFailedPrecondition:
		// Defensive. publish does not check acyclicity - version pinning makes a
		// true cycle unreachable there - so this arm exists for the resolver
		// errors a future change could surface, not because writes detect cycles.
		return "invalid inheritance: " + msg
	default:
		// Internal faults must not leak storage or wiring detail to the client.
		return "could not publish frame"
	}
}

// connectMessage extracts the human-readable part of a connect error, dropping
// the code prefix connect adds to Error().
func connectMessage(err error) string {
	var cerr *connect.Error
	if errors.As(err, &cerr) {
		return cerr.Message()
	}
	return strings.TrimSpace(err.Error())
}
