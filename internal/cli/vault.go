package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	vaultClusterName string
	vaultOutputDir   string
	vaultGitopsPath  string
)

// vaultCmd represents the vault command group
var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Manage Vault integration",
	Long:  `Commands for managing HashiCorp Vault integration with the Kubernetes cluster.`,
}

// vaultSetupCmd represents the vault setup command
var vaultSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Generate Vault manifests for GitOps deployment",
	Long: `Generate Flux CD manifests for Vault integration based on the cluster config.

In 'external' mode: generates a ClusterSecretStore pointing at your existing Vault instance.
In 'deploy' mode: generates HelmRepository, HelmRelease, and ClusterSecretStore
to deploy Vault into the cluster and connect ESO to it.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return setupVault(cmd)
	},
}

func init() {
	rootCmd.AddCommand(vaultCmd)
	vaultCmd.AddCommand(vaultSetupCmd)

	vaultSetupCmd.Flags().StringVarP(&vaultClusterName, "cluster", "c", "", "Cluster name (required)")
	vaultSetupCmd.MarkFlagRequired("cluster")
	vaultSetupCmd.Flags().StringVar(&vaultOutputDir, "output-dir", "", "Path to local gitops repo root (prints to stdout if omitted)")
	vaultSetupCmd.Flags().StringVar(&vaultGitopsPath, "gitops-path", "clusters/production", "Path within repo for Kustomization CRDs")
}

func setupVault(cmd *cobra.Command) error {
	cfg, err := loadClusterConfig(vaultClusterName)
	if err != nil {
		return fmt.Errorf("failed to load cluster config: %w", err)
	}

	if !cfg.Components.Vault.Enabled {
		return fmt.Errorf("vault is not enabled in the cluster config")
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	switch cfg.Components.Vault.Mode {
	case "external":
		return setupVaultExternal(cfg.Components.Vault.Address)
	case "deploy":
		return setupVaultDeploy()
	default:
		return fmt.Errorf("unsupported vault mode: %s", cfg.Components.Vault.Mode)
	}
}

func setupVaultExternal(address string) error {
	clusterSecretStoreYAML := generateVaultClusterSecretStoreYAML(address)

	if vaultOutputDir != "" {
		return writeVaultExternalFiles(clusterSecretStoreYAML)
	}

	printVaultExternalYAML(clusterSecretStoreYAML)
	return nil
}

func setupVaultDeploy() error {
	helmRepoYAML := generateHelmRepositoryYAML("hashicorp", "https://helm.releases.hashicorp.com")
	helmReleaseYAML := generateHelmReleaseYAML("vault", "vault-system", "vault", "hashicorp", "*", vaultDeployValues())
	kustomizationYAML := generateAppKustomizationYAML("vault", "infrastructure", "")
	clusterSecretStoreYAML := generateVaultClusterSecretStoreYAML("http://vault-system-vault.vault-system.svc:8200")

	if vaultOutputDir != "" {
		return writeVaultDeployFiles(helmRepoYAML, helmReleaseYAML, kustomizationYAML, clusterSecretStoreYAML)
	}

	printVaultDeployYAML(helmRepoYAML, helmReleaseYAML, kustomizationYAML, clusterSecretStoreYAML)
	return nil
}

func generateVaultClusterSecretStoreYAML(server string) string {
	return fmt.Sprintf(`apiVersion: external-secrets.io/v1
kind: ClusterSecretStore
metadata:
  name: vault
spec:
  provider:
    vault:
      server: %s
      path: secret
      version: v2
      auth:
        kubernetes:
          mountPath: kubernetes
          role: external-secrets
`, server)
}

func vaultDeployValues() string {
	return `server:
  dev:
    enabled: true
  standalone:
    enabled: false
ui:
  enabled: true
injector:
  enabled: false`
}

func writeVaultExternalFiles(clusterSecretStoreYAML string) error {
	manifestDir := filepath.Join(vaultOutputDir, "infrastructure", "external-secrets")
	cssPath := filepath.Join(manifestDir, "vault-clustersecretstore.yaml")

	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(cssPath, []byte(clusterSecretStoreYAML), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", cssPath, err)
	}

	fmt.Println("Files written:")
	fmt.Printf("  %s\n", cssPath)

	printVaultExternalNextSteps()
	return nil
}

func writeVaultDeployFiles(helmRepoYAML, helmReleaseYAML, kustomizationYAML, clusterSecretStoreYAML string) error {
	vaultDir := filepath.Join(vaultOutputDir, "infrastructure", "vault")
	kustomizationPath := filepath.Join(vaultOutputDir, vaultGitopsPath, "infrastructure", "vault.yaml")
	cssDir := filepath.Join(vaultOutputDir, "infrastructure", "external-secrets")
	cssPath := filepath.Join(cssDir, "vault-clustersecretstore.yaml")

	if err := os.MkdirAll(vaultDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(kustomizationPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	if err := os.MkdirAll(cssDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	files := []struct {
		path    string
		content string
	}{
		{filepath.Join(vaultDir, "helmrepository.yaml"), helmRepoYAML},
		{filepath.Join(vaultDir, "helmrelease.yaml"), helmReleaseYAML},
		{kustomizationPath, kustomizationYAML},
		{cssPath, clusterSecretStoreYAML},
	}

	fmt.Println("Files written:")
	for _, f := range files {
		if err := os.WriteFile(f.path, []byte(f.content), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", f.path, err)
		}
		fmt.Printf("  %s\n", f.path)
	}

	printVaultDeployNextSteps()
	return nil
}

func printVaultExternalYAML(clusterSecretStoreYAML string) {
	fmt.Println("# infrastructure/external-secrets/vault-clustersecretstore.yaml")
	fmt.Print(clusterSecretStoreYAML)

	printVaultExternalNextSteps()
}

func printVaultDeployYAML(helmRepoYAML, helmReleaseYAML, kustomizationYAML, clusterSecretStoreYAML string) {
	fmt.Println("# infrastructure/vault/helmrepository.yaml")
	fmt.Print(helmRepoYAML)
	fmt.Println("---")
	fmt.Println("# infrastructure/vault/helmrelease.yaml")
	fmt.Print(helmReleaseYAML)
	fmt.Println("---")
	fmt.Printf("# %s/infrastructure/vault.yaml\n", vaultGitopsPath)
	fmt.Print(kustomizationYAML)
	fmt.Println("---")
	fmt.Println("# infrastructure/external-secrets/vault-clustersecretstore.yaml")
	fmt.Print(clusterSecretStoreYAML)

	printVaultDeployNextSteps()
}

func printVaultExternalNextSteps() {
	fmt.Println("\nNext steps (external Vault):")
	fmt.Println("  1. Ensure ESO is deployed (use 'tdls-easy-k8s app add external-secrets ...')")
	fmt.Println("  2. Configure Vault Kubernetes auth:")
	fmt.Println("     vault auth enable kubernetes")
	fmt.Println("     vault write auth/kubernetes/config \\")
	fmt.Println("       kubernetes_host=\"https://<k8s-api>:6443\"")
	fmt.Println("     vault write auth/kubernetes/role/external-secrets \\")
	fmt.Println("       bound_service_account_names=external-secrets \\")
	fmt.Println("       bound_service_account_namespaces=external-secrets \\")
	fmt.Println("       policies=default")
	if vaultOutputDir != "" {
		fmt.Println("  3. Commit and push the generated files")
	} else {
		fmt.Println("  3. Apply the ClusterSecretStore manifest above")
	}
	fmt.Println("  4. Create ExternalSecret resources to sync secrets from Vault")
	fmt.Println()
}

func printVaultDeployNextSteps() {
	fmt.Println("\nNext steps (deploy Vault):")
	if vaultOutputDir != "" {
		fmt.Println("  1. Commit and push the generated files")
		fmt.Println("  2. Flux will deploy Vault into the vault-system namespace")
	} else {
		fmt.Println("  1. Write the manifests above to your gitops repo")
		fmt.Println("  2. Push — Flux will deploy Vault into the vault-system namespace")
	}
	fmt.Println("  3. Initialize and unseal Vault:")
	fmt.Println("     kubectl exec -n vault-system vault-0 -- vault operator init")
	fmt.Println("     kubectl exec -n vault-system vault-0 -- vault operator unseal <key>")
	fmt.Println("  4. Enable Kubernetes auth in Vault:")
	fmt.Println("     kubectl exec -n vault-system vault-0 -- vault auth enable kubernetes")
	fmt.Println("  5. Create ExternalSecret resources to sync secrets from Vault")
	fmt.Println()
}
