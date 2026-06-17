# 🌟 The Cloud - Features & Implementation Deep Dive

This document provides a comprehensive overview of every feature currently implemented in **The Cloud**, including the technologies used and the architectural details of how they are built.

---

## 🏗️ Core Infrastructure

### 1. Compute (Instances)
**What it is**: Launch virtual machines or containers to run applications.

**Tech Stack**: 
- **Docker Engine** (Container Backend - Simulators)
- **Libvirt/KVM** (VM Backend - Production)
- **Go** (Backend with unified `ComputeBackend` interface)

**Implementation**:
- **Multi-Backend Architecture**: The system supports two compute backends via a unified `ComputeBackend` interface:
  
  **Docker Backend** (Simulation/Dev):
  - **Simulation**: Uses Docker Containers to simulate instances with sub-second boot times.
  - **Isolation**: Process-level isolation via namespaces.
  - **Networking**: Instances attach to custom Bridge Networks (simulating VPCs).
  - **Persistence**: Docker Volumes for persistent storage.
  
  **Libvirt Backend** (Production/KVM):
  - **Real VMs**: Uses KVM/QEMU for full hardware virtualization.
  - **Isolation**: Complete VM isolation with dedicated kernels.
  - **Console**: **VNC Console** access via dynamic websockets (`/console` endpoint).
  - **Storage**: Supports both file-based QCOW2 and raw LVM block devices.
  - **Networking**: Integrated with Open vSwitch (OVS) for true SDN.

- **Backend Selection**: Set via `COMPUTE_BACKEND` environment variable (`docker` or `libvirt`).
- **Lifecycle**: The `InstanceService` manages the backend API to Create, Start, Stop, Pause, Resume, Resize, and Remove instances.
- **Instance States**: Instances transition through states: `INITIALIZING` → `RUNNING` → `STOPPED` / `PAUSED` → `DELETED`. The `PAUSED` state freezes CPU while retaining memory and network connections.
- **Pause/Resume**: RUNNING instances can be paused (via `DomainSuspend` for Libvirt, `ContainerPause` for Docker) and later resumed. State transitions are validated to ensure proper ordering.
- **Instance Metadata & Labels**: Support for arbitrary key-value pairs assigned to instances for organization and filtering.
- **Cloud-Init (Docker Simulation)**: Simulates Cloud-Init configuration injection in containers (SSH keys, script execution).
- **Self-Healing**: Automated background worker that detects instances in `ERROR` state and attempts recovery via restart.
- **Resilient Compute**: Backend operations are wrapped with circuit breaker and bulkhead patterns for fault tolerance.

### 2. Networking (VPC & Elastic IPs)
**What it is**: Isolated virtual networks and static public IP addresses.
**Tech Stack**: Docker Networks, Open vSwitch (OVS), pgx.

**VPC Implementation**:
- **Docker Mode**: A "VPC" maps directly to a **Docker Bridge Network**.
- **Libvirt Mode**: Uses **Open vSwitch (OVS)** bridges and VXLANs for tenant isolation.

**Elastic IP Implementation**:
- **Static Reservation**: Reserve static IPv4 addresses from a public pool (simulated via 100.64.0.0/10).
- **Dynamic Association**: Attach/detach IPs to any compute instance without changing the instance's private setup.
- **Persistence**: IPs remain reserved to the tenant even when not associated with a resource.
- **Constraints**: Enforces one Elastic IP per instance via partial unique database indexes.

**VPC Peering 🆕**:
- **Same-Tenant Connectivity**: Securely connect two VPCs within the same tenant to allow private IP communication.
- **CIDR Validation**: Prevents peering between VPCs with overlapping CIDR blocks to avoid routing conflicts.
- **OVS Flow Rules**: Dynamically programs Open vSwitch flow rules for cross-bridge routing between VPC bridges.
- **VPC Guard**: Prevents deletion of VPCs that have active peering connections.

**Isolation**: strict traffic segregation rules enforced by generic or OVS flow rules.

### 3. Block Storage (Volumes)
**What it is**: Persistent disks that can be attached/detached from instances.
**Tech Stack**: Docker Volumes or Linux LVM.
**Implementation**:
- **Docker Mode**: Uses Docker bind mounts via stop→recreate→start cycle. When a volume is attached, the container is gracefully stopped, recreated with updated bind mounts, and restarted. This approach ensures container ID tracking and rollback on failures.
- **LVM Mode**:
  - **Creation**: Allocates raw logical volumes (`lvcreate`) from a volume group.
  - **Snapshots**: Instant, copy-on-write snapshots for backups.
  - **Performance**: Near-native block device performance for VMs.
- **Attachment**: Hot-pluggable (in Libvirt mode) or via container recreation (in Docker mode).
- **Persistence**: Data survives instance termination.
- **Container ID Tracking**: After attach/detach, the instance's `ContainerID` is updated to reflect the new container created during the operation.

### 4. Object Storage (S3-compatible)
**What it is**: Store and retrieve files (blobs) via API with enterprise-grade features.
**Tech Stack**: Go (IO/FS), Local Filesystem, AES-GCM Encryption.
**Implementation**:

**Bucket Management**:
- **Create/Delete Buckets**: Full lifecycle management with validation.
- **Bucket Versioning**: Enable/disable versioning per bucket to preserve object history.
- **Bucket Listing**: List all buckets for a user/tenant.

**Object Operations**:
- **Upload/Download**: Stream-based PUT/GET with efficient memory usage.
- **List Objects**: Enumerate all objects within a bucket.
- **Delete**: Remove objects with optional version targeting.

**Versioning**:
- **Version History**: List all versions of an object.
- **Version Download**: Retrieve specific historical versions.
- **Version Delete**: Remove specific versions while preserving others.

**Multipart Upload** (for large files):
- **Initiate**: Start a multipart upload session.
- **Upload Parts**: Upload chunks in parallel with part numbers.
- **Complete**: Assemble all parts into final object.
- **Abort**: Cancel and clean up incomplete uploads.

**Security & Access**:
- **Presigned URLs**: Generate temporary signed URLs for time-limited access.
- **Encryption**: Objects encrypted at rest using AES-GCM via EncryptionService.
- **Audit Trail**: All operations logged for compliance.

**Reliability & Integrity**:
- **Two-Phase Upload**: Atomic metadata creation (`PENDING` -> `AVAILABLE`) prevents "invisible" orphaned files from failed transfers.
- **Automated Orphan GC**: Background `StorageCleanupWorker` identifies and purges stalled uploads (older than configurable `PendingUploadTTL`) and their partial files.
- **Streaming Checksums**: Real-time SHA-256 calculation during the upload stream for verifiable end-to-end data integrity.
- **S3-Compatible Validation**: Strict bucket naming rules (3-63 chars, lowercase, no IP formats) and automatic content-sniffing for accurate MIME metadata.

**Distributed Storage**:
- **Cluster Status**: Monitor distributed storage cluster health.
- **Multi-node Support**: Architecture ready for horizontal scaling.

---

## 🛠️ Managed Services

### 5. Managed Databases (RDS)
**What it is**: Provision fully managed PostgreSQL or MySQL databases with high availability.
**Tech Stack**: Docker (Official Images), PgBouncer (Pooling), Prometheus Exporters, Go, TCP Health Checks, HashiCorp Vault.
**Implementation**:
- **Multi-Engine Support**: PostgreSQL and MySQL with configurable versions.
- **Provisioning**: Spawns Docker containers using official images (`postgres:<version>-alpine`, `mysql:<version>`).
- **Data Persistence**: Automatically provisions and attaches persistent block volumes to every database instance. Data is stored on durable block devices, ensuring survival across container lifecycles.
- **Volume Expansion 🆕**: Support for dynamic storage scaling. Users can increase allocated storage via the API, which automatically grows the underlying LVM logical volume and filesystem (ext4/XFS) using `lvextend -r`.
- **Dynamic Configuration**: Supports updating allocated storage, engine parameters, and feature flags without manual intervention.
- **Connection Pooling**: Optional high-performance pooling for PostgreSQL via a **PgBouncer** sidecar. Dynamically toggleable on existing instances.
- **Observability Sidecars**: Built-in metrics collection via Prometheus exporters (`postgres-exporter`, `mysqld-exporter`) running as managed sidecars.
- **Read Replicas**: Support for creating read-only replicas linked to a primary instance. Replicas inherit the primary's configuration, including storage size and feature flags.
- **Automated Failover**: A dedicated `DatabaseFailoverWorker` performs periodic TCP health checks on primary instances. If a primary becomes unreachable, the worker checks replication lag on all linked PostgreSQL standbys (using `pg_last_wal_receive_lsn()` / `pg_last_wal_replay_lsn()` and `pg_last_xact_replay_timestamp()` with a ≤ 5 seconds threshold) and promotes the healthiest replica. Non-PostgreSQL engines fall back to TCP-only health checks and are treated as having zero lag for promotion selection.
- **Manual Promotion**: API support for manually promoting a replica to primary status for maintenance or planned transitions.
- **Security & Vault Integration 🆕**: Optional integration with **HashiCorp Vault (KV v2)** for secure credential storage. The system orchestrates storing database passwords in Vault, while maintaining a `password` field in the database metadata store for fallback support and legacy compatibility.
- **Credential Rotation 🆕**: Support for automated password rotation. The system regenerates secure passwords, executes `ALTER USER` commands inside the database container, and updates Vault on success. If present, the **PgBouncer** sidecar is automatically restarted (recreated) to apply new credentials. Note: Exporter sidecars are not reloaded as part of this flow, and rotation is not fully atomic across the database and Vault.
- **Encryption at Rest 🆕**: Optional volume encryption using **Vault Transit** for DEK management. When `kms_key_id` is provided at creation, each volume gets a unique 256-bit DEK encrypted by Vault. Supports AES-256-GCM and works transparently across all storage backends. Meets compliance requirements (SOC 2, GDPR, HIPAA).

- **Credentials**: Auto-generates secure passwords (16-char random) and default usernames.
- **VPC Integration**: Databases can be deployed into specific VPCs for network isolation.
- **Connection Strings**: `GetConnectionString()` API returns ready-to-use connection URLs, automatically routing through the connection pooler when enabled.
- **Event & Audit**: All operations, including failover events and role changes, are logged for compliance tracking.
- **Metrics**: Prometheus metrics for RDS instance counts by engine and role.

### 6. Managed Caches (CloudCache)
**What it is**: Provision fully managed Redis instances.
**Tech Stack**: Redis (Alpine), Go, Docker Exec.
**Implementation**:
- **Provisioning**: Launches `redis-server` with custom configuration (AOF enabled, RDB disabled).
- **Security**: Generates a random password and enables `--requirepass`.
- **VPC Integration**: Caches can be deployed into specific VPCs.
- **Management Operations**:
  - **FlushCache**: Executes `FLUSHALL` via Docker Exec.
  - **GetCacheStats**: Parses `redis-cli INFO` for connected clients, memory usage, keys count, uptime.
- **Connection Strings**: API returns ready-to-use Redis URLs with auth.
- **Lookup**: Get cache by ID or name for flexibility.

### 7. Cloud Functions (Serverless)
**What it is**: Run code snippets without provisioning servers.
**Tech Stack**: Docker (Ephemeral Containers), Go.
**Implementation**:
- **Multi-Runtime Support**:
  - `nodejs20` - Node.js 20 Alpine
  - `python312` - Python 3.12 Alpine
  - `go122` - Go 1.22 Alpine
  - `rust` - Rust 1.75 Alpine
  - `java21` - Eclipse Temurin 21 Alpine
- **Code Deployment**: Supports raw code or ZIP archives with auto-extraction.
- **Execution**: One-shot containers with configurable timeouts.
- **Async Invocation**: Background execution via Go routines.
- **Invocation Logs**: Stored history of function executions with output/errors.
- **Handler Configuration**: Specify entry point file for each function.
- **Partial Updates**: Update timeout, memory, handler, and status via `PATCH /functions/:id`.
- **Environment Variables**: Inject key-value env vars at runtime without code changes.
- **Secret Integration**: Env vars can reference secrets (`@secretname`) resolved at invocation time via SecretService — no plaintext secrets stored in the database.

### 8. Load Balancer (ELB)
**What it is**: Distribute incoming HTTP traffic across multiple instances.
**Tech Stack**: Go (Reverse Proxy `httputil`), Configurable Algorithms.
**Implementation**:
- **VPC-Aware**: Load balancers are scoped to VPCs for network isolation.
- **Algorithms**: Supports `round-robin` (default) with architecture for additional algorithms.
- **Target Management**:
  - **AddTarget**: Register instances with port and weight.
  - **RemoveTarget**: Deregister instances.
  - **ListTargets**: View all registered targets.
- **Cross-VPC Validation**: Prevents adding instances from different VPCs.
- **Health Tracking**: Target health status tracking (`unknown`, `healthy`, `unhealthy`).
- **Idempotency**: Idempotency keys prevent duplicate LB creation.
- **Versioning**: Optimistic locking via version field for concurrent updates.
- **Global Scope**: Use Global Load Balancers (GLB) for multi-region traffic distribution across different regional ELBs.

### 9. Global Load Balancer (GLB) 🆕
**What it is**: Multi-region traffic steering at the DNS level.
**Tech Stack**: PowerDNS (LUA/Geo), Go, PostgreSQL.
**Implementation**:
- **GeoDNS Orchestration**: Dynamically manages DNS A/CNAME records based on regional health.
- **Routing Policies**: Supports `LATENCY`, `GEOLOCATION`, `WEIGHTED`, and `FAILOVER`.
- **Health Tracking**: Global probes monitor regional endpoints; unhealthy targets are automatically pulled from DNS.
- **Hybrid Support**: Can route to internal Regional LBs or external static IPs.

### 9. Managed Kubernetes (KaaS) 🆕
**What it is**: Provision and manage production-ready Kubernetes clusters.
**Tech Stack**: Kubeadm, Containerd, LoadBalancer, Go Workers.
**Implementation**:
- **Async Provisioning**: Uses a Redis-backed **Task Queue** to handle cluster creation/deletion asynchronously. 
- **High Availability (HA)**: Supports 3-node HA Control Plane with an automated API Server Load Balancer.
- **Node Management**: Automatically bootstraps nodes with `kubeadm` and required CNI plugins.
- **Isolation**: Each cluster is isolated within its own VPC.

### 10. Auto-Scaling
**What it is**: Automatically add/remove instances based on CPU load.
**Tech Stack**: Go Background Workers.
**Implementation**:
- **Metrics Loop**: A background worker polls Docker Stats for every instance in a scaling group.
- **Decision Engine**: Checks if average metric > target (e.g., CPU > 50%).
- **Scale Out**: Calls `InstanceService` to clone the template instance.
- **Scale In**: Terminates the oldest instance in the group.

### 11. Secrets Manager
**What it is**: Store sensitive data (API keys, passwords) securely.
**Tech Stack**: AES-GCM Encryption, Go `crypto/aes`, HKDF Key Derivation.
**Implementation**:
- **Encryption**: Secrets encrypted at rest using AES-256-GCM.
- **Per-User Key Derivation**: Master key + user ID via HKDF for user-isolated encryption.
- **Access Tracking**: `LastAccessedAt` updated on every secret read.
- **Lookup**: Get secret by ID or by name.
- **Redaction**: List operations return `[REDACTED]` values for security.
- **Event & Audit Logging**: CREATE, ACCESS, DELETE events tracked.
- **Production Safety**: Enforces encryption key requirement in production mode.

### 12. Managed DNS (Route Cloud) 🆕
**What it is**: Managed DNS zones and records with automatic instance registration.
**Tech Stack**: PowerDNS, Go.
**Implementation**:
- **Zone Management**: Create and manage private DNS zones for VPCs.
- **Auto-Registration**: Instances automatically register their private IP addresses in the VPC's DNS zone upon launch.
- **Record Types**: Supports A, AAAA, CNAME, MX, and TXT records.
- **PowerDNS Integration**: Powered by a PowerDNS backend for production-grade reliability.
- **VPC Scoped**: Zones are scoped to VPCs for private network resolution.

### 13. API Gateway 🆕
**What it is**: Managed entry point for microservices with advanced routing, pattern matching, and rate limiting.
**Tech Stack**: Go `httputil.ReverseProxy`, Redis, Regex Matcher.
**Implementation**:
- **Advanced Pattern Matching**: Support for RESTful patterns like `/users/{id}`, regex-constrained parameters `/id/{id:[0-9]+}`, and wildcards `/static/*`.
- **HTTP Method Routing**: Route requests to different backends based on the HTTP verb (GET, POST, etc.) for the same path.
- **Dynamic Specificity Scoring**: Automatic route selection based on prefix specificity, exact match bonuses, and explicit user-defined priority.
- **Prefix Stripping**: Intelligent stripping of path patterns before forwarding to downstream services.
- **Rate Limiting**: Integrated distributed rate limiting per route.
- **Audit Logging**: Comprehensive tracking of all route changes and gateway operations.

### 14. CloudStacks (Native IaC) 🆕
**What it is**: Declarative infrastructure management (similar to AWS CloudFormation).
**Tech Stack**: YAML, Go, Hexagonal Orchestration.
**Implementation**:
- **Declarative Templates**: Define multiple resources (VPCs, Instances, Volumes) in a single YAML file.
- **Dependency Management**: Automatically resolves references between resources (e.g., attaching a volume to an instance created in the same stack).
- **Atomic Operations**: Support for rollback on failure during stack creation.
- **Validation**: API endpoint for validating templates before deployment.

---

## 🧩 Platform Services

### 15. Identity & Auth (IAM)
**What it is**: Secure access to the platform via RBAC and Granular IAM Policies.

**Tech Stack**: JWT (JSON Web Tokens), BCrypt, ABAC (Attribute-Based Access Control).

**Implementation**:
- **Passwords**: Hashed using `bcrypt` cost 12.
- **Tokens**: Stateless JWTs signed with HMAC-SHA256.
- **API Keys**: Alternative authentication method with tenant context.
- **Middleware**: Go middleware validates API Key or JWT on every authenticated route.

**Role-Based Access Control (RBAC)**:
- **Built-in Roles**: `admin`, `developer`, `viewer` with default permissions.
- **Custom Roles**: Create, update, delete custom roles via API.
- **Permission System**: Fine-grained permissions (e.g., `instance:read`, `volume:write`).
- **Role Binding**: Assign roles to users by ID or email.
- **Authorization**: `Authorize()` checks user permissions before operations.
- **Fallback Logic**: Default permissions apply if role not in DB.

### 16. Multitenancy & Organizations 🆕
**What it is**: Logical grouping of resources into "Tenants" (organizations) for isolated management.

**Implementation**:
- **Tenant Context**: All resources (Instances, VPCs, Buckets) are scoped to a `TenantID`.
- **Default Tenant**: Users have a default tenant but can belong to multiple organizations.
- **CLI Support**: Global `--tenant` flag and `cloud tenant switch` command to manage active context.
- **Isolation**: Shared database with row-level logical isolation enforced by service-layer filters.

### 17. IAM Policies (ABAC) 🆕
**What it is**: Fine-grained, document-based authorization supplementing legacy RBAC.
**Implementation**:
- **JSON Policy Documents**: Supports AWS-style JSON policies with `Effect`, `Action`, `Resource`, and `Condition`.
- **Policy Evaluation**: The `IAMEvaluator` uses wildcard pattern matching for granular resource targeting.
- **Security-First**: "Explicit Deny" logic ensures that any Deny statement overrides all Allows, and presence of policies stops fallthrough to legacy roles unless evaluation is "Allow".
- **Dynamic Context**: Infrastructure ready for attribute-based evaluation via statement conditions.
- **User Attachment**: Policies can be dynamically attached to or detached from users without modifying their primary role.

### 17. Observability
**What it is**: Monitor system health and logs.
**Tech Stack**: Docker API, WebSockets, Prometheus, Grafana, PostgreSQL (CloudLogs).
**Implementation**:
- **Real-time Stats**: Stream Docker container stats (CPU/Mem/Net) via WebSocket.
- **Instance Logs**: Attach to container `stdout/stderr` streams for real-time viewing.
- **CloudLogs (Persistent Logs) 🆕**:
    - **Persistence**: Automatically ingests and stores logs in PostgreSQL upon instance termination.
    - **Search & Filter**: API and CLI support for searching historical logs by resource, time range, and keywords.
    - **Trace correlation**: Links logs to request TraceIDs for integrated debugging with Jaeger.
    - **Retention**: Configurable background worker that automatically purges logs older than X days.
- **Dashboard Service**: Aggregated system metrics and health status.
- **Prometheus Metrics**: Custom metrics for instances, databases, caches, functions.
- **Grafana Dashboards**: Pre-configured dashboards for visualization.
- **Event System**: Event recording for all resource state changes.
- **Audit Logs**: Comprehensive audit trail for compliance.

### 17. CLI (Command Line Interface)
**What it is**: Terminal tool to manage "The Cloud".
**Tech Stack**: Cobra (CLI framework), Viper (Config).
**Implementation**:
- **Structure**: Command-based (`cloud <resource> <action>`).
- **State**: Stateless client; talks to the Backend API via HTTP.

### 18. Console (Frontend)
**What it is**: Visual dashboard.
**Tech Stack**: Next.js 16, React 19, Tailwind CSS, TypeScript.
**Implementation**:
- **SSR**: Server-Side Rendering for main dashboard views.
- **Architecture**: Component-based modern React.

### 19. Terraform Provider 🆕
**What it is**: Infrastructure as Code (IaC) support via Terraform.
**Tech Stack**: Terraform Plugin SDK v2, Go.
**Implementation**:
- **Custom Provider**: `terraform-provider-thecloud` allows defining resources in HCL.
- **Supported Resources**: Lifecycle management for Instances, VPCs, DNS Zones, and Volumes.
- **State Management**: Integrates with Terraform's state for consistent infrastructure management.

---

## 💾 Storage & Database
**State Management**:
- **Primary DB**: **PostgreSQL 16** holds all metadata (users, instance configs, VPCs, permissions).
- **Object Storage**: Local file system (simulating S3).

---

## 🚀 How It's Built
We use a **Clean / Hexagonal Architecture**:
1.  **Adapters (Handlers/Repositories)**: External facing code (HTTP, SQL, Docker).
2.  **Core (Services/Ports)**: Pure business logic (e.g., "Create Instance", "Hash Password").
3.  **Domain**: Shared Go structs and entities.

This ensures that we can swap out Docker for Kubernetes, or Postgres for SQLite, without changing the core business logic.
