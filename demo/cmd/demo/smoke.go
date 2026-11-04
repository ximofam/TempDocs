// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"dxos-demo/internal/auth"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func smoke(ctx context.Context) error {
	mustFail := func(label string, e error) error {
		if e == nil {
			return fmt.Errorf("%s: unexpectedly accepted", label)
		}
		fmt.Println("PASS", label)
		return nil
	}
	// Hasura phải xác minh JWT thật, không gửi admin secret.
	bad := auth.Auth{Key: []byte("wrong-signing-key-at-least-thirty-two-bytes")}
	id, _ := auth.DemoIdentity("alice")
	forged, _ := bad.Sign(id, time.Hour)
	expired, _ := (auth.Auth{Key: []byte(os.Getenv("JWT_KEY"))}).Sign(id, -time.Minute)
	for _, tok := range []string{forged, expired} {
		_, e := gqlToken(ctx, tok, `{ procurement_requests { id } }`)
		if e = mustFail("Hasura rejects invalid JWT", e); e != nil {
			return e
		}
	}
	b, e := gql(ctx, "alice", `{ procurement_requests(where:{workspace_id:{_eq:"ws-b"}}) { id } }`)
	if e != nil {
		return e
	}
	var filtered struct {
		Data struct {
			Rows []any `json:"procurement_requests"`
		} `json:"data"`
	}
	if e = json.Unmarshal(b, &filtered); e != nil {
		return e
	}
	if len(filtered.Data.Rows) != 0 {
		return fmt.Errorf("workspace leak: %s", b)
	}
	fmt.Println("PASS workspace isolation")
	q := `mutation { insert_procurement_requests_one(object:{work_item_id:"10000000-0000-0000-0000-000000000001",description:"Smoke test",amount:100}) {id version status} }`
	b, e = gql(ctx, "alice", q)
	if e != nil {
		return e
	}
	var created struct {
		Data struct {
			Row struct {
				ID      string `json:"id"`
				Version int    `json:"version"`
			} `json:"insert_procurement_requests_one"`
		} `json:"data"`
	}
	if e = json.Unmarshal(b, &created); e != nil {
		return e
	}
	requestID := created.Data.Row.ID
	if requestID == "" {
		return fmt.Errorf("no created request: %s", b)
	}
	fmt.Println("Request:", requestID)
	_, e = gql(ctx, "alice", `mutation { insert_procurement_requests_one(object:{work_item_id:"10000000-0000-0000-0000-000000000002",amount:100}) {id} }`)
	if e = mustFail("cross workspace relation rejected", e); e != nil {
		return e
	}
	_, e = gql(ctx, "alice", fmt.Sprintf(`mutation { update_procurement_requests_by_pk(pk_columns:{id:%q},_set:{status:"APPROVED"}) {id} }`, requestID))
	if e = mustFail("CRUD cannot write status", e); e != nil {
		return e
	}
	_, e = gql(ctx, "alice", fmt.Sprintf(`mutation { update_procurement_requests_by_pk(pk_columns:{id:%q},_set:{amount:101}) {version} }`, requestID))
	if e != nil {
		return e
	}
	if e = subscription(ctx, "alice", false); e != nil {
		return e
	}
	_, e = command(ctx, "submit", "alice", requestID, 1)
	if e = mustFail("old draft version rejected", e); e != nil {
		return e
	}
	if _, e = command(ctx, "submit", "alice", requestID, 2); e != nil {
		return e
	}
	deadline := time.Now().Add(90 * time.Second)
	for {
		b, e = gql(ctx, "manager-a", fmt.Sprintf(`{ procurement_approval_inbox(where:{request_id:{_eq:%q}}) {event_id} }`, requestID))
		if e != nil {
			return e
		}
		var inbox struct {
			Data struct {
				Rows []any `json:"procurement_approval_inbox"`
			} `json:"data"`
		}
		if e = json.Unmarshal(b, &inbox); e != nil {
			return e
		}
		if len(inbox.Data.Rows) == 1 {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("inbox timeout: inspect Hasura Event Trigger delivery and Kestra executions")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	fmt.Println("PASS Hasura event → Kestra → Go inbox")
	for _, who := range []string{"alice", "manager-b"} {
		_, e = command(ctx, "approve", who, requestID, 3)
		if e = mustFail("wrong approver "+who, e); e != nil {
			return e
		}
	}
	_, e = command(ctx, "approve", "manager-a", requestID, 2)
	if e = mustFail("old approval version rejected", e); e != nil {
		return e
	}
	b, e = command(ctx, "approve", "manager-a", requestID, 3)
	if e != nil {
		return e
	}
	var approved struct {
		Status  string `json:"status"`
		Version int    `json:"version"`
	}
	if e = json.Unmarshal(b, &approved); e != nil {
		return e
	}
	if approved.Status != "APPROVED" || approved.Version != 4 {
		return fmt.Errorf("unexpected approval %s", b)
	}
	if e = replay(ctx, requestID); e != nil {
		return e
	}
	b, e = gql(ctx, "manager-a", fmt.Sprintf(`{ procurement_approval_inbox(where:{request_id:{_eq:%q}}) {event_id} }`, requestID))
	if e != nil {
		return e
	}
	var final struct {
		Data struct {
			Rows []any `json:"procurement_approval_inbox"`
		} `json:"data"`
	}
	if e = json.Unmarshal(b, &final); e != nil {
		return e
	}
	if len(final.Data.Rows) != 1 {
		return fmt.Errorf("replay duplicated inbox: %s", b)
	}
	fmt.Println("PASS approval + replay without duplicate inbox")
	if e = retry(ctx, 2); e != nil {
		return e
	}
	fmt.Println("SMOKE PASS")
	return nil
}
