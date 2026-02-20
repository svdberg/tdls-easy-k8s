# Plan: Add Harvester HCI Provider

## Context

The project supports Hetzner and AWS as cloud providers. Harvester is an on-prem HCI solution (SUSE) that runs VMs via KubeVirt. Adding it enables bare-metal/on-prem Kubernetes provisioning through the same CLI. Key differences from cloud providers: no cloud LB (use kube-vip), no metadata API (interface-based IP detection), VMs managed via Harvester API (kubeconfig-based).

## Design Decisions (confirmed with user)

- **API Load Balancing**: kube-vip static pod on CP nodes (ARP mode, leader election)
- **IP Discovery**: Interface-based detection (try `eth0`, `enp1s0`, `enp2s0`, fallback to `hostname -I`)
- **Networking**: VLAN network, user provides VLAN ID
- **VM Image**: openSUSE Leap (zypper, not apt)
- **Auth**: `HARVESTER_KUBECONFIG` env var pointing to Harvester cluster kubeconfig

## Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `internal/config/config.go` | **Modify** | Add `HarvesterNetworkConfig`, `VIP`, `Namespace` fields; accept `"harvester"` in validation |
| `internal/provider/provider.go` | **Modify** | Add `"harvester"` case to `GetProvider()` factory |
| `internal/provider/harvester.go` | **Create** | Full provider implementation (~500 lines) |
| `providers/harvester/terraform/versions.tf` | **Create** | Provider requirements (harvester, tls, random) |
| `providers/harvester/terraform/variables.tf` | **Create** | All input variables |
| `providers/harvester/terraform/main.tf` | **Create** | SSH key, image, VLAN network, CP VMs, worker VMs, cloud-init secrets |
| `providers/harvester/terraform/outputs.tf` | **Create** | vip_address, first_cp_ip, node IPs, ssh_private_key |
| `providers/harvester/terraform/user-data-cp.tpl` | **Create** | CP cloud-init: RKE2 server + kube-vip static pod |
| `providers/harvester/terraform/user-data-worker.tpl` | **Create** | Worker cloud-init: RKE2 agent |
| `internal/provider/harvester_test.go` | **Create** | Unit tests (validation, state-missing, interface compliance) |
| `internal/provider/provider_test.go` | **Modify** | Add `TestGetProvider_Harvester` |
| `internal/config/config_test.go` | **Modify** | Add harvester validation test, update error message test |
| `examples/cluster-harvester.yaml` | **Create** | Example config file |

## 1. Config Changes

**`internal/config/config.go`** — Add to `ProviderConfig`:

```go
type HarvesterNetworkConfig struct {
    VlanID int `yaml:"vlanId"`
}

// Add to ProviderConfig struct:
Network   HarvesterNetworkConfig `yaml:"network,omitempty"`   // Harvester VLAN config
VIP       string                 `yaml:"vip,omitempty"`       // kube-vip virtual IP
Namespace string                 `yaml:"namespace,omitempty"` // Harvester VM namespace
```

Add `"harvester"` to the provider type validation. Default `Namespace` to `"default"` in loader.

## 2. Provider Implementation

**`internal/provider/harvester.go`** — Follow exact Hetzner pattern:

- `HarvesterProvider` struct with `workDir` field
- `ValidateConfig`: check type=harvester, require VLAN ID, require VIP (valid IPv4), require `HARVESTER_KUBECONFIG` env var pointing to existing file
- `CreateInfrastructure`: copy TF modules from `providers/harvester/terraform/` to `~/.tdls-k8s/clusters/{name}/terraform`, generate tfvars.json, run `tofu init/plan/apply`
- `DestroyInfrastructure`: check state file, `tofu destroy -auto-approve`
- `GetKubeconfig`: SSH into first CP node (using generated ED25519 key), download `/etc/rancher/rke2/rke2.yaml`, patch server URL to use VIP
- All validation methods: delegate to `common.go` kubectl functions (identical to Hetzner)
- `generateTerraformVars`: map config to JSON — `cluster_name`, `namespace`, `vlan_id`, `vip_address`, `cp_count`, `worker_count`, `network_cidr`, `kubernetes_version`
- Pass `HARVESTER_KUBECONFIG` through to tofu subprocess env

**`internal/provider/provider.go`** — Add to factory switch:
```go
case "harvester":
    return NewHarvesterProvider(), nil
```

## 3. Terraform Modules

**`providers/harvester/terraform/main.tf`** — Key resources:

| Resource | Purpose |
|----------|---------|
| `tls_private_key.ssh` | ED25519 SSH key (same as Hetzner) |
| `harvester_ssh_key.cluster` | Register SSH key with Harvester |
| `harvester_image.os` | Download openSUSE Leap qcow2 from `download.opensuse.org` |
| `harvester_network.vlan` | VLAN network with user-provided VLAN ID |
| `random_password.cluster_token` | RKE2 cluster join token |
| `harvester_virtualmachine.control_plane_init` | First CP node (bootstrap) |
| `harvester_virtualmachine.control_plane_join` | Additional CP nodes (count - 1) |
| `harvester_virtualmachine.worker` | Worker nodes |

Cloud-init is passed via `cloudinit { user_data }` block (base64-encoded templatefile). Each VM has a single virtio NIC on the VLAN network with `wait_for_lease = true` so Terraform can capture the IP.

**VM sizing** uses direct CPU/memory/disk variables (not cloud instance types):
- CP default: 4 CPU, 8 GB RAM, 50 GB disk
- Worker default: 4 CPU, 8 GB RAM, 100 GB disk

**`providers/harvester/terraform/outputs.tf`**:
- `vip_address` — echoes back the input VIP (used by Go to patch kubeconfig)
- `first_cp_ip` — from `control_plane_init.network_interface[0].ip_address`
- `control_plane_ips`, `worker_ips` — all node IPs
- `ssh_private_key` — sensitive, for kubeconfig download

## 4. Cloud-Init Templates

**`user-data-cp.tpl`** — Differences from Hetzner:

1. **Package manager**: `zypper --non-interactive install` instead of `apt-get`
2. **IP detection**: Loop through `eth0`/`enp1s0`/`enp2s0` interfaces instead of Hetzner metadata API
3. **kube-vip static pod**: Written to `/var/lib/rancher/rke2/server/manifests/kube-vip.yaml`
   - ARP mode (L2, suitable for VLAN)
   - Leader election across CP nodes
   - Uses same interface as node IP
   - VIP address from config
   - Mounts `/etc/rancher/rke2/rke2.yaml` as kubeconfig
4. **VIP in TLS SANs**: Added alongside node IP and 127.0.0.1
5. **No Canal `flannel.iface` override needed** (single NIC per VM, unlike Hetzner's dual public+private)

**`user-data-worker.tpl`** — Same structure as Hetzner worker template but with zypper and interface-based IP detection.

## 5. Tests

**`internal/provider/harvester_test.go`**:
- `TestHarvesterProvider_Name`
- `TestHarvesterProvider_ValidateConfig_WrongType`
- `TestHarvesterProvider_ValidateConfig_MissingVlanID`
- `TestHarvesterProvider_ValidateConfig_MissingVIP`
- `TestHarvesterProvider_ValidateConfig_InvalidVIP`
- `TestHarvesterProvider_ValidateConfig_MissingKubeconfig`
- `TestHarvesterProvider_DestroyInfrastructure_NoState`
- `TestHarvesterProvider_GetStatus_MissingWorkDir`
- `TestHarvesterProvider_GetKubeconfig_MissingCluster`
- Compile-time interface check: `var _ Provider = (*HarvesterProvider)(nil)`

## 6. Example Config

```yaml
name: my-harvester-cluster
provider:
  type: harvester
  namespace: default
  vip: 10.0.1.100          # Free IP on VLAN for kube-vip
  network:
    vlanId: 100
  vpc:
    cidr: 10.0.0.0/16
kubernetes:
  version: "1.30"
  distribution: rke2
nodes:
  controlPlane:
    count: 1
  workers:
    count: 2
# Requires: HARVESTER_KUBECONFIG=/path/to/harvester.kubeconfig
```

## Implementation Order

1. Config changes (`config.go`, loader) — foundation
2. Provider registration (`provider.go` factory) — unlocks testing
3. Provider Go implementation (`harvester.go`) — core logic
4. Terraform modules (`providers/harvester/terraform/*`) — infrastructure
5. Tests (`harvester_test.go`, update existing tests) — verification
6. Example config — documentation

## Verification

```bash
# Unit tests
go test ./internal/...

# Build check
go build ./cmd/tdls-easy-k8s

# Manual test (requires Harvester cluster + VLAN)
export HARVESTER_KUBECONFIG=/path/to/harvester.kubeconfig
tdls-easy-k8s init --config examples/cluster-harvester.yaml
tdls-easy-k8s kubeconfig --cluster my-harvester-cluster
tdls-easy-k8s validate --cluster my-harvester-cluster
tdls-easy-k8s destroy --cluster my-harvester-cluster --force --cleanup
```

## Known Risks

- **openSUSE cloud-init**: May need `#cloud-config` + `runcmd` wrapper if raw bash scripts aren't executed. Test early.
- **Harvester TF provider maturity**: Pin to `~> 0.6`, check exact `cloudinit` block schema (inline `user_data` vs kubernetes secret reference).
- **Image download time**: First `tofu apply` downloads ~600MB qcow2. Document the delay.
- **Interface naming**: virtio NIC names vary by guest OS. Detection loop + fallback covers common cases.
