package provider

import "github.com/user/tdls-easy-k8s/internal/config"

// Provider defines the interface that all cloud providers must implement
type Provider interface {
	// Name returns the provider name (e.g., "aws", "vsphere")
	Name() string

	// ValidateConfig validates the provider-specific configuration
	ValidateConfig(config *config.ClusterConfig) error

	// CreateInfrastructure creates the cloud infrastructure for the cluster
	CreateInfrastructure(config *config.ClusterConfig) error

	// DestroyInfrastructure destroys the cloud infrastructure
	DestroyInfrastructure(config *config.ClusterConfig) error

	// GetKubeconfig retrieves the kubeconfig for accessing the cluster
	GetKubeconfig(config *config.ClusterConfig) (string, error)

	// GetStatus returns the current status of the infrastructure
	GetStatus(config *config.ClusterConfig) (string, error)
}

// GetProvider returns a provider instance based on the provider type
func GetProvider(providerType string) (Provider, error) {
	switch providerType {
	case "aws":
		return NewAWSProvider(), nil
	case "vsphere":
		return NewVSphereProvider(), nil
	default:
		return nil, ErrUnsupportedProvider
	}
}

// Error definitions
var (
	ErrUnsupportedProvider = &ProviderError{Message: "unsupported provider type"}
)

// ProviderError represents a provider-specific error
type ProviderError struct {
	Message string
}

func (e *ProviderError) Error() string {
	return e.Message
}
