# Plan: Add Proxmox VE Provider

## Context

Proxmox VE is a lightweight, open-source virtualization platform that runs on almost any x86 hardware. Unlike Harvester (which requires 64GB+ RAM), Proxmox works well on modest hardware, making it ideal for homelabs and small on-prem environments. Adding Proxmox support enables easy Kubernetes provisioning on the most popular homelab hypervisor.

The [bpg/proxmox](https://github.com/bpg/terraform-provider-proxmox) Terraform provider is mature (v0.78+), actively maintained, and has excellent cloud-init support — making it a natural fit.

## Design Decisions

- **API Load Balancing**: kube-vip static pod on CP nodes (ARP mode, same as Harvester plan)
- **IP Assignment**: DHCP from existing network (simplest for home/lab networks)
- **IP Discovery**: Interface-based detection (same approach as Harvester — try common interface names)
- **VM Image**: Ubuntu 22.04 cloud image (consistent with Hetzner/AWS providers, apt-get based)
- **Networking**: Bridge-based (default `vmbr0`), optional VLAN tag
- **Auth**: `PROXMOX_VE_ENDPOINT`, `PROXMOX_VE_API_TOKEN` env vars (or username/password)

## Similarities to Existing Providers

Proxmox is the closest to Hetzner in terms of implementation:
- Same OS (Ubuntu 22.04) = same cloud-init scripts (apt-get, same interface detection)
- Same kubeconfig retrieval pattern (SSH into first CP, download rke2.yaml, patch server URL)
- Same Terraform workflow (init/plan/apply)
- Main difference: kube-vip instead of cloud LB, Proxmox API instead of Hetzner API

## Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `internal/config/config.go` | **Modify** | Add `ProxmoxConfig` fields (`Node`, `Bridge`, `VlanTag`, `Datastore`); add `VIP` field (shared with Harvester) |
| `internal/provider/provider.go` | **Modify** | Add `"proxmox"` case to `GetProvider()` factory |
| `internal/provider/proxmox.go` | **Create** | Full provider implementation (~500 lines) |
| `providers/proxmox/terraform/versions.tf` | **Create** | bpg/proxmox + tls + random providers |
| `providers/proxmox/terraform/variables.tf` | **Create** | All input variables |
| `providers/proxmox/terraform/main.tf` | **Create** | SSH key, image download, cloud-init snippets, CP VMs, worker VMs |
| `providers/proxmox/terraform/outputs.tf` | **Create** | vip_address, first_cp_ip, node IPs, ssh_private_key |
| `providers/proxmox/terraform/user-data-cp.tpl` | **Create** | CP cloud-init: RKE2 server + kube-vip (very close to Hetzner template) |
| `providers/proxmox/terraform/user-data-worker.tpl` | **Create** | Worker cloud-init: RKE2 agent (very close to Hetzner template) |
| `internal/provider/proxmox_test.go` | **Create** | Unit tests |
| `internal/provider/provider_test.go` | **Modify** | Add `TestGetProvider_Proxmox` |
| `internal/config/config_test.go` | **Modify** | Add proxmox validation test |
| `examples/cluster-proxmox.yaml` | **Create** | Example config file |

## 1. Config Changes

**`internal/config/config.go`** — Add Proxmox-specific fields to `ProviderConfig`:

```go
// Add to ProviderConfig struct:
Node      string `yaml:"node,omitempty"`      // Proxmox node name (e.g. "pve")
Bridge    string `yaml:"bridge,omitempty"`    // Network bridge (default "vmbr0")
VlanTag   int    `yaml:"vlanTag,omitempty"`   // Optional VLAN tag on the bridge
Datastore string `yaml:"datastore,omitempty"` // Storage datastore (default "local-lvm")
VIP       string `yaml:"vip,omitempty"`       // kube-vip virtual IP (shared with Harvester)
```

The `VIP` field is shared with the Harvester plan — both on-prem providers need it. `Node` is the Proxmox hostname where VMs will be created.

Add `"proxmox"` to the provider type validation.

## 2. Provider Implementation

**`internal/provider/proxmox.go`** — Follow exact Hetzner pattern:

- `ProxmoxProvider` struct with `workDir` field
- `ValidateConfig`:
  - Check type=proxmox
  - Require `Node` (Proxmox node name)
  - Require `VIP` (valid IPv4 for kube-vip)
  - Require `PROXMOX_VE_ENDPOINT` env var (e.g. `https://proxmox.local:8006`)
  - Require `PROXMOX_VE_API_TOKEN` env var (e.g. `terraform@pve!provider=xxx-xxx`)
  - Default `Bridge` to `"vmbr0"`, `Datastore` to `"local-lvm"`
- `CreateInfrastructure`: copy TF modules, generate tfvars.json, run tofu
- `DestroyInfrastructure`: check state, `tofu destroy -auto-approve`
- `GetKubeconfig`: SSH into first CP (using generated ED25519 key), download rke2.yaml, patch server URL with VIP
- All validation methods: delegate to `common.go` kubectl functions
- `generateTerraformVars`: `cluster_name`, `proxmox_node`, `bridge`, `vlan_tag`, `datastore`, `vip_address`, `cp_count`, `worker_count`, `cp_cpu`, `cp_memory`, `cp_disk`, `worker_cpu`, `worker_memory`, `worker_disk`, `kubernetes_version`

**`internal/provider/provider.go`** — Add to factory:
```go
case "proxmox":
    return NewProxmoxProvider(), nil
```

## 3. Terraform Modules

**`providers/proxmox/terraform/versions.tf`**:
```hcl
terraform {
  required_version = ">= 1.0"
  required_providers {
    proxmox = {
      source  = "bpg/proxmox"
      version = "~> 0.78"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
}

provider "proxmox" {
  # Uses PROXMOX_VE_ENDPOINT and PROXMOX_VE_API_TOKEN env vars
}
```

**`providers/proxmox/terraform/main.tf`** — Key resources:

| Resource | Purpose |
|----------|---------|
| `tls_private_key.ssh` | ED25519 SSH key |
| `proxmox_virtual_environment_download_file.ubuntu` | Download Ubuntu 22.04 cloud image to Proxmox |
| `proxmox_virtual_environment_file.cloudinit_cp_init` | Cloud-init snippet for first CP node |
| `proxmox_virtual_environment_file.cloudinit_cp_join` | Cloud-init snippets for join CP nodes |
| `proxmox_virtual_environment_file.cloudinit_worker` | Cloud-init snippets for workers |
| `proxmox_virtual_environment_vm.control_plane_init` | First CP node (bootstrap) |
| `proxmox_virtual_environment_vm.control_plane_join` | Additional CP nodes |
| `proxmox_virtual_environment_vm.worker` | Worker nodes |
| `random_password.cluster_token` | RKE2 join token |

Each VM:
```hcl
resource "proxmox_virtual_environment_vm" "control_plane_init" {
  name      = "${var.cluster_name}-cp-0"
  node_name = var.proxmox_node

  agent { enabled = true }

  cpu {
    cores = var.cp_cpu
    type  = "x86-64-v2-AES"
  }

  memory { dedicated = var.cp_memory_mb }

  disk {
    datastore_id = var.datastore
    import_from  = proxmox_virtual_environment_download_file.ubuntu.id
    interface    = "virtio0"
    size         = var.cp_disk_gb
    discard      = "on"
    iothread     = true
  }

  network_device {
    bridge  = var.bridge
    vlan_id = var.vlan_tag > 0 ? var.vlan_tag : null
  }

  initialization {
    ip_config {
      ipv4 { address = "dhcp" }
    }
    user_data_file_id = proxmox_virtual_environment_file.cloudinit_cp_init.id
  }
}
```

**VM IP retrieval**: The bpg provider exposes `ipv4_addresses` on the VM resource when the QEMU guest agent is running. Cloud-init installs `qemu-guest-agent` on boot. Join nodes and outputs reference:
```hcl
proxmox_virtual_environment_vm.control_plane_init.ipv4_addresses[1][0]
```
(Index `[1]` = first non-loopback interface, `[0]` = first address)

**VM sizing** defaults (same as Harvester):
- CP: 4 CPU, 8192 MB RAM, 50 GB disk
- Worker: 4 CPU, 8192 MB RAM, 100 GB disk

**`providers/proxmox/terraform/outputs.tf`**:
- `vip_address` — echoes input VIP
- `first_cp_ip` — from VM's `ipv4_addresses`
- `control_plane_ips`, `worker_ips` — all node IPs
- `ssh_private_key` — sensitive

## 4. Cloud-Init Templates

**Key advantage**: Since Proxmox uses Ubuntu 22.04 (same as Hetzner), the cloud-init templates are very similar to Hetzner's — `apt-get` based, same RKE2 install process.

**`user-data-cp.tpl`** — Differences from Hetzner:

1. **No Hetzner metadata API** — use interface-based IP detection (same as Harvester)
2. **kube-vip static pod** — same as Harvester plan (ARP mode, leader election)
3. **VIP in TLS SANs** — added alongside node IP
4. **qemu-guest-agent** — installed so Terraform can read VM IPs
5. **No Canal `flannel.iface` override** — single NIC per VM
6. **No Hetzner firewall rules needed** — Proxmox firewall is optional, managed outside Terraform

```bash
#!/bin/bash
# cloud-init format: #cloud-config with runcmd also works,
# but raw bash is consistent with Hetzner provider

# Install qemu-guest-agent first (for Terraform IP detection)
apt-get update && apt-get install -y qemu-guest-agent curl jq
systemctl enable qemu-guest-agent && systemctl start qemu-guest-agent

# Detect IP from interface (no metadata API)
NODE_IP=""
for iface in eth0 ens18 ens19 enp0s18; do
  IP=$(ip -4 addr show "$iface" 2>/dev/null | grep -oP '(?<=inet )\d+\.\d+\.\d+\.\d+' | head -1)
  if [ -n "$IP" ] && [ "$IP" != "127.0.0.1" ]; then
    NODE_IP="$IP"; break
  fi
done

# Install RKE2, configure kube-vip, start cluster...
# (same flow as Hetzner CP template)
```

Note: Proxmox VMs with virtio NICs on Ubuntu typically get `ens18` (not `eth0` or `enp7s0`). The detection loop covers this.

**`user-data-worker.tpl`** — Nearly identical to Hetzner worker template:
- `apt-get` based (same OS)
- Interface-based IP detection instead of metadata API
- `qemu-guest-agent` installed
- Otherwise same: wait for API, install RKE2 agent, configure, start

## 5. Tests

**`internal/provider/proxmox_test.go`**:
- `TestProxmoxProvider_Name`
- `TestProxmoxProvider_ValidateConfig_WrongType`
- `TestProxmoxProvider_ValidateConfig_MissingNode`
- `TestProxmoxProvider_ValidateConfig_MissingVIP`
- `TestProxmoxProvider_ValidateConfig_InvalidVIP`
- `TestProxmoxProvider_ValidateConfig_MissingEndpoint` (env var)
- `TestProxmoxProvider_ValidateConfig_MissingAPIToken` (env var)
- `TestProxmoxProvider_DestroyInfrastructure_NoState`
- `TestProxmoxProvider_GetStatus_MissingWorkDir`
- `TestProxmoxProvider_GetKubeconfig_MissingCluster`
- Compile-time interface check: `var _ Provider = (*ProxmoxProvider)(nil)`

## 6. Example Config

```yaml
name: my-proxmox-cluster
provider:
  type: proxmox
  node: pve                   # Proxmox node hostname
  bridge: vmbr0               # Network bridge (default)
  datastore: local-lvm        # Storage datastore (default)
  vip: 192.168.1.200          # Free IP on network for kube-vip
  # vlanTag: 100              # Optional VLAN tag

kubernetes:
  version: "1.30"
  distribution: rke2

nodes:
  controlPlane:
    count: 1
  workers:
    count: 2

# Environment variables required:
#   PROXMOX_VE_ENDPOINT=https://proxmox.local:8006
#   PROXMOX_VE_API_TOKEN=terraform@pve!provider=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

## Implementation Order

1. Config changes (`config.go`) — add Proxmox fields + `"proxmox"` validation
2. Provider registration (`provider.go` factory)
3. Provider Go implementation (`proxmox.go`)
4. Terraform modules (`providers/proxmox/terraform/*`)
5. Tests (`proxmox_test.go`, update existing tests)
6. Example config

## Verification

```bash
# Unit tests
go test ./internal/...

# Build check
go build ./cmd/tdls-easy-k8s

# Manual test (requires Proxmox server)
export PROXMOX_VE_ENDPOINT=https://proxmox.local:8006
export PROXMOX_VE_API_TOKEN=terraform@pve!provider=xxx
tdls-easy-k8s init --config examples/cluster-proxmox.yaml
tdls-easy-k8s kubeconfig --cluster my-proxmox-cluster
tdls-easy-k8s validate --cluster my-proxmox-cluster
tdls-easy-k8s destroy --cluster my-proxmox-cluster --force --cleanup
```

## Proxmox Prerequisites (document for users)

1. **API token**: Create via Datacenter > Permissions > API Tokens. Needs `VM.Allocate`, `VM.Config.*`, `VM.Monitor`, `Datastore.AllocateSpace`, `Sys.Modify` permissions.
2. **Snippets enabled**: The `local` datastore must have "Snippets" content type enabled (for cloud-init files).
3. **DHCP server**: Network must have a DHCP server (most home routers provide this).
4. **Free IP for VIP**: One unused IP on the network for kube-vip.

## Known Risks

- **qemu-guest-agent timing**: Terraform reads IPs from the guest agent. If cloud-init hasn't finished installing the agent when Terraform queries, IP may be empty. The `agent { enabled = true }` block with timeout handles this.
- **Proxmox API token permissions**: Insufficient permissions cause cryptic errors. Document the required roles.
- **Snippets datastore**: If snippets aren't enabled on the target datastore, cloud-init file upload fails. Document this prereq.
- **Interface naming on Ubuntu/Proxmox**: virtio NICs get `ens18` on Ubuntu with Proxmox. Detection loop covers this but should be verified.

## Shared Work with Harvester

If both providers are implemented, the `VIP` field and kube-vip static pod manifest can be shared. Consider extracting:
- kube-vip manifest generation into a shared template snippet
- Interface-based IP detection into a shared shell function
- Both use the same pattern: generate cloud-init with kube-vip, SSH for kubeconfig, patch with VIP
