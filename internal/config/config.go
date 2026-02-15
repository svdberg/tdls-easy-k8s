package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ClusterConfig represents the complete cluster configuration
type ClusterConfig struct {
	Name       string           `yaml:"name"`
	Provider   ProviderConfig   `yaml:"provider"`
	Kubernetes KubernetesConfig `yaml:"kubernetes"`
	Nodes      NodesConfig      `yaml:"nodes"`
	GitOps     GitOpsConfig     `yaml:"gitops"`
	Components ComponentsConfig `yaml:"components"`
}

// ProviderConfig contains cloud provider configuration
type ProviderConfig struct {
	Type     string    `yaml:"type"`               // aws, vsphere, hetzner
	Region   string    `yaml:"region,omitempty"`   // For AWS
	Location string    `yaml:"location,omitempty"` // For Hetzner (fsn1, nbg1, hel1, ash, hil)
	VPC      VPCConfig `yaml:"vpc"`
	// vSphere-specific fields can be added here
	VCenter    string `yaml:"vcenter,omitempty"`
	Datacenter string `yaml:"datacenter,omitempty"`
}

// VPCConfig contains VPC/network configuration
type VPCConfig struct {
	CIDR string `yaml:"cidr"`
}

// KubernetesConfig contains Kubernetes-specific configuration
type KubernetesConfig struct {
	Version      string `yaml:"version"`      // e.g., "1.30"
	Distribution string `yaml:"distribution"` // rke2, k3s
}

// NodesConfig contains node configuration for control plane and workers
type NodesConfig struct {
	ControlPlane NodeGroupConfig `yaml:"controlPlane"`
	Workers      NodeGroupConfig `yaml:"workers"`
}

// NodeGroupConfig represents a group of nodes
type NodeGroupConfig struct {
	Count        int    `yaml:"count"`
	InstanceType string `yaml:"instanceType"` // e.g., t3.medium
}

// GitOpsConfig contains GitOps configuration
type GitOpsConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Repository string `yaml:"repository"` // e.g., github.com/user/cluster-gitops
	Branch     string `yaml:"branch"`
	Path       string `yaml:"path"` // Path in repository, e.g., clusters/production
}

// ComponentsConfig contains configuration for cluster components
type ComponentsConfig struct {
	Traefik         TraefikConfig         `yaml:"traefik"`
	Vault           VaultConfig           `yaml:"vault"`
	ExternalSecrets ExternalSecretsConfig `yaml:"externalSecrets"`
}

// TraefikConfig contains Traefik ingress controller configuration
type TraefikConfig struct {
	Enabled bool   `yaml:"enabled"`
	Version string `yaml:"version"` // e.g., "26.x"
}

// VaultConfig contains Vault configuration
type VaultConfig struct {
	Enabled bool   `yaml:"enabled"`
	Mode    string `yaml:"mode"`    // external or deploy
	Address string `yaml:"address"` // URL for external Vault
}

// ExternalSecretsConfig contains External Secrets Operator configuration
type ExternalSecretsConfig struct {
	Enabled bool `yaml:"enabled"`
}

// Validate validates the cluster configuration
func (c *ClusterConfig) Validate() error {
	if c.Name == "" {
		return &ConfigError{Message: "cluster name is required"}
	}

	if c.Provider.Type == "" {
		return &ConfigError{Message: "provider type is required"}
	}

	if c.Provider.Type != "aws" && c.Provider.Type != "vsphere" && c.Provider.Type != "hetzner" {
		return &ConfigError{Message: "provider type must be 'aws', 'vsphere', or 'hetzner'"}
	}

	if c.Nodes.ControlPlane.Count < 1 {
		return &ConfigError{Message: "at least one control plane node is required"}
	}

	if c.Kubernetes.Version == "" {
		return &ConfigError{Message: "kubernetes version is required"}
	}

	if c.Components.Vault.Enabled {
		if c.Components.Vault.Mode != "external" && c.Components.Vault.Mode != "deploy" {
			return &ConfigError{Message: "vault mode must be 'external' or 'deploy'"}
		}
		if c.Components.Vault.Mode == "external" && c.Components.Vault.Address == "" {
			return &ConfigError{Message: "vault address is required when mode is 'external'"}
		}
	}

	return nil
}

// ConfigError represents a configuration error
type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return e.Message
}

// LoadConfig loads cluster configuration from a YAML file
func LoadConfig(path string) (*ClusterConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg ClusterConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}
