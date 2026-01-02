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
- **Console**: Interactive Next.js Dashboard for visual resource management

## 🚀 Quick Start (Backend)
```bash
# 1. Clone & Setup
git clone https://github.com/PoyrazK/thecloud.git
cd thecloud
make run

# 2. Register & Login (Get API Key)
curl -X POST http://localhost:8080/auth/register -d '{"email":"user@example.com", "password":"password", "name":"User"}'
curl -X POST http://localhost:8080/auth/login -d '{"email":"user@example.com", "password":"password"}'
# Copy the "token" from the response

# 3. Test Access
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
- **Frontend**: Next.js 16, Tailwind CSS, GSAP
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

### 🤝 Community
| Doc | Description |
|-----|-------------|
| [Contributing Guide](CONTRIBUTING.md) | How to propose changes and run checks |
| [Security Policy](SECURITY.md) | How to report vulnerabilities |

### 🏛️ Architecture & Services
| Doc | Description |
|-----|-------------|
| [Architecture Overview](docs/architecture.md) | System design and patterns |
| [Backend Guide](docs/backend.md) | Go service implementation |
| [Database Guide](docs/database.md) | Schema, tables, and migrations |
| [CLI Reference](docs/cli-reference.md) | All commands and flags |

### 📖 Guides
| Guide | Description |
|-------|-------------|
| [Auto-Scaling](docs/guides/autoscaling.md) | Scalability patterns and usage |
| [Load Balancer](docs/guides/loadbalancer.md) | Traffic distribution guide |
| [Networking](docs/guides/networking.md) | VPCs and Network isolation |
| [Storage](docs/guides/storage.md) | Object and Block storage |

## 📊 KPIs
- Time to Hello World: < 5 min
- API Latency (P95): < 200ms
- CLI Success Rate: > 95%
