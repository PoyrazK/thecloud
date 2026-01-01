# Mini AWS 🚀

To build the world's best local-first cloud simulator that teaches cloud concepts through practice.

## ✨ Features
- **Compute**: Docker-based instance management (Launch, Stop, List)
- **Storage**: S3-compatible object storage (Upload, Download, Delete)
- **Identity**: API Key authentication

## 🚀 Quick Start
```bash
# 1. Clone & Setup
git clone https://github.com/PoyrazK/Mini_AWS.git
cd Mini_AWS
make run

# 2. Test health
curl localhost:8080/health

# 3. Get an API Key
cloud auth create-demo my-user

# 4. Launch an instance
cloud compute launch --name my-server --image nginx:alpine

# 5. Upload a file
cloud storage upload my-bucket README.md
```

## 🏗️ Architecture
- **Backend**: Go (Clean Architecture)
- **Database**: PostgreSQL (pgx)
- **Infrastructure**: Docker Engine
- **CLI**: Cobra (command-based) + Survey (interactive)

## 📚 Documentation

### 🎓 Getting Started
| Doc | Description |
|-----|-------------|
| [Development Guide](docs/development.md) | Setup on Windows, Mac, or Linux |
| [Roadmap](docs/roadmap.md) | Project phases and progress |

### 📖 How-to Guides
| Guide | What you'll learn |
|-------|-------------------|
| [Storage Guide](docs/guides/storage.md) | Upload, download, and manage files |

### 🔧 Reference
| Reference | Contents |
|-----------|----------|
| [CLI Reference](docs/cli-reference.md) | All commands and flags |
| [Database Guide](docs/database.md) | Schema, tables, and migrations |

### 🏛️ Architecture
| Doc | Description |
|-----|-------------|
| [Architecture Overview](docs/architecture.md) | System design and patterns |
| [Backend Guide](docs/backend.md) | Go service implementation |
| [Infrastructure](docs/infrastructure.md) | Docker and deployment |

## 📊 KPIs
- Time to Hello World: < 5 min
- API Latency (P95): < 200ms
- CLI Success Rate: > 95%
