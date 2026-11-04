// SPDX-License-Identifier: Apache-2.0
package server

import (
	"context"
	"crypto/subtle"
	"dxos-demo/internal/auth"
	"dxos-demo/internal/store"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

func New(s store.Store, a auth.Auth, serviceToken string) http.Handler {
	mux := http.NewServeMux()
	var mu sync.Mutex
	attempts := map[string]int{}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if s.DB == nil || s.DB.PingContext(ctx) != nil {
			write(w, 503, map[string]string{"status": "db unavailable"})
			return
		}
		write(w, 200, map[string]string{"status": "ok"})
	})
	svc := func(w http.ResponseWriter, r *http.Request) bool {
		expected := "Bearer " + serviceToken
		if serviceToken == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(expected)) != 1 {
			write(w, 401, map[string]string{"error": "service token required"})
			return false
		}
		return true
	}
	mux.HandleFunc("POST /internal/practice/{run}", func(w http.ResponseWriter, r *http.Request) {
		if !svc(w, r) {
			return
		}
		n, e := strconv.Atoi(r.URL.Query().Get("failures"))
		if e != nil || n < 0 || n > 10 {
			write(w, 422, map[string]string{"error": "failures must be 0..10"})
			return
		}
		mu.Lock()
		attempts[r.PathValue("run")]++
		count := attempts[r.PathValue("run")]
		mu.Unlock()
		code := 200
		if count <= n {
			code = 503
		}
		write(w, code, map[string]any{"attempt": count, "run": r.PathValue("run"), "ok": code == 200})
	})
	mux.HandleFunc("POST /internal/events", func(w http.ResponseWriter, r *http.Request) {
		if !svc(w, r) {
			return
		}
		var ev store.Event
		if !decode(w, r, &ev) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		result, e := s.Deliver(ctx, ev)
		if e != nil {
			respondError(w, e)
			return
		}
		write(w, 200, map[string]string{"result": result})
	})
	mux.HandleFunc("POST /requests/{id}/{action}", func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		i, e := a.Verify(raw)
		if e != nil {
			write(w, 401, map[string]string{"error": "valid JWT required"})
			return
		}
		var body struct {
			Version int `json:"expected_version"`
		}
		if !decode(w, r, &body) {
			return
		}
		if body.Version < 1 {
			write(w, 422, map[string]string{"error": "expected_version must be positive"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		var result store.Result
		switch action := r.PathValue("action"); action {
		case "submit":
			result, e = s.Submit(ctx, i, r.PathValue("id"), body.Version)
		case "approve", "reject":
			result, e = s.Decide(ctx, i, r.PathValue("id"), body.Version, action)
		default:
			write(w, 404, map[string]string{"error": "unknown command"})
			return
		}
		if e != nil {
			respondError(w, e)
			return
		}
		write(w, 200, result)
	})
	return mux
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		write(w, 400, map[string]string{"error": "invalid JSON body"})
		return false
	}
	if e := d.Decode(new(any)); !errors.Is(e, io.EOF) {
		write(w, 400, map[string]string{"error": "expected one JSON object"})
		return false
	}
	return true
}
func write(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
func respondError(w http.ResponseWriter, e error) {
	code := store.ErrorCode(e)
	msg := e.Error()
	if code == 500 {
		log.Printf("request failed: %v", e)
		msg = "internal error; inspect API logs"
	}
	write(w, code, map[string]string{"error": msg})
}
