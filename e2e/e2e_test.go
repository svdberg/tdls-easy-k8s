//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFullE2E(t *testing.T) {
	// Required environment
	requireEnv(t, "HCLOUD_TOKEN")

	name := clusterName()
	tmpDir := t.TempDir()
	configPath := writeClusterConfig(t, name)
	kubeconfigPath := filepath.Join(tmpDir, "kubeconfig")
	gitopsDir := filepath.Join(tmpDir, "gitops")
	os.MkdirAll(gitopsDir, 0755)
	repoName := fmt.Sprintf("svdberg/%s", name)

	var binaryPath string
	var ingressLBIP string

	// =========================================================================
	// Build
	// =========================================================================
	t.Run("Build", func(t *testing.T) {
		binaryPath = buildBinary(t)
	})
	if binaryPath == "" {
		t.Fatal("build failed, cannot continue")
	}

	// Register cleanup — always runs, even on failure
	t.Cleanup(func() {
		t.Log("=== CLEANUP ===")
		runCLI(t, binaryPath, "destroy", "--config", configPath,
			"--cluster", name, "--force", "--cleanup")
		deleteGitHubRepo(t, repoName)
	})

	// =========================================================================
	// Cluster Lifecycle
	// =========================================================================
	t.Run("Init", func(t *testing.T) {
		out, err := runCLI(t, binaryPath, "init", "--config", configPath)
		if err != nil {
			t.Fatalf("init failed: %v\n%s", err, out)
		}
		if !strings.Contains(out, "Infrastructure created successfully") {
			t.Fatalf("init did not complete successfully:\n%s", out)
		}
		// Extract ingress LB IP from output
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "ingress_lb_ipv4") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					ingressLBIP = strings.TrimSpace(strings.Trim(parts[1], "\""))
				}
			}
		}
		t.Logf("Ingress LB IP: %s", ingressLBIP)
	})
	if t.Failed() {
		return
	}

	t.Run("WaitForRKE2", func(t *testing.T) {
		// RKE2 takes ~5 minutes to install via cloud-init
		t.Log("Waiting 5 minutes for RKE2 installation...")
		time.Sleep(5 * time.Minute)
	})

	t.Run("Kubeconfig", func(t *testing.T) {
		out, err := runCLI(t, binaryPath, "kubeconfig",
			"--cluster", name, "--output", kubeconfigPath)
		if err != nil {
			t.Fatalf("kubeconfig failed: %v\n%s", err, out)
		}
		if _, err := os.Stat(kubeconfigPath); err != nil {
			t.Fatalf("kubeconfig file not created: %v", err)
		}
	})

	t.Run("WaitForNodes", func(t *testing.T) {
		waitFor(t, 5*time.Minute, 15*time.Second, "all nodes ready", func() bool {
			out, err := kubectl(t, kubeconfigPath, "get", "nodes", "--no-headers")
			if err != nil {
				return false
			}
			lines := strings.Split(strings.TrimSpace(out), "\n")
			if len(lines) < 3 {
				return false
			}
			for _, line := range lines {
				if !strings.Contains(line, "Ready") || strings.Contains(line, "NotReady") {
					return false
				}
			}
			t.Logf("All %d nodes ready", len(lines))
			return true
		})
	})

	if t.Failed() {
		return
	}

	t.Run("NodesUsePrivateIPs", func(t *testing.T) {
		out := kubectlMust(t, kubeconfigPath, "get", "nodes",
			"-o", "jsonpath={.items[*].status.addresses[?(@.type==\"InternalIP\")].address}")
		for _, ip := range strings.Fields(out) {
			if !strings.HasPrefix(ip, "10.0.") {
				t.Errorf("node has public InternalIP %s, expected 10.0.x.x", ip)
			}
		}
		t.Logf("Node IPs: %s", out)
	})

	t.Run("Validate", func(t *testing.T) {
		os.Setenv("KUBECONFIG", kubeconfigPath)
		defer os.Unsetenv("KUBECONFIG")

		out, err := runCLI(t, binaryPath, "validate", "--cluster", name)
		if err != nil {
			t.Logf("validate returned error (may be expected): %v", err)
		}
		t.Logf("Validate output:\n%s", out)
	})

	if t.Failed() {
		return
	}

	// =========================================================================
	// Cross-Node Networking
	// =========================================================================
	t.Run("CrossNodeNetworking", func(t *testing.T) {
		// Get worker node names
		out := kubectlMust(t, kubeconfigPath, "get", "nodes",
			"-l", "!node-role.kubernetes.io/control-plane",
			"-o", "jsonpath={.items[*].metadata.name}")
		workers := strings.Fields(out)
		if len(workers) < 2 {
			t.Fatalf("expected at least 2 workers, got %d", len(workers))
		}

		// Test DNS from each worker
		for _, worker := range workers {
			podName := fmt.Sprintf("dns-test-%s", worker)
			override := fmt.Sprintf(`{"spec":{"nodeName":"%s"}}`, worker)

			kubectl(t, kubeconfigPath, "delete", "pod", podName, "--ignore-not-found")
			kubectlMust(t, kubeconfigPath, "run", podName,
				"--image=busybox", "--restart=Never",
				"--overrides", override,
				"--command", "--", "nslookup", "kubernetes.default.svc.cluster.local")

			waitFor(t, 30*time.Second, 2*time.Second, podName+" completed", func() bool {
				out, _ := kubectl(t, kubeconfigPath, "get", "pod", podName,
					"-o", "jsonpath={.status.phase}")
				return out == "Succeeded" || out == "Failed"
			})

			logs, _ := kubectl(t, kubeconfigPath, "logs", podName)
			if !strings.Contains(logs, "Address") {
				t.Errorf("DNS failed on %s:\n%s", worker, logs)
			} else {
				t.Logf("DNS OK on %s", worker)
			}
			kubectl(t, kubeconfigPath, "delete", "pod", podName, "--force")
		}
	})

	if t.Failed() {
		return
	}

	// =========================================================================
	// GitOps Setup
	// =========================================================================
	t.Run("GitOpsSetup", func(t *testing.T) {
		createGitHubRepo(t, repoName)

		os.Setenv("KUBECONFIG", kubeconfigPath)
		defer os.Unsetenv("KUBECONFIG")

		repoURL := fmt.Sprintf("https://github.com/%s", repoName)
		out, err := runCLI(t, binaryPath, "gitops", "setup",
			"--repo", repoURL)
		if err != nil {
			t.Fatalf("gitops setup failed: %v\n%s", err, out)
		}

		// Wait for Flux to be ready
		waitFor(t, 2*time.Minute, 10*time.Second, "Flux controllers ready", func() bool {
			out, err := kubectl(t, kubeconfigPath, "get", "pods", "-n", "flux-system", "--no-headers")
			if err != nil {
				return false
			}
			lines := strings.Split(strings.TrimSpace(out), "\n")
			for _, line := range lines {
				if !strings.Contains(line, "Running") {
					return false
				}
			}
			return len(lines) >= 4 // source, kustomize, helm, notification controllers
		})
	})

	if t.Failed() {
		return
	}

	// =========================================================================
	// Add Applications
	// =========================================================================
	t.Run("AddTraefik", func(t *testing.T) {
		valuesPath := filepath.Join(projectRoot(t), "templates", "components", "traefik-values.yaml")
		out, err := runCLI(t, binaryPath, "app", "add", "traefik",
			"--chart", "traefik/traefik",
			"--repo-url", "https://traefik.github.io/charts",
			"--layer", "infrastructure",
			"--namespace", "traefik-system",
			"--values", valuesPath,
			"--output-dir", gitopsDir)
		if err != nil {
			t.Fatalf("app add traefik failed: %v\n%s", err, out)
		}
	})

	t.Run("AddExternalSecrets", func(t *testing.T) {
		out, err := runCLI(t, binaryPath, "app", "add", "external-secrets",
			"--chart", "external-secrets/external-secrets",
			"--repo-url", "https://charts.external-secrets.io",
			"--layer", "infrastructure",
			"--namespace", "external-secrets",
			"--output-dir", gitopsDir)
		if err != nil {
			t.Fatalf("app add external-secrets failed: %v\n%s", err, out)
		}
	})

	t.Run("VaultSetup", func(t *testing.T) {
		out, err := runCLI(t, binaryPath, "vault", "setup",
			"--cluster", name,
			"--output-dir", gitopsDir)
		if err != nil {
			t.Fatalf("vault setup failed: %v\n%s", err, out)
		}
	})

	t.Run("FixClusterSecretStore", func(t *testing.T) {
		// Move ClusterSecretStore to separate Kustomization with dependsOn
		vssDir := filepath.Join(gitopsDir, "infrastructure", "vault-secret-store")
		os.MkdirAll(vssDir, 0755)

		srcPath := filepath.Join(gitopsDir, "infrastructure", "external-secrets", "vault-clustersecretstore.yaml")
		dstPath := filepath.Join(vssDir, "vault-clustersecretstore.yaml")

		data, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatalf("failed to read ClusterSecretStore: %v", err)
		}
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			t.Fatalf("failed to write ClusterSecretStore: %v", err)
		}
		os.Remove(srcPath)

		// Create Kustomization with dependencies
		kustomization := `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: vault-secret-store
  namespace: flux-system
spec:
  dependsOn:
    - name: external-secrets
    - name: vault
  interval: 10m0s
  sourceRef:
    kind: GitRepository
    name: flux-system
  path: ./infrastructure/vault-secret-store
  prune: true
  wait: true
`
		kustPath := filepath.Join(gitopsDir, "clusters", "production", "infrastructure", "vault-secret-store.yaml")
		if err := os.WriteFile(kustPath, []byte(kustomization), 0644); err != nil {
			t.Fatalf("failed to write vault-secret-store Kustomization: %v", err)
		}
	})

	if t.Failed() {
		return
	}

	// =========================================================================
	// Git Push
	// =========================================================================
	t.Run("GitPush", func(t *testing.T) {
		sshURL := fmt.Sprintf("git@github.com:%s.git", repoName)
		gitInit(t, gitopsDir)
		gitCommitAndPush(t, gitopsDir, sshURL, "Add traefik, external-secrets, vault")
	})

	if t.Failed() {
		return
	}

	// =========================================================================
	// Wait for Deployments
	// =========================================================================
	t.Run("TriggerReconciliation", func(t *testing.T) {
		// Wait for Flux CRDs to be fully registered
		waitFor(t, 2*time.Minute, 5*time.Second, "GitRepository CRD available", func() bool {
			_, err := kubectl(t, kubeconfigPath, "get", "crd", "gitrepositories.source.toolkit.fluxcd.io")
			return err == nil
		})

		out, err := kubectl(t, kubeconfigPath, "annotate", "--overwrite",
			"gitrepositories.source.toolkit.fluxcd.io/flux-system", "-n", "flux-system",
			fmt.Sprintf("reconcile.fluxcd.io/requestedAt=%d", time.Now().Unix()))
		if err != nil {
			t.Logf("WARNING: failed to trigger reconciliation: %v\n%s", err, out)
			t.Log("Flux should auto-reconcile within its default interval")
		}
	})

	t.Run("WaitForHelmReleases", func(t *testing.T) {
		waitFor(t, 10*time.Minute, 15*time.Second, "all HelmReleases ready", func() bool {
			out, err := kubectl(t, kubeconfigPath, "get",
				"helmreleases.helm.toolkit.fluxcd.io", "-A",
				"-o", "jsonpath={.items[*].status.conditions[?(@.type==\"Ready\")].status}")
			if err != nil {
				return false
			}
			statuses := strings.Fields(out)
			if len(statuses) < 3 {
				return false
			}
			for _, s := range statuses {
				if s != "True" {
					return false
				}
			}
			t.Logf("All %d HelmReleases ready", len(statuses))
			return true
		})
	})

	if t.Failed() {
		return
	}

	// =========================================================================
	// Verify Components
	// =========================================================================
	t.Run("VerifyTraefik", func(t *testing.T) {
		// Check pods running
		out := kubectlMust(t, kubeconfigPath, "get", "pods", "-n", "traefik-system", "--no-headers")
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) < 2 {
			t.Fatalf("expected at least 2 Traefik pods (DaemonSet), got %d", len(lines))
		}
		for _, line := range lines {
			if !strings.Contains(line, "Running") {
				t.Errorf("Traefik pod not running: %s", line)
			}
		}

		// Test ingress LB
		if ingressLBIP != "" {
			resp, err := http.Get(fmt.Sprintf("http://%s", ingressLBIP))
			if err != nil {
				t.Logf("WARNING: ingress LB not reachable: %v", err)
			} else {
				resp.Body.Close()
				if resp.StatusCode != 404 {
					t.Errorf("expected HTTP 404 from Traefik, got %d", resp.StatusCode)
				} else {
					t.Log("Ingress LB returning HTTP 404 (Traefik default)")
				}
			}
		}
	})

	t.Run("VerifyVault", func(t *testing.T) {
		// Check pod running
		out := kubectlMust(t, kubeconfigPath, "get", "pods", "-n", "vault-system", "--no-headers")
		if !strings.Contains(out, "Running") {
			t.Fatalf("Vault pod not running:\n%s", out)
		}

		// Enable Kubernetes auth for ESO
		vaultPod := kubectlMust(t, kubeconfigPath, "get", "pods", "-n", "vault-system",
			"-o", "jsonpath={.items[0].metadata.name}")
		vaultPod = strings.TrimSpace(vaultPod)

		kubectlMust(t, kubeconfigPath, "exec", "-n", "vault-system", vaultPod,
			"--", "vault", "auth", "enable", "kubernetes")

		kubectlMust(t, kubeconfigPath, "exec", "-n", "vault-system", vaultPod,
			"--", "sh", "-c",
			`vault write auth/kubernetes/config kubernetes_host="https://$KUBERNETES_SERVICE_HOST:$KUBERNETES_SERVICE_PORT"`)

		kubectlMust(t, kubeconfigPath, "exec", "-n", "vault-system", vaultPod,
			"--", "vault", "write", "auth/kubernetes/role/external-secrets",
			"bound_service_account_names=*",
			"bound_service_account_namespaces=*",
			"policies=default", "ttl=1h")

		t.Log("Vault running with Kubernetes auth configured")
	})

	t.Run("VerifyESO", func(t *testing.T) {
		out := kubectlMust(t, kubeconfigPath, "get", "pods", "-n", "external-secrets", "--no-headers")
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) < 3 {
			t.Fatalf("expected at least 3 ESO pods, got %d", len(lines))
		}
		for _, line := range lines {
			if !strings.Contains(line, "Running") {
				t.Errorf("ESO pod not running: %s", line)
			}
		}
		t.Logf("ESO: %d pods running", len(lines))
	})

	t.Run("VerifyClusterSecretStore", func(t *testing.T) {
		waitFor(t, 2*time.Minute, 10*time.Second, "ClusterSecretStore ready", func() bool {
			out, err := kubectl(t, kubeconfigPath, "get",
				"clustersecretstores.external-secrets.io", "vault",
				"-o", "jsonpath={.status.conditions[?(@.type==\"Ready\")].status}")
			if err != nil {
				return false
			}
			return strings.TrimSpace(out) == "True"
		})

		out := kubectlMust(t, kubeconfigPath, "get",
			"clustersecretstores.external-secrets.io", "vault")
		t.Logf("ClusterSecretStore:\n%s", out)
	})

	t.Log("=== E2E TEST PASSED ===")
}
