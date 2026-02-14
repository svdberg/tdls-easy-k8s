# E2E Test Findings — 2026-02-14

End-to-end test of the `app add` command with a live cluster (test-e2e) and Flux CD.

## Bugs Found

### 1. CNI mismatch: cilium config vs canal install (Critical)
- **Symptom**: No pods could start on any node — `plugin type="cilium-cni" failed: unable to connect to Cilium agent`
- **Root cause**: RKE2 is started with `--cni cilium` which writes `/etc/cni/net.d/05-cilium.conflist`, but the default Canal Helm chart is also installed, writing `10-canal.conflist`. Cilium wins alphabetically but is never actually installed.
- **Workaround**: Manually removed `05-cilium.conflist` from all nodes via `kubectl exec` into canal pods.
- **Fix needed**: Either install Cilium properly when `--cni cilium` is set, or don't pass `--cni cilium` to RKE2 if Canal is the intended CNI.
- **File**: Likely in terraform userdata / RKE2 config templates.

### 2. `kubeconfig` command fails after `init` (Medium)
- **Symptom**: `tdls-easy-k8s kubeconfig --cluster=test-e2e` errors with `open ~/.tdls-k8s/clusters/test-e2e/cluster.yaml: no such file or directory`
- **Root cause**: The `init` command doesn't persist a cluster config file that the `kubeconfig` command expects.
- **Workaround**: Downloaded kubeconfig directly from S3 using the terraform output command.
- **Fix needed**: `init` should save cluster metadata to `~/.tdls-k8s/clusters/<name>/cluster.yaml`.

### 3. Kubeconfig S3 download has wrong server address (Medium)
- **Symptom**: Downloaded kubeconfig has `server: https://:6443` (empty host) after sed replacement.
- **Root cause**: The kubeconfig in S3 doesn't use `127.0.0.1` in the expected format, so the sed replacement in the terraform output doesn't match.
- **Workaround**: Used `kubectl config set-cluster` to manually set the NLB DNS.
- **Fix needed**: Fix the sed command in terraform outputs, or have the `kubeconfig` command handle this.

### 4. SSM agents not available after cluster creation (Low)
- **Symptom**: `aws ssm describe-instance-information` returns empty list after cluster creation completes.
- **Root cause**: SSM agents may not be persistently registered, or IAM permissions for SSM are session-scoped.
- **Impact**: Can't SSH into nodes for debugging via SSM after initial setup.

### 5. Flannel crash on some nodes after CNI fix (Low)
- **Symptom**: Canal's flannel container crashes with `failed to set interface flannel.1 to UP state: address already in use` on 3 of 5 nodes.
- **Root cause**: The flannel.1 VXLAN interface was left in a stale state from previous failed attempts.
- **Workaround**: Deleting and recreating canal pods eventually resolved it on some nodes.

## Improvements for `app add`

### 6. OCI Helm repository support needed (High)
- **Symptom**: Bitnami chart failed with `unsupported protocol scheme "oci"`.
- **Root cause**: `generateHelmRepositoryYAML` only generates standard HTTP HelmRepository manifests. Major repos (Bitnami, etc.) have migrated to OCI format.
- **Fix needed**: Add `--repo-type` flag (default `default`, option `oci`). When `oci`, generate `spec.type: oci` and accept `oci://` URLs.

### 7. Missing layer directory causes Flux failure (High)
- **Symptom**: `infrastructure` Kustomization fails with `kustomization path not found` when only an app is added. Since `apps` depends on `infrastructure`, apps can't deploy either.
- **Root cause**: `gitops setup` creates Kustomizations for both `infrastructure` and `apps` paths, but `app add` only creates files for one layer. The other layer directory doesn't exist in the repo.
- **Fix needed**: Either:
  - (a) `app add` should create empty `.gitkeep` files for missing layer directories, or
  - (b) `gitops setup` instructions should mention creating both directories, or
  - (c) `app add` should auto-create both layer directories when using `--output-dir`.

### 8. Flux health check blocks revision updates (Info)
- **Symptom**: When a Kustomization is stuck waiting on a health check (e.g., broken HelmRelease), it won't apply newer git revisions.
- **Impact**: Fixing a broken manifest in git doesn't take effect until the health check times out (9m30s default).
- **Workaround**: Suspend and resume the Kustomization.
- **Possible improvement**: Document this behavior in the `app add` output / next steps.

### 9. `app add` stdout mode lacks file path comments (Minor)
- **Current behavior**: Stdout mode prints `# <path>` comments before each YAML document.
- **Improvement**: Consider adding a note that users need to create both layer directories when setting up their gitops repo for the first time.

## Test Results Summary

| Scenario | Result |
|----------|--------|
| `app add` writes correct files to `--output-dir` | PASS |
| Flux picks up manifests from git and deploys HelmRelease | PASS |
| `--depends-on` generates correct `dependsOn` in Kustomization CRD | PASS |
| Dependency ordering enforced by Flux (podinfo before podinfo-frontend) | PASS |
| App removal via file deletion triggers Flux prune | PASS |
| Stdout mode prints valid YAML | PASS (manual) |
