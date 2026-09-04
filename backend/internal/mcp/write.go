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
// one clears it. Without it, an AI updating only the body would silently erase
// the Frame's metadata and - far worse - its inheritance edges.
//
// TestWriteFrameInputCoversDocFields walks the frames.Doc field set, so adding a
// document field without adding it here fails.
type writeFrameInput struct {
	Name      string `json:"name" jsonschema:"Frame name: lowercase letters, digits and dashes, e.g. brand-voice"`
	Version   string `json:"version" jsonschema:"semantic version for the new revision, e.g. 1.1.0; must not already exist"`
	Changelog string `json:"changelog,omitempty" jsonschema:"optional note describing what changed in this version"`
	// Required by update_frame. Inferring it server-side would defeat the
	// purpose: the window that loses a change is between the client's read and
	// its write, and only the client knows what it read.
	BaseVersion string `json:"base_version,omitempty" jsonschema:"required for update_frame: the version shown by get_frame when you read this Frame. The update is refused if someone else has published since, so you can re-read and reapply instead of silently overwriting their change"`

	Description *string `json:"description,omitempty" jsonschema:"one-line summary of what this Frame carries. Required when creating; when updating, omit to keep the current one"`
	Visibility  *string `json:"visibility,omitempty" jsonschema:"declared intent, one of private, internal, shared, public; omit to keep the current one, pass an empty string to clear it. Access is decided by registry permissions, not by this field"`
	Scope       *string `json:"scope,omitempty" jsonschema:"who this Frame applies to, e.g. company or team-platform; omit to keep the current one, pass an empty string to clear it"`
	Maintainer  *string `json:"maintainer,omitempty" jsonschema:"who owns this Frame; omit to keep the current one, pass an empty string to clear it"`

	// The whole content of a Frame. Frame Spec v0.2 defines no body structure,
	// so there is nothing to break it into: headings, lists and prose are the
	// author's choice. Callers that read a legacy slot-shaped Frame get it back
	// already rendered as markdown, so editing and republishing it is a plain
	// string edit rather than a schema migration.
	Body *string `json:"body,omitempty" jsonschema:"the Frame's content as free-form markdown. Structure it however the guidance reads best - headings, lists, prose. Omit to keep the current body, pass an empty string to clear it"`
	// Registry metadata rather than spec metadata, but MCP writes must carry it:
	// without it create_frame could not produce a template at all, and an
	// omitted-means-keep pointer stops update_frame de-listing one by accident.
	Template *bool `json:"template,omitempty" jsonschema:"true to offer this Frame in the authoring UI's template picker; omit to keep the current setting"`

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
	setString(&d.Body, in.Body)

	if in.Template != nil {
		d.Template = *in.Template
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
		return rs.publish(ctx, in, in.applyTo(&frames.Doc{}), frames.PublishCreate, "")
	}
}

// updateFrameTool publishes a new version of an existing Frame, merging the
// caller's changes onto the Frame's current document. PublishDoc enforces edit
// permission on the target and rejects an unknown name.
//
// The merge base is SourceDoc - the Frame's OWN document - and not the composed
// form get_frame returns. Merging onto a resolved document would bake every
// ancestor's body into the child and drop its extends edges, quietly destroying
// the inheritance graph.
func (rs *resourceServer) updateFrameTool(claims *auth.Claims) gomcp.ToolHandlerFor[writeFrameInput, any] {
	return func(ctx context.Context, _ *gomcp.CallToolRequest, in writeFrameInput) (*gomcp.CallToolResult, any, error) {
		ctx = auth.WithClaims(ctx, claims)
		if in.BaseVersion == "" {
			return errorResult("base_version is required: read the Frame first with " +
				"get_frame source=true and pass the version it reports, so a change " +
				"published by someone else in the meantime is not silently overwritten"), nil, nil
		}
		current, err := rs.src.SourceDoc(ctx, in.Name, "")
		if err != nil {
			return errorResult(writeErrorText(err)), nil, nil
		}
		// Merge onto the current document, but assert against the version the
		// caller actually read. Deriving the base from this read instead would
		// make the check vacuous - it would always match.
		return rs.publish(ctx, in, in.applyTo(current), frames.PublishUpdate, in.BaseVersion)
	}
}

func (rs *resourceServer) publish(ctx context.Context, in writeFrameInput, doc *frames.Doc, intent frames.PublishIntent, baseVersion string) (*gomcp.CallToolResult, any, error) {
	frame, version, err := rs.src.PublishDocFrom(ctx, doc, in.Changelog, intent, baseVersion)
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
		// A concurrent update moved the frame on after this one read it. The
		// message names both versions, so a client can re-read and retry.
		return "frame changed while you were editing it: " + msg
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
