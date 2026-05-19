// Package libvirt provides Libvirt adapter implementations.
package libvirt

import (
	"context"
	"fmt"
	"math"

	"github.com/digitalocean/go-libvirt"
)

// RealLibvirtClient implements LibvirtClient using the actual libvirt library.
type RealLibvirtClient struct {
	conn *libvirt.Libvirt
}

func (r *RealLibvirtClient) Connect(ctx context.Context) error {
	// Check context before starting - if cancelled, return early
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	// r.conn.Connect() is a blocking C call; we run it synchronously since
	// it cannot be interrupted anyway. A goroutine wouldn't help cancellation.
	return r.conn.Connect()
}

func (r *RealLibvirtClient) ConnectToURI(ctx context.Context, uri string) error {
	// Check context before starting - if cancelled, return early
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	// r.conn.ConnectToURI() is a blocking C call; run it synchronously
	// since it cannot be interrupted anyway
	return r.conn.ConnectToURI(libvirt.ConnectURI(uri))
}

func (r *RealLibvirtClient) Close() error {
	return r.conn.Disconnect()
}

// Domain

func (r *RealLibvirtClient) DomainLookupByName(ctx context.Context, name string) (libvirt.Domain, error) {
	select {
	case <-ctx.Done():
		return libvirt.Domain{}, ctx.Err()
	default:
	}
	return r.conn.DomainLookupByName(name)
}

func (r *RealLibvirtClient) DomainDefineXML(ctx context.Context, xml string) (libvirt.Domain, error) {
	select {
	case <-ctx.Done():
		return libvirt.Domain{}, ctx.Err()
	default:
	}
	return r.conn.DomainDefineXML(xml)
}

func (r *RealLibvirtClient) DomainCreate(ctx context.Context, dom libvirt.Domain) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return r.conn.DomainCreate(dom)
}

func (r *RealLibvirtClient) DomainDestroy(ctx context.Context, dom libvirt.Domain) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return r.conn.DomainDestroy(dom)
}

func (r *RealLibvirtClient) DomainSuspend(ctx context.Context, dom libvirt.Domain) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return r.conn.DomainSuspend(dom)
}

func (r *RealLibvirtClient) DomainResume(ctx context.Context, dom libvirt.Domain) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return r.conn.DomainResume(dom)
}

func (r *RealLibvirtClient) DomainUndefine(ctx context.Context, dom libvirt.Domain) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return r.conn.DomainUndefine(dom)
}

func (r *RealLibvirtClient) DomainGetState(ctx context.Context, dom libvirt.Domain, flags uint32) (int32, int32, error) {
	select {
	case <-ctx.Done():
		return 0, 0, ctx.Err()
	default:
	}
	return r.conn.DomainGetState(dom, flags)
}

func (r *RealLibvirtClient) DomainGetXMLDesc(ctx context.Context, dom libvirt.Domain, flags libvirt.DomainXMLFlags) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	return r.conn.DomainGetXMLDesc(dom, flags)
}

func (r *RealLibvirtClient) DomainAttachDevice(ctx context.Context, dom libvirt.Domain, xml string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return r.conn.DomainAttachDevice(dom, xml)
}

func (r *RealLibvirtClient) DomainDetachDevice(ctx context.Context, dom libvirt.Domain, xml string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return r.conn.DomainDetachDevice(dom, xml)
}

func (r *RealLibvirtClient) DomainMemoryStats(ctx context.Context, dom libvirt.Domain, maxStats uint32, flags uint32) ([]libvirt.DomainMemoryStat, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return r.conn.DomainMemoryStats(dom, maxStats, flags)
}

func (r *RealLibvirtClient) DomainGetCPUStats(ctx context.Context, dom libvirt.Domain, nparams uint32, startCPU int32, ncpus uint32, flags libvirt.TypedParameterFlags) ([]libvirt.TypedParam, int32, error) {
	select {
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	default:
	}
	return r.conn.DomainGetCPUStats(dom, nparams, startCPU, ncpus, flags)
}

// Network

func (r *RealLibvirtClient) NetworkLookupByName(ctx context.Context, name string) (libvirt.Network, error) {
	select {
	case <-ctx.Done():
		return libvirt.Network{}, ctx.Err()
	default:
	}
	return r.conn.NetworkLookupByName(name)
}

func (r *RealLibvirtClient) NetworkDefineXML(ctx context.Context, xml string) (libvirt.Network, error) {
	select {
	case <-ctx.Done():
		return libvirt.Network{}, ctx.Err()
	default:
	}
	return r.conn.NetworkDefineXML(xml)
}

func (r *RealLibvirtClient) NetworkCreate(ctx context.Context, net libvirt.Network) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return r.conn.NetworkCreate(net)
}

func (r *RealLibvirtClient) NetworkDestroy(ctx context.Context, net libvirt.Network) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return r.conn.NetworkDestroy(net)
}

func (r *RealLibvirtClient) NetworkUndefine(ctx context.Context, net libvirt.Network) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return r.conn.NetworkUndefine(net)
}

func (r *RealLibvirtClient) NetworkGetDhcpLeases(ctx context.Context, net libvirt.Network, mac libvirt.OptString, needResults uint32, flags uint32) ([]libvirt.NetworkDhcpLease, uint32, error) {
	select {
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	default:
	}
	if needResults > math.MaxInt32 {
		return nil, 0, fmt.Errorf("needResults exceeds max allowed value: %d > %d", needResults, math.MaxInt32)
	}
	return r.conn.NetworkGetDhcpLeases(net, mac, int32(needResults), flags)
}

// Storage

func (r *RealLibvirtClient) StoragePoolLookupByName(ctx context.Context, name string) (libvirt.StoragePool, error) {
	select {
	case <-ctx.Done():
		return libvirt.StoragePool{}, ctx.Err()
	default:
	}
	return r.conn.StoragePoolLookupByName(name)
}

func (r *RealLibvirtClient) StoragePoolRefresh(ctx context.Context, pool libvirt.StoragePool, flags uint32) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return r.conn.StoragePoolRefresh(pool, flags)
}

func (r *RealLibvirtClient) StorageVolLookupByName(ctx context.Context, pool libvirt.StoragePool, name string) (libvirt.StorageVol, error) {
	select {
	case <-ctx.Done():
		return libvirt.StorageVol{}, ctx.Err()
	default:
	}
	return r.conn.StorageVolLookupByName(pool, name)
}

func (r *RealLibvirtClient) StorageVolCreateXML(ctx context.Context, pool libvirt.StoragePool, xml string, flags uint32) (libvirt.StorageVol, error) {
	select {
	case <-ctx.Done():
		return libvirt.StorageVol{}, ctx.Err()
	default:
	}
	if flags > math.MaxInt32 {
		return libvirt.StorageVol{}, fmt.Errorf("flags overflow int32: %d", flags)
	}
	return r.conn.StorageVolCreateXML(pool, xml, libvirt.StorageVolCreateFlags(int32(flags)))
}

func (r *RealLibvirtClient) StorageVolDelete(ctx context.Context, vol libvirt.StorageVol, flags uint32) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if flags > math.MaxInt32 {
		return fmt.Errorf("flags overflow int32: %d", flags)
	}
	return r.conn.StorageVolDelete(vol, libvirt.StorageVolDeleteFlags(int32(flags)))
}

func (r *RealLibvirtClient) StorageVolGetPath(ctx context.Context, vol libvirt.StorageVol) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	return r.conn.StorageVolGetPath(vol)
}
