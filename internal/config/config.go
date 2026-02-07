package config

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
	Type   string    `yaml:"type"`   // aws, vsphere
	Region string    `yaml:"region"` // For AWS
	VPC    VPCConfig `yaml:"vpc"`
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

	if c.Provider.Type != "aws" && c.Provider.Type != "vsphere" {
		return &ConfigError{Message: "provider type must be 'aws' or 'vsphere'"}
	}

	if c.Nodes.ControlPlane.Count < 1 {
		return &ConfigError{Message: "at least one control plane node is required"}
	}

	if c.Kubernetes.Version == "" {
		return &ConfigError{Message: "kubernetes version is required"}
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
