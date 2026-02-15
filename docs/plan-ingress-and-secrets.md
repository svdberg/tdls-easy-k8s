# Plan: Add Ingress (Traefik) and Secrets Management (ESO)

## Context

The cluster can run workloads but has no way to expose them (no ingress, no LB for 80/443) and no secrets management. The config schema already has `components.traefik` and `components.externalSecrets` fields but they're never acted upon. This plan wires them up: terraform creates the infrastructure when components are enabled, and `app add --layer=infrastructure` deploys them via GitOps.

No new CLI command needed — `app add` already supports `--layer=infrastructure`.

## Architecture

```
config (components.traefik.enabled: true)
  → init: terraform creates ingress NLB (80/443 → workers) + IAM for Secrets Manager
  → gitops setup: installs Flux with infrastructure/ and apps/ layers
  → app add --layer=infrastructure: generates manifests for Traefik/ESO
  → Flux deploys from infrastructure/ layer (before apps)
```

## Files to modify

| File | Change |
|------|--------|
| **Terraform** | |
| `providers/aws/terraform/variables.tf` | Add `enable_ingress_nlb`, `enable_secrets_manager` |
| `providers/aws/terraform/main.tf` | Wire new vars to security, iam, loadbalancer modules |
| `providers/aws/terraform/outputs.tf` | Add `ingress_nlb_dns_name`, `ingress_nlb_zone_id` |
| `providers/aws/terraform/modules/security/main.tf` | Add conditional SG rules for 80/443 on workers |
| `providers/aws/terraform/modules/security/variables.tf` | Add `enable_ingress_nlb` variable |
| `providers/aws/terraform/modules/loadbalancer/main.tf` | Add ingress NLB, target groups, listeners |
| `providers/aws/terraform/modules/loadbalancer/variables.tf` | Add `enable_ingress`, `worker_instance_ids` |
| `providers/aws/terraform/modules/loadbalancer/outputs.tf` | Add ingress NLB outputs |
| `providers/aws/terraform/modules/iam/main.tf` | Add conditional Secrets Manager policy on worker role |
| `providers/aws/terraform/modules/iam/variables.tf` | Add `enable_secrets_manager` variable |
| **Go CLI** | |
| `internal/provider/aws.go` | Add component vars to `generateTerraformVars` (~line 464) |
| `internal/cli/app.go` | Add `--create-namespace` flag; support `--extra-manifests` dir |
| **Examples** | |
| `examples/traefik-values.yaml` | **New** — pre-built Traefik values for NLB + DaemonSet |
| `examples/eso-values.yaml` | **New** — ESO values with `installCRDs: true` |
| `examples/eso-clustersecretstore.yaml` | **New** — ClusterSecretStore template |

## Phase 1: Terraform — Ingress NLB

### 1.1 Root variables (`variables.tf`)

```hcl
variable "enable_ingress_nlb" {
  description = "Enable internet-facing NLB for ingress traffic (ports 80/443 to workers)"
  type        = bool
  default     = false
}

variable "enable_secrets_manager" {
  description = "Enable AWS Secrets Manager IAM permissions on worker nodes"
  type        = bool
  default     = false
}
```

### 1.2 Security module — SG rules

**`modules/security/variables.tf`**: Add `enable_ingress_nlb` (bool, default false)

**`modules/security/main.tf`**: Add two conditional rules. NLBs preserve source IP, so `cidr_blocks = ["0.0.0.0/0"]`:

```hcl
resource "aws_security_group_rule" "worker_ingress_http" {
  count             = var.enable_ingress_nlb ? 1 : 0
  type              = "ingress"
  from_port         = 80
  to_port           = 80
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  description       = "HTTP ingress via NLB"
  security_group_id = aws_security_group.worker.id
}

resource "aws_security_group_rule" "worker_ingress_https" {
  count             = var.enable_ingress_nlb ? 1 : 0
  type              = "ingress"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  description       = "HTTPS ingress via NLB"
  security_group_id = aws_security_group.worker.id
}
```

### 1.3 Loadbalancer module — ingress NLB

**`modules/loadbalancer/variables.tf`**: Add `enable_ingress` (bool) and `worker_instance_ids` (list(string), default [])

**`modules/loadbalancer/main.tf`**: Add a **separate** internet-facing NLB (don't mix with API NLB which targets control plane). All resources gated by `count = var.enable_ingress ? 1 : 0`:
- `aws_lb.ingress` — NLB, internet-facing, cross-zone enabled
- `aws_lb_target_group.ingress_http` — TCP port 80, health check TCP/80
- `aws_lb_target_group.ingress_https` — TCP port 443, health check TCP/443
- `aws_lb_target_group_attachment.ingress_http` — one per worker
- `aws_lb_target_group_attachment.ingress_https` — one per worker
- `aws_lb_listener.ingress_http` — port 80 → http TG
- `aws_lb_listener.ingress_https` — port 443 → https TG

NLB does TCP passthrough — Traefik handles TLS termination.

**`modules/loadbalancer/outputs.tf`**: Add `ingress_nlb_dns_name`, `ingress_nlb_zone_id`

### 1.4 IAM module — Secrets Manager

**`modules/iam/variables.tf`**: Add `enable_secrets_manager` (bool, default false)

**`modules/iam/main.tf`**: Add conditional policy on worker role (same pattern as `worker_cloudwatch`, `worker_ssm`):

```hcl
resource "aws_iam_role_policy" "worker_secrets_manager" {
  count       = var.enable_secrets_manager ? 1 : 0
  name_prefix = "secrets-manager-"
  role        = aws_iam_role.worker.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["secretsmanager:GetSecretValue", "secretsmanager:ListSecrets", "secretsmanager:DescribeSecret"]
      Resource = "*"
    }]
  })
}
```

### 1.5 Root main.tf — wire variables

- `module "security"`: add `enable_ingress_nlb = var.enable_ingress_nlb`
- `module "iam"`: add `enable_secrets_manager = var.enable_secrets_manager`
- `module "loadbalancer"`: add `enable_ingress = var.enable_ingress_nlb`, `worker_instance_ids = module.worker.instance_ids`, add `module.worker` to `depends_on`

### 1.6 Root outputs.tf

```hcl
output "ingress_nlb_dns_name" {
  value = var.enable_nlb && var.enable_ingress_nlb ? module.loadbalancer[0].ingress_nlb_dns_name : null
}
output "ingress_nlb_zone_id" {
  value = var.enable_nlb && var.enable_ingress_nlb ? module.loadbalancer[0].ingress_nlb_zone_id : null
}
```

## Phase 2: Go CLI — wire config to terraform

### 2.1 `generateTerraformVars` (`internal/provider/aws.go:464`)

Add two entries to the `vars` map:

```go
"enable_ingress_nlb":     cfg.Components.Traefik.Enabled,
"enable_secrets_manager": cfg.Components.ExternalSecrets.Enabled,
```

## Phase 3: `app add` enhancements

### 3.1 Add `--create-namespace` flag (`internal/cli/app.go`)

When set, generates a `namespace.yaml` alongside the HelmRelease and HelmRepository:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: <namespace>
```

Written to `<layer>/<appname>/namespace.yaml`. Simple addition to `writeAppFiles`.

## Phase 4: Example files

### 4.1 `examples/traefik-values.yaml`

Pre-built values for Traefik running as DaemonSet with hostPort (NLB targets workers directly):

```yaml
deployment:
  kind: DaemonSet
ports:
  web:
    hostPort: 80
  websecure:
    hostPort: 443
service:
  enabled: false
ingressRoute:
  dashboard:
    enabled: false
providers:
  kubernetesCRD:
    enabled: true
  kubernetesIngress:
    enabled: true
```

### 4.2 `examples/eso-values.yaml`

```yaml
installCRDs: true
```

### 4.3 `examples/eso-clustersecretstore.yaml`

Template — user fills in region:

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: aws-secrets-manager
spec:
  provider:
    aws:
      service: SecretsManager
      region: us-east-1  # change to your region
```

No auth block = uses worker node IAM instance profile.

## User workflow

```bash
# 1. Enable components in config
# cluster.yaml:
#   components:
#     traefik:
#       enabled: true
#     externalSecrets:
#       enabled: true

# 2. Create cluster (terraform creates ingress NLB + IAM)
tdls-easy-k8s init --config=cluster.yaml

# 3. Setup GitOps
tdls-easy-k8s gitops setup --repo=https://github.com/user/gitops

# 4. Add Traefik to infrastructure layer
tdls-easy-k8s app add traefik \
  --chart=traefik/traefik \
  --repo-url=https://traefik.github.io/charts \
  --layer=infrastructure \
  --namespace=traefik-system \
  --create-namespace \
  --values=examples/traefik-values.yaml \
  --output-dir=<gitops-repo>/clusters/production

# 5. Add ESO to infrastructure layer
tdls-easy-k8s app add external-secrets \
  --chart=external-secrets/external-secrets \
  --repo-url=https://charts.external-secrets.io \
  --layer=infrastructure \
  --namespace=external-secrets \
  --create-namespace \
  --values=examples/eso-values.yaml \
  --output-dir=<gitops-repo>/clusters/production

# 6. Copy ClusterSecretStore into the ESO directory
cp examples/eso-clustersecretstore.yaml <gitops-repo>/infrastructure/external-secrets/

# 7. Push — Flux deploys infrastructure layer first, then apps
cd <gitops-repo> && git add -A && git commit -m "Add ingress and secrets" && git push

# 8. Point DNS to ingress NLB
# Get DNS: tofu output ingress_nlb_dns_name
```

## Verification

```bash
# Unit tests
go test ./... && gofmt -l . && go vet ./...

# E2E on live cluster (e2e-test-2):
# 1. Update e2e-test-config.yaml to enable traefik + externalSecrets
# 2. tofu apply to create ingress NLB + IAM (incremental)
# 3. app add traefik + ESO to gitops repo
# 4. Push, verify Flux deploys both
# 5. Verify NLB health checks pass after Traefik starts
# 6. curl http://<ingress-nlb-dns> returns Traefik 404 (proves traffic flows)
# 7. kubectl get clustersecretstore — verify aws-secrets-manager is ready
```
