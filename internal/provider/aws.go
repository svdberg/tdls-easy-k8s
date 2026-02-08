package provider

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/user/tdls-easy-k8s/internal/config"
)

// AWSProvider implements the Provider interface for AWS
type AWSProvider struct {
	workDir string
}

// NewAWSProvider creates a new AWS provider instance
func NewAWSProvider() *AWSProvider {
	return &AWSProvider{}
}

// Name returns the provider name
func (p *AWSProvider) Name() string {
	return "aws"
}

// ValidateConfig validates the AWS-specific configuration
func (p *AWSProvider) ValidateConfig(cfg *config.ClusterConfig) error {
	if cfg.Provider.Type != "aws" {
		return fmt.Errorf("provider type must be 'aws'")
	}

	if cfg.Provider.Region == "" {
		return fmt.Errorf("AWS region is required")
	}

	// TODO: Add more validation
	// - VPC CIDR validation
	// - Instance type validation
	// - AWS credentials check

	return nil
}

// CreateInfrastructure creates the AWS infrastructure for the cluster
func (p *AWSProvider) CreateInfrastructure(cfg *config.ClusterConfig) error {
	fmt.Println("[AWS] Creating infrastructure for cluster:", cfg.Name)

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

	// 5. Run tofu plan
	fmt.Println("\n[OpenTofu] Planning infrastructure changes...")
	if err := p.runTofu("plan", "-out=tfplan"); err != nil {
		return fmt.Errorf("terraform plan failed: %w", err)
	}

	// 6. Run tofu apply
	fmt.Println("\n[OpenTofu] Applying infrastructure changes...")
	fmt.Println("This may take 10-15 minutes...")
	if err := p.runTofu("apply", "tfplan"); err != nil {
		return fmt.Errorf("terraform apply failed: %w", err)
	}

	fmt.Println("\n✅ Infrastructure created successfully!")
	fmt.Println("\n📝 Next steps:")
	fmt.Println("  1. Download kubeconfig:")
	fmt.Printf("     aws s3 cp s3://%s/kubeconfig/%s/rke2.yaml ./kubeconfig\n", p.getStateBucket(cfg), cfg.Name)
	fmt.Println("  2. Test cluster:")
	fmt.Println("     export KUBECONFIG=./kubeconfig")
	fmt.Println("     kubectl get nodes")

	return nil
}

// DestroyInfrastructure destroys the AWS infrastructure
func (p *AWSProvider) DestroyInfrastructure(cfg *config.ClusterConfig) error {
	fmt.Println("[AWS] Destroying infrastructure for cluster:", cfg.Name)

	// Setup working directory
	if err := p.setupWorkingDirectory(cfg); err != nil {
		return fmt.Errorf("failed to setup working directory: %w", err)
	}

	// Run tofu destroy
	fmt.Println("\n[OpenTofu] Destroying infrastructure...")
	if err := p.runTofu("destroy", "-auto-approve"); err != nil {
		return fmt.Errorf("terraform destroy failed: %w", err)
	}

	fmt.Println("\n✅ Infrastructure destroyed successfully!")

	return nil
}

// GetKubeconfig retrieves the kubeconfig for the cluster
func (p *AWSProvider) GetKubeconfig(cfg *config.ClusterConfig) (string, error) {
	// TODO: Implement downloading kubeconfig from S3
	s3Path := fmt.Sprintf("s3://%s/kubeconfig/%s/rke2.yaml", p.getStateBucket(cfg), cfg.Name)
	return s3Path, nil
}

// GetStatus returns the current status of the AWS infrastructure
func (p *AWSProvider) GetStatus(cfg *config.ClusterConfig) (string, error) {
	if err := p.setupWorkingDirectory(cfg); err != nil {
		return "unknown", err
	}

	// Run tofu show to get status
	cmd := exec.Command("tofu", "show", "-json")
	cmd.Dir = p.workDir
	output, err := cmd.Output()
	if err != nil {
		return "unknown", fmt.Errorf("failed to get status: %w", err)
	}

	// Parse output (simplified)
	if len(output) > 0 {
		return "deployed", nil
	}

	return "unknown", nil
}

// setupWorkingDirectory creates and sets up the working directory for the cluster
func (p *AWSProvider) setupWorkingDirectory(cfg *config.ClusterConfig) error {
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

// copyTerraformModules copies the Terraform modules to the working directory
func (p *AWSProvider) copyTerraformModules() error {
	// Get the path to the terraform directory
	// This assumes the binary is run from the project root or installed via go install
	terraformDir := "providers/aws/terraform"

	// Try multiple possible locations
	possiblePaths := []string{
		terraformDir,
		filepath.Join("../../", terraformDir),
		filepath.Join(os.Getenv("GOPATH"), "src/github.com/user/tdls-easy-k8s", terraformDir),
	}

	var sourcePath string
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			sourcePath = path
			break
		}
	}

	if sourcePath == "" {
		return fmt.Errorf("could not find terraform modules directory")
	}

	// Copy the directory
	return filepath.WalkDir(sourcePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(p.workDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		// Copy file
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(targetPath, content, 0644)
	})
}

// generateTerraformVars generates terraform.tfvars.json from the cluster config
func (p *AWSProvider) generateTerraformVars(cfg *config.ClusterConfig) error {
	vars := map[string]interface{}{
		"cluster_name":                 cfg.Name,
		"environment":                  "production",
		"aws_region":                   cfg.Provider.Region,
		"vpc_cidr":                     cfg.Provider.VPC.CIDR,
		"control_plane_count":          cfg.Nodes.ControlPlane.Count,
		"control_plane_instance_type":  cfg.Nodes.ControlPlane.InstanceType,
		"worker_count":                 cfg.Nodes.Workers.Count,
		"worker_instance_type":         cfg.Nodes.Workers.InstanceType,
		"kubernetes_version":           cfg.Kubernetes.Version,
		"rke2_version":                 p.getRKE2Version(cfg.Kubernetes.Version),
		"kubernetes_distribution":      cfg.Kubernetes.Distribution,
		"state_bucket":                 p.getStateBucket(cfg),
		"enable_nlb":                   true,
		"enable_cloudwatch_logs":       true,
		"enable_session_manager":       true,
		"enable_encryption":            true,
	}

	jsonData, err := json.MarshalIndent(vars, "", "  ")
	if err != nil {
		return err
	}

	varFile := filepath.Join(p.workDir, "terraform.tfvars.json")
	return os.WriteFile(varFile, jsonData, 0644)
}

// runTofu executes a tofu command in the working directory
func (p *AWSProvider) runTofu(args ...string) error {
	cmd := exec.Command("tofu", args...)
	cmd.Dir = p.workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Set environment variables
	cmd.Env = append(os.Environ(),
		"TF_IN_AUTOMATION=1",
	)

	return cmd.Run()
}

// getRKE2Version maps Kubernetes version to RKE2 version
func (p *AWSProvider) getRKE2Version(k8sVersion string) string {
	// TODO: Implement proper version mapping or fetch from RKE2 releases
	// For now, return empty to use latest
	return ""
}

// getStateBucket returns the S3 bucket name for cluster state
func (p *AWSProvider) getStateBucket(cfg *config.ClusterConfig) string {
	// TODO: Allow user to specify bucket or create one
	return fmt.Sprintf("tdls-k8s-%s-state", cfg.Name)
}
