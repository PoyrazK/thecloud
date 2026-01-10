# The Cloud - Kubernetes Deployment

## Prerequisites
- Docker Desktop with Kubernetes enabled using kind (Or minikube, k3s).

## Deployment Steps

### 1. Create Namespace
```bash
kubectl apply -f k8s/namespace.yaml
```

### 2. Configuration & Secrets
```bash
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secrets.yaml
```

### 3. Deploy Backing Services
```bash
# Postgres & PgBouncer
kubectl apply -f k8s/postgres.yaml
kubectl apply -f k8s/pgbouncer.yaml

# Redis Cluster
kubectl apply -f k8s/redis-cluster.yaml
```

### 4. Bootstrap Redis Cluster (One-time)
Wait for Redis pods to be ready (`kubectl get pods -n thecloud`), then run:
```bash
kubectl exec -it cloud-redis-0 -n thecloud -- redis-cli --cluster create \
  $(kubectl get pods -l app=cloud-redis -n thecloud -o jsonpath='{range.items[*]}{.status.podIP}:6379 {end}') \
  --cluster-replicas 1
```
*(Accept the configuration implementation when prompted)*

### 5. Deploy API & Workers
```bash
# Workers
kubectl apply -f k8s/worker-deployment.yaml

# API
kubectl apply -f k8s/api-deployment.yaml
kubectl apply -f k8s/api-service.yaml
kubectl apply -f k8s/api-hpa.yaml
```

### 6. Expose Service (Ingress)
```bash
kubectl apply -f k8s/ingress.yaml
```

## Scaling
The API is configured with HPA to auto-scale between 5 and 20 pods based on CPU.
```bash
kubectl get hpa -n thecloud
```
