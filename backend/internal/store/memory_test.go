package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMemory_OrgAndMembership(t *testing.T) {
	m := store.NewMemory()
	ctx := context.Background()

	// Seed an org used by the read scenarios below.
	if err := m.CreateOrg(ctx, &framesv1.Org{Id: "o1", Slug: "openteams", DisplayName: "OpenTeams"}); err != nil {
		t.Fatalf("seed CreateOrg: %v", err)
	}

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "duplicate slug returns ErrAlreadyExists",
			run: func(t *testing.T) {
				err := m.CreateOrg(ctx, &framesv1.Org{Id: "o2", Slug: "openteams"})
				if !errors.Is(err, store.ErrAlreadyExists) {
					t.Fatalf("duplicate slug: want ErrAlreadyExists, got %v", err)
				}
			},
		},
		{
			name: "org round-trip by slug",
			run: func(t *testing.T) {
				got, err := m.GetOrgBySlug(ctx, "openteams")
				if err != nil || got.Id != "o1" {
					t.Fatalf("GetOrgBySlug: got %v, %v", got, err)
				}
			},
		},
		{
			name: "missing membership returns ErrNotFound",
			run: func(t *testing.T) {
				if _, err := m.GetMembership(ctx, "nobody"); !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("missing membership: want ErrNotFound, got %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestMemoryMembershipReads(t *testing.T) {
	ctx := context.Background()
	m := store.NewMemory()
	_ = m.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "s1", Role: "admin", Email: "a@x.io"})
	_ = m.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "", Role: "viewer", Email: "p@x.io"})
	_ = m.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o2", UserSub: "s9", Role: "admin", Email: "z@x.io"})

	tests := []struct {
		name    string
		run     func() (int, error)
		wantLen int
	}{
		{"list org o1", func() (int, error) { l, e := m.ListMembershipsByOrg(ctx, "o1"); return len(l), e }, 2},
		{"count admins o1", func() (int, error) { return m.CountAdmins(ctx, "o1") }, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.run()
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.wantLen {
				t.Fatalf("got %d want %d", got, tt.wantLen)
			}
		})
	}

	pend, err := m.GetPendingMembershipByEmail(ctx, "p@x.io")
	if err != nil || pend.UserSub != "" || pend.OrgId != "o1" {
		t.Fatalf("pending lookup got %+v err %v", pend, err)
	}
	if _, err := m.GetPendingMembershipByEmail(ctx, "a@x.io"); err == nil {
		t.Fatal("active email should not match a pending lookup")
	}
}

func TestMemoryCountAdmins_PendingInviteNotCounted(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		seed      func(m *store.Memory)
		wantCount int
	}{
		{
			name: "active admin only - pending invite admin not counted",
			seed: func(m *store.Memory) {
				// 1 active admin (real user_sub)
				_ = m.UpsertMembership(ctx, &framesv1.Membership{OrgId: "o1", UserSub: "s1", Role: "admin", Email: "a@x.io"})
				// 1 pending admin invite (user_sub = "")
				_ = m.AddPendingMembership(ctx, &framesv1.Membership{OrgId: "o1", Role: "admin", Email: "pending-admin@x.io"})
			},
			wantCount: 1,
		},
		{
			name: "no active admins - only pending invite admin",
			seed: func(m *store.Memory) {
				_ = m.AddPendingMembership(ctx, &framesv1.Membership{OrgId: "o1", Role: "admin", Email: "pending@x.io"})
			},
			wantCount: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := store.NewMemory()
			tt.seed(m)
			got, err := m.CountAdmins(ctx, "o1")
			if err != nil {
				t.Fatalf("CountAdmins: %v", err)
			}
			if got != tt.wantCount {
				t.Fatalf("CountAdmins = %d, want %d", got, tt.wantCount)
			}
		})
	}
}

func TestMemoryMembershipWrites(t *testing.T) {
	ctx := context.Background()
	m := store.NewMemory()
	if err := m.AddPendingMembership(ctx, &framesv1.Membership{OrgId: "o1", Role: "viewer", Email: "p@x.io"}); err != nil {
		t.Fatal(err)
	}
	if err := m.AddPendingMembership(ctx, &framesv1.Membership{OrgId: "o1", Role: "viewer", Email: "p@x.io"}); err != store.ErrAlreadyExists {
		t.Fatalf("want ErrAlreadyExists, got %v", err)
	}
	if err := m.ActivatePendingMembership(ctx, "p@x.io", "sub-1"); err != nil {
		t.Fatal(err)
	}
	got, err := m.GetMembership(ctx, "sub-1")
	if err != nil || got.OrgId != "o1" || got.Email != "p@x.io" {
		t.Fatalf("activate result %+v err %v", got, err)
	}
	if err := m.UpdateMembershipRole(ctx, "o1", "sub-1", "", "admin"); err != nil {
		t.Fatal(err)
	}
	if g, _ := m.GetMembership(ctx, "sub-1"); g.Role != "admin" {
		t.Fatalf("role not updated: %+v", g)
	}
	if err := m.DeleteMembership(ctx, "o1", "sub-1", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetMembership(ctx, "sub-1"); err != store.ErrNotFound {
		t.Fatalf("want ErrNotFound after delete, got %v", err)
	}
}

func TestMemoryDeleteFrameDetachesChildren(t *testing.T) {
	ctx := context.Background()
	m := store.NewMemory()
	_ = m.CreateOrg(ctx, &framesv1.Org{Id: "o1", Slug: "openteams", DisplayName: "OT", CreatedAt: timestamppb.Now()})
	parent := &framesv1.Frame{Id: "p", OrgId: "o1", Name: "parent", LatestVersion: "1.0.0", CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()}
	child := &framesv1.Frame{Id: "c", OrgId: "o1", Name: "child", LatestVersion: "1.0.0", CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()}
	_ = m.CreateFrameVersion(ctx, store.CreateFrameVersionInput{Frame: parent, Version: &framesv1.FrameVersion{Version: "1.0.0", PublishedAt: timestamppb.Now()}, IsNewFrame: true})
	_ = m.CreateFrameVersion(ctx, store.CreateFrameVersionInput{Frame: child, Version: &framesv1.FrameVersion{Version: "1.0.0", PublishedAt: timestamppb.Now()}, Extends: []store.ParentEdge{{ParentFrameID: "p", ParentVersion: "1.0.0", OrderIndex: 0}}, IsNewFrame: true})

	kids, err := m.FrameChildren(ctx, "p")
	if err != nil || len(kids) != 1 || kids[0].Id != "c" {
		t.Fatalf("children %+v err %v", kids, err)
	}
	if err := m.DeleteFrame(ctx, "p"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetFrameByID(ctx, "p"); err != store.ErrNotFound {
		t.Fatalf("parent should be gone, got %v", err)
	}
	if _, err := m.GetFrameByID(ctx, "c"); err != nil {
		t.Fatalf("child should survive, got %v", err)
	}
	if kids, _ := m.FrameChildren(ctx, "p"); len(kids) != 0 {
		t.Fatalf("edges should be detached, got %+v", kids)
	}
}

func TestMemory_CreateFrameVersionAndGrants(t *testing.T) {
	m := store.NewMemory()
	ctx := context.Background()
	if err := m.CreateOrg(ctx, &framesv1.Org{Id: "o1", Slug: "openteams"}); err != nil {
		t.Fatalf("seed CreateOrg: %v", err)
	}

	in := store.CreateFrameVersionInput{
		Frame:      &framesv1.Frame{Id: "f1", OrgId: "o1", Name: "brand-voice", OwnerSub: "u1", LatestVersion: "1.0.0"},
		Version:    &framesv1.FrameVersion{Version: "1.0.0", Content: []byte("x"), Digest: "d", PublishedBy: "u1"},
		Grants:     []store.Grant{{SubjectType: "user", SubjectID: "u1", Permission: "edit"}, {SubjectType: "org", SubjectID: "o1", Permission: "read"}},
		IsNewFrame: true,
	}

	// These scenarios share mutable state and assert a sequence
	// (create -> grants present -> duplicate rejected), so run in order.
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "create new frame version succeeds",
			run: func(t *testing.T) {
				if err := m.CreateFrameVersion(ctx, in); err != nil {
					t.Fatalf("CreateFrameVersion: %v", err)
				}
			},
		},
		{
			name: "default grants are stored on new frame",
			run: func(t *testing.T) {
				grants, _ := m.FrameGrants(ctx, "f1")
				if len(grants) != 2 {
					t.Fatalf("want 2 grants, got %d", len(grants))
				}
			},
		},
		{
			name: "duplicate version returns ErrAlreadyExists",
			run: func(t *testing.T) {
				if err := m.CreateFrameVersion(ctx, in); !errors.Is(err, store.ErrAlreadyExists) {
					t.Fatalf("duplicate version: want ErrAlreadyExists, got %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestMemoryFrameTemplateCRUD(t *testing.T) {
	ctx := context.Background()
	m := store.NewMemory()
	row := &store.FrameTemplate{
		ID: "t1", OrgID: "org-a", Title: "Brand Voice", Description: "How we sound",
		Prefill: []byte("slots: {}\n"), FieldRules: []byte(`{"style":{"level":"required"}}`),
		CreatedBy: "user-1", CreatedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC(),
	}
	if err := m.CreateFrameTemplate(ctx, row); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := m.GetFrameTemplate(ctx, "org-a", "t1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "Brand Voice" || string(got.FieldRules) != `{"style":{"level":"required"}}` {
		t.Errorf("round trip lost data: %+v", got)
	}

	// A caller in another org must not see it, and must not learn it exists.
	if _, err := m.GetFrameTemplate(ctx, "org-b", "t1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("cross-org get error = %v, want ErrNotFound", err)
	}
	rows, err := m.ListFrameTemplatesByOrg(ctx, "org-b")
	if err != nil {
		t.Fatalf("list org-b: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("org-b sees %d templates, want 0", len(rows))
	}

	rows, err = m.ListFrameTemplatesByOrg(ctx, "org-a")
	if err != nil {
		t.Fatalf("list org-a: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "t1" {
		t.Fatalf("org-a list = %+v, want one row t1", rows)
	}

	row.Title = "Brand Voice v2"
	row.UpdatedAt = time.Unix(60, 0).UTC()
	if err := m.UpdateFrameTemplate(ctx, row); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err = m.GetFrameTemplate(ctx, "org-a", "t1")
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Title != "Brand Voice v2" {
		t.Errorf("title = %q, want the updated one", got.Title)
	}

	if err := m.DeleteFrameTemplate(ctx, "org-a", "t1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := m.GetFrameTemplate(ctx, "org-a", "t1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("get after delete = %v, want ErrNotFound", err)
	}
}

func TestMemoryFrameTemplateConstraints(t *testing.T) {
	ctx := context.Background()
	base := func(id, orgID, title string) *store.FrameTemplate {
		return &store.FrameTemplate{ID: id, OrgID: orgID, Title: title, Prefill: []byte("slots: {}\n"), FieldRules: []byte("{}")}
	}
	tests := []struct {
		name    string
		seed    []*store.FrameTemplate
		op      func(*store.Memory) error
		wantErr error
	}{
		{
			name: "duplicate title in the same org is rejected",
			seed: []*store.FrameTemplate{base("t1", "org-a", "Brand Voice")},
			op: func(m *store.Memory) error {
				return m.CreateFrameTemplate(context.Background(), base("t2", "org-a", "Brand Voice"))
			},
			wantErr: store.ErrAlreadyExists,
		},
		{
			name: "the same title in a different org is fine",
			seed: []*store.FrameTemplate{base("t1", "org-a", "Brand Voice")},
			op: func(m *store.Memory) error {
				return m.CreateFrameTemplate(context.Background(), base("t2", "org-b", "Brand Voice"))
			},
		},
		{
			name: "duplicate id is rejected",
			seed: []*store.FrameTemplate{base("t1", "org-a", "Brand Voice")},
			op: func(m *store.Memory) error {
				return m.CreateFrameTemplate(context.Background(), base("t1", "org-a", "Other"))
			},
			wantErr: store.ErrAlreadyExists,
		},
		{
			name: "renaming onto another row's title is rejected",
			seed: []*store.FrameTemplate{base("t1", "org-a", "Brand Voice"), base("t2", "org-a", "Engineering Norms")},
			op: func(m *store.Memory) error {
				return m.UpdateFrameTemplate(context.Background(), base("t2", "org-a", "Brand Voice"))
			},
			wantErr: store.ErrAlreadyExists,
		},
		{
			name: "renaming a row to its own title is fine",
			seed: []*store.FrameTemplate{base("t1", "org-a", "Brand Voice")},
			op: func(m *store.Memory) error {
				return m.UpdateFrameTemplate(context.Background(), base("t1", "org-a", "Brand Voice"))
			},
		},
		{
			name: "updating a missing row is not found",
			op: func(m *store.Memory) error {
				return m.UpdateFrameTemplate(context.Background(), base("nope", "org-a", "X"))
			},
			wantErr: store.ErrNotFound,
		},
		{
			name: "updating another org's row is not found",
			seed: []*store.FrameTemplate{base("t1", "org-a", "Brand Voice")},
			op: func(m *store.Memory) error {
				return m.UpdateFrameTemplate(context.Background(), base("t1", "org-b", "Brand Voice"))
			},
			wantErr: store.ErrNotFound,
		},
		{
			name:    "deleting a missing row is not found",
			op:      func(m *store.Memory) error { return m.DeleteFrameTemplate(context.Background(), "org-a", "nope") },
			wantErr: store.ErrNotFound,
		},
		{
			name:    "deleting another org's row is not found",
			seed:    []*store.FrameTemplate{base("t1", "org-a", "Brand Voice")},
			op:      func(m *store.Memory) error { return m.DeleteFrameTemplate(context.Background(), "org-b", "t1") },
			wantErr: store.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := store.NewMemory()
			for _, row := range tt.seed {
				if err := m.CreateFrameTemplate(ctx, row); err != nil {
					t.Fatalf("seeding %s: %v", row.ID, err)
				}
			}
			err := tt.op(m)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryFrameTemplateReturnsCopies(t *testing.T) {
	// A handler applying an update mutates the struct it was handed, so the
	// store must not be sharing its own row.
	ctx := context.Background()
	m := store.NewMemory()
	if err := m.CreateFrameTemplate(ctx, &store.FrameTemplate{
		ID: "t1", OrgID: "org-a", Title: "Brand Voice",
		Prefill: []byte("slots: {}\n"), FieldRules: []byte("{}"),
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := m.GetFrameTemplate(ctx, "org-a", "t1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	got.Title = "clobbered"
	again, err := m.GetFrameTemplate(ctx, "org-a", "t1")
	if err != nil {
		t.Fatalf("get again: %v", err)
	}
	if again.Title != "Brand Voice" {
		t.Errorf("mutating the returned row changed the store: %q", again.Title)
	}
}
