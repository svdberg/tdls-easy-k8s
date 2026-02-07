package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadFromFile loads cluster configuration from a YAML file
func LoadFromFile(filepath string) (*ClusterConfig, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ClusterConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults
	applyDefaults(&config)

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// applyDefaults applies default values to the configuration
func applyDefaults(config *ClusterConfig) {
	// Provider defaults
	if config.Provider.Region == "" && config.Provider.Type == "aws" {
		config.Provider.Region = "us-east-1"
	}

	if config.Provider.VPC.CIDR == "" {
		config.Provider.VPC.CIDR = "10.0.0.0/16"
	}

	// Kubernetes defaults
	if config.Kubernetes.Distribution == "" {
		config.Kubernetes.Distribution = "rke2"
	}

	// GitOps defaults
	if config.GitOps.Branch == "" {
		config.GitOps.Branch = "main"
	}

	// Node defaults
	if config.Nodes.ControlPlane.InstanceType == "" {
		config.Nodes.ControlPlane.InstanceType = "t3.medium"
	}

	if config.Nodes.Workers.InstanceType == "" {
		config.Nodes.Workers.InstanceType = "t3.large"
	}
}

// SaveToFile saves the cluster configuration to a YAML file
func SaveToFile(config *ClusterConfig, filepath string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
