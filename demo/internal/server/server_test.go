// SPDX-License-Identifier: Apache-2.0
package server

import (
	"dxos-demo/internal/auth"
	"dxos-demo/internal/store"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPRejectsUnauthorizedAndMalformed(t *testing.T) {
	a := auth.Auth{Key: []byte("test-key-at-least-thirty-two-bytes")}
	h := New(store.Store{}, a, "svc")
	token, _ := a.Sign(auth.Identity{UserID: "alice", WorkspaceID: "ws-a", Role: "employee"}, time.Hour)
	for _, tc := range []struct {
		path, body, token string
		code              int
	}{{"/requests/id/submit", "{}", "", 401}, {"/requests/id/submit", "{}", "bad", 401}, {"/requests/id/submit", "{", token, 400}, {"/requests/id/submit", `{"expected_version":0}`, token, 422}, {"/internal/events", "{}", token, 401}, {"/internal/events", "{", "svc", 400}} {
		r := httptest.NewRequest("POST", tc.path, strings.NewReader(tc.body))
		if tc.token != "" {
			r.Header.Set("Authorization", "Bearer "+tc.token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("%s want %d got %d %s", tc.path, tc.code, w.Code, w.Body.String())
		}
	}
}
func TestPracticeFailsExactCountAndIsolatesRunIDs(t *testing.T) {
	h := New(store.Store{}, auth.Auth{}, "svc")
	for _, tc := range []struct {
		path string
		code int
	}{{"a?failures=2", 503}, {"a?failures=2", 503}, {"a?failures=2", 200}, {"b?failures=2", 503}, {"c?failures=-1", 422}, {"d?failures=11", 422}, {"e?failures=oops", 422}} {
		r := httptest.NewRequest("POST", "/internal/practice/"+tc.path, nil)
		r.Header.Set("Authorization", "Bearer svc")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("%s want %d got %d", tc.path, tc.code, w.Code)
		}
	}
}
