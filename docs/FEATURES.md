# 🌟 The Cloud - Features & Implementation Deep Dive

This document provides a comprehensive overview of every feature currently implemented in **The Cloud**, including the technologies used and the architectural details of how they are built.

---

## 🏗️ Core Infrastructure

### 1. Compute (Instances)
**What it is**: Launch virtual machines (simulated as containers) to run applications.
**Tech Stack**: Docker Engine, Go (Backend).
**Implementation**:
- **Simulation**: Instead of full VMs (KVM), we use **Docker Containers** to simulate instances. This allows sub-second boot times and low resource overhead.
- **Isolation**: Each "Instance" is a Docker container.
- **Networking**: Instances are attached to custom Bridge Networks (simulating VPCs).
- **Persistence**: Usage of Docker Volumes for persistent storage.
- **Lifecycle**: The `InstanceService` manages the Docker API to Create, Start, Stop, and Remove containers.

### 2. Networking (VPC)
**What it is**: Isolated virtual networks to secure resources.
**Tech Stack**: Docker Networks (Bridge Driver).
**Implementation**:
- **Abstraction**: A "VPC" maps directly to a **Docker Bridge Network**.
- **Isolation**: Containers in one VPC cannot communicate with containers in another (unless peered/exposed).
- **DNS**: Uses Docker's internal DNS for service discovery within the VPC (e.g., `ping my-db`).

### 3. Block Storage (Volumes)
**What it is**: Persistent disks that can be attached/detached from instances.
**Tech Stack**: Docker Volumes.
**Implementation**:
- **Creation**: Maps to `docker volume create`.
- **Attachment**: When "attaching" a volume to an instance, we currently have to recreate the container with the new bind mount (due to Docker limitations on live mounting).
- **Persistence**: Data survives container termination.

### 4. Object Storage (S3-compatible)
**What it is**: Store and retrieve files (blobs) via API.
**Tech Stack**: Go (IO/FS), Local Filesystem.
**Implementation**:
- **Storage Backend**: Files are stored in a dedicated local directory (`thecloud-data/storage`).
- **API**: Implements standard HTTP PUT/GET methods.
- **Streaming**: Uses `io.Reader/Writer` to stream data efficiently without loading entire files into RAM.

---

## 🛠️ Managed Services

### 5. Managed Databases (RDS)
**What it is**: Provision fully managed PostgreSQL or MySQL databases.
**Tech Stack**: Docker (Official Images), Go.
**Implementation**:
- **Provisioning**: Spawns a Docker container using official images (`postgres:alpine`, `mysql:debet`).
- **Configuration**: Automatically sets up internal credentials and exposes standard ports (5432/3306).
- **Persistence**: Automatically mounts a managed volume so data isn't lost on restart.

### 6. Managed Caches (CloudCache) 🆕
**What it is**: Provision fully managed Redis instances.
**Tech Stack**: Redis (Alpine), Go, Docker Exec.
**Implementation**:
- **Provisioning**: Launches `redis-server` with custom configuration (AOF enabled, RDB disabled).
- **Security**: Generates a random password and enables `--requirepass`.
- **Management**: Uses **Docker Exec** to run `redis-cli` commands (like `FLUSHALL` or `INFO`) safely inside the container for management and stats.

### 7. Cloud Functions (Serverless)
**What it is**: Run code snippets without provisioning servers.
**Tech Stack**: Docker (Ephemeral Containers), Go.
**Implementation**:
- **Architecture**: "One-shot" containers.
- **Execution**: When a function is invoked:
    1.  A temporary container is spun up with the requested runtime (e.g., `node:20`).
    2.  User code (injected via bind mount) is executed.
    3.  Output is captured, and the container is destroyed immediately context.
- **Async**: Supports background execution using Go routines.

### 8. Load Balancer (ELB)
**What it is**: Distribute incoming HTTP traffic across multiple instances.
**Tech Stack**: Go (Reverse Proxy `httputil`), Round Robin Algorithm.
**Implementation**:
- **Proxy**: A pure Go reverse proxy server listening on a specific port.
- **Routing**: Maintains a list of healthy targets (Instances).
- **Algorithm**: Distributes requests sequentially (Round Robin) to targets.
- **Health Checks**: Background workers ping targets to ensure availability.

### 9. Auto-Scaling
**What it is**: Automatically add/remove instances based on CPU load.
**Tech Stack**: Go Background Workers.
**Implementation**:
- **Metrics Loop**: A background worker polls Docker Stats for every instance in a scaling group.
- **Decision Engine**: Checks if average metric > target (e.g., CPU > 50%).
- **Scale Out**: Calls `InstanceService` to clone the template instance.
- **Scale In**: Terminates the oldest instance in the group.

### 10. Secrets Manager
**What it is**: Store sensitive data (API keys, passwords) securely.
**Tech Stack**: AES-GCM Encryption, Go `crypto/aes`.
**Implementation**:
- **Encryption**: Secrets are encrypted *before* writing to the database using AES-256-GCM.
- **Key Management**: Uses a master key derived from environment configuration.
- **Access**: Decryption only happens on-demand via the API.

---

## 🧩 Platform Services

### 11. Identity & Auth (IAM)
**What it is**: Secure access to the platform.
**Tech Stack**: JWT (JSON Web Tokens), BCrypt.
**Implementation**:
- **Passwords**: Hashed using `bcrypt` cost 12.
- **Tokens**: Stateless JWTs signed with HMAC-SHA256.
- **Middleware**: Go middleware validates the API Key or JWT on every authenticated route.

### 12. Observability
**What it is**: Monitor system health and logs.
**Tech Stack**: Docker API (Stats/Logs), WebSockets (`gorilla/websocket`).
**Implementation**:
- **Stats**: Real-time stream of Docker container stats (CPU/Mem/Net).
- **Logs**: Attaches to Docker container `stdout/stderr` streams.
- **Dashboard**: Pushes real-time updates to the frontend via WebSockets.

### 13. CLI (Command Line Interface)
**What it is**: Terminal tool to manage "The Cloud".
**Tech Stack**: Cobra (CLI framework), Viper (Config).
**Implementation**:
- **Structure**: Command-based (`cloud <resource> <action>`).
- **State**: Stateless client; talks to the Backend API via HTTP.

### 14. Console (Frontend)
**What it is**: Visual dashboard.
**Tech Stack**: Next.js 14, Tailwind CSS, TypeScript.
**Implementation**:
- **SSR**: Server-Side Rendering for main dashboard views.
- **Architecture**: Component-based modern React.

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
