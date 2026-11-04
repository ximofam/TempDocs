// SPDX-License-Identifier: Apache-2.0
// Package store giữ các lệnh nghiệp vụ trong giao dịch PostgreSQL.
package store

import (
	"context"
	"database/sql"
	"dxos-demo/internal/auth"
	"errors"
	"fmt"
)

type Store struct{ DB *sql.DB }
type Result struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Version int    `json:"version"`
}
type Event struct {
	EventID     string `json:"event_id"`
	EventType   string `json:"event_type"`
	WorkspaceID string `json:"workspace_id"`
	RequestID   string `json:"request_id"`
	Version     int    `json:"version"`
}
type failure struct {
	code    int
	message string
}

func (e failure) Error() string       { return e.message }
func fail(code int, msg string) error { return failure{code, msg} }
func ErrorCode(err error) int {
	if err == nil {
		return 0
	}
	var f failure
	if errors.As(err, &f) {
		return f.code
	}
	return 500
}
func begin(ctx context.Context, db *sql.DB) (*sql.Tx, error) { return db.BeginTx(ctx, nil) }
func load(ctx context.Context, tx *sql.Tx, i auth.Identity, id string) (Result, string, error) {
	var r Result
	var owner string
	e := tx.QueryRowContext(ctx, `SELECT id,status,version,created_by FROM procurement.requests WHERE id::text=$1 AND workspace_id=$2 FOR UPDATE`, id, i.WorkspaceID).Scan(&r.ID, &r.Status, &r.Version, &owner)
	if errors.Is(e, sql.ErrNoRows) {
		return r, "", fail(404, "request not found in workspace")
	}
	return r, owner, e
}
func (s Store) Submit(ctx context.Context, i auth.Identity, id string, v int) (Result, error) {
	tx, e := begin(ctx, s.DB)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback()
	r, owner, e := load(ctx, tx, i, id)
	if e != nil {
		return r, e
	}
	if i.Role != "employee" || owner != i.UserID {
		return r, fail(403, "only owning employee can submit")
	}
	if r.Version != v || r.Status != "DRAFT" {
		return r, fail(409, "state or version conflict")
	}
	r.Version++
	r.Status = "PENDING_APPROVAL"
	if _, e = tx.ExecContext(ctx, `UPDATE procurement.requests SET status=$2,version=$3 WHERE id=$1`, id, r.Status, r.Version); e != nil {
		return r, e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO core.outbox(event_type,workspace_id,request_id,version) VALUES('request.submitted',$1,$2,$3)`, i.WorkspaceID, id, r.Version); e != nil {
		return r, e
	}
	return r, tx.Commit()
}
func (s Store) Decide(ctx context.Context, i auth.Identity, id string, v int, decision string) (Result, error) {
	if i.Role != "manager" {
		return Result{}, fail(403, "manager role required")
	}
	if decision != "approve" && decision != "reject" {
		return Result{}, fail(422, "unknown decision")
	}
	tx, e := begin(ctx, s.DB)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback()
	r, owner, e := load(ctx, tx, i, id)
	if e != nil {
		return r, e
	}
	if owner == i.UserID {
		return r, fail(403, "cannot decide own request")
	}
	if r.Status != "PENDING_APPROVAL" || r.Version != v {
		return r, fail(409, "state or version conflict")
	}
	var ready bool
	if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM procurement.approval_inbox WHERE workspace_id=$1 AND request_id=$2 AND version=$3)`, i.WorkspaceID, id, v).Scan(&ready); e != nil {
		return r, e
	}
	if !ready {
		return r, fail(409, "approval inbox not ready; inspect workflow")
	}
	r.Status = "APPROVED"
	if decision == "reject" {
		r.Status = "REJECTED"
	}
	r.Version++
	if _, e = tx.ExecContext(ctx, `UPDATE procurement.requests SET status=$2,version=$3 WHERE id=$1`, id, r.Status, r.Version); e != nil {
		return r, e
	}
	return r, tx.Commit()
}
func (s Store) Event(ctx context.Context, id string) (Event, error) {
	var e Event
	err := s.DB.QueryRowContext(ctx, `SELECT event_id,event_type,workspace_id,request_id,version FROM core.outbox WHERE request_id::text=$1 ORDER BY version DESC LIMIT 1`, id).Scan(&e.EventID, &e.EventType, &e.WorkspaceID, &e.RequestID, &e.Version)
	return e, err
}
func (s Store) Deliver(ctx context.Context, ev Event) (string, error) {
	tx, e := begin(ctx, s.DB)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	// Advisory lock theo event_id giữ đến commit; không cần cấp UPDATE cho
	// bảng outbox vốn chỉ được API thêm sự kiện. Hash trùng chỉ làm chờ thêm.
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, ev.EventID); e != nil {
		return "", e
	}
	var actual Event
	e = tx.QueryRowContext(ctx, `SELECT event_id,event_type,workspace_id,request_id,version FROM core.outbox WHERE event_id::text=$1`, ev.EventID).Scan(&actual.EventID, &actual.EventType, &actual.WorkspaceID, &actual.RequestID, &actual.Version)
	if errors.Is(e, sql.ErrNoRows) {
		return "", fail(422, "unknown event")
	}
	if e != nil {
		return "", e
	}
	if actual != ev || ev.EventType != "request.submitted" {
		return "", fail(422, "event payload differs from persisted outbox")
	}
	var outcome string
	e = tx.QueryRowContext(ctx, `SELECT result FROM core.processed_events WHERE event_id=$1`, ev.EventID).Scan(&outcome)
	if e == nil {
		return outcome, tx.Commit()
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return "", e
	}
	r, _, e := load(ctx, tx, auth.Identity{WorkspaceID: ev.WorkspaceID}, ev.RequestID)
	if e != nil {
		return "", e
	}
	outcome = "inbox_created"
	if r.Status != "PENDING_APPROVAL" || r.Version != ev.Version {
		outcome = "skipped_stale"
	} else {
		if _, e = tx.ExecContext(ctx, `INSERT INTO procurement.approval_inbox(event_id,workspace_id,request_id,version) VALUES($1,$2,$3,$4)`, ev.EventID, ev.WorkspaceID, ev.RequestID, ev.Version); e != nil {
			return "", e
		}
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO core.processed_events(event_id,result) VALUES($1,$2)`, ev.EventID, outcome); e != nil {
		return "", fmt.Errorf("record processing: %w", e)
	}
	return outcome, tx.Commit()
}
