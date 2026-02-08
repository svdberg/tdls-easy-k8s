package provider

import (
	"testing"

	"github.com/user/tdls-easy-k8s/internal/config"
)

func TestVSphereProvider_Name(t *testing.T) {
	p := NewVSphereProvider()
	if p.Name() != "vsphere" {
		t.Errorf("expected 'vsphere', got %q", p.Name())
	}
}

func TestVSphereProvider_ValidateConfig_Valid(t *testing.T) {
	p := NewVSphereProvider()
	cfg := &config.ClusterConfig{
		Provider: config.ProviderConfig{Type: "vsphere"},
	}
	if err := p.ValidateConfig(cfg); err != nil {
		t.Errorf("expected valid config to pass, got: %v", err)
	}
}

func TestVSphereProvider_ValidateConfig_WrongType(t *testing.T) {
	p := NewVSphereProvider()
	cfg := &config.ClusterConfig{
		Provider: config.ProviderConfig{Type: "aws"},
	}
	if err := p.ValidateConfig(cfg); err == nil {
		t.Error("expected error for wrong provider type")
	}
}

func TestVSphereProvider_CreateInfrastructure_NotImplemented(t *testing.T) {
	p := NewVSphereProvider()
	cfg := &config.ClusterConfig{}
	err := p.CreateInfrastructure(cfg)
	if err == nil {
		t.Error("expected not-implemented error")
	}
}

func TestVSphereProvider_DestroyInfrastructure_NotImplemented(t *testing.T) {
	p := NewVSphereProvider()
	cfg := &config.ClusterConfig{}
	err := p.DestroyInfrastructure(cfg)
	if err == nil {
		t.Error("expected not-implemented error")
	}
}

func TestVSphereProvider_GetKubeconfig_NotImplemented(t *testing.T) {
	p := NewVSphereProvider()
	cfg := &config.ClusterConfig{}
	_, err := p.GetKubeconfig(cfg)
	if err == nil {
		t.Error("expected not-implemented error")
	}
}

func TestVSphereProvider_GetStatus_NotImplemented(t *testing.T) {
	p := NewVSphereProvider()
	cfg := &config.ClusterConfig{}
	status, err := p.GetStatus(cfg)
	if err == nil {
		t.Error("expected not-implemented error")
	}
	if status != "unknown" {
		t.Errorf("expected status 'unknown', got %q", status)
	}
}

// Verify VSphereProvider satisfies the Provider interface at compile time.
var _ Provider = (*VSphereProvider)(nil)
