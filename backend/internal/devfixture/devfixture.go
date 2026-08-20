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

	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
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
	isTemplate  bool
	versions    []frameVersion // oldest first
}

var members = []pendingMember{
	{email: "viewer@dev", role: "viewer"},
	{email: "publisher@dev", role: "publisher"},
	{email: "admin@dev", role: "admin"},
}

// mustMarshalDoc builds a Frame document and marshals it to YAML via the
// canonical frames.Marshal, so fixture content is always shaped exactly like
// what frames.Parse/frames.Validate accept. It panics on error: the doc
// fields below are static/derived from orgSlug and controlled entirely by
// this package, so a marshal failure would indicate a programming error, not
// bad input.
func mustMarshalDoc(doc *frames.Doc) string {
	b, err := frames.Marshal(doc)
	if err != nil {
		panic(fmt.Sprintf("devfixture: marshal frame %q: %v", doc.Name, err))
	}
	return string(b)
}

// buildFrames constructs the fixture frame set for the given org slug. The
// two inheriting frames (pytorch-gpu, team-notebook) reference their parent
// via an org-qualified extends ref (<orgSlug>/<parent-name>), matching the
// canonical schema's requirement that extends.ref be "org_slug/frame_name".
// Content is built from frames.Doc + frames.Marshal rather than hand-written
// YAML so it can never drift from what frames.Parse/frames.Validate accept.
//
// The set forms a three-level inheritance chain plus one standalone frame:
//
//	base-ml-env (v1.0.0, v2.0.0)
//	  └── pytorch-gpu (v1.0.0)        extends base-ml-env@2.0.0
//	        └── team-notebook (v1.0.0) extends pytorch-gpu@1.0.0
//	standalone-frame (v1.0.0)          no parents
//
// Every frame carries a real free-form markdown body so the detail view and
// the inheritance resolver have representative data to render.
func buildFrames(orgSlug string) []fixtureFrame {
	return []fixtureFrame{
		{
			id: idBaseMLEnv, name: "base-ml-env", description: "Base machine-learning environment: shared conventions every ML frame inherits.",
			versions: []frameVersion{
				{version: "1.0.0", changelog: "Initial base environment",
					content: mustMarshalDoc(&frames.Doc{
						Name:        "base-ml-env",
						Description: "Base machine-learning environment: shared conventions every ML frame inherits.",
						Version:     "1.0.0",
						Visibility:  "internal",
						Scope:       "company",
						Maintainer:  "platform engineering",
						Body: `Pin every dependency to an exact version; never use unbounded ranges. Reproducibility is non-negotiable: the same frame must resolve identically on every machine.

Prefer conda-forge as the primary channel for scientific packages.

Changes to pinned tooling require a version bump and a changelog entry. Favor declarative configuration over imperative setup scripts.`,
					})},
				{version: "2.0.0", changelog: "Bump pinned tooling (Python 3.12)",
					content: mustMarshalDoc(&frames.Doc{
						Name:        "base-ml-env",
						Description: "Base machine-learning environment: shared conventions every ML frame inherits.",
						Version:     "2.0.0",
						Visibility:  "internal",
						Scope:       "company",
						Maintainer:  "platform engineering",
						Body: `Pin every dependency to an exact version; never use unbounded ranges. Reproducibility is non-negotiable: the same frame must resolve identically on every machine. Rebuilds should be byte-identical from the lockfile.

Prefer conda-forge as the primary channel for scientific packages. Target Python 3.12 unless a downstream frame pins otherwise. Build and publish internal conda packages to the private channel.

Changes to pinned tooling require a version bump and a changelog entry. Favor declarative configuration over imperative setup scripts.`,
					})},
			},
		},
		{
			id: idPyTorchGPU, name: "pytorch-gpu", description: "PyTorch GPU environment layered on the base ML env, tuned for CUDA training.",
			versions: []frameVersion{
				{version: "1.0.0", changelog: "PyTorch on top of base",
					content: mustMarshalDoc(&frames.Doc{
						Name:        "pytorch-gpu",
						Description: "PyTorch GPU environment layered on the base ML env, tuned for CUDA training.",
						Version:     "1.0.0",
						Visibility:  "internal",
						Scope:       "department",
						Maintainer:  "ml platform",
						Extends: []frames.ExtendRef{
							{Ref: orgSlug + "/base-ml-env", Version: "2.0.0"},
						},
						Body: `The stack is torch>=2.2 with torchvision on cuda-toolkit 12.1 and cudnn; ` + "`nvidia-smi`" + ` is available for GPU introspection. Single-node, single-or-multi-GPU: data loaders run on CPU workers feeding the GPU, and checkpoints are written to the shared volume.

Pin the CUDA toolkit version to match the target driver; mismatches fail silently at runtime. Always guard GPU code with a CPU fallback so tests run in CI without a GPU.

Enable mixed precision for training runs unless numerical stability requires float32. When memory is tight, tune batch size and gradient accumulation before reaching for a bigger GPU, and profile GPU utilization to find data-loading bottlenecks.`,
					}),
					extends: []store.ParentEdge{{ParentFrameID: idBaseMLEnv, ParentVersion: "2.0.0", OrderIndex: 0}}},
			},
		},
		{
			id: idTeamNotebook, name: "team-notebook", description: "Team notebook profile: PyTorch GPU env plus the data-science team's shared conventions.",
			versions: []frameVersion{
				{version: "1.0.0", changelog: "Team notebook on PyTorch",
					content: mustMarshalDoc(&frames.Doc{
						Name:        "team-notebook",
						Description: "Team notebook profile: PyTorch GPU env plus the data-science team's shared conventions.",
						Version:     "1.0.0",
						Visibility:  "private",
						Scope:       "project",
						Maintainer:  "data science",
						Extends: []frames.ExtendRef{
							{Ref: orgSlug + "/pytorch-gpu", Version: "1.0.0"},
						},
						Body: `Commit notebooks with cleared outputs; large embedded outputs bloat the repo. Notebooks should read like documented experiments, not scratch pads.

Shared datasets live under /shared/data (read-only); never copy them into a home directory. Long-running jobs belong in the batch queue, not in an interactive notebook kernel.

Exploration happens in notebooks; promising results are promoted to a tracked, reproducible pipeline before any production use. New shared JupyterLab extensions are proposed in the team channel and added here via a version bump.`,
					}),
					extends: []store.ParentEdge{{ParentFrameID: idPyTorchGPU, ParentVersion: "1.0.0", OrderIndex: 0}}},
			},
		},
		{
			id: idStandalone, name: "standalone-frame", description: "Standalone data-cleaning frame with no parents, for exercising the non-inheriting case.",
			isTemplate: true, // exercises the "start from a template" picker
			versions: []frameVersion{
				{version: "1.0.0", changelog: "Initial standalone frame",
					content: mustMarshalDoc(&frames.Doc{
						Name:        "standalone-frame",
						Description: "Standalone data-cleaning frame with no parents, for exercising the non-inheriting case.",
						Version:     "1.0.0",
						Visibility:  "shared",
						Scope:       "project",
						Maintainer:  "data science",
						Template:    true,
						Body: `Never mutate the raw input in place; write cleaned output to a new location, and record every transformation so the cleaning run is fully auditable.

Profile a dataset for missing values, outliers, and type inconsistencies before cleaning it, and write idempotent, re-runnable transforms. Validate assumptions loudly and fail fast on malformed input.

Raw data lands, is validated, is cleaned into a tidy table (each variable a column, each observation a row), and only then is handed to downstream analysis.`,
					})},
			},
		},
	}
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

	for _, f := range buildFrames(orgSlug) {
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
				IsTemplate: f.isTemplate,
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
