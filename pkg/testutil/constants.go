// Package testutil provides shared constants and helpers for testing.
package testutil

import (
	"os"
)

func getEnvVar(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

var (
	// TestIPLocalhost is the loopback address used in tests.
	TestIPLocalhost = "127.0.0.1"
	// TestIPHost is a sample host IP used in tests.
	TestIPHost = "10.0.0.1"
	// TestIPPrivate is a sample private IP used in tests.
	TestIPPrivate = "192.168.1.1"
	// TestCIDR is a default CIDR block used in tests.
	TestCIDR = "10.0.0.0/16"
	// TestOtherCIDR is an alternate CIDR block used in tests.
	TestOtherCIDR = "192.168.1.0/24"
	// TestAnyCIDR represents the catch-all CIDR block.
	TestAnyCIDR = "0.0.0.0/0"
	// TestGoogleDNSCIDR is the CIDR for a public DNS IP used in tests.
	TestGoogleDNSCIDR = "8.8.8.8/32"
	// TestTenCIDR is a 10.0.0.0/8 CIDR used in tests.
	TestTenCIDR = "10.0.0.0/8"
	// TestSubnetCIDR is a sample subnet CIDR.
	TestSubnetCIDR = "10.0.1.0/24"
	// TestGatewayIP is a sample subnet gateway IP.
	TestGatewayIP = "10.0.1.1"
	// TestInstanceIP is a sample instance IP.
	TestInstanceIP = "10.0.1.2"
	// TestDockerInstanceIP is a sample Docker instance IP.
	TestDockerInstanceIP = "10.0.0.2"
	// TestNoopIP1 is a placeholder IP for noop tests.
	TestNoopIP1 = "1.1.1.1"
	// TestNoopIP2 is a placeholder IP for noop tests.
	TestNoopIP2 = "1.1.1.2"
	// TestNoopIP3 is a placeholder IP for noop tests.
	TestNoopIP3 = "1.2.3.4"
	// TestNoopIP4 is a placeholder IP for noop tests.
	TestNoopIP4 = "2.2.2.2"
	// TestLibvirtInstanceIP is a sample libvirt instance IP.
	TestLibvirtInstanceIP = "192.168.122.10"
	// TestLibvirtDHCPStart is a sample libvirt DHCP start IP.
	TestLibvirtDHCPStart = "10.0.0.2"
	// TestLibvirtDHCPEnd is a sample libvirt DHCP end IP.
	TestLibvirtDHCPEnd = "10.0.0.50"
	// TestLibvirtPoolStart is the start of the libvirt pool range.
	TestLibvirtPoolStart = "192.168.100.0"
	// TestLibvirtPoolEnd is the end of the libvirt pool range.
	TestLibvirtPoolEnd = "192.168.200.255"
	// TestVXLANRemoteIP is a sample VXLAN remote IP.
	TestVXLANRemoteIP = "192.168.1.1"
	// TestSubnet2CIDR is an alternate subnet CIDR.
	TestSubnet2CIDR = "10.0.2.0/24"

	// TestPasswordStrong is a sample strong password.
	TestPasswordStrong = "CorrectHorseBatteryStaple123!"
	// TestPasswordWeak is a sample weak password.
	TestPasswordWeak = "123"
	// TestEmail is a sample email address.
	TestEmail = "test@example.com"
	// TestUserAgent is a sample user agent string.
	TestUserAgent = "test-agent"
	// TestBaseURL is the API base URL used in tests.
	TestBaseURL = getEnvVar("TEST_BASE_URL", "http://localhost:8080")
	// TestHeaderAPIKey is the header name used for API keys.
	TestHeaderAPIKey = "X-API-Key" //#nosec G101
	// TestContentTypeAppJSON is the JSON content type used in tests.
	TestContentTypeAppJSON = "application/json"
	// TestDatabaseURL is the local database URL used in tests.
	TestDatabaseURL = getEnvVar("TEST_DATABASE_URL", "postgres://cloud:cloud@localhost:5433/thecloud")
	// TestProdDatabaseURL is a sample production database URL.
	TestProdDatabaseURL = "postgres://test:test@localhost:5432/testdb" //#nosec G101
	// TestEnvDev is the development environment name.
	TestEnvDev = "development"
	// TestEnvProd is the production environment name.
	TestEnvProd = "production"
	// TestPort is a default port used in tests.
	TestPort = getEnvVar("TEST_PORT", "8080")
	// TestProdPort is a production port used in tests.
	TestProdPort = getEnvVar("TEST_PROD_PORT", "9090")
	// TestCacheID is a default cache ID used in tests.
	TestCacheID = "cache-1"
	// TestQueueID is a default queue ID used in tests.
	TestQueueID = "queue-1"
	// TestTopicID is a default topic ID used in tests.
	TestTopicID = "topic-1"

	// TestRouteInstances is the endpoint for instances.
	TestRouteInstances = "/instances"
	// TestRouteVpcs is the endpoint for VPCs.
	TestRouteVpcs = "/vpcs"
	// TestRouteSnapshots is the endpoint for snapshots.
	TestRouteSnapshots = "/snapshots"
	// TestRouteStorageBuckets is the endpoint for storage buckets.
	TestRouteStorageBuckets = "/storage/buckets"
	// TestRouteDNSZones is the endpoint for DNS zones.
	TestRouteDNSZones = "/dns/zones"
	// TestRouteDNSRecords is the endpoint for DNS records (base).
	TestRouteDNSRecords = "/dns/records"
	// TestRouteFormat is the default template for specific resource routes.
	TestRouteFormat = "%s%s/%s"
	// TestDockerNetwork is the name of the Docker network used in tests.
	TestDockerNetwork = getEnvVar("TEST_DOCKER_NETWORK", "cloud-network")
)
