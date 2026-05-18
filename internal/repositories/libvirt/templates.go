// Package libvirt provides XML template generation for libvirt domain definitions.
package libvirt

import (
	"fmt"
	"html"
	"strings"
)

var xmlEscape = html.EscapeString

func generateVolumeXML(name string, sizeGB int, backingStorePath string) string {
	escapedName := xmlEscape(name)
	escapedBackingPath := xmlEscape(backingStorePath)

	backingXML := ""
	if backingStorePath != "" {
		backingXML = fmt.Sprintf(`
  <backingStore>
    <path>%s</path>
    <format type='qcow2'/>
  </backingStore>`, escapedBackingPath)
	}

	return fmt.Sprintf(`
<volume>
  <name>%s</name>
  <capacity unit="G">%d</capacity>
  <target>
    <format type='qcow2'/>
  </target>%s
</volume>`, escapedName, sizeGB, backingXML)
}

func generateDomainXML(name, diskPath, networkID, isoPath string, memoryMB, vcpu int, additionalDisks []string, ports []string, arch string) string {
	escapedName := xmlEscape(name)
	escapedDiskPath := xmlEscape(diskPath)
	escapedNetworkID := xmlEscape(networkID)
	escapedIsoPath := xmlEscape(isoPath)

	if networkID == "" {
		escapedNetworkID = "default"
	}
	memoryKB := memoryMB * 1024

	var isoXML string
	if isoPath != "" {
		isoXML = fmt.Sprintf(`
    <disk type='file' device='cdrom'>
      <driver name='qemu' type='raw'/>
      <source file='%s'/>
      <target dev='hda' bus='ide'/>
      <readonly/>
    </disk>`, escapedIsoPath)
	}

	additionalDisksXML := ""
	for i, dPath := range additionalDisks {
		if i >= 25 { // Limit to vd[b-z]
			break
		}
		escapedDPath := xmlEscape(dPath)
		dev := fmt.Sprintf("vd%c", 'b'+i)
		diskType := "file"
		driverType := "qcow2"
		sourceAttr := "file"

		if strings.HasPrefix(dPath, "/dev/") {
			diskType = "block"
			driverType = "raw"
			sourceAttr = "dev"
		}

		additionalDisksXML += fmt.Sprintf(`
    <disk type='%s' device='disk'>
      <driver name='qemu' type='%s'/>
      <source %s='%s'/>
      <target dev='%s' bus='virtio'/>
    </disk>`, diskType, driverType, sourceAttr, escapedDPath, dev)
	}

	qemuArgsXML := ""
	hasNetworkMapping := false
	var hostfwds []string

	for _, p := range ports {
		parts := strings.Split(p, ":")
		if len(parts) == 2 {
			// Validation is expected at the caller (adapter)
			hPort := xmlEscape(parts[0])
			cPort := xmlEscape(parts[1])
			hostfwds = append(hostfwds, fmt.Sprintf("hostfwd=tcp::%s-:%s", hPort, cPort))
			hasNetworkMapping = true
		}
	}

	if hasNetworkMapping {
		qemuArgsXML += fmt.Sprintf(`
    <qemu:arg value='-netdev'/>
    <qemu:arg value='user,id=net0,%s'/>
    <qemu:arg value='-device'/>
    <qemu:arg value='virtio-net-pci,netdev=net0,bus=pci.0,addr=0x8'/>`, strings.Join(hostfwds, ","))
	}

	if arch == "" {
		arch = "x86_64"
		if strings.Contains(isoPath, "arm64") || strings.Contains(diskPath, "arm64") {
			arch = "aarch64"
		}
	}

	machine := "pc"
	loaderXML := ""
	if arch == "aarch64" {
		machine = "virt"
		// Standard path for UEFI on Ubuntu ARM64 (AAVMF)
		loaderXML = "<loader readonly='yes' type='pflash'>/usr/share/AAVMF/AAVMF_CODE.fd</loader>"
	}

	interfaceXML := ""
	if !hasNetworkMapping {
		interfaceXML = fmt.Sprintf(`
    <interface type='network'>
      <source network='%s'/>
      <model type='virtio'/>
    </interface>`, escapedNetworkID)
	}

	return fmt.Sprintf(`
<domain type='qemu' xmlns:qemu='http://libvirt.org/schemas/domain/qemu/1.0'>
  <name>%s</name>
  <memory unit='KiB'>%d</memory>
  <vcpu placement='static'>%d</vcpu>
  <os>
    <type arch='%s' machine='%s'>hvm</type>
    %s
    <boot dev='hd'/>
  </os>
  <features>
    <acpi/>
    <apic/>
  </features>
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='%s'/>
      <target dev='vda' bus='virtio'/>
    </disk>%s%s%s
    <graphics type='vnc' port='-1' autoport='yes' listen='0.0.0.0'>
      <listen type='address' address='0.0.0.0'/>
    </graphics>
    <serial type='pty'>
      <target type='isa-serial' port='0'>
        <model name='isa-serial'/>
      </target>
    </serial>
    <console type='pty'>
      <target type='serial' port='0'/>
    </console>
    <channel type='unix'>
      <target type='virtio' name='org.qemu.guest_agent.0'/>
      <source mode='server' path='/var/lib/libvirt/qemu/channel/target/%s.0'/>
    </channel>
    <rng model='virtio'>
      <backend model='random'>/dev/urandom</backend>
    </rng>
  </devices>
  <qemu:commandline>%s
  </qemu:commandline>
</domain>`, escapedName, memoryKB, vcpu, arch, machine, loaderXML, escapedDiskPath, isoXML, additionalDisksXML, interfaceXML, escapedName, qemuArgsXML)
}

func generateNetworkXML(name, bridgeName, gatewayIP, rangeStart, rangeEnd string) string {
	escapedName := xmlEscape(name)
	escapedBridgeName := xmlEscape(bridgeName)
	escapedGatewayIP := xmlEscape(gatewayIP)
	escapedRangeStart := xmlEscape(rangeStart)
	escapedRangeEnd := xmlEscape(rangeEnd)

	return fmt.Sprintf(`
<network>
  <name>%s</name>
  <forward mode='nat'/>
  <bridge name='%s' stp='on' delay='0'/>
  <ip address='%s' netmask='255.255.255.0'>
    <dhcp>
      <range start='%s' end='%s'/>
    </dhcp>
  </ip>
</network>`, escapedName, escapedBridgeName, escapedGatewayIP, escapedRangeStart, escapedRangeEnd)
}
