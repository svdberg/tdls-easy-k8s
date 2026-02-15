package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/tdls-easy-k8s/internal/config"
	"github.com/user/tdls-easy-k8s/internal/provider"
)

var (
	statusClusterName string
	statusWatch       bool
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cluster status and health",
	Long: `Display the current status of a Kubernetes cluster including:
- API server accessibility
- Node status (control plane and workers)
- System component health
- Basic cluster metrics`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return showStatus(cmd)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().StringVarP(&statusClusterName, "cluster", "c", "", "Cluster name (required)")
	statusCmd.MarkFlagRequired("cluster")
	statusCmd.Flags().BoolVarP(&statusWatch, "watch", "w", false, "Watch status continuously")
}

func showStatus(cmd *cobra.Command) error {
	// Load cluster config
	cfg, err := loadClusterConfig(statusClusterName)
	if err != nil {
		return fmt.Errorf("failed to load cluster config: %w", err)
	}

	// Get provider
	p, err := getProvider(cfg.Provider.Type)
	if err != nil {
		return err
	}

	// Show status
	if statusWatch {
		return watchStatus(p, cfg)
	}

	return displayStatus(p, cfg)
}

func displayStatus(p provider.Provider, cfg *config.ClusterConfig) error {
	fmt.Printf("Cluster: %s\n", cfg.Name)
	fmt.Printf("Provider: %s\n", cfg.Provider.Type)
	fmt.Printf("Region: %s\n", cfg.Provider.Region)
	fmt.Println()

	// Get cluster status from provider
	status, err := p.GetClusterStatus(cfg)
	if err != nil {
		fmt.Printf("❌ Failed to get cluster status: %v\n", err)
		return err
	}

	// Display API endpoint
	if status.APIEndpoint != "" {
		fmt.Printf("API Endpoint: %s\n", status.APIEndpoint)
	}
	fmt.Println()

	// Display node status
	fmt.Println("Nodes:")
	if status.ControlPlaneReady > 0 {
		symbol := "✓"
		if status.ControlPlaneReady < status.ControlPlaneTotal {
			symbol = "⚠"
		}
		fmt.Printf("  %s Control Plane: %d/%d ready\n", symbol, status.ControlPlaneReady, status.ControlPlaneTotal)
	}
	if status.WorkerReady > 0 {
		symbol := "✓"
		if status.WorkerReady < status.WorkerTotal {
			symbol = "⚠"
		}
		fmt.Printf("  %s Workers: %d/%d ready\n", symbol, status.WorkerReady, status.WorkerTotal)
	}
	fmt.Println()

	// Display system components
	if len(status.Components) > 0 {
		fmt.Println("System Components:")
		for _, comp := range status.Components {
			symbol := "✓"
			if comp.Status != "healthy" && comp.Status != "running" {
				symbol = "⚠"
			}
			fmt.Printf("  %s %-20s %s\n", symbol, comp.Name, comp.Message)
		}
		fmt.Println()
	}

	// Display overall status
	if status.Ready {
		fmt.Println("Status: ✓ Cluster is ready")
	} else {
		fmt.Println("Status: ⚠ Cluster is not fully ready")
		if status.Message != "" {
			fmt.Printf("  %s\n", status.Message)
		}
	}

	// Show age
	if !status.CreatedAt.IsZero() {
		age := time.Since(status.CreatedAt)
		fmt.Printf("Age: %s\n", formatDuration(age))
	}

	return nil
}

func watchStatus(p provider.Provider, cfg *config.ClusterConfig) error {
	fmt.Println("Watching cluster status (Press Ctrl+C to stop)...")
	fmt.Println()

	for {
		// Clear screen (simple version)
		fmt.Print("\033[H\033[2J")

		if err := displayStatus(p, cfg); err != nil {
			fmt.Printf("Error: %v\n", err)
		}

		time.Sleep(5 * time.Second)
	}
}

func loadClusterConfig(clusterName string) (*config.ClusterConfig, error) {
	// First try to load from config file if specified
	if cfgFile != "" {
		return config.LoadConfig(cfgFile)
	}

	// Try to load from cluster working directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".tdls-k8s", "clusters", clusterName, "cluster.yaml")
	return config.LoadConfig(configPath)
}

func getProvider(providerType string) (provider.Provider, error) {
	switch providerType {
	case "aws":
		return provider.NewAWSProvider(), nil
	case "vsphere":
		return nil, fmt.Errorf("vSphere provider not yet implemented")
	default:
		return nil, fmt.Errorf("unknown provider type: %s", providerType)
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%d hours", int(d.Hours()))
	}
	return fmt.Sprintf("%d days", int(d.Hours()/24))
}
