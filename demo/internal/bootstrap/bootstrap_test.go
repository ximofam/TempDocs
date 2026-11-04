// SPDX-License-Identifier: Apache-2.0
package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDoRejectsHTTPAndGraphQLErrors(t *testing.T) {
	for _, tc := range []struct {
		code int
		body string
		bad  bool
	}{{200, `{"data":{"ok":true}}`, false}, {200, `{"errors":[{"message":"denied"}]}`, true}, {401, `{"error":"denied"}`, true}} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.code); w.Write([]byte(tc.body)) }))
		c := Client{}
		_, e := c.Do(context.Background(), "POST", srv.URL, "application/json", nil, nil)
		srv.Close()
		if (e != nil) != tc.bad {
			t.Fatalf("%d %s: %v", tc.code, tc.body, e)
		}
	}
}

func TestKestraBasicAuthIsScopedToKestra(t *testing.T) {
	for _, isKestra := range []bool{true, false} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, p, ok := r.BasicAuth()
			if isKestra && (!ok || u != "demo@example.com" || p != "Password123!") {
				t.Error("missing Kestra credentials")
			}
			if !isKestra && ok {
				t.Error("leaked Kestra credentials")
			}
			w.Write([]byte(`{}`))
		}))
		if isKestra {
			t.Setenv("KESTRA_URL", srv.URL)
		} else {
			t.Setenv("KESTRA_URL", "http://different-host:8080")
		}
		t.Setenv("KESTRA_USER", "demo@example.com")
		t.Setenv("KESTRA_PASSWORD", "Password123!")
		if _, e := (Client{}).Do(context.Background(), "GET", srv.URL+"/api/v1/configs", "", nil, nil); e != nil {
			t.Fatal(e)
		}
		srv.Close()
	}
}

func TestBootstrapUpsertsNamespaceWithoutDeletingOtherFlows(t *testing.T) {
	t.Chdir(t.TempDir())
	os.Mkdir("kestra", 0755)
	os.Mkdir("hasura", 0755)
	for _, f := range []string{"retry.yaml", "procurement.yaml"} {
		os.WriteFile("kestra/"+f, []byte("id: example\nnamespace: dxos.demo\n"), 0644)
	}
	os.WriteFile("hasura/event-trigger-security.sql", []byte("SELECT 1"), 0644)
	os.WriteFile("hasura/metadata.json", []byte(`{"version":3}`), 0644)
	count := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/main/flows") {
			if r.URL.Path != "/api/v1/main/flows/dxos.demo" || r.URL.Query().Get("delete") != "false" {
				t.Errorf("wrong flow endpoint %s", r.URL)
			}
			count++
			w.Write([]byte(`[]`))
			return
		}
		if r.URL.Path == "/v1/metadata" {
			w.Write([]byte(`{"is_consistent":true}`))
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	t.Setenv("HASURA_URL", srv.URL)
	t.Setenv("KESTRA_URL", srv.URL)
	if e := (Client{}).Run(context.Background()); e != nil {
		t.Fatal(e)
	}
	if count != 2 {
		t.Fatalf("imported %d flows", count)
	}
}
