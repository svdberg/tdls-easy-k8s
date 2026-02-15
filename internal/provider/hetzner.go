package provider

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/user/tdls-easy-k8s/internal/config"
)

// hetznerLocations is the set of valid Hetzner Cloud locations.
var hetznerLocations = map[string]bool{
	"fsn1": true, // Falkenstein, Germany
	"nbg1": true, // Nuremberg, Germany
	"hel1": true, // Helsinki, Finland
	"ash":  true, // Ashburn, USA
	"hil":  true, // Hillsboro, USA
}

// HetznerProvider implements the Provider interface for Hetzner Cloud
type HetznerProvider struct {
	workDir string
}

// NewHetznerProvider creates a new Hetzner provider instance
func NewHetznerProvider() *HetznerProvider {
	return &HetznerProvider{}
}

// Name returns the provider name
func (p *HetznerProvider) Name() string {
	return "hetzner"
}

// ValidateConfig validates the Hetzner-specific configuration
func (p *HetznerProvider) ValidateConfig(cfg *config.ClusterConfig) error {
	if cfg.Provider.Type != "hetzner" {
		return fmt.Errorf("provider type must be 'hetzner'")
	}

	// Determine location from Location or Region field
	location := p.getLocation(cfg)
	if location == "" {
		return fmt.Errorf("Hetzner location is required (set provider.location or provider.region)")
	}

	if !hetznerLocations[location] {
		return fmt.Errorf("invalid Hetzner location %q (valid: fsn1, nbg1, hel1, ash, hil)", location)
	}

	if cfg.Nodes.ControlPlane.InstanceType == "" {
		return fmt.Errorf("control plane server type is required (e.g., cx22)")
	}

	if cfg.Nodes.Workers.Count > 0 && cfg.Nodes.Workers.InstanceType == "" {
		return fmt.Errorf("worker server type is required (e.g., cx32)")
	}

	// Check HCLOUD_TOKEN is set
	if os.Getenv("HCLOUD_TOKEN") == "" {
		return fmt.Errorf("HCLOUD_TOKEN environment variable is required\nGet a token from: https://console.hetzner.cloud → API tokens")
	}

	return nil
}

// getLocation returns the Hetzner location from config, preferring Location over Region.
func (p *HetznerProvider) getLocation(cfg *config.ClusterConfig) string {
	if cfg.Provider.Location != "" {
		return cfg.Provider.Location
	}
	return cfg.Provider.Region
}

// CreateInfrastructure creates the Hetzner infrastructure for the cluster
func (p *HetznerProvider) CreateInfrastructure(cfg *config.ClusterConfig) error {
	fmt.Println("[Hetzner] Creating infrastructure for cluster:", cfg.Name)

	// 1. Setup working directory
	if err := p.setupWorkingDirectory(cfg); err != nil {
		return fmt.Errorf("failed to setup working directory: %w", err)
	}

	// 2. Copy Terraform modules
	if err := p.copyTerraformModules(); err != nil {
		return fmt.Errorf("failed to copy terraform modules: %w", err)
	}

	// 3. Generate terraform.tfvars.json
	if err := p.generateTerraformVars(cfg); err != nil {
		return fmt.Errorf("failed to generate terraform vars: %w", err)
	}

	// 4. Run tofu init
	fmt.Println("\n[OpenTofu] Initializing...")
	if err := p.runTofu("init"); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}

	// Fix provider permissions
	if err := p.fixProviderPermissions(); err != nil {
		fmt.Printf("Warning: failed to fix provider permissions: %v\n", err)
	}

	// 5. Run tofu plan
	fmt.Println("\n[OpenTofu] Planning infrastructure changes...")
	if err := p.runTofu("plan", "-out=tfplan"); err != nil {
		return fmt.Errorf("terraform plan failed: %w", err)
	}

	// 6. Run tofu apply
	fmt.Println("\n[OpenTofu] Applying infrastructure changes...")
	fmt.Println("This may take 5-10 minutes...")
	if err := p.runTofu("apply", "tfplan"); err != nil {
		return fmt.Errorf("terraform apply failed: %w", err)
	}

	fmt.Println("\n✅ Infrastructure created successfully!")

	fmt.Println("\n📝 Next steps:")
	fmt.Println("  1. Wait for RKE2 to complete installation (~5 minutes)")
	fmt.Println("  2. Download and configure kubeconfig:")
	fmt.Printf("     tdls-easy-k8s kubeconfig --cluster=%s\n", cfg.Name)
	fmt.Println()
	fmt.Println("  3. Verify cluster:")
	fmt.Printf("     tdls-easy-k8s status --cluster=%s\n", cfg.Name)
	fmt.Println()
	fmt.Println("  4. (Optional) Validate cluster health:")
	fmt.Printf("     tdls-easy-k8s validate --cluster=%s\n", cfg.Name)

	return nil
}

// DestroyInfrastructure destroys the Hetzner infrastructure
func (p *HetznerProvider) DestroyInfrastructure(cfg *config.ClusterConfig) error {
	fmt.Println("[Hetzner] Destroying infrastructure for cluster:", cfg.Name)

	// Setup working directory
	if err := p.setupWorkingDirectory(cfg); err != nil {
		return fmt.Errorf("failed to setup working directory: %w", err)
	}

	// Check if terraform state exists
	stateFile := filepath.Join(p.workDir, "terraform.tfstate")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		fmt.Println("\n⚠️  No terraform state file found - infrastructure may already be destroyed")
		return nil
	}

	// Run tofu destroy
	fmt.Println("\n[OpenTofu] Destroying infrastructure...")
	fmt.Println("This may take 2-5 minutes...")
	if err := p.runTofu("destroy", "-auto-approve"); err != nil {
		return fmt.Errorf("terraform destroy failed: %w", err)
	}

	fmt.Println("\n✅ Infrastructure destroyed successfully!")
	fmt.Println("All Hetzner resources (servers, network, load balancer, etc.) have been removed")

	return nil
}

// GetKubeconfig retrieves the kubeconfig for the cluster
func (p *HetznerProvider) GetKubeconfig(cfg *config.ClusterConfig) (string, error) {
	if err := p.setupWorkingDirectory(cfg); err != nil {
		return "", fmt.Errorf("failed to setup working directory: %w", err)
	}

	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to download kubeconfig: %w", err)
	}

	return kubeconfigPath, nil
}

// GetStatus returns the current status of the Hetzner infrastructure
func (p *HetznerProvider) GetStatus(cfg *config.ClusterConfig) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "unknown", err
	}

	p.workDir = filepath.Join(homeDir, ".tdls-k8s", "clusters", cfg.Name, "terraform")

	stateFile := filepath.Join(p.workDir, "terraform.tfstate")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		return "unknown", nil
	}

	return "deployed", nil
}

// GetClusterStatus returns detailed cluster status
func (p *HetznerProvider) GetClusterStatus(cfg *config.ClusterConfig) (*ClusterStatus, error) {
	if err := p.setupWorkingDirectory(cfg); err != nil {
		return nil, err
	}

	// Get API endpoint from Terraform
	apiEndpoint, _ := p.getTerraformOutput("lb_ipv4")

	// Download kubeconfig
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return &ClusterStatus{
			Ready:   false,
			Message: "Unable to download kubeconfig",
		}, nil
	}
	defer os.Remove(kubeconfigPath)

	return kubectlGetClusterStatus(kubeconfigPath, apiEndpoint)
}

// --- Validation methods (delegate to common kubectl logic) ---

func (p *HetznerProvider) ValidateAPIServer(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", fmt.Errorf("cannot download kubeconfig: %w", err)
	}
	defer os.Remove(kubeconfigPath)
	return kubectlValidateAPIServer(kubeconfigPath)
}

func (p *HetznerProvider) ValidateNodes(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfigPath)
	return kubectlValidateNodes(kubeconfigPath)
}

func (p *HetznerProvider) ValidateSystemPods(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfigPath)
	return kubectlValidateSystemPods(kubeconfigPath)
}

func (p *HetznerProvider) ValidateEtcd(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfigPath)
	return kubectlValidateEtcd(kubeconfigPath)
}

func (p *HetznerProvider) ValidateDNS(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfigPath)
	return kubectlValidateDNS(kubeconfigPath)
}

func (p *HetznerProvider) ValidateNetworking(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfigPath)
	return kubectlValidateNetworking(kubeconfigPath)
}

func (p *HetznerProvider) ValidatePodScheduling(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfigPath)
	return kubectlValidatePodScheduling(kubeconfigPath)
}

// --- Internal helpers ---

func (p *HetznerProvider) setupWorkingDirectory(cfg *config.ClusterConfig) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	p.workDir = filepath.Join(homeDir, ".tdls-k8s", "clusters", cfg.Name, "terraform")

	if err := os.MkdirAll(p.workDir, 0755); err != nil {
		return err
	}

	return nil
}

func (p *HetznerProvider) copyTerraformModules() error {
	sourcePath, err := p.findTerraformSource()
	if err != nil {
		return err
	}

	// Clean stale source files before copying
	if err := p.cleanTerraformSourceFiles(); err != nil {
		return fmt.Errorf("failed to clean stale module files: %w", err)
	}

	return filepath.WalkDir(sourcePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return err
		}

		if d.IsDir() && (d.Name() == ".terraform" || d.Name() == ".git") {
			return filepath.SkipDir
		}
		if d.Name() == ".terraform.lock.hcl" || d.Name() == "terraform.tfstate" || d.Name() == "terraform.tfstate.backup" {
			return nil
		}

		targetPath := filepath.Join(p.workDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(targetPath, content, 0644)
	})
}

func (p *HetznerProvider) findTerraformSource() (string, error) {
	terraformDir := "providers/hetzner/terraform"

	// Try the binary's directory first
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		candidate := filepath.Join(execDir, terraformDir)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	possiblePaths := []string{
		terraformDir,
		filepath.Join("../../", terraformDir),
		filepath.Join(os.Getenv("GOPATH"), "src/github.com/user/tdls-easy-k8s", terraformDir),
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("could not find Hetzner terraform modules directory")
}

func (p *HetznerProvider) cleanTerraformSourceFiles() error {
	entries, err := os.ReadDir(p.workDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := filepath.Ext(name)
		if ext == ".tf" || ext == ".tpl" || name == ".gitkeep" {
			if err := os.Remove(filepath.Join(p.workDir, name)); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *HetznerProvider) generateTerraformVars(cfg *config.ClusterConfig) error {
	location := p.getLocation(cfg)

	networkCIDR := cfg.Provider.VPC.CIDR
	if networkCIDR == "" {
		networkCIDR = "10.0.0.0/16"
	}

	vars := map[string]interface{}{
		"cluster_name":       cfg.Name,
		"location":           location,
		"server_type_cp":     cfg.Nodes.ControlPlane.InstanceType,
		"server_type_worker": cfg.Nodes.Workers.InstanceType,
		"cp_count":           cfg.Nodes.ControlPlane.Count,
		"worker_count":       cfg.Nodes.Workers.Count,
		"network_cidr":       networkCIDR,
		"kubernetes_version": cfg.Kubernetes.Version,
	}

	jsonData, err := json.MarshalIndent(vars, "", "  ")
	if err != nil {
		return err
	}

	varFile := filepath.Join(p.workDir, "terraform.tfvars.json")
	return os.WriteFile(varFile, jsonData, 0644)
}

func (p *HetznerProvider) runTofu(args ...string) error {
	cmd := exec.Command("tofu", args...)
	cmd.Dir = p.workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	cmd.Env = append(os.Environ(), "TF_IN_AUTOMATION=1")

	return cmd.Run()
}

func (p *HetznerProvider) getTerraformOutput(outputName string) (string, error) {
	cmd := exec.Command("tofu", "output", "-raw", outputName)
	cmd.Dir = p.workDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get output %s: %w", outputName, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (p *HetznerProvider) fixProviderPermissions() error {
	providersDir := filepath.Join(p.workDir, ".terraform", "providers")

	return filepath.WalkDir(providersDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		basename := filepath.Base(path)
		if !d.IsDir() && strings.HasPrefix(basename, "terraform-provider-") {
			if err := os.Chmod(path, 0755); err != nil {
				return nil
			}
		}

		return nil
	})
}

// downloadKubeconfig retrieves kubeconfig via SSH from the first control plane node.
func (p *HetznerProvider) downloadKubeconfig(cfg *config.ClusterConfig) (string, error) {
	if p.workDir == "" {
		if err := p.setupWorkingDirectory(cfg); err != nil {
			return "", fmt.Errorf("failed to setup working directory: %w", err)
		}
	}

	// Get the first control plane IP
	firstCPIP, err := p.getTerraformOutput("first_cp_ip")
	if err != nil || firstCPIP == "" {
		return "", fmt.Errorf("failed to get control plane IP: %w", err)
	}

	// Get the SSH private key from terraform output
	sshKeyCmd := exec.Command("tofu", "output", "-raw", "ssh_private_key")
	sshKeyCmd.Dir = p.workDir
	sshKeyOutput, err := sshKeyCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get SSH private key: %w", err)
	}

	// Write SSH key to temp file
	sshKeyFile, err := os.CreateTemp("", "hetzner-ssh-key-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(sshKeyFile.Name())

	if _, err := sshKeyFile.Write(sshKeyOutput); err != nil {
		sshKeyFile.Close()
		return "", err
	}
	sshKeyFile.Close()
	os.Chmod(sshKeyFile.Name(), 0600)

	// SSH into the first control plane node and download kubeconfig
	sshCmd := exec.Command("ssh",
		"-i", sshKeyFile.Name(),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("root@%s", firstCPIP),
		"cat /etc/rancher/rke2/rke2.yaml",
	)

	kubeconfigData, err := sshCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve kubeconfig via SSH: %w", err)
	}

	// Get LB IP to patch server URL
	lbIP, _ := p.getTerraformOutput("lb_ipv4")

	// Patch server URL: replace 127.0.0.1 with LB IP
	kubeconfig := string(kubeconfigData)
	if lbIP != "" {
		lines := strings.Split(kubeconfig, "\n")
		for i, line := range lines {
			if strings.Contains(line, "server: https://") {
				lines[i] = fmt.Sprintf("    server: https://%s:6443", lbIP)
				break
			}
		}
		kubeconfig = strings.Join(lines, "\n")
	}

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "kubeconfig-*.yaml")
	if err != nil {
		return "", err
	}

	if _, err := tmpFile.WriteString(kubeconfig); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", err
	}
	tmpFile.Close()
	os.Chmod(tmpFile.Name(), 0600)

	return tmpFile.Name(), nil
}
