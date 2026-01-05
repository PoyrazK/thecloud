# Future Plans &amp; Contributing

This document outlines planned features and how you can contribute to The Cloud.

---

## 🎯 Active Development

### Now Accepting Contributions

| Feature | Difficulty | Good First Issue? | Description |
|---------|------------|-------------------|-------------|
| **Postgres Repo Tests** | Easy | ✅ Yes | Add tests to `internal/repositories/postgres/` |
| **SDK Tests** | Easy | ✅ Yes | Add tests to `pkg/sdk/` |
| **API Docs (OpenAPI)** | Medium | ✅ Yes | Generate Swagger spec from handlers |
| **Snapshots** | Medium | No | Volume backup/restore |
| **RBAC** | Hard | No | Role-Based Access Control system |

### In Progress (Maintainers)

| Feature | Branch | Owner | ETA |
|---------|--------|-------|-----|
| Web Dashboard | `jack/main` | @jack | Q1 2026 |

---

## 📋 Feature Status

### ✅ Complete
- [x] **Compute** - Docker-based instance management
- [x] **Storage** - S3-compatible object storage
- [x] **Networking** - VPC with isolated Docker networks
- [x] **Block Storage** - Persistent volumes
- [x] **Load Balancer** - Layer 7 HTTP traffic distribution
- [x] **Auto-Scaling** - Dynamic scaling based on metrics
- [x] **RDS** - Managed PostgreSQL/MySQL containers
- [x] **Secrets Manager** - Encrypted secret storage
- [x] **CloudFunctions** - Serverless functions (Lambda-like)
- [x] **CloudCache** - Managed Redis instances
- [x] **CloudQueue** - SQS-like message queue
- [x] **CloudNotify** - Pub/Sub topics and subscriptions
- [x] **CloudCron** - Scheduled tasks with execution history
- [x] **CloudGateway** - API routing and reverse proxy
- [x] **CloudContainers** - Container orchestration with replication
- [x] **Audit Logging** - Comprehensive audit trail for all services
- [x] **Identity** - API key authentication and management

### 🚧 In Progress
- [ ] **Next.js Web Dashboard** - Visual resource management

### 📋 Backlog
- [ ] **RBAC** - User roles (admin, developer, read-only)
- [ ] **Snapshots** - Volume backup/restore
- [ ] **CloudFormation Templates** - IaC YAML definitions
- [ ] **Multi-region** - Cluster support

---

## 🏗️ Infrastructure &amp; CI/CD

| Item | Status | Description |
|------|--------|-------------|
| **CI Pipeline** | ✅ Done | Linting, testing, coverage with Codecov |
| **Staging Deployment** | ✅ Done | GHCR-based staging workflow |
| **Production Deployment** | ✅ Done | Tag-based releases |
| **Dependabot** | ✅ Done | Automated dependency updates |
| **Multi-Platform Builds** | 📋 Planned | ARM64/AMD64 Docker images |
| **E2E Integration** | 📋 Planned | E2E tests in CI pipeline |
| **Security Gates** | 📋 Planned | Trivy vulnerability scanning |

---

## 🛠️ How to Contribute

### 1. Pick an Issue
Choose from "Good First Issue" items above or check [GitHub Issues](https://github.com/PoyrazK/thecloud/issues).

### 2. Fork &amp; Clone
```bash
git clone https://github.com/YOUR_USERNAME/thecloud.git
cd thecloud
```

### 3. Create a Branch
```bash
git checkout -b feature/your-feature-name
```

### 4. Follow Project Structure
```
internal/
├── core/domain/    # Data structures
├── core/ports/     # Interfaces
├── core/services/  # Business logic
├── handlers/       # HTTP endpoints
└── repositories/   # Database/Docker adapters
```

### 5. Write Tests
- Place `_test.go` files next to the code
- Use `testify/mock` for mocking

### 6. Submit PR
- Reference any related issues
- Include test coverage
- Update docs if needed

---

## 📊 Current Test Coverage

| Package | Current | Target |
|---------|---------|--------|
| `services/` | **54.6%** | 60% |
| `handlers/` | **57.0%** | 60% |
| `handlers/ws/` | **81.6%** | 80% ✅ |
| `pkg/sdk/` | 26% | 50% |

---

## 📞 Contact

- Open an issue for questions
- Tag maintainers for review

*Last updated: 2026-01-05*
