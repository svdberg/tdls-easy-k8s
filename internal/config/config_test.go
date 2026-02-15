package config

import "testing"

func validConfig() *ClusterConfig {
	return &ClusterConfig{
		Name: "test-cluster",
		Provider: ProviderConfig{
			Type:   "aws",
			Region: "us-east-1",
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
}

func TestClusterConfig_Validate_Valid(t *testing.T) {
	cfg := validConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid config to pass validation, got: %v", err)
	}
}

func TestClusterConfig_Validate_MissingName(t *testing.T) {
	cfg := validConfig()
	cfg.Name = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if err.Error() != "cluster name is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClusterConfig_Validate_MissingProviderType(t *testing.T) {
	cfg := validConfig()
	cfg.Provider.Type = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing provider type")
	}
	if err.Error() != "provider type is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClusterConfig_Validate_InvalidProviderType(t *testing.T) {
	cfg := validConfig()
	cfg.Provider.Type = "gcp"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid provider type")
	}
	if err.Error() != "provider type must be 'aws', 'vsphere', or 'hetzner'" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClusterConfig_Validate_ZeroControlPlaneNodes(t *testing.T) {
	cfg := validConfig()
	cfg.Nodes.ControlPlane.Count = 0
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for zero control plane nodes")
	}
	if err.Error() != "at least one control plane node is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClusterConfig_Validate_MissingKubernetesVersion(t *testing.T) {
	cfg := validConfig()
	cfg.Kubernetes.Version = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing kubernetes version")
	}
	if err.Error() != "kubernetes version is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClusterConfig_Validate_VSphereProvider(t *testing.T) {
	cfg := validConfig()
	cfg.Provider.Type = "vsphere"
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected vsphere provider to be valid, got: %v", err)
	}
}

func TestClusterConfig_Validate_VaultInvalidMode(t *testing.T) {
	cfg := validConfig()
	cfg.Components.Vault.Enabled = true
	cfg.Components.Vault.Mode = "invalid"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid vault mode")
	}
	if err.Error() != "vault mode must be 'external' or 'deploy'" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClusterConfig_Validate_VaultExternalMissingAddress(t *testing.T) {
	cfg := validConfig()
	cfg.Components.Vault.Enabled = true
	cfg.Components.Vault.Mode = "external"
	cfg.Components.Vault.Address = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for external vault without address")
	}
	if err.Error() != "vault address is required when mode is 'external'" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClusterConfig_Validate_VaultExternalValid(t *testing.T) {
	cfg := validConfig()
	cfg.Components.Vault.Enabled = true
	cfg.Components.Vault.Mode = "external"
	cfg.Components.Vault.Address = "https://vault.example.com"
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid vault external config, got: %v", err)
	}
}

func TestClusterConfig_Validate_VaultDeployValid(t *testing.T) {
	cfg := validConfig()
	cfg.Components.Vault.Enabled = true
	cfg.Components.Vault.Mode = "deploy"
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid vault deploy config, got: %v", err)
	}
}

func TestClusterConfig_Validate_VaultDisabledSkipsValidation(t *testing.T) {
	cfg := validConfig()
	cfg.Components.Vault.Enabled = false
	cfg.Components.Vault.Mode = "invalid"
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected disabled vault to skip validation, got: %v", err)
	}
}

func TestConfigError_Error(t *testing.T) {
	err := &ConfigError{Message: "something went wrong"}
	if err.Error() != "something went wrong" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}
