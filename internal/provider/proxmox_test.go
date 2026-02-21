package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/tdls-easy-k8s/internal/config"
)

func TestProxmoxProvider_Name(t *testing.T) {
	p := NewProxmoxProvider()
	if p.Name() != "proxmox" {
		t.Errorf("expected 'proxmox', got %q", p.Name())
	}
}

func validProxmoxConfig() *config.ClusterConfig {
	return &config.ClusterConfig{
		Name: "test-proxmox-cluster",
		Provider: config.ProviderConfig{
			Type:      "proxmox",
			Node:      "pve",
			Bridge:    "vmbr0",
			Datastore: "local-lvm",
			VIP:       "192.168.1.200",
		},
		Kubernetes: config.KubernetesConfig{
			Version:      "1.30",
			Distribution: "rke2",
		},
		Nodes: config.NodesConfig{
			ControlPlane: config.NodeGroupConfig{Count: 1},
			Workers:      config.NodeGroupConfig{Count: 2},
		},
	}
}

func TestProxmoxProvider_ValidateConfig_WrongType(t *testing.T) {
	p := NewProxmoxProvider()
	cfg := validProxmoxConfig()
	cfg.Provider.Type = "aws"
	if err := p.ValidateConfig(cfg); err == nil {
		t.Error("expected error for wrong provider type")
	}
}

func TestProxmoxProvider_ValidateConfig_MissingNode(t *testing.T) {
	p := NewProxmoxProvider()
	cfg := validProxmoxConfig()
	cfg.Provider.Node = ""
	// Set env vars so we don't fail on those checks first
	t.Setenv("PROXMOX_VE_ENDPOINT", "https://proxmox.local:8006")
	t.Setenv("PROXMOX_VE_API_TOKEN", "test@pve!provider=xxx")
	err := p.ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for missing node name")
	}
}

func TestProxmoxProvider_ValidateConfig_MissingVIP(t *testing.T) {
	p := NewProxmoxProvider()
	cfg := validProxmoxConfig()
	cfg.Provider.VIP = ""
	t.Setenv("PROXMOX_VE_ENDPOINT", "https://proxmox.local:8006")
	t.Setenv("PROXMOX_VE_API_TOKEN", "test@pve!provider=xxx")
	err := p.ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for missing VIP")
	}
}

func TestProxmoxProvider_ValidateConfig_InvalidVIP(t *testing.T) {
	p := NewProxmoxProvider()
	cfg := validProxmoxConfig()
	cfg.Provider.VIP = "not-an-ip"
	t.Setenv("PROXMOX_VE_ENDPOINT", "https://proxmox.local:8006")
	t.Setenv("PROXMOX_VE_API_TOKEN", "test@pve!provider=xxx")
	err := p.ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for invalid VIP address")
	}
}

func TestProxmoxProvider_ValidateConfig_MissingEndpoint(t *testing.T) {
	p := NewProxmoxProvider()
	cfg := validProxmoxConfig()
	t.Setenv("PROXMOX_VE_ENDPOINT", "")
	t.Setenv("PROXMOX_VE_API_TOKEN", "test@pve!provider=xxx")
	err := p.ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for missing PROXMOX_VE_ENDPOINT")
	}
}

func TestProxmoxProvider_ValidateConfig_MissingAPIToken(t *testing.T) {
	p := NewProxmoxProvider()
	cfg := validProxmoxConfig()
	t.Setenv("PROXMOX_VE_ENDPOINT", "https://proxmox.local:8006")
	t.Setenv("PROXMOX_VE_API_TOKEN", "")
	t.Setenv("PROXMOX_VE_USERNAME", "")
	err := p.ValidateConfig(cfg)
	if err == nil {
		t.Error("expected error for missing API token/username")
	}
}

func TestProxmoxProvider_ValidateConfig_Valid(t *testing.T) {
	p := NewProxmoxProvider()
	cfg := validProxmoxConfig()
	t.Setenv("PROXMOX_VE_ENDPOINT", "https://proxmox.local:8006")
	t.Setenv("PROXMOX_VE_API_TOKEN", "test@pve!provider=xxx")
	if err := p.ValidateConfig(cfg); err != nil {
		t.Errorf("expected valid config to pass, got: %v", err)
	}
}

func TestProxmoxProvider_DestroyInfrastructure_NoState(t *testing.T) {
	p := NewProxmoxProvider()
	cfg := &config.ClusterConfig{
		Name:     "nonexistent-proxmox-cluster",
		Provider: config.ProviderConfig{Type: "proxmox"},
	}
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

func TestProxmoxProvider_GetStatus_MissingWorkDir(t *testing.T) {
	p := NewProxmoxProvider()
	cfg := &config.ClusterConfig{
		Name:     "nonexistent-proxmox-cluster",
		Provider: config.ProviderConfig{Type: "proxmox"},
	}
	status, err := p.GetStatus(cfg)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if status != "unknown" {
		t.Errorf("expected status 'unknown', got %q", status)
	}
}

func TestProxmoxProvider_GetKubeconfig_MissingCluster(t *testing.T) {
	p := NewProxmoxProvider()
	cfg := &config.ClusterConfig{
		Name:     "nonexistent-proxmox-cluster",
		Provider: config.ProviderConfig{Type: "proxmox"},
	}
	_, err := p.GetKubeconfig(cfg)
	if err == nil {
		t.Error("expected error for nonexistent cluster")
	}
}

// Verify ProxmoxProvider satisfies the Provider interface at compile time.
var _ Provider = (*ProxmoxProvider)(nil)
