# CI/CD Pipeline Documentation

## Overview

The Cloud uses GitHub Actions for continuous integration and deployment. The pipeline is designed to test both Docker and Libvirt compute backends, ensuring compatibility across different infrastructure providers.

## Pipeline Structure

### Workflows

1. **CI Pipeline** (`.github/workflows/ci.yml`)
   - Runs on: Push to `main`, `feature/libvirt-adapter`, and all PRs
   - Tests both Docker and Libvirt backends
   - Builds and scans Docker images
   - Deploys to staging/production

2. **Release Pipeline** (`.github/workflows/release.yml`)
   - Runs on: Version tags (`v*`)
   - Creates GitHub releases
   - Publishes artifacts

## CI Pipeline Jobs

### 1. Test (Docker Backend)

**Purpose**: Validate code quality and Docker backend functionality

**Steps**:
- ✅ Checkout code
- ✅ Setup Go 1.24
- ✅ Install dependencies
- ✅ Verify Swagger documentation is up-to-date
- ✅ Run golangci-lint
- ✅ Run unit tests with race detection
- ✅ Generate coverage report
- ✅ Upload coverage to Codecov
- ✅ Run database migrations
- ✅ Run integration tests against PostgreSQL

**Services**:
- PostgreSQL 16 (port 5433)

**Environment**:
```yaml
DATABASE_URL: postgres://cloud:cloud@localhost:5433/thecloud?sslmode=disable
```

### 2. Test Libvirt Backend (NEW)

**Purpose**: Validate Libvirt/KVM backend functionality

**Steps**:
- ✅ Checkout code
- ✅ Setup Go 1.24
- ✅ Install libvirt dependencies:
  - `qemu-kvm`
  - `libvirt-daemon-system`
  - `libvirt-clients`
  - `genisoimage`
  - `qemu-utils`
- ✅ Start libvirtd daemon
- ✅ Configure permissions
- ✅ Create default storage pool (`/tmp/libvirt-images`)
- ✅ Create default NAT network
- ✅ Run Libvirt unit tests
- ✅ Run Libvirt integration test (`scripts/test_libvirt.go`)
- ✅ Test backend switching (Docker ↔ Libvirt)

**Services**:
- PostgreSQL 16 (port 5433)
- Libvirt daemon (local)

**Environment**:
```yaml
DATABASE_URL: postgres://cloud:cloud@localhost:5433/thecloud?sslmode=disable
COMPUTE_BACKEND: libvirt  # Switches between docker/libvirt
```

### 3. Build

**Purpose**: Build and scan Docker images

**Dependencies**: Requires both `test` and `test-libvirt` jobs to pass

**Steps**:
- ✅ Build Docker image
- ✅ Scan for vulnerabilities using Trivy
- ✅ Report CRITICAL and HIGH severity issues

**Security Scanning**:
- Tool: Aqua Security Trivy
- Scan types: OS packages, libraries
- Currently non-blocking (exit-code: 0)

### 4. Deploy Staging

**Purpose**: Deploy to staging environment on main branch

**Triggers**: Push to `main` branch

**Steps**:
- ✅ Login to GitHub Container Registry (ghcr.io)
- ✅ Build and push images with tags:
  - `ghcr.io/REPO:staging`
  - `ghcr.io/REPO:SHA`

**Permissions**:
- `packages: write`
- `contents: read`

### 5. Deploy Production

**Purpose**: Deploy to production on version tags

**Triggers**: Tags matching `v*` (e.g., `v1.0.0`)

**Steps**:
- ✅ Login to GitHub Container Registry
- ✅ Build and push images with tags:
  - `ghcr.io/REPO:latest`
  - `ghcr.io/REPO:v1.0.0` (version tag)
  - `ghcr.io/REPO:SHA`

**Permissions**:
- `packages: write`
- `contents: read`

## Testing Matrix

### Backend Coverage

| Backend | Unit Tests | Integration Tests | E2E Tests |
|---------|-----------|-------------------|-----------|
| Docker | ✅ | ✅ | ✅ |
| Libvirt | ✅ | ✅ | ✅ |

### Test Scenarios

1. **Docker Backend**:
   - Container lifecycle (create, start, stop, delete)
   - Volume management
   - Network isolation
   - Port mapping
   - Snapshots

2. **Libvirt Backend**:
   - VM lifecycle (create, start, stop, delete)
   - QCOW2 volume management
   - NAT network configuration
   - Cloud-Init ISO generation
   - DHCP lease tracking
   - Port forwarding (iptables)
   - Volume snapshots (qemu-img)

3. **Backend Switching**:
   - Services work with both backends
   - API compatibility maintained
   - Configuration switching (`COMPUTE_BACKEND` env var)

## Environment Variables

### Required for CI

```bash
# Database
DATABASE_URL=postgres://cloud:cloud@localhost:5433/thecloud?sslmode=disable

# Compute Backend Selection
COMPUTE_BACKEND=docker  # or 'libvirt'

# GitHub Actions (automatic)
GITHUB_TOKEN=<auto-provided>
GITHUB_ACTOR=<auto-provided>
GITHUB_REPOSITORY=<auto-provided>
```

### Optional

```bash
# Codecov
CODECOV_TOKEN=<optional-for-private-repos>
```

## Local Testing

### Run Tests Locally

```bash
# Run all tests (Docker backend)
make test

# Run with coverage
go test -race -coverprofile=coverage.out -covermode=atomic ./...

# Run integration tests
DATABASE_URL=postgres://cloud:cloud@localhost:5432/thecloud?sslmode=disable \
  go test -tags=integration -v ./internal/repositories/postgres/...

# Run Libvirt tests (requires libvirt installed)
go test -v ./internal/repositories/libvirt/...
go run scripts/test_libvirt.go
```

### Simulate CI Locally

```bash
# Install act (GitHub Actions local runner)
brew install act  # or: curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash

# Run CI pipeline locally
act push

# Run specific job
act -j test
act -j test-libvirt
```

## Deployment Process

### Staging Deployment

1. Merge PR to `main` branch
2. CI pipeline runs automatically
3. On success, image is pushed to `ghcr.io/REPO:staging`
4. Staging environment pulls latest image
5. Automatic deployment (if configured)

### Production Deployment

1. Create a version tag:
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```
2. CI pipeline runs automatically
3. On success, images are pushed with multiple tags
4. Production environment pulls `latest` or specific version
5. Manual approval required (if configured)

## Monitoring & Alerts

### Build Status

- GitHub Actions UI: `https://github.com/REPO/actions`
- Status badge in README.md
- Email notifications on failure

### Coverage Reports

- Codecov dashboard: `https://codecov.io/gh/REPO`
- Coverage badge in README.md
- PR comments with coverage diff

### Security Scans

- Trivy scan results in Actions logs
- Vulnerability reports (non-blocking currently)
- Dependabot alerts for dependencies

## Troubleshooting

### Common Issues

#### 1. Libvirt Tests Failing

**Symptom**: `test-libvirt` job fails with permission errors

**Solution**:
```yaml
# Ensure proper permissions in CI config
sudo chmod 666 /var/run/libvirt/libvirt-sock
```

#### 2. Network Already Exists

**Symptom**: `network 'default' already exists`

**Solution**:
```bash
# Add cleanup step before network creation
sudo virsh net-destroy default 2>/dev/null || true
sudo virsh net-undefine default 2>/dev/null || true
```

#### 3. Storage Pool Issues

**Symptom**: `Storage pool not found`

**Solution**:
```bash
# Verify pool is active
sudo virsh pool-list --all
sudo virsh pool-refresh default
```

#### 4. Swagger Out of Sync

**Symptom**: CI fails on "Verify Swagger" step

**Solution**:
```bash
# Regenerate swagger docs locally
swag init -g cmd/api/main.go --output docs/swagger
git add docs/swagger
git commit -m "Update swagger docs"
```

## Best Practices

### 1. Branch Protection

Enable branch protection on `main`:
- ✅ Require status checks to pass
- ✅ Require `test` and `test-libvirt` jobs
- ✅ Require up-to-date branches
- ✅ Require code review

### 2. Secrets Management

Store sensitive data in GitHub Secrets:
- `CODECOV_TOKEN` (if using private repo)
- `DOCKER_HUB_TOKEN` (if pushing to Docker Hub)
- `SLACK_WEBHOOK` (for notifications)

### 3. Caching

The pipeline uses Go module caching:
```yaml
uses: actions/setup-go@v5
with:
  cache: true
```

### 4. Parallel Execution

Jobs run in parallel when possible:
- `test` and `test-libvirt` run simultaneously
- `build` waits for both to complete

### 5. Fast Feedback

- Lint and unit tests run first (fastest)
- Integration tests run after
- Build and deploy only on success

## Future Enhancements

### Planned Improvements

1. **Multi-Architecture Builds**
   - Build for AMD64 and ARM64
   - Use Docker buildx

2. **E2E Testing**
   - Full API integration tests
   - Browser-based UI tests (Playwright)

3. **Performance Testing**
   - Load testing with k6
   - Benchmark comparisons (Docker vs Libvirt)

4. **Security Hardening**
   - Make Trivy scan blocking
   - Add SAST scanning (CodeQL)
   - Container signing (cosign)

5. **Deployment Automation**
   - Kubernetes manifests
   - Helm charts
   - ArgoCD integration

6. **Notifications**
   - Slack integration
   - Discord webhooks
   - Email alerts

## References

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Libvirt CI Testing](https://libvirt.org/ci.html)
- [Docker Build Best Practices](https://docs.docker.com/develop/dev-best-practices/)
- [Codecov Documentation](https://docs.codecov.com/)
