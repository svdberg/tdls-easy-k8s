# tdls-easy-k8s

Easy Kubernetes cluster management for non-expert engineers.

## Overview

`tdls-easy-k8s` is a CLI tool that simplifies the deployment and management of production-ready Kubernetes clusters across multiple cloud providers (AWS, Hetzner Cloud). It uses OpenTofu for infrastructure provisioning, automatically installs RKE2 via cloud-init, and provides GitOps-compliant workflows with built-in support for Traefik, Vault, and External Secrets.

## Features

- **Multi-Cloud Support**: Deploy on AWS or Hetzner Cloud with a unified CLI
- **Production-Ready Infrastructure**: HA deployments with load balancers, firewalls, and private networking
- **Automated RKE2 Installation**: Cloud-init scripts handle complete cluster bootstrap
- **OpenTofu-Based**: Uses open-source OpenTofu for infrastructure as code
- **User-Friendly Validation**: Built-in `status` and `validate` commands for non-expert engineers
- **Automated Kubeconfig Management**: One command to download and configure cluster access
- **GitOps Ready**: Built-in Flux integration for GitOps workflows
- **Simple Configuration**: YAML-based configuration files
- **Cluster Monitoring**: One-command k9s launch with automatic installation
- **Easy to Use**: Simple CLI commands for cluster lifecycle management

## Architecture

### High-Level Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                        tdls-easy-k8s CLI                         │
└──────────────────────────┬───────────────────────────────────────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
         ┌────▼─────┐ ┌───▼────┐ ┌─────▼──────┐
         │   AWS    │ │Hetzner │ │  vSphere   │
         │ Provider │ │Provider│ │  (stub)    │
         └────┬─────┘ └───┬────┘ └────────────┘
              │           │
         ┌────▼───────────▼──────┐
         │    OpenTofu Modules   │
         └────┬──────────────────┘
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

### Hetzner Cloud Infrastructure Details

```
┌──────── Private Network (10.0.0.0/16) ────────┐
│                                                 │
│  ┌───────────────────────────────────────────┐  │
│  │         Load Balancer (lb11)              │  │
│  │   Kubernetes API (6443) + RKE2 (9345)    │  │
│  └───────────────────────────────────────────┘  │
│                                                 │
│  ┌──────── Subnet 10.0.1.0/24 ───────────────┐ │
│  │                                            │ │
│  │  ┌─────────────────┐                      │ │
│  │  │ Control Plane 0 │  cpx22 (2C/4GB)     │ │
│  │  │   (bootstrap)   │                      │ │
│  │  └─────────────────┘                      │ │
│  │                                            │ │
│  │  ┌─────────────────┐ ┌─────────────────┐  │ │
│  │  │   Worker 0      │ │   Worker 1      │  │ │
│  │  │ cpx32 (4C/8GB)  │ │ cpx32 (4C/8GB)  │  │ │
│  │  └─────────────────┘ └─────────────────┘  │ │
│  └────────────────────────────────────────────┘ │
│                                                 │
│  ┌── Firewall ──────────────────────────────┐   │
│  │  22, 6443, 9345, 80, 443 (public)       │   │
│  │  10250, 2379-2381, 8472 (private net)   │   │
│  └──────────────────────────────────────────┘   │
└─────────────────────────────────────────────────┘
```

**Key differences from AWS:**
- **Simpler networking**: Flat private network with a single subnet (no NAT gateways)
- **LB created first**: Load balancer IP is known before servers boot, so TLS certificates include it from the start (no multi-phase deployment)
- **SSH-based kubeconfig**: Retrieved via SSH from the control plane node (no S3 bucket needed)
- **API token auth**: Uses `HCLOUD_TOKEN` environment variable (no IAM roles)
- **Locations**: fsn1 (Falkenstein), nbg1 (Nuremberg), hel1 (Helsinki), ash (Ashburn), hil (Hillsboro)

## Prerequisites

- **OpenTofu** >= 1.6.0 ([Installation guide](https://opentofu.org/docs/intro/install/))
- **Go** >= 1.24 (for building from source)

**For AWS:**
- AWS CLI configured with credentials (`aws configure`)

**For Hetzner Cloud:**
- Hetzner Cloud API token (`export HCLOUD_TOKEN=<your-token>`)

```bash
# Install OpenTofu (macOS)
brew install opentofu

# Install OpenTofu (Linux)
# See https://opentofu.org/docs/intro/install/
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
# AWS example
cat examples/cluster.yaml

# Hetzner Cloud example
cat examples/cluster-hetzner.yaml
```

### 2. Create Your Cluster Configuration

```bash
# Copy and edit an example
cp examples/cluster-hetzner.yaml my-cluster.yaml
vim my-cluster.yaml

# Or generate a new one
tdls-easy-k8s init --generate-config > my-cluster.yaml
```

Key settings to customize:
- `name`: Your cluster name
- `provider.type`: Cloud provider (`aws` or `hetzner`)
- `provider.region` / `provider.location`: Where to deploy
- `nodes.controlPlane.count`: Number of control plane nodes (must be odd: 1, 3, 5)
- `nodes.workers.count`: Number of worker nodes
- `nodes.*.instanceType`: Instance types (e.g., `t3.medium` for AWS, `cpx22` for Hetzner)

### 3. Create the Cluster

```bash
./bin/tdls-easy-k8s init --config=my-cluster.yaml
```

This provisions infrastructure via OpenTofu, installs RKE2 via cloud-init, and configures the cluster. Typical deploy time: **5-10 minutes** (Hetzner) or **15-20 minutes** (AWS).

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

See [examples/cluster.yaml](examples/cluster.yaml) (AWS) and [examples/cluster-hetzner.yaml](examples/cluster-hetzner.yaml) (Hetzner) for complete configuration examples.

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
```

### Minimal Hetzner Cloud Configuration

```yaml
name: my-cluster
provider:
  type: hetzner
  location: nbg1    # Nuremberg (options: fsn1, nbg1, hel1, ash, hil)
  vpc:
    cidr: 10.0.0.0/16

kubernetes:
  version: "1.30"
  distribution: rke2

nodes:
  controlPlane:
    count: 1
    instanceType: cpx22    # 2 vCPU AMD, 4 GB RAM
  workers:
    count: 2
    instanceType: cpx32    # 4 vCPU AMD, 8 GB RAM
```

### Optional Components

```yaml
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

### Cost Estimation

**Hetzner Cloud (EU):**
- **Dev (1 CP + 2 workers)**: ~$25/month
  - Control plane: 1 × cpx22 = ~$5
  - Workers: 2 × cpx32 = ~$14
  - Load balancer (lb11): ~$6
- **Production (3 CP + 3 workers)**: ~$55/month

**AWS (us-east-1):**
- **Production (3 CP + 3 workers)**: ~$536/month
  - Control plane: 3 × t3.medium = $90
  - Workers: 3 × t3.large = $270
  - NAT Gateways (3): $96
  - NLB: $18
  - EBS + data transfer: ~$62
- **Dev (single NAT, spot workers)**: ~$250/month

## Commands

### `tdls-easy-k8s init`

Initialize a new Kubernetes cluster.

```bash
# Initialize from config file
tdls-easy-k8s init --config=cluster.yaml

# Initialize with flags
tdls-easy-k8s init --provider=aws --region=us-east-1 --name=production
tdls-easy-k8s init --provider=hetzner --region=nbg1 --name=my-cluster

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

### `tdls-easy-k8s vault setup`

Generate Vault integration manifests for GitOps deployment. Supports two modes based on the cluster config:

- **`external`**: Generates a `ClusterSecretStore` pointing at your existing Vault instance
- **`deploy`**: Generates a full Vault deployment (HelmRepository, HelmRelease, namespace) plus a `ClusterSecretStore`

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--cluster` / `-c` | Yes | | Cluster name |
| `--output-dir` | No | | Path to local gitops repo root (prints to stdout if omitted) |
| `--gitops-path` | No | `clusters/production` | Path within repo for Kustomization CRDs |

**Examples:**

```bash
# Preview manifests for external Vault
tdls-easy-k8s vault setup --cluster=production

# Write deploy-mode manifests to gitops repo
tdls-easy-k8s vault setup --cluster=production --output-dir=~/gitops-repo
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
- All compute instances (control plane and workers)
- Networking (VPC/subnets on AWS, private network on Hetzner)
- Load balancers and firewall/security group rules
- Storage volumes
- With `--cleanup`: S3 bucket (AWS) and local terraform state files

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

When components are enabled in the cluster config, `init` provisions the required infrastructure (ingress load balancers, IAM policies). The components themselves are deployed via GitOps using `app add` and `vault setup`.

### Traefik (Ingress)

Enabling Traefik creates an ingress load balancer during `init` (NLB on AWS, LB on Hetzner) that routes HTTP/HTTPS traffic to worker nodes. Traefik runs as a DaemonSet with hostPort, so no Kubernetes `LoadBalancer` service is needed.

```bash
# 1. Enable in cluster config and run init (creates the ingress LB)
# 2. Deploy Traefik via GitOps
tdls-easy-k8s app add traefik \
  --chart=traefik/traefik \
  --repo-url=https://traefik.github.io/charts \
  --layer=infrastructure \
  --namespace=traefik-system \
  --values=templates/components/traefik-values.yaml \
  --output-dir=~/gitops-repo

# 3. Point DNS to the ingress LB
#    AWS:     tofu output ingress_nlb_dns_name
#    Hetzner: tofu output ingress_lb_ipv4
```

### External Secrets Operator (ESO)

ESO syncs secrets from external stores into Kubernetes Secrets. On AWS, enabling it grants worker nodes IAM permissions for Secrets Manager.

```bash
# Deploy ESO via GitOps
tdls-easy-k8s app add external-secrets \
  --chart=external-secrets/external-secrets \
  --repo-url=https://charts.external-secrets.io \
  --layer=infrastructure \
  --namespace=external-secrets \
  --values=templates/components/eso-values.yaml \
  --output-dir=~/gitops-repo
```

See `templates/components/eso-clustersecretstore.yaml` for an AWS Secrets Manager ClusterSecretStore template.

### Vault Integration

Vault can be used as the secrets backend for ESO. Two modes are supported:

**External Vault** — connect ESO to an existing Vault instance:

```yaml
components:
  vault:
    enabled: true
    mode: external
    address: https://vault.example.com
  externalSecrets:
    enabled: true
```

**Deploy Vault** — install Vault into the cluster via Helm:

```yaml
components:
  vault:
    enabled: true
    mode: deploy
  externalSecrets:
    enabled: true
```

After deploying ESO, generate the Vault manifests:

```bash
tdls-easy-k8s vault setup --cluster=production --output-dir=~/gitops-repo
```

This generates a `ClusterSecretStore` (both modes) and, for deploy mode, also the Vault HelmRelease, HelmRepository, and namespace. See `templates/components/` for reference values files (`vault-values.yaml` for dev, `vault-ha-values.yaml` for production).

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

**AWS** uses modular OpenTofu configurations:

- **[Networking](providers/aws/terraform/modules/networking/)**: VPC, subnets, NAT gateways, Internet Gateway, VPC endpoints
- **[Security](providers/aws/terraform/modules/security/)**: Security groups for control plane, workers, and NLB
- **[IAM](providers/aws/terraform/modules/iam/)**: Roles, policies, KMS encryption keys
- **[Storage](providers/aws/terraform/modules/storage/)**: EBS volumes for etcd
- **[Control Plane](providers/aws/terraform/modules/compute/control-plane/)**: EC2 instances with automated RKE2 server installation
- **[Worker](providers/aws/terraform/modules/compute/worker/)**: EC2 instances with automated RKE2 agent installation
- **[Load Balancer](providers/aws/terraform/modules/loadbalancer/)**: Network Load Balancer for Kubernetes API

**Hetzner Cloud** uses a flat OpenTofu configuration (no submodules):

- **[Terraform](providers/hetzner/terraform/)**: Network, firewall, load balancer, servers, SSH keys — all in a single `main.tf`

### RKE2 Installation

RKE2 is installed automatically via cloud-init user-data scripts. The process is similar across providers:

1. **First control plane node**: Installs RKE2 server, initializes cluster
2. **Additional control plane nodes**: Wait for first node, join cluster, maintain etcd quorum
3. **Worker nodes**: Wait for API server, install RKE2 agent, join cluster

**Provider-specific differences:**

| | AWS | Hetzner |
|---|-----|---------|
| Kubeconfig retrieval | Downloaded from S3 bucket | Retrieved via SSH from control plane |
| TLS certificates | Multi-phase: NLB DNS added after deploy | LB IP known before servers boot |
| Node access | AWS Session Manager (no SSH keys) | SSH with auto-generated ED25519 key |
| etcd storage | Dedicated encrypted EBS volumes | Local disk |

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

**AWS — Using Session Manager (recommended):**
```bash
aws ec2 describe-instances --filters "Name=tag:Cluster,Values=<cluster-name>"
aws ssm start-session --target <instance-id>
```

**Hetzner — Using SSH:**
```bash
# Get the SSH key from terraform output
cd ~/.tdls-k8s/clusters/<cluster-name>/terraform
tofu output -raw ssh_private_key > /tmp/hetzner-key && chmod 600 /tmp/hetzner-key

# SSH into a node
ssh -i /tmp/hetzner-key root@<node-ip>
```

### Troubleshooting

**Check RKE2 installation logs:**
```bash
# SSH into node (method depends on provider)

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
- [x] **AWS provider** (VPC, EC2, NLB, IAM, EBS, S3, multi-AZ HA)
- [x] **Hetzner Cloud provider** (network, servers, LB, firewall, SSH-based kubeconfig)
- [x] Automated RKE2 installation via cloud-init
- [x] Shared kubectl validation across providers (`common.go`)
- [x] **Kubeconfig automation** (`kubeconfig` command with kubectl integration)
- [x] **Cluster status monitoring** (`status` command for quick health checks)
- [x] **Comprehensive validation** (`validate` command with 7 health checks)
- [x] **Cluster destroy command** (`destroy` command with provider-aware warnings)
- [x] **Flux GitOps setup** (`gitops setup` command with Flux CD installation)
- [x] **Application deployment** (`app add` command with Helm chart support)
- [x] **Cluster monitoring** (`monitor` command with k9s auto-installation)
- [x] **Unit tests** and **CI/CD pipeline** (GitHub Actions)
- [x] **Ingress support** (Traefik via ingress NLB on AWS, ingress LB on Hetzner)
- [x] **Secrets management** (ESO with AWS Secrets Manager IAM, Vault external + deploy modes)
- [x] **Vault setup command** (`vault setup` for both external and deploy modes)

### Planned 📋
- [ ] vSphere provider implementation
- [ ] S3 backend for OpenTofu state (with DynamoDB locking)
- [ ] Cluster upgrade automation
- [ ] Worker node scaling command
- [ ] K3s support (in addition to RKE2)
- [ ] Backup and restore functionality
- [ ] Integration tests

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

[MIT License](LICENSE)
