// Command synthetic-ops runs the deliberately vulnerable lab service.
//
// CI AND LAB USE ONLY. Bind to loopback, never to a routable interface.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joeyvictorino/assay/labs/synthetic-ops/handlers"
)

func main() {
	addr := flag.String("addr", ":9000", "listen address")
	dsn := flag.String("db", ":memory:", "sqlite dsn (default in-memory, reseeded on start)")
	public := flag.String("public", "./public", "directory served by /files")
	unsafeSSRF := flag.Bool("unsafe-ssrf", false, "allow /api/fetch to reach non-loopback hosts (never in CI)")
	flag.Parse()

	db, err := handlers.Open(*dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if err := handlers.Seed(db); err != nil {
		log.Fatalf("seed: %v", err)
	}
	h := handlers.New(db, handlers.Options{UnsafeSSRF: *unsafeSSRF, PublicDir: *public})
	srv := &http.Server{
		Addr:              *addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Fprintf(os.Stderr, "synthetic-ops: DELIBERATELY VULNERABLE lab listening on %s (unsafe-ssrf=%v)\n", *addr, *unsafeSSRF)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
