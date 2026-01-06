package libvirt

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"github.com/digitalocean/go-libvirt"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

const defaultPoolName = "default"

type LibvirtAdapter struct {
	conn   *libvirt.Libvirt
	logger *slog.Logger
	uri    string
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

	l := libvirt.New(c)
	if err := l.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to libvirt: %w", err)
	}

	return &LibvirtAdapter{
		conn:   l,
		logger: logger,
		uri:    uri,
	}, nil
}

// Ping checks if libvirt is reachable
func (a *LibvirtAdapter) Ping(ctx context.Context) error {
	// Simple check: get version
	_, err := a.conn.ConnectGetLibVersion()
	return err
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

	// Get volume path
	// We need the key or path.
	// In go-libvirt, struct is returned.
	// We can construct path: /var/lib/libvirt/images/name-root
	// Or query XML.
	// For now, assume standard path.
	diskPath := fmt.Sprintf("/var/lib/libvirt/images/%s-root", name)

	// 2. Define Domain
	// Memory: 512MB
	// CPU: 1
	if networkID == "" {
		networkID = "default"
	}

	domainXML := generateDomainXML(name, diskPath, networkID, 512, 1)

	dom, err := a.conn.DomainDefineXML(domainXML)
	if err != nil {
		// Clean up volume
		_ = a.conn.StorageVolDelete(vol, 0)
		return "", fmt.Errorf("failed to define domain: %w", err)
	}

	// 3. Start Domain
	if err := a.conn.DomainCreate(dom); err != nil {
		return "", fmt.Errorf("failed to start domain: %w", err)
	}

	// Determine UUID (Name is used as ID here usually, but libvirt has UUID)
	// We return Name as ID to keep standard with arguments
	return name, nil
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
	return 0, fmt.Errorf("port forwarding not supported in libvirt adapter")
}

func (a *LibvirtAdapter) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	return "", fmt.Errorf("exec not supported in libvirt adapter")
}

func (a *LibvirtAdapter) RunTask(ctx context.Context, opts ports.RunTaskOptions) (string, error) {
	return "", fmt.Errorf("runtask not supported in libvirt adapter")
}

func (a *LibvirtAdapter) WaitTask(ctx context.Context, id string) (int64, error) {
	return 0, fmt.Errorf("waittask not supported in libvirt adapter")
}

func (a *LibvirtAdapter) CreateNetwork(ctx context.Context, name string) (string, error) {
	// Simple NAT network
	xml := generateNetworkXML(name, "virbr-"+name, "192.168.123.1", "192.168.123.2", "192.168.123.254")

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
