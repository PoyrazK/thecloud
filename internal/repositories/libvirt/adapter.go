// Package libvirt provides a Libvirt/KVM implementation of the ComputeBackend interface.
// It enables running VMs using QEMU/KVM with features like Cloud-Init, volume management,
// snapshots, and network configuration.
package libvirt

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/digitalocean/go-libvirt"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

const defaultPoolName = "default"

type LibvirtAdapter struct {
	conn   *libvirt.Libvirt
	logger *slog.Logger
	uri    string

	mu           sync.RWMutex
	portMappings map[string]map[string]int // instanceID -> internalPort -> hostPort

	// Network pool configuration
	networkCounter int
	poolStart      net.IP
	poolEnd        net.IP
}

func NewLibvirtAdapter(logger *slog.Logger, uri string) (*LibvirtAdapter, error) {
	if uri == "" {
		uri = "/var/run/libvirt/libvirt-sock"
	}

	// Connect to libvirt
	// We use a dialer for the unix socket
	c, err := net.DialTimeout("unix", uri, 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to dial libvirt: %w", err)
	}

	//nolint:staticcheck // libvirt.New is deprecated but NewWithDialer doesn't work with our setup
	l := libvirt.New(c)
	if err := l.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to libvirt: %w", err)
	}

	return &LibvirtAdapter{
		conn:           l,
		logger:         logger,
		uri:            uri,
		portMappings:   make(map[string]map[string]int),
		networkCounter: 0,
		poolStart:      net.ParseIP("192.168.100.0"),
		poolEnd:        net.ParseIP("192.168.200.255"),
	}, nil
}

// Ping checks if libvirt is reachable
func (a *LibvirtAdapter) Ping(ctx context.Context) error {
	return a.conn.Connect() // returns error only
}

func (a *LibvirtAdapter) Type() string {
	return "libvirt"
}

func (a *LibvirtAdapter) CreateInstance(ctx context.Context, name, imageName string, ports []string, networkID string, volumeBinds []string, env []string, cmd []string) (string, error) {
	// 1. Prepare storage
	// We assume 'imageName' is a backing volume in the default pool.
	pool, err := a.conn.StoragePoolLookupByName(defaultPoolName)
	if err != nil {
		return "", fmt.Errorf("default pool not found: %w", err)
	}

	// Create root disk for the VM
	// For simplicity, we just create an empty 10GB qcows if image not found, or clone if we knew how (omitted for brevity)
	volXML := generateVolumeXML(name+"-root", 10)

	vol, err := a.conn.StorageVolCreateXML(pool, volXML, 0)
	if err != nil {
		// Try to continue if exists? No, better fail.
		return "", fmt.Errorf("failed to create root volume: %w", err)
	}

	// Get volume path from libvirt
	diskPath, err := a.conn.StorageVolGetPath(vol)
	if err != nil {
		_ = a.conn.StorageVolDelete(vol, 0)
		return "", fmt.Errorf("failed to get volume path: %w", err)
	}

	// 2. Cloud-Init ISO (if env or cmd provided)
	isoPath := ""
	if len(env) > 0 || len(cmd) > 0 {
		var err error
		isoPath, err = a.generateCloudInitISO(ctx, name, env, cmd)
		if err != nil {
			a.logger.Warn("failed to generate cloud-init iso, proceeding without it", "error", err)
		}
	}

	// 3. Resolve Volume Binds to host paths
	var additionalDisks []string
	for _, volName := range volumeBinds {
		vol, err := a.conn.StorageVolLookupByName(pool, volName)
		if err == nil {
			path, err := a.conn.StorageVolGetPath(vol)
			if err == nil {
				additionalDisks = append(additionalDisks, path)
			}
		}
	}

	// 4. Define Domain
	// Memory: 512MB
	// CPU: 1
	if networkID == "" {
		networkID = "default"
	}

	domainXML := generateDomainXML(name, diskPath, networkID, isoPath, 512, 1, additionalDisks)

	dom, err := a.conn.DomainDefineXML(domainXML)
	if err != nil {
		// Clean up volume
		_ = a.conn.StorageVolDelete(vol, 0)
		if isoPath != "" {
			if err := os.Remove(isoPath); err != nil {
				// Log but don't fail cleanup
				fmt.Printf("Warning: failed to remove ISO %s: %v\n", isoPath, err)
			}
		}
		return "", fmt.Errorf("failed to define domain: %w", err)
	}

	// 5. Start Domain
	if err := a.conn.DomainCreate(dom); err != nil {
		return "", fmt.Errorf("failed to start domain: %w", err)
	}

	// 4. Port Forwarding (Best effort)
	if len(ports) > 0 {
		go func() {
			// Wait for VM to get an IP
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			ip, err := a.waitInitialIP(ctx, name)
			if err != nil {
				a.logger.Error("failed to get ip for port forwarding", "instance", name, "error", err)
				return
			}

			for _, p := range ports {
				// Format: [hostPort:]containerPort
				parts := strings.Split(p, ":")
				var hostPort, containerPort string
				if len(parts) == 2 {
					hostPort = parts[0]
					containerPort = parts[1]
				} else {
					hostPort = "0"
					containerPort = parts[0]
				}

				// Security: Validate ports are numeric
				var hP, cP int
				_, errH := fmt.Sscanf(hostPort, "%d", &hP)
				_, errC := fmt.Sscanf(containerPort, "%d", &cP)
				if (hostPort != "0" && errH != nil) || errC != nil {
					a.logger.Warn("invalid port format, skipping forwarding", "port", p)
					continue
				}

				// Validate IP format
				if net.ParseIP(ip) == nil {
					a.logger.Error("invalid vm ip for port forwarding", "ip", ip)
					return
				}

				hPort := 0
				if hostPort == "0" {
					// Allocate random port (deterministic for simplicity in this POC)
					hPort = 30000 + int(uuid.New().ID()%10000)
				} else {
					// Parse host port, ignore error as we validate hPort > 0 below
					_, _ = fmt.Sscanf(hostPort, "%d", &hPort)
				}

				if hPort > 0 {
					a.mu.Lock()
					if a.portMappings[name] == nil {
						a.portMappings[name] = make(map[string]int)
					}
					a.portMappings[name][containerPort] = hPort
					a.mu.Unlock()

					a.logger.Info("setting up port forwarding", "host", hPort, "vm", containerPort, "ip", ip)
					// iptables -t nat -A PREROUTING -p tcp --dport <hPort> -j DNAT --to <ip>:<containerPort>
					path, err := exec.LookPath("iptables")
					if err != nil {
						a.logger.Error("iptables not found, cannot set up port forwarding", "error", err)
						continue
					}
					if path != "" {
						cmd := exec.Command("sudo", "iptables", "-t", "nat", "-A", "PREROUTING", "-p", "tcp", "--dport", fmt.Sprintf("%d", hPort), "-j", "DNAT", "--to", ip+":"+containerPort)
						if err := cmd.Run(); err != nil {
							a.logger.Error("failed to set up iptables rule", "command", cmd.String(), "error", err)
						}
					}
				}
			}
		}()
	}

	return name, nil
}

func (a *LibvirtAdapter) waitInitialIP(ctx context.Context, id string) (string, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			ip, err := a.GetInstanceIP(ctx, id)
			if err == nil && ip != "" {
				return ip, nil
			}
		}
	}
}

func (a *LibvirtAdapter) StopInstance(ctx context.Context, id string) error {
	dom, err := a.conn.DomainLookupByName(id)
	if err != nil {
		return fmt.Errorf("domain not found: %w", err)
	}

	if err := a.conn.DomainDestroy(dom); err != nil {
		return fmt.Errorf("failed to destroy domain: %w", err)
	}
	return nil
}

func (a *LibvirtAdapter) DeleteInstance(ctx context.Context, id string) error {
	dom, err := a.conn.DomainLookupByName(id)
	if err != nil {
		return nil // Assume already gone
	}

	// Stop if running
	state, _, err := a.conn.DomainGetState(dom, 0)
	if err == nil && state == 1 { // Running
		_ = a.conn.DomainDestroy(dom)
	}

	// Undefine (remove XML)
	if err := a.conn.DomainUndefine(dom); err != nil {
		return fmt.Errorf("failed to undefine domain: %w", err)
	}

	// Cleanup port mappings and iptables rules if possible
	a.mu.Lock()
	if mappings, ok := a.portMappings[id]; ok {
		for _, hPort := range mappings {
			// Best effort cleanup: iptables -t nat -D PREROUTING ...
			_ = exec.Command("sudo", "iptables", "-t", "nat", "-D", "PREROUTING", "-p", "tcp", "--dport", fmt.Sprintf("%d", hPort), "-j", "DNAT").Run()
		}
		delete(a.portMappings, id)
	}
	a.mu.Unlock()

	// Cleanup Cloud-Init ISO if exists
	if err := validateID(id); err != nil {
		a.logger.Warn("invalid id for iso cleanup", "id", id)
		return nil
	}
	isoPath := filepath.Join("/tmp", "cloud-init-"+id+".iso")
	_ = os.Remove(isoPath)

	// Try to delete root volume?
	// Name-root
	pool, err := a.conn.StoragePoolLookupByName(defaultPoolName)
	if err == nil {
		volName := id + "-root"
		vol, err := a.conn.StorageVolLookupByName(pool, volName)
		if err == nil {
			_ = a.conn.StorageVolDelete(vol, 0)
		}
	}

	return nil
}

func (a *LibvirtAdapter) GetInstanceLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	// Read from standard qemu log location
	// Note: This contains QEMU output, not necessarily guest console output unless serial is redirected there.
	// To get guest console, we'd need to attach to console or read a file if defined in XML.
	// Our XML defined <console type='pty'> which goes to a PTY. Reading PTY from outside is complex.
	// We'll fall back to QEMU log for debug info.
	logPath := fmt.Sprintf("/var/log/libvirt/qemu/%s.log", id)
	f, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	return f, nil
}

func (a *LibvirtAdapter) GetInstanceStats(ctx context.Context, id string) (io.ReadCloser, error) {
	dom, err := a.conn.DomainLookupByName(id)
	if err != nil {
		return nil, fmt.Errorf("domain not found: %w", err)
	}

	// Memory stats
	// We use standard libvirt stats. The format for ComputeBackend expects JSON.
	// We'll construct a simple JSON.
	memStats, err := a.conn.DomainMemoryStats(dom, 10, 0)
	if err != nil {
		return nil, err
	}

	var usage, limit uint64
	for _, stat := range memStats {
		if stat.Tag == 6 { // rss
			usage = stat.Val * 1024 // KB to Bytes
		}
		if stat.Tag == 5 { // actual
			limit = stat.Val * 1024
		}
	}

	json := fmt.Sprintf(`{"memory_stats":{"usage":%d,"limit":%d}}`, usage, limit)
	return io.NopCloser(strings.NewReader(json)), nil
}

func (a *LibvirtAdapter) GetInstancePort(ctx context.Context, id string, internalPort string) (int, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if mappings, ok := a.portMappings[id]; ok {
		if hostPort, ok := mappings[internalPort]; ok {
			return hostPort, nil
		}
	}

	return 0, fmt.Errorf("no host port mapping found for instance %s port %s", id, internalPort)
}

func (a *LibvirtAdapter) GetInstanceIP(ctx context.Context, id string) (string, error) {
	// 1. Get Domain
	dom, err := a.conn.DomainLookupByName(id)
	if err != nil {
		return "", fmt.Errorf("domain not found: %w", err)
	}

	// 2. We need the MAC address to look up DHCP leases.
	// We can get XML desc and parse it.
	xmlDesc, err := a.conn.DomainGetXMLDesc(dom, 0)
	if err != nil {
		return "", fmt.Errorf("failed to get domain xml: %w", err)
	}

	// Extract MAC using string parsing (dirty but lighter than XML decoder for one field)
	// Look for <mac address='XX:XX:XX:XX:XX:XX'/>
	// This assumes one interface.
	start := strings.Index(xmlDesc, "<mac address='")
	if start == -1 {
		return "", fmt.Errorf("no mac address found in xml")
	}
	start += len("<mac address='")
	end := strings.Index(xmlDesc[start:], "'/>")
	if end == -1 {
		return "", fmt.Errorf("malformed xml mac address")
	}
	mac := xmlDesc[start : start+end]

	// 3. Lookup leases in default network
	// We assume "default" network
	net, err := a.conn.NetworkLookupByName("default")
	if err != nil {
		return "", fmt.Errorf("default network not found: %w", err)
	}

	// Pass nil for mac to get all leases (simplifies type handling of OptString)
	leases, _, err := a.conn.NetworkGetDhcpLeases(net, nil, 0, 0)
	if err != nil {
		return "", fmt.Errorf("failed to get leases: %w", err)
	}

	for _, lease := range leases {
		if len(lease.Mac) > 0 && lease.Mac[0] == mac {
			return lease.Ipaddr, nil
		}
	}

	return "", fmt.Errorf("no ip lease found for %s (%s)", id, mac)
}

func (a *LibvirtAdapter) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	return "", fmt.Errorf("exec not supported in libvirt adapter")
}

func (a *LibvirtAdapter) RunTask(ctx context.Context, opts ports.RunTaskOptions) (string, error) {
	// For now, we assume a base image "alpine" exists in the default pool or we fail.
	// We create a new instance with a randomized name.
	name := "task-" + uuid.New().String()[:8]

	// Create volumes for binders?
	// The ports.RunTaskOptions has Binds []string.
	// We currently only support simple host binds or ignore them for the POC.
	// To perform the snapshot task, we actually rely on bind mounts.
	// As noted before, we replaced SnapshotService logic to use CreateVolumeSnapshot,
	// so RunTask is less critical for Snapshots now, but still useful for other things.

	// Since we don't have a dynamic ISO generator linked yet, we just start the VM.
	// If the user wants to run a command, we'd need Cloud-Init.

	// 1. Create a disk for the task VM (clone alpine-base if we could, or just new one)
	// We use the same create logic as CreateInstance but force a small size
	// We assume "alpine" is the image source.
	// In CreateInstance we assume image is passed as name.
	// Here opts.Image is "alpine".

	// Check if base image exists
	pool, err := a.conn.StoragePoolLookupByName(defaultPoolName)
	if err != nil {
		return "", fmt.Errorf("failed to find default pool: %w", err)
	}

	// For brevity, we just create a new empty vol.
	// Real implementation should clone base image.
	volXML := generateVolumeXML(name+"-root", 1)
	vol, err := a.conn.StorageVolCreateXML(pool, volXML, 0)
	if err != nil {
		return "", fmt.Errorf("failed to create root volume: %w", err)
	}

	// Get path
	diskPath, err := a.conn.StorageVolGetPath(vol)
	if err != nil {
		_ = a.conn.StorageVolDelete(vol, 0)
		return "", fmt.Errorf("failed to get volume path: %w", err)
	}

	// 2. Cloud-Init
	isoPath, err := a.generateCloudInitISO(ctx, name, nil, opts.Command)
	if err != nil {
		a.logger.Warn("failed to generate cloud-init iso for task", "error", err)
	}

	// 3. Define Domain
	// Memory: opts.MemoryMB
	domainXML := generateDomainXML(name, diskPath, "default", isoPath, int(opts.MemoryMB), 1, nil)

	dom, err := a.conn.DomainDefineXML(domainXML)
	if err != nil {
		_ = a.conn.StorageVolDelete(vol, 0)
		return "", fmt.Errorf("failed to define domain: %w", err)
	}

	// 3. Start Domain
	if err := a.conn.DomainCreate(dom); err != nil {
		_ = a.conn.DomainUndefine(dom)
		_ = a.conn.StorageVolDelete(vol, 0)
		return "", fmt.Errorf("failed to start domain: %w", err)
	}

	return name, nil
}

func (a *LibvirtAdapter) WaitTask(ctx context.Context, id string) (int64, error) {
	// Poll for domain state to be Shutoff
	// Since we can't easily get the exit code from inside the VM without qemu-agent,
	// we assume 0 if it shuts down gracefully (state Shutoff).

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return -1, ctx.Err()
		case <-ticker.C:
			dom, err := a.conn.DomainLookupByName(id)
			if err != nil {
				// If domain is gone, maybe it was deleted?
				return -1, fmt.Errorf("domain not found: %w", err)
			}

			state, _, err := a.conn.DomainGetState(dom, 0)
			if err != nil {
				continue
			}

			// libvirt.DomainShutoff = 5
			if state == 5 {
				return 0, nil
			}
		}
	}
}

func (a *LibvirtAdapter) CreateNetwork(ctx context.Context, name string) (string, error) {
	// Allocate network range from pool
	gateway, rangeStart, rangeEnd := a.getNextNetworkRange()

	// Simple NAT network
	xml := generateNetworkXML(name, "virbr-"+name, gateway, rangeStart, rangeEnd)

	net, err := a.conn.NetworkDefineXML(xml)
	if err != nil {
		return "", fmt.Errorf("failed to define network: %w", err)
	}

	if err := a.conn.NetworkCreate(net); err != nil {
		return "", fmt.Errorf("failed to start network: %w", err)
	}

	return net.Name, nil
}

func (a *LibvirtAdapter) DeleteNetwork(ctx context.Context, id string) error {
	net, err := a.conn.NetworkLookupByName(id)
	if err != nil {
		return nil // assume deleted
	}

	if err := a.conn.NetworkDestroy(net); err != nil {
		a.logger.Warn("failed to destroy network", "error", err)
	}
	if err := a.conn.NetworkUndefine(net); err != nil {
		return fmt.Errorf("failed to undefine network: %w", err)
	}
	return nil
}

func (a *LibvirtAdapter) CreateVolume(ctx context.Context, name string) error {
	pool, err := a.conn.StoragePoolLookupByName(defaultPoolName)
	if err != nil {
		return fmt.Errorf("failed to find default storage pool: %w", err)
	}

	// 10GB default
	xml := generateVolumeXML(name, 10)

	// Refresh pool first
	if err := a.conn.StoragePoolRefresh(pool, 0); err != nil {
		// Log but continue
		a.logger.Warn("failed to refresh pool", "error", err)
	}

	_, err = a.conn.StorageVolCreateXML(pool, xml, 0)
	if err != nil {
		return fmt.Errorf("failed to create volume xml: %w", err)
	}
	return nil
}

func (a *LibvirtAdapter) DeleteVolume(ctx context.Context, name string) error {
	pool, err := a.conn.StoragePoolLookupByName(defaultPoolName)
	if err != nil {
		return fmt.Errorf("failed to find default storage pool: %w", err)
	}

	vol, err := a.conn.StorageVolLookupByName(pool, name)
	if err != nil {
		// Check if not found
		return nil
	}

	if err := a.conn.StorageVolDelete(vol, 0); err != nil {
		return fmt.Errorf("failed to delete volume: %w", err)
	}
	return nil
}

func (a *LibvirtAdapter) CreateVolumeSnapshot(ctx context.Context, volumeID string, destinationPath string) error {
	// volumeID is the libvirt volume name
	pool, err := a.conn.StoragePoolLookupByName(defaultPoolName)
	if err != nil {
		return fmt.Errorf("failed to find default storage pool: %w", err)
	}

	vol, err := a.conn.StorageVolLookupByName(pool, volumeID)
	if err != nil {
		return fmt.Errorf("failed to find volume: %w", err)
	}

	volPath, err := a.conn.StorageVolGetPath(vol)
	if err != nil {
		return fmt.Errorf("failed to get volume path: %w", err)
	}

	// Use qemu-img to convert the volume to a temporary qcow2
	tmpQcow2 := destinationPath + ".qcow2"
	cmd := exec.Command("qemu-img", "convert", "-O", "qcow2", volPath, tmpQcow2)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("qemu-img convert failed: %w", err)
	}
	defer func() {
		if err := os.Remove(tmpQcow2); err != nil {
			fmt.Printf("Warning: failed to remove temp file %s: %v\n", tmpQcow2, err)
		}
	}()

	// Now tar it to match the SnapshotService expectation (tarball of contents)
	// Actually, if we just want a "blob", a tarred qcow2 is fine.
	tarCmd := exec.Command("tar", "czf", destinationPath, "-C", filepath.Dir(tmpQcow2), filepath.Base(tmpQcow2))
	if err := tarCmd.Run(); err != nil {
		return fmt.Errorf("tar archive failed: %w", err)
	}

	return nil
}

func (a *LibvirtAdapter) RestoreVolumeSnapshot(ctx context.Context, volumeID string, sourcePath string) error {
	pool, err := a.conn.StoragePoolLookupByName(defaultPoolName)
	if err != nil {
		return fmt.Errorf("failed to find default storage pool: %w", err)
	}

	vol, err := a.conn.StorageVolLookupByName(pool, volumeID)
	if err != nil {
		return fmt.Errorf("failed to find volume: %w", err)
	}

	volPath, err := a.conn.StorageVolGetPath(vol)
	if err != nil {
		return fmt.Errorf("failed to get volume path: %w", err)
	}

	// 1. Untar
	tmpDir, err := os.MkdirTemp("", "restore-")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			fmt.Printf("Warning: failed to remove temp dir %s: %v\n", tmpDir, err)
		}
	}()

	untarCmd := exec.Command("tar", "xzf", sourcePath, "-C", tmpDir)
	if err := untarCmd.Run(); err != nil {
		return fmt.Errorf("untar failed: %w", err)
	}

	files, _ := os.ReadDir(tmpDir)
	if len(files) == 0 {
		return fmt.Errorf("empty snapshot archive")
	}
	tmpQcow2 := filepath.Join(tmpDir, files[0].Name())

	// 2. Restore using qemu-img
	cmd := exec.Command("qemu-img", "convert", "-O", "qcow2", tmpQcow2, volPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("qemu-img restore failed: %w", err)
	}

	return nil
}
func (a *LibvirtAdapter) generateCloudInitISO(ctx context.Context, name string, env []string, cmd []string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "cloud-init-"+name)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			fmt.Printf("Warning: failed to remove temp dir %s: %v\n", tmpDir, err)
		}
	}()

	// meta-data
	metaData := fmt.Sprintf("instance-id: %s\nlocal-hostname: %s\n", name, name)
	if err := os.WriteFile(filepath.Join(tmpDir, "meta-data"), []byte(metaData), 0644); err != nil {
		return "", err
	}

	// user-data
	var userData bytes.Buffer
	userData.WriteString("#cloud-config\n")

	if len(env) > 0 {
		userData.WriteString("write_files:\n")
		userData.WriteString("  - path: /etc/profile.d/cloud-env.sh\n")
		userData.WriteString("    content: |\n")
		for _, e := range env {
			userData.WriteString(fmt.Sprintf("      export %s\n", e))
		}
	}

	if len(cmd) > 0 {
		userData.WriteString("runcmd:\n")
		for _, c := range cmd {
			userData.WriteString(fmt.Sprintf("  - [ sh, -c, %q ]\n", c))
		}
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "user-data"), userData.Bytes(), 0644); err != nil {
		return "", err
	}

	isoPath := filepath.Join("/tmp", "cloud-init-"+name+".iso")

	// Create ISO
	genCmd := exec.Command("genisoimage", "-output", isoPath, "-volid", "config-2", "-joliet", "-rock",
		filepath.Join(tmpDir, "user-data"), filepath.Join(tmpDir, "meta-data"))

	if _, err := genCmd.CombinedOutput(); err != nil {
		genCmd = exec.Command("mkisofs", "-output", isoPath, "-volid", "config-2", "-joliet", "-rock",
			filepath.Join(tmpDir, "user-data"), filepath.Join(tmpDir, "meta-data"))
		if _, err2 := genCmd.CombinedOutput(); err2 != nil {
			return "", fmt.Errorf("failed to generate iso (genisoimage/mkisofs): %w", err2)
		}
	}

	return isoPath, nil
}

// getNextNetworkRange allocates the next /24 network from the pool
func (a *LibvirtAdapter) getNextNetworkRange() (gateway, rangeStart, rangeEnd string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Calculate base IP: poolStart + (networkCounter * 256)
	baseIP := make(net.IP, len(a.poolStart))
	copy(baseIP, a.poolStart)

	// Add offset for this network (each network gets a /24)
	offset := a.networkCounter * 256
	for i := len(baseIP) - 1; i >= 0 && offset > 0; i-- {
		sum := int(baseIP[i]) + offset
		baseIP[i] = byte(sum % 256)
		offset = sum / 256
	}

	a.networkCounter++

	// Gateway is .1, DHCP range is .2 to .254
	gw := make(net.IP, len(baseIP))
	copy(gw, baseIP)
	gw[len(gw)-1] = 1

	start := make(net.IP, len(baseIP))
	copy(start, baseIP)
	start[len(start)-1] = 2

	end := make(net.IP, len(baseIP))
	copy(end, baseIP)
	end[len(end)-1] = 254

	return gw.String(), start.String(), end.String()
}

func validateID(id string) error {
	if strings.Contains(id, "..") || strings.Contains(id, "/") || strings.Contains(id, "\\") {
		return fmt.Errorf("invalid id: contains path traversal characters")
	}
	return nil
}
