package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
	"github.com/user/tdls-easy-k8s/internal/config"
	"github.com/user/tdls-easy-k8s/internal/provider"
)

var (
	providerType string
	region       string
	clusterName  string
	nodes        int
	generateCfg  bool
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new Kubernetes cluster",
	Long: `Initialize a new Kubernetes cluster on the specified cloud provider.
This command will create the necessary infrastructure and install Kubernetes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if generateCfg {
			return generateConfig(cmd)
		}

		return initCluster(cmd)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVar(&providerType, "provider", "aws", "Cloud provider (aws, vsphere, hetzner)")
	initCmd.Flags().StringVar(&region, "region", "us-east-1", "Cloud provider region")
	initCmd.Flags().StringVar(&clusterName, "name", "", "Cluster name")
	initCmd.Flags().IntVar(&nodes, "nodes", 3, "Number of worker nodes")
	initCmd.Flags().BoolVar(&generateCfg, "generate-config", false, "Generate a sample config file")
}

func generateConfig(cmd *cobra.Command) error {
	fmt.Println("# Example cluster configuration")
	fmt.Println("# Save this to cluster.yaml and customize as needed")
	fmt.Println("")
	fmt.Println("name: production")
	fmt.Println("provider:")
	fmt.Println("  type: aws")
	fmt.Println("  region: us-east-1")
	fmt.Println("  vpc:")
	fmt.Println("    cidr: 10.0.0.0/16")
	fmt.Println("")
	fmt.Println("kubernetes:")
	fmt.Println("  version: \"1.30\"")
	fmt.Println("  distribution: rke2")
	fmt.Println("")
	fmt.Println("nodes:")
	fmt.Println("  controlPlane:")
	fmt.Println("    count: 3")
	fmt.Println("    instanceType: t3.medium")
	fmt.Println("  workers:")
	fmt.Println("    count: 3")
	fmt.Println("    instanceType: t3.large")
	fmt.Println("")
	fmt.Println("gitops:")
	fmt.Println("  enabled: true")
	fmt.Println("  repository: github.com/user/cluster-gitops")
	fmt.Println("  branch: main")
	fmt.Println("")
	fmt.Println("components:")
	fmt.Println("  traefik:")
	fmt.Println("    enabled: true")
	fmt.Println("    version: \"26.x\"")
	fmt.Println("  vault:")
	fmt.Println("    enabled: true")
	fmt.Println("    mode: external  # or \"deploy\"")
	fmt.Println("    address: https://vault.example.com")
	fmt.Println("  externalSecrets:")
	fmt.Println("    enabled: true")

	return nil
}

func initCluster(cmd *cobra.Command) error {
	var cfg *config.ClusterConfig
	var err error

	// Load configuration from file or flags
	if cfgFile != "" {
		// Load from config file
		cfg, err = config.LoadConfig(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if verbose {
			fmt.Printf("✓ Loaded configuration from %s\n", cfgFile)
		}
	} else {
		// Use flags (require name when not using config file)
		if clusterName == "" {
			return fmt.Errorf("cluster name is required when not using a config file")
		}

		// Create config from flags (basic config)
		cfg = &config.ClusterConfig{
			Name: clusterName,
			Provider: config.ProviderConfig{
				Type:   providerType,
				Region: region,
				VPC: config.VPCConfig{
					CIDR: "10.0.0.0/16",
				},
			},
			Kubernetes: config.KubernetesConfig{
				Version:      "1.30",
				Distribution: "rke2",
			},
			Nodes: config.NodesConfig{
				ControlPlane: config.NodeGroupConfig{
					Count:        3,
					InstanceType: "t3.medium",
				},
				Workers: config.NodeGroupConfig{
					Count:        nodes,
					InstanceType: "t3.large",
				},
			},
		}
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	fmt.Printf("\n🚀 Initializing cluster '%s'\n", cfg.Name)
	fmt.Printf("   Provider: %s\n", cfg.Provider.Type)
	fmt.Printf("   Region: %s\n", cfg.Provider.Region)
	fmt.Printf("   Control Plane: %d nodes\n", cfg.Nodes.ControlPlane.Count)
	fmt.Printf("   Workers: %d nodes\n\n", cfg.Nodes.Workers.Count)

	// Get the appropriate provider
	var p provider.Provider
	switch cfg.Provider.Type {
	case "aws":
		p = provider.NewAWSProvider()
	case "vsphere":
		return fmt.Errorf("vSphere provider not yet implemented")
	case "hetzner":
		p = provider.NewHetznerProvider()
	default:
		return fmt.Errorf("unsupported provider: %s", cfg.Provider.Type)
	}

	// Validate provider configuration
	if err := p.ValidateConfig(cfg); err != nil {
		return fmt.Errorf("provider validation failed: %w", err)
	}

	// Create infrastructure
	if err := p.CreateInfrastructure(cfg); err != nil {
		return fmt.Errorf("infrastructure creation failed: %w", err)
	}

	// Persist cluster config for subsequent commands (kubeconfig, status, validate, etc.)
	if err := saveClusterConfig(cfg); err != nil {
		fmt.Printf("Warning: failed to save cluster config: %v\n", err)
		fmt.Println("You may need to pass --config to subsequent commands.")
	}

	return nil
}

func saveClusterConfig(cfg *config.ClusterConfig) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	clusterDir := filepath.Join(homeDir, ".tdls-k8s", "clusters", cfg.Name)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(clusterDir, "cluster.yaml"), data, 0644)
}
