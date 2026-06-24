package store

import (
	"context"
	"sync"

	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

// Memory is an in-memory Repository for development and tests.
type Memory struct {
	mu          sync.RWMutex
	orgs        map[string]*framesv1.Org        // id -> org
	slugToOrg   map[string]string               // slug -> id
	memberships map[string]*framesv1.Membership // user_sub -> membership
	frames      map[string]*framesv1.Frame      // id -> frame
	keyToFrame  map[string]string               // orgID+"/"+name -> id
	versions    map[string]*frameVersionRow     // frameID+"@"+version
	grants      map[string][]Grant              // frameID -> grants
}

type frameVersionRow struct {
	v        *framesv1.FrameVersion
	extends  []ParentEdge
	excludes []string
}

var _ Repository = (*Memory)(nil)

func NewMemory() *Memory {
	return &Memory{
		orgs:        map[string]*framesv1.Org{},
		slugToOrg:   map[string]string{},
		memberships: map[string]*framesv1.Membership{},
		frames:      map[string]*framesv1.Frame{},
		keyToFrame:  map[string]string{},
		versions:    map[string]*frameVersionRow{},
		grants:      map[string][]Grant{},
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
	if mem, ok := m.memberships[userSub]; ok {
		return mem, nil
	}
	return nil, ErrNotFound
}

func (m *Memory) UpsertMembership(_ context.Context, mem *framesv1.Membership) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memberships[mem.UserSub] = mem
	return nil
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
