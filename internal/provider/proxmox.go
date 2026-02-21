package provider

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/user/tdls-easy-k8s/internal/config"
)

// ProxmoxProvider implements the Provider interface for Proxmox VE
type ProxmoxProvider struct {
	workDir string
}

// NewProxmoxProvider creates a new Proxmox provider instance
func NewProxmoxProvider() *ProxmoxProvider {
	return &ProxmoxProvider{}
}

// Name returns the provider name
func (p *ProxmoxProvider) Name() string {
	return "proxmox"
}

// ValidateConfig validates the Proxmox-specific configuration
func (p *ProxmoxProvider) ValidateConfig(cfg *config.ClusterConfig) error {
	if cfg.Provider.Type != "proxmox" {
		return fmt.Errorf("provider type must be 'proxmox'")
	}

	if cfg.Provider.Node == "" {
		return fmt.Errorf("Proxmox node name is required (set provider.node, e.g. 'pve')")
	}

	// VIP is required (no cloud LB available)
	if cfg.Provider.VIP == "" {
		return fmt.Errorf("kube-vip VIP address is required (set provider.vip)\nThis must be a free IP on your network for the Kubernetes API endpoint")
	}

	if net.ParseIP(cfg.Provider.VIP) == nil {
		return fmt.Errorf("invalid VIP address %q: must be a valid IPv4 address", cfg.Provider.VIP)
	}

	if cfg.Nodes.ControlPlane.Count < 1 {
		return fmt.Errorf("at least one control plane node is required")
	}

	// Check Proxmox API credentials
	if os.Getenv("PROXMOX_VE_ENDPOINT") == "" {
		return fmt.Errorf("PROXMOX_VE_ENDPOINT environment variable is required (e.g. https://proxmox.local:8006)")
	}

	if os.Getenv("PROXMOX_VE_API_TOKEN") == "" && os.Getenv("PROXMOX_VE_USERNAME") == "" {
		return fmt.Errorf("PROXMOX_VE_API_TOKEN or PROXMOX_VE_USERNAME environment variable is required")
	}

	return nil
}

// CreateInfrastructure creates the Proxmox infrastructure for the cluster
func (p *ProxmoxProvider) CreateInfrastructure(cfg *config.ClusterConfig) error {
	fmt.Println("[Proxmox] Creating infrastructure for cluster:", cfg.Name)

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
	fmt.Println("This may take 5-10 minutes (includes image download on first run)...")
	if err := p.runTofu("apply", "tfplan"); err != nil {
		return fmt.Errorf("terraform apply failed: %w", err)
	}

	fmt.Println("\nInfrastructure created successfully!")

	fmt.Println("\nNext steps:")
	fmt.Println("  1. Wait for RKE2 to complete installation (~5 minutes)")
	fmt.Println("  2. Download and configure kubeconfig:")
	fmt.Printf("     tdls-easy-k8s kubeconfig --cluster=%s\n", cfg.Name)
	fmt.Println()
	fmt.Println("  3. Verify cluster:")
	fmt.Printf("     tdls-easy-k8s validate --cluster=%s\n", cfg.Name)

	return nil
}

// DestroyInfrastructure destroys the Proxmox infrastructure
func (p *ProxmoxProvider) DestroyInfrastructure(cfg *config.ClusterConfig) error {
	fmt.Println("[Proxmox] Destroying infrastructure for cluster:", cfg.Name)

	// Setup working directory
	if err := p.setupWorkingDirectory(cfg); err != nil {
		return fmt.Errorf("failed to setup working directory: %w", err)
	}

	// Check if terraform state exists
	stateFile := filepath.Join(p.workDir, "terraform.tfstate")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		fmt.Println("\nNo terraform state file found - infrastructure may already be destroyed")
		return nil
	}

	// Run tofu destroy
	fmt.Println("\n[OpenTofu] Destroying infrastructure...")
	fmt.Println("This may take 2-5 minutes...")
	if err := p.runTofu("destroy", "-auto-approve"); err != nil {
		return fmt.Errorf("terraform destroy failed: %w", err)
	}

	fmt.Println("\nInfrastructure destroyed successfully!")
	fmt.Println("All Proxmox VMs and resources have been removed")

	return nil
}

// GetKubeconfig retrieves the kubeconfig for the cluster
func (p *ProxmoxProvider) GetKubeconfig(cfg *config.ClusterConfig) (string, error) {
	if err := p.setupWorkingDirectory(cfg); err != nil {
		return "", fmt.Errorf("failed to setup working directory: %w", err)
	}

	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to download kubeconfig: %w", err)
	}

	return kubeconfigPath, nil
}

// GetStatus returns the current status of the Proxmox infrastructure
func (p *ProxmoxProvider) GetStatus(cfg *config.ClusterConfig) (string, error) {
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
func (p *ProxmoxProvider) GetClusterStatus(cfg *config.ClusterConfig) (*ClusterStatus, error) {
	if err := p.setupWorkingDirectory(cfg); err != nil {
		return nil, err
	}

	// Get API endpoint (VIP)
	apiEndpoint, _ := p.getTerraformOutput("vip_address")

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

func (p *ProxmoxProvider) ValidateAPIServer(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", fmt.Errorf("cannot download kubeconfig: %w", err)
	}
	defer os.Remove(kubeconfigPath)
	return kubectlValidateAPIServer(kubeconfigPath)
}

func (p *ProxmoxProvider) ValidateNodes(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfigPath)
	return kubectlValidateNodes(kubeconfigPath)
}

func (p *ProxmoxProvider) ValidateSystemPods(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfigPath)
	return kubectlValidateSystemPods(kubeconfigPath)
}

func (p *ProxmoxProvider) ValidateEtcd(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfigPath)
	return kubectlValidateEtcd(kubeconfigPath)
}

func (p *ProxmoxProvider) ValidateDNS(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfigPath)
	return kubectlValidateDNS(kubeconfigPath)
}

func (p *ProxmoxProvider) ValidateNetworking(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfigPath)
	return kubectlValidateNetworking(kubeconfigPath)
}

func (p *ProxmoxProvider) ValidatePodScheduling(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfigPath)
	return kubectlValidatePodScheduling(kubeconfigPath)
}

// --- Internal helpers ---

func (p *ProxmoxProvider) setupWorkingDirectory(cfg *config.ClusterConfig) error {
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

func (p *ProxmoxProvider) copyTerraformModules() error {
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

func (p *ProxmoxProvider) findTerraformSource() (string, error) {
	terraformDir := "providers/proxmox/terraform"

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

	return "", fmt.Errorf("could not find Proxmox terraform modules directory")
}

func (p *ProxmoxProvider) cleanTerraformSourceFiles() error {
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

func (p *ProxmoxProvider) generateTerraformVars(cfg *config.ClusterConfig) error {
	bridge := cfg.Provider.Bridge
	if bridge == "" {
		bridge = "vmbr0"
	}

	datastore := cfg.Provider.Datastore
	if datastore == "" {
		datastore = "local-lvm"
	}

	vars := map[string]interface{}{
		"cluster_name":       cfg.Name,
		"proxmox_node":       cfg.Provider.Node,
		"bridge":             bridge,
		"datastore":          datastore,
		"vip_address":        cfg.Provider.VIP,
		"cp_count":           cfg.Nodes.ControlPlane.Count,
		"worker_count":       cfg.Nodes.Workers.Count,
		"kubernetes_version": cfg.Kubernetes.Version,
	}

	if cfg.Provider.VlanTag > 0 {
		vars["vlan_tag"] = cfg.Provider.VlanTag
	}

	jsonData, err := json.MarshalIndent(vars, "", "  ")
	if err != nil {
		return err
	}

	varFile := filepath.Join(p.workDir, "terraform.tfvars.json")
	return os.WriteFile(varFile, jsonData, 0644)
}

func (p *ProxmoxProvider) runTofu(args ...string) error {
	cmd := exec.Command("tofu", args...)
	cmd.Dir = p.workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	cmd.Env = append(os.Environ(), "TF_IN_AUTOMATION=1")

	return cmd.Run()
}

func (p *ProxmoxProvider) getTerraformOutput(outputName string) (string, error) {
	cmd := exec.Command("tofu", "output", "-raw", outputName)
	cmd.Dir = p.workDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get output %s: %w", outputName, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (p *ProxmoxProvider) fixProviderPermissions() error {
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
func (p *ProxmoxProvider) downloadKubeconfig(cfg *config.ClusterConfig) (string, error) {
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
	sshKeyFile, err := os.CreateTemp("", "proxmox-ssh-key-*")
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

	// Get VIP to patch server URL
	vipIP, _ := p.getTerraformOutput("vip_address")

	// Patch server URL: replace 127.0.0.1 with VIP
	kubeconfig := string(kubeconfigData)
	if vipIP != "" {
		lines := strings.Split(kubeconfig, "\n")
		for i, line := range lines {
			if strings.Contains(line, "server: https://") {
				lines[i] = fmt.Sprintf("    server: https://%s:6443", vipIP)
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
