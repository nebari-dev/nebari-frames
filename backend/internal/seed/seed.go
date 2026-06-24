// Package seed provisions an initial org and its first admin from config on
// server startup. Idempotent: existing rows are left untouched.
package seed

import (
	"context"
	"crypto/rand"
	"errors"

	"github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

type Config struct {
	OrgSlug        string
	OrgDisplayName string
	AdminSub       string
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
	if cfg.AdminSub == "" {
		return nil
	}
	if _, err := repo.GetMembership(ctx, cfg.AdminSub); errors.Is(err, store.ErrNotFound) {
		return repo.UpsertMembership(ctx, &framesv1.Membership{
			OrgId: org.Id, UserSub: cfg.AdminSub, Role: "admin", AddedAt: timestamppb.Now(),
		})
	} else if err != nil {
		return err
	}
	return nil
}
