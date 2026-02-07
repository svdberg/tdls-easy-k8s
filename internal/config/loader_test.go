package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyDefaults_AWSRegion(t *testing.T) {
	cfg := &ClusterConfig{Provider: ProviderConfig{Type: "aws"}}
	applyDefaults(cfg)
	if cfg.Provider.Region != "us-east-1" {
		t.Errorf("expected default AWS region 'us-east-1', got %q", cfg.Provider.Region)
	}
}

func TestApplyDefaults_NoRegionForVSphere(t *testing.T) {
	cfg := &ClusterConfig{Provider: ProviderConfig{Type: "vsphere"}}
	applyDefaults(cfg)
	if cfg.Provider.Region != "" {
		t.Errorf("expected no default region for vsphere, got %q", cfg.Provider.Region)
	}
}

func TestApplyDefaults_PreservesExplicitRegion(t *testing.T) {
	cfg := &ClusterConfig{Provider: ProviderConfig{Type: "aws", Region: "eu-west-1"}}
	applyDefaults(cfg)
	if cfg.Provider.Region != "eu-west-1" {
		t.Errorf("expected region to remain 'eu-west-1', got %q", cfg.Provider.Region)
	}
}

func TestApplyDefaults_VPCCIDR(t *testing.T) {
	cfg := &ClusterConfig{}
	applyDefaults(cfg)
	if cfg.Provider.VPC.CIDR != "10.0.0.0/16" {
		t.Errorf("expected default VPC CIDR '10.0.0.0/16', got %q", cfg.Provider.VPC.CIDR)
	}
}

func TestApplyDefaults_Distribution(t *testing.T) {
	cfg := &ClusterConfig{}
	applyDefaults(cfg)
	if cfg.Kubernetes.Distribution != "rke2" {
		t.Errorf("expected default distribution 'rke2', got %q", cfg.Kubernetes.Distribution)
	}
}

func TestApplyDefaults_GitOpsBranch(t *testing.T) {
	cfg := &ClusterConfig{}
	applyDefaults(cfg)
	if cfg.GitOps.Branch != "main" {
		t.Errorf("expected default gitops branch 'main', got %q", cfg.GitOps.Branch)
	}
}

func TestApplyDefaults_InstanceTypes(t *testing.T) {
	cfg := &ClusterConfig{}
	applyDefaults(cfg)
	if cfg.Nodes.ControlPlane.InstanceType != "t3.medium" {
		t.Errorf("expected default control plane instance type 't3.medium', got %q", cfg.Nodes.ControlPlane.InstanceType)
	}
	if cfg.Nodes.Workers.InstanceType != "t3.large" {
		t.Errorf("expected default worker instance type 't3.large', got %q", cfg.Nodes.Workers.InstanceType)
	}
}

func TestApplyDefaults_PreservesExplicitValues(t *testing.T) {
	cfg := &ClusterConfig{
		Provider: ProviderConfig{
			Type:   "aws",
			Region: "ap-southeast-1",
			VPC:    VPCConfig{CIDR: "172.16.0.0/16"},
		},
		Kubernetes: KubernetesConfig{Distribution: "k3s"},
		GitOps:     GitOpsConfig{Branch: "develop"},
		Nodes: NodesConfig{
			ControlPlane: NodeGroupConfig{InstanceType: "m5.xlarge"},
			Workers:      NodeGroupConfig{InstanceType: "m5.2xlarge"},
		},
	}
	applyDefaults(cfg)
	if cfg.Provider.Region != "ap-southeast-1" {
		t.Errorf("region should not be overwritten, got %q", cfg.Provider.Region)
	}
	if cfg.Provider.VPC.CIDR != "172.16.0.0/16" {
		t.Errorf("VPC CIDR should not be overwritten, got %q", cfg.Provider.VPC.CIDR)
	}
	if cfg.Kubernetes.Distribution != "k3s" {
		t.Errorf("distribution should not be overwritten, got %q", cfg.Kubernetes.Distribution)
	}
	if cfg.GitOps.Branch != "develop" {
		t.Errorf("branch should not be overwritten, got %q", cfg.GitOps.Branch)
	}
	if cfg.Nodes.ControlPlane.InstanceType != "m5.xlarge" {
		t.Errorf("control plane instance type should not be overwritten, got %q", cfg.Nodes.ControlPlane.InstanceType)
	}
	if cfg.Nodes.Workers.InstanceType != "m5.2xlarge" {
		t.Errorf("worker instance type should not be overwritten, got %q", cfg.Nodes.Workers.InstanceType)
	}
}

const validYAML = `name: test-cluster
provider:
  type: aws
  region: us-west-2
kubernetes:
  version: "1.30"
nodes:
  controlPlane:
    count: 3
  workers:
    count: 5
`

func TestLoadFromFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cluster.yaml")
	if err := os.WriteFile(path, []byte(validYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Name != "test-cluster" {
		t.Errorf("expected name 'test-cluster', got %q", cfg.Name)
	}
	if cfg.Provider.Region != "us-west-2" {
		t.Errorf("expected region 'us-west-2', got %q", cfg.Provider.Region)
	}
	if cfg.Nodes.Workers.Count != 5 {
		t.Errorf("expected 5 workers, got %d", cfg.Nodes.Workers.Count)
	}
	// Check defaults were applied
	if cfg.Kubernetes.Distribution != "rke2" {
		t.Errorf("expected default distribution 'rke2', got %q", cfg.Kubernetes.Distribution)
	}
	if cfg.Nodes.ControlPlane.InstanceType != "t3.medium" {
		t.Errorf("expected default instance type 't3.medium', got %q", cfg.Nodes.ControlPlane.InstanceType)
	}
}

func TestLoadFromFile_FileNotFound(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/path/cluster.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadFromFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("{{invalid yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromFile(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadFromFile_ValidationFailure(t *testing.T) {
	// Missing name and provider type
	yaml := `kubernetes:
  version: "1.30"
nodes:
  controlPlane:
    count: 1
`
	dir := t.TempDir()
	path := filepath.Join(dir, "cluster.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFromFile(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSaveToFile(t *testing.T) {
	cfg := &ClusterConfig{
		Name: "save-test",
		Provider: ProviderConfig{
			Type:   "aws",
			Region: "us-east-1",
			VPC:    VPCConfig{CIDR: "10.0.0.0/16"},
		},
		Kubernetes: KubernetesConfig{
			Version:      "1.30",
			Distribution: "rke2",
		},
		Nodes: NodesConfig{
			ControlPlane: NodeGroupConfig{Count: 3, InstanceType: "t3.medium"},
			Workers:      NodeGroupConfig{Count: 3, InstanceType: "t3.large"},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "output.yaml")

	if err := SaveToFile(cfg, path); err != nil {
		t.Fatalf("expected no error saving, got: %v", err)
	}

	// Round-trip: load the saved file and verify
	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("expected no error loading saved file, got: %v", err)
	}
	if loaded.Name != cfg.Name {
		t.Errorf("name mismatch: expected %q, got %q", cfg.Name, loaded.Name)
	}
	if loaded.Provider.Type != cfg.Provider.Type {
		t.Errorf("provider type mismatch: expected %q, got %q", cfg.Provider.Type, loaded.Provider.Type)
	}
	if loaded.Nodes.Workers.Count != cfg.Nodes.Workers.Count {
		t.Errorf("worker count mismatch: expected %d, got %d", cfg.Nodes.Workers.Count, loaded.Nodes.Workers.Count)
	}
}

func TestSaveToFile_InvalidPath(t *testing.T) {
	cfg := &ClusterConfig{Name: "test"}
	err := SaveToFile(cfg, "/nonexistent/dir/file.yaml")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}
