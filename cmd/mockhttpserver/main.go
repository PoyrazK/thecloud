package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"url":      r.URL.RequestURI(),
			"method":   r.Method,
			"headers":  r.Header,
			"path":     r.URL.Path,
			"raw_path": r.URL.RawPath,
		})
	})

	log.Println("Mock HTTP server listening on :8089")
	if err := http.ListenAndServe(":8089", nil); err != nil {
		log.Fatal(err)
	}
}
