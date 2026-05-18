// Package platform provides infrastructure initialization helpers.
package platform

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metrics exported by the platform package.
var (
	WSConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "thecloud_ws_connections_active",
		Help: "The total number of active WebSocket connections",
	})

	// Auto-Scaling metrics
	AutoScalingEvaluations = promauto.NewCounter(prometheus.CounterOpts{
		Name: "thecloud_autoscaling_evaluations_total",
		Help: "Total number of auto-scaling evaluation cycles",
	})
	AutoScalingScaleOutEvents = promauto.NewCounter(prometheus.CounterOpts{
		Name: "thecloud_autoscaling_scale_out_total",
		Help: "Total number of scale-out events",
	})
	AutoScalingScaleInEvents = promauto.NewCounter(prometheus.CounterOpts{
		Name: "thecloud_autoscaling_scale_in_total",
		Help: "Total number of scale-in events",
	})
	AutoScalingCurrentInstances = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "thecloud_autoscaling_current_instances",
		Help: "Current instance count per scaling group",
	}, []string{"scaling_group_id"})

	// Load Balancer metrics
	LBRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "thecloud_lb_requests_total",
		Help: "Total requests proxied by load balancers",
	}, []string{"lb_id"})

	// HTTP metrics
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "thecloud_http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"method", "path", "status"})
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "thecloud_http_request_duration_seconds",
		Help:    "Duration of HTTP requests in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	// Compute metrics
	InstancesTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "thecloud_instances_total",
		Help: "Total number of instances",
	}, []string{"status", "backend"})
	InstanceOperationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "thecloud_instance_operations_total",
		Help: "Total number of instance operations",
	}, []string{"operation", "status"})

	// Storage metrics
	VolumesTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "thecloud_volumes_total",
		Help: "Total number of volumes",
	}, []string{"status"})
	VolumeSizeBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "thecloud_volume_size_bytes",
		Help: "Total volume size in bytes",
	})
	StorageOperationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "thecloud_storage_operations_total",
		Help: "Total number of storage operations",
	}, []string{"operation"})

	// Managed Services metrics
	RDSInstancesTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "thecloud_rds_instances_total",
		Help: "Total number of RDS instances",
	}, []string{"engine", "status"})
	CacheInstancesTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "thecloud_cache_instances_total",
		Help: "Total number of cache instances",
	}, []string{"status"})
	QueueMessagesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "thecloud_queue_messages_total",
		Help: "Total number of queue message operations",
	}, []string{"queue_id", "operation"})

	// Auth metrics
	AuthAttemptsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "thecloud_auth_attempts_total",
		Help: "Total number of authentication attempts",
	}, []string{"result"})
	APIKeysActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "thecloud_api_keys_active",
		Help: "Total number of active API keys",
	})

	// Credential rotation metrics
	CredentialCleanupFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "thecloud_credential_cleanup_failures_total",
		Help: "Total number of credential version cleanup failures in Vault",
	})
)
