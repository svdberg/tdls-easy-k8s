# E2E Test Findings — 2026-02-15

End-to-end test of the full CLI workflow: init, kubeconfig, validate, status, gitops setup, and app add.

## Test Configuration

- **Cluster name**: e2e-test-2
- **Control plane**: 1 node (t3.medium)
- **Workers**: 2 nodes (t3.large)
- **Region**: us-east-1
- **Kubernetes**: v1.34.3+rke2r3

## Test Results Summary

| Step | Command | Result | Notes |
|------|---------|--------|-------|
| 1 | `init --config=e2e-test-config.yaml` | PASS | Cluster created, 3 phases completed |
| 2 | `kubeconfig --cluster=e2e-test-2` | FAIL | Missing cluster.yaml (known bug #2) |
| 2b | S3 download workaround | PASS | Kubeconfig retrieved from S3 |
| 2c | Kubeconfig server address | FAIL | Empty server address (known bug #3) |
| 2d | Manual NLB DNS fix | PASS | Fixed with `kubectl config set-cluster` |
| 3 | `kubectl get nodes` | PASS | All 3 nodes Ready |
| 4 | `validate --cluster=e2e-test-2` | FAIL | Missing cluster.yaml (same as bug #2) |
| 4b | `status --cluster=e2e-test-2` | FAIL | Missing cluster.yaml (same as bug #2) |
| 5 | kubectl integration | PASS | Canal CNI running, CoreDNS running, no errors |
| 6 | `gitops setup` | PASS | Flux installed, all controllers ready |
| 7 | `app add podinfo` | PARTIAL | Files generated but kustomization path is wrong (new bug) |
| 7b | Flux reconciliation | PASS | HelmRelease deployed, pod running |

## Bugs Found

### 1. `init` does not persist cluster.yaml — all post-init commands fail (Critical, Regression from previous test)

- **Symptom**: `kubeconfig`, `validate`, and `status` all fail with:
  ```
  open ~/.tdls-k8s/clusters/e2e-test-2/cluster.yaml: no such file or directory
  ```
- **Root cause**: The `init` command creates infrastructure but does not save the cluster config to `~/.tdls-k8s/clusters/<name>/cluster.yaml`.
- **Impact**: **Every command that uses `--cluster` is broken** after init. Users cannot use kubeconfig, validate, status, or destroy without workarounds.
- **Workaround**: Download kubeconfig directly from S3 bucket.
- **Fix needed**: `init` must persist the cluster config file for subsequent commands.
- **Status**: Same as bug #2 from 2026-02-14 test. Still unfixed.

### 2. Kubeconfig S3 download has empty server address — only affects direct S3 download (Medium, Partially fixed)

- **Symptom**: Kubeconfig downloaded directly from S3 has `server: https://:6443` (empty host).
- **Root cause**: The kubeconfig in S3 uses `127.0.0.1` but the sed replacement to inject the NLB DNS doesn't match the format.
- **Workaround**: Use the CLI command with `--config` flag: `kubeconfig -c <name> --config <config.yaml>`. This correctly patches the server address with the NLB DNS.
- **Note**: The `kubeconfig` command works correctly when `--config` is provided. The bug only manifests when downloading directly from S3.
- **Status**: Partially fixed. CLI workaround available via `--config` flag.

### 3. `app add` writes kustomization file to wrong path (Medium, NEW)

- **Symptom**: When using `--output-dir=/path/to/clusters/production` and `--gitops-path=clusters/production` (default), the app Kustomization CRD file is written to:
  ```
  <output-dir>/clusters/production/apps/podinfo.yaml
  ```
  instead of:
  ```
  <output-dir>/apps/podinfo.yaml
  ```
- **Root cause**: The `--gitops-path` value is appended to `--output-dir`, but `--output-dir` already points to the gitops path directory. The path is effectively doubled.
- **Impact**: The Kustomization CRD ends up in a nested directory that Flux doesn't scan. The app still deploys because the HelmRelease and HelmRepository are in the correct `apps/podinfo/` directory which the `apps` Kustomization does scan.
- **Workaround**: Manually move the file to the correct path.
- **Fix needed**: Either don't prepend `--gitops-path` to the kustomization output path when `--output-dir` is set, or document that `--output-dir` should point to the repo root (not the gitops path).

### 4. `podinfo` Kustomization CRD path doesn't match directory structure (Low, NEW)

- **Symptom**: The generated `podinfo.yaml` Kustomization CRD has `path: ./apps/podinfo` but the actual app manifests are in `apps/podinfo/` with individual YAML files (helmrepository.yaml, helmrelease.yaml), not a kustomization.yaml file.
- **Impact**: The `podinfo` Kustomization stays in `False` state:
  ```
  kustomization path not found: stat /tmp/kustomization-1052039139/apps/podinfo: no such file or directory
  ```
  However, the app still deploys correctly because the parent `apps` Kustomization picks up all files under `apps/`.
- **Fix options**:
  - (a) Generate a `kustomization.yaml` file inside `apps/podinfo/` that references the HelmRepository and HelmRelease, or
  - (b) Don't generate the per-app Kustomization CRD since the parent `apps` Kustomization already handles it, or
  - (c) Change the per-app Kustomization path to just `./apps` if it's meant to cover the whole apps layer

### 5. `validate` fails even with `--config` flag (Medium, NEW)

- **Symptom**: `validate -c e2e-test-2 --config e2e-test-config.yaml` runs but every check fails with "API server is not responding", "exit status 1", etc.
- **Root cause**: `validate` downloads kubeconfig via a different code path (likely `downloadKubeconfig` in the provider) that doesn't patch the server address with the NLB DNS. `status` and `kubeconfig` commands use a path that does the patching.
- **Impact**: The validate command is unusable for clusters created with `init`.
- **Fix needed**: Ensure `validate` uses the same kubeconfig retrieval path as `status`, or have all provider methods use a shared `GetKubeconfig` that patches the server address.

## Verified Fixes

### CNI Mismatch Fix — VERIFIED

- **Previous bug**: RKE2 defaulted to `--cni cilium` but Canal was actually installed, causing all pods to fail.
- **Fix applied**: Changed default CNI from `cilium` to `canal` in terraform variables.
- **Result**: **CONFIRMED FIXED**. All 3 `rke2-canal` pods are running. No cilium references in CNI config. All pods started successfully without manual intervention.
  ```
  kube-system   rke2-canal-57xl8   2/2   Running
  kube-system   rke2-canal-5wgdc   2/2   Running
  kube-system   rke2-canal-fs876   2/2   Running
  ```

## Improvements Noted

### 5. Single control plane node worked fine

- 1 CP + 2 workers is a viable configuration for testing/dev.
- All 3 phases completed successfully.
- RKE2 installed cleanly on all nodes.
- Total init time: ~15 minutes.

### 6. All commands work with `--config` flag as workaround for missing cluster.yaml

- Passing `--config <original-config.yaml>` bypasses the missing cluster.yaml issue.
- **`status --config`**: Works correctly. Shows nodes, components, canal CNI.
- **`kubeconfig --config`**: Works correctly. Downloads and patches server address with NLB DNS.
- **`monitor --config`**: Works correctly. Launches k9s with proper kubeconfig.
- **`validate --config`**: Runs but all checks fail — appears to use a separate kubeconfig download path that doesn't patch the server address.
- **`gitops setup`**: Uses KUBECONFIG env var directly, doesn't need `--cluster` at all.
- **`app add`**: Pure file generator, doesn't need cluster access.
- **Recommendation**: Either fix `init` to persist cluster.yaml, or document that `--config` must be passed to all subsequent commands.

### 7. Flux dependency ordering works correctly

- The `apps` Kustomization correctly waits for `infrastructure` to be ready before applying.
- Once infrastructure is Ready, apps starts reconciling and deploys the HelmRelease.

## Environment

- **CLI version**: dev (commit 576fad5)
- **Kubernetes**: v1.34.3+rke2r3
- **Flux**: v2 (latest manifests)
- **Podinfo**: v6.10.1 (deployed via HelmRelease)
- **AWS Region**: us-east-1
- **Date**: 2026-02-15

## Cluster Status

Cluster `e2e-test-2` is **running** and has podinfo deployed. Ready for manual inspection via `monitor` command.

```
NLB DNS: e2e-test-2-nlb-766bc123ace71356.elb.us-east-1.amazonaws.com
Kubeconfig: /tmp/e2e-kubeconfig
```
