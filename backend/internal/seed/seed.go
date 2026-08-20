// Package seed provisions an initial org and its first admin from config on
// server startup. Idempotent: existing rows are left untouched.
package seed

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"strings"

	"github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

type Config struct {
	OrgSlug        string
	OrgDisplayName string
	AdminSub       string
	AdminEmail     string
}

func Run(ctx context.Context, repo store.Repository, cfg Config) error {
	if cfg.OrgSlug == "" {
		return nil
	}
	org, err := repo.GetOrgBySlug(ctx, cfg.OrgSlug)
	if errors.Is(err, store.ErrNotFound) {
		display := cfg.OrgDisplayName
		if display == "" {
			display = cfg.OrgSlug
		}
		org = &framesv1.Org{
			Id:          ulid.MustNew(ulid.Now(), rand.Reader).String(),
			Slug:        cfg.OrgSlug,
			DisplayName: display,
			CreatedAt:   timestamppb.Now(),
		}
		if err := repo.CreateOrg(ctx, org); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if cfg.AdminSub != "" {
		existing, err := repo.GetMembership(ctx, cfg.AdminSub)
		switch {
		case errors.Is(err, store.ErrNotFound):
			if err := repo.UpsertMembership(ctx, &framesv1.Membership{
				OrgId: org.Id, UserSub: cfg.AdminSub, Role: "admin", AddedAt: timestamppb.Now(),
			}); err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			// The configured admin already has a membership. That is the normal
			// case on restart, but it is also what a default-role deployment
			// produces for someone who merely signed in - and skipping here
			// would leave the org with no admin and no way to make one. Promote
			// only when the org has none, so a deliberate demotion sticks.
			promoted, err := promoteIfNoAdmins(ctx, repo, org.Id, existing)
			if err != nil {
				return err
			}
			if promoted {
				slog.Warn("seed: promoted configured admin because the organization had none",
					"org", org.Slug, "user_sub", cfg.AdminSub)
			}
		}
	}

	if cfg.AdminEmail != "" {
		// Idempotent: skip if already invited (pending) or already an active member.
		if _, err := repo.GetPendingMembershipByEmail(ctx, cfg.AdminEmail); err == nil {
			return nil
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		members, err := repo.ListMembershipsByOrg(ctx, org.Id)
		if err != nil {
			return err
		}
		for _, m := range members {
			if strings.EqualFold(m.Email, cfg.AdminEmail) && m.UserSub != "" {
				// Already an active member. Same reasoning as the AdminSub path:
				// with a default role configured this is what a user who simply
				// signed in looks like, so promote when the org has no admin
				// rather than silently doing nothing.
				promoted, perr := promoteIfNoAdmins(ctx, repo, org.Id, m)
				if perr != nil {
					return perr
				}
				if promoted {
					slog.Warn("seed: promoted configured admin because the organization had none",
						"org", org.Slug, "email", cfg.AdminEmail)
				}
				return nil
			}
		}
		if err := repo.AddPendingMembership(ctx, &framesv1.Membership{
			OrgId: org.Id, Email: cfg.AdminEmail, Role: "admin", AddedAt: timestamppb.Now(),
		}); err != nil && !errors.Is(err, store.ErrAlreadyExists) {
			return err
		}
		return nil
	}

	return nil
}

// promoteIfNoAdmins raises m to admin when its org has no admin at all,
// reporting whether it did. It is the break-glass path for a deployment that
// enabled a default role before configuring an admin: without it the org can
// reach zero admins, and every route that could fix that requires an admin.
func promoteIfNoAdmins(ctx context.Context, repo store.Repository, orgID string, m *framesv1.Membership) (bool, error) {
	if m.Role == "admin" {
		return false, nil
	}
	// A membership is unique per subject across all orgs, so the configured
	// admin's existing row may belong to a different org than the one being
	// seeded - which is what happens when seed.orgSlug changes while the
	// database persists. Promoting by (orgID, sub) would match nothing and the
	// resulting error would exit the server on startup, so leave it alone: the
	// operator's admin is a member of somewhere else, and this org's admin
	// problem is not fixable by rewriting that row.
	if m.OrgId != orgID {
		slog.Warn("seed: configured admin belongs to a different organization, not promoting",
			"configured_org", orgID, "membership_org", m.OrgId, "user_sub", m.UserSub)
		return false, nil
	}
	admins, err := repo.CountAdmins(ctx, orgID)
	if err != nil {
		return false, err
	}
	if admins > 0 {
		// Someone can still administer the org, so an explicit demotion of the
		// configured admin is a decision to respect, not a state to undo.
		return false, nil
	}
	if err := repo.UpdateMembershipRole(ctx, orgID, m.UserSub, m.Email, "admin"); err != nil {
		// Nothing matched. Startup must not fail over a membership that moved or
		// vanished between the read and the update.
		if errors.Is(err, store.ErrNotFound) {
			slog.Warn("seed: configured admin membership not found while promoting, skipping",
				"org", orgID, "user_sub", m.UserSub)
			return false, nil
		}
		return false, err
	}
	return true, nil
}
