# The Cloud 🚀

To build the world's best open-source cloud platform that anyone can run, modify, and own.

## ✨ Features
- **Compute**: Docker-based instance management (Launch, Stop, Terminate, Stats)
- **Storage**: S3-compatible object storage (Upload, Download, Delete)
- **Block Storage**: Persistent volumes that survive instance termination
- **Networking**: VPC with isolated Docker networks
- **Identity**: API Key authentication ([Guide](docs/guides/authentication.md))
- **Observability**: Real-time CPU/Memory metrics and System Events
- **Load Balancer**: Layer 7 HTTP traffic distribution
- **Auto-Scaling**: Dynamic scaling of compute resources based on metrics
- **Managed Databases (RDS)**: Launch PostgreSQL/MySQL instances with a single command ([Guide](docs/guides/rds.md))
- **Managed Caches (Redis)**: Launch and manage Redis instances ([Guide](docs/guides/cache.md))
- **Cloud Functions (Serverless)**: Run logic without managing servers ([Guide](docs/guides/functions.md))
- **Secrets Manager**: Encrypted storage for API keys and sensitive config ([Guide](docs/guides/secrets.md))
- **CloudQueue**: Distributed message queuing with visibility timeouts ([Guide](docs/services/cloud-queue.md))
- **CloudNotify**: Pub/Sub topic and subscription service via Webhooks/Queues ([Guide](docs/services/cloud-notify.md))
- **CloudCron**: Managed scheduled tasks with execution history ([Guide](docs/services/cloud-cron.md))
- **CloudGateway**: API Routing and Reverse Proxy with path stripping ([Guide](docs/services/cloud-gateway.md))
- **CloudContainers**: Managed container deployments with replication and auto-healing ([Guide](docs/services/cloud-containers.md))
- **Console**: Interactive Next.js Dashboard for visual resource management

## 🔐 Authentication
The Cloud uses API Key authentication.
1. **Register**: `POST /auth/register` to create an account.
2. **Login**: `POST /auth/login` to receive your API Key.
3. **Authenticate**: Send `X-API-Key: <your-key>` header with all requests.

## 🩺 System Health
- **Liveness**: `GET /health/live` (Returns 200 OK)
- **Readiness**: `GET /health/ready` (Returns 200 if DB/Docker connected, 503 if not)

## 🚀 Quick Start (Backend)
```bash
# 1. Clone & Setup
git clone https://github.com/PoyrazK/thecloud.git
cd thecloud
make run

# 2. Register & Login (Get API Key)
curl -X POST http://localhost:8080/auth/register \
  -d '{"email":"user@example.com", "password":"password", "name":"User"}'

curl -X POST http://localhost:8080/auth/login \
  -d '{"email":"user@example.com", "password":"password"}'
# Copy the "api_key" from the response

# 3. Test Access
# All protected endpoints require the X-API-Key header
curl -H "X-API-Key: YOUR_API_KEY" http://localhost:8080/instances
```

## 🎮 Quick Start (Console - Frontend)
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

## 🏗️ Architecture
- **Frontend**: Next.js 14, Tailwind CSS, GSAP
- **Backend**: Go (Clean Architecture, Hexagonal)
- **Database**: PostgreSQL (pgx)
- **Infrastructure**: Docker Engine (Containers, Networks, Volumes)
- **Observability**: Prometheus Metrics & Real-time WebSockets
- **CLI**: Cobra (command-based) + Survey (interactive)

##  Documentation

### 🎓 Getting Started
| Doc | Description |
|-----|-------------|
| [Development Guide](docs/development.md) | Setup on Windows, Mac, or Linux |
| [Roadmap](docs/roadmap.md) | Project phases and progress |
| [Future Plans & Contributing](docs/future-plans.md) | How to contribute + feature backlog |
| [Future Vision](docs/vision.md) | Long-term strategy and goals |

### 🏛️ Architecture & Services
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

### 📖 Guides
| Guide | Description |
|-------|-------------|
| [Auto-Scaling](docs/guides/autoscaling.md) | Scalability patterns and usage |
| [Load Balancer](docs/guides/loadbalancer.md) | Traffic distribution guide |
| [Managed Databases](docs/guides/rds.md) | RDS patterns and usage |
| [Secrets Manager](docs/guides/secrets.md) | Security and encryption guide |
| [Networking](docs/guides/networking.md) | VPCs and Network isolation |
| [Storage](docs/guides/storage.md) | Object and Block storage |
| [Managed Caches](docs/guides/cache.md) | Redis cache management |
| [Cloud Functions](docs/guides/functions.md) | Serverless execution |

## 📊 KPIs
- Time to Hello World: < 5 min
- API Latency (P95): < 200ms
- CLI Success Rate: > 95%
