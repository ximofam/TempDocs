// SPDX-License-Identifier: Apache-2.0
// CLI cho người học; thao tác bootstrap/replay cần cấu hình quản trị local.
package main

import (
	"context"
	"database/sql"
	"dxos-demo/internal/auth"
	"dxos-demo/internal/bootstrap"
	"dxos-demo/internal/store"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	"log"
	"os"
	"strconv"
	"time"
)

var client = bootstrap.Client{}

func main() {
	if e := run(os.Args[1:]); e != nil {
		log.Fatal(e)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		return errors.New("commands: bootstrap | token <name> | graphql <name> <file> | submit/approve/reject <name> <id> <version> | retry <failures> | replay <request-id> | subscribe <name> | smoke")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	switch args[0] {
	case "bootstrap":
		if e := client.Run(ctx); e != nil {
			return e
		}
		fmt.Println("Bootstrap OK: workflows + Hasura metadata")
		return nil
	case "token":
		if len(args) != 2 {
			return errors.New("token <alice|bob|manager-a|manager-b>")
		}
		tok, e := token(args[1])
		if e != nil {
			return e
		}
		fmt.Println(tok)
		return nil
	case "graphql":
		if len(args) != 3 {
			return errors.New("graphql <name> <file.graphql>")
		}
		b, e := os.ReadFile(args[2])
		if e != nil {
			return e
		}
		out, e := gql(ctx, args[1], string(b))
		if e != nil {
			return e
		}
		fmt.Println(string(out))
		return nil
	case "submit", "approve", "reject":
		if len(args) != 4 {
			return errors.New("command <identity> <request-id> <expected-version>")
		}
		v, e := strconv.Atoi(args[3])
		if e != nil {
			return e
		}
		out, e := command(ctx, args[0], args[1], args[2], v)
		if e != nil {
			return e
		}
		fmt.Println(string(out))
		return nil
	case "retry":
		if len(args) != 2 {
			return errors.New("retry <0..10>")
		}
		n, e := strconv.Atoi(args[1])
		if e != nil || n < 0 || n > 10 {
			return errors.New("failures must be 0..10")
		}
		return retry(ctx, n)
	case "replay":
		if len(args) != 2 {
			return errors.New("replay <request-id>")
		}
		return replay(ctx, args[1])
	case "subscribe":
		if len(args) != 2 {
			return errors.New("subscribe <identity>")
		}
		return subscription(ctx, args[1], true)
	case "smoke":
		return smoke(ctx)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
func token(name string) (string, error) {
	i, e := auth.DemoIdentity(name)
	if e != nil {
		return "", e
	}
	return (auth.Auth{Key: []byte(os.Getenv("JWT_KEY"))}).Sign(i, time.Hour)
}
func gql(ctx context.Context, name, q string) ([]byte, error) {
	t, e := token(name)
	if e != nil {
		return nil, e
	}
	return gqlToken(ctx, t, q)
}
func gqlToken(ctx context.Context, t, q string) ([]byte, error) {
	b, _ := json.Marshal(map[string]any{"query": q})
	return client.Do(ctx, "POST", os.Getenv("HASURA_URL")+"/v1/graphql", "application/json", b, map[string]string{"Authorization": "Bearer " + t})
}
func command(ctx context.Context, action, name, id string, v int) ([]byte, error) {
	t, e := token(name)
	if e != nil {
		return nil, e
	}
	b, _ := json.Marshal(map[string]int{"expected_version": v})
	return client.Do(ctx, "POST", os.Getenv("API_URL")+"/requests/"+id+"/"+action, "application/json", b, map[string]string{"Authorization": "Bearer " + t})
}
func execution(ctx context.Context, b []byte) error {
	var x struct {
		ID string `json:"id"`
	}
	if e := json.Unmarshal(b, &x); e != nil || x.ID == "" {
		return fmt.Errorf("missing execution id: %s", b)
	}
	fmt.Println("Execution:", x.ID)
	for {
		b, e := client.Do(ctx, "GET", os.Getenv("KESTRA_URL")+"/api/v1/main/executions/"+x.ID, "", nil, nil)
		if e != nil {
			return e
		}
		var status struct {
			State struct {
				Current string `json:"current"`
			} `json:"state"`
		}
		if e = json.Unmarshal(b, &status); e != nil {
			return e
		}
		switch status.State.Current {
		case "SUCCESS":
			fmt.Println("Execution SUCCESS")
			return nil
		case "FAILED", "KILLED", "CANCELLED", "WARNING":
			return fmt.Errorf("execution %s: %s; inspect Kestra UI", x.ID, status.State.Current)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
func retry(ctx context.Context, n int) error {
	runID := fmt.Sprintf("run-%d", time.Now().UnixNano())
	body := []byte(fmt.Sprintf("--dxos\r\nContent-Disposition: form-data; name=\"run_id\"\r\n\r\n%s\r\n--dxos\r\nContent-Disposition: form-data; name=\"failures\"\r\n\r\n%d\r\n--dxos--\r\n", runID, n))
	b, e := client.Do(ctx, "POST", os.Getenv("KESTRA_URL")+"/api/v1/main/executions/dxos.demo/retry-http", "multipart/form-data; boundary=dxos", body, nil)
	if e != nil {
		return e
	}
	return execution(ctx, b)
}
func replay(ctx context.Context, id string) error {
	db, e := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if e != nil {
		return e
	}
	defer db.Close()
	ev, e := (store.Store{DB: db}).Event(ctx, id)
	if e != nil {
		return e
	}
	b, _ := json.Marshal(map[string]any{"event": map[string]any{"data": map[string]any{"new": ev}}})
	out, e := client.Do(ctx, "POST", os.Getenv("KESTRA_WEBHOOK_URL"), "application/json", b, nil)
	if e != nil {
		return e
	}
	return execution(ctx, out)
}
