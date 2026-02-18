//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// clusterName returns a unique cluster name based on the current timestamp.
func clusterName() string {
	return fmt.Sprintf("e2e-%d", time.Now().Unix())
}

// buildBinary compiles the CLI binary into the project's bin/ directory.
// The binary must be next to providers/ so it can find terraform modules.
func buildBinary(t *testing.T) string {
	t.Helper()
	root := projectRoot(t)
	binPath := filepath.Join(root, "bin", "tdls-easy-k8s")

	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/tdls-easy-k8s/")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, out)
	}
	return binPath
}

// projectRoot returns the absolute path to the project root.
func projectRoot(t *testing.T) string {
	t.Helper()
	// e2e tests run from the e2e/ directory, project root is one level up
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	// If we're in the e2e directory, go up one level
	if filepath.Base(dir) == "e2e" {
		return filepath.Dir(dir)
	}
	return dir
}

// runCLI executes the CLI binary with the given arguments and returns the output.
// It runs from the project root so the binary can find providers/hetzner/terraform/.
func runCLI(t *testing.T, binary string, args ...string) (string, error) {
	t.Helper()
	t.Logf("Running: %s %s", filepath.Base(binary), strings.Join(args, " "))
	cmd := exec.Command(binary, args...)
	cmd.Dir = projectRoot(t)
	cmd.Env = append(os.Environ())
	out, err := cmd.CombinedOutput()
	output := string(out)
	if len(output) > 0 {
		t.Logf("Output:\n%s", output)
	}
	return output, err
}

// kubectl runs kubectl with the given arguments using the provided kubeconfig.
func kubectl(t *testing.T, kubeconfigPath string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("kubectl", args...)
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// kubectlMust runs kubectl and fails the test on error.
func kubectlMust(t *testing.T, kubeconfigPath string, args ...string) string {
	t.Helper()
	out, err := kubectl(t, kubeconfigPath, args...)
	if err != nil {
		t.Fatalf("kubectl %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

// waitFor polls fn until it returns true or the timeout expires.
func waitFor(t *testing.T, timeout, interval time.Duration, description string, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	t.Logf("Waiting for %s (timeout %s)...", description, timeout)
	for {
		if fn() {
			t.Logf("%s: OK", description)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s after %s", description, timeout)
		}
		time.Sleep(interval)
	}
}

// writeClusterConfig writes a Hetzner cluster config YAML to a temp file.
func writeClusterConfig(t *testing.T, name string) string {
	t.Helper()
	location := os.Getenv("E2E_LOCATION")
	if location == "" {
		location = "nbg1"
	}

	config := fmt.Sprintf(`name: %s
provider:
  type: hetzner
  location: %s
  vpc:
    cidr: 10.0.0.0/16
kubernetes:
  version: "1.30"
  distribution: rke2
nodes:
  controlPlane:
    count: 1
    instanceType: cpx22
  workers:
    count: 2
    instanceType: cpx32
components:
  traefik:
    enabled: true
  vault:
    enabled: true
    mode: deploy
  externalSecrets:
    enabled: true
`, name, location)

	dir := t.TempDir()
	path := filepath.Join(dir, "cluster.yaml")
	if err := os.WriteFile(path, []byte(config), 0644); err != nil {
		t.Fatalf("failed to write cluster config: %v", err)
	}
	return path
}

// createGitHubRepo creates a public GitHub repo and returns its SSH URL.
func createGitHubRepo(t *testing.T, name string) string {
	t.Helper()
	t.Logf("Creating GitHub repo: %s", name)
	cmd := exec.Command("gh", "repo", "create", name, "--public", "--confirm")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to create GitHub repo %s: %v\n%s", name, err, out)
	}
	return fmt.Sprintf("git@github.com:%s.git", name)
}

// deleteGitHubRepo deletes a GitHub repo. Does not fail the test on error.
func deleteGitHubRepo(t *testing.T, name string) {
	t.Helper()
	t.Logf("Deleting GitHub repo: %s", name)
	cmd := exec.Command("gh", "repo", "delete", name, "--yes")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("WARNING: failed to delete GitHub repo %s: %v\n%s", name, err, out)
	}
}

// gitInit initializes a git repo in the given directory.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "e2e@test.local")
	run(t, dir, "git", "config", "user.name", "E2E Test")
	run(t, dir, "git", "branch", "-M", "main")
}

// gitCommitAndPush commits all files and pushes to the remote.
func gitCommitAndPush(t *testing.T, dir, repoURL, message string) {
	t.Helper()
	// Set remote if not already set
	exec.Command("git", "-C", dir, "remote", "add", "origin", repoURL).Run()
	run(t, dir, "git", "add", "-A")
	run(t, dir, "git", "commit", "-m", message)
	run(t, dir, "git", "push", "-u", "origin", "main")
}

// run executes a command in the given directory and fails on error.
func run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}

// requireEnv checks that an environment variable is set and returns its value.
func requireEnv(t *testing.T, key string) string {
	t.Helper()
	val := os.Getenv(key)
	if val == "" {
		t.Skipf("skipping: %s not set", key)
	}
	return val
}
