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
// Every frame fills real content slots (terminology, rules, skills, prompts,
// and the free-text goals/style/norms/architecture/business_process) so the
// detail view and the inheritance resolver have representative data to render.
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
						Slots: frames.Slots{
							Terminology: []frames.Term{
								{Term: "Frame", Definition: "A versioned, composable unit of environment and agent context that other frames can extend."},
								{Term: "Environment", Definition: "The reproducible set of pinned dependencies (conda/pip) a workload runs against."},
							},
							Rules: []string{
								"Pin every dependency to an exact version; never use unbounded ranges.",
								"Reproducibility is non-negotiable: the same frame must resolve identically on every machine.",
								"Prefer conda-forge as the primary channel for scientific packages.",
							},
							Skills: []string{
								"Resolving and locking conda environments.",
								"Reading and writing reproducible dependency manifests.",
							},
							Goals: "Provide a stable, reproducible foundation so downstream frames can focus on their specialization rather than base setup.",
							Style: "Terse, explicit, and reproducible. Favor declarative configuration over imperative setup scripts.",
							Norms: "Changes to pinned tooling require a version bump and a changelog entry.",
						},
					})},
				{version: "2.0.0", changelog: "Bump pinned tooling (Python 3.12) and add packaging skill",
					content: mustMarshalDoc(&frames.Doc{
						Name:        "base-ml-env",
						Description: "Base machine-learning environment: shared conventions every ML frame inherits.",
						Version:     "2.0.0",
						Slots: frames.Slots{
							Terminology: []frames.Term{
								{Term: "Frame", Definition: "A versioned, composable unit of environment and agent context that other frames can extend."},
								{Term: "Environment", Definition: "The reproducible set of pinned dependencies (conda/pip) a workload runs against."},
								{Term: "Lockfile", Definition: "A fully-resolved, hash-pinned snapshot of an environment used for byte-identical rebuilds."},
							},
							Rules: []string{
								"Pin every dependency to an exact version; never use unbounded ranges.",
								"Reproducibility is non-negotiable: the same frame must resolve identically on every machine.",
								"Prefer conda-forge as the primary channel for scientific packages.",
								"Target Python 3.12 unless a downstream frame pins otherwise.",
							},
							Skills: []string{
								"Resolving and locking conda environments.",
								"Reading and writing reproducible dependency manifests.",
								"Building and publishing conda packages to a private channel.",
							},
							Goals: "Provide a stable, reproducible foundation so downstream frames can focus on their specialization rather than base setup.",
							Style: "Terse, explicit, and reproducible. Favor declarative configuration over imperative setup scripts.",
							Norms: "Changes to pinned tooling require a version bump and a changelog entry.",
						},
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
						Extends: []frames.ExtendRef{
							{Ref: orgSlug + "/base-ml-env", Version: "2.0.0"},
						},
						Slots: frames.Slots{
							Terminology: []frames.Term{
								{Term: "CUDA", Definition: "NVIDIA's parallel computing platform used to run PyTorch tensor ops on the GPU."},
								{Term: "Mixed precision", Definition: "Training with a mix of float16 and float32 to cut memory use and speed up compute."},
							},
							Rules: []string{
								"Pin the CUDA toolkit version to match the target driver; mismatches fail silently at runtime.",
								"Always guard GPU code with a CPU fallback so tests run in CI without a GPU.",
								"Enable mixed precision for training runs unless numerical stability requires float32.",
							},
							Skills: []string{
								"Configuring PyTorch for a specific CUDA/cuDNN version.",
								"Diagnosing out-of-memory errors and tuning batch size and gradient accumulation.",
								"Profiling GPU utilization to find data-loading bottlenecks.",
							},
							Prompts: []string{
								"Given a training script, suggest the largest batch size that fits in the available GPU memory.",
								"Review this model code and flag any operations that will silently fall back to CPU.",
							},
							ToolSpecs:    "torch>=2.2, torchvision, cuda-toolkit 12.1, cudnn. Expose `nvidia-smi` for GPU introspection.",
							Goals:        "Give ML engineers a ready-to-train GPU environment with sane defaults for CUDA, so they iterate on models rather than plumbing.",
							Architecture: "Single-node, single-or-multi-GPU. Data loaders run on CPU workers feeding the GPU; checkpoints written to the shared volume.",
						},
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
						Extends: []frames.ExtendRef{
							{Ref: orgSlug + "/pytorch-gpu", Version: "1.0.0"},
						},
						Slots: frames.Slots{
							Terminology: []frames.Term{
								{Term: "Notebook profile", Definition: "A JupyterLab configuration bundle (kernels, extensions, resource limits) applied to a team's servers."},
							},
							Rules: []string{
								"Commit notebooks with cleared outputs; large embedded outputs bloat the repo.",
								"Shared datasets live under /shared/data (read-only); never copy them into a home directory.",
								"Long-running jobs belong in the batch queue, not in an interactive notebook kernel.",
							},
							Skills: []string{
								"Using the team's shared JupyterLab extensions and kernels.",
								"Moving an exploratory notebook into a reproducible pipeline.",
							},
							Prompts: []string{
								"Convert this exploratory notebook cell into a parameterized, testable function.",
								"Suggest where in this notebook to checkpoint intermediate results to the shared volume.",
							},
							Goals:           "Let the data-science team share one reproducible, GPU-ready notebook environment with agreed-upon conventions.",
							Style:           "Collaborative and review-friendly: notebooks should read like documented experiments, not scratch pads.",
							Norms:           "New shared extensions are proposed in the team channel and added here via a version bump.",
							BusinessProcess: "Exploration happens in notebooks; promising results are promoted to a tracked pipeline before any production use.",
						},
					}),
					extends: []store.ParentEdge{{ParentFrameID: idPyTorchGPU, ParentVersion: "1.0.0", OrderIndex: 0}}},
			},
		},
		{
			id: idStandalone, name: "standalone-frame", description: "Standalone data-cleaning frame with no parents, for exercising the non-inheriting case.",
			versions: []frameVersion{
				{version: "1.0.0", changelog: "Initial standalone frame",
					content: mustMarshalDoc(&frames.Doc{
						Name:        "standalone-frame",
						Description: "Standalone data-cleaning frame with no parents, for exercising the non-inheriting case.",
						Version:     "1.0.0",
						Slots: frames.Slots{
							Terminology: []frames.Term{
								{Term: "Tidy data", Definition: "A table where each variable is a column, each observation a row, and each cell a single value."},
							},
							Rules: []string{
								"Never mutate the raw input in place; write cleaned output to a new location.",
								"Record every transformation so the cleaning run is fully auditable.",
							},
							Skills: []string{
								"Profiling a dataset for missing values, outliers, and type inconsistencies.",
								"Writing idempotent, re-runnable data-cleaning transforms.",
							},
							Prompts: []string{
								"Given this dataframe schema, propose a set of validation checks to run before cleaning.",
							},
							Goals:           "Provide a self-contained frame for tabular data cleaning that depends on nothing else.",
							Style:           "Defensive and explicit: validate assumptions loudly and fail fast on malformed input.",
							BusinessProcess: "Raw data lands, is validated, is cleaned into a tidy table, and only then is handed to downstream analysis.",
						},
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
