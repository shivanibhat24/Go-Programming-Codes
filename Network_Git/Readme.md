# Network Git (NetGit)

Git-like version control for network configurations with policy verification and safe deployment.

## Features

- **Version Control**: Git-like operations (commit, diff, revert, branch, merge)
- **Content-Addressable Storage**: Uses BoltDB for efficient object storage
- **Policy Verification**: Built-in policy engine with custom rules
- **Safe Deployment**: Dry-run, canary deployments, instant rollback
- **Multi-Backend Support**: AWS, GCP, Azure, Kubernetes, and more
- **Audit Logging**: Complete audit trail with structured JSON logs
- **Observability**: Prometheus metrics and monitoring

## Quick Start

```bash
# Initialize repository
netgit init

# Add your network configurations (YAML/JSON)
cp examples/sample-configs/aws-security-groups.yaml ./

# Commit changes
netgit commit -m "Add production security groups"

# Verify against policies
netgit verify

# Deploy with dry run
netgit deploy --target=aws --dry-run

# Deploy with canary rollout
netgit deploy --target=aws --canary

# View history
netgit history
```

## Installation

```bash
make deps
make build
make install
```

## Configuration Examples

See `examples/sample-configs/` for YAML and JSON configuration examples for:
- AWS Security Groups
- GCP Firewall Rules  
- Kubernetes Network Policies

## Testing

```bash
make test
make test-coverage
```

## Demo

```bash
make demo
```
    
