package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	appChart           string
	appValues          string
	appNamespace       string
	appRepoURL         string
	appVersion         string
	appLayer           string
	appOutputDir       string
	appGitopsPath      string
	appDependsOn       string
	appCreateNamespace bool
)

// appCmd represents the app command group
var appCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage applications",
	Long:  `Commands for managing applications deployed on the Kubernetes cluster.`,
}

// appAddCmd represents the app add command
var appAddCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Add a new application to the cluster",
	Long: `Add a new application to the cluster via GitOps.
This generates Flux CD manifests (Kustomization CRD, HelmRepository, HelmRelease)
for deploying an application using the app-of-apps pattern.

If --output-dir is provided, files are written to the local gitops repo.
Otherwise, YAML is printed to stdout.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]
		return addApplication(cmd, appName)
	},
}

func init() {
	rootCmd.AddCommand(appCmd)
	appCmd.AddCommand(appAddCmd)

	appAddCmd.Flags().StringVar(&appChart, "chart", "", "Helm chart in reponame/chartname format (e.g., bitnami/nginx) (required)")
	appAddCmd.Flags().StringVar(&appValues, "values", "", "Path to Helm values YAML file")
	appAddCmd.Flags().StringVar(&appNamespace, "namespace", "default", "Target Kubernetes namespace")
	appAddCmd.Flags().StringVar(&appRepoURL, "repo-url", "", "Helm repository URL (e.g., https://charts.bitnami.com/bitnami) (required)")
	appAddCmd.Flags().StringVar(&appVersion, "version", "*", "Chart version constraint")
	appAddCmd.Flags().StringVar(&appLayer, "layer", "apps", "Target layer: apps or infrastructure")
	appAddCmd.Flags().StringVar(&appOutputDir, "output-dir", "", "Path to local gitops repo root (prints to stdout if omitted)")
	appAddCmd.Flags().StringVar(&appGitopsPath, "gitops-path", "clusters/production", "Path within repo for Kustomization CRDs")
	appAddCmd.Flags().StringVar(&appDependsOn, "depends-on", "", "Name of another app this one depends on")
	appAddCmd.Flags().BoolVar(&appCreateNamespace, "create-namespace", false, "Generate a namespace manifest")

	appAddCmd.MarkFlagRequired("chart")
	appAddCmd.MarkFlagRequired("repo-url")
}

func parseChartReference(chart string) (repoName, chartName string, err error) {
	parts := strings.Split(chart, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid chart reference %q: expected format reponame/chartname", chart)
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid chart reference %q: repo name and chart name must not be empty", chart)
	}
	return parts[0], parts[1], nil
}

func generateAppKustomizationYAML(appName, layer, dependsOn string) string {
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
  path: ./%s/%s
  prune: true
  wait: true
%s`, appName, layer, appName, dependsOnBlock)
}

func generateHelmRepositoryYAML(name, url string) string {
	return fmt.Sprintf(`apiVersion: source.toolkit.fluxcd.io/v1
kind: HelmRepository
metadata:
  name: %s
  namespace: flux-system
spec:
  interval: 1h0m0s
  url: %s
`, name, url)
}

func generateHelmReleaseYAML(name, namespace, chart, repoName, version, valuesYAML string) string {
	valuesBlock := ""
	if valuesYAML != "" {
		indented := indentYAML(valuesYAML, "    ")
		valuesBlock = fmt.Sprintf("  values:\n%s\n", indented)
	}

	return fmt.Sprintf(`apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: %s
  namespace: %s
spec:
  interval: 5m0s
  chart:
    spec:
      chart: %s
      version: "%s"
      sourceRef:
        kind: HelmRepository
        name: %s
        namespace: flux-system
%s`, name, namespace, chart, version, repoName, valuesBlock)
}

func generateNamespaceYAML(namespace string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s
`, namespace)
}

func indentYAML(yaml, prefix string) string {
	lines := strings.Split(yaml, "\n")
	var result []string
	for _, line := range lines {
		if line == "" {
			continue
		}
		result = append(result, prefix+line)
	}
	return strings.Join(result, "\n")
}

func addApplication(cmd *cobra.Command, appName string) error {
	repoName, chartName, err := parseChartReference(appChart)
	if err != nil {
		return err
	}

	if appLayer != "apps" && appLayer != "infrastructure" {
		return fmt.Errorf("invalid layer %q: must be 'apps' or 'infrastructure'", appLayer)
	}

	if verbose {
		fmt.Printf("Adding application '%s' from chart '%s/%s'\n", appName, repoName, chartName)
		fmt.Printf("Namespace: %s\n", appNamespace)
		fmt.Printf("Layer: %s\n", appLayer)
		if appValues != "" {
			fmt.Printf("Values file: %s\n", appValues)
		}
	}

	var valuesYAML string
	if appValues != "" {
		data, err := os.ReadFile(appValues)
		if err != nil {
			return fmt.Errorf("failed to read values file: %w", err)
		}
		valuesYAML = string(data)
	}

	kustomizationYAML := generateAppKustomizationYAML(appName, appLayer, appDependsOn)
	helmRepoYAML := generateHelmRepositoryYAML(repoName, appRepoURL)
	helmReleaseYAML := generateHelmReleaseYAML(appName, appNamespace, chartName, repoName, appVersion, valuesYAML)

	var namespaceYAML string
	if appCreateNamespace && appNamespace != "default" {
		namespaceYAML = generateNamespaceYAML(appNamespace)
	}

	if appOutputDir != "" {
		if err := writeAppFiles(appName, kustomizationYAML, helmRepoYAML, helmReleaseYAML, namespaceYAML); err != nil {
			return err
		}
	} else {
		printAppYAML(appName, kustomizationYAML, helmRepoYAML, helmReleaseYAML, namespaceYAML)
	}

	printAppNextSteps(appName)
	return nil
}

func writeAppFiles(appName, kustomizationYAML, helmRepoYAML, helmReleaseYAML, namespaceYAML string) error {
	kustomizationPath := filepath.Join(appOutputDir, appGitopsPath, appLayer, appName+".yaml")
	manifestDir := filepath.Join(appOutputDir, appLayer, appName)
	helmRepoPath := filepath.Join(manifestDir, "helmrepository.yaml")
	helmReleasePath := filepath.Join(manifestDir, "helmrelease.yaml")

	if err := os.MkdirAll(filepath.Dir(kustomizationPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(kustomizationPath, []byte(kustomizationYAML), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", kustomizationPath, err)
	}
	if err := os.WriteFile(helmRepoPath, []byte(helmRepoYAML), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", helmRepoPath, err)
	}
	if err := os.WriteFile(helmReleasePath, []byte(helmReleaseYAML), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", helmReleasePath, err)
	}

	fmt.Println("Files written:")
	fmt.Printf("  %s\n", kustomizationPath)
	fmt.Printf("  %s\n", helmRepoPath)
	fmt.Printf("  %s\n", helmReleasePath)

	if namespaceYAML != "" {
		namespacePath := filepath.Join(manifestDir, "namespace.yaml")
		if err := os.WriteFile(namespacePath, []byte(namespaceYAML), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", namespacePath, err)
		}
		fmt.Printf("  %s\n", namespacePath)
	}

	return nil
}

func printAppYAML(appName, kustomizationYAML, helmRepoYAML, helmReleaseYAML, namespaceYAML string) {
	fmt.Printf("# %s/%s/%s.yaml\n", appGitopsPath, appLayer, appName)
	fmt.Print(kustomizationYAML)
	fmt.Println("---")
	fmt.Printf("# %s/%s/helmrepository.yaml\n", appLayer, appName)
	fmt.Print(helmRepoYAML)
	fmt.Println("---")
	fmt.Printf("# %s/%s/helmrelease.yaml\n", appLayer, appName)
	fmt.Print(helmReleaseYAML)
	if namespaceYAML != "" {
		fmt.Println("---")
		fmt.Printf("# %s/%s/namespace.yaml\n", appLayer, appName)
		fmt.Print(namespaceYAML)
	}
}

func printAppNextSteps(appName string) {
	fmt.Println("\nNext steps:")
	if appOutputDir != "" {
		fmt.Println("  1. Commit and push the generated files:")
		fmt.Printf("     cd %s && git add -A && git commit -m \"Add %s\" && git push\n", appOutputDir, appName)
		fmt.Println()
	}
	fmt.Println("  Check application status:")
	fmt.Println("     kubectl get helmrelease -A")
	fmt.Printf("     kubectl get pods -n %s\n", appNamespace)
	fmt.Println()
	fmt.Printf("  To remove %s, delete the generated files and push.\n", appName)
}
