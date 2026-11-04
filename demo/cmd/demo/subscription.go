// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	"os"
	"strings"
	"time"
)

func subscription(ctx context.Context, name string, watch bool) error {
	t, e := token(name)
	if e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	url := strings.Replace(os.Getenv("HASURA_URL"), "http", "ws", 1) + "/v1/graphql"
	conn, _, e := websocket.Dial(ctx, url, &websocket.DialOptions{Subprotocols: []string{"graphql-transport-ws"}})
	if e != nil {
		return e
	}
	defer conn.CloseNow()
	send := func(v any) error { b, _ := json.Marshal(v); return conn.Write(ctx, websocket.MessageText, b) }
	if e = send(map[string]any{"type": "connection_init", "payload": map[string]any{"headers": map[string]string{"Authorization": "Bearer " + t}}}); e != nil {
		return e
	}
	var b []byte
	for {
		_, b, e = conn.Read(ctx)
		if e != nil {
			return e
		}
		var ack struct {
			Type string `json:"type"`
		}
		if e = json.Unmarshal(b, &ack); e != nil {
			return e
		}
		if ack.Type == "connection_ack" {
			break
		}
		if ack.Type == "ping" {
			if e = send(map[string]string{"type": "pong"}); e != nil {
				return e
			}
			continue
		}
		return fmt.Errorf("subscription handshake: %s", b)
	}
	if e = send(map[string]any{"id": "1", "type": "subscribe", "payload": map[string]string{"query": "subscription { procurement_requests(order_by: {id: asc}) { id status version } }"}}); e != nil {
		return e
	}
	for {
		_, b, e = conn.Read(ctx)
		if e != nil {
			return e
		}
		var msg struct {
			Type string `json:"type"`
		}
		if e = json.Unmarshal(b, &msg); e != nil {
			return e
		}
		switch msg.Type {
		case "next":
			fmt.Println("Subscription:", string(b))
			if !watch {
				return nil
			}
		case "error", "connection_error":
			return fmt.Errorf("subscription: %s", b)
		case "ping":
			if e = send(map[string]string{"type": "pong"}); e != nil {
				return e
			}
		}
	}
}
