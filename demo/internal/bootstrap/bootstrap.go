// SPDX-License-Identifier: Apache-2.0
package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Client struct{ HTTP *http.Client }

func (c Client) Do(ctx context.Context, method, url, contentType string, body []byte, headers map[string]string) ([]byte, error) {
	req, e := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if base := os.Getenv("KESTRA_URL"); base != "" && strings.HasPrefix(url, strings.TrimRight(base, "/")+"/") && !strings.Contains(req.URL.Path, "/executions/webhook/") {
		req.SetBasicAuth(os.Getenv("KESTRA_USER"), os.Getenv("KESTRA_PASSWORD"))
	}
	h := c.HTTP
	if h == nil {
		h = &http.Client{Timeout: 35 * time.Second}
	}
	resp, e := h.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if e != nil {
		return nil, e
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return b, fmt.Errorf("HTTP %d: %s", resp.StatusCode, b)
	}
	var result map[string]json.RawMessage
	if json.Unmarshal(b, &result) == nil {
		if x, ok := result["errors"]; ok && string(x) != "[]" && string(x) != "null" {
			return b, fmt.Errorf("GraphQL errors: %s", x)
		}
	}
	return b, nil
}
func (c Client) Wait(ctx context.Context, url string) error {
	for {
		if _, e := c.Do(ctx, "GET", url, "", nil, nil); e == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("readiness %s: %w", url, ctx.Err())
		case <-time.After(time.Second):
		}
	}
}
func (c Client) Run(ctx context.Context) error {
	hasura := os.Getenv("HASURA_URL")
	kestra := os.Getenv("KESTRA_URL")
	if e := c.Wait(ctx, hasura+"/healthz"); e != nil {
		return e
	}
	if e := c.Wait(ctx, kestra+"/api/v1/configs"); e != nil {
		return e
	}
	for _, file := range []string{"kestra/retry.yaml", "kestra/procurement.yaml"} {
		b, e := os.ReadFile(file)
		if e != nil {
			return e
		}
		if _, e = c.Do(ctx, "POST", kestra+"/api/v1/main/flows/dxos.demo?delete=false", "application/x-yaml", b, nil); e != nil {
			return fmt.Errorf("import %s: %w", file, e)
		}
	}
	meta, e := os.ReadFile("hasura/metadata.json")
	if e != nil {
		return e
	}
	body := []byte(`{"type":"replace_metadata","args":{"allow_inconsistent_metadata":false,"metadata":` + string(meta) + `}}`)
	headers := map[string]string{"x-hasura-admin-secret": os.Getenv("HASURA_ADMIN_SECRET")}
	if _, e = c.Do(ctx, "POST", hasura+"/v1/metadata", "application/json", body, headers); e != nil {
		return e
	}
	// Metadata tạo/recreate trigger; áp dụng lại quyền hẹp sau mỗi bootstrap.
	securitySQL, e := os.ReadFile("hasura/event-trigger-security.sql")
	if e != nil {
		return e
	}
	securityBody, _ := json.Marshal(map[string]any{"type": "run_sql", "args": map[string]any{"source": "default", "sql": string(securitySQL)}})
	if _, e = c.Do(ctx, "POST", hasura+"/v2/query", "application/json", securityBody, headers); e != nil {
		return fmt.Errorf("event trigger permissions: %w", e)
	}
	b, e := c.Do(ctx, "POST", hasura+"/v1/metadata", "application/json", []byte(`{"type":"get_inconsistent_metadata","args":{}}`), headers)
	if e != nil {
		return e
	}
	var result struct {
		Consistent bool `json:"is_consistent"`
	}
	if json.Unmarshal(b, &result) != nil || !result.Consistent {
		return fmt.Errorf("inconsistent metadata: %s", b)
	}
	return nil
}
