# Infractl - Unified Infrastructure Management Platform

## Project Overview

**Infractl** is a unified platform designed to provide developers with a seamless experience for managing hybrid cloud and on-premises infrastructure. The core philosophy is simple: **developers should only need SSH credentials and a Git repository to deploy complete applications**. Infractl handles all infrastructure provisioning, dependency installation, and configuration.

### Vision

Infractl serves as an abstraction layer that orchestrates disparate infrastructure providers (cloud, on-premises, virtualization platforms, DNS services, databases, storage) into a single, intuitive CLI and web dashboard. Users can:

1. **Register Infrastructure Resources** - Define available computing, storage, database, networking, and secret management resources
2. **Catalog Applications** - Register applications via Git URLs or OCI container images
3. **Configure Deployment** - Specify resource requirements and dependencies for each application
4. **Deploy at Scale** - Execute deployments with a single command, managing all infrastructure requirements automatically

### Target Users

- **Infrastructure Teams** - Managing hybrid cloud/on-prem environments
- **DevOps/Platform Engineers** - Automating infrastructure provisioning
- **Developers** - Deploying applications without infrastructure expertise

---

## Project Architecture

### Technology Stack

| Component | Technology | Purpose |
|-----------|-----------|---------|
| CLI Tool | Go + Cobra | Command-line interface for infractl operations |
| API Backend | Go + gorilla/websocket | REST API + WebSocket support for real-time operations |
| Database | PostgreSQL | Persisting infrastructure inventory, deployments, users |
| Remote Execution | SSH (goph/v2) | Execute commands on remote systems |
| Virtualization | Proxmox VE API | Manage virtual machines and LXC containers |
| DNS Management | Cloudflare API | Dynamic DNS record management |
| Container Registry Integration | OCI/Docker | Deploy containerized applications |
| Authentication | JWT + OAuth2 (planned) | Secure API access |

### High-Level Data Flow

```
User/API Request
    ↓
Infractl CLI/Web UI
    ↓
infractl Command Handler (cmd/)
    ↓
Service Layer (deployer/, ssh/, proxmox/, providers/)
    ↓
Remote Execution (SSH) / API Calls
    ↓
Infrastructure Resources (VMs, Databases, Networks, etc.)
```

---

## Package Documentation

### Core Packages

#### **cmd/** - Command Handlers
*Location*: `/cmd/*.go`  
*Purpose*: Cobra command definitions and handlers for the infractl CLI

**Key Commands:**
- `root` - Root command definition and global flag setup
- `auth` - Authentication commands (JWT token generation, secret management)
- `database` - Database operations (create app DBs, install PostgreSQL, manage Goosey migrations)
- `deploy_*` - Application deployment commands (local systemd, remote systemd)
- `dns` - DNS record management (Cloudflare integration)
- `proxmox_*` - Proxmox VM and LXC container management
- `jwt` - JWT secret and token generation
- `cicd` - CI/CD integration and automation
- `pack` - Application packaging utilities
- `cluster_ssh` - Multi-host SSH operations

**Key Responsibilities:**
- Parse and validate command-line arguments using Cobra/Viper
- Coordinate between service layers
- Handle user I/O and formatting
- Execute business logic workflows

**Related Files:**
- Flag definitions and bindings
- Configuration file parsing (YAML, TOML, JSON)
- User prompt helpers

---

#### **ssh/** - Remote SSH Operations
*Location*: `/ssh/*.go`  
*Purpose*: SSH client abstraction and remote command execution

**Key Components:**
- `ssh_client.go` - SSH client initialization with multi-auth support (keys, agent, password)
- `remote_app_deployer.go` - Application deployment via SSH (systemd, user creation)
- `ssh_helpers.go` - Utility functions for SSH operations
- `ssh_errors.go` - SSH-specific error handling

**Capabilities:**
- Multi-authentication methods: SSH keys (Ed25519, RSA), SSH agent, password
- Known hosts management and verification
- Automatic key path resolution (~/.ssh/id_ed25519, ~/.ssh/id_rsa)
- SFTP file upload/download
- Remote command execution with environment variables
- Application deployment and lifecycle management

**Key Types:**
```go
RemoteAppDeploymentAgent {
    SshClient           // goph.Client for SSH operations
    SourceUtilsDir      // Local utilities directory
    DestinationUtilsDir // Remote installation directory
    EnvVars             // Environment variables for remote commands
}
```

**Used By**: deployer, cmd/deploy_* commands

---

#### **deployer/** - Application Deployment Orchestration
*Location*: `/deployer/*.go`  
*Purpose*: Orchestrate application deployment including systemd services, PostgreSQL setup, and post-deployment configuration

**Key Components:**
- `deployer.go` - Main deployment orchestration logic
- `postgres_installer.go` - Remote PostgreSQL installation and configuration
- `postgres_remote_access.go` - PostgreSQL network access configuration
- `archiver.go` - Application archiving and compression utilities

**Deployment Workflow:**
1. Archive application files
2. Upload to remote host via SSH/SFTP
3. Create system user for the application
4. Configure PostgreSQL (if needed)
5. Create systemd service file
6. Deploy remote utilities for ongoing management
7. Enable and start service

**Key Functions:**
- `DeployRemoteSystemdApp()` - Deploy application as systemd service
- `InstallPostgres()` - Install PostgreSQL on remote system
- `ConfigureRemoteAccess()` - Configure PostgreSQL for network connections

**Dependencies**: ssh/, internal/archiver, cmd/

---

#### **proxmox/** - Proxmox VE Virtualization Management
*Location*: `/proxmox/*.go`  
*Purpose*: Interact with Proxmox VE API for VM and LXC container management

**Key Components:**
- `client.go` - Thread-safe Proxmox API client with authentication
- `vm.go` - Virtual machine operations (list, get, create, delete, manage)
- `prox_lxc.go` - LXC container operations
- `node.go` - Proxmox node information and management
- `storage.go` - Storage pool and volume management
- `structs.go` - API request/response data structures

**Capabilities:**
- Query cluster resources and resource usage
- Create, list, and manage VMs
- Create and manage LXC containers
- Control VM lifecycle (start, stop, reboot)
- Configure storage pools
- Query node information and statistics

**Authentication Methods:**
- Username/password with realm specification (user@realm)
- Token-based authentication (user@realm!tokenid)

**Key Types:**
```go
Client {
    baseURL        // Proxmox API endpoint
    authMethod     // Password or Token
    authTicket     // Session token for API requests
    httpClient     // Configured HTTP client with TLS
}
```

**Used By**: cmd/proxmox_* commands, deployer

---

#### **vmmgr/** - Virtual Machine Manager
*Location*: `/vmmgr/*.go`  
*Purpose*: High-level VM management abstractions built on top of proxmox client

**Key Components:**
- `vmmgr.go` - VM manager interface and core logic
- `proxmox_manager.go` - Proxmox-specific implementations

**Responsibilities:**
- Unified interface for VM operations across different providers (currently Proxmox)
- Resource validation and allocation
- Batch operations for multiple VMs

**Used By**: cmd/proxmox_* commands

---

#### **providers/** - Cloud Provider Integrations
*Location*: `/providers/*.go`  
*Purpose*: Abstractions and utilities for cloud provider integrations

**Current Providers:**
- `cloudflare_utils/` - Cloudflare API utilities for DNS and other services

**Future Providers:**
- AWS (EC2, RDS, etc.)
- Azure (VMs, databases, networking)
- Google Cloud

---

#### **dbhelper/** - Database Helper Service
*Location*: `/dbhelper/*.go`  
*Purpose*: HTTP API and utilities for PostgreSQL database setup and script generation

**Key Components:**
- `dbhelper.go` - HTTP handlers for database helper operations
- `script_download_handler.go` - Download generated SQL scripts
- Kubernetes/ingress configuration files

**Endpoints:**
- Generate PostgreSQL setup scripts based on configuration
- Download SQL migration scripts
- Create application databases and users

**Used By**: db-helper-ui (frontend), API consumers

---

### Internal/Utility Packages

#### **internal/archiver/** - Archive Management
*Purpose*: Create and manage compressed archives for application deployment

**Functionality:**
- Compress directories to tar.gz format
- Extract archives on local and remote systems
- Validate archive integrity

**Used By**: deployer/, deployment commands

---

#### **internal/bumper/** - Version Management
*Purpose*: Semantic version bumping and Git operations

**Functionality:**
- Increment semantic versions (major, minor, patch)
- Tag Git commits with version numbers
- Push tags to remote repositories

**Used By**: CICD commands, release automation

---

#### **internal/git/** - Git Operations
*Purpose*: Clone, pull, and manage Git repositories

**Used By**: Application registration and updates

---

#### **internal/goose/** - Database Migrations
*Purpose*: Goose database migration management for PostgreSQL

**Functionality:**
- Run database migrations
- Track migration history
- Rollback migrations if needed

**Used By**: Database setup commands

---

#### **internal/deployment/** - Deployment Helpers
*Purpose*: Utilities for systemd service creation and deployment

**Subpackages:**
- `validate/` - User validation utilities
- `createuser/` - System user creation utilities

---

#### **internal/pretty/** - Output Formatting
*Purpose*: Format and colorize CLI output for better user experience

**Features:**
- Color-coded status messages
- Table formatting
- JSON/YAML pretty-printing

**Used By**: All cmd/* packages

---

#### **internal/type_helper/** - Type Utilities
*Purpose*: Type conversions and helpers

**Functionality:**
- Integer parsing and validation
- Type conversion utilities
- Error formatting

---

#### **bob/** - Binary Builder
*Location*: `/bob/the_builder.go`  
*Purpose*: Build and deploy Go binaries to remote systems

**Functionality:**
- Compile Go source code
- Deploy compiled binaries to remote hosts
- Set proper ownership and permissions
- Integration with systemd deployment

---

### Supporting Components

#### **remote_utils/** - Remote Utilities
*Location*: `/remote_utils/bin/`  
*Purpose*: Pre-built utilities deployed to remote systems

**Utilities:**
- `deploy-utils` - Systemd service deployment and validation
- `user-utils` - System user creation and management

These are embedded in the binary and deployed via SSH.

---

#### **internal/cors/** - CORS Handling
*Purpose*: Cross-Origin Resource Sharing configuration for API

**Used By**: API server, dbhelper

---

#### **internal/swaggerui/** - Swagger/OpenAPI Documentation
*Purpose*: Dynamic Swagger UI server for API documentation

---

## Key Workflows

### Workflow 1: Deploy Application to Remote Host

```
User runs: infractl deploy remote-systemd --app-name myapp --host 192.168.1.10 --app-dir ./myapp

1. cmd/deploy_remote_systemd_app.go
   └─> Parse flags and validate inputs
   
2. ssh.InitializeSshClient()
   └─> Establish SSH connection to remote host
       - Try SSH agent, fall back to key files
       - Verify host keys
   
3. deployer.DeployRemoteSystemdApp()
   └─> archiver.CreateArchive() - Compress application
   └─> ssh.Upload() - Copy app to remote host
   └─> ssh.RunCommand() - Create system user
   └─> ssh.RunCommand() - Create systemd service file
   └─> ssh.RunCommand() - Enable and start service
   
4. Return success/failure status to user
```

### Workflow 2: Create Application Database

```
User runs: infractl database new-appdb --host 10.2.10.248 --connect-ssh --ssh-user root

1. cmd/database_newappdb.go
   └─> Parse database configuration
   
2. If --connect-ssh flag:
   └─> ssh.InitializeSshClient() - Connect to remote host
   └─> Execute SQL via \`sudo -u postgres psql\`
       - CREATE DATABASE appdb
       - CREATE ROLE appuser
       - GRANT privileges
   
3. Else (direct TCP connection):
   └─> sql.Open() - Direct PostgreSQL connection
   └─> Execute same SQL statements
   
4. Return database details to user
```

### Workflow 3: Deploy VM via Proxmox

```
User runs: infractl proxmox vm create --node pve1 --vm-name web-server --template ubuntu2204

1. cmd/proxmox_vm_create.go
   └─> Parse VM configuration
   
2. proxmox.Client.CreateVM()
   └─> Call Proxmox API: POST /nodes/pve1/qemu
   └─> Configure: CPU, RAM, disk, network
   └─> Clone from template if specified
   
3. proxmox.Client.GetVM() - Check creation status
   
4. vmmgr - Register VM in local inventory
   
5. Return VM ID and connection info
```

### Workflow 4: Manage DNS Records

```
User runs: infractl dns crud create --name www.example.com --type A --value 192.168.1.100

1. cmd/dns_cf_crud.go
   └─> Load Cloudflare API credentials
   
2. providers/cloudflare_utils/
   └─> Initialize Cloudflare client
   
3. Call Cloudflare API
   └─> Create/Update/Delete DNS record
   └─> Handle zone lookups and record types
   
4. Return operation result
```

---

## Configuration

### Configuration Sources (in priority order)

1. **Command-line flags** (highest priority)
2. **Environment variables** (prefixed with `INFRACTL_`)
3. **Configuration files** (YAML, TOML, JSON)
4. **Default values** (lowest priority)

### Key Configuration Files

- `default.yaml` - Default configuration settings
- `pve.yaml` - Proxmox VE credentials and endpoints
- `dnsRecord.yaml` - DNS record batch operations
- `cobra.yaml` - Cobra CLI framework settings

### Environment Variables

```bash
# SSH Configuration
INFRACTL_SSH_USER=username
INFRACTL_SSH_KEY=/path/to/key
INFRACTL_SSH_PASSPHRASE=xxx
INFRACTL_SSH_AGENT=true
INFRACTL_SSH_PORT=22

# Proxmox
INFRACTL_PVE_HOST=192.168.1.100
INFRACTL_PVE_USER=root@pam
INFRACTL_PVE_PASSWORD=xxx

# Cloudflare
INFRACTL_CF_API_TOKEN=xxx
INFRACTL_CF_ZONE=example.com

# Database
INFRACTL_DB_HOST=localhost
INFRACTL_DB_PORT=5432
INFRACTL_DB_USER=postgres
INFRACTL_DB_PASSWORD=xxx

# API
INFRACTL_API_URL=http://localhost:8080
INFRACTL_API_TOKEN=xxx
```

---

## Building and Running

### Build from Source

```bash
# Build with remote utilities
make build

# Build without utilities
go build -o dist/infractl .

# Install to $HOME/go/bin
make install

# Build Docker image
docker build -t infractl:latest .
```

### Running the CLI

```bash
# View help
infractl --help

# List available commands
infractl -h

# Run a command with verbose output
infractl database new-appdb --help

# Use configuration file
infractl -c config.yaml database new-appdb ...
```

---

## API Integration

Infractl provides both a CLI and an HTTP API for programmatic access. The API is defined in the companion **go-infra** package.

### API Base Path
```
http://localhost:8080/api/v1/
```

### Key API Endpoints
- `POST /auth/login` - Authenticate user
- `GET /hosts` - List registered hosts
- `POST /hosts` - Register new host
- `GET /databases` - List databases
- `POST /databases` - Create application database
- `GET /applications` - List registered applications
- `POST /applications` - Register application
- `POST /applications/{id}/deploy` - Deploy application

### WebSocket Support
- `/ws/deployments/{id}` - Real-time deployment logs
- `/ws/ssh/{host_id}` - Interactive SSH terminal

---

## Testing

### Run Unit Tests
```bash
go test ./...
```

### Run Tests with Coverage
```bash
go test -cover ./...
```

### Build Test Artifacts
```bash
make build-test
```

---

## Security Considerations

### SSH Key Management
- Supports multiple key formats: Ed25519 (preferred), RSA
- SSH agent integration for secure key handling
- Automatic host key verification and tracking
- Passphrase support for encrypted keys

### Authentication
- JWT tokens for API access
- Support for multiple OAuth providers (planned)
- LDAP integration (planned)
- Multi-factor authentication support (planned)

### Network Security
- TLS/HTTPS for all API communications
- Certificate validation for remote connections
- Credential encryption in configuration files
- Secrets stored securely in go-infra database

### Audit Logging
- All operations logged with user, timestamp, and result
- Deployment activities tracked in database

---

## Troubleshooting

### Common Issues

#### SSH Authentication Failures
```bash
# Check SSH connectivity
ssh -vvv user@host

# List available SSH keys
ls -la ~/.ssh/

# Test specific key
ssh -i ~/.ssh/id_ed25519 -vvv user@host

# Verify SSH agent is running
echo $SSH_AUTH_SOCK
ssh-add -l
```

#### Proxmox Connection Issues
```
# Verify Proxmox credentials
# Check that user account exists: root@pam or user@pve
# Verify token credentials if using token auth
# Check network connectivity to Proxmox host
```

#### Database Connection Issues
```bash
# Test PostgreSQL connection
psql -h hostname -U username -d database_name

# Check if PostgreSQL is running
systemctl status postgresql
```

---

## Development

### Project Structure
```
infra-cli/
├── cmd/               # Command handlers
├── ssh/               # SSH abstractions
├── deployer/          # Deployment orchestration
├── proxmox/           # Proxmox API client
├── vmmgr/             # VM manager
├── providers/         # Cloud provider integrations
├── dbhelper/          # Database helper service
├── bob/               # Binary builder
├── internal/          # Internal utilities
│   ├── archiver/      # Archive utilities
│   ├── bumper/        # Version management
│   ├── git/           # Git operations
│   ├── goose/         # Database migrations
│   ├── deployment/    # Deployment helpers
│   ├── pretty/        # Output formatting
│   └── type_helper/   # Type utilities
├── remote_utils/      # Pre-built remote utilities
└── main.go            # Entry point
```

### Adding New Commands

1. Create `cmd/newcommand.go`:
```go
package cmd

var newCmd = &cobra.Command{
    Use:   "newcommand",
    Short: "Description",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Implementation
        return nil
    },
}

func init() {
    rootCmd.AddCommand(newCmd)
}
```

2. Add flags and initialization in `init()` function
3. Register in `root.go` by calling `newCmd` initialization

### Adding New Providers

1. Create directory under `providers/`:
```bash
mkdir -p providers/newprovider_utils
```

2. Implement provider client interface
3. Add commands to `cmd/` that use the new provider
4. Add configuration loading in `cmd/root.go`

### Testing SSH Locally

Use OpenSSH in test mode:
```bash
# Start SSH server locally
service ssh start

# Test connection
infractl cluster-ssh --target localhost --user currentuser
```

---

## Future Roadmap

### Planned Features

#### 1. SSO Integration
- GitHub OAuth integration
- Microsoft Entra 365 (Azure AD)
- Google OAuth
- LDAP authentication
- Multi-factor authentication (MFA)

#### 2. Frontend Enhancement
- React-based web dashboard (db-helper-ui)
- Full deployment UI with real-time progress
- Application registry with marketplace
- Infrastructure inventory visualization
- User and permission management

#### 3. Database Management
- User database registration workflow
- PostgreSQL secondary (standby) deployment
- Automatic WAL (Write-Ahead Logging) replication
- Database backup and restore
- Point-in-time recovery (PITR)

#### 4. Systemd Service Management
- Service and timer deployment
- Recurring job scheduling
- Job monitoring and alerts
- Log aggregation and analysis

#### 5. Multi-Cloud Support
- AWS EC2 and RDS integration
- Azure VM and database integration
- Google Cloud integration
- Kubernetes cluster management
- Cost optimization and reporting

#### 6. Advanced Features
- Application dependency graph visualization
- Automated rollback on deployment failure
- Canary deployments and blue-green deployments
- Service mesh integration (Istio)
- Policy enforcement and compliance checking
- Infrastructure as Code (Terraform) integration

---

## Contributing

### Development Environment Setup

```bash
# Clone repository
git clone <repo-url>
cd infra-cli

# Install dependencies
go mod download

# Build
make build

# Run tests
go test ./...
```

### Code Style

- Follow Go conventions (gofmt, golint)
- Write unit tests for new functionality
- Document public functions and packages
- Update this documentation when adding features

### Submitting Changes

1. Create feature branch
2. Make changes and write tests
3. Run `go test ./...` to ensure tests pass
4. Run `go vet ./...` for code quality checks
5. Update AGENTS.md if needed
6. Submit pull request

---

## Support and Documentation

### API Documentation
API documentation is available via Swagger UI when go-infra is running:
```
http://localhost:8080/swagger
```

### Command Help
```bash
infractl --help
infractl <command> --help
```

### Repository Structure

- **infra-cli/** - This CLI tool (you are here)
- **go-infra/** - Backend API and database layer
- **db-helper-ui/** - React-based web frontend

### Contact and Issues

- GitHub Issues: [Project Issues](https://github.com/babbage88/infractl/issues)
- Documentation: [Project Wiki](https://github.com/babbage88/infractl/wiki)

---

## License

See LICENSE file in repository root.

---

**Last Updated**: April 2026  
**Version**: 1.0  
**Maintained By**: Infractl Team
