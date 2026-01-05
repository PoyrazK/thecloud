# Production Deployment Checklist - v0.2.0

**Release:** v0.2.0  
**Date:** 2026-01-05  
**Status:** 🚀 Ready to Deploy

---

## ✅ Pre-Deployment Checklist

### Code & Tests
- [x] All tests passing (unit + integration)
- [x] Code review complete
- [x] Linting passed
- [x] Swagger docs up to date
- [x] Migration rollback tested

### Infrastructure
- [x] Kubernetes manifests created
- [x] Docker images build successfully
- [x] Resource limits defined
- [x] Health probes configured
- [x] Auto-scaling configured (HPA)
- [x] Monitoring stack ready

### Security
- [x] Secrets encrypted
- [x] Rate limiting configured
- [x] SSL/TLS ready (cert-manager)
- [x] Security headers configured
- [x] Non-root containers

### Documentation
- [x] Deployment guide (docs/DEPLOYMENT.md)
- [x] API reference updated
- [x] README updated
- [x] Configuration documented

---

## 🚢 Deployment Options

### Option 1: Docker Compose (Recommended for Quick Start)

```bash
# 1. Clone repository (on production server)
git clone https://github.com/PoyrazK/thecloud.git
cd thecloud
git checkout v0.2.0

# 2. Create .env file
cp .env.example .env
# Edit .env with production values

# 3. Generate secrets
export SECRETS_ENCRYPTION_KEY=$(openssl rand -hex 32)
export REDIS_PASSWORD=$(openssl rand -hex 16)
export GRAFANA_PASSWORD=$(openssl rand -hex 16)

# Update .env file with these values

# 4. Deploy full stack
docker-compose -f docker-compose.yml -f docker-compose.prod-full.yml up -d

# 5. Verify deployment
docker-compose ps
curl http://localhost:8080/health/ready
```

**Access:**
- API: http://your-server:8080
- Grafana: http://your-server:3000
- Prometheus: http://your-server:9090

### Option 2: Kubernetes (For Production Clusters)

```bash
# 1. Create namespace
kubectl create namespace thecloud

# 2. Create secrets
kubectl create secret generic api-secrets \
  --from-literal=SECRETS_ENCRYPTION_KEY=$(openssl rand -hex 32) \
  -n thecloud

# 3. Update ConfigMap (if needed)
kubectl apply -f k8s/configmap.yaml

# 4. Deploy database
kubectl apply -f k8s/db-deployment.yaml
kubectl apply -f k8s/service.yaml

# Wait for DB to be ready
kubectl wait --for=condition=ready pod -l app=postgres -n thecloud --timeout=120s

# 5. Run migrations
kubectl run migration --rm -it \
  --image=ghcr.io/poyrazk/thecloud:v0.2.0 \
  --restart=Never \
  -n thecloud \
  --env="DATABASE_URL=postgres://cloud:cloud@postgres:5432/thecloud?sslmode=disable" \
  -- /app/thecloud -migrate-only

# 6. Deploy API
kubectl apply -f k8s/api-deployment.yaml

# 7. Deploy supporting resources
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/ingress.yaml
kubectl apply -f k8s/hpa.yaml
kubectl apply -f k8s/pdb.yaml

# 8. Verify deployment
kubectl get pods -n thecloud
kubectl get hpa -n thecloud
kubectl get ingress -n thecloud
```

**Access:**
- API: https://api.thecloud.example.com (via Ingress)
- Grafana: Port-forward or expose via separate Ingress

---

## 📋 Post-Deployment Verification

### 1. Health Checks
```bash
# Liveness
curl http://your-api/health/live

# Readiness (checks DB + Docker)
curl http://your-api/health/ready

# Expected: {"status":"UP","checks":{...}}
```

### 2. Authentication Test
```bash
# Register
curl -X POST http://your-api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"TestPass123!","name":"Test User"}'

# Login
curl -X POST http://your-api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"TestPass123!"}'
  
# Save the API key from response
```

### 3. Protected Endpoint
```bash
# Test with API key
curl -H "X-API-Key: YOUR_API_KEY" http://your-api/instances
```

### 4. Monitoring
```bash
# Check Prometheus targets
curl http://your-server:9090/api/v1/targets

# Access Grafana
# Navigate to http://your-server:3000
# Login with credentials from .env
# Verify Prometheus datasource is connected
```

### 5. Auto-scaling (K8s only)
```bash
# Check HPA status
kubectl get hpa -n thecloud

# Should show current/target CPU and memory
# Min replicas: 2, Max: 10
```

---

## 🔧 Configuration

### Required Environment Variables

```bash
# Database
DATABASE_URL=postgres://user:password@host:5432/dbname?sslmode=disable

# Application
PORT=8080
APP_ENV=production
SECRETS_ENCRYPTION_KEY=<32-byte-hex-string>

# Optional: Redis (for caching)
REDIS_URL=redis://:password@redis:6379/0

# Optional: Monitoring
LOG_LEVEL=info
```

### SSL/TLS Setup (Nginx)

1. Obtain certificates (Let's Encrypt recommended)
2. Place in `nginx/ssl/`:
   - `fullchain.pem`
   - `privkey.pem`
3. Update `nginx/nginx.conf` with your domain
4. Restart nginx:
   ```bash
   docker-compose restart nginx
   ```

---

## 🔍 Monitoring & Observability

### Metrics Available
- HTTP request rate and latency
- Error rates by endpoint
- Active connections
- Database connection pool
- CPU and memory usage
- Go runtime metrics

### Grafana Dashboards
1. Login to Grafana (default: admin/admin)
2. Navigate to Dashboards
3. Import pre-built dashboards or create custom ones
4. Prometheus datasource is auto-configured

### Logs
```bash
# Docker Compose
docker-compose logs -f api
docker-compose logs -f postgres

# Kubernetes
kubectl logs -f deployment/thecloud-api -n thecloud
kubectl logs -f statefulset/postgres -n thecloud
```

---

## 🚨 Troubleshooting

### API Not Starting
```bash
# Check logs
docker logs cloud-api --tail 100

# Common issues:
# - Database not ready: Wait for postgres health check
# - Missing secrets: Verify SECRETS_ENCRYPTION_KEY is set
# - Docker socket: Ensure /var/run/docker.sock is mounted
```

### Database Connection Errors
```bash
# Test database connectivity
docker exec cloud-db psql -U cloud -d thecloud -c "SELECT 1"

# Check DATABASE_URL format
# Should be: postgres://user:pass@host:5432/dbname?sslmode=disable
```

### HPA Not Scaling (K8s)
```bash
# Check metrics server
kubectl top nodes
kubectl top pods -n thecloud

# If metrics unavailable, install metrics-server:
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

### Ingress Not Working
```bash
# Check Ingress status
kubectl describe ingress thecloud-ingress -n thecloud

# Ensure Nginx Ingress Controller is installed
# Install if needed:
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.1/deploy/static/provider/cloud/deploy.yaml
```

---

## 📊 Success Criteria

- [ ] API health check returns 200 OK
- [ ] Can register and login
- [ ] Protected endpoints return 401 without API key
- [ ] Protected endpoints return data with valid API key
- [ ] Grafana shows incoming metrics
- [ ] (K8s) HPA shows current metrics
- [ ] (K8s) At least 2 API pods running
- [ ] No error logs in application
- [ ] Database migrations completed successfully

---

## 🔄 Rollback Procedure

### Docker Compose
```bash
# Stop current deployment
docker-compose down

# Deploy previous version
git checkout v0.1.0
docker-compose -f docker-compose.yml -f docker-compose.prod-full.yml up -d
```

### Kubernetes
```bash
# Rollback deployment
kubectl rollout undo deployment/thecloud-api -n thecloud

# Or to specific revision
kubectl rollout history deployment/thecloud-api -n thecloud
kubectl rollout undo deployment/thecloud-api --to-revision=2 -n thecloud
```

---

## 📞 Support & Contact

- **Documentation**: `docs/DEPLOYMENT.md`
- **API Reference**: `docs/api-reference.md`
- **Issues**: GitHub Issues
- **Repository**: https://github.com/PoyrazK/thecloud

---

## 🎉 Deployment Complete!

**Next Steps:**
1. Monitor Grafana dashboards for 24 hours
2. Check error rates and latency
3. Verify auto-scaling behavior under load
4. Set up alerting (future)
5. Configure backups (future)

**Version:** v0.2.0  
**Deployed:** $(date)  
**Status:** ✅ Production Ready
