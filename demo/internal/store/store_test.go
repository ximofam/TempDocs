// SPDX-License-Identifier: Apache-2.0
package store

import (
	"context"
	"database/sql"
	"dxos-demo/internal/auth"
	_ "github.com/jackc/pgx/v5/stdlib"
	"net/url"
	"os"
	"sync"
	"testing"
)

func dbTest(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL for PostgreSQL integration tests")
	}
	db, e := sql.Open("pgx", url)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
func fixture(t *testing.T, db *sql.DB) string {
	t.Helper()
	var id string
	e := db.QueryRow(`INSERT INTO procurement.requests(workspace_id,work_item_id,created_by,description,amount) VALUES('ws-a','10000000-0000-0000-0000-000000000001','alice','test',100) RETURNING id`).Scan(&id)
	if e != nil {
		t.Fatal(e)
	}
	return id
}

var alice = auth.Identity{UserID: "alice", WorkspaceID: "ws-a", Role: "employee"}
var mgr = auth.Identity{UserID: "manager-a", WorkspaceID: "ws-a", Role: "manager"}

func wantCode(t *testing.T, err error, code int) {
	t.Helper()
	if ErrorCode(err) != code {
		t.Fatalf("want code %d got %v", code, err)
	}
}
func TestSubmitOwnershipAndWorkspace(t *testing.T) {
	db := dbTest(t)
	s := Store{DB: db}
	id := fixture(t, db)
	ctx := context.Background()
	_, e := s.Submit(ctx, auth.Identity{UserID: "bob", WorkspaceID: "ws-b", Role: "employee"}, id, 1)
	wantCode(t, e, 404)
	_, e = s.Submit(ctx, auth.Identity{UserID: "eve", WorkspaceID: "ws-a", Role: "employee"}, id, 1)
	wantCode(t, e, 403)
	r, e := s.Submit(ctx, alice, id, 1)
	if e != nil || r.Status != "PENDING_APPROVAL" || r.Version != 2 {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestDecideRequiresManagerAndDifferentAuthor(t *testing.T) {
	db := dbTest(t)
	s := Store{DB: db}
	ctx := context.Background()
	id := fixture(t, db)
	r, e := s.Submit(ctx, alice, id, 1)
	if e != nil {
		t.Fatal(e)
	}
	var ev Event
	if e = db.QueryRow(`SELECT event_id,event_type,workspace_id,request_id,version FROM core.outbox WHERE request_id=$1`, id).Scan(&ev.EventID, &ev.EventType, &ev.WorkspaceID, &ev.RequestID, &ev.Version); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Deliver(ctx, ev); e != nil {
		t.Fatal(e)
	}
	_, e = s.Decide(ctx, alice, id, r.Version, "approve")
	wantCode(t, e, 403)
	_, e = s.Decide(ctx, auth.Identity{UserID: "alice", WorkspaceID: "ws-a", Role: "manager"}, id, 2, "approve")
	wantCode(t, e, 403)
	_, e = s.Decide(ctx, auth.Identity{UserID: "manager-b", WorkspaceID: "ws-b", Role: "manager"}, id, 2, "approve")
	wantCode(t, e, 404)
	got, e := s.Decide(ctx, mgr, id, 2, "approve")
	if e != nil || got.Status != "APPROVED" || got.Version != 3 {
		t.Fatalf("%+v %v", got, e)
	}
	_, e = s.Decide(ctx, mgr, id, 2, "approve")
	wantCode(t, e, 409)
}
func TestStaleVersion(t *testing.T) {
	db := dbTest(t)
	s := Store{DB: db}
	id := fixture(t, db)
	_, e := s.Submit(context.Background(), alice, id, 99)
	wantCode(t, e, 409)
}
func TestConcurrentDeliverCreatesOneInbox(t *testing.T) {
	db := dbTest(t)
	s := Store{DB: db}
	ctx := context.Background()
	id := fixture(t, db)
	if _, e := s.Submit(ctx, alice, id, 1); e != nil {
		t.Fatal(e)
	}
	ev, e := s.Event(ctx, id)
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := s.Deliver(ctx, ev); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	var count int
	db.QueryRow(`SELECT count(*) FROM procurement.approval_inbox WHERE request_id=$1`, id).Scan(&count)
	if count != 1 {
		t.Fatalf("inbox count %d", count)
	}
	ev.WorkspaceID = "ws-b"
	_, e = s.Deliver(ctx, ev)
	wantCode(t, e, 422)
}
func TestOldUnprocessedEventIsSkipped(t *testing.T) {
	db := dbTest(t)
	s := Store{DB: db}
	ctx := context.Background()
	id := fixture(t, db)
	// Công bố sự kiện cũ cùng transaction với trạng thái mới; workflow thật
	// không thể xử lý trước khi fixture đã sẵn sàng.
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	if _, e = tx.Exec(`UPDATE procurement.requests SET status='PENDING_APPROVAL',version=3 WHERE id=$1`, id); e != nil {
		t.Fatal(e)
	}
	var ev Event
	e = tx.QueryRow(`INSERT INTO core.outbox(event_type,workspace_id,request_id,version) VALUES('request.submitted','ws-a',$1,2) RETURNING event_id,event_type,workspace_id,request_id,version`, id).Scan(&ev.EventID, &ev.EventType, &ev.WorkspaceID, &ev.RequestID, &ev.Version)
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	result, e := s.Deliver(ctx, ev)
	if e != nil || result != "skipped_stale" {
		t.Fatalf("%s %v", result, e)
	}
}
func TestCrossWorkspaceForeignKey(t *testing.T) {
	db := dbTest(t)
	_, e := db.Exec(`INSERT INTO procurement.requests(workspace_id,work_item_id,created_by,amount) VALUES('ws-a','10000000-0000-0000-0000-000000000002','alice',10)`)
	if e == nil {
		t.Fatal("accepted cross workspace reference")
	}
}
func TestDraftEditIncrementsVersion(t *testing.T) {
	db := dbTest(t)
	s := Store{DB: db}
	id := fixture(t, db)
	var v int
	e := db.QueryRow(`UPDATE procurement.requests SET amount=123 WHERE id=$1 RETURNING version`, id).Scan(&v)
	if e != nil || v != 2 {
		t.Fatalf("%d %v", v, e)
	}
	if _, e = s.Submit(context.Background(), alice, id, 2); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`UPDATE procurement.requests SET amount=124 WHERE id=$1`, id); e == nil {
		t.Fatal("edited submitted content")
	}
}

func TestRuntimeRoleCanSubmitAndDeliver(t *testing.T) {
	admin := dbTest(t)
	id := fixture(t, admin)
	u, e := url.Parse(os.Getenv("TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	u.User = url.UserPassword("api_runtime", "demo-api")
	runtime, e := sql.Open("pgx", u.String())
	if e != nil {
		t.Fatal(e)
	}
	defer runtime.Close()
	s := Store{DB: runtime}
	ctx := context.Background()
	if _, e = s.Submit(ctx, alice, id, 1); e != nil {
		t.Fatal(e)
	}
	ev, e := s.Event(ctx, id)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Deliver(ctx, ev); e != nil {
		t.Fatal(e)
	}
}
