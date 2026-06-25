// Package sqlite provides a SQLite-backed implementation of store.Repository.
// Callers must register the driver: import _ "modernc.org/sqlite".
package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
	"github.com/nebari-dev/nebari-frames/backend/internal/store"
	sqlitedrv "modernc.org/sqlite"
	lib "modernc.org/sqlite/lib"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Open opens a SQLite database with WAL mode and recommended pragmas.
// SQLite is single-writer, so MaxOpenConns is pinned to 1.
func Open(path string) (*sql.DB, error) {
	dsn := path + "?" + strings.Join([]string{
		"_pragma=journal_mode=WAL",
		"_pragma=busy_timeout=5000",
		"_pragma=foreign_keys=ON",
		"_pragma=synchronous=NORMAL",
	}, "&")
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite %s: %w", path, err)
	}
	return db, nil
}

// Repository is a SQLite-backed implementation of store.Repository.
type Repository struct{ db *sql.DB }

var _ store.Repository = (*Repository)(nil)

// New creates a new Repository backed by the given database.
func New(db *sql.DB) *Repository { return &Repository{db: db} }

// ts parses an RFC3339 string into a timestamppb.Timestamp.
func ts(s string) *timestamppb.Timestamp {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return timestamppb.New(t)
}

// isUnique returns true if err is a SQLite UNIQUE or PRIMARY KEY constraint violation.
// Uses typed error codes from modernc.org/sqlite/lib.
func isUnique(err error) bool {
	if err == nil {
		return false
	}
	var se *sqlitedrv.Error
	if errors.As(err, &se) {
		c := se.Code()
		return c == lib.SQLITE_CONSTRAINT_UNIQUE || c == lib.SQLITE_CONSTRAINT_PRIMARYKEY
	}
	// Fallback in case typed error is not available
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// newGrantID generates a new ULID string for grant IDs.
func newGrantID() (string, error) {
	id, err := ulid.New(ulid.Now(), rand.Reader)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func (r *Repository) CreateOrg(ctx context.Context, o *framesv1.Org) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO orgs (id, slug, display_name, created_at) VALUES (?, ?, ?, ?)`,
		o.Id, o.Slug, o.DisplayName, o.CreatedAt.AsTime().UTC().Format(time.RFC3339))
	if isUnique(err) {
		return store.ErrAlreadyExists
	}
	return err
}

func (r *Repository) GetOrgByID(ctx context.Context, id string) (*framesv1.Org, error) {
	return r.scanOrg(r.db.QueryRowContext(ctx,
		`SELECT id, slug, display_name, created_at FROM orgs WHERE id = ?`, id))
}

func (r *Repository) GetOrgBySlug(ctx context.Context, slug string) (*framesv1.Org, error) {
	return r.scanOrg(r.db.QueryRowContext(ctx,
		`SELECT id, slug, display_name, created_at FROM orgs WHERE slug = ?`, slug))
}

func (r *Repository) scanOrg(row *sql.Row) (*framesv1.Org, error) {
	var o framesv1.Org
	var created string
	if err := row.Scan(&o.Id, &o.Slug, &o.DisplayName, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	o.CreatedAt = ts(created)
	return &o, nil
}

func (r *Repository) GetMembership(ctx context.Context, userSub string) (*framesv1.Membership, error) {
	var m framesv1.Membership
	var added string
	var email sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT org_id, user_sub, role, added_at, email FROM org_memberships WHERE user_sub = ? AND user_sub <> ''`, userSub).
		Scan(&m.OrgId, &m.UserSub, &m.Role, &added, &email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	m.AddedAt = ts(added)
	m.Email = email.String
	return &m, nil
}

func (r *Repository) UpsertMembership(ctx context.Context, m *framesv1.Membership) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE org_memberships SET org_id=?, role=?, email=? WHERE user_sub=? AND user_sub <> ''`,
		m.OrgId, m.Role, nullStr(m.Email), m.UserSub)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO org_memberships (org_id, user_sub, role, added_at, email) VALUES (?, ?, ?, ?, ?)`,
		m.OrgId, m.UserSub, m.Role, m.AddedAt.AsTime().UTC().Format(time.RFC3339), nullStr(m.Email))
	if isUnique(err) {
		return store.ErrAlreadyExists
	}
	return err
}

// nullStr maps "" to SQL NULL so the partial unique email index ignores it.
func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (r *Repository) scanMemberships(rows *sql.Rows) ([]*framesv1.Membership, error) {
	defer func() { _ = rows.Close() }()
	out := []*framesv1.Membership{}
	for rows.Next() {
		var m framesv1.Membership
		var added string
		var email sql.NullString
		if err := rows.Scan(&m.OrgId, &m.UserSub, &m.Role, &added, &email); err != nil {
			return nil, err
		}
		m.AddedAt = ts(added)
		m.Email = email.String
		out = append(out, &m)
	}
	return out, rows.Err()
}

func (r *Repository) ListMembershipsByOrg(ctx context.Context, orgID string) ([]*framesv1.Membership, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT org_id, user_sub, role, added_at, email FROM org_memberships WHERE org_id = ? ORDER BY added_at`, orgID)
	if err != nil {
		return nil, err
	}
	return r.scanMemberships(rows)
}

func (r *Repository) GetPendingMembershipByEmail(ctx context.Context, email string) (*framesv1.Membership, error) {
	var m framesv1.Membership
	var added string
	var e sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT org_id, user_sub, role, added_at, email FROM org_memberships WHERE email = ? AND user_sub = '' LIMIT 1`, email).
		Scan(&m.OrgId, &m.UserSub, &m.Role, &added, &e)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	m.AddedAt = ts(added)
	m.Email = e.String
	return &m, nil
}

func (r *Repository) CountAdmins(ctx context.Context, orgID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM org_memberships WHERE org_id = ? AND role = 'admin' AND user_sub <> ''`, orgID).Scan(&n)
	return n, err
}

func (r *Repository) CreateFrameVersion(ctx context.Context, in store.CreateFrameVersionInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	f := in.Frame
	now := f.UpdatedAt.AsTime().UTC().Format(time.RFC3339)
	if in.IsNewFrame {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO frames (id, org_id, name, description, owner_sub, latest_version, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			f.Id, f.OrgId, f.Name, f.Description, f.OwnerSub, f.LatestVersion,
			f.CreatedAt.AsTime().UTC().Format(time.RFC3339), now); err != nil {
			if isUnique(err) {
				return store.ErrAlreadyExists
			}
			return err
		}
	} else {
		if _, err := tx.ExecContext(ctx,
			`UPDATE frames SET description=?, latest_version=?, updated_at=? WHERE id=?`,
			f.Description, f.LatestVersion, now, f.Id); err != nil {
			return err
		}
	}

	v := in.Version
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO frame_versions (frame_id, version, content, digest, size_bytes, published_by, published_at, changelog)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		f.Id, v.Version, v.Content, v.Digest, v.SizeBytes, v.PublishedBy,
		v.PublishedAt.AsTime().UTC().Format(time.RFC3339), v.Changelog); err != nil {
		if isUnique(err) {
			return store.ErrAlreadyExists
		}
		return err
	}

	for _, e := range in.Extends {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO frame_extends (frame_id, version, parent_frame_id, parent_version, order_index)
			 VALUES (?, ?, ?, ?, ?)`,
			f.Id, v.Version, e.ParentFrameID, e.ParentVersion, e.OrderIndex); err != nil {
			return err
		}
	}
	for _, ex := range in.Excludes {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO frame_excludes (frame_id, version, excluded_frame_id) VALUES (?, ?, ?)`,
			f.Id, v.Version, ex); err != nil {
			return err
		}
	}
	if in.IsNewFrame {
		for _, g := range in.Grants {
			id, err := newGrantID()
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO frame_grants (id, frame_id, section_id, subject_type, subject_id, permission, granted_by, granted_at)
				 VALUES (?, ?, NULL, ?, ?, ?, ?, ?)`,
				id, f.Id, g.SubjectType, g.SubjectID, g.Permission, f.OwnerSub, now); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (r *Repository) GetFrameBySlugName(ctx context.Context, orgSlug, name string) (*framesv1.Frame, error) {
	return r.scanFrame(r.db.QueryRowContext(ctx,
		`SELECT f.id, f.org_id, f.name, f.description, f.owner_sub, f.latest_version, f.created_at, f.updated_at
		   FROM frames f JOIN orgs o ON o.id = f.org_id
		  WHERE o.slug = ? AND f.name = ?`, orgSlug, name))
}

func (r *Repository) GetFrameByID(ctx context.Context, id string) (*framesv1.Frame, error) {
	return r.scanFrame(r.db.QueryRowContext(ctx,
		`SELECT id, org_id, name, description, owner_sub, latest_version, created_at, updated_at
		   FROM frames WHERE id = ?`, id))
}

func (r *Repository) scanFrame(row *sql.Row) (*framesv1.Frame, error) {
	var f framesv1.Frame
	var created, updated string
	if err := row.Scan(&f.Id, &f.OrgId, &f.Name, &f.Description, &f.OwnerSub, &f.LatestVersion, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	f.CreatedAt, f.UpdatedAt = ts(created), ts(updated)
	return &f, nil
}

func (r *Repository) GetFrameVersion(ctx context.Context, frameID, version string) (*framesv1.FrameVersion, []store.ParentEdge, []string, error) {
	var v framesv1.FrameVersion
	var published string
	err := r.db.QueryRowContext(ctx,
		`SELECT version, content, digest, size_bytes, published_by, published_at, changelog
		   FROM frame_versions WHERE frame_id = ? AND version = ?`, frameID, version).
		Scan(&v.Version, &v.Content, &v.Digest, &v.SizeBytes, &v.PublishedBy, &published, &v.Changelog)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil, store.ErrNotFound
	}
	if err != nil {
		return nil, nil, nil, err
	}
	v.PublishedAt = ts(published)

	// The version row and its edges/excludes are read in sequential queries.
	// This is safe because Open pins MaxOpenConns=1 (single-writer), so no
	// concurrent writer can interleave between these reads.
	rows, err := r.db.QueryContext(ctx,
		`SELECT parent_frame_id, parent_version, order_index FROM frame_extends
		  WHERE frame_id = ? AND version = ? ORDER BY order_index`, frameID, version)
	if err != nil {
		return nil, nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	var edges []store.ParentEdge
	for rows.Next() {
		var e store.ParentEdge
		if err := rows.Scan(&e.ParentFrameID, &e.ParentVersion, &e.OrderIndex); err != nil {
			return nil, nil, nil, err
		}
		edges = append(edges, e)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, err
	}

	exRows, err := r.db.QueryContext(ctx,
		`SELECT excluded_frame_id FROM frame_excludes WHERE frame_id = ? AND version = ?`, frameID, version)
	if err != nil {
		return nil, nil, nil, err
	}
	defer func() { _ = exRows.Close() }()
	var excludes []string
	for exRows.Next() {
		var id string
		if err := exRows.Scan(&id); err != nil {
			return nil, nil, nil, err
		}
		excludes = append(excludes, id)
	}
	if err := exRows.Err(); err != nil {
		return nil, nil, nil, err
	}
	return &v, edges, excludes, nil
}

// ListFrameVersions returns version metadata for a frame, newest first.
// Content is intentionally omitted to keep the list lightweight.
func (r *Repository) ListFrameVersions(ctx context.Context, frameID string) ([]*framesv1.FrameVersionSummary, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT version, changelog, published_by, published_at
		   FROM frame_versions WHERE frame_id = ? ORDER BY published_at DESC, version DESC`, frameID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*framesv1.FrameVersionSummary
	for rows.Next() {
		var v framesv1.FrameVersionSummary
		var published string
		if err := rows.Scan(&v.Version, &v.Changelog, &v.PublishedBy, &published); err != nil {
			return nil, err
		}
		v.PublishedAt = ts(published)
		out = append(out, &v)
	}
	return out, rows.Err()
}

func (r *Repository) ListFramesByOrg(ctx context.Context, orgID string) ([]*framesv1.Frame, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, org_id, name, description, owner_sub, latest_version, created_at, updated_at
		   FROM frames WHERE org_id = ? ORDER BY updated_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []*framesv1.Frame{}
	for rows.Next() {
		var f framesv1.Frame
		var created, updated string
		if err := rows.Scan(&f.Id, &f.OrgId, &f.Name, &f.Description, &f.OwnerSub, &f.LatestVersion, &created, &updated); err != nil {
			return nil, err
		}
		f.CreatedAt, f.UpdatedAt = ts(created), ts(updated)
		out = append(out, &f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) FrameGrants(ctx context.Context, frameID string) ([]store.Grant, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT subject_type, subject_id, permission FROM frame_grants WHERE frame_id = ? AND section_id IS NULL`, frameID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []store.Grant{}
	for rows.Next() {
		var g store.Grant
		if err := rows.Scan(&g.SubjectType, &g.SubjectID, &g.Permission); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) AddPendingMembership(ctx context.Context, m *framesv1.Membership) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO org_memberships (org_id, user_sub, role, added_at, email) VALUES (?, '', ?, ?, ?)`,
		m.OrgId, m.Role, m.AddedAt.AsTime().UTC().Format(time.RFC3339), nullStr(m.Email))
	if isUnique(err) {
		return store.ErrAlreadyExists
	}
	return err
}

func (r *Repository) ActivatePendingMembership(ctx context.Context, email, sub string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE org_memberships SET user_sub = ? WHERE email = ? AND user_sub = ''`, sub, email)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (r *Repository) UpdateMembershipRole(ctx context.Context, orgID, userSub, email, role string) error {
	var res sql.Result
	var err error
	if userSub != "" {
		res, err = r.db.ExecContext(ctx,
			`UPDATE org_memberships SET role = ? WHERE org_id = ? AND user_sub = ?`, role, orgID, userSub)
	} else {
		res, err = r.db.ExecContext(ctx,
			`UPDATE org_memberships SET role = ? WHERE org_id = ? AND email = ? AND user_sub = ''`, role, orgID, email)
	}
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteMembership(ctx context.Context, orgID, userSub, email string) error {
	var res sql.Result
	var err error
	if userSub != "" {
		res, err = r.db.ExecContext(ctx,
			`DELETE FROM org_memberships WHERE org_id = ? AND user_sub = ?`, orgID, userSub)
	} else {
		res, err = r.db.ExecContext(ctx,
			`DELETE FROM org_memberships WHERE org_id = ? AND email = ? AND user_sub = ''`, orgID, email)
	}
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrNotFound
	}
	return nil
}

func (r *Repository) FrameChildren(ctx context.Context, parentFrameID string) ([]*framesv1.Frame, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT f.id, f.org_id, f.name, f.description, f.owner_sub, f.latest_version, f.created_at, f.updated_at
		   FROM frames f
		   JOIN frame_extends fe ON fe.frame_id = f.id
		  WHERE fe.parent_frame_id = ? AND f.id <> ?`, parentFrameID, parentFrameID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []*framesv1.Frame{}
	for rows.Next() {
		var f framesv1.Frame
		var created, updated string
		if err := rows.Scan(&f.Id, &f.OrgId, &f.Name, &f.Description, &f.OwnerSub, &f.LatestVersion, &created, &updated); err != nil {
			return nil, err
		}
		f.CreatedAt, f.UpdatedAt = ts(created), ts(updated)
		out = append(out, &f)
	}
	return out, rows.Err()
}

func (r *Repository) DeleteFrame(ctx context.Context, frameID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Detach incoming inheritance edges (children survive; only the edge is removed).
	if _, err := tx.ExecContext(ctx, `DELETE FROM frame_extends WHERE parent_frame_id = ?`, frameID); err != nil {
		return err
	}
	// Deleting the frame cascades its frame_versions, frame_grants, and its own
	// outgoing frame_extends/excludes (ON DELETE CASCADE).
	res, err := tx.ExecContext(ctx, `DELETE FROM frames WHERE id = ?`, frameID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return store.ErrNotFound
	}
	return tx.Commit()
}
