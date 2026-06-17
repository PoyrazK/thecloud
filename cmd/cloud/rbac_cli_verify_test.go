package main

import (
	"testing"
)

func TestVerifyOptsKey(t *testing.T) {
	// Direct test: set opts.APIKey and call createClient
	opts.APIKey = "test-key"
	opts.APIURL = "http://localhost:8080"

	client := createClient(opts)
	if client == nil {
		t.Fatal("createClient returned nil")
	}
	t.Log("createClient returned valid client")
}
