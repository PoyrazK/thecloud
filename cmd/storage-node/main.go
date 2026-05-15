// Package main provides the storage node entrypoint.
package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/poyrazk/thecloud/internal/storage/node"
	pb "github.com/poyrazk/thecloud/internal/storage/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	port := flag.String("port", "9101", "Port to listen on")
	dataDir := flag.String("data-dir", "./data/storage-node", "Directory to store data")
	peers := flag.String("peers", "", "Comma-separated list of peer addresses (e.g. localhost:9102)")
	nodeID := flag.String("id", "", "Unique Node ID (defaults to port)")
	tlsEnabled := flag.Bool("tls", false, "Enable TLS for peer communication")
	tlsCertFile := flag.String("tls-cert", "", "Path to TLS certificate file")
	tlsKeyFile := flag.String("tls-key", "", "Path to TLS key file")
	tlsCAFile := flag.String("tls-ca", "", "Path to CA certificate file for TLS verification")
	tlsSkipVerify := flag.Bool("tls-skip-verify", false, "Skip TLS certificate verification (dev only)")
	flag.Parse()

	if *nodeID == "" {
		*nodeID = "node-" + *port
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	logger.Info("starting storage node", "id", *nodeID, "port", *port, "dataDir", *dataDir, "tls", *tlsEnabled)

	// Build dial options based on TLS settings
	var dialOpts []grpc.DialOption
	if *tlsEnabled {
		if *tlsCertFile == "" || *tlsKeyFile == "" {
			return fmt.Errorf("--tls-cert and --tls-key are required when --tls is enabled")
		}
		cert, err := tls.LoadX509KeyPair(*tlsCertFile, *tlsKeyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS cert: %w", err)
		}
		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
		}
		if *tlsCAFile != "" {
			caCert, err := os.ReadFile(*tlsCAFile)
			if err != nil {
				return fmt.Errorf("failed to read TLS CA cert: %w", err)
			}
			caPool := x509.NewCertPool()
			if !caPool.AppendCertsFromPEM(caCert) {
				return fmt.Errorf("failed to parse TLS CA cert PEM from %s", *tlsCAFile)
			}
			tlsCfg.RootCAs = caPool
		}
		if *tlsSkipVerify {
			tlsCfg.InsecureSkipVerify = true
		}
		creds := credentials.NewTLS(tlsCfg)
		dialOpts = []grpc.DialOption{grpc.WithTransportCredentials(creds)}
	} else {
		dialOpts = []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	}

	// 1. Init Store
	store, err := node.NewLocalStore(*dataDir)
	if err != nil {
		logger.Error("failed to init store", "error", err)
		return err
	}

	// 2. Init Gossiper
	gossiper := node.NewGossipProtocol(*nodeID, "localhost:"+*port, dialOpts, logger)
	if *peers != "" {
		for _, peerAddr := range strings.Split(*peers, ",") {
			gossiper.AddPeer(peerAddr, peerAddr)
		}
	}
	gossiper.Start(1 * time.Second)
	defer gossiper.Stop()

	// 3. Init RPC Server
	rpcServer := node.NewRPCServer(store, gossiper)
	grpcServer := grpc.NewServer()
	pb.RegisterStorageNodeServer(grpcServer, rpcServer)

	// 3. Listen
	lis, err := net.Listen("tcp", ":"+*port)
	if err != nil {
		logger.Error("failed to listen", "error", err)
		return err
	}

	// 4. Handle Shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		logger.Info("shutting down")
		grpcServer.GracefulStop()
	}()

	logger.Info("storage node ready")
	if err := grpcServer.Serve(lis); err != nil {
		logger.Error("server failed", "error", err)
		return err
	}
	return nil
}
