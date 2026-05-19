# The Cloud

An open-source cloud platform that anyone can run, modify, and own.

## Features
- **Compute**: Multi-backend instance management (Docker or Libvirt/KVM)
  - Docker: Fast container-based instances
  - Libvirt: Full VM isolation with KVM/QEMU and **VNC Console** access ([Guide](docs/guides/libvirt-backend.md))
- **RBAC**: Role-Based Access Control with fine-grained permissions ([Guide](docs/guides/rbac.md))
- **Storage**: Distributed S3-compatible object storage with **Consistent Hashing** and **Gossip Protocol** ([Guide](docs/guides/storage.md))
- **Block Storage**: Persistent volumes via **LVM** (Production) or Docker Volumes (Simulation)
- **Networking**: Advanced VPC with SDN (Open vSwitch), Subnet isolation, IPAM, and **VPC Peering**.
- **Identity**: API Key authentication ([Guide](docs/guides/authentication.md))
- **Observability**: Prometheus metrics and Grafana dashboards ([Guide](docs/guides/observability.md)) with **Distributed Tracing** (Jaeger).
- **Load Balancer**: Layer 7 HTTP traffic distribution (Regional)
- **Global Load Balancer**: Multi-region traffic steering at the DNS level ([Guide](docs/services/cloud-global-lb.md))
- **Auto-Scaling**: Dynamic scaling of compute resources based on metrics
- **Managed Databases (RDS)**: Launch PostgreSQL/MySQL instances with a single command ([Guide](docs/guides/rds.md))
- **Managed Caches (Redis)**: Launch and manage Redis instances ([Guide](docs/guides/cache.md))
- **Managed Kubernetes (KaaS)**: Provision production-ready HA clusters with automated LB setup ([Guide](docs/api-reference.md#managed-kubernetes-kaas))
- **Cloud Functions (Serverless)**: Run logic without managing servers ([Guide](docs/guides/functions.md))
- **Secrets Manager**: Encrypted storage for API keys and sensitive config ([Guide](docs/guides/secrets.md))
- **CloudQueue**: Distributed message queuing with visibility timeouts ([Guide](docs/services/cloud-queue.md))
- **CloudNotify**: Pub/Sub topic and subscription service via Webhooks/Queues ([Guide](docs/services/cloud-notify.md))
- **CloudCron**: Managed scheduled tasks with execution history ([Guide](docs/services/cloud-cron.md))
- **CloudGateway**: API Routing and Reverse Proxy with path stripping ([Guide](docs/services/cloud-gateway.md))
- **CloudContainers**: Managed container deployments with replication and auto-healing ([Guide](docs/services/cloud-containers.md))
- **CloudDNS**: Managed authoritative DNS with VPC auto-registration ([Guide](docs/guides/dns.md))
- **Console**: Interactive Next.js Dashboard for visual resource management

## Authentication
The Cloud uses API Key authentication with comprehensive security features.

### User Registration & Login
1. **Register**: `POST /auth/register` to create an account.
2. **Login**: `POST /auth/login` to receive your API Key.
3. **Authenticate**: Send `X-API-Key: <your-key>` header with all requests.

### Password Reset
- **Request Reset**: `POST /auth/forgot-password` with your email (rate limited: 5/min)
- **Reset Password**: `POST /auth/reset-password` with token and new password

### API Key Management
- **Create Key**: `POST /auth/keys` (requires authentication)
- **List Keys**: `GET /auth/keys`
- **Rotate Key**: `POST /auth/keys/:id/rotate`
- **Regenerate Key**: `POST /auth/keys/:id/regenerate`
- **Revoke Key**: `DELETE /auth/keys/:id`

## Role-Based Access Control (RBAC) & IAM
Manage users, roles, and granular policies via the CLI or API.

- **Create Role**: `cloud roles create developer --permissions "instance:read,volume:read"`
- **Bind Role**: `cloud roles bind user@example.com developer`
- **IAM Policy**: `cloud iam create ReadOnlyS3 ./policy.json`
- **Attach Policy**: `cloud iam attach user-uuid policy-uuid`

## System Health
- **Liveness**: `GET /health/live` (Returns 200 OK)
- **Readiness**: `GET /health/ready` (Returns 200 if DB/Docker connected, 503 if not)

## Quick Start (Backend)
```bash
# 1. Clone & Setup
git clone https://github.com/PoyrazK/thecloud.git
cd thecloud
make run
# Or with Libvirt/KVM (Requires Linux Host):
# make run COMPUTE_BACKEND=libvirt STORAGE_BACKEND=lvm

# 2. Register & Login (Get API Key)
curl -X POST http://localhost:8080/auth/register \
  -d '{"email":"user@example.com", "password":"StrongPassword123!", "name":"User"}'

curl -X POST http://localhost:8080/auth/login \
  -d '{"email":"user@example.com", "password":"StrongPassword123!"}'
# Copy the "api_key" from the response

# 3. Test Access
# All protected endpoints require the X-API-Key header
curl -H "X-API-Key: YOUR_API_KEY" http://localhost:8080/instances
```

## Quick Start (Console - Frontend)
```bash
# 1. Enter web directory
cd web

# 2. Install dependencies
npm install

# 3. Start development server
npm run dev

# 4. Open in browser
# http://localhost:3000
```

## Architecture
- **Frontend**: Next.js 14, Tailwind CSS, GSAP
- **Backend**: Go (Clean Architecture, Hexagonal)
- **Database**: PostgreSQL (pgx)
- **Infrastructure**: 
  - Docker Engine (Containers, Networks, Volumes)
  - Libvirt/KVM (Virtual Machines, QCOW2 Storage, NAT Networks)
  - Open vSwitch (SDN, VXLAN, VPC Isolation, Subnets)
- **Observability**: Prometheus Metrics, Real-time WebSockets, OpenTelemetry (Tracing)
- **CLI**: Cobra (command-based) + Survey (interactive)

## Recent Improvements

### CLI Enhancements
- **Multiple Output Formats**: `--output table|json|yaml` flag (default: table)
- **Shell Completion**: `cloud completion [bash|zsh|fish|powershell]`
- **Persistent Configuration**: `cloud config show|set|unset` for storing API URL and defaults
- **Debug Mode**: `--debug` flag for tracing API calls
- **Pagination Support**: `--limit` and `--offset` flags on list commands

### Code Quality & Features
- **Simplified Architecture**: Refactored `InstanceService` using parameter structs and helper methods.
- **Enhanced Storage**: Added support for **LVM Block Storage** and **VNC Console** access.
- **Asynchronous Core**: Refactored long-running operations (K8s clusters, instance deletions) to use a durable **Redis Task Queue**.
- **HA Control Plane**: Supported 1-click **High-Availability** Kubernetes clusters with 3 control plane nodes and automated API Server Load Balancers.
- **VPC Peering**: Implemented inter-VPC connectivity using Open vSwitch (OVS) flow rules, including automated CIDR overlap validation and VPC deletion guards.
- **Distributed Storage (v2)**: Replaced local filesystem storage with a multi-node **Distributed Object Store** featuring:
    - **Consistent Hash Ring**: Dynamic data distribution across nodes.
    - **Gossip Protocol**: Fully decentralized node discovery and health tracking.
    - **Quorum-based Replication**: Configurable N-way replication with write-quorum consistency.
- **Clean Code**: Eliminated duplicate literals and improved test security across all service layers.
- **Global Load Balancing**: Implemented a **GeoDNS-based** global steering service with multi-region health tracking and latency-optimized routing.
- **Compute Modernization**: Enabled **ARM64 (UEFI)** support for the Libvirt backend using the `virt` machine type and `AAVMF` firmware.
- **Kubernetes Storage**: Implemented a feature-complete **CSI Driver** for dynamic block storage provisioning in KaaS clusters, including automated device formatting and attachment.
- **Database Security (v2)**: Integrated **HashiCorp Vault** for secure credential management in Managed Databases (RDS), including automated password storage and zero-downtime **Credential Rotation**.

### AI & Automation
- **AI Context**: Added `GEMINI.md` to provide AI assistants with project-specific hexagonal architecture rules and coding standards.
- **Interactive Workflows**: Introduced `.agent/workflows/` for rapid development:
    - `/new-service`: Scaffold full hexagonal layers (Domain → Port → Service → Repo → Handler).
    - `/deploy`: Automated build and deployment with **integrated smoke tests**.
    - `/test-coverage`: Detailed coverage analysis and reporting.
    - `/swagger`: One-click OpenAPI documentation updates.

### CI/CD & DevSecOps
- **Quality Gates**: Integrated `golangci-lint`, SonarQube, and automated k6 performance testing.
- **Project Hygiene**: Major cleanup of legacy artifacts and repository optimization.

See [CHANGELOG.md](CHANGELOG.md) for detailed changes.

## Documentation

### Getting Started
| Doc | Description |
|-----|-------------|
| [Development Guide](docs/development.md) | Setup on Windows, Mac, or Linux |
| [Roadmap](docs/ROADMAP.md) | Project roadmap and feature status |
| [Vision](docs/vision.md) | Long-term strategy and goals |

### Architecture & Services
| Doc | Description |
|-----|-------------|
| [Architecture Overview](docs/architecture.md) | System design and patterns |
| [Backend Guide](docs/backend.md) | Go service implementation |
| [Database Guide](docs/database.md) | Schema, tables, and migrations |
| [CLI Reference](docs/cli-reference.md) | All commands and flags |
| [CloudQueue](docs/services/cloud-queue.md) | Message Queue deep dive |
| [CloudNotify](docs/services/cloud-notify.md) | Pub/Sub details |
| [CloudCron](docs/services/cloud-cron.md) | Scheduler internals |
| [CloudGateway](docs/services/cloud-gateway.md) | Gateway & Proxy guide |
| [CloudContainers](docs/services/cloud-containers.md) | Container Orchestration |
| [CloudDNS](docs/services/cloud-dns.md) | Managed DNS Service |
| [CloudGlobalLB](docs/services/cloud-global-lb.md) | Multi-Region Traffic Steering |

### Guides
| Guide | Description |
|-------|-------------|
| [Libvirt Backend](docs/guides/libvirt-backend.md) | KVM/QEMU virtualization setup and usage |
| [RBAC Management](docs/guides/rbac.md) | Roles, permissions, and bindings |
| [Auto-Scaling](docs/guides/autoscaling.md) | Scalability patterns and usage |
| [Load Balancer](docs/guides/loadbalancer.md) | Traffic distribution guide |
| [Managed Databases](docs/guides/rds.md) | RDS patterns and usage |
| [Secrets Manager](docs/guides/secrets.md) | Security and encryption guide |
| [Networking](docs/guides/networking.md) | VPCs and Network isolation |
| [Storage](docs/guides/storage.md) | Object and Block storage |
| [Managed Caches](docs/guides/cache.md) | Redis cache management |
| [Cloud Functions](docs/guides/functions.md) | Serverless execution |
| [CloudDNS](docs/guides/dns.md) | Managed DNS & Auto-Registration |

## KPIs
- Time to Hello World: < 5 min
- API Latency (P95): < 200ms
- CLI Success Rate: > 95%
- **Test Coverage: 59.7%** (Unit + Integration Tests)

## Testing
The Cloud has comprehensive test coverage across all layers:
- **Unit Tests**: Core services, handlers, and business logic
- **SDK Tests**: 80.1% coverage with httptest mocking
- **Repository Tests**: 70.1% coverage with pgxmock
- **Overall Coverage**: 59.7% (Services: 71.5%, Handlers: 65.8%, Repositories: 70.1%)

Run tests:
```bash
# All tests
go test ./...

# With coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Specific packages
go test ./pkg/sdk/...          # SDK tests only
go test ./internal/core/services/...  # Service tests only
```

For comprehensive testing guide, see [docs/TESTING.md](docs/TESTING.md).
