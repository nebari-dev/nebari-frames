// Package rbac is the single decision point for Nebari Frames authorization.
// All read/write paths consult Can; clients are untrusted.
package rbac

import "context"

type Role string

const (
	RoleViewer    Role = "viewer"
	RolePublisher Role = "publisher"
	RoleAdmin     Role = "admin"
)

type Permission string

const (
	PermRead   Permission = "read"
	PermEdit   Permission = "edit"
	PermDelete Permission = "delete"
)

// Caller is the resolved identity + org membership of the requester.
type Caller struct {
	Subject string
	Email   string
	OrgID   string
	Role    Role
}

// Grant is a whole-frame permission grant (section_id is always NULL in MVP).
type Grant struct {
	SubjectType string // "user" | "org"
	SubjectID   string
	Permission  Permission
}

// GrantLookup fetches the grants attached to a frame.
type GrantLookup interface {
	FrameGrants(ctx context.Context, frameID string) ([]Grant, error)
}

// Can decides whether caller may perform perm on the frame (frameOrgID/frameID).
// Order: cross-org deny -> admin allow -> grant match for the user or the org.
func Can(ctx context.Context, lookup GrantLookup, caller Caller, frameOrgID, frameID string, perm Permission) (bool, error) {
	if frameOrgID != caller.OrgID {
		return false, nil
	}
	if caller.Role == RoleAdmin {
		return true, nil
	}
	grants, err := lookup.FrameGrants(ctx, frameID)
	if err != nil {
		return false, err
	}
	for _, g := range grants {
		if g.Permission != perm {
			continue
		}
		if g.SubjectType == "user" && g.SubjectID == caller.Subject {
			return true, nil
		}
		if g.SubjectType == "org" && g.SubjectID == caller.OrgID {
			return true, nil
		}
	}
	return false, nil
}

// CanPublish reports whether the caller's role may publish new frames.
func CanPublish(caller Caller) bool {
	return caller.Role == RolePublisher || caller.Role == RoleAdmin
}

// DefaultGrants are written on first publish (migration doc §3.5).
func DefaultGrants(ownerSub, orgID string) []Grant {
	return []Grant{
		{SubjectType: "user", SubjectID: ownerSub, Permission: PermEdit},
		{SubjectType: "user", SubjectID: ownerSub, Permission: PermDelete},
		{SubjectType: "org", SubjectID: orgID, Permission: PermRead},
	}
}
