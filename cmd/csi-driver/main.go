package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/poyrazk/thecloud/internal/csi"
	"github.com/poyrazk/thecloud/pkg/sdk"
	"log/slog"
)

func main() {
	var (
		endpoint   = flag.String("endpoint", "unix:///tmp/csi.sock", "CSI endpoint")
		driverName = flag.String("drivername", "csi.thecloud.io", "name of the driver")
		nodeID     = flag.String("nodeid", "", "node id")
		version    = flag.String("version", "1.0.0", "driver version")
		apiURL     = flag.String("api-url", "http://localhost:8080", "Cloud API URL")
		apiKey     = flag.String("api-key", "", "Cloud API Key")
	)
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if *nodeID == "" {
		*nodeID = os.Getenv("NODE_ID")
	}

	if *nodeID == "" {
		fmt.Println("nodeid must be provided via flag or NODE_ID environment variable")
		os.Exit(1)
	}

	if *apiKey == "" {
		*apiKey = os.Getenv("CLOUD_API_KEY")
	}

	if *apiKey == "" {
		logger.Error("CLOUD_API_KEY is required")
		os.Exit(1)
	}

	cloudClient := sdk.NewClient(*apiURL, *apiKey)
	d := csi.NewDriver(*driverName, *version, *nodeID, *endpoint, cloudClient, logger)

	var wg sync.WaitGroup
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-c
		d.Stop()
	}()

	if err := d.Run(); err != nil {
		fmt.Printf("Failed to run driver: %v\n", err)
		os.Exit(1)
	}

	// Wait for signal handler to complete cleanup with timeout
	done := make(chan struct{}, 1)
	go func() {
		wg.Wait()
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		fmt.Println("cleanup timed out after 5s, exiting anyway")
	}
}
