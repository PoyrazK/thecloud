# 💻 Cross-Platform Development Guide

Welcome to the **The Cloud** operational manual. This guide will help you set up your development environment on **macOS** and **Windows**.

---

## 🍎 Developing on macOS

### 1. Prerequisites
- **Homebrew**: The missing package manager for macOS.
  ```bash
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  ```
- **Go (Golang)**: Version 1.24 or higher (matches go.mod).
  ```bash
  brew install go
  ```
- **Node.js & npm**: Version 20 or higher (for the Console).
  ```bash
  brew install node
  ```
- **Docker Desktop**: [Download for Mac](https://www.docker.com/products/docker-desktop/).
  - *Note*: Ensure "Use Docker Compose V2" is enabled in settings.
- **Git**: `brew install git`

### 2. Setup
1.  **Clone the Repository**:
    ```bash
    git clone https://github.com/PoyrazK/thecloud.git
    cd thecloud
    ```

2.  **Environment Variables**:
    Create a `.env` file in the root directory:
    ```bash
    echo "DATABASE_URL=postgres://cloud:password@localhost:5432/thecloud" > .env
    ```

### 3. Running the Project
The `Makefile` works natively on macOS.

- **Start Infrastructure**:
  ```bash
  make run
  ```
- **Build CLIs**:
  ```bash
  make build
  ```
- **Run Tests**:
  ```bash
  make test
  ```

---

## 🪟 Developing on Windows

### 1. Prerequisites
- **Go (Golang)**: [Download Installer](https://go.dev/dl/).
- **Node.js & npm**: [Download Installer](https://nodejs.org/).
- **Docker Desktop**: [Download for Windows](https://www.docker.com/products/docker-desktop).
  - *Critical*: Enable **WSL 2** (Windows Subsystem for Linux) backend for best performance.
- **Git Bash** (Recommended) or PowerShell.
- **Make**: Windows doesn't have `make` by default.
  - Option A: Install via Chocolatey: `choco install make`
  - Option B: Use `mingw32-make` if you have MinGW.
  - Option C: Just run the commands manually (see below).

### 2. Setup
1.  **Clone the Repository**:
    ```powershell
    git clone https://github.com/PoyrazK/thecloud.git
    cd thecloud
    ```

2.  **Environment Variables**:
    Create a `.env` file manually or via PowerShell:
    ```powershell
    Set-Content .env "DATABASE_URL=postgres://cloud:password@localhost:5432/thecloud"
    ```

### 3. Running the Project (Manual / PowerShell)
If you don't have `make`, run these commands:

- **Start Database**:
  ```powershell
  docker compose up -d
  ```

- **Run API**:
  ```powershell
  go run cmd/api/main.go
  ```

- **Build CLI (PowerShell)**:
  ```powershell
  mkdir bin
  # Build the CLI binaries using the project's cmd/cloud entrypoints
  go build -o bin/cloud.exe cmd/cloud/*.go
  ```

### 🛑 Common Windows Issues
1.  **"make: command not found"**: See "Prerequisites" above, or use manual commands.
2.  **Firewall**: Allow Docker access when prompted by Windows Defender.
3.  **Line Endings**: Git might change LF to CRLF. Configure git to handle this:
    ```bash
    git config --global core.autocrlf true
    ```


---

## 🧪 Testing

The Cloud has comprehensive test coverage (59.7%) across all layers:

### Running Tests

**Unit Tests Only** (no database required):
```bash
go test ./...
```

**Integration Tests** (requires PostgreSQL):
```bash
# Start PostgreSQL first
docker compose up -d postgres

# Run all tests including integration tests
go test -tags=integration ./...
```

**Coverage Report**:
```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...

# View in browser
go tool cover -html=coverage.out

# View summary
go tool cover -func=coverage.out | grep total
```

### Test Organization
- **Unit Tests**: `internal/core/services/*_test.go`, `internal/handlers/*_test.go`
- **Integration Tests**: `internal/repositories/postgres/*_test.go`
- **SDK Tests**: `pkg/sdk/*_test.go` (use httptest for HTTP mocking)
- **Mocks**: `internal/core/services/shared_test.go`

### Current Coverage
- **Overall**: 59.7%
- **SDK**: 80.1%
- **Services**: 71.5%
- **Handlers**: 65.8%
- **Repositories**: 70.1%

For detailed testing guide, see [docs/TESTING.md](./TESTING.md).

---

## 🐳 Docker Note (All OS)
Ensure your Docker Daemon is running before starting the project!
- **Mac**: Look for the whale icon in the menu bar.
- **Windows**: Look for the whale icon in the system tray.
