package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", handle)

	log.Println("upstream listening on :9000")
	log.Fatal(http.ListenAndServe(":9000", nil))
}

func handle(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("content-type", "application/json")

	body := map[string]any{
		"ok":   true,
		"path": r.URL.Path,
	}

	_ = json.NewEncoder(w).Encode(body)
}
