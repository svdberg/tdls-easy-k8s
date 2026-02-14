# tdls-easy-k8s

Easy Kubernetes cluster management for non-expert engineers.

## Overview

`tdls-easy-k8s` is a CLI tool that simplifies the deployment and management of production-ready Kubernetes clusters across multiple cloud providers (AWS, vSphere). It uses OpenTofu for infrastructure provisioning, automatically installs RKE2 via cloud-init, and provides GitOps-compliant workflows with built-in support for Traefik, Vault, and External Secrets.

## Features

- **Production-Ready AWS Infrastructure**: Multi-AZ deployment with HA, private/public subnet split, Network Load Balancer
- **Automated RKE2 Installation**: Cloud-init scripts handle complete cluster bootstrap
- **Automatic TLS Certificate Management**: Two-phase deployment ensures NLB DNS is included in API server certificates
- **OpenTofu-Based**: Uses open-source OpenTofu for infrastructure as code
- **Security-First**: Encrypted EBS volumes, strict security groups, IAM least privilege, AWS Session Manager
- **User-Friendly Validation**: Built-in `status` and `validate` commands for non-expert engineers
- **Automated Kubeconfig Management**: One command to download and configure cluster access
- **Cost-Optimized**: Options for single NAT gateway, spot instances, VPC endpoints
- **GitOps Ready**: Built-in Flux integration for GitOps workflows
- **Simple Configuration**: YAML-based configuration files
- **Cluster Monitoring**: One-command k9s launch with automatic installation
- **Easy to Use**: Simple CLI commands for cluster lifecycle management

## Architecture

### High-Level Architecture

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
   │      OpenTofu Modules       │
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

### AWS Infrastructure Details

```
┌──────────────────── VPC (10.0.0.0/16) ────────────────────┐
│                                                            │
│  ┌─────────────────────────────────────────────────────┐  │
│  │              Network Load Balancer                  │  │
│  │           (Kubernetes API - Port 6443)              │  │
│  └─────────────────────────────────────────────────────┘  │
│                                                            │
│  ┌──────── AZ-A ────────┬──────── AZ-B ────────┬─── AZ-C ──┐
│  │ Public  10.0.1.0/24  │ Public  10.0.2.0/24  │ 10.0.3/24││
│  │ ┌─────────────────┐  │ ┌─────────────────┐  │ ┌──────┐ ││
│  │ │ Control Plane 1 │  │ │ Control Plane 2 │  │ │ CP 3 │ ││
│  │ │   + etcd EBS    │  │ │   + etcd EBS    │  │ │+ EBS │ ││
│  │ └─────────────────┘  │ └─────────────────┘  │ └──────┘ ││
│  │         │            │         │            │     │     ││
│  │      NAT GW         │      NAT GW         │  NAT GW   ││
│  │         │            │         │            │     │     ││
│  │ Private 10.0.11/24  │ Private 10.0.12/24  │ 10.0.13/24││
│  │ ┌─────────────────┐  │ ┌─────────────────┐  │ ┌──────┐ ││
│  │ │   Worker 1,4    │  │ │   Worker 2,5    │  │ │ W3,6 │ ││
│  │ └─────────────────┘  │ └─────────────────┘  │ └──────┘ ││
│  └──────────────────────┴──────────────────────┴───────────┘
│                                                            │
└────────────────────────────────────────────────────────────┘
```

**Key Components:**
- **VPC**: Isolated network with DNS support
- **Subnets**: 3 public (control plane) + 3 private (workers) across 3 AZs
- **NAT Gateways**: Per-AZ for HA (or single for cost savings)
- **Network Load Balancer**: Highly available API endpoint
- **Security Groups**: Strict ingress rules for control plane, workers, and NLB
- **IAM Roles**: Least-privilege access for ECR, EBS, S3, CloudWatch
- **EBS Volumes**: Dedicated GP3 volumes for etcd with encryption
- **VPC Endpoints**: S3 and ECR for reduced data transfer costs

## Prerequisites

- **OpenTofu** >= 1.6.0 ([Installation guide](https://opentofu.org/docs/intro/install/))
- **AWS CLI** configured with credentials
- **Go** >= 1.24 (for building from source)

```bash
# Install OpenTofu (macOS)
brew install opentofu

# Install OpenTofu (Linux)
# See https://opentofu.org/docs/intro/install/

# Configure AWS credentials
aws configure
```

## Installation

### Option 1: Build from Source

```bash
git clone https://github.com/svdberg/tdls-easy-k8s.git
cd tdls-easy-k8s
make build
make install
```

### Option 2: Run from Project Directory

```bash
git clone https://github.com/svdberg/tdls-easy-k8s.git
cd tdls-easy-k8s
make build
./bin/tdls-easy-k8s version
```

## Quick Start

### 1. Review the Example Configuration

```bash
cat examples/cluster.yaml
```

### 2. Create Your Cluster Configuration

```bash
# Copy and edit the example
cp examples/cluster.yaml my-cluster.yaml
vim my-cluster.yaml

# Or generate a new one
tdls-easy-k8s init --generate-config > my-cluster.yaml
```

Key settings to customize:
- `name`: Your cluster name
- `provider.region`: AWS region (e.g., us-east-1)
- `nodes.controlPlane.count`: Number of control plane nodes (must be odd: 1, 3, 5)
- `nodes.workers.count`: Number of worker nodes
- `nodes.*.instanceType`: EC2 instance types

### 3. Create the Cluster

```bash
# This will:
# - Create S3 bucket for kubeconfig storage
# - Create OpenTofu working directory (~/.tdls-k8s/clusters/<name>/terraform)
# - Generate terraform.tfvars.json from your config
# - Run: tofu init, plan, and apply
# - Deploy AWS infrastructure (VPC, EC2, NLB, etc.)
# - Install RKE2 automatically via cloud-init
# - Update TLS certificates with NLB DNS (Phase 2)
# - Upload kubeconfig to S3

./bin/tdls-easy-k8s init --config=my-cluster.yaml
```

This typically takes **15-20 minutes** to complete (including TLS certificate updates).

### 4. Access Your Cluster

```bash
# Download and configure kubeconfig (automatically updates to use NLB endpoint)
./bin/tdls-easy-k8s kubeconfig --cluster=production

# Use the cluster
export KUBECONFIG=./kubeconfig
kubectl get nodes
kubectl get pods -A
```

**Or merge into your kubectl config:**
```bash
# Merge and set as current context
./bin/tdls-easy-k8s kubeconfig --cluster=production --merge --set-context
kubectl get nodes  # Just works!
```

### 5. Verify Cluster Health

```bash
# Quick status check
./bin/tdls-easy-k8s status --cluster=production

# Comprehensive validation
./bin/tdls-easy-k8s validate --cluster=production
```

### 6. (Optional) Setup GitOps

```bash
tdls-easy-k8s gitops setup --repo=github.com/youruser/cluster-gitops
```

### 7. (Optional) Add Applications

```bash
tdls-easy-k8s app add myapp \
  --chart=mycompany/myapp \
  --repo-url=https://charts.mycompany.com \
  --namespace=production
```

### 8. Clean Up

When you're done with the cluster:

```bash
# Destroy infrastructure (with confirmation prompt)
./bin/tdls-easy-k8s destroy --cluster=production

# Or destroy everything including local files
./bin/tdls-easy-k8s destroy --cluster=production --cleanup --force
```

## Configuration

See [examples/cluster.yaml](examples/cluster.yaml) for a complete configuration example.

### Minimal AWS Configuration

```yaml
name: production
provider:
  type: aws
  region: us-east-1
  vpc:
    cidr: 10.0.0.0/16

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

gitops:
  enabled: true
  repository: github.com/user/cluster-gitops
  branch: main

components:
  traefik:
    enabled: true
    version: "26.x"
  vault:
    enabled: true
    mode: external
    address: https://vault.example.com
  externalSecrets:
    enabled: true
```

### Cost Optimization Options

For development/testing, you can reduce costs by:

**Using the OpenTofu variables directly:**

```bash
cd ~/.tdls-k8s/clusters/<cluster-name>/terraform

# Edit terraform.tfvars.json to add:
{
  "single_nat_gateway": true,        # ~$64/month savings
  "enable_spot_instances": true,     # ~70% savings on workers
  "worker_instance_type": "t3.small" # Smaller instances
}

# Re-apply
tofu apply
```

**Estimated Monthly Costs (us-east-1):**
- **Production (3 CP + 3 workers)**: ~$536/month
  - Control plane: 3 × t3.medium = $90
  - Workers: 3 × t3.large = $270
  - NAT Gateways (3): $96
  - NLB: $18
  - EBS + data transfer: ~$62

- **Dev (single NAT, spot workers)**: ~$250/month
  - Single NAT: $32 (vs $96)
  - Spot workers: ~$80 (vs $270)
  - Other: ~$138

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

Add a new application to the cluster via GitOps. Generates Flux CD manifests
(Kustomization CRD, HelmRepository, HelmRelease) for deploying an application
using the app-of-apps pattern.

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--chart` | Yes | | Helm chart in `reponame/chartname` format |
| `--repo-url` | Yes | | Helm repository URL |
| `--namespace` | No | `default` | Target Kubernetes namespace |
| `--version` | No | `*` | Chart version constraint |
| `--values` | No | | Path to Helm values YAML file |
| `--layer` | No | `apps` | Target layer: `apps` or `infrastructure` |
| `--output-dir` | No | | Path to local gitops repo root (prints to stdout if omitted) |
| `--gitops-path` | No | `clusters/production` | Path within repo for Kustomization CRDs |
| `--depends-on` | No | | Name of another app this one depends on |

**Examples:**

```bash
# Print generated manifests to stdout (for review or piping)
tdls-easy-k8s app add nginx \
  --chart=bitnami/nginx \
  --repo-url=https://charts.bitnami.com/bitnami \
  --namespace=web

# Write files directly to a local gitops repository
tdls-easy-k8s app add nginx \
  --chart=bitnami/nginx \
  --repo-url=https://charts.bitnami.com/bitnami \
  --namespace=web \
  --output-dir=~/gitops-repo

# Deploy with a specific version, custom values, and dependency ordering
tdls-easy-k8s app add my-api \
  --chart=mycompany/my-api \
  --repo-url=https://charts.mycompany.com \
  --namespace=production \
  --version=1.2.3 \
  --values=./my-api-values.yaml \
  --depends-on=redis
```

### `tdls-easy-k8s kubeconfig`

Download and configure kubeconfig for cluster access.

```bash
# Download to ./kubeconfig
tdls-easy-k8s kubeconfig --cluster=production

# Download to specific location
tdls-easy-k8s kubeconfig --cluster=production --output=~/.kube/prod-config

# Merge into ~/.kube/config and set as current context
tdls-easy-k8s kubeconfig --cluster=production --merge --set-context
```

### `tdls-easy-k8s status`

Show cluster status and health overview.

```bash
# Quick status check
tdls-easy-k8s status --cluster=production

# Watch mode (updates every 5 seconds)
tdls-easy-k8s status --cluster=production --watch
```

**Example Output:**
```
Cluster: production
Provider: aws
Region: us-east-1

API Endpoint: production-nlb-xxx.elb.us-east-1.amazonaws.com

Nodes:
  ✓ Control Plane: 3/3 ready
  ✓ Workers: 3/3 ready

System Components:
  ✓ coredns            2/2 running
  ✓ canal              6/6 running
  ✓ kube-apiserver     3/3 running
  ✓ etcd               3/3 running

Status: ✓ Cluster is ready
Age: 45 minutes
```

### `tdls-easy-k8s validate`

Run comprehensive validation checks on cluster health.

```bash
# Full validation
tdls-easy-k8s validate --cluster=production

# Quick validation (skips optional checks)
tdls-easy-k8s validate --cluster=production --quick
```

**Example Output:**
```
Validating cluster: production
═══════════════════════════════════════════
Checking API server accessibility...
  ✓ API server is accessible

Checking Node readiness...
  ✓ All 6 nodes are ready

Checking System pods...
  ✓ All 12 system pods are running

Checking etcd health...
  ✓ etcd cluster healthy (3 members)

Checking DNS resolution...
  ✓ DNS is working (2 pods running)

Checking Pod networking...
  ✓ Pod networking is operational (6 Canal pods running)

═══════════════════════════════════════════
Validation Summary (8 seconds elapsed)
═══════════════════════════════════════════
Passed:   6

✓ Validation: PASSED
Cluster is healthy and ready for workload deployment!
```

### `tdls-easy-k8s destroy`

Destroy a cluster and all associated infrastructure.

```bash
# Destroy with confirmation prompt
tdls-easy-k8s destroy --cluster=production

# Destroy without confirmation (use with caution!)
tdls-easy-k8s destroy --cluster=dev --force

# Destroy and cleanup all local files and S3 bucket
tdls-easy-k8s destroy --cluster=dev --force --cleanup
```

**What gets destroyed:**
- All EC2 instances (control plane and workers)
- VPC and networking components (subnets, NAT gateways, IGW, route tables)
- Network Load Balancer and target groups
- EBS volumes (including etcd data)
- Security groups and IAM roles/policies
- With `--cleanup`: S3 bucket and local terraform state files

**Safety features:**
- Requires typing cluster name to confirm (unless `--force`)
- Shows detailed list of resources to be destroyed
- Irreversible operation - use with caution

### `tdls-easy-k8s monitor`

Launch k9s terminal UI for cluster monitoring. k9s is automatically installed if not found.

```bash
# Launch k9s for a cluster
tdls-easy-k8s monitor --cluster=production
```

k9s will be downloaded from GitHub releases and installed to `~/.tdls-k8s/bin/k9s` if not already available in your PATH.

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

## Infrastructure Details

### OpenTofu Modules

The AWS infrastructure is built using modular OpenTofu configurations:

- **[Networking](providers/aws/terraform/modules/networking/)**: VPC, subnets, NAT gateways, Internet Gateway, VPC endpoints
- **[Security](providers/aws/terraform/modules/security/)**: Security groups for control plane, workers, and NLB
- **[IAM](providers/aws/terraform/modules/iam/)**: Roles, policies, KMS encryption keys
- **[Storage](providers/aws/terraform/modules/storage/)**: EBS volumes for etcd
- **[Control Plane](providers/aws/terraform/modules/compute/control-plane/)**: EC2 instances with automated RKE2 server installation
- **[Worker](providers/aws/terraform/modules/compute/worker/)**: EC2 instances with automated RKE2 agent installation
- **[Load Balancer](providers/aws/terraform/modules/loadbalancer/)**: Network Load Balancer for Kubernetes API

### RKE2 Installation

RKE2 is installed automatically via cloud-init user-data scripts:

1. **First control plane node**:
   - Installs RKE2 server
   - Mounts dedicated etcd EBS volume
   - Initializes cluster
   - Uploads kubeconfig to S3

2. **Additional control plane nodes**:
   - Wait for first node via S3 kubeconfig
   - Join cluster via first node's IP
   - Maintain etcd quorum

3. **Worker nodes**:
   - Wait for API server availability
   - Install RKE2 agent
   - Join cluster via NLB (or first control plane IP)

### Security Features

- **Encrypted EBS volumes** with KMS
- **Private workers** with no direct internet access
- **Strict security groups** with minimal ingress rules
- **IAM least privilege** - separate roles for control plane and workers
- **AWS Session Manager** - no SSH keys required
- **VPC Flow Logs** (optional)
- **CloudWatch Logs** for centralized logging

## Advanced Usage

### Manual OpenTofu Operations

```bash
# Navigate to cluster's OpenTofu directory
cd ~/.tdls-k8s/clusters/<cluster-name>/terraform

# View current state
tofu show

# Plan changes
tofu plan

# Apply changes
tofu apply

# Destroy infrastructure
tofu destroy
```

### Accessing Cluster Nodes

**Using AWS Session Manager (recommended):**
```bash
# List instances
aws ec2 describe-instances --filters "Name=tag:Cluster,Values=<cluster-name>"

# Start session to control plane
aws ssm start-session --target <instance-id>
```

**Using SSH (if SSH key configured):**
```bash
ssh -i ~/.ssh/<key-name>.pem ubuntu@<public-ip>
```

### Troubleshooting

**Check RKE2 installation logs:**
```bash
# SSH into node
aws ssm start-session --target <instance-id>

# View installation log
sudo tail -f /var/log/rke2-install.log

# Check RKE2 status
sudo systemctl status rke2-server  # or rke2-agent
```

**View OpenTofu state:**
```bash
cd ~/.tdls-k8s/clusters/<cluster-name>/terraform
tofu show
tofu output
```

## Roadmap

### Completed ✅
- [x] CLI framework with Cobra
- [x] Provider abstraction layer
- [x] Configuration management (YAML-based)
- [x] OpenTofu AWS modules (networking, compute, security, IAM, storage, LB)
- [x] Automated RKE2 installation via cloud-init
- [x] Multi-AZ high availability architecture
- [x] AWS Session Manager integration
- [x] KMS encryption for EBS volumes
- [x] VPC endpoints for cost optimization
- [x] S3 bucket auto-creation for kubeconfig storage
- [x] Two-phase TLS certificate management (NLB DNS in API certificates)
- [x] **Kubeconfig automation** (`kubeconfig` command with kubectl integration)
- [x] **Cluster status monitoring** (`status` command for quick health checks)
- [x] **Comprehensive validation** (`validate` command with 7 health checks)
- [x] **Cluster destroy command** (`destroy` command with safety confirmations and cleanup options)
- [x] **Flux GitOps setup** (`gitops setup` command with Flux CD installation and repository configuration)
- [x] **GitOps template generation** (Kustomization, HelmRepository, HelmRelease manifests)
- [x] **Application deployment** (`app add` command with Helm chart support and dependency ordering)
- [x] **Unit tests** (CLI commands, flags, YAML generation)
- [x] **CI/CD pipeline** (GitHub Actions: fmt, vet, test, build)
- [x] **Cluster monitoring** (`monitor` command with k9s auto-installation)

### Planned 📋
- [ ] S3 backend for OpenTofu state (with DynamoDB locking)
- [ ] Cluster upgrade automation
- [ ] Worker node scaling command
- [ ] Component logs viewing (`logs` command)
- [ ] vSphere provider implementation
- [ ] K3s support (in addition to RKE2)
- [ ] Auto-scaling worker groups
- [ ] Backup and restore functionality
- [ ] Cost estimation before deployment
- [ ] Integration tests

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

[MIT License](LICENSE)
