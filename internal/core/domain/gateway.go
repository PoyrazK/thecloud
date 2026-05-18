// Package domain defines core business entities.
package domain

import (
	"net"
	"time"

	"github.com/google/uuid"
)

// GatewayRoute defines an ingress rule for mapping external HTTP traffic to internal resources.
type GatewayRoute struct {
	ID                      uuid.UUID    `json:"id"`
	UserID                  uuid.UUID    `json:"user_id"`
	TenantID                uuid.UUID    `json:"tenant_id"`
	Name                    string       `json:"name"`
	PathPrefix              string       `json:"path_prefix"`                         // Legacy: Request path to match (e.g., "/api/v1")
	PathPattern             string       `json:"path_pattern"`                        // New: Pattern with {params}
	PatternType             string       `json:"pattern_type"`                        // "prefix" or "pattern"
	ParamNames              []string     `json:"param_names"`                         // Extracted parameter names
	TargetURL               string       `json:"target_url"`                          // Internal destination (e.g., "http://service-a:8080")
	Methods                 []string     `json:"methods"`                             // New: HTTP methods to match (empty = all)
	StripPrefix             bool         `json:"strip_prefix"`                        // If true, removes path_prefix from request before forwarding
	RateLimit               int          `json:"rate_limit"`                          // Maximum allowed requests per second per IP
	DialTimeout             int64        `json:"dial_timeout,omitempty"`              // TCP dial timeout in milliseconds
	ResponseHeaderTimeout   int64        `json:"response_header_timeout,omitempty"`   // Time to receive headers in milliseconds
	IdleConnTimeout         int64        `json:"idle_conn_timeout,omitempty"`         // Idle connection timeout in milliseconds
	TLSSkipVerify           bool         `json:"tls_skip_verify,omitempty"`           // Skip TLS verification for backend
	RequireTLS              bool         `json:"require_tls,omitempty"`               // Force HTTPS for backend
	AllowedCIDRs            []string     `json:"allowed_cidrs,omitempty"`             // IPs allowed to access (empty = all)
	AllowedIPNets           []*net.IPNet `json:"-"`                                   // pre-parsed at creation/refresh for fast lookup
	BlockedCIDRs            []string     `json:"blocked_cidrs,omitempty"`             // IPs blocked from access
	BlockedIPNets           []*net.IPNet `json:"-"`                                   // pre-parsed at creation/refresh for fast lookup
	MaxBodySize             int64        `json:"max_body_size,omitempty"`             // Max request body size in bytes
	CircuitBreakerThreshold int          `json:"circuit_breaker_threshold,omitempty"` // consecutive failures to trip open (0=disabled)
	CircuitBreakerTimeout   int64        `json:"circuit_breaker_timeout,omitempty"`   // ms in open before half-open
	MaxRetries              int          `json:"max_retries,omitempty"`               // max retry attempts (0=disabled)
	RetryTimeout            int64        `json:"retry_timeout,omitempty"`             // total retry window in ms
	Priority                int          `json:"priority"`                            // Manual priority for tie-breaking
	CreatedAt               time.Time    `json:"created_at"`
	UpdatedAt               time.Time    `json:"updated_at"`
}

// RouteMatch represents a successful route pattern match.
type RouteMatch struct {
	Route      *GatewayRoute
	Params     map[string]string // Extracted path parameters
	MatchScore int               // Specificity score
}
