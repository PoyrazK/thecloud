// iptables-wrapper provides a safe wrapper around iptables for Firecracker port forwarding.
// It accepts arguments via stdin JSON to avoid shell injection.
package main

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"strconv"
)

type iptablesRequest struct {
	HostPort   int    `json:"host_port"`
	TargetIP   string `json:"target_ip"`
	TargetPort int    `json:"target_port"`
}

func main() {
	var req iptablesRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		log.Fatalf("failed to decode request: %v", err)
	}

	if err := setupNAT(req.HostPort, req.TargetIP, req.TargetPort); err != nil {
		log.Fatalf("setup NAT failed: %v", err)
	}
}

func setupNAT(hostPort int, targetIP string, targetPort int) error {
	args := []string{
		"-t", "nat", "-A", "PREROUTING",
		"-p", "tcp", "--dport", strconv.Itoa(hostPort),
		"-j", "DNAT", "--to-destination", targetIP + ":" + strconv.Itoa(targetPort),
	}
	cmd := exec.Command("iptables", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
