//go:build linux && amd64

package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// VSOCK listener configuration.
// Firecracker guest agent listens on vsock (CID 3) and executes commands.
// This binary is meant to be embedded in the Firecracker guest rootfs.
const (
	// Guest CID - must match the CID configured in Firecracker adapter
	ListenCID = 3
	// Vsock port for agent connections
	ListenPort = 3
)

// vsockListener creates an AF_VSOCK socket listener.
func vsockListen(cid, port int) (net.Listener, error) {
	fd, err := unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM, 0)
	if err != nil {
		return nil, fmt.Errorf("socket create failed: %w", err)
	}

	// Bind to the specific CID and port
	addr := &unix.SockaddrVM{
		CID:  uint32(cid),
		Port: uint32(port),
	}
	if err := unix.Bind(fd, addr); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("bind failed: %w", err)
	}

	if err := unix.Listen(fd, 1); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("listen failed: %w", err)
	}

	return &vsockListener{fd: fd}, nil
}

type vsockListener struct {
	fd int
}

func (l *vsockListener) Accept() (net.Conn, error) {
	connfd, _, err := unix.Accept(l.fd)
	if err != nil {
		return nil, err
	}
	return &vsockConn{fd: connfd, localAddr: &vsockAddr{}, remoteAddr: &vsockAddr{}}, nil
}

func (l *vsockListener) Close() error {
	return unix.Close(l.fd)
}

func (l *vsockListener) Addr() net.Addr {
	return &vsockAddr{}
}

type vsockConn struct {
	fd         int
	localAddr  net.Addr
	remoteAddr net.Addr
}

func (c *vsockConn) Read(b []byte) (n int, err error) {
	return unix.Read(c.fd, b)
}

func (c *vsockConn) Write(b []byte) (n int, err error) {
	return unix.Write(c.fd, b)
}

func (c *vsockConn) Close() error {
	return unix.Close(c.fd)
}

func (c *vsockConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *vsockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *vsockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (c *vsockConn) LocalAddr() net.Addr  { return c.localAddr }
func (c *vsockConn) RemoteAddr() net.Addr { return c.remoteAddr }

type vsockAddr struct{}

func (a *vsockAddr) String() string {
	return fmt.Sprintf("vsock:%d:%d", ListenCID, ListenPort)
}

func (a *vsockAddr) Network() string {
	return "vsock"
}

var (
	mu       sync.Mutex
	active   int
	validate = commandValidator()
)

func commandValidator() *validator {
	return &validator{
		allowed: map[string]bool{
			"node":   true,
			"python": true,
			"ruby":   true,
			"go":     true,
			"java":   true,
		},
		dangerous: []*regexp.Regexp{
			regexp.MustCompile(`[;&|` + "\x24" + `<>]`),
			regexp.MustCompile(`\.\./`),
			regexp.MustCompile(`\$\(`),
			regexp.MustCompile(`\$\{`),
		},
	}
}

type validator struct {
	allowed   map[string]bool
	dangerous []*regexp.Regexp
}

func (v *validator) validate(cmd []string) error {
	if len(cmd) == 0 {
		return fmt.Errorf("empty command")
	}
	entrypoint := filepath.Base(cmd[0])
	if !v.allowed[entrypoint] {
		return fmt.Errorf("invalid entrypoint: %s", entrypoint)
	}
	for i := 1; i < len(cmd); i++ {
		for _, pat := range v.dangerous {
			if pat.MatchString(cmd[i]) {
				return fmt.Errorf("dangerous pattern detected")
			}
		}
	}
	return nil
}

func parseCommand(line string) ([]string, error) {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)
	for i := 0; i < len(line); i++ {
		c := line[i]
		if !inQuote && (c == '"' || c == '\'') {
			inQuote = true
			quoteChar = c
		} else if inQuote && c == quoteChar {
			inQuote = false
		} else if !inQuote && c == ' ' {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		} else {
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args, nil
}

func main() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)

	log.Printf("firecracker-agent starting, listening on vsock CID %d port %d", ListenCID, ListenPort)

	lis, err := vsockListen(ListenCID, ListenPort)
	if err != nil {
		log.Fatalf("failed to listen on vsock: %v", err)
	}
	defer lis.Close()

	log.Printf("firecracker-agent listening on vsock CID %d port %d", ListenCID, ListenPort)

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

		// Validate and parse command
		args, err := parseCommand(line)
		if err != nil {
			conn.Write([]byte(fmt.Sprintf("ERROR: %v\n", err)))
			continue
		}
		if err := validate.validate(args); err != nil {
			conn.Write([]byte(fmt.Sprintf("ERROR: %v\n", err)))
			continue
		}

		// Execute command - only entrypoint with args, no shell interpretation
		cmd := exec.Command(args[0], args[1:]...)
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
