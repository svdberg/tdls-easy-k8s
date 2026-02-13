package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

const fluxInstallURL = "https://github.com/fluxcd/flux2/releases/latest/download/install.yaml"

var (
	gitopsRepo   string
	gitopsBranch string
	gitopsPath   string
)

// gitopsCmd represents the gitops command group
var gitopsCmd = &cobra.Command{
	Use:   "gitops",
	Short: "Manage GitOps configuration",
	Long:  `Commands for managing GitOps setup, including Flux installation and repository configuration.`,
}

// gitopsSetupCmd represents the gitops setup command
var gitopsSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup GitOps on the cluster",
	Long: `Setup GitOps (Flux) on the cluster and configure it to sync with your Git repository.
This will install Flux controllers and configure them to watch your repository for changes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return setupGitOps(cmd)
	},
}

func init() {
	rootCmd.AddCommand(gitopsCmd)
	gitopsCmd.AddCommand(gitopsSetupCmd)

	gitopsSetupCmd.Flags().StringVar(&gitopsRepo, "repo", "", "Git repository URL (required)")
	gitopsSetupCmd.Flags().StringVar(&gitopsBranch, "branch", "main", "Git branch to track")
	gitopsSetupCmd.Flags().StringVar(&gitopsPath, "path", "clusters/production", "Path in repository")

	gitopsSetupCmd.MarkFlagRequired("repo")
}

func setupGitOps(cmd *cobra.Command) error {
	fmt.Println("\nSetting up GitOps with Flux CD")
	fmt.Printf("  Repository: %s\n", gitopsRepo)
	fmt.Printf("  Branch:     %s\n", gitopsBranch)
	fmt.Printf("  Path:       %s\n\n", gitopsPath)

	if err := checkGitOpsPrerequisites(); err != nil {
		return fmt.Errorf("prerequisite check failed: %w", err)
	}

	if err := installFluxControllers(); err != nil {
		return fmt.Errorf("failed to install Flux: %w", err)
	}

	if err := waitForFluxReady(); err != nil {
		return fmt.Errorf("Flux controllers not ready: %w", err)
	}

	if err := createGitRepositorySource(gitopsRepo, gitopsBranch); err != nil {
		return fmt.Errorf("failed to create GitRepository: %w", err)
	}

	if err := createFluxKustomizations(gitopsPath); err != nil {
		return fmt.Errorf("failed to create Kustomizations: %w", err)
	}

	if err := verifyGitOpsSetup(); err != nil {
		fmt.Printf("\nWarning: verification incomplete: %v\n", err)
		fmt.Println("  Flux resources were created but may need time to reconcile.")
	} else {
		fmt.Println("\nFlux is reconciling your repository!")
	}

	printGitOpsNextSteps()
	return nil
}

func checkGitOpsPrerequisites() error {
	fmt.Println("[1/6] Checking prerequisites...")

	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl not found in PATH\nInstall kubectl: https://kubernetes.io/docs/tasks/tools/")
	}
	fmt.Println("  kubectl is available")

	cmd := exec.Command("kubectl", "cluster-info")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cannot connect to cluster: %s\nEnsure kubeconfig is configured (tdls-easy-k8s kubeconfig --cluster=<name>)", strings.TrimSpace(string(output)))
	}
	fmt.Println("  Cluster is reachable")

	return nil
}

func installFluxControllers() error {
	fmt.Println("[2/6] Installing Flux controllers...")

	checkCmd := exec.Command("kubectl", "get", "namespace", "flux-system")
	if err := checkCmd.Run(); err == nil {
		fmt.Println("  Flux namespace already exists, updating installation...")
	}

	cmd := exec.Command("kubectl", "apply", "--server-side", "-f", fluxInstallURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl apply failed: %w", err)
	}

	fmt.Println("  Flux controllers installed")
	return nil
}

func waitForFluxReady() error {
	fmt.Println("[3/6] Waiting for Flux controllers to be ready...")

	deployments := []string{
		"source-controller",
		"kustomize-controller",
		"helm-controller",
		"notification-controller",
	}

	for _, deploy := range deployments {
		fmt.Printf("  Waiting for %s...\n", deploy)
		cmd := exec.Command("kubectl", "wait", "--for=condition=available",
			"--timeout=120s",
			fmt.Sprintf("deployment/%s", deploy),
			"-n", "flux-system")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s not ready: %s", deploy, strings.TrimSpace(string(output)))
		}
	}

	fmt.Println("  All Flux controllers are ready")
	return nil
}

func createGitRepositorySource(repo, branch string) error {
	fmt.Println("[4/6] Creating GitRepository source...")

	yaml := generateGitRepositoryYAML(repo, branch)

	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(yaml)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to apply GitRepository: %s", strings.TrimSpace(string(output)))
	}

	fmt.Println("  GitRepository 'flux-system' created")
	return nil
}

func generateGitRepositoryYAML(repo, branch string) string {
	return fmt.Sprintf(`apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: flux-system
  namespace: flux-system
spec:
  interval: 1m0s
  ref:
    branch: %s
  url: %s
`, branch, repo)
}

func createFluxKustomizations(path string) error {
	fmt.Println("[5/6] Creating Kustomizations...")

	path = strings.TrimPrefix(path, "/")
	infraYAML := generateKustomizationYAML("infrastructure", path+"/infrastructure", "")
	appsYAML := generateKustomizationYAML("apps", path+"/apps", "infrastructure")

	combined := infraYAML + "---\n" + appsYAML

	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(combined)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to apply Kustomizations: %s", strings.TrimSpace(string(output)))
	}

	fmt.Println("  Kustomization 'infrastructure' created")
	fmt.Println("  Kustomization 'apps' created (depends on infrastructure)")
	return nil
}

func generateKustomizationYAML(name, path, dependsOn string) string {
	dependsOnBlock := ""
	if dependsOn != "" {
		dependsOnBlock = fmt.Sprintf("  dependsOn:\n    - name: %s\n", dependsOn)
	}

	return fmt.Sprintf(`apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: %s
  namespace: flux-system
spec:
  interval: 10m0s
  sourceRef:
    kind: GitRepository
    name: flux-system
  path: ./%s
  prune: true
  wait: true
%s`, name, path, dependsOnBlock)
}

func verifyGitOpsSetup() error {
	fmt.Println("[6/6] Verifying GitOps setup...")

	resources := []struct {
		kind string
		name string
	}{
		{"gitrepository", "flux-system"},
		{"kustomization", "infrastructure"},
		{"kustomization", "apps"},
	}

	for _, r := range resources {
		cmd := exec.Command("kubectl", "get", r.kind, r.name, "-n", "flux-system")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s '%s' not found: %w", r.kind, r.name, err)
		}
		fmt.Printf("  %s '%s' exists\n", r.kind, r.name)
	}

	return nil
}

func printGitOpsNextSteps() {
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Push Kubernetes manifests to your repository:")
	fmt.Printf("     %s (branch: %s)\n", gitopsRepo, gitopsBranch)
	fmt.Println()
	fmt.Printf("  2. Place infrastructure manifests in: %s/infrastructure/\n", gitopsPath)
	fmt.Printf("  3. Place application manifests in:    %s/apps/\n", gitopsPath)
	fmt.Println()
	fmt.Println("  4. Check Flux status:")
	fmt.Println("     kubectl get gitrepositories -n flux-system")
	fmt.Println("     kubectl get kustomizations -n flux-system")
	fmt.Println()
	fmt.Println("  For private repositories, create a deploy key secret:")
	fmt.Println("     kubectl create secret generic flux-system \\")
	fmt.Println("       --from-file=identity=./deploy-key \\")
	fmt.Println("       --from-file=identity.pub=./deploy-key.pub \\")
	fmt.Println("       --from-file=known_hosts=./known_hosts \\")
	fmt.Println("       -n flux-system")
	fmt.Println("     Then patch the GitRepository to reference it.")
}
