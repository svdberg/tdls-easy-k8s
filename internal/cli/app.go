package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	appChart     string
	appValues    string
	appNamespace string
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
This will create a HelmRelease manifest in your GitOps repository.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]
		return addApplication(cmd, appName)
	},
}

func init() {
	rootCmd.AddCommand(appCmd)
	appCmd.AddCommand(appAddCmd)

	appAddCmd.Flags().StringVar(&appChart, "chart", "", "Helm chart (e.g., mycompany/myapp) (required)")
	appAddCmd.Flags().StringVar(&appValues, "values", "", "Path to values file")
	appAddCmd.Flags().StringVar(&appNamespace, "namespace", "default", "Kubernetes namespace")

	appAddCmd.MarkFlagRequired("chart")
}

func addApplication(cmd *cobra.Command, appName string) error {
	if verbose {
		fmt.Printf("Adding application '%s' from chart '%s'\n", appName, appChart)
		fmt.Printf("Namespace: %s\n", appNamespace)
		if appValues != "" {
			fmt.Printf("Values file: %s\n", appValues)
		}
	}

	fmt.Println("[STUB] Application add not yet implemented")
	fmt.Println("Configuration:")
	fmt.Printf("  Application: %s\n", appName)
	fmt.Printf("  Chart: %s\n", appChart)
	fmt.Printf("  Namespace: %s\n", appNamespace)
	if appValues != "" {
		fmt.Printf("  Values: %s\n", appValues)
	}

	fmt.Println("\nPlanned steps:")
	fmt.Println("  1. Create HelmRepository source if needed")
	fmt.Println("  2. Generate HelmRelease manifest")
	fmt.Println("  3. Commit to GitOps repository")
	fmt.Println("  4. Wait for Flux to sync")

	return nil
}
