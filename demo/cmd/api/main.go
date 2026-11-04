// SPDX-License-Identifier: Apache-2.0
package main

import (
	"database/sql"
	"dxos-demo/internal/auth"
	"dxos-demo/internal/server"
	"dxos-demo/internal/store"
	_ "github.com/jackc/pgx/v5/stdlib"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	db, e := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	key := os.Getenv("JWT_KEY")
	svc := os.Getenv("SERVICE_TOKEN")
	if len(key) < 32 || len(svc) < 16 {
		log.Fatal("JWT_KEY >=32 bytes and SERVICE_TOKEN >=16 bytes required")
	}
	srv := http.Server{Addr: ":8090", Handler: server.New(store.Store{DB: db}, auth.Auth{Key: []byte(key)}, svc), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	log.Print("API listening :8090")
	log.Fatal(srv.ListenAndServe())
}
