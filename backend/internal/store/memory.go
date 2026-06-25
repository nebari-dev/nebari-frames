package store

import (
	"context"
	"strings"
	"sync"

	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

// Memory is an in-memory Repository for development and tests.
type Memory struct {
	mu          sync.RWMutex
	orgs        map[string]*framesv1.Org    // id -> org
	slugToOrg   map[string]string           // slug -> id
	memberships []*framesv1.Membership      // active + pending; active rows have non-empty UserSub
	frames      map[string]*framesv1.Frame  // id -> frame
	keyToFrame  map[string]string           // orgID+"/"+name -> id
	versions    map[string]*frameVersionRow // frameID+"@"+version
	grants      map[string][]Grant          // frameID -> grants
}

type frameVersionRow struct {
	v        *framesv1.FrameVersion
	extends  []ParentEdge
	excludes []string
}

var _ Repository = (*Memory)(nil)

func NewMemory() *Memory {
	return &Memory{
		orgs:       map[string]*framesv1.Org{},
		slugToOrg:  map[string]string{},
		frames:     map[string]*framesv1.Frame{},
		keyToFrame: map[string]string{},
		versions:   map[string]*frameVersionRow{},
		grants:     map[string][]Grant{},
	}
}

func (m *Memory) CreateOrg(_ context.Context, org *framesv1.Org) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.slugToOrg[org.Slug]; ok {
		return ErrAlreadyExists
	}
	m.orgs[org.Id] = org
	m.slugToOrg[org.Slug] = org.Id
	return nil
}

func (m *Memory) GetOrgByID(_ context.Context, id string) (*framesv1.Org, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if o, ok := m.orgs[id]; ok {
		return o, nil
	}
	return nil, ErrNotFound
}

func (m *Memory) GetOrgBySlug(_ context.Context, slug string) (*framesv1.Org, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if id, ok := m.slugToOrg[slug]; ok {
		return m.orgs[id], nil
	}
	return nil, ErrNotFound
}

func (m *Memory) GetMembership(_ context.Context, userSub string) (*framesv1.Membership, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, mem := range m.memberships {
		if mem.UserSub == userSub && userSub != "" {
			return mem, nil
		}
	}
	return nil, ErrNotFound
}

func (m *Memory) UpsertMembership(_ context.Context, mem *framesv1.Membership) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, existing := range m.memberships {
		if existing.UserSub == mem.UserSub && mem.UserSub != "" {
			m.memberships[i] = mem
			return nil
		}
	}
	m.memberships = append(m.memberships, mem)
	return nil
}

func (m *Memory) ListMembershipsByOrg(_ context.Context, orgID string) ([]*framesv1.Membership, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []*framesv1.Membership{}
	for _, mem := range m.memberships {
		if mem.OrgId == orgID {
			out = append(out, mem)
		}
	}
	return out, nil
}

func (m *Memory) GetPendingMembershipByEmail(_ context.Context, email string) (*framesv1.Membership, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, mem := range m.memberships {
		if mem.UserSub == "" && mem.Email == email {
			return mem, nil
		}
	}
	return nil, ErrNotFound
}

func (m *Memory) CountAdmins(_ context.Context, orgID string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := 0
	for _, mem := range m.memberships {
		if mem.OrgId == orgID && mem.Role == "admin" && mem.UserSub != "" {
			n++
		}
	}
	return n, nil
}

func (m *Memory) CreateFrameVersion(_ context.Context, in CreateFrameVersionInput) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := in.Frame.OrgId + "/" + in.Frame.Name
	if in.IsNewFrame {
		if _, ok := m.keyToFrame[key]; ok {
			return ErrAlreadyExists
		}
		if _, ok := m.frames[in.Frame.Id]; ok {
			return ErrAlreadyExists
		}
		m.frames[in.Frame.Id] = in.Frame
		m.keyToFrame[key] = in.Frame.Id
		m.grants[in.Frame.Id] = append(m.grants[in.Frame.Id], in.Grants...)
	} else {
		m.frames[in.Frame.Id] = in.Frame // updated latest_version/updated_at
	}
	vkey := in.Frame.Id + "@" + in.Version.Version
	if _, ok := m.versions[vkey]; ok {
		return ErrAlreadyExists
	}
	m.versions[vkey] = &frameVersionRow{v: in.Version, extends: in.Extends, excludes: in.Excludes}
	return nil
}

func (m *Memory) GetFrameBySlugName(_ context.Context, orgSlug, name string) (*framesv1.Frame, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	orgID, ok := m.slugToOrg[orgSlug]
	if !ok {
		return nil, ErrNotFound
	}
	id, ok := m.keyToFrame[orgID+"/"+name]
	if !ok {
		return nil, ErrNotFound
	}
	return m.frames[id], nil
}

func (m *Memory) GetFrameByID(_ context.Context, id string) (*framesv1.Frame, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if f, ok := m.frames[id]; ok {
		return f, nil
	}
	return nil, ErrNotFound
}

func (m *Memory) GetFrameVersion(_ context.Context, frameID, version string) (*framesv1.FrameVersion, []ParentEdge, []string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	row, ok := m.versions[frameID+"@"+version]
	if !ok {
		return nil, nil, nil, ErrNotFound
	}
	return row.v, row.extends, row.excludes, nil
}

func (m *Memory) ListFrameVersions(_ context.Context, frameID string) ([]*framesv1.FrameVersionSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []*framesv1.FrameVersionSummary
	for key, row := range m.versions {
		// key format: frameID+"@"+version
		if len(key) <= len(frameID)+1 || key[:len(frameID)+1] != frameID+"@" {
			continue
		}
		out = append(out, &framesv1.FrameVersionSummary{
			Version:     row.v.Version,
			Changelog:   row.v.Changelog,
			PublishedBy: row.v.PublishedBy,
			PublishedAt: row.v.PublishedAt,
		})
	}
	// Sort newest first by published_at then version descending.
	for i := 0; i < len(out)-1; i++ {
		for j := i + 1; j < len(out); j++ {
			ti := out[i].PublishedAt.AsTime()
			tj := out[j].PublishedAt.AsTime()
			if tj.After(ti) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

func (m *Memory) ListFramesByOrg(_ context.Context, orgID string) ([]*framesv1.Frame, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []*framesv1.Frame{}
	for _, f := range m.frames {
		if f.OrgId == orgID {
			out = append(out, f)
		}
	}
	return out, nil
}

func (m *Memory) FrameGrants(_ context.Context, frameID string) ([]Grant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if g, ok := m.grants[frameID]; ok {
		return g, nil
	}
	return []Grant{}, nil
}

func (m *Memory) AddPendingMembership(_ context.Context, mem *framesv1.Membership) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.memberships {
		if e.OrgId == mem.OrgId && e.Email == mem.Email {
			return ErrAlreadyExists
		}
	}
	m.memberships = append(m.memberships, &framesv1.Membership{
		OrgId:   mem.OrgId,
		Role:    mem.Role,
		Email:   mem.Email,
		AddedAt: mem.AddedAt,
	})
	return nil
}

func (m *Memory) ActivatePendingMembership(_ context.Context, email, sub string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.memberships {
		if e.UserSub == "" && e.Email == email {
			e.UserSub = sub
			return nil
		}
	}
	return ErrNotFound
}

func (m *Memory) UpdateMembershipRole(_ context.Context, orgID, userSub, email, role string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.memberships {
		if e.OrgId != orgID {
			continue
		}
		if (userSub != "" && e.UserSub == userSub) || (userSub == "" && email != "" && e.Email == email) {
			e.Role = role
			return nil
		}
	}
	return ErrNotFound
}

func (m *Memory) DeleteMembership(_ context.Context, orgID, userSub, email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, e := range m.memberships {
		if e.OrgId != orgID {
			continue
		}
		if (userSub != "" && e.UserSub == userSub) || (userSub == "" && email != "" && e.Email == email) {
			m.memberships = append(m.memberships[:i], m.memberships[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

func (m *Memory) FrameChildren(_ context.Context, parentFrameID string) ([]*framesv1.Frame, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	seen := map[string]bool{}
	out := []*framesv1.Frame{}
	for key, row := range m.versions {
		for _, e := range row.extends {
			if e.ParentFrameID != parentFrameID {
				continue
			}
			childID := key[:strings.Index(key, "@")]
			if childID == parentFrameID || seen[childID] {
				continue
			}
			if f, ok := m.frames[childID]; ok {
				seen[childID] = true
				out = append(out, f)
			}
		}
	}
	return out, nil
}

func (m *Memory) DeleteFrame(_ context.Context, frameID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.frames[frameID]
	if !ok {
		return ErrNotFound
	}
	delete(m.frames, frameID)
	delete(m.keyToFrame, f.OrgId+"/"+f.Name)
	delete(m.grants, frameID)
	for key := range m.versions {
		if strings.HasPrefix(key, frameID+"@") {
			delete(m.versions, key)
		}
	}
	// detach incoming edges (children that extend this frame keep their rows minus the edge)
	for _, row := range m.versions {
		kept := row.extends[:0]
		for _, e := range row.extends {
			if e.ParentFrameID != frameID {
				kept = append(kept, e)
			}
		}
		row.extends = kept
	}
	return nil
}
