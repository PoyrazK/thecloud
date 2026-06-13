package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

func main() {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"url":      r.URL.RequestURI(),
			"method":   r.Method,
			"headers":  r.Header,
			"path":     r.URL.Path,
			"raw_path": r.URL.RawPath,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("failed to encode response: %v", err)
		}
	})

	srv := &http.Server{
		Addr:        ":8089",
		Handler:     handler,
		ReadTimeout: 10 * time.Second,
		// WriteTimeout is set via context deadline
	}

	log.Println("Mock HTTP server listening on :8089")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
