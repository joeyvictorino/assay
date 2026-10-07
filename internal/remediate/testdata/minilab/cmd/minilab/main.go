// Command minilab is a stand-in lab for remediate.Verify tests: one
// endpoint whose body the test diff changes from VULNERABLE to FIXED.
package main

import (
	"flag"
	"log"
	"net/http"
)

const banner = "VULNERABLE"

func main() {
	addr := flag.String("addr", "127.0.0.1:9900", "listen address")
	flag.Parse()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(banner))
	})
	log.Fatal(http.ListenAndServe(*addr, mux))
}
