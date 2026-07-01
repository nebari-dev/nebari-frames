// Package devfixture seeds representative sample data (org members and frames
// with inheritance and multiple versions) for local development. It runs only
// when SEED_DEV_FIXTURE=true and is idempotent: existing rows are left
// untouched. It must never run in production.
package devfixture

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

// Deterministic IDs let child frames reference parents without a runtime
// lookup and let re-runs detect existing rows.
const (
	idBaseMLEnv    = "fx_base_ml_env"
	idPyTorchGPU   = "fx_pytorch_gpu"
	idTeamNotebook = "fx_team_notebook"
	idStandalone   = "fx_standalone_frame"

	ownerSub = "dev-user"
)

type pendingMember struct {
	email string
	role  string
}

type frameVersion struct {
	version   string
	changelog string
	content   string
	extends   []store.ParentEdge
}

type fixtureFrame struct {
	id          string
	name        string
	description string
	versions    []frameVersion // oldest first
}

var members = []pendingMember{
	{email: "viewer@dev", role: "viewer"},
	{email: "publisher@dev", role: "publisher"},
	{email: "admin@dev", role: "admin"},
}

// Ordered parents-first so inheritance edges resolve.
var frames = []fixtureFrame{
	{
		id: idBaseMLEnv, name: "base-ml-env", description: "Base machine-learning environment",
		versions: []frameVersion{
			{version: "1.0.0", changelog: "Initial base environment",
				content: "name: base-ml-env\nversion: 1.0.0\ndescription: Base machine-learning environment\n"},
			{version: "2.0.0", changelog: "Bump pinned tooling",
				content: "name: base-ml-env\nversion: 2.0.0\ndescription: Base machine-learning environment\n"},
		},
	},
	{
		id: idPyTorchGPU, name: "pytorch-gpu", description: "PyTorch GPU environment",
		versions: []frameVersion{
			{version: "1.0.0", changelog: "PyTorch on top of base",
				content: "name: pytorch-gpu\nversion: 1.0.0\ndescription: PyTorch GPU environment\nextends:\n  - base-ml-env@2.0.0\n",
				extends: []store.ParentEdge{{ParentFrameID: idBaseMLEnv, ParentVersion: "2.0.0", OrderIndex: 0}}},
		},
	},
	{
		id: idTeamNotebook, name: "team-notebook", description: "Team notebook profile",
		versions: []frameVersion{
			{version: "1.0.0", changelog: "Team notebook on PyTorch",
				content: "name: team-notebook\nversion: 1.0.0\ndescription: Team notebook profile\nextends:\n  - pytorch-gpu@1.0.0\n",
				extends: []store.ParentEdge{{ParentFrameID: idPyTorchGPU, ParentVersion: "1.0.0", OrderIndex: 0}}},
		},
	},
	{
		id: idStandalone, name: "standalone-frame", description: "Standalone frame with no parents",
		versions: []frameVersion{
			{version: "1.0.0", changelog: "Initial standalone frame",
				content: "name: standalone-frame\nversion: 1.0.0\ndescription: Standalone frame with no parents\n"},
		},
	},
}

// Load seeds the members and frames for the given org slug. The org must already
// be seeded (by package seed). Safe to call on every dev-mode startup.
func Load(ctx context.Context, repo store.Repository, orgSlug string) error {
	if orgSlug == "" {
		return errors.New("devfixture: org slug is required")
	}
	org, err := repo.GetOrgBySlug(ctx, orgSlug)
	if err != nil {
		return fmt.Errorf("devfixture: org %q must be seeded first: %w", orgSlug, err)
	}

	for _, m := range members {
		mErr := repo.AddPendingMembership(ctx, &framesv1.Membership{
			OrgId: org.Id, Email: m.email, Role: m.role, AddedAt: timestamppb.Now(),
		})
		if mErr != nil && !errors.Is(mErr, store.ErrAlreadyExists) {
			return fmt.Errorf("devfixture: add member %q: %w", m.email, mErr)
		}
	}

	for _, f := range frames {
		if err := loadFrame(ctx, repo, org, f); err != nil {
			return err
		}
	}
	return nil
}

func loadFrame(ctx context.Context, repo store.Repository, org *framesv1.Org, f fixtureFrame) error {
	_, err := repo.GetFrameBySlugName(ctx, org.Slug, f.name)
	frameExists := err == nil
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("devfixture: lookup frame %q: %w", f.name, err)
	}

	for i, v := range f.versions {
		if frameExists {
			if _, _, _, verr := repo.GetFrameVersion(ctx, f.id, v.version); verr == nil {
				continue // version already present
			} else if !errors.Is(verr, store.ErrNotFound) {
				return fmt.Errorf("devfixture: lookup %s@%s: %w", f.name, v.version, verr)
			}
		}
		isNew := !frameExists && i == 0
		now := timestamppb.Now()
		content := []byte(v.content)
		digest := sha256.Sum256(content)
		in := store.CreateFrameVersionInput{
			Frame: &framesv1.Frame{
				Id: f.id, OrgId: org.Id, Name: f.name, Description: f.description,
				OwnerSub: ownerSub, LatestVersion: v.version, CreatedAt: now, UpdatedAt: now,
			},
			Version: &framesv1.FrameVersion{
				Version: v.version, Changelog: v.changelog, Content: content,
				Digest: hex.EncodeToString(digest[:]), SizeBytes: int64(len(content)),
				PublishedBy: ownerSub, PublishedAt: now,
			},
			Extends:    v.extends,
			IsNewFrame: isNew,
		}
		if isNew {
			in.Grants = []store.Grant{
				{SubjectType: "user", SubjectID: ownerSub, Permission: "edit"},
				{SubjectType: "user", SubjectID: ownerSub, Permission: "delete"},
				{SubjectType: "org", SubjectID: org.Id, Permission: "read"},
			}
		}
		if err := repo.CreateFrameVersion(ctx, in); err != nil && !errors.Is(err, store.ErrAlreadyExists) {
			return fmt.Errorf("devfixture: create %s@%s: %w", f.name, v.version, err)
		}
		frameExists = true // first version created; later versions are updates
	}
	return nil
}
