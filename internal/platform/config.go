// Package platform provides platform-specific configurations and utilities.
package platform

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config stores application configuration loaded from environment variables.
type Config struct {
	Port                 string
	DatabaseURL          string
	DatabaseReadURL      string
	Environment          string
	SecretsEncryptionKey string
	ComputeBackend       string
	NetworkBackend       string
	DefaultVPCCIDR       string
	NetworkPoolStart     string
	NetworkPoolEnd       string
	DBMaxConns           string
	DBMinConns           string
	RedisURL             string
	RateLimitGlobal      string
	RateLimitAuth        string
	StorageBackend       string
	// StorageSecret is the secret key used for signing presigned URLs
	StorageSecret string
	// StorageTLSEnabled enables TLS for storage node communication
	StorageTLSEnabled bool
	// StorageTLSCertFile is the path to the PEM certificate file for TLS
	StorageTLSCertFile string
	// StorageTLSKeyFile is the path to the PEM key file for TLS
	StorageTLSKeyFile string
	// WSAllowedOrigins is a comma-separated allowlist of Origin headers
	// permitted to open a WebSocket connection. Empty means deny all
	// cross-origin upgrades. See #249.
	WSAllowedOrigins string
	// DashboardAllowedOrigins is a comma-separated allowlist of Origin headers
	// permitted for SSE connections. Empty means deny all cross-origin SSE.
	// See #347.
	DashboardAllowedOrigins string
	LvmVgName               string
	ObjectStorageMode       string
	ObjectStorageNodes      string
	PowerDNSAPIURL          string
	PowerDNSAPIKey          string
	PowerDNSServerID        string
	LibvirtURI              string
	DockerDefaultNetwork    string
	FirecrackerBinary       string
	FirecrackerKernel       string
	FirecrackerRootfs       string
	FirecrackerMockMode     bool
	VaultAddress            string
	VaultToken              string
	VaultMountPath          string
	// ServiceAccountTokenTTL is the lifetime in seconds of SA JWTs. Defaults to 3600.
	ServiceAccountTokenTTL int
}

// NewConfig loads configuration from the environment with defaults.
func NewConfig() (*Config, error) {
	_ = godotenv.Load() // Ignore error if .env doesn't exist

	cfg := &Config{
		Port:                    getEnv("PORT", "8080"),
		DatabaseURL:             getEnv("DATABASE_URL", "postgres://cloud:cloud@localhost:5433/thecloud"),
		DatabaseReadURL:         getEnv("DATABASE_READ_URL", ""), // Default to empty (use primary)
		Environment:             getEnv("APP_ENV", "development"),
		SecretsEncryptionKey:    os.Getenv("SECRETS_ENCRYPTION_KEY"),
		ComputeBackend:          getEnv("COMPUTE_BACKEND", "docker"),
		NetworkBackend:          getEnv("NETWORK_BACKEND", "ovs"),
		DefaultVPCCIDR:          getEnv("DEFAULT_VPC_CIDR", "10.0.0.0/16"),
		NetworkPoolStart:        getEnv("NETWORK_POOL_START", "192.168.100.0"),
		NetworkPoolEnd:          getEnv("NETWORK_POOL_END", "192.168.200.255"),
		DBMaxConns:              getEnv("DB_MAX_CONNS", "20"),
		DBMinConns:              getEnv("DB_MIN_CONNS", "2"),
		RedisURL:                getEnv("REDIS_URL", "localhost:6379"),
		RateLimitGlobal:         getEnv("RATE_LIMIT_GLOBAL", "100"),
		RateLimitAuth:           getEnv("RATE_LIMIT_AUTH", "10"),
		StorageBackend:          getEnv("STORAGE_BACKEND", "noop"),
		StorageSecret:           os.Getenv("STORAGE_SECRET"),
		StorageTLSEnabled:       getEnv("STORAGE_TLS_ENABLED", "false") == "true",
		StorageTLSCertFile:      os.Getenv("STORAGE_TLS_CERT_FILE"),
		StorageTLSKeyFile:       os.Getenv("STORAGE_TLS_KEY_FILE"),
		WSAllowedOrigins:        os.Getenv("WS_ALLOWED_ORIGINS"),
		DashboardAllowedOrigins: os.Getenv("DASHBOARD_ALLOWED_ORIGINS"),
		LvmVgName:               getEnv("LVM_VG_NAME", "thecloud-vg"),
		ObjectStorageMode:       getEnv("OBJECT_STORAGE_MODE", "local"),

		ObjectStorageNodes:     getEnv("OBJECT_STORAGE_NODES", ""),
		PowerDNSAPIURL:         getEnv("POWERDNS_API_URL", "http://localhost:8081"),
		PowerDNSAPIKey:         os.Getenv("POWERDNS_API_KEY"),
		PowerDNSServerID:       getEnv("POWERDNS_SERVER_ID", "localhost"),
		LibvirtURI:             getEnv("LIBVIRT_URI", ""),
		DockerDefaultNetwork:   getEnv("DOCKER_DEFAULT_NETWORK", "cloud-network"),
		FirecrackerBinary:      getEnv("FIRECRACKER_BINARY", "/usr/local/bin/firecracker"),
		FirecrackerKernel:      getEnv("FIRECRACKER_KERNEL", "/var/lib/thecloud/vmlinux"),
		FirecrackerRootfs:      getEnv("FIRECRACKER_ROOTFS", "/var/lib/thecloud/rootfs.ext4"),
		FirecrackerMockMode:    getEnv("FIRECRACKER_MOCK_MODE", "false") == "true",
		VaultAddress:           getEnv("VAULT_ADDR", "http://localhost:8200"),
		VaultToken:             getEnv("VAULT_TOKEN", ""),
		VaultMountPath:         getEnv("VAULT_MOUNT_PATH", "secret/data/thecloud/rds"),
		ServiceAccountTokenTTL: getEnvInt("SERVICE_ACCOUNT_TOKEN_TTL", 3600),
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, exists := os.LookupEnv(key); exists {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return fallback
}

// GetSecretsEncryptionKey returns the secrets encryption key from environment.
// Returns empty string if not set.
func GetSecretsEncryptionKey() string {
	return os.Getenv("SECRETS_ENCRYPTION_KEY")
}

// validateConfig ensures required configuration values are present.
func validateConfig(cfg *Config) error {
	if cfg.StorageSecret == "" {
		return fmt.Errorf("STORAGE_SECRET environment variable is required")
	}
	if cfg.PowerDNSAPIKey == "" {
		return fmt.Errorf("POWERDNS_API_KEY environment variable is required")
	}
	return nil
}
