package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/tdls-easy-k8s/internal/config"
)

func TestAWSProvider_Name(t *testing.T) {
	p := NewAWSProvider()
	if p.Name() != "aws" {
		t.Errorf("expected 'aws', got %q", p.Name())
	}
}

// validAWSConfig returns a config that passes all non-credential validations.
func validAWSConfig() *config.ClusterConfig {
	return &config.ClusterConfig{
		Provider: config.ProviderConfig{
			Type:   "aws",
			Region: "us-east-1",
			VPC:    config.VPCConfig{CIDR: "10.0.0.0/16"},
		},
		Nodes: config.NodesConfig{
			ControlPlane: config.NodeGroupConfig{Count: 3, InstanceType: "t3.medium"},
			Workers:      config.NodeGroupConfig{Count: 3, InstanceType: "t3.large"},
		},
	}
}

func TestAWSProvider_ValidateConfig_Valid(t *testing.T) {
	t.Skip("Requires AWS credentials - integration test")
	p := NewAWSProvider()
	if err := p.ValidateConfig(validAWSConfig()); err != nil {
		t.Errorf("expected valid config to pass, got: %v", err)
	}
}

func TestAWSProvider_ValidateConfig_WrongType(t *testing.T) {
	p := NewAWSProvider()
	cfg := validAWSConfig()
	cfg.Provider.Type = "vsphere"
	if err := p.ValidateConfig(cfg); err == nil {
		t.Error("expected error for wrong provider type")
	}
}

func TestAWSProvider_ValidateConfig_MissingRegion(t *testing.T) {
	p := NewAWSProvider()
	cfg := validAWSConfig()
	cfg.Provider.Region = ""
	if err := p.ValidateConfig(cfg); err == nil {
		t.Error("expected error for missing region")
	}
}

func TestAWSProvider_ValidateConfig_InvalidRegion(t *testing.T) {
	p := NewAWSProvider()
	cfg := validAWSConfig()
	cfg.Provider.Region = "us-east-11"
	if err := p.ValidateConfig(cfg); err == nil {
		t.Error("expected error for invalid region")
	}
}

func TestValidateVPCCIDR(t *testing.T) {
	tests := []struct {
		name    string
		cidr    string
		wantErr bool
	}{
		{"valid 10.x /16", "10.0.0.0/16", false},
		{"valid 172.16.x /16", "172.16.0.0/16", false},
		{"valid 192.168.x /16", "192.168.0.0/16", false},
		{"valid /20", "10.1.0.0/20", false},
		{"valid /24", "10.0.1.0/24", false},
		{"empty", "", true},
		{"invalid format", "not-a-cidr", true},
		{"prefix too large (/15)", "10.0.0.0/15", true},
		{"prefix too small (/25)", "10.0.0.0/25", true},
		{"public IP", "8.8.8.0/24", true},
		{"public IP /16", "52.0.0.0/16", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVPCCIDR(tt.cidr)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateVPCCIDR(%q) error = %v, wantErr %v", tt.cidr, err, tt.wantErr)
			}
		})
	}
}

func TestValidateInstanceType(t *testing.T) {
	tests := []struct {
		name         string
		instanceType string
		wantErr      bool
	}{
		{"t3.medium", "t3.medium", false},
		{"m5.xlarge", "m5.xlarge", false},
		{"c5.2xlarge", "c5.2xlarge", false},
		{"r6i.large", "r6i.large", false},
		{"empty", "", true},
		{"no dot", "t3medium", true},
		{"uppercase", "T3.Medium", true},
		{"starts with number", "3t.medium", true},
		{"spaces", "t3. medium", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInstanceType("test", tt.instanceType)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateInstanceType(%q) error = %v, wantErr %v", tt.instanceType, err, tt.wantErr)
			}
		})
	}
}

func TestAWSProvider_CreateInfrastructure_MissingName(t *testing.T) {
	t.Skip("Requires AWS credentials and terraform - integration test")
	p := NewAWSProvider()
	cfg := &config.ClusterConfig{
		Name:     "", // Missing name should cause validation error
		Provider: config.ProviderConfig{Type: "aws", Region: "us-east-1"},
		Nodes: config.NodesConfig{
			ControlPlane: config.NodeGroupConfig{Count: 1},
			Workers:      config.NodeGroupConfig{Count: 1},
		},
	}
	err := p.CreateInfrastructure(cfg)
	if err == nil {
		t.Error("expected error for missing cluster name")
	}
}

func TestAWSProvider_DestroyInfrastructure_NoState(t *testing.T) {
	p := NewAWSProvider()
	cfg := &config.ClusterConfig{
		Name:     "nonexistent-cluster",
		Provider: config.ProviderConfig{Type: "aws", Region: "us-east-1"},
	}
	// Clean up any directory created by setupWorkingDirectory
	t.Cleanup(func() {
		homeDir, _ := os.UserHomeDir()
		os.RemoveAll(filepath.Join(homeDir, ".tdls-k8s", "clusters", cfg.Name))
	})
	// Should succeed even if no state exists (idempotent)
	err := p.DestroyInfrastructure(cfg)
	if err != nil {
		t.Errorf("expected no error for nonexistent state, got: %v", err)
	}
}

func TestAWSProvider_GetKubeconfig_MissingCluster(t *testing.T) {
	p := NewAWSProvider()
	cfg := &config.ClusterConfig{
		Name:     "nonexistent-cluster",
		Provider: config.ProviderConfig{Type: "aws", Region: "us-east-1"},
	}
	_, err := p.GetKubeconfig(cfg)
	if err == nil {
		t.Error("expected error for nonexistent cluster")
	}
}

func TestAWSProvider_GetStatus_MissingWorkDir(t *testing.T) {
	p := NewAWSProvider()
	cfg := &config.ClusterConfig{
		Name:     "nonexistent-cluster",
		Provider: config.ProviderConfig{Type: "aws", Region: "us-east-1"},
	}
	status, err := p.GetStatus(cfg)
	// Should return unknown status when working directory doesn't exist
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if status != "unknown" {
		t.Errorf("expected status 'unknown', got %q", status)
	}
}

func TestCleanTerraformSourceFiles(t *testing.T) {
	// Create a temporary working directory with source and runtime files
	workDir := t.TempDir()

	// Create source-originated files (should be removed)
	os.WriteFile(filepath.Join(workDir, "main.tf"), []byte("# main"), 0644)
	os.WriteFile(filepath.Join(workDir, "variables.tf"), []byte("# vars"), 0644)
	os.WriteFile(filepath.Join(workDir, ".gitkeep"), []byte(""), 0644)
	os.MkdirAll(filepath.Join(workDir, "modules", "networking"), 0755)
	os.WriteFile(filepath.Join(workDir, "modules", "networking", "main.tf"), []byte("# net"), 0644)

	// Create runtime files (should be preserved)
	os.WriteFile(filepath.Join(workDir, "terraform.tfstate"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(workDir, "terraform.tfstate.backup"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(workDir, "terraform.tfvars.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(workDir, ".terraform.lock.hcl"), []byte("# lock"), 0644)
	os.MkdirAll(filepath.Join(workDir, ".terraform", "providers"), 0755)
	os.WriteFile(filepath.Join(workDir, ".terraform", "providers", "registry"), []byte("data"), 0644)

	p := &AWSProvider{workDir: workDir}
	if err := p.cleanTerraformSourceFiles(); err != nil {
		t.Fatalf("cleanTerraformSourceFiles() error: %v", err)
	}

	// Source files should be gone
	for _, name := range []string{"main.tf", "variables.tf", ".gitkeep"} {
		if _, err := os.Stat(filepath.Join(workDir, name)); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed", name)
		}
	}
	if _, err := os.Stat(filepath.Join(workDir, "modules")); !os.IsNotExist(err) {
		t.Error("expected modules/ directory to be removed")
	}

	// Runtime files should remain
	for _, name := range []string{"terraform.tfstate", "terraform.tfstate.backup", "terraform.tfvars.json", ".terraform.lock.hcl"} {
		if _, err := os.Stat(filepath.Join(workDir, name)); err != nil {
			t.Errorf("expected %s to be preserved, got error: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(workDir, ".terraform", "providers", "registry")); err != nil {
		t.Error("expected .terraform/ directory to be preserved")
	}
}

func TestCleanTerraformSourceFiles_NoWorkDir(t *testing.T) {
	p := &AWSProvider{workDir: filepath.Join(t.TempDir(), "nonexistent")}
	// Should not error when working directory doesn't exist
	if err := p.cleanTerraformSourceFiles(); err != nil {
		t.Fatalf("expected no error for nonexistent workDir, got: %v", err)
	}
}

// Verify AWSProvider satisfies the Provider interface at compile time.
var _ Provider = (*AWSProvider)(nil)
