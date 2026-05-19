// Package libvirt provides a Libvirt/KVM implementation of the ComputeBackend interface.
// It enables running VMs using QEMU/KVM with features like Cloud-Init, volume management,
// snapshots, and network configuration.
package libvirt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	stdlib_errors "errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/digitalocean/go-libvirt"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/ports"
	apierrors "github.com/poyrazk/thecloud/internal/errors"
)

var domainNameSanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9-]`)

const (
	defaultPoolName   = "default"
	userDataFileName  = "user-data"
	metaDataFileName  = "meta-data"
	errGetVolumePath  = "failed to get volume path: %w"
	errDomainNotFound = "domain not found: %w"
	errPoolNotFound   = "storage pool not found: %w"
	prefixCloudInit   = "cloud-init-"

	// Domain states
	domainStateRunning = 1
	domainStatePaused  = 3
	domainStateShutoff = 5

	// Memory stat tags
	memStatTagActual = 5
	memStatTagRSS    = 6

	// typedParamULLONG is the discriminator value for uint64 in libvirt.TypedParamValue.
	// The go-libvirt library uses a struct{D uint32; I interface{}} where D==4 means ULLONG.
	typedParamULLONG = 4
)

// domainStateName returns a human-readable name for a libvirt domain state.
func domainStateName(state int32) string {
	switch state {
	case domainStateRunning:
		return "RUNNING"
	case domainStatePaused:
		return "PAUSED"
	case domainStateShutoff:
		return "SHUTOFF"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", state)
	}
}

// LibvirtAdapter implements compute backend operations using libvirt/KVM.
type LibvirtAdapter struct {
	client LibvirtClient
	logger *slog.Logger
	uri    string

	isSession    bool
	mu           sync.RWMutex
	portMappings map[string]map[string]int // instanceID -> internalPort -> hostPort

	// Network pool configuration
	networkCounter   int
	poolStart        net.IP
	poolEnd          net.IP
	ipWaitInterval   time.Duration
	taskWaitInterval time.Duration

	// OS dependencies for testability
	execCommand        func(name string, arg ...string) *exec.Cmd
	execCommandContext func(ctx context.Context, name string, arg ...string) *exec.Cmd
	lookPath           func(file string) (string, error)
	osOpen             func(name string) (*os.File, error)

	// Pre-compiled regexes for applyDomainResize
	memoryResizeRe     *regexp.Regexp
	currentMemResizeRe *regexp.Regexp
	vcpuResizeRe       *regexp.Regexp
}

func (a *LibvirtAdapter) recordPortMapping(name string, hPortStr string, cPort string) error {
	hp, err := strconv.Atoi(hPortStr)
	if err != nil {
		return fmt.Errorf("invalid host port %q: %w", hPortStr, err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.portMappings[name] == nil {
		a.portMappings[name] = make(map[string]int)
	}
	a.portMappings[name][cPort] = hp
	return nil
}

// NewLibvirtAdapter creates a LibvirtAdapter connected to the provided URI.
func NewLibvirtAdapter(logger *slog.Logger, uri string) (*LibvirtAdapter, error) {
	if uri == "" {
		uri = os.Getenv("LIBVIRT_URI")
	}
	if uri == "" {
		uri = "/var/run/libvirt/libvirt-sock"
	}

	// Connect to libvirt socket
	c, err := net.DialTimeout("unix", uri, 2*time.Second)
	if err != nil {
		// Fallback to session mode if system socket fails
		if !strings.Contains(uri, "session") {
			sessionUri := filepath.Join(os.Getenv("HOME"), ".cache/libvirt/libvirt-sock")
			if c2, err2 := net.DialTimeout("unix", sessionUri, 2*time.Second); err2 == nil {
				c = c2
				uri = sessionUri
			} else {
				return nil, fmt.Errorf("failed to dial libvirt (system and session): %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to dial libvirt: %w", err)
		}
	}

	//nolint:staticcheck
	l := libvirt.New(c)

	memoryRe := regexp.MustCompile(`(?i)<memory(?:\s[^>]*)?>\d+</memory>`)
	currentMemRe := regexp.MustCompile(`(?i)<currentMemory(?:\s[^>]*)?>\d+</currentMemory>`)
	vcpuRe := regexp.MustCompile(`(?i)<vcpu(?:\s[^>]*)?>\d+</vcpu>`)

	adapter := &LibvirtAdapter{
		client:             &RealLibvirtClient{conn: l},
		logger:             logger,
		uri:                uri,
		portMappings:       make(map[string]map[string]int),
		networkCounter:     0,
		poolStart:          net.ParseIP("192.168.100.0"),
		poolEnd:            net.ParseIP("192.168.200.255"),
		ipWaitInterval:     5 * time.Second,
		taskWaitInterval:   2 * time.Second,
		execCommand:        exec.Command,
		execCommandContext: exec.CommandContext,
		lookPath:           exec.LookPath,
		osOpen:             os.Open,
		memoryResizeRe:     memoryRe,
		currentMemResizeRe: currentMemRe,
		vcpuResizeRe:       vcpuRe,
	}

	connectCtx, connectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer connectCancel()

	// Inferred URI for the hypervisor connection
	hypervisorUri := "qemu:///session"
	adapter.isSession = true
	if strings.Contains(uri, "/var/run/libvirt/libvirt-sock") {
		hypervisorUri = "qemu:///system"
		adapter.isSession = false
	}

	if err := adapter.client.ConnectToURI(connectCtx, hypervisorUri); err != nil {
		logger.Warn("failed to connect to hypervisor URI, trying session fallback", "uri", hypervisorUri, "error", err)
		if err2 := adapter.client.ConnectToURI(connectCtx, "qemu:///session"); err2 != nil {
			return nil, fmt.Errorf("failed to connect to libvirt hypervisor: %w", err2)
		}
		adapter.isSession = true
	}

	return adapter, nil
}

// Close gracefully disconnects from libvirt
func (a *LibvirtAdapter) Close() error {
	return a.client.Close()
}

// Ping checks if libvirt is reachable
func (a *LibvirtAdapter) Ping(ctx context.Context) error {
	return a.client.Connect(ctx) // returns error only
}

func (a *LibvirtAdapter) Type() string {
	return "libvirt"
}

// PauseInstance suspends a running domain (freezes CPU, retains memory/network).
func (a *LibvirtAdapter) PauseInstance(ctx context.Context, id string) error {
	dom, err := a.client.DomainLookupByName(ctx, id)
	if err != nil {
		return fmt.Errorf(errDomainNotFound, err)
	}

	state, _, err := a.client.DomainGetState(ctx, dom, 0)
	if err != nil {
		return fmt.Errorf("failed to get domain state: %w", err)
	}
	if state != domainStateRunning {
		return fmt.Errorf("%w: domain is %s, must be RUNNING", apierrors.ErrInstanceNotPausable, domainStateName(state))
	}

	if err := a.client.DomainSuspend(ctx, dom); err != nil {
		return fmt.Errorf("failed to suspend domain: %w", err)
	}
	a.logger.Info("domain paused", "domain", id)
	return nil
}

// ResumeInstance resumes a paused domain back to running state.
func (a *LibvirtAdapter) ResumeInstance(ctx context.Context, id string) error {
	dom, err := a.client.DomainLookupByName(ctx, id)
	if err != nil {
		return fmt.Errorf(errDomainNotFound, err)
	}

	state, _, err := a.client.DomainGetState(ctx, dom, 0)
	if err != nil {
		return fmt.Errorf("failed to get domain state: %w", err)
	}
	if state != domainStatePaused {
		return fmt.Errorf("%w: domain is %s, must be PAUSED", apierrors.ErrInstanceNotResumable, domainStateName(state))
	}

	if err := a.client.DomainResume(ctx, dom); err != nil {
		return fmt.Errorf("failed to resume domain: %w", err)
	}
	a.logger.Info("domain resumed", "domain", id)
	return nil
}

func (a *LibvirtAdapter) ResizeInstance(ctx context.Context, id string, cpu, memory int64) error {
	dom, err := a.client.DomainLookupByName(ctx, id)
	if err != nil {
		return fmt.Errorf(errDomainNotFound, err)
	}

	// Cold resize: stop → update domain XML → start
	state, _, err := a.client.DomainGetState(ctx, dom, 0)
	if err != nil {
		return fmt.Errorf("failed to get domain state: %w", err)
	}

	wasRunning := state == domainStateRunning
	if wasRunning {
		if err := a.client.DomainDestroy(ctx, dom); err != nil {
			return fmt.Errorf("failed to stop domain for resize: %w", err)
		}
	}

	// Update domain XML with new memory and vCPU settings
	domXML, err := a.client.DomainGetXMLDesc(ctx, dom, 0)
	if err != nil {
		return fmt.Errorf("failed to get domain XML: %w", err)
	}

	// Create pre-resize snapshot for rollback in case of failure
	snapshotName := fmt.Sprintf("pre-resize-%s", uuid.New().String()[:8])
	if err := a.CreateSnapshot(ctx, id, snapshotName); err != nil {
		a.logger.Warn("failed to create pre-resize snapshot, proceeding without safety net", "domain", id, "error", err)
	}

	// Modify memory (in KiB) and vCPU in the XML
	newDOMXML, err := a.applyDomainResize(domXML, int(memory/1024), int(cpu/1e9))
	if err != nil {
		if rbErr := a.rollbackFromSnapshot(ctx, id, snapshotName, domXML); rbErr != nil {
			return fmt.Errorf("failed to modify domain XML (instance_id=%s): %w", id, errors.Join(err, rbErr))
		}
		return fmt.Errorf("failed to modify domain XML: %w", err)
	}

	if err := a.client.DomainUndefine(ctx, dom); err != nil {
		if rbErr := a.rollbackFromSnapshot(ctx, id, snapshotName, domXML); rbErr != nil {
			return fmt.Errorf("failed to undefine domain (instance_id=%s): %w", id, errors.Join(err, rbErr))
		}
		return fmt.Errorf("failed to undefine domain: %w", err)
	}

	newDom, err := a.client.DomainDefineXML(ctx, newDOMXML)
	if err != nil {
		// Rollback: restore volume and redefine original domain
		rollbackErr := a.rollbackFromSnapshot(ctx, id, snapshotName, domXML)
		if rollbackErr != nil {
			return fmt.Errorf("failed to redefine domain with new resources (instance_id=%s, target=%s): %w", id, newDOMXML, errors.Join(err, rollbackErr))
		}
		return fmt.Errorf("failed to redefine domain with new resources: %w", err)
	}

	if wasRunning {
		if err := a.client.DomainCreate(ctx, newDom); err != nil {
			if rbErr := a.rollbackFromSnapshot(ctx, id, snapshotName, domXML); rbErr != nil {
				return fmt.Errorf("failed to start domain after resize (instance_id=%s): %w", id, errors.Join(err, rbErr))
			}
			return fmt.Errorf("failed to start domain after resize: %w", err)
		}
	}

	// Resize succeeded — clean up the pre-resize snapshot
	if err := a.DeleteSnapshot(ctx, id, snapshotName); err != nil {
		a.logger.Warn("failed to delete pre-resize snapshot", "domain", id, "snapshot", snapshotName, "error", err)
	}

	a.logger.Info("domain resized", "domain", id, "vcpus", cpu/1e9, "memory_kib", memory/1024)
	return nil
}

// applyDomainResize updates vCPU and memory in domain XML using targeted regex replacements
// that preserve all other elements, attributes, and namespaces.
func (a *LibvirtAdapter) applyDomainResize(xmlContent string, memoryKiB, vcpus int) (string, error) {
	result := xmlContent

	// Replace <memory unit="KiB">...</memory> or <memory>...</memory>
	result = a.memoryResizeRe.ReplaceAllString(result, fmt.Sprintf(`<memory unit="KiB">%d</memory>`, memoryKiB))

	// Replace <currentMemory unit="KiB">...</currentMemory> or <currentMemory>...</currentMemory>
	result = a.currentMemResizeRe.ReplaceAllString(result, fmt.Sprintf(`<currentMemory unit="KiB">%d</currentMemory>`, memoryKiB))

	// Replace <vcpu placement="static">...</vcpu> or <vcpu>...</vcpu>
	result = a.vcpuResizeRe.ReplaceAllString(result, fmt.Sprintf(`<vcpu>%d</vcpu>`, vcpus))

	// Verify we actually made replacements
	if result == xmlContent {
		return "", fmt.Errorf("no memory or vcpu elements found in domain XML to modify")
	}

	return result, nil
}

func (a *LibvirtAdapter) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (string, []string, error) {
	name := a.sanitizeDomainName(opts.Name)

	diskPath, vol, err := a.prepareRootVolume(ctx, name, opts.ImageName)
	if err != nil {
		return "", nil, err
	}

	isoPath := a.prepareCloudInit(ctx, name, opts.Env, opts.Cmd, opts.UserData)
	additionalDisks := a.resolveBinds(ctx, opts.VolumeBinds)

	networkID := opts.NetworkID
	if networkID == "" {
		networkID = "default"
	}

	allocatedPorts := make([]string, 0, len(opts.Ports))
	for _, p := range opts.Ports {
		parts := strings.Split(p, ":")
		if len(parts) == 2 {
			hPort := parts[0]
			cPort := parts[1]
			if hPort == "0" {
				freePort, err := findFreePort()
				if err != nil {
					a.logger.Warn("failed to auto-allocate port", "container_port", cPort, "error", err)
					continue
				}
				hPort = strconv.Itoa(freePort)
			}
			if err := a.recordPortMapping(name, hPort, cPort); err != nil {
				a.logger.Warn("failed to record port mapping", "port", p, "error", err)
				continue
			}
			allocatedPorts = append(allocatedPorts, fmt.Sprintf("%s:%s", hPort, cPort))
		} else if len(parts) == 1 {
			cPort := parts[0]
			freePort, err := findFreePort()
			if err != nil {
				a.logger.Warn("failed to auto-allocate port", "container_port", cPort, "error", err)
				continue
			}
			hPort := strconv.Itoa(freePort)
			if err := a.recordPortMapping(name, hPort, cPort); err != nil {
				a.logger.Warn("failed to record port mapping", "port", p, "error", err)
				continue
			}
			allocatedPorts = append(allocatedPorts, fmt.Sprintf("%s:%s", hPort, cPort))
		}
	}
	opts.Ports = allocatedPorts

	// Use user-specified limits or defaults
	memMB := 512
	if opts.MemoryLimit > 0 {
		memMB = int(opts.MemoryLimit / 1024 / 1024)
	}
	vcpu := 1
	if opts.CPULimit > 0 {
		vcpu = int(opts.CPULimit)
	}

	arch := ""
	if os.Getenv("GOARCH") == "arm64" || strings.Contains(diskPath, "arm64") {
		arch = "aarch64"
	}
	domainXML := generateDomainXML(name, diskPath, networkID, isoPath, memMB, vcpu, additionalDisks, allocatedPorts, arch)
	dom, err := a.client.DomainDefineXML(ctx, domainXML)
	if err != nil {
		a.cleanupCreateFailure(ctx, vol, isoPath)
		return "", nil, fmt.Errorf("failed to define domain: %w", err)
	}

	if err := a.client.DomainCreate(ctx, dom); err != nil {
		_ = a.client.DomainUndefine(ctx, dom)
		_ = a.client.StorageVolDelete(ctx, vol, 0)
		return "", nil, fmt.Errorf("failed to start domain: %w", err)
	}

	if len(allocatedPorts) > 0 {
		go a.setupPortForwarding(name, allocatedPorts)
	}

	return name, allocatedPorts, nil
}

func (a *LibvirtAdapter) sanitizeDomainName(name string) string {
	name = domainNameSanitizeRe.ReplaceAllString(name, "")
	if name == "" {
		return uuid.New().String()[:8]
	}
	return name
}

func (a *LibvirtAdapter) prepareCloudInit(ctx context.Context, name string, env []string, cmd []string, userData string) string {
	if len(env) == 0 && len(cmd) == 0 && userData == "" {
		return ""
	}
	isoPath, err := a.generateCloudInitISO(ctx, name, env, cmd, userData)
	if err != nil {
		a.logger.Warn("failed to generate cloud-init iso, proceeding without it", "error", err)
		return ""
	}
	return isoPath
}

func (a *LibvirtAdapter) cleanupCreateFailure(ctx context.Context, vol libvirt.StorageVol, isoPath string) {
	_ = a.client.StorageVolDelete(ctx, vol, 0)
	if isoPath != "" {
		if !strings.HasPrefix(filepath.Clean(isoPath), filepath.Clean(os.TempDir())) || !strings.Contains(filepath.Base(isoPath), prefixCloudInit) {
			a.logger.Warn("skipping removal of non-temp ISO path for security", "path", isoPath)
			return
		}
		if err := os.Remove(isoPath); err != nil {
			a.logger.Warn("failed to remove ISO", "path", isoPath, "error", err)
		}
	}
}

func (a *LibvirtAdapter) waitInitialIP(ctx context.Context, id string) (string, error) {
	ticker := time.NewTicker(a.ipWaitInterval)
	defer ticker.Stop()

	// Safety limit: max 5 minutes regardless of context
	timeout := time.NewTimer(5 * time.Minute)
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout.C:
			return "", fmt.Errorf("timed out waiting for IP for instance %s", id)
		case <-ticker.C:
			ip, err := a.GetInstanceIP(ctx, id)
			if err == nil && ip != "" {
				return ip, nil
			}
		}
	}
}

func (a *LibvirtAdapter) StartInstance(ctx context.Context, id string) error {
	dom, err := a.client.DomainLookupByName(ctx, id)
	if err != nil {
		return fmt.Errorf(errDomainNotFound, err)
	}

	if err := a.client.DomainCreate(ctx, dom); err != nil {
		return fmt.Errorf("failed to start domain: %w", err)
	}
	return nil
}

func (a *LibvirtAdapter) StopInstance(ctx context.Context, id string) error {
	dom, err := a.client.DomainLookupByName(ctx, id)
	if err != nil {
		return fmt.Errorf(errDomainNotFound, err)
	}

	if err := a.client.DomainDestroy(ctx, dom); err != nil {
		return fmt.Errorf("failed to destroy domain: %w", err)
	}
	return nil
}

func (a *LibvirtAdapter) DeleteInstance(ctx context.Context, id string) error {
	dom, err := a.client.DomainLookupByName(ctx, id)
	if err != nil {
		if a.isNotFound(err) {
			return nil
		}
		return err
	}

	a.stopDomainIfRunning(ctx, dom)

	if err := a.client.DomainUndefine(ctx, dom); err != nil {
		return fmt.Errorf("failed to undefine domain: %w", err)
	}

	a.cleanupPortMappings(id)
	a.cleanupDomainISO(id)
	a.cleanupRootVolume(ctx, id)

	return nil
}

func (a *LibvirtAdapter) stopDomainIfRunning(ctx context.Context, dom libvirt.Domain) {
	state, _, err := a.client.DomainGetState(ctx, dom, 0)
	if err == nil && state == domainStateRunning {
		_ = a.client.DomainDestroy(ctx, dom)
	}
}

func (a *LibvirtAdapter) cleanupPortMappings(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.portMappings, id)
}

func (a *LibvirtAdapter) cleanupDomainISO(id string) {
	if err := validateID(id); err != nil {
		a.logger.Warn("invalid id for iso cleanup", "id", id)
		return
	}
	isoPath := filepath.Join(os.TempDir(), prefixCloudInit+id+".iso")
	_ = os.Remove(isoPath)
}

func (a *LibvirtAdapter) cleanupRootVolume(ctx context.Context, id string) {
	pool, err := a.client.StoragePoolLookupByName(ctx, defaultPoolName)
	if err != nil {
		return
	}
	volName := id + "-root"
	vol, err := a.client.StorageVolLookupByName(ctx, pool, volName)
	if err == nil {
		_ = a.client.StorageVolDelete(ctx, vol, 0)
	}
}

func (a *LibvirtAdapter) GetInstanceLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	logPaths := []string{
		fmt.Sprintf("/tmp/%s-console.log", id),
		fmt.Sprintf("/tmp/%s-qemu.log", id),
		fmt.Sprintf("/var/log/libvirt/qemu/%s.log", id),
		filepath.Join(os.Getenv("HOME"), ".cache/libvirt/qemu/log", id+".log"),
	}

	var f *os.File
	var err error
	for _, logPath := range logPaths {
		f, err = a.osOpen(logPath)
		if err == nil {
			return f, nil
		}
	}

	return nil, fmt.Errorf("failed to open log file in any known location: %w", err)
}

func (a *LibvirtAdapter) GetInstanceStats(ctx context.Context, id string) (io.ReadCloser, error) {
	dom, err := a.client.DomainLookupByName(ctx, id)
	if err != nil {
		return nil, fmt.Errorf(errDomainNotFound, err)
	}

	memStats, err := a.client.DomainMemoryStats(ctx, dom, 10, 0)
	if err != nil {
		return nil, err
	}

	var usage, limit uint64
	for _, stat := range memStats {
		if stat.Tag == memStatTagRSS {
			usage = stat.Val * 1024 // KB to Bytes
		}
		if stat.Tag == memStatTagActual {
			limit = stat.Val * 1024
		}
	}

	// Get CPU stats via DomainGetCPUStats
	cpuParams, _, err := a.client.DomainGetCPUStats(ctx, dom, 0, 0, 0, 0)
	var cpuTime uint64
	if err != nil {
		return nil, fmt.Errorf("DomainGetCPUStats failed: %w", err)
	}
	for _, p := range cpuParams {
		if p.Field == "cpu_time" && p.Value.D == typedParamULLONG {
			if v, ok := p.Value.I.(uint64); ok {
				cpuTime = v
			}
		}
	}

	statJSON, err := json.Marshal(map[string]interface{}{
		"memory_stats": map[string]uint64{
			"usage": usage,
			"limit": limit,
		},
		"cpu_stats": map[string]uint64{
			"cpu_time": cpuTime,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal stats: %w", err)
	}
	return io.NopCloser(bytes.NewReader(statJSON)), nil
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

func (a *LibvirtAdapter) AttachVolume(ctx context.Context, id string, volumePath string) (string, string, error) {
	dom, err := a.client.DomainLookupByName(ctx, id)
	if err != nil {
		return "", "", fmt.Errorf(errDomainNotFound, err)
	}

	dev := "vdb"

	diskType := "file"
	driverType := "qcow2"
	sourceAttr := "file"

	if strings.HasPrefix(volumePath, "/dev/") {
		diskType = "block"
		driverType = "raw"
		sourceAttr = "dev"
	}

	xml := fmt.Sprintf(`
    <disk type='%s' device='disk'>
      <driver name='qemu' type='%s'/>
      <source %s='%s'/>
      <target dev='%s' bus='virtio'/>
    </disk>`, diskType, driverType, sourceAttr, volumePath, dev)

	if err := a.client.DomainAttachDevice(ctx, dom, xml); err != nil {
		return "", "", err
	}
	return "/dev/" + dev, "", nil
}

func (a *LibvirtAdapter) DetachVolume(ctx context.Context, id string, volumePath string) (string, error) {
	dom, err := a.client.DomainLookupByName(ctx, id)
	if err != nil {
		return "", fmt.Errorf(errDomainNotFound, err)
	}

	xml := fmt.Sprintf("<disk type='file' device='disk'><source file='%s'/><target dev='vdb' bus='virtio'/></disk>", volumePath)
	if strings.HasPrefix(volumePath, "/dev/") {
		xml = fmt.Sprintf("<disk type='block' device='disk'><source dev='%s'/><target dev='vdb' bus='virtio'/></disk>", volumePath)
	}

	return "", a.client.DomainDetachDevice(ctx, dom, xml)
}

func (a *LibvirtAdapter) GetConsoleURL(ctx context.Context, id string) (string, error) {
	if err := validateID(id); err != nil {
		return "", err
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	domain, err := a.client.DomainLookupByName(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to lookup domain: %w", err)
	}

	xml, err := a.client.DomainGetXMLDesc(ctx, domain, 0)
	if err != nil {
		return "", fmt.Errorf("failed to get domain xml: %w", err)
	}

	startIdx := strings.Index(xml, "<graphics type='vnc' port='")
	if startIdx == -1 {
		return "", fmt.Errorf("vnc graphics not found in domain xml")
	}
	xml = xml[startIdx+len("<graphics type='vnc' port='"):]
	endIdx := strings.Index(xml, "'")
	if endIdx == -1 {
		return "", fmt.Errorf("failed to parse vnc port from xml")
	}

	portStr := xml[:endIdx]
	return fmt.Sprintf("vnc://%s:%s", "127.0.0.1", portStr), nil
}

func (a *LibvirtAdapter) GetInstanceIP(ctx context.Context, id string) (string, error) {
	dom, err := a.client.DomainLookupByName(ctx, id)
	if err != nil {
		return "", fmt.Errorf(errDomainNotFound, err)
	}

	xmlDesc, err := a.client.DomainGetXMLDesc(ctx, dom, 0)
	if err != nil {
		return "", fmt.Errorf("failed to get domain xml: %w", err)
	}

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

	net, err := a.client.NetworkLookupByName(ctx, "default")
	if err != nil {
		return "", fmt.Errorf("default network not found: %w", err)
	}

	leases, _, err := a.client.NetworkGetDhcpLeases(ctx, net, nil, 0, 0)
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

func (a *LibvirtAdapter) RunTask(ctx context.Context, opts ports.RunTaskOptions) (string, []string, error) {
	name := "task-" + uuid.New().String()[:8]

	pool, err := a.client.StoragePoolLookupByName(ctx, defaultPoolName)
	if err != nil {
		return "", nil, fmt.Errorf("failed to find default pool: %w", err)
	}

	volXML := generateVolumeXML(name+"-root", 1, "")
	vol, err := a.client.StorageVolCreateXML(ctx, pool, volXML, 0)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create root volume: %w", err)
	}

	diskPath, err := a.client.StorageVolGetPath(ctx, vol)
	if err != nil {
		_ = a.client.StorageVolDelete(ctx, vol, 0)
		return "", nil, fmt.Errorf(errGetVolumePath, err)
	}

	isoPath := a.prepareCloudInit(ctx, name, nil, opts.Command, "")
	additionalDisks := a.resolveBinds(ctx, opts.Binds)
	domainXML := generateDomainXML(name, diskPath, "default", isoPath, int(opts.MemoryMB), 1, additionalDisks, nil, "")

	dom, err := a.client.DomainDefineXML(ctx, domainXML)
	if err != nil {
		a.cleanupCreateFailure(ctx, vol, isoPath)
		return "", nil, fmt.Errorf("failed to define domain: %w", err)
	}

	if err := a.client.DomainCreate(ctx, dom); err != nil {
		_ = a.client.DomainUndefine(ctx, dom)
		_ = a.client.StorageVolDelete(ctx, vol, 0)
		return "", nil, fmt.Errorf("failed to start domain: %w", err)
	}

	return name, nil, nil
}

func (a *LibvirtAdapter) WaitTask(ctx context.Context, id string) (int64, error) {
	interval := a.taskWaitInterval
	if interval == 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return -1, ctx.Err()
		case <-ticker.C:
			dom, err := a.client.DomainLookupByName(ctx, id)
			if err != nil {
				return -1, fmt.Errorf(errDomainNotFound, err)
			}

			state, _, err := a.client.DomainGetState(ctx, dom, 0)
			if err != nil {
				continue
			}

			if state == domainStateShutoff {
				return 0, nil
			}
		}
	}
}

func (a *LibvirtAdapter) CreateNetwork(ctx context.Context, name string) (string, error) {
	gateway, rangeStart, rangeEnd := a.getNextNetworkRange()
	xml := generateNetworkXML(name, "virbr-"+name, gateway, rangeStart, rangeEnd)

	net, err := a.client.NetworkDefineXML(ctx, xml)
	if err != nil {
		return "", fmt.Errorf("failed to define network: %w", err)
	}

	if err := a.client.NetworkCreate(ctx, net); err != nil {
		return "", fmt.Errorf("failed to start network: %w", err)
	}

	return net.Name, nil
}

func (a *LibvirtAdapter) DeleteNetwork(ctx context.Context, id string) error {
	net, err := a.client.NetworkLookupByName(ctx, id)
	if err != nil {
		if a.isNotFound(err) {
			return nil
		}
		return err
	}

	if err := a.client.NetworkDestroy(ctx, net); err != nil {
		a.logger.Warn("failed to destroy network", "error", err)
	}
	if err := a.client.NetworkUndefine(ctx, net); err != nil {
		return fmt.Errorf("failed to undefine network: %w", err)
	}
	return nil
}

func (a *LibvirtAdapter) CreateVolume(ctx context.Context, name string) error {
	pool, err := a.client.StoragePoolLookupByName(ctx, defaultPoolName)
	if err != nil {
		return fmt.Errorf(errPoolNotFound, err)
	}

	xml := generateVolumeXML(name, 10, "")

	if err := a.client.StoragePoolRefresh(ctx, pool, 0); err != nil {
		a.logger.Warn("failed to refresh pool", "error", err)
	}

	_, err = a.client.StorageVolCreateXML(ctx, pool, xml, 0)
	if err != nil {
		return fmt.Errorf("failed to create volume xml: %w", err)
	}
	return nil
}

func (a *LibvirtAdapter) DeleteVolume(ctx context.Context, name string) error {
	pool, err := a.client.StoragePoolLookupByName(ctx, defaultPoolName)
	if err != nil {
		return fmt.Errorf(errPoolNotFound, err)
	}

	vol, err := a.client.StorageVolLookupByName(ctx, pool, name)
	if err != nil {
		if a.isNotFound(err) {
			return nil
		}
		return err
	}

	if err := a.client.StorageVolDelete(ctx, vol, 0); err != nil {
		return fmt.Errorf("failed to delete volume: %w", err)
	}
	return nil
}

// snapshotPath returns the path to the snapshot archive for the given instance and snapshot name.
func snapshotPath(id, name string) string {
	return fmt.Sprintf("/tmp/snapshot-%s-%s.tar.gz", id, name)
}

// CreateSnapshot creates a point-in-time snapshot of the instance's root disk.
// The snapshot is stored at a temp path derived from the snapshot name.
func (a *LibvirtAdapter) CreateSnapshot(ctx context.Context, id, name string) error {
	dom, err := a.client.DomainLookupByName(ctx, id)
	if err != nil {
		return fmt.Errorf(errDomainNotFound, err)
	}
	_ = dom // verified existence

	volName := id + "-root"
	snapshotPath := snapshotPath(id, name)

	return a.CreateVolumeSnapshot(ctx, volName, snapshotPath)
}

// RestoreSnapshot restores the instance's root disk from a previously created snapshot.
func (a *LibvirtAdapter) RestoreSnapshot(ctx context.Context, id, name string) error {
	dom, err := a.client.DomainLookupByName(ctx, id)
	if err != nil {
		return fmt.Errorf(errDomainNotFound, err)
	}
	_ = dom // verified existence

	volName := id + "-root"
	snapshotPath := snapshotPath(id, name)

	return a.RestoreVolumeSnapshot(ctx, volName, snapshotPath)
}

// DeleteSnapshot deletes a previously created snapshot archive.
func (a *LibvirtAdapter) DeleteSnapshot(ctx context.Context, id, name string) error {
	dom, err := a.client.DomainLookupByName(ctx, id)
	if err != nil {
		return fmt.Errorf(errDomainNotFound, err)
	}
	_ = dom // verified existence

	snapshotPath := snapshotPath(id, name)
	if err := os.Remove(snapshotPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}
	return nil
}

// rollbackFromSnapshot restores the root volume from a snapshot and redefines the domain
// with the original XML. This is used when a resize operation fails after DomainUndefine.
func (a *LibvirtAdapter) rollbackFromSnapshot(ctx context.Context, id, snapshotName, originalXML string) error {
	if err := a.RestoreSnapshot(ctx, id, snapshotName); err != nil {
		return fmt.Errorf("failed to restore snapshot volume: %w", err)
	}
	// Redefine the domain so it can boot from the restored volume
	if _, err := a.client.DomainDefineXML(ctx, originalXML); err != nil {
		return fmt.Errorf("failed to redefine domain after snapshot restore: %w", err)
	}
	a.logger.Info("rollback from snapshot complete", "domain", id, "snapshot", snapshotName)
	return nil
}

func (a *LibvirtAdapter) prepareRootVolume(ctx context.Context, name string, imageName string) (string, libvirt.StorageVol, error) {
	pool, err := a.client.StoragePoolLookupByName(ctx, defaultPoolName)
	if err != nil {
		return "", libvirt.StorageVol{}, fmt.Errorf("default pool not found: %w", err)
	}

	var backingXML string
	if imageName != "" {
		backingVol, err := a.client.StorageVolLookupByName(ctx, pool, imageName)
		if err == nil {
			backingPath, err := a.client.StorageVolGetPath(ctx, backingVol)
			if err == nil {
				backingXML = backingPath
			}
		} else {
			a.logger.Warn("backing image not found, creating empty volume", "image", imageName)
		}
	}

	volXML := generateVolumeXML(name+"-root", 10, backingXML)

	vol, err := a.client.StorageVolCreateXML(ctx, pool, volXML, 0)
	if err != nil {
		return "", libvirt.StorageVol{}, fmt.Errorf("failed to create root volume: %w", err)
	}

	diskPath, err := a.client.StorageVolGetPath(ctx, vol)
	if err != nil {
		_ = a.client.StorageVolDelete(ctx, vol, 0)
		return "", libvirt.StorageVol{}, fmt.Errorf(errGetVolumePath, err)
	}

	return diskPath, vol, nil
}

func (a *LibvirtAdapter) CreateVolumeSnapshot(ctx context.Context, volumeID string, destinationPath string) error {
	pool, err := a.client.StoragePoolLookupByName(ctx, defaultPoolName)
	if err != nil {
		return fmt.Errorf(errPoolNotFound, err)
	}

	vol, err := a.client.StorageVolLookupByName(ctx, pool, volumeID)
	if err != nil {
		return fmt.Errorf("failed to find volume: %w", err)
	}

	volPath, err := a.client.StorageVolGetPath(ctx, vol)
	if err != nil {
		return fmt.Errorf(errGetVolumePath, err)
	}

	tmpQcow2 := destinationPath + ".qcow2"
	cmd := a.execCommand("qemu-img", "convert", "-O", "qcow2", volPath, tmpQcow2)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("qemu-img convert failed: %w", err)
	}
	defer func() {
		if err := os.Remove(tmpQcow2); err != nil {
			a.logger.Warn("failed to remove temp file", "path", tmpQcow2, "error", err)
		}
	}()

	tarCmd := a.execCommand("tar", "czf", destinationPath, "-C", filepath.Dir(tmpQcow2), filepath.Base(tmpQcow2))
	if err := tarCmd.Run(); err != nil {
		return fmt.Errorf("tar archive failed: %w", err)
	}

	return nil
}

func (a *LibvirtAdapter) RestoreVolumeSnapshot(ctx context.Context, volumeID string, sourcePath string) error {
	pool, err := a.client.StoragePoolLookupByName(ctx, defaultPoolName)
	if err != nil {
		return fmt.Errorf(errPoolNotFound, err)
	}

	vol, err := a.client.StorageVolLookupByName(ctx, pool, volumeID)
	if err != nil {
		return fmt.Errorf("failed to find volume: %w", err)
	}

	volPath, err := a.client.StorageVolGetPath(ctx, vol)
	if err != nil {
		return fmt.Errorf(errGetVolumePath, err)
	}

	tmpDir, err := os.MkdirTemp("", "restore-")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			a.logger.Warn("failed to remove temp dir", "path", tmpDir, "error", err)
		}
	}()

	untarCmd := a.execCommand("tar", "xzf", sourcePath, "-C", tmpDir)
	if err := untarCmd.Run(); err != nil {
		return fmt.Errorf("untar failed: %w", err)
	}

	files, _ := os.ReadDir(tmpDir)
	if len(files) == 0 {
		return fmt.Errorf("empty snapshot archive")
	}
	tmpQcow2 := filepath.Join(tmpDir, files[0].Name())

	cmd := a.execCommand("qemu-img", "convert", "-O", "qcow2", tmpQcow2, volPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("qemu-img restore failed: %w", err)
	}

	return nil
}

func (a *LibvirtAdapter) generateCloudInitISO(_ context.Context, name string, env []string, cmd []string, userData string) (string, error) {
	safeName := a.sanitizeDomainName(name)
	safeName = filepath.Base(safeName)
	if safeName == "." || safeName == ".." || safeName == "/" {
		safeName = fmt.Sprintf("cloudinit-%d", time.Now().UnixNano())
	}

	tmpDir, err := os.MkdirTemp(os.TempDir(), "thecloud-cloudinit-*")
	if err != nil {
		return "", err
	}
	defer a.cleanupTempDir(tmpDir)

	if err := a.writeCloudInitFiles(tmpDir, name, env, cmd, userData); err != nil {
		return "", err
	}

	isoPath := filepath.Join(os.TempDir(), filepath.Clean(prefixCloudInit+safeName+".iso"))
	return isoPath, a.runIsoCommand(isoPath, tmpDir)
}

func (a *LibvirtAdapter) cleanupTempDir(tmpDir string) {
	if err := os.RemoveAll(tmpDir); err != nil {
		a.logger.Warn("failed to remove temp dir", "path", tmpDir, "error", err)
	}
}

func (a *LibvirtAdapter) writeCloudInitFiles(tmpDir, name string, env, cmd []string, userDataRaw string) error {
	metaData := map[string]string{
		"instance-id":    name,
		"local-hostname": name,
	}
	metaDataBytes, err := json.Marshal(metaData)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, metaDataFileName), metaDataBytes, 0600); err != nil {
		return err
	}

	userData := a.generateUserData(env, cmd, userDataRaw)
	return os.WriteFile(filepath.Join(tmpDir, userDataFileName), userData, 0600)
}

func (a *LibvirtAdapter) generateUserData(env, cmd []string, userDataRaw string) []byte {
	var userData bytes.Buffer
	if userDataRaw != "" {
		userData.WriteString(userDataRaw)
		if !strings.HasSuffix(userDataRaw, "\n") {
			userData.WriteString("\n")
		}
	} else {
		userData.WriteString("#cloud-config\n")
	}

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
	return userData.Bytes()
}

func (a *LibvirtAdapter) runIsoCommand(isoPath, tmpDir string) error {
	var tool string
	if p, err := a.lookPath("mkisofs"); err == nil {
		tool = p
	} else if p, err := a.lookPath("genisoimage"); err == nil {
		tool = p
	} else {
		return fmt.Errorf("neither mkisofs nor genisoimage found in PATH")
	}

	genCmd := a.execCommand(tool, "-output", isoPath, "-volid", "cidata", "-l", "-R", "-J", tmpDir)
	if output, err := genCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to generate iso using %s: %w (output: %s)", tool, err, string(output))
	}
	return nil
}

func (a *LibvirtAdapter) getNextNetworkRange() (gateway, rangeStart, rangeEnd string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	baseIP := make(net.IP, len(a.poolStart))
	copy(baseIP, a.poolStart)

	offset := a.networkCounter * 256
	for i := len(baseIP) - 1; i >= 0 && offset > 0; i-- {
		sum := int(baseIP[i]) + offset
		baseIP[i] = byte(sum % 256)
		offset = sum / 256
	}

	a.networkCounter++

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

func (a *LibvirtAdapter) setupPortForwarding(name string, ports []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ip, err := a.waitInitialIP(ctx, name)
	if err != nil {
		a.logger.Error("failed to get ip for port forwarding", "instance", name, "error", err)
		return
	}

	a.logger.Info("setting up port mapping record", "instance", name, "ip", ip, "ports", ports)

	for _, p := range ports {
		hP, cP, err := a.parseAndValidatePort(p)
		if err != nil {
			a.logger.Warn("skipping invalid port forwarding configuration", "port", p, "error", err)
			continue
		}

		if hP > 0 {
			a.configureIptables(name, ip, strconv.Itoa(cP), hP)
		}
	}
}

func (a *LibvirtAdapter) configureIptables(name, ip, containerPort string, hPort int) {
	a.mu.Lock()
	if a.portMappings[name] == nil {
		a.portMappings[name] = make(map[string]int)
	}
	a.portMappings[name][containerPort] = hPort
	a.mu.Unlock()

	a.logger.Info("setting up port forwarding record", "host", hPort, "vm", containerPort, "ip", ip)
}

func (a *LibvirtAdapter) parseAndValidatePort(p string) (int, int, error) {
	parts := strings.Split(p, ":")
	var hostPort, containerPort string
	switch len(parts) {
	case 2:
		hostPort = parts[0]
		containerPort = parts[1]
	case 1:
		hostPort = "0"
		containerPort = parts[0]
	default:
		return 0, 0, fmt.Errorf("invalid port format: too many colons")
	}

	var hP, cP int
	_, errH := fmt.Sscanf(hostPort, "%d", &hP)
	_, errC := fmt.Sscanf(containerPort, "%d", &cP)
	if (hostPort != "0" && errH != nil) || errC != nil {
		return 0, 0, fmt.Errorf("invalid port format")
	}

	hPort := 0
	if hostPort == "0" {
		freePort, err := findFreePort()
		if err != nil {
			return 0, 0, fmt.Errorf("failed to find free port: %w", err)
		}
		hPort = freePort
	} else {
		hPort = hP
	}

	return hPort, cP, nil
}

func (a *LibvirtAdapter) resolveBinds(ctx context.Context, volumeBinds []string) []string {
	var additionalDisks []string
	if len(volumeBinds) == 0 {
		return additionalDisks
	}

	pool, poolErr := a.client.StoragePoolLookupByName(ctx, defaultPoolName)

	for _, bind := range volumeBinds {
		parts := strings.Split(bind, ":")
		volName := parts[0]

		if path := a.resolveVolumePath(ctx, volName, pool, poolErr); path != "" {
			additionalDisks = append(additionalDisks, path)
		}
	}
	return additionalDisks
}

func (a *LibvirtAdapter) resolveVolumePath(ctx context.Context, volName string, pool libvirt.StoragePool, poolErr error) string {
	if strings.HasPrefix(volName, "/dev/") {
		return volName
	}

	if poolErr == nil {
		v, err := a.client.StorageVolLookupByName(ctx, pool, volName)
		if err == nil {
			path, err := a.client.StorageVolGetPath(ctx, v)
			if err == nil {
				return path
			}
		}
	}

	if strings.HasPrefix(volName, "/") {
		if _, statErr := os.Stat(volName); statErr == nil {
			return volName
		}
	}
	return ""
}

func findFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	tcpAddr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		return 0, stdlib_errors.New("failed to get TCP address")
	}
	return tcpAddr.Port, nil
}

func (a *LibvirtAdapter) isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var libvirtErr libvirt.Error
	if stdlib_errors.As(err, &libvirtErr) {
		// 42: Domain not found, 43: Network not found, 45: Storage vol not found
		return libvirtErr.Code == 42 || libvirtErr.Code == 43 || libvirtErr.Code == 45
	}
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}

// ResetCircuitBreaker is a no-op for the raw Libvirt adapter.
// The circuit breaker lives in ResilientCompute wrapping this backend.
func (a *LibvirtAdapter) ResetCircuitBreaker() {}
