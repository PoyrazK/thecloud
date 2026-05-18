// Firecracker guest agent listens on vsock and executes commands.
// This binary is meant to be embedded in the Firecracker guest rootfs.
package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// VSOCK listener configuration.
// In Firecracker, the guest CID is configured in the vsock device.
// The host connects to the Unix socket path specified in the vsock device config.
const (
	// ListenAddr is the address the agent listens on.
	// For vsock, we listen on CID 3 (host-initiated connections).
	ListenAddr = "/var/run/firecracker-agent.sock"
)

var (
	mu     sync.Mutex
	active int
)

func main() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)

	// Create a Unix domain socket for the agent
	if err := os.RemoveAll(ListenAddr); err != nil {
		log.Fatalf("failed to remove existing socket: %v", err)
	}

	lis, err := net.Listen("unix", ListenAddr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", ListenAddr, err)
	}
	defer lis.Close()

	log.Printf("firecracker-agent listening on %s", ListenAddr)

	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}

		mu.Lock()
		active++
		mu.Unlock()

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		mu.Lock()
		active--
		mu.Unlock()
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse command - simple newline-delimited protocol
		// Empty line or "quit" closes connection
		if line == "quit" || line == "exit" {
			return
		}

		// Execute command via shell
		cmd := exec.Command("sh", "-c", line)
		out, err := cmd.CombinedOutput()

		var response string
		if err != nil {
			response = fmt.Sprintf("ERROR: %v\n%s\n", err, string(out))
		} else {
			response = string(out)
			if !strings.HasSuffix(response, "\n") {
				response += "\n"
			}
		}

		if _, err := conn.Write([]byte(response)); err != nil {
			log.Printf("write error: %v", err)
			return
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("scanner error: %v", err)
	}
}
