package cli

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	monitorClusterName string
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Launch k9s terminal UI for cluster monitoring",
	Long: `Launch k9s, a terminal-based UI for interacting with your Kubernetes cluster.

k9s will be automatically installed if not found. The kubeconfig for the
specified cluster will be retrieved and passed to k9s.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMonitor(cmd)
	},
}

func init() {
	rootCmd.AddCommand(monitorCmd)

	monitorCmd.Flags().StringVarP(&monitorClusterName, "cluster", "c", "", "Cluster name (required)")
	monitorCmd.MarkFlagRequired("cluster")
}

func runMonitor(cmd *cobra.Command) error {
	fmt.Printf("Preparing to monitor cluster: %s\n", monitorClusterName)

	k9sPath, err := ensureK9sInstalled()
	if err != nil {
		return fmt.Errorf("failed to ensure k9s is available: %w", err)
	}

	cfg, err := loadClusterConfig(monitorClusterName)
	if err != nil {
		return fmt.Errorf("failed to load cluster config: %w", err)
	}

	p, err := getProvider(cfg.Provider.Type)
	if err != nil {
		return err
	}

	kubeconfigPath, err := p.GetKubeconfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	fmt.Printf("Launching k9s for cluster '%s'...\n", monitorClusterName)
	return launchK9s(k9sPath, kubeconfigPath)
}

func ensureK9sInstalled() (string, error) {
	if path, err := exec.LookPath("k9s"); err == nil {
		if verbose {
			fmt.Printf("Found k9s in PATH: %s\n", path)
		}
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	localK9sPath := filepath.Join(home, ".tdls-k8s", "bin", "k9s")
	if _, err := os.Stat(localK9sPath); err == nil {
		if verbose {
			fmt.Printf("Found k9s at: %s\n", localK9sPath)
		}
		return localK9sPath, nil
	}

	fmt.Println("k9s not found. Installing...")
	if err := installK9s(localK9sPath); err != nil {
		return "", err
	}

	return localK9sPath, nil
}

func installK9s(targetPath string) error {
	osName := titleCase(runtime.GOOS)
	archName := runtime.GOARCH

	downloadURL := fmt.Sprintf(
		"https://github.com/derailed/k9s/releases/latest/download/k9s_%s_%s.tar.gz",
		osName, archName,
	)

	if verbose {
		fmt.Printf("Downloading k9s from: %s\n", downloadURL)
	}

	binDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "k9s-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download k9s: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download k9s: HTTP %d", resp.StatusCode)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return fmt.Errorf("failed to save k9s download: %w", err)
	}
	tmpFile.Close()

	if err := extractK9sFromTarGz(tmpFile.Name(), targetPath); err != nil {
		return fmt.Errorf("failed to extract k9s: %w", err)
	}

	if err := os.Chmod(targetPath, 0o755); err != nil {
		return fmt.Errorf("failed to make k9s executable: %w", err)
	}

	fmt.Printf("k9s installed to: %s\n", targetPath)
	return nil
}

func extractK9sFromTarGz(tarGzPath, targetPath string) error {
	f, err := os.Open(tarGzPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		if filepath.Base(header.Name) == "k9s" && header.Typeflag == tar.TypeReg {
			outFile, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write k9s binary: %w", err)
			}
			outFile.Close()
			return nil
		}
	}

	return fmt.Errorf("k9s binary not found in archive")
}

func launchK9s(k9sPath, kubeconfigPath string) error {
	cmd := exec.Command(k9sPath, "--kubeconfig", kubeconfigPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("k9s exited with error: %w", err)
	}

	return nil
}

func titleCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
