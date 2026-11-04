// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSubscriptionHandlesPingBeforeAck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"graphql-transport-ws"}})
		if e != nil {
			t.Error(e)
			return
		}
		defer c.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		_, initBody, e := c.Read(ctx)
		if e != nil {
			t.Error(e)
			return
		}
		var init struct {
			Payload struct {
				Headers map[string]string `json:"headers"`
			} `json:"payload"`
		}
		if e = json.Unmarshal(initBody, &init); e != nil || !strings.HasPrefix(init.Payload.Headers["Authorization"], "Bearer ") {
			t.Error("JWT must be inside payload.headers")
			return
		}
		for _, m := range []string{`{"type":"ping"}`, `{"type":"connection_ack"}`} {
			if e = c.Write(ctx, websocket.MessageText, []byte(m)); e != nil {
				return
			}
		}
		for {
			_, b, e := c.Read(ctx)
			if e != nil {
				return
			}
			if strings.Contains(string(b), `"subscribe"`) {
				c.Write(ctx, websocket.MessageText, []byte(`{"id":"1","type":"next","payload":{"data":{"procurement_requests":[]}}}`))
				return
			}
		}
	}))
	defer srv.Close()
	t.Setenv("HASURA_URL", srv.URL)
	t.Setenv("JWT_KEY", "test-key-at-least-thirty-two-bytes")
	if e := subscription(context.Background(), "alice", false); e != nil {
		t.Fatal(e)
	}
}
