// Package libvirt provides Libvirt adapter implementations.
package libvirt

import (
	"context"
	"io"

	"github.com/digitalocean/go-libvirt"
)

// ConsoleFlags represents flags for DomainOpenConsole.
type ConsoleFlags uint32

const (
	ConsoleForce ConsoleFlags = 1 << iota // VIR_DOMAIN_CONSOLE_FORCE
	ConsoleSafe     ConsoleFlags = 1 << iota // VIR_DOMAIN_CONSOLE_SAFE
)

// LibvirtClient defines the interface for libvirt operations.
// This allows mocking the underlying libvirt connection for testing.
type LibvirtClient interface {
	Connect(ctx context.Context) error
	ConnectToURI(ctx context.Context, uri string) error
	Close() error

	// Domain
	DomainLookupByName(ctx context.Context, name string) (libvirt.Domain, error)
	DomainDefineXML(ctx context.Context, xml string) (libvirt.Domain, error)
	DomainCreate(ctx context.Context, dom libvirt.Domain) error
	DomainDestroy(ctx context.Context, dom libvirt.Domain) error
	DomainSuspend(ctx context.Context, dom libvirt.Domain) error
	DomainResume(ctx context.Context, dom libvirt.Domain) error
	DomainUndefine(ctx context.Context, dom libvirt.Domain) error
	DomainGetState(ctx context.Context, dom libvirt.Domain, flags uint32) (int32, int32, error)
	DomainGetXMLDesc(ctx context.Context, dom libvirt.Domain, flags libvirt.DomainXMLFlags) (string, error)
	DomainAttachDevice(ctx context.Context, dom libvirt.Domain, xml string) error
	DomainDetachDevice(ctx context.Context, dom libvirt.Domain, xml string) error
	DomainMemoryStats(ctx context.Context, dom libvirt.Domain, maxStats uint32, flags uint32) ([]libvirt.DomainMemoryStat, error)
	DomainGetCPUStats(ctx context.Context, dom libvirt.Domain, nparams uint32, startCPU int32, ncpus uint32, flags libvirt.TypedParameterFlags) ([]libvirt.TypedParam, int32, error)
	DomainOpenConsole(ctx context.Context, dom libvirt.Domain, devName string, flags ConsoleFlags) (io.ReadCloser, io.WriteCloser, error)

	// Network
	NetworkLookupByName(ctx context.Context, name string) (libvirt.Network, error)
	NetworkDefineXML(ctx context.Context, xml string) (libvirt.Network, error)
	NetworkCreate(ctx context.Context, net libvirt.Network) error
	NetworkDestroy(ctx context.Context, net libvirt.Network) error
	NetworkUndefine(ctx context.Context, net libvirt.Network) error
	NetworkGetDhcpLeases(ctx context.Context, net libvirt.Network, mac libvirt.OptString, needResults uint32, flags uint32) ([]libvirt.NetworkDhcpLease, uint32, error)

	// Storage
	StoragePoolLookupByName(ctx context.Context, name string) (libvirt.StoragePool, error)
	StoragePoolRefresh(ctx context.Context, pool libvirt.StoragePool, flags uint32) error
	StorageVolLookupByName(ctx context.Context, pool libvirt.StoragePool, name string) (libvirt.StorageVol, error)
	StorageVolCreateXML(ctx context.Context, pool libvirt.StoragePool, xml string, flags uint32) (libvirt.StorageVol, error)
	StorageVolDelete(ctx context.Context, vol libvirt.StorageVol, flags uint32) error
	StorageVolGetPath(ctx context.Context, vol libvirt.StorageVol) (string, error)
}
