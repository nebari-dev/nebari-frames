package rbac_test

import (
	"context"
	"testing"

	"github.com/nebari-dev/nebari-frames/backend/internal/rbac"
)

type fakeLookup map[string][]rbac.Grant

func (f fakeLookup) FrameGrants(_ context.Context, id string) ([]rbac.Grant, error) { return f[id], nil }

func TestCan(t *testing.T) {
	orgGrants := fakeLookup{"f1": {
		{SubjectType: "user", SubjectID: "owner", Permission: rbac.PermEdit},
		{SubjectType: "org", SubjectID: "o1", Permission: rbac.PermRead},
	}}
	tests := []struct {
		name      string
		caller    rbac.Caller
		frameOrg  string
		perm      rbac.Permission
		wantAllow bool
	}{
		{"admin overrides everything", rbac.Caller{Subject: "a", OrgID: "o1", Role: rbac.RoleAdmin}, "o1", rbac.PermDelete, true},
		{"cross-org denied even for admin", rbac.Caller{Subject: "a", OrgID: "o2", Role: rbac.RoleAdmin}, "o1", rbac.PermRead, false},
		{"org read grant lets viewer read", rbac.Caller{Subject: "v", OrgID: "o1", Role: rbac.RoleViewer}, "o1", rbac.PermRead, true},
		{"viewer cannot edit", rbac.Caller{Subject: "v", OrgID: "o1", Role: rbac.RoleViewer}, "o1", rbac.PermEdit, false},
		{"owner user grant allows edit", rbac.Caller{Subject: "owner", OrgID: "o1", Role: rbac.RolePublisher}, "o1", rbac.PermEdit, true},
		{"non-owner publisher cannot edit", rbac.Caller{Subject: "pub", OrgID: "o1", Role: rbac.RolePublisher}, "o1", rbac.PermEdit, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rbac.Can(context.Background(), orgGrants, tt.caller, tt.frameOrg, "f1", tt.perm)
			if err != nil {
				t.Fatalf("Can: %v", err)
			}
			if got != tt.wantAllow {
				t.Fatalf("Can = %v, want %v", got, tt.wantAllow)
			}
		})
	}
}

func TestCanPublish(t *testing.T) {
	if rbac.CanPublish(rbac.Caller{Role: rbac.RoleViewer}) {
		t.Fatal("viewer must not publish")
	}
	if !rbac.CanPublish(rbac.Caller{Role: rbac.RolePublisher}) {
		t.Fatal("publisher must publish")
	}
	if !rbac.CanPublish(rbac.Caller{Role: rbac.RoleAdmin}) {
		t.Fatal("admin must publish")
	}
}
