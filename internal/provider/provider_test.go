package provider

import "testing"

func TestGetProvider_AWS(t *testing.T) {
	p, err := GetProvider("aws")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if p.Name() != "aws" {
		t.Errorf("expected provider name 'aws', got %q", p.Name())
	}
}

func TestGetProvider_VSphere(t *testing.T) {
	p, err := GetProvider("vsphere")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if p.Name() != "vsphere" {
		t.Errorf("expected provider name 'vsphere', got %q", p.Name())
	}
}

func TestGetProvider_Unsupported(t *testing.T) {
	_, err := GetProvider("gcp")
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
	if err != ErrUnsupportedProvider {
		t.Errorf("expected ErrUnsupportedProvider, got: %v", err)
	}
}

func TestGetProvider_Empty(t *testing.T) {
	_, err := GetProvider("")
	if err == nil {
		t.Fatal("expected error for empty provider type")
	}
}

func TestProviderError_Error(t *testing.T) {
	err := &ProviderError{Message: "test error"}
	if err.Error() != "test error" {
		t.Errorf("expected 'test error', got %q", err.Error())
	}
}
