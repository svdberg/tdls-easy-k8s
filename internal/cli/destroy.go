package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var (
	destroyClusterName string
	destroyForce       bool
	destroyCleanup     bool
)

// destroyCmd represents the destroy command
var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy a Kubernetes cluster and its infrastructure",
	Long: `Destroy a Kubernetes cluster and all associated cloud infrastructure.

This command will:
  - Run OpenTofu destroy to remove all cloud resources
  - Optionally remove local state files and working directory

WARNING: This action is irreversible and will permanently delete all cluster resources.

Examples:
  # Destroy with confirmation prompt
  tdls-easy-k8s destroy --cluster=production

  # Destroy without confirmation
  tdls-easy-k8s destroy --cluster=dev --force

  # Destroy and cleanup all local files
  tdls-easy-k8s destroy --cluster=dev --force --cleanup`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return destroyCluster(cmd)
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)

	destroyCmd.Flags().StringVarP(&destroyClusterName, "cluster", "c", "", "Cluster name (required)")
	destroyCmd.MarkFlagRequired("cluster")
	destroyCmd.Flags().BoolVar(&destroyForce, "force", false, "Skip confirmation prompt")
	destroyCmd.Flags().BoolVar(&destroyCleanup, "cleanup", false, "Remove local state files and working directory")
}

func destroyCluster(cmd *cobra.Command) error {
	fmt.Printf("Preparing to destroy cluster: %s\n\n", destroyClusterName)

	// Load cluster config
	cfg, err := loadClusterConfig(destroyClusterName)
	if err != nil {
		return fmt.Errorf("failed to load cluster config: %w", err)
	}

	fmt.Printf("Provider: %s\n", cfg.Provider.Type)
	location := cfg.Provider.Region
	if cfg.Provider.Location != "" {
		location = cfg.Provider.Location
	}
	if location != "" {
		fmt.Printf("Location: %s\n", location)
	}
	fmt.Println()

	// Show provider-specific warning
	fmt.Println("⚠️  WARNING: This will permanently delete the following resources:")
	switch cfg.Provider.Type {
	case "hetzner":
		fmt.Println("  - All servers (control plane and workers)")
		fmt.Println("  - Private network and subnets")
		fmt.Println("  - Load balancer")
		fmt.Println("  - Firewall rules")
		fmt.Println("  - SSH keys")
	case "aws":
		fmt.Println("  - All EC2 instances (control plane and workers)")
		fmt.Println("  - VPC and all networking components (subnets, NAT gateways, IGW)")
		fmt.Println("  - Network Load Balancer")
		fmt.Println("  - EBS volumes (including etcd data)")
		fmt.Println("  - Security groups and IAM roles")
	default:
		fmt.Println("  - All compute instances (control plane and workers)")
		fmt.Println("  - All networking components")
		fmt.Println("  - Load balancers and firewall rules")
	}
	if destroyCleanup {
		if cfg.Provider.Type == "aws" {
			fmt.Println("  - S3 bucket for kubeconfig and state (with --cleanup)")
		}
		fmt.Println("  - Local terraform state and working directory (with --cleanup)")
	}
	fmt.Println()

	// Confirmation prompt (unless --force)
	if !destroyForce {
		fmt.Printf("Are you sure you want to destroy cluster '%s'? ", destroyClusterName)
		fmt.Print("Type the cluster name to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		input = strings.TrimSpace(input)
		if input != destroyClusterName {
			fmt.Println("\nDestroy cancelled - cluster name did not match")
			return nil
		}
		fmt.Println()
	}

	// Get provider
	p, err := getProvider(cfg.Provider.Type)
	if err != nil {
		return err
	}

	// Destroy infrastructure
	fmt.Println("Starting infrastructure destruction...")
	if err := p.DestroyInfrastructure(cfg); err != nil {
		return fmt.Errorf("failed to destroy infrastructure: %w", err)
	}

	// Cleanup local files (and S3 bucket for AWS) if requested
	if destroyCleanup {
		fmt.Println("\nCleaning up additional resources...")

		// Delete S3 bucket (AWS only)
		if cfg.Provider.Type == "aws" {
			bucketName := fmt.Sprintf("tdls-k8s-%s", cfg.Name)
			fmt.Printf("Deleting S3 bucket: %s\n", bucketName)

			// Empty bucket first (required before deletion)
			emptyCmd := fmt.Sprintf("aws s3 rm s3://%s --recursive --region %s 2>/dev/null", bucketName, cfg.Provider.Region)
			if err := runShellCommandQuiet(emptyCmd); err != nil {
				fmt.Printf("Note: bucket may already be empty or not exist\n")
			}

			// Delete bucket
			deleteCmd := fmt.Sprintf("aws s3 rb s3://%s --region %s 2>/dev/null", bucketName, cfg.Provider.Region)
			if err := runShellCommandQuiet(deleteCmd); err != nil {
				fmt.Printf("Note: S3 bucket may already be deleted\n")
			} else {
				fmt.Printf("✓ Deleted S3 bucket: %s\n", bucketName)
			}
		}

		// Remove local working directory
		workDir := fmt.Sprintf("%s/.tdls-k8s/clusters/%s", os.Getenv("HOME"), cfg.Name)
		if err := os.RemoveAll(workDir); err != nil {
			fmt.Printf("Warning: failed to remove working directory: %v\n", err)
		} else {
			fmt.Printf("✓ Removed local directory: %s\n", workDir)
		}
	}

	fmt.Println("\n✅ Cluster destroyed successfully!")
	fmt.Println("\nAll infrastructure has been removed.")
	if !destroyCleanup {
		fmt.Println("\nTo remove local state files, run:")
		fmt.Printf("  tdls-easy-k8s destroy --cluster=%s --cleanup\n", destroyClusterName)
		fmt.Println("\nOr manually remove:")
		fmt.Printf("  rm -rf ~/.tdls-k8s/clusters/%s\n", destroyClusterName)
	}

	return nil
}

func runShellCommandQuiet(cmd string) error {
	shellCmd := exec.Command("bash", "-c", cmd)
	return shellCmd.Run()
}
