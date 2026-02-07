# tdls-easy-k8s

Easy Kubernetes cluster management for non-expert engineers.

## Overview

`tdls-easy-k8s` is a CLI tool that simplifies the deployment and management of Kubernetes clusters across multiple cloud providers (AWS, vSphere). It provides GitOps-compliant workflows and automatically sets up essential components like Traefik for ingress and integration with Vault for secrets management.

## Features

- **Multi-cloud Support**: Deploy on AWS EC2 or vSphere
- **GitOps Ready**: Built-in Flux integration for GitOps workflows
- **Batteries Included**: Pre-configured with Traefik, Vault integration, and External Secrets
- **Simple Configuration**: YAML-based configuration files
- **Easy to Use**: Simple CLI commands for common operations

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                   tdls-easy-k8s CLI                     │
└────────────────┬────────────────────────────────────────┘
                 │
        ┌────────┴──────────┐
        │                   │
   ┌────▼─────┐      ┌─────▼──────┐
   │   AWS    │      │  vSphere   │
   │ Provider │      │  Provider  │
   └────┬─────┘      └─────┬──────┘
        │                  │
   ┌────▼──────────────────▼─────┐
   │      Terraform Modules      │
   └────┬────────────────────────┘
        │
   ┌────▼─────────────────────────┐
   │   Kubernetes Cluster (RKE2)  │
   │  ┌────────────────────────┐  │
   │  │  Flux (GitOps)         │  │
   │  │  Traefik (Ingress)     │  │
   │  │  External Secrets      │  │
   │  └────────────────────────┘  │
   └──────────────────────────────┘
```

## Installation

### Build from source

```bash
git clone https://github.com/user/tdls-easy-k8s.git
cd tdls-easy-k8s
make build
make install
```

## Quick Start

### 1. Generate a sample configuration

```bash
tdls-easy-k8s init --generate-config > cluster.yaml
```

### 2. Edit the configuration to match your requirements

```bash
# Edit cluster.yaml with your preferred editor
vim cluster.yaml
```

### 3. Initialize the cluster

```bash
tdls-easy-k8s init --config=cluster.yaml
```

### 4. Setup GitOps

```bash
tdls-easy-k8s gitops setup --repo=github.com/youruser/cluster-gitops
```

### 5. Add applications

```bash
tdls-easy-k8s app add myapp --chart=mycompany/myapp --values=values.yaml
```

## Configuration

See [examples/cluster.yaml](examples/cluster.yaml) for a complete configuration example.

### Minimal AWS Configuration

```yaml
name: production
provider:
  type: aws
  region: us-east-1

kubernetes:
  version: "1.30"
  distribution: rke2

nodes:
  controlPlane:
    count: 3
    instanceType: t3.medium
  workers:
    count: 3
    instanceType: t3.large
```

## Commands

### `tdls-easy-k8s init`

Initialize a new Kubernetes cluster.

```bash
# Initialize with flags
tdls-easy-k8s init --provider=aws --region=us-east-1 --name=production

# Initialize from config file
tdls-easy-k8s init --config=cluster.yaml

# Generate sample config
tdls-easy-k8s init --generate-config
```

### `tdls-easy-k8s gitops setup`

Setup GitOps (Flux) on the cluster.

```bash
tdls-easy-k8s gitops setup --repo=github.com/user/cluster-gitops --branch=main
```

### `tdls-easy-k8s app add`

Add a new application to the cluster via GitOps.

```bash
tdls-easy-k8s app add myapp --chart=mycompany/myapp --namespace=production
```

### `tdls-easy-k8s version`

Display version information.

```bash
tdls-easy-k8s version
```

## Components

### Traefik

Traefik is automatically deployed as the ingress controller. Configure it in your cluster config:

```yaml
components:
  traefik:
    enabled: true
    version: "26.x"
```

### Vault Integration

Integrate with an external Vault instance for secrets management:

```yaml
components:
  vault:
    enabled: true
    mode: external
    address: https://vault.example.com
```

### External Secrets Operator

Automatically sync secrets from Vault to Kubernetes:

```yaml
components:
  externalSecrets:
    enabled: true
```

## Development

### Build

```bash
make build
```

### Test

```bash
make test
```

### Clean

```bash
make clean
```

## Roadmap

- [x] CLI framework with Cobra
- [x] Provider abstraction layer
- [x] Configuration management
- [ ] AWS Terraform modules
- [ ] RKE2/K3s installation
- [ ] Flux GitOps setup
- [ ] GitOps template generation
- [ ] vSphere provider implementation
- [ ] Unit tests
- [ ] Integration tests

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

[MIT License](LICENSE)
