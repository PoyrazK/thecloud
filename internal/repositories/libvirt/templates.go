package libvirt

import (
	"fmt"
)

func generateVolumeXML(name string, sizeGB int) string {
	return fmt.Sprintf(`
<volume>
  <name>%s</name>
  <capacity unit="G">%d</capacity>
  <target>
    <format type='qcow2'/>
  </target>
</volume>`, name, sizeGB)
}

func generateDomainXML(name, diskPath, networkID string, memoryMB, vcpu int) string {
	if networkID == "" {
		networkID = "default"
	}
	// Convert MB to KB for libvirt
	memoryKB := memoryMB * 1024

	return fmt.Sprintf(`
<domain type='kvm'>
  <name>%s</name>
  <memory unit='KiB'>%d</memory>
  <vcpu placement='static'>%d</vcpu>
  <os>
    <type arch='x86_64' machine='pc-q35-4.2'>hvm</type>
    <boot dev='hd'/>
  </os>
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='%s'/>
      <target dev='vda' bus='virtio'/>
    </disk>
    <interface type='network'>
      <source network='%s'/>
      <model type='virtio'/>
    </interface>
    <serial type='pty'>
      <target port='0'/>
    </serial>
    <console type='pty'>
      <target type='serial' port='0'/>
    </console>
  </devices>
</domain>`, name, memoryKB, vcpu, diskPath, networkID)
}

func generateNetworkXML(name, bridgeName, gatewayIP, rangeStart, rangeEnd string) string {
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
</network>`, name, bridgeName, gatewayIP, rangeStart, rangeEnd)
}
