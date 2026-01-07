# GitHub Copilot Instructions

## Project Overview

This repository contains a GitHub Action for managing deployment locks for Terraform (and other IaC tools) through pull requests. The project provides a distributed locking mechanism to prevent concurrent deployments and ensure safe infrastructure changes.

### Main Components

The project consists of four primary components:

1. **Service**: A distributed locking service that provides an HTTP/HTTPS API for managing deployment locks
2. **Client**: A CLI tool for interacting with the locking service from CI/CD pipelines
3. **GitHub Action**: A GitHub Action wrapper that simplifies integration with GitHub workflows
4. **Helm Chart**: Kubernetes deployment manifests for running the service in production

## Tech Stack

### Programming Language
- **Go**: Latest stable version
- Follow official Go best practices and idioms
- Use Go modules for dependency management

### Frameworks and Libraries
- **Chi**: HTTP router and middleware framework
- **Olric**: Distributed in-memory key-value store for lock management
- **Prometheus client_golang**: Metrics collection and exposition
- **Cobra**: CLI framework for the client application
- **Viper**: Configuration management
- **Zap**: Structured logging
- **OIDC Libraries**: OpenID Connect authentication for GitHub Actions/Apps

### Infrastructure
- **Docker**: Containerization with hardened base images
- **Helm**: Kubernetes package management (v3)
- **Kubernetes**: Target deployment platform

## Repository Structure

The repository should follow this organizational structure:

```
.
├── .github/
│   ├── workflows/          # CI/CD pipeline definitions
│   ├── ISSUE_TEMPLATE/     # Issue templates
│   ├── PULL_REQUEST_TEMPLATE.md
│   └── copilot-instructions.md
├── cmd/
│   ├── service/            # Service entry point
│   └── client/             # Client CLI entry point
├── internal/
│   ├── service/            # Service implementation
│   ├── client/             # Client implementation
│   ├── auth/               # OIDC authentication
│   ├── lock/               # Locking logic
│   ├── metrics/            # Prometheus metrics
│   └── config/             # Configuration handling
├── pkg/                    # Public packages (if any)
├── charts/
│   └── deployment-lock/    # Helm chart
│       ├── templates/
│       ├── tests/          # Helm unit tests
│       ├── Chart.yaml
│       ├── values.yaml
│       └── README.md
├── action.yaml             # GitHub Action definition
├── docs/
│   ├── RUNBOOK.md          # Operational procedures
│   ├── SLO.md              # Service Level Objectives
│   └── THREATS.md          # Threat model
├── Taskfile.yaml           # Task runner for local development
├── .goreleaser.yaml        # Release configuration
├── Dockerfile              # Service container image
├── README.md
├── CONTRIBUTING.md
├── CODE_OF_CONDUCT.md
├── SECURITY.md
└── LICENSE
```

## Component Requirements

### Service Component

The service must provide:

#### HTTP/HTTPS API
- RESTful endpoints for lock management
- Idempotent operations for locking and unlocking
- Support for both HTTP and HTTPS
- Proper HTTP status codes and error responses
- Request validation and sanitization

#### Core Functionality
- **Lock Acquisition**: Idempotent lock creation with owner identification
- **Lock Release**: Idempotent unlock operations
- **Lock Status**: Query lock state and metadata
- **Lock Expiration**: Automatic timeout handling for stale locks
- **Owner Verification**: Ensure only lock owners can release locks

#### Authentication
- OIDC-based authentication for GitHub Actions workflows
- Token validation and verification
- Support for GitHub App authentication
- Secure credential handling

#### Observability
- **Prometheus Metrics**: Expose standard and custom metrics
  - Lock acquisition rate and duration
  - Active locks gauge
  - Authentication failures
  - HTTP request metrics (duration, status codes)
- **Health Probes**: Kubernetes-compatible health endpoints
  - Liveness probe: Service is running
  - Readiness probe: Service can accept traffic
  - Startup probe: Service initialization status
- **Structured Logging**: JSON-formatted logs with appropriate levels

#### Reliability
- **Graceful Shutdown**: Handle SIGTERM/SIGINT properly
- **Connection Draining**: Complete in-flight requests before shutdown
- **Distributed Consensus**: Use Olric for shared state across replicas
- **High Availability**: Support multiple replicas

### Client Component

The CLI client must provide:

#### Commands
- `lock`: Acquire a deployment lock
- `unlock`: Release a deployment lock
- `status`: Check lock status
- `version`: Display version information

#### Authentication
- **GitHub Workflow OIDC**: Use GitHub Actions OIDC tokens
- **GitHub App**: Support GitHub App authentication
- **Token Management**: Secure credential handling
- **Automatic Token Refresh**: Handle token expiration

#### Features
- **Retry Logic**: Automatic retries with exponential backoff
- **Status Reporting**: Clear output for success/failure
- **Exit Codes**: Appropriate exit codes for different scenarios
- **Timeout Handling**: Configurable operation timeouts
- **JSON Output**: Machine-readable output option

### GitHub Action Component

#### action.yaml Structure
```yaml
name: 'Deployment Lock'
description: 'Manage deployment locks for safe infrastructure changes'
inputs:
  operation:
    description: 'Operation to perform (lock/unlock/status)'
    required: true
  lock-name:
    description: 'Name of the lock'
    required: true
  service-url:
    description: 'URL of the locking service'
    required: true
  timeout:
    description: 'Operation timeout in seconds'
    required: false
    default: '60'
outputs:
  lock-status:
    description: 'Current status of the lock'
  lock-owner:
    description: 'Current owner of the lock'
runs:
  using: 'composite'
```

#### Implementation Requirements
- Use composite action with shell steps
- Handle GITHUB_TOKEN or GitHub App credentials
- Provide clear error messages
- Set outputs for downstream steps
- Support all three operations (lock/unlock/status)

### Helm Chart Component

The Helm chart must include:

#### Deployment Manifest
- **Replicas**: Configurable replica count (default: 3)
- **Container Specs**:
  - Resource requests and limits
  - Security context (non-root user, read-only root filesystem)
  - Liveness, readiness, and startup probes
  - Environment variables from ConfigMap/Secret
- **Pod Specs**:
  - Security context (runAsNonRoot, drop capabilities)
  - Affinity rules for distribution across nodes
  - Topology spread constraints
  - Service account configuration

#### Secret Manifest
- OIDC configuration (client ID, client secret)
- Certificate/key pairs for HTTPS
- Mark as required in values.yaml

#### Service Manifest
- ClusterIP service for internal communication
- Named ports (http, https, metrics)
- Proper selectors

#### PodDisruptionBudget
- Ensure minimum availability during disruptions
- Configure based on replica count

#### Ingress/Gateway
- Support for both Ingress and Gateway API
- TLS configuration
- Path-based routing
- Annotations for ingress controllers

#### ServiceMonitor (Prometheus Operator)
- Scrape configuration for metrics endpoint
- Labels for Prometheus discovery
- Scrape interval and timeout

#### PrometheusRule
- **Alerts**: Define alerting rules
  - High error rate
  - Service unavailability
  - High lock acquisition latency
  - Stale locks
- **SLOs**: Service Level Objective recording rules
  - Availability SLO (e.g., 99.9%)
  - Latency SLO (e.g., p99 < 500ms)

#### NetworkPolicy
- Ingress rules for allowed traffic
- Egress rules for external dependencies
- Default deny policy

#### ConfigMap
- Non-sensitive configuration
- Feature flags
- Tuning parameters

#### Tests
- Helm unittest for template validation
- Test framework for integration testing

## CI/CD Pipeline

### GitHub Actions Workflows

#### Build and Test (`build.yaml`)
- **Go Matrix Testing**: Test against multiple Go versions
- **Unit Tests**: Run with race detector
- **Integration Tests**: Test service and client together
- **Code Coverage**: Generate and upload to CodeCov
- **Artifact Upload**: Build binaries and save as artifacts

#### Linting (`lint.yaml`)
- **Go Linting**: golangci-lint with strict configuration
  - Allow specific exceptions in `.golangci.yaml`
  - Maximum line length, cyclomatic complexity checks
- **Markdown Linting**: markdownlint for documentation
- **JSON Linting**: Validate JSON files
- **YAML Linting**: yamllint for YAML files and manifests
- **Dockerfile Linting**: hadolint for Dockerfile best practices
- **Helm Linting**: `helm lint` for chart validation

#### Release (`release.yaml`)
- **GoReleaser**: Automated binary builds and releases
- **Multi-platform**: Build for Linux, macOS, Windows
- **Docker Images**: Build and push container images
- **Helm Chart**: Package and publish chart
- **GitHub Release**: Create release with changelog
- **Trigger**: Run on version tags (v*)

#### Security Scanning (`security.yaml`)
- **Trivy/Clair**: Container image vulnerability scanning
- **CodeQL**: Static analysis security testing (SAST)
- **Dependency Scanning**: Check for vulnerable dependencies
- **SBOM Generation**: Software Bill of Materials

#### Other Workflows
- **Release Drafter**: Automatically draft releases from PRs
- **Dependabot**: Keep dependencies up to date
- **Stale Issues**: Manage inactive issues and PRs

### Taskfile

Create a `Taskfile.yaml` for local development:

```yaml
version: '3'

tasks:
  default:
    desc: List all available tasks
    cmds:
      - task --list

  build:
    desc: Build all components
    cmds:
      - task: build:service
      - task: build:client

  build:service:
    desc: Build the service
    cmds:
      - go build -o bin/service ./cmd/service

  build:client:
    desc: Build the client
    cmds:
      - go build -o bin/client ./cmd/client

  test:
    desc: Run all tests
    cmds:
      - go test -race -coverprofile=coverage.out ./...

  lint:
    desc: Run all linters
    cmds:
      - task: lint:go
      - task: lint:yaml
      - task: lint:markdown

  lint:go:
    desc: Lint Go code
    cmds:
      - golangci-lint run

  docker:build:
    desc: Build Docker image
    cmds:
      - docker build -t deployment-lock:local .

  helm:lint:
    desc: Lint Helm chart
    cmds:
      - helm lint ./charts/deployment-lock

  helm:test:
    desc: Run Helm unit tests
    cmds:
      - helm unittest ./charts/deployment-lock
```

## Coding Standards

### Go Best Practices

#### Code Style
- Follow official Go style guide and conventions
- Use `gofmt` and `goimports` for formatting
- Write idiomatic Go code
- Keep functions small and focused
- Use meaningful variable and function names

#### Error Handling
- Always check and handle errors explicitly
- Wrap errors with context using `fmt.Errorf` with `%w`
- Return errors rather than panicking in libraries
- Log errors at appropriate levels

#### Testing
- Write table-driven tests
- Use subtests with `t.Run()`
- Mock external dependencies
- Test error paths and edge cases
- Aim for >80% code coverage

#### Documentation
- Document all exported functions, types, and packages
- Use Go doc comments (start with the name being documented)
- Include examples for complex functionality
- Keep documentation up to date with code changes

#### Security
- Validate all inputs
- Sanitize outputs
- Use parameterized queries
- Avoid hardcoded credentials
- Follow OWASP security guidelines
- Use security linters (gosec)

### Commit Messages

Follow conventional commits format:

```
<type>(<scope>): <subject>

<body>

<footer>
```

#### Commit Types
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks
- `perf`: Performance improvements
- `ci`: CI/CD changes

#### Rules
- Use imperative mood in subject line ("add" not "added")
- First line should be 50 characters or less
- Separate subject from body with a blank line
- Wrap body at 72 characters
- Use body to explain what and why, not how

### Pull Request Guidelines

#### PR Title
- Use present tense ("Add feature" not "Added feature")
- Be descriptive but concise
- Reference issue number if applicable

#### PR Description
- Describe the changes and their purpose
- Include testing instructions
- List any breaking changes
- Reference related issues
- Add screenshots for UI changes

#### Code Review
- Ensure all CI checks pass
- Address reviewer feedback promptly
- Keep PRs focused and reasonably sized
- Squash commits before merging if requested

### File Extensions

- **YAML files**: Use `.yaml` extension (not `.yml`)
- **Configuration files**: Prefer YAML over JSON where appropriate
- **Documentation**: Use `.md` for Markdown files

## Documentation Requirements

### Required Documentation Files

#### README.md (Root)
- Project overview and purpose
- Quick start guide
- Installation instructions
- Usage examples
- Architecture overview
- Contributing guidelines link
- License information

#### charts/deployment-lock/README.md
- Chart description
- Prerequisites
- Installation steps
- Configuration options (values.yaml parameters)
- Upgrading guide
- Uninstallation steps
- Examples of common configurations

#### CONTRIBUTING.md
- How to contribute
- Development setup
- Code style guidelines
- Testing requirements
- Pull request process
- Code of conduct reference

#### CODE_OF_CONDUCT.md
- Community standards
- Expected behavior
- Unacceptable behavior
- Enforcement procedures
- Contact information

#### SECURITY.md
- Supported versions
- How to report vulnerabilities
- Security update policy
- PGP key for encrypted communication

#### PULL_REQUEST_TEMPLATE.md
- PR checklist
- Description template
- Testing instructions
- Related issues
- Breaking changes section

#### docs/RUNBOOK.md
- Operational procedures
- Common issues and solutions
- Deployment steps
- Rollback procedures
- Monitoring and alerting
- Incident response

#### docs/SLO.md
- Service Level Objectives definition
- SLI (Service Level Indicators) measurement
- Error budget policy
- Monitoring and alerting thresholds

#### docs/THREATS.md
- Threat model overview
- Attack vectors
- Mitigations
- Security controls
- Assumptions and dependencies

## Testing Requirements

### Code Coverage

- Maintain **>80% code coverage** for all components
- Measure coverage for both service and client
- Include coverage reports in CI pipeline
- Upload coverage to CodeCov or similar service

### Unit Tests

#### Service
- Test all HTTP handlers
- Test authentication logic
- Test lock management operations
- Test metrics collection
- Test configuration loading
- Mock external dependencies (Olric, OIDC)

#### Client
- Test all CLI commands
- Test authentication flows
- Test API client communication
- Test error handling and retries
- Mock HTTP responses

### Integration Tests

- Test service with actual Olric instance
- Test client against running service
- Test end-to-end lock/unlock flows
- Test concurrent operations
- Test authentication with mock OIDC provider
- Test graceful shutdown behavior

### Helm Chart Tests

#### Helm Unittest
- Test template rendering with various values
- Test conditional resources
- Test label and annotation propagation
- Test default values
- Test security contexts

#### Test Framework
- Deploy chart to test cluster
- Verify all resources are created
- Test service accessibility
- Test health probes
- Test metrics endpoints
- Test OIDC authentication

### Testing Commands

```bash
# Unit tests with coverage
go test -race -coverprofile=coverage.out ./...

# View coverage report
go tool cover -html=coverage.out

# Helm unit tests
helm unittest ./charts/deployment-lock

# Integration tests
go test -tags=integration -v ./test/integration/...
```

## Security Best Practices

### Code Security

- **Input Validation**: Validate all user inputs
- **Output Sanitization**: Escape outputs to prevent injection
- **Authentication**: Always verify OIDC tokens
- **Authorization**: Check permissions before operations
- **Secrets Management**: Never commit secrets to repository
- **Dependencies**: Keep dependencies up to date
- **Security Scanning**: Run security scanners in CI

### Container Security

- Use minimal base images (distroless, alpine)
- Run as non-root user
- Set read-only root filesystem
- Drop all capabilities
- Scan images for vulnerabilities
- Sign container images

### Kubernetes Security

- Apply pod security standards
- Use network policies
- Set resource limits
- Enable RBAC with least privilege
- Use secrets for sensitive data
- Enable audit logging

## Additional Guidelines

### Performance

- Profile code for bottlenecks
- Use appropriate data structures
- Minimize memory allocations
- Use connection pooling
- Implement caching where appropriate
- Monitor performance metrics

### Observability

- Log at appropriate levels (debug, info, warn, error)
- Include structured fields in logs
- Emit metrics for key operations
- Implement distributed tracing (optional)
- Create dashboards for monitoring
- Set up alerts for critical conditions

### Scalability

- Design for horizontal scaling
- Use stateless services where possible
- Implement proper health checks
- Handle backpressure appropriately
- Consider rate limiting
- Plan for distributed deployment

---

These instructions should guide all development work in this repository. When making changes, always consider the impact on security, performance, and maintainability. Follow the established patterns and conventions to maintain consistency across the codebase.
