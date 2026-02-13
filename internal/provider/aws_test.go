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

func TestAWSProvider_ValidateConfig_Valid(t *testing.T) {
	p := NewAWSProvider()
	cfg := &config.ClusterConfig{
		Provider: config.ProviderConfig{
			Type:   "aws",
			Region: "us-east-1",
		},
	}
	if err := p.ValidateConfig(cfg); err != nil {
		t.Errorf("expected valid config to pass, got: %v", err)
	}
}

func TestAWSProvider_ValidateConfig_WrongType(t *testing.T) {
	p := NewAWSProvider()
	cfg := &config.ClusterConfig{
		Provider: config.ProviderConfig{
			Type:   "vsphere",
			Region: "us-east-1",
		},
	}
	if err := p.ValidateConfig(cfg); err == nil {
		t.Error("expected error for wrong provider type")
	}
}

func TestAWSProvider_ValidateConfig_MissingRegion(t *testing.T) {
	p := NewAWSProvider()
	cfg := &config.ClusterConfig{
		Provider: config.ProviderConfig{
			Type: "aws",
		},
	}
	if err := p.ValidateConfig(cfg); err == nil {
		t.Error("expected error for missing region")
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

// Verify AWSProvider satisfies the Provider interface at compile time.
var _ Provider = (*AWSProvider)(nil)
