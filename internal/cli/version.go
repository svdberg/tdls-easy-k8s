package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	// These will be set by build flags
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of tdls-easy-k8s",
	Long:  `Print the version number, commit hash, build date, and Go version.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("tdls-easy-k8s %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built: %s\n", buildDate)
		fmt.Printf("  go version: %s\n", runtime.Version())
		fmt.Printf("  platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
