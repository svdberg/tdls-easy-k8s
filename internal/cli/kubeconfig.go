package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	kubeconfigClusterName string
	kubeconfigOutput      string
	kubeconfigMerge       bool
	kubeconfigSetContext  bool
)

// kubeconfigCmd represents the kubeconfig command
var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig",
	Short: "Download and configure kubeconfig for cluster access",
	Long: `Download the kubeconfig file from S3 and configure it for cluster access.

By default, the kubeconfig is saved to ./kubeconfig with the correct API endpoint.
You can merge it into your kubectl config or set it as the current context.

Examples:
  # Download to ./kubeconfig
  tdls-easy-k8s kubeconfig --cluster=production

  # Download to specific file
  tdls-easy-k8s kubeconfig --cluster=production --output=~/.kube/production-config

  # Merge into ~/.kube/config and set as current context
  tdls-easy-k8s kubeconfig --cluster=production --merge --set-context`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return getKubeconfig(cmd)
	},
}

func init() {
	rootCmd.AddCommand(kubeconfigCmd)

	kubeconfigCmd.Flags().StringVarP(&kubeconfigClusterName, "cluster", "c", "", "Cluster name (required)")
	kubeconfigCmd.MarkFlagRequired("cluster")
	kubeconfigCmd.Flags().StringVarP(&kubeconfigOutput, "output", "o", "./kubeconfig", "Output file path")
	kubeconfigCmd.Flags().BoolVar(&kubeconfigMerge, "merge", false, "Merge into ~/.kube/config")
	kubeconfigCmd.Flags().BoolVar(&kubeconfigSetContext, "set-context", false, "Set as current kubectl context (requires --merge)")
}

func getKubeconfig(cmd *cobra.Command) error {
	fmt.Printf("Downloading kubeconfig for cluster: %s\n", kubeconfigClusterName)

	// Load cluster config
	cfg, err := loadClusterConfig(kubeconfigClusterName)
	if err != nil {
		return fmt.Errorf("failed to load cluster config: %w", err)
	}

	// Get provider
	p, err := getProvider(cfg.Provider.Type)
	if err != nil {
		return err
	}

	// Get kubeconfig from provider
	kubeconfigPath, err := p.GetKubeconfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	// Handle merge vs save to file
	if kubeconfigMerge {
		return mergeKubeconfig(kubeconfigPath, cfg.Name, kubeconfigSetContext)
	}

	return saveKubeconfig(kubeconfigPath, kubeconfigOutput, cfg.Name)
}

func saveKubeconfig(sourcePath, outputPath, clusterName string) error {
	// Expand home directory
	if outputPath[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		outputPath = filepath.Join(home, outputPath[2:])
	}

	// Read source
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read kubeconfig: %w", err)
	}

	// Write to output
	if err := os.WriteFile(outputPath, content, 0600); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	fmt.Println()
	fmt.Printf("✅ Kubeconfig saved to: %s\n", outputPath)
	fmt.Println()
	fmt.Println("To use this cluster, run:")
	fmt.Printf("  export KUBECONFIG=%s\n", outputPath)
	fmt.Println("  kubectl get nodes")
	fmt.Println()
	fmt.Println("Or merge into your kubectl config:")
	fmt.Printf("  tdls-easy-k8s kubeconfig --cluster=%s --merge --set-context\n", clusterName)

	return nil
}

func mergeKubeconfig(sourcePath, clusterName string, setContext bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	kubeDir := filepath.Join(home, ".kube")
	kubeConfigPath := filepath.Join(kubeDir, "config")

	// Ensure .kube directory exists
	if err := os.MkdirAll(kubeDir, 0755); err != nil {
		return fmt.Errorf("failed to create .kube directory: %w", err)
	}

	// Backup existing config if it exists
	if _, err := os.Stat(kubeConfigPath); err == nil {
		backupPath := kubeConfigPath + ".backup"
		fmt.Printf("Backing up existing config to: %s\n", backupPath)
		if err := copyFile(kubeConfigPath, backupPath); err != nil {
			return fmt.Errorf("failed to backup config: %w", err)
		}
	}

	// Use kubectl to merge the configs
	contextName := fmt.Sprintf("tdls-%s", clusterName)

	// Set KUBECONFIG to include both files
	mergeCmd := fmt.Sprintf("KUBECONFIG=%s:%s kubectl config view --flatten > %s.tmp && mv %s.tmp %s",
		kubeConfigPath, sourcePath, kubeConfigPath, kubeConfigPath, kubeConfigPath)

	fmt.Println("Merging kubeconfig...")
	if err := runShellCommand(mergeCmd); err != nil {
		return fmt.Errorf("failed to merge kubeconfig: %w", err)
	}

	// Rename context to something meaningful
	renameCmd := fmt.Sprintf("kubectl config rename-context $(kubectl config current-context --kubeconfig=%s) %s",
		sourcePath, contextName)
	if err := runShellCommand(renameCmd); err != nil {
		// Context might already have the right name, not critical
		fmt.Printf("Note: Could not rename context: %v\n", err)
	}

	fmt.Println()
	fmt.Printf("✅ Kubeconfig merged into: %s\n", kubeConfigPath)
	fmt.Printf("Context name: %s\n", contextName)
	fmt.Println()

	// Set context if requested
	if setContext {
		fmt.Printf("Setting current context to: %s\n", contextName)
		setContextCmd := fmt.Sprintf("kubectl config use-context %s", contextName)
		if err := runShellCommand(setContextCmd); err != nil {
			return fmt.Errorf("failed to set context: %w", err)
		}
		fmt.Println()
		fmt.Println("✅ Context set! You can now use kubectl:")
		fmt.Println("  kubectl get nodes")
	} else {
		fmt.Println("To use this cluster, run:")
		fmt.Printf("  kubectl config use-context %s\n", contextName)
		fmt.Println("  kubectl get nodes")
	}

	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}

func runShellCommand(cmd string) error {
	// Use bash to execute the command
	shellCmd := exec.Command("bash", "-c", cmd)
	shellCmd.Stdout = os.Stdout
	shellCmd.Stderr = os.Stderr
	return shellCmd.Run()
}
