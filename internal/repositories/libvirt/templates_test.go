package libvirt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateVolumeXML(t *testing.T) {
	xml := generateVolumeXML("test-vol", 20)
	assert.Contains(t, xml, "<name>test-vol</name>")
	assert.Contains(t, xml, "<capacity unit=\"G\">20</capacity>")
	assert.Contains(t, xml, "<format type='qcow2'/>")
}

func TestGenerateDomainXML(t *testing.T) {
	xml := generateDomainXML("test-vm", "/path/to/disk", "default", 1024, 2)
	assert.Contains(t, xml, "<name>test-vm</name>")
	assert.Contains(t, xml, "<memory unit='KiB'>1048576</memory>") // 1024 * 1024
	assert.Contains(t, xml, "<vcpu placement='static'>2</vcpu>")
	assert.Contains(t, xml, "<source file='/path/to/disk'/>")
	assert.Contains(t, xml, "<source network='default'/>")
}

func TestGenerateNetworkXML(t *testing.T) {
	xml := generateNetworkXML("test-net", "test-br", "10.0.0.1", "10.0.0.2", "10.0.0.50")
	assert.Contains(t, xml, "<name>test-net</name>")
	assert.Contains(t, xml, "<bridge name='test-br' stp='on' delay='0'/>")
	assert.Contains(t, xml, "<ip address='10.0.0.1'")
	assert.Contains(t, xml, "<range start='10.0.0.2' end='10.0.0.50'/>")
}
