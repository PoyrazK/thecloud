# DevOps & Deployment Guide

This guide covers deploying The Cloud to Kubernetes or Docker Compose production environment.

---

## 📦 Kubernetes Deployment

### Prerequisites
- Kubernetes cluster (1.24+)
- `kubectl` configured
- Nginx Ingress Controller installed
- cert-manager for TLS (optional)

### Quick Deploy

```bash
# Create namespace
kubectl apply -f k8s/namespace.yaml

# Apply all manifests
kubectl apply -f k8s/

# Or use Kustomize (when implemented)
kubectl apply -k k8s/overlays/production
```

### Components

| File | Description |
|------|-------------|
| `namespace.yaml` | Creates `thecloud` namespace |
| `api-deployment.yaml` | API deployment with 2+ replicas |
| `db-deployment.yaml` | PostgreSQL StatefulSet |
| `service.yaml` | ClusterIP services for API & DB |
| `ingress.yaml` | Nginx Ingress with TLS |
| `hpa.yaml` | Horizontal Pod Autoscaler (CPU/Memory) |
| `pdb.yaml` | Pod Disruption Budgets for HA |
| `configmap.yaml` | Environment configuration |
| `secrets.yaml` | Sensitive credentials |

### Resource Requirements

**API Pods:**
- Requests: 500m CPU, 512Mi memory
- Limits: 1000m CPU, 1Gi memory
- Min replicas: 2, Max: 10 (HPA)

**PostgreSQL:**
- Requests: 1000m CPU, 1Gi memory
- Limits: 2000m CPU, 2Gi memory
- PVC: 20Gi

### Monitoring

The API exposes Prometheus metrics at `/metrics`:
- HTTP request durations
- Active connections
- Go runtime metrics

### Scaling

Horizontal scaling is automatic via HPA:
- Target CPU: 70%
- Target Memory: 80%
- Scale up: Fast (2 pods every 15s)
- Scale down: Slow (50% every 60s, min 5min stabilization)

---

## 🐳 Docker Compose Production

### Prerequisites
- Docker 20.10+
- Docker Compose V2

### Deployment Options

#### Option 1: Basic Production (existing compose + overlay)
```bash
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

#### Option 2: Full Stack with Monitoring
```bash
docker-compose -f docker-compose.yml -f docker-compose.prod-full.yml up -d
```

### What's Included (Full Stack)

| Service | Port | Description |
|---------|------|-------------|
| **nginx** | 80, 443 | Reverse proxy with SSL/TLS |
| **api** | 8080 | The Cloud API (internal) |
| **postgres** | 5432 | Database (internal) |
| **redis** | 6379 | Cache layer |
| **prometheus** | 9090 | Metrics collection |
| **grafana** | 3000 | Monitoring dashboards |
| **node-exporter** | 9100 | System metrics |

### Configuration

Create `.env` file:
```bash
# Database
DB_USER=cloud
DB_PASSWORD=<strong-password>
DB_NAME=thecloud

# Redis
REDIS_PASSWORD=<strong-password>

# API
PORT=8080
SECRETS_ENCRYPTION_KEY=<32-byte-hex-key>

# Grafana
GRAFANA_USER=admin
GRAFANA_PASSWORD=<strong-password>
```

### SSL/TLS Setup

1. Generate or obtain SSL certificates
2. Place in `nginx/ssl/`:
   - `fullchain.pem`
   - `privkey.pem`
3. Update `nginx/nginx.conf` with your domain

### Resource Limits

Docker Compose includes resource limits:
- **API**: 2 CPU, 2GB RAM
- **PostgreSQL**: 2 CPU, 2GB RAM
- **Redis**: 0.5 CPU, 512MB RAM

### Health Checks

All services have health checks:
- API: `/health/live` every 30s
- PostgreSQL: `pg_isready` every 10s
- Redis: `redis-cli ping` every 10s

---

## 🔧 Configuration

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DB_URL` | Yes | PostgreSQL connection string |
| `PORT` | No | API port (default: 8080) |
| `APP_ENV` | Yes | `development` or `production` |
| `SECRETS_ENCRYPTION_KEY` | Yes | 32-byte hex for secret encryption |
| `REDIS_URL` | No | Redis connection (if using cache) |
| `LOG_LEVEL` | No | `debug`, `info`, `warn`, `error` |

### Secrets Management

**Kubernetes:**
```bash
kubectl create secret generic api-secrets \
  --from-literal=SECRETS_ENCRYPTION_KEY=$(openssl rand -hex 32) \
  -n thecloud
```

**Docker Compose:**
Add to `.env` file (never commit!)

---

## 📊 Monitoring & Observability

### Accessing Grafana

1. Navigate to `http://localhost:3000`
2. Login with credentials from `.env`
3. Prometheus datasource is auto-configured

### Pre-configured Dashboards

- API Request Rate & Latency
- Error Rates by Endpoint
- Resource Usage (CPU/Memory)
- Database Connections
- Go Runtime Metrics

### Alerting (TODO)

Future: AlertManager integration for:
- High error rates
- Resource exhaustion
- Service downtime

---

## 🚀 CI/CD Integration

### GitHub Actions Workflows

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `ci.yml` | Push, PR | Tests, lint, coverage |
| `release.yml` | Tag `v*` | Build & publish binaries |

### Container Registry

Images are pushed to GitHub Container Registry:
```
ghcr.io/poyrazk/thecloud:latest      # Production
ghcr.io/poyrazk/thecloud:staging     # Staging
ghcr.io/poyrazk/thecloud:<sha>       # Commit-specific
ghcr.io/poyrazk/thecloud:v1.0.0      # Tagged releases
```

---

## 🔒 Security Best Practices

1. **Never commit secrets** - Use environment variables
2. **Enable TLS** - Use Let's Encrypt or valid certificates
3. **Rate limiting** - Nginx enforces per-IP limits
4. **Resource limits** - Prevent resource exhaustion
5. **Network policies** - Isolate services (K8s)
6. **Regular updates** - Keep base images updated
7. **Scan images** - Trivy runs in CI/CD

---

## 📝 Maintenance

### Database Backups

```bash
# Manual backup
docker exec cloud-db pg_dump -U cloud thecloud > backup.sql

# Scheduled backups (add to cron)
0 2 * * * docker exec cloud-db pg_dump -U cloud thecloud | gzip > /backups/$(date +\%Y\%m\%d).sql.gz
```

### Log Rotation

Docker Compose includes log rotation:
- Max size: 100MB per file
- Max files: 3
- Total: ~300MB per service

### Updates

```bash
# Pull latest images
docker-compose pull

# Restart with zero downtime (with 2+ replicas)
docker-compose up -d --no-deps --build api
```

---

## 🐛 Troubleshooting

### Pod not starting (K8s)
```bash
kubectl describe pod <pod-name> -n thecloud
kubectl logs <pod-name> -n thecloud
```

### Service unreachable
```bash
# Check service endpoints
kubectl get endpoints -n thecloud

# Test internal connectivity
kubectl run -it --rm debug --image=busybox --restart=Never -n thecloud -- wget -O- http://thecloud-api:8080/health/live
```

### Database connection issues
```bash
# Test DB connection
docker exec cloud-db psql -U cloud -d thecloud -c "SELECT 1"

# Check API logs
docker logs cloud-api --tail 100
```

---

## 📚 Additional Resources

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Docker Compose Reference](https://docs.docker.com/compose/)
- [Prometheus Queries](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Grafana Dashboards](https://grafana.com/grafana/dashboards/)

---

**Status:** Production Ready ✅  
**Last Updated:** 2026-01-05
