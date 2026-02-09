package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/tdls-easy-k8s/internal/config"
	"github.com/user/tdls-easy-k8s/internal/provider"
)

var (
	validateClusterName string
	validateQuick       bool
)

// validateCmd represents the validate command
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate cluster health and readiness",
	Long: `Run comprehensive validation checks on a Kubernetes cluster:
- API server connectivity
- Node readiness (all nodes)
- System pod health (kube-system namespace)
- etcd cluster health
- DNS functionality
- Network connectivity
- Pod scheduling capability`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return validateCluster(cmd)
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)

	validateCmd.Flags().StringVarP(&validateClusterName, "cluster", "c", "", "Cluster name (required)")
	validateCmd.MarkFlagRequired("cluster")
	validateCmd.Flags().BoolVar(&validateQuick, "quick", false, "Run quick validation (skip optional checks)")
}

func validateCluster(cmd *cobra.Command) error {
	startTime := time.Now()

	fmt.Printf("Validating cluster: %s\n", validateClusterName)
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println()

	// Load cluster config
	cfg, err := loadClusterConfig(validateClusterName)
	if err != nil {
		return fmt.Errorf("failed to load cluster config: %w", err)
	}

	// Get provider
	p, err := getProvider(cfg.Provider.Type)
	if err != nil {
		return err
	}

	// Run validation checks
	checks := []validationCheck{
		{name: "API server accessibility", fn: checkAPIServer},
		{name: "Node readiness", fn: checkNodes},
		{name: "System pods", fn: checkSystemPods},
		{name: "etcd health", fn: checkEtcd},
		{name: "DNS resolution", fn: checkDNS},
		{name: "Pod networking", fn: checkNetworking},
	}

	if !validateQuick {
		checks = append(checks, validationCheck{
			name: "Pod scheduling",
			fn:   checkPodScheduling,
		})
	}

	passed := 0
	failed := 0
	warnings := 0

	for _, check := range checks {
		fmt.Printf("Checking %s...\n", check.name)
		result := check.fn(p, cfg)

		switch result.Status {
		case "pass":
			fmt.Printf("  ✓ %s\n", result.Message)
			passed++
		case "fail":
			fmt.Printf("  ❌ %s\n", result.Message)
			failed++
		case "warn":
			fmt.Printf("  ⚠ %s\n", result.Message)
			warnings++
		case "skip":
			fmt.Printf("  ⊘ %s\n", result.Message)
		}

		if result.Details != "" {
			fmt.Printf("     %s\n", result.Details)
		}
		fmt.Println()
	}

	// Summary
	elapsed := time.Since(startTime)
	fmt.Println("═══════════════════════════════════════════")
	fmt.Printf("Validation Summary (%s elapsed)\n", formatDuration(elapsed))
	fmt.Println("═══════════════════════════════════════════")
	fmt.Printf("Passed:   %d\n", passed)
	if warnings > 0 {
		fmt.Printf("Warnings: %d\n", warnings)
	}
	if failed > 0 {
		fmt.Printf("Failed:   %d\n", failed)
	}
	fmt.Println()

	if failed == 0 && warnings == 0 {
		fmt.Println("✓ Validation: PASSED")
		fmt.Println("Cluster is healthy and ready for workload deployment!")
		return nil
	} else if failed == 0 {
		fmt.Println("⚠ Validation: PASSED (with warnings)")
		fmt.Println("Cluster is functional but has some issues that should be addressed.")
		return nil
	} else {
		fmt.Println("❌ Validation: FAILED")
		fmt.Println("Cluster has critical issues that must be resolved.")
		return fmt.Errorf("validation failed with %d error(s)", failed)
	}
}

type validationCheck struct {
	name string
	fn   func(provider.Provider, *config.ClusterConfig) validationResult
}

type validationResult struct {
	Status  string // "pass", "fail", "warn", "skip"
	Message string
	Details string
}

func checkAPIServer(p provider.Provider, cfg *config.ClusterConfig) validationResult {
	result, err := p.ValidateAPIServer(cfg)
	if err != nil {
		return validationResult{
			Status:  "fail",
			Message: "API server is not accessible",
			Details: err.Error(),
		}
	}
	return validationResult{
		Status:  "pass",
		Message: result,
	}
}

func checkNodes(p provider.Provider, cfg *config.ClusterConfig) validationResult {
	result, err := p.ValidateNodes(cfg)
	if err != nil {
		return validationResult{
			Status:  "fail",
			Message: "Not all nodes are ready",
			Details: err.Error(),
		}
	}
	return validationResult{
		Status:  "pass",
		Message: result,
	}
}

func checkSystemPods(p provider.Provider, cfg *config.ClusterConfig) validationResult {
	result, err := p.ValidateSystemPods(cfg)
	if err != nil {
		return validationResult{
			Status:  "fail",
			Message: "Some system pods are not running",
			Details: err.Error(),
		}
	}
	return validationResult{
		Status:  "pass",
		Message: result,
	}
}

func checkEtcd(p provider.Provider, cfg *config.ClusterConfig) validationResult {
	result, err := p.ValidateEtcd(cfg)
	if err != nil {
		return validationResult{
			Status:  "warn",
			Message: "Could not verify etcd health",
			Details: err.Error(),
		}
	}
	return validationResult{
		Status:  "pass",
		Message: result,
	}
}

func checkDNS(p provider.Provider, cfg *config.ClusterConfig) validationResult {
	result, err := p.ValidateDNS(cfg)
	if err != nil {
		return validationResult{
			Status:  "fail",
			Message: "DNS resolution is not working",
			Details: err.Error(),
		}
	}
	return validationResult{
		Status:  "pass",
		Message: result,
	}
}

func checkNetworking(p provider.Provider, cfg *config.ClusterConfig) validationResult {
	result, err := p.ValidateNetworking(cfg)
	if err != nil {
		return validationResult{
			Status:  "warn",
			Message: "Could not verify pod networking",
			Details: err.Error(),
		}
	}
	return validationResult{
		Status:  "pass",
		Message: result,
	}
}

func checkPodScheduling(p provider.Provider, cfg *config.ClusterConfig) validationResult {
	result, err := p.ValidatePodScheduling(cfg)
	if err != nil {
		return validationResult{
			Status:  "warn",
			Message: "Could not verify pod scheduling",
			Details: err.Error(),
		}
	}
	return validationResult{
		Status:  "pass",
		Message: result,
	}
}
