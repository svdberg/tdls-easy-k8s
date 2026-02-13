package provider

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/user/tdls-easy-k8s/internal/config"
)

// awsRegions is the set of valid AWS commercial regions.
var awsRegions = map[string]bool{
	"us-east-1":      true,
	"us-east-2":      true,
	"us-west-1":      true,
	"us-west-2":      true,
	"af-south-1":     true,
	"ap-east-1":      true,
	"ap-south-1":     true,
	"ap-south-2":     true,
	"ap-southeast-1": true,
	"ap-southeast-2": true,
	"ap-southeast-3": true,
	"ap-southeast-4": true,
	"ap-southeast-5": true,
	"ap-northeast-1": true,
	"ap-northeast-2": true,
	"ap-northeast-3": true,
	"ca-central-1":   true,
	"ca-west-1":      true,
	"eu-central-1":   true,
	"eu-central-2":   true,
	"eu-west-1":      true,
	"eu-west-2":      true,
	"eu-west-3":      true,
	"eu-south-1":     true,
	"eu-south-2":     true,
	"eu-north-1":     true,
	"il-central-1":   true,
	"me-south-1":     true,
	"me-central-1":   true,
	"sa-east-1":      true,
}

// instanceTypePattern matches AWS EC2 instance type names (e.g., t3.medium, m5.xlarge, c6i.2xlarge).
var instanceTypePattern = regexp.MustCompile(`^[a-z][a-z0-9]*\.[a-z0-9]+$`)

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

	if !awsRegions[cfg.Provider.Region] {
		return fmt.Errorf("invalid AWS region %q", cfg.Provider.Region)
	}

	if err := validateVPCCIDR(cfg.Provider.VPC.CIDR); err != nil {
		return err
	}

	if err := validateInstanceType("control plane", cfg.Nodes.ControlPlane.InstanceType); err != nil {
		return err
	}

	if err := validateInstanceType("worker", cfg.Nodes.Workers.InstanceType); err != nil {
		return err
	}

	// Check AWS CLI is available and credentials are configured
	if err := checkAWSCredentials(); err != nil {
		return err
	}

	return nil
}

// validateVPCCIDR validates that the VPC CIDR is valid, private, and appropriately sized.
func validateVPCCIDR(cidr string) error {
	if cidr == "" {
		return fmt.Errorf("VPC CIDR is required")
	}

	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid VPC CIDR %q: %w", cidr, err)
	}

	ones, _ := ipNet.Mask.Size()
	if ones < 16 || ones > 24 {
		return fmt.Errorf("VPC CIDR prefix length must be between /16 and /24, got /%d", ones)
	}

	// Must be RFC 1918 private range
	privateRanges := []net.IPNet{
		{IP: net.IP{10, 0, 0, 0}, Mask: net.CIDRMask(8, 32)},
		{IP: net.IP{172, 16, 0, 0}, Mask: net.CIDRMask(12, 32)},
		{IP: net.IP{192, 168, 0, 0}, Mask: net.CIDRMask(16, 32)},
	}
	isPrivate := false
	for _, r := range privateRanges {
		if r.Contains(ip) {
			isPrivate = true
			break
		}
	}
	if !isPrivate {
		return fmt.Errorf("VPC CIDR %q must be in a private range (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)", cidr)
	}

	return nil
}

// validateInstanceType validates that an EC2 instance type name has the correct format.
func validateInstanceType(role, instanceType string) error {
	if instanceType == "" {
		return fmt.Errorf("%s instance type is required", role)
	}
	if !instanceTypePattern.MatchString(instanceType) {
		return fmt.Errorf("invalid %s instance type %q: must match AWS format (e.g. t3.medium, m5.xlarge)", role, instanceType)
	}
	return nil
}

// checkAWSCredentials verifies that the AWS CLI is installed and credentials are configured.
func checkAWSCredentials() error {
	cmd := exec.Command("aws", "sts", "get-caller-identity")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("AWS credentials check failed: %s\nEnsure AWS CLI is installed and credentials are configured (aws configure)", strings.TrimSpace(string(output)))
	}
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

	// 3.5. Create S3 bucket for kubeconfig storage
	if err := p.createS3Bucket(cfg); err != nil {
		return fmt.Errorf("failed to create S3 bucket: %w", err)
	}

	// 4. Run tofu init
	fmt.Println("\n[OpenTofu] Initializing...")
	if err := p.runTofu("init"); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}

	// 4.5. Fix provider executable permissions
	if err := p.fixProviderPermissions(); err != nil {
		fmt.Printf("Warning: failed to fix provider permissions: %v\n", err)
	}

	// 5. Run tofu plan
	fmt.Println("\n[OpenTofu] Planning infrastructure changes...")
	if err := p.runTofu("plan", "-out=tfplan"); err != nil {
		return fmt.Errorf("terraform plan failed: %w", err)
	}

	// 6. Run tofu apply (Phase 1)
	fmt.Println("\n[OpenTofu] Applying infrastructure changes (Phase 1)...")
	fmt.Println("This may take 10-15 minutes...")
	if err := p.runTofu("apply", "tfplan"); err != nil {
		return fmt.Errorf("terraform apply failed: %w", err)
	}

	fmt.Println("\n✅ Infrastructure created successfully!")

	// 7. Phase 2: Update TLS certificates with NLB DNS (if NLB is enabled)
	if err := p.updateTLSCertificatesWithNLB(cfg); err != nil {
		fmt.Printf("\n⚠️  Warning: Failed to update TLS certificates with NLB DNS: %v\n", err)
		fmt.Println("You can manually update certificates later if needed.")
	}

	// 8. Phase 3: Restart worker agents so they reconnect with updated TLS certs
	if err := p.restartWorkerAgents(cfg); err != nil {
		fmt.Printf("\n⚠️  Warning: Failed to restart worker agents: %v\n", err)
		fmt.Println("You can manually restart workers: aws ssm send-command --document-name AWS-RunShellScript --parameters '{\"commands\":[\"sudo systemctl restart rke2-agent\"]}' --instance-ids <id>")
	}

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
	fmt.Println()
	fmt.Println("Or merge into your kubectl config:")
	fmt.Printf("  tdls-easy-k8s kubeconfig --cluster=%s --merge --set-context\n", cfg.Name)

	return nil
}

// DestroyInfrastructure destroys the AWS infrastructure
func (p *AWSProvider) DestroyInfrastructure(cfg *config.ClusterConfig) error {
	fmt.Println("[AWS] Destroying infrastructure for cluster:", cfg.Name)

	// Setup working directory
	if err := p.setupWorkingDirectory(cfg); err != nil {
		return fmt.Errorf("failed to setup working directory: %w", err)
	}

	// Check if terraform state exists
	stateFile := filepath.Join(p.workDir, "terraform.tfstate")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		fmt.Println("\n⚠️  No terraform state file found - infrastructure may already be destroyed")
		fmt.Printf("State file checked: %s\n", stateFile)
		return nil
	}

	// Run tofu destroy
	fmt.Println("\n[OpenTofu] Destroying infrastructure...")
	fmt.Println("This may take 5-10 minutes...")
	if err := p.runTofu("destroy", "-auto-approve"); err != nil {
		return fmt.Errorf("terraform destroy failed: %w", err)
	}

	fmt.Println("\n✅ Infrastructure destroyed successfully!")
	fmt.Println("All AWS resources (VPC, EC2, NLB, EBS, etc.) have been removed")

	return nil
}

// GetKubeconfig retrieves the kubeconfig for the cluster
func (p *AWSProvider) GetKubeconfig(cfg *config.ClusterConfig) (string, error) {
	// Setup working directory to get Terraform outputs
	if err := p.setupWorkingDirectory(cfg); err != nil {
		return "", fmt.Errorf("failed to setup working directory: %w", err)
	}

	// Download and prepare kubeconfig
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to download kubeconfig: %w", err)
	}

	// Note: The downloadKubeconfig function already updates the server URL to use NLB
	return kubeconfigPath, nil
}

// GetStatus returns the current status of the AWS infrastructure
func (p *AWSProvider) GetStatus(cfg *config.ClusterConfig) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "unknown", err
	}

	p.workDir = filepath.Join(homeDir, ".tdls-k8s", "clusters", cfg.Name, "terraform")

	// If terraform state doesn't exist, the cluster was never provisioned
	stateFile := filepath.Join(p.workDir, "terraform.tfstate")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		return "unknown", nil
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

		// Skip .terraform directory and other temporary files
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
		"cluster_name":                cfg.Name,
		"environment":                 "production",
		"aws_region":                  cfg.Provider.Region,
		"vpc_cidr":                    cfg.Provider.VPC.CIDR,
		"control_plane_count":         cfg.Nodes.ControlPlane.Count,
		"control_plane_instance_type": cfg.Nodes.ControlPlane.InstanceType,
		"worker_count":                cfg.Nodes.Workers.Count,
		"worker_instance_type":        cfg.Nodes.Workers.InstanceType,
		"kubernetes_version":          cfg.Kubernetes.Version,
		"rke2_version":                p.getRKE2Version(cfg.Kubernetes.Version),
		"kubernetes_distribution":     cfg.Kubernetes.Distribution,
		"state_bucket":                p.getStateBucket(cfg),
		"enable_nlb":                  true,
		"enable_cloudwatch_logs":      true,
		"enable_session_manager":      true,
		"enable_encryption":           true,
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

// fixProviderPermissions fixes execute permissions on OpenTofu provider binaries
func (p *AWSProvider) fixProviderPermissions() error {
	providersDir := filepath.Join(p.workDir, ".terraform", "providers")

	return filepath.WalkDir(providersDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Ignore errors, continue walking
		}

		// Fix permissions on provider executables
		basename := filepath.Base(path)
		if !d.IsDir() && strings.HasPrefix(basename, "terraform-provider-") {
			if err := os.Chmod(path, 0755); err != nil {
				return nil // Ignore permission errors
			}
		}

		return nil
	})
}

// createS3Bucket creates the S3 bucket for cluster state if it doesn't exist
func (p *AWSProvider) createS3Bucket(cfg *config.ClusterConfig) error {
	bucketName := p.getStateBucket(cfg)
	region := cfg.Provider.Region

	fmt.Printf("[S3] Ensuring bucket exists: %s\n", bucketName)

	// Check if bucket exists
	checkCmd := exec.Command("aws", "s3", "ls", fmt.Sprintf("s3://%s", bucketName), "--region", region)
	if err := checkCmd.Run(); err == nil {
		fmt.Printf("[S3] Bucket already exists: %s\n", bucketName)
		return nil
	}

	// Create bucket
	fmt.Printf("[S3] Creating bucket: %s\n", bucketName)
	createCmd := exec.Command("aws", "s3", "mb", fmt.Sprintf("s3://%s", bucketName), "--region", region)
	createCmd.Stdout = os.Stdout
	createCmd.Stderr = os.Stderr
	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("failed to create S3 bucket: %w", err)
	}

	// Enable encryption
	fmt.Printf("[S3] Enabling encryption on bucket: %s\n", bucketName)
	encryptCmd := exec.Command("aws", "s3api", "put-bucket-encryption",
		"--bucket", bucketName,
		"--server-side-encryption-configuration", `{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"},"BucketKeyEnabled":true}]}`,
		"--region", region)
	encryptCmd.Stdout = os.Stdout
	encryptCmd.Stderr = os.Stderr
	if err := encryptCmd.Run(); err != nil {
		fmt.Printf("Warning: failed to enable encryption: %v\n", err)
	}

	// Enable versioning
	fmt.Printf("[S3] Enabling versioning on bucket: %s\n", bucketName)
	versionCmd := exec.Command("aws", "s3api", "put-bucket-versioning",
		"--bucket", bucketName,
		"--versioning-configuration", "Status=Enabled",
		"--region", region)
	versionCmd.Stdout = os.Stdout
	versionCmd.Stderr = os.Stderr
	if err := versionCmd.Run(); err != nil {
		fmt.Printf("Warning: failed to enable versioning: %v\n", err)
	}

	fmt.Printf("[S3] Bucket ready: %s\n", bucketName)
	return nil
}

// getTerraformOutput retrieves a string output value from Terraform state
func (p *AWSProvider) getTerraformOutput(outputName string) (string, error) {
	cmd := exec.Command("tofu", "output", "-raw", outputName)
	cmd.Dir = p.workDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get output %s: %w", outputName, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// getTerraformOutputJSON retrieves a complex (list/map) output value as a JSON string
func (p *AWSProvider) getTerraformOutputJSON(outputName string) (string, error) {
	cmd := exec.Command("tofu", "output", "-json", outputName)
	cmd.Dir = p.workDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get output %s: %w", outputName, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// updateTLSCertificatesWithNLB updates RKE2 TLS certificates to include NLB DNS name
func (p *AWSProvider) updateTLSCertificatesWithNLB(cfg *config.ClusterConfig) error {
	fmt.Println("\n[Phase 2] Updating TLS certificates with NLB DNS...")

	// Get NLB DNS name from Terraform outputs
	nlbDNS, err := p.getTerraformOutput("nlb_dns_name")
	if err != nil || nlbDNS == "" {
		return fmt.Errorf("NLB not enabled or DNS not available: %w", err)
	}

	fmt.Printf("[Phase 2] NLB DNS: %s\n", nlbDNS)

	// Get control plane instance IDs (list output, needs JSON format)
	controlPlaneIDs, err := p.getTerraformOutputJSON("control_plane_instance_ids")
	if err != nil {
		return fmt.Errorf("failed to get control plane instance IDs: %w", err)
	}

	// Parse instance IDs (JSON array format)
	var instanceIDs []string
	if err := json.Unmarshal([]byte(controlPlaneIDs), &instanceIDs); err != nil {
		return fmt.Errorf("failed to parse instance IDs: %w", err)
	}

	if len(instanceIDs) == 0 {
		return fmt.Errorf("no control plane instances found")
	}

	fmt.Printf("[Phase 2] Updating %d control plane nodes...\n", len(instanceIDs))

	// Wait for instances to be ready for SSM
	fmt.Println("[Phase 2] Waiting for SSM agent to be ready (30s)...")
	cmd := exec.Command("sleep", "30")
	cmd.Run()

	// Update each control plane node
	for i, instanceID := range instanceIDs {
		fmt.Printf("[Phase 2] Updating node %d/%d: %s\n", i+1, len(instanceIDs), instanceID)
		if err := p.updateNodeTLSCert(instanceID, nlbDNS, cfg.Provider.Region); err != nil {
			fmt.Printf("Warning: Failed to update node %s: %v\n", instanceID, err)
			continue
		}
	}

	fmt.Println("[Phase 2] ✅ TLS certificates updated successfully!")
	fmt.Println("[Phase 2] Cluster is now accessible via NLB DNS")

	return nil
}

// updateNodeTLSCert updates RKE2 config on a single node and restarts the service
func (p *AWSProvider) updateNodeTLSCert(instanceID, nlbDNS, region string) error {
	// Create update script
	updateScript := fmt.Sprintf(`#!/bin/bash
set -e

echo "Backing up RKE2 config..."
sudo cp /etc/rancher/rke2/config.yaml /etc/rancher/rke2/config.yaml.backup

echo "Adding NLB DNS to TLS SANs..."
if ! grep -q "%s" /etc/rancher/rke2/config.yaml; then
  sudo sed -i '/^tls-san:/a\  - %s' /etc/rancher/rke2/config.yaml
fi

echo "Removing old TLS certificates..."
sudo rm -f /var/lib/rancher/rke2/server/tls/serving-kube-apiserver.crt
sudo rm -f /var/lib/rancher/rke2/server/tls/serving-kube-apiserver.key

echo "Restarting RKE2 to regenerate certificates..."
sudo systemctl restart rke2-server

echo "Waiting for RKE2 to be ready..."
for i in {1..60}; do
  if sudo /var/lib/rancher/rke2/bin/kubectl --kubeconfig /etc/rancher/rke2/rke2.yaml get nodes >/dev/null 2>&1; then
    echo "RKE2 is ready!"
    break
  fi
  sleep 5
done

echo "TLS certificate update complete!"
`, nlbDNS, nlbDNS)

	// Write script to a temp file for SSM to consume
	tmpFile, err := os.CreateTemp("", "rke2-tls-update-*.sh")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(updateScript); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Build JSON parameters with the script commands as an array of strings
	lines := strings.Split(strings.TrimSpace(updateScript), "\n")
	jsonLines, _ := json.Marshal(lines)
	params := fmt.Sprintf(`{"commands":%s}`, string(jsonLines))

	// Send command via SSM
	cmd := exec.Command("aws", "ssm", "send-command",
		"--document-name", "AWS-RunShellScript",
		"--instance-ids", instanceID,
		"--parameters", params,
		"--region", region,
		"--output", "text",
		"--query", "Command.CommandId")

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("failed to send SSM command: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return fmt.Errorf("failed to send SSM command: %w", err)
	}

	commandID := strings.TrimSpace(string(output))

	// Wait for command to complete
	fmt.Printf("  Waiting for update to complete (command: %s)...\n", commandID)
	for i := 0; i < 60; i++ {
		statusCmd := exec.Command("aws", "ssm", "get-command-invocation",
			"--command-id", commandID,
			"--instance-id", instanceID,
			"--region", region,
			"--query", "Status",
			"--output", "text")

		statusOutput, err := statusCmd.Output()
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		status := strings.TrimSpace(string(statusOutput))
		if status == "Success" {
			fmt.Println("  ✓ Update completed successfully")
			return nil
		} else if status == "Failed" || status == "Cancelled" || status == "TimedOut" {
			return fmt.Errorf("command failed with status: %s", status)
		}

		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("timeout waiting for update to complete")
}

// restartWorkerAgents restarts the RKE2 agent on all worker nodes so they
// reconnect using the updated TLS certificates.
func (p *AWSProvider) restartWorkerAgents(cfg *config.ClusterConfig) error {
	workerIDsJSON, err := p.getTerraformOutputJSON("worker_instance_ids")
	if err != nil {
		return fmt.Errorf("failed to get worker instance IDs: %w", err)
	}

	var workerIDs []string
	if err := json.Unmarshal([]byte(workerIDsJSON), &workerIDs); err != nil {
		return fmt.Errorf("failed to parse worker instance IDs: %w", err)
	}

	if len(workerIDs) == 0 {
		fmt.Println("\n[Phase 3] No worker nodes to restart")
		return nil
	}

	fmt.Printf("\n[Phase 3] Restarting RKE2 agent on %d worker nodes...\n", len(workerIDs))

	// Wait for SSM agent to be available on workers
	fmt.Println("[Phase 3] Waiting for SSM agent to be ready (30s)...")
	time.Sleep(30 * time.Second)

	for i, workerID := range workerIDs {
		fmt.Printf("[Phase 3] Restarting worker %d/%d: %s\n", i+1, len(workerIDs), workerID)

		params := `{"commands":["sudo systemctl restart rke2-agent"]}`
		cmd := exec.Command("aws", "ssm", "send-command",
			"--document-name", "AWS-RunShellScript",
			"--instance-ids", workerID,
			"--parameters", params,
			"--region", cfg.Provider.Region,
			"--output", "text",
			"--query", "Command.CommandId")

		output, err := cmd.Output()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				fmt.Printf("  Warning: Failed to restart worker %s: %s\n", workerID, strings.TrimSpace(string(exitErr.Stderr)))
			} else {
				fmt.Printf("  Warning: Failed to restart worker %s: %v\n", workerID, err)
			}
			continue
		}

		commandID := strings.TrimSpace(string(output))
		fmt.Printf("  Sent restart command: %s\n", commandID)
	}

	fmt.Println("[Phase 3] Worker agent restart commands sent")
	fmt.Println("[Phase 3] Workers will rejoin the cluster within 1-2 minutes")
	return nil
}

// GetClusterStatus returns detailed cluster status
func (p *AWSProvider) GetClusterStatus(cfg *config.ClusterConfig) (*ClusterStatus, error) {
	if err := p.setupWorkingDirectory(cfg); err != nil {
		return nil, err
	}

	status := &ClusterStatus{
		Ready:   false,
		Message: "Checking cluster status...",
	}

	// Get API endpoint from Terraform
	apiEndpoint, err := p.getTerraformOutput("kubernetes_api_endpoint")
	if err == nil {
		status.APIEndpoint = apiEndpoint
	}

	// Download kubeconfig
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		status.Message = "Unable to download kubeconfig"
		return status, nil
	}
	defer os.Remove(kubeconfigPath)

	// Check nodes
	cmd := exec.Command("kubectl", "get", "nodes", "-o", "json")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err := cmd.Output()
	if err != nil {
		status.Message = "Unable to connect to API server"
		return status, nil
	}

	// Parse nodes
	var nodesResult struct {
		Items []struct {
			Metadata struct {
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Status struct {
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &nodesResult); err == nil {
		for _, node := range nodesResult.Items {
			isControlPlane := false
			if _, ok := node.Metadata.Labels["node-role.kubernetes.io/control-plane"]; ok {
				isControlPlane = true
				status.ControlPlaneTotal++
			} else {
				status.WorkerTotal++
			}

			// Check if ready
			for _, condition := range node.Status.Conditions {
				if condition.Type == "Ready" && condition.Status == "True" {
					if isControlPlane {
						status.ControlPlaneReady++
					} else {
						status.WorkerReady++
					}
				}
			}
		}
	}

	// Check system pods
	cmd = exec.Command("kubectl", "get", "pods", "-n", "kube-system", "-o", "json")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err = cmd.Output()
	if err == nil {
		var podsResult struct {
			Items []struct {
				Metadata struct {
					Name string `json:"name"`
				} `json:"metadata"`
				Status struct {
					Phase string `json:"phase"`
				} `json:"status"`
			} `json:"items"`
		}

		if err := json.Unmarshal(output, &podsResult); err == nil {
			componentCounts := make(map[string]int)
			componentReady := make(map[string]int)

			for _, pod := range podsResult.Items {
				// Identify component type
				name := pod.Metadata.Name
				component := "other"
				if strings.Contains(name, "coredns") {
					component = "coredns"
				} else if strings.Contains(name, "cilium") {
					component = "cilium"
				} else if strings.Contains(name, "etcd") {
					component = "etcd"
				} else if strings.Contains(name, "kube-apiserver") {
					component = "kube-apiserver"
				}

				componentCounts[component]++
				if pod.Status.Phase == "Running" {
					componentReady[component]++
				}
			}

			// Create component status
			for comp, total := range componentCounts {
				ready := componentReady[comp]
				compStatus := ComponentStatus{
					Name:   comp,
					Status: "healthy",
				}
				if ready == total {
					compStatus.Message = fmt.Sprintf("%d/%d running", ready, total)
				} else {
					compStatus.Status = "degraded"
					compStatus.Message = fmt.Sprintf("%d/%d running", ready, total)
				}
				status.Components = append(status.Components, compStatus)
			}
		}
	}

	// Determine overall readiness
	allNodesReady := status.ControlPlaneReady == status.ControlPlaneTotal &&
		status.WorkerReady == status.WorkerTotal &&
		status.ControlPlaneTotal > 0 &&
		status.WorkerTotal > 0

	if allNodesReady {
		status.Ready = true
		status.Message = "Cluster is healthy"
	} else {
		status.Message = "Cluster is not fully ready"
	}

	return status, nil
}

// downloadKubeconfig downloads the kubeconfig from S3 and returns the path
func (p *AWSProvider) downloadKubeconfig(cfg *config.ClusterConfig) (string, error) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "kubeconfig-*.yaml")
	if err != nil {
		return "", err
	}
	tmpFile.Close()

	// Download from S3
	s3Path := fmt.Sprintf("s3://%s/kubeconfig/%s/rke2.yaml", p.getStateBucket(cfg), cfg.Name)
	cmd := exec.Command("aws", "s3", "cp", s3Path, tmpFile.Name(), "--region", cfg.Provider.Region)
	if err := cmd.Run(); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to download kubeconfig: %w", err)
	}

	// Update server URL to use NLB
	nlbDNS, _ := p.getTerraformOutput("nlb_dns_name")
	if nlbDNS != "" {
		content, err := os.ReadFile(tmpFile.Name())
		if err == nil {
			// Replace private IP with NLB DNS
			updated := strings.ReplaceAll(string(content), "https://10.0.", "https://10.0.")
			// Find and replace the IP
			lines := strings.Split(string(content), "\n")
			for i, line := range lines {
				if strings.Contains(line, "server: https://") {
					lines[i] = fmt.Sprintf("    server: https://%s:6443", nlbDNS)
					break
				}
			}
			updated = strings.Join(lines, "\n")
			os.WriteFile(tmpFile.Name(), []byte(updated), 0600)
		}
	}

	return tmpFile.Name(), nil
}

// ValidateAPIServer checks if the API server is accessible
func (p *AWSProvider) ValidateAPIServer(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", fmt.Errorf("cannot download kubeconfig: %w", err)
	}
	defer os.Remove(kubeconfigPath)

	cmd := exec.Command("kubectl", "cluster-info")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("API server is not responding")
	}

	return "API server is accessible", nil
}

// ValidateNodes checks if all nodes are ready
func (p *AWSProvider) ValidateNodes(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfigPath)

	cmd := exec.Command("kubectl", "get", "nodes", "-o", "json")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get nodes: %w", err)
	}

	var result struct {
		Items []struct {
			Status struct {
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}

	total := len(result.Items)
	ready := 0

	for _, node := range result.Items {
		for _, condition := range node.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				ready++
				break
			}
		}
	}

	if ready < total {
		return "", fmt.Errorf("%d/%d nodes ready", ready, total)
	}

	return fmt.Sprintf("All %d nodes are ready", total), nil
}

// ValidateSystemPods checks if all system pods are running
func (p *AWSProvider) ValidateSystemPods(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfigPath)

	cmd := exec.Command("kubectl", "get", "pods", "-n", "kube-system", "-o", "json")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get pods: %w", err)
	}

	var result struct {
		Items []struct {
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}

	total := len(result.Items)
	running := 0

	for _, pod := range result.Items {
		if pod.Status.Phase == "Running" {
			running++
		}
	}

	if running < total {
		return "", fmt.Errorf("%d/%d pods running", running, total)
	}

	return fmt.Sprintf("All %d system pods are running", total), nil
}

// ValidateEtcd checks etcd cluster health
func (p *AWSProvider) ValidateEtcd(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfigPath)

	// Check if etcd pods are running
	cmd := exec.Command("kubectl", "get", "pods", "-n", "kube-system", "-l", "component=etcd", "-o", "json")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to check etcd: %w", err)
	}

	var result struct {
		Items []struct {
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}

	members := len(result.Items)
	if members == 0 {
		return "etcd is running on control plane nodes", nil
	}

	running := 0
	for _, pod := range result.Items {
		if pod.Status.Phase == "Running" {
			running++
		}
	}

	return fmt.Sprintf("etcd cluster healthy (%d members)", running), nil
}

// ValidateDNS checks DNS functionality
func (p *AWSProvider) ValidateDNS(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfigPath)

	// Check CoreDNS pods
	cmd := exec.Command("kubectl", "get", "pods", "-n", "kube-system", "-l", "k8s-app=kube-dns", "-o", "json")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to check DNS: %w", err)
	}

	var result struct {
		Items []struct {
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}

	running := 0
	for _, pod := range result.Items {
		if pod.Status.Phase == "Running" {
			running++
		}
	}

	if running == 0 {
		return "", fmt.Errorf("no DNS pods running")
	}

	return fmt.Sprintf("DNS is working (%d pods running)", running), nil
}

// ValidateNetworking checks pod networking
func (p *AWSProvider) ValidateNetworking(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfigPath)

	// Check CNI pods (Cilium)
	cmd := exec.Command("kubectl", "get", "pods", "-n", "kube-system", "-l", "k8s-app=cilium", "-o", "json")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to check networking: %w", err)
	}

	var result struct {
		Items []struct {
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}

	running := 0
	for _, pod := range result.Items {
		if pod.Status.Phase == "Running" {
			running++
		}
	}

	if running == 0 {
		return "", fmt.Errorf("no CNI pods running")
	}

	return fmt.Sprintf("Pod networking is operational (%d Cilium pods running)", running), nil
}

// ValidatePodScheduling checks if pods can be scheduled
func (p *AWSProvider) ValidatePodScheduling(cfg *config.ClusterConfig) (string, error) {
	kubeconfigPath, err := p.downloadKubeconfig(cfg)
	if err != nil {
		return "", err
	}
	defer os.Remove(kubeconfigPath)

	// Check if there are any pending pods
	cmd := exec.Command("kubectl", "get", "pods", "--all-namespaces", "--field-selector=status.phase=Pending", "-o", "json")
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to check pod scheduling: %w", err)
	}

	var result struct {
		Items []interface{} `json:"items"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}

	if len(result.Items) > 0 {
		return "", fmt.Errorf("%d pods are pending", len(result.Items))
	}

	return "Pod scheduling is working correctly", nil
}
